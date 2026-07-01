// ----- Seção: Dicionário -----
import { LookupWord } from "../../wailsjs/go/main/App";

interface ModalAdicionarHanziProps {
  modalAdicionarHanzi: { open: boolean, status: string };
  setModalAdicionarHanzi: (val: { open: boolean, status: string }) => void;
  inputAdicionarHanzi: string;
  setInputAdicionarHanzi: (val: string) => void;
  sugestoesPinyin: string[];
  setSugestoesPinyin: (val: string[]) => void;
  SalvarPalavra: (card: any, status: string) => void;
  setStatus: (msg: string) => void;
}

export function ModalAdicionarHanzi(props: ModalAdicionarHanziProps) {
  const {
    modalAdicionarHanzi, setModalAdicionarHanzi,
    inputAdicionarHanzi, setInputAdicionarHanzi,
    sugestoesPinyin, setSugestoesPinyin,
    SalvarPalavra, setStatus
  } = props;

  if (!modalAdicionarHanzi.open) return null;

  return (
    <div className="modal-overlay" onClick={() => setModalAdicionarHanzi({ open: false, status: '' })} style={{ zIndex: 1002 }}>
      <div
        className="modal-content"
        style={{ maxWidth: '400px', padding: '24px', flexDirection: 'column', height: 'auto' }}
        onClick={e => e.stopPropagation()}
      >
        <div className="modal-header">
          <h2 style={{ fontSize: '18px' }}>Adicionar Hanzi Manualmente</h2>
          <button className="modal-close" onClick={() => setModalAdicionarHanzi({ open: false, status: '' })}>×</button>
        </div>
        
        <div style={{ display: 'flex', flexDirection: 'column', gap: '8px', marginTop: '16px', marginBottom: '24px' }}>
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
                const btn = document.getElementById('btn-add-hanzi-confirm');
                if(btn) btn.click();
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
        </div>
        
        <div style={{ display: 'flex', gap: '12px', alignSelf: 'flex-end' }}>
          <button
            className="scan-btn"
            style={{ backgroundColor: 'var(--cor-fundo-secundario)', padding: '6px 16px' }}
            onClick={() => setModalAdicionarHanzi({ open: false, status: '' })}
          >
            Cancelar
          </button>
          <button
            id="btn-add-hanzi-confirm"
            className="scan-btn"
            style={{ backgroundColor: '#2196f3', padding: '6px 16px' }}
            onClick={() => {
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
                setModalAdicionarHanzi({ open: false, status: '' });
                setInputAdicionarHanzi('');
              });
            }}
          >
            Adicionar ({modalAdicionarHanzi.status.toUpperCase()})
          </button>
        </div>
      </div>
    </div>
  );
}
