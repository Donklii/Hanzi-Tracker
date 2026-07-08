// ----- Seção: Configurações — helpers compartilhados entre as abas do painel -----
import { ReactNode } from 'react';

// Andamento do pré-carregamento em lote do cache de áudio (espelha main.ProgressoPreCacheTts do Go).
// Tipado aqui porque payloads de evento não entram nos models.ts gerados; compartilhado entre o App
// (que recebe o evento "tts_precache_progresso") e a aba de Motores (que exibe a barra de progresso).
export interface ProgressoPreCacheTts {
  total: number;
  processados: number;
  sintetizados: number;
  jaEmCache: number;
  falhas: number;
  emAndamento: boolean;
  mensagem: string;
}

// FormatarTamanho exibe bytes de forma amigável (usada nas abas de Motores e Armazenamento).
export function FormatarTamanho(bytes: number): string {
  if (bytes === 0) return '0 B';
  const k = 1024;
  const tamanhos = ['B', 'KB', 'MB', 'GB', 'TB'];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + tamanhos[i];
}

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
