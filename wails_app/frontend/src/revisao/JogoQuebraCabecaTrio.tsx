// ----- Seção: Componente Modular de Quebra-Cabeça de 3 Colunas (Trio) -----
// O tabuleiro é uma MESA FÍSICA cercada por uma barreira: peças de QUALQUER coluna são arrastáveis
// (a da direita livre conecta ao meio), ficam onde são soltas e não podem sair da arena — ao
// esbarrar num limite a parede acende (roxo) e a peça é grampeada de volta. Cada CONJUNTO conectado
// é ancorado na peça do MEIO (o hub): esquerda/direita conectadas derivam a posição da base do meio,
// então o conjunto se move junto e é arrastável por qualquer membro — completo ou não. Conexões por
// clique movem AS DUAS peças (membro e hub), cada uma pela metade do caminho, até se encontrarem no
// meio. Sons reativos pontuam o gesto (tique ao pegar, clô ao encaixar, toc ao bater na parede).
import { useState, useEffect, useRef } from 'react';
import { t } from '../i18n/i18n';
import { FormaPeca, PROFUNDIDADE_PINO } from './FormaPecaQuebraCabeca';
import {
  calcularEmpurrao,
  distanciaConectores,
  grampearNaArena,
  animarProgresso,
  passoSeparacaoEstatica,
  repulsaoContinua,
  sobreposicaoAlvo,
  DURACAO_DESLIZE,
  DURACAO_SEPARACAO_ESTATICA,
  RAIO_CONEXAO,
  MapaVetores,
  ParedesTocadas,
  PecaSeparacao,
  SEM_PAREDES,
  algumaParede,
  VETOR_ZERO,
} from './fisicaQuebraCabeca';
import { IconeCoracao, IconeCoracaoPartido } from './IconesPlacar';
import { tocarSomClique, tocarSomEncaixe, tocarSomBarreira } from '../comum/sons';

export interface ItemQuebraCabeca {
  id: string;
  conteudo: React.ReactNode;
}

type Coluna = 'esquerda' | 'meio' | 'direita';

interface JogoQuebraCabecaTrioProps {
  itensEsquerda: ItemQuebraCabeca[];
  itensMeio: ItemQuebraCabeca[];
  itensDireita: ItemQuebraCabeca[];
  combinacaoCorreta: (idEsq: string, idAlvo: string) => boolean;
  vidasIniciais?: number;
  instrucao?: string;
  aoConcluir: (acertou: boolean, matchedIds: string[]) => void;
  aoAcertarItem?: (id: string, conexao: 'esq_mid' | 'mid_dir') => void;
  aoErrarItem?: (idArrastado: string, idAlvo: string) => void;
}

// PecaEmDestaque identifica uma peça por id E coluna: no trio o mesmo id existe nas três colunas,
// então estados como seleção/quebra precisam do par para não vazar para as colunas vizinhas.
interface PecaEmDestaque {
  id: string;
  coluna: Coluna;
}

// BasePeca é o retângulo do SLOT de layout de uma peça (o wrapper, que nunca é transformado):
// a referência estática de onde os deslocamentos persistentes partem.
interface BasePeca {
  left: number;
  top: number;
  width: number;
  height: number;
}

// DURACAO_QUEBRA cobre a animação dos fragmentos da peça quebrando (ver revisao.css) com uma folga
// pequena antes de restaurar o conjunto no seu lugar de descanso.
const DURACAO_QUEBRA = 650;
// ARENA_INSET é a margem (px) entre a borda da arena e o limite de grampeamento das peças.
const ARENA_INSET = 8;

function prefixoCard(coluna: Coluna): string {
  return coluna === 'esquerda' ? 'card-trio-esq-' : coluna === 'meio' ? 'card-trio-mid-' : 'card-trio-dir-';
}

// prefixoWrapper aponta para o SLOT de layout (o wrapper nunca é transformado) — a referência
// estável para medir repouso, ao contrário do card, que carrega transform e transição.
function prefixoWrapper(coluna: Coluna): string {
  return coluna === 'esquerda' ? 'wrapper-trio-esq-' : coluna === 'meio' ? 'wrapper-trio-mid-' : 'wrapper-trio-dir-';
}

