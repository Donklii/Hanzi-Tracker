// ----- Seção: Componente Modular de Jogo de Quebra-Cabeça (2 Colunas) -----
// O tabuleiro é uma MESA FÍSICA cercada por uma barreira: cada peça (de qualquer coluna) pode ser
// arrastada e fica onde é solta, mas não pode ser retirada da arena central — ao esbarrar num
// limite a parede acende (roxo) e a peça é grampeada de volta para dentro. Pares já conectados
// continuam arrastáveis (arrastar qualquer um dos dois move o par inteiro, rigidamente). Peças
// conectadas por clique se movem uma em direção à outra e se encontram no meio (lerp simétrico)
// antes de o par se formar. Sons reativos pontuam o gesto: "tique" ao pegar, "clô" ao encaixar,
// "toc" ao bater na parede.
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

type Coluna = 'esquerda' | 'direita';

interface JogoQuebraCabecaProps {
  itensEsquerda: ItemQuebraCabeca[];
  itensDireita: ItemQuebraCabeca[];
  combinacaoCorreta: (idEsq: string, idDir: string) => boolean;
  vidasIniciais?: number;
  instrucao?: string;
  // colunaDireitaCompacta estreita a coluna de destino para conteúdos curtos (ex.: pinyin da
  // fonética), evitando cards com muito espaço vazio horizontal.
  colunaDireitaCompacta?: boolean;
  aoConcluir: (acertou: boolean, matchedIds: string[]) => void;
  aoAcertarItem?: (idEsq: string, idDir: string) => void;
  aoErrarItem?: (idEsq: string, idDir: string) => void;
}

// DURACAO_QUEBRA cobre a animação dos fragmentos da peça quebrando (ver revisao.css) com uma folga
// pequena antes de restaurar a peça inteira no seu lugar de descanso.
const DURACAO_QUEBRA = 650;
// ARENA_INSET é a margem (px) entre a borda da arena e o limite de grampeamento das peças —
// deixa a parede visível "por dentro" do esbarrão.
const ARENA_INSET = 8;

interface BasePeca {
  left: number;
  top: number;
  width: number;
  height: number;
}

// retanguloUniao devolve o menor retângulo que contém os dois — usado para grampear na arena o
// par INTEIRO (as duas peças) durante o arrasto de um conjunto já conectado.
function retanguloUniao(a: DOMRect, b: DOMRect): BasePeca {
  const left = Math.min(a.left, b.left);
  const top = Math.min(a.top, b.top);
  const right = Math.max(a.right, b.right);
  const bottom = Math.max(a.bottom, b.bottom);
  return { left, top, width: right - left, height: bottom - top };
}

