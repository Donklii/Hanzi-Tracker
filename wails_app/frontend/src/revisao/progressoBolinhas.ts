/**
 * Módulo para cálculo dos tipos de bolinhas de progresso (máximo 3 visíveis por vez)
 * com substituição em ordem FIFO quando o streak ultrapassa 3 acertos:
 * - Pontos 0 a 3: introduzem bolinhas comuns
 * - Pontos 4 a 6: introduzem bolinhas prateadas em ordem FIFO (substituindo as comuns)
 * - Pontos 7 a 9: introduzem bolinhas douradas em ordem FIFO (substituindo as prateadas)
 */

export type TipoBolinha = 'vazia' | 'comum' | 'prateada' | 'dourada';

export function obterTiposBolinhas(streak: number): [TipoBolinha, TipoBolinha, TipoBolinha] {
  const s = Math.max(0, Math.min(9, streak || 0));

  // Slot 0 (atualizado nos pontos 1, 4, 7)
  const slot0: TipoBolinha = s >= 7 ? 'dourada' : s >= 4 ? 'prateada' : s >= 1 ? 'comum' : 'vazia';
  // Slot 1 (atualizado nos pontos 2, 5, 8)
  const slot1: TipoBolinha = s >= 8 ? 'dourada' : s >= 5 ? 'prateada' : s >= 2 ? 'comum' : 'vazia';
  // Slot 2 (atualizado nos pontos 3, 6, 9)
  const slot2: TipoBolinha = s >= 9 ? 'dourada' : s >= 6 ? 'prateada' : s >= 3 ? 'comum' : 'vazia';

  return [slot0, slot1, slot2];
}
