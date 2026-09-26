// ----- Seção: Comum -----
import { t } from '../i18n/i18n';

interface ModalConfirmacaoProps {
  confirmacao: any | null;
  setConfirmacao: (val: any | null) => void;
}

export function ModalConfirmacao(props: ModalConfirmacaoProps) {
  const { confirmacao, setConfirmacao } = props;

  if (!confirmacao) return null;

  const corBotaoPrincipal = confirmacao.perigoso !== false ? '#f44336' : 'var(--cor-destaque)';
  const iconeCabecalho = confirmacao.perigoso !== false ? '⚠️' : 'ℹ️';
  const corCabecalho = confirmacao.perigoso !== false ? '#f44336' : 'var(--cor-destaque)';

  return (
    <div className="modal-overlay" onClick={() => {
      if (confirmacao.cancelarAcao) confirmacao.cancelarAcao();
      setConfirmacao(null);
    }} style={{ zIndex: 1001 }}>
      <div
        className="modal-content"
        style={{ maxWidth: '480px', padding: '24px', flexDirection: 'column', height: 'auto' }}
        onClick={e => e.stopPropagation()}
      >
        <div className="modal-header">
          <h2 style={{ fontSize: '18px', color: corCabecalho }}>{iconeCabecalho} {confirmacao.titulo}</h2>
          <button className="modal-close" onClick={() => {
            if (confirmacao.cancelarAcao) confirmacao.cancelarAcao();
            setConfirmacao(null);
          }}>×</button>
        </div>
        <div style={{ color: 'var(--cor-texto-primario)', fontSize: '14px', lineHeight: 1.5, marginTop: '8px', marginBottom: '24px' }}>
          {confirmacao.mensagem}
        </div>
        <div style={{ display: 'flex', gap: '12px', alignSelf: 'flex-end', flexWrap: 'wrap' }}>
          <button
            className="scan-btn"
            style={{ backgroundColor: 'var(--cor-fundo-secundario)', padding: '6px 16px' }}
            onClick={() => {
              if (confirmacao.cancelarAcao) confirmacao.cancelarAcao();
              setConfirmacao(null);
            }}
          >
            {confirmacao.rotuloCancelar || t('Cancelar')}
          </button>
          {confirmacao.rotuloAcao2 && (
            <button
              className="scan-btn"
              style={{ backgroundColor: 'var(--cor-fundo-secundario)', padding: '6px 16px' }}
              onClick={() => {
                if (confirmacao.acao2) confirmacao.acao2();
                setConfirmacao(null);
              }}
            >
              {confirmacao.rotuloAcao2}
            </button>
          )}
          <button
            className="scan-btn"
            style={{ backgroundColor: corBotaoPrincipal, padding: '6px 16px' }}
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