export function JogoQuebraCabecaTrio({
  itensEsquerda,
  itensMeio,
  itensDireita,
  combinacaoCorreta,
  vidasIniciais = 2,
  instrucao,
  aoConcluir,
  aoAcertarItem,
  aoErrarItem,
}: JogoQuebraCabecaTrioProps) {
  const [vidas, setVidas] = useState(vidasIniciais);

  const [paresEsqMid, setParesEsqMid] = useState<string[]>([]);
  const [paresMidDir, setParesMidDir] = useState<string[]>([]);
  // Espelhos em ref: o laço da separação estática roda em rAF sobre a CLAUSURA do render que o
  // iniciou, e ele começa no mesmo tick da conexão. Lendo o estado dali, o vínculo recém-fechado
  // ainda apareceria como duas peças LIVRES sobrepostas pelo pino — e como elas são rígidas entre
  // si a sobreposição nunca se resolveria: o laço empurraria o conjunto até a parede.
  const paresEsqMidRef = useRef<string[]>([]);
  const paresMidDirRef = useRef<string[]>([]);

  const [idArrastando, setIdArrastando] = useState<string | null>(null);
  const [origemArrasto, setOrigemArrasto] = useState<Coluna | null>(null);
  // alvoArrasto carrega id E coluna: o mesmo id existe nas três colunas, então só o par identifica
  // o destino real (decidir a coluna depois, pelo id, registraria a conexão no par errado).
  const [alvoArrasto, setAlvoArrasto] = useState<PecaEmDestaque | null>(null);
  const [posicaoArrasto, setPosicaoArrasto] = useState({ x: 0, y: 0 });

  const [alvoErro, setAlvoErro] = useState<PecaEmDestaque | null>(null);
  // pecaQuebrando marca o conjunto em animação de quebra (tentativa de encaixe errada): o hub fica
  // invisível na posição do arrasto enquanto os fragmentos rachados se separam e caem.
  const [pecaQuebrando, setPecaQuebrando] = useState<PecaEmDestaque | null>(null);
  // deslizandoPar é o lerp de conexão por clique: o MEMBRO e o HUB se movem, cada um pela metade da
  // distância necessária, até se encontrarem no meio.
  const [deslizandoPar, setDeslizandoPar] = useState<{
    idMembro: string;
    colMembro: 'esquerda' | 'direita';
    idHub: string;
    offsetMembro: { x: number; y: number };
    offsetHub: { x: number; y: number };
  } | null>(null);

  // bases guarda o slot estático de cada peça (chave "coluna-id"); deslocamentos é o repouso
  // persistente e empurroes o deslocamento transitório do arrasto em curso.
  const [bases, setBases] = useState<Record<string, BasePeca>>({});
  const [deslocamentos, setDeslocamentos] = useState<MapaVetores>({});
  const deslocamentosRef = useRef<MapaVetores>({});
  const [empurroes, setEmpurroes] = useState<MapaVetores>({});
  const empurroesRef = useRef<MapaVetores>({});

  const [paredes, setParedes] = useState<ParedesTocadas>(SEM_PAREDES);

  // Seleção via clique (Clique em peça de uma coluna -> Clique em peça de outra coluna)
  const [pecaSelecionada, setPecaSelecionada] = useState<PecaEmDestaque | null>(null);

  const pointerStartRef = useRef({ x: 0, y: 0 });
  const slotArrastoRef = useRef<BasePeca | null>(null);
  const arenaRef = useRef<HTMLDivElement>(null);
  const arenaBoxRef = useRef<{ left: number; top: number; right: number; bottom: number } | null>(null);
  const cancelarDeslizeRef = useRef<(() => void) | null>(null);
  const cancelarSeparacaoRef = useRef<(() => void) | null>(null);
  const ultimoSomBarreiraRef = useRef(0);

  useEffect(() => {
    medirBases();
    window.addEventListener('resize', medirBases);
    return () => window.removeEventListener('resize', medirBases);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [itensEsquerda, itensMeio, itensDireita, paresEsqMid, paresMidDir]);

  useEffect(() => () => {
    cancelarDeslizeRef.current?.();
    cancelarSeparacaoRef.current?.();
  }, []);

  // medirBases captura os slots de layout pelos WRAPPERS (nunca transformados). As posições são de
  // viewport, mas só as DIFERENÇAS entre bases é que são usadas — rolagem não invalida a medição.
  function medirBases() {
    const novas: Record<string, BasePeca> = {};
    const medir = (chave: string, idElemento: string) => {
      const el = document.getElementById(idElemento);
      if (!el) return;
      const rect = el.getBoundingClientRect();
      novas[chave] = { left: rect.left, top: rect.top, width: rect.width, height: rect.height };
    };
    itensEsquerda.forEach((item) => medir(`esquerda-${item.id}`, `wrapper-trio-esq-${item.id}`));
    itensMeio.forEach((item) => medir(`meio-${item.id}`, `wrapper-trio-mid-${item.id}`));
    itensDireita.forEach((item) => medir(`direita-${item.id}`, `wrapper-trio-dir-${item.id}`));
    setBases(novas);
  }

  // ----- Deslocamentos persistentes (mesa física) -----

  function somarDeslocamento(chave: string, delta: { x: number; y: number }) {
    if (delta.x === 0 && delta.y === 0) return;
    const atual = deslocamentosRef.current[chave] || VETOR_ZERO;
    deslocamentosRef.current = { ...deslocamentosRef.current, [chave]: { x: atual.x + delta.x, y: atual.y + delta.y } };
    setDeslocamentos(deslocamentosRef.current);
  }

  function assentarEmpurroes() {
    const vigentes = empurroesRef.current;
    if (Object.keys(vigentes).length === 0) return;
    const novos = { ...deslocamentosRef.current };
    Object.entries(vigentes).forEach(([chave, empurrao]) => {
      const atual = novos[chave] || VETOR_ZERO;
      novos[chave] = { x: atual.x + empurrao.x, y: atual.y + empurrao.y };
    });
    deslocamentosRef.current = novos;
    empurroesRef.current = {};
    setDeslocamentos(novos);
    setEmpurroes({});
  }

  // ----- Posições derivadas dos membros do conjunto (âncora = peça do meio) -----

  // posicaoMembro devolve o transform de uma peça-MEMBRO relativo ao hub `idHub`: a esquerda cola no
  // buraco esquerdo do hub, a direita no pino direito. Aceita ids distintos (conexão por clique
  // acontece antes de saber se casam) — no par correto membro e hub compartilham o id.
  function posicaoMembro(idMembro: string, coluna: 'esquerda' | 'direita', idHub: string, arrastoX = 0, arrastoY = 0): { x: number; y: number } {
    const baseHub = bases[`meio-${idHub}`];
    const baseMembro = bases[`${coluna}-${idMembro}`];
    if (!baseHub || !baseMembro) return VETOR_ZERO;
    const deslocHub = deslocamentosRef.current[`meio-${idHub}`] || VETOR_ZERO;
    if (coluna === 'esquerda') {
      return {
        x: baseHub.left + deslocHub.x + PROFUNDIDADE_PINO - baseMembro.left - baseMembro.width + arrastoX,
        y: baseHub.top + deslocHub.y - baseMembro.top + arrastoY,
      };
    }
    return {
      x: baseHub.left + baseHub.width + deslocHub.x - PROFUNDIDADE_PINO - baseMembro.left + arrastoX,
      y: baseHub.top + deslocHub.y - baseMembro.top + arrastoY,
    };
  }

  // retanguloRepouso devolve onde a peça ESTÁ EM REPOUSO: o slot do wrapper (nunca transformado,
  // logo sem atraso de transição) somado ao deslocamento de repouso — o próprio, ou o derivado do
  // hub quando a peça já é membro de um conjunto. Medir o card em vez disso devolveria a posição
  // ATRASADA pela transição CSS: durante os ~300ms em que uma peça recém-conectada viaja até o hub,
  // o laço a veria varrendo o tabuleiro e empurraria todas as vizinhas até a parede.
  function retanguloRepouso(id: string, coluna: Coluna): BasePeca | null {
    const wrapper = document.getElementById(`${prefixoWrapper(coluna)}${id}`);
    if (!wrapper) return null;
    const slot = wrapper.getBoundingClientRect();
    let repouso = deslocamentosRef.current[`${coluna}-${id}`] || VETOR_ZERO;
    // Membro conectado: a posição vem do hub (mesmo id no par correto), não do desloc próprio.
    if (coluna === 'esquerda' && paresEsqMidRef.current.includes(id)) repouso = posicaoMembro(id, 'esquerda', id);
    if (coluna === 'direita' && paresMidDirRef.current.includes(id)) repouso = posicaoMembro(id, 'direita', id);
    return { left: slot.left + repouso.x, top: slot.top + repouso.y, width: slot.width, height: slot.height };
  }

  // passoSeparacaoDaMesa executa UM quadro da separação estática: mede as peças em jogo (livres =
  // móveis; membros/hubs de conjunto = pesados, empurram sem sair do lugar), calcula os
  // afastamentos por pares e os aplica DIRETO no repouso persistente, grampeados na arena. Peças em
  // animação (quebrando/deslizando) ficam fora — suas posições momentâneas não são repouso.
  function passoSeparacaoDaMesa(): boolean {
    const pecas: PecaSeparacao[] = [];
    const coletar = (item: ItemQuebraCabeca, coluna: Coluna, movel: boolean) => {
      if (pecaQuebrando?.id === item.id) return;
      if (deslizandoPar && ((deslizandoPar.colMembro === coluna && deslizandoPar.idMembro === item.id) || (coluna === 'meio' && deslizandoPar.idHub === item.id))) return;
      const rect = retanguloRepouso(item.id, coluna);
      if (!rect) return;
      pecas.push({ chave: `${coluna}-${item.id}`, rect, movel });
    };
    itensEsquerda.forEach((o) => coletar(o, 'esquerda', !paresEsqMidRef.current.includes(o.id)));
    itensMeio.forEach((o) => coletar(o, 'meio', !paresEsqMidRef.current.includes(o.id) && !paresMidDirRef.current.includes(o.id)));
    itensDireita.forEach((o) => coletar(o, 'direita', !paresMidDirRef.current.includes(o.id)));

    const deltas = passoSeparacaoEstatica(pecas);
    const chaves = Object.keys(deltas);
    if (chaves.length === 0) return false;

    const arena = arenaRef.current?.getBoundingClientRect();
    const arenaBox = arena
      ? { left: arena.left + ARENA_INSET, top: arena.top + ARENA_INSET, right: arena.right - ARENA_INSET, bottom: arena.bottom - ARENA_INSET }
      : null;
    const rectPorChave = new Map(pecas.map((p) => [p.chave, p.rect]));

    const novos = { ...deslocamentosRef.current };
    let moveu = false;
    chaves.forEach((chave) => {
      let { x, y } = deltas[chave];
      const rect = rectPorChave.get(chave);
      if (arenaBox && rect) {
        const grampeado = grampearNaArena(rect, arenaBox, x, y);
        x = grampeado.x;
        y = grampeado.y;
      }
      if (x === 0 && y === 0) return;
      moveu = true;
      const atual = novos[chave] || VETOR_ZERO;
      novos[chave] = { x: atual.x + x, y: atual.y + y };
    });
    // Sem movimento EFETIVO (tudo grampeado na parede, por exemplo) a mesa não vai melhorar mais:
    // encerra o laço em vez de insistir contra o limite até o teto de tempo.
    if (!moveu) return false;
    deslocamentosRef.current = novos;
    setDeslocamentos(novos);
    return true;
  }

  // iniciarSeparacaoEstatica liga o relaxamento da mesa por até DURACAO_SEPARACAO_ESTATICA (ou até
  // estabilizar): chamado após qualquer assentamento — soltura, encaixe ou retorno da quebra.
  function iniciarSeparacaoEstatica() {
    cancelarSeparacaoRef.current?.();
    cancelarSeparacaoRef.current = repulsaoContinua(
      DURACAO_SEPARACAO_ESTATICA,
      passoSeparacaoDaMesa,
      () => {
        cancelarSeparacaoRef.current = null;
      },
    );
  }

  // ----- Arrasto -----

  function iniciarArrasto(e: React.PointerEvent<HTMLDivElement>, id: string, origem: Coluna) {
    if (origem === 'esquerda' && paresEsqMid.includes(id)) return;
    if (origem === 'direita' && paresMidDir.includes(id)) return;
    if (origem === 'meio' && paresEsqMid.includes(id) && paresMidDir.includes(id)) return;
    if (pecaQuebrando?.id === id || deslizandoPar) return;
    // A separação estática escreve direto no repouso persistente: basta interromper o laço.
    cancelarSeparacaoRef.current?.();
    cancelarSeparacaoRef.current = null;

    e.currentTarget.setPointerCapture(e.pointerId);
    pointerStartRef.current = { x: e.clientX, y: e.clientY };
    const rect = e.currentTarget.getBoundingClientRect();
    slotArrastoRef.current = { left: rect.left, top: rect.top, width: rect.width, height: rect.height };
    const arena = arenaRef.current?.getBoundingClientRect();
    arenaBoxRef.current = arena
      ? { left: arena.left + ARENA_INSET, top: arena.top + ARENA_INSET, right: arena.right - ARENA_INSET, bottom: arena.bottom - ARENA_INSET }
      : null;

    setIdArrastando(id);
    setOrigemArrasto(origem);
    setAlvoArrasto(null);
    setPosicaoArrasto({ x: 0, y: 0 });
    tocarSomClique();
  }

  // iniciarArrastoConjunto é o handle das peças-MEMBRO (esquerda/direita já conectadas, completo ou
  // não): arrastar por elas arrasta o conjunto inteiro, delegando ao hub (origem "meio", mesmo id).
  function iniciarArrastoConjunto(e: React.PointerEvent<HTMLDivElement>, id: string) {
    iniciarArrasto(e, id, 'meio');
  }

  function moverArrasto(e: React.PointerEvent<HTMLDivElement>, id: string) {
    if (idArrastando !== id) return;
    let dx = e.clientX - pointerStartRef.current.x;
    let dy = e.clientY - pointerStartRef.current.y;

    const slot = slotArrastoRef.current;
    const arena = arenaBoxRef.current;
    if (slot && arena) {
      const grampeado = grampearNaArena(slot, arena, dx, dy);
      dx = grampeado.x;
      dy = grampeado.y;
      aplicarParedes(grampeado.paredes);
    }
    setPosicaoArrasto({ x: dx, y: dy });

    const colOrigem = origemArrasto;
    if (!colOrigem) return;

    // A detecção de alvo usa o retângulo do card com o conector relevante (o hub no arrasto de
    // conjunto), mesmo quando o usuário segura por uma peça-membro.
    const elArrastado = document.getElementById(`${prefixoCard(colOrigem)}${id}`);
    if (!elArrastado) return;

    // Alvo: conectores mais próximos (pino↔buraco), com sobreposição grande como fallback.
    // `arrastadoTemPino` diz qual lado do PAR conecta: quem oferece o pino e quem oferece o buraco.
    const rectArrastado = elArrastado.getBoundingClientRect();
    let melhorDestino: PecaEmDestaque | null = null;
    let menorDistancia = Infinity;
    const considerar = (idDestino: string, coluna: Coluna, arrastadoTemPino: boolean) => {
      const el = document.getElementById(`${prefixoCard(coluna)}${idDestino}`);
      if (!el) return;
      const elRect = el.getBoundingClientRect();
      const distancia = arrastadoTemPino
        ? distanciaConectores(rectArrastado, elRect)
        : distanciaConectores(elRect, rectArrastado);
      const s = calcularSobreposicao(rectArrastado, elRect);
      const qualifica = distancia < RAIO_CONEXAO || sobreposicaoAlvo(rectArrastado, elRect, s);
      if (qualifica && distancia < menorDistancia) {
        menorDistancia = distancia;
        melhorDestino = { id: idDestino, coluna };
      }
    };

    if (colOrigem === 'esquerda') {
      // esquerda (pino) → buraco do meio
      itensMeio.forEach((it) => { if (!paresEsqMid.includes(it.id)) considerar(it.id, 'meio', true); });
    } else if (colOrigem === 'direita') {
      // direita (buraco) → pino do meio
      itensMeio.forEach((it) => { if (!paresMidDir.includes(it.id)) considerar(it.id, 'meio', false); });
    } else {
      // meio: pino direito → buraco da direita; buraco esquerdo → pino da esquerda
      if (!paresMidDir.includes(id)) itensDireita.forEach((it) => { if (!paresMidDir.includes(it.id)) considerar(it.id, 'direita', true); });
      if (!paresEsqMid.includes(id)) itensEsquerda.forEach((it) => { if (!paresEsqMid.includes(it.id)) considerar(it.id, 'esquerda', false); });
    }

    setAlvoArrasto(melhorDestino);
    atualizarEmpurroes(rectArrastado, id, colOrigem, melhorDestino);
  }

  function aplicarParedes(novas: ParedesTocadas) {
    setParedes(novas);
    if (algumaParede(novas)) {
      const agora = performance.now();
      if (agora - ultimoSomBarreiraRef.current > 220) {
        ultimoSomBarreiraRef.current = agora;
        tocarSomBarreira();
      }
    }
  }

  // atualizarEmpurroes recalcula o empurrão físico das peças LIVRES: fora ficam a arrastada, o alvo
  // atual (empurrá-lo o tiraria de debaixo do encaixe) e qualquer peça já conectada — conjuntos
  // encaixados são "pesados" e não se movem sozinhos.
  function atualizarEmpurroes(rectArrastado: DOMRect, idArrastadoAtual: string, colOrigem: Coluna, alvo: PecaEmDestaque | null) {
    const novos: MapaVetores = {};
    const empurrarSePerto = (idPeca: string, coluna: Coluna) => {
      if (alvo && alvo.id === idPeca && alvo.coluna === coluna) return;
      if (coluna === colOrigem && idPeca === idArrastadoAtual) return;
      const rect = retanguloRepouso(idPeca, coluna);
      if (!rect) return;
      const empurrao = calcularEmpurrao(rectArrastado, rect);
      if (empurrao) novos[`${coluna}-${idPeca}`] = empurrao;
    };

    itensEsquerda.forEach((o) => { if (!paresEsqMid.includes(o.id)) empurrarSePerto(o.id, 'esquerda'); });
    itensMeio.forEach((o) => { if (!paresEsqMid.includes(o.id) && !paresMidDir.includes(o.id)) empurrarSePerto(o.id, 'meio'); });
    itensDireita.forEach((o) => { if (!paresMidDir.includes(o.id)) empurrarSePerto(o.id, 'direita'); });

    empurroesRef.current = novos;
    setEmpurroes(novos);
  }

  // processarConexao efetiva (ou quebra) uma ligação. O vínculo é definido pelas colunas envolvidas:
  // {esquerda,meio} = esq_mid; {meio,direita} = mid_dir. `deltaArrasto` propaga o repouso quando foi
  // o HUB que se arrastou (o conjunto fica onde parou).
  function processarConexao(idOrigem: string, colOrigem: Coluna, idDestino: string, colDestino: Coluna, deltaArrasto = VETOR_ZERO) {
    setAlvoArrasto(null);
    const ehEsqMid = (colOrigem === 'esquerda' || colDestino === 'esquerda');
    const idBond = idOrigem; // no par correto idOrigem === idDestino

    if (combinacaoCorreta(idOrigem, idDestino)) {
      // Se o HUB (meio) foi o arrastado, o conjunto fica onde parou (o delta vira repouso).
      if (colOrigem === 'meio') somarDeslocamento(`meio-${idOrigem}`, deltaArrasto);
      setPosicaoArrasto({ x: 0, y: 0 });
      tocarSomEncaixe();

      let novosEsqMid = paresEsqMidRef.current;
      let novosMidDir = paresMidDirRef.current;
      if (ehEsqMid) {
        novosEsqMid = [...novosEsqMid, idBond];
        paresEsqMidRef.current = novosEsqMid;
        setParesEsqMid(novosEsqMid);
        if (aoAcertarItem) aoAcertarItem(idBond, 'esq_mid');
      } else {
        novosMidDir = [...novosMidDir, idBond];
        paresMidDirRef.current = novosMidDir;
        setParesMidDir(novosMidDir);
        if (aoAcertarItem) aoAcertarItem(idBond, 'mid_dir');
      }

      const concluidos = itensEsquerda.filter((item) => novosEsqMid.includes(item.id) && novosMidDir.includes(item.id));
      if (concluidos.length === itensEsquerda.length) {
        setTimeout(() => aoConcluir(true, concluidos.map((i) => i.id)), 600);
      }
    } else {
      if (aoErrarItem) aoErrarItem(idOrigem, idDestino);
      const novasVidas = vidas - 1;
      setVidas(novasVidas);
      setAlvoErro({ id: idDestino, coluna: colDestino });
      setPecaQuebrando({ id: idOrigem, coluna: colOrigem });

      setTimeout(() => {
        setPecaQuebrando(null);
        setAlvoErro(null);
        setPosicaoArrasto({ x: 0, y: 0 });
        // A peça reaparece no repouso — pode ter voltado sobre alguém: relaxa a mesa de novo.
        iniciarSeparacaoEstatica();
      }, DURACAO_QUEBRA);

      if (novasVidas <= 0) {
        const parciais = itensEsquerda.filter((item) => paresEsqMid.includes(item.id) || paresMidDir.includes(item.id)).map((i) => i.id);
        setTimeout(() => aoConcluir(false, parciais), DURACAO_QUEBRA + 100);
      }
    }
  }

  function pararArrasto(e: React.PointerEvent<HTMLDivElement>, id: string) {
    if (idArrastando !== id) return;

    e.currentTarget.releasePointerCapture(e.pointerId);
    const colOrigem = origemArrasto;
    setIdArrastando(null);
    setOrigemArrasto(null);
    setParedes(SEM_PAREDES);
    // Os empurrões do arrasto assentam já; a separação estática (por pares) segue dali sem salto.
    assentarEmpurroes();

    const movimento = Math.hypot(e.clientX - pointerStartRef.current.x, e.clientY - pointerStartRef.current.y);
    if (movimento < 6 && colOrigem) {
      setAlvoArrasto(null);
      setPosicaoArrasto({ x: 0, y: 0 });
      tratarCliquePeca(id, colOrigem);
      return;
    }
    if (!colOrigem) {
      setAlvoArrasto(null);
      return;
    }

    const alvo = alvoArrasto;
    setAlvoArrasto(null);

    if (!alvo) {
      // Soltura livre: a peça (ou o conjunto, ancorado no hub) FICA onde foi solta; a separação
      // estática continua abrindo espaço nas vizinhas por mais um tempo.
      somarDeslocamento(`${colOrigem}-${id}`, posicaoArrasto);
      setPosicaoArrasto({ x: 0, y: 0 });
      iniciarSeparacaoEstatica();
      return;
    }

    processarConexao(id, colOrigem, alvo.id, alvo.coluna, posicaoArrasto);
    iniciarSeparacaoEstatica();
  }

  // ----- Conexão por clique (meet-in-the-middle) -----

  // deslizarEConectar move o MEMBRO e o HUB, cada um pela metade da distância relativa necessária —
  // encontram-se no meio, simétrico — e só então resolve, sem salto (o fim do lerp já coincide com o
  // repouso docked do conjunto).
  function deslizarEConectar(idMembro: string, colMembro: 'esquerda' | 'direita', idHub: string) {
    const deslocMembroAtual = deslocamentosRef.current[`${colMembro}-${idMembro}`] || VETOR_ZERO;
    // Quanto o membro precisaria se mover, sozinho, para encostar no hub (que ficaria parado).
    const alvoMembroSeHubFicasse = posicaoMembro(idMembro, colMembro, idHub);
    const deltaTotal = { x: alvoMembroSeHubFicasse.x - deslocMembroAtual.x, y: alvoMembroSeHubFicasse.y - deslocMembroAtual.y };
    // Metade para cada lado, em sentidos opostos: as duas peças se encontram no meio do caminho.
    const metadeMembro = { x: deltaTotal.x / 2, y: deltaTotal.y / 2 };
    const metadeHub = { x: -deltaTotal.x / 2, y: -deltaTotal.y / 2 };

    cancelarDeslizeRef.current?.();
    cancelarDeslizeRef.current = animarProgresso(
      DURACAO_DESLIZE,
      (suave) =>
        setDeslizandoPar({
          idMembro,
          colMembro,
          idHub,
          offsetMembro: { x: metadeMembro.x * suave, y: metadeMembro.y * suave },
          offsetHub: { x: metadeHub.x * suave, y: metadeHub.y * suave },
        }),
      () => {
        setDeslizandoPar(null);
        cancelarDeslizeRef.current = null;
        somarDeslocamento(`${colMembro}-${idMembro}`, metadeMembro);
        somarDeslocamento(`meio-${idHub}`, metadeHub);
        processarConexao(idMembro, colMembro, idHub, 'meio');
        iniciarSeparacaoEstatica();
      },
    );
  }

  function tratarCliquePeca(id: string, coluna: Coluna) {
    if (coluna === 'esquerda' && paresEsqMid.includes(id)) return;
    if (coluna === 'meio' && paresEsqMid.includes(id) && paresMidDir.includes(id)) return;
    if (coluna === 'direita' && paresMidDir.includes(id)) return;
    if (pecaQuebrando?.id === id || deslizandoPar) return;
    // O "tique" já tocou no pointer-down (iniciarArrasto); esta função só chega via soltura curta.

    if (!pecaSelecionada) {
      setPecaSelecionada({ id, coluna });
      return;
    }
    if (pecaSelecionada.id === id && pecaSelecionada.coluna === coluna) {
      setPecaSelecionada(null);
      return;
    }
    if (pecaSelecionada.coluna === coluna) {
      setPecaSelecionada({ id, coluna });
      return;
    }

    const sel = pecaSelecionada;
    setPecaSelecionada(null);

    // A ligação existe só entre esquerda↔meio e meio↔direita; a peça-membro (esquerda/direita)
    // encontra o hub (meio) no meio do caminho. Um par esquerda↔direita não conecta: vira seleção.
    const colunas = new Set([sel.coluna, coluna]);
    if (colunas.has('esquerda') && colunas.has('meio')) {
      const idMembro = sel.coluna === 'esquerda' ? sel.id : id;
      const idHub = sel.coluna === 'meio' ? sel.id : id;
      deslizarEConectar(idMembro, 'esquerda', idHub);
    } else if (colunas.has('direita') && colunas.has('meio')) {
      const idMembro = sel.coluna === 'direita' ? sel.id : id;
      const idHub = sel.coluna === 'meio' ? sel.id : id;
      deslizarEConectar(idMembro, 'direita', idHub);
    } else {
      setPecaSelecionada({ id, coluna });
    }
  }

  function calcularSobreposicao(r1: DOMRect, r2: DOMRect): number {
    const xOverlap = Math.max(0, Math.min(r1.right, r2.right) - Math.max(r1.left, r2.left));
    const yOverlap = Math.max(0, Math.min(r1.bottom, r2.bottom) - Math.max(r1.top, r2.top));
    return xOverlap * yOverlap;
  }

  function renderizarFragmentos(item: ItemQuebraCabeca, pinoDireita: boolean, buracoEsquerda: boolean) {
    return (
      <div className="puzzle-fragmentos">
        <div className="puzzle-fragmento metade-esquerda">
          <FormaPeca pinoDireita={pinoDireita} buracoEsquerda={buracoEsquerda} />
          <div className="puzzle-card-conteudo">{item.conteudo}</div>
        </div>
        <div className="puzzle-fragmento metade-direita">
          <FormaPeca pinoDireita={pinoDireita} buracoEsquerda={buracoEsquerda} />
          <div className="puzzle-card-conteudo">{item.conteudo}</div>
        </div>
      </div>
    );
  }

  // posicaoLivre resolve o transform de uma peça LIVRE (não-membro) das colunas esquerda/direita.
  function posicaoLivre(id: string, coluna: 'esquerda' | 'direita', arrastandoSelf: boolean, quebrandoSelf: boolean): { x: number; y: number } {
    const desloc = deslocamentos[`${coluna}-${id}`] || VETOR_ZERO;
    if (arrastandoSelf || quebrandoSelf) {
      return { x: desloc.x + posicaoArrasto.x, y: desloc.y + posicaoArrasto.y };
    }
    if (deslizandoPar && deslizandoPar.colMembro === coluna && deslizandoPar.idMembro === id) {
      return { x: desloc.x + deslizandoPar.offsetMembro.x, y: desloc.y + deslizandoPar.offsetMembro.y };
    }
    const empurrao = empurroes[`${coluna}-${id}`] || VETOR_ZERO;
    return { x: desloc.x + empurrao.x, y: desloc.y + empurrao.y };
  }

  return (
    <div className="revisao-puzzle-trio-container">
      {/* Vidas & Instruções */}
      <div className="revisao-puzzle-cabecalho">
        <div className="revisao-puzzle-vidas">
          {Array.from({ length: vidasIniciais }).map((_, idx) => {
            const ativa = idx < vidas;
            return (
              <span key={idx} className={`revisao-puzzle-coracao ${ativa ? 'ativo' : 'quebrado'}`}>
                {ativa ? <IconeCoracao tamanho={22} /> : <IconeCoracaoPartido tamanho={22} />}
              </span>
            );
          })}
        </div>
        <div className="revisao-puzzle-instrucao">
          {instrucao || t('Arraste ou clique nas peças para conectar Hanzi (esquerda), Significado (meio) e Som (direita)')}
        </div>
      </div>

      <div ref={arenaRef} className={`revisao-puzzle-trio-tabuleiro revisao-puzzle-arena ${algumaParede(paredes) ? 'barreira-ativa' : ''}`}>
        <span className={`puzzle-parede cima ${paredes.cima ? 'ativa' : ''}`} aria-hidden="true" />
        <span className={`puzzle-parede baixo ${paredes.baixo ? 'ativa' : ''}`} aria-hidden="true" />
        <span className={`puzzle-parede esquerda ${paredes.esquerda ? 'ativa' : ''}`} aria-hidden="true" />
        <span className={`puzzle-parede direita ${paredes.direita ? 'ativa' : ''}`} aria-hidden="true" />

        {/* Coluna 1: Hanzi (Esquerda - Pino no lado direito) */}
        <div className="revisao-puzzle-trio-coluna esquerda">
          {itensEsquerda.map((item) => {
            const membro = paresEsqMid.includes(item.id);
            const arrastandoSelf = idArrastando === item.id && origemArrasto === 'esquerda';
            const arrastandoConjunto = idArrastando === item.id && origemArrasto === 'meio';
            const quebrandoSelf = pecaQuebrando?.id === item.id && pecaQuebrando.coluna === 'esquerda';
            const quebrandoConjunto = pecaQuebrando?.id === item.id && pecaQuebrando.coluna === 'meio' && membro;
            const ehAlvoErro = alvoErro?.id === item.id && alvoErro.coluna === 'esquerda';
            const selecionado = pecaSelecionada?.id === item.id && pecaSelecionada?.coluna === 'esquerda';
            const ehAlvoArrasto = alvoArrasto?.id === item.id && alvoArrasto.coluna === 'esquerda';

            const posicao = membro
              ? posicaoMembro(item.id, 'esquerda', item.id, arrastandoConjunto || quebrandoConjunto ? posicaoArrasto.x : 0, arrastandoConjunto || quebrandoConjunto ? posicaoArrasto.y : 0)
              : posicaoLivre(item.id, 'esquerda', arrastandoSelf, quebrandoSelf);
            const estiloTransform = posicao.x || posicao.y ? `translate(${posicao.x}px, ${posicao.y}px)` : 'none';

            return (
              <div key={item.id} id={`wrapper-trio-esq-${item.id}`} className="puzzle-card-wrapper">
                <div
                  id={`card-trio-esq-${item.id}`}
                  className={`puzzle-card-left ${membro ? 'resolvido' : ''} ${arrastandoSelf ? 'arrastando' : ''} ${arrastandoConjunto || quebrandoConjunto ? 'seguindo' : ''} ${quebrandoSelf ? 'quebrando' : ''} ${ehAlvoErro ? 'repelido' : ''} ${selecionado ? 'selecionado' : ''} ${ehAlvoArrasto ? 'alvo-arrasto' : ''}`}
                  style={{ transform: estiloTransform }}
                  onPointerDown={(e) => (membro ? iniciarArrastoConjunto(e, item.id) : iniciarArrasto(e, item.id, 'esquerda'))}
                  onPointerMove={(e) => moverArrasto(e, item.id)}
                  onPointerUp={(e) => pararArrasto(e, item.id)}
                >
                  <FormaPeca pinoDireita />
                  <div className="puzzle-card-conteudo">{item.conteudo}</div>
                  {quebrandoSelf && renderizarFragmentos(item, true, false)}
                </div>
              </div>
            );
          })}
        </div>

        {/* Coluna 2: Significado (Meio - o HUB do conjunto: buraco na esquerda, pino na direita) */}
        <div className="revisao-puzzle-trio-coluna meio">
          {itensMeio.map((item) => {
            const conectadoDir = paresMidDir.includes(item.id);
            const conjuntoCompleto = paresEsqMid.includes(item.id) && conectadoDir;
            const arrastando = idArrastando === item.id && origemArrasto === 'meio';
            const quebrando = pecaQuebrando?.id === item.id && pecaQuebrando.coluna === 'meio';
            const ehAlvoErro = alvoErro?.id === item.id && alvoErro.coluna === 'meio';
            const selecionado = pecaSelecionada?.id === item.id && pecaSelecionada?.coluna === 'meio';
            const ehAlvoArrasto = alvoArrasto?.id === item.id && alvoArrasto.coluna === 'meio';
            const deslizandoEsteHub = deslizandoPar?.idHub === item.id;

            const desloc = deslocamentos[`meio-${item.id}`] || VETOR_ZERO;
            const empurrao = empurroes[`meio-${item.id}`] || VETOR_ZERO;
            const emArrasto = arrastando || quebrando;
            const offsetExtra = emArrasto ? posicaoArrasto : deslizandoEsteHub ? deslizandoPar!.offsetHub : empurrao;
            const posicaoX = desloc.x + offsetExtra.x;
            const posicaoY = desloc.y + offsetExtra.y;
            const estiloTransform = posicaoX || posicaoY ? `translate(${posicaoX}px, ${posicaoY}px)` : 'none';

            return (
              <div key={item.id} id={`wrapper-trio-mid-${item.id}`} className="puzzle-card-wrapper">
                <div
                  id={`card-trio-mid-${item.id}`}
                  className={`puzzle-card-middle ${conectadoDir ? 'resolvido' : ''} ${conjuntoCompleto ? 'completo' : ''} ${arrastando ? 'arrastando' : ''} ${quebrando ? 'quebrando' : ''} ${deslizandoEsteHub ? 'deslizando' : ''} ${ehAlvoErro ? 'repelido' : ''} ${selecionado ? 'selecionado' : ''} ${ehAlvoArrasto ? 'alvo-arrasto' : ''}`}
                  style={{ transform: estiloTransform }}
                  onPointerDown={(e) => iniciarArrasto(e, item.id, 'meio')}
                  onPointerMove={(e) => moverArrasto(e, item.id)}
                  onPointerUp={(e) => pararArrasto(e, item.id)}
                >
                  <FormaPeca pinoDireita buracoEsquerda />
                  <div className="puzzle-card-conteudo">{item.conteudo}</div>
                  {quebrando && renderizarFragmentos(item, true, true)}
                </div>
              </div>
            );
          })}
        </div>

        {/* Coluna 3: Fonética (Direita - Buraco na esquerda; livre arrasta para o meio, membro cola no hub) */}
        <div className="revisao-puzzle-trio-coluna direita">
          {itensDireita.map((item) => {
            const membro = paresMidDir.includes(item.id);
            const arrastandoSelf = !membro && idArrastando === item.id && origemArrasto === 'direita';
            const arrastandoConjunto = membro && idArrastando === item.id && origemArrasto === 'meio';
            const quebrandoSelf = pecaQuebrando?.id === item.id && pecaQuebrando.coluna === 'direita';
            const quebrandoConjunto = membro && pecaQuebrando?.id === item.id && pecaQuebrando.coluna === 'meio';
            const ehAlvoErro = alvoErro?.id === item.id && alvoErro.coluna === 'direita';
            const selecionado = pecaSelecionada?.id === item.id && pecaSelecionada?.coluna === 'direita';
            const ehAlvoArrasto = alvoArrasto?.id === item.id && alvoArrasto.coluna === 'direita';

            const posicao = membro
              ? posicaoMembro(item.id, 'direita', item.id, arrastandoConjunto || quebrandoConjunto ? posicaoArrasto.x : 0, arrastandoConjunto || quebrandoConjunto ? posicaoArrasto.y : 0)
              : posicaoLivre(item.id, 'direita', arrastandoSelf, quebrandoSelf);
            const estiloTransform = posicao.x || posicao.y ? `translate(${posicao.x}px, ${posicao.y}px)` : 'none';

            return (
              <div key={item.id} id={`wrapper-trio-dir-${item.id}`} className="puzzle-card-wrapper">
                <div
                  id={`card-trio-dir-${item.id}`}
                  className={`puzzle-card-right ${membro ? 'resolvido' : ''} ${arrastandoSelf ? 'arrastando' : ''} ${arrastandoConjunto || quebrandoConjunto ? 'seguindo' : ''} ${quebrandoSelf ? 'quebrando' : ''} ${ehAlvoErro ? 'repelido' : ''} ${selecionado ? 'selecionado' : ''} ${ehAlvoArrasto ? 'alvo-arrasto' : ''}`}
                  style={{ transform: estiloTransform }}
                  onPointerDown={(e) => (membro ? iniciarArrastoConjunto(e, item.id) : iniciarArrasto(e, item.id, 'direita'))}
                  onPointerMove={(e) => moverArrasto(e, item.id)}
                  onPointerUp={(e) => pararArrasto(e, item.id)}
                >
                  <FormaPeca buracoEsquerda />
                  <div className="puzzle-card-conteudo">{item.conteudo}</div>
                  {quebrandoSelf && renderizarFragmentos(item, false, true)}
                </div>
              </div>
            );
          })}
        </div>
      </div>
    </div>
  );
}
