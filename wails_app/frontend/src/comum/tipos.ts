// ----- Seção: Tipos compartilhados -----
// Tipos de payloads de evento que não entram nos models.ts gerados pelo Wails e são consumidos por
// mais de uma seção. Vivem em comum/ para ter uma única fonte da verdade.

// Andamento do pré-carregamento em lote do cache de áudio (espelha main.ProgressoPreCacheTts do Go).
// Compartilhado entre o App (que recebe o evento "tts_precache_progresso") e a aba de Motores (que
// exibe a barra de progresso).
export interface ProgressoPreCacheTts {
  total: number;
  processados: number;
  sintetizados: number;
  jaEmCache: number;
  falhas: number;
  emAndamento: boolean;
  mensagem: string;
}
