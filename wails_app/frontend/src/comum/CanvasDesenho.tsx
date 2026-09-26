// ----- Seção: Revisão — Canvas de Desenho (motor Hanzi Writer) -----
// Wrapper React do Hanzi Writer para os modos de desenho. Os traçados NÃO vêm de CDN: o
// charDataLoader busca o JSON no backend Go (ObterDadosEscritaHanzi), que serve o banco embarcado
// hanzi-writer-data — o app funciona 100% offline.
//
// Duas formas de uso (prop modoMemoria):
//   false — quiz direto: canvas vazio, o usuário desenha o hanzi traço a traço.
//   true  — desenho de memória: o caractere é exibido; ao clicar em "Prosseguir" ele sofre um
//           fadeout e o usuário o redesenha de memória.
//
// Com a prop tracosAlvo o desenho é PARCIAL (revisão de componente): o canvas vira duas camadas
// sobrepostas, uma com os traços do componente cobrado e outra com o resto do caractere. Só a
// camada do componente some no fadeout e é cobrada no quiz — o resto fica na tela como guia. As
// coordenadas do Hanzi Writer são absolutas no quadrado do caractere, então recortar traços
// preserva posição e proporção de cada um.
import { useEffect, useRef, useState } from 'react';
import HanziWriter from 'hanzi-writer';
import { ObterDadosEscritaHanzi } from '../../wailsjs/go/main/App';
import { tocarSomTracoOk, tocarSomTracoErro } from './sons';
import { EsconderComFadeEhContinuar, DURACAO_FADEOUT_PADRAO_MS } from './EsconderComFade';
import { t } from '../i18n/i18n';

// SVGs Profissionais para a UI
const IconRefresh = () => (
  <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5" strokeLinecap="round" strokeLinejoin="round" style={{ width: '18px', height: '18px' }}>
    <path d="M3 12a9 9 0 1 0 9-9 9.75 9.75 0 0 0-6.74 2.74L3 8" />
    <path d="M3 3v5h5" />
  </svg>
);

const IconEye = () => (
  <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" style={{ width: '20px', height: '20px' }}>
    <path d="M1 12s4-8 11-8 11 8 11 8-4 8-11 8-11-8-11-8z" />
    <circle cx="12" cy="12" r="3" />
  </svg>
);

const IconEyeOff = () => (
  <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" style={{ width: '20px', height: '20px' }}>
    <path d="M17.94 17.94A10.07 10.07 0 0 1 12 20c-7 0-11-8-11-8a18.45 18.45 0 0 1 5.06-5.94M9.9 4.24A9.12 9.12 0 0 1 12 4c7 0 11 8 11 8a18.5 18.5 0 0 1-2.16 3.19m-6.72-1.07a3 3 0 1 1-4.24-4.24" />
    <line x1="1" y1="1" x2="23" y2="23" />
  </svg>
);

const IconDica = () => (
  <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" style={{ width: '16px', height: '16px' }}>
    <path d="M15 14c.2-1 .7-1.7 1.5-2.5 1-.9 1.5-2.2 1.5-3.5A6 6 0 0 0 6 8c0 1 .2 2.2 1.5 3.5.7.9 1.2 1.5 1.5 2.5" />
    <path d="M9 18h6" />
    <path d="M10 22h4" />
  </svg>
);

const IconPular = () => (
  <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" style={{ width: '16px', height: '16px' }}>
    <polygon points="5 4 15 12 5 20 5 4" />
    <line x1="19" y1="5" x2="19" y2="19" />
  </svg>
);

// COR_TRACO_HANZI é a cor do caractere exibido — a que some no fadeout. Vale para as duas camadas
// do desenho parcial (no início elas têm de formar um hanzi de cor uniforme) e para as peças da
// montagem (revisao/MontagemHanzi.tsx), que recompõem o mesmo caractere.
export const COR_TRACO_HANZI = '#e0e0e0';

// DELAY_MEMORIZACAO_MS é a pausa antes do fadeout automático, para dar tempo de olhar o caractere.
const DELAY_MEMORIZACAO_MS = 1000;

