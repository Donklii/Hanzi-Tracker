// ----- Seção: Física de Arrasto do Quebra-Cabeça -----
// Física compartilhada pelos jogos de 2 e 3 colunas: empurrão das peças livres perto da peça
// arrastada, separação estática por pares (peças paradas também se repelem quando ficam
// sobrepostas), barreira da arena e detecção de conexão por proximidade dos CONECTORES. Ao fim do
// arrasto os empurrões são ASSENTADOS no deslocamento persistente de cada peça — o tabuleiro é uma
// mesa física: as peças ficam onde foram soltas/empurradas, não voltam para as colunas.
import { PROFUNDIDADE_PINO } from './FormaPecaQuebraCabeca';

// RetanguloPeca é o retângulo mínimo usado pela física (mesma forma do DOMRect, sem depender dele).
export interface RetanguloPeca {
  left: number;
  top: number;
  width: number;
  height: number;
}

// EMPURRAO_MAXIMO é só um TETO de segurança (px) para o empurrão de uma peça livre: o empurrão
// normal é o mínimo necessário para sair de baixo da peça arrastada, não um campo de força — peça
// que não está sendo coberta não se mexe.
const EMPURRAO_MAXIMO = 34;

// MARGEM_EMPURRAO é a folga (px) somada ao desencostar: a peça empurrada para assim que libera a
// arrastada, com uma sobra mínima para as bordas não ficarem coladas.
const MARGEM_EMPURRAO = 4;

// MapaVetores indexa um vetor {x,y} por peça (chave "coluna-id"): serve tanto para os empurrões
// transitórios do arrasto quanto para os deslocamentos persistentes das peças.
export type MapaVetores = Record<string, { x: number; y: number }>;

// VETOR_ZERO é o vetor nulo compartilhado (evita alocar um objeto novo por render).
export const VETOR_ZERO = { x: 0, y: 0 };

// calcularEmpurrao devolve o vetor que tira uma peça parada de BAIXO da peça arrastada — ou null
// quando as duas nem se sobrepõem (peça só de passagem perto não é empurrada). O empurrão é o
// deslocamento MÍNIMO que desencosta as caixas, pelo eixo de menor penetração: mesma regra da
// separação estática, para o gesto e o repouso terem a mesma física.
// `rectBase` é o retângulo de REPOUSO da peça empurrada (slot do wrapper + deslocamento
// persistente), nunca a medição do card: o card tem transição CSS, então medi-lo devolveria uma
// posição atrasada e o empurrão se realimentaria quadro a quadro até a peça disparar para a parede.
export function calcularEmpurrao(rectArrastado: RetanguloPeca, rectBase: RetanguloPeca): { x: number; y: number } | null {
  const penetracaoX = Math.min(rectBase.left + rectBase.width, rectArrastado.left + rectArrastado.width) - Math.max(rectBase.left, rectArrastado.left) + MARGEM_EMPURRAO;
  const penetracaoY = Math.min(rectBase.top + rectBase.height, rectArrastado.top + rectArrastado.height) - Math.max(rectBase.top, rectArrastado.top) + MARGEM_EMPURRAO;
  if (penetracaoX <= 0 || penetracaoY <= 0) return null;

  if (penetracaoX < penetracaoY) {
    const direcao = rectBase.left + rectBase.width / 2 >= rectArrastado.left + rectArrastado.width / 2 ? 1 : -1;
    return { x: Math.min(penetracaoX, EMPURRAO_MAXIMO) * direcao, y: 0 };
  }
  const direcao = rectBase.top + rectBase.height / 2 >= rectArrastado.top + rectArrastado.height / 2 ? 1 : -1;
  return { x: 0, y: Math.min(penetracaoY, EMPURRAO_MAXIMO) * direcao };
}

// ----- Separação estática (peças paradas também se repelem) -----
// Depois de qualquer assentamento (soltura, encaixe, retorno da quebra), peças podem ter ficado
// sobrepostas na mesa. Um laço de relaxamento por PARES roda por até DURACAO_SEPARACAO_ESTATICA
// (ou até estabilizar): a cada quadro, todo par de peças próximo demais é afastado — peças livres
// dividem o afastamento; peças/conjuntos conectados são "pesados" (empurram sem sair do lugar).
export const DURACAO_SEPARACAO_ESTATICA = 1500;

// MARGEM_SEPARACAO é a folga mínima (px) exigida entre duas peças paradas — menor que o gap do
// layout em repouso (as colunas não podem se auto-empurrar) e suficiente para desgrudar sobrepostas.
const MARGEM_SEPARACAO = 6;
// PASSO_MAXIMO_SEPARACAO limita o deslocamento por quadro (a transição CSS suaviza o movimento).
const PASSO_MAXIMO_SEPARACAO = 10;

// PecaSeparacao descreve uma peça na malha de separação: retângulo atual e se pode ser movida.
export interface PecaSeparacao {
  chave: string;
  rect: RetanguloPeca;
  movel: boolean;
}

