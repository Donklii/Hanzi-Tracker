// ----- Seção: Forma Geométrica das Peças do Quebra-Cabeça -----
// Gera o contorno SVG das peças em coordenadas de PIXEL, medidas no elemento real, em vez de um
// viewBox esticado (preserveAspectRatio="none"): o pino e o buraco ficam com o mesmo tamanho
// físico em qualquer largura de coluna, então o encaixe visual é exato e o bulbo não deforma.
// O pino projeta-se PROFUNDIDADE_PINO px além do corpo da peça; o buraco é escavado no corpo com
// a MESMA aba (os dois usam trechoAba), garantindo o casamento perfeito das bordas.
import { useLayoutEffect, useRef, useState } from 'react';

// PROFUNDIDADE_PINO é o avanço do pino além do corpo (e o recuo do buraco). Também é usado pelos
// jogos para calcular o deslocamento de encaixe e pelo CSS de conteúdo (padding do lado da aba).
export const PROFUNDIDADE_PINO = 16;

const RAIO_CANTO = 9;
const MEIA_ABERTURA_PESCOCO = 7;
const RAIO_BULBO = 11;
// Ângulo (a partir do eixo da aba) em que o bulbo encontra as curvas do pescoço: acima de 90°
// o bulbo "engorda" além da abertura, criando o estrangulamento típico de quebra-cabeça real.
const ANGULO_BULBO = (100 * Math.PI) / 180;

export interface LadosPeca {
  pinoDireita?: boolean;
  buracoEsquerda?: boolean;
}

const px = (valor: number) => Math.round(valor * 100) / 100;

// pontosAba calcula os pontos de contato do bulbo para uma aba em uma borda vertical (o bulbo
// sempre se projeta para +x: para fora do corpo no pino, para dentro do corpo no buraco).
function pontosAba(bordaX: number, centroY: number) {
  const centroBulboX = bordaX + (PROFUNDIDADE_PINO - RAIO_BULBO);
  const afastamentoX = RAIO_BULBO * Math.cos(ANGULO_BULBO);
  const alcanceY = RAIO_BULBO * Math.sin(ANGULO_BULBO);
  return {
    topo: { x: centroBulboX + afastamentoX, y: centroY - alcanceY },
    baixo: { x: centroBulboX + afastamentoX, y: centroY + alcanceY },
  };
}

// trechoAba desenha pescoço + bulbo ao percorrer uma borda vertical em `bordaX`. `descendo` indica
// o sentido do traçado (borda direita desce, borda esquerda sobe, no contorno horário da peça).
function trechoAba(bordaX: number, centroY: number, descendo: boolean): string {
  const { topo, baixo } = pontosAba(bordaX, centroY);
  const pescocoTopoY = centroY - MEIA_ABERTURA_PESCOCO;
  const pescocoBaixoY = centroY + MEIA_ABERTURA_PESCOCO;

  if (descendo) {
    return [
      `L ${px(bordaX)} ${px(pescocoTopoY)}`,
      `C ${px(bordaX + 0.5)} ${px(pescocoTopoY - 3)}, ${px(topo.x - 4)} ${px(topo.y + 3)}, ${px(topo.x)} ${px(topo.y)}`,
      `A ${RAIO_BULBO} ${RAIO_BULBO} 0 1 1 ${px(baixo.x)} ${px(baixo.y)}`,
      `C ${px(baixo.x - 4)} ${px(baixo.y - 3)}, ${px(bordaX + 0.5)} ${px(pescocoBaixoY + 3)}, ${px(bordaX)} ${px(pescocoBaixoY)}`,
    ].join(' ');
  }
  return [
    `L ${px(bordaX)} ${px(pescocoBaixoY)}`,
    `C ${px(bordaX + 0.5)} ${px(pescocoBaixoY + 3)}, ${px(baixo.x - 4)} ${px(baixo.y - 3)}, ${px(baixo.x)} ${px(baixo.y)}`,
    `A ${RAIO_BULBO} ${RAIO_BULBO} 0 1 0 ${px(topo.x)} ${px(topo.y)}`,
    `C ${px(topo.x - 4)} ${px(topo.y + 3)}, ${px(bordaX + 0.5)} ${px(pescocoTopoY - 3)}, ${px(bordaX)} ${px(pescocoTopoY)}`,
  ].join(' ');
}

// caminhoPeca monta o contorno completo (horário) de uma peça de `largura`×`altura` px com os
// lados pedidos: corpo de cantos arredondados, pino na borda direita e/ou buraco na esquerda.
export function caminhoPeca(largura: number, altura: number, lados: LadosPeca): string {
  const r = RAIO_CANTO;
  const centroY = altura / 2;
  const corpoDireita = lados.pinoDireita ? largura - PROFUNDIDADE_PINO : largura;

  const partes: string[] = [`M ${r} 0`, `L ${px(corpoDireita - r)} 0`, `A ${r} ${r} 0 0 1 ${px(corpoDireita)} ${r}`];

  if (lados.pinoDireita) {
    partes.push(trechoAba(corpoDireita, centroY, true));
  }

  partes.push(`L ${px(corpoDireita)} ${px(altura - r)}`);
  partes.push(`A ${r} ${r} 0 0 1 ${px(corpoDireita - r)} ${px(altura)}`);
  partes.push(`L ${r} ${px(altura)}`);
  partes.push(`A ${r} ${r} 0 0 1 0 ${px(altura - r)}`);

  if (lados.buracoEsquerda) {
    partes.push(trechoAba(0, centroY, false));
  }

  partes.push(`L 0 ${r}`);
  partes.push(`A ${r} ${r} 0 0 1 ${r} 0`);
  partes.push('Z');
  return partes.join(' ');
}

// FormaPeca é o fundo SVG de uma peça: mede o próprio tamanho renderizado (e acompanha resize)
// e desenha o contorno em px. Substitui os antigos paths fixos esticados por viewBox.
export function FormaPeca({ pinoDireita, buracoEsquerda }: LadosPeca) {
  const svgRef = useRef<SVGSVGElement>(null);
  const [dimensoes, setDimensoes] = useState({ largura: 0, altura: 0 });

  useLayoutEffect(() => {
    const el = svgRef.current;
    if (!el) return;

    const medir = () => {
      const rect = el.getBoundingClientRect();
      setDimensoes((atual) => {
        if (Math.abs(atual.largura - rect.width) < 0.5 && Math.abs(atual.altura - rect.height) < 0.5) {
          return atual;
        }
        return { largura: rect.width, altura: rect.height };
      });
    };

    medir();
    const observador = new ResizeObserver(medir);
    observador.observe(el);
    return () => observador.disconnect();
  }, []);

  return (
    <svg ref={svgRef} className="puzzle-svg-bg" aria-hidden="true">
      {dimensoes.largura > 0 && (
        <path d={caminhoPeca(dimensoes.largura, dimensoes.altura, { pinoDireita, buracoEsquerda })} />
      )}
    </svg>
  );
}
