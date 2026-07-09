
interface PopupRevisaoProps {
  info: {
    hanzi: string;
    pinyin: string;
    significados: string;
    x: number;
    y: number;
  } | null;
}

export function PopupRevisao({ info }: PopupRevisaoProps) {
  if (!info) return null;

  return (
      <div
      className="popup-revisao"
      style={{
        position: 'fixed',
        left: Math.max(10, info.x - 120), // 120 é metade da largura estimada
        top: info.y - 15, // Acima da palavra alvo
        transform: 'translateY(-100%)',
        width: '240px',
        backgroundColor: 'var(--cor-fundo)',
        border: '1px solid var(--cor-borda)',
        borderRadius: '8px',
        padding: '12px',
        boxShadow: '0 -4px 12px rgba(0,0,0,0.3)',
        pointerEvents: 'none',
        zIndex: 9999,
        display: 'flex',
        flexDirection: 'column',
        fontFamily: 'var(--fonte-interface)'
      }}
    >
      <div style={{ textAlign: 'center', marginBottom: '8px' }}>
        <div style={{ fontSize: '18px', color: 'var(--cor-pinyin)', marginBottom: '4px' }}>
          {info.pinyin}
        </div>
        <div style={{ fontSize: '36px', fontFamily: 'var(--fonte-hanzi)', color: 'var(--cor-texto-primario)', lineHeight: '1.2' }}>
          {info.hanzi}
        </div>
      </div>
      <div style={{ fontSize: '14px', color: 'var(--cor-texto-suave)', lineHeight: '1.4', borderTop: '1px solid var(--cor-borda)', paddingTop: '8px', textAlign: 'center' }}>
        {info.significados || 'Sem definição.'}
      </div>
    </div>
  );
}
