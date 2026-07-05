// ----- Seção: Revisão — Placar Final -----
// Tela de celebração da sessão: percentual com contagem animada, chuva de confetes quando o
// desempenho é bom (>= 60%), e as estatísticas de gamificação (pontos e melhor sequência).
import { useEffect, useMemo, useState } from 'react';

interface PlacarRevisaoProps {
  acertos: number;
  total: number;
  modo: string;
  pontos: number;
  melhorSequencia: number;
  aoRepetir: () => void;
  aoTrocarModo: () => void;
}

const ROTULOS_MODO: Record<string, string> = {
  geral: 'Geral',
  significado: 'Significado',
  fonetica: 'Fonética',
  desenho: 'Desenho',
  contexto: 'Contexto',
};

const CORES_CONFETE = ['#6366f1', '#10b981', '#f59e0b', '#ef4444', '#a5b4fc', '#34d399'];
const TOTAL_CONFETES = 40;

function mensagemPorFaixa(percentual: number): string {
  if (percentual === 100) return 'Perfeito! 完美!';
  if (percentual >= 80) return 'Excelente!';
  if (percentual >= 60) return 'Bom progresso!';
  if (percentual >= 40) return 'Continue praticando!';
  return 'Não desista — a repetição é o caminho.';
}

export function PlacarRevisao({ acertos, total, modo, pontos, melhorSequencia, aoRepetir, aoTrocarModo }: PlacarRevisaoProps) {
  const percentual = total > 0 ? Math.round((acertos / total) * 100) : 0;

  // Contagem animada: o número sobe de 0 até o percentual em ~900ms (ease-out).
  const [percentualExibido, setPercentualExibido] = useState(0);
  useEffect(() => {
    let quadro = 0;
    const inicio = performance.now();
    const duracaoMs = 900;

    function animar(agora: number) {
      const progresso = Math.min((agora - inicio) / duracaoMs, 1);
      const suavizado = 1 - Math.pow(1 - progresso, 3); // ease-out cúbico
      setPercentualExibido(Math.round(percentual * suavizado));
      if (progresso < 1) quadro = requestAnimationFrame(animar);
    }

    quadro = requestAnimationFrame(animar);
    return () => cancelAnimationFrame(quadro);
  }, [percentual]);

  // Confetes: posições/cores/tempos sorteados uma única vez por montagem do placar.
  const confetes = useMemo(() => {
    if (percentual < 60) return [];
    return Array.from({ length: TOTAL_CONFETES }, (_, i) => ({
      chave: i,
      esquerda: Math.random() * 100,
      cor: CORES_CONFETE[i % CORES_CONFETE.length],
      atrasoS: Math.random() * 0.8,
      duracaoS: 2 + Math.random() * 1.5,
      tamanhoPx: 6 + Math.random() * 6,
    }));
  }, [percentual]);

  return (
    <div className="revisao-placar">
      {confetes.length > 0 && (
        <div className="revisao-confetes" aria-hidden="true">
          {confetes.map(c => (
            <span
              key={c.chave}
              className="revisao-confete"
              style={{
                left: `${c.esquerda}%`,
                backgroundColor: c.cor,
                width: `${c.tamanhoPx}px`,
                height: `${c.tamanhoPx * 0.45}px`,
                animationDelay: `${c.atrasoS}s`,
                animationDuration: `${c.duracaoS}s`,
              }}
            />
          ))}
        </div>
      )}

      <div className="revisao-placar-percentual">{percentualExibido}%</div>
      <div className="revisao-placar-mensagem">{mensagemPorFaixa(percentual)}</div>
      <div className="revisao-placar-detalhe">
        {acertos} de {total} questões corretas — modo {ROTULOS_MODO[modo] || modo}
      </div>

      <div className="revisao-placar-barra">
        <div className="revisao-placar-barra-preenchimento" style={{ width: `${percentualExibido}%` }}></div>
      </div>

      {/* Estatísticas de gamificação da sessão */}
      <div className="revisao-placar-estatisticas">
        <div className="revisao-placar-estatistica">
          <div className="revisao-placar-estatistica-valor">🔥 {melhorSequencia}</div>
          <div className="revisao-placar-estatistica-rotulo">melhor sequência</div>
        </div>
        <div className="revisao-placar-estatistica">
          <div className="revisao-placar-estatistica-valor">✓ {acertos}</div>
          <div className="revisao-placar-estatistica-rotulo">acertos</div>
        </div>
      </div>

      <div className="revisao-placar-acoes">
        <button className="scan-btn" onClick={aoRepetir}>Revisar novamente</button>
        <button
          className="scan-btn"
          style={{ backgroundColor: 'var(--cor-fundo-cartao)', border: '1px solid var(--cor-borda)' }}
          onClick={aoTrocarModo}
        >
          Trocar de modo
        </button>
      </div>
    </div>
  );
}