// passoSeparacaoEstatica calcula UM quadro do relaxamento: para cada par mais próximo que
// MARGEM_SEPARACAO, afasta pelo eixo de MENOR penetração (deslocamento mínimo) — meio a meio entre
// duas móveis, inteiro para a móvel quando a outra é pesada. Devolve os deltas do quadro (vazio =
// mesa estável).
export function passoSeparacaoEstatica(pecas: PecaSeparacao[]): MapaVetores {
  const deltas: MapaVetores = {};
  const somar = (chave: string, dx: number, dy: number) => {
    const atual = deltas[chave] || VETOR_ZERO;
    deltas[chave] = { x: atual.x + dx, y: atual.y + dy };
  };

  for (let i = 0; i < pecas.length; i++) {
    for (let j = i + 1; j < pecas.length; j++) {
      const a = pecas[i];
      const b = pecas[j];
      if (!a.movel && !b.movel) continue;

      const penetracaoX = Math.min(a.rect.left + a.rect.width, b.rect.left + b.rect.width) - Math.max(a.rect.left, b.rect.left) + MARGEM_SEPARACAO;
      const penetracaoY = Math.min(a.rect.top + a.rect.height, b.rect.top + b.rect.height) - Math.max(a.rect.top, b.rect.top) + MARGEM_SEPARACAO;
      if (penetracaoX <= 0 || penetracaoY <= 0) continue;

      let deltaX = 0;
      let deltaY = 0;
      if (penetracaoX < penetracaoY) {
        const direcao = a.rect.left + a.rect.width / 2 <= b.rect.left + b.rect.width / 2 ? 1 : -1;
        deltaX = Math.min(penetracaoX / 2, PASSO_MAXIMO_SEPARACAO) * direcao;
      } else {
        const direcao = a.rect.top + a.rect.height / 2 <= b.rect.top + b.rect.height / 2 ? 1 : -1;
        deltaY = Math.min(penetracaoY / 2, PASSO_MAXIMO_SEPARACAO) * direcao;
      }

      if (a.movel && b.movel) {
        somar(a.chave, -deltaX / 2, -deltaY / 2);
        somar(b.chave, deltaX / 2, deltaY / 2);
      } else if (b.movel) {
        somar(b.chave, deltaX, deltaY);
      } else {
        somar(a.chave, -deltaX, -deltaY);
      }
    }
  }
  return deltas;
}

// repulsaoContinua roda um laço de requestAnimationFrame por até `duracaoMaximaMs`: a cada quadro
// chama `calcularQuadro`, que recalcula/aplica um passo de física e devolve `true` enquanto algo
// ainda se moveu (o laço para mais cedo se estabilizar). Chama `aoTerminar` ao final (teto de
// tempo OU estabilização). Devolve um cancelador (novo arrasto, desmontagem).
export function repulsaoContinua(duracaoMaximaMs: number, calcularQuadro: () => boolean, aoTerminar: () => void): () => void {
  const inicio = performance.now();
  let cancelado = false;
  let raf = 0;

  const passo = () => {
    if (cancelado) return;
    const aindaEmpurrando = calcularQuadro();
    const decorrido = performance.now() - inicio;
    if (aindaEmpurrando && decorrido < duracaoMaximaMs) {
      raf = requestAnimationFrame(passo);
    } else {
      aoTerminar();
    }
  };

  raf = requestAnimationFrame(passo);
  return () => {
    cancelado = true;
    cancelAnimationFrame(raf);
  };
}

// ----- Detecção de conexão por proximidade dos conectores -----
// A sobreposição de retângulos falha no gesto mais natural do quebra-cabeça: aproximar o LADO
// conector (o pino de uma peça do buraco da outra) — as caixas mal se sobrepõem e o alvo não era
// detectado. A detecção principal passa a ser a distância entre o centro do PINO de uma peça e o
// centro do BURACO da outra; a sobreposição grande segue valendo como fallback (soltar por cima).

// RAIO_CONEXAO é a distância máxima (px) entre os dois conectores para o encaixe valer.
export const RAIO_CONEXAO = 60;

// distanciaConectores mede a distância entre o centro do pino (borda direita de `rectComPino`) e o
// centro do buraco (borda esquerda de `rectComBuraco`). Quando encaixadas, os dois pontos coincidem.
export function distanciaConectores(rectComPino: RetanguloPeca, rectComBuraco: RetanguloPeca): number {
  const pinoX = rectComPino.left + rectComPino.width - PROFUNDIDADE_PINO / 2;
  const pinoY = rectComPino.top + rectComPino.height / 2;
  const buracoX = rectComBuraco.left + PROFUNDIDADE_PINO / 2;
  const buracoY = rectComBuraco.top + rectComBuraco.height / 2;
  return Math.hypot(pinoX - buracoX, pinoY - buracoY);
}