interface CanvasDesenhoProps {
  hanzi: string;
  modoMemoria: boolean;
  aoConcluir: (acertou: boolean, totalErros: number) => void;
  tamanho?: number;
  fadeoutAutomatico?: boolean;
  mostrarDicaAposErros?: number;
  apenasTreino?: boolean;
  tracosAlvo?: number[];
  modoGuiado?: boolean;
}

// 'sumindo' é o intervalo do fadeout, entre o "Prosseguir" e a abertura do quiz: sem ela o botão
// continuaria na tela durante o sumiço e um segundo clique reiniciaria a animação.
type FaseCanvas = 'carregando' | 'memorizando' | 'sumindo' | 'desenhando' | 'concluido' | 'erro';

export function CanvasDesenho({
  hanzi,
  modoMemoria,
  aoConcluir,
  tamanho = 280,
  fadeoutAutomatico = false,
  mostrarDicaAposErros = 3,
  apenasTreino = false,
  tracosAlvo,
  modoGuiado = false
}: CanvasDesenhoProps) {
  const containerRef = useRef<HTMLDivElement>(null);
  const guiaRef = useRef<HTMLDivElement>(null);
  const writerRef = useRef<any>(null);
  const [fase, setFase] = useState<FaseCanvas>('carregando');
  const [tracosTotais, setTracosTotais] = useState(0);
  const [tracosFeitos, setTracosFeitos] = useState(0);
  const [dicasUsadas, setDicasUsadas] = useState(0);
  const [outlineVisivel, setOutlineVisivel] = useState(false);
  const [errouTracoAtual, setErrouTracoAtual] = useState(false);
  const [dadosHanzi, setDadosHanzi] = useState<any>(null);

  const errosCustomizadosRef = useRef(0);
  const errosNoTracoAtualRef = useRef(0);
  const timeoutMemorizacaoRef = useRef<number | null>(null);
  // cancelarFadeRef guarda a função de cancelamento devolvida por EsconderComFadeEhContinuar.
  const cancelarFadeRef = useRef<(() => void) | null>(null);

  // aoConcluir vive num ref: o closure do quiz é criado uma vez, mas o pai troca o callback a cada
  // questão respondida (setState) — sem o ref, o quiz chamaria uma version obsoleta.
  const aoConcluirRef = useRef(aoConcluir);
  useEffect(() => { aoConcluirRef.current = aoConcluir; }, [aoConcluir]);

  // A dependência do efeito é a CHAVE dos traços, não o array: o pai remonta o array a cada render e
  // a identidade nova reinicializaria o canvas em laço.
  const chaveTracosAlvo = (tracosAlvo ?? []).join(',');

  useEffect(() => {
    if (!containerRef.current || !hanzi) return;

    let cancelado = false;
    setFase('carregando');
    setTracosFeitos(0);
    setDicasUsadas(0);
    setOutlineVisivel(false);
    setErrouTracoAtual(false);
    setDadosHanzi(null);

    ObterDadosEscritaHanzi(hanzi)
      .then(json => {
        if (cancelado || !containerRef.current || !guiaRef.current) return;

        const dados = JSON.parse(json);
        const totalNoCaractere: number = (dados.strokes || []).length;
        const doComponente = tracosDoComponente(chaveTracosAlvo, dados, totalNoCaractere);

        const dadosDoQuiz = doComponente.length > 0 ? fatiarTracados(dados, doComponente) : dados;
        const numTracos: number = doComponente.length > 0 ? doComponente.length : totalNoCaractere;
        setTracosTotais(numTracos);
        setDadosHanzi(dadosDoQuiz);

        // Camada de guia: o que NÃO é cobrado. Fica vazia no desenho do caractere inteiro.
        guiaRef.current.innerHTML = '';
        if (doComponente.length > 0) {
          const dadosDaGuia = fatiarTracados(dados, complementoDeTracos(doComponente, totalNoCaractere));
          HanziWriter.create(guiaRef.current, hanzi, {
            width: tamanho,
            height: tamanho,
            padding: 12,
            showCharacter: true,
            showOutline: false,
            strokeColor: COR_TRACO_HANZI,
            charDataLoader: (_char: string, onComplete: (d: any) => void) => onComplete(dadosDaGuia),
          });
        }

        containerRef.current.innerHTML = ''; // remove o writer da questão anterior
        const writer = HanziWriter.create(containerRef.current, hanzi, {
          width: tamanho,
          height: tamanho,
          padding: 12,
          showCharacter: modoMemoria,
          showOutline: false,
          strokeColor: COR_TRACO_HANZI, // Hanzi inicial totalmente opaco (cor do texto, some no fadeOut)
          outlineColor: 'rgba(255, 255, 255, 0.2)', // Guia (Outline) semi-transparente quando a dica é ativada
          drawingColor: '#ffffff', // Os traços do usuário são brancos opacos
          highlightColor: '#2196f3',
          drawingWidth: 18,
          charDataLoader: (_char: string, onComplete: (d: any) => void) => onComplete(dadosDoQuiz),
        });
        writerRef.current = writer;

        if (modoMemoria) {
          setFase('memorizando');
          if (fadeoutAutomatico) {
            timeoutMemorizacaoRef.current = window.setTimeout(() => {
              timeoutMemorizacaoRef.current = null;
              if (cancelado || !writerRef.current) return;
              esconderComFadeEhIniciarQuiz(writerRef.current, numTracos);
            }, DELAY_MEMORIZACAO_MS);
          }
        } else {
          iniciarQuiz(writer, numTracos);
        }
      })
      .catch(() => {
        if (!cancelado) setFase('erro');
      });

    return () => {
      cancelado = true;
      if (timeoutMemorizacaoRef.current !== null) {
        window.clearTimeout(timeoutMemorizacaoRef.current);
        timeoutMemorizacaoRef.current = null;
      }
      if (cancelarFadeRef.current) {
        cancelarFadeRef.current();
        cancelarFadeRef.current = null;
      }
      if (writerRef.current) {
        writerRef.current.cancelQuiz();
        writerRef.current = null;
      }
      if (guiaRef.current) guiaRef.current.innerHTML = '';
    };
  }, [hanzi, modoMemoria, fadeoutAutomatico, tamanho, chaveTracosAlvo]);

  // esconderComFadeEhIniciarQuiz apaga o que vai ser cobrado e só DEPOIS abre o quiz — ver
  // EsconderComFadeEhContinuar (comum/EsconderComFade.ts) para o motivo da ordem ser obrigatória.
  const esconderComFadeEhIniciarQuiz = (writer: any, numTracos: number) => {
    setFase('sumindo');
    cancelarFadeRef.current = EsconderComFadeEhContinuar(writer, () => {
      cancelarFadeRef.current = null;
      if (!writerRef.current) return;
      iniciarQuiz(writerRef.current, numTracos);
    }, DURACAO_FADEOUT_PADRAO_MS);
  };

  const iniciarQuiz = (writer: any, numTracos: number) => {
    setFase('desenhando');
    errosCustomizadosRef.current = 0;
    errosNoTracoAtualRef.current = 0;
    setErrouTracoAtual(false);

    writer.quiz({
      leniency: 1.3,          // desenho "aproximado": mais tolerante que o padrão
      showHintAfterMisses: modoGuiado ? 999 : mostrarDicaAposErros,
      onCorrectStroke: (dados: any) => {
        errosNoTracoAtualRef.current = 0;
        setErrouTracoAtual(false);
        tocarSomTracoOk();
        setTracosFeitos(dados.strokesRemaining >= 0 ? numTracos - dados.strokesRemaining : 0);
      },
      onMistake: () => {
        errosNoTracoAtualRef.current += 1;
        setErrouTracoAtual(true);
        if (errosNoTracoAtualRef.current > 1) {
          errosCustomizadosRef.current += 1;
        }
        tocarSomTracoErro();
      },
      onComplete: (resumo: any) => {
        setFase('concluido');
        const totalErros = errosCustomizadosRef.current;
        // No modo treino livre e no modo guiado, sempre acerta. Senão, 50% max erros para acertar.
        const acertou = (apenasTreino || modoGuiado) ? true : (totalErros <= Math.ceil(numTracos * 0.5));
        aoConcluirRef.current(acertou, totalErros);
      },
    });
  };

  // "Prosseguir" (desenho manual): o que é cobrado some com fadeout e então o quiz começa.
  const prosseguirParaDesenho = () => {
    const writer = writerRef.current;
    if (!writer) return;
    esconderComFadeEhIniciarQuiz(writer, tracosTotais);
  };

  const toggleOutline = () => {
    if (!writerRef.current || fase !== 'desenhando') return;
    if (outlineVisivel) {
      writerRef.current.hideOutline();
    } else {
      // Ao mostrar, forçamos que o Hanzi original fique visível em opacidade controlada pelo strokeColor.
      writerRef.current.showOutline();
    }
    setOutlineVisivel(!outlineVisivel);
  };

  const resetarTreino = () => {
    if (!writerRef.current || fase !== 'desenhando') return;
    writerRef.current.cancelQuiz(); // Interrompe o atual
    setTracosFeitos(0);
    setDicasUsadas(0);
    // Reinicia
    iniciarQuiz(writerRef.current, tracosTotais);
  };

  const exibirDica = () => {
    if (!writerRef.current || fase !== 'desenhando') return;
    const maxDicas = Math.max(1, Math.floor(tracosTotais / 3));
    if (dicasUsadas >= maxDicas) return;
    
    writerRef.current.highlightStroke(tracosFeitos);
    setDicasUsadas(prev => prev + 1);
  };

  const pularDesenho = () => {
    if (!writerRef.current || fase !== 'desenhando') return;
    writerRef.current.cancelQuiz();
    writerRef.current.showCharacter();
    setFase('concluido');
    aoConcluirRef.current(false, tracosTotais);
  };

  if (fase === 'erro') {
    return (
      <div style={{ color: 'var(--cor-texto-suave)', textAlign: 'center', padding: '20px' }}>
        {t('Não há dados de traçado para "{hanzi}".', { hanzi })}
      </div>
    );
  }

  return (
    <div style={{ display: 'flex', flexDirection: 'column', alignItems: 'center', gap: '10px' }}>
      <div style={{ position: 'relative' }}>
        <div
          style={{
            position: 'relative',
            width: tamanho,
            height: tamanho,
            backgroundColor: 'var(--cor-fundo-secundario)',
            border: '2px dashed var(--cor-borda)',
            borderRadius: '8px',
          }}
        >
          <div
            ref={guiaRef}
            style={{ position: 'absolute', top: 0, left: 0, pointerEvents: 'none' }}
          />
          <div
            ref={containerRef}
            style={{ position: 'absolute', top: 0, left: 0, touchAction: 'none' }}
          />
          {fase === 'desenhando' && modoGuiado && renderizarOverlayGuiado(tamanho, tracosFeitos, errouTracoAtual, dadosHanzi)}
        </div>

        {fase === 'desenhando' && apenasTreino && (
          <>
            <button
              onClick={resetarTreino}
              title={t("Recomeçar Desenho")}
              style={{
                position: 'absolute', top: '8px', left: '8px',
                background: 'var(--cor-fundo-cartao)', border: '1px solid var(--cor-borda)',
                color: 'var(--cor-texto-primario)',
                borderRadius: '50%', width: '36px', height: '36px', display: 'flex', 
                alignItems: 'center', justifyContent: 'center', cursor: 'pointer',
                zIndex: 10, opacity: 0.8, transition: 'all 0.2s'
              }}
              onMouseOver={e => e.currentTarget.style.opacity = '1'}
              onMouseOut={e => e.currentTarget.style.opacity = '0.8'}
            >
              <IconRefresh />
            </button>
            <button
              onClick={toggleOutline}
              title={outlineVisivel ? t("Esconder Guia") : t("Mostrar Guia")}
              style={{
                position: 'absolute', top: '8px', right: '8px',
                background: 'var(--cor-fundo-cartao)', border: '1px solid var(--cor-borda)',
                color: outlineVisivel ? 'var(--cor-destaque)' : 'var(--cor-texto-primario)',
                borderRadius: '50%', width: '36px', height: '36px', display: 'flex', 
                alignItems: 'center', justifyContent: 'center', cursor: 'pointer',
                zIndex: 10, opacity: 0.8, transition: 'all 0.2s'
              }}
              onMouseOver={e => e.currentTarget.style.opacity = '1'}
              onMouseOut={e => e.currentTarget.style.opacity = '0.8'}
            >
              {outlineVisivel ? <IconEye /> : <IconEyeOff />}
            </button>
          </>
        )}
      </div>

      {fase === 'memorizando' && !fadeoutAutomatico && (
        <button className="scan-btn" onClick={prosseguirParaDesenho}>
          {t('Prosseguir')}
        </button>
      )}

      {fase === 'desenhando' && !apenasTreino && (
        <div style={{ display: 'flex', gap: '12px', marginTop: '10px' }}>
          <button
            onClick={exibirDica}
            disabled={dicasUsadas >= Math.max(1, Math.floor(tracosTotais / 3))}
            style={{
              padding: '6px 12px',
              borderRadius: '6px',
              border: '1px solid var(--cor-borda)',
              background: 'var(--cor-fundo-cartao)',
              color: 'var(--cor-texto-primario)',
              cursor: dicasUsadas >= Math.max(1, Math.floor(tracosTotais / 3)) ? 'not-allowed' : 'pointer',
              opacity: dicasUsadas >= Math.max(1, Math.floor(tracosTotais / 3)) ? 0.5 : 1,
              display: 'flex',
              alignItems: 'center',
              gap: '6px',
              fontSize: '13px',
              transition: 'all 0.2s'
            }}
            onMouseOver={e => !e.currentTarget.disabled && (e.currentTarget.style.borderColor = 'var(--cor-destaque)')}
            onMouseOut={e => e.currentTarget.style.borderColor = 'var(--cor-borda)'}
          >
            <IconDica /> {t('Dica ({quantidade})', { quantidade: Math.max(1, Math.floor(tracosTotais / 3)) - dicasUsadas })}
          </button>
          <button
            onClick={pularDesenho}
            style={{
              padding: '6px 12px',
              borderRadius: '6px',
              border: '1px solid var(--cor-borda)',
              background: 'var(--cor-fundo-cartao)',
              color: 'var(--cor-texto-primario)',
              cursor: 'pointer',
              display: 'flex',
              alignItems: 'center',
              gap: '6px',
              fontSize: '13px',
              transition: 'all 0.2s'
            }}
            onMouseOver={e => e.currentTarget.style.borderColor = 'var(--cor-destaque)'}
            onMouseOut={e => e.currentTarget.style.borderColor = 'var(--cor-borda)'}
          >
            <IconPular /> {t('Pular')}
          </button>
        </div>
      )}

      {(fase === 'memorizando' || fase === 'sumindo' || fase === 'desenhando') && tracosTotais > 0 && (
        <div style={{ fontSize: '12px', color: 'var(--cor-texto-suave)', marginTop: '4px' }}>
          {t('Traços: {atual} / {total}', { atual: tracosFeitos, total: tracosTotais })}
        </div>
      )}
    </div>
  );
}


