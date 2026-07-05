import React, { useRef, useState, useEffect } from 'react';
import * as HanziLookup from 'hanzilookup-js';

interface CanvasHanziLookupProps {
  onRecognize: (sugestoes: string[]) => void;
  targetHanzi?: string;
  configuracoesApp?: any;
}

export function CanvasHanziLookup({ onRecognize, targetHanzi, configuracoesApp }: CanvasHanziLookupProps) {
  const canvasRef = useRef<HTMLCanvasElement>(null);
  const [isDrawing, setIsDrawing] = useState(false);
  const [strokes, setStrokes] = useState<number[][][]>([]);
  const [currentStroke, setCurrentStroke] = useState<number[][]>([]);
  const [isLoaded, setIsLoaded] = useState(false);

  useEffect(() => {
    // Carregar o modelo MMAH na montagem
    // @ts-ignore
    HanziLookup.init('mmah', '/mmah.json', (success: boolean) => {
      if (success) {
        setIsLoaded(true);
      } else {
        console.error('Falha ao carregar o modelo de dados mmah.json');
      }
    });
  }, []);

  // Redesenha todos os traços sempre que mudarem
  useEffect(() => {
    const canvas = canvasRef.current;
    if (!canvas) return;
    const ctx = canvas.getContext('2d');
    if (!ctx) return;

    ctx.clearRect(0, 0, canvas.width, canvas.height);

    // Desenhar Hanzi guia (tracing) se fornecido
    if (targetHanzi) {
      ctx.save();
      ctx.font = '220px sans-serif';
      ctx.textAlign = 'center';
      ctx.textBaseline = 'middle';
      // Cinza transparente para servir de guia
      ctx.fillStyle = 'rgba(128, 128, 128, 0.15)';
      ctx.fillText(targetHanzi, canvas.width / 2, canvas.height / 2 + 15); // +15 para compensar levemente o alinhamento central
      ctx.restore();
    }
    
    // Config de estilo da caneta
    ctx.lineCap = 'round';
    ctx.lineJoin = 'round';
    ctx.lineWidth = 6;
    ctx.strokeStyle = '#fff'; // Usa cor branca por enquanto

    // Desenhar os traços já concluídos
    strokes.forEach(stroke => {
      if (stroke.length < 1) return;
      ctx.beginPath();
      ctx.moveTo(stroke[0][0], stroke[0][1]);
      for (let i = 1; i < stroke.length; i++) {
        ctx.lineTo(stroke[i][0], stroke[i][1]);
      }
      ctx.stroke();
    });

    // Desenhar o traço atual em andamento
    if (currentStroke.length > 0) {
      ctx.beginPath();
      ctx.moveTo(currentStroke[0][0], currentStroke[0][1]);
      for (let i = 1; i < currentStroke.length; i++) {
        ctx.lineTo(currentStroke[i][0], currentStroke[i][1]);
      }
      ctx.stroke();
    }
  }, [strokes, currentStroke, targetHanzi]);

  const performLookup = (currentAllStrokes: number[][][]) => {
    if (!isLoaded || currentAllStrokes.length === 0) return;
    
    try {
      const char = new HanziLookup.AnalyzedCharacter(currentAllStrokes);
      const matcher = new HanziLookup.Matcher('mmah');
      
      matcher.match(char, 8, async (matches: any[]) => {
        if (matches && matches.length > 0) {
          let suggestions = matches.map(m => m.character);
          
          // Aplica o filtro de exibição caso a restrição esteja ativa
          if (configuracoesApp && configuracoesApp.restringirHanziDesenho && configuracoesApp.tipoHanziExibicao && configuracoesApp.tipoHanziExibicao !== 'ambos') {
            const isSimp = configuracoesApp.tipoHanziExibicao === 'simplificado';
            
            // Requer AvaliarTipoHanzi
            const AvaliarTipoHanzi = (window as any).go?.main?.App?.AvaliarTipoHanzi;
            if (AvaliarTipoHanzi) {
              const filterPromises = suggestions.map(async (hanzi: string) => {
                const tipo = await AvaliarTipoHanzi(hanzi);
                if (isSimp && tipo === 'Tradicional') return null;
                if (!isSimp && tipo === 'Simplificado') return null;
                return hanzi;
              });
              
              const filtered = await Promise.all(filterPromises);
              suggestions = filtered.filter(Boolean);
            }
          }
          
          onRecognize(suggestions);
        } else {
          onRecognize([]);
        }
      });
    } catch (e) {
      console.error("Erro no HanziLookup: ", e);
    }
  };

  const getCoordinates = (e: React.PointerEvent<HTMLCanvasElement>) => {
    const canvas = canvasRef.current;
    if (!canvas) return [0, 0];
    const rect = canvas.getBoundingClientRect();
    const x = e.clientX - rect.left;
    const y = e.clientY - rect.top;
    return [x, y];
  };

  const onPointerDown = (e: React.PointerEvent<HTMLCanvasElement>) => {
    e.currentTarget.setPointerCapture(e.pointerId);
    setIsDrawing(true);
    const coords = getCoordinates(e);
    setCurrentStroke([coords]);
  };

  const onPointerMove = (e: React.PointerEvent<HTMLCanvasElement>) => {
    if (!isDrawing) return;
    const coords = getCoordinates(e);
    setCurrentStroke(prev => [...prev, coords]);
  };

  const onPointerUp = (e: React.PointerEvent<HTMLCanvasElement>) => {
    e.currentTarget.releasePointerCapture(e.pointerId);
    if (!isDrawing) return;
    setIsDrawing(false);
    
    if (currentStroke.length > 0) {
      const newStrokes = [...strokes, currentStroke];
      setStrokes(newStrokes);
      setCurrentStroke([]);
      performLookup(newStrokes);
    }
  };

  const clearCanvas = () => {
    setStrokes([]);
    setCurrentStroke([]);
    onRecognize([]);
  };

  const undoLastStroke = () => {
    if (strokes.length === 0) return;
    const newStrokes = strokes.slice(0, -1);
    setStrokes(newStrokes);
    performLookup(newStrokes);
    if (newStrokes.length === 0) onRecognize([]);
  };

  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      if ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === 'z') {
        e.preventDefault();
        undoLastStroke();
      }
    };
    window.addEventListener('keydown', handleKeyDown);
    return () => window.removeEventListener('keydown', handleKeyDown);
  }, [strokes, isLoaded]); // undoLastStroke also depends on performLookup which needs isLoaded

  return (
    <div style={{ display: 'flex', flexDirection: 'column', alignItems: 'center', width: '100%' }}>
      {!isLoaded && (
        <div style={{ position: 'absolute', color: 'var(--cor-texto-suave)', fontSize: '12px', marginTop: '100px' }}>
          Carregando...
        </div>
      )}
      <canvas
        ref={canvasRef}
        width={300}
        height={300}
        style={{
          border: '1px solid var(--cor-borda)',
          borderRadius: '8px',
          backgroundColor: 'var(--cor-fundo-secundario)',
          touchAction: 'none', // Previne scrolling durante o desenho em dispositivos touch
          cursor: 'crosshair',
          opacity: isLoaded ? 1 : 0.5,
          pointerEvents: isLoaded ? 'auto' : 'none'
        }}
        onPointerDown={onPointerDown}
        onPointerMove={onPointerMove}
        onPointerUp={onPointerUp}
        onPointerCancel={onPointerUp}
      />
      
      <div style={{ display: 'flex', gap: '12px', marginTop: '12px', width: '300px', justifyContent: 'space-between' }}>
        <button
          className="scan-btn"
          style={{ backgroundColor: 'var(--cor-fundo-terciario)', fontSize: '12px', padding: '6px 12px', flex: 1 }}
          onClick={clearCanvas}
          disabled={strokes.length === 0}
        >
          Limpar
        </button>
        <button
          className="scan-btn"
          style={{ backgroundColor: 'var(--cor-fundo-terciario)', fontSize: '12px', padding: '6px 12px', flex: 1 }}
          onClick={undoLastStroke}
          disabled={strokes.length === 0}
        >
          Desfazer Traço
        </button>
      </div>
    </div>
  );
}