export function JogoQuebraCabeca({
  itensEsquerda,
  itensDireita,
  combinacaoCorreta,
  vidasIniciais = 2,
  instrucao,
  colunaDireitaCompacta,
  aoConcluir,
  aoAcertarItem,
  aoErrarItem,
}: JogoQuebraCabecaProps) {
  const [vidas, setVidas] = useState(vidasIniciais);
  const [paresMatched, setParesMatched] = useState<string[]>([]);
  // ancoras[id] = coluna que FICA como referência do par (a outra peça é sempre posicionada
  // adjacente a ela). Ao arrastar um par, o delta é acumulado no desloc da âncora.
  const [ancoras, setAncoras] = useState<Record<string, Coluna>>({});
  // Espelhos em ref de paresMatched/ancoras: o laço da separação estática roda em rAF sobre a
  // CLAUSURA do render que o iniciou, e ele é iniciado no mesmo tick em que o par se forma. Lendo o
  // estado dali, o par recém-conectado ainda apareceria como duas peças LIVRES sobrepostas pelo pino
  // — e como elas são rígidas entre si a sobreposição nunca se resolveria: o laço empurraria o par
  // até a parede. Os refs dão ao laço a verdade do instante.
  const paresMatchedRef = useRef<string[]>([]);
  const ancorasRef = useRef<Record<string, Coluna>>({});

  const [arrasto, setArrasto] = useState<{ id: string; coluna: Coluna } | null>(null);
  const [posicaoArrasto, setPosicaoArrasto] = useState({ x: 0, y: 0 });
  const [alvoArrasto, setAlvoArrasto] = useState<{ id: string; coluna: Coluna } | null>(null);

  const [erroTemporario, setErroTemporario] = useState<{ id: string; coluna: Coluna } | null>(null);
  const [quebrando, setQuebrando] = useState<{ id: string; coluna: Coluna } | null>(null);

  // deslizandoPar é o lerp de conexão por clique: as DUAS peças se movem, cada uma pela metade da
  // distância necessária, até se encontrarem no meio (offsetEsq/offsetDir somam ao desloc de cada).
  const [deslizandoPar, setDeslizandoPar] = useState<{
    idEsq: string;
    idDir: string;
    offsetEsq: { x: number; y: number };
    offsetDir: { x: number; y: number };
  } | null>(null);

  const [bases, setBases] = useState<Record<string, BasePeca>>({});
  const [deslocamentos, setDeslocamentos] = useState<MapaVetores>({});
  const deslocamentosRef = useRef<MapaVetores>({});
  const [empurroes, setEmpurroes] = useState<MapaVetores>({});
  const empurroesRef = useRef<MapaVetores>({});

  const [paredes, setParedes] = useState<ParedesTocadas>(SEM_PAREDES);

  const [pecaSelecionada, setPecaSelecionada] = useState<{ id: string; coluna: Coluna } | null>(null);

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
  }, [itensEsquerda, itensDireita, paresMatched]);

  useEffect(() => () => {
    cancelarDeslizeRef.current?.();
    cancelarSeparacaoRef.current?.();
  }, []);

  // medirBases captura o slot de layout de cada peça pelos WRAPPERS (nunca transformados). As
  // diferenças entre bases é que importam, então rolagem não invalida a medição.
  function medirBases() {
    const novas: Record<string, BasePeca> = {};
    const medir = (chave: string, idElemento: string) => {
      const el = document.getElementById(idElemento);
      if (!el) return;
      const r = el.getBoundingClientRect();
      novas[chave] = { left: r.left, top: r.top, width: r.width, height: r.height };
    };
    itensEsquerda.forEach((item) => medir(`esq-${item.id}`, `wrapper-esq-${item.id}`));
    itensDireita.forEach((item) => medir(`dir-${item.id}`, `wrapper-dir-${item.id}`));
    setBases(novas);
  }

  function prefixoColuna(coluna: Coluna) {
    return coluna === 'esquerda' ? 'esq' : 'dir';
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

  // ----- Posição de repouso de um par conectado (o mover cola na âncora) -----

  // posicaoMoverAdjacente devolve o transform da peça-MOVER de um par para encostar na âncora: o
  // pino da esquerda entra PROFUNDIDADE_PINO px no buraco da direita.
  function posicaoMoverAdjacente(id: string, mover: Coluna): { x: number; y: number } {
    const baseEsq = bases[`esq-${id}`];
    const baseDir = bases[`dir-${id}`];
    if (!baseEsq || !baseDir) return VETOR_ZERO;
    if (mover === 'esquerda') {
      const deslocDir = deslocamentosRef.current[`dir-${id}`] || VETOR_ZERO;
      return {
        x: baseDir.left + deslocDir.x + PROFUNDIDADE_PINO - baseEsq.left - baseEsq.width,
        y: baseDir.top + deslocDir.y - baseEsq.top,
      };
    }
    const deslocEsq = deslocamentosRef.current[`esq-${id}`] || VETOR_ZERO;
    return {
      x: baseEsq.left + deslocEsq.x + baseEsq.width - PROFUNDIDADE_PINO - baseDir.left,
      y: baseEsq.top + deslocEsq.y - baseDir.top,
    };
  }

  // posicaoMoverAdjacenteComAncora é posicaoMoverAdjacente para um par idMover/idAncora que ainda
  // NÃO compartilham id (conexão por clique acontece antes de saber se casam): usa as bases de cada
  // peça e o desloc da âncora.
  function posicaoMoverAdjacenteComAncora(idMover: string, colunaMover: Coluna, idAncora: string, colunaAncora: Coluna): { x: number; y: number } {
    const baseMover = bases[`${prefixoColuna(colunaMover)}-${idMover}`];
    const baseAncora = bases[`${prefixoColuna(colunaAncora)}-${idAncora}`];
    if (!baseMover || !baseAncora) return VETOR_ZERO;
    const deslocAncora = deslocamentosRef.current[`${prefixoColuna(colunaAncora)}-${idAncora}`] || VETOR_ZERO;
    if (colunaMover === 'esquerda') {
      return {
        x: baseAncora.left + deslocAncora.x + PROFUNDIDADE_PINO - baseMover.left - baseMover.width,
        y: baseAncora.top + deslocAncora.y - baseMover.top,
      };
    }
    return {
      x: baseAncora.left + deslocAncora.x + baseAncora.width - PROFUNDIDADE_PINO - baseMover.left,
      y: baseAncora.top + deslocAncora.y - baseMover.top,
    };
  }

  // ----- Arrasto -----

  // retanguloRepouso devolve onde a peça ESTÁ EM REPOUSO: o slot do wrapper (nunca transformado,
  // logo sem atraso de transição) somado ao deslocamento de repouso — o próprio, ou o derivado da
  // âncora quando a peça é a metade "mover" de um par. Medir o card em vez disso devolveria a
  // posição ATRASADA pela transição CSS: durante os ~300ms em que uma peça recém-conectada viaja
  // até a âncora, o laço a veria varrendo o tabuleiro e empurraria todas as vizinhas até a parede.
  function retanguloRepouso(id: string, coluna: Coluna): BasePeca | null {
    const wrapper = document.getElementById(`wrapper-${prefixoColuna(coluna)}-${id}`);
    if (!wrapper) return null;
    const slot = wrapper.getBoundingClientRect();
    const repouso = paresMatchedRef.current.includes(id) && ancorasRef.current[id] !== coluna
      ? posicaoMoverAdjacente(id, coluna)
      : deslocamentosRef.current[`${prefixoColuna(coluna)}-${id}`] || VETOR_ZERO;
    return { left: slot.left + repouso.x, top: slot.top + repouso.y, width: slot.width, height: slot.height };
  }

  // passoSeparacaoDaMesa executa UM quadro da separação estática: mede as peças em jogo (livres =
  // móveis; membros de par = pesados, empurram sem sair do lugar), calcula os afastamentos por
  // pares e os aplica DIRETO no repouso persistente, grampeados na arena. Peças em animação
  // (quebrando/deslizando) ficam fora — suas posições momentâneas não são repouso.
  function passoSeparacaoDaMesa(): boolean {
    const pecas: PecaSeparacao[] = [];
    const coletar = (item: ItemQuebraCabeca, coluna: Coluna) => {
      if (quebrando?.id === item.id && quebrando.coluna === coluna) return;
      if (deslizandoPar && ((coluna === 'esquerda' && deslizandoPar.idEsq === item.id) || (coluna === 'direita' && deslizandoPar.idDir === item.id))) return;
      const rect = retanguloRepouso(item.id, coluna);
      if (!rect) return;
      pecas.push({
        chave: `${prefixoColuna(coluna)}-${item.id}`,
        rect,
        movel: !paresMatchedRef.current.includes(item.id),
      });
    };
    itensEsquerda.forEach((it) => coletar(it, 'esquerda'));
    itensDireita.forEach((it) => coletar(it, 'direita'));

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

  function iniciarArrasto(e: React.PointerEvent<HTMLDivElement>, id: string, colunaClicada: Coluna) {
    if (quebrando || deslizandoPar) return;
    // A separação estática escreve direto no repouso persistente: basta interromper o laço.
    cancelarSeparacaoRef.current?.();
    cancelarSeparacaoRef.current = null;

    const resolvido = paresMatched.includes(id);
    // Pares já conectados são arrastados sempre pela referência da ÂNCORA, não importa qual das
    // duas peças o usuário segurou — o par se move como um corpo rígido.
    const coluna = resolvido ? ancoras[id] : colunaClicada;

    e.currentTarget.setPointerCapture(e.pointerId);
    pointerStartRef.current = { x: e.clientX, y: e.clientY };

    if (resolvido) {
      const elEsq = document.getElementById(`card-esq-${id}`);
      const elDir = document.getElementById(`card-dir-${id}`);
      slotArrastoRef.current = elEsq && elDir
        ? retanguloUniao(elEsq.getBoundingClientRect(), elDir.getBoundingClientRect())
        : e.currentTarget.getBoundingClientRect();
    } else {
      const rect = e.currentTarget.getBoundingClientRect();
      slotArrastoRef.current = { left: rect.left, top: rect.top, width: rect.width, height: rect.height };
    }

    const arena = arenaRef.current?.getBoundingClientRect();
    arenaBoxRef.current = arena
      ? { left: arena.left + ARENA_INSET, top: arena.top + ARENA_INSET, right: arena.right - ARENA_INSET, bottom: arena.bottom - ARENA_INSET }
      : null;

    setArrasto({ id, coluna });
    setAlvoArrasto(null);
    setPosicaoArrasto({ x: 0, y: 0 });
    tocarSomClique();
  }

  function moverArrasto(e: React.PointerEvent<HTMLDivElement>, id: string, coluna: Coluna) {
    const resolvido = paresMatched.includes(id);
    // Peça de um par já conectado: identificada só pelo id (a âncora fixa a referência do estado,
    // não a coluna fisicamente segurada). Peça livre: id E coluna, para não confundir com a
    // contraparte ainda não casada do mesmo id na coluna oposta.
    const combina = resolvido ? arrasto?.id === id : arrasto?.id === id && arrasto.coluna === coluna;
    if (!combina) return;

    let dx = e.clientX - pointerStartRef.current.x;
    let dy = e.clientY - pointerStartRef.current.y;

    // Barreira: grampeia o delta para a peça (ou o par inteiro) caber na arena; acende as paredes.
    const slot = slotArrastoRef.current;
    const arena = arenaBoxRef.current;
    if (slot && arena) {
      const grampeado = grampearNaArena(slot, arena, dx, dy);
      dx = grampeado.x;
      dy = grampeado.y;
      aplicarParedes(grampeado.paredes);
    }
    setPosicaoArrasto({ x: dx, y: dy });

    // Par já conectado: só se move, sem tentar nova conexão nem empurrar peças livres.
    if (resolvido) return;

    // Alvo: peça livre da coluna OPOSTA com os CONECTORES mais próximos (pino↔buraco); a
    // sobreposição grande vale como fallback (soltar a peça por cima da outra).
    const elArrastado = document.getElementById(`card-${prefixoColuna(coluna)}-${id}`);
    if (!elArrastado) return;
    const rectArrastado = elArrastado.getBoundingClientRect();
    const alvos = coluna === 'esquerda' ? itensDireita : itensEsquerda;
    const colunaAlvo: Coluna = coluna === 'esquerda' ? 'direita' : 'esquerda';

    let melhorId: string | null = null;
    let menorDistancia = Infinity;
    alvos.forEach((outro) => {
      if (paresMatched.includes(outro.id)) return;
      const el = document.getElementById(`card-${prefixoColuna(colunaAlvo)}-${outro.id}`);
      if (!el) return;
      const elRect = el.getBoundingClientRect();
      const distancia = coluna === 'esquerda'
        ? distanciaConectores(rectArrastado, elRect)
        : distanciaConectores(elRect, rectArrastado);
      const s = calcularSobreposicao(rectArrastado, elRect);
      const qualifica = distancia < RAIO_CONEXAO || sobreposicaoAlvo(rectArrastado, elRect, s);
      if (qualifica && distancia < menorDistancia) {
        menorDistancia = distancia;
        melhorId = outro.id;
      }
    });
    setAlvoArrasto(melhorId ? { id: melhorId, coluna: colunaAlvo } : null);
    atualizarEmpurroes(rectArrastado, { id, coluna }, melhorId ? { id: melhorId, coluna: colunaAlvo } : null);
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

  function atualizarEmpurroes(rectArrastado: DOMRect, arrastada: { id: string; coluna: Coluna }, alvo: { id: string; coluna: Coluna } | null) {
    const novos: MapaVetores = {};
    const considerar = (item: ItemQuebraCabeca, coluna: Coluna) => {
      if (coluna === arrastada.coluna && item.id === arrastada.id) return;
      if (alvo && coluna === alvo.coluna && item.id === alvo.id) return;
      if (paresMatched.includes(item.id)) return;
      const rect = retanguloRepouso(item.id, coluna);
      if (!rect) return;
      const empurrao = calcularEmpurrao(rectArrastado, rect);
      if (empurrao) novos[`${prefixoColuna(coluna)}-${item.id}`] = empurrao;
    };
    itensEsquerda.forEach((it) => considerar(it, 'esquerda'));
    itensDireita.forEach((it) => considerar(it, 'direita'));
    empurroesRef.current = novos;
    setEmpurroes(novos);
  }

  function pararArrasto(e: React.PointerEvent<HTMLDivElement>, id: string, coluna: Coluna) {
    const resolvido = paresMatched.includes(id);
    const combina = resolvido ? arrasto?.id === id : arrasto?.id === id && arrasto.coluna === coluna;
    if (!combina) return;

    e.currentTarget.releasePointerCapture(e.pointerId);
    setArrasto(null);
    setParedes(SEM_PAREDES);
    // Os empurrões do arrasto assentam já; a separação estática (por pares) segue dali sem salto.
    assentarEmpurroes();

    if (resolvido) {
      // O par fica onde foi solto: o delta do arrasto vira repouso persistente da ÂNCORA (a outra
      // peça deriva sua posição dela, então acompanha automaticamente).
      const ancoraCol = ancoras[id];
      somarDeslocamento(`${prefixoColuna(ancoraCol)}-${id}`, posicaoArrasto);
      setPosicaoArrasto({ x: 0, y: 0 });
      iniciarSeparacaoEstatica();
      return;
    }

    const movimento = Math.hypot(e.clientX - pointerStartRef.current.x, e.clientY - pointerStartRef.current.y);
    if (movimento < 6) {
      setAlvoArrasto(null);
      setPosicaoArrasto({ x: 0, y: 0 });
      tratarCliquePeca(id, coluna);
      return;
    }

    const alvo = alvoArrasto;
    setAlvoArrasto(null);

    if (!alvo) {
      // Soltura livre: a peça FICA onde foi solta (o delta do arrasto vira repouso persistente); a
      // separação estática continua abrindo espaço nas vizinhas por mais um tempo.
      somarDeslocamento(`${prefixoColuna(coluna)}-${id}`, posicaoArrasto);
      setPosicaoArrasto({ x: 0, y: 0 });
      iniciarSeparacaoEstatica();
      return;
    }

    // Quem foi arrastado é o MOVER; quem estava parado (o alvo) é a âncora.
    const idEsq = coluna === 'esquerda' ? id : alvo.id;
    const idDir = coluna === 'direita' ? id : alvo.id;
    // Antes de resolver, o delta do arrasto do mover vira repouso (o par assenta onde encostou).
    somarDeslocamento(`${prefixoColuna(coluna)}-${id}`, posicaoArrasto);
    setPosicaoArrasto({ x: 0, y: 0 });
    finalizarConexao(idEsq, idDir, alvo.coluna);
    iniciarSeparacaoEstatica();
  }

  // ----- Conexão -----

  function finalizarConexao(idEsq: string, idDir: string, ancora: Coluna) {
    setAlvoArrasto(null);
    if (combinacaoCorreta(idEsq, idDir)) {
      const id = idEsq; // par correto: idEsq === idDir
      ancorasRef.current = { ...ancorasRef.current, [id]: ancora };
      setAncoras(ancorasRef.current);
      const novosPares = [...paresMatchedRef.current, id];
      paresMatchedRef.current = novosPares;
      setParesMatched(novosPares);
      tocarSomEncaixe();
      if (aoAcertarItem) aoAcertarItem(idEsq, idDir);

      if (novosPares.length === itensEsquerda.length) {
        setTimeout(() => aoConcluir(true, novosPares), 600);
      }
    } else {
      if (aoErrarItem) aoErrarItem(idEsq, idDir);
      const novasVidas = vidas - 1;
      setVidas(novasVidas);
      // A peça-mover (a que NÃO é a âncora) quebra; o alvo (âncora) sacode.
      const moverColuna: Coluna = ancora === 'esquerda' ? 'direita' : 'esquerda';
      const moverId = moverColuna === 'esquerda' ? idEsq : idDir;
      const ancoraId = ancora === 'esquerda' ? idEsq : idDir;
      setQuebrando({ id: moverId, coluna: moverColuna });
      setErroTemporario({ id: ancoraId, coluna: ancora });

      setTimeout(() => {
        setQuebrando(null);
        setErroTemporario(null);
        // A peça reaparece no repouso — pode ter voltado sobre alguém: relaxa a mesa de novo.
        iniciarSeparacaoEstatica();
      }, DURACAO_QUEBRA);

      if (novasVidas <= 0) {
        setTimeout(() => aoConcluir(false, paresMatched), DURACAO_QUEBRA + 100);
      }
    }
  }

  // deslizarEConectar (conexão por clique): as DUAS peças se movem, cada uma cobrindo metade da
  // distância relativa necessária para o par se encostar — encontram-se no meio, simétrico. Depois
  // de se moverem, a peça da direita vira a âncora convencional (ver finalizarConexao).
  function deslizarEConectar(id1: string, col1: Coluna, id2: string, col2: Coluna) {
    const idEsq = col1 === 'esquerda' ? id1 : id2;
    const idDir = col1 === 'direita' ? id1 : id2;

    const deslocDirAtual = deslocamentosRef.current[`dir-${idDir}`] || VETOR_ZERO;
    // Quanto a direita precisaria se mover, sozinha, para encostar na esquerda (que ficaria parada).
    const alvoDirSeEsqFicasse = posicaoMoverAdjacenteComAncora(idDir, 'direita', idEsq, 'esquerda');
    const deltaTotal = { x: alvoDirSeEsqFicasse.x - deslocDirAtual.x, y: alvoDirSeEsqFicasse.y - deslocDirAtual.y };
    // Metade para cada lado, em sentidos opostos: as duas se encontram no meio do caminho.
    const metadeEsq = { x: -deltaTotal.x / 2, y: -deltaTotal.y / 2 };
    const metadeDir = { x: deltaTotal.x / 2, y: deltaTotal.y / 2 };

    cancelarDeslizeRef.current?.();
    cancelarDeslizeRef.current = animarProgresso(
      DURACAO_DESLIZE,
      (suave) =>
        setDeslizandoPar({
          idEsq,
          idDir,
          offsetEsq: { x: metadeEsq.x * suave, y: metadeEsq.y * suave },
          offsetDir: { x: metadeDir.x * suave, y: metadeDir.y * suave },
        }),
      () => {
        setDeslizandoPar(null);
        cancelarDeslizeRef.current = null;
        somarDeslocamento(`esq-${idEsq}`, metadeEsq);
        somarDeslocamento(`dir-${idDir}`, metadeDir);
        finalizarConexao(idEsq, idDir, 'direita');
        iniciarSeparacaoEstatica();
      },
    );
  }

  function tratarCliquePeca(id: string, coluna: Coluna) {
    // O "tique" já tocou no pointer-down (iniciarArrasto); esta função só chega via soltura curta.
    if (paresMatched.includes(id) || quebrando || deslizandoPar) return;

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

    const outra = pecaSelecionada;
    setPecaSelecionada(null);
    deslizarEConectar(id, coluna, outra.id, outra.coluna);
  }

  function calcularSobreposicao(r1: DOMRect, r2: DOMRect): number {
    const xOverlap = Math.max(0, Math.min(r1.right, r2.right) - Math.max(r1.left, r2.left));
    const yOverlap = Math.max(0, Math.min(r1.bottom, r2.bottom) - Math.max(r1.top, r2.top));
    return xOverlap * yOverlap;
  }

  // ----- Render -----

  // transformDaPeca resolve o transform de uma peça no estado atual (repouso, arrasto, deslize,
  // empurrão ou encaixe de par).
  function transformDaPeca(id: string, coluna: Coluna): string {
    const chave = `${prefixoColuna(coluna)}-${id}`;
    const desloc = deslocamentos[chave] || VETOR_ZERO;

    if (paresMatched.includes(id)) {
      const ancora = ancoras[id];
      const arrastandoPar = arrasto?.id === id;
      if (ancora === coluna) {
        const x = desloc.x + (arrastandoPar ? posicaoArrasto.x : 0);
        const y = desloc.y + (arrastandoPar ? posicaoArrasto.y : 0);
        return x || y ? `translate(${x}px, ${y}px)` : 'none';
      }
      const pos = posicaoMoverAdjacente(id, coluna);
      if (arrastandoPar) {
        pos.x += posicaoArrasto.x;
        pos.y += posicaoArrasto.y;
      }
      return `translate(${pos.x}px, ${pos.y}px)`;
    }

    const arrastandoEsta = arrasto?.id === id && arrasto.coluna === coluna;
    const quebrandoEsta = quebrando?.id === id && quebrando.coluna === coluna;
    if (arrastandoEsta || quebrandoEsta) {
      return `translate(${desloc.x + posicaoArrasto.x}px, ${desloc.y + posicaoArrasto.y}px)`;
    }

    if (deslizandoPar) {
      if (coluna === 'esquerda' && deslizandoPar.idEsq === id) {
        return `translate(${desloc.x + deslizandoPar.offsetEsq.x}px, ${desloc.y + deslizandoPar.offsetEsq.y}px)`;
      }
      if (coluna === 'direita' && deslizandoPar.idDir === id) {
        return `translate(${desloc.x + deslizandoPar.offsetDir.x}px, ${desloc.y + deslizandoPar.offsetDir.y}px)`;
      }
    }

    const empurrao = empurroes[chave] || VETOR_ZERO;
    const x = desloc.x + empurrao.x;
    const y = desloc.y + empurrao.y;
    return x || y ? `translate(${x}px, ${y}px)` : 'none';
  }

  function renderizarPeca(item: ItemQuebraCabeca, coluna: Coluna) {
    const resolvido = paresMatched.includes(item.id);
    const arrastandoEsta = resolvido ? arrasto?.id === item.id : arrasto?.id === item.id && arrasto.coluna === coluna;
    const quebrandoEsta = quebrando?.id === item.id && quebrando.coluna === coluna;
    const deslizandoEsta =
      !!deslizandoPar &&
      ((coluna === 'esquerda' && deslizandoPar.idEsq === item.id) || (coluna === 'direita' && deslizandoPar.idDir === item.id));
    const erroEsta = erroTemporario?.id === item.id && erroTemporario.coluna === coluna;
    const selecionado = pecaSelecionada?.id === item.id && pecaSelecionada.coluna === coluna;
    const ehAlvo = alvoArrasto?.id === item.id && alvoArrasto.coluna === coluna;
    const ladoClasse = coluna === 'esquerda' ? 'puzzle-card-left' : 'puzzle-card-right';
    const formaProps = coluna === 'esquerda' ? { pinoDireita: true } : { buracoEsquerda: true };

    return (
      <div key={item.id} id={`wrapper-${prefixoColuna(coluna)}-${item.id}`} className="puzzle-card-wrapper">
        <div
          id={`card-${prefixoColuna(coluna)}-${item.id}`}
          className={`${ladoClasse} ${resolvido ? 'resolvido' : ''} ${arrastandoEsta ? 'arrastando' : ''} ${quebrandoEsta ? 'quebrando' : ''} ${deslizandoEsta ? 'deslizando' : ''} ${erroEsta ? 'repelido' : ''} ${selecionado ? 'selecionado' : ''} ${ehAlvo ? 'alvo-arrasto' : ''}`}
          style={{ transform: transformDaPeca(item.id, coluna) }}
          onPointerDown={(e) => iniciarArrasto(e, item.id, coluna)}
          onPointerMove={(e) => moverArrasto(e, item.id, coluna)}
          onPointerUp={(e) => pararArrasto(e, item.id, coluna)}
        >
          <FormaPeca {...formaProps} />
          <div className="puzzle-card-conteudo">{item.conteudo}</div>
          {quebrandoEsta && (
            <div className="puzzle-fragmentos">
              <div className="puzzle-fragmento metade-esquerda">
                <FormaPeca {...formaProps} />
                <div className="puzzle-card-conteudo">{item.conteudo}</div>
              </div>
              <div className="puzzle-fragmento metade-direita">
                <FormaPeca {...formaProps} />
                <div className="puzzle-card-conteudo">{item.conteudo}</div>
              </div>
            </div>
          )}
        </div>
      </div>
    );
  }

  return (
    <div className="revisao-puzzle-container">
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
          {instrucao || t('Arraste ou clique nas peças para ligar o Hanzi ao seu significado')}
        </div>
      </div>

      <div ref={arenaRef} className={`revisao-puzzle-tabuleiro revisao-puzzle-arena ${algumaParede(paredes) ? 'barreira-ativa' : ''}`}>
        <span className={`puzzle-parede cima ${paredes.cima ? 'ativa' : ''}`} aria-hidden="true" />
        <span className={`puzzle-parede baixo ${paredes.baixo ? 'ativa' : ''}`} aria-hidden="true" />
        <span className={`puzzle-parede esquerda ${paredes.esquerda ? 'ativa' : ''}`} aria-hidden="true" />
        <span className={`puzzle-parede direita ${paredes.direita ? 'ativa' : ''}`} aria-hidden="true" />

        {/* Coluna da Esquerda (Hanzi - Pino no lado direito) */}
        <div className="revisao-puzzle-coluna esquerda">
          {itensEsquerda.map((item) => renderizarPeca(item, 'esquerda'))}
        </div>

        {/* Coluna da Direita (Significado/Som - Buraco no lado esquerdo) */}
        <div className={`revisao-puzzle-coluna direita ${colunaDireitaCompacta ? 'compacta' : ''}`}>
          {itensDireita.map((item) => renderizarPeca(item, 'direita'))}
        </div>
      </div>
    </div>
  );
}
