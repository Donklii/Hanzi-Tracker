// ----- Seção: Revisão — Montagem de Hanzi (arrastar peças da decomposição) -----
// Atividade de nível iniciante com uma coreografia própria: o caractere aparece inteiro para
// memorização; ao prosseguir ele é FRATURADO nos quadrados de cada componente, que viram cartas;
// as cartas são viradas para baixo, reunidas num monte no centro do quadro, embaralhadas e então
// distribuídas na mesa (as peças reais MAIS alguns distratores parecidos) viradas para cima. O
// usuário arrasta a carta de cada componente até a moldura exata onde ele fica dentro do quadro do
// hanzi. Peças de MESMO caractere (os quatro 口 de 器) são intercambiáveis; distratores não têm
// moldura e são recusados em qualquer uma. As coordenadas do Hanzi Writer são absolutas no quadrado
// 1024, então cada moldura e cada peça encaixada é desenhada direto nos paths originais.
//
// As fases encadeiam por temporizadores; o transform/opacidade de cada carta é derivado da fase
// (função pura geometriaDaCarta), e a transição CSS anima sozinha entre um estado e o seguinte.
import { useEffect, useMemo, useRef, useState } from 'react';
import HanziWriter from 'hanzi-writer';
import { ObterDadosEscritaHanzi } from '../../wailsjs/go/main/App';
import { tocarSomTracoOk, tocarSomTracoErro, tocarSomClique } from '../comum/sons';
import { COR_TRACO_HANZI } from '../comum/CanvasDesenho';
import { t } from '../i18n/i18n';

// PADDING_QUADRADO acompanha o padding do canvas de desenho: os quadros têm de enquadrar o caractere
// igual, senão o hanzi "pularia" entre a memorização e a montagem.
const PADDING_QUADRADO = 12;

// FOLGA_CAIXA_PECA é a folga (em unidades do quadrado 1024) somada à caixa das medians de cada peça:
// as medians são a linha central dos traços, e sem folga a moldura cortaria a espessura deles.
const FOLGA_CAIXA_PECA = 60;

// Dimensões da carta na mesa (px) e o respiro entre elas na bandeja.
const CARTA_LARGURA = 58;
const CARTA_ALTURA = 74;
const CARTA_RESPIRO = 12;
const BANDEJA_MARGEM_TOPO = 26;

// Durações (ms) de cada trecho da coreografia, na ordem em que acontecem.
const MS_FRATURA = 900;
const MS_EMPILHAR = 700;
const MS_EMBARALHAR = 700;
const MS_DISTRIBUIR = 800;

interface PecaMontagem {
  caractere: string;
  tracos: number[];
}

interface CaixaGlifo {
  x: number;
  y: number;
  largura: number;
  altura: number;
}

interface CaixaPx {
  left: number;
  top: number;
  largura: number;
  altura: number;
}

interface FaceCarta {
  paths: string[];
  viewBox: string;
}

interface CartaMontagem {
  id: string;
  componente: string;
  ehReal: boolean;
  face: FaceCarta;
  // fraturaPx só existe na carta real: a caixa em px de onde ela se destaca do hanzi.
  fraturaPx: CaixaPx | null;
}

interface MolduraSlot {
  componente: string;
  tracos: number[];
  caixa: CaixaPx;
}

interface Geometria {
  left: number;
  top: number;
  largura: number;
  altura: number;
  giroY: number;
  opacidade: number;
}

interface MontagemHanziProps {
  hanzi: string;
  componentes: PecaMontagem[];
  distratores: string[];
  aoConcluir: (acertou: boolean, totalErros: number) => void;
  tamanho?: number;
}

type FaseMontagem =
  | 'carregando'
  | 'inteiro'
  | 'fraturando'
  | 'empilhando'
  | 'embaralhando'
  | 'distribuindo'
  | 'montando'
  | 'concluido'
  | 'erro';

