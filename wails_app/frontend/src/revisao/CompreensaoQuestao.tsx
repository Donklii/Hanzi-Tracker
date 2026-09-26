// ----- Seção: Revisão — Atividade de Compreensão de Leitura -----
// Apresenta o texto/contexto em chinês, a pergunta sobre o texto e opções de resposta.
// Exibe feedback visual de acerto/erro e revela traduções ao final ou no modo guiado.

import React, { useState, useEffect } from 'react';
import { revisao, busca } from '../../wailsjs/go/models';
import { t } from '../i18n/i18n';
import { PopupRevisao } from './PopupRevisao';
import { BotaoAudio } from './BotaoAudio';

interface CompreensaoQuestaoProps {
  questao: revisao.QuestaoRevisao;
  respondida: boolean;
  indiceEscolhido: number | null;
  aoEscolher: (indice: number) => void;
  mostrarTraducaoDireta?: boolean;
  AoClicarNoCartao?: (info: { Hanzi: string; Pinyin: string; significados?: string[] }) => void;
  aoTocarAudio?: (texto: string) => void;
  hanziTocando?: string | null;
  hanziSintetizando?: string | null;
}

const removerPrefixoPersonagem = (texto: string): string => {
  if (!texto) return '';
  return texto.replace(/^[^:：]{1,5}[:：]\s*/, '').trim();
};

const filtrarTokensPersonagem = (tokens: any[]): any[] => {
  if (!tokens || tokens.length === 0) return [];
  let colonIdx = -1;
  let charCount = 0;
  for (let i = 0; i < tokens.length; i++) {
    charCount += tokens[i].texto.length;
    if (charCount > 15) break; 
    if (tokens[i].texto === '：' || tokens[i].texto === ':') {
      colonIdx = i;
      break;
    }
  }
  if (colonIdx !== -1) {
    return tokens.slice(colonIdx + 1);
  }
  return tokens;
};

const obterCorPersonagem = (textoSpeaker: string): string => {
  if (textoSpeaker.includes('女')) {
    return '#f472b6'; // Rosa claro
  }
  if (textoSpeaker.includes('男')) {
    return '#2563eb'; // Azul escuro
  }
  return '#2563eb'; // Azul escuro padrão
};

