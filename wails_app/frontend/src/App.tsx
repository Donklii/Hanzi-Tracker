import { useState, useEffect } from 'react';
import './comum/base.css';
import './casca/casca.css';
import { PainelConfiguracoes } from './configuracoes/PainelConfiguracoes';
import { ModalCartaoDetalhes } from './dicionario/ModalCartaoDetalhes';
import { ModalAdicionarHanzi } from './dicionario/ModalAdicionarHanzi';
import { ModalBuscaPorDesenho } from './dicionario/ModalBuscaPorDesenho';
import { ModalAvisoCompatibilidade } from './comum/ModalAvisoCompatibilidade';
import { ModalNotasVersao } from './notasVersao/ModalNotasVersao';
import { useNotasVersao } from './notasVersao/useNotasVersao';
import { ModalConfirmacao } from './comum/ModalConfirmacao';
import { ModalConflitoNuvem } from './nuvem/ModalConflitoNuvem';
import { useNuvem } from './nuvem/useNuvem';
import { useArmazenamento } from './configuracoes/useArmazenamento';
import { useCatalogos } from './configuracoes/useCatalogos';
import { AbaDescobrimento } from './descobrimento/AbaDescobrimento';
import { SugestaoEstudoPopup } from './descobrimento/SugestaoEstudoPopup';
import { AbaEstudos } from './estudos/AbaEstudos';
import { AbaRevisao } from './revisao/AbaRevisao';
import { GrupoFocoCabecalho } from './revisao/GrupoFocoCabecalho';
import { AbaBuscaGlobal } from './busca/AbaBuscaGlobal';
import { CampoBuscaGlobal } from './busca/CampoBuscaGlobal';
import { useBuscaGlobal } from './busca/useBuscaGlobal';
import { BarraLateral } from './casca/BarraLateral';
import { ABAS, Aba, TITULOS_POR_ABA } from './casca/abas';
import { FiltrarPorTipoHanzi } from './comum/cartoes';
import { STATUS_VOCABULARIO } from './comum/status';
import { useDestaquesTela } from './descobrimento/useDestaquesTela';
import { useRastreamentoMouse } from './descobrimento/useRastreamentoMouse';
import { useRefEspelho } from './comum/useRefEspelho';
import { useLeituraPinyin } from './comum/useLeituraPinyin';
import { config, main, progresso, revisao, tela } from '../wailsjs/go/models';
import { t } from './i18n/i18n';
import { CaptureAndOCR, GetConfig, SaveConfig, AddVocab, RemoveVocab, GetVocab, ShowHighlight, HideHoverPopup, LookupWord, DecomposeCharacter, CaractereCompleto, MarcarVistoSilencioso, GetSystemHardware, GetCaptureResolution, GetSessionImage, GetLastScreenshot, GetLastCards, GetMonitores, GetCotaTraducao, GetCotaGemini, ObterFocoRevisao, ReiniciarAplicativo, ConverterTexto } from "../wailsjs/go/main/App";
import { AtalhosProvider } from './atalhos/AtalhosContext';
import { ModalGuiaAtalhos } from './atalhos/ModalGuiaAtalhos';
import { EventsOn } from "../wailsjs/runtime/runtime";