// ----- Seção: Recorte de Traços (desenho parcial) -----

// tracosDoComponente valida os índices pedidos contra os traçados que o backend devolveu e os põe em
// ORDEM DE ESCRITA (o quiz do Hanzi Writer cobra traço a traço na ordem do charData). Devolve vazio
// — ou seja, "desenhe o caractere inteiro" — quando não sobra nada de fora do recorte ou quando os
// dados não têm medians, que é o que o quiz usa para avaliar o traço do usuário.
function tracosDoComponente(chave: string, dados: any, totalNoCaractere: number): number[] {
  if (chave === '' || !dados.medians) return [];

  const validos = new Set<number>();
  for (const bruto of chave.split(',')) {
    const indice = Number(bruto);
    if (!Number.isInteger(indice) || indice < 0 || indice >= totalNoCaractere) continue;
    validos.add(indice);
  }

  if (validos.size === 0 || validos.size >= totalNoCaractere) return [];
  return [...validos].sort((a, b) => a - b);
}


// fatiarTracados monta um charData do Hanzi Writer só com os traços dos índices pedidos.
function fatiarTracados(dados: any, indices: number[]) {
  return {
    strokes: indices.map(i => dados.strokes[i]),
    medians: indices.map(i => dados.medians[i]),
  };
}


