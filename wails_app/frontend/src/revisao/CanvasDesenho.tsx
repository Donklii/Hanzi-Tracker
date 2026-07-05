// ----- Seção: Revisão — Canvas de Desenho (motor Hanzi Writer) -----
// Wrapper React do Hanzi Writer para os modos de desenho. Os traçados NÃO vêm de CDN: o
// charDataLoader busca o JSON no backend Go (ObterDadosEscritaHanzi), que serve o banco embarcado
// hanzi-writer-data — o app funciona 100% offline.
//
// Duas formas de uso (prop modoMemoria):
//   false — quiz direto: canvas vazio, o usuário desenha o hanzi traço a traço.
//   true  — desenho de memória: o caractere é exibido; ao clicar em "Prosseguir" ele sofre um
//           fadeout e o usuário o redesenha de memória.
import { useEffect, useRef, useState } from 'react';
import HanziWriter from 'hanzi-writer';
import { ObterDadosEscritaHanzi } from '../../wailsjs/go/main/App';
import { tocarSomTracoOk, tocarSomTracoErro } from './sons';

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

interface CanvasDesenhoProps {
  hanzi: string;
  modoMemoria: boolean;
  aoConcluir: (acertou: boolean, totalErros: number) => void;
  tamanho?: number;
  fadeoutAutomatico?: boolean;
  mostrarDicaAposErros?: number;
  apenasTreino?: boolean;
}

type FaseCanvas = 'carregando' | 'memorizando' | 'desenhando' | 'concluido' | 'erro';

export function CanvasDesenho({ 
  hanzi, 
  modoMemoria, 
  aoConcluir, 
  tamanho = 280,
  fadeoutAutomatico = false,
  mostrarDicaAposErros = 3,
  apenasTreino = false
}: CanvasDesenhoProps) {
  const containerRef = useRef<HTMLDivElement>(null);
  const writerRef = useRef<any>(null);
  const [fase, setFase] = useState<FaseCanvas>('carregando');
  const [tracosTotais, setTracosTotais] = useState(0);
  const [tracosFeitos, setTracosFeitos] = useState(0);
  const [outlineVisivel, setOutlineVisivel] = useState(false);

  // aoConcluir vive num ref: o closure do quiz é criado uma vez, mas o pai troca o callback a cada
  // questão respondida (setState) — sem o ref, o quiz chamaria uma versão obsoleta.
  const aoConcluirRef = useRef(aoConcluir);
  useEffect(() => { aoConcluirRef.current = aoConcluir; }, [aoConcluir]);

  useEffect(() => {
    if (!containerRef.current || !hanzi) return;

    let cancelado = false;
    setFase('carregando');
    setTracosFeitos(0);
    setOutlineVisivel(false);

    ObterDadosEscritaHanzi(hanzi)
      .then(json => {
        if (cancelado || !containerRef.current) return;

        const dados = JSON.parse(json);
        const numTracos: number = (dados.strokes || []).length;
        setTracosTotais(numTracos);

        containerRef.current.innerHTML = ''; // remove o writer da questão anterior
        const writer = HanziWriter.create(containerRef.current, hanzi, {
          width: tamanho,
          height: tamanho,
          padding: 12,
          showCharacter: modoMemoria,
          showOutline: false,
          strokeColor: '#e0e0e0', // Hanzi inicial totalmente opaco (cor do texto, some no fadeOut)
          outlineColor: 'rgba(255, 255, 255, 0.2)', // Guia (Outline) semi-transparente quando a dica é ativada
          drawingColor: '#ffffff', // Os traços do usuário são brancos opacos
          highlightColor: '#2196f3',
          drawingWidth: 18,
          charDataLoader: (_char: string, onComplete: (d: any) => void) => onComplete(dados),
        });
        writerRef.current = writer;

        if (modoMemoria) {
          setFase('memorizando');
          if (fadeoutAutomatico) {
            setTimeout(() => {
              if (cancelado || !writerRef.current) return;
              
              // Usa a opção nativa do hanzi-writer
              writerRef.current.hideCharacter({ duration: 1000 });
              
              setTimeout(() => {
                if (cancelado || !writerRef.current) return;
                iniciarQuiz(writerRef.current, numTracos);
              }, 1000); // Inicia o quiz apenas quando terminar o fadeout
              
            }, 1000);
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
      if (writerRef.current) {
        writerRef.current.cancelQuiz();
        writerRef.current = null;
      }
    };
  }, [hanzi, modoMemoria, fadeoutAutomatico, tamanho]);

  const iniciarQuiz = (writer: any, numTracos: number) => {
    setFase('desenhando');
    writer.quiz({
      leniency: 1.3,          // desenho "aproximado": mais tolerante que o padrão
      showHintAfterMisses: mostrarDicaAposErros, // default é 3, mas pode ser configurado pra 1 em treino livre
      onCorrectStroke: (dados: any) => {
        tocarSomTracoOk();
        setTracosFeitos(dados.strokesRemaining >= 0 ? numTracos - dados.strokesRemaining : 0);
      },
      onMistake: () => tocarSomTracoErro(),
      onComplete: (resumo: any) => {
        setFase('concluido');
        const totalErros: number = resumo.totalMistakes ?? 0;
        // No modo treino livre, sempre acerta. Senão, 50% max erros para acertar.
        const acertou = apenasTreino ? true : (totalErros <= Math.ceil(numTracos * 0.5));
        aoConcluirRef.current(acertou, totalErros);
      },
    });
  };

  // "Prosseguir" (desenho de memória manual): o caractere some com fadeout e o quiz começa.
  const prosseguirParaDesenho = () => {
    const writer = writerRef.current;
    if (!writer) return;
    writer.hideCharacter();
    iniciarQuiz(writer, tracosTotais);
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
    // Reinicia
    iniciarQuiz(writerRef.current, tracosTotais);
  };

  if (fase === 'erro') {
    return (
      <div className="canvas-desenho-erro" style={{ color: 'var(--cor-texto-suave)', textAlign: 'center', padding: '20px' }}>
        Não há dados de traçado para "{hanzi}".
      </div>
    );
  }

  return (
    <div className="canvas-desenho" style={{ display: 'flex', flexDirection: 'column', alignItems: 'center', gap: '10px' }}>
      <div style={{ position: 'relative' }}>
        <div
          ref={containerRef}
          className="canvas-desenho-area"
          style={{
            width: tamanho,
            height: tamanho,
            backgroundColor: 'var(--cor-fundo-secundario)',
            border: '2px dashed var(--cor-borda)',
            borderRadius: '8px',
            touchAction: 'none',
          }}
        />
        
        {fase === 'desenhando' && apenasTreino && (
          <>
            <button
              onClick={resetarTreino}
              title="Recomeçar Desenho"
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
              title={outlineVisivel ? "Esconder Guia" : "Mostrar Guia"}
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
          Prosseguir
        </button>
      )}

      {(fase === 'memorizando' || fase === 'desenhando') && tracosTotais > 0 && (
        <div style={{ fontSize: '12px', color: 'var(--cor-texto-suave)' }}>
          Traços: {tracosFeitos} / {tracosTotais}
        </div>
      )}
    </div>
  );
}
