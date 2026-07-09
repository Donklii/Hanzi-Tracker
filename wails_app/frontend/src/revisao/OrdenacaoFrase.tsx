import React, { useState, useEffect } from 'react';
import { main } from '../../wailsjs/go/models';
import { PopupRevisao } from './PopupRevisao';
import { BotaoAudio } from './BotaoAudio';

interface OrdenacaoFraseProps {
  questao: main.QuestaoRevisao;
  respondida: boolean;
  aoConcluir: (acertou: boolean) => void;
  aoTocarAudio: (texto: string) => void;
  hanziTocando: string | null;
  hanziSintetizando: string | null;
  AoClicarNoCartao?: (hanziInfo: { Hanzi: string, Pinyin: string, significados?: string[] }) => void;
}

interface SlotState {
  pileIndex: number;
  hanzi: string;
}

export function OrdenacaoFrase({ questao, respondida, aoConcluir, aoTocarAudio, hanziTocando, hanziSintetizando, AoClicarNoCartao }: OrdenacaoFraseProps) {
  // Array de slots correspondendo a cada '＿' na frase oculta
  const qtdSlots = (questao.fraseOculta.match(/＿/g) || []).length;
  const [slots, setSlots] = useState<(SlotState | null)[]>(new Array(qtdSlots).fill(null));
  const [statusSlots, setStatusSlots] = useState<('correto' | 'posicao_errada' | 'errado' | null)[]>(new Array(qtdSlots).fill(null));
  const [tentativas, setTentativas] = useState(0);
  const [popupInfo, setPopupInfo] = useState<{hanzi: string, pinyin: string, significados: string, x: number, y: number} | null>(null);

  const [usadosNaPilha, setUsadosNaPilha] = useState<boolean[]>(new Array(questao.pilhaOrdenacao?.length || 0).fill(false));

  // Reseta o estado quando a questão muda
  useEffect(() => {
    setSlots(new Array(qtdSlots).fill(null));
    setStatusSlots(new Array(qtdSlots).fill(null));
    setTentativas(0);
    setUsadosNaPilha(new Array(questao.pilhaOrdenacao?.length || 0).fill(false));
  }, [questao, qtdSlots]);

  function clicarPilha(index: number) {
    if (respondida || usadosNaPilha[index]) return;

    const primeiroVazio = slots.findIndex(s => s === null);
    if (primeiroVazio === -1) return; // Todos cheios

    const hanziSelecionado = questao.pilhaOrdenacao[index].hanzi;
    aoTocarAudio(hanziSelecionado);

    const novosSlots = [...slots];
    novosSlots[primeiroVazio] = { pileIndex: index, hanzi: hanziSelecionado };
    setSlots(novosSlots);

    const novosStatus = [...statusSlots];
    novosStatus[primeiroVazio] = null;
    setStatusSlots(novosStatus);

    const novosUsados = [...usadosNaPilha];
    novosUsados[index] = true;
    setUsadosNaPilha(novosUsados);
  }

  function removerSlot(slotIndex: number) {
    if (respondida || slots[slotIndex] === null) return;
    if (statusSlots[slotIndex] === 'correto') return; // Imutável

    const s = slots[slotIndex]!;
    const novosSlots = [...slots];
    novosSlots[slotIndex] = null;
    setSlots(novosSlots);

    const novosStatus = [...statusSlots];
    novosStatus[slotIndex] = null;
    setStatusSlots(novosStatus);

    const novosUsados = [...usadosNaPilha];
    novosUsados[s.pileIndex] = false;
    setUsadosNaPilha(novosUsados);
  }

  const getExpectedHanzis = () => {
    const expected: string[] = [];
    const origChars = Array.from(questao.fraseOriginal);
    const ocultaChars = Array.from(questao.fraseOculta);
    
    let origIdx = 0;
    for (let i = 0; i < ocultaChars.length; i++) {
      if (ocultaChars[i] === '＿') {
        expected.push(origChars[origIdx]);
        origIdx++;
      } else {
        origIdx++;
      }
    }
    return expected;
  };

  function validar() {
    if (slots.includes(null)) return; // não preencheu tudo ainda
    
    const expected = getExpectedHanzis();
    const currentHanzis = slots.map(s => s!.hanzi);
    
    let allCorrect = true;
    for (let i = 0; i < expected.length; i++) {
      if (currentHanzis[i] !== expected[i]) {
        allCorrect = false;
        break;
      }
    }

    if (allCorrect) {
      aoConcluir(true);
      return;
    }

    // Se errou e é a primeira tentativa
    if (tentativas === 0) {
      let correctHanzisCount = 0;
      slots.forEach(s => {
        if (questao.pilhaOrdenacao[s!.pileIndex].correta) {
          correctHanzisCount++;
        }
      });
      
      const percent = correctHanzisCount / qtdSlots;
      if (percent >= 0.75) {
        // Segunda chance
        const novosStatus = [...statusSlots];
        slots.forEach((s, i) => {
          if (s!.hanzi === expected[i]) {
            novosStatus[i] = 'correto';
          } else if (questao.pilhaOrdenacao[s!.pileIndex].correta) {
            novosStatus[i] = 'posicao_errada';
          } else {
            novosStatus[i] = 'errado';
          }
        });
        setStatusSlots(novosStatus);
        setTentativas(1);
        return;
      }
    }

    // Se não for a primeira tentativa ou não atingiu 75%
    aoConcluir(false);
  }

  // Efeito para validar automaticamente quando todos os slots são preenchidos
  useEffect(() => {
    if (!respondida && !slots.includes(null) && slots.length > 0) {
      validar();
    }
  }, [slots, respondida]);

  const partes = questao.fraseOculta.split('＿');

  const getSlotStyle = (i: number) => {
    const preenchido = !!slots[i];
    const status = statusSlots[i];
    
    let borderColor = 'var(--cor-borda)';
    let bgColor = 'rgba(255, 255, 255, 0.05)';
    let textColor = 'var(--cor-destaque)';

    if (preenchido) {
      if (status === 'correto') {
        borderColor = 'var(--cor-sucesso)';
        bgColor = 'rgba(16, 185, 129, 0.1)';
        textColor = 'var(--cor-sucesso)';
      } else if (status === 'posicao_errada') {
        borderColor = 'var(--cor-alerta)';
        bgColor = 'rgba(245, 158, 11, 0.1)';
        textColor = 'var(--cor-alerta)';
      } else if (status === 'errado') {
        borderColor = 'var(--cor-perigo)';
        bgColor = 'rgba(239, 68, 68, 0.1)';
        textColor = 'var(--cor-perigo)';
      } else {
        borderColor = 'var(--cor-destaque)';
        bgColor = 'transparent';
      }
    }

    return { borderColor, bgColor, textColor };
  };

  return (
    <div className="revisao-ordenacao-container" style={{ display: 'flex', flexDirection: 'column', alignItems: 'center', gap: '20px', width: '100%' }}>
      
      {/* Área da Frase */}
      <div className="revisao-ordenacao-frase" style={{ 
        display: 'flex', 
        flexWrap: 'wrap', 
        alignItems: 'center', 
        justifyContent: 'center', 
        fontSize: '32px', 
        lineHeight: 1.6, 
        fontFamily: 'var(--fonte-hanzi)',
        minHeight: '80px',
        padding: '20px',
        background: 'var(--cor-fundo)',
        borderRadius: '12px',
        width: '100%',
        boxShadow: 'none',
        transition: 'box-shadow 0.2s'
      }}>
        {respondida && questao.fraseOriginalSegmentada ? (
          // Exibição pós-resposta: igual ao Modo Contexto
          <div style={{ display: 'flex', gap: '8px', alignItems: 'center', flexWrap: 'wrap', justifyContent: 'center' }}>
            <BotaoAudio
              rotulo=""
              tocando={hanziTocando === questao.fraseOriginal}
              carregando={hanziSintetizando === questao.fraseOriginal}
              aoClicar={() => aoTocarAudio(questao.fraseOriginal)}
            />
            {questao.fraseOriginalSegmentada.map((t, idx) => {
              if (t.ehChines && t.pinyin) {
                return (
                  <span
                    key={idx}
                    style={{ color: t.ehChines ? 'inherit' : 'var(--cor-texto-suave)', cursor: 'pointer', transition: 'color 0.2s' }}
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
                      e.currentTarget.style.color = 'inherit';
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
              return (
                <span key={idx} style={{ color: t.ehChines ? 'inherit' : 'var(--cor-texto-suave)' }}>
                  {t.texto}
                </span>
              );
            })}
          </div>
        ) : (
          // Exibição interativa
          partes.map((parte, i) => {
            const slotObj = i < slots.length ? slots[i] : null;
            const styleProps = i < slots.length ? getSlotStyle(i) : null;
            return (
              <React.Fragment key={i}>
                <span style={{ whiteSpace: 'pre-wrap', color: 'var(--cor-texto-suave)' }}>{parte}</span>
                {i < slots.length && styleProps && (
                  <div 
                    className={`revisao-ordenacao-slot ${slotObj ? 'preenchido' : 'vazio'}`}
                    onClick={() => removerSlot(i)}
                    style={{
                      display: 'inline-flex',
                      alignItems: 'center',
                      justifyContent: 'center',
                      minWidth: '40px',
                      height: '48px',
                      margin: '0 4px',
                      borderBottom: `2px solid ${styleProps.borderColor}`,
                      background: styleProps.bgColor,
                      borderRadius: '4px 4px 0 0',
                      color: styleProps.textColor,
                      cursor: slotObj && !respondida && statusSlots[i] !== 'correto' ? 'pointer' : 'default',
                      transition: 'all 0.2s',
                      verticalAlign: 'bottom'
                    }}
                  >
                    {slotObj ? slotObj.hanzi : ' '}
                  </div>
                )}
              </React.Fragment>
            );
          })
        )}
      </div>

      {/* Pilha de Hanzis */}
      {!respondida && (
        <div className="revisao-ordenacao-pilha" style={{ 
          display: 'flex', 
          flexWrap: 'wrap', 
          gap: '12px', 
          justifyContent: 'center',
          marginTop: '10px' 
        }}>
          {questao.pilhaOrdenacao?.map((opcao, index) => {
            const usado = usadosNaPilha[index];
            return (
              <button
                key={index}
                className="revisao-opcao-btn"
                onClick={() => clicarPilha(index)}
                disabled={usado}
                style={{
                  padding: '12px 20px',
                  fontSize: '28px',
                  fontFamily: 'var(--fonte-hanzi)',
                  background: usado ? 'transparent' : 'var(--cor-fundo-secundario)',
                  color: usado ? 'transparent' : 'var(--cor-texto-primario)',
                  border: usado ? '2px dashed var(--cor-texto-suave)' : '2px solid var(--cor-borda)',
                  borderRadius: '12px',
                  cursor: usado ? 'default' : 'pointer',
                  transition: 'all 0.2s',
                  minWidth: '60px',
                  boxShadow: usado ? 'none' : '0 4px 6px rgba(0,0,0,0.1)'
                }}
              >
                {opcao.hanzi}
              </button>
            );
          })}
        </div>
      )}
      <PopupRevisao info={popupInfo} />
    </div>
  );
}