export function CompreensaoQuestao({
  questao,
  respondida,
  indiceEscolhido,
  aoEscolher,
  mostrarTraducaoDireta = false,
  AoClicarNoCartao,
  aoTocarAudio,
  hanziTocando,
  hanziSintetizando,
}: CompreensaoQuestaoProps) {
  const [popupInfo, setPopupInfo] = useState<{hanzi: string, pinyin: string, significados: string, x: number, y: number} | null>(null);
  const [tokensFala2, setTokensFala2] = useState<any[]>([]);
  const [tokensOpcoes, setTokensOpcoes] = useState<{ [idx: number]: any[] }>({});

  const ehDialogo = questao.variante === 'resposta_dialogo';

  const executarAudio = (texto: string) => {
    if (!texto) return;
    const textoSemPersonagem = removerPrefixoPersonagem(texto);
    if (aoTocarAudio) {
      aoTocarAudio(textoSemPersonagem);
    } else {
      const falarFn = (window as any).go?.main?.App?.FalarPinyinRevisao;
      if (falarFn) {
        falarFn(textoSemPersonagem, 'system').catch(() => {});
      }
    }
  };

  let fala2Texto = '';
  if (ehDialogo && respondida && indiceEscolhido !== null && questao.opcoes[indiceEscolhido]) {
    fala2Texto = removerPrefixoPersonagem(questao.opcoes[indiceEscolhido].hanzi || questao.opcoes[indiceEscolhido].definicao || '');
  } else if (ehDialogo && respondida) {
    const corretaOpt = questao.opcoes.find((o: busca.OpcaoRevisao) => o.correta);
    if (corretaOpt) {
      fala2Texto = removerPrefixoPersonagem(corretaOpt.hanzi || corretaOpt.definicao || '');
    }
  }

  useEffect(() => {
    if (ehDialogo && respondida && fala2Texto) {
      if (indiceEscolhido !== null && questao.opcoes[indiceEscolhido]?.correta && (questao as any).perguntaCompreensaoSegmentada) {
        setTokensFala2(filtrarTokensPersonagem((questao as any).perguntaCompreensaoSegmentada));
      } else {
        const decomporFn = (window as any).go?.main?.App?.DecomporTextoRevisao;
        if (decomporFn) {
          decomporFn(fala2Texto)
            .then((toks: any[]) => setTokensFala2(filtrarTokensPersonagem(toks)))
            .catch(() => setTokensFala2([]));
        } else {
          setTokensFala2([]);
        }
      }
    } else {
      setTokensFala2([]);
    }
  }, [ehDialogo, respondida, indiceEscolhido, fala2Texto, questao]);

  useEffect(() => {
    if (!ehDialogo && respondida) {
      const decomporFn = (window as any).go?.main?.App?.DecomporTextoRevisao;
      if (decomporFn && questao.opcoes) {
        const promises = questao.opcoes.map((opcao: busca.OpcaoRevisao) => {
          const txt = opcao.hanzi || opcao.definicao || '';
          return decomporFn(txt).catch(() => []);
        });
        Promise.all(promises).then((results) => {
          const mapToks: { [idx: number]: any[] } = {};
          results.forEach((toks: any[], idx: number) => {
            mapToks[idx] = toks;
          });
          setTokensOpcoes(mapToks);
        });
      }
    } else {
      setTokensOpcoes({});
    }
  }, [ehDialogo, respondida, questao]);

  const classesOpcao = (indice: number): string => {
    const classes = ['revisao-opcao'];
    if (respondida && questao.opcoes[indice]?.correta) classes.push('correta');
    if (respondida && indice === indiceEscolhido && !questao.opcoes[indice]?.correta) classes.push('errada');
    return classes.join(' ');
  };

  const renderOpcaoTexto = (texto?: string) => {
    if (!texto) return null;
    const match = texto.match(/^([^:：]{1,5}[:：])\s*(.*)$/);
    if (match) {
      const speaker = match[1];
      const rest = match[2];
      const color = obterCorPersonagem(speaker);
      return (
        <span>
          <span style={{ color, fontWeight: 'bold', marginRight: '6px' }}>{speaker}</span>
          <span>{rest}</span>
        </span>
      );
    }
    return <span>{texto}</span>;
  };

  const renderSegmentos = (tokens: any[], fontSizeHanzi = '28px', fontSizePinyin = '14px', marginBottom = '16px') => {
    if (!tokens || tokens.length === 0) return null;
    const lines: any[][] = [];
    let currentLine: any[] = [];
    tokens.forEach((t: any) => {
      if (t.texto === '\n') {
        lines.push(currentLine);
        currentLine = [];
      } else {
        currentLine.push(t);
      }
    });
    if (currentLine.length > 0) lines.push(currentLine);

    return lines.map((lineTokens, lineIdx) => {
      let colonIdx = -1;
      let charCount = 0;
      for (let i = 0; i < lineTokens.length; i++) {
        charCount += lineTokens[i].texto.length;
        if (charCount > 15) break; 
        if (lineTokens[i].texto === '：' || lineTokens[i].texto === ':') {
          colonIdx = i;
          break;
        }
      }

      let speakerColor = '#2563eb';
      if (colonIdx !== -1) {
        const speakerText = lineTokens.slice(0, colonIdx).map((tk: any) => tk.texto).join('');
        speakerColor = obterCorPersonagem(speakerText);
      }

      return (
        <div key={lineIdx} style={{ marginBottom }}>
          {lineTokens.map((t, idx) => {
            const isSpeaker = colonIdx !== -1 && idx < colonIdx;
            const isColon = idx === colonIdx;
            const defaultColor = (isSpeaker || isColon) ? speakerColor : (t.ehNaoVista ? undefined : 'inherit');

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
                      fontSize: fontSizePinyin,
                      color: (isSpeaker || isColon) ? speakerColor : (t.ehNaoVista ? undefined : 'var(--cor-pinyin, #a0aec0)'),
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
                      color: defaultColor,
                      fontWeight: (isSpeaker || isColon) ? 'bold' : 'normal',
                      fontSize: fontSizeHanzi,
                      lineHeight: 1.2,
                      borderBottom: (isSpeaker || isColon) ? 'none' : '1px dotted #475569',
                    }}
                    onMouseEnter={(e) => {
                      e.currentTarget.style.color = 'var(--cor-destaque)';
                      const rect = e.currentTarget.getBoundingClientRect();
                      setPopupInfo({
                        pinyin: t.pinyin || '',
                        hanzi: t.texto,
                        significados: t.significados ? t.significados.join(', ') : '',
                        x: rect.left + rect.width / 2,
                        y: rect.top
                      });
                    }}
                    onMouseLeave={(e) => {
                      e.currentTarget.style.color = defaultColor || '';
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
                  fontSize: fontSizeHanzi, 
                  lineHeight: 1.2,
                  color: (isSpeaker || isColon) ? speakerColor : 'inherit',
                  fontWeight: (isSpeaker || isColon) ? 'bold' : 'normal'
                }}
              >
                {t.texto}
              </span>
            );
          })}
        </div>
      );
    });
  };

  if (ehDialogo) {
    const tokensFala1 = filtrarTokensPersonagem(questao.fraseOriginalSegmentada || []);
    const fala1Texto = removerPrefixoPersonagem(questao.fraseOriginal || '');

    return (
      <div className="compreensao-container" style={{ display: 'flex', flexDirection: 'column', gap: '20px', alignItems: 'center', width: '100%', maxWidth: '640px', margin: '0 auto' }}>
        <div style={{ fontSize: '12px', color: 'var(--cor-destaque, #7c3aed)', textTransform: 'uppercase', letterSpacing: '1px', fontWeight: 600 }}>
          {t('Completar Diálogo')}
        </div>

        {/* --- Balões de Diálogo Simétricos --- */}
        <div style={{ display: 'flex', flexDirection: 'column', gap: '16px', width: '100%' }}>
          {/* Fala 1 (Interlocutor) - Alinhado à Esquerda com Botão TTS */}
          <div style={{ display: 'flex', justifyContent: 'flex-start', width: '100%' }}>
            <div
              className="balao-dialogo fala-1"
              style={{
                position: 'relative',
                background: 'var(--cor-fundo-card, #1e1e24)',
                border: '1px solid var(--cor-borda, #2e2e38)',
                borderRadius: '18px 18px 18px 4px',
                padding: '20px 24px',
                width: '88%',
                boxShadow: '0 4px 12px rgba(0,0,0,0.15)',
                display: 'flex',
                alignItems: 'flex-start',
                gap: '14px',
              }}
            >
              {fala1Texto && (
                <div style={{ flexShrink: 0, marginTop: '2px' }}>
                  <BotaoAudio
                    rotulo=""
                    tocando={hanziTocando === fala1Texto}
                    carregando={hanziSintetizando === fala1Texto}
                    aoClicar={() => executarAudio(fala1Texto)}
                    titulo={t('Ouvir fala')}
                  />
                </div>
              )}

              <div style={{ flex: 1 }}>
                <div style={{ fontFamily: 'var(--fonte-hanzi, "PingFang SC", sans-serif)', fontSize: '26px', lineHeight: 1.6 }}>
                  {tokensFala1.length > 0 ? (
                    renderSegmentos(tokensFala1, '26px', '13px', '4px')
                  ) : (
                    <span>{fala1Texto}</span>
                  )}
                </div>
                {(mostrarTraducaoDireta || respondida) && (
                  <div style={{ fontSize: '13px', color: 'var(--cor-texto-suave, #9ca3af)', marginTop: '6px', fontStyle: 'italic' }}>
                    {removerPrefixoPersonagem(questao.contextoTraduzido || questao.fraseTraducao || '')}
                  </div>
                )}
              </div>
            </div>
          </div>

          {/* Fala 2 (Resposta) - Alinhado à Direita com Botão TTS ao responder */}
          <div style={{ display: 'flex', justifyContent: 'flex-end', width: '100%' }}>
            <div
              className={`balao-dialogo fala-2 ${respondida ? 'respondido' : 'vazio'}`}
              style={{
                position: 'relative',
                background: respondida 
                  ? (questao.opcoes[indiceEscolhido ?? -1]?.correta ? 'rgba(16, 185, 129, 0.15)' : 'rgba(239, 68, 68, 0.15)')
                  : 'var(--cor-fundo-card, #1e1e24)',
                border: respondida
                  ? (questao.opcoes[indiceEscolhido ?? -1]?.correta ? '2px solid #10b981' : '2px solid #ef4444')
                  : '2px dashed var(--cor-destaque, #7c3aed)',
                borderRadius: '18px 18px 4px 18px',
                padding: '20px 24px',
                width: '88%',
                minHeight: '68px',
                display: 'flex',
                alignItems: 'flex-start',
                justifyContent: respondida ? 'flex-start' : 'center',
                gap: '14px',
                boxShadow: '0 4px 12px rgba(0,0,0,0.15)',
                transition: 'all 0.3s ease',
              }}
            >
              {fala2Texto && respondida && (
                <div style={{ flexShrink: 0, marginTop: '2px' }}>
                  <BotaoAudio
                    rotulo=""
                    tocando={hanziTocando === fala2Texto}
                    carregando={hanziSintetizando === fala2Texto}
                    aoClicar={() => executarAudio(fala2Texto)}
                    titulo={t('Ouvir resposta')}
                  />
                </div>
              )}

              {fala2Texto ? (
                <div style={{ fontFamily: 'var(--fonte-hanzi, "PingFang SC", sans-serif)', fontSize: '26px', lineHeight: 1.6, color: 'var(--cor-texto, #f3f4f6)', flex: 1 }}>
                  {tokensFala2 && tokensFala2.length > 0 ? (
                    renderSegmentos(tokensFala2, '26px', '13px', '4px')
                  ) : (
                    <span>{fala2Texto}</span>
                  )}
                </div>
              ) : (
                <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'center', gap: '8px', color: 'var(--cor-destaque, #7c3aed)', opacity: 0.8, fontSize: '15px', fontWeight: 500, width: '100%', height: '100%' }}>
                  <span style={{ fontSize: '20px' }}>💬</span>
                  <span>{t('Selecione a resposta abaixo...')}</span>
                </div>
              )}
            </div>
          </div>
        </div>

        <div style={{ width: '100%', textAlign: 'center', padding: '4px 12px' }}>
          <h3 style={{ fontSize: '16px', fontWeight: 600, color: 'var(--cor-texto-suave, #9ca3af)', margin: 0 }}>
            {t('Qual é a resposta mais adequada?')}
          </h3>
        </div>

        {/* --- Grade de Opções de Múltipla Escolha --- */}
        <div className="revisao-opcoes vertical" style={{ width: '100%', display: 'flex', flexDirection: 'column', gap: '14px' }}>
          {questao.opcoes.map((opcao: busca.OpcaoRevisao, indice: number) => {
            const textoOpcaoLimpo = removerPrefixoPersonagem(opcao.hanzi || opcao.definicao || '');
            return (
              <button
                key={indice}
                className={classesOpcao(indice)}
                disabled={respondida}
                onClick={() => aoEscolher(indice)}
                style={{
                  width: '100%',
                  padding: '18px 24px',
                  textAlign: 'left',
                  fontSize: '22px',
                  lineHeight: 1.5,
                  minHeight: '62px',
                  fontFamily: 'var(--fonte-hanzi, "PingFang SC", sans-serif)',
                  display: 'flex',
                  alignItems: 'center',
                  justifyContent: 'space-between',
                  gap: '16px',
                  borderRadius: '14px',
                }}
              >
                <div><span>{textoOpcaoLimpo}</span></div>
                <span style={{ fontSize: '15px', opacity: 0.7, fontWeight: 600, flexShrink: 0 }}>
                  {['A', 'B', 'C', 'D'][indice]}
                </span>
              </button>
            );
          })}
        </div>

        <PopupRevisao info={popupInfo} />
      </div>
    );
  }

  const fraseContextoTexto = questao.fraseOriginal || questao.fraseLacuna;

  return (
    <div className="compreensao-container" style={{ display: 'flex', flexDirection: 'column', gap: '16px', alignItems: 'center', width: '100%', maxWidth: '640px', margin: '0 auto' }}>
      {/* --- Cartão de Contexto --- */}
      <div
        className="compreensao-contexto-card"
        style={{
          background: 'var(--cor-fundo-card, #1e1e24)',
          border: '1px solid var(--cor-borda, #2e2e38)',
          borderRadius: '12px',
          padding: '20px 24px',
          width: '100%',
          textAlign: 'center',
          boxShadow: '0 4px 12px rgba(0,0,0,0.15)',
        }}
      >
        <div style={{ fontSize: '12px', color: 'var(--cor-destaque, #7c3aed)', textTransform: 'uppercase', letterSpacing: '1px', fontWeight: 600, marginBottom: '8px' }}>
          {t('Contexto de Leitura')}
        </div>
        <div
          style={{
            fontFamily: 'var(--fonte-hanzi, "PingFang SC", "Microsoft YaHei", sans-serif)',
            fontSize: '24px',
            lineHeight: '1.6',
            color: 'var(--cor-texto, #f3f4f6)',
            marginBottom: '8px',
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center',
            gap: '12px',
          }}
        >
          {fraseContextoTexto && (
            <div style={{ flexShrink: 0 }}>
              <BotaoAudio
                rotulo=""
                tocando={hanziTocando === removerPrefixoPersonagem(fraseContextoTexto)}
                carregando={hanziSintetizando === removerPrefixoPersonagem(fraseContextoTexto)}
                aoClicar={() => executarAudio(fraseContextoTexto)}
                titulo={t('Ouvir pronúncia')}
              />
            </div>
          )}

          {questao.fraseOriginalSegmentada && questao.fraseOriginalSegmentada.length > 0 ? (
            <div style={{ whiteSpace: 'pre-wrap', lineHeight: 1.8, textAlign: 'left', display: 'inline-block' }}>
              {renderSegmentos(questao.fraseOriginalSegmentada, '28px', '14px', '16px')}
            </div>
          ) : (
            <div style={{ whiteSpace: 'pre-wrap' }}>{renderOpcaoTexto(fraseContextoTexto)}</div>
          )}
        </div>

        {(mostrarTraducaoDireta || respondida) && (
          <div style={{ fontSize: '14px', color: 'var(--cor-texto-suave, #9ca3af)', marginTop: '8px', fontStyle: 'italic' }}>
            {removerPrefixoPersonagem(questao.contextoTraduzido || questao.fraseTraducao || '')}
          </div>
        )}
      </div>

      {/* --- Caixa de Pergunta Formatada --- */}
      <div
        className="compreensao-pergunta-box"
        style={{
          width: '100%',
          textAlign: 'center',
          padding: '14px 18px',
          background: 'var(--cor-fundo-card, #1e1e24)',
          border: '1px solid var(--cor-borda, #2e2e38)',
          borderRadius: '14px',
          boxShadow: '0 4px 12px rgba(0,0,0,0.12)',
        }}
      >
        <div style={{ fontSize: '20px', fontWeight: 600, color: 'var(--cor-texto, #f3f4f6)', margin: 0, display: 'inline-block' }}>
          {(questao as any).perguntaCompreensaoSegmentada && (questao as any).perguntaCompreensaoSegmentada.length > 0 ? (
            renderSegmentos((questao as any).perguntaCompreensaoSegmentada, '24px', '13px', '4px')
          ) : (
            renderOpcaoTexto(questao.perguntaCompreensao) || t('Responda com base no contexto acima:')
          )}
        </div>
        {(mostrarTraducaoDireta || respondida) && questao.perguntaTraduzida && (
          <div style={{ fontSize: '14px', color: 'var(--cor-texto-suave, #9ca3af)', marginTop: '6px', fontStyle: 'italic' }}>
            {questao.perguntaTraduzida}
          </div>
        )}
      </div>

      {/* --- Grade de Opções de Múltipla Escolha --- */}
      <div className="revisao-opcoes vertical" style={{ width: '100%', display: 'flex', flexDirection: 'column', gap: '14px' }}>
        {questao.opcoes.map((opcao: busca.OpcaoRevisao, indice: number) => {
          const toks = tokensOpcoes[indice];
          const textoFull = opcao.hanzi || opcao.definicao || '';
          return (
            <button
              key={indice}
              className={classesOpcao(indice)}
              disabled={respondida}
              onClick={() => aoEscolher(indice)}
              style={{
                width: '100%',
                padding: '18px 24px',
                textAlign: 'left',
                fontSize: '22px',
                lineHeight: 1.5,
                minHeight: '62px',
                fontFamily: 'var(--fonte-hanzi, "PingFang SC", sans-serif)',
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'space-between',
                gap: '16px',
                borderRadius: '14px',
              }}
            >
              <div style={{ flex: 1 }}>
                {toks && toks.length > 0 ? (
                  renderSegmentos(toks, '22px', '13px', '0px')
                ) : (
                  renderOpcaoTexto(textoFull)
                )}
              </div>
              <span style={{ fontSize: '15px', opacity: 0.7, fontWeight: 600, flexShrink: 0 }}>
                {['A', 'B', 'C', 'D'][indice]}
              </span>
            </button>
          );
        })}
      </div>
      
      <PopupRevisao info={popupInfo} />
    </div>
  );
}
