// ----- Seção: Dicionário -----
import { useState, useEffect } from 'react';
import './dicionario.css';
import { CanvasHanziLookup } from './CanvasHanziLookup';
import { CanvasDesenho } from '../comum/CanvasDesenho';
import { BuscarCaracteresCompostosPor, ObterEstatisticasPalavra } from '../../wailsjs/go/main/App';
import { TocarAudio } from '../comum/TocarAudio';

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
    aoBuscarPalavrasCompostas
  } = props;

  const [modoDesenhoLivre, setModoDesenhoLivre] = useState(false);
  const [modoTreinoGuiado, setModoTreinoGuiado] = useState(false);
  const [modoCompostos, setModoCompostos] = useState(false);
  const [modoEstatisticas, setModoEstatisticas] = useState(false);
  const [estatisticas, setEstatisticas] = useState<Record<string, number> | null>(null);
  const [caracteresCompostos, setCaracteresCompostos] = useState<string[]>([]);
  const [sugestoesPratica, setSugestoesPratica] = useState<string[]>([]);
  const [treinoConcluidoMsg, setTreinoConcluidoMsg] = useState('');
  const [audioTocando, setAudioTocando] = useState(false);
  const hanziAtual = cartaoSelecionado ? (cartaoSelecionado.hanzi || cartaoSelecionado.Hanzi) : '';
  const isMultiChar = hanziAtual.length > 1;

  // Resetar modos de prática sempre que o modal for reaberto ou mudar de caractere
  useEffect(() => {
    setModoDesenhoLivre(false);
    setModoTreinoGuiado(false);
    setModoCompostos(false);
    setModoEstatisticas(false);
    setEstatisticas(null);
    setCaracteresCompostos([]);
    setSugestoesPratica([]);
    setTreinoConcluidoMsg('');
    setAudioTocando(false);
  }, [cartaoSelecionado?.hanzi, cartaoSelecionado?.Hanzi]);

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
            <h2>Detalhes da Captura</h2>
            <button className="modal-close" onClick={() => setCartaoSelecionado(null)}>×</button>
          </div>
          
          <div className="modal-body" style={{ display: 'flex', flexDirection: 'column', alignItems: 'center', gap: '20px' }}>
            <div style={{ border: '1px solid var(--cor-borda)', padding: '4px', borderRadius: '4px', backgroundColor: 'var(--cor-fundo-cartao)', width: '100%', display: 'flex', justifyContent: 'center' }}>
              <img 
                src={imagemModalBase64 ? "data:image/png;base64," + imagemModalBase64 : ''} 
                alt="Última captura OCR" 
                style={{ maxWidth: '100%', maxHeight: '40vh', objectFit: 'contain' }} 
              />
            </div>

            <div style={{ textAlign: 'center', position: 'relative', width: '100%' }}>
              <div style={{ fontSize: '28px', color: 'var(--cor-pinyin)', fontWeight: 'bold', marginBottom: '10px' }}>
                Visualizar Print Escaneado
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
                Status: Processado com sucesso
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
            {modoDesenhoLivre ? "Desenho Livre (Busca)" : 
             modoTreinoGuiado ? "Treino Guiado de Caligrafia" : 
             modoCompostos ? "Caracteres Compostos" :
             "Detalhes"}
          </h2>
          <button className="modal-close" onClick={() => setCartaoSelecionado(null)}>×</button>
        </div>
        
        <div className="modal-body" style={{display: 'flex', flexDirection: 'column', alignItems: 'center', gap: '20px'}}>
          
          {modoDesenhoLivre ? (
            <div style={{ width: '100%', display: 'flex', flexDirection: 'column', alignItems: 'center' }}>
              <div style={{ marginBottom: '16px', color: 'var(--cor-texto-suave)', fontSize: '14px', textAlign: 'center' }}>
                Desenhe por cima do Hanzi para pesquisar caracteres estruturalmente semelhantes ou compostos por ele.
                <br />
                <span style={{ fontSize: '12px', opacity: 0.8 }}>⚠️ A ordem e a direção dos traços importa para o reconhecimento correto.</span>
              </div>
              <CanvasHanziLookup 
                onRecognize={(sugestoes) => setSugestoesPratica(sugestoes)}
                targetHanzi={hanziAtual}
                configuracoesApp={configuracoesApp}
              />
              
              <div style={{ display: 'flex', flexWrap: 'wrap', gap: '8px', marginTop: '12px', justifyContent: 'center', minHeight: '40px' }}>
                {sugestoesPratica.length === 0 ? (
                   <span style={{ fontSize: '12px', color: 'var(--cor-texto-suave)', marginTop: '10px' }}>
                     Nenhuma correspondência ainda.
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
                Caracteres que contêm <strong style={{ fontSize: '20px', color: 'var(--cor-texto-primario)' }}>{hanziAtual}</strong>
              </div>
              <div style={{ display: 'flex', flexWrap: 'wrap', gap: '8px', justifyContent: 'center', width: '100%', maxWidth: '300px' }}>
                {caracteresCompostos.length === 0 ? (
                   <span style={{ fontSize: '12px', color: 'var(--cor-texto-suave)', marginTop: '10px' }}>
                     Nenhum caractere encontrado.
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
                    title="Ouvir Pronúncia"
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
                  O caractere desaparecerá rapidamente. Desenhe-o de memória.<br/>(Ao errar, a dica será mostrada automaticamente).
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
                    Tentar Novamente
                  </button>
                </div>
              )}
            </div>
          ) : modoEstatisticas ? (
            <div style={{ width: '100%', display: 'flex', flexDirection: 'column', alignItems: 'center' }}>
              <div style={{ fontSize: '16px', marginBottom: '10px', color: 'var(--cor-texto-suave)' }}>
                Estatísticas de Aprendizado
              </div>
              <div style={{ fontFamily: 'var(--fonte-hanzi)', fontSize: '56px', fontWeight: 'bold', marginBottom: '10px' }}>
                {hanziAtual}
              </div>
              <div style={{ color: 'var(--cor-pinyin)', fontSize: '18px', marginBottom: '20px' }}>
                {cartaoSelecionado.pinyin || cartaoSelecionado.Pinyin}
              </div>

              <div style={{ width: '100%', maxWidth: '320px', display: 'flex', flexDirection: 'column', gap: '16px', marginBottom: '24px' }}>
                {estatisticas ? (
                  Object.entries(estatisticas).map(([cat, val]) => {
                    const rotulo = cat.charAt(0).toUpperCase() + cat.slice(1);
                    const progressoPercent = Math.min((val / 3) * 100, 100);
                    return (
                      <div key={cat} style={{ display: 'flex', flexDirection: 'column', gap: '6px' }}>
                        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', fontSize: '14px' }}>
                          <span style={{ fontWeight: 'bold', color: 'var(--cor-texto-primario)' }}>
                            {rotulo === 'Fonetica' ? 'Fonética' : rotulo === 'Pronuncia' ? 'Pronúncia' : rotulo}
                          </span>
                          <span style={{ color: val >= 3 ? 'var(--cor-sucesso)' : 'var(--cor-texto-suave)', fontWeight: 'bold', marginLeft: 'auto' }}>
                            {val >= 3 ? '✓ Concluído' : `${val} / 3`}
                          </span>
                        </div>
                        <div style={{ width: '100%', height: '8px', backgroundColor: 'var(--cor-borda)', borderRadius: '4px', overflow: 'hidden' }}>
                          <div 
                            style={{ 
                              height: '100%', 
                              width: `${progressoPercent}%`, 
                              backgroundColor: val >= 3 ? 'var(--cor-sucesso)' : 'var(--cor-destaque)',
                              transition: 'width 0.4s ease-out',
                              borderRadius: '4px'
                            }} 
                          />
                        </div>
                      </div>
                    );
                  })
                ) : (
                  <div style={{ textAlign: 'center', color: 'var(--cor-texto-suave)' }}>Carregando estatísticas...</div>
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
                {!isMultiChar && (
                  <>
                    <button 
                      style={{
                        position: 'absolute', top: '10px', right: '10px',
                        background: 'none', border: 'none', cursor: 'pointer', fontSize: '24px', 
                        opacity: '0.6', transition: 'opacity 0.2s', zIndex: 10
                      }}
                      title="Pesquisar por Desenho (Livre)"
                      onMouseOver={e => e.currentTarget.style.opacity = '1'}
                      onMouseOut={e => e.currentTarget.style.opacity = '0.6'}
                      onClick={() => setModoDesenhoLivre(true)}
                    >
                      <IconPencil />
                    </button>
                    <button 
                      style={{
                        position: 'absolute', top: '44px', right: '10px',
                        background: 'none', border: 'none', cursor: 'pointer', fontSize: '24px', 
                        opacity: '0.6', transition: 'opacity 0.2s', zIndex: 10
                      }}
                      title="Buscar Palavras Compostas"
                      onMouseOver={e => e.currentTarget.style.opacity = '1'}
                      onMouseOut={e => e.currentTarget.style.opacity = '0.6'}
                      onClick={() => aoBuscarPalavrasCompostas(hanziAtual)}
                    >
                      <IconSearch />
                    </button>
                    <button 
                      style={{
                        position: 'absolute', top: '78px', right: '10px',
                        background: 'none', border: 'none', cursor: 'pointer', fontSize: '24px', 
                        opacity: '0.6', transition: 'opacity 0.2s', zIndex: 10
                      }}
                      title="Explorar Composição"
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
                    top: isMultiChar ? '10px' : '112px', 
                    right: '10px',
                    background: 'none', border: 'none', cursor: 'pointer', fontSize: '24px', 
                    opacity: '0.6', transition: 'opacity 0.2s', zIndex: 10
                  }}
                  title="Visualizar Estatísticas"
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
                    title="Ouvir Pronúncia"
                    onMouseOver={e => e.currentTarget.style.backgroundColor = 'var(--cor-borda)'}
                    onMouseOut={e => e.currentTarget.style.backgroundColor = 'var(--cor-fundo-secundario)'}
                  >
                    🔊
                  </button>
                  <div style={{color: 'var(--cor-pinyin)', fontSize: '24px'}}>{cartaoSelecionado.pinyin || cartaoSelecionado.Pinyin}</div>
                  <div style={{ width: '32px' }} />
                </div>
                <div style={{fontFamily: 'var(--fonte-hanzi)', fontSize: '64px', fontWeight: 'bold', lineHeight: '1.2', margin: '10px 0'}}>
                  {hanziAtual}
                </div>
                <div style={{color: 'var(--cor-texto-suave)', fontSize: '18px', marginTop: '10px'}}>{significadoPrioritario}</div>
              </div>

              {dadosDecomposicao && (
                <div style={{width: '100%', borderTop: '1px solid var(--cor-borda)', paddingTop: '20px'}}>
                  <h3 style={{fontSize: '16px', color: 'var(--cor-texto-primario)', marginBottom: '16px'}}>Decomposição</h3>
                  
                  {dadosDecomposicao.type === 'single' ? (
                    <div style={{backgroundColor: 'var(--cor-fundo-cartao)', padding: '16px', borderRadius: '8px'}}>
                      {dadosDecomposicao.data?.pinyin && dadosDecomposicao.data.pinyin.length > 0 && (
                        <div style={{fontSize: '14px', marginBottom: '8px'}}>
                          <strong>Pinyin (MakeMeAHanzi):</strong> {dadosDecomposicao.data.pinyin.join(', ')}
                        </div>
                      )}
                      {dadosDecomposicao.data?.definition && (
                        <div style={{fontSize: '14px', marginBottom: '8px'}}>
                          <strong>Definição:</strong> {dadosDecomposicao.data.definition}
                        </div>
                      )}
                      <div style={{fontSize: '14px'}}>
                        <strong>Radical:</strong> {dadosDecomposicao.data?.radical || '-'}
                      </div>
                      {dadosDecomposicao.data?.abreviacoes && dadosDecomposicao.data.abreviacoes.length > 0 && (
                        <div style={{fontSize: '14px', marginTop: '8px'}}>
                          <strong>Abreviações visuais:</strong>
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
                        <strong>Estrutura:</strong> {(() => {
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
                          
                          return estrutura
                            ? `${chars[0]} ${estrutura}`
                            : raw;
                        })()}
                      </div>
                      {dadosDecomposicao.data?.etymology && Object.keys(dadosDecomposicao.data.etymology).length > 0 && (
                        <div style={{fontSize: '14px', marginTop: '8px'}}>
                          <strong>Etimologia:</strong> {(() => {
                            const e = dadosDecomposicao.data.etymology;
                            const tMap: Record<string, string> = {
                              'pictophonetic': 'Pictofonético',
                              'ideographic': 'Ideográfico',
                              'pictographic': 'Pictográfico',
                            };
                            let txt = tMap[e.type] || e.type || 'Desconhecida';
                            if (e.hint) txt += ` — ${e.hint}`;
                            if (e.semantic) txt += ` (Semântica: ${e.semantic})`;
                            if (e.phonetic) txt += ` (Fonética: ${e.phonetic})`;
                            return txt;
                          })()}
                        </div>
                      )}
                      {(() => {
                        const raw = dadosDecomposicao.data?.decomposition || '';
                        if (!raw) return null;

                        const mapaIdc: Record<string, string> = {
                          '⿰': '', '⿱': '', '⿲': '', '⿳': '', '⿴': '', '⿵': '',
                          '⿶': '', '⿷': '', '⿸': '', '⿹': '', '⿺': '', '⿻': '',
                        };

                        const componentes: string[] = (Array.from(raw) as string[]).filter((c: string) => !(c in mapaIdc) && c !== '?');

                        if (componentes.length === 0) return null;

                        return (
                          <div style={{marginTop: '12px'}}>
                            <strong style={{fontSize: '14px'}}>Componentes:</strong>
                            <div style={{display: 'flex', gap: '8px', marginTop: '8px', flexWrap: 'wrap'}}>
                              {componentes.map((comp, idx) => (
                                <div
                                  key={idx}
                                  style={{
                                    fontSize: '28px',
                                    fontFamily: 'var(--fonte-hanzi)',
                                    backgroundColor: 'var(--cor-fundo-entrada)',
                                    border: '1px solid var(--cor-borda)',
                                    borderRadius: '8px',
                                    padding: '8px 14px',
                                    cursor: 'pointer',
                                    transition: 'all 0.2s',
                                  }}
                                  onClick={() => AoClicarNoCaractereDecomposto(comp)}
                                >
                                  {comp}
                                </div>
                              ))}
                            </div>
                          </div>
                        );
                      })()}
                    </div>
                  ) : (
                    <div style={{display: 'grid', gridTemplateColumns: 'repeat(auto-fill, minmax(140px, 1fr))', gap: '10px'}}>
                      {dadosDecomposicao.data.map((d: any, idx: number) => (
                        <div 
                           key={idx} 
                           className="card" 
                           style={{width: '100%', height: 'auto', padding: '12px', minHeight: '120px'}}
                           onClick={() => AoClicarNoCaractereDecomposto(d.char)}
                        >
                           <div style={{fontSize: '28px', fontFamily: 'var(--fonte-hanzi)'}}>{d.char}</div>
                           {d.res && d.res.length > 0 ? (
                             <>
                               <div style={{fontSize: '12px', color: 'var(--cor-pinyin)', marginTop: '8px'}}>{d.res[0].Pinyin}</div>
                               <div style={{fontSize: '11px', color: 'var(--cor-texto-suave)', marginTop: '4px', overflow: 'hidden', textOverflow: 'ellipsis', display: '-webkit-box', WebkitLineClamp: 3, WebkitBoxOrient: 'vertical'}}>
                                 {d.res[0].Significados.join(', ')}
                               </div>
                             </>
                           ) : (
                             <div style={{fontSize: '11px', color: 'var(--cor-texto-suave)', marginTop: '4px'}}>Desconhecido</div>
                           )}
                        </div>
                      ))}
                    </div>
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
                {qualquerModoAtivo ? "Voltar aos Detalhes" : "Praticar Escrita Guiada"}
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
                  {isEstudando ? "Remover de Estudando" : "Adicionar a Estudando"}
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
                  {isAprendida ? "Remover de Aprendidas" : "Adicionar a Aprendidas"}
                </button>
              </>
            )}
          </div>
        </div>
      </div>
    </div>
  );
}
