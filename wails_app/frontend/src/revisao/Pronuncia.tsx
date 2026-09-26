import { useState, useEffect, useMemo, useRef } from 'react';
import { revisao } from '../../wailsjs/go/models';
import { useSTT } from '../comum/useSTT';
import { PopupRevisao } from './PopupRevisao';
import { t } from '../i18n/i18n';
import { DecomporTextoRevisao } from '../../wailsjs/go/main/App';
import { pronunciaCasa, TokenFalado } from './comparacaoPronuncia';
import './revisao.css';

// ----- Tempos das animações e dos feedbacks (ms) -----
// As transições dos cartões do baralho são inline e usam estas constantes; só o tremor do erro
// (revisao-tremor) vive no revisao.css, com duração própria.
const MS_FEEDBACK_COR = 800;            // cartão parado, verde/vermelho, antes de se mover
const MS_CARTAO_SAI_DO_BARALHO = 700;   // deslizar do acerto / descarte definitivo do erro
const MS_RECICLAR_FASE_SAIDA = 380;     // erro recuperável, fase 1: cartão levanta e sai de lado
const MS_RECICLAR_FASE_ENTRADA = 450;   // erro recuperável, fase 2: cartão mergulha atrás do baralho
const MS_EXIBIR_ERRO_SEQUENCIA = 2000;  // variante sequência: cartão vermelho antes de reciclar
const MS_EXIBIR_ACERTO_SEQUENCIA = 1000; // variante sequência: cartão verde antes de avançar

interface PronunciaProps {
  questao: revisao.QuestaoRevisao;
  respondida: boolean;
  aoConcluir: (acertou: boolean, foiPulada?: boolean) => void;
  aoTocarAudio: (texto: string) => void;
  hanziTocando: string | null;
  hanziSintetizando: string | null;
  AoClicarNoCartao?: (card: any) => void;
}

export function Pronuncia(props: PronunciaProps) {
  let conteudo;
  if (props.questao.variante === 'pronuncia_frase') {
    conteudo = <PronunciaFrase {...props} />;
  } else if (props.questao.variante === 'pronuncia_tipo') {
    conteudo = <PronunciaTipo {...props} />;
  } else if (props.questao.variante === 'pronuncia_baralho') {
    conteudo = <PronunciaBaralho {...props} />;
  } else {
    conteudo = <PronunciaSequencia {...props} />;
  }

  return conteudo;
}

// Mensagem exibida quando NENHUM caminho de STT existe (sem Web Speech e sem motor instalado).
// `motivo` orienta o próximo passo (baixar o motor em Configurações → Motores).
function PronunciaIndisponivel({ motivo }: { motivo: string | null }) {
  return (
    <div className="revisao-pronuncia-erro">
      {t('⚠️ Reconhecimento de voz indisponível.')}
      {motivo && <div style={{ marginTop: '6px', fontSize: '0.9em' }}>{t(motivo)}</div>}
    </div>
  );
}

