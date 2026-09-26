// ----- Seção: Descobrimento — pop-up de sugestão de estudo (estilo Balatro) -----
// Ao entrar nas abas "Palavras Dessa Seção" e "Já Vistas", oferece como uma mão de cartas de
// baralho (visual inspirado no jogo Balatro — ver sugestaoEstudo.css) as palavras 'vistas' que o
// OCR mais encontrou (contador vezesVistaOcr), perguntando se o usuário quer movê-las para
// "Em estudo". Dá para adicionar uma a uma, adicionar todas ou ignorar todas; as cartas marcadas
// com "não sugerir" (individual ou geral) são silenciadas para sempre no banco ao dispensar o
// pop-up. O toggle das configurações (mostrarSugestaoPalavrasVistas) desliga tudo.

import { CSSProperties, useEffect, useRef, useState } from 'react';
import { progresso } from '../../wailsjs/go/models';
import { ObterSugestoesEstudoOcr, OcultarSugestoesEstudoOcr } from '../../wailsjs/go/main/App';
import { ABAS, Aba } from '../casca/abas';
import { STATUS_VOCABULARIO } from '../comum/status';
import { t } from '../i18n/i18n';
import './sugestaoEstudo.css';

// Inclinação de cada carta no leque e afundamento das pontas (o arco da mão), por posição.
const GRAUS_POR_POSICAO_LEQUE = 5;
const PX_ARCO_POR_POSICAO = 9;
const SEGUNDOS_DEFASAGEM_BALANCO = 0.35;

// Cooldown geral (10 minutos) para evitar que o pop-up reapareça insistentemente.
const COOLDOWN_GERAL_MS = 10 * 60 * 1000;
let ultimoFechamentoGlobal = 0;

interface SugestaoEstudoPopupProps {
  abaAtiva: Aba;
  habilitado: boolean;
  SalvarPalavra: (cartao: any, status: string) => void;
  setStatus: (status: string) => void;
  AoClicarNoCartao: (c: any) => void;
  cartoesSecao: any[];
}

