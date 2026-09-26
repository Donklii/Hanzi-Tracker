// ----- Seção: Dicionário -----
import { useState, useEffect } from 'react';
import './dicionario.css';
import { CanvasHanziLookup } from './CanvasHanziLookup';
import { ArvoreGenealogicaDecomposicao } from './ArvoreGenealogicaDecomposicao';
import { CanvasDesenho } from '../comum/CanvasDesenho';
import { BuscarCaracteresCompostosPor, ObterEstatisticasPalavra, ObterInformacaoExpansao, LookupWord, SegmentarTexto } from '../../wailsjs/go/main/App';
import { dicionario } from '../../wailsjs/go/models';
import { TocarAudio } from '../comum/TocarAudio';
import { t } from '../i18n/i18n';
import { AREAS_APRENDIZADO, IconeCheck } from '../revisao/IconesPlacar';
import { obterTiposBolinhas } from '../revisao/progressoBolinhas';
import '../revisao/revisao.css';

const IconPencil = () => (
  <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" style={{ width: '24px', height: '24px' }}>
    <path d="M12 20h9" />
    <path d="M16.5 3.5a2.121 2.121 0 0 1 3 3L7 19l-4 1 1-4L16.5 3.5z" />
  </svg>
);

const IconList = () => (
  <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" style={{ width: '24px', height: '24px' }}>
    <line x1="8" y1="6" x2="21" y2="6" />
    <line x1="8" y1="12" x2="21" y2="12" />
    <line x1="8" y1="18" x2="21" y2="18" />
    <line x1="3" y1="6" x2="3.01" y2="6" />
    <line x1="3" y1="12" x2="3.01" y2="12" />
    <line x1="3" y1="18" x2="3.01" y2="18" />
  </svg>
);

const IconStats = () => (
  <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" style={{ width: '24px', height: '24px' }}>
    <line x1="18" y1="20" x2="18" y2="10" />
    <line x1="12" y1="20" x2="12" y2="4" />
    <line x1="6" y1="20" x2="6" y2="14" />
  </svg>
);

const IconSearch = () => (
  <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" style={{ width: '24px', height: '24px' }}>
    <circle cx="11" cy="11" r="8" />
    <line x1="21" y1="21" x2="16.65" y2="16.65" />
  </svg>
);

const IconRanking = ({ size = 14 }: { size?: number }) => (
  <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" style={{ width: `${size}px`, height: `${size}px` }}>
    <line x1="18" y1="20" x2="18" y2="10" />
    <line x1="12" y1="20" x2="12" y2="4" />
    <line x1="6" y1="20" x2="6" y2="14" />
  </svg>
);

const IconRefresh = ({ size = 14 }: { size?: number }) => (
  <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" style={{ width: `${size}px`, height: `${size}px` }}>
    <path d="M23 4v6h-6" />
    <path d="M1 20v-6h6" />
    <path d="M3.51 9a9 9 0 0 1 14.85-3.36L23 10M1 14l4.64 4.36A9 9 0 0 0 20.49 15" />
  </svg>
);

const IconHSK = ({ size = 14 }: { size?: number }) => (
  <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" style={{ width: `${size}px`, height: `${size}px` }}>
    <circle cx="12" cy="8" r="6" />
    <path d="M15.477 12.89L17 22l-5-3-5 3 1.523-9.11" />
  </svg>
);

interface ModalCartaoDetalhesProps {
  cartaoSelecionado: any | null;
  setCartaoSelecionado: (val: any | null) => void;
  imagemModalBase64: string | null;
  dadosDecomposicao: any | null;
  AoClicarNoCaractereDecomposto: (char: string) => void;
  isEstudando: boolean;
  onToggleEstudo: () => void;
  isAprendida: boolean;
  onToggleAprendida: () => void;
  motorTtsAtivo: string;
  lerPinyinAoCompletarDesenho: boolean;
  configuracoesApp?: any;
  aoBuscarPalavrasCompostas: (hanzi: string) => void;
  aoBuscarPorRanking: (hanzi: string) => void;
  aoBuscarPorHSK?: (hanzi: string) => void;
}

