// ----- Seção: Nuvem — conflito da primeira conexão com o Google Drive -----
// Mostrado quando o Drive já tem um backup de outra instalação: o usuário escolhe qual lado
// sobrevive (o outro é sobrescrito). Nada é sincronizado até a escolha.
import { CSSProperties } from 'react';
import { nuvem } from '../../wailsjs/go/models';
import { FormatarTamanho } from '../comum/formatacao';
import { EscolhaConflitoNuvem } from './tipos';

interface ModalConflitoNuvemProps {
  aberto: boolean;
  infoNuvem: nuvem.Info | null;
  ocupado: boolean;
  aoEscolher: (escolha: EscolhaConflitoNuvem) => void;
  aoFechar: () => void;
}

// formatarData converte um RFC3339 do backend em data legível ("" = desconhecida).
function formatarData(rfc3339: string): string {
  if (!rfc3339) return 'data desconhecida';
  return new Date(rfc3339).toLocaleString();
}

export function ModalConflitoNuvem({ aberto, infoNuvem, ocupado, aoEscolher, aoFechar }: ModalConflitoNuvemProps) {
  if (!aberto || !infoNuvem) return null;

  const estiloOpcao: CSSProperties = {
    flex: 1,
    display: 'flex',
    flexDirection: 'column',
    gap: '6px',
    alignItems: 'flex-start',
    textAlign: 'left',
    border: '1px solid var(--cor-borda)',
    borderRadius: '8px',
    padding: '14px',
    backgroundColor: 'var(--cor-fundo-cartao)',
    cursor: ocupado ? 'wait' : 'pointer',
    opacity: ocupado ? 0.6 : 1,
  };

  return (
    <div className="modal-overlay" onClick={aoFechar} style={{ zIndex: 1001 }}>
      <div
        className="modal-content"
        style={{ maxWidth: '560px', padding: '24px', flexDirection: 'column', height: 'auto' }}
        onClick={e => e.stopPropagation()}
      >
        <div className="modal-header">
          <h2 style={{ fontSize: '18px' }}>☁️ Já existe um backup no seu Google Drive</h2>
          <button className="modal-close" onClick={aoFechar}>×</button>
        </div>

        <div style={{ color: 'var(--cor-texto-primario)', fontSize: '14px', lineHeight: 1.5, marginTop: '8px', marginBottom: '20px' }}>
          Este Google Drive já guarda um banco de outra instalação do Hanzi Tracker.
          Escolha qual banco vale — <strong>o outro lado será sobrescrito</strong> e isso não pode ser desfeito.
        </div>

        <div style={{ display: 'flex', gap: '12px' }}>
          <button style={estiloOpcao} disabled={ocupado} onClick={() => aoEscolher('manterLocal')}>
            <div style={{ fontWeight: 'bold', fontSize: '14px' }}>💻 Manter os dados deste computador</div>
            <div style={{ fontSize: '12px', color: 'var(--cor-texto-suave)' }}>
              Banco local · {FormatarTamanho(infoNuvem.localBytes) || '0 MB'}
            </div>
            <div style={{ fontSize: '12px', color: '#f44336' }}>O backup na nuvem será sobrescrito.</div>
          </button>

          <button style={estiloOpcao} disabled={ocupado} onClick={() => aoEscolher('usarNuvem')}>
            <div style={{ fontWeight: 'bold', fontSize: '14px' }}>☁️ Usar os dados da nuvem</div>
            <div style={{ fontSize: '12px', color: 'var(--cor-texto-suave)' }}>
              Backup de {formatarData(infoNuvem.remotoModificadoEm)} · {FormatarTamanho(infoNuvem.remotoBytes) || '0 MB'}
            </div>
            <div style={{ fontSize: '12px', color: '#f44336' }}>O vocabulário deste computador será sobrescrito.</div>
          </button>
        </div>

        <div style={{ fontSize: '12px', color: 'var(--cor-texto-suave)', marginTop: '16px' }}>
          Dá para decidir depois: enquanto isso, nada é sincronizado (o botão “Resolver conflito” fica na aba Armazenamento).
        </div>
      </div>
    </div>
  );
}
