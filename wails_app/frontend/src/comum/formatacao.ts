// ----- Seção: Formatação (compartilhado) -----
// Helpers de exibição usados por múltiplas seções (abas de Motores e Armazenamento, modal de
// conflito da nuvem). Vive em comum/ por ser consumido por mais de uma seção.

const BASE_BYTES = 1024;
const UNIDADES_TAMANHO = ['B', 'KB', 'MB', 'GB', 'TB'] as const;

// FormatarTamanho exibe uma quantidade de bytes de forma amigável (ex.: 1536 → "1.5 KB").
export function FormatarTamanho(bytes: number): string {
  if (bytes === 0) {
    return '0 B';
  }
  const indice = Math.floor(Math.log(bytes) / Math.log(BASE_BYTES));
  return parseFloat((bytes / Math.pow(BASE_BYTES, indice)).toFixed(2)) + ' ' + UNIDADES_TAMANHO[indice];
}