export function ModalCartaoDetalhes(props: ModalCartaoDetalhesProps) {
  const {
    cartaoSelecionado, setCartaoSelecionado,
    imagemModalBase64, dadosDecomposicao,
    AoClicarNoCaractereDecomposto,
    isEstudando, onToggleEstudo,
    isAprendida, onToggleAprendida,
    motorTtsAtivo,
    lerPinyinAoCompletarDesenho,
    configuracoesApp,
    aoBuscarPalavrasCompostas,
    aoBuscarPorRanking,
    aoBuscarPorHSK
  } = props;

  const [modoDesenhoLivre, setModoDesenhoLivre] = useState(false);
  const [modoTreinoGuiado, setModoTreinoGuiado] = useState(false);
  const [modoCompostos, setModoCompostos] = useState(false);
  const [modoEstatisticas, setModoEstatisticas] = useState(false);
  const [estatisticas, setEstatisticas] = useState<Record<string, number> | null>(null);
  const [infoExpansao, setInfoExpansao] = useState<{ posicaoRanking: number; alternativa: string; tipoHanzi: string; nivelHSK?: number } | null>(null);
  const [caracteresCompostos, setCaracteresCompostos] = useState<string[]>([]);
  const [sugestoesPratica, setSugestoesPratica] = useState<string[]>([]);
  const [treinoConcluidoMsg, setTreinoConcluidoMsg] = useState('');
  const [audioTocando, setAudioTocando] = useState(false);
  const [informacoesDoPopup, setInformacoesDoPopup] = useState<{
    hanzi: string;
    pinyin: string;
    significados: string;
    x: number;
    y: number;
  } | null>(null);
  const [leiturasModal, setLeiturasModal] = useState<dicionario.EntradaDicionario[] | null>(null);
  const [pinyinModalExibicao, setPinyinModalExibicao] = useState('');
  const [mostrarMaisSignificadosModal, setMostrarMaisSignificadosModal] = useState(false);
  const [hoverNoRanking, setHoverNoRanking] = useState(false);

  const hanziAtual = cartaoSelecionado ? (cartaoSelecionado.hanzi || cartaoSelecionado.Hanzi) : '';
  const isMultiChar = hanziAtual.length > 1;

  // Resetar modos de prática sempre que o modal for reaberto ou mudar de caractere.
  // abrirEmEstatisticas (ver AbaRevisao.tsx) já abre direto nas Estatísticas de Aprendizado —
  // usado pelos cards do grupo de foco, onde é isso que o usuário quer ver primeiro.
  useEffect(() => {
    setModoDesenhoLivre(false);
    setModoTreinoGuiado(false);
    setModoCompostos(false);
    setModoEstatisticas(!!cartaoSelecionado?.abrirEmEstatisticas);
    setEstatisticas(null);
    setInfoExpansao(null);
    setCaracteresCompostos([]);
    setSugestoesPratica([]);
    setTreinoConcluidoMsg('');
    setAudioTocando(false);
    setInformacoesDoPopup(null);
    setMostrarMaisSignificadosModal(false);
    setLeiturasModal(null);
    setPinyinModalExibicao(cartaoSelecionado?.pinyin || cartaoSelecionado?.Pinyin || '');
    setHoverNoRanking(false);
  }, [cartaoSelecionado?.hanzi, cartaoSelecionado?.Hanzi, cartaoSelecionado?.abrirEmEstatisticas]);

  // Carregar leituras e pinyins variantes da palavra para o modal
  useEffect(() => {
    if (!hanziAtual) {
      setLeiturasModal(null);
      setPinyinModalExibicao('');
      return;
    }

    LookupWord(hanziAtual)
      .then(entradas => {
        if (entradas && entradas.length > 0) {
          if (entradas.length > 1) {
            setLeiturasModal(entradas);
          } else {
            setLeiturasModal(null);
          }

          const pinyinsUnicos: string[] = [];
          for (const ent of entradas) {
            if (ent.Pinyin && !pinyinsUnicos.includes(ent.Pinyin)) {
              pinyinsUnicos.push(ent.Pinyin);
            }
          }

          if (pinyinsUnicos.length > 0) {
            setPinyinModalExibicao(pinyinsUnicos.join(' / '));
          }
        }
      })
  }, [hanziAtual]);

  // Carregar estatísticas do caractere
  useEffect(() => {
    if (modoEstatisticas && hanziAtual) {
      ObterEstatisticasPalavra(hanziAtual)
        .then((stats: any) => {
          setEstatisticas(stats);
        })
        .catch((err: any) => {
          console.error("Erro ao obter estatísticas:", err);
        });
    }
  }, [modoEstatisticas, hanziAtual]);

  // Carregar informações extras do card (ranking e alternativa tradicional/simplificada)
  useEffect(() => {
    if (hanziAtual) {
      ObterInformacaoExpansao(hanziAtual)
        .then((info: any) => {
          setInfoExpansao(info);
        })
        .catch((err: any) => {
          console.error("Erro ao obter informações de expansão:", err);
        });
    } else {
      setInfoExpansao(null);
    }
  }, [hanziAtual]);


  if (!cartaoSelecionado) return null;
  
  // Priorizar tradução do makemeahanzi se for um caractere único
  const significadoCartao = cartaoSelecionado.significados ? cartaoSelecionado.significados.join(', ') : cartaoSelecionado.Significado;
  const significadoPrioritario = (dadosDecomposicao?.type === 'single' && dadosDecomposicao.data?.definition)
    ? dadosDecomposicao.data.definition
    : significadoCartao;

  const qualquerModoAtivo = modoDesenhoLivre || modoTreinoGuiado || modoCompostos || modoEstatisticas;

  const fecharModos = () => {
    setModoDesenhoLivre(false);
    setModoTreinoGuiado(false);
    setModoCompostos(false);
    setModoEstatisticas(false);
    setSugestoesPratica([]);
    setTreinoConcluidoMsg('');
  };

  const carregarCompostos = () => {
    BuscarCaracteresCompostosPor(hanziAtual).then(chars => {
      setCaracteresCompostos(chars || []);
      setModoCompostos(true);
    });
  };

  const tocarAudioPratica = async () => {
    if (audioTocando) return;
    setAudioTocando(true);
    await TocarAudio(hanziAtual, motorTtsAtivo);
    setAudioTocando(false);
  };
  if (cartaoSelecionado.isScreenshotCard) {
    return (
      <div className="modal-overlay" onClick={() => setCartaoSelecionado(null)}>
        <div className="modal-content hanzi-modal-content" onClick={e => e.stopPropagation()} style={{ maxWidth: '800px', width: '90%' }}>
          <div className="modal-header">
            <h2>{t('Detalhes da Captura')}</h2>
            <button className="modal-close" onClick={() => setCartaoSelecionado(null)}>×</button>
          </div>
          
          <div className="modal-body" style={{ display: 'flex', flexDirection: 'column', alignItems: 'center', gap: '20px' }}>
            <div style={{ border: '1px solid var(--cor-borda)', padding: '4px', borderRadius: '4px', backgroundColor: 'var(--cor-fundo-cartao)', width: '100%', display: 'flex', justifyContent: 'center' }}>
              <img 
                src={imagemModalBase64 ? "data:image/png;base64," + imagemModalBase64 : ''} 
                alt={t("Última captura OCR")} 
                style={{ maxWidth: '100%', maxHeight: '40vh', objectFit: 'contain' }} 
              />
            </div>

            <div style={{ textAlign: 'center', position: 'relative', width: '100%' }}>
              <div style={{ fontSize: '28px', color: 'var(--cor-pinyin)', fontWeight: 'bold', marginBottom: '10px' }}>
                {t('Visualizar Print Escaneado')}
              </div>
              
              <div style={{ display: 'flex', justifyContent: 'center', alignItems: 'center', margin: '20px 0' }}>
                <svg width="80" height="80" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round" style={{ color: 'var(--cor-texto-primario)' }}>
                  <path d="M23 19a2 2 0 0 1-2 2H3a2 2 0 0 1-2-2V8a2 2 0 0 1 2-2h4l2-3h6l2 3h4a2 2 0 0 1 2 2z"></path>
                  <circle cx="12" cy="13" r="4"></circle>
                </svg>
              </div>

              <div style={{ fontSize: '18px', color: 'var(--cor-texto-primario)', marginBottom: '10px' }}>
                {significadoCartao}
              </div>
              
              <div style={{ fontSize: '14px', color: 'var(--cor-texto-suave)' }}>
                {t('Status: Processado com sucesso')}
              </div>
            </div>
          </div>
        </div>
      </div>
    );
  }

  return (
    <div className="modal-overlay" onClick={() => setCartaoSelecionado(null)}>
      <div className="modal-content hanzi-modal-content" onClick={e => e.stopPropagation()}>
        <div className="modal-header">
          <h2>
            {modoDesenhoLivre ? t("Desenho Livre (Busca)") :
             modoTreinoGuiado ? t("Treino Guiado de Caligrafia") :
             modoCompostos ? t("Caracteres Compostos") :
             modoEstatisticas ? t("Estatísticas de Aprendizado") :
             t("Detalhes")}
          </h2>
          <button className="modal-close" onClick={() => setCartaoSelecionado(null)}>×</button>
        </div>
        
        <div className="modal-body" style={{display: 'flex', flexDirection: 'column', alignItems: 'center', gap: '20px'}}>
          
          {modoDesenhoLivre ? (
            <div style={{ width: '100%', display: 'flex', flexDirection: 'column', alignItems: 'center' }}>
              <div style={{ marginBottom: '16px', color: 'var(--cor-texto-suave)', fontSize: '14px', textAlign: 'center' }}>
                {t('Desenhe por cima do Hanzi para pesquisar caracteres estruturalmente semelhantes ou compostos por ele.')}
                <br />
                <span style={{ fontSize: '12px', opacity: 0.8 }}>{t('⚠️ A ordem e a direção dos traços importa para o reconhecimento correto.')}</span>
              </div>
              <CanvasHanziLookup 
                onRecognize={(sugestoes) => setSugestoesPratica(sugestoes)}
                targetHanzi={hanziAtual}
                configuracoesApp={configuracoesApp}
              />
              
              <div style={{ display: 'flex', flexWrap: 'wrap', gap: '8px', marginTop: '12px', justifyContent: 'center', minHeight: '40px' }}>
                {sugestoesPratica.length === 0 ? (
                   <span style={{ fontSize: '12px', color: 'var(--cor-texto-suave)', marginTop: '10px' }}>
                     {t('Nenhuma correspondência ainda.')}
                   </span>
                ) : (
                  sugestoesPratica.map((hz, idx) => (
                    <div
                      key={idx}
                      className="scan-btn"
                      style={{ fontSize: '20px', padding: '4px 12px', fontFamily: 'var(--fonte-hanzi)', cursor: 'pointer' }}
                      onClick={() => {
                        fecharModos();
                        AoClicarNoCaractereDecomposto(hz);
                      }}
                    >
                      {hz}
                    </div>
                  ))
                )}
              </div>
            </div>
          ) : modoCompostos ? (
            <div style={{ width: '100%', display: 'flex', flexDirection: 'column', alignItems: 'center' }}>
              <div style={{ fontSize: '16px', marginBottom: '15px', color: 'var(--cor-texto-suave)' }}>
                {t('Caracteres que contêm {hanzi}', { hanzi: hanziAtual })}
              </div>
              <div style={{ display: 'flex', flexWrap: 'wrap', gap: '8px', justifyContent: 'center', width: '100%', maxWidth: '300px' }}>
                {caracteresCompostos.length === 0 ? (
                   <span style={{ fontSize: '12px', color: 'var(--cor-texto-suave)', marginTop: '10px' }}>
                     {t('Nenhum caractere encontrado.')}
                   </span>
                ) : (
                  caracteresCompostos.map((hz, idx) => (
                    <div
                      key={idx}
                      className="scan-btn"
                      style={{ fontSize: '20px', padding: '4px 12px', fontFamily: 'var(--fonte-hanzi)', cursor: 'pointer' }}
                      onClick={() => {
                        fecharModos();
                        AoClicarNoCaractereDecomposto(hz);
                      }}
                    >
                      {hz}
                    </div>
                  ))
                )}
              </div>
            </div>
          ) : modoTreinoGuiado ? (
            <div style={{ width: '100%', display: 'flex', flexDirection: 'column', alignItems: 'center' }}>
              {/* O Pinyin fica em cima se concluído */}
              {treinoConcluidoMsg && (
                <div style={{ display: 'grid', gridTemplateColumns: '40px 1fr 40px', alignItems: 'center', gap: '15px', width: '100%', maxWidth: '300px', marginBottom: '15px', animation: 'fadeIn 0.5s ease-in-out' }}>
                  <button 
                    onClick={tocarAudioPratica}
                    style={{
                      background: 'var(--cor-fundo-secundario)', border: '1px solid var(--cor-borda)', 
                      borderRadius: '50%', width: '40px', height: '40px', cursor: 'pointer', fontSize: '20px', 
                      display: 'flex', alignItems: 'center', justifyContent: 'center', margin: '0 auto',
                      opacity: audioTocando ? 0.6 : 1, transition: 'background-color 0.2s'
                    }}
                    title={t("Ouvir Pronúncia")}
                    onMouseOver={e => e.currentTarget.style.backgroundColor = 'var(--cor-borda)'}
                    onMouseOut={e => e.currentTarget.style.backgroundColor = 'var(--cor-fundo-secundario)'}
                  >
                    🔊
                  </button>
                  <div style={{ fontSize: '28px', color: 'var(--cor-pinyin)', fontWeight: 'bold', textAlign: 'center' }}>
                    {cartaoSelecionado.pinyin || cartaoSelecionado.Pinyin}
                  </div>
                  <div />
                </div>
              )}

              {/* O CanvasDesenho permanece sempre visível no modo guiado */}
              {!treinoConcluidoMsg && (
                <div style={{ marginBottom: '16px', color: 'var(--cor-texto-suave)', fontSize: '14px', textAlign: 'center' }}>
                  {t('O caractere desaparecerá rapidamente. Desenhe-o de memória.')}<br/>{t('(Ao errar, a dica será mostrada automaticamente).')}
                </div>
              )}
              
              <CanvasDesenho 
                hanzi={hanziAtual}
                modoMemoria={true}
                fadeoutAutomatico={true}
                mostrarDicaAposErros={1}
                apenasTreino={true}
                aoConcluir={() => {
                  setTreinoConcluidoMsg('sim');
                  if (lerPinyinAoCompletarDesenho !== false) {
                    tocarAudioPratica(); // Toca automaticamente se ativado
                  }
                }}
              />

              {/* O significado fica embaixo do desenho se concluído */}
              {treinoConcluidoMsg && (
                <div style={{ textAlign: 'center', marginTop: '20px', animation: 'fadeIn 0.5s ease-in-out' }}>
                  <div style={{ fontSize: '18px', color: 'var(--cor-texto-primario)', marginBottom: '20px' }}>
                    {significadoPrioritario}
                  </div>
                  <button 
                    className="scan-btn"
                    onClick={() => {
                      // Ao tentar novamente, recarregamos forçando o state de conclusão a sumir
                      // O CanvasDesenho sozinho não reseta externamente por prop exceto se trocarmos a chave, 
                      // mas vamos fechar e reabrir o modoTreinoGuiado rapidinho para recriar o Canvas
                      setModoTreinoGuiado(false);
                      setTimeout(() => setModoTreinoGuiado(true), 0);
                      setTreinoConcluidoMsg('');
                    }} 
                  >
                    {t('Tentar Novamente')}
                  </button>
                </div>
              )}
            </div>
          ) : modoEstatisticas ? (
            <div style={{ width: '100%', display: 'flex', flexDirection: 'column', alignItems: 'center' }}>
              <div style={{ fontSize: '16px', marginBottom: '10px', color: 'var(--cor-texto-suave)' }}>
                {t('Estatísticas de Aprendizado')}
              </div>
              <div style={{ fontFamily: 'var(--fonte-hanzi)', fontSize: '56px', fontWeight: 'bold', marginBottom: '10px' }}>
                {hanziAtual}
              </div>
              <div style={{ color: 'var(--cor-pinyin)', fontSize: '18px', marginBottom: '20px' }}>
                {cartaoSelecionado.pinyin || cartaoSelecionado.Pinyin}
              </div>

              <div style={{ width: '100%', maxWidth: '340px', display: 'flex', flexDirection: 'column', gap: '10px', marginBottom: '24px' }}>
                {estatisticas ? (
                  AREAS_APRENDIZADO.map(({ area, rotulo, Icone }) => {
                    const val = estatisticas[area] || 0;
                    const meta = 3;
                    const concluida = val >= meta;
                    const rotuloTraduzido = rotulo === 'Fonetica' ? t('Fonética') : rotulo === 'Pronuncia' ? t('Pronúncia') : t(rotulo);

                    return (
                      <div 
                        key={area} 
                        className={`revisao-area-pilula ${concluida ? 'concluida' : ''}`}
                        style={{ 
                          display: 'flex', 
                          alignItems: 'center', 
                          justifyContent: 'space-between',
                          padding: '10px 14px', 
                          borderRadius: '10px',
                          fontSize: '14px',
                          width: '100%',
                          boxSizing: 'border-box'
                        }}
                      >
                        <div style={{ display: 'flex', alignItems: 'center', gap: '10px' }}>
                          <Icone tamanho={18} />
                          <span style={{ fontWeight: 'bold', color: 'var(--cor-texto-primario)' }}>
                            {rotuloTraduzido}
                          </span>
                        </div>
                        <div style={{ display: 'flex', alignItems: 'center', gap: '10px' }}>
                          <span className="revisao-area-pontinhos">
                            {obterTiposBolinhas(val).map((tipo, i) => (
                              <span 
                                key={i} 
                                className={`revisao-area-pontinho ${tipo === 'vazia' ? '' : `cheio ${tipo}`}`}
                                style={{ width: '9px', height: '9px' }}
                              />
                            ))}
                          </span>
                          <span style={{ 
                            fontSize: '12px', 
                            fontWeight: 'bold',
                            color: concluida ? 'var(--cor-sucesso)' : 'var(--cor-texto-suave)',
                            minWidth: '55px',
                            textAlign: 'right'
                          }}>
                            {concluida ? (
                              <span style={{ display: 'inline-flex', alignItems: 'center', gap: '3px' }}>
                                <IconeCheck tamanho={14} /> {t('Concluído')}
                              </span>
                            ) : (
                              `${val} / ${meta}`
                            )}
                          </span>
                        </div>
                      </div>
                    );
                  })
                ) : (
                  <div style={{ textAlign: 'center', color: 'var(--cor-texto-suave)' }}>{t('Carregando estatísticas...')}</div>
                )}
              </div>
            </div>
          ) : (
            <>
              {imagemModalBase64 && (
                <div style={{border: '1px solid var(--cor-borda)', padding: '4px', borderRadius: '4px', backgroundColor: 'var(--cor-fundo-cartao)'}}>
                  <img src={"data:image/png;base64," + imagemModalBase64} alt="Recorte" style={{maxWidth: '100%', maxHeight: '150px'}} />
                </div>
              )}

              <div style={{textAlign: 'center', position: 'relative', width: '100%'}}>
                <div style={{
                  position: 'absolute',
                  top: '10px',
                  left: '10px',
                  display: 'flex',
                  alignItems: 'center',
                  gap: '6px',
                  color: 'var(--cor-texto-suave)',
                  fontSize: '14px',
                  fontWeight: 600,
                  userSelect: 'none'
                }} title={t('Vista em {vezes} scans de OCR', { vezes: cartaoSelecionado.vezesVistaOcr || 0 })}>
                  <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5" strokeLinecap="round" strokeLinejoin="round">
                    <path d="M1 12s4-8 11-8 11 8 11 8-4 8-11 8-11-8-11-8z" />
                    <circle cx="12" cy="12" r="3" />
                  </svg>
                  <span>{cartaoSelecionado.vezesVistaOcr || 0}</span>
                </div>

                {!isMultiChar && (
                  <>
                    <button 
                      style={{
                        position: 'absolute', top: '10px', right: '10px',
                        background: 'none', border: 'none', cursor: 'pointer', fontSize: '24px', 
                        opacity: '0.6', transition: 'opacity 0.2s', zIndex: 10
                      }}
                      title={t("Pesquisar por Desenho (Livre)")}
                      onMouseOver={e => e.currentTarget.style.opacity = '1'}
                      onMouseOut={e => e.currentTarget.style.opacity = '0.6'}
                      onClick={() => setModoDesenhoLivre(true)}
                    >
                      <IconPencil />
                    </button>
                    <button 
                      style={{
                        position: 'absolute', top: '78px', right: '10px',
                        background: 'none', border: 'none', cursor: 'pointer', fontSize: '24px', 
                        opacity: '0.6', transition: 'opacity 0.2s', zIndex: 10
                      }}
                      title={t("Explorar Composição")}
                      onMouseOver={e => e.currentTarget.style.opacity = '1'}
                      onMouseOut={e => e.currentTarget.style.opacity = '0.6'}
                      onClick={() => carregarCompostos()}
                    >
                      <IconList />
                    </button>
                  </>
                )}
                <button 
                  style={{
                    position: 'absolute', 
                    top: isMultiChar ? '10px' : '44px', 
                    right: '10px',
                    background: 'none', border: 'none', cursor: 'pointer', fontSize: '24px', 
                    opacity: '0.6', transition: 'opacity 0.2s', zIndex: 10
                  }}
                  title={t("Buscar Palavras Compostas")}
                  onMouseOver={e => e.currentTarget.style.opacity = '1'}
                  onMouseOut={e => e.currentTarget.style.opacity = '0.6'}
                  onClick={() => aoBuscarPalavrasCompostas(hanziAtual)}
                >
                  <IconSearch />
                </button>
                <button 
                  style={{
                    position: 'absolute', 
                    top: isMultiChar ? '44px' : '112px', 
                    right: '10px',
                    background: 'none', border: 'none', cursor: 'pointer', fontSize: '24px', 
                    opacity: '0.6', transition: 'opacity 0.2s', zIndex: 10
                  }}
                  title={t("Visualizar Estatísticas")}
                  onMouseOver={e => e.currentTarget.style.opacity = '1'}
                  onMouseOut={e => e.currentTarget.style.opacity = '0.6'}
                  onClick={() => setModoEstatisticas(true)}
                >
                  <IconStats />
                </button>
                <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'center', gap: '15px' }}>
                  <button 
                    onClick={tocarAudioPratica}
                    style={{
                      background: 'var(--cor-fundo-secundario)', border: '1px solid var(--cor-borda)', 
                      borderRadius: '50%', width: '32px', height: '32px', cursor: 'pointer', fontSize: '16px', 
                      display: 'flex', alignItems: 'center', justifyContent: 'center',
                      opacity: audioTocando ? 0.6 : 1, transition: 'background-color 0.2s'
                    }}
                    title={t("Ouvir Pronúncia")}
                    onMouseOver={e => e.currentTarget.style.backgroundColor = 'var(--cor-borda)'}
                    onMouseOut={e => e.currentTarget.style.backgroundColor = 'var(--cor-fundo-secundario)'}
                  >
                    🔊
                  </button>
                  <div style={{color: 'var(--cor-pinyin)', fontSize: '24px'}}>{pinyinModalExibicao || cartaoSelecionado.pinyin || cartaoSelecionado.Pinyin}</div>
                  <div style={{ width: '32px' }} />
                </div>
                <div style={{fontFamily: 'var(--fonte-hanzi)', fontSize: '64px', fontWeight: 'bold', lineHeight: '1.2', margin: '10px 0'}}>
                  {hanziAtual}
                </div>
                {mostrarMaisSignificadosModal && leiturasModal && leiturasModal.length > 1 ? (
                  <div style={{ width: '100%', marginTop: '10px', display: 'flex', flexDirection: 'column', alignItems: 'center' }}>
                    <div className="lista-leituras" style={{ maxWidth: '400px' }}>
                      {(() => {
                        const obterPrioridadeTipoModal = (item: dicionario.EntradaDicionario): number => {
                          const tipo = ((item as any).Tipo || (item as any).tipo || '').toLowerCase();
                          if (tipo === 'sobrenome' || tipo.includes('surname')) return 1;
                          if (tipo === 'variante' || tipo.includes('variant')) return 2;

                          const significadosStr = item.Significados ? item.Significados.join(' ').toLowerCase() : '';
                          if (significadosStr.includes('sobrenome') || significadosStr.includes('surname')) return 1;
                          if (significadosStr.startsWith('variante de') || significadosStr.startsWith('variant of')) return 2;

                          return 0;
                        };

                        const leiturasOrdenadas = [...leiturasModal].sort(
                          (a, b) => obterPrioridadeTipoModal(a) - obterPrioridadeTipoModal(b)
                        );
                        const pinyinsDiferentes = leiturasOrdenadas.some(
                          item => item.Pinyin && item.Pinyin !== leiturasOrdenadas[0].Pinyin
                        );
                        return leiturasOrdenadas.map((item, idx) => (
                          <div key={idx} className="item-leitura" style={{ fontSize: '14px', padding: '6px 10px' }}>
                            {pinyinsDiferentes && <span className="pinyin-leitura" style={{ fontSize: '14px' }}>[{item.Pinyin}] </span>}
                            <span className="texto-leitura" style={{ fontSize: '14px' }}>
                              <TextoComHanzis 
                                texto={item.Significados ? item.Significados.join(', ') : ''} 
                                aoClicarNoCaractere={AoClicarNoCaractereDecomposto} 
                                setPopupInfo={setInformacoesDoPopup} 
                              />
                            </span>
                          </div>
                        ));
                      })()}
                    </div>
                    <button 
                      className="botao-expandir-leituras" 
                      style={{ fontSize: '12px', marginTop: '6px' }}
                      onClick={() => setMostrarMaisSignificadosModal(false)}
                    >
                      {t('Mostrar menos...')}
                    </button>
                  </div>
                ) : (
                  <div style={{ color: 'var(--cor-texto-suave)', fontSize: '18px', marginTop: '10px' }}>
                    <TextoComHanzis texto={significadoPrioritario} aoClicarNoCaractere={AoClicarNoCaractereDecomposto} setPopupInfo={setInformacoesDoPopup} />
                    {leiturasModal && leiturasModal.length > 1 && (
                      <div>
                        <button 
                          className="botao-expandir-leituras" 
                          style={{ fontSize: '12px', marginTop: '6px' }}
                          onClick={() => setMostrarMaisSignificadosModal(true)}
                        >
                          {t('Mostrar mais...')}
                        </button>
                      </div>
                    )}
                  </div>
                )}
                {infoExpansao && (
                  <div style={{ display: 'flex', justifyContent: 'center', gap: '8px', marginTop: '14px', flexWrap: 'wrap' }}>
                    <span 
                      className="badge-extra clickable" 
                      title={t('Posição no ranking das palavras mais usadas')}
                      onClick={() => aoBuscarPorRanking(hanziAtual)}
                      onMouseEnter={() => setHoverNoRanking(true)}
                      onMouseLeave={() => setHoverNoRanking(false)}
                    >
                      <IconRanking size={14} />
                      <span>{t('Ranking')}: {(() => {
                        const posicao = infoExpansao.posicaoRanking;
                        let textoPadrao = '+10000';
                        if (posicao === 0) textoPadrao = '+75000';
                        else if (posicao <= 1000) textoPadrao = `${posicao}`;
                        else if (posicao <= 10000) textoPadrao = '+1000';

                        const textoReal = posicao > 0 ? `${posicao}` : textoPadrao;

                        return (
                          <strong 
                            style={{ 
                              color: 'var(--cor-destaque)', 
                              display: 'inline-grid', 
                              gridTemplateColumns: '1fr', 
                              alignItems: 'center', 
                              justifyItems: 'center',
                              verticalAlign: 'bottom'
                            }}
                          >
                            <span style={{ gridArea: '1/1', visibility: hoverNoRanking ? 'hidden' : 'visible', textAlign: 'center' }}>
                              {textoPadrao}
                            </span>
                            <span style={{ gridArea: '1/1', visibility: hoverNoRanking ? 'visible' : 'hidden', textAlign: 'center' }}>
                              {textoReal}
                            </span>
                          </strong>
                        );
                      })()}</span>
                    </span>
                    {infoExpansao.nivelHSK && infoExpansao.tipoHanzi !== 'Tradicional' && (
                      <span 
                        className="badge-extra clickable"
                        title={t('Nível HSK 3.0 da palavra')}
                        onClick={() => aoBuscarPorHSK ? aoBuscarPorHSK(hanziAtual) : (aoBuscarPorRanking && aoBuscarPorRanking(hanziAtual))}
                      >
                        <IconHSK size={14} />
                        <span>HSK: <strong style={{ color: 'var(--cor-destaque)' }}>{infoExpansao.nivelHSK}</strong></span>
                      </span>
                    )}
                    {infoExpansao.alternativa && (
                      <span 
                        className="badge-extra clickable"
                        onClick={() => AoClicarNoCaractereDecomposto(infoExpansao.alternativa)}
                      >
                        <IconRefresh size={14} />
                        <span>{
                          infoExpansao.tipoHanzi === 'Tradicional' ? t('Simplificado') : 
                          infoExpansao.tipoHanzi === 'Simplificado' ? t('Tradicional') : 
                          t('Alternativo')
                        }: <strong style={{ color: 'var(--cor-destaque)', fontFamily: 'var(--fonte-hanzi)', fontSize: '14px' }}>{infoExpansao.alternativa}</strong></span>
                      </span>
                    )}
                  </div>
                )}
              </div>

              {dadosDecomposicao && (
                <div style={{width: '100%', borderTop: '1px solid var(--cor-borda)', paddingTop: '20px'}}>
                  <h3 style={{fontSize: '16px', color: 'var(--cor-texto-primario)', marginBottom: '16px'}}>{t('Decomposição')}</h3>
                  
                  {dadosDecomposicao.type === 'single' ? (
                    <div style={{backgroundColor: 'var(--cor-fundo-cartao)', padding: '16px', borderRadius: '8px'}}>
                      {dadosDecomposicao.data?.pinyin && dadosDecomposicao.data.pinyin.length > 0 && (
                        <div style={{fontSize: '14px', marginBottom: '8px'}}>
                          <strong>{t('Pinyin (MakeMeAHanzi):')}</strong> {dadosDecomposicao.data.pinyin.join(', ')}
                        </div>
                      )}
                      {dadosDecomposicao.data?.definition && (
                        <div style={{fontSize: '14px', marginBottom: '8px'}}>
                          <strong>{t('Definição:')}</strong> <TextoComHanzis texto={dadosDecomposicao.data.definition} aoClicarNoCaractere={AoClicarNoCaractereDecomposto} setPopupInfo={setInformacoesDoPopup} />
                        </div>
                      )}
                      <div style={{fontSize: '14px'}}>
                        <strong>{t('Radical:')}</strong> <TextoComHanzis texto={dadosDecomposicao.data?.radical || '-'} aoClicarNoCaractere={AoClicarNoCaractereDecomposto} setPopupInfo={setInformacoesDoPopup} />
                      </div>
                      {dadosDecomposicao.data?.abreviacoes && dadosDecomposicao.data.abreviacoes.length > 0 && (
                        <div style={{fontSize: '14px', marginTop: '8px'}}>
                          <strong>{t('Abreviações visuais:')}</strong>
                          <span style={{display: 'inline-flex', gap: '6px', marginLeft: '8px', alignItems: 'center'}}>
                            {dadosDecomposicao.data.abreviacoes.map((a: string, i: number) => (
                              <span
                                key={i}
                                style={{
                                  fontFamily: 'var(--fonte-hanzi)',
                                  fontSize: '22px',
                                  backgroundColor: 'var(--cor-fundo-entrada)',
                                  border: '1px solid var(--cor-borda)',
                                  borderRadius: '6px',
                                  padding: '2px 8px',
                                }}
                              >{a}</span>
                            ))}
                          </span>
                        </div>
                      )}
                      <div style={{fontSize: '14px', marginTop: '8px'}}>
                        <strong>{t('Estrutura:')}</strong> {(() => {
                          const raw = dadosDecomposicao.data?.decomposition || '';
                          if (!raw) return '-';

                          const mapaIdc: Record<string, string> = {
                            '⿰': 'Esquerda–Direita',
                            '⿱': 'Cima–Baixo',
                            '⿲': 'Esquerda–Centro–Direita',
                            '⿳': 'Cima–Centro–Baixo',
                            '⿴': 'Cercado',
                            '⿵': 'Aberto embaixo',
                            '⿶': 'Aberto em cima',
                            '⿷': 'Aberto à direita',
                            '⿸': 'Cobertura superior-esquerda',
                            '⿹': 'Cobertura superior-direita',
                            '⿺': 'Cobertura inferior-esquerda',
                            '⿻': 'Sobreposto',
                          };

                          const chars: string[] = Array.from(raw);
                          const estrutura = mapaIdc[chars[0]] || null;
                          
                          const resultado = estrutura
                            ? `${chars[0]} ${t(estrutura)}`
                            : raw;
                          return <TextoComHanzis texto={resultado} aoClicarNoCaractere={AoClicarNoCaractereDecomposto} setPopupInfo={setInformacoesDoPopup} />;
                        })()}
                      </div>
                      {dadosDecomposicao.data?.etymology && Object.keys(dadosDecomposicao.data.etymology).length > 0 && (
                        <div style={{fontSize: '14px', marginTop: '8px'}}>
                          <strong>{t('Etimologia:')}</strong> {(() => {
                            const e = dadosDecomposicao.data.etymology;
                            const tMap: Record<string, string> = {
                              'pictophonetic': t('Pictofonético'),
                              'ideographic': t('Ideográfico'),
                              'pictographic': t('Pictográfico'),
                            };
                            let txt = tMap[e.type] || e.type || t('Desconhecida');
                            if (e.hint) txt += ` — ${e.hint}`;
                            if (e.semantic) txt += ` (${t('Semântica')}: ${e.semantic})`;
                            if (e.phonetic) txt += ` (${t('Fonética')}: ${e.phonetic})`;
                            return <TextoComHanzis texto={txt} aoClicarNoCaractere={AoClicarNoCaractereDecomposto} setPopupInfo={setInformacoesDoPopup} />;
                          })()}
                        </div>
                      )}
                      <div style={{marginTop: '14px'}}>
                        <ArvoreGenealogicaDecomposicao
                          hanziAlvo={hanziAtual}
                          pinyinAlvo={pinyinModalExibicao}
                          significadoAlvo={significadoPrioritario}
                          aoClicarNoCaractere={AoClicarNoCaractereDecomposto}
                          setPopupInfo={setInformacoesDoPopup}
                        />
                      </div>
                    </div>
                  ) : (
                    <ArvoreGenealogicaDecomposicao
                      hanziAlvo={hanziAtual}
                      pinyinAlvo={pinyinModalExibicao}
                      significadoAlvo={significadoPrioritario}
                      aoClicarNoCaractere={AoClicarNoCaractereDecomposto}
                      setPopupInfo={setInformacoesDoPopup}
                    />
                  )}
                </div>
              )}
            </>
          )}
          
          {/* Botões de Ação Principais */}
          <div style={{display: 'flex', gap: '15px', marginTop: '15px', width: '100%', justifyContent: 'center', flexWrap: 'wrap'}}>
            {(!isMultiChar || qualquerModoAtivo) && (
              <button 
                onClick={() => {
                  if (qualquerModoAtivo) {
                    fecharModos();
                  } else {
                    setModoTreinoGuiado(true);
                  }
                }}
                style={{
                  padding: '10px 20px', 
                  borderRadius: '8px', 
                  border: '1px solid var(--cor-destaque)', 
                  cursor: 'pointer', 
                  fontWeight: 'bold',
                  backgroundColor: 'transparent',
                  color: 'var(--cor-destaque)',
                  transition: 'opacity 0.2s',
                  flex: '1',
                  maxWidth: '250px'
                }}
                onMouseOver={(e) => e.currentTarget.style.opacity = '0.7'}
                onMouseOut={(e) => e.currentTarget.style.opacity = '1'}
              >
                {qualquerModoAtivo ? t("Voltar aos Detalhes") : t("Praticar Escrita Guiada")}
              </button>
            )}

            {!qualquerModoAtivo && (
              <>
                <button 
                  onClick={onToggleEstudo}
                  style={{
                    padding: '10px 20px', 
                    borderRadius: '8px', 
                    border: 'none', 
                    cursor: 'pointer', 
                    fontWeight: 'bold',
                    backgroundColor: isEstudando ? '#f44336' : 'var(--cor-destaque)',
                    color: 'white',
                    transition: 'opacity 0.2s',
                    flex: '1',
                    maxWidth: '250px'
                  }}
                  onMouseOver={(e) => e.currentTarget.style.opacity = '0.8'}
                  onMouseOut={(e) => e.currentTarget.style.opacity = '1'}
                >
                  {isEstudando ? t("Remover de Estudando") : t("Adicionar a Estudando")}
                </button>

                <button 
                  onClick={onToggleAprendida}
                  style={{
                    padding: '10px 20px', 
                    borderRadius: '8px', 
                    border: 'none', 
                    cursor: 'pointer', 
                    fontWeight: 'bold',
                    backgroundColor: isAprendida ? '#f44336' : '#4CAF50',
                    color: 'white',
                    transition: 'opacity 0.2s',
                    flex: '1',
                    maxWidth: '250px'
                  }}
                  onMouseOver={(e) => e.currentTarget.style.opacity = '0.8'}
                  onMouseOut={(e) => e.currentTarget.style.opacity = '1'}
                >
                  {isAprendida ? t("Remover de Aprendidas") : t("Adicionar a Aprendidas")}
                </button>
              </>
            )}
          </div>
        </div>
      </div>

      {informacoesDoPopup && (
        <div
          className="popup-revisao"
          style={{
            position: 'fixed',
            left: Math.max(10, informacoesDoPopup.x - 120),
            top: informacoesDoPopup.y - 15,
            transform: 'translateY(-100%)',
            width: '240px',
            backgroundColor: 'var(--cor-fundo-painel, #171717)',
            backgroundImage: 'linear-gradient(180deg, rgba(38, 38, 38, 0.95) 0%, rgba(23, 23, 23, 0.98) 100%)',
            border: '1.5px solid var(--cor-destaque, #6366f1)',
            borderRadius: '12px',
            padding: '14px',
            boxShadow: '0 12px 32px rgba(0, 0, 0, 0.85), 0 0 0 1px rgba(99, 102, 241, 0.25)',
            backdropFilter: 'blur(16px)',
            WebkitBackdropFilter: 'blur(16px)',
            pointerEvents: 'none',
            zIndex: 9999,
            display: 'flex',
            flexDirection: 'column',
            fontFamily: 'var(--fonte-interface, system-ui, sans-serif)'
          }}
        >
          <div style={{ textAlign: 'center', marginBottom: '8px' }}>
            <div style={{ fontSize: '18px', color: 'var(--cor-pinyin)', marginBottom: '4px' }}>
              {informacoesDoPopup.pinyin}
            </div>
            <div style={{ fontSize: '36px', fontFamily: 'var(--fonte-hanzi)', color: 'var(--cor-texto-primario)', lineHeight: '1.2' }}>
              {informacoesDoPopup.hanzi}
            </div>
          </div>
          <div style={{ fontSize: '14px', color: 'var(--cor-texto-suave)', lineHeight: '1.4', borderTop: '1px solid var(--cor-borda)', paddingTop: '8px', textAlign: 'center' }}>
            {informacoesDoPopup.significados || t('Sem definição.')}
          </div>
        </div>
      )}
    </div>
  );
}

