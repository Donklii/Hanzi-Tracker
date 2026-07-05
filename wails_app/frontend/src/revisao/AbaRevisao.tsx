// ----- Seção: Revisão de Hanzis -----
// Orquestrador da sessão de revisão: pede as questões prontas ao backend (ObterQuestoesRevisao),
// conduz o fluxo seleção de modo → sessão → placar e decide o layout de cada questão pela variante
// sorteada no Go. O áudio da revisão fonética usa o mesmo pipeline do popup (FalarPinyin + cache
// SQLite), mas SEM passar pelo toggle habilitarLeituraPinyin — aqui o áudio é parte da questão.
//
// Gamificação (estilo Duolingo): barra de progresso no topo, pontos com bônus de sequência (🔥),
// jingles sintetizados de acerto/erro/conclusão (./sons.ts, toggle sonsRevisao), banner de
// feedback fixo no rodapé com botão "Continuar" e atalhos de teclado (1–4 escolhem, Enter avança).
import { useEffect, useRef, useState } from 'react';
import { main, config } from '../../wailsjs/go/models';
import { ObterQuestoesRevisao, FalarPinyin } from '../../wailsjs/go/main/App';
import { CanvasDesenho } from './CanvasDesenho';
import { SelecaoModoRevisao } from './SelecaoModoRevisao';
import { OpcoesRevisao } from './OpcoesRevisao';
import { BotaoAudio } from './BotaoAudio';
import { PlacarRevisao } from './PlacarRevisao';
import { definirSonsRevisaoHabilitados, tocarSomAcerto, tocarSomErro, tocarSomConclusao } from './sons';

const QUESTOES_POR_SESSAO = 10;

// Pontuação: base por acerto + bônus crescente por manter a sequência (máx. +10).
const PONTOS_POR_ACERTO = 10;
const BONUS_MAXIMO_SEQUENCIA = 10;

const ELOGIOS = ['Excelente!', 'Muito bem!', 'Perfeito!', 'Mandou bem!', '加油! Continue assim!', 'Incrível!'];

interface AbaRevisaoProps {
  abaAtiva: string;
  configuracoesApp: config.Config | null;
  setStatus: (s: string) => void;
}

type FaseRevisao = 'selecao' | 'carregando' | 'sessao' | 'placar';

