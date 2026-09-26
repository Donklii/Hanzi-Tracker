import React, { useEffect, useRef, useState } from 'react';
import { main, progresso } from '../../wailsjs/go/models';
import { ObterRecomendacoesBaralho, VirarCartaBaralho } from '../../wailsjs/go/main/App';
import { STATUS_VOCABULARIO } from '../comum/status';
import { t } from '../i18n/i18n';
import './baralhoRecomendacoes.css';

const IconeBaralho = ({ size = 22 }: { size?: number }) => (
  <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" style={{ width: `${size}px`, height: `${size}px` }}>
    <rect x="2" y="6" width="14" height="16" rx="2" ry="2" />
    <path d="M6 2h14a2 2 0 0 1 2 2v14" />
    <path d="M9 12h.01" />
    <path d="M11 16h.01" />
  </svg>
);

const IconeVersoCarta = ({ size = 44 }: { size?: number }) => (
  <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round" style={{ width: `${size}px`, height: `${size}px` }}>
    <rect x="3" y="3" width="18" height="18" rx="3" ry="3" />
    <path d="M7 7l10 10" />
    <path d="M17 7L7 17" />
    <circle cx="12" cy="12" r="2.5" strokeWidth="1.5" />
  </svg>
);

const IconeLampada = ({ size = 13 }: { size?: number }) => (
  <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" style={{ width: `${size}px`, height: `${size}px`, flexShrink: 0 }}>
    <path d="M9 18h6" />
    <path d="M10 22h4" />
    <path d="M15.09 14A6 6 0 0 0 18 9 6 6 0 0 0 6 9a6 6 0 0 0 2.91 5" />
  </svg>
);

const IconeFechar = ({ size = 18 }: { size?: number }) => (
  <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" style={{ width: `${size}px`, height: `${size}px` }}>
    <line x1="18" y1="6" x2="6" y2="18" />
    <line x1="6" y1="6" x2="18" y2="18" />
  </svg>
);

const IconeCheck = ({ size = 14 }: { size?: number }) => (
  <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5" strokeLinecap="round" strokeLinejoin="round" style={{ width: `${size}px`, height: `${size}px` }}>
    <polyline points="20 6 9 17 4 12" />
  </svg>
);

const IconeEstudoAtivo = ({ size = 13 }: { size?: number }) => (
  <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" style={{ width: `${size}px`, height: `${size}px` }}>
    <path d="M2 3h6a4 4 0 0 1 4 4v14a3 3 0 0 0-3-3H2z"></path>
    <path d="M22 3h-6a4 4 0 0 0-4 4v14a3 3 0 0 1 3-3h7z"></path>
  </svg>
);

interface PopUpBaralhoRecomendacoesProps {
  aberto: boolean;
  aoFechar: () => void;
  SalvarPalavra: (cartao: any, status: string) => void;
  AoClicarNoCartao?: (cartao: any) => void;
  cartoesVocabulario?: progresso.Vocab[];
}

type FaseAnimacao = 'parado' | 'recolhendo' | 'embaralhando' | 'distribuindo';