// complementoDeTracos devolve os índices que ficaram DE FORA do recorte — o resto do caractere.
function complementoDeTracos(recorte: number[], totalNoCaractere: number): number[] {
  const noRecorte = new Set(recorte);
  const restantes: number[] = [];
  for (let indice = 0; indice < totalNoCaractere; indice++) {
    if (noRecorte.has(indice)) continue;
    restantes.push(indice);
  }
  return restantes;
}


// ----- Seção: Overlay do Desenho Guiado (ponto inicial e dica) -----

// renderizarOverlayGuiado exibe o ponto inicial do próximo traço (com animação de pulso) ou,
// em caso de erro, exibe a dica visual do traço inteiro até que o acerto ocorra.
function renderizarOverlayGuiado(
  tamanho: number,
  tracosFeitos: number,
  errouTracoAtual: boolean,
  dadosHanzi: any
) {
  if (!dadosHanzi || !dadosHanzi.strokes || !dadosHanzi.medians) return null;
  if (tracosFeitos < 0 || tracosFeitos >= dadosHanzi.strokes.length) return null;

  const padding = 12;
  const escala = (tamanho - padding * 2) / 1024;
  // O Positioner oficial do Hanzi Writer posiciona o topo em y=900:
  // yOffset = padding + 900 * escala
  const yOffset = padding + 900 * escala;

  const mediansDoTraco = dadosHanzi.medians[tracosFeitos];
  const pontoInicial = Array.isArray(mediansDoTraco) && mediansDoTraco.length > 0 ? mediansDoTraco[0] : null;
  const tracoSvg = dadosHanzi.strokes[tracosFeitos];

  // Coordenadas em pixels exatos da tela (fórmulas oficiais do Hanzi Writer):
  // pxX = padding + x * escala
  // pxY = yOffset - y * escala
  const pxX = pontoInicial ? padding + pontoInicial[0] * escala : 0;
  const pxY = pontoInicial ? yOffset - pontoInicial[1] * escala : 0;

  return (
    <svg
      style={{
        position: 'absolute',
        top: 0,
        left: 0,
        width: tamanho,
        height: tamanho,
        pointerEvents: 'none',
        zIndex: 5,
      }}
    >
      {/* Ponto inicial em pixels exatos da tela com animação de pulso */}
      {!errouTracoAtual && pontoInicial && (
        <g key={`ponto-${tracosFeitos}`}>
          <circle
            cx={pxX}
            cy={pxY}
            r={16}
            fill="var(--cor-destaque, #2196f3)"
            opacity={0.35}
          >
            <animate attributeName="r" values="10;20;10" dur="1.5s" repeatCount="indefinite" />
            <animate attributeName="opacity" values="0.5;0.15;0.5" dur="1.5s" repeatCount="indefinite" />
          </circle>
          <circle
            cx={pxX}
            cy={pxY}
            r={7}
            fill="var(--cor-destaque, #2196f3)"
            stroke="#ffffff"
            strokeWidth={2.5}
          />
        </g>
      )}

      {/* Dica de traço inteiro em coordenadas com a transformação oficial do Hanzi Writer */}
      {errouTracoAtual && tracoSvg && (
        <g transform={`translate(${padding}, ${yOffset}) scale(${escala}, ${-escala})`}>
          <path
            key={`dica-${tracosFeitos}`}
            d={tracoSvg}
            fill="var(--cor-destaque, #2196f3)"
            opacity={0.7}
          />
        </g>
      )}
    </svg>
  );
}