export function AbaRevisao({ abaAtiva, configuracoesApp, setStatus }: AbaRevisaoProps) {
  const [fase, setFase] = useState<FaseRevisao>('selecao');
  const [modo, setModo] = useState('');
  const [questoes, setQuestoes] = useState<main.QuestaoRevisao[]>([]);
  const [indiceAtual, setIndiceAtual] = useState(0);
  const [acertos, setAcertos] = useState(0);

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

  // Áudio TTS: cache local (hanzi -> wav base64) por cima do cache SQLite do Go, para cliques
  // repetidos nem cruzarem a ponte; um único <audio> por vez.
  const audioCacheRef = useRef<Map<string, string>>(new Map());
  const audioAtualRef = useRef<HTMLAudioElement | null>(null);
  const [hanziTocando, setHanziTocando] = useState<string | null>(null);
  const [hanziSintetizando, setHanziSintetizando] = useState<string | null>(null);

  const autoAudioTimeoutRef = useRef<number | null>(null);

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

  // Limpa timeout no unmount
  useEffect(() => {
    return () => {
      if (autoAudioTimeoutRef.current) clearTimeout(autoAudioTimeoutRef.current);
    };
  }, []);

  // Atalhos de teclado (estilo Duolingo): 1–4 escolhem a opção; Enter/Espaço continuam.
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
        const mostrandoOpcoes = q && q.opcoes && q.opcoes.length === 4 &&
          q.modo !== 'desenho' && !(q.variante === 'contexto' && respondendoComDesenho);
        if (mostrandoOpcoes) {
          e.preventDefault();
          escolherOpcao(parseInt(e.key) - 1);
        }
      }
    }

    window.addEventListener('keydown', aoTeclar);
    return () => window.removeEventListener('keydown', aoTeclar);
  }, [fase, abaAtiva, acertouAtual, indiceAtual, questoes, respondendoComDesenho]);

  // Pré-carrega o áudio da questão atual assim que ela for renderizada,
  // para que o auto-play ao acertar seja instantâneo (evitando latência de síntese).
  useEffect(() => {
    if (fase === 'sessao' && questoes[indiceAtual]) {
      const q = questoes[indiceAtual];
      let textoParaPrecarregar = '';
      if (q.variante === 'contexto' || q.variante === 'desenho_contexto') {
        textoParaPrecarregar = q.fraseOriginal;
      } else if (q.variante === 'hanzi_para_significado' || q.variante === 'significado_para_hanzi' || q.variante === 'desenho_memoria') {
        textoParaPrecarregar = q.hanzi;
      }

      if (textoParaPrecarregar && !audioCacheRef.current.has(textoParaPrecarregar)) {
        FalarPinyin(textoParaPrecarregar, configuracoesApp?.motorTtsAtivo || '')
          .then(b64 => {
            if (b64) audioCacheRef.current.set(textoParaPrecarregar, b64);
          })
          .catch(() => {});
      }
    }
  }, [indiceAtual, fase, questoes, configuracoesApp?.motorTtsAtivo]);

  if (abaAtiva !== 'revisao') return null;

  // ----- Fluxo da sessão -----

  function iniciarSessao(modoEscolhido: string) {
    setFase('carregando');
    setModo(modoEscolhido);
    ObterQuestoesRevisao(modoEscolhido, QUESTOES_POR_SESSAO)
      .then(qs => {
        setQuestoes(qs);
        setIndiceAtual(0);
        setAcertos(0);
        setSequencia(0);
        setMelhorSequencia(0);
        setPontos(0);
        prepararQuestao();
        setFase('sessao');
      })
      .catch((err: any) => {
        setStatus('⚠️ Revisão: ' + String(err));
        setFase('selecao');
      });
  }

  function prepararQuestao() {
    if (autoAudioTimeoutRef.current) {
      clearTimeout(autoAudioTimeoutRef.current);
      autoAudioTimeoutRef.current = null;
    }
    setIndiceEscolhido(null);
    setAcertouAtual(null);
    setRespondendoComDesenho(false);
    pararAudio();
  }

  function registrarResposta(acertou: boolean) {
    if (acertouAtual !== null) return; // já respondida (ex.: clique duplo)
    setAcertouAtual(acertou);

    if (acertou) {
      const novaSequencia = sequencia + 1;
      setSequencia(novaSequencia);
      if (novaSequencia > melhorSequencia) setMelhorSequencia(novaSequencia);

      const bonus = Math.min((novaSequencia - 1) * 2, BONUS_MAXIMO_SEQUENCIA);
      setPontos(p => p + PONTOS_POR_ACERTO + bonus);
      setAcertos(a => a + 1);
      setElogio(ELOGIOS[Math.floor(Math.random() * ELOGIOS.length)]);
      tocarSomAcerto(novaSequencia - 1); // o tom sobe com o combo

      const q = questoes[indiceAtual];
      if (q) {
        if (q.variante === 'contexto' || q.variante === 'desenho_contexto') {
          autoAudioTimeoutRef.current = window.setTimeout(() => tocarAudio(q.fraseOriginal), 400);
        } else if (q.variante === 'hanzi_para_significado' || q.variante === 'significado_para_hanzi' || q.variante === 'desenho_memoria') {
          autoAudioTimeoutRef.current = window.setTimeout(() => tocarAudio(q.hanzi), 400);
        }
      }
    } else {
      setSequencia(0);
      tocarSomErro();
    }
  }

  function escolherOpcao(indice: number) {
    const q = questoes[indiceAtual];
    if (!q || acertouAtual !== null) return;
    setIndiceEscolhido(indice);
    registrarResposta(!!q.opcoes[indice]?.correta);
  }

  function proximaQuestao() {
    if (indiceAtual + 1 >= questoes.length) {
      tocarSomConclusao();
      setFase('placar');
      return;
    }
    setIndiceAtual(i => i + 1);
    prepararQuestao();
  }

  // ----- Áudio (revisão fonética e desenho guiado) -----

  function pararAudio() {
    if (audioAtualRef.current) {
      audioAtualRef.current.pause();
      audioAtualRef.current = null;
    }
    setHanziTocando(null);
  }

  function tocarAudio(hanzi: string) {
    if (!hanzi) return;
    pararAudio();

    const emCache = audioCacheRef.current.get(hanzi);
    if (emCache) {
      reproduzir(hanzi, emCache);
      return;
    }

    setHanziSintetizando(hanzi);
    FalarPinyin(hanzi, configuracoesApp?.motorTtsAtivo || '')
      .then(b64 => {
        if (!b64) return;
        audioCacheRef.current.set(hanzi, b64);
        reproduzir(hanzi, b64);
      })
      .catch((err: any) => setStatus('⚠️ Áudio da revisão: ' + String(err)))
      .finally(() => setHanziSintetizando(null));
  }

  function reproduzir(hanzi: string, b64: string) {
    const audio = new Audio('data:audio/wav;base64,' + b64);
    audioAtualRef.current = audio;
    setHanziTocando(hanzi);
    audio.onended = () => setHanziTocando(atual => (atual === hanzi ? null : atual));
    audio.play().catch(() => setHanziTocando(null));
  }

  // ----- Renderização -----

  if (fase === 'selecao') {
    return <SelecaoModoRevisao aoEscolherModo={iniciarSessao} />;
  }

  if (fase === 'carregando') {
    return <div style={{ color: 'var(--cor-texto-suave)', textAlign: 'center', marginTop: '40px' }}>Preparando questões…</div>;
  }

  if (fase === 'placar') {
    return (
      <PlacarRevisao
        acertos={acertos}
        total={questoes.length}
        modo={modo}
        pontos={pontos}
        melhorSequencia={melhorSequencia}
        aoRepetir={() => iniciarSessao(modo)}
        aoTrocarModo={() => setFase('selecao')}
      />
    );
  }

  if (!questaoAtual) return null;

  // Barra de progresso: conta as questões já RESPONDIDAS (a atual só preenche após responder).
  const percentualProgresso = ((indiceAtual + (respondida ? 1 : 0)) / questoes.length) * 100;
  const ehUltima = indiceAtual + 1 >= questoes.length;

  return (
    <div className="revisao-container">
      {/* Topo: progresso da sessão + chips de pontos/combo */}
      <div className="revisao-topo">
        <div className="revisao-barra-progresso">
          <div className="revisao-barra-progresso-preenchimento" style={{ width: `${percentualProgresso}%` }}></div>
        </div>
        <div className="revisao-topo-info">
          <span className="revisao-topo-contador">{indiceAtual + 1} / {questoes.length}</span>
          {sequencia >= 2 && (
            <span key={sequencia} className="revisao-chip revisao-chip-combo" title="Acertos seguidos">🔥 {sequencia}</span>
          )}
          {questaoAtual.emEstudo && <span className="revisao-chip revisao-chip-estudo">EM ESTUDO</span>}
          <span className="revisao-dica-teclas">1–4 escolhem · Enter continua</span>
        </div>
      </div>

      {/* key = índice: reinicia a animação de entrada a cada questão nova */}
      <div key={indiceAtual} className="revisao-questao">
        {RenderizarEnunciado()}
        {RenderizarResposta()}
      </div>

      {/* Banner de feedback fixo no rodapé (estilo Duolingo) */}
      {respondida && (
        <div className={`revisao-banner ${acertouAtual ? 'acerto' : 'erro'}`}>
          <div className="revisao-banner-icone">{acertouAtual ? '✓' : '✗'}</div>
          <div className="revisao-banner-textos">
            <div className="revisao-banner-titulo">{acertouAtual ? elogio : 'Resposta correta:'}</div>
            <div className="revisao-banner-detalhe">
              <span className="revisao-banner-hanzi">{questaoAtual.hanzi}</span>
              <span className="revisao-banner-pinyin">{questaoAtual.pinyin}</span>
              <span>— {questaoAtual.definicao}</span>
            </div>
          </div>
          <button className={`revisao-banner-continuar ${acertouAtual ? 'acerto' : 'erro'}`} onClick={proximaQuestao}>
            {ehUltima ? 'Ver resultado' : 'Continuar'}
          </button>
        </div>
      )}
    </div>
  );

  // Enunciado: o que a questão MOSTRA (acima da área de resposta).
  function RenderizarEnunciado() {
    if (!questaoAtual) return null;

    switch (questaoAtual.variante) {
      case 'hanzi_para_significado':
      case 'hanzi_para_audio':
        return <div className="revisao-hanzi-grande">{questaoAtual.hanzi}</div>;

      case 'significado_para_hanzi':
        return <div className="revisao-enunciado-texto">{questaoAtual.definicao}</div>;

      case 'audio_para_hanzi':
        return (
          <div style={{ display: 'flex', justifyContent: 'center', margin: '30px 0' }}>
            <BotaoAudio
              rotulo="Ouvir o som"
              tocando={hanziTocando === questaoAtual.hanzi}
              carregando={hanziSintetizando === questaoAtual.hanzi}
              aoClicar={() => tocarAudio(questaoAtual.hanzi)}
            />
          </div>
        );

      case 'desenho_contexto':
      case 'contexto':
        return (
          <div className="revisao-frase" style={{ textAlign: 'center', margin: '20px 0' }}>
            <div style={{ display: 'flex', gap: '8px', alignItems: 'center', justifyContent: 'center' }}>
              <BotaoAudio
                rotulo=""
                tocando={hanziTocando === questaoAtual.fraseOriginal}
                carregando={hanziSintetizando === questaoAtual.fraseOriginal}
                aoClicar={() => tocarAudio(questaoAtual.fraseOriginal)}
              />
              <div style={{ fontSize: '26px', lineHeight: 1.6, fontFamily: 'var(--fonte-hanzi)' }}>
                {respondida ? questaoAtual.fraseOriginal : questaoAtual.fraseLacuna}
              </div>
            </div>
            <div style={{ color: 'var(--cor-texto-suave)', fontSize: '13px', marginTop: '6px' }}>{questaoAtual.fraseTraducao}</div>
            {questaoAtual.variante === 'desenho_contexto' && (
              <div style={{ display: 'flex', gap: '12px', alignItems: 'center', justifyContent: 'center', marginTop: '12px' }}>
                <BotaoAudio
                  rotulo="Ouvir"
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
            Memorize o caractere. Ao prosseguir, ele desaparece e você o desenha de memória.
          </div>
        );

      default:
        return null;
    }
  }

  // Área de resposta: opções, canvas, ou os dois (contexto).
  function RenderizarResposta() {
    if (!questaoAtual) return null;

    const varianteComOpcoes =
      questaoAtual.variante === 'hanzi_para_significado' ||
      questaoAtual.variante === 'significado_para_hanzi' ||
      questaoAtual.variante === 'audio_para_hanzi' ||
      questaoAtual.variante === 'hanzi_para_audio' ||
      (questaoAtual.variante === 'contexto' && !respondendoComDesenho);

    if (varianteComOpcoes) {
      const tipoConteudo =
        questaoAtual.variante === 'hanzi_para_significado' ? 'definicao' :
        questaoAtual.variante === 'hanzi_para_audio' ? 'audio' : 'hanzi';

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
          />
          {questaoAtual.variante === 'contexto' && !respondida && (
            <div style={{ textAlign: 'center', marginTop: '12px' }}>
              <button className="revisao-alternar-desenho" style={{ background: 'none', border: 'none', color: 'var(--cor-destaque)', cursor: 'pointer', fontSize: '13px' }}
                onClick={() => setRespondendoComDesenho(true)}>
                ✏️ Prefiro desenhar a resposta
              </button>
            </div>
          )}
        </>
      );
    }

    // Modos de desenho (e contexto quando o usuário optou pelo canvas)
    return (
      <div style={{ display: 'flex', flexDirection: 'column', alignItems: 'center', gap: '8px' }}>
        <CanvasDesenho
          hanzi={questaoAtual.hanzi}
          modoMemoria={questaoAtual.variante === 'desenho_memoria'}
          aoConcluir={(acertou) => registrarResposta(acertou)}
        />
        {questaoAtual.variante === 'contexto' && !respondida && (
          <button className="revisao-alternar-desenho" style={{ background: 'none', border: 'none', color: 'var(--cor-destaque)', cursor: 'pointer', fontSize: '13px' }}
            onClick={() => setRespondendoComDesenho(false)}>
            ↩ Voltar para as opções
          </button>
        )}
      </div>
    );
  }
}