export function PopUpBaralhoRecomendacoes({
  aberto,
  aoFechar,
  SalvarPalavra,
  AoClicarNoCartao,
  cartoesVocabulario = [],
}: PopUpBaralhoRecomendacoesProps) {
  const [cartas, setCartas] = useState<main.RecomendacaoBaralho[]>([]);
  const [faseAnimacao, setFaseAnimacao] = useState<FaseAnimacao>('distribuindo');
  const [adicionadas, setAdicionadas] = useState<Set<string>>(new Set());

  const timerRef = useRef<any[]>([]);

  const limparTimers = () => {
    timerRef.current.forEach(t => clearTimeout(t));
    timerRef.current = [];
  };

  const executarFluxoEmbaralhamento = (forcarNovo: boolean) => {
    limparTimers();

    const temCartasNaMesa = cartas.length > 0;

    const iniciarPassoEmbaralhar = (novasCartasPromessa: Promise<main.RecomendacaoBaralho[]>) => {
      setFaseAnimacao('embaralhando');

      let novasCartasObtidas: main.RecomendacaoBaralho[] | null = null;
      novasCartasPromessa
        .then(res => {
          novasCartasObtidas = res || [];
        })
        .catch(err => {
          console.error("Erro ao obter recomendações do baralho:", err);
        });

      // Duração do corte/riffle do baralho
      const tDistribuir = setTimeout(() => {
        if (novasCartasObtidas) {
          setCartas(novasCartasObtidas);
        }
        setFaseAnimacao('distribuindo');

        const tParado = setTimeout(() => {
          setFaseAnimacao('parado');
        }, 550);
        timerRef.current.push(tParado);
      }, 950);
      timerRef.current.push(tDistribuir);
    };

    const iniciarPassoRecolher = () => {
      setFaseAnimacao('recolhendo');
      const requisicao = ObterRecomendacoesBaralho(forcarNovo);
      const tRecolher = setTimeout(() => {
        iniciarPassoEmbaralhar(requisicao);
      }, 350);
      timerRef.current.push(tRecolher);
    };

    if (temCartasNaMesa && forcarNovo) {
      const temCartasReveladas = cartas.some(c => c.revelada);
      if (temCartasReveladas) {
        // Desvira cartas antes de recolher para um efeito limpo
        setCartas(prev => prev.map(c => ({ ...c, revelada: false })));
        const tDesvirar = setTimeout(() => {
          iniciarPassoRecolher();
        }, 260);
        timerRef.current.push(tDesvirar);
      } else {
        iniciarPassoRecolher();
      }
    } else {
      const requisicao = ObterRecomendacoesBaralho(forcarNovo);
      iniciarPassoEmbaralhar(requisicao);
    }
  };

  useEffect(() => {
    if (aberto) {
      setAdicionadas(new Set());
      limparTimers();
      setFaseAnimacao('distribuindo');

      ObterRecomendacoesBaralho(false)
        .then(res => {
          setCartas(res || []);
          const tFimDistribuir = setTimeout(() => {
            setFaseAnimacao('parado');
          }, 550);
          timerRef.current.push(tFimDistribuir);
        })
        .catch(err => {
          console.error("Erro ao carregar recomendações:", err);
          setFaseAnimacao('parado');
        });
    } else {
      limparTimers();
      setCartas([]);
      setFaseAnimacao('parado');
    }
    return () => limparTimers();
  }, [aberto]);

  useEffect(() => {
    if (!aberto) return;
    function aoTeclarHandler(e: KeyboardEvent) {
      if (e.key === 'Escape') aoFechar();
    }
    window.addEventListener('keydown', aoTeclarHandler);
    return () => window.removeEventListener('keydown', aoTeclarHandler);
  }, [aberto, aoFechar]);

  if (!aberto) return null;

  const virarCarta = (indice: number, hanzi: string) => {
    setCartas(prev => prev.map((item, idx) => {
      if (idx === indice) {
        return { ...item, revelada: true };
      }
      return item;
    }));

    VirarCartaBaralho(hanzi, true).catch(err => {
      console.error("Erro ao salvar revelação da carta:", err);
    });
  };

  const aoClicarNaCarta = (indice: number, c: main.RecomendacaoBaralho) => {
    if (faseAnimacao !== 'parado') return;

    if (!c.revelada) {
      virarCarta(indice, c.hanzi);
      return;
    }

    if (AoClicarNoCartao) {
      AoClicarNoCartao({
        hanzi: c.hanzi,
        pinyin: c.pinyin,
        significados: c.significado ? [c.significado] : [],
      });
    }
  };

  const adicionarAoEstudo = (evento: React.MouseEvent, c: main.RecomendacaoBaralho) => {
    evento.stopPropagation();
    if (adicionadas.has(c.hanzi)) return;

    SalvarPalavra(
      {
        hanzi: c.hanzi,
        pinyin: c.pinyin,
        significados: c.significado ? [c.significado] : [],
      },
      STATUS_VOCABULARIO.Estudo
    );

    setAdicionadas(prev => new Set(prev).add(c.hanzi));
  };

  const emAnimacao = faseAnimacao !== 'parado';

  // Sincronização de status com o vocabulário real
  const statusPorHanzi = new Map((cartoesVocabulario || []).map(v => [v.Hanzi, v.Status]));

  return (
    <div className="overlay-baralho" onClick={aoFechar}>
      <div className="modal-baralho-container" onClick={e => e.stopPropagation()}>
        <button className="modal-baralho-fechar" onClick={aoFechar} title={t("Fechar")}>
          <IconeFechar size={18} />
        </button>

        <div className="modal-baralho-cabecalho">
          <h2 className="modal-baralho-titulo">
            <IconeBaralho size={26} /> {t("Recomendações")}
          </h2>
          <p className="modal-baralho-subtitulo">
            {t("Vire as cartas para descobrir palavras recomendadas para o seu estudo com base no seu aprendizado.")}
          </p>
        </div>

        <div className={`area-cartas-baralho fase-${faseAnimacao}`}>
          {cartas.map((c, i) => {
            const statusDB = statusPorHanzi.get(c.hanzi);
            const ehAdicionadaLocal = adicionadas.has(c.hanzi);
            const ehEstudo = ehAdicionadaLocal || statusDB === STATUS_VOCABULARIO.Estudo;
            const ehAprendida = statusDB === STATUS_VOCABULARIO.Aprendido;
            const hsk = (c as any).nivelHSK || (c as any).NivelHSK;
            const ranking = (c as any).posicaoRanking || (c as any).PosicaoRanking;

            return (
              <div
                key={c.hanzi + i}
                className={`carta-flip-container pos-slot-${i}`}
                onClick={() => aoClicarNaCarta(i, c)}
              >
                <div className={`carta-inner ${c.revelada ? 'revelada' : ''}`}>
                  {/* Verso da Carta */}
                  <div className="carta-verso">
                    <div className="carta-verso-moldura">
                      <div className="carta-verso-padrao">
                        <span className="carta-verso-icone"><IconeVersoCarta size={44} /></span>
                        <span className="carta-verso-texto">{t("Clique p/ virar")}</span>
                      </div>
                    </div>
                  </div>

                  {/* Frente da Carta */}
                  <div className="carta-frente">
                    <div className="carta-frente-topo">
                      <div className="motivo-badge" title={c.motivo}>
                        <IconeLampada size={13} />
                        <span className="motivo-texto">{t(c.motivo)}</span>
                      </div>

                      <div className="badges-meta-container">
                        {hsk > 0 && (
                          <span className="badge-meta badge-hsk">HSK {hsk}</span>
                        )}
                        {ranking > 0 && (
                          <span className="badge-meta badge-ranking">Top #{ranking}</span>
                        )}
                      </div>
                    </div>

                    <div className="carta-frente-conteudo">
                      <span className="carta-pinyin">{c.pinyin || '---'}</span>
                      <span className="carta-hanzi">{c.hanzi}</span>
                      <span className="carta-significado" title={c.significado}>
                        {c.significado || t('Sem tradução')}
                      </span>
                    </div>

                    <div className="carta-frente-rodape">
                      {ehAprendida ? (
                        <button
                          className="btn-adicionar-estudo btn-status-aprendida"
                          disabled
                          title={t("Esta palavra já foi marcada como aprendida")}
                        >
                          <IconeCheck size={14} /> {t("Já Aprendida")}
                        </button>
                      ) : ehEstudo ? (
                        <button
                          className="btn-adicionar-estudo btn-status-estudo"
                          disabled
                          title={t("Esta palavra já está na sua lista de estudo")}
                        >
                          <IconeEstudoAtivo size={13} /> {t("Em Estudo")}
                        </button>
                      ) : (
                        <button
                          className="btn-adicionar-estudo btn-status-novo"
                          onClick={e => adicionarAoEstudo(e, c)}
                        >
                          {t("+ Mover p/ Estudo")}
                        </button>
                      )}
                    </div>
                  </div>
                </div>
              </div>
            );
          })}
        </div>

        <div className="modal-baralho-rodape">
          <button
            className="btn-novas-cartas"
            disabled={emAnimacao}
            onClick={() => executarFluxoEmbaralhamento(true)}
          >
            <IconeBaralho size={18} /> {t("Tirar novas 3 cartas")}
          </button>
        </div>
      </div>
    </div>
  );
}
