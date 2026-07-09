// ----- Seção: Casca — abas de navegação -----
// Fonte única da verdade das abas: o identificador de cada aba e o título exibido no cabeçalho.
// Antes esses valores eram magic strings espalhadas pelo App (comparações, onClick e um switch de
// títulos); aqui viram um enum + uma lookup table.

export const ABAS = {
  Descobrimento: 'descobrimento',
  TelaUnica: 'tela_unica',
  Vistas: 'vistas',
  Estudando: 'estudando',
  Aprendidas: 'aprendidas',
  Revisao: 'revisao',
} as const;

export type Aba = typeof ABAS[keyof typeof ABAS];

export const TITULOS_POR_ABA: Record<Aba, string> = {
  [ABAS.Descobrimento]: 'Descobrimento (Último OCR)',
  [ABAS.TelaUnica]: 'Palavras Dessa Seção (Acumulado)',
  [ABAS.Vistas]: 'Histórico: Já Vistas',
  [ABAS.Estudando]: 'Estudando',
  [ABAS.Aprendidas]: 'Vocabulário (Aprendidas)',
  [ABAS.Revisao]: 'Revisão de Hanzis',
};
