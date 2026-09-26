// ----- Seção: Fade + Continuação do Hanzi Writer -----
//
// Mecânica compartilhada por toda atividade que esconde um caractere (ou um recorte de traços dele)
// com fadeout e só ENTÃO libera a continuação (abrir o quiz, revelar a resposta, etc). A ordem é
// obrigatória: o método quiz() do Hanzi Writer cancela qualquer animação em curso e salta para o
// estado final dela — chamar hideCharacter() e quiz() em sequência direta faz o caractere sumir de
// estalo, sem fadeout nenhum. Usada pelo desenho de memória e pelo desenho por componente
// (CanvasDesenho.tsx) e pela montagem de peças (revisao/MontagemHanzi.tsx); qualquer nova
// atividade que esconde o caractere reaproveita esta mesma função.

// DURACAO_FADEOUT_PADRAO_MS é o tempo do sumiço quando o chamador não pede uma duração própria.
export const DURACAO_FADEOUT_PADRAO_MS = 1000;

// EsconderComFadeEhContinuar esconde o que o writer tem carregado (o hideCharacter dele já respeita
// um charData parcial, se for o caso) e chama aoTerminarFade quando a animação acaba. Devolve uma
// função de cancelamento — o chamador DEVE invocá-la no cleanup do efeito/desmonte, para a
// continuação não disparar depois que a questão ou o componente já foram trocados.
export function EsconderComFadeEhContinuar(
  writer: any,
  aoTerminarFade: () => void,
  duracaoMs: number = DURACAO_FADEOUT_PADRAO_MS
): () => void {
  writer.hideCharacter({ duration: duracaoMs });
  const idTimeout = window.setTimeout(aoTerminarFade, duracaoMs);
  return () => window.clearTimeout(idTimeout);
}