function AppConteudo() {
  const [abaAtiva, setAbaAtiva] = useState<Aba>(ABAS.Descobrimento);
  const [painelConfigAberto, setPainelConfigAberto] = useState(false);
  const notasVersao = useNotasVersao();

  const [cartoes, setCartoes] = useState<any[]>([]); // Raw OCR result (Descobrimento)
  const [cartoesSecao, setCartoesSecao] = useState<any[]>([]); // Accumulated OCR (Palavras dessa Seção)
  const [cartaoSelecionado, setCartaoSelecionado] = useState<any | null>(null);
  const [imagemModalBase64, setImagemModalBase64] = useState<string | null>(null);
  const [dadosDecomposicao, setDadosDecomposicao] = useState<any>(null);
  
  // Histórico de navegação interno ao modal
  const [historicoModal, setHistoricoModal] = useState<any[]>([]);
  const [indiceHistoricoModal, setIndiceHistoricoModal] = useState(-1);

  const [cartoesVocabulario, setCartoesVocabulario] = useState<progresso.Vocab[]>([]);
  const [status, setStatus] = useState(t('Aguardando...'));
  const [configuracoesApp, setConfiguracoesApp] = useState<config.Config | null>(null);
  const [infoHardware, setInfoHardware] = useState<main.SystemHardware | null>(null);
  const [monitores, setMonitores] = useState<any[]>([]);
  const [resCaptura, setResCaptura] = useState<tela.Resolucao | null>(null);
  const armazenamento = useArmazenamento({
    setStatus,
    aoExcluirTudo: () => {
      catalogos.CarregarModelos();
      CarregarVocabulario();
    },
  });
  const [infoCotaTraducao, setInfoCotaTraducao] = useState<main.InfoCotaTraducao | null>(null);
  const [infoCotaGemini, setInfoCotaGemini] = useState<main.InfoCotaGemini | null>(null);
  const [confirmacao, setConfirmacao] = useState<{
    titulo: string;
    mensagem: string;
    rotuloAcao: string;
    acao: () => void;
    rotuloCancelar?: string;
    cancelarAcao?: () => void;
    rotuloAcao2?: string;
    acao2?: () => void;
    perigoso?: boolean;
  } | null>(null);
  // Sincronização com o Google Drive (aba Armazenamento + modal de conflito da 1ª conexão).
  const nuvem = useNuvem({
    setStatus,
    aoRestaurarDaNuvem: () => {
      CarregarVocabulario();
      armazenamento.CarregarArmazenamento();
      // A restauração troca também o configuracoes.json: sem reler, a tela continuaria mostrando
      // (e regravando) as preferências antigas.
      GetConfig().then(cfg => setConfiguracoesApp(cfg));
    },
  });

  const { termoBuscaGlobal, setTermoBuscaGlobal, resultadosBuscaGlobal } = useBuscaGlobal();
  const [ordenarPorRanking, setOrdenarPorRanking] = useState(false);
  const [buscaGlobalAtiva, setBuscaGlobalAtiva] = useState(false);
  const [scrollTargetHanzi, setScrollTargetHanzi] = useState<string | null>(null);

  const [cartaoEmFoco, setCartaoEmFoco] = useState<any | null>(null);
  // Grupo global de foco da revisão: o estado mora aqui porque o painel é exibido no cabeçalho
  // da página (GrupoFocoCabecalho), mas quem carrega/sincroniza os dados é a AbaRevisao.
  const [focoRevisao, setFocoRevisao] = useState<revisao.ItemFocoRevisao[]>([]);
  // Re-sincroniza o grupo de foco após adições manuais pelo pop-up (a consulta já rotaciona no backend).
  const recarregarFoco = () => {
    ObterFocoRevisao().then(itens => setFocoRevisao(itens || [])).catch(() => {});
  };
  const [abaConfiguracao, setAbaConfiguracao] = useState('Geral');
  const [termoBusca, setTermoBusca] = useState('');
  const [totalHanzis, setTotalHanzis] = useState<number>(0);
  const [modalBuscaPorDesenhoOpen, setModalBuscaPorDesenhoOpen] = useState(false);
  const [modalAdicionarHanzi, setModalAdicionarHanzi] = useState<{ open: boolean, status: string }>({ open: false, status: '' });
  const [inputAdicionarHanzi, setInputAdicionarHanzi] = useState('');
  const [sugestoesPinyin, setSugestoesPinyin] = useState<string[]>([]);
  const [faseRevisao, setFaseRevisao] = useState<string>('selecao');

  // Espelhos de state para os handlers registrados uma única vez (EventsOn, listeners de window).
  const cartoesRef = useRefEspelho(cartoes);
  const cartaoEmFocoRef = useRefEspelho(cartaoEmFoco);
  const configuracoesAppRef = useRefEspelho(configuracoesApp);
  const abaAtivaRef = useRefEspelho(abaAtiva);

  const TocarLeituraPinyin = useLeituraPinyin({ configuracoesAppRef, setStatus });

  const { definirOffsetMonitor, definirMouseSobreCartaoUI, revalidarFocoContra } = useRastreamentoMouse({
    cartoesRef, cartaoEmFocoRef, abaAtivaRef, configuracoesAppRef,
    setCartaoEmFoco, TocarLeituraPinyin,
  });

  useEffect(() => {
    if (cartaoSelecionado) {
      if (cartaoSelecionado.isScreenshotCard) {
        GetLastScreenshot().then((res: string) => {
          setImagemModalBase64(res.replace('data:image/png;base64,', ''));
        });
      } else if (cartaoSelecionado.imageId) {
        GetSessionImage(cartaoSelecionado.imageId).then(base64 => {
          setImagemModalBase64(base64);
        });
      } else {
        setImagemModalBase64(null);
      }
    } else {
      setImagemModalBase64(null);
    }
  }, [cartaoSelecionado]);

  useEffect(() => {
    GetConfig().then(cfg => setConfiguracoesApp(cfg));
    GetSystemHardware().then(hw => setInfoHardware(hw));
    GetMonitores().then(m => {
      setMonitores(m || []);
      // Atualizar o offset do monitor alvo
      GetConfig().then(cfg => {
        if (cfg && m && m.length > 0) {
          const alvo = m.find((mon: any) => mon.id === (cfg.monitorAlvo || 0)) || m[0];
          definirOffsetMonitor({ x: alvo?.x || 0, y: alvo?.y || 0 });
        }
      });
    });
    GetCaptureResolution().then(res => setResCaptura(res));
    CarregarVocabulario();

    // @ts-ignore
    window.go.main.App.ObterTotalHanzisDicionario().then(setTotalHanzis).catch(console.error);

    // Leitura em voz alta: estado da síntese na barra de status (subida do motor, download dos
    // pesos na primeira vez, síntese). Mensagem vazia = terminou.
    EventsOn("tts_estado", (mensagem: string) => {
      setStatus(mensagem || 'Aguardando...');
    });

    EventsOn("trigger_scan", () => {
      EscanearTelaEhProcessar();
    });

    // O vigia do Go detectou card fantasma (ou reencontrou um perdido): re-busca os cards para o
    // hover, os destaques e os pop-ups pararem de apontar para posições onde o texto já não está.
    EventsOn("vigia_cards_atualizados", (cards: any) => {
      const lista = cards || [];
      setCartoes(prev => {
        const pseudoCartoes = prev.filter(c => c.isScreenshotCard);
        return [...lista, ...pseudoCartoes];
      });
      // Derruba na hora o foco/pop-up/destaque preso a um card que virou fantasma — o
      // rastreamento do mouse, sozinho, só reavaliaria no próximo movimento do cursor.
      revalidarFocoContra(lista);
    });

    EventsOn("trigger_save", () => {
      if (cartaoEmFocoRef.current) {
        SalvarPalavra(cartaoEmFocoRef.current, STATUS_VOCABULARIO.Estudo);
      }
    });

  }, []);

  useEffect(() => {
    EventsOn("toggle_popup_hover", () => {
      if (configuracoesAppRef.current) {
        const newValue = !configuracoesAppRef.current.habilitarPopupHover;
        const novo = { ...configuracoesAppRef.current, habilitarPopupHover: newValue } as config.Config;
        setConfiguracoesApp(novo);
        SaveConfig(novo);
      }
    });
  }, []);

  // Atalhos de Mouse para Histórico do Modal (Mouse 4 e Mouse 5)
  useEffect(() => {
    if (!cartaoSelecionado) return;

    const handleMouseUp = (e: MouseEvent) => {
      // 3 = Back (Mouse 4), 4 = Forward (Mouse 5)
      if (e.button === 3) {
        if (indiceHistoricoModal > 0) {
          const novoInd = indiceHistoricoModal - 1;
          setIndiceHistoricoModal(novoInd);
          AoClicarNoCartao(historicoModal[novoInd], true);
        }
      } else if (e.button === 4) {
        if (indiceHistoricoModal >= 0 && indiceHistoricoModal < historicoModal.length - 1) {
          const novoInd = indiceHistoricoModal + 1;
          setIndiceHistoricoModal(novoInd);
          AoClicarNoCartao(historicoModal[novoInd], true);
        }
      }
    };

    window.addEventListener('mouseup', handleMouseUp);
    return () => window.removeEventListener('mouseup', handleMouseUp);
  }, [cartaoSelecionado, historicoModal, indiceHistoricoModal]);

  // Carrega o uso de disco ao abrir a aba Armazenamento (ou ao pesquisar nas configurações).
  useEffect(() => {
    if (painelConfigAberto && (abaConfiguracao === 'Armazenamento' || termoBusca)) {
      armazenamento.CarregarArmazenamento();
      nuvem.CarregarInfoNuvem();
    }
    if (painelConfigAberto && (abaConfiguracao === 'Tradução' || termoBusca)) {
      GetCotaTraducao().then(setInfoCotaTraducao);
      GetCotaGemini().then(setInfoCotaGemini);
    }
  }, [painelConfigAberto, abaConfiguracao, termoBusca]);

  // Destaques na tela real (overlay do Go) das palavras/caracteres em estudo.
  useDestaquesTela({
    abaAtiva,
    cartoes,
    cartoesSecao,
    cartoesVocabulario,
    cartaoEmFoco,
    destacarEstudoTela: configuracoesApp?.destacarEstudoTela,
    destacarEstudoParcialTela: configuracoesApp?.destacarEstudoParcialTela,
  });

  const CarregarVocabulario = () => {
    GetVocab().then(v => setCartoesVocabulario(v || []));
  };

  const AtualizarConfiguracao = (key: keyof config.Config, value: any) => {
    if (!configuracoesApp) return;

    let mudancas: Partial<config.Config> = { [key]: value };

    if (key === 'idiomaTraducao' && value !== configuracoesApp.idiomaTraducao) {
      setConfirmacao({
        titulo: t('Alterar Idioma'),
        mensagem: t('É necessário reiniciar o aplicativo para que a alteração de idioma faça efeito. Deseja reiniciar agora?'),
        rotuloCancelar: t('Cancelar'),
        cancelarAcao: () => {},
        rotuloAcao2: t('Reiniciar depois'),
        acao2: () => {
          AplicarConfiguracao(mudancas);
        },
        rotuloAcao: t('Reiniciar agora'),
        acao: () => {
          const novo = { ...configuracoesApp, ...mudancas } as config.Config;
          setConfiguracoesApp(novo);
          SaveConfig(novo).then(() => {
            ReiniciarAplicativo().catch(err => {
              console.error('Erro ao reiniciar o aplicativo:', err);
            });
          });
        },
        perigoso: false
      });
      return;
    }

    // Atualizar offset do monitor quando o alvo mudar
    if (key === 'monitorAlvo' && monitores.length > 0) {
      const newMon = monitores.find((mon: any) => mon.id === value) || monitores[0];
      if (newMon) {
        definirOffsetMonitor({ x: newMon.x || 0, y: newMon.y || 0 });
      }
    }

    AplicarConfiguracao(mudancas);
  };

  // Aplica várias mudanças de config de uma só vez, sobre o mesmo snapshot.
  // Necessário porque chamadas encadeadas de AtualizarConfiguracao usariam o
  // configuracoesApp "velho" do closure e uma sobrescreveria a outra ao salvar.
  const AplicarConfiguracao = (mudancas: Partial<config.Config>) => {
    if (!configuracoesApp) return;
    const novo = { ...configuracoesApp, ...mudancas } as config.Config;
    setConfiguracoesApp(novo);
    SaveConfig(novo);
  };

  // Declarado aqui (e não no topo) porque consome AplicarConfiguracao no ato da chamada: a troca de
  // motor pode precisar migrar o hardware selecionado na config.
  const catalogos = useCatalogos({ configuracoesApp, infoHardware, AplicarConfiguracao });

  const SalvarPalavra = (c: any, newStatus: string) => {
    const sig = c.significados ? c.significados.join(', ') : c.Significado || '';
    const py = c.pinyin || c.Pinyin || '';
    const hz = c.hanzi || c.Hanzi;

    AddVocab(hz, py, sig, newStatus).then(() => {
      CarregarVocabulario();
      // NOTA: O usuário expressou preferência por manter os cartoes na seção em vez de "inbox zero",
      // usando um tratamento visual (cor + sort) para diferenciá-los, então não faremos setCartoes(filter).
      setStatus(t('Palavra movida para {status}: {hanzi}', { status: t(newStatus), hanzi: hz }));
    });
  };

  const [statusOcr, setStatusOcr] = useState(t('Aguardando captura...'));

  const EscanearTelaEhProcessar = () => {
    setStatusOcr(t('Capturando e processando OCR...'));
    CaptureAndOCR()
      .then((res: any) => {
        setStatusOcr(t('Captura concluída!'));
        const palavrasDetectadas: any[] = res || [];

        // Pseudo-cartão com a captura de tela usada nesse OCR: reaproveita o mesmo sistema de card
        // (ListaCartoes / ModalCartaoDetalhes) das palavras normais. Entra por último no array para
        // sempre ficar como o último card da lista — o grid preenche na ordem do array, de cima
        // para baixo — em vez de um bloco separado flutuando acima da lista.
        const pseudoCartaoScreenshot = {
          isScreenshotCard: true,
          hanzi: "📷",
          pinyin: t("Visualizar Captura"),
          significados: [t('{quantidade} palavra(s) detectada(s) nesta captura', { quantidade: palavrasDetectadas.length })],
          tipoHanzi: "SISTEMA",
        };
        const newCards = [...palavrasDetectadas, pseudoCartaoScreenshot];

        setCartoes(newCards);

        // Acumula na "Seção" só as palavras reais: o pseudo-cartão é um atalho pra última captura,
        // não faz sentido acumulado (nem deduplicaria bem — toda captura usa o mesmo hanzi "📷").
        const secaoClean = palavrasDetectadas.map((c: any) => ({ ...c, caixa: [] }));
        setCartoesSecao(prev => [...prev, ...secaoClean]);

        CarregarVocabulario(); // Reload vocab para status refletir na UI imediatamente
      })
      .catch((err: any) => {
        let msg = String(err);
        try {
          const match = msg.match(/API error \(\d+\): (\{.*\})/);
          if (match && match[1]) {
            const parsed = JSON.parse(match[1]);
            if (parsed.error) msg = parsed.error;
          }
        } catch (e) { }
        setStatusOcr('⚠️ ' + msg);
      });
  };

  const AoEntrarNoCartao = (c: any) => {
    definirMouseSobreCartaoUI(true);
    setCartaoEmFoco(c);
    // Card fantasma: o texto já saiu da posição original, destacá-la marcaria outra coisa.
    if (c.caixa && c.caixa.length === 4 && !c.fantasma) {
      ShowHighlight(
        Math.round(c.caixa[0]),
        Math.round(c.caixa[1]),
        Math.round(c.caixa[2]),
        Math.round(c.caixa[3])
      );
    }
  };

  const AoSairDoCartao = () => {
    definirMouseSobreCartaoUI(false);
    setCartaoEmFoco(null);
    HideHoverPopup();
  };

  const FecharModalCartao = () => {
    setCartaoSelecionado(null);
    setHistoricoModal([]);
    setIndiceHistoricoModal(-1);
  };

  const AoClicarNoCartao = (c: any, isNavegacaoHistorico = false) => {
    const hz = c.hanzi || c.Hanzi;
    const vocabEntry = cartoesVocabulario.find(v => v.Hanzi === hz);
    const cardCompleto = {
      ...c,
      vezesVistaOcr: vocabEntry ? vocabEntry.vezesVistaOcr : (c.vezesVistaOcr || 0)
    };

    setCartaoSelecionado(cardCompleto);
    setDadosDecomposicao(null);

    if (!isNavegacaoHistorico) {
      setHistoricoModal(prev => {
        const newHist = prev.slice(0, indiceHistoricoModal + 1);
        return [...newHist, c];
      });
      setIndiceHistoricoModal(prev => prev + 1);
    }

    if (c.isScreenshotCard) return;

    const cfgTts = configuracoesAppRef.current;
    if (cfgTts?.lerPinyinAoExpandirCard && hz) {
      TocarLeituraPinyin(hz);
    }

    if (hz.length > 1) {
      // Multi-character: decompose into individual characters
      const chars = Array.from(hz);
      Promise.all(chars.map(char => LookupWord(char as string).then(res => ({ char, res }))))
        .then(results => {
          setDadosDecomposicao({ type: 'multi', data: results });
        });
    } else if (hz.length === 1) {
      // Single character: decompose into radicals using MakeMeAHanzi
      DecomposeCharacter(hz).then(res => {
        setDadosDecomposicao({ type: 'single', data: res });
      });
    }
  };

  const AoClicarNoCaractereDecomposto = async (char: string) => {
    // Se for abreviação visual, redireciona para o caractere completo
    const completo = await CaractereCompleto(char);
    const charFinal = completo || char;
    const foiAbreviacao = completo !== '';

    // Tenta obter o simplificado correspondente
    const simplified = await ConverterTexto(charFinal, 'simplificado');

    LookupWord(charFinal).then(entradas => {
      if (entradas && entradas.length > 0) {
        const ent = entradas[0];
        const newCard = {
          hanzi: charFinal,
          Hanzi: charFinal,
          pinyin: ent.Pinyin,
          significados: ent.Significados
        };
        AoClicarNoCartao(newCard);

        if (!foiAbreviacao) {
          MarcarVistoSilencioso(charFinal).then(() => CarregarVocabulario());
        }
      } else if (simplified && simplified !== charFinal) {
        // Se a busca com o caractere tradicional falhar diretamente, tenta buscar com a versão simplificada correspondente, mas mantendo o glifo tradicional no card
        LookupWord(simplified).then(entradasSimp => {
          if (entradasSimp && entradasSimp.length > 0) {
            const ent = entradasSimp[0];
            const newCard = {
              hanzi: charFinal,
              Hanzi: charFinal,
              pinyin: ent.Pinyin,
              significados: ent.Significados
            };
            AoClicarNoCartao(newCard);

            if (!foiAbreviacao) {
              MarcarVistoSilencioso(charFinal).then(() => CarregarVocabulario());
            }
          } else {
            // Fallback genérico se a versão simplificada também falhar no CEDICT
            const newCard = {
              hanzi: charFinal,
              Hanzi: charFinal,
              pinyin: '',
              significados: []
            };
            AoClicarNoCartao(newCard);
          }
        });
      } else {
        // Fallback genérico para caracteres sem entrada no dicionário
        const newCard = {
          hanzi: charFinal,
          Hanzi: charFinal,
          pinyin: '',
          significados: []
        };
        AoClicarNoCartao(newCard);
      }
    });
  };

  // Obter listas
  const vistas = cartoesVocabulario; // Tudo que está no banco foi "visto"
  const hanzisDescobertos = new Set(
    cartoesVocabulario
      .filter(c => (c.Hanzi || '').length === 1)
      .map(c => c.Hanzi)
  ).size;
  const estudando = cartoesVocabulario.filter(c => c.Status === STATUS_VOCABULARIO.Estudo);
  const aprendidas = cartoesVocabulario.filter(c => c.Status === STATUS_VOCABULARIO.Aprendido);

  const hzSelecionado = cartaoSelecionado?.hanzi || cartaoSelecionado?.Hanzi;
  const pySelecionado = cartaoSelecionado?.pinyin || cartaoSelecionado?.Pinyin;
  const sigSelecionado = cartaoSelecionado?.significados ? cartaoSelecionado.significados.join(', ') : cartaoSelecionado?.Significado;

  const isEstudandoSelecionado = cartoesVocabulario.some(v => v.Hanzi === hzSelecionado && v.Status === STATUS_VOCABULARIO.Estudo);
  const isAprendidaSelecionado = cartoesVocabulario.some(v => v.Hanzi === hzSelecionado && v.Status === STATUS_VOCABULARIO.Aprendido);

  // Alterna o vínculo do cartão selecionado com um status: remove se já está nele, senão grava.
  const alternarStatusSelecionado = (status: string, jaNesteStatus: boolean) => {
    if (!hzSelecionado) return;
    const operacao = jaNesteStatus
      ? RemoveVocab(hzSelecionado)
      : AddVocab(hzSelecionado, pySelecionado || '', sigSelecionado || '', status);
    operacao.then(CarregarVocabulario);
  };

  const alternarEstudoSelecionado = () => alternarStatusSelecionado(STATUS_VOCABULARIO.Estudo, isEstudandoSelecionado);
  const alternarAprendidaSelecionado = () => alternarStatusSelecionado(STATUS_VOCABULARIO.Aprendido, isAprendidaSelecionado);

  // Filtro de exibição por tipo de Hanzi (Simplificado / Tradicional), lido das configurações.
  const filtrarPorTipo = (cartoes: any[]) => FiltrarPorTipoHanzi(cartoes, configuracoesApp?.tipoHanziExibicao);
  const emSessaoOuJornadaRevisao = abaAtiva === ABAS.Revisao && faseRevisao !== 'selecao';

  return (
    <div id="App">
      <BarraLateral
        abaAtiva={abaAtiva}
        aoTrocarAba={setAbaAtiva}
        cartaoEmFoco={cartaoEmFoco}
        aoAbrirConfiguracoes={() => setPainelConfigAberto(true)}
      />

      {/* Main Content Area */}
      <div className="main-content">
        {!emSessaoOuJornadaRevisao && (
          <div className="header">
            <div className="header-title">
              {t(TITULOS_POR_ABA[abaAtiva])}
            </div>

            <div style={{ display: 'flex', gap: '12px', alignItems: 'center', minWidth: 0 }}>
              {abaAtiva !== ABAS.Revisao && (
                <CampoBuscaGlobal
                  termoBuscaGlobal={termoBuscaGlobal}
                  aoMudarTermo={setTermoBuscaGlobal}
                  aoAbrirBuscaPorDesenho={() => setModalBuscaPorDesenhoOpen(true)}
                  ordenarPorRanking={ordenarPorRanking}
                  aoAlternarRanking={() => setOrdenarPorRanking(prev => {
                    const proximo = !prev;
                    if (proximo) {
                      if (termoBuscaGlobal.startsWith("[HSK]")) {
                        setTermoBuscaGlobal("");
                      }
                      if (buscaGlobalAtiva) {
                        setTermoBuscaGlobal("[RANKING]");
                      }
                    } else {
                      if (termoBuscaGlobal === "[RANKING]") {
                        setTermoBuscaGlobal("");
                      }
                    }
                    return proximo;
                  })}
                  buscaGlobalAtiva={buscaGlobalAtiva}
                  aoAlternarGlobal={() => {
                    setBuscaGlobalAtiva(prev => {
                      const proximo = !prev;
                      if (ordenarPorRanking) {
                        if (proximo) {
                          setTermoBuscaGlobal("[RANKING]");
                        } else {
                          setTermoBuscaGlobal("");
                        }
                      }
                      return proximo;
                    });
                  }}
                />
              )}

              {/* Na Revisão, o espaço da busca/adicionar é do painel do grupo de foco */}
              {abaAtiva === ABAS.Revisao && (
                <GrupoFocoCabecalho
                  configuracoesApp={configuracoesApp}
                  AtualizarConfiguracao={AtualizarConfiguracao}
                  foco={focoRevisao}
                  recarregarFoco={recarregarFoco}
                  aoClicarNoFoco={item => AoClicarNoCartao({ Hanzi: item.hanzi, Pinyin: item.pinyin, significados: item.significados, abrirEmEstatisticas: true })}
                />
              )}

              {abaAtiva === ABAS.Descobrimento && (
                <div className="compact-scan-control">
                  <button
                    className={`compact-scan-btn ${configuracoesApp?.autoScanAtivo ? 'active' : 'paused'}`}
                    title={configuracoesApp?.autoScanAtivo ? t("Pausar Captura Automática") : t("Iniciar Captura Automática")}
                    onClick={() => {
                      if (configuracoesApp) {
                        AtualizarConfiguracao('autoScanAtivo', !configuracoesApp.autoScanAtivo);
                      }
                    }}
                  >
                    {configuracoesApp?.autoScanAtivo ? (
                      <svg width="16" height="16" viewBox="0 0 24 24" fill="currentColor" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                        <rect x="6" y="4" width="4" height="16"></rect>
                        <rect x="14" y="4" width="4" height="16"></rect>
                      </svg>
                    ) : (
                      <svg width="16" height="16" viewBox="0 0 24 24" fill="currentColor" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                        <polygon points="5 3 19 12 5 21 5 3"></polygon>
                      </svg>
                    )}
                  </button>
                  <button
                    className="compact-scan-btn action"
                    title={`${t('Escanear Tela Agora')} (${configuracoesApp?.atalhoEscanear || 'ctrl+shift+e'})`}
                    onClick={EscanearTelaEhProcessar}
                  >
                    <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5" strokeLinecap="round" strokeLinejoin="round">
                      <path d="M3 7V5a2 2 0 0 1 2-2h2m10 0h2a2 2 0 0 1 2 2v2m0 10v2a2 2 0 0 1-2 2h-2M7 21H5a2 2 0 0 1-2-2v-2"></path>
                      <line x1="4" y1="12" x2="20" y2="12" stroke="currentColor" strokeWidth="2.5"></line>
                    </svg>
                  </button>
                </div>
              )}

              {abaAtiva === ABAS.TelaUnica && (
                <div className="compact-scan-control">
                  <button
                    className="compact-scan-btn danger"
                    title={t("Limpar Seção")}
                    onClick={() => {
                      setCartoes([]);
                      setCartoesSecao([]);
                    }}
                  >
                    <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                      <polyline points="3 6 5 6 21 6"></polyline>
                      <path d="M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6m3 0V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2"></path>
                    </svg>
                  </button>
                </div>
              )}

              {(abaAtiva === ABAS.Estudando || abaAtiva === ABAS.Aprendidas) && (
                <button
                  className="scan-btn"
                  style={{ backgroundColor: '#2196f3', display: 'flex', alignItems: 'center', gap: '6px' }}
                  onClick={() => setModalAdicionarHanzi({ open: true, status: abaAtiva === ABAS.Estudando ? STATUS_VOCABULARIO.Estudo : STATUS_VOCABULARIO.Aprendido })}
                >
                  <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2"><line x1="12" y1="5" x2="12" y2="19"></line><line x1="5" y1="12" x2="19" y2="12"></line></svg>
                  {t('Adicionar Hanzi')}
                </button>
              )}

              {abaAtiva === ABAS.Vistas && (
                <div style={{ display: 'flex', flexDirection: 'column', alignItems: 'flex-end', fontSize: '12px', color: 'var(--cor-texto-suave)' }}>
                  <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
                    <div style={{ width: '120px', height: '8px', backgroundColor: 'var(--cor-fundo-secundario)', borderRadius: '4px', overflow: 'hidden' }}>
                      <div style={{ width: `${totalHanzis > 0 ? (hanzisDescobertos / totalHanzis) * 100 : 0}%`, height: '100%', backgroundColor: 'var(--cor-destaque)' }}></div>
                    </div>
                    <strong style={{ color: 'var(--cor-texto-primario)' }}>
                      {totalHanzis > 0 ? ((hanzisDescobertos / totalHanzis) * 100).toFixed(2) : 0}%
                    </strong>
                  </div>
                  <span style={{ fontSize: '10px', marginTop: '2px' }}>{t('{descobertos} / {total} hanzis descobertos', { descobertos: hanzisDescobertos, total: totalHanzis })}</span>
                </div>
              )}
            </div>
          </div>
        )}

        {status !== 'Aguardando...' && status !== '' && (
          <div style={{
            position: 'fixed',
            bottom: '24px',
            right: '24px',
            backgroundColor: 'var(--cor-fundo)',
            color: 'var(--cor-texto-primario)',
            padding: '12px 16px',
            borderRadius: '8px',
            border: '1px solid var(--cor-borda)',
            boxShadow: '0 4px 12px rgba(0,0,0,0.5)',
            zIndex: 9999,
            fontSize: '13px',
            pointerEvents: 'none',
            maxWidth: '300px'
          }}>
            {status}
          </div>
        )}

        {abaAtiva === ABAS.Descobrimento && (
          <div style={{ color: 'var(--cor-texto-suave)', marginBottom: '24px' }}>
            {statusOcr}
          </div>
        )}

        {termoBuscaGlobal && abaAtiva !== ABAS.Revisao ? (
          <AbaBuscaGlobal
            termoBuscaGlobal={termoBuscaGlobal}
            resultadosBuscaGlobal={resultadosBuscaGlobal}
            cartoes={cartoes}
            cartoesSecao={cartoesSecao}
            vistas={vistas}
            estudando={estudando}
            aprendidas={aprendidas}
            cartoesVocabulario={cartoesVocabulario}
            AoEntrarNoCartao={AoEntrarNoCartao}
            AoSairDoCartao={AoSairDoCartao}
            AoClicarNoCartao={AoClicarNoCartao}
            ordenarPorRanking={ordenarPorRanking}
            scrollTargetHanzi={scrollTargetHanzi}
            setScrollTargetHanzi={setScrollTargetHanzi}
            SalvarPalavra={SalvarPalavra}
            buscaGlobalAtiva={buscaGlobalAtiva}
            abaAtiva={abaAtiva}
          />
        ) : (
          <>
            <AbaDescobrimento
              abaAtiva={abaAtiva}
              cartoes={filtrarPorTipo(cartoes)}
              cartoesSecao={filtrarPorTipo(cartoesSecao)}
              vistas={vistas}
              cartoesVocabulario={cartoesVocabulario}
              AoEntrarNoCartao={AoEntrarNoCartao}
              AoSairDoCartao={AoSairDoCartao}
              AoClicarNoCartao={AoClicarNoCartao}
              SalvarPalavra={SalvarPalavra}
              ocultarBadgeTipo={configuracoesApp?.tipoHanziExibicao !== 'ambos'}
              ordenarPorRanking={ordenarPorRanking}
            />

            <AbaEstudos
              abaAtiva={abaAtiva}
              estudando={estudando}
              aprendidas={aprendidas}
              cartoesVocabulario={cartoesVocabulario}
              AoEntrarNoCartao={AoEntrarNoCartao}
              AoSairDoCartao={AoSairDoCartao}
              AoClicarNoCartao={AoClicarNoCartao}
              SalvarPalavra={SalvarPalavra}
              ocultarBadgeTipo={false}
              ordenarPorRanking={ordenarPorRanking}
            />
          </>
        )}

        <AbaRevisao
          abaAtiva={abaAtiva}
          configuracoesApp={configuracoesApp}
          setStatus={setStatus}
          AoClicarNoCartao={AoClicarNoCartao}
          AtualizarConfiguracao={AtualizarConfiguracao}
          foco={focoRevisao}
          setFoco={setFocoRevisao}
          aoMudarFase={setFaseRevisao}
        />
      </div>

      {/* Pop-up estilo Balatro: sugere estudar as palavras que o OCR mais viu (abas Seção/Já Vistas) */}
      <SugestaoEstudoPopup
        abaAtiva={abaAtiva}
        habilitado={configuracoesApp?.mostrarSugestaoPalavrasVistas ?? true}
        SalvarPalavra={SalvarPalavra}
        setStatus={setStatus}
        AoClicarNoCartao={AoClicarNoCartao}
        cartoesSecao={cartoesSecao}
      />

      {/* Settings Modal Overlay */}
      <PainelConfiguracoes
        painelConfigAberto={painelConfigAberto}
        setPainelConfigAberto={setPainelConfigAberto}
        configuracoesApp={configuracoesApp!}
        AtualizarConfiguracao={AtualizarConfiguracao}
        AplicarConfiguracao={AplicarConfiguracao}
        setConfirmacao={setConfirmacao}
        abaConfiguracao={abaConfiguracao}
        setAbaConfiguracao={setAbaConfiguracao}
        termoBusca={termoBusca}
        setTermoBusca={setTermoBusca}
        infoHardware={infoHardware}
        resCaptura={resCaptura}
        monitores={monitores}
        infoCotaTraducao={infoCotaTraducao}
        infoCotaGemini={infoCotaGemini}
        catalogos={catalogos}
        armazenamento={armazenamento}
        nuvem={nuvem}
      />

      {/* Card Details Modal Overlay */}
      <ModalCartaoDetalhes
        cartaoSelecionado={cartaoSelecionado}
        setCartaoSelecionado={() => FecharModalCartao()}
        imagemModalBase64={imagemModalBase64}
        dadosDecomposicao={dadosDecomposicao}
        AoClicarNoCaractereDecomposto={AoClicarNoCaractereDecomposto}
        isEstudando={isEstudandoSelecionado}
        onToggleEstudo={alternarEstudoSelecionado}
        isAprendida={isAprendidaSelecionado}
        onToggleAprendida={alternarAprendidaSelecionado}
        motorTtsAtivo={configuracoesApp?.motorTtsAtivo || ''}
        lerPinyinAoCompletarDesenho={configuracoesApp?.lerPinyinAoCompletarDesenho ?? true}
        configuracoesApp={configuracoesApp}
        aoBuscarPalavrasCompostas={(hanzi) => {
          setTermoBuscaGlobal(hanzi);
          if (abaAtiva === ABAS.Revisao) {
            setAbaAtiva(ABAS.Descobrimento);
          }
          FecharModalCartao();
        }}
        aoBuscarPorRanking={(hanzi) => {
          setOrdenarPorRanking(true);
          setScrollTargetHanzi(hanzi);
          
          const isAlreadyInSearch = resultadosBuscaGlobal.some(c => (c.hanzi || (c as any).Hanzi) === hanzi);
          if (!termoBuscaGlobal || !isAlreadyInSearch) {
            setTermoBuscaGlobal(`[RANKING]${hanzi}`);
          }

          if (abaAtiva === ABAS.Revisao) {
            setAbaAtiva(ABAS.Descobrimento);
          }
          FecharModalCartao();
        }}
        aoBuscarPorHSK={(hanzi) => {
          setScrollTargetHanzi(hanzi);
          setTermoBuscaGlobal(`[HSK]${hanzi}`);
          if (abaAtiva === ABAS.Revisao) {
            setAbaAtiva(ABAS.Descobrimento);
          }
          FecharModalCartao();
        }}
      />

      {/* Pop-up de aviso de compatibilidade */}
      <ModalAvisoCompatibilidade
        avisoCompatibilidade={catalogos.avisoCompatibilidade}
        setAvisoCompatibilidade={catalogos.setAvisoCompatibilidade}
      />

      {/* Pop-up de notas de versão pós-atualização */}
      <ModalNotasVersao
        aberto={notasVersao.aberto}
        titulo={notasVersao.titulo}
        notas={notasVersao.notas}
        aoFechar={notasVersao.fechar}
      />

      {/* Modal de confirmação */}
      <ModalConfirmacao
        confirmacao={confirmacao}
        setConfirmacao={setConfirmacao}
      />

      {/* Conflito da 1ª conexão com o Google Drive: escolher entre o banco local e o da nuvem */}
      <ModalConflitoNuvem
        aberto={nuvem.conflitoNuvemAberto}
        infoNuvem={nuvem.infoNuvem}
        ocupado={nuvem.nuvemOcupada}
        aoEscolher={nuvem.ResolverConflitoNuvemDrive}
        aoFechar={nuvem.fecharConflitoNuvem}
      />

      {/* Modal Adicionar Hanzi Manualmente */}
      <ModalAdicionarHanzi
        modalAdicionarHanzi={modalAdicionarHanzi}
        setModalAdicionarHanzi={setModalAdicionarHanzi}
        inputAdicionarHanzi={inputAdicionarHanzi}
        setInputAdicionarHanzi={setInputAdicionarHanzi}
        sugestoesPinyin={sugestoesPinyin}
        setSugestoesPinyin={setSugestoesPinyin}
        SalvarPalavra={SalvarPalavra}
        setStatus={setStatus}
        configuracoesApp={configuracoesApp}
      />

      <ModalBuscaPorDesenho
        isOpen={modalBuscaPorDesenhoOpen}
        onClose={() => setModalBuscaPorDesenhoOpen(false)}
        onSelect={(hanzi) => setTermoBuscaGlobal(hanzi)}
        configuracoesApp={configuracoesApp}
      />

      <ModalGuiaAtalhos configuracoesApp={configuracoesApp} />

    </div>
  );
}

function App() {
  return (
    <AtalhosProvider>
      <AppConteudo />
    </AtalhosProvider>
  );
}

export default App;
