// ----- Seção: Jornada de Revisão — Mapa -----
// Árvore navegável de ramos e níveis. O conteúdo (posições, títulos, revisões) vem de
// ObterArvoreJornada; o progresso, de ObterProgressoJornada (nivel_id -> revisões concluídas).
// TUDO o que a tela mostra além disso é DERIVADO aqui — nível concluído, ramo desbloqueado e nível
// atual não são persistidos (ver revisao/jornada e progresso/sqlite.go).
import { useEffect, useRef, useState } from 'react';
import './jornada.css';
import { t } from '../../i18n/i18n';
import { ICONES_JORNADA } from './iconesJornada';
import { ArvoreJornada, DIFICULDADES_JORNADA, NivelJornada, RamoJornada } from './tiposJornada';

interface MapaJornadaProps {
  arvore: ArvoreJornada;
  progresso: Record<string, number>;
  aoAbrirNivel: (nivelId: string) => void;
  aoVoltar: () => void;
}

// Circunferência do anel de progresso do nó (2πr, r = 38 no viewBox 84x84).
const CIRCUNFERENCIA_ANEL = 238.76;

// Raio de ancoragem das arestas: elas partem da BORDA do círculo, não do centro.
const RAIO_ANCORAGEM = 46;

// Arrasto acima deste deslocamento (px, soma dos eixos) conta como pan, não como clique no nó.
const LIMIAR_ARRASTO = 6;

type EstadoNivel = 'concluido' | 'atual' | 'bloqueado';

interface ArestaMapa {
  d: string;
  classe: string;
}