export function MontagemHanzi({ hanzi, componentes, distratores, aoConcluir, tamanho = 280 }: MontagemHanziProps) {
  const quadroWriterRef = useRef<HTMLDivElement>(null);
  const writerRef = useRef<any>(null);
  const cartasRef = useRef<CartaMontagem[]>([]);
  const temporizadoresRef = useRef<number[]>([]);
  const arrastoInicioRef = useRef({ x: 0, y: 0 });
  const errosRef = useRef(0);

  const [fase, setFase] = useState<FaseMontagem>('carregando');
  const [dadosHanzi, setDadosHanzi] = useState<any>(null);
  const [cartas, setCartas] = useState<CartaMontagem[]>([]);
  const [slots, setSlots] = useState<MolduraSlot[]>([]);
  const [preenchidas, setPreenchidas] = useState<boolean[]>([]);
  const [colocadas, setColocadas] = useState<Record<string, boolean>>({});
  const [ordemBandeja, setOrdemBandeja] = useState<string[]>([]);
  const [arrasto, setArrasto] = useState<{ id: string; dx: number; dy: number } | null>(null);
  const [sacudirId, setSacudirId] = useState<string | null>(null);
  const [slotSobrevoado, setSlotSobrevoado] = useState(-1);

  // aoConcluir vive num ref: o pai troca o callback a cada questão respondida (setState) e o closure
  // do fim da montagem chamaria uma versão obsoleta.
  const aoConcluirRef = useRef(aoConcluir);
  useEffect(() => { aoConcluirRef.current = aoConcluir; }, [aoConcluir]);

  // A dependência do efeito é a CHAVE da questão, não os arrays: o pai remonta os arrays a cada
  // render e a identidade nova reinicializaria a atividade em laço.
  const chaveQuestao = hanzi + '|' + componentes.map(p => p.caractere + ':' + p.tracos.join('.')).join(',') + '|' + distratores.join(',');

  useEffect(() => {
    if (!quadroWriterRef.current || !hanzi) return;
    if (componentes.length < 2) {
      setFase('erro');
      return;
    }

    let cancelado = false;
    reiniciarEstado();

    ObterDadosEscritaHanzi(hanzi)
      .then(async (json) => {
        if (cancelado || !quadroWriterRef.current) return;

        const dados = JSON.parse(json);
        if (!pecasCabemNosDados(componentes, dados)) {
          setFase('erro');
          return;
        }

        const dadosDistratores = await Promise.all(
          distratores.map(caractere =>
            ObterDadosEscritaHanzi(caractere)
              .then(j => ({ caractere, dados: JSON.parse(j) }))
              .catch(() => null)
          )
        );
        if (cancelado || !quadroWriterRef.current) return;

        const molduras = componentes.map(peca => ({
          componente: peca.caractere,
          tracos: peca.tracos,
          caixa: caixaEmPx(caixaDoRecorte(dados.medians, peca.tracos), tamanho),
        }));
        const cartasReais: CartaMontagem[] = componentes.map((peca, indice) => ({
          id: 'r' + indice,
          componente: peca.caractere,
          ehReal: true,
          face: faceDoRecorte(dados, peca.tracos),
          fraturaPx: molduras[indice].caixa,
        }));
        const cartasDistratoras: CartaMontagem[] = [];
        dadosDistratores.forEach((item, indice) => {
          if (!item || !item.dados.strokes || !item.dados.medians) return;
          const indices = item.dados.strokes.map((_: unknown, i: number) => i);
          cartasDistratoras.push({
            id: 'd' + indice,
            componente: item.caractere,
            ehReal: false,
            face: faceDoRecorte(item.dados, indices),
            fraturaPx: null,
          });
        });

        const todas = [...cartasReais, ...cartasDistratoras];
        cartasRef.current = todas;
        setDadosHanzi(dados);
        setSlots(molduras);
        setCartas(todas);
        setPreenchidas(componentes.map(() => false));

        quadroWriterRef.current.innerHTML = '';
        writerRef.current = HanziWriter.create(quadroWriterRef.current, hanzi, {
          width: tamanho,
          height: tamanho,
          padding: PADDING_QUADRADO,
          showCharacter: true,
          showOutline: false,
          strokeColor: COR_TRACO_HANZI,
          charDataLoader: (_char: string, onComplete: (d: any) => void) => onComplete(dados),
        });
        setFase('inteiro');
      })
      .catch(() => {
        if (!cancelado) setFase('erro');
      });

    return () => {
      cancelado = true;
      limparTemporizadores();
      writerRef.current = null;
      if (quadroWriterRef.current) quadroWriterRef.current.innerHTML = '';
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [chaveQuestao, tamanho]);

  function reiniciarEstado() {
    limparTemporizadores();
    errosRef.current = 0;
    setFase('carregando');
    setDadosHanzi(null);
    setCartas([]);
    setSlots([]);
    setPreenchidas([]);
    setColocadas({});
    setOrdemBandeja([]);
    setArrasto(null);
    setSacudirId(null);
    setSlotSobrevoado(-1);
  }

  function limparTemporizadores() {
    temporizadoresRef.current.forEach(id => window.clearTimeout(id));
    temporizadoresRef.current = [];
  }

  function agendar(atrasoMs: number, acao: () => void) {
    temporizadoresRef.current.push(window.setTimeout(acao, atrasoMs));
  }

  // ----- Coreografia -----

  // prosseguir dispara a sequência fratura → monte → embaralho → distribuição. O writer é apagado já
  // no início: as cartas de fratura nascem exatamente sobre as peças, então a troca é imperceptível.
  const prosseguir = () => {
    if (fase !== 'inteiro') return;
    writerRef.current = null;
    if (quadroWriterRef.current) quadroWriterRef.current.innerHTML = '';
    tocarSomClique();

    setFase('fraturando');
    agendar(MS_FRATURA, () => setFase('empilhando'));
    agendar(MS_FRATURA + MS_EMPILHAR, () => setFase('embaralhando'));
    agendar(MS_FRATURA + MS_EMPILHAR + MS_EMBARALHAR, () => {
      setOrdemBandeja(embaralhar(cartasRef.current.map(c => c.id)));
      setFase('distribuindo');
    });
    agendar(MS_FRATURA + MS_EMPILHAR + MS_EMBARALHAR + MS_DISTRIBUIR, () => setFase('montando'));
  };

  // ----- Arrasto -----

  const iniciarArrasto = (e: React.PointerEvent<HTMLDivElement>, carta: CartaMontagem) => {
    if (fase !== 'montando' || colocadas[carta.id]) return;
    e.currentTarget.setPointerCapture(e.pointerId);
    arrastoInicioRef.current = { x: e.clientX, y: e.clientY };
    setArrasto({ id: carta.id, dx: 0, dy: 0 });
    tocarSomClique();
  };

  const moverArrasto = (e: React.PointerEvent<HTMLDivElement>, carta: CartaMontagem) => {
    if (!arrasto || arrasto.id !== carta.id) return;
    const dx = e.clientX - arrastoInicioRef.current.x;
    const dy = e.clientY - arrastoInicioRef.current.y;
    setArrasto({ id: carta.id, dx, dy });
    setSlotSobrevoado(slotSob(carta, dx, dy));
  };

  const soltarArrasto = (e: React.PointerEvent<HTMLDivElement>, carta: CartaMontagem) => {
    if (!arrasto || arrasto.id !== carta.id) return;
    e.currentTarget.releasePointerCapture(e.pointerId);
    const dx = e.clientX - arrastoInicioRef.current.x;
    const dy = e.clientY - arrastoInicioRef.current.y;
    setArrasto(null);
    setSlotSobrevoado(-1);
    resolverSoltura(carta, dx, dy);
  };

  function resolverSoltura(carta: CartaMontagem, dx: number, dy: number) {
    const alvo = slotSob(carta, dx, dy);
    // Solta no vazio: a carta volta para a bandeja (a geometria da fase a traz de volta sozinha).
    if (alvo === -1) return;

    if (!carta.ehReal || slots[alvo].componente !== carta.componente) {
      errosRef.current += 1;
      tocarSomTracoErro();
      setSacudirId(carta.id);
      agendar(400, () => setSacudirId(null));
      return;
    }

    tocarSomTracoOk();
    const novas = preenchidas.slice();
    novas[alvo] = true;
    setPreenchidas(novas);
    setColocadas(anteriores => ({ ...anteriores, [carta.id]: true }));

    if (novas.every(Boolean)) {
      setFase('concluido');
      const totalErros = errosRef.current;
      agendar(150, () => aoConcluirRef.current(totalErros <= Math.ceil(slots.length * 0.5), totalErros));
    }
  }

  // slotSob devolve o índice da moldura vazia sob o centro da carta arrastada (a menor, quando há
  // molduras aninhadas — a mais específica), ou -1. As caixas das molduras estão em px do quadro, a
  // mesma origem das posições da bandeja, então basta comparar o ponto.
  function slotSob(carta: CartaMontagem, dx: number, dy: number): number {
    const base = posicaoBandeja(carta.id);
    const cx = base.left + dx + CARTA_LARGURA / 2;
    const cy = base.top + dy + CARTA_ALTURA / 2;

    let escolhido = -1;
    let menorArea = Infinity;
    for (let j = 0; j < slots.length; j++) {
      if (preenchidas[j]) continue;
      const c = slots[j].caixa;
      if (cx < c.left || cx > c.left + c.largura || cy < c.top || cy > c.top + c.altura) continue;
      const area = c.largura * c.altura;
      if (area < menorArea) {
        menorArea = area;
        escolhido = j;
      }
    }
    return escolhido;
  }

  // ----- Geometria por fase -----

  const layoutBandeja = useMemo(() => {
    const total = Math.max(cartas.length, 1);
    const colunas = Math.max(1, Math.min(total, Math.floor((tamanho + CARTA_RESPIRO) / (CARTA_LARGURA + CARTA_RESPIRO))));
    return { colunas, linhas: Math.ceil(total / colunas) };
  }, [cartas.length, tamanho]);

  const pilha = { left: tamanho / 2 - CARTA_LARGURA / 2, top: tamanho / 2 - CARTA_ALTURA / 2 };
  const alturaPalco = tamanho + BANDEJA_MARGEM_TOPO + layoutBandeja.linhas * (CARTA_ALTURA + CARTA_RESPIRO);

  // posicaoBandeja devolve o canto (px) da carta na mesa distribuída, centrando cada linha.
  function posicaoBandeja(idCarta: string): { left: number; top: number } {
    const k = ordemBandeja.indexOf(idCarta);
    const indice = k >= 0 ? k : 0;
    const { colunas } = layoutBandeja;
    const linha = Math.floor(indice / colunas);
    const coluna = indice % colunas;
    const nestaLinha = Math.min(colunas, cartas.length - linha * colunas);
    const larguraLinha = nestaLinha * CARTA_LARGURA + (nestaLinha - 1) * CARTA_RESPIRO;
    const inicioX = (tamanho - larguraLinha) / 2;
    return {
      left: inicioX + coluna * (CARTA_LARGURA + CARTA_RESPIRO),
      top: tamanho + BANDEJA_MARGEM_TOPO + linha * (CARTA_ALTURA + CARTA_RESPIRO),
    };
  }

  function geometriaDaCarta(carta: CartaMontagem): Geometria {
    const naPilha = (giroY: number, opacidade: number): Geometria => {
      const desvio = Math.max(0, cartas.findIndex(c => c.id === carta.id));
      return {
        left: pilha.left + desvio * 0.8,
        top: pilha.top - desvio * 0.8,
        largura: CARTA_LARGURA,
        altura: CARTA_ALTURA,
        giroY,
        opacidade,
      };
    };

    if (fase === 'carregando' || fase === 'inteiro') {
      if (carta.ehReal && carta.fraturaPx) {
        return { ...caixaParaGeometria(carta.fraturaPx), giroY: 0, opacidade: 0 };
      }
      return naPilha(180, 0);
    }

    if (fase === 'fraturando') {
      if (carta.ehReal && carta.fraturaPx) {
        return { ...caixaParaGeometria(carta.fraturaPx), giroY: 0, opacidade: 1 };
      }
      return naPilha(180, 0);
    }

    if (fase === 'empilhando' || fase === 'embaralhando') {
      return naPilha(180, 1);
    }

    // distribuindo / montando / concluido: cada carta no seu lugar na bandeja, virada para cima.
    const pos = posicaoBandeja(carta.id);
    return { left: pos.left, top: pos.top, largura: CARTA_LARGURA, altura: CARTA_ALTURA, giroY: 0, opacidade: 1 };
  }

  // ----- Render -----

  if (fase === 'erro') {
    return (
      <div style={{ color: 'var(--cor-texto-suave)', textAlign: 'center', padding: '20px' }}>
        {t('Não há dados de traçado para "{hanzi}".', { hanzi })}
      </div>
    );
  }

  const mostrarMolduras = fase === 'montando' || fase === 'concluido';
  const pecasColocadas = preenchidas.filter(Boolean).length;

  return (
    <div style={{ display: 'flex', flexDirection: 'column', alignItems: 'center', gap: '10px' }}>
      <div className="montagem-palco" style={{ width: tamanho, height: alturaPalco }}>
        <div
          className="montagem-quadro"
          style={{ width: tamanho, height: tamanho }}
        >
          <div ref={quadroWriterRef} style={{ position: 'absolute', top: 0, left: 0, pointerEvents: 'none' }} />
          {mostrarMolduras && dadosHanzi && (
            <svg className="montagem-quadro-svg" width={tamanho} height={tamanho}>
              <g transform={transformacaoDoQuadrado(tamanho)}>
                {slots.map((slot, indice) => preenchidas[indice] && slot.tracos.map(iTraco => (
                  <path key={indice + '-' + iTraco} d={dadosHanzi.strokes[iTraco]} fill={COR_TRACO_HANZI} />
                )))}
              </g>
              {fase === 'montando' && slots.map((slot, indice) => !preenchidas[indice] && (
                <rect
                  key={indice}
                  className={'montagem-moldura' + (slotSobrevoado === indice ? ' sobrevoada' : '')}
                  x={slot.caixa.left}
                  y={slot.caixa.top}
                  width={slot.caixa.largura}
                  height={slot.caixa.altura}
                  rx={8}
                />
              ))}
            </svg>
          )}
        </div>

        {cartas.map(carta => {
          if (colocadas[carta.id]) return null;
          const g = geometriaDaCarta(carta);
          const arrastando = arrasto?.id === carta.id;
          const dx = arrastando ? arrasto!.dx : 0;
          const dy = arrastando ? arrasto!.dy : 0;
          const classe =
            'montagem-carta' +
            (fase === 'montando' ? ' ativa' : '') +
            (arrastando ? ' arrastando' : '') +
            (fase === 'embaralhando' ? ' embaralhando' : '') +
            (sacudirId === carta.id ? ' sacudir' : '');
          return (
            <div
              key={carta.id}
              className={classe}
              style={{
                left: g.left,
                top: g.top,
                width: g.largura,
                height: g.altura,
                opacity: g.opacidade,
                transform: `translate(${dx}px, ${dy}px) rotateY(${g.giroY}deg)`,
                zIndex: arrastando ? 50 : carta.ehReal ? 10 : 8,
              }}
              onPointerDown={(e) => iniciarArrasto(e, carta)}
              onPointerMove={(e) => moverArrasto(e, carta)}
              onPointerUp={(e) => soltarArrasto(e, carta)}
            >
              <div className="montagem-carta-face montagem-carta-frente">
                <svg className="montagem-carta-svg" viewBox={carta.face.viewBox} preserveAspectRatio="xMidYMid meet">
                  <g transform="scale(1, -1)">
                    {carta.face.paths.map((d, i) => <path key={i} d={d} fill="currentColor" />)}
                  </g>
                </svg>
              </div>
              <div className="montagem-carta-face montagem-carta-verso" />
            </div>
          );
        })}
      </div>

      {fase === 'inteiro' && (
        <button className="scan-btn" onClick={prosseguir}>
          {t('Prosseguir')}
        </button>
      )}

      {(fase === 'montando' || fase === 'concluido') && (
        <div style={{ fontSize: '12px', color: 'var(--cor-texto-suave)', marginTop: '4px' }}>
          {t('Peças: {atual} / {total}', { atual: pecasColocadas, total: slots.length })}
        </div>
      )}
    </div>
  );
}


// ----- Seção: Geometria dos Recortes (caixas, faces e transformação do quadro) -----

// transformacaoDoQuadrado leva das coordenadas do Hanzi Writer para os pixels do quadro. Os traçados
// vêm em um quadrado 1024 com o eixo y para cima, com o topo em y=900 (Positioner oficial do Hanzi Writer).
function transformacaoDoQuadrado(tamanho: number): string {
  const escala = (tamanho - PADDING_QUADRADO * 2) / 1024;
  const yOffset = PADDING_QUADRADO + 900 * escala;
  return `translate(${PADDING_QUADRADO}, ${yOffset}) scale(${escala}, ${-escala})`;
}


// caixaDoRecorte é o retângulo (em coordenadas do quadrado 1024) que envolve as medians dos traços
// pedidos, com a folga que cobre a espessura do traço.
function caixaDoRecorte(medians: any[], indices: number[]): CaixaGlifo {
  let minX = Infinity, minY = Infinity, maxX = -Infinity, maxY = -Infinity;
  for (const iTraco of indices) {
    for (const [x, y] of medians[iTraco]) {
      if (x < minX) minX = x;
      if (x > maxX) maxX = x;
      if (y < minY) minY = y;
      if (y > maxY) maxY = y;
    }
  }
  return {
    x: minX - FOLGA_CAIXA_PECA,
    y: minY - FOLGA_CAIXA_PECA,
    largura: (maxX - minX) + FOLGA_CAIXA_PECA * 2,
    altura: (maxY - minY) + FOLGA_CAIXA_PECA * 2,
  };
}


// caixaEmPx converte a caixa em coordenadas do quadrado 1024 (eixo y para cima) para px do quadro,
// espelhando o y como faz transformacaoDoQuadrado — o topo em px corresponde ao MAIOR y do glifo.
function caixaEmPx(caixa: CaixaGlifo, tamanho: number): CaixaPx {
  const escala = (tamanho - PADDING_QUADRADO * 2) / 1024;
  const yOffset = PADDING_QUADRADO + 900 * escala;
  return {
    left: PADDING_QUADRADO + escala * caixa.x,
    top: yOffset - escala * (caixa.y + caixa.altura),
    largura: escala * caixa.largura,
    altura: escala * caixa.altura,
  };
}


// faceDoRecorte monta a face da carta: os paths dos traços pedidos e o viewBox na caixa deles. Os
// traços vêm com y para cima, então a face é desenhada dentro de um grupo scale(1,-1) (ver o render)
// e o viewBox usa o y JÁ espelhado (-(y+altura)), para o recorte ficar reto e no tamanho da carta.
function faceDoRecorte(dados: any, indices: number[]): FaceCarta {
  const caixa = caixaDoRecorte(dados.medians, indices);
  return {
    paths: indices.map(i => dados.strokes[i]),
    viewBox: `${caixa.x} ${-(caixa.y + caixa.altura)} ${caixa.largura} ${caixa.altura}`,
  };
}


// caixaParaGeometria adapta a caixa em px do quadro para a geometria da carta (mesmos campos).
function caixaParaGeometria(caixa: CaixaPx): Omit<Geometria, 'giroY' | 'opacidade'> {
  return { left: caixa.left, top: caixa.top, largura: caixa.largura, altura: caixa.altura };
}


// ----- Seção: Utilitários -----

// pecasCabemNosDados confere o contrato com o backend: todo índice de peça existe nos traçados e há
// medians (o desenho da face depende delas) — dados de arquivos diferentes, o frontend não confia às
// cegas.
function pecasCabemNosDados(componentes: PecaMontagem[], dados: any): boolean {
  const totalTracos = (dados.strokes || []).length;
  if (totalTracos === 0 || !dados.medians || dados.medians.length !== totalTracos) return false;
  for (const peca of componentes) {
    for (const indice of peca.tracos) {
      if (!Number.isInteger(indice) || indice < 0 || indice >= totalTracos) return false;
    }
  }
  return true;
}


// embaralhar devolve uma cópia da lista em ordem aleatória (Fisher-Yates).
function embaralhar(lista: string[]): string[] {
  const copia = [...lista];
  for (let i = copia.length - 1; i > 0; i--) {
    const j = Math.floor(Math.random() * (i + 1));
    [copia[i], copia[j]] = [copia[j], copia[i]];
  }
  return copia;
}