export function SugestaoEstudoPopup({ abaAtiva, habilitado, SalvarPalavra, setStatus, AoClicarNoCartao, cartoesSecao }: SugestaoEstudoPopupProps) {
  const [cartas, setCartas] = useState<progresso.Vocab[]>([]);
  const [naoSugerir, setNaoSugerir] = useState<Set<string>>(new Set());
  const [aberto, setAberto] = useState(false);
  const [abaOndeAbriu, setAbaOndeAbriu] = useState<Aba | null>(null);

  // Palavras já ofertadas nesta sessão do app: sem persistir nada, o pop-up não reaparece com as
  // mesmas cartas a cada troca de aba — só quando palavras NOVAS atingirem o mínimo de vistas.
  const ofertadasSessaoRef = useRef<Set<string>>(new Set());

  // Busca as sugestões ao entrar numa das duas abas-gatilho. Falha na consulta só suprime o
  // pop-up (ele é opportunístico — não pode atrapalhar a navegação com erro).
  useEffect(() => {
    if (!habilitado) return;
    if (aberto) return;
    if (abaAtiva !== ABAS.TelaUnica && abaAtiva !== ABAS.Vistas) return;

    // Se estiver no período de cooldown geral, ignora o reaparecimento
    if (Date.now() - ultimoFechamentoGlobal < COOLDOWN_GERAL_MS) return;

    ObterSugestoesEstudoOcr()
      .then(sugestoes => {
        let filtradas = sugestoes || [];

        // Se estiver na aba "Palavras Dessa Seção" (ABAS.TelaUnica),
        // filtra para sugerir apenas palavras que pertencem à seção atual
        if (abaAtiva === ABAS.TelaUnica) {
          const hanzisSecao = new Set(cartoesSecao.map(c => c.hanzi || c.Hanzi));
          filtradas = filtradas.filter(v => hanzisSecao.has(v.Hanzi));
        }

        const novas = filtradas.filter(v => !ofertadasSessaoRef.current.has(v.Hanzi));
        if (novas.length === 0) return;

        // Limita a exibição às 5 palavras com maior pontuação inteligente
        const limitadas = novas.slice(0, 5);
        limitadas.forEach(v => ofertadasSessaoRef.current.add(v.Hanzi));
        // Inverte a ordem para que a pontuação reflita da direita para a esquerda no leque (maior pontuação à direita)
        const ordenadasDireitaParaEsquerda = [...limitadas].reverse();
        setCartas(ordenadasDireitaParaEsquerda);
        setNaoSugerir(new Set());
        setAbaOndeAbriu(abaAtiva);
        setAberto(true);
      })
      .catch(() => {});
  }, [abaAtiva, habilitado, cartoesSecao, aberto]);

  // ESC dispensa o pop-up (mesmo efeito de "Ignorar todas": aplica os "não sugerir" marcados).
  useEffect(() => {
    const visivel = aberto && abaAtiva === abaOndeAbriu;
    if (!visivel) return;

    function aoTeclarHandler(e: KeyboardEvent) {
      if (e.key === 'Escape') IgnorarTodas();
    }
    window.addEventListener('keydown', aoTeclarHandler);
    return () => window.removeEventListener('keydown', aoTeclarHandler);
  });

  const scrollPosRef = useRef<number>(0);

  // Bloqueia a rolagem da seção principal (.main-content) enquanto o pop-up está aberto e visível,
  // mantendo o pop-up visível no topo e restaurando a rolagem original ao fechar.
  useEffect(() => {
    const visivel = aberto && abaAtiva === abaOndeAbriu;
    if (!visivel) return;
    const painelPrincipal = document.querySelector('.main-content') as HTMLElement | null;
    if (!painelPrincipal) return;

    scrollPosRef.current = painelPrincipal.scrollTop;
    const overflowOriginal = painelPrincipal.style.overflowY;
    painelPrincipal.style.overflowY = 'hidden';
    painelPrincipal.scrollTop = 0;

    return () => {
      painelPrincipal.style.overflowY = overflowOriginal;
      painelPrincipal.scrollTop = scrollPosRef.current;
    };
  }, [aberto, abaAtiva, abaOndeAbriu]);

  if (!aberto || cartas.length === 0 || abaAtiva !== abaOndeAbriu) {
    return null;
  }

  function EstudarUma(carta: progresso.Vocab) {
    SalvarPalavra(carta, STATUS_VOCABULARIO.Estudo);
    const restantes = cartas.filter(c => c.Hanzi !== carta.Hanzi);
    setCartas(restantes);
    if (restantes.length === 0) {
      setAberto(false);
      setAbaOndeAbriu(null);
      ultimoFechamentoGlobal = Date.now();
    }
  }

  function EstudarTodas() {
    cartas.forEach(carta => SalvarPalavra(carta, STATUS_VOCABULARIO.Estudo));
    setAberto(false);
    setAbaOndeAbriu(null);
    ultimoFechamentoGlobal = Date.now();
  }

  // Fecha o pop-up persistindo o silenciamento das cartas marcadas com "não sugerir".
  function IgnorarTodas() {
    const marcadas = cartas.filter(c => naoSugerir.has(c.Hanzi)).map(c => c.Hanzi);
    if (marcadas.length > 0) {
      OcultarSugestoesEstudoOcr(marcadas).catch(e => setStatus(t('⚠️ Falha ao silenciar sugestões: {erro}', { erro: String(e) })));
    }
    setAberto(false);
    setAbaOndeAbriu(null);
    ultimoFechamentoGlobal = Date.now();
  }

  function alternarNaoSugerir(hanzi: string) {
    const novo = new Set(naoSugerir);
    if (novo.has(hanzi)) {
      novo.delete(hanzi);
    } else {
      novo.add(hanzi);
    }
    setNaoSugerir(novo);
  }

  const todasMarcadas = naoSugerir.size === cartas.length;

  function alternarNaoSugerirTodas() {
    setNaoSugerir(todasMarcadas ? new Set() : new Set(cartas.map(c => c.Hanzi)));
  }

  return (
    <div className="sugestao-estudo-overlay" onClick={IgnorarTodas}>
      <div className="sugestao-estudo-mesa">
        <div className="sugestao-estudo-titulo">{t('Palavras muito vistas!')}</div>
        <div className="sugestao-estudo-subtitulo">
          {t('O OCR encontrou estas palavras várias vezes na sua tela. Quer movê-las para "Em estudo"?')}
        </div>

        <div className="sugestao-estudo-mao">
          {cartas.map((carta, indice) => {
            const centro = (cartas.length - 1) / 2;
            const estiloLeque = {
              '--rotacao': `${(indice - centro) * GRAUS_POR_POSICAO_LEQUE}deg`,
              '--arco': `${Math.abs(indice - centro) * PX_ARCO_POR_POSICAO}px`,
              '--fase': `${indice * SEGUNDOS_DEFASAGEM_BALANCO}s`,
            } as CSSProperties;

            return (
              <div key={carta.Hanzi} className="sugestao-estudo-slot" style={estiloLeque}>
                <div
                  className={`sugestao-estudo-carta${naoSugerir.has(carta.Hanzi) ? ' marcada' : ''}`}
                  onClick={(e) => {
                    e.stopPropagation();
                    AoClicarNoCartao(carta);
                  }}
                  style={{ cursor: 'pointer' }}
                >
                  <div className="sugestao-estudo-carta-topo">
                    <span className="sugestao-estudo-chip" title={t('Vista em {vezes} scans de OCR', { vezes: carta.vezesVistaOcr })}>
                      <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5" strokeLinecap="round" strokeLinejoin="round">
                        <path d="M1 12s4-8 11-8 11 8 11 8-4 8-11 8-11-8-11-8z" />
                        <circle cx="12" cy="12" r="3" />
                      </svg>
                      {carta.vezesVistaOcr}×
                    </span>
                    <span
                      className="sugestao-estudo-ranking"
                      title={carta.posicaoRanking ? t('Posição no ranking de frequência: #{pos}', { pos: carta.posicaoRanking }) : t('Sem posição no ranking')}
                    >
                      {carta.posicaoRanking ? `#${carta.posicaoRanking}` : '#-'}
                    </span>
                  </div>
                  <div className="sugestao-estudo-carta-pinyin">{carta.Pinyin}</div>
                  <div className="sugestao-estudo-carta-hanzi" lang="zh">{carta.Hanzi}</div>
                  <div className="sugestao-estudo-carta-significado" title={carta.Significado}>
                    {carta.Significado}
                  </div>
                  <button
                    className="scan-btn"
                    style={{ padding: '4px 8px', fontSize: '11px', marginTop: 'auto', width: '100%' }}
                    onClick={(e) => {
                      e.stopPropagation();
                      EstudarUma(carta);
                    }}
                  >
                    {t('+ Mover p/ Estudo')}
                  </button>
                  <label className="sugestao-estudo-carta-nao" onClick={(e) => e.stopPropagation()}>
                    <input
                      type="checkbox"
                      checked={naoSugerir.has(carta.Hanzi)}
                      onChange={() => alternarNaoSugerir(carta.Hanzi)}
                    />
                    {t('Não sugerir')}
                  </label>
                </div>
              </div>
            );
          })}
        </div>

        <label className="sugestao-estudo-nao-geral" onClick={(e) => e.stopPropagation()}>
          <input type="checkbox" checked={todasMarcadas} onChange={alternarNaoSugerirTodas} />
          {t('Não sugerir nenhuma destas palavras novamente')}
        </label>

        <div className="sugestao-estudo-acoes">
          <button
            className="sugestao-estudo-btn sugestao-estudo-btn-adicionar"
            onClick={(e) => {
              e.stopPropagation();
              EstudarTodas();
            }}
          >
            {t('Adicionar todas')}
          </button>
          <button
            className="sugestao-estudo-btn sugestao-estudo-btn-ignorar"
            onClick={(e) => {
              e.stopPropagation();
              IgnorarTodas();
            }}
          >
            {t('Ignorar todas')}
          </button>
        </div>

        <div className="sugestao-estudo-dica">
          {t('As cartas marcadas com "não sugerir" nunca mais aparecem aqui (Esc = ignorar todas).')}
        </div>
      </div>
    </div>
  );
}
