// ----- Seção: Nuvem — tipos e estados -----
// Fonte única da verdade dos valores trocados com o backend de sincronização (nuvem.Info.estado)
// e da escolha feita no modal de conflito.

// Lado que sobrevive quando o Drive já tem um backup de outra instalação.
export type EscolhaConflitoNuvem = 'manterLocal' | 'usarNuvem';

// nuvem.Info.estado === 'conflito' quando local e remoto divergem e ninguém escolheu ainda.
export const ESTADO_NUVEM_CONFLITO = 'conflito';
