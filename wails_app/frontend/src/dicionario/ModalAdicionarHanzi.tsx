// ----- Seção: Dicionário -----
import { useState } from "react";
import { LookupWord } from "../../wailsjs/go/main/App";
import { CanvasHanziLookup } from "./CanvasHanziLookup";

interface ModalAdicionarHanziProps {
  modalAdicionarHanzi: { open: boolean, status: string };
  setModalAdicionarHanzi: (val: { open: boolean, status: string }) => void;
  inputAdicionarHanzi: string;
  setInputAdicionarHanzi: (val: string) => void;
  sugestoesPinyin: string[];
  setSugestoesPinyin: React.Dispatch<React.SetStateAction<string[]>>;
  SalvarPalavra: (card: any, status: string) => void;
  setStatus: (msg: string) => void;
  configuracoesApp: any;
}

export function ModalAdicionarHanzi(props: ModalAdicionarHanziProps) {
  const {
    modalAdicionarHanzi, setModalAdicionarHanzi,
    inputAdicionarHanzi, setInputAdicionarHanzi,
    sugestoesPinyin, setSugestoesPinyin,
    SalvarPalavra, setStatus, configuracoesApp
  } = props;

  const [modo, setModo] = useState<'teclado' | 'desenho'>('teclado');

  if (!modalAdicionarHanzi.open) return null;

  const fecharModal = () => {
    setModalAdicionarHanzi({ open: false, status: '' });
    setInputAdicionarHanzi('');
    setSugestoesPinyin([]);
    setModo('teclado');
  };

  const confirmarAdicao = () => {
    if (!inputAdicionarHanzi) return;
    LookupWord(inputAdicionarHanzi).then(entradas => {
      if (entradas && entradas.length > 0) {
        const ent = entradas[0];
        const newCard = {
          hanzi: ent.Simplificado,
          Hanzi: ent.Simplificado,
          pinyin: ent.Pinyin,
          significados: ent.Significados
        };
        SalvarPalavra(newCard, modalAdicionarHanzi.status);
      } else {
        setStatus(`⚠️ Hanzi não encontrado no dicionário local: ${inputAdicionarHanzi}`);
      }
      fecharModal();
    });
  };

  return (
    <div className="modal-overlay" onClick={fecharModal} style={{ zIndex: 1002 }}>
      <div
        className="modal-content"
        style={{ maxWidth: '400px', padding: '24px', flexDirection: 'column', height: 'auto' }}
        onClick={e => e.stopPropagation()}
      >
        <div className="modal-header">
          <h2 style={{ fontSize: '18px' }}>Adicionar Hanzi Manualmente</h2>
          <button className="modal-close" onClick={fecharModal}>×</button>
        </div>

        {/* Toggle Mode */}
        <div style={{ display: 'flex', border: '1px solid var(--cor-borda)', borderRadius: '8px', overflow: 'hidden', marginTop: '16px' }}>
          <button
            style={{
              flex: 1, padding: '8px', border: 'none', cursor: 'pointer',
              backgroundColor: modo === 'teclado' ? 'var(--cor-destaque)' : 'var(--cor-fundo-secundario)',
              color: modo === 'teclado' ? '#fff' : 'var(--cor-texto-primario)'
            }}
            onClick={() => setModo('teclado')}
          >
            Teclado
          </button>
          <button
            style={{
              flex: 1, padding: '8px', border: 'none', cursor: 'pointer',
              backgroundColor: modo === 'desenho' ? 'var(--cor-destaque)' : 'var(--cor-fundo-secundario)',
              color: modo === 'desenho' ? '#fff' : 'var(--cor-texto-primario)'
            }}
            onClick={() => setModo('desenho')}
          >
            Desenho Livre
          </button>
        </div>

        <div style={{ display: 'flex', flexDirection: 'column', gap: '8px', marginTop: '16px', marginBottom: '24px' }}>
          {modo === 'teclado' ? (
            <>
              <label style={{ fontSize: '12px', color: 'var(--cor-texto-suave)' }}>Caractere Chinês (Hanzi):</label>
              <input
                type="text"
                value={inputAdicionarHanzi}
                onChange={(e) => {
                  const val = e.target.value;
                  setInputAdicionarHanzi(val);
                  if (/^[a-zA-Z]+$/.test(val)) {
                    // @ts-ignore
                    window.go.main.App.BuscarPorPinyin(val).then(res => setSugestoesPinyin(res || []));
                  } else {
                    setSugestoesPinyin([]);
                  }
                }}
                autoFocus
                style={{
                  backgroundColor: 'var(--cor-fundo-secundario)',
                  border: '1px solid var(--cor-borda)',
                  color: 'var(--cor-texto-primario)',
                  padding: '10px',
                  borderRadius: '6px',
                  fontFamily: 'var(--fonte-hanzi)',
                  fontSize: '24px',
                  textAlign: 'center'
                }}
                onKeyDown={(e) => {
                  if (e.key === 'Enter') {
                    confirmarAdicao();
                  }
                }}
              />

              {sugestoesPinyin.length > 0 && (
                <div style={{ display: 'flex', flexWrap: 'wrap', gap: '8px', marginTop: '12px', justifyContent: 'center' }}>
                  {sugestoesPinyin.map((hz, idx) => (
                    <div
                      key={idx}
                      className="scan-btn"
                      style={{ fontSize: '20px', padding: '4px 12px', fontFamily: 'var(--fonte-hanzi)', cursor: 'pointer' }}
                      onClick={() => {
                        setInputAdicionarHanzi(hz);
                        setSugestoesPinyin([]);
                      }}
                    >{hz}</div>
                  ))}
                </div>
              )}
            </>
          ) : (
            <div style={{ display: 'flex', flexDirection: 'column', alignItems: 'center' }}>
              <label style={{ fontSize: '12px', color: 'var(--cor-texto-suave)', marginBottom: '8px', alignSelf: 'flex-start' }}>
                Desenhe o Hanzi abaixo:
              </label>
              <CanvasHanziLookup 
                onRecognize={(sugestoes) => setSugestoesPinyin(sugestoes)} 
                configuracoesApp={configuracoesApp}
              />
              
              <div style={{ display: 'flex', flexWrap: 'wrap', gap: '8px', marginTop: '12px', justifyContent: 'center', minHeight: '40px' }}>
                {sugestoesPinyin.length === 0 ? (
                   <span style={{ fontSize: '12px', color: 'var(--cor-texto-suave)', marginTop: '10px' }}>
                     Nenhuma correspondência ainda.
                   </span>
                ) : (
                  sugestoesPinyin.map((hz, idx) => (
                    <div
                      key={idx}
                      className="scan-btn"
                      style={{ fontSize: '20px', padding: '4px 12px', fontFamily: 'var(--fonte-hanzi)', cursor: 'pointer' }}
                      onClick={() => {
                        setInputAdicionarHanzi(hz);
                        setModo('teclado');
                        setSugestoesPinyin([]);
                      }}
                    >{hz}</div>
                  ))
                )}
              </div>
            </div>
          )}
        </div>

        <div style={{ display: 'flex', gap: '12px', alignSelf: 'flex-end' }}>
          <button
            className="scan-btn"
            style={{ backgroundColor: 'var(--cor-fundo-secundario)', padding: '6px 16px' }}
            onClick={fecharModal}
          >
            Cancelar
          </button>
          <button
            id="btn-add-hanzi-confirm"
            className="scan-btn"
            style={{ backgroundColor: '#2196f3', padding: '6px 16px' }}
            onClick={confirmarAdicao}
          >
            Adicionar ({modalAdicionarHanzi.status.toUpperCase()})
          </button>
        </div>
      </div>
    </div>
  );
}
