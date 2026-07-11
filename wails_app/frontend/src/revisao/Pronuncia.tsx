import { useState, useEffect, useMemo, useRef } from 'react';
import { main } from '../../wailsjs/go/models';
import { useSTT } from '../comum/useSTT';
import { PopupRevisao } from './PopupRevisao';
import { DecomporTextoRevisao } from '../../wailsjs/go/main/App';
import { pronunciaCasa, TokenFalado } from './comparacaoPronuncia';
import './revisao.css';

interface PronunciaProps {
  questao: main.QuestaoRevisao;
  respondida: boolean;
  aoConcluir: (acertou: boolean) => void;
  aoTocarAudio: (texto: string) => void;
  hanziTocando: string | null;
  hanziSintetizando: string | null;
  AoClicarNoCartao?: (card: any) => void;
}

export function Pronuncia(props: PronunciaProps) {
  if (props.questao.variante === 'pronuncia_frase') {
    return <PronunciaFrase {...props} />;
  }
  if (props.questao.variante === 'pronuncia_baralho') {
    return <PronunciaBaralho {...props} />;
  }
  return <PronunciaSequencia {...props} />;
}

// Mensagem exibida quando NENHUM caminho de STT existe (sem Web Speech e sem motor instalado).
// `motivo` orienta o próximo passo (baixar o motor em Configurações → Motores).
function PronunciaIndisponivel({ motivo }: { motivo: string | null }) {
  return (
    <div className="revisao-pronuncia-erro">
      ⚠️ Reconhecimento de voz indisponível.
      {motivo && <div style={{ marginTop: '6px', fontSize: '0.9em' }}>{motivo}</div>}
    </div>
  );
}

