import React, { useState, useEffect, useMemo, useRef, useLayoutEffect } from 'react';
import { main } from '../../wailsjs/go/models';
import { PopupRevisao } from './PopupRevisao';
import { BotaoAudio } from './BotaoAudio';
import { t } from '../i18n/i18n';

// ----- Seção: Revisão — Montagem de Frase -----
// Componente unificado para as variantes "ordenacao" e "fonetica_frase".
// O usuário monta a frase encaixando peças (hanzis isolados ou palavras compostas) nos slots.

interface ElementoPeca {
  texto: string;
  pinyin: string;
  definicao: string;
  correta: boolean;
  indicePilha: number; // índice original na pilha embaralhada
}

interface SlotState {
  indicePilha: number;
  texto: string;
  isSpanChild?: boolean;
}

interface MontagemFraseProps {
  questao: main.QuestaoRevisao;
  respondida: boolean;
  acertou: boolean | null;
  aoConcluir: (acertou: boolean) => void;
  aoTocarAudio: (texto: string) => void;
  hanziTocando: string | null;
  hanziSintetizando: string | null;
  // Leitura lenta ("tartaruga"): fala a frase palavra por palavra com pausas — só na fonética.
  aoTocarAudioLento?: () => void;
  audioLentoTocando?: boolean;
  audioLentoSintetizando?: boolean;
  AoClicarNoCartao?: (hanziInfo: { Hanzi: string, Pinyin: string, significados?: string[] }) => void;
}

// Retorna o caractere correspondente à posição 'idx' do slot, lidando com spans de multi-caracteres.
function obterCaractereNoSlot(slots: (SlotState | null)[], idx: number): string | null {
  const s = slots[idx];
  if (!s) return null;
  let startIdx = idx;
  while (startIdx > 0 && slots[startIdx - 1] && slots[startIdx - 1]!.indicePilha === s.indicePilha) {
    startIdx--;
  }
  const chars = [...s.texto];
  const offset = idx - startIdx;
  if (offset < chars.length) {
    return chars[offset];
  }
  return null;
}

// Retorna true se o slot de índice 'k' é parte (filho) de um bloco maior que começou antes.
function eSpanChild(slots: (SlotState | null)[], k: number): boolean {
  const s = slots[k];
  if (!s) return false;
  if (k > 0 && slots[k - 1] && slots[k - 1]!.indicePilha === s.indicePilha) {
    return true;
  }
  return false;
}

// Retorna true se o caractere no slot 'k' bate exatamente com o gabarito.
function eSlotCorreto(slots: (SlotState | null)[], E: string[], k: number, variante?: string): boolean {
  if (variante === 'ordenacao_traducao' || variante === 'contexto') {
    const s = slots[k];
    return s ? s.texto === E[k] : false;
  }
  const char = obterCaractereNoSlot(slots, k);
  if (char === null) return false;
  return char === E[k];
}

// Retorna todas as peças posicionadas no grid de slots
function obterPecasEmSlots(slots: (SlotState | null)[]): { start: number; length: number; state: SlotState }[] {
  const pecas: { start: number; length: number; state: SlotState }[] = [];
  let i = 0;
  while (i < slots.length) {
    const s = slots[i];
    if (s && !s.isSpanChild) {
      let length = 1;
      while (i + length < slots.length && slots[i + length] && slots[i + length]!.indicePilha === s.indicePilha) {
        length++;
      }
      pecas.push({ start: i, length, state: s });
      i += length;
    } else {
      i++;
    }
  }
  return pecas;
}

// Tenta acomodar uma peça de tamanho 'tamanhoL' a partir de 'targetIdx', empurrando recursivamente
// as peças que colidirem para a direita. Retorna a nova configuração de slots se der certo, ou null.
function tentarAcomodarPeca(
  slotsAtuais: (SlotState | null)[],
  targetIdx: number,
  tamanhoL: number,
  dadosPeca: { texto: string; indicePilha: number },
  bloqueados?: Set<number>
): (SlotState | null)[] | null {
  const novosSlots = [...slotsAtuais];

  if (targetIdx + tamanhoL > novosSlots.length) {
    return null;
  }

  // Se a peça já estava no grid, removemos para evitar colisão com ela mesma
  for (let i = 0; i < novosSlots.length; i++) {
    if (novosSlots[i] && novosSlots[i]!.indicePilha === dadosPeca.indicePilha) {
      novosSlots[i] = null;
    }
  }

  function empurrar(inicio: number, qtd: number): boolean {
    const fim = inicio + qtd - 1;
    if (fim >= novosSlots.length) {
      return false;
    }

    const pecasColidindo: { start: number; length: number; state: SlotState }[] = [];
    const todasPecas = obterPecasEmSlots(novosSlots);

    todasPecas.forEach(p => {
      const pFim = p.start + p.length - 1;
      const colide = !(pFim < inicio || p.start > fim);
      if (colide) {
        pecasColidindo.push(p);
      }
    });

    if (pecasColidindo.length === 0) {
      return true;
    }

    // Peças travadas (verdes, na posição correta na segunda chance) são muros: não podem ser
    // empurradas. Se a acomodação exigiria mover uma delas, a acomodação inteira falha.
    if (bloqueados && bloqueados.size > 0) {
      for (const p of pecasColidindo) {
        if (bloqueados.has(p.state.indicePilha)) {
          return false;
        }
      }
    }

    // Remove do grid temporariamente
    pecasColidindo.forEach(p => {
      for (let idx = p.start; idx < p.start + p.length; idx++) {
        novosSlots[idx] = null;
      }
    });

    pecasColidindo.sort((a, b) => a.start - b.start);
    let proximaPosicaoLivre = inicio + qtd;

    for (let i = 0; i < pecasColidindo.length; i++) {
      const p = pecasColidindo[i];
      const destino = Math.max(p.start, proximaPosicaoLivre);

      const ok = empurrar(destino, p.length);
      if (!ok) {
        return false;
      }

      novosSlots[destino] = {
        indicePilha: p.state.indicePilha,
        texto: p.state.texto
      };
      for (let idx = 1; idx < p.length; idx++) {
        novosSlots[destino + idx] = {
          indicePilha: p.state.indicePilha,
          texto: p.state.texto,
          isSpanChild: true
        };
      }

      proximaPosicaoLivre = destino + p.length;
    }

    return true;
  }

  const sucesso = empurrar(targetIdx, tamanhoL);
  if (!sucesso) {
    return null;
  }

  novosSlots[targetIdx] = {
    indicePilha: dadosPeca.indicePilha,
    texto: dadosPeca.texto
  };
  for (let idx = 1; idx < tamanhoL; idx++) {
    novosSlots[targetIdx + idx] = {
      indicePilha: dadosPeca.indicePilha,
      texto: dadosPeca.texto,
      isSpanChild: true
    };
  }

  return novosSlots;
}

// Helper para encontrar segmentos consecutivos corretos nos slots baseando-se no gabarito
function obterSegmentosFusoes(S: (SlotState | null)[], E: string[], variante?: string) {
  const segmentos: { start: number, end: number, fusionado: boolean }[] = [];
  let i = 0;
  while (i < S.length) {
    if (obterCaractereNoSlot(S, i) === null || eSlotCorreto(S, E, i, variante)) {
      segmentos.push({ start: i, end: i, fusionado: false });
      i++;
      continue;
    }

    let maxLen = 1;
    let maxIdxE = -1;
    for (let idxE = 0; idxE <= E.length - maxLen; idxE++) {
      let len = 0;
      while (
        i + len < S.length &&
        idxE + len < E.length &&
        obterCaractereNoSlot(S, i + len) !== null &&
        obterCaractereNoSlot(S, i + len) === E[idxE + len] &&
        !eSlotCorreto(S, E, i + len, variante) // Não pode ser uma peça correta na posição correta!
      ) {
        len++;
      }
      if (len > maxLen) {
        maxLen = len;
        maxIdxE = idxE;
      }
    }

    if (maxLen >= 2 && maxIdxE !== -1) {
      // REGRA CRÍTICA: Se qualquer uma das posições de destino correspondentes no gabarito E
      // já estiver preenchida de forma correta e travada nos slots, então a fusão é inválida.
      let destinoComprometido = false;
      for (let pos = maxIdxE; pos < maxIdxE + maxLen; pos++) {
        if (eSlotCorreto(S, E, pos, variante)) {
          destinoComprometido = true;
          break;
        }
      }

      if (!destinoComprometido) {
        segmentos.push({ start: i, end: i + maxLen - 1, fusionado: true });
        i += maxLen;
        continue;
      }
    }

    segmentos.push({ start: i, end: i, fusionado: false });
    i++;
  }
  return segmentos;
}