// ----- Limiar de sobreposição para detecção de alvo -----
// Ao arrastar uma peça sobre outra, o "alvo" é decidido pela maior área de sobreposição — mas um
// limiar em pixels FIXO fica cada vez mais fácil de disparar por acidente conforme as peças crescem
// (mesa maior, cards maiores). FRACAO_SOBREPOSICAO_ALVO torna o limiar proporcional ao tamanho da
// MENOR peça envolvida, então a exigência de overlap continua significativa em qualquer tamanho —
// reduz falsos positivos de alvo quando peças vizinhas ficam próximas numa mesa cheia.
export const FRACAO_SOBREPOSICAO_ALVO = 0.3;

export function sobreposicaoAlvo(r1: { width: number; height: number }, r2: { width: number; height: number }, areaSobreposta: number): boolean {
  const limiar = Math.min(r1.width * r1.height, r2.width * r2.height) * FRACAO_SOBREPOSICAO_ALVO;
  return areaSobreposta > limiar;
}

// ----- Barreira do tabuleiro -----
// As peças não podem ser retiradas da arena central: o deslocamento do arrasto é GRAMPEADO para a
// peça caber inteira dentro dela. Ao esbarrar num limite, aquela parede acende (feedback roxo).

// ParedesTocadas indica quais bordas da arena o arrasto está pressionando neste instante.
export interface ParedesTocadas {
  cima: boolean;
  baixo: boolean;
  esquerda: boolean;
  direita: boolean;
}

export const SEM_PAREDES: ParedesTocadas = { cima: false, baixo: false, esquerda: false, direita: false };

export function algumaParede(p: ParedesTocadas): boolean {
  return p.cima || p.baixo || p.esquerda || p.direita;
}

// grampearNaArena limita o delta bruto do arrasto para a peça (retângulo `slot` medido no início do
// gesto, já com o deslocamento vigente) não ultrapassar a `arena`. Devolve o delta grampeado e as
// paredes efetivamente pressionadas (quando o delta desejado foi cortado por um limite). Se a peça
// já é maior que a arena num eixo, aquele eixo não é grampeado (evita travar de vez).
export function grampearNaArena(
  slot: { left: number; top: number; width: number; height: number },
  arena: { left: number; top: number; right: number; bottom: number },
  deltaBrutoX: number,
  deltaBrutoY: number,
): { x: number; y: number; paredes: ParedesTocadas } {
  const paredes: ParedesTocadas = { cima: false, baixo: false, esquerda: false, direita: false };

  let x = deltaBrutoX;
  if (slot.width <= arena.right - arena.left) {
    const minX = arena.left - slot.left;
    const maxX = arena.right - (slot.left + slot.width);
    if (deltaBrutoX < minX) {
      x = minX;
      paredes.esquerda = true;
    } else if (deltaBrutoX > maxX) {
      x = maxX;
      paredes.direita = true;
    }
  }

  let y = deltaBrutoY;
  if (slot.height <= arena.bottom - arena.top) {
    const minY = arena.top - slot.top;
    const maxY = arena.bottom - (slot.top + slot.height);
    if (deltaBrutoY < minY) {
      y = minY;
      paredes.cima = true;
    } else if (deltaBrutoY > maxY) {
      y = maxY;
      paredes.baixo = true;
    }
  }

  return { x, y, paredes };
}

// ----- Deslize animado (conexão por clique) -----
// Ao ligar duas peças por CLIQUE (em vez de arrastar), as DUAS peças fazem um lerp suave uma em
// direção à outra até se encontrarem no meio — o "imã" visual do encaixe, mútuo.

// DURACAO_DESLIZE é o tempo (ms) do lerp de conexão por clique.
export const DURACAO_DESLIZE = 260;

// animarProgresso interpola um progresso suavizado (easeOutCubic) de 0 a 1 em `duracaoMs`, chamando
// `aoAtualizar` a cada quadro e `aoTerminar` ao fim. Devolve um cancelador (para desmontagem/nova
// animação). O chamador escala o progresso pelos vetores que precisar (permite animar duas peças em
// sincronia, cada uma com seu próprio deslocamento-alvo, a partir do mesmo progresso).
export function animarProgresso(duracaoMs: number, aoAtualizar: (suave: number) => void, aoTerminar: () => void): () => void {
  const inicio = performance.now();
  let cancelado = false;
  let raf = 0;

  const passo = (agora: number) => {
    if (cancelado) return;
    const t = Math.min(1, (agora - inicio) / duracaoMs);
    const suave = 1 - Math.pow(1 - t, 3); // easeOutCubic
    aoAtualizar(suave);
    if (t < 1) {
      raf = requestAnimationFrame(passo);
    } else {
      aoTerminar();
    }
  };

  raf = requestAnimationFrame(passo);
  return () => {
    cancelado = true;
    cancelAnimationFrame(raf);
  };
}
