// ----- Seção: Dicionário -----

interface ModalCartaoDetalhesProps {
  cartaoSelecionado: any | null;
  setCartaoSelecionado: (val: any | null) => void;
  imagemModalBase64: string | null;
  dadosDecomposicao: any | null;
  AoClicarNoCaractereDecomposto: (char: string) => void;
}

export function ModalCartaoDetalhes(props: ModalCartaoDetalhesProps) {
  const {
    cartaoSelecionado, setCartaoSelecionado,
    imagemModalBase64, dadosDecomposicao,
    AoClicarNoCaractereDecomposto
  } = props;

  if (!cartaoSelecionado) return null;

  return (
    <div className="modal-overlay" onClick={() => setCartaoSelecionado(null)}>
      <div className="modal-content hanzi-modal-content" onClick={e => e.stopPropagation()}>
        <div className="modal-header">
          <h2>Detalhes</h2>
          <button className="modal-close" onClick={() => setCartaoSelecionado(null)}>×</button>
        </div>
        <div className="modal-body" style={{display: 'flex', flexDirection: 'column', alignItems: 'center', gap: '20px'}}>
          
          {imagemModalBase64 && (
            <div style={{border: '1px solid var(--cor-borda)', padding: '4px', borderRadius: '4px', backgroundColor: 'var(--cor-fundo-cartao)'}}>
              <img src={"data:image/png;base64," + imagemModalBase64} alt="Recorte" style={{maxWidth: '100%', maxHeight: '150px'}} />
            </div>
          )}

          <div style={{textAlign: 'center'}}>
            <div style={{color: 'var(--cor-pinyin)', fontSize: '24px'}}>{cartaoSelecionado.pinyin || cartaoSelecionado.Pinyin}</div>
            <div style={{fontFamily: 'var(--fonte-hanzi)', fontSize: '64px', fontWeight: 'bold', lineHeight: '1.2'}}>{cartaoSelecionado.hanzi || cartaoSelecionado.Hanzi}</div>
            <div style={{color: 'var(--cor-texto-suave)', fontSize: '18px', marginTop: '10px'}}>{cartaoSelecionado.significados ? cartaoSelecionado.significados.join(', ') : cartaoSelecionado.Significado}</div>
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
        </div>
      </div>
    </div>
  );
}
