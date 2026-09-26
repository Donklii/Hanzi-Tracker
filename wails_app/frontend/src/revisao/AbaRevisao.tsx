// ----- Seção: Revisão de Hanzis -----
// Orquestrador da sessão de revisão: pede as questões ao backend (ObterQuestoesRevisao), conduz
// o fluxo seleção de modo → sessão → placar e decide o layout de cada questão pela variante
// sorteada no Go. O áudio da revisão usa a variante SUPERÁVEL do pipeline do popup
// (FalarPinyinRevisao + cache SQLite só de palavras), com buffer pré-sintetizado por questão e
// token "só o último pedido toca"; SEM o toggle habilitarLeituraPinyin — aqui o áudio é parte da
// questão.
//
// Gamificação: barra de progresso no topo, pontos com bônus de sequência (🔥),
// jingles sintetizados de acerto/erro/conclusão (../comum/sons.ts, toggle sonsRevisao), banner de
// feedback fixo no rodapé com botão "Continuar" e atalhos de teclado (1–4 escolhem, Enter avança).
import { useEffect, useRef, useState } from 'react';
import './revisao.css';
import { t } from '../i18n/i18n';
import { main, config } from '../../wailsjs/go/models';
import { ObterQuestoesRevisao, ObterQuestoesRevisaoComPrimeira, ObterPrimeiraQuestaoRevisao, FalarPinyinRevisao, InvalidarSintesesTts, ObterClipesCacheTts, RegistrarRespostaRevisao, RegistrarRespostaFrase, ObterSugestoesAprendidoLote, ObterProgressoRevisaoPalavras, ObterFocoRevisao, AddVocab, ObterArvoreJornada, ObterProgressoJornada, ObterQuestoesJornada, ObterQuestoesJornadaComPrimeira, ObterPrimeiraQuestaoJornada, RegistrarRevisaoJornadaConcluida } from '../../wailsjs/go/main/App';
import { CanvasDesenho } from '../comum/CanvasDesenho';
import { SelecaoModoRevisao } from './SelecaoModoRevisao';
import { ModalConfigAtividades, SUB_ATIVIDADES_POR_MODO } from './ModalConfigAtividades';
import { OpcoesRevisao } from './OpcoesRevisao';
import { MontagemFrase } from './MontagemFrase';
import { MontagemHanzi } from './MontagemHanzi';
import { RespostaTeclado } from './RespostaTeclado';
import { Pronuncia } from './Pronuncia';
import { FilaPinyin } from './FilaPinyin';
import { BotaoAudio } from './BotaoAudio';
import { PalavraPinyin } from './PalavraPinyin';
import { MetaFrase } from './MetaFrase';
import { QuebraCabecaSignificado } from './QuebraCabecaSignificado';
import { QuebraCabecaFonetica } from './QuebraCabecaFonetica';
import { QuebraCabecaTrio } from './QuebraCabecaTrio';
import { SkeletonRevisao } from './SkeletonRevisao';
import { PlacarRevisao, ProgressoPalavraPlacar, SugestaoAprendido, montarProgressoPlacar, normalizarSugestoes } from './PlacarRevisao';
import { PopupRevisao } from './PopupRevisao';
import { obterUrlImagemHanzi } from './obterImagemHanzi';
import { MapaJornada } from './jornada/MapaJornada';
import { ModalNivelJornada } from './jornada/ModalNivelJornada';
import { ArvoreJornada } from './jornada/tiposJornada';
import { CompreensaoQuestao } from './CompreensaoQuestao';
import { definirSonsRevisaoHabilitados, tocarSomAcerto, tocarSomErro, tocarSomConclusao } from '../comum/sons';

const QUESTOES_POR_SESSAO = 10;

// Pontuação: base por acerto + bônus crescente por manter a sequência (máx. +10).
const PONTOS_POR_ACERTO = 10;
const BONUS_MAXIMO_SEQUENCIA = 10;

const ELOGIOS = ['Excelente!', 'Muito bem!', 'Perfeito!', 'Mandou bem!', '加油! Continue assim!', 'Incrível!'];

// Leitura lenta ("tartaruga") da fonética-frase: velocidade dos clipes, pausa entre as
// palavras e prefixo que distingue essa reprodução nos estados tocando/sintetizando.
const VELOCIDADE_LEITURA_LENTA = 0.8;
const PAUSA_ENTRE_PALAVRAS_MS = 20;
const CHAVE_AUDIO_LENTO = 'lento:';

// Fallback "frase fora do buffer": monta a frase com os clipes das palavras já cacheadas —
// velocidade normal e SEM pausa entre elas, para soar o mais contínuo/fluido possível.
const VELOCIDADE_SEQUENCIA_FRASE = 1.0;

// Reprodução "só o último pedido vale": um áudio que ficou pronto tarde demais — porque o usuário
// pediu outra coisa ou seguiu adiante — não toca. Janela medida a partir do momento do pedido.
const VALIDADE_AUDIO_MS = 8000;

// Retorna true para as variantes de fonética onde é objetivamente necessário ouvir o áudio para responder
function ehVarianteFoneticaQueExigeAudio(variante: string): boolean {
  return (
    variante === 'audio_para_hanzi' ||
    variante === 'hanzi_para_audio' ||
    variante === 'fonetica_traducao' ||
    variante === 'quebracabeca_fonetica' ||
    variante === 'fonetica_frase'
  );
}

interface AbaRevisaoProps {
  abaAtiva: string;
  configuracoesApp: config.Config | null;
  setStatus: (s: string) => void;
  AoClicarNoCartao?: (card: any) => void;
  AtualizarConfiguracao?: (key: keyof config.Config, value: any) => void;
  // Grupo global de foco: o estado vive no App, que também o exibe no cabeçalho da página
  // (GrupoFocoCabecalho); esta aba o carrega/sincroniza e usa o snapshot no placar.
  foco: main.ItemFocoRevisao[];
  setFoco: (itens: main.ItemFocoRevisao[]) => void;
  aoMudarFase?: (fase: FaseRevisao) => void;
}

export type FaseRevisao = 'selecao' | 'carregando' | 'jornada' | 'sessao' | 'placar';

// Sessão vinda da Jornada: qual revisão de qual nível está sendo praticada. null = sessão comum.
interface SessaoJornada {
  nivelId: string;
  revIndex: number;
}

// Palavras-alvo da sessão (dedup): no desenho de palavra multi-hanzi q.hanzi vira o hanzi
// componente e q.palavraFoco é a palavra inteira — o progresso acompanha a PALAVRA.
function palavrasPraticadasDe(questoes: main.QuestaoRevisao[]): string[] {
  return Array.from(new Set(questoes.map(q => q.palavraFoco || q.hanzi))).filter(Boolean);
}