function PronunciaFrase({ questao, respondida, aoConcluir, aoTocarAudio, AoClicarNoCartao }: PronunciaProps) {
  const [acertoPreviamenteDetectado, setAcertoPreviamenteDetectado] = useState(false);
  const [indicesAcertados, setIndicesAcertados] = useState<Set<number>>(new Set());
  const [tentativasRestantes, setTentativasRestantes] = useState(3);
  const tamanhoAcertosNoInicio = useRef(0);
  const gravouAlgumaVez = useRef(false);

  const tokens: revisao.PalavraRevisao[] = useMemo(() => {
    return questao.fraseOriginalSegmentada || [];
  }, [questao.fraseOriginalSegmentada]);

  const totalChineses = useMemo(() => {
    return tokens.filter((t: revisao.PalavraRevisao) => t.ehChines).length;
  }, [tokens]);

  const questaoAcertada = totalChineses > 0 ? (indicesAcertados.size / totalChineses) >= 0.75 : false;

  // Total de hanzis da frase — dimensiona os limites anti-escuta-infinita do useSTT
  const caracteresAlvo = useMemo(() => {
    return tokens
      .filter((t: revisao.PalavraRevisao) => t.ehChines)
      .reduce((soma: number, t: revisao.PalavraRevisao) => soma + t.texto.length, 0);
  }, [tokens]);

  const {
    suportado, motivoIndisponivel, escutando, processando,
    transcricaoParcial, transcricaoFinal, estadoMotor, iniciar, parar, limpar, erro,
    motivoParadaForcada, nivelAudio,
  } = useSTT({ idioma: 'zh-CN', continuo: true, caracteresAlvo });

  const transcricaoAtual = transcricaoFinal + (transcricaoParcial ? ' ' + transcricaoParcial : '');
  const { transcricaoEmPinyin, transcricaoEmHanzi, tokensFalados } = useTranscricaoDecomposta(transcricaoAtual);

  const pinyinAlvo = useMemo(() => {
    return tokens
      .map((t: revisao.PalavraRevisao) => t.ehChines ? t.pinyin : t.texto)
      .join(' ');
  }, [tokens]);

  const pinyinsFaltando = useMemo(() => {
    return tokens
      .filter((t: revisao.PalavraRevisao, i: number) => t.ehChines && t.pinyin && !indicesAcertados.has(i))
      .map((t: revisao.PalavraRevisao) => t.pinyin)
      .join(' ');
  }, [tokens, indicesAcertados]);

  // Track if recording has actually started during this session
  useEffect(() => {
    if (escutando) {
      gravouAlgumaVez.current = true;
    }
  }, [escutando]);

  // Detect correct pronunciation in real-time
  useEffect(() => {
    if (!escutando || respondida || tokensFalados.length === 0 || tokens.length === 0) return;

    setIndicesAcertados(prev => {
      const novosAcertos = new Set(prev);
      let mudou = false;

      for (let i = 0; i < tokens.length; i++) {
        const t = tokens[i];
        if (!t.ehChines || novosAcertos.has(i)) continue;

        if (pronunciaCasa({ hanzi: t.texto, pinyin: t.pinyin }, tokensFalados)) {
          novosAcertos.add(i);
          mudou = true;
          
          // "após algum hanzi ser acertado, todas as pronuncias do usuario são comparadas com todos os hanzis unicos até acertar o primeiro = break"
          if (prev.size > 0) {
            break;
          }
        }
      }

      if (mudou) {
        const todosChinesesAcertados = tokens.every((t: revisao.PalavraRevisao, i: number) => !t.ehChines || novosAcertos.has(i));
        if (todosChinesesAcertados) {
          setAcertoPreviamenteDetectado(true);
          parar();
        }
        return novosAcertos;
      }

      return prev;
    });
  }, [escutando, tokensFalados, tokens, respondida, parar]);



  // Evaluate the transcription when the microphone stops
  useEffect(() => {
    if (escutando || respondida || !gravouAlgumaVez.current) return;

    // Reset immediately to prevent multiple runs due to React rendering re-evaluations
    gravouAlgumaVez.current = false;

    const avaliarFinal = (acertosAtuais: Set<number>) => {
      limpar();
      const todosChinesesAcertados = tokens.every((t: revisao.PalavraRevisao, i: number) => !t.ehChines || acertosAtuais.has(i));
      
      const totalChinesesVal = tokens.filter((t: revisao.PalavraRevisao) => t.ehChines).length;
      const acertou75Pct = totalChinesesVal > 0 ? (acertosAtuais.size / totalChinesesVal) >= 0.75 : false;

      if (todosChinesesAcertados) {
        aoConcluir(true);
        return;
      }

      if (acertou75Pct) {
        // Já acertou 75%, não finaliza automaticamente mas também não perde vidas nem finaliza com erro
        return;
      }

      // Progresso parcial normalmente poupa a vida (o usuário continua tentando o resto da frase)
      // — MAS não quando o microfone fechou à força por excesso de fala/tempo: aí é para valer,
      // senão dava para ficar falando sem parar até acertar mais um token por vez sem custo algum.
      if (!motivoParadaForcada && acertosAtuais.size > tamanhoAcertosNoInicio.current) {
        return;
      }

      // Sem progresso (ou parada forçada): lose a life
      const novasVidas = tentativasRestantes - 1;
      setTentativasRestantes(novasVidas);

      if (novasVidas <= 0) {
        aoConcluir(false);
      }
    };

    if (acertoPreviamenteDetectado) {
      setAcertoPreviamenteDetectado(false);
      limpar();
      aoConcluir(true);
      return;
    }

    if (!transcricaoFinal) {
      avaliarFinal(indicesAcertados);
      return;
    }

    DecomporTextoRevisao(transcricaoFinal)
      .then(tokensSpoken => {
        if (!tokensSpoken) {
          avaliarFinal(indicesAcertados);
          return;
        }

        setIndicesAcertados(prev => {
          const novosAcertos = new Set(prev);
          let mudou = false;

          for (let i = 0; i < tokens.length; i++) {
            const t = tokens[i];
            if (!t.ehChines || novosAcertos.has(i)) continue;

            if (pronunciaCasa({ hanzi: t.texto, pinyin: t.pinyin }, tokensSpoken)) {
              novosAcertos.add(i);
              mudou = true;
              
              if (prev.size > 0) {
                break;
              }
            }
          }

          avaliarFinal(mudou ? novosAcertos : prev);
          return mudou ? novosAcertos : prev;
        });
      })
      .catch(() => {
        avaliarFinal(indicesAcertados);
      });
  }, [escutando, transcricaoFinal, acertoPreviamenteDetectado, respondida, tokens, aoConcluir, tentativasRestantes, indicesAcertados, motivoParadaForcada]);

  if (!suportado) {
    return <PronunciaIndisponivel motivo={motivoIndisponivel} />;
  }

  const aoClicarMicrofone = () => {
    if (escutando) {
      parar();
    } else {
      setAcertoPreviamenteDetectado(false);
      tamanhoAcertosNoInicio.current = indicesAcertados.size;
      iniciar();
    }
  };

  const aoClicarProsseguir = () => {
    if (escutando) {
      parar();
    }
    aoConcluir(true);
  };

  const aoPularAtividade = () => {
    if (escutando) {
      parar();
    }
    aoConcluir(false, true);
  };

  return (
    <div className="revisao-pronuncia-container">
      <div className="revisao-frase" style={{ textAlign: 'center', margin: '10px 0 20px 0' }}>
        <div style={{ display: 'flex', gap: '8px', alignItems: 'center', justifyContent: 'center' }}>
          <button
            className="revisao-btn-audio"
            style={{
              background: 'transparent',
              border: 'none',
              cursor: 'pointer',
              color: 'var(--cor-texto-primario)',
              padding: '6px',
              borderRadius: '50%',
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center',
              transition: 'background-color 0.2s'
            }}
            onClick={() => aoTocarAudio(questao.fraseOriginal)}
            title={t("Tocar áudio da frase")}
          >
            <svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
              <polygon points="11 5 6 9 2 9 2 15 6 15 11 19 11 5"></polygon>
              <path d="M19.07 4.93a10 10 0 0 1 0 14.14M15.54 8.46a5 5 0 0 1 0 7.07"></path>
            </svg>
          </button>
          <div 
            style={{ 
              display: 'flex', 
              flexWrap: 'wrap', 
              justifyContent: 'center', 
              alignItems: 'flex-end', 
              fontFamily: 'var(--fonte-hanzi)',
              gap: '4px 2px'
            }}
          >
            {tokens.map((t: revisao.PalavraRevisao, idx: number) => {
              const ehAcertado = indicesAcertados.has(idx);
              if (t.ehChines && t.pinyin) {
                return (
                  <div
                    key={idx}
                    style={{
                      display: 'inline-flex',
                      flexDirection: 'column',
                      alignItems: 'center',
                      margin: '0 4px',
                      verticalAlign: 'bottom'
                    }}
                  >
                    <span 
                      className={!ehAcertado && t.ehNaoVista ? 'revisao-pinyin-nao-visto' : ''}
                      style={{ 
                        fontSize: '13px', 
                        color: (!ehAcertado && t.ehNaoVista) ? undefined : 'var(--cor-pinyin, #a0aec0)', 
                        fontWeight: 'normal',
                        marginBottom: '2px',
                        userSelect: 'none'
                      }}
                    >
                      {t.pinyin}
                    </span>
                    <span
                      className={!ehAcertado && t.ehNaoVista ? 'revisao-palavra-nao-vista' : ''}
                      style={{
                        cursor: 'pointer',
                        transition: 'color 0.2s',
                        color: ehAcertado ? 'var(--cor-sucesso)' : ((!ehAcertado && t.ehNaoVista) ? undefined : 'inherit'),
                        fontWeight: ehAcertado ? 'bold' : 'normal',
                        fontSize: '26px'
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
                    fontSize: '26px', 
                    margin: '0 2px', 
                    verticalAlign: 'bottom',
                    alignSelf: 'flex-end'
                  }}
                >
                  {t.texto}
                </span>
              );
            })}
          </div>
        </div>
        <div style={{ marginTop: '8px', color: 'var(--cor-texto-suave)', fontSize: '14px' }}>
          {questao.fraseTraducao}
        </div>
      </div>

      <div style={{ display: 'flex', flexDirection: 'column', alignItems: 'center', gap: '8px' }}>
        <div style={{ display: 'flex', gap: '16px', alignItems: 'center', justifyContent: 'center' }}>
          <button
            className={`revisao-btn-microfone ${escutando ? 'escutando' : ''}`}
            onClick={aoClicarMicrofone}
            disabled={respondida || processando}
            style={escutando ? ({ '--nivel-audio': nivelAudio } as React.CSSProperties) : undefined}
          >
            <svg width="32" height="32" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
              <path d="M12 2a3 3 0 0 0-3 3v7a3 3 0 0 0 6 0V5a3 3 0 0 0-3-3Z"></path>
              <path d="M19 10v2a7 7 0 0 1-14 0v-2"></path>
              <line x1="12" y1="19" x2="12" y2="22"></line>
            </svg>
          </button>

          <button
            className={`revisao-btn-pular ${questaoAcertada ? 'prosseguir' : ''}`}
            onClick={questaoAcertada ? aoClicarProsseguir : aoPularAtividade}
            disabled={respondida || processando}
            title={questaoAcertada ? t("Prosseguir") : t("Pular atividade (falhar instantaneamente)")}
          >
            <svg width="22" height="22" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
              <polygon points="5 4 15 12 5 20 5 4" />
              <line x1="19" y1="5" x2="19" y2="19" />
            </svg>
          </button>
        </div>

        {!respondida && (
          <div className="revisao-cartao-tentativas" title={t("Tentativas restantes")}>
            {Array.from({ length: 3 }).map((_, i) => (
              <span 
                key={i} 
                className={`ponto-tentativa ${i < tentativasRestantes ? 'ativo' : 'gasto'}`}
                style={{ fontSize: '18px' }}
              >
                ❤
              </span>
            ))}
          </div>
        )}
      </div>

      <FeedbackPronuncia
        escutando={escutando}
        processando={processando}
        estadoMotor={estadoMotor}
        transcricaoEmPinyin={transcricaoEmPinyin}
        transcricaoEmHanzi={transcricaoEmHanzi}
      />

      {erro && <div className="revisao-pronuncia-erro">{t('Erro: {erro}', { erro })}</div>}

      <BotaoDicaPinyin pinyinFaltando={pinyinsFaltando} aoTocarAudio={aoTocarAudio} />
    </div>
  );
}

interface FilaItem {
  id: string;
  hanzi: string;
  pinyin: string;
  significados: string;
  erros?: number;
}

function PronunciaSequencia({ questao, respondida, aoConcluir, aoTocarAudio, AoClicarNoCartao }: PronunciaProps) {
  // Inicializa a fila de caracteres com base na frase segmentada (mantendo palavras compostas juntas)
  const filaInicial = useMemo(() => {
    const itens: FilaItem[] = [];
    questao.fraseOriginalSegmentada?.forEach((p: revisao.PalavraRevisao, idx: number) => {
      if (!p.ehChines) return;
      itens.push({
        id: `${idx}`,
        hanzi: p.texto,
        pinyin: p.pinyin,
        significados: p.significados?.join(', ') || '',
        erros: 0
      });
    });
    return itens;
  }, [questao]);

  const [fila, setFila] = useState<FilaItem[]>(filaInicial);
  const [erroAtual, setErroAtual] = useState<FilaItem | null>(null);
  const [acertoAtual, setAcertoAtual] = useState<FilaItem | null>(null);
  const [listaAcertos, setListaAcertos] = useState<string[]>([]);
  const questaoAcertada = useMemo(() => {
    return filaInicial.length > 0 ? (listaAcertos.length / filaInicial.length) >= 0.5 : false;
  }, [listaAcertos, filaInicial]);
  const [acertoPreviamenteDetectado, setAcertoPreviamenteDetectado] = useState(false);
  const [informacoesPopup, setInformacoesPopup] = useState<{
    hanzi: string;
    pinyin: string;
    significados: string;
    x: number;
    y: number;
  } | null>(null);

  const proximoAutoMicrofone = useRef(false);

  const {
    suportado, motivoIndisponivel, escutando, processando,
    transcricaoParcial, transcricaoFinal, estadoMotor, iniciar, parar, limpar, erro,
    motivoParadaForcada, nivelAudio,
  } = useSTT({ idioma: 'zh-CN', continuo: true, caracteresAlvo: fila[0]?.hanzi.length || 0 });

  const transcricaoAtual = transcricaoFinal + (transcricaoParcial ? ' ' + transcricaoParcial : '');
  const { transcricaoEmPinyin, transcricaoEmHanzi, tokensFalados } = useTranscricaoDecomposta(transcricaoAtual);

  const processarFalha = () => {
    if (fila.length === 0 || erroAtual || acertoAtual) return;

    proximoAutoMicrofone.current = false;
    const itemFalho = fila[0];
    const novosErros = (itemFalho.erros || 0) + 1;
    const itemAtualizado = { ...itemFalho, erros: novosErros };

    setErroAtual(itemAtualizado);
    aoTocarAudio(itemFalho.hanzi); // Toca o áudio de feedback

    setTimeout(() => {
      setErroAtual(null);
      limpar();

      if (novosErros >= 3) {
        // Remove completamente o card por excesso de erros
        const novaFila = fila.slice(1);
        setFila(novaFila);

        if (novaFila.length === 0) {
          const taxaSucesso = listaAcertos.length / filaInicial.length;
          aoConcluir(taxaSucesso >= 0.5);
        }
      } else {
        // Move o cartão updated com mais um erro para o final da fila
        setFila(prev => {
          if (prev.length === 0) return prev;
          const [primeiro, ...resto] = prev;
          return [...resto, itemAtualizado];
        });
      }
    }, MS_EXIBIR_ERRO_SEQUENCIA);
  };


  const processarSucesso = () => {
    if (fila.length === 0 || erroAtual || acertoAtual) return;

    proximoAutoMicrofone.current = true;
    const itemSucesso = fila[0];
    setAcertoAtual(itemSucesso);
    
    // Registra o ID do card na lista de acertos
    const novaListaAcertos = [...listaAcertos, itemSucesso.id];
    setListaAcertos(novaListaAcertos);

    setTimeout(() => {
      setAcertoAtual(null);
      limpar();
      const novaFila = fila.slice(1);
      setFila(novaFila);

      if (novaFila.length === 0) {
        const taxaSucesso = novaListaAcertos.length / filaInicial.length;
        aoConcluir(taxaSucesso >= 0.5);
      }
    }, MS_EXIBIR_ACERTO_SEQUENCIA);
  };

  // Detect correct pronunciation in real-time
  useEffect(() => {
    if (!escutando || respondida || fila.length === 0 || tokensFalados.length === 0) return;
    if (pronunciaCasa({ hanzi: fila[0].hanzi, pinyin: fila[0].pinyin }, tokensFalados)) {
      setAcertoPreviamenteDetectado(true);
      parar();
    }
  }, [escutando, tokensFalados, fila, respondida, parar]);



  // Final evaluation of transcription
  useEffect(() => {
    if (escutando || respondida || fila.length === 0 || acertoAtual || erroAtual) return;

    const finalizarEdicao = (acertou: boolean) => {
      if (acertou) {
        processarSucesso();
      } else {
        processarFalha();
      }
    };

    if (acertoPreviamenteDetectado) {
      setAcertoPreviamenteDetectado(false);
      finalizarEdicao(true);
      return;
    }

    // Microfone fechado à força (excesso de fala/tempo esgotado): conta direto como erro, sem
    // reavaliar a transcrição — se já tivesse acertado, a detecção em tempo real já teria parado.
    if (motivoParadaForcada) {
      finalizarEdicao(false);
      return;
    }

    const transcricaoParaAvaliar = transcricaoFinal || transcricaoParcial;
    if (!transcricaoParaAvaliar) return;

    DecomporTextoRevisao(transcricaoParaAvaliar)
      .then(tokens => {
        if (!tokens) {
          finalizarEdicao(false);
          return;
        }
        finalizarEdicao(pronunciaCasa({ hanzi: fila[0].hanzi, pinyin: fila[0].pinyin }, tokens));
      })
      .catch(() => {
        finalizarEdicao(false);
      });
  }, [escutando, transcricaoFinal, transcricaoParcial, acertoPreviamenteDetectado, respondida, fila, limpar, acertoAtual, erroAtual, motivoParadaForcada]);

  // Auto-abre o microfone só quando a fila AVANÇA para um novo card (após acerto). O efeito NÃO
  // pode depender de `iniciar`: como `iniciar` troca de identidade a cada mudança de `escutando`,
  // ele reexecutaria no mesmo commit em que a escuta fecha e reabriria o microfone no meio da
  // animação de acerto — deixando o microfone preso aberto pelo baralho inteiro. Por isso lê
  // `iniciar` por ref e depende apenas de `fila`.
  const iniciarRef = useRef(iniciar);
  iniciarRef.current = iniciar;
  useEffect(() => {
    if (fila.length > 0 && proximoAutoMicrofone.current) {
      proximoAutoMicrofone.current = false;
      setAcertoPreviamenteDetectado(false);
      iniciarRef.current();
    }
  }, [fila]);

  if (!suportado) {
    return <PronunciaIndisponivel motivo={motivoIndisponivel} />;
  }

  if (fila.length === 0) {
    return (
      <div className="revisao-pronuncia-container resumo">
        <div className="revisao-resumo-lista">
          {filaInicial.map((item) => {
            const acertou = listaAcertos.includes(item.id);
            return (
              <div 
                key={item.id} 
                className={`revisao-resumo-card ${acertou ? 'correto' : 'incorreto'}`}
                onMouseEnter={(e) => {
                  const rect = e.currentTarget.getBoundingClientRect();
                  setInformacoesPopup({
                    pinyin: item.pinyin,
                    hanzi: item.hanzi,
                    significados: item.significados,
                    x: rect.left + rect.width / 2,
                    y: rect.top
                  });
                }}
                onMouseLeave={() => {
                  setInformacoesPopup(null);
                }}
                onClick={() => {
                  if (AoClicarNoCartao) {
                    AoClicarNoCartao({
                      Hanzi: item.hanzi,
                      Pinyin: item.pinyin,
                      significados: item.significados ? item.significados.split(', ') : []
                    });
                  }
                }}
              >
                <span className="pinyin-feedback">{item.pinyin}</span>
                <span className="hanzi">{item.hanzi}</span>
                <span className="significado">{item.significados}</span>
              </div>
            );
          })}
        </div>
        <PopupRevisao info={informacoesPopup} />
      </div>
    );
  }

  const aoClicarMicrofone = () => {
    if (escutando) {
      parar();
    } else {
      setAcertoPreviamenteDetectado(false);
      iniciar();
    }
  };

  const aoDesistirDoCartao = () => {
    if (escutando) {
      parar();
    }
    processarFalha();
  };

  const aoClicarProsseguir = () => {
    if (escutando) {
      parar();
    }
    aoConcluir(true);
  };

  const aoPularAtividade = () => {
    if (escutando) {
      parar();
    }
    aoConcluir(false, true);
  };

  return (
    <div className="revisao-pronuncia-container">
      <div className="revisao-pronuncia-instrucao">
        {t('Fale o termo em destaque. Clique no microfone para falar.')}
      </div>

      <div className="revisao-pronuncia-sequencia">
        {fila.slice(0, 3).map((item, index) => {
          const larguraCartao = item.hanzi.length * (index === 0 ? 90 : 70);
          return (
            <div
              key={item.id}
              className={`revisao-cartao-pronuncia ${index === 0 ? 'destaque' : 'fila'} ${erroAtual?.id === item.id ? 'erro' : ''} ${acertoAtual?.id === item.id ? 'acerto' : ''}`}
              style={{ cursor: 'pointer', width: `${larguraCartao}px` }}
              onMouseEnter={(e) => {
                const rect = e.currentTarget.getBoundingClientRect();
                setInformacoesPopup({
                  pinyin: item.pinyin,
                  hanzi: item.hanzi,
                  significados: item.significados,
                  x: rect.left + rect.width / 2,
                  y: rect.top
                });
              }}
              onMouseLeave={() => {
                setInformacoesPopup(null);
              }}
              onClick={() => {
                if (!AoClicarNoCartao) return;
                AoClicarNoCartao({
                  Hanzi: item.hanzi,
                  Pinyin: item.pinyin,
                  significados: item.significados ? item.significados.split(', ') : []
                });
              }}
            >
              {index === 0 && (erroAtual?.id === item.id || acertoAtual?.id === item.id) && (
                <span 
                  className="pinyin-feedback"
                  style={{ color: acertoAtual?.id === item.id ? 'var(--cor-sucesso)' : 'var(--cor-perigo)' }}
                >
                  {item.pinyin}
                </span>
              )}
              <span className="hanzi">{item.hanzi}</span>
              {index === 0 && !erroAtual && !acertoAtual && (
                <div className="revisao-cartao-tentativas" title={t("Tentativas restantes")}>
                  {Array.from({ length: 3 }).map((_, i) => (
                    <span 
                      key={i} 
                      className={`ponto-tentativa ${i < (3 - (item.erros || 0)) ? 'ativo' : 'gasto'}`}
                    >
                      ❤
                    </span>
                  ))}
                </div>
              )}
            </div>
          );
        })}
        {fila.length > 3 && <div className="revisao-cartao-pronuncia mais">+ {fila.length - 3}</div>}
      </div>

      <div style={{ display: 'flex', flexDirection: 'column', alignItems: 'center', gap: '8px', marginTop: '10px' }}>
        <div style={{ display: 'flex', gap: '16px', alignItems: 'center', justifyContent: 'center' }}>
          <button
            className={`revisao-btn-microfone ${escutando ? 'escutando' : ''}`}
            onClick={aoClicarMicrofone}
            disabled={respondida || processando || erroAtual !== null || acertoAtual !== null}
            style={escutando ? ({ '--nivel-audio': nivelAudio } as React.CSSProperties) : undefined}
          >
            <svg width="32" height="32" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
              <path d="M12 2a3 3 0 0 0-3 3v7a3 3 0 0 0 6 0V5a3 3 0 0 0-3-3Z"></path>
              <path d="M19 10v2a7 7 0 0 1-14 0v-2"></path>
              <line x1="12" y1="19" x2="12" y2="22"></line>
            </svg>
          </button>

          <button
            className="revisao-btn-desistir"
            onClick={aoDesistirDoCartao}
            disabled={respondida || processando || erroAtual !== null || acertoAtual !== null}
            title={t("Desistir do cartão (Reciclar)")}
          >
            <svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
              <path d="M7 11V7a5 5 0 0 1 5-5c1.38 0 2.63.56 3.54 1.46L19 7"/>
              <path d="M2 13h4.14c.9 0 1.54.56 2.46 1.46L12 18"/>
              <path d="M22 17v-4a5 5 0 0 0-5-5c-1.38 0-2.63.56-3.54 1.46L10 13"/>
              <path d="M17 22h-4.14c-.9 0-1.54-.56-2.46-1.46L7 17"/>
            </svg>
          </button>

          <button
            className={`revisao-btn-pular ${questaoAcertada ? 'prosseguir' : ''}`}
            onClick={questaoAcertada ? aoClicarProsseguir : aoPularAtividade}
            disabled={respondida || processando || erroAtual !== null || acertoAtual !== null}
            title={questaoAcertada ? t("Prosseguir") : t("Pular atividade (falhar instantaneamente)")}
          >
            <svg width="22" height="22" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
              <polygon points="5 4 15 12 5 20 5 4" />
              <line x1="19" y1="5" x2="19" y2="19" />
            </svg>
          </button>
        </div>
      </div>

      <FeedbackPronuncia
        escutando={escutando}
        processando={processando}
        estadoMotor={estadoMotor}
        transcricaoEmPinyin={transcricaoEmPinyin}
        transcricaoEmHanzi={transcricaoEmHanzi}
      />

      {erro && <div className="revisao-pronuncia-erro">{t('Erro: {erro}', { erro })}</div>}

      <PopupRevisao info={informacoesPopup} />
      <BotaoDicaPinyin pinyinFaltando={fila[0]?.pinyin || ''} aoTocarAudio={aoTocarAudio} />
    </div>
  );
}

function PronunciaBaralho({ questao, respondida, aoConcluir, aoTocarAudio, AoClicarNoCartao, modoDicaInicial = 'consoantes' }: PronunciaProps & { modoDicaInicial?: 'consoantes' | 'vogais' }) {
  const filaInicial = useMemo(() => {
    const itens: FilaItem[] = [];
    questao.fraseOriginalSegmentada?.forEach((p: revisao.PalavraRevisao, idx: number) => {
      if (!p.ehChines) return;
      itens.push({
        id: `${idx}`,
        hanzi: p.texto,
        pinyin: p.pinyin,
        significados: p.significados?.join(', ') || '',
        erros: 0
      });
    });
    return itens;
  }, [questao]);

  const [fila, setFila] = useState<FilaItem[]>(filaInicial);
  // Estados do erro recuperável ('erro-reciclar-*'): o cartão sai pela lateral (fase saída) e
  // mergulha ATRÁS do baralho até a última posição da pilha (fase entrada), em vez de só cair.
  const [estadoAnimacao, setEstadoAnimacao] = useState<'normal' | 'acerto-verde' | 'acerto-deslizar' | 'erro-vermelho' | 'erro-descartar' | 'erro-reciclar-saida' | 'erro-reciclar-entrada'>('normal');
  const [listaAcertos, setListaAcertos] = useState<string[]>([]);
  const questaoAcertada = useMemo(() => {
    return filaInicial.length > 0 ? (listaAcertos.length / filaInicial.length) >= 0.5 : false;
  }, [listaAcertos, filaInicial]);
  const [acertoPreviamenteDetectado, setAcertoPreviamenteDetectado] = useState(false);
  const [informacoesPopup, setInformacoesPopup] = useState<{
    hanzi: string;
    pinyin: string;
    significados: string;
    x: number;
    y: number;
  } | null>(null);

  const proximoAutoMicrofone = useRef(false);

  const {
    suportado, motivoIndisponivel, escutando, processando,
    transcricaoParcial, transcricaoFinal, estadoMotor, iniciar, parar, limpar, erro,
    motivoParadaForcada, nivelAudio,
  } = useSTT({ idioma: 'zh-CN', continuo: true, caracteresAlvo: fila[0]?.hanzi.length || 0 });

  const transcricaoAtual = transcricaoFinal + (transcricaoParcial ? ' ' + transcricaoParcial : '');
  const { transcricaoEmPinyin, transcricaoEmHanzi, tokensFalados } = useTranscricaoDecomposta(transcricaoAtual);

  const processarFalha = () => {
    if (fila.length === 0 || estadoAnimacao !== 'normal') return;

    proximoAutoMicrofone.current = false;
    const itemFalho = fila[0];
    const novosErros = (itemFalho.erros || 0) + 1;
    const itemAtualizado = { ...itemFalho, erros: novosErros };

    aoTocarAudio(itemFalho.hanzi); // Toca o pinyin de feedback

    // 1º passo: Cartão fica vermelho por 800ms
    setEstadoAnimacao('erro-vermelho');

    // 2º passo: descarte definitivo (3 erros) ou reciclagem em arco para trás do baralho
    setTimeout(() => {
      if (novosErros >= 3) {
        setEstadoAnimacao('erro-descartar');

        setTimeout(() => {
          setEstadoAnimacao('normal');
          limpar();
          const novaFila = fila.slice(1);
          setFila(novaFila);

          if (novaFila.length === 0) {
            const taxaSucesso = listaAcertos.length / filaInicial.length;
            aoConcluir(taxaSucesso >= 0.5);
          }
        }, MS_CARTAO_SAI_DO_BARALHO);
        return;
      }

      if (fila.length === 1) {
        setEstadoAnimacao('normal');
        limpar();
        setFila([itemAtualizado]);
        return;
      }

      // Fase 1: o cartão levanta e sai pela lateral, ainda na frente do baralho
      setEstadoAnimacao('erro-reciclar-saida');

      setTimeout(() => {
        // Fase 2: passa para TRÁS do baralho (zIndex baixo) e mergulha até o fim da pilha
        setEstadoAnimacao('erro-reciclar-entrada');

        setTimeout(() => {
          setEstadoAnimacao('normal');
          limpar();
          setFila(prev => {
            if (prev.length === 0) return prev;
            const [primeiro, ...resto] = prev;
            return [...resto, itemAtualizado];
          });
        }, MS_RECICLAR_FASE_ENTRADA);
      }, MS_RECICLAR_FASE_SAIDA);
    }, MS_FEEDBACK_COR);
  };

  const processarSucesso = () => {
    if (fila.length === 0 || estadoAnimacao !== 'normal') return;

    proximoAutoMicrofone.current = true;
    const itemSucesso = fila[0];
    
    // 1º passo: Cartão fica verde por 800ms
    setEstadoAnimacao('acerto-verde');

    const novaListaAcertos = [...listaAcertos, itemSucesso.id];
    setListaAcertos(novaListaAcertos);

    // 2º passo: Inicia a animação de deslizamento para a direita
    setTimeout(() => {
      setEstadoAnimacao('acerto-deslizar');

      setTimeout(() => {
        setEstadoAnimacao('normal');
        limpar();
        const novaFila = fila.slice(1);
        setFila(novaFila);

        if (novaFila.length === 0) {
          const taxaSucesso = novaListaAcertos.length / filaInicial.length;
          aoConcluir(taxaSucesso >= 0.5);
        }
      }, MS_CARTAO_SAI_DO_BARALHO);
    }, MS_FEEDBACK_COR);
  };

  // Detect correct pronunciation in real-time
  useEffect(() => {
    if (!escutando || respondida || fila.length === 0 || tokensFalados.length === 0) return;
    if (pronunciaCasa({ hanzi: fila[0].hanzi, pinyin: fila[0].pinyin }, tokensFalados)) {
      setAcertoPreviamenteDetectado(true);
      parar();
    }
  }, [escutando, tokensFalados, fila, respondida, parar]);



  // Final evaluation of transcription
  useEffect(() => {
    if (escutando || respondida || fila.length === 0 || estadoAnimacao !== 'normal') return;

    const finalizarEdicao = (acertou: boolean) => {
      if (acertou) {
        processarSucesso();
      } else {
        processarFalha();
      }
    };

    if (acertoPreviamenteDetectado) {
      setAcertoPreviamenteDetectado(false);
      finalizarEdicao(true);
      return;
    }

    // Microfone fechado à força (excesso de fala/tempo esgotado): conta direto como erro, sem
    // reavaliar a transcrição — se já tivesse acertado, a detecção em tempo real já teria parado.
    if (motivoParadaForcada) {
      finalizarEdicao(false);
      return;
    }

    const transcricaoParaAvaliar = transcricaoFinal || transcricaoParcial;
    if (!transcricaoParaAvaliar) return;

    DecomporTextoRevisao(transcricaoParaAvaliar)
      .then(tokens => {
        if (!tokens) {
          finalizarEdicao(false);
          return;
        }
        finalizarEdicao(pronunciaCasa({ hanzi: fila[0].hanzi, pinyin: fila[0].pinyin }, tokens));
      })
      .catch(() => {
        finalizarEdicao(false);
      });
  }, [escutando, transcricaoFinal, transcricaoParcial, acertoPreviamenteDetectado, respondida, fila, limpar, estadoAnimacao, motivoParadaForcada]);

  // Auto-abre o microfone só quando a fila AVANÇA para um novo card (após acerto). O efeito NÃO
  // pode depender de `iniciar`: como `iniciar` troca de identidade a cada mudança de `escutando`,
  // ele reexecutaria no mesmo commit em que a escuta fecha e reabriria o microfone no meio da
  // animação de acerto — deixando o microfone preso aberto pelo baralho inteiro. Por isso lê
  // `iniciar` por ref e depende apenas de `fila`.
  const iniciarRef = useRef(iniciar);
  iniciarRef.current = iniciar;
  useEffect(() => {
    if (fila.length > 0 && proximoAutoMicrofone.current) {
      proximoAutoMicrofone.current = false;
      setAcertoPreviamenteDetectado(false);
      iniciarRef.current();
    }
  }, [fila]);

  if (!suportado) {
    return <PronunciaIndisponivel motivo={motivoIndisponivel} />;
  }

  if (fila.length === 0) {
    return (
      <div className="revisao-pronuncia-container resumo">
        <div className="revisao-resumo-lista">
          {filaInicial.map((item) => {
            const acertou = listaAcertos.includes(item.id);
            return (
              <div 
                key={item.id} 
                className={`revisao-resumo-card ${acertou ? 'correto' : 'incorreto'}`}
                onMouseEnter={(e) => {
                  const rect = e.currentTarget.getBoundingClientRect();
                  setInformacoesPopup({
                    pinyin: item.pinyin,
                    hanzi: item.hanzi,
                    significados: item.significados,
                    x: rect.left + rect.width / 2,
                    y: rect.top
                  });
                }}
                onMouseLeave={() => {
                  setInformacoesPopup(null);
                }}
                onClick={() => {
                  if (AoClicarNoCartao) {
                    AoClicarNoCartao({
                      Hanzi: item.hanzi,
                      Pinyin: item.pinyin,
                      significados: item.significados ? item.significados.split(', ') : []
                    });
                  }
                }}
              >
                <span className="pinyin-feedback">{item.pinyin}</span>
                <span className="hanzi">{item.hanzi}</span>
                <span className="significado">{item.significados}</span>
              </div>
            );
          })}
        </div>
        <PopupRevisao info={informacoesPopup} />
      </div>
    );
  }

  const aoClicarMicrofone = () => {
    if (escutando) {
      parar();
    } else {
      setAcertoPreviamenteDetectado(false);
      iniciar();
    }
  };

  const aoDesistirDoCartao = () => {
    if (escutando) {
      parar();
    }
    processarFalha();
  };

  const aoClicarProsseguir = () => {
    if (escutando) {
      parar();
    }
    aoConcluir(true);
  };

  const aoPularAtividade = () => {
    if (escutando) {
      parar();
    }
    aoConcluir(false, true);
  };

  const calcularEstiloCard = (index: number) => {
    const reciclando = estadoAnimacao === 'erro-reciclar-saida' || estadoAnimacao === 'erro-reciclar-entrada';
    const topoSaindo = estadoAnimacao === 'acerto-deslizar' ||
                       estadoAnimacao === 'erro-descartar' ||
                       reciclando;

    if (index === 0) {
      let transformStr = 'scale(1) translateY(0) rotate(0deg)';
      let opacityVal = 1;
      let zIndexVal = 100;
      let transitionStr = 'border-color 0.2s, box-shadow 0.2s';
      let pointerEventsVal: 'auto' | 'none' = 'auto';

      if (estadoAnimacao === 'acerto-deslizar') {
        transformStr = 'translateX(190%) translateY(-40px) rotate(18deg) scale(1.02)';
        opacityVal = 0;
        transitionStr = `all ${MS_CARTAO_SAI_DO_BARALHO}ms cubic-bezier(0.35, 0, 0.65, 1)`;
        pointerEventsVal = 'none';
      } else if (estadoAnimacao === 'erro-descartar') {
        transformStr = 'translateX(-190%) translateY(-40px) rotate(-18deg) scale(1.02)';
        opacityVal = 0;
        transitionStr = `all ${MS_CARTAO_SAI_DO_BARALHO}ms cubic-bezier(0.35, 0, 0.65, 1)`;
        pointerEventsVal = 'none';
      } else if (estadoAnimacao === 'erro-reciclar-saida') {
        // Fase 1 do erro recuperável: sai pela lateral, ainda na FRENTE do baralho (-60% cabe na
        // .revisao-baralho-area de 350px com overflow hidden sem o cartão ser cortado)
        transformStr = 'translateX(-60%) translateY(10px) rotate(-8deg) scale(0.95)';
        transitionStr = `all ${MS_RECICLAR_FASE_SAIDA}ms cubic-bezier(0.4, 0, 0.7, 1)`;
        pointerEventsVal = 'none';
      } else if (estadoAnimacao === 'erro-reciclar-entrada') {
        // Fase 2: o zIndex cai na hora (o cartão "passa para trás") e ele mergulha até a última
        // posição da pilha, apagando como os cartões do fundo
        transformStr = 'translateX(0) translateY(-42px) rotate(0deg) scale(0.82)';
        opacityVal = 0;
        zIndexVal = 1;
        transitionStr = `all ${MS_RECICLAR_FASE_ENTRADA}ms cubic-bezier(0.25, 0.8, 0.25, 1)`;
        pointerEventsVal = 'none';
      }

      return {
        transform: transformStr,
        opacity: opacityVal,
        zIndex: zIndexVal,
        pointerEvents: pointerEventsVal,
        transition: transitionStr,
      };
    }

    // Enquanto o topo sai, os demais cartões já avançam uma posição na pilha
    const visualIndex = topoSaindo ? index - 1 : index;
    const scaleVal = Math.max(0.85, 1 - visualIndex * 0.05);
    const translateYVal = visualIndex * -12;
    const opacityVal = visualIndex > 2 ? 0 : 1 - visualIndex * 0.35;
    const zIndexVal = 100 - index;

    return {
      transform: `scale(${scaleVal}) translateY(${translateYVal}px)`,
      opacity: opacityVal,
      zIndex: zIndexVal,
      pointerEvents: 'none' as const,
      transition: `all ${MS_CARTAO_SAI_DO_BARALHO}ms cubic-bezier(0.25, 0.8, 0.25, 1)`,
    };
  };

  const emFeedbackDeAcerto = estadoAnimacao === 'acerto-verde' || estadoAnimacao === 'acerto-deslizar';
  const emFeedbackDeErro = estadoAnimacao === 'erro-vermelho' || estadoAnimacao === 'erro-descartar' ||
                           estadoAnimacao === 'erro-reciclar-saida' || estadoAnimacao === 'erro-reciclar-entrada';

  return (
    <div className="revisao-pronuncia-container">
      <div className="revisao-pronuncia-instrucao">
        {t('Pronuncie o caractere do topo do baralho.')}
      </div>

      <div className="revisao-baralho-area">
        {fila.slice(0, 4).map((item, index) => {
          const estilo = calcularEstiloCard(index);
          const isTop = index === 0;

          return (
            <div
              key={item.id}
              className={`revisao-baralho-card ${isTop ? 'destaque' : ''} ${isTop && emFeedbackDeAcerto ? 'acerto-anim' : ''} ${isTop && emFeedbackDeErro ? 'erro-anim' : ''} ${isTop && fila.length === 1 && estadoAnimacao === 'erro-vermelho' ? 'shake' : ''}`}
              style={estilo}
              onMouseEnter={(e) => {
                if (!isTop) return;
                const rect = e.currentTarget.getBoundingClientRect();
                setInformacoesPopup({
                  pinyin: item.pinyin,
                  hanzi: item.hanzi,
                  significados: item.significados,
                  x: rect.left + rect.width / 2,
                  y: rect.top
                });
              }}
              onMouseLeave={() => {
                setInformacoesPopup(null);
              }}
              onClick={() => {
                if (!isTop || !AoClicarNoCartao) return;
                AoClicarNoCartao({
                  Hanzi: item.hanzi,
                  Pinyin: item.pinyin,
                  significados: item.significados ? item.significados.split(', ') : []
                });
              }}
            >
              <span
                className="pinyin-feedback"
                style={{
                  color: isTop && emFeedbackDeAcerto
                    ? 'var(--cor-sucesso)'
                    : isTop && emFeedbackDeErro
                      ? 'var(--cor-perigo)'
                      : 'var(--cor-pinyin, #a0aec0)'
                }}
              >
                {item.pinyin}
              </span>
              <span className="hanzi">{item.hanzi}</span>
              <div className="revisao-cartao-tentativas" title={t("Tentativas restantes")}>
                {Array.from({ length: 3 }).map((_, i) => (
                  <span 
                    key={i} 
                    className={`ponto-tentativa ${i < (3 - (item.erros || 0)) ? 'ativo' : 'gasto'}`}
                    style={{ fontSize: '16px' }}
                  >
                    ❤
                  </span>
                ))}
              </div>
            </div>
          );
        })}
      </div>

      <div style={{ display: 'flex', flexDirection: 'column', alignItems: 'center', gap: '8px', marginTop: '10px' }}>
        <div style={{ display: 'flex', gap: '16px', alignItems: 'center', justifyContent: 'center' }}>
          <button
            className={`revisao-btn-microfone ${escutando ? 'escutando' : ''}`}
            onClick={aoClicarMicrofone}
            disabled={respondida || processando || estadoAnimacao !== 'normal'}
            style={escutando ? ({ '--nivel-audio': nivelAudio } as React.CSSProperties) : undefined}
          >
            <svg width="32" height="32" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
              <path d="M12 2a3 3 0 0 0-3 3v7a3 3 0 0 0 6 0V5a3 3 0 0 0-3-3Z"></path>
              <path d="M19 10v2a7 7 0 0 1-14 0v-2"></path>
              <line x1="12" y1="19" x2="12" y2="22"></line>
            </svg>
          </button>

          <button
            className="revisao-btn-desistir"
            onClick={aoDesistirDoCartao}
            disabled={respondida || processando || estadoAnimacao !== 'normal'}
            title={t("Desistir do cartão (Reciclar)")}
          >
            <svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
              <path d="M7 11V7a5 5 0 0 1 5-5c1.38 0 2.63.56 3.54 1.46L19 7"/>
              <path d="M2 13h4.14c.9 0 1.54.56 2.46 1.46L12 18"/>
              <path d="M22 17v-4a5 5 0 0 0-5-5c-1.38 0-2.63.56-3.54 1.46L10 13"/>
              <path d="M17 22h-4.14c-.9 0-1.54-.56-2.46-1.46L7 17"/>
            </svg>
          </button>

          <button
            className={`revisao-btn-pular ${questaoAcertada ? 'prosseguir' : ''}`}
            onClick={questaoAcertada ? aoClicarProsseguir : aoPularAtividade}
            disabled={respondida || processando || estadoAnimacao !== 'normal'}
            title={questaoAcertada ? t("Prosseguir") : t("Pular atividade (falhar instantaneamente)")}
          >
            <svg width="22" height="22" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
              <polygon points="5 4 15 12 5 20 5 4" />
              <line x1="19" y1="5" x2="19" y2="19" />
            </svg>
          </button>
        </div>
      </div>

      <FeedbackPronuncia
        escutando={escutando}
        processando={processando}
        estadoMotor={estadoMotor}
        transcricaoEmPinyin={transcricaoEmPinyin}
        transcricaoEmHanzi={transcricaoEmHanzi}
      />

      {erro && <div className="revisao-pronuncia-erro">{t('Erro: {erro}', { erro })}</div>}

      <PopupRevisao info={informacoesPopup} />
      <BotaoDicaPinyin pinyinFaltando={fila[0]?.pinyin || ''} aoTocarAudio={aoTocarAudio} modoInicial={modoDicaInicial} />
    </div>
  );
}


// ----- Utilitários compartilhados pelas três variantes -----

// Converte a transcrição corrente (final + parcial) em pinyin/hanzi exibíveis e nos tokens usados
// pela comparação de pronúncia, via DecomporTextoRevisao do backend.
function useTranscricaoDecomposta(transcricaoAtual: string) {
  const [transcricaoEmPinyin, setTranscricaoEmPinyin] = useState('');
  const [transcricaoEmHanzi, setTranscricaoEmHanzi] = useState('');
  const [tokensFalados, setTokensFalados] = useState<TokenFalado[]>([]);

  useEffect(() => {
    if (!transcricaoAtual) {
      setTranscricaoEmPinyin('');
      setTranscricaoEmHanzi('');
      setTokensFalados([]);
      return;
    }

    // Guard de corrida: com parciais em tempo real, `transcricaoAtual` muda muito rápido e dispara
    // várias DecomporTextoRevisao sobrepostas. Sem isto, uma resposta antiga pode resolver DEPOIS
    // de uma mais nova e sobrescrever os tokens com uma transcrição desatualizada — fazendo o hanzi
    // correto sumir antes da detecção contabilizar. O cleanup invalida a resposta pendente ao trocar
    // a transcrição, então só a mais recente é aplicada.
    let atual = true;
    DecomporTextoRevisao(transcricaoAtual)
      .then(tokens => {
        if (!atual || !tokens) return;
        setTranscricaoEmPinyin(tokens.map(t => t.ehChines ? t.pinyin : t.texto).join(' '));
        setTranscricaoEmHanzi(tokens.map(t => t.texto).join(''));
        setTokensFalados(tokens);
      })
      .catch(() => {});

    return () => {
      atual = false;
    };
  }, [transcricaoAtual]);

  return { transcricaoEmPinyin, transcricaoEmHanzi, tokensFalados };
}


interface FeedbackPronunciaProps {
  escutando: boolean;
  processando: boolean;
  estadoMotor: string;
  transcricaoEmPinyin: string;
  transcricaoEmHanzi: string;
  nivelAudio?: number;
}

// Linha de status abaixo do microfone: transcrição reconhecida (pinyin + hanzi) > barras de som
// enquanto ouve > spinner enquanto transcreve > dica de uso quando parado.
function FeedbackPronuncia({ escutando, processando, estadoMotor, transcricaoEmPinyin, transcricaoEmHanzi, nivelAudio = 0 }: FeedbackPronunciaProps) {
  if (transcricaoEmPinyin) {
    return (
      <div className="revisao-pronuncia-feedback">
        <div className="revisao-pronuncia-transcricao">
          <div className="pinyin">{transcricaoEmPinyin}</div>
          <div className="hanzi">{transcricaoEmHanzi}</div>
        </div>
      </div>
    );
  }

  if (escutando) {
    return (
      <div className="revisao-pronuncia-feedback">
        <div className="revisao-pronuncia-status ouvindo">
          <span className="barras-som" aria-hidden="true" style={{ '--nivel-audio': nivelAudio } as React.CSSProperties}>
            <span /><span /><span /><span /><span />
          </span>
          {t('Ouvindo... pode falar')}
        </div>
      </div>
    );
  }

  if (processando) {
    return (
      <div className="revisao-pronuncia-feedback">
        <div className="revisao-pronuncia-status processando">
          <span className="revisao-spinner" aria-hidden="true" />
          {t(estadoMotor) || t('Transcrevendo...')}
        </div>
      </div>
    );
  }

  return (
    <div className="revisao-pronuncia-feedback">
      <div className="revisao-pronuncia-status dica">
        {t(estadoMotor) || t('Clique no microfone e fale')}
      </div>
    </div>
  );
}

// ----- Painel de Dicas de Pronúncia do Pinyin (Consoantes) -----

// ----- Painel de Dicas de Pronúncia do Pinyin (Consoantes e Vogais) -----

interface LinhaConsoante {
  consoante: string;
  ipa: string;
  dica: string;
  exemploHanzi: string;
  exemploPinyin: string;
}

interface SecaoConsoantes {
  titulo: string;
  linhas: LinhaConsoante[];
}

interface LinhaVogal {
  vogal: string;
  ipa: string;
  dica: string;
  exemploHanzi: string;
  exemploPinyin: string;
}

interface SecaoVogais {
  titulo: string;
  linhas: LinhaVogal[];
}

const LISTA_CONSOANTES_PINYIN = [
  'zh', 'ch', 'sh', 'b', 'p', 'd', 't', 'g', 'k', 
  'z', 'c', 's', 'j', 'q', 'x', 'r', 'f', 'h', 
  'l', 'm', 'n', 'ng', 'w', 'y'
];

const SECOES_CONSOANTES: SecaoConsoantes[] = [
  {
    titulo: "Plosivas (Não Aspiradas / Aspiradas)",
    linhas: [
      { consoante: 'b', ipa: '[p]', dica: 'Como o <strong>P</strong> em <em>pato</em> (sem sopro de ar)', exemploHanzi: '八', exemploPinyin: 'bā' },
      { consoante: 'p', ipa: '[pʰ]', dica: '<strong>P</strong> aspirado (com forte sopro de ar)', exemploHanzi: '趴', exemploPinyin: 'pā' },
      { consoante: 'd', ipa: '[t]', dica: 'Como o <strong>T</strong> em <em>tatu</em> (sem sopro de ar)', exemploHanzi: '搭', exemploPinyin: 'dā' },
      { consoante: 't', ipa: '[tʰ]', dica: '<strong>T</strong> aspirado (com forte sopro de ar)', exemploHanzi: '他', exemploPinyin: 'tā' },
      { consoante: 'g', ipa: '[k]', dica: 'Como o <strong>C</strong> em <em>casa</em> (sem sopro de ar)', exemploHanzi: '嘎', exemploPinyin: 'gā' },
      { consoante: 'k', ipa: '[kʰ]', dica: '<strong>K / C</strong> aspirado (com forte sopro de ar)', exemploHanzi: '咖', exemploPinyin: 'kā' },
    ]
  },
  {
    titulo: "Sibilantes e Fricativas (Dentes Fechados)",
    linhas: [
      { consoante: 'z', ipa: '[ts]', dica: 'Som de <strong>DS / TS</strong> seco (como em <em>pizza</em>)', exemploHanzi: '咂', exemploPinyin: 'zā' },
      { consoante: 'c', ipa: '[tsʰ]', dica: 'Som de <strong>TS</strong> explosivo (soprando bastante ar)', exemploHanzi: '擦', exemploPinyin: 'cā' },
      { consoante: 's', ipa: '[s]', dica: 'Como o <strong>S</strong> em português (som de <em>sapo</em>)', exemploHanzi: '撒', exemploPinyin: 'sā' },
    ]
  },
  {
    titulo: "Alveolopalatais (Língua Baixa no Palato)",
    linhas: [
      { consoante: 'j', ipa: '[tɕ]', dica: 'Som de <strong>DI</strong> seco (língua colada atrás dos dentes inferiores)', exemploHanzi: '家', exemploPinyin: 'jiā' },
      { consoante: 'q', ipa: '[tɕʰ]', dica: 'Som de <strong>TCH</strong> aspirado (língua colada atrás dos dentes inferiores)', exemploHanzi: '掐', exemploPinyin: 'qiā' },
      { consoante: 'x', ipa: '[ɕ]', dica: 'Som de <strong>S</strong> chiado (língua colada atrás dos dentes inferiores)', exemploHanzi: '虾', exemploPinyin: 'xiā' },
    ]
  },
  {
    titulo: "Consoantes Retroflexas (Língua Dobrada)",
    linhas: [
      { consoante: 'zh', ipa: '[tʂ]', dica: 'Som de <strong>DJ</strong> forte (ponta da língua dobrada para o céu da boca)', exemploHanzi: '扎', exemploPinyin: 'zhā' },
      { consoante: 'ch', ipa: '[tʂʰ]', dica: 'Som de <strong>TCH</strong> aspirado (ponta da língua dobrada para o céu da boca)', exemploHanzi: '叉', exemploPinyin: 'chā' },
      { consoante: 'sh', ipa: '[ʂ]', dica: 'Som de <strong>CH / SH</strong> (ponta da língua dobrada para o céu da boca)', exemploHanzi: '沙', exemploPinyin: 'shā' },
      { consoante: 'r', ipa: '[ɻ / ʐ]', dica: 'Som retroflexo grave (ponta da língua dobrada para trás, como o <strong>R</strong> caipira)', exemploHanzi: '日', exemploPinyin: 'rì' },
    ]
  },
  {
    titulo: "Outras Consoantes",
    linhas: [
      { consoante: 'f', ipa: '[f]', dica: 'Como o <strong>F</strong> em português (som de <em>faca</em>)', exemploHanzi: '发', exemploPinyin: 'fā' },
      { consoante: 'h', ipa: '[x]', dica: 'Som gutural de <strong>R</strong> forte (como o <strong>RR</strong> em <em>carro</em>)', exemploHanzi: '哈', exemploPinyin: 'hā' },
      { consoante: 'l', ipa: '[l]', dica: 'Como o <strong>L</strong> em português (som de <em>lata</em>)', exemploHanzi: '拉', exemploPinyin: 'lā' },
      { consoante: 'm', ipa: '[m]', dica: 'Como o <strong>M</strong> em português (som de <em>mesa</em>)', exemploHanzi: '妈', exemploPinyin: 'mā' },
      { consoante: 'n', ipa: '[n]', dica: 'Como o <strong>N</strong> em português (som de <em>nada</em>)', exemploHanzi: '拿', exemploPinyin: 'ná' },
      { consoante: 'ng', ipa: '[ŋ]', dica: 'Som nasal forte ao final das sílabas (como em <em>canta</em> ou no inglês <em>sing</em>)', exemploHanzi: '肮', exemploPinyin: 'āng' },
      { consoante: 'w', ipa: '[w]', dica: 'Semivogal, funciona no início das sílabas como o som de <strong>U</strong>', exemploHanzi: '蛙', exemploPinyin: 'wā' },
      { consoante: 'y', ipa: '[j]', dica: 'Semivogal, funciona no início das sílabas como o som de <strong>I</strong>', exemploHanzi: '鸭', exemploPinyin: 'yā' },
    ]
  }
];

const SECOES_VOGAIS: SecaoVogais[] = [
  {
    titulo: "Vogais Simples (Monotongos)",
    linhas: [
      { vogal: 'a', ipa: '[a]', dica: 'Som de <strong>A</strong> aberto (como em <em>pai</em>)', exemploHanzi: '阿', exemploPinyin: 'ā' },
      { vogal: 'o', ipa: '[o]', dica: 'Som de <strong>Ó / O</strong> (como em <em>pó</em> ou <em>avó</em>)', exemploHanzi: '噢', exemploPinyin: 'ō' },
      { vogal: 'e', ipa: '[ɤ]', dica: 'Som gutural neutro entre <strong>Ê</strong> e <strong>Â</strong> (como o <em>e</em> em <em>mesa</em>)', exemploHanzi: '鹅', exemploPinyin: 'é' },
      { vogal: 'i', ipa: '[i]', dica: 'Som de <strong>I</strong> (como em <em>fita</em>); após <em>zh, ch, sh, r, z, c, s</em> soa como zumbido prolongado', exemploHanzi: '衣', exemploPinyin: 'yī' },
      { vogal: 'u', ipa: '[u]', dica: 'Som de <strong>U</strong> (como em <em>uva</em>)', exemploHanzi: '乌', exemploPinyin: 'wū' },
      { vogal: 'ü', ipa: '[y]', dica: 'Não existe no português. Posicione a boca para falar <strong>U</strong> e emita <strong>I</strong>', exemploHanzi: '迂', exemploPinyin: 'yū' },
      { vogal: 'er', ipa: '[aɚ]', dica: 'Som de <strong>ER</strong> retroflexo (língua dobrada para trás, como em <em>porta</em> no sotaque caipira)', exemploHanzi: '二', exemploPinyin: 'èr' },
    ]
  },
  {
    titulo: "Vogais Compostas (Ditongos e Tritongos)",
    linhas: [
      { vogal: 'ai', ipa: '[aɪ]', dica: 'Som de <strong>AI</strong> (como na palavra <em>pai</em>)', exemploHanzi: '爱', exemploPinyin: 'ài' },
      { vogal: 'ei', ipa: '[eɪ]', dica: 'Som de <strong>EI</strong> (como na palavra <em>lei</em>)', exemploHanzi: '诶', exemploPinyin: 'éi' },
      { vogal: 'ao', ipa: '[ɑʊ]', dica: 'Som de <strong>AU / AO</strong> (como em <em>pau</em>)', exemploHanzi: '翱', exemploPinyin: 'áo' },
      { vogal: 'ou', ipa: '[oʊ]', dica: 'Som de <strong>OU</strong> (como em <em>ouvir</em>)', exemploHanzi: '欧', exemploPinyin: 'ōu' },
      { vogal: 'ia', ipa: '[i̯a]', dica: 'Som de <strong>IA</strong> (como em <em>dia</em>)', exemploHanzi: '鸭', exemploPinyin: 'yā' },
      { vogal: 'ie', ipa: '[i̯ɛ]', dica: 'Som de <strong>IÉ</strong> (como em <em>café</em>)', exemploHanzi: '椰', exemploPinyin: 'yē' },
      { vogal: 'ua', ipa: '[u̯a]', dica: 'Som de <strong>UA</strong> (como em <em>quatro</em>)', exemploHanzi: '蛙', exemploPinyin: 'wā' },
      { vogal: 'uo', ipa: '[u̯o]', dica: 'Som de <strong>UÓ</strong> (como em <em>suor</em>)', exemploHanzi: '窝', exemploPinyin: 'wō' },
      { vogal: 'üe', ipa: '[y̯ɛ]', dica: 'Som de <strong>Ü-É</strong> (inicia em <em>ü</em> e desliza para <em>é</em>)', exemploHanzi: '约', exemploPinyin: 'yuē' },
      { vogal: 'iao', ipa: '[i̯ɑʊ]', dica: 'Som de <strong>IAU</strong> (como em <em>miou</em>)', exemploHanzi: '腰', exemploPinyin: 'yāo' },
      { vogal: 'iu', ipa: '[i̯oʊ]', dica: 'Som de <strong>IOU / IU</strong> (como em <em>tuiu</em>)', exemploHanzi: '优', exemploPinyin: 'yōu' },
      { vogal: 'uai', ipa: '[u̯aɪ]', dica: 'Som de <strong>UAI</strong> (como em <em>paraguai</em>)', exemploHanzi: '歪', exemploPinyin: 'wāi' },
      { vogal: 'ui', ipa: '[u̯eɪ]', dica: 'Som de <strong>UEI / UI</strong> (como em <em>paguei</em>)', exemploHanzi: '威', exemploPinyin: 'wēi' },
    ]
  },
  {
    titulo: "Finais Nasais",
    linhas: [
      { vogal: 'an', ipa: '[an]', dica: 'Som de <strong>AN</strong> frontal (como em <em>chaminé</em>)', exemploHanzi: '安', exemploPinyin: 'ān' },
      { vogal: 'en', ipa: '[ən]', dica: 'Som de <strong>EN / ÂN</strong> neutro (como em <em>vento</em>)', exemploHanzi: '恩', exemploPinyin: 'ēn' },
      { vogal: 'in', ipa: '[in]', dica: 'Som de <strong>IN</strong> (como em <em>tinta</em>)', exemploHanzi: '因', exemploPinyin: 'yīn' },
      { vogal: 'un', ipa: '[wən]', dica: 'Som de <strong>UÊN</strong> (como em <em>frequente</em>)', exemploHanzi: '温', exemploPinyin: 'wēn' },
      { vogal: 'ün', ipa: '[yn]', dica: 'Som de <strong>ÜN</strong> (boca de <em>U</em> pronunciando <em>in</em>)', exemploHanzi: '晕', exemploPinyin: 'yūn' },
      { vogal: 'ang', ipa: '[ɑŋ]', dica: 'Som de <strong>ÂNG</strong> nasal gutural (como em <em>manga</em>)', exemploHanzi: '昂', exemploPinyin: 'áng' },
      { vogal: 'eng', ipa: '[ɤŋ]', dica: 'Som de <strong>ÊNG</strong> nasal gutural', exemploHanzi: '鞥', exemploPinyin: 'ēng' },
      { vogal: 'ing', ipa: '[iŋ]', dica: 'Som de <strong>ING</strong> nasal (como no inglês <em>sing</em>)', exemploHanzi: '英', exemploPinyin: 'yīng' },
      { vogal: 'ong', ipa: '[ʊŋ]', dica: 'Som de <strong>ÔNG</strong> nasal redondo (como em <em>pongo</em>)', exemploHanzi: '轰', exemploPinyin: 'hōng' },
      { vogal: 'iong', ipa: '[i̯ʊŋ]', dica: 'Som de <strong>IÔNG</strong> (como em <em>mion</em>)', exemploHanzi: '雍', exemploPinyin: 'yōng' },
    ]
  }
];

const LISTA_INICIAIS_PINYIN = [
  'zh', 'ch', 'sh',
  'b', 'p', 'd', 't', 'g', 'k', 'z', 'c', 's', 'j', 'q', 'x', 'r', 'f', 'h', 'l', 'm', 'n', 'w', 'y'
];

function extrairConsoantesDePinyin(pinyin: string): Set<string> {
  const resultado = new Set<string>();
  if (!pinyin) return resultado;

  const pinyinLimpo = pinyin
    .toLowerCase()
    .normalize('NFD')
    .replace(/[\u0300-\u036f]/g, '')
    .replace(/[^a-z\s]/g, '');

  const silabas = pinyinLimpo.split(/\s+/);

  for (const silaba of silabas) {
    if (!silaba) continue;

    for (const inicial of LISTA_INICIAIS_PINYIN) {
      if (silaba.startsWith(inicial)) {
        resultado.add(inicial);
        break;
      }
    }

    if (silaba.endsWith('ng')) {
      resultado.add('ng');
    }
  }

  return resultado;
}

function extrairInicialDePinyin(pinyin: string): string {
  if (!pinyin) return '';
  const pinyinLimpo = pinyin
    .toLowerCase()
    .normalize('NFD')
    .replace(/[\u0300-\u036f]/g, '')
    .replace(/[^a-z\s]/g, '');

  const silaba = pinyinLimpo.split(/\s+/)[0] || '';
  for (const inicial of LISTA_INICIAIS_PINYIN) {
    if (silaba.startsWith(inicial)) {
      return inicial;
    }
  }
  return '';
}

function extrairVogaisDePinyin(pinyin: string): Set<string> {
  const resultado = new Set<string>();
  if (!pinyin) return resultado;

  const pinyinLimpo = pinyin
    .toLowerCase()
    .normalize('NFD')
    .replace(/[\u0300-\u036f]/g, '')
    .replace(/[^a-z\s]/g, '');

  const silabas = pinyinLimpo.split(/\s+/);
  for (const silaba of silabas) {
    if (!silaba) continue;

    for (const secao of SECOES_VOGAIS) {
      for (const linha of secao.linhas) {
        if (silaba.includes(linha.vogal)) {
          resultado.add(linha.vogal);
        }
      }
    }
  }

  return resultado;
}

function encontrarInfoConsoante(consoante: string): LinhaConsoante | null {
  for (const secao of SECOES_CONSOANTES) {
    for (const linha of secao.linhas) {
      if (linha.consoante === consoante) {
        return linha;
      }
    }
  }
  return null;
}

// ----- Componente PronunciaTipo (Atividade por tipo de pronúncia/consoante) -----

function PronunciaTipo(props: PronunciaProps) {
  const { questao, aoTocarAudio } = props;

  const pinyinAlvo = useMemo(() => {
    return questao.fraseOriginalSegmentada?.[0]?.pinyin || questao.pinyin || '';
  }, [questao]);

  const consoanteAlvo = useMemo(() => {
    return extrairInicialDePinyin(pinyinAlvo);
  }, [pinyinAlvo]);

  const infoConsoante = useMemo(() => {
    return encontrarInfoConsoante(consoanteAlvo);
  }, [consoanteAlvo]);

  return (
    <div className="revisao-pronuncia-tipo-wrapper" style={{ width: '100%' }}>
      {infoConsoante ? (
        <div className="revisao-pronuncia-consoante-header">
          <div className="consoante-linha-topo">
            <span className="rotulo-tipo">{t('Atividade de Pronúncia — Consoante')}</span>
            <span className="consoante-badge">{infoConsoante.consoante}</span>
            <span className="consoante-ipa">{infoConsoante.ipa}</span>
            <button 
              className="revisao-dica-play-btn"
              onClick={() => aoTocarAudio(infoConsoante.exemploHanzi)}
              title={t('Ouvir pronúncia da consoante')}
              style={{ marginLeft: 'auto' }}
            >
              <svg viewBox="0 0 24 24" width="13" height="13" fill="none" stroke="currentColor" strokeWidth="2.5" strokeLinecap="round" strokeLinejoin="round">
                <polygon points="11 5 6 9 2 9 2 15 6 15 11 19 11 5" />
                <path d="M19.07 4.93a10 10 0 0 1 0 14.14M15.54 8.46a5 5 0 0 1 0 7.07" />
              </svg>
              <span>{infoConsoante.exemploPinyin}</span>
            </button>
          </div>
          <div className="consoante-dica-desc" dangerouslySetInnerHTML={{ __html: t(infoConsoante.dica) }} />
        </div>
      ) : null}

      <PronunciaBaralho {...props} modoDicaInicial="vogais" />
    </div>
  );
}

function BotaoDicaPinyin({ 
  pinyinFaltando, 
  aoTocarAudio,
  modoInicial = 'consoantes' 
}: { 
  pinyinFaltando: string; 
  aoTocarAudio: (texto: string) => void;
  modoInicial?: 'consoantes' | 'vogais';
}) {
  const [painelAberto, setPainelAberto] = useState(false);
  const [hoverAtivo, setHoverAtivo] = useState(false);

  const aoAlternarPainel = () => {
    setPainelAberto((prev) => !prev);
  };

  const consoantesFaltando = useMemo(() => {
    return extrairConsoantesDePinyin(pinyinFaltando);
  }, [pinyinFaltando]);

  const vogaisFaltando = useMemo(() => {
    return extrairVogaisDePinyin(pinyinFaltando);
  }, [pinyinFaltando]);

  const linhasConsoantesFaltantes = useMemo(() => {
    const resultado: LinhaConsoante[] = [];
    if (consoantesFaltando.size === 0) return resultado;
    for (const secao of SECOES_CONSOANTES) {
      for (const linha of secao.linhas) {
        if (consoantesFaltando.has(linha.consoante)) {
          resultado.push(linha);
        }
      }
    }
    return resultado;
  }, [consoantesFaltando]);

  const linhasVogaisFaltantes = useMemo(() => {
    const resultado: LinhaVogal[] = [];
    if (vogaisFaltando.size === 0) return resultado;
    for (const secao of SECOES_VOGAIS) {
      for (const linha of secao.linhas) {
        if (vogaisFaltando.has(linha.vogal)) {
          resultado.push(linha);
        }
      }
    }
    return resultado;
  }, [vogaisFaltando]);

  const popupItems = useMemo(() => {
    if (modoInicial === 'vogais') {
      let vgs = [...linhasVogaisFaltantes];
      if (vgs.length === 0 && pinyinFaltando) {
        const pinyinLimpo = pinyinFaltando.toLowerCase().normalize('NFD').replace(/[\u0300-\u036f]/g, '').replace(/[^a-z]/g, '');
        for (const secao of SECOES_VOGAIS) {
          for (const linha of secao.linhas) {
            if (pinyinLimpo.includes(linha.vogal) && !vgs.some(item => item.vogal === linha.vogal)) {
              vgs.push(linha);
            }
          }
        }
      }
      return vgs.map(l => ({ simbolo: l.vogal, ipa: l.ipa, dica: l.dica }));
    }
    return linhasConsoantesFaltantes.map(l => ({ simbolo: l.consoante, ipa: l.ipa, dica: l.dica }));
  }, [modoInicial, linhasVogaisFaltantes, linhasConsoantesFaltantes, pinyinFaltando]);

  return (
    <div className="revisao-dica-botao-container">
      <button 
        className="revisao-dica-botao"
        onClick={aoAlternarPainel}
        onMouseEnter={() => setHoverAtivo(true)}
        onMouseLeave={() => setHoverAtivo(false)}
        title={t("Dicas de Pronúncia do Pinyin")}
      >
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
          <circle cx="12" cy="12" r="10" />
          <path d="M12 16v-4" />
          <path d="M12 8h.01" />
        </svg>
      </button>

      {hoverAtivo && popupItems.length > 0 && !painelAberto && (
        <div className="revisao-dica-popover">
          <div className="revisao-dica-popover-titulo">
            {modoInicial === 'vogais' ? t('Dicas de Vogais') : t('Consoantes Pendentes')}
          </div>
          <div className="revisao-dica-popover-lista">
            {popupItems.map(item => (
              <div key={item.simbolo} className="revisao-dica-popover-item">
                <span className="consoante">{item.simbolo}</span>
                <span className="ipa">{item.ipa}</span>
                <span className="dica">{t(item.dica).replace(/<\/?[^>]+(>|$)/g, "")}</span>
              </div>
            ))}
          </div>
        </div>
      )}

      {painelAberto && (
        <PainelDicaPinyin 
          aoFechar={aoAlternarPainel} 
          aoTocarAudio={aoTocarAudio} 
          pinyinFaltando={pinyinFaltando} 
          modoInicial={modoInicial}
        />
      )}
    </div>
  );
}

function PainelDicaPinyin({ 
  aoFechar, 
  aoTocarAudio, 
  pinyinFaltando,
  modoInicial = 'consoantes'
}: { 
  aoFechar: () => void; 
  aoTocarAudio: (texto: string) => void; 
  pinyinFaltando: string;
  modoInicial?: 'consoantes' | 'vogais';
}) {
  const [posicao, setPosicao] = useState({ x: 0, y: 0 });
  const [abaAtiva, setAbaAtiva] = useState<'consoantes' | 'vogais'>(modoInicial);
  const [mostrarTudo, setMostrarTudo] = useState(false);
  const arrastando = useRef(false);
  const offsetRef = useRef({ x: 0, y: 0 });

  const consoantesFaltando = useMemo(() => extrairConsoantesDePinyin(pinyinFaltando), [pinyinFaltando]);
  const vogaisFaltando = useMemo(() => extrairVogaisDePinyin(pinyinFaltando), [pinyinFaltando]);

  const secoesExibidasConsoantes = useMemo(() => {
    if (mostrarTudo || consoantesFaltando.size === 0) {
      return SECOES_CONSOANTES;
    }
    return SECOES_CONSOANTES.map(secao => ({
      ...secao,
      linhas: secao.linhas.filter(l => consoantesFaltando.has(l.consoante))
    })).filter(secao => secao.linhas.length > 0);
  }, [mostrarTudo, consoantesFaltando]);

  const secoesExibidasVogais = useMemo(() => {
    if (mostrarTudo || vogaisFaltando.size === 0) {
      return SECOES_VOGAIS;
    }
    return SECOES_VOGAIS.map(secao => ({
      ...secao,
      linhas: secao.linhas.filter(l => vogaisFaltando.has(l.vogal))
    })).filter(secao => secao.linhas.length > 0);
  }, [mostrarTudo, vogaisFaltando]);

  useEffect(() => {
    const larguraPainel = Math.min(380, window.innerWidth * 0.95);
    const padraoX = window.innerWidth - larguraPainel - 24;
    const padraoY = window.innerHeight - 560;
    setPosicao({ 
      x: Math.max(10, padraoX), 
      y: Math.max(10, padraoY) 
    });
  }, []);

  const aoIniciarArrasto = (e: React.MouseEvent) => {
    arrastando.current = true;
    offsetRef.current = {
      x: e.clientX - posicao.x,
      y: e.clientY - posicao.y
    };
    
    document.addEventListener('mousemove', aoArrastar);
    document.addEventListener('mouseup', aoPararArrasto);
  };

  const aoArrastar = (e: MouseEvent) => {
    if (!arrastando.current) return;
    
    const novoX = Math.max(10, Math.min(window.innerWidth - 100, e.clientX - offsetRef.current.x));
    const novoY = Math.max(10, Math.min(window.innerHeight - 80, e.clientY - offsetRef.current.y));
    
    setPosicao({ x: novoX, y: novoY });
  };

  const aoPararArrasto = () => {
    arrastando.current = false;
    document.removeEventListener('mousemove', aoArrastar);
    document.removeEventListener('mouseup', aoPararArrasto);
  };

  useEffect(() => {
    return () => {
      document.removeEventListener('mousemove', aoArrastar);
      document.removeEventListener('mouseup', aoPararArrasto);
    };
  }, []);

  return (
    <div 
      className="revisao-dica-painel"
      style={{ left: `${posicao.x}px`, top: `${posicao.y}px` }}
    >
      <div className="revisao-dica-header" onMouseDown={aoIniciarArrasto}>
        <div className="revisao-dica-drag-handle" title="Arraste para mover">
          <svg viewBox="0 0 24 24" width="16" height="16" fill="currentColor">
            <circle cx="9" cy="5" r="1.5" />
            <circle cx="9" cy="12" r="1.5" />
            <circle cx="9" cy="19" r="1.5" />
            <circle cx="15" cy="5" r="1.5" />
            <circle cx="15" cy="12" r="1.5" />
            <circle cx="15" cy="19" r="1.5" />
          </svg>
        </div>
        <div className="revisao-dica-titulo">
          {abaAtiva === 'consoantes' ? t('Consoantes do Pinyin') : t('Vogais do Pinyin')}
        </div>
        <div style={{ marginLeft: 'auto', display: 'flex', alignItems: 'center', gap: '6px' }}>
          <button 
            className="revisao-dica-expand" 
            onClick={() => setMostrarTudo(prev => !prev)} 
            title={mostrarTudo ? t("Mostrar apenas letras do termo") : t("Mostrar guia completo")}
          >
            {mostrarTudo ? (
              <svg viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="currentColor" strokeWidth="2.5" strokeLinecap="round" strokeLinejoin="round">
                <polyline points="4 14 10 14 10 20" />
                <polyline points="20 10 14 10 14 4" />
                <line x1="14" y1="10" x2="21" y2="3" />
                <line x1="3" y1="21" x2="10" y2="14" />
              </svg>
            ) : (
              <svg viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="currentColor" strokeWidth="2.5" strokeLinecap="round" strokeLinejoin="round">
                <polyline points="15 3 21 3 21 9" />
                <polyline points="9 21 3 21 3 15" />
                <line x1="21" y1="3" x2="14" y2="10" />
                <line x1="3" y1="21" x2="10" y2="14" />
              </svg>
            )}
          </button>
          <button className="revisao-dica-close" onClick={aoFechar} title={t("Fechar Guia")}>
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5" strokeLinecap="round" strokeLinejoin="round">
              <line x1="18" y1="6" x2="6" y2="18" />
              <line x1="6" y1="6" x2="18" y2="18" />
            </svg>
          </button>
        </div>
      </div>

      <div className="revisao-dica-tabs">
        <button 
          className={`revisao-dica-tab-btn ${abaAtiva === 'consoantes' ? 'ativo' : ''}`}
          onClick={() => setAbaAtiva('consoantes')}
        >
          {t('Consoantes')}
        </button>
        <button 
          className={`revisao-dica-tab-btn ${abaAtiva === 'vogais' ? 'ativo' : ''}`}
          onClick={() => setAbaAtiva('vogais')}
        >
          {t('Vogais')}
        </button>
      </div>

      <div className="revisao-dica-corpo">
        {abaAtiva === 'consoantes' ? (
          secoesExibidasConsoantes.length === 0 ? (
            <div style={{ textAlign: 'center', padding: '24px 0', color: 'var(--cor-texto-suave)' }}>
              {t('Nenhuma consoante encontrada.')}
            </div>
          ) : (
            secoesExibidasConsoantes.map(secao => (
              <div key={secao.titulo}>
                <div className="revisao-dica-secao-titulo">{t(secao.titulo)}</div>
                <table className="revisao-dica-tabela">
                  <thead>
                    <tr>
                      <th style={{ width: '35%' }}>{t('Pinyin')}</th>
                      <th style={{ width: '25%' }}>{t('IPA')}</th>
                      <th style={{ width: '40%' }}>{t('Ouvir')}</th>
                    </tr>
                  </thead>
                  <tbody>
                    {secao.linhas.map(linha => {
                      const destacar = mostrarTudo && consoantesFaltando.has(linha.consoante);
                      return (
                        <tr key={linha.consoante} className={destacar ? 'highlight' : ''}>
                          <td className="revisao-dica-pinyin-cell">
                            <span className="pinyin-consoante">{linha.consoante}</span>
                            <span className="revisao-dica-tooltip">
                              <span dangerouslySetInnerHTML={{ __html: t(linha.dica) }} />
                            </span>
                          </td>
                          <td className="revisao-dica-ipa-cell">{linha.ipa}</td>
                          <td>
                            <button className="revisao-dica-play-btn" onClick={() => aoTocarAudio(linha.exemploHanzi)}>
                              <svg viewBox="0 0 24 24" width="13" height="13" fill="none" stroke="currentColor" strokeWidth="2.5" strokeLinecap="round" strokeLinejoin="round">
                                <polygon points="11 5 6 9 2 9 2 15 6 15 11 19 11 5" />
                                <path d="M19.07 4.93a10 10 0 0 1 0 14.14M15.54 8.46a5 5 0 0 1 0 7.07" />
                              </svg>
                              <span>{linha.exemploPinyin}</span>
                            </button>
                          </td>
                        </tr>
                      );
                    })}
                  </tbody>
                </table>
              </div>
            ))
          )
        ) : (
          secoesExibidasVogais.length === 0 ? (
            <div style={{ textAlign: 'center', padding: '24px 0', color: 'var(--cor-texto-suave)' }}>
              {t('Nenhuma vogal encontrada.')}
            </div>
          ) : (
            secoesExibidasVogais.map(secao => (
              <div key={secao.titulo}>
                <div className="revisao-dica-secao-titulo">{t(secao.titulo)}</div>
                <table className="revisao-dica-tabela">
                  <thead>
                    <tr>
                      <th style={{ width: '35%' }}>{t('Vogal')}</th>
                      <th style={{ width: '25%' }}>{t('IPA')}</th>
                      <th style={{ width: '40%' }}>{t('Ouvir')}</th>
                    </tr>
                  </thead>
                  <tbody>
                    {secao.linhas.map(linha => {
                      const destacar = mostrarTudo && vogaisFaltando.has(linha.vogal);
                      return (
                        <tr key={linha.vogal} className={destacar ? 'highlight' : ''}>
                          <td className="revisao-dica-pinyin-cell">
                            <span className="pinyin-consoante">{linha.vogal}</span>
                            <span className="revisao-dica-tooltip">
                              <span dangerouslySetInnerHTML={{ __html: t(linha.dica) }} />
                            </span>
                          </td>
                          <td className="revisao-dica-ipa-cell">{linha.ipa}</td>
                          <td>
                            <button className="revisao-dica-play-btn" onClick={() => aoTocarAudio(linha.exemploHanzi)}>
                              <svg viewBox="0 0 24 24" width="13" height="13" fill="none" stroke="currentColor" strokeWidth="2.5" strokeLinecap="round" strokeLinejoin="round">
                                <polygon points="11 5 6 9 2 9 2 15 6 15 11 19 11 5" />
                                <path d="M19.07 4.93a10 10 0 0 1 0 14.14M15.54 8.46a5 5 0 0 1 0 7.07" />
                              </svg>
                              <span>{linha.exemploPinyin}</span>
                            </button>
                          </td>
                        </tr>
                      );
                    })}
                  </tbody>
                </table>
              </div>
            ))
          )
        )}
      </div>
    </div>
  );
}
