// ----- Seção: Configurações — helpers compartilhados entre as abas do painel -----
import { ReactNode } from 'react';

interface SecaoDependenteProps {
  ativa: boolean;
  children: ReactNode;
}

// SecaoDependente recua (e esconde) controles que dependem de um toggle-pai ligado.
export function SecaoDependente({ ativa, children }: SecaoDependenteProps) {
  if (!ativa) {
    return null;
  }
  return (
    <div style={{ paddingLeft: '24px', marginTop: '12px', marginBottom: '24px', borderLeft: '2px solid var(--cor-borda)' }}>
      {children}
    </div>
  );
}