interface TextoComHanzisProps {
  texto: string;
  aoClicarNoCaractere: (char: string) => void;
  setPopupInfo: (info: any) => void;
}

export function TextoComHanzis({ texto, aoClicarNoCaractere, setPopupInfo }: TextoComHanzisProps) {
  const [tokens, setTokens] = useState<string[]>([]);

  useEffect(() => {
    if (!texto) {
      setTokens([]);
      return;
    }
    SegmentarTexto(texto)
      .then(resultado => {
        setTokens(resultado || []);
      })
      .catch(erro => {
        console.error("Erro ao segmentar texto:", erro);
        setTokens([texto]);
      });
  }, [texto]);

  if (!texto) {
    return null;
  }

  return (
    <>
      {tokens.map((token, indice) => {
        if (!/[\u4e00-\u9fff]/.test(token)) {
          return <span key={indice}>{token}</span>;
        }

        return (
          <span
            key={indice}
            style={{
              cursor: 'pointer',
              fontFamily: 'var(--fonte-hanzi)',
              transition: 'color 0.2s',
              fontWeight: 'bold'
            }}
            onMouseEnter={(evento) => {
              evento.currentTarget.style.color = 'var(--cor-destaque)';
              const retangulo = evento.currentTarget.getBoundingClientRect();
              LookupWord(token).then(entradas => {
                if (!entradas || entradas.length === 0) {
                  return;
                }
                const verbete = entradas[0];
                setPopupInfo({
                  hanzi: token,
                  pinyin: verbete.Pinyin || '',
                  significados: verbete.Significados ? verbete.Significados.join(', ') : '',
                  x: retangulo.left + retangulo.width / 2,
                  y: retangulo.top
                });
              });
            }}
            onMouseLeave={(evento) => {
              evento.currentTarget.style.color = '';
              setPopupInfo(null);
            }}
            onClick={() => {
              setPopupInfo(null);
              aoClicarNoCaractere(token);
            }}
          >
            {token}
          </span>
        );
      })}
    </>
  );
}