export function MontagemFrase({ questao, respondida, acertou, aoConcluir, aoTocarAudio, hanziTocando, hanziSintetizando, aoTocarAudioLento, audioLentoTocando, audioLentoSintetizando, AoClicarNoCartao }: MontagemFraseProps) {
  const ehFoneticaFrase = questao.variante === 'fonetica_frase';
  const ehOrdenacaoTraducao = questao.variante === 'ordenacao_traducao';
  const ehContexto = questao.variante === 'contexto';
  
  // Estado para gerenciar dinamicamente as peças ativas na pilha (pode sofrer fusão)
  const [pecasAtivas, setPecasAtivas] = useState<ElementoPeca[]>([]);
  // Estado para gerenciar dinamicamente o gabarito de peças esperadas (sempre em hanzis individuais para manter o comprimento de slots constante)
  const [pecasEsperadasAtivas, setPecasEsperadasAtivas] = useState<string[]>([]);

  const qtdSlots = pecasEsperadasAtivas.length;
  const [slots, setSlots] = useState<(SlotState | null)[]>([]);
  const [statusSlots, setStatusSlots] = useState<('correto' | 'posicao_errada' | 'errado' | null)[]>([]);
  const [tentativas, setTentativas] = useState(0);
  const [usadosNaPilha, setUsadosNaPilha] = useState<boolean[]>([]);
  // Ordem dinâmica das peças que estão descansando na pilha (por indicePilha). Peça colocada num
  // slot sai da lista; peça devolvida entra na posição do encaixe (ou no fim, se por clique).
  const [ordemPilha, setOrdemPilha] = useState<number[]>([]);
  const [popupInfo, setPopupInfo] = useState<{hanzi: string, pinyin: string, significados: string, x: number, y: number} | null>(null);

  // Estados de arrasto customizado
  const [draggedPiece, setDraggedPiece] = useState<{
    type: 'from_pile' | 'from_slot';
    index: number;
    texto: string;
    indicePilhaOriginal: number;
    width: number;
  } | null>(null);
  const [isDragging, setIsDragging] = useState(false);
  const [currentHoverSlotIdx, setCurrentHoverSlotIdx] = useState<number | null>(null);
  // Índice de inserção na pilha enquanto o arrasto paira sobre ela (abre um vão para "encaixar").
  const [currentHoverPilhaIdx, setCurrentHoverPilhaIdx] = useState<number | null>(null);

  // ----- Fatos persistentes da segunda chance (por peça, definidos na entrada) -----
  // Verdes: peças corretas e na posição certa. Ficam travadas (imóveis e intransponíveis).
  const [pecasTravadas, setPecasTravadas] = useState<Set<number>>(new Set());
  // Amarelas: peça que pertence à frase mas está mal posicionada → guarda a posição de origem
  // (início do slot) da primeira tentativa. Reencaixá-la ali volta a pintá-la de amarelo.
  const [posicaoAmarelaOriginal, setPosicaoAmarelaOriginal] = useState<Map<number, number>>(new Map());
  // Vermelhas reveladas: distratores que já apareceram num slot. Continuam vermelhos na pilha.
  const [distratoresRevelados, setDistratoresRevelados] = useState<Set<number>>(new Set());

  const indicesTravados = pecasTravadas;

  // Segunda chance: se sobraram só distratores na pilha, todos ficam vermelhos.
  const pilhaSoDistratores = React.useMemo(() => {
    return tentativas >= 1 && ordemPilha.length > 0 &&
      ordemPilha.every(i => pecasAtivas[i] && !pecasAtivas[i].correta);
  }, [tentativas, ordemPilha, pecasAtivas]);

  // Uma peça na pilha aparece vermelha quando é um distrator já revelado (passou por um slot)
  // ou quando a pilha inteira virou distratores.
  function pecaPilhaVermelha(idx: number): boolean {
    if (tentativas < 1) return false;
    const peca = pecasAtivas[idx];
    if (!peca || peca.correta) return false;
    return distratoresRevelados.has(idx) || pilhaSoDistratores;
  }

  // Refs de arrasto e cache de coordenadas para alta performance (120fps+)
  const dragInfoRef = useRef<{
    type: 'from_pile' | 'from_slot';
    index: number;
    texto: string;
    indicePilhaOriginal: number;
    offsetX: number;
    offsetY: number;
    width: number;
  } | null>(null);
  const floatingCardRef = useRef<HTMLDivElement | null>(null);
  // Container estável usado como alvo da captura do ponteiro: ao contrário das peças da pilha
  // (que desmontam ao virar placeholder), ele nunca sai do DOM, então a captura não se perde.
  const containerRef = useRef<HTMLDivElement | null>(null);
  const slotBoundsRef = useRef<{
    idx: number;
    left: number;
    right: number;
    top: number;
    bottom: number;
    centerX: number;
    centerY: number;
    width: number;
    height: number;
    isChild: boolean;
  }[]>([]);
  const containerBoundsRef = useRef<{
    top: number;
    bottom: number;
    left: number;
    right: number;
  } | null>(null);
  // Bounds vivos da pilha e de suas peças, para detectar o índice de encaixe durante o arrasto.
  const pilhaBoundsRef = useRef<{
    top: number;
    bottom: number;
    left: number;
    right: number;
  } | null>(null);
  const pilhaItemBoundsRef = useRef<{ oi: number; centerX: number; centerY: number }[]>([]);
  const hoverPilhaIdxRef = useRef<number | null>(null);

  // Coalescência de movimento por requestAnimationFrame: garante tracking 1:1 do cursor
  // sem disparar reflow/detecção de hover por evento de ponteiro (que chegam mais rápido que o frame).
  const rafPendenteRef = useRef<number | null>(null);
  const ultimoPointerRef = useRef<{ x: number; y: number } | null>(null);
  const hoverIdxRef = useRef<number | null>(null);

  interface LinhaSlotsBounds {
    top: number;
    bottom: number;
    centerY: number;
    slots: typeof slotBoundsRef.current;
  }

  function findHoverSlotIndexCached(clientX: number, clientY: number): number | null {
    const bounds = containerBoundsRef.current;
    const slotsArr = slotBoundsRef.current;
    if (!bounds || slotsArr.length === 0) return null;

    // Margem generosa ao redor do container da frase para facilitar o arraste vindo da pilha
    const margemYTopo = 30;
    const margemYFundo = 30;
    const margemX = 60;

    if (
      clientY < bounds.top - margemYTopo ||
      clientY > bounds.bottom + margemYFundo ||
      clientX < bounds.left - margemX ||
      clientX > bounds.right + margemX
    ) {
      return null;
    }

    // Agrupa os slots em linhas visuais ordenadas com base no alinhamento vertical
    const linhas: LinhaSlotsBounds[] = [];
    const slotsOrdenados = [...slotsArr].sort((a, b) => {
      if (Math.abs(a.centerY - b.centerY) > 14) {
        return a.centerY - b.centerY;
      }
      return a.left - b.left;
    });

    slotsOrdenados.forEach(slot => {
      const linhaExistente = linhas.find(l => Math.abs(l.centerY - slot.centerY) <= 16);
      if (linhaExistente) {
        linhaExistente.slots.push(slot);
        linhaExistente.top = Math.min(linhaExistente.top, slot.top);
        linhaExistente.bottom = Math.max(linhaExistente.bottom, slot.bottom);
        linhaExistente.centerY = (linhaExistente.top + linhaExistente.bottom) / 2;
      } else {
        linhas.push({
          top: slot.top,
          bottom: slot.bottom,
          centerY: slot.centerY,
          slots: [slot],
        });
      }
    });

    if (linhas.length === 0) return null;

    // Ordena os slots dentro de cada linha pela coordenada horizontal (esquerda -> direita)
    linhas.forEach(l => l.slots.sort((a, b) => a.left - b.left));
    linhas.sort((a, b) => a.centerY - b.centerY);

    // Etapa 1: Encontrar a linha visual mais próxima verticalmente de clientY
    let melhorLinha = linhas[0];
    let menorDistY = Infinity;
    let menorDistCentroY = Infinity;

    for (const linha of linhas) {
      let distY = 0;
      if (clientY < linha.top) {
        distY = linha.top - clientY;
      } else if (clientY > linha.bottom) {
        distY = clientY - linha.bottom;
      }

      const distCentroY = Math.abs(clientY - linha.centerY);

      if (distY < menorDistY || (distY === menorDistY && distCentroY < menorDistCentroY)) {
        menorDistY = distY;
        menorDistCentroY = distCentroY;
        melhorLinha = linha;
      }
    }

    // Etapa 2: Dentro da linha escolhida, encontrar o slot correspondente a clientX
    const slotsDaLinha = melhorLinha.slots;
    if (slotsDaLinha.length === 0) return null;

    // Se estiver à esquerda de todos os slots da linha
    if (clientX <= slotsDaLinha[0].left) {
      return slotsDaLinha[0].idx;
    }

    // Se estiver à direita de todos os slots da linha
    const ultimoSlot = slotsDaLinha[slotsDaLinha.length - 1];
    if (clientX >= ultimoSlot.right) {
      return ultimoSlot.idx;
    }

    // Busca direta se o cursor estiver contido no intervalo horizontal do slot
    for (const s of slotsDaLinha) {
      if (clientX >= s.left && clientX <= s.right) {
        return s.idx;
      }
    }

    // Caso caia em um vão/gap entre slots, escolhe o de centro mais próximo na mesma linha
    let slotMaisProximo = slotsDaLinha[0];
    let menorDistX = Infinity;

    for (const s of slotsDaLinha) {
      const distX = Math.abs(clientX - s.centerX);
      if (distX < menorDistX) {
        menorDistX = distX;
        slotMaisProximo = s;
      }
    }

    return slotMaisProximo.idx;
  }

  // Relê as posições vivas dos slots e do container. Necessário a cada frame porque a
  // pré-visualização empurra os vizinhos e desloca os alvos de hit-testing durante o arrasto.
  function recalcularBoundsSlots() {
    const slotElements = document.querySelectorAll('.revisao-ordenacao-frase .revisao-ordenacao-slot');
    const arr: typeof slotBoundsRef.current = [];
    slotElements.forEach((el, fallbackIdx) => {
      const htmlEl = el as HTMLElement;
      if (htmlEl.classList.contains('slot-child')) return;
      const rawSlotIdx = htmlEl.dataset.slotIdx;
      const slotIdx = rawSlotIdx !== undefined ? parseInt(rawSlotIdx, 10) : fallbackIdx;
      if (isNaN(slotIdx) || slotIdx < 0) return;

      const rect = el.getBoundingClientRect();
      arr.push({
        idx: slotIdx,
        left: rect.left,
        right: rect.right,
        top: rect.top,
        bottom: rect.bottom,
        centerX: rect.left + rect.width / 2,
        centerY: rect.top + rect.height / 2,
        width: rect.width,
        height: rect.height,
        isChild: false,
      });
    });
    slotBoundsRef.current = arr;

    const slotsContainer = document.querySelector('.revisao-ordenacao-frase');
    if (slotsContainer) {
      const rect = slotsContainer.getBoundingClientRect();
      containerBoundsRef.current = {
        top: rect.top,
        bottom: rect.bottom,
        left: rect.left,
        right: rect.right,
      };
    }
  }

  // Relê a posição viva da pilha e das suas peças. Como a pilha é dinâmica (colapsa/afasta),
  // os centros mudam durante o arrasto, então recalculamos a cada frame igual aos slots.
  function recalcularBoundsPilha() {
    const cont = document.querySelector('.revisao-ordenacao-pilha');
    if (cont) {
      const rect = cont.getBoundingClientRect();
      pilhaBoundsRef.current = { top: rect.top, bottom: rect.bottom, left: rect.left, right: rect.right };
    } else {
      pilhaBoundsRef.current = null;
    }

    const itens = document.querySelectorAll('.revisao-pilha-item');
    const arr: typeof pilhaItemBoundsRef.current = [];
    itens.forEach(el => {
      const oi = parseInt((el as HTMLElement).dataset.oi || '-1', 10);
      if (oi < 0) return;
      const rect = el.getBoundingClientRect();
      arr.push({ oi, centerX: rect.left + rect.width / 2, centerY: rect.top + rect.height / 2 });
    });
    arr.sort((a, b) => a.oi - b.oi);
    pilhaItemBoundsRef.current = arr;
  }

  // Retorna o índice de inserção na ordem da pilha (0..N) para as coordenadas do ponteiro.
  function findInsertPileIndexCached(clientX: number, clientY: number): number {
    const itens = pilhaItemBoundsRef.current;
    if (itens.length === 0) return 0;

    let maisProximo = itens[0];
    let melhorDist = Infinity;
    itens.forEach(item => {
      const dx = clientX - item.centerX;
      const dy = clientY - item.centerY;
      const dist = dx * dx + dy * dy;
      if (dist < melhorDist) {
        melhorDist = dist;
        maisProximo = item;
      }
    });

    return clientX < maisProximo.centerX ? maisProximo.oi : maisProximo.oi + 1;
  }

  // Executado uma vez por frame durante o arrasto: reposiciona a peça flutuante (via transform,
  // compositor puro) e atualiza o slot em hover só quando ele realmente muda.
  function processarFrameDrag() {
    rafPendenteRef.current = null;
    const pos = ultimoPointerRef.current;
    const info = dragInfoRef.current;
    const card = floatingCardRef.current;
    if (!pos || !info || !card) return;

    card.style.transform = `translate3d(${pos.x - info.offsetX}px, ${pos.y - info.offsetY}px, 0) scale(1.04) rotate(1.5deg)`;

    recalcularBoundsSlots();
    recalcularBoundsPilha();

    // Prioridade da pilha: se o ponteiro está sobre a área da pilha, o alvo é o encaixe entre
    // peças; caso contrário, testamos os slots do gabarito. Nunca os dois ao mesmo tempo.
    const pilhaBounds = pilhaBoundsRef.current;
    const sobrePilha = !!pilhaBounds && pos.y >= pilhaBounds.top - 12 && pos.y <= pilhaBounds.bottom + 12;

    let hoverSlot: number | null = null;
    let hoverPilha: number | null = null;
    if (sobrePilha) {
      hoverPilha = findInsertPileIndexCached(pos.x, pos.y);
    } else {
      hoverSlot = findHoverSlotIndexCached(pos.x, pos.y);
    }

    if (hoverSlot !== hoverIdxRef.current) {
      hoverIdxRef.current = hoverSlot;
      setCurrentHoverSlotIdx(hoverSlot);
    }
    if (hoverPilha !== hoverPilhaIdxRef.current) {
      hoverPilhaIdxRef.current = hoverPilha;
      setCurrentHoverPilhaIdx(hoverPilha);
    }
  }

  function agendarFrameDrag() {
    if (rafPendenteRef.current == null) {
      rafPendenteRef.current = requestAnimationFrame(processarFrameDrag);
    }
  }

  // Cancela qualquer frame pendente ao desmontar o componente.
  useEffect(() => {
    return () => {
      if (rafPendenteRef.current != null) {
        cancelAnimationFrame(rafPendenteRef.current);
      }
    };
  }, []);

  // Para animar a transição FLIP no clique ou deslocamento
  const lastPositionsRef = useRef<Record<string, DOMRect>>({});
  const pecaEmTransicaoRef = useRef<string | null>(null);

  // A fusão da segunda chance preenche todos os slots de novo; sem isso o efeito de validação
  // dispararia na hora e encerraria a rodada. Este flag pula essa validação espúria imediata.
  const ignorarProximaValidacaoRef = useRef(false);

  const pointerDownInfoRef = useRef<{
    type: 'from_pile' | 'from_slot';
    index: number;
    startX: number;
    startY: number;
    pointerId: number;
    // Offset do ponto de agarre dentro da peça + largura, medidos no pointerdown.
    // Guardados aqui porque o move/up é tratado no container (e.currentTarget != peça).
    offsetX: number;
    offsetY: number;
    width: number;
  } | null>(null);

  useLayoutEffect(() => {
    // Coleta as posições atuais (Last) de todos os elementos com data-flip-id
    const elementos = document.querySelectorAll('[data-flip-id]');
    
    elementos.forEach(el => {
      const id = el.getAttribute('data-flip-id');
      if (id && lastPositionsRef.current[id]) {
        // Anima (FLIP) a peça que está voando para um slot E as peças da pilha, que deslizam
        // ao colapsar o vão de uma peça retirada ou ao se afastarem para abrir o encaixe.
        const ehPilha = (el as HTMLElement).dataset.flipScope === 'pilha';
        if (id !== pecaEmTransicaoRef.current && !ehPilha) {
          return;
        }

        const rectAntigo = lastPositionsRef.current[id];
        
        // Remove temporariamente a transição ativa e o transform para medir a posição real "limpa" do layout
        const htmlEl = el as HTMLElement;
        const prevTransform = htmlEl.style.transform;
        const prevTransition = htmlEl.style.transition;
        htmlEl.style.transform = '';
        htmlEl.style.transition = '';
        
        const rectNovo = htmlEl.getBoundingClientRect();
        
        const dx = rectAntigo.left - rectNovo.left;
        const dy = rectAntigo.top - rectNovo.top;
        
        if (dx !== 0 || dy !== 0) {
          // 1. Inverte (Invert)
          htmlEl.style.transition = 'none';
          htmlEl.style.transform = `translate3d(${dx}px, ${dy}px, 0)`;
          
          // Força reflow
          htmlEl.offsetHeight;
          
          // 2. Play com transição suave
          htmlEl.style.transition = 'transform 0.3s cubic-bezier(0.2, 0.8, 0.2, 1)';
          htmlEl.style.transform = 'translate3d(0, 0, 0)';
          
          const limparEstilos = (e: any) => {
            if (e.propertyName === 'transform') {
              htmlEl.style.transition = '';
              htmlEl.style.transform = '';
              htmlEl.removeEventListener('transitionend', limparEstilos);
            }
          };
          htmlEl.addEventListener('transitionend', limparEstilos);
        } else {
          // Se não mudou de lugar, restaura os estilos pré-existentes
          htmlEl.style.transform = prevTransform;
          htmlEl.style.transition = prevTransition;
        }
      }
    });

    // Consome e reseta a peça em transição após o processamento das animações do render corrente
    pecaEmTransicaoRef.current = null;

    // Salva as posições atuais "limpas" para o próximo render
    const novasPosicoes: Record<string, DOMRect> = {};
    elementos.forEach(el => {
      const id = el.getAttribute('data-flip-id');
      if (id) {
        const htmlEl = el as HTMLElement;
        const prevTransform = htmlEl.style.transform;
        const prevTransition = htmlEl.style.transition;
        htmlEl.style.transform = '';
        htmlEl.style.transition = '';
        
        novasPosicoes[id] = htmlEl.getBoundingClientRect();
        
        htmlEl.style.transform = prevTransform;
        htmlEl.style.transition = prevTransition;
      }
    });
    lastPositionsRef.current = novasPosicoes;
  }, [slots, usadosNaPilha, ordemPilha, currentHoverPilhaIdx]);

  // Inicializa os estados a partir da questão recebida
  useEffect(() => {
    const pecasOriginais: ElementoPeca[] = (() => {
      const originalElementos = (questao as any).elementosOrdenacao;
      if (originalElementos && originalElementos.length > 0) {
        return originalElementos.map((e: any, i: number) => ({
          texto: e.texto,
          pinyin: e.pinyin,
          definicao: e.definicao,
          correta: e.correta,
          indicePilha: i,
        }));
      }
      return (questao.pilhaOrdenacao || []).map((o: any, i: number) => ({
        texto: o.hanzi,
        pinyin: o.pinyin,
        definicao: o.definicao,
        correta: o.correta,
        indicePilha: i,
      }));
    })();

    const esperadasOriginais: string[] = (() => {
      const origPecas = (questao as any).pecasEsperadas;
      if (questao.variante === 'ordenacao_traducao' || questao.variante === 'contexto') {
        return origPecas || [];
      }
      const baseList = (origPecas && origPecas.length > 0) ? origPecas : [questao.fraseOriginal || ''];
      const esperadas: string[] = [];
      for (const item of baseList) {
        for (const ch of item) {
          if (/[\u4e00-\u9fff]/.test(ch)) {
            esperadas.push(ch);
          }
        }
      }
      return esperadas;
    })();

    setPecasAtivas(pecasOriginais);
    setPecasEsperadasAtivas(esperadasOriginais);

    const qtd = esperadasOriginais.length;
    setSlots(new Array(qtd).fill(null));
    setStatusSlots(new Array(qtd).fill(null));
    setTentativas(0);
    setUsadosNaPilha(new Array(pecasOriginais.length).fill(false));
    setOrdemPilha(pecasOriginais.map(p => p.indicePilha));
    setDraggedPiece(null);
    setCurrentHoverSlotIdx(null);
    setCurrentHoverPilhaIdx(null);
    setPecasTravadas(new Set());
    setPosicaoAmarelaOriginal(new Map());
    setDistratoresRevelados(new Set());
  }, [questao]);

  // Índice do slot em que a peça de 'i' começa (lida com blocos multi-hanzi).
  function inicioDoSlot(slotsArr: (SlotState | null)[], i: number): number {
    const s = slotsArr[i];
    if (!s) return i;
    let start = i;
    while (start > 0 && slotsArr[start - 1] && slotsArr[start - 1]!.indicePilha === s.indicePilha) {
      start--;
    }
    return start;
  }

  // Extrai os fatos de cor da segunda chance a partir de um grid: quais peças estão totalmente
  // corretas (verde/travada), quais pertencem à frase mas mal posicionadas (amarela, guardando a
  // posição de origem) e quais são distratores já visíveis num slot (vermelhas reveladas).
  function extrairFatosSegundaChance(slotsArr: (SlotState | null)[], pecas: ElementoPeca[]) {
    const travadas = new Set<number>();
    const amareloMap = new Map<number, number>();
    const revelados = new Set<number>();

    obterPecasEmSlots(slotsArr).forEach(({ start, length, state }) => {
      const idx = state.indicePilha;
      const peca = pecas[idx];
      if (!peca) return;

      let todosCorretos = true;
      for (let k = start; k < start + length; k++) {
        if (!eSlotCorreto(slotsArr, pecasEsperadasAtivas, k, questao.variante)) {
          todosCorretos = false;
          break;
        }
      }

      if (todosCorretos) {
        travadas.add(idx);
      } else if (!peca.correta) {
        revelados.add(idx);
      } else {
        amareloMap.set(idx, start);
      }
    });

    return { travadas, amareloMap, revelados };
  }

  // Deriva o status de cada slot a partir dos fatos persistentes: verde (travada), vermelho
  // (distrator num slot), amarelo (peça da frase de volta à posição da 1ª tentativa), ou neutro.
  function statusDeFatos(
    slotsArr: (SlotState | null)[],
    pecas: ElementoPeca[],
    travadas: Set<number>,
    amareloMap: Map<number, number>
  ): ('correto' | 'posicao_errada' | 'errado' | null)[] {
    return slotsArr.map((s, i) => {
      if (!s) return null;
      const idx = s.indicePilha;
      if (travadas.has(idx)) return 'correto';
      const peca = pecas[idx];
      if (peca && !peca.correta) return 'errado';
      if (peca && peca.correta && amareloMap.get(idx) === inicioDoSlot(slotsArr, i)) {
        return 'posicao_errada';
      }
      return null;
    });
  }

  // Recalcula o status dos slots após uma jogada. Só colore na segunda chance; na 1ª tentativa
  // tudo fica neutro. Assim, retirar/colocar uma peça mexe só na cor dela, sem apagar as demais.
  function recomputarStatusSlots(slotsArr: (SlotState | null)[]) {
    if (tentativas < 1) {
      return slotsArr.map(() => null);
    }
    return statusDeFatos(slotsArr, pecasAtivas, pecasTravadas, posicaoAmarelaOriginal);
  }

  function clicarPeca(indice: number) {
    if (respondida || usadosNaPilha[indice]) return;
    const peca = pecasAtivas[indice];
    const tamanhoL = (ehOrdenacaoTraducao || ehContexto) ? 1 : [...peca.texto].length;

    // Encontra o primeiro índice nos slots onde a peça cabe sem deslocar nada (sem pushing)
    let targetIdx = -1;
    for (let i = 0; i <= slots.length - tamanhoL; i++) {
      let fit = true;
      for (let j = 0; j < tamanhoL; j++) {
        if (slots[i + j] !== null) {
          fit = false;
          break;
        }
      }
      if (fit) {
        targetIdx = i;
        break;
      }
    }

    if (targetIdx === -1) return; // Nega se não couber em lugar nenhum

    pecaEmTransicaoRef.current = `card-${peca.indicePilha}`;
    if (!ehOrdenacaoTraducao) {
      aoTocarAudio(peca.texto);
    }

    const novosSlots = [...slots];
    novosSlots[targetIdx] = { indicePilha: peca.indicePilha, texto: peca.texto };
    for (let idx = 1; idx < tamanhoL; idx++) {
      novosSlots[targetIdx + idx] = { indicePilha: peca.indicePilha, texto: peca.texto, isSpanChild: true };
    }
    setSlots(novosSlots);

    const novosUsados = [...usadosNaPilha];
    novosUsados[indice] = true;
    setUsadosNaPilha(novosUsados);

    // Sai da pilha; as peças à direita colapsam para preencher o vácuo.
    setOrdemPilha(prev => prev.filter(x => x !== indice));

    // Recalcula as cores a partir dos fatos: mexe só na cor da peça movida, sem apagar as outras.
    setStatusSlots(recomputarStatusSlots(novosSlots));
  }

  function removerSlot(indiceSlot: number) {
    if (respondida || slots[indiceSlot] === null) return;
    if (statusSlots[indiceSlot] === 'correto') return;

    const s = slots[indiceSlot]!;
    const novosSlots = [...slots];
    const novosUsados = [...usadosNaPilha];

    // Remove todas as posições ocupadas por esta peça (mesmo indicePilha)
    for (let idx = 0; idx < novosSlots.length; idx++) {
      if (novosSlots[idx] && novosSlots[idx]!.indicePilha === s.indicePilha) {
        novosSlots[idx] = null;
      }
    }

    novosUsados[s.indicePilha] = false;

    setSlots(novosSlots);
    setUsadosNaPilha(novosUsados);
    // Retirada por clique devolve a peça para a última posição da pilha.
    setOrdemPilha(prev => [...prev.filter(x => x !== s.indicePilha), s.indicePilha]);
    setStatusSlots(recomputarStatusSlots(novosSlots));
  }

  function validar() {
    if (slots.includes(null)) return;

    let todosCorretos = true;
    for (let i = 0; i < pecasEsperadasAtivas.length; i++) {
      if (!eSlotCorreto(slots, pecasEsperadasAtivas, i, questao.variante)) {
        todosCorretos = false;
        break;
      }
    }

    if (todosCorretos) {
      aoConcluir(true);
      return;
    }

    // Se errou na primeira tentativa e possui boa margem, dá uma segunda chance. Para hanzis, tenta
    // fundir os hanzis adjacentes corretos; a tradução (palavras em inglês) vai direto à 2ª chance.
    if (tentativas === 0) {
      let slotsCorretos = 0;
      slots.forEach((s, i) => {
        if (!s || eSpanChild(slots, i)) return;
        if (pecasAtivas[s.indicePilha].correta) {
          // Hanzi: cada peça ocupa tantos slots quantos hanzis tem; tradução/contexto: 1 slot por palavra.
          slotsCorretos += (ehOrdenacaoTraducao || ehContexto) ? 1 : [...s.texto].length;
        }
      });
      const percent = slotsCorretos / qtdSlots;
      if (percent >= 0.75) {
        // Fusão só faz sentido para hanzis adjacentes; na tradução e contexto não há o que fundir.
        const segmentos = (ehOrdenacaoTraducao || ehContexto) ? [] : obterSegmentosFusoes(slots, pecasEsperadasAtivas, questao.variante);
        const temFusoes = segmentos.some(seg => seg.fusionado);

        if (temFusoes) {
          const indicesPilhaFusionados = new Set<number>();
          segmentos.forEach(seg => {
            if (seg.fusionado) {
              for (let idx = seg.start; idx <= seg.end; idx++) {
                const s = slots[idx];
                if (s) {
                  indicesPilhaFusionados.add(s.indicePilha);
                }
              }
            }
          });

          const novasPecasAtivas: ElementoPeca[] = [];
          const novosSlotsValores: (SlotState | null)[] = new Array(slots.length).fill(null);
          let proximoIndicePilha = 0;

          const mapaIndicePilha = new Map<number, number>();

          // 1. Processa e insere as peças fusionadas
          segmentos.forEach(seg => {
            if (seg.fusionado) {
              const textoFusionado = slots.slice(seg.start, seg.end + 1).map((_, idx) => obterCaractereNoSlot(slots, seg.start + idx)).join('');
              
              const pecasNoSegmento: ElementoPeca[] = [];
              for (let idx = seg.start; idx <= seg.end; idx++) {
                const s = slots[idx];
                if (s) {
                  const p = pecasAtivas[s.indicePilha];
                  if (p && !pecasNoSegmento.some(item => item.indicePilha === p.indicePilha)) {
                    pecasNoSegmento.push(p);
                  }
                }
              }
              
              const pinyins = pecasNoSegmento.map(p => p.pinyin).filter(Boolean);
              const definicoes = pecasNoSegmento.map(p => p.definicao).filter(Boolean);
              
              const novoIdx = proximoIndicePilha++;
              const novaPeca: ElementoPeca = {
                texto: textoFusionado,
                pinyin: pinyins.join(' '),
                definicao: Array.from(new Set(definicoes)).join(' ; '),
                correta: true,
                indicePilha: novoIdx
              };
              novasPecasAtivas.push(novaPeca);

              // Preenche os slots para a peça fusionada
              novosSlotsValores[seg.start] = {
                texto: textoFusionado,
                indicePilha: novoIdx
              };
              for (let idx = seg.start + 1; idx <= seg.end; idx++) {
                novosSlotsValores[idx] = {
                  texto: textoFusionado,
                  indicePilha: novoIdx,
                  isSpanChild: true
                };
              }
            }
          });

          // 2. Adiciona as peças antigas que não foram fusionadas à nova pilha
          pecasAtivas.forEach(p => {
            if (!indicesPilhaFusionados.has(p.indicePilha)) {
              const novoIdx = proximoIndicePilha++;
              mapaIndicePilha.set(p.indicePilha, novoIdx);
              novasPecasAtivas.push({
                ...p,
                indicePilha: novoIdx
              });
            }
          });

          // 3. Copia todas as peças não fusionadas de volta para seus slots de origem, remapeando os índices
          for (let idx = 0; idx < slots.length; idx++) {
            if (novosSlotsValores[idx] !== null) {
              continue;
            }
            
            const s = slots[idx];
            if (s) {
              const novoIdx = mapaIndicePilha.get(s.indicePilha);
              if (novoIdx !== undefined) {
                novosSlotsValores[idx] = {
                  texto: s.texto,
                  indicePilha: novoIdx,
                  isSpanChild: s.isSpanChild
                };
              }
            }
          }

          // Atualiza os estados finais para a nova rodada
          setPecasAtivas(novasPecasAtivas);

          const novosUsados = new Array(novasPecasAtivas.length).fill(false);
          novosSlotsValores.forEach(s => {
            if (s) {
              novosUsados[s.indicePilha] = true;
            }
          });

          ignorarProximaValidacaoRef.current = true;
          setSlots(novosSlotsValores);
          setUsadosNaPilha(novosUsados);
          // Reconstrói a ordem da pilha com as peças que sobraram (não colocadas em slots).
          setOrdemPilha(novasPecasAtivas.filter(p => !novosUsados[p.indicePilha]).map(p => p.indicePilha));
          setTentativas(1);

          // Deriva e guarda os fatos de cor (verde/amarelo/vermelho) desta segunda chance.
          const fatos = extrairFatosSegundaChance(novosSlotsValores, novasPecasAtivas);
          setPecasTravadas(fatos.travadas);
          setPosicaoAmarelaOriginal(fatos.amareloMap);
          setDistratoresRevelados(fatos.revelados);
          setStatusSlots(statusDeFatos(novosSlotsValores, novasPecasAtivas, fatos.travadas, fatos.amareloMap));
          return;
        } else {
          // Segunda chance clássica sem fusões: guarda os fatos e destaca as cores iniciais.
          const fatos = extrairFatosSegundaChance(slots, pecasAtivas);
          setPecasTravadas(fatos.travadas);
          setPosicaoAmarelaOriginal(fatos.amareloMap);
          setDistratoresRevelados(fatos.revelados);
          setStatusSlots(statusDeFatos(slots, pecasAtivas, fatos.travadas, fatos.amareloMap));
          setTentativas(1);
          return;
        }
      }
    }

    const novosStatus = slots.map((s, i) => {
      if (eSlotCorreto(slots, pecasEsperadasAtivas, i, questao.variante)) return 'correto' as const;
      if (pecasAtivas[s!.indicePilha].correta) return 'posicao_errada' as const;
      return 'errado' as const;
    });
  setStatusSlots(novosStatus);
    aoConcluir(false);
  }

  useEffect(() => {
    if (ignorarProximaValidacaoRef.current) {
      ignorarProximaValidacaoRef.current = false;
      return;
    }
    if (!respondida && !slots.includes(null) && slots.length > 0) {
      validar();
    }
  }, [slots, respondida]);

  // Na segunda chance, todo distrator que passa por um slot fica "revelado": continua vermelho
  // mesmo depois de retirado para a pilha (as palavras vermelhas seguem vermelhas).
  useEffect(() => {
    if (tentativas < 1) return;
    setDistratoresRevelados(prev => {
      let mudou = false;
      const proximo = new Set(prev);
      slots.forEach(s => {
        if (s && pecasAtivas[s.indicePilha] && !pecasAtivas[s.indicePilha].correta && !proximo.has(s.indicePilha)) {
          proximo.add(s.indicePilha);
          mudou = true;
        }
      });
      return mudou ? proximo : prev;
    });
  }, [slots, tentativas, pecasAtivas]);

  // Se a pilha tiver apenas distratores (todas as corretas foram alocadas nos slots),
  // revelamos imediatamente todos esses distratores para que nunca percam a cor vermelha ao mover peças.
  useEffect(() => {
    if (pilhaSoDistratores) {
      setDistratoresRevelados(prev => {
        const proximo = new Set(prev);
        let mudou = false;
        ordemPilha.forEach(idx => {
          if (!proximo.has(idx)) {
            proximo.add(idx);
            mudou = true;
          }
        });
        return mudou ? proximo : prev;
      });
    }
  }, [pilhaSoDistratores, ordemPilha]);

  // --- Pointer Drag Handlers ---

  function aoIniciarPointerDown(e: React.PointerEvent<HTMLElement>, type: 'from_pile' | 'from_slot', index: number) {
    if (respondida) return;
    
    // Se for um slot e estiver marcado como correto (travado), não faz nada
    if (type === 'from_slot' && statusSlots[index] === 'correto') {
      return;
    }

    // Se for da pilha e já estiver usado, não faz nada
    if (type === 'from_pile' && usadosNaPilha[index]) {
      return;
    }

    // Mede o ponto de agarre relativo à peça agora (o move/up ocorre no container).
    const rect = e.currentTarget.getBoundingClientRect();
    pointerDownInfoRef.current = {
      type,
      index,
      startX: e.clientX,
      startY: e.clientY,
      pointerId: e.pointerId,
      offsetX: e.clientX - rect.left,
      offsetY: e.clientY - rect.top,
      width: rect.width,
    };
  }

  function aoMoverDrag(e: React.PointerEvent<HTMLElement>) {
    const downInfo = pointerDownInfoRef.current;
    if (!downInfo) return;

    if (!isDragging) {
      // Calcula a distância do movimento
      const dx = e.clientX - downInfo.startX;
      const dy = e.clientY - downInfo.startY;
      const dist = Math.sqrt(dx * dx + dy * dy);

      // Limite de 8 pixels para considerar um arrasto real
      if (dist > 8) {
        // Cache inicial das posições dos slots (recalculado a cada frame durante o arrasto).
        recalcularBoundsSlots();

        let texto = '';
        let indicePilhaOriginal = -1;
        let startIdx = downInfo.index;

        if (downInfo.type === 'from_pile') {
          const peca = pecasAtivas[downInfo.index];
          texto = peca.texto;
          indicePilhaOriginal = peca.indicePilha;

          // Marca como usado e a retira da ordem da pilha imediatamente: ela passa a ser a
          // peça flutuante e as demais colapsam para preencher o espaço.
          const novosUsados = [...usadosNaPilha];
          novosUsados[downInfo.index] = true;
          setUsadosNaPilha(novosUsados);
          setOrdemPilha(prev => prev.filter(x => x !== downInfo.index));
        } else {
          const s = slots[downInfo.index];
          if (!s) {
            pointerDownInfoRef.current = null;
            return;
          }
          
          // Encontra o índice de início real da peça
          while (startIdx > 0 && slots[startIdx - 1] && slots[startIdx - 1]!.indicePilha === s.indicePilha) {
            startIdx--;
          }
          
          texto = s.texto;
          indicePilhaOriginal = s.indicePilha;

          // Limpa a peça do grid de slots imediatamente
          const novosSlots = [...slots];
          let length = 0;
          while (startIdx + length < slots.length && slots[startIdx + length] && slots[startIdx + length]!.indicePilha === s.indicePilha) {
            novosSlots[startIdx + length] = null;
            length++;
          }
          setSlots(novosSlots);
        }

        // Offset/largura vêm do pointerdown (medidos na peça de origem).
        const { offsetX, offsetY, width } = downInfo;

        dragInfoRef.current = {
          type: downInfo.type,
          index: startIdx,
          texto,
          indicePilhaOriginal,
          offsetX,
          offsetY,
          width
        };

        // Atualiza os estados que ativam placeholders e lógica de slots
        setDraggedPiece({
          type: downInfo.type,
          index: startIdx,
          texto,
          indicePilhaOriginal,
          width
        });
        setIsDragging(true);

        // Configura e exibe o elemento flutuante. A posição é controlada só por transform
        // (translate3d), que roda no compositor e evita reflow a cada quadro.
        ultimoPointerRef.current = { x: e.clientX, y: e.clientY };
        hoverIdxRef.current = null;
        hoverPilhaIdxRef.current = null;

        const card = floatingCardRef.current;
        if (card) {
          card.innerText = texto;
          card.style.width = width + 'px';
          card.style.fontSize = ehOrdenacaoTraducao ? '20px' : '28px';
          card.style.fontFamily = ehOrdenacaoTraducao ? 'inherit' : 'var(--fonte-hanzi)';
          card.style.transform = `translate3d(${e.clientX - offsetX}px, ${e.clientY - offsetY}px, 0) scale(1.04) rotate(1.5deg)`;
          card.style.display = 'flex';
        }

        // Captura o ponteiro no CONTAINER (estável), não na peça de origem: peças da pilha
        // desmontam ao virar placeholder e perderiam a captura, travando o arrasto.
        containerRef.current?.setPointerCapture(downInfo.pointerId);
      }
    } else {
      // Apenas registra a última posição e agenda o frame; todo o trabalho visual
      // (mover a peça, recalcular bounds, detectar hover) acontece uma vez por quadro.
      ultimoPointerRef.current = { x: e.clientX, y: e.clientY };
      agendarFrameDrag();
    }
  }

  function aoFinalizarPointerUp(e: React.PointerEvent<HTMLElement>) {
    const downInfo = pointerDownInfoRef.current;
    pointerDownInfoRef.current = null;

    if (!downInfo) return;

    if (isDragging) {
      // Libera o pointer capture (feito no container, alvo estável da captura)
      if (containerRef.current?.hasPointerCapture(downInfo.pointerId)) {
        containerRef.current.releasePointerCapture(downInfo.pointerId);
      }

      // Cancela qualquer frame de arrasto ainda pendente
      if (rafPendenteRef.current != null) {
        cancelAnimationFrame(rafPendenteRef.current);
        rafPendenteRef.current = null;
      }

      // Oculta o elemento flutuante diretamente no DOM
      if (floatingCardRef.current) {
        floatingCardRef.current.style.display = 'none';
      }

      // Usa o valor mais recente do ref (o estado pode ainda não ter sido "flushado")
      const hoverIdx = hoverIdxRef.current;
      const hoverPilha = hoverPilhaIdxRef.current;
      const info = dragInfoRef.current;

      if (info) {
        const indexPilha = info.indicePilhaOriginal;
        const texto = info.texto;
        const tamanhoL = (ehOrdenacaoTraducao || ehContexto) ? 1 : [...texto].length;

        // Devolve a peça à pilha na posição indicada (ou no fim, quando posicao === null).
        const devolverParaPilha = (posicao: number | null) => {
          setUsadosNaPilha(prev => {
            const n = [...prev];
            n[indexPilha] = false;
            return n;
          });
          setOrdemPilha(prev => {
            const sem = prev.filter(x => x !== indexPilha);
            const p = posicao === null ? sem.length : Math.min(Math.max(posicao, 0), sem.length);
            return [...sem.slice(0, p), indexPilha, ...sem.slice(p)];
          });
        };

        if (hoverPilha !== null) {
          // Soltou sobre a pilha: encaixa entre as peças, na posição do vão pré-visualizado.
          devolverParaPilha(hoverPilha);
        } else if (hoverIdx !== null) {
          const maxTargetIdx = Math.max(0, slots.length - tamanhoL);
          const slotDestino = Math.min(hoverIdx, maxTargetIdx);
          const novosSlots = tentarAcomodarPeca(slots, slotDestino, tamanhoL, { texto, indicePilha: indexPilha }, indicesTravados);
          if (novosSlots) {
            pecaEmTransicaoRef.current = `card-${indexPilha}`;
            setSlots(novosSlots);
            setStatusSlots(recomputarStatusSlots(novosSlots));
            if (!ehOrdenacaoTraducao) {
              aoTocarAudio(texto);
            }
          } else {
            // Não coube (ex.: empurraria uma peça travada): devolve ao fim da pilha.
            devolverParaPilha(null);
          }
        } else {
          // Soltou fora dos slots e da pilha: devolve ao fim da pilha.
          devolverParaPilha(null);
        }
      }

      setDraggedPiece(null);
      setIsDragging(false);
      setCurrentHoverSlotIdx(null);
      setCurrentHoverPilhaIdx(null);
      dragInfoRef.current = null;
      hoverIdxRef.current = null;
      hoverPilhaIdxRef.current = null;
      ultimoPointerRef.current = null;
    } else {
      // Trata como um clique normal
      if (downInfo.type === 'from_pile') {
        clicarPeca(downInfo.index);
      } else {
        removerSlot(downInfo.index);
      }
    }
  }

  // Se o navegador cancelar o ponteiro no meio do arrasto (gesto do SO, etc.), devolve a
  // peça para a pilha e limpa o estado, evitando que o drag fique "preso".
  function aoCancelarPointer() {
    const emArrasto = isDragging;
    pointerDownInfoRef.current = null;

    if (rafPendenteRef.current != null) {
      cancelAnimationFrame(rafPendenteRef.current);
      rafPendenteRef.current = null;
    }

    if (!emArrasto) return;

    if (floatingCardRef.current) {
      floatingCardRef.current.style.display = 'none';
    }

    const info = dragInfoRef.current;
    if (info) {
      const idx = info.indicePilhaOriginal;
      const novosUsados = [...usadosNaPilha];
      novosUsados[idx] = false;
      setUsadosNaPilha(novosUsados);
      setOrdemPilha(prev => [...prev.filter(x => x !== idx), idx]);
    }

    setDraggedPiece(null);
    setIsDragging(false);
    setCurrentHoverSlotIdx(null);
    setCurrentHoverPilhaIdx(null);
    dragInfoRef.current = null;
    hoverIdxRef.current = null;
    hoverPilhaIdxRef.current = null;
    ultimoPointerRef.current = null;
  }

  // Visualização provisória dos slots (com peça temporária afastando as demais via lerp)
  const slotsVisual = useMemo(() => {
    if (!isDragging || currentHoverSlotIdx === null || !draggedPiece) {
      return slots;
    }

    const { texto, indicePilhaOriginal } = draggedPiece;
    const tamanhoL = (ehOrdenacaoTraducao || ehContexto) ? 1 : [...texto].length;
    const maxTargetIdx = Math.max(0, slots.length - tamanhoL);
    const slotDestino = Math.min(currentHoverSlotIdx, maxTargetIdx);

    // Tenta simular a acomodação no slots atual (respeitando as peças travadas).
    const novosSlots = tentarAcomodarPeca(slots, slotDestino, tamanhoL, { texto, indicePilha: indicePilhaOriginal }, indicesTravados);
    return novosSlots || slots;
  }, [slots, isDragging, currentHoverSlotIdx, draggedPiece, indicesTravados, ehOrdenacaoTraducao, ehContexto]);

  // Ordem visual da pilha durante o arrasto: insere um "vão" (gap) na posição de encaixe,
  // afastando as demais peças — reaproveitando a mesma ideia de pré-visualização dos slots.
  const pilhaVisual = useMemo(() => {
    const entradas: ({ tipo: 'peca'; idxPilha: number; oi: number } | { tipo: 'gap' })[] =
      ordemPilha.map((idx, oi) => ({ tipo: 'peca' as const, idxPilha: idx, oi }));

    if (isDragging && currentHoverPilhaIdx !== null && draggedPiece) {
      const p = Math.min(Math.max(currentHoverPilhaIdx, 0), ordemPilha.length);
      entradas.splice(p, 0, { tipo: 'gap' as const });
    }
    return entradas;
  }, [ordemPilha, isDragging, currentHoverPilhaIdx, draggedPiece]);

  // --- Helpers de Renderização ---

  const getSlotStyle = (i: number, slotsArr: (SlotState | null)[]) => {
    const isChild = eSpanChild(slotsArr, i);
    const slotObj = slotsArr[i];
    const status = statusSlots[i];

    // A peça em arrasto aparece na grade apenas como PRÉ-VISUALIZAÇÃO do encaixe. Renderizamos
    // esse alvo como um "fantasma" tracejado (sem o texto sólido), para não duplicar/colidir
    // com a peça flutuante que segue o cursor.
    const isGhost = !!(isDragging && draggedPiece && slotObj && !isChild &&
      slotObj.indicePilha === draggedPiece.indicePilhaOriginal);

    let borderColor = '#334155'; // Slate-700
    let bgColor = 'rgba(255, 255, 255, 0.01)';
    let textColor = 'var(--cor-destaque)';
    let borderStyle = 'solid';
    let opacity = 1;
    let minWidth = slotMinWidth(i, slotsArr);

    if (isChild) {
      minWidth = 0;
      opacity = 0;
      bgColor = 'transparent';
      borderColor = 'transparent';
    } else if (isGhost) {
      borderColor = 'var(--cor-destaque)';
      bgColor = 'rgba(99, 102, 241, 0.15)';
      textColor = 'var(--cor-destaque)';
      borderStyle = 'dashed';
    } else if (slotObj) {
      if (status === 'correto') {
        borderColor = 'var(--cor-sucesso)';
        bgColor = 'rgba(16, 185, 129, 0.08)';
        textColor = 'var(--cor-sucesso)';
      } else if (status === 'posicao_errada') {
        borderColor = 'var(--cor-alerta)';
        bgColor = 'rgba(245, 158, 11, 0.08)';
        textColor = 'var(--cor-alerta)';
      } else if (status === 'errado') {
        borderColor = 'var(--cor-perigo)';
        bgColor = 'rgba(239, 68, 68, 0.08)';
        textColor = 'var(--cor-perigo)';
      } else {
        borderColor = '#475569'; // Slate-600
        bgColor = '#1e293b'; // Slate-800
        textColor = '#ffffff';
      }
    }

    return { borderColor, bgColor, textColor, borderStyle, opacity, minWidth, isGhost };
  };

  // Divide o texto para exibição (apenas no modo ordenação clássico)
  const partesExibicao = useMemo(() => {
    if (ehFoneticaFrase) {
      return null;
    }
    return questao.fraseOculta.split('＿');
  }, [questao, ehFoneticaFrase]);

  // Determina o tamanho do slot: vazio tem exatamente o tamanho de 1 caractere (70px para Hanzi, 80px para tradução),
  // e peças de múltiplos caracteres expandem proporcionalmente.
  const slotMinWidth = (indiceSlot: number, slotsArr: (SlotState | null)[]) => {
    const slotObj = slotsArr[indiceSlot];
    if (ehOrdenacaoTraducao) {
      if (!slotObj) return 80;
      const numChars = [...slotObj.texto].length;
      return numChars * 14 + 28;
    }
    // Para ordenação clássica, fonética e contexto com lacunas (atividades com Hanzi):
    // Cada slot unitário (vazio ou com 1 caractere) tem exatamente 70px.
    if (!slotObj) {
      return 70;
    }
    const numChars = [...slotObj.texto].length;
    const gap = ehFoneticaFrase ? 8 : 4;
    return numChars * 70 + (numChars - 1) * gap;
  };

  return (
    <div
      ref={containerRef}
      className="revisao-ordenacao-container"
      onPointerMove={aoMoverDrag}
      onPointerUp={aoFinalizarPointerUp}
      onPointerCancel={aoCancelarPointer}
      style={{
        display: 'flex',
        flexDirection: 'column',
        alignItems: 'center',
        gap: '20px',
        width: '100%',
        // UI de arrastar/montar: nada aqui deve virar seleção de texto ao passar o mouse.
        userSelect: 'none',
        WebkitUserSelect: 'none',
      }}
    >

      {/* Área da Frase Traduzida / Gabarito Centralizada */}
      <div style={{ 
        display: 'flex', 
        flexDirection: 'column',
        alignItems: 'center', 
        justifyContent: 'center',
        gap: '8px', 
        width: '100%', 
        margin: '10px 0 20px 0' 
      }}>
        <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'center', gap: '12px', flexWrap: 'wrap' }}>
          {(ehFoneticaFrase || ehOrdenacaoTraducao) && !respondida && (
            <BotaoAudio
              rotulo=""
              grande={ehFoneticaFrase}
              tocando={hanziTocando === questao.fraseOriginal}
              carregando={hanziSintetizando === questao.fraseOriginal}
              aoClicar={() => aoTocarAudio(questao.fraseOriginal)}
            />
          )}
          {ehFoneticaFrase && !respondida && aoTocarAudioLento && (
            <BotaoAudio
              rotulo=""
              lento
              grande
              titulo={t("Ouvir devagar, palavra por palavra")}
              tocando={!!audioLentoTocando}
              carregando={!!audioLentoSintetizando}
              aoClicar={aoTocarAudioLento}
            />
          )}
          {ehFoneticaFrase && !respondida ? null : ehOrdenacaoTraducao ? (
            <div style={{ 
              display: 'flex', 
              gap: '6px 12px', 
              alignItems: 'flex-end', 
              flexWrap: 'wrap', 
              justifyContent: 'center',
              fontFamily: 'var(--fonte-hanzi)'
            }}>
              {questao.fraseOriginalSegmentada?.map((t: any, idx: number) => {
                if (t.ehChines && t.pinyin) {
                  return (
                    <div
                      key={idx}
                      style={{
                        display: 'inline-flex',
                        flexDirection: 'column',
                        alignItems: 'center',
                        margin: '0 2px',
                        verticalAlign: 'bottom'
                      }}
                    >
                      <span
                        className={t.ehNaoVista ? 'revisao-pinyin-nao-visto' : ''}
                        style={{
                          fontSize: '14px',
                          color: t.ehNaoVista ? undefined : 'var(--cor-pinyin, #a0aec0)',
                          fontWeight: 'normal',
                          marginBottom: '2px',
                          userSelect: 'none',
                          lineHeight: 1.2
                        }}
                      >
                        {t.pinyin}
                      </span>
                      <span
                        className={t.ehNaoVista ? 'revisao-palavra-nao-vista' : ''}
                        style={{
                          cursor: 'pointer',
                          transition: 'color 0.2s',
                          color: t.ehNaoVista ? undefined : 'inherit',
                          fontSize: '28px',
                          lineHeight: 1.2,
                          borderBottom: '1px dotted #475569',
                        }}
                        onMouseEnter={(e) => {
                          e.currentTarget.style.color = 'var(--cor-destaque)';
                          const rect = e.currentTarget.getBoundingClientRect();
                          setPopupInfo({
                            pinyin: t.pinyin,
                            hanzi: t.texto,
                            significados: t.significados ? t.significados.join(', ') : '',
                            x: rect.left + rect.width / 2,
                            y: rect.top
                          });
                        }}
                        onMouseLeave={(e) => {
                          e.currentTarget.style.color = 'inherit';
                          setPopupInfo(null);
                        }}
                        onClick={() => {
                          if (AoClicarNoCartao) {
                            AoClicarNoCartao({ Hanzi: t.texto, Pinyin: t.pinyin, significados: t.significados });
                          }
                        }}
                      >
                        {t.texto}
                      </span>
                    </div>
                  );
                }
                return (
                  <span
                    key={idx}
                    style={{
                      fontSize: '28px',
                      margin: '0 2px',
                      verticalAlign: 'bottom',
                      alignSelf: 'flex-end',
                      color: t.ehChines ? 'inherit' : 'var(--cor-texto-suave)',
                      lineHeight: 1.2
                    }}
                  >
                    {t.texto}
                  </span>
                );
              })}
            </div>
          ) : (
            <span
              style={{
                fontSize: '24px',
                color: '#ffffff',
                textAlign: 'center',
                borderBottom: '2px dotted #475569',
                paddingBottom: '2px',
              }}
            >
              {questao.fraseTraducao}
            </span>
          )}
        </div>
        {/* Créditos / Atribuição da Frase */}
        {questao.fraseAtribuicao && (
          <div style={{ color: 'var(--cor-texto-suave)', fontSize: '10px', opacity: 0.6, marginTop: '4px', textAlign: 'center' }}>
            {questao.fraseAtribuicao}
          </div>
        )}
      </div>

      {/* Área da Frase delimitada por duas linhas horizontais */}
      <div className={`revisao-ordenacao-frase ${isDragging ? 'arrastando' : ''}`} style={{
        display: 'flex',
        flexWrap: 'wrap',
        alignItems: 'center',
        justifyContent: 'flex-start',
        fontSize: '32px',
        lineHeight: 1.6,
        fontFamily: 'var(--fonte-hanzi)',
        minHeight: '88px',
        padding: '20px 0',
        // Durante o arrasto as divisórias recuam e a área ganha um leve realce, para
        // a peça flutuante não "colidir" visualmente com as linhas.
        borderTop: `2px solid ${isDragging ? '#273449' : '#334155'}`,
        borderBottom: `2px solid ${isDragging ? '#273449' : '#334155'}`,
        background: isDragging ? 'rgba(99, 102, 241, 0.04)' : 'transparent',
        width: '100%',
        gap: '4px',
        position: 'relative',
        transition: 'background 0.2s ease, border-color 0.2s ease',
      }}>
        {respondida && questao.fraseOriginalSegmentada ? (
          ehOrdenacaoTraducao ? (
            <div style={{ 
              display: 'flex', 
              justifyContent: 'center', 
              alignItems: 'center', 
              width: '100%', 
              fontSize: '24px', 
              color: 'var(--cor-sucesso)', 
              padding: '10px 0',
              fontWeight: '500',
              fontFamily: 'inherit'
            }}>
              {questao.fraseTraducao}
            </div>
          ) : (
            <div style={{ 
              display: 'flex', 
              gap: '8px 16px', 
              alignItems: 'flex-end', 
              flexWrap: 'wrap', 
              justifyContent: 'center',
              width: '100%'
            }}>
              <div style={{ display: 'inline-flex', alignItems: 'center', gap: '8px', verticalAlign: 'bottom', marginRight: '8px', alignSelf: 'flex-end' }}>
                <BotaoAudio
                  rotulo=""
                  tocando={hanziTocando === questao.fraseOriginal}
                  carregando={hanziSintetizando === questao.fraseOriginal}
                  aoClicar={() => aoTocarAudio(questao.fraseOriginal)}
                />
                {ehFoneticaFrase && aoTocarAudioLento && (
                  <BotaoAudio
                    rotulo=""
                    lento
                    titulo={t("Ouvir devagar, palavra por palavra")}
                    tocando={!!audioLentoTocando}
                    carregando={!!audioLentoSintetizando}
                    aoClicar={aoTocarAudioLento}
                  />
                )}
              </div>
              {questao.fraseOriginalSegmentada.map((t: any, idx: number) => {
                if (t.ehChines && t.pinyin) {
                  return (
                    <div
                      key={idx}
                      style={{
                        display: 'inline-flex',
                        flexDirection: 'column',
                        alignItems: 'center',
                        margin: '0 2px',
                        verticalAlign: 'bottom'
                      }}
                    >
                      <span 
                        className={t.ehNaoVista ? 'revisao-pinyin-nao-visto' : ''}
                        style={{ 
                          fontSize: '14px', 
                          color: t.ehNaoVista ? undefined : 'var(--cor-pinyin, #a0aec0)', 
                          fontWeight: 'normal',
                          marginBottom: '2px',
                          userSelect: 'none',
                          lineHeight: 1.2
                        }}
                      >
                        {t.pinyin}
                      </span>
                      <span
                        className={t.ehNaoVista ? 'revisao-palavra-nao-vista' : ''}
                        style={{
                          cursor: 'pointer',
                          transition: 'color 0.2s',
                          color: t.ehNaoVista ? undefined : 'inherit',
                          fontSize: '28px',
                          lineHeight: 1.2
                        }}
                        onMouseEnter={(e) => {
                          e.currentTarget.style.color = 'var(--cor-destaque)';
                          const rect = e.currentTarget.getBoundingClientRect();
                          setPopupInfo({
                            pinyin: t.pinyin,
                            hanzi: t.texto,
                            significados: t.significados ? t.significados.join(', ') : '',
                            x: rect.left + rect.width / 2,
                            y: rect.top
                          });
                        }}
                        onMouseLeave={(e) => {
                          e.currentTarget.style.color = 'inherit';
                          setPopupInfo(null);
                        }}
                        onClick={() => {
                          if (AoClicarNoCartao) {
                            AoClicarNoCartao({ Hanzi: t.texto, Pinyin: t.pinyin, significados: t.significados });
                          }
                        }}
                      >
                        {t.texto}
                      </span>
                    </div>
                  );
                }
                return (
                  <span 
                    key={idx} 
                    style={{ 
                      fontSize: '28px', 
                      margin: '0 2px', 
                      verticalAlign: 'bottom',
                      alignSelf: 'flex-end',
                      color: t.ehChines ? 'inherit' : 'var(--cor-texto-suave)',
                      lineHeight: 1.2
                    }}
                  >
                    {t.texto}
                  </span>
                );
              })}
            </div>
          )
        ) : ehContexto ? (
          <div style={{ display: 'flex', flexWrap: 'wrap', gap: '8px 12px', alignItems: 'flex-end', justifyContent: 'center', width: '100%', fontFamily: 'var(--fonte-hanzi)' }}>
            <div style={{ display: 'inline-flex', alignItems: 'center', gap: '8px', verticalAlign: 'bottom', marginRight: '4px', alignSelf: 'flex-end' }}>
              <BotaoAudio
                rotulo=""
                tocando={hanziTocando === questao.fraseOriginal}
                carregando={hanziSintetizando === questao.fraseOriginal}
                aoClicar={() => aoTocarAudio(questao.fraseOriginal)}
              />
            </div>
            {(() => {
              let lacunaSlotCounter = 0;
              const segmentadaLacuna = (questao as any).fraseOriginalLacunaSegmentada || questao.fraseOriginalSegmentada || [];
              return segmentadaLacuna.map((t: any, idx: number) => {
                if (t.ehLacuna) {
                  const slotIdx = lacunaSlotCounter++;
                  const slotObj = slotsVisual[slotIdx];
                  const styleProps = getSlotStyle(slotIdx, slotsVisual);
                  return (
                    <div
                      key={`lacuna-${slotIdx}`}
                      data-slot-idx={slotIdx}
                      data-flip-id={slotObj ? `card-${slotObj.indicePilha}` : undefined}
                      className={`revisao-ordenacao-slot ${slotObj ? 'preenchido' : 'vazio'}`}
                      onPointerDown={(e) => aoIniciarPointerDown(e, 'from_slot', slotIdx)}
                      style={{
                        display: 'inline-flex',
                        flexDirection: 'column',
                        alignItems: 'center',
                        justifyContent: 'center',
                        minWidth: `${styleProps.minWidth}px`,
                        width: `${styleProps.minWidth}px`,
                        boxSizing: 'border-box',
                        flexShrink: 0,
                        height: '48px',
                        margin: '0 4px',
                        border: !styleProps.isGhost ? 'none' : `2px dashed ${styleProps.borderColor}`,
                        borderBottom: `2px ${styleProps.isGhost ? 'dashed' : styleProps.borderStyle} ${styleProps.borderColor}`,
                        background: styleProps.bgColor,
                        borderRadius: styleProps.isGhost ? '8px' : '8px 8px 0 0',
                        color: styleProps.textColor,
                        cursor: slotObj && !respondida && statusSlots[slotIdx] !== 'correto' ? 'grab' : (respondida ? 'default' : 'pointer'),
                        transition: 'min-width 0.18s cubic-bezier(0.2, 0, 0, 1), width 0.18s cubic-bezier(0.2, 0, 0, 1), background 0.15s ease, border-color 0.15s ease, color 0.12s ease',
                        verticalAlign: 'bottom',
                        fontSize: '28px',
                        fontFamily: 'var(--fonte-hanzi)',
                        touchAction: 'none',
                      }}
                    >
                      {slotObj ? (() => {
                        const pecaPinyin = pecasAtivas[slotObj.indicePilha]?.pinyin;
                        return (
                          <div style={{ display: 'inline-flex', flexDirection: 'column', alignItems: 'center', lineHeight: 1.1 }}>
                            {pecaPinyin && (
                              <span style={{ fontSize: '13px', color: 'var(--cor-pinyin, #a0aec0)', fontWeight: 'normal', userSelect: 'none' }}>
                                {pecaPinyin}
                              </span>
                            )}
                            <span>{slotObj.texto}</span>
                          </div>
                        );
                      })() : (
                        ' '
                      )}
                    </div>
                  );
                }

                if (t.ehChines && t.pinyin) {
                  return (
                    <div
                      key={idx}
                      style={{
                        display: 'inline-flex',
                        flexDirection: 'column',
                        alignItems: 'center',
                        margin: '0 2px',
                        verticalAlign: 'bottom',
                      }}
                    >
                      <span
                        className={t.ehNaoVista ? 'revisao-pinyin-nao-visto' : ''}
                        style={{
                          fontSize: '14px',
                          color: t.ehNaoVista ? undefined : 'var(--cor-pinyin, #a0aec0)',
                          fontWeight: 'normal',
                          marginBottom: '2px',
                          userSelect: 'none',
                          lineHeight: 1.2,
                        }}
                      >
                        {t.pinyin}
                      </span>
                      <span
                        className={t.ehNaoVista ? 'revisao-palavra-nao-vista' : ''}
                        style={{
                          cursor: 'pointer',
                          transition: 'color 0.2s',
                          color: t.ehNaoVista ? undefined : 'inherit',
                          fontSize: '28px',
                          lineHeight: 1.2,
                          borderBottom: '1px dotted #475569',
                        }}
                        onMouseEnter={(e) => {
                          e.currentTarget.style.color = 'var(--cor-destaque)';
                          const rect = e.currentTarget.getBoundingClientRect();
                          setPopupInfo({
                            pinyin: t.pinyin,
                            hanzi: t.texto,
                            significados: t.significados ? t.significados.join(', ') : '',
                            x: rect.left + rect.width / 2,
                            y: rect.top,
                          });
                        }}
                        onMouseLeave={(e) => {
                          e.currentTarget.style.color = 'inherit';
                          setPopupInfo(null);
                        }}
                        onClick={() => {
                          if (AoClicarNoCartao) {
                            AoClicarNoCartao({ Hanzi: t.texto, Pinyin: t.pinyin, significados: t.significados });
                          }
                        }}
                      >
                        {t.texto}
                      </span>
                    </div>
                  );
                }

                return (
                  <span
                    key={idx}
                    style={{
                      fontSize: '28px',
                      margin: '0 2px',
                      verticalAlign: 'bottom',
                      alignSelf: 'flex-end',
                      color: t.ehChines ? 'inherit' : 'var(--cor-texto-suave)',
                      lineHeight: 1.2,
                    }}
                  >
                    {t.texto}
                  </span>
                );
              });
            })()}
          </div>
        ) : (ehFoneticaFrase || ehOrdenacaoTraducao) ? (
          <div style={{ display: 'flex', flexWrap: 'wrap', gap: '8px', alignItems: 'center', justifyContent: 'flex-start', width: '100%' }}>
            {pecasEsperadasAtivas.map((_, i) => {
              const slotObj = slotsVisual[i];
              const styleProps = getSlotStyle(i, slotsVisual);
              const isChild = eSpanChild(slotsVisual, i);
              return (
                <div
                  key={i}
                  data-slot-idx={i}
                  data-flip-id={slotObj && !isChild ? `card-${slotObj.indicePilha}` : undefined}
                  className={`revisao-ordenacao-slot ${slotObj ? 'preenchido' : 'vazio'} ${isChild ? 'slot-child' : ''}`}
                  onPointerDown={(e) => aoIniciarPointerDown(e, 'from_slot', i)}
                  style={{
                    display: 'inline-flex',
                    alignItems: 'center',
                    justifyContent: 'center',
                    minWidth: `${styleProps.minWidth}px`,
                    width: `${styleProps.minWidth}px`,
                    boxSizing: 'border-box',
                    flexShrink: 0,
                    opacity: styleProps.opacity,
                    height: isChild ? '0px' : '48px',
                    // Filho colapsa a 0; a margem negativa anula o gap do flex (8px) que
                    // sobraria antes dele, para a peça multi-hanzi ocupar o footprint certo.
                    margin: isChild ? '0 0 0 -8px' : '0 2px',
                    border: isChild || !styleProps.isGhost ? 'none' : `2px dashed ${styleProps.borderColor}`,
                    borderBottom: isChild ? 'none' : `2px ${styleProps.isGhost ? 'dashed' : styleProps.borderStyle} ${styleProps.borderColor}`,
                    background: styleProps.bgColor,
                    borderRadius: styleProps.isGhost ? '8px' : '8px 8px 0 0',
                    color: styleProps.textColor,
                    cursor: slotObj && !respondida && statusSlots[i] !== 'correto' ? 'grab' : (respondida ? 'default' : 'pointer'),
                    transition: 'min-width 0.18s cubic-bezier(0.2, 0, 0, 1), width 0.18s cubic-bezier(0.2, 0, 0, 1), background 0.15s ease, border-color 0.15s ease, color 0.12s ease',
                    verticalAlign: 'bottom',
                    fontSize: ehOrdenacaoTraducao ? '20px' : '28px',
                    fontFamily: ehOrdenacaoTraducao ? 'inherit' : 'var(--fonte-hanzi)',
                    overflow: 'hidden',
                    pointerEvents: isChild ? 'none' : 'auto',
                    touchAction: 'none',
                  }}
                >
                  {slotObj && !isChild ? slotObj.texto : ' '}
                </div>
              );
            })}
          </div>
        ) : (
          <div style={{ display: 'flex', flexWrap: 'wrap', gap: '8px 12px', alignItems: 'flex-end', justifyContent: 'center', width: '100%', fontFamily: 'var(--fonte-hanzi)' }}>
            <div style={{ display: 'inline-flex', alignItems: 'center', gap: '8px', verticalAlign: 'bottom', marginRight: '4px', alignSelf: 'flex-end' }}>
              <BotaoAudio
                rotulo=""
                tocando={hanziTocando === questao.fraseOriginal}
                carregando={hanziSintetizando === questao.fraseOriginal}
                aoClicar={() => aoTocarAudio(questao.fraseOriginal)}
              />
            </div>
            {partesExibicao!.map((parte: string, i: number) => {
              const slotObj = i < slotsVisual.length ? slotsVisual[i] : null;
              const styleProps = i < slotsVisual.length ? getSlotStyle(i, slotsVisual) : null;
              const isChild = i < slotsVisual.length ? eSpanChild(slotsVisual, i) : false;
              return (
                <React.Fragment key={i}>
                  <span style={{ whiteSpace: 'pre-wrap', color: 'var(--cor-texto-suave)', fontSize: '28px' }}>{parte}</span>
                  {i < slotsVisual.length && styleProps && (
                    <div
                      data-slot-idx={i}
                      data-flip-id={slotObj && !isChild ? `card-${slotObj.indicePilha}` : undefined}
                      className={`revisao-ordenacao-slot ${slotObj ? 'preenchido' : 'vazio'} ${isChild ? 'slot-child' : ''}`}
                      onPointerDown={(e) => aoIniciarPointerDown(e, 'from_slot', i)}
                      style={{
                        display: 'inline-flex',
                        alignItems: 'center',
                        justifyContent: 'center',
                        minWidth: `${styleProps.minWidth}px`,
                        width: `${styleProps.minWidth}px`,
                        boxSizing: 'border-box',
                        flexShrink: 0,
                        opacity: styleProps.opacity,
                        height: isChild ? '0px' : '48px',
                        // Filho colapsa a 0; a margem negativa anula o gap do flex antes dele.
                        margin: isChild ? '0 0 0 -12px' : '0 4px',
                        border: isChild || !styleProps.isGhost ? 'none' : `2px dashed ${styleProps.borderColor}`,
                        borderBottom: isChild ? 'none' : `2px ${styleProps.isGhost ? 'dashed' : styleProps.borderStyle} ${styleProps.borderColor}`,
                        background: styleProps.bgColor,
                        borderRadius: styleProps.isGhost ? '8px' : '8px 8px 0 0',
                        color: styleProps.textColor,
                        cursor: slotObj && !respondida && statusSlots[i] !== 'correto' ? 'grab' : (respondida ? 'default' : 'pointer'),
                        transition: 'min-width 0.18s cubic-bezier(0.2, 0, 0, 1), width 0.18s cubic-bezier(0.2, 0, 0, 1), background 0.15s ease, border-color 0.15s ease, color 0.12s ease',
                        verticalAlign: 'bottom',
                        fontSize: '28px',
                        overflow: 'hidden',
                        pointerEvents: isChild ? 'none' : 'auto',
                        touchAction: 'none',
                      }}
                    >
                      {slotObj && !isChild ? slotObj.texto : ' '}
                    </div>
                  )}
                </React.Fragment>
              );
            })}
          </div>
        )}
      </div>

      {/* Tentativa do usuário (apenas se errou) */}
      {respondida && acertou === false && (
        <div style={{ display: 'flex', flexDirection: 'column', alignItems: 'center', gap: '8px', width: '100%' }}>
          <div style={{ fontSize: '16px', fontWeight: '500', color: 'var(--cor-texto-suave)' }}>{t('Sua tentativa:')}</div>
          <div className="revisao-ordenacao-frase-tentativa" style={{
            display: 'flex',
            flexWrap: 'wrap',
            alignItems: 'center',
            justifyContent: 'center',
            fontSize: ehOrdenacaoTraducao ? '20px' : '28px',
            lineHeight: 1.6,
            fontFamily: ehOrdenacaoTraducao ? 'inherit' : 'var(--fonte-hanzi)',
            minHeight: '60px',
            padding: '15px 20px',
            background: 'rgba(255, 255, 255, 0.02)',
            border: '1px dashed var(--cor-borda)',
            borderRadius: '12px',
            width: '100%',
            opacity: 0.85,
            gap: '4px',
          }}>
            {slots.map((s, i) => {
              if (eSpanChild(slots, i)) return null;
              const styleProps = getSlotStyle(i, slots);
              return (
                <div
                  key={i}
                  className={`revisao-ordenacao-slot ${s ? 'preenchido' : 'vazio'}`}
                  style={{
                    display: 'inline-flex',
                    alignItems: 'center',
                    justifyContent: 'center',
                    minWidth: `${slotMinWidth(i, slots)}px`,
                    height: '42px',
                    margin: '0 2px',
                    borderBottom: `2px solid ${styleProps.borderColor}`,
                    background: styleProps.bgColor,
                    borderRadius: '4px 4px 0 0',
                    color: styleProps.textColor,
                    cursor: 'default',
                    transition: 'all 0.2s',
                    verticalAlign: 'bottom',
                    fontSize: ehOrdenacaoTraducao ? '20px' : '28px',
                    fontFamily: ehOrdenacaoTraducao ? 'inherit' : 'var(--fonte-hanzi)',
                  }}
                >
                  {s ? s.texto : ' '}
                </div>
              );
            })}
          </div>
        </div>
      )}

      {/* Pilha de Peças — dinâmica: colapsa o vão ao retirar uma peça e afasta as demais para
          abrir o encaixe durante o arrasto. alignItems:center + altura fixa evitam que as
          células se estiquem verticalmente ("respirando") quando a composição muda. */}
      {!respondida && (
        <div className="revisao-ordenacao-pilha" style={{
          display: 'flex',
          flexWrap: 'wrap',
          gap: '12px',
          justifyContent: 'center',
          alignItems: 'center',
          alignContent: 'center',
          marginTop: '24px',
          width: '100%',
          minHeight: '56px',
        }}>
          {pilhaVisual.map((ent) => {
            if (ent.tipo === 'gap') {
              const larguraVao = draggedPiece?.width ?? 60;
              return (
                <div
                  key="__gap__"
                  aria-hidden
                  style={{
                    flex: '0 0 auto',
                    width: `${larguraVao}px`,
                    height: '56px',
                    borderRadius: '12px',
                    border: '2px dashed var(--cor-destaque, #6366f1)',
                    background: 'rgba(99, 102, 241, 0.10)',
                    pointerEvents: 'none',
                  }}
                />
              );
            }

            const peca = pecasAtivas[ent.idxPilha];
            const vermelha = pecaPilhaVermelha(ent.idxPilha);
            return (
              <button
                key={ent.idxPilha}
                data-oi={ent.oi}
                data-flip-id={`card-${peca.indicePilha}`}
                data-flip-scope="pilha"
                className="revisao-opcao-btn revisao-pilha-item"
                onPointerDown={(e) => aoIniciarPointerDown(e, 'from_pile', ent.idxPilha)}
                style={{
                  boxSizing: 'border-box',
                  height: '56px',
                  padding: ehOrdenacaoTraducao ? '0 16px' : '0 20px',
                  fontSize: ehOrdenacaoTraducao ? '20px' : '28px',
                  fontFamily: ehOrdenacaoTraducao ? 'inherit' : 'var(--fonte-hanzi)',
                  background: vermelha ? 'rgba(239, 68, 68, 0.10)' : 'var(--cor-fundo-secundario)',
                  color: vermelha ? 'var(--cor-perigo)' : 'var(--cor-texto-primario)',
                  border: vermelha ? '2px solid var(--cor-perigo)' : '2px solid var(--cor-borda)',
                  borderBottom: vermelha ? '4px solid var(--cor-perigo)' : '4px solid #334155', // Borda 3D flat
                  borderRadius: '12px',
                  cursor: 'grab',
                  transition: 'filter 0.12s ease, transform 0.12s ease, box-shadow 0.12s ease',
                  minWidth: '60px',
                  display: 'inline-flex',
                  alignItems: 'center',
                  justifyContent: 'center',
                  boxShadow: '0 4px 6px rgba(0,0,0,0.1)',
                  touchAction: 'none',
                }}
                onMouseEnter={(e) => {
                  if (isDragging) return;
                  e.currentTarget.style.filter = 'brightness(1.12)';
                  e.currentTarget.style.transform = 'translateY(-2px)';
                  e.currentTarget.style.boxShadow = '0 7px 12px rgba(0,0,0,0.22)';
                }}
                onMouseLeave={(e) => {
                  if (isDragging) return;
                  e.currentTarget.style.filter = '';
                  e.currentTarget.style.transform = '';
                  e.currentTarget.style.boxShadow = '0 4px 6px rgba(0,0,0,0.1)';
                }}
              >
                {peca.texto}
              </button>
            );
          })}
        </div>
      )}

      {/* Peça arrastada flutuante (manipulada por ref direta para alta performance).
          A posição vem 100% de transform:translate3d — left/top ficam fixos em 0. */}
      <div
        ref={floatingCardRef}
        style={{
          position: 'fixed',
          left: 0,
          top: 0,
          display: 'none',
          alignItems: 'center',
          justifyContent: 'center',
          background: '#1e293b', // Slate-800
          border: '2px solid var(--cor-destaque, #6366f1)',
          borderBottom: '4px solid var(--cor-destaque-hover, #4f46e5)',
          borderRadius: '12px',
          color: '#ffffff',
          boxShadow: '0 12px 28px rgba(0, 0, 0, 0.45), 0 0 0 1px rgba(99, 102, 241, 0.25)',
          pointerEvents: 'none',
          zIndex: 99999,
          height: '56px',
          willChange: 'transform',
          transition: 'none',
        }}
      />

      <PopupRevisao info={popupInfo} />
    </div>
  );
}
