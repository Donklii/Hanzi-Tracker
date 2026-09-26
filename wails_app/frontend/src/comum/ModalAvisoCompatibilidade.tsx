// ----- Seção: Comum -----
import { t } from '../i18n/i18n';

interface ModalAvisoCompatibilidadeProps {
  avisoCompatibilidade: string | null;
  setAvisoCompatibilidade: (val: string | null) => void;
}

export function ModalAvisoCompatibilidade(props: ModalAvisoCompatibilidadeProps) {
  const { avisoCompatibilidade, setAvisoCompatibilidade } = props;

  if (!avisoCompatibilidade) return null;

  return (
    <div className="modal-overlay" onClick={() => setAvisoCompatibilidade(null)} style={{ zIndex: 3000 }}>
      <div
        className="modal-content"
        style={{ maxWidth: '440px', padding: '24px', flexDirection: 'column', height: 'auto' }}
        onClick={e => e.stopPropagation()}
      >
        <div className="modal-header">
          <h2 style={{ fontSize: '18px' }}>{t('⚠️ Hardware ajustado')}</h2>
          <button className="modal-close" onClick={() => setAvisoCompatibilidade(null)}>×</button>
        </div>
        <div style={{ color: 'var(--cor-texto-primario)', fontSize: '14px', lineHeight: 1.5, marginTop: '8px' }}>
          {avisoCompatibilidade}
        </div>
        <button
          className="scan-btn"
          style={{ marginTop: '20px', alignSelf: 'flex-end', padding: '6px 16px' }}
          onClick={() => setAvisoCompatibilidade(null)}
        >
          {t('Entendi')}
        </button>
      </div>
    </div>
  );
}
