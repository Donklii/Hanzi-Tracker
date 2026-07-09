// ----- Seção: Status do vocabulário (compartilhado) -----
// Os três estados que uma palavra pode ter no banco (progresso.Vocab.Status). Antes eram magic
// strings repetidas em seis arquivos; aqui viram uma fonte única, com os MESMOS valores gravados
// pelo Go — mudar um valor aqui muda em todo lugar que lê ou escreve o status.

export const STATUS_VOCABULARIO = {
  Visto: 'visto',
  Estudo: 'estudo',
  Aprendido: 'aprendido',
} as const;

export type StatusVocabulario = typeof STATUS_VOCABULARIO[keyof typeof STATUS_VOCABULARIO];