export function AbaRevisao({ abaAtiva, configuracoesApp, setStatus, AoClicarNoCartao, AtualizarConfiguracao, foco, setFoco, aoMudarFase }: AbaRevisaoProps) {
  const [fase, setFase] = useState<FaseRevisao>('selecao');

  useEffect(() => {
    aoMudarFase?.(fase);
  }, [fase, aoMudarFase]);
  const [modo, setModo] = useState('');
  const [popupInfo, setPopupInfo] = useState<{hanzi: string, pinyin: string, significados: string, x: number, y: number} | null>(null);
  const [questoes, setQuestoes] = useState<main.QuestaoRevisao[]>([]);
  const [totalInicial, setTotalInicial] = useState(0);
  const [indiceAtual, setIndiceAtual] = useState(0);
  const [acertos, setAcertos] = useState(0);
  const [mostrarTraducaoFrase, setMostrarTraducaoFrase] = useState(() => {
    return localStorage.getItem('mostrarTraducaoFrase') !== 'false';
  });
  const [sugestoesAprendido, setSugestoesAprendido] = useState<SugestaoAprendido[]>([]);

  // Jornada: árvore + progresso (nivel_id -> revisões concluídas) do mapa, o nível aberto no modal
  // e, durante a sessão, qual revisão da Jornada está em curso. A sessão da Jornada exige acertar
  // TUDO: cada erro re-enfileira a questão em filaErros, que vira a rodada de recuperação.
  const [arvoreJornada, setArvoreJornada] = useState<ArvoreJornada | null>(null);
  const [progressoJornada, setProgressoJornada] = useState<Record<string, number>>({});
  const [nivelAbertoId, setNivelAbertoId] = useState<string | null>(null);
  const [sessaoJornada, setSessaoJornada] = useState<SessaoJornada | null>(null);
  const [filaErros, setFilaErros] = useState<main.QuestaoRevisao[]>([]);
  const [emRecuperacao, setEmRecuperacao] = useState(false);
  // rodadaRecuperacao entra na key da questão: sem ela, uma questão repetida na MESMA posição da
  // fila (2ª rodada de recuperação em diante) não remontaria, e os componentes com estado próprio
  // (quebra-cabeça, montagem de frase, canvas) reapareceriam com a resposta anterior — sem como
  // responder de novo, a sessão travava e a revisão nunca era concluída.
  const [rodadaRecuperacao, setRodadaRecuperacao] = useState(0);
  // Espelho da sessão da Jornada em ref: a conclusão da revisão é gravada a partir daqui, para não
  // depender do estado capturado pelo closure do handler que fechou a sessão.
  const sessaoJornadaRef = useRef<SessaoJornada | null>(null);

  function definirSessaoJornada(sessao: SessaoJornada | null) {
    sessaoJornadaRef.current = sessao;
    setSessaoJornada(sessao);
  }

  // Progresso por palavra para o placar: retrato ANTES da sessão (buscado assim que as questões
  // chegam) e a visão consolidada com os deltas por área (montada ao concluir a última questão).
  const todasQuestoesSessaoRef = useRef<main.QuestaoRevisao[]>([]);
  const progressoAntesRef = useRef<Map<string, main.ProgressoPalavraRevisao> | null>(null);
  const [progressoPlacar, setProgressoPlacar] = useState<ProgressoPalavraPlacar[]>([]);
  const [metaAcertos, setMetaAcertos] = useState(3);
  const [modoCarregando, setModoCarregando] = useState<string | null>(null);
  const [primeiraQuestaoCarregando, setPrimeiraQuestaoCarregando] = useState<main.QuestaoRevisao | null>(null);

  // Grupo global de foco (foco_revisao.go): os caracteres priorizados entre sessões — estado no
  // App (exibido no cabeçalho via GrupoFocoCabecalho), carregado/sincronizado aqui. O snapshot
  // pré-sessão permite destacar no placar quem ENTROU no grupo ao marcar palavras como aprendidas.
  const [focoNovas, setFocoNovas] = useState<main.ItemFocoRevisao[]>([]);
  const focoAntesDaSessaoRef = useRef<string[]>([]);

  // Gamificação: sequência de acertos (combo), melhor sequência da sessão e pontos acumulados.
  const [sequencia, setSequencia] = useState(0);
  const [melhorSequencia, setMelhorSequencia] = useState(0);
  const [pontos, setPontos] = useState(0);
  const [elogio, setElogio] = useState(ELOGIOS[0]); // sorteado a cada acerto (fixo por questão)

  // Resposta da questão atual: null = ainda não respondeu.
  const [indiceEscolhido, setIndiceEscolhido] = useState<number | null>(null);
  const [acertouAtual, setAcertouAtual] = useState<boolean | null>(null);
  // Na revisão por contexto o usuário escolhe COMO responder: opções ou desenho no canvas.
  const [respondendoComDesenho, setRespondendoComDesenho] = useState(false);
  // ...ou digitar livremente a resposta (hanzi/pinyin) — vale em contexto, fonética-frase e ordenação.
  const [respondendoComTeclado, setRespondendoComTeclado] = useState(false);
  // Revelação manual da frase na variante fonetica_traducao (olho de ocultar/mostrar)
  const [revelarFraseManual, setRevelarFraseManual] = useState(false);

  // Áudio TTS: cache local (hanzi -> wav base64) por cima do cache SQLite do Go, para cliques
  // repetidos nem cruzarem a ponte; um único <audio> por vez.
  const audioCacheRef = useRef<Map<string, string>>(new Map());
  // Sínteses em voo (texto -> Promise do base64): dedup entre o pré-carregamento e um clique. As
  // FRASES não são cacheadas no banco (só o buffer em memória), então sem isto duas chamadas à mesma
  // frase — pré-carregar + tocar — sintetizariam em duplicidade no motor.
  const bufferPromessaRef = useRef<Map<string, Promise<string>>>(new Map());
  const audioAtualRef = useRef<HTMLAudioElement | null>(null);
  const [hanziTocando, setHanziTocando] = useState<string | null>(null);
  const [hanziSintetizando, setHanziSintetizando] = useState<string | null>(null);

  const autoAudioTimeoutRef = useRef<number | null>(null);
  const transicaoSessaoTimeoutRef = useRef<number | null>(null);

  // Token de reprodução: "vez" monotônica de QUALQUER áudio em curso (leitura lenta, sequência de
  // frase ou reprodução adiada após síntese). pararAudio a incrementa; um áudio que fica pronto com
  // a vez já vencida não toca — garante que só o ÚLTIMO pedido soe, sem sobrepor vários.
  const leituraLentaRef = useRef(0);
  // Momento do último pedido de reprodução: junto do token, descarta áudio que ficou pronto tarde
  // demais (o usuário já seguiu adiante) — ver VALIDADE_AUDIO_MS.
  const tsReproducaoRef = useRef(0);

  const questaoAtual: main.QuestaoRevisao | undefined = questoes[indiceAtual];
  const respondida = acertouAtual !== null;

  // Os jingles obedecem ao toggle das configurações (ligado por padrão).
  useEffect(() => {
    definirSonsRevisaoHabilitados(configuracoesApp?.sonsRevisao !== false);
  }, [configuracoesApp?.sonsRevisao]);

  // Sair da aba no meio da sessão: volta para a seleção de modo (a sessão não sobrevive).
  // E ao entrar, "desperta" o TTS em background para que a primeira leitura seja rápida.
  useEffect(() => {
    if (abaAtiva !== 'revisao') {
      pararAudio();
      InvalidarSintesesTts().catch(() => {}); // descarta o backlog de síntese ao sair da aba
      if (autoAudioTimeoutRef.current) {
        clearTimeout(autoAudioTimeoutRef.current);
        autoAudioTimeoutRef.current = null;
      }
      setFase('selecao');
    } else {
      // Desperta o motor TTS se habilitado no sistema (carrega modelo em background)
      (window as any).go?.main?.App?.DespertarMotorTts?.();
    }
  }, [abaAtiva]);

  // Limpa timeout e cancela a leitura lenta no unmount
  useEffect(() => {
    return () => {
      leituraLentaRef.current++;
      if (autoAudioTimeoutRef.current) clearTimeout(autoAudioTimeoutRef.current);
    };
  }, []);

  // Carrega o grupo de foco na tela de seleção (a consulta já sincroniza a rotação no backend).
  useEffect(() => {
    if (abaAtiva === 'revisao' && fase === 'selecao') {
      ObterFocoRevisao()
        .then(itens => setFoco(itens || []))
        .catch(() => setFoco([]));
    }
  }, [abaAtiva, fase]);

  // O stepper de tamanho e o interruptor de priorização (painel de foco no cabeçalho, visível
  // em qualquer fase) alteram a config; a re-sincronização com debounce faz os chips refletirem
  // a mudança na hora — o atraso cobre cliques em rajada no stepper e o SaveConfig ainda em voo
  // no backend.
  useEffect(() => {
    if (abaAtiva !== 'revisao') return;
    const timer = window.setTimeout(() => {
      ObterFocoRevisao()
        .then(itens => setFoco(itens || []))
        .catch(() => {});
    }, 300);
    return () => window.clearTimeout(timer);
  }, [configuracoesApp?.tamanhoFocoRevisao, configuracoesApp?.tamanhoFocoAutomatico, configuracoesApp?.priorizarEstudoRevisao]);

  // Atalhos de teclado: 1–4 escolhem a opção; Enter/Espaço continuam.
  // O preventDefault no Enter também evita o clique nativo de um botão que tenha ficado focado
  // (senão a questão avançaria duas vezes).
  useEffect(() => {
    if (fase !== 'sessao' || abaAtiva !== 'revisao') return;

    function aoTeclar(e: KeyboardEvent) {
      if (acertouAtual !== null && (e.key === 'Enter' || e.key === ' ')) {
        e.preventDefault();
        proximaQuestao();
        return;
      }

      if (acertouAtual === null && ['1', '2', '3', '4'].indexOf(e.key) !== -1) {
        const q = questoes[indiceAtual];
        const numOpcoes = q?.opcoes?.length || 0;
        const mostrandoOpcoes = q && numOpcoes > 0 &&
          q.modo !== 'desenho' && !respondendoComTeclado &&
          !(q.variante === 'contexto' && respondendoComDesenho);
        if (mostrandoOpcoes) {
          const keyNum = parseInt(e.key);
          if (keyNum <= numOpcoes) {
            e.preventDefault();
            escolherOpcao(keyNum - 1);
          }
        }
      }
    }

    window.addEventListener('keydown', aoTeclar);
    return () => window.removeEventListener('keydown', aoTeclar);
  }, [fase, abaAtiva, acertouAtual, indiceAtual, questoes, respondendoComDesenho, respondendoComTeclado]);

  // Pré-carrega o áudio da questão atual assim que ela for renderizada,
  // para que o auto-play ao acertar seja instantâneo (evitando latência de síntese).
  useEffect(() => {
    if (fase === 'sessao' && questoes[indiceAtual]) {
      const q = questoes[indiceAtual];
      let textoParaPrecarregar = '';
      if (q.variante === 'contexto' || q.variante === 'traducao_contexto' || q.variante === 'desenho_contexto' || q.variante === 'ordenacao' || q.variante === 'fonetica_frase' || q.variante === 'pronuncia_frase' || q.variante === 'pronuncia_sequencia' || q.variante === 'fonetica_traducao' || q.variante === 'fonetica_fila_pinyin') {
        textoParaPrecarregar = q.fraseOriginal;
      } else if (q.variante === 'hanzi_para_significado' || q.variante === 'significado_para_hanzi' || q.variante === 'desenho_memoria' || q.variante === 'desenho_componente' || q.variante === 'desenho_montagem' || q.variante === 'pronuncia_baralho' || q.variante === 'pronuncia_tipo' || q.variante === 'hanzi_para_pinyin' || q.variante === 'audio_para_hanzi' || q.variante === 'hanzi_para_audio') {
        textoParaPrecarregar = q.hanzi;
      } else if (q.variante === 'quebracabeca_significado' || q.variante === 'quebracabeca_fonetica' || q.variante === 'quebracabeca_trio') {
        // Pré-carrega o áudio de todas as 6 opções do quebra-cabeça (palavras: vão para o cache).
        q.opcoes.forEach(opt => {
          if (opt.hanzi) obterAudioBuffer(opt.hanzi).catch(() => {});
        });
      }

      // A frase é pré-sintetizada e guardada no buffer em memória (não no banco): pronta na hora
      // quando o usuário clicar para ouvir. obterAudioBuffer dedup contra o clique.
      if (textoParaPrecarregar) {
        obterAudioBuffer(textoParaPrecarregar).catch(() => {});
      }

      // Fonética-frase / Fonética-tradução: pré-carrega também os clipes por palavra (leitura lenta "tartaruga")
      if (q.variante === 'fonetica_frase' || q.variante === 'fonetica_traducao' || q.variante === 'fonetica_fila_pinyin') {
        const palavras = (q.fraseOriginalSegmentada || [])
          .filter(t => t.ehChines && t.texto)
          .map(t => t.texto);
        palavras.reduce(
          (fila, palavra) => fila.then(() => obterAudioBuffer(palavra).then(() => {}, () => {})),
          Promise.resolve()
        );
      }
    }
  }, [indiceAtual, fase, questoes, configuracoesApp?.motorTtsAtivo]);

  if (abaAtiva !== 'revisao') return null;

  // ----- Fluxo da sessão -----

  function iniciarSessao(modoEscolhido: string, ehExemplo: boolean = false) {
    if (modoEscolhido === 'jornada') {
      abrirJornada();
      return;
    }

    setFase('carregando');
    setModo(modoEscolhido);
    setModoCarregando(modoEscolhido);
    setPrimeiraQuestaoCarregando(null);
    definirSessaoJornada(null);
    focoAntesDaSessaoRef.current = foco.map(f => f.hanzi);
    setFocoNovas([]);

    const qtdQuestoes = ehExemplo
      ? 1
      : configuracoesApp?.revisaoQuantidadeQuestoes ?? QUESTOES_POR_SESSAO;

    const modoParaBackend = ehExemplo ? `exemplo:${modoEscolhido}` : modoEscolhido;

    // Busca rápida da PRIMEIRA questão (~3ms) para exibir o Skeleton exato enquanto o resto é gerado
    ObterPrimeiraQuestaoRevisao(modoParaBackend)
      .then(q0 => {
        if (q0 && q0.variante) {
          setPrimeiraQuestaoCarregando(q0);
          return ObterQuestoesRevisaoComPrimeira(modoParaBackend, qtdQuestoes, q0);
        }
        return ObterQuestoesRevisao(modoParaBackend, qtdQuestoes);
      })
      .then((qs: main.QuestaoRevisao[]) => iniciarQuestoes(qs))
      .catch((err: any) => {
        setStatus(t('⚠️ Revisão: {erro}', { erro: String(err) }));
        setFase('selecao');
      });
  }

  // iniciarQuestoes zera o estado da sessão e entra na fase de questões. Serve a qualquer origem
  // de questões (modo comum ou Jornada).
  function iniciarQuestoes(qs: main.QuestaoRevisao[]) {
    if (transicaoSessaoTimeoutRef.current) {
      clearTimeout(transicaoSessaoTimeoutRef.current);
      transicaoSessaoTimeoutRef.current = null;
    }

    todasQuestoesSessaoRef.current = [...qs];
    const primeira = qs.length > 0 ? qs[0] : null;
    setPrimeiraQuestaoCarregando(primeira);
    setQuestoes(qs);
    setTotalInicial(qs.length);
    setIndiceAtual(0);
    setAcertos(0);
    setSequencia(0);
    setMelhorSequencia(0);
    setPontos(0);
    setSugestoesAprendido([]);
    setProgressoPlacar([]);
    setFilaErros([]);
    setEmRecuperacao(false);
    setRodadaRecuperacao(0);

    // Retrato do progresso ANTES da sessão, em background: o placar compara com o retrato
    // pós-sessão para destacar o que avançou em cada área. Falha aqui só apaga os deltas.
    progressoAntesRef.current = null;
    const palavras = palavrasPraticadasDe(qs);
    if (palavras.length > 0) {
      ObterProgressoRevisaoPalavras(palavras)
        .then(retrato => {
          progressoAntesRef.current = new Map((retrato.palavras || []).map(p => [p.hanzi, p]));
        })
        .catch(() => { progressoAntesRef.current = null; });
    }

    prepararQuestao();

    // Transição fluida (200ms): permite ao SkeletonRevisao renderizar com fidelidade a primeira
    // atividade recebida (qs[0]) antes de apresentar os componentes interativos.
    transicaoSessaoTimeoutRef.current = window.setTimeout(() => {
      setFase('sessao');
    }, 200);
  }

  // ----- Jornada -----

  // abrirJornada carrega a árvore e o progresso e vai para o mapa. As revisões de cada nível já
  // chegam substituídas pela disponibilidade de motor de voz (RevisoesEfetivas, no backend).
  function abrirJornada() {
    setFase('carregando');
    setModo('jornada');
    setModoCarregando('jornada');
    setPrimeiraQuestaoCarregando(null);
    setNivelAbertoId(null);
    Promise.all([ObterArvoreJornada(), ObterProgressoJornada()])
      .then(([arvore, progresso]) => {
        setArvoreJornada(arvore as ArvoreJornada);
        setProgressoJornada((progresso as Record<string, number>) || {});
        setFase('jornada');
      })
      .catch((err: any) => {
        setStatus(t('⚠️ Jornada: {erro}', { erro: String(err) }));
        setFase('selecao');
      });
  }

  function comecarRevisaoJornada(nivelId: string, revIndex: number) {
    setFase('carregando');
    setModo('jornada');
    setModoCarregando('jornada');
    setPrimeiraQuestaoCarregando(null);
    setNivelAbertoId(null);
    definirSessaoJornada({ nivelId, revIndex });
    focoAntesDaSessaoRef.current = foco.map(f => f.hanzi);
    setFocoNovas([]);

    // Busca rápida da PRIMEIRA questão da Jornada (~3ms) para exibir o Skeleton exato enquanto o resto é gerado
    ObterPrimeiraQuestaoJornada(nivelId, revIndex)
      .then(q0 => {
        if (q0 && q0.variante) {
          setPrimeiraQuestaoCarregando(q0);
          return ObterQuestoesJornadaComPrimeira(nivelId, revIndex, q0);
        }
        return ObterQuestoesJornada(nivelId, revIndex);
      })
      .then(qs => iniciarQuestoes(qs))
      .catch((err: any) => {
        setStatus(t('⚠️ Jornada: {erro}', { erro: String(err) }));
        definirSessaoJornada(null);
        setFase('jornada');
      });
  }

  // voltarAoMapaJornada recarrega o progresso (a revisão recém-concluída precisa aparecer no anel
  // do nó) e volta ao mapa.
  function voltarAoMapaJornada() {
    definirSessaoJornada(null);
    setFase('jornada');
    ObterProgressoJornada()
      .then(progresso => setProgressoJornada((progresso as Record<string, number>) || {}))
      .catch((err: any) => console.error('Erro ao recarregar o progresso da Jornada:', err));
  }

  function prepararQuestao() {
    if (autoAudioTimeoutRef.current) {
      clearTimeout(autoAudioTimeoutRef.current);
      autoAudioTimeoutRef.current = null;
    }
    setIndiceEscolhido(null);
    setAcertouAtual(null);
    setRespondendoComDesenho(false);
    setRespondendoComTeclado(false);
    setRevelarFraseManual(false);
    setMostrarTraducaoFrase(localStorage.getItem('mostrarTraducaoFrase') !== 'false');
    pararAudio();
    // Nova questão = novo contexto de TTS: descarta as sínteses da questão anterior que ainda
    // estejam na fila do motor, para ele não moer o áudio de uma frase que o usuário já passou.
    InvalidarSintesesTts().catch(() => {});
  }

  function registrarResposta(acertou: boolean, itensAcertados?: string[], foiPulada: boolean = false) {
    if (acertouAtual !== null) return; // já respondida (ex.: clique duplo)
    setAcertouAtual(acertou);
    setMostrarTraducaoFrase(true); // Desoculta automaticamente após responder, mas sem salvar no localStorage

    const q = questoes[indiceAtual];
    if (q) {
      if (q.variante === 'quebracabeca_significado' || q.variante === 'quebracabeca_fonetica' || q.variante === 'quebracabeca_trio') {
        // Credita individualmente cada palavra do quebra-cabeça nas categorias praticadas
        q.opcoes.forEach((opt: any) => {
          const foiAcerto = itensAcertados ? itensAcertados.includes(opt.hanzi) : acertou;
          if (q.variante === 'quebracabeca_trio') {
            RegistrarRespostaRevisao(opt.hanzi, opt.pinyin, opt.definicao, 'significado', foiAcerto)
              .catch((err: any) => console.error("Erro ao registrar resposta da revisão (quebra-cabeça trio significado):", err));
            RegistrarRespostaRevisao(opt.hanzi, opt.pinyin, opt.definicao, 'fonetica', foiAcerto)
              .catch((err: any) => console.error("Erro ao registrar resposta da revisão (quebra-cabeça trio fonética):", err));
          } else {
            const categoriaModo = q.variante === 'quebracabeca_significado' ? 'significado' : (q.variante === 'quebracabeca_fonetica' ? 'fonetica' : q.modo);
            RegistrarRespostaRevisao(opt.hanzi, opt.pinyin, opt.definicao, categoriaModo, foiAcerto)
              .catch((err: any) => console.error("Erro ao registrar resposta da revisão (quebra-cabeça):", err));
          }
        });
      } else {
        const deveContar = q.modo !== 'ordenacao' && q.variante !== 'fonetica_frase' && !(q.modo === 'pronuncia' && q.variante === 'pronuncia_frase');
        if (deveContar) {
          RegistrarRespostaRevisao(q.hanzi, q.pinyin, q.definicao, q.modo, acertou)
            .catch((err: any) => console.error("Erro ao registrar resposta da revisão:", err));
        }
      }
    }

    if (acertou) {
      const novaSequencia = sequencia + 1;
      setSequencia(novaSequencia);
      if (novaSequencia > melhorSequencia) setMelhorSequencia(novaSequencia);

      const bonus = Math.min((novaSequencia - 1) * 2, BONUS_MAXIMO_SEQUENCIA);
      setPontos(p => p + PONTOS_POR_ACERTO + bonus);
      // Acerto na recuperação não conta no placar: a questão já havia sido errada de primeira.
      if (!emRecuperacao) setAcertos(a => a + 1);
      setElogio(ELOGIOS[Math.floor(Math.random() * ELOGIOS.length)]);
      tocarSomAcerto(novaSequencia - 1); // o tom sobe com o combo

      const q = questoes[indiceAtual];
      if (q) {
        if (q.variante === 'contexto' || q.variante === 'traducao_contexto' || q.variante === 'desenho_contexto' || q.variante === 'ordenacao' || q.variante === 'fonetica_frase' || q.variante === 'pronuncia_frase' || q.variante === 'pronuncia_sequencia') {
          autoAudioTimeoutRef.current = window.setTimeout(() => tocarAudio(q.fraseOriginal), 400);
        } else if (q.variante === 'hanzi_para_significado' || q.variante === 'significado_para_hanzi' || q.variante === 'desenho_memoria' || q.variante === 'desenho_componente' || q.variante === 'desenho_montagem' || q.variante === 'pronuncia_baralho' || q.variante === 'pronuncia_tipo' || q.variante === 'hanzi_para_pinyin' || q.variante === 'audio_para_hanzi' || q.variante === 'hanzi_para_audio') {
          autoAudioTimeoutRef.current = window.setTimeout(() => tocarAudio(q.hanzi), 400);
        }

        // Ao montar a frase com sucesso, premia TODAS as palavras dela: a ordenação credita
        // significado + contexto; a fonética credita fonética. Um único chamado por frase.
        if (q.variante === 'ordenacao' || q.variante === 'ordenacao_traducao') {
          RegistrarRespostaFrase(q.fraseOriginalSegmentada || [], ['significado', 'contexto'], true)
            .catch((err: any) => console.error('Erro ao creditar palavras da frase (ordenação):', err));
        } else if (q.variante === 'fonetica_frase' || q.variante === 'fonetica_traducao' || q.variante === 'fonetica_fila_pinyin') {
          RegistrarRespostaFrase(q.fraseOriginalSegmentada || [], ['fonetica'], true)
            .catch((err: any) => console.error('Erro ao creditar palavras da frase (fonética):', err));
        }
      }
    } else {
      setSequencia(0);
      tocarSomErro();
      // Na Jornada (ou quando revisarErradasAoFinal estiver ativo), a questão errada volta para
      // uma rodada de recuperação no final da revisão (uma CÓPIA, para não vazar estado).
      // Se a questão foi PULADA pelo usuário, ela NÃO deve reaparecer para correção!
      const deveRevisarErradas = !!sessaoJornada || (configuracoesApp?.revisarErradasAoFinal ?? true);
      if (deveRevisarErradas && q && !foiPulada) {
        const copia = main.QuestaoRevisao.createFrom(q);
        setFilaErros(fila => [...fila, copia]);
      }
    }
  }

  function escolherOpcao(indice: number) {
    const q = questoes[indiceAtual];
    if (!q || acertouAtual !== null) return;
    setIndiceEscolhido(indice);
    registrarResposta(!!q.opcoes[indice]?.correta);
  }

  function pularQuestaoFonetica() {
    if (acertouAtual !== null) return;
    pararAudio();
    registrarResposta(false, undefined, true);
  }

  function proximaQuestao() {
    if (indiceAtual + 1 >= questoes.length) {
      // Se houver questões erradas na fila (Jornada ou quando a configuração de revisar erradas estiver ativa),
      // inicia a rodada de recuperação.
      const deveRevisarErradas = !!sessaoJornada || (configuracoesApp?.revisarErradasAoFinal ?? true);
      if (deveRevisarErradas && filaErros.length > 0) {
        setQuestoes(filaErros);
        setFilaErros([]);
        setEmRecuperacao(true);
        setRodadaRecuperacao(r => r + 1);
        setIndiceAtual(0);
        prepararQuestao();
        return;
      }

      tocarSomConclusao();

      // A revisão da Jornada só conta como concluída aqui, com a fila zerada (o backend ignora
      // índice repetido ou fora de ordem, então reprisar a revisão não infla o progresso).
      const sessao = sessaoJornadaRef.current;
      if (sessao) {
        // Avanço otimista: o mapa e o modal já mostram a próxima revisão liberada; a releitura
        // logo abaixo reconcilia com o que o banco realmente gravou.
        setProgressoJornada(atual => ({
          ...atual,
          [sessao.nivelId]: Math.max(atual[sessao.nivelId] || 0, sessao.revIndex + 1),
        }));
        RegistrarRevisaoJornadaConcluida(sessao.nivelId, sessao.revIndex)
          .then(() => ObterProgressoJornada())
          .then(progresso => setProgressoJornada((progresso as Record<string, number>) || {}))
          .catch((err: any) => {
            setStatus(t('⚠️ Não foi possível salvar o progresso da Jornada: {erro}', { erro: String(err) }));
          });
      }

      // Sugestões de APRENDIDO + retrato pós-sessão em paralelo; qualquer falha não trava o
      // placar (a seção correspondente só deixa de aparecer). Exibe todas as palavras da revisão completa.
      const questoesCompletas = todasQuestoesSessaoRef.current.length > 0 ? todasQuestoesSessaoRef.current : questoes;
      const palavrasPraticadas = palavrasPraticadasDe(questoesCompletas);
      const modosDaSessao = (modo === 'geral' || modo === 'jornada') ? [] : [modo];
      Promise.all([
        ObterSugestoesAprendidoLote(palavrasPraticadas, modosDaSessao).catch((err: any) => {
          console.error('Erro ao obter sugestões de aprendido:', err);
          return [];
        }),
        ObterProgressoRevisaoPalavras(palavrasPraticadas).catch((err: any) => {
          console.error('Erro ao obter o progresso das palavras da sessão:', err);
          return null;
        }),
      ]).then(([sugs, retrato]) => {
        setSugestoesAprendido(normalizarSugestoes(sugs || []));
        if (retrato) {
          setMetaAcertos(retrato.meta || 3);
          setProgressoPlacar(montarProgressoPlacar(retrato, progressoAntesRef.current));
        }
        setFase('placar');
      });
      return;
    }
    setIndiceAtual(i => i + 1);
    prepararQuestao();
  }

  // ----- Áudio (revisão fonética e desenho guiado) -----

  function pararAudio() {
    leituraLentaRef.current++; // cancela a leitura lenta em curso, se houver
    if (audioAtualRef.current) {
      audioAtualRef.current.pause();
      audioAtualRef.current = null;
    }
    setHanziTocando(null);
  }

  // Toca o áudio de um texto. Detecta a FRASE da questão atual e a trata de forma inteligente
  // (buffer pré-sintetizado → sequência de palavras já cacheadas → aguardar com aviso); qualquer
  // outro texto é uma PALAVRA isolada (peça, opção, hanzi-foco) e vai pela síntese direta + cache.
  function tocarAudio(hanzi: string) {
    if (!hanzi) return;

    const q = questaoAtual;
    if (q && hanzi === q.fraseOriginal) {
      tocarAudioFrase(q.fraseOriginal, q.fraseOriginalSegmentada);
      return;
    }

    pararAudio();
    const vez = ++leituraLentaRef.current;
    tsReproducaoRef.current = Date.now();

    const emCache = audioCacheRef.current.get(hanzi);
    if (emCache) {
      reproduzir(hanzi, emCache);
      return;
    }

    setHanziSintetizando(hanzi);
    obterAudioBuffer(hanzi)
      .then(b64 => {
        if (!b64) return;
        if (!reproducaoAindaValida(vez)) return; // já pediram outro áudio ou passou tempo demais
        reproduzir(hanzi, b64);
      })
      .catch((err: any) => setStatus(t('⚠️ Áudio da revisão: {erro}', { erro: String(err) })))
      .finally(() => setHanziSintetizando(atual => (atual === hanzi ? null : atual)));
  }

  // Só o pedido de reprodução mais recente pode tocar, e só se ainda for fresco: descarta áudios que
  // ficaram prontos depois que o usuário pediu outra coisa (token) ou tarde demais (janela de tempo).
  function reproducaoAindaValida(vez: number): boolean {
    return leituraLentaRef.current === vez && (Date.now() - tsReproducaoRef.current) <= VALIDADE_AUDIO_MS;
  }

  // Palavras chinesas da frase: uma por segmento; sem segmentação, cai para caractere a caractere.
  // Base da verificação de cache e da leitura em sequência.
  function palavrasChinesasDe(frase: string, segmentos: main.PalavraRevisao[] | undefined): string[] {
    const palavras = (segmentos || []).filter(s => s.ehChines && s.texto).map(s => s.texto);
    if (palavras.length > 0) return palavras;
    return [...frase].filter(ch => /[\u4e00-\u9fff]/.test(ch));
  }

  // Toca a FRASE da questão. Caminho feliz: já pré-sintetizada no buffer → toca na hora. Fora do
  // buffer (síntese ainda em voo), faz uma verificação RÁPIDA no banco: se TODAS as palavras
  // separadas já estão em cache, toca-as em sequência (frase o mais fluida possível); senão, aguarda
  // a frase carregar por completo sem tocar nada — só um aviso —, deixando o buffer pronto para o
  // próximo clique.
  async function tocarAudioFrase(frase: string, segmentos: main.PalavraRevisao[] | undefined) {
    if (!frase) return;
    pararAudio();

    const emBuffer = audioCacheRef.current.get(frase);
    if (emBuffer) {
      reproduzir(frase, emBuffer);
      return;
    }

    const vez = ++leituraLentaRef.current;
    tsReproducaoRef.current = Date.now();
    const palavras = palavrasChinesasDe(frase, segmentos);
    if (palavras.length === 0) {
      aguardarFraseComAviso(frase, vez);
      return;
    }

    // Verificação rápida: só leitura do banco, não bloqueia atrás da síntese da frase em voo.
    let clipes: string[] = [];
    try {
      clipes = await ObterClipesCacheTts(palavras, configuracoesApp?.motorTtsAtivo || '');
    } catch {
      clipes = [];
    }
    if (!reproducaoAindaValida(vez)) return; // outra reprodução assumiu durante a consulta

    if (clipes && clipes.length > 0) {
      // Palavras em cache: monta a frase tocando-as em sequência, o mais fluido possível. Uma vez
      // iniciada, a sequência só é cortada por um novo pedido (token), nunca pela janela de tempo —
      // uma frase longa pode passar de VALIDADE_AUDIO_MS e ainda assim deve terminar de tocar.
      setHanziTocando(frase);
      for (const clipe of clipes) {
        if (leituraLentaRef.current !== vez) return;
        if (clipe) await reproduzirClipe(clipe, VELOCIDADE_SEQUENCIA_FRASE);
      }
      if (leituraLentaRef.current === vez) {
        setHanziTocando(atual => (atual === frase ? null : atual));
      }
      return;
    }

    // Nada em cache: aguarda a frase carregar por completo e a toca automaticamente ao ficar pronta.
    aguardarFraseComAviso(frase, vez);
  }

  // Fallback da frase fora do buffer e sem palavras em cache: garante que a síntese da frase inteira
  // esteja em curso, avisa e, ao ficar pronta, TOCA automaticamente — desde que o pedido ainda seja o
  // último e não tenha passado tempo demais (reproducaoAindaValida). Se estiver superado, só deixa o
  // buffer preenchido para o próximo clique. Não toca sequência com lacunas: aguardar soa melhor.
  function aguardarFraseComAviso(frase: string, vez: number) {
    setStatus(t('⏳ Preparando o áudio da frase… já já fica pronto.'));
    setHanziSintetizando(frase);
    obterAudioBuffer(frase)
      .then(b64 => {
        if (!b64) return;
        if (!reproducaoAindaValida(vez)) return; // usuário seguiu adiante ou demorou demais
        reproduzir(frase, b64);
      })
      .catch((err: any) => setStatus(t('⚠️ Áudio da frase: {erro}', { erro: String(err) })))
      .finally(() => setHanziSintetizando(atual => (atual === frase ? null : atual)));
  }

  function reproduzir(hanzi: string, b64: string) {
    const audio = new Audio('data:audio/wav;base64,' + b64);
    audioAtualRef.current = audio;
    setHanziTocando(hanzi);
    audio.onended = () => setHanziTocando(atual => (atual === hanzi ? null : atual));
    audio.play().catch(() => setHanziTocando(null));
  }

  // Busca o WAV (base64) no buffer em memória; senão sintetiza no Go (FalarPinyinRevisao — variante
  // SUPERÁVEL que descarta a si mesma se a revisão mudou de questão enquanto esperava na fila do
  // motor; só cacheia no banco as PALAVRAS do dicionário) e guarda. Chamadas concorrentes ao mesmo
  // texto compartilham a MESMA síntese em voo (bufferPromessaRef) — sem isso, uma frase (não cacheada
  // no banco) seria sintetizada em duplicidade quando o pré-carregamento e um clique caíssem juntos.
  function obterAudioBuffer(texto: string): Promise<string> {
    const pronto = audioCacheRef.current.get(texto);
    if (pronto) return Promise.resolve(pronto);
    const emVoo = bufferPromessaRef.current.get(texto);
    if (emVoo) return emVoo;

    const promessa = FalarPinyinRevisao(texto, configuracoesApp?.motorTtsAtivo || '')
      .then(b64 => {
        if (b64) audioCacheRef.current.set(texto, b64);
        return b64 || '';
      })
      .finally(() => { bufferPromessaRef.current.delete(texto); });
    bufferPromessaRef.current.set(texto, promessa);
    return promessa;
  }

  // Toca um clipe e resolve quando ele termina (ou quando outra reprodução o pausa).
  function reproduzirClipe(b64: string, velocidade: number): Promise<void> {
    return new Promise(resolve => {
      const audio = new Audio('data:audio/wav;base64,' + b64);
      audio.playbackRate = velocidade;
      audioAtualRef.current = audio;
      audio.onended = () => resolve();
      audio.onpause = () => resolve();
      audio.onerror = () => resolve();
      audio.play().catch(() => resolve());
    });
  }

  // Leitura "tartaruga" da fonética-frase: fala palavra por palavra, mais devagar e com uma
  // pausa suave entre elas, para o ouvido isolar cada som antes de montar a frase.
  async function tocarAudioLento(frase: string, segmentos: main.PalavraRevisao[] | undefined) {
    if (!frase) return;
    pararAudio();
    const vez = ++leituraLentaRef.current;
    const chave = CHAVE_AUDIO_LENTO + frase;

    // Uma palavra chinesa por clipe; sem segmentação, cai para caractere a caractere.
    const palavras = palavrasChinesasDe(frase, segmentos);
    if (palavras.length === 0) return;

    setHanziSintetizando(chave);
    let clipes: string[];
    try {
      clipes = await Promise.all(palavras.map(p => obterAudioBuffer(p)));
    } catch (err) {
      setStatus(t('⚠️ Áudio da revisão: {erro}', { erro: String(err) }));
      return;
    } finally {
      setHanziSintetizando(atual => (atual === chave ? null : atual));
    }
    if (leituraLentaRef.current !== vez) return;

    setHanziTocando(chave);
    for (const clipe of clipes) {
      if (leituraLentaRef.current !== vez) return;
      if (clipe) await reproduzirClipe(clipe, VELOCIDADE_LEITURA_LENTA);
      if (leituraLentaRef.current !== vez) return;
      await new Promise(r => setTimeout(r, PAUSA_ENTRE_PALAVRAS_MS));
    }
    if (leituraLentaRef.current === vez) {
      setHanziTocando(atual => (atual === chave ? null : atual));
    }
  }

  // ----- Renderização -----

  if (fase === 'selecao') {
    const ttsAtivo = !!configuracoesApp?.motorTtsAtivo && configuracoesApp.motorTtsAtivo !== 'nenhum';
    const sttAtivo = !!configuracoesApp?.motorSttAtivo && configuracoesApp.motorSttAtivo !== 'nenhum';
    return (
      <SelecaoModoRevisao
        aoEscolherModo={(m) => iniciarSessao(m, false)}
        aoEscolherExemplo={(chave) => iniciarSessao(chave, true)}
        ttsAtivo={ttsAtivo}
        sttAtivo={sttAtivo}
        configuracoesApp={configuracoesApp}
        AtualizarConfiguracao={AtualizarConfiguracao}
        foco={foco}
      />
    );
  }

  if (fase === 'carregando') {
    return (
      <SkeletonRevisao
        modo={modoCarregando}
        questao={primeiraQuestaoCarregando}
        modosDesativados={configuracoesApp?.modosRevisaoGeralDesativados}
      />
    );
  }

  if (fase === 'jornada') {
    if (!arvoreJornada) return null;
    const ramoAberto = nivelAbertoId
      ? arvoreJornada.ramos.find(r => r.niveis.some(n => n.id === nivelAbertoId))
      : undefined;
    const nivelAberto = ramoAberto?.niveis.find(n => n.id === nivelAbertoId);

    return (
      <>
        <MapaJornada
          arvore={arvoreJornada}
          progresso={progressoJornada}
          aoAbrirNivel={setNivelAbertoId}
          aoVoltar={() => { setNivelAbertoId(null); setFase('selecao'); }}
        />
        {ramoAberto && nivelAberto && (
          <ModalNivelJornada
            nivel={nivelAberto}
            ramo={ramoAberto}
            revisoesFeitas={progressoJornada[nivelAberto.id] || 0}
            aoComecar={revIndex => comecarRevisaoJornada(nivelAberto.id, revIndex)}
            aoFechar={() => setNivelAbertoId(null)}
          />
        )}
      </>
    );
  }

  if (fase === 'placar') {
    return (
      <PlacarRevisao
        acertos={acertos}
        total={totalInicial || questoes.length}
        modo={modo}
        pontos={pontos}
        melhorSequencia={melhorSequencia}
        sugestoes={sugestoesAprendido}
        progresso={progressoPlacar}
        meta={metaAcertos}
        focoNovas={focoNovas}
        aoRepetir={() => (sessaoJornada ? comecarRevisaoJornada(sessaoJornada.nivelId, sessaoJornada.revIndex) : iniciarSessao(modo))}
        aoTrocarModo={() => (sessaoJornada ? voltarAoMapaJornada() : setFase('selecao'))}
        aoAdicionarComoAprendida={(hanzi, pinyin, significado) =>
          // A Promise volta para o placar, que conduz o feedback do botão (salvando → sucesso).
          AddVocab(hanzi, pinyin, significado, 'aprendido').then(() =>
            // A consulta sincroniza a rotação do foco: a palavra aprendida sai e o próximo da
            // fila entra — destacamos no placar quem acabou de entrar. Falha aqui não desfaz a
            // marcação, então não pode rejeitar a Promise devolvida.
            ObterFocoRevisao()
              .then(itens => {
                const atual = itens || [];
                setFoco(atual);
                setFocoNovas(atual.filter(item => focoAntesDaSessaoRef.current.indexOf(item.hanzi) === -1));
              })
              .catch((err: any) => console.error('Erro ao sincronizar o foco após marcar aprendida:', err))
          )
        }
      />
    );
  }

  if (!questaoAtual) return null;

  // Barra de progresso: conta as questões já RESPONDIDAS (a atual só preenche após responder).
  const percentualProgresso = ((indiceAtual + (respondida ? 1 : 0)) / questoes.length) * 100;
  // Na Jornada, com erros pendentes a fila ainda vai render outra rodada — o botão não promete
  // resultado que não vem.
  const ehUltima = indiceAtual + 1 >= questoes.length && !(sessaoJornada && filaErros.length > 0);
  // O quebra-cabeça precisa de uma arena bem maior que o resto da revisão (640px): a barreira
  // invisível das peças é o próprio tabuleiro, então o container padrão o comprimiria por fora.
  const ehQuebraCabeca =
    questaoAtual.variante === 'quebracabeca_significado' ||
    questaoAtual.variante === 'quebracabeca_fonetica' ||
    questaoAtual.variante === 'quebracabeca_trio';

  return (
    <div className={`revisao-container ${ehQuebraCabeca ? 'revisao-container--quebracabeca' : ''}`}>
      {/* Topo: progresso da sessão + chips de pontos/combo */}
      <div className="revisao-topo">
        <div className="revisao-barra-progresso">
          <div className="revisao-barra-progresso-preenchimento" style={{ width: `${percentualProgresso}%` }}></div>
        </div>
        <div className="revisao-topo-info">
          <span className="revisao-topo-contador">{indiceAtual + 1} / {questoes.length}</span>
          {sequencia >= 2 && (
            <span key={sequencia} className="revisao-chip revisao-chip-combo" title={t('Acertos seguidos')}>🔥 {sequencia}</span>
          )}
          {emRecuperacao && (
            <span className="revisao-chip revisao-chip-recuperacao" title={t('As questões que você errou voltaram para a fila')}>
              {t('⟳ Recuperação de erros')}
            </span>
          )}
          <MetaFrase questao={questaoAtual} respondida={respondida} revelarFraseManual={revelarFraseManual} />
          {questaoAtual.emFoco
            ? <span className="revisao-chip revisao-chip-foco" title={t('Este caractere está no seu grupo de foco')}>{t('🎯 FOCO')}</span>
            : questaoAtual.emEstudo && <span className="revisao-chip revisao-chip-estudo">{t('EM ESTUDO')}</span>}
        </div>
      </div>

      {/* key = rodada + índice: reinicia a animação a cada questão nova E força a remontagem quando
          a mesma posição da fila é reaproveitada numa rodada de recuperação (senão o componente da
          atividade voltaria com a resposta anterior já preenchida). */}
      <div key={`${rodadaRecuperacao}-${indiceAtual}`} className="revisao-questao">
        {RenderizarEnunciado()}
        {RenderizarResposta()}
        {!respondida && ehVarianteFoneticaQueExigeAudio(questaoAtual.variante) && (
          <div className="revisao-pular-linha" style={{ display: 'flex', justifyContent: 'center', marginTop: '16px' }}>
            <button
              className="revisao-btn-pular-texto"
              onClick={pularQuestaoFonetica}
              disabled={respondida}
              title={t('Pular atividade (falhar instantaneamente)')}
            >
              ⏭ {t('Pular atividade')}
            </button>
          </div>
        )}
      </div>

      {/* Banner de feedback fixo no rodapé */}
      {respondida && (
        <div className={`revisao-banner ${acertouAtual ? 'acerto' : 'erro'}`}>
          <div className="revisao-banner-icone">{acertouAtual ? '✓' : '✗'}</div>
          <div className="revisao-banner-textos">
            <div className="revisao-banner-titulo">{acertouAtual ? t(elogio) : t('Resposta correta:')}</div>
            <div className="revisao-banner-detalhe">
              <span
                className="revisao-banner-hanzi"
                style={{ cursor: 'pointer', transition: 'color 0.2s, transform 0.2s' }}
                onMouseEnter={(e) => {
                  e.currentTarget.style.color = 'var(--cor-destaque)';
                  const rect = e.currentTarget.getBoundingClientRect();
                  setPopupInfo({
                    pinyin: questaoAtual.pinyin,
                    hanzi: questaoAtual.hanzi,
                    significados: questaoAtual.definicao || '',
                    x: rect.left + rect.width / 2,
                    y: rect.top
                  });
                }}
                onMouseLeave={(e) => {
                  e.currentTarget.style.color = '';
                  setPopupInfo(null);
                }}
                onClick={() => {
                  if (AoClicarNoCartao) {
                    AoClicarNoCartao({
                      Hanzi: questaoAtual.hanzi,
                      Pinyin: questaoAtual.pinyin,
                      significados: questaoAtual.definicao ? questaoAtual.definicao.split('; ') : []
                    });
                  }
                }}
              >
                {questaoAtual.hanzi}
              </span>
              <span className="revisao-banner-pinyin">{questaoAtual.pinyin}</span>
              <span>— {questaoAtual.definicao}</span>
            </div>
          </div>
          <button className={`revisao-banner-continuar ${acertouAtual ? 'acerto' : 'erro'}`} onClick={proximaQuestao}>
            {ehUltima ? t('Ver resultado') : t('Continuar')}
          </button>
        </div>
      )}
      <PopupRevisao info={popupInfo} />
    </div>
  );

  // Enunciado: o que a questão MOSTRA (acima da área de resposta).
  function RenderizarEnunciado() {
    if (!questaoAtual) return null;

    switch (questaoAtual.variante) {
      case 'imagem_para_significado': {
        const urlImagem = obterUrlImagemHanzi(questaoAtual.hanzi);
        return (
          <div className="revisao-imagem-enunciado">
            {urlImagem && (
              <div className="revisao-imagem-moldura">
                <img
                  src={urlImagem}
                  alt={questaoAtual.hanzi}
                  className="revisao-imagem-conteudo"
                />
              </div>
            )}
            <div className="revisao-hanzi-grande">{questaoAtual.hanzi}</div>
          </div>
        );
      }

      case 'hanzi_para_significado':
      case 'hanzi_para_audio':
        return <div className="revisao-hanzi-grande">{questaoAtual.hanzi}</div>;

      case 'hanzi_para_pinyin':
        return (
          <div style={{ display: 'flex', flexDirection: 'column', alignItems: 'center', gap: '12px', margin: '20px 0' }}>
            <div className="revisao-hanzi-grande">{questaoAtual.hanzi}</div>
            <BotaoAudio
              rotulo={t('Ouvir o som')}
              tocando={hanziTocando === questaoAtual.hanzi}
              carregando={hanziSintetizando === questaoAtual.hanzi}
              aoClicar={() => tocarAudio(questaoAtual.hanzi)}
            />
          </div>
        );

      case 'significado_para_hanzi':
      case 'significado_para_imagem':
      case 'significado_para_hanzi_conhecido':
        return <div className="revisao-enunciado-texto">{questaoAtual.definicao}</div>;

      case 'audio_para_hanzi':
        return (
          <div style={{ display: 'flex', justifyContent: 'center', margin: '30px 0' }}>
            <BotaoAudio
              rotulo={t('Ouvir o som')}
              tocando={hanziTocando === questaoAtual.hanzi}
              carregando={hanziSintetizando === questaoAtual.hanzi}
              aoClicar={() => tocarAudio(questaoAtual.hanzi)}
            />
          </div>
        );

      case 'ordenacao':
      case 'ordenacao_traducao':
      case 'fonetica_frase':
      case 'contexto':
      case 'quebracabeca_significado':
      case 'quebracabeca_fonetica':
      case 'quebracabeca_trio':
        return null;

      case 'fonetica_traducao': {
        const revelada = respondida || revelarFraseManual;
        const segmentada = revelada ? questaoAtual.fraseOriginalSegmentada : null;

        return (
          <div className="revisao-frase" style={{ textAlign: 'center', margin: '20px 0' }}>
            <div style={{ display: 'flex', gap: '10px', alignItems: 'center', justifyContent: 'center', flexWrap: 'wrap' }}>
              <BotaoAudio
                rotulo=""
                tocando={hanziTocando === questaoAtual.fraseOriginal}
                carregando={hanziSintetizando === questaoAtual.fraseOriginal}
                aoClicar={() => tocarAudio(questaoAtual.fraseOriginal)}
              />
              <BotaoAudio
                rotulo=""
                lento
                tocando={hanziTocando === CHAVE_AUDIO_LENTO + questaoAtual.fraseOriginal}
                carregando={hanziSintetizando === CHAVE_AUDIO_LENTO + questaoAtual.fraseOriginal}
                aoClicar={() => tocarAudioLento(questaoAtual.fraseOriginal, questaoAtual.fraseOriginalSegmentada)}
              />

              <div style={{ fontSize: '26px', lineHeight: 1.6, fontFamily: 'var(--fonte-hanzi)' }}>
                {revelada ? (
                  (!segmentada || segmentada.length === 0) ? (
                    <span>{questaoAtual.fraseOriginal}</span>
                  ) : (
                    segmentada.map((t, idx) => {
                      if (t.ehChines && t.pinyin) {
                        return (
                          <span
                            key={idx}
                            className={t.ehNaoVista ? 'revisao-palavra-nao-vista' : ''}
                            style={{ cursor: 'pointer', transition: 'color 0.2s' }}
                            onMouseEnter={(e) => {
                              e.currentTarget.style.color = 'var(--cor-destaque)';
                              const rect = e.currentTarget.getBoundingClientRect();
                              setPopupInfo({
                                pinyin: t.pinyin,
                                hanzi: t.texto,
                                significados: t.significados ? t.significados.join(', ') : '',
                                x: rect.left + rect.width / 2,
                                y: rect.top
                              });
                            }}
                            onMouseLeave={(e) => {
                              e.currentTarget.style.color = '';
                              setPopupInfo(null);
                            }}
                            onClick={() => {
                              if (AoClicarNoCartao) {
                                AoClicarNoCartao({ Hanzi: t.texto, Pinyin: t.pinyin, significados: t.significados });
                              }
                            }}
                          >
                            {t.texto}
                          </span>
                        );
                      }
                      return <span key={idx}>{t.texto}</span>;
                    })
                  )
                ) : (
                  <span style={{ letterSpacing: '4px', fontWeight: 600, color: 'var(--cor-texto-secundario)' }}>
                    {[...questaoAtual.fraseOriginal].map(ch => /[\u4e00-\u9fff]/.test(ch) ? '_' : ch).join(' ')}
                  </span>
                )}
              </div>

              <button
                className="revisao-botao-olho"
                onClick={() => setRevelarFraseManual(r => !r)}
                title={revelarFraseManual ? t("Ocultar frase") : t("Visualizar frase")}
              >
                {revelarFraseManual ? (
                  <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                    <path d="M9.88 9.88a3 3 0 1 0 4.24 4.24"/>
                    <path d="M10.73 5.08A10.43 10.43 0 0 1 12 5c7 0 10 7 10 7a13.16 13.16 0 0 1-1.67 2.68"/>
                    <path d="M6.61 6.61A13.52 13.52 0 0 0 2 12s3 7 10 7a9.74 9.74 0 0 0 5.39-1.61"/>
                    <line x1="2" x2="22" y1="2" y2="22"/>
                  </svg>
                ) : (
                  <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                    <path d="M2 12s3-7 10-7 10 7 10 7-3 7-10 7-10-7-10-7Z"/>
                    <circle cx="12" cy="12" r="3"/>
                  </svg>
                )}
              </button>
            </div>
          </div>
        );
      }

      case 'hanzi_frase_para_significado':
        return (
          <div className="revisao-frase" style={{ textAlign: 'center', margin: '20px 0' }}>
            <div style={{ display: 'grid', gridTemplateColumns: '1fr auto 1fr', alignItems: 'center', width: '100%', fontFamily: 'var(--fonte-hanzi)' }}>
              <div style={{ justifySelf: 'end', paddingRight: '12px' }}>
                <BotaoAudio
                  rotulo=""
                  tocando={hanziTocando === questaoAtual.hanzi}
                  carregando={hanziSintetizando === questaoAtual.hanzi}
                  aoClicar={() => tocarAudio(questaoAtual.hanzi)}
                />
              </div>
              <div style={{ justifySelf: 'center', display: 'inline-flex', flexDirection: 'column', alignItems: 'center' }}>
                <span
                  style={{
                    fontSize: '15px',
                    color: 'var(--cor-pinyin, #a0aec0)',
                    fontWeight: 'normal',
                    marginBottom: '2px',
                    userSelect: 'none',
                  }}
                >
                  {questaoAtual.pinyin}
                </span>
                <span
                  style={{
                    fontSize: '32px',
                    lineHeight: 1.1,
                    color: 'var(--cor-texto-primario)',
                    cursor: 'pointer',
                    transition: 'color 0.2s',
                    borderBottom: '1px dotted #475569',
                  }}
                  onMouseEnter={(e) => {
                    e.currentTarget.style.color = 'var(--cor-destaque)';
                    const rect = e.currentTarget.getBoundingClientRect();
                    setPopupInfo({
                      pinyin: questaoAtual.pinyin,
                      hanzi: questaoAtual.hanzi,
                      significados: questaoAtual.definicao || '',
                      x: rect.left + rect.width / 2,
                      y: rect.top
                    });
                  }}
                  onMouseLeave={(e) => {
                    e.currentTarget.style.color = 'var(--cor-texto-primario)';
                    setPopupInfo(null);
                  }}
                  onClick={() => {
                    if (AoClicarNoCartao) {
                      AoClicarNoCartao({ Hanzi: questaoAtual.hanzi, Pinyin: questaoAtual.pinyin, significados: [questaoAtual.definicao] });
                    }
                  }}
                >
                  {questaoAtual.hanzi}
                </span>
              </div>
              <div />
            </div>
          </div>
        );

      case 'desenho_contexto':
      case 'traducao_contexto':
      case 'pronuncia_frase':
      case 'pronuncia_sequencia':
        return (
          <div className="revisao-frase" style={{ textAlign: 'center', margin: '20px 0' }}>
            {questaoAtual.variante !== 'pronuncia_sequencia' && questaoAtual.variante !== 'pronuncia_frase' && (
            <div style={{ display: 'flex', gap: '8px 12px', alignItems: 'flex-end', justifyContent: 'center', flexWrap: 'wrap', fontFamily: 'var(--fonte-hanzi)' }}>
              <BotaoAudio
                rotulo=""
                tocando={hanziTocando === questaoAtual.fraseOriginal}
                carregando={hanziSintetizando === questaoAtual.fraseOriginal}
                aoClicar={() => tocarAudio(questaoAtual.fraseOriginal)}
              />
              <div style={{ display: 'inline-flex', gap: '6px 12px', alignItems: 'flex-end', flexWrap: 'wrap', justifyContent: 'center', fontSize: '28px', lineHeight: 1.2 }}>
                {(() => {
                  const ehTraducaoContexto = questaoAtual.variante === 'traducao_contexto';
                  const segmentada = (respondida || ehTraducaoContexto) ? questaoAtual.fraseOriginalSegmentada : questaoAtual.fraseLacunaSegmentada;
                  if (!segmentada || segmentada.length === 0) {
                    return <span>{(respondida || ehTraducaoContexto) ? questaoAtual.fraseOriginal : questaoAtual.fraseLacuna}</span>;
                  }
                  return segmentada.map((t, idx) => {
                    if (t.ehChines && t.pinyin) {
                      return (
                        <div
                          key={idx}
                          style={{
                            display: 'inline-flex',
                            flexDirection: 'column',
                            alignItems: 'center',
                            margin: '0 2px',
                            verticalAlign: 'bottom'
                          }}
                        >
                          <span
                            className={t.ehNaoVista ? 'revisao-pinyin-nao-visto' : ''}
                            style={{
                              fontSize: '14px',
                              color: t.ehNaoVista ? undefined : 'var(--cor-pinyin, #a0aec0)',
                              fontWeight: 'normal',
                              marginBottom: '2px',
                              userSelect: 'none',
                              lineHeight: 1.2
                            }}
                          >
                            {t.pinyin}
                          </span>
                          <span
                            className={t.ehNaoVista ? 'revisao-palavra-nao-vista' : ''}
                            style={{
                              cursor: 'pointer',
                              transition: 'color 0.2s',
                              color: t.ehNaoVista ? undefined : 'inherit',
                              fontSize: '28px',
                              lineHeight: 1.2,
                              borderBottom: '1px dotted #475569',
                            }}
                            onMouseEnter={(e) => {
                              e.currentTarget.style.color = 'var(--cor-destaque)';
                              const rect = e.currentTarget.getBoundingClientRect();
                              setPopupInfo({
                                pinyin: t.pinyin,
                                hanzi: t.texto,
                                significados: t.significados ? t.significados.join(', ') : '',
                                x: rect.left + rect.width / 2,
                                y: rect.top
                              });
                            }}
                            onMouseLeave={(e) => {
                              e.currentTarget.style.color = t.ehNaoVista ? '' : 'inherit';
                              setPopupInfo(null);
                            }}
                            onClick={() => {
                              if (AoClicarNoCartao) {
                                AoClicarNoCartao({ Hanzi: t.texto, Pinyin: t.pinyin, significados: t.significados });
                              }
                            }}
                          >
                            {t.texto}
                          </span>
                        </div>
                      );
                    }
                    return (
                      <span
                        key={idx}
                        style={{
                          fontSize: '28px',
                          margin: '0 2px',
                          verticalAlign: 'bottom',
                          alignSelf: 'flex-end',
                          color: t.ehChines ? 'inherit' : 'var(--cor-texto-suave)',
                          lineHeight: 1.2
                        }}
                      >
                        {t.texto}
                      </span>
                    );
                  });
                })()}
              </div>
            </div>
            )}
            {questaoAtual.variante !== 'traducao_contexto' && (
            <div style={{ display: 'flex', gap: '8px', alignItems: 'center', justifyContent: 'center', marginTop: '6px', color: 'var(--cor-texto-suave)', fontSize: '13px' }}>
              <span style={{ opacity: mostrarTraducaoFrase || questaoAtual.variante === 'pronuncia_frase' || questaoAtual.variante === 'pronuncia_sequencia' ? 1 : 0.3, transition: 'opacity 0.2s' }}>
                {mostrarTraducaoFrase || questaoAtual.variante === 'pronuncia_frase' || questaoAtual.variante === 'pronuncia_sequencia' ? questaoAtual.fraseTraducao : t('Tradução oculta')}
              </span>
              {questaoAtual.variante !== 'pronuncia_frase' && questaoAtual.variante !== 'pronuncia_sequencia' && (
              <button
                className="revisao-ocultar-traducao-btn"
                onClick={() => {
                  const novo = !mostrarTraducaoFrase;
                  setMostrarTraducaoFrase(novo);
                  localStorage.setItem('mostrarTraducaoFrase', String(novo));
                }}
                title={mostrarTraducaoFrase ? t("Ocultar tradução") : t("Exibir tradução")}
                style={{ 
                  padding: '4px', 
                  opacity: 0.6, 
                  display: 'flex', 
                  color: mostrarTraducaoFrase ? 'var(--cor-destaque)' : 'var(--cor-texto-primario)',
                  background: 'transparent',
                  border: 'none',
                  cursor: 'pointer'
                }}
              >
                {mostrarTraducaoFrase ? (
                  <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                    <path d="M1 12s4-8 11-8 11 8 11 8-4 8-11 8-11-8-11-8z" />
                    <circle cx="12" cy="12" r="3" />
                  </svg>
                ) : (
                  <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                    <path d="M17.94 17.94A10.07 10.07 0 0 1 12 20c-7 0-11-8-11-8a18.45 18.45 0 0 1 5.06-5.94M9.9 4.24A9.12 9.12 0 0 1 12 4c7 0 11 8 11 8a18.5 18.5 0 0 1-2.16 3.19m-6.72-1.07a3 3 0 1 1-4.24-4.24" />
                    <line x1="1" y1="1" x2="23" y2="23" />
                  </svg>
                )}
              </button>
              )}
            </div>
            )}
            {questaoAtual.variante === 'desenho_contexto' && (
              <div style={{ display: 'flex', gap: '12px', alignItems: 'center', justifyContent: 'center', marginTop: '12px' }}>
                <BotaoAudio
                  rotulo={t('Ouvir')}
                  tocando={hanziTocando === questaoAtual.hanzi}
                  carregando={hanziSintetizando === questaoAtual.hanzi}
                  aoClicar={() => tocarAudio(questaoAtual.hanzi)}
                />
                <span style={{ color: 'var(--cor-texto-suave)', fontSize: '14px' }}>{questaoAtual.definicao}</span>
              </div>
            )}
            <div className="revisao-atribuicao" style={{ color: 'var(--cor-texto-suave)', fontSize: '9px', marginTop: '8px', opacity: 0.6 }}>
              {questaoAtual.fraseAtribuicao}
            </div>
          </div>
        );

      case 'desenho_memoria':
        return (
          <div style={{ textAlign: 'center', margin: '16px 0', color: 'var(--cor-texto-suave)', fontSize: '14px' }}>
            {t('Memorize o caractere. Ao prosseguir, ele desaparece e você o desenha de memória.')}
          </div>
        );

      case 'desenho_componente':
        return (
          <div style={{ textAlign: 'center', margin: '16px 0', color: 'var(--cor-texto-suave)', fontSize: '14px' }}>
            {t('Memorize o caractere. Ao prosseguir, uma parte dele desaparece — redesenhe só ela, no lugar, traço a traço.')}
          </div>
        );

      case 'desenho_montagem':
        return (
          <div style={{ textAlign: 'center', margin: '16px 0', color: 'var(--cor-texto-suave)', fontSize: '14px' }}>
            {t('Memorize o caractere. Ao prosseguir, ele vira um baralho de peças — arraste cada peça para o lugar dela no quadro.')}
          </div>
        );

      default:
        return null;
    }
  }

  // Área de resposta: opções, canvas, ou os dois (contexto).
  function RenderizarResposta() {
    if (!questaoAtual) return null;

    if (questaoAtual.variante === 'quebracabeca_significado') {
      return (
        <QuebraCabecaSignificado
          questao={questaoAtual}
          aoConcluir={(acertou, itensAcertados) => registrarResposta(acertou, itensAcertados)}
          aoTocarAudio={tocarAudio}
        />
      );
    }

    if (questaoAtual.variante === 'quebracabeca_fonetica') {
      return (
        <QuebraCabecaFonetica
          questao={questaoAtual}
          aoConcluir={(acertou, itensAcertados) => registrarResposta(acertou, itensAcertados)}
          aoTocarAudio={tocarAudio}
          hanziTocando={hanziTocando}
          hanziSintetizando={hanziSintetizando}
        />
      );
    }

    if (questaoAtual.variante === 'quebracabeca_trio') {
      return (
        <QuebraCabecaTrio
          questao={questaoAtual}
          aoConcluir={(acertou, itensAcertados) => registrarResposta(acertou, itensAcertados)}
          aoTocarAudio={tocarAudio}
          hanziTocando={hanziTocando}
          hanziSintetizando={hanziSintetizando}
        />
      );
    }

    // Modo teclado: substitui a área de resposta por um campo de texto livre (hanzi ou pinyin).
    // Disponível em contexto (escrever o hanzi), fonética-frase e ordenação.
    const tecladoElegivel =
      questaoAtual.variante === 'contexto' ||
      questaoAtual.variante === 'fonetica_frase' ||
      questaoAtual.variante === 'ordenacao';

    if (respondendoComTeclado && tecladoElegivel) {
      return (
        <RespostaTeclado
          questao={questaoAtual}
          escopo={questaoAtual.variante === 'contexto' ? 'hanzi' : 'frase'}
          respondida={respondida}
          acertou={acertouAtual}
          aoConcluir={(acertou) => registrarResposta(acertou)}
          aoTocarAudio={tocarAudio}
          hanziTocando={hanziTocando}
          hanziSintetizando={hanziSintetizando}
          aoVoltar={() => setRespondendoComTeclado(false)}
        />
      );
    }

    if (questaoAtual.variante === 'fonetica_fila_pinyin') {
      return (
        <FilaPinyin
          questao={questaoAtual}
          respondida={respondida}
          acertou={acertouAtual}
          aoConcluir={(acertou) => registrarResposta(acertou)}
          aoTocarAudio={tocarAudio}
          hanziTocando={hanziTocando}
          hanziSintetizando={hanziSintetizando}
          AoClicarNoCartao={AoClicarNoCartao}
        />
      );
    }

    if (questaoAtual.variante === 'fonetica_palavra_pinyin') {
      return (
        <PalavraPinyin
          questao={questaoAtual}
          respondida={respondida}
          acertou={acertouAtual}
          aoConcluir={(acertou) => registrarResposta(acertou)}
          aoTocarAudio={tocarAudio}
          hanziTocando={hanziTocando}
          hanziSintetizando={hanziSintetizando}
        />
      );
    }

    const varianteComOpcoes =
      questaoAtual.variante === 'hanzi_para_significado' ||
      questaoAtual.variante === 'imagem_para_significado' ||
      questaoAtual.variante === 'significado_para_hanzi' ||
      questaoAtual.variante === 'significado_para_imagem' ||
      questaoAtual.variante === 'significado_para_hanzi_conhecido' ||
      questaoAtual.variante === 'hanzi_frase_para_significado' ||
      questaoAtual.variante === 'audio_para_hanzi' ||
      questaoAtual.variante === 'hanzi_para_audio' ||
      questaoAtual.variante === 'hanzi_para_pinyin' ||
      questaoAtual.variante === 'traducao_contexto' ||
      questaoAtual.variante === 'fonetica_traducao' ||
      (questaoAtual.variante === 'contexto' && !respondendoComDesenho);

    if (questaoAtual.variante === 'pronuncia_frase' || questaoAtual.variante === 'pronuncia_sequencia' || questaoAtual.variante === 'pronuncia_baralho' || questaoAtual.variante === 'pronuncia_tipo') {
      return (
        <Pronuncia
          questao={questaoAtual}
          respondida={respondida}
          aoConcluir={(acertou, foiPulada) => registrarResposta(acertou, undefined, foiPulada)}
          aoTocarAudio={tocarAudio}
          hanziTocando={hanziTocando}
          hanziSintetizando={hanziSintetizando}
          AoClicarNoCartao={AoClicarNoCartao}
        />
      );
    }

    if (questaoAtual.variante === 'compreensao' || questaoAtual.variante === 'compreensao_traduzida' || questaoAtual.variante === 'resposta_dialogo') {
      return (
        <CompreensaoQuestao
          questao={questaoAtual}
          respondida={respondida}
          indiceEscolhido={indiceEscolhido}
          aoEscolher={(indice) => escolherOpcao(indice)}
          mostrarTraducaoDireta={questaoAtual.variante === 'compreensao_traduzida'}
          AoClicarNoCartao={AoClicarNoCartao}
          aoTocarAudio={tocarAudio}
          hanziTocando={hanziTocando}
          hanziSintetizando={hanziSintetizando}
        />
      );
    }

    if (questaoAtual.variante === 'ordenacao' || questaoAtual.variante === 'ordenacao_traducao' || questaoAtual.variante === 'fonetica_frase' || questaoAtual.variante === 'contexto') {
      return (
        <>
          <MontagemFrase
            questao={questaoAtual}
            respondida={respondida}
            acertou={acertouAtual}
            aoConcluir={(acertou) => registrarResposta(acertou)}
            aoTocarAudio={tocarAudio}
            hanziTocando={hanziTocando}
            hanziSintetizando={hanziSintetizando}
            aoTocarAudioLento={() => tocarAudioLento(questaoAtual.fraseOriginal, questaoAtual.fraseOriginalSegmentada)}
            audioLentoTocando={hanziTocando === CHAVE_AUDIO_LENTO + questaoAtual.fraseOriginal}
            audioLentoSintetizando={hanziSintetizando === CHAVE_AUDIO_LENTO + questaoAtual.fraseOriginal}
            AoClicarNoCartao={AoClicarNoCartao}
          />
          {!respondida && questaoAtual.variante !== 'ordenacao_traducao' && (
            <div className="revisao-alternar-linha">
              <button className="revisao-alternar-resposta" onClick={() => setRespondendoComTeclado(true)}>
                {t('⌨️ Prefiro digitar a resposta')}
              </button>
            </div>
          )}
        </>
      );
    }

    if (varianteComOpcoes) {
      const tipoConteudo =
        (questaoAtual.variante === 'hanzi_para_significado' || questaoAtual.variante === 'imagem_para_significado' || questaoAtual.variante === 'hanzi_frase_para_significado' || questaoAtual.variante === 'traducao_contexto' || questaoAtual.variante === 'fonetica_traducao') ? 'definicao' :
        questaoAtual.variante === 'significado_para_imagem' ? 'imagem_hanzi' :
        questaoAtual.variante === 'hanzi_para_audio' ? 'audio' :
        questaoAtual.variante === 'hanzi_para_pinyin' ? 'pinyin' : 'hanzi';

      return (
        <>
          <OpcoesRevisao
            opcoes={questaoAtual.opcoes}
            tipoConteudo={tipoConteudo}
            respondida={respondida}
            indiceEscolhido={indiceEscolhido}
            aoEscolher={escolherOpcao}
            aoTocarAudio={tocarAudio}
            hanziTocando={hanziTocando}
            hanziSintetizando={hanziSintetizando}
            vertical={questaoAtual.variante === 'traducao_contexto' || questaoAtual.variante === 'fonetica_traducao'}
          />
          {questaoAtual.variante === 'contexto' && !respondida && (
            <div className="revisao-alternar-linha">
              <button className="revisao-alternar-resposta" onClick={() => setRespondendoComDesenho(true)}>
                {t('✏️ Prefiro desenhar a resposta')}
              </button>
              <button className="revisao-alternar-resposta" onClick={() => setRespondendoComTeclado(true)}>
                {t('⌨️ Prefiro digitar a resposta')}
              </button>
            </div>
          )}
        </>
      );
    }

    if (questaoAtual.variante === 'desenho_montagem') {
      return (
        <MontagemHanzi
          hanzi={questaoAtual.hanzi}
          componentes={questaoAtual.componentesMontagem}
          distratores={questaoAtual.distratoresMontagem}
          aoConcluir={(acertou) => registrarResposta(acertou)}
        />
      );
    }

    // Modos de desenho (e contexto quando o usuário optou pelo canvas)
    return (
      <div style={{ display: 'flex', flexDirection: 'column', alignItems: 'center', gap: '8px' }}>
        <CanvasDesenho
          hanzi={questaoAtual.hanzi}
          modoMemoria={questaoAtual.variante === 'desenho_memoria' || questaoAtual.variante === 'desenho_componente' || questaoAtual.variante === 'desenho_guiado'}
          modoGuiado={questaoAtual.variante === 'desenho_guiado'}
          tracosAlvo={questaoAtual.tracosAlvo}
          aoConcluir={(acertou) => registrarResposta(acertou)}
        />
        {questaoAtual.variante === 'contexto' && !respondida && (
          <button className="revisao-alternar-desenho" style={{ background: 'none', border: 'none', color: 'var(--cor-destaque)', cursor: 'pointer', fontSize: '13px' }}
            onClick={() => setRespondendoComDesenho(false)}>
            {t('↩ Voltar para as opções')}
          </button>
        )}
      </div>
    );
  }
}