export function MapaJornada({ arvore, progresso, aoAbrirNivel, aoVoltar }: MapaJornadaProps) {
  const refRolagem = useRef<HTMLDivElement>(null);
  const arrastoRef = useRef<{ x: number; y: number; scrollLeft: number; scrollTop: number } | null>(null);
  const arrastouRef = useRef(false);
  const [arrastando, setArrastando] = useState(false);

  // --- Derivações de progresso ---
  const revisoesFeitas = (nivel: NivelJornada) => progresso[nivel.id] || 0;
  const nivelConcluido = (nivel: NivelJornada) => revisoesFeitas(nivel) >= nivel.revisoes.length;
  const ramoPorId = (ramoId: string) => arvore.ramos.find(r => r.id === ramoId);
  const ramoConcluido = (ramoId: string) => {
    const ramo = ramoPorId(ramoId);
    return !!ramo && ramo.niveis.every(nivelConcluido);
  };
  const ramoDesbloqueado = (ramo: RamoJornada) => !ramo.pai || ramoConcluido(ramo.pai);
  const estadoNivel = (ramo: RamoJornada, indice: number): EstadoNivel => {
    if (!ramoDesbloqueado(ramo)) return 'bloqueado';
    if (nivelConcluido(ramo.niveis[indice])) return 'concluido';
    if (indice === 0 || nivelConcluido(ramo.niveis[indice - 1])) return 'atual';
    return 'bloqueado';
  };

  // --- Centralização da câmera ---
  useEffect(() => {
    const elemento = refRolagem.current;
    if (!elemento) return;

    let ultimoConcluido: NivelJornada | undefined;

    for (const ramo of arvore.ramos) {
      for (const nivel of ramo.niveis) {
        if (nivelConcluido(nivel)) {
          ultimoConcluido = nivel;
        }
      }
    }

    const nivelAlvo = ultimoConcluido || arvore.ramos[0]?.niveis[0];
    if (!nivelAlvo || !nivelAlvo.pos || nivelAlvo.pos.length < 2) return;

    const centralizar = () => {
      if (!elemento) return;
      elemento.scrollLeft = nivelAlvo.pos[0] - elemento.clientWidth / 2;
      elemento.scrollTop = nivelAlvo.pos[1] - elemento.clientHeight / 2;
    };

    centralizar();
    const animFrameId = requestAnimationFrame(centralizar);
    return () => cancelAnimationFrame(animFrameId);
  }, [arvore, progresso]);

  // --- Navegação por arrasto (pan) ---
  function aoPressionar(e: React.PointerEvent<HTMLDivElement>) {
    // Clique num nó não inicia o arrasto: o pointer capture roubaria o clique do botão.
    if (e.target instanceof Element && e.target.closest('button')) return;
    const elemento = refRolagem.current;
    if (!elemento) return;

    arrastoRef.current = { x: e.clientX, y: e.clientY, scrollLeft: elemento.scrollLeft, scrollTop: elemento.scrollTop };
    arrastouRef.current = false;
    e.currentTarget.setPointerCapture(e.pointerId);
    setArrastando(true);
  }

  function aoMover(e: React.PointerEvent<HTMLDivElement>) {
    const arrasto = arrastoRef.current;
    const elemento = refRolagem.current;
    if (!arrasto || !elemento) return;

    const dx = e.clientX - arrasto.x;
    const dy = e.clientY - arrasto.y;
    if (Math.abs(dx) + Math.abs(dy) > LIMIAR_ARRASTO) arrastouRef.current = true;
    elemento.scrollLeft = arrasto.scrollLeft - dx;
    elemento.scrollTop = arrasto.scrollTop - dy;
  }

  function aoSoltar() {
    arrastoRef.current = null;
    setArrastando(false);
    // Zera no próximo tick: o clique do nó dispara depois do pointerup e precisa ver o arrasto.
    setTimeout(() => { arrastouRef.current = false; }, 0);
  }

  function clicarNoNivel(nivelId: string) {
    if (arrastouRef.current) return;
    aoAbrirNivel(nivelId);
  }

  // --- Geometria das arestas ---
  // Curva entre dois centros de nó, ancorada na borda dos círculos. Funciona em qualquer direção
  // (descendo, subindo ou na horizontal), por isso a curvatura sai da distância vertical ancorada.
  function aresta(p1: number[], p2: number[], classe: string): ArestaMapa {
    const dx = p2[0] - p1[0];
    const dy = p2[1] - p1[1];
    const comprimento = Math.hypot(dx, dy) || 1;
    const a1 = [p1[0] + (dx / comprimento) * RAIO_ANCORAGEM, p1[1] + (dy / comprimento) * RAIO_ANCORAGEM];
    const a2 = [p2[0] - (dx / comprimento) * RAIO_ANCORAGEM, p2[1] - (dy / comprimento) * RAIO_ANCORAGEM];
    const curvatura = (a2[1] - a1[1]) * 0.5;

    const d =
      `M ${a1[0].toFixed(1)} ${a1[1].toFixed(1)}` +
      ` C ${a1[0].toFixed(1)} ${(a1[1] + curvatura).toFixed(1)},` +
      ` ${a2[0].toFixed(1)} ${(a2[1] - curvatura).toFixed(1)},` +
      ` ${a2[0].toFixed(1)} ${a2[1].toFixed(1)}`;
    return { d, classe };
  }

  // --- Montagem do mapa ---
  const arestas: ArestaMapa[] = [];
  arvore.ramos.forEach(ramo => {
    const desbloqueado = ramoDesbloqueado(ramo);
    ramo.niveis.forEach((nivel, i) => {
      if (i === 0) return;
      const anteriorConcluido = nivelConcluido(ramo.niveis[i - 1]) && desbloqueado;
      const classe = nivelConcluido(nivel) ? 'concluida' : anteriorConcluido ? 'ativa' : 'bloqueada';
      arestas.push(aresta(ramo.niveis[i - 1].pos, nivel.pos, classe));
    });
  });
  // Conectores pai → filho (qualquer geração): do último nível do pai ao primeiro do filho.
  arvore.ramos.forEach(ramo => {
    if (!ramo.pai) return;
    const pai = ramoPorId(ramo.pai);
    if (!pai || pai.niveis.length === 0 || ramo.niveis.length === 0) return;
    const ultimoDoPai = pai.niveis[pai.niveis.length - 1];
    arestas.push(aresta(ultimoDoPai.pos, ramo.niveis[0].pos, ramoConcluido(ramo.pai) ? 'concluida' : 'bloqueada'));
  });

  return (
    <div className="jornada-mapa">
      <div className="jornada-barra">
        <button className="jornada-botao-voltar" onClick={aoVoltar}>
          <span className="jornada-icone-botao">{ICONES_JORNADA.ramo}</span>
          {t('Modos de revisão')}
        </button>
        <div className="jornada-titulo">
          <span className="jornada-icone-titulo">{ICONES_JORNADA.ramo}</span>
          {t('Jornada')}
        </div>
        <div className="jornada-legenda">
          <span><i className="jornada-ponto concluido" />{t('Concluído')}</span>
          <span><i className="jornada-ponto atual" />{t('Atual')}</span>
          <span><i className="jornada-ponto bloqueado" />{t('Bloqueado')}</span>
        </div>
      </div>

      <div className="jornada-rolagem" ref={refRolagem}>
        <div
          className={`jornada-tela${arrastando ? ' arrastando' : ''}`}
          style={{ width: arvore.mapa.largura, height: arvore.mapa.altura }}
          onPointerDown={aoPressionar}
          onPointerMove={aoMover}
          onPointerUp={aoSoltar}
          onPointerCancel={aoSoltar}
        >
          <svg
            className="jornada-arestas"
            width={arvore.mapa.largura}
            height={arvore.mapa.altura}
            viewBox={`0 0 ${arvore.mapa.largura} ${arvore.mapa.altura}`}
          >
            {arestas.map((a, i) => (
              <path key={i} className={`jornada-aresta ${a.classe}`} d={a.d} />
            ))}
          </svg>

          {arvore.ramos.map(ramo => (
            <div
              key={`rotulo-${ramo.id}`}
              className="jornada-rotulo-ramo"
              style={{ left: ramo.rotuloMapa[0], top: ramo.rotuloMapa[1] }}
            >
              <span className="jornada-chip-ramo">{t(ramo.nome)}</span>
              <span className="jornada-chip-dificuldade">{t(DIFICULDADES_JORNADA[ramo.dificuldade] || ramo.dificuldade)}</span>
            </div>
          ))}

          {arvore.ramos.map(ramo =>
            ramo.niveis.map((nivel, i) => {
              const estado = estadoNivel(ramo, i);
              const feitas = Math.min(revisoesFeitas(nivel), nivel.revisoes.length);
              const fracao = nivel.revisoes.length > 0 ? feitas / nivel.revisoes.length : 0;
              const bloqueado = estado === 'bloqueado';
              const dica = bloqueado
                ? ramoDesbloqueado(ramo)
                  ? t('Conclua o nível anterior para desbloquear')
                  : t('Conclua o ramo anterior para desbloquear este caminho')
                : `${t(nivel.titulo)} · ${t('{feitas}/{total} revisões', { feitas, total: nivel.revisoes.length })}`;

              return (
                <div key={nivel.id} className="jornada-no" style={{ left: nivel.pos[0], top: nivel.pos[1] }}>
                  <button
                    className={`jornada-no-botao ${estado}`}
                    title={dica}
                    disabled={bloqueado}
                    onClick={() => clicarNoNivel(nivel.id)}
                  >
                    <svg className="jornada-anel" viewBox="0 0 84 84">
                      <circle className="jornada-anel-trilho" cx="42" cy="42" r="38" />
                      <circle
                        className="jornada-anel-progresso"
                        cx="42"
                        cy="42"
                        r="38"
                        strokeDasharray={`${(CIRCUNFERENCIA_ANEL * fracao).toFixed(1)} ${CIRCUNFERENCIA_ANEL}`}
                        transform="rotate(-90 42 42)"
                      />
                    </svg>
                    <span className="jornada-no-disco">
                      {bloqueado ? ICONES_JORNADA.lock : ICONES_JORNADA[ramo.icone] || ICONES_JORNADA.ramo}
                    </span>
                    {estado === 'concluido' && (
                      <span className="jornada-selo-concluido">{ICONES_JORNADA.check}</span>
                    )}
                  </button>
                  <div className={`jornada-no-rotulo ${estado}`}>{t(nivel.titulo)}</div>
                </div>
              );
            })
          )}
        </div>
      </div>
    </div>
  );
}