function PronunciaFrase({ questao, respondida, aoConcluir, aoTocarAudio, AoClicarNoCartao }: PronunciaProps) {
  const [acertoPreviamenteDetectado, setAcertoPreviamenteDetectado] = useState(false);
  const [transcricaoEmPinyin, setTranscricaoEmPinyin] = useState('');
  const [transcricaoEmHanzi, setTranscricaoEmHanzi] = useState('');
  const [tokensFalados, setTokensFalados] = useState<TokenFalado[]>([]);
  const [indicesAcertados, setIndicesAcertados] = useState<Set<number>>(new Set());
  const [tentativasRestantes, setTentativasRestantes] = useState(3);
  const tamanhoAcertosNoInicio = useRef(0);
  const gravouAlgumaVez = useRef(false);

  const {
    suportado, motivoIndisponivel, escutando, processando,
    transcricaoParcial, transcricaoFinal, estadoMotor, iniciar, parar, limpar, erro,
  } = useSTT({ idioma: 'zh-CN', continuo: false });

  const transcricaoAtual = transcricaoFinal + (transcricaoParcial ? ' ' + transcricaoParcial : '');

  const tokens = useMemo(() => {
    return questao.fraseOriginalSegmentada || [];
  }, [questao.fraseOriginalSegmentada]);

  const pinyinAlvo = useMemo(() => {
    return tokens
      .map(t => t.ehChines ? t.pinyin : t.texto)
      .join(' ');
  }, [tokens]);

  // Track if recording has actually started during this session
  useEffect(() => {
    if (escutando) {
      gravouAlgumaVez.current = true;
    }
  }, [escutando]);

  // Resolves the pinyin and hanzi representation of spoken text
  useEffect(() => {
    if (!transcricaoAtual) {
      setTranscricaoEmPinyin('');
      setTranscricaoEmHanzi('');
      setTokensFalados([]);
      return;
    }

    DecomporTextoRevisao(transcricaoAtual)
      .then(tokensSpoken => {
        if (!tokensSpoken) return;
        const pinyins = tokensSpoken.map(t => t.ehChines ? t.pinyin : t.texto).join(' ');
        const hanzis = tokensSpoken.map(t => t.texto).join('');
        setTranscricaoEmPinyin(pinyins);
        setTranscricaoEmHanzi(hanzis);
        setTokensFalados(tokensSpoken);
      })
      .catch(() => {});
  }, [transcricaoAtual]);

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
        const todosChinesesAcertados = tokens.every((t, i) => !t.ehChines || novosAcertos.has(i));
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
      const todosChinesesAcertados = tokens.every((t, i) => !t.ehChines || acertosAtuais.has(i));
      if (todosChinesesAcertados) {
        aoConcluir(true);
        return;
      }

      // Check if progress was made
      if (acertosAtuais.size > tamanhoAcertosNoInicio.current) {
        // Progress made, but not completed yet. Keep recording button active, do not lose a life
        return;
      }

      // No progress made: lose a life
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
  }, [escutando, transcricaoFinal, acertoPreviamenteDetectado, respondida, tokens, aoConcluir, tentativasRestantes, indicesAcertados]);

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

  const feedback = transcricaoAtual
    || (escutando ? 'Ouvindo...' : '')
    || estadoMotor
    || (processando ? 'Transcrevendo...' : '...');

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
            title="Tocar áudio da frase"
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
            {tokens.map((t, idx) => {
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
                      style={{ 
                        fontSize: '14px', 
                        color: 'var(--cor-pinyin, #a0aec0)', 
                        fontWeight: 'normal',
                        marginBottom: '2px',
                        userSelect: 'none'
                      }}
                    >
                      {t.pinyin}
                    </span>
                    <span
                      style={{
                        cursor: 'pointer',
                        transition: 'color 0.2s',
                        color: ehAcertado ? 'var(--cor-sucesso)' : 'inherit',
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
        <button
          className={`revisao-btn-microfone ${escutando ? 'escutando' : ''}`}
          onClick={aoClicarMicrofone}
          disabled={respondida || processando}
        >
          <svg width="32" height="32" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
            <path d="M12 2a3 3 0 0 0-3 3v7a3 3 0 0 0 6 0V5a3 3 0 0 0-3-3Z"></path>
            <path d="M19 10v2a7 7 0 0 1-14 0v-2"></path>
            <line x1="12" y1="19" x2="12" y2="22"></line>
          </svg>
        </button>

        {!respondida && (
          <div className="revisao-cartao-tentativas" title="Tentativas restantes">
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

      <div className="revisao-pronuncia-feedback" style={{ marginTop: '16px' }}>
        {transcricaoEmPinyin ? (
          <div style={{ display: 'flex', flexDirection: 'column', alignItems: 'center', gap: '4px' }}>
            <div style={{ fontSize: '18px', color: 'var(--cor-pinyin)' }}>{transcricaoEmPinyin}</div>
            <div style={{ fontSize: '24px', fontFamily: 'var(--fonte-hanzi)', color: 'var(--cor-texto-primario)' }}>{transcricaoEmHanzi}</div>
          </div>
        ) : (
          feedback
        )}
      </div>

      {erro && <div className="revisao-pronuncia-erro">Erro: {erro}</div>}
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
    questao.fraseOriginalSegmentada?.forEach((p, idx) => {
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
  const [acertoPreviamenteDetectado, setAcertoPreviamenteDetectado] = useState(false);
  const [transcricaoEmPinyin, setTranscricaoEmPinyin] = useState('');
  const [transcricaoEmHanzi, setTranscricaoEmHanzi] = useState('');
  const [tokensFalados, setTokensFalados] = useState<TokenFalado[]>([]);
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
  } = useSTT({ idioma: 'zh-CN', continuo: false });

  const transcricaoAtual = transcricaoFinal + (transcricaoParcial ? ' ' + transcricaoParcial : '');

  // Resolves the pinyin and hanzi representation of spoken text
  useEffect(() => {
    if (!transcricaoAtual) {
      setTranscricaoEmPinyin('');
      setTranscricaoEmHanzi('');
      setTokensFalados([]);
      return;
    }

    DecomporTextoRevisao(transcricaoAtual)
      .then(tokens => {
        if (!tokens) return;
        const pinyins = tokens.map(t => t.ehChines ? t.pinyin : t.texto).join(' ');
        const hanzis = tokens.map(t => t.texto).join('');
        setTranscricaoEmPinyin(pinyins);
        setTranscricaoEmHanzi(hanzis);
        setTokensFalados(tokens);
      })
      .catch(() => {});
  }, [transcricaoAtual]);

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
    }, 2000);
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
    }, 1000);
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
  }, [escutando, transcricaoFinal, transcricaoParcial, acertoPreviamenteDetectado, respondida, fila, limpar, acertoAtual, erroAtual]);

  // Auto-start microphone if previous card was correctly pronounced
  useEffect(() => {
    if (fila.length > 0 && proximoAutoMicrofone.current) {
      proximoAutoMicrofone.current = false;
      setAcertoPreviamenteDetectado(false);
      iniciar();
    }
  }, [fila, iniciar]);

  if (!suportado) {
    return <PronunciaIndisponivel motivo={motivoIndisponivel} />;
  }

  if (fila.length === 0) {
    return null; // Já concluiu, aba de revisão lidará com o feedback geral
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

  const feedback = transcricaoAtual
    || (escutando ? 'Ouvindo...' : '')
    || estadoMotor
    || (processando ? 'Transcrevendo...' : '...');

  return (
    <div className="revisao-pronuncia-container">
      <div className="revisao-pronuncia-instrucao">
        Fale o termo em destaque. Clique no microfone para falar.
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
                <div className="revisao-cartao-tentativas" title="Tentativas restantes">
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

      <div style={{ display: 'flex', gap: '16px', alignItems: 'center', justifyContent: 'center' }}>
        <button
          className={`revisao-btn-microfone ${escutando ? 'escutando' : ''}`}
          onClick={aoClicarMicrofone}
          disabled={respondida || processando || erroAtual !== null || acertoAtual !== null}
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
          title="Desistir do cartão (Reciclar/Pular)"
        >
          <svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
            <path d="M7 11V7a5 5 0 0 1 5-5c1.38 0 2.63.56 3.54 1.46L19 7"/>
            <path d="M2 13h4.14c.9 0 1.54.56 2.46 1.46L12 18"/>
            <path d="M22 17v-4a5 5 0 0 0-5-5c-1.38 0-2.63.56-3.54 1.46L10 13"/>
            <path d="M17 22h-4.14c-.9 0-1.54-.56-2.46-1.46L7 17"/>
          </svg>
        </button>
      </div>

      <div className="revisao-pronuncia-feedback">
        {transcricaoEmPinyin ? (
          <div style={{ display: 'flex', flexDirection: 'column', alignItems: 'center', gap: '4px' }}>
            <div style={{ fontSize: '18px', color: 'var(--cor-pinyin)' }}>{transcricaoEmPinyin}</div>
            <div style={{ fontSize: '24px', fontFamily: 'var(--fonte-hanzi)', color: 'var(--cor-texto-primario)' }}>{transcricaoEmHanzi}</div>
          </div>
        ) : (
          feedback
        )}
      </div>

      {erro && <div className="revisao-pronuncia-erro">Erro: {erro}</div>}

      <PopupRevisao info={informacoesPopup} />
    </div>
  );
}

function PronunciaBaralho({ questao, respondida, aoConcluir, aoTocarAudio, AoClicarNoCartao }: PronunciaProps) {
  const filaInicial = useMemo(() => {
    const itens: FilaItem[] = [];
    questao.fraseOriginalSegmentada?.forEach((p, idx) => {
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
  const [estadoAnimacao, setEstadoAnimacao] = useState<'normal' | 'acerto-verde' | 'acerto-deslizar' | 'erro-vermelho' | 'erro-descartar' | 'erro-reciclar'>('normal');
  const [listaAcertos, setListaAcertos] = useState<string[]>([]);
  const [acertoPreviamenteDetectado, setAcertoPreviamenteDetectado] = useState(false);
  const [transcricaoEmPinyin, setTranscricaoEmPinyin] = useState('');
  const [transcricaoEmHanzi, setTranscricaoEmHanzi] = useState('');
  const [tokensFalados, setTokensFalados] = useState<TokenFalado[]>([]);
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
  } = useSTT({ idioma: 'zh-CN', continuo: false });

  const transcricaoAtual = transcricaoFinal + (transcricaoParcial ? ' ' + transcricaoParcial : '');

  // Resolves the pinyin and hanzi representation of spoken text
  useEffect(() => {
    if (!transcricaoAtual) {
      setTranscricaoEmPinyin('');
      setTranscricaoEmHanzi('');
      setTokensFalados([]);
      return;
    }

    DecomporTextoRevisao(transcricaoAtual)
      .then(tokens => {
        if (!tokens) return;
        const pinyins = tokens.map(t => t.ehChines ? t.pinyin : t.texto).join(' ');
        const hanzis = tokens.map(t => t.texto).join('');
        setTranscricaoEmPinyin(pinyins);
        setTranscricaoEmHanzi(hanzis);
        setTokensFalados(tokens);
      })
      .catch(() => {});
  }, [transcricaoAtual]);

  const processarFalha = () => {
    if (fila.length === 0 || estadoAnimacao !== 'normal') return;

    proximoAutoMicrofone.current = false;
    const itemFalho = fila[0];
    const novosErros = (itemFalho.erros || 0) + 1;
    const itemAtualizado = { ...itemFalho, erros: novosErros };

    aoTocarAudio(itemFalho.hanzi); // Toca o pinyin de feedback

    // 1º passo: Cartão fica vermelho por 800ms
    setEstadoAnimacao('erro-vermelho');

    // 2º passo: Inicia a animação de deslizamento/descarte
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
        }, 700);
      } else {
        setEstadoAnimacao('erro-reciclar');

        setTimeout(() => {
          setEstadoAnimacao('normal');
          limpar();
          setFila(prev => {
            if (prev.length === 0) return prev;
            const [primeiro, ...resto] = prev;
            return [...resto, itemAtualizado];
          });
        }, 700);
      }
    }, 800);
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
      }, 700);
    }, 800);
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
  }, [escutando, transcricaoFinal, transcricaoParcial, acertoPreviamenteDetectado, respondida, fila, limpar, estadoAnimacao]);

  // Auto-start microphone if previous card was correctly pronounced
  useEffect(() => {
    if (fila.length > 0 && proximoAutoMicrofone.current) {
      proximoAutoMicrofone.current = false;
      setAcertoPreviamenteDetectado(false);
      iniciar();
    }
  }, [fila, iniciar]);

  if (!suportado) {
    return <PronunciaIndisponivel motivo={motivoIndisponivel} />;
  }

  if (fila.length === 0) {
    return null; // Concluído
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

  const feedback = transcricaoAtual
    || (escutando ? 'Ouvindo...' : '')
    || estadoMotor
    || (processando ? 'Transcrevendo...' : '...');

  const calcularEstiloCard = (index: number) => {
    const isSliding = estadoAnimacao === 'acerto-deslizar' || 
                      estadoAnimacao === 'erro-descartar' || 
                      estadoAnimacao === 'erro-reciclar';
    
    if (index === 0) {
      let transformStr = 'scale(1) translateY(0) rotate(0deg)';
      let opacityVal = 1;
      let pointerEventsVal: 'auto' | 'none' = 'auto';

      if (estadoAnimacao === 'acerto-deslizar') {
        transformStr = 'translateX(180%) rotate(25deg) translateY(-20px)';
        opacityVal = 0;
        pointerEventsVal = 'none';
      } else if (estadoAnimacao === 'erro-descartar') {
        transformStr = 'translateX(-180%) rotate(-25deg) translateY(-20px)';
        opacityVal = 0;
        pointerEventsVal = 'none';
      } else if (estadoAnimacao === 'erro-reciclar') {
        transformStr = 'translateY(130%) rotate(-10deg) scale(0.9)';
        opacityVal = 0;
        pointerEventsVal = 'none';
      }

      return {
        transform: transformStr,
        opacity: opacityVal,
        zIndex: 100,
        pointerEvents: pointerEventsVal,
        transition: isSliding ? 'all 0.7s cubic-bezier(0.25, 0.8, 0.25, 1)' : 'border-color 0.2s, box-shadow 0.2s',
      };
    }

    const visualIndex = isSliding ? index - 1 : index;
    const scaleVal = Math.max(0.85, 1 - visualIndex * 0.05);
    const translateYVal = visualIndex * -12;
    const opacityVal = visualIndex > 2 ? 0 : 1 - visualIndex * 0.35;
    const zIndexVal = 100 - index;

    return {
      transform: `scale(${scaleVal}) translateY(${translateYVal}px)`,
      opacity: opacityVal,
      zIndex: zIndexVal,
      pointerEvents: 'none' as const,
      transition: 'all 0.7s cubic-bezier(0.25, 0.8, 0.25, 1)',
    };
  };

  return (
    <div className="revisao-pronuncia-container">
      <div className="revisao-pronuncia-instrucao">
        Pronuncie o caractere do topo do baralho.
      </div>

      <div className="revisao-baralho-area">
        {fila.slice(0, 4).map((item, index) => {
          const estilo = calcularEstiloCard(index);
          const isTop = index === 0;

          return (
            <div
              key={item.id}
              className={`revisao-baralho-card ${isTop ? 'destaque' : ''} ${isTop && (estadoAnimacao === 'acerto-verde' || estadoAnimacao === 'acerto-deslizar') ? 'acerto-anim' : ''} ${isTop && (estadoAnimacao === 'erro-vermelho' || estadoAnimacao === 'erro-descartar' || estadoAnimacao === 'erro-reciclar') ? 'erro-anim' : ''}`}
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
                  color: isTop && (estadoAnimacao === 'acerto-verde' || estadoAnimacao === 'acerto-deslizar') 
                    ? 'var(--cor-sucesso)' 
                    : isTop && (estadoAnimacao === 'erro-vermelho' || estadoAnimacao === 'erro-descartar' || estadoAnimacao === 'erro-reciclar') 
                      ? 'var(--cor-perigo)' 
                      : 'var(--cor-pinyin, #a0aec0)'
                }}
              >
                {item.pinyin}
              </span>
              <span className="hanzi">{item.hanzi}</span>
              <div className="revisao-cartao-tentativas" title="Tentativas restantes">
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

      <div style={{ display: 'flex', gap: '16px', alignItems: 'center', justifyContent: 'center', marginTop: '10px' }}>
        <button
          className={`revisao-btn-microfone ${escutando ? 'escutando' : ''}`}
          onClick={aoClicarMicrofone}
          disabled={respondida || processando || estadoAnimacao !== 'normal'}
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
          title="Desistir do cartão (Reciclar/Pular)"
        >
          <svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
            <path d="M7 11V7a5 5 0 0 1 5-5c1.38 0 2.63.56 3.54 1.46L19 7"/>
            <path d="M2 13h4.14c.9 0 1.54.56 2.46 1.46L12 18"/>
            <path d="M22 17v-4a5 5 0 0 0-5-5c-1.38 0-2.63.56-3.54 1.46L10 13"/>
            <path d="M17 22h-4.14c-.9 0-1.54-.56-2.46-1.46L7 17"/>
          </svg>
        </button>
      </div>

      <div className="revisao-pronuncia-feedback" style={{ marginTop: '16px' }}>
        {transcricaoEmPinyin ? (
          <div style={{ display: 'flex', flexDirection: 'column', alignItems: 'center', gap: '4px' }}>
            <div style={{ fontSize: '18px', color: 'var(--cor-pinyin)' }}>{transcricaoEmPinyin}</div>
            <div style={{ fontSize: '24px', fontFamily: 'var(--fonte-hanzi)', color: 'var(--cor-texto-primario)' }}>{transcricaoEmHanzi}</div>
          </div>
        ) : (
          feedback
        )}
      </div>

      {erro && <div className="revisao-pronuncia-erro">Erro: {erro}</div>}

      <PopupRevisao info={informacoesPopup} />
    </div>
  );
}
