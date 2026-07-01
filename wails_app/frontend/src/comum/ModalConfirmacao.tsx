// ----- Seção: Comum -----

interface ModalConfirmacaoProps {
  confirmacao: any | null;
  setConfirmacao: (val: any | null) => void;
}

export function ModalConfirmacao(props: ModalConfirmacaoProps) {
  const { confirmacao, setConfirmacao } = props;

  if (!confirmacao) return null;

  return (
    <div className="modal-overlay" onClick={() => setConfirmacao(null)} style={{ zIndex: 1001 }}>
      <div
        className="modal-content"
        style={{ maxWidth: '460px', padding: '24px', flexDirection: 'column', height: 'auto' }}
        onClick={e => e.stopPropagation()}
      >
        <div className="modal-header">
          <h2 style={{ fontSize: '18px', color: '#f44336' }}>⚠️ {confirmacao.titulo}</h2>
          <button className="modal-close" onClick={() => setConfirmacao(null)}>×</button>
        </div>
        <div style={{ color: 'var(--cor-texto-primario)', fontSize: '14px', lineHeight: 1.5, marginTop: '8px', marginBottom: '24px' }}>
          {confirmacao.mensagem}
        </div>
        <div style={{ display: 'flex', gap: '12px', alignSelf: 'flex-end' }}>
          <button
            className="scan-btn"
            style={{ backgroundColor: 'var(--cor-fundo-secundario)', padding: '6px 16px' }}
            onClick={() => setConfirmacao(null)}
          >
            Cancelar
          </button>
          <button
            className="scan-btn"
            style={{ backgroundColor: '#f44336', padding: '6px 16px' }}
            onClick={() => {
              confirmacao.acao();
              setConfirmacao(null);
            }}
          >
            {confirmacao.rotuloAcao}
          </button>
        </div>
      </div>
    </div>
  );
}
