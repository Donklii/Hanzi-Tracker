// ----- Seção: Cartões (compartilhado) -----
// Transformações puras sobre listas de cartões, consumidas pelas abas de Descobrimento/Estudos e
// pelo App. Puras de propósito: não leem config nem state — recebem tudo por parâmetro.

// Valores possíveis de config.tipoHanziExibicao (lado Go) e do campo tipoHanzi de um cartão.
const EXIBICAO_AMBOS = 'ambos';
const EXIBICAO_SIMPLIFICADO = 'simplificado';
const TIPO_CARTAO_SIMPLIFICADO = 'Simplificado';
const TIPO_CARTAO_TRADICIONAL = 'Tradicional';
const TIPO_CARTAO_AMBOS = 'Ambos';


// DeduplicarCartoes mantém a primeira ocorrência de cada hanzi, preservando a ordem de entrada.
export function DeduplicarCartoes(cartoes: any[]): any[] {
  const porHanzi = new Map<string, any>();
  for (const cartao of cartoes) {
    if (porHanzi.has(cartao.hanzi)) continue;
    porHanzi.set(cartao.hanzi, cartao);
  }
  return Array.from(porHanzi.values());
}


// FiltrarPorTipoHanzi esconde cartões que não batem com o tipo de escrita escolhido nas configurações.
// Cartões marcados como "Ambos" (ou sem tipo) sobrevivem a qualquer filtro.
export function FiltrarPorTipoHanzi(cartoes: any[], tipoExibicao?: string): any[] {
  if (!tipoExibicao || tipoExibicao === EXIBICAO_AMBOS) {
    return cartoes;
  }

  const exibindoSimplificado = tipoExibicao === EXIBICAO_SIMPLIFICADO;
  return cartoes.filter(cartao => {
    const tipo = cartao.tipoHanzi || cartao.TipoHanzi || TIPO_CARTAO_AMBOS;
    if (exibindoSimplificado && tipo === TIPO_CARTAO_TRADICIONAL) return false;
    if (!exibindoSimplificado && tipo === TIPO_CARTAO_SIMPLIFICADO) return false;
    return true;
  });
}
