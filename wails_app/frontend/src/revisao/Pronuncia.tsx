import { useState, useEffect, useMemo, useRef } from 'react';
import { main } from '../../wailsjs/go/models';
import { useSTT } from '../comum/useSTT';
import './revisao.css';

interface PronunciaProps {
  questao: main.QuestaoRevisao;
  respondida: boolean;
  aoConcluir: (acertou: boolean) => void;
  aoTocarAudio: (texto: string) => void;
  hanziTocando: string | null;
  hanziSintetizando: string | null;
  AoClicarNoCartao?: (card: any) => void;
}

export function Pronuncia(props: PronunciaProps) {
  if (props.questao.variante === 'pronuncia_frase') {
    return <PronunciaFrase {...props} />;
  }
  return <PronunciaSequencia {...props} />;
}

function calcularAcerto(textoOriginal: string, transcricao: string): number {
  // Remove pontuações e espaços para comparar apenas os Hanzis
  const limpar = (t: string) => t.replace(/[^\p{L}\p{N}]/gu, '');
  const alvo = limpar(textoOriginal);
  const obtido = limpar(transcricao);

  if (alvo.length === 0) return 0;

  let acertos = 0;
  for (let i = 0; i < alvo.length; i++) {
    if (obtido.includes(alvo[i])) {
      acertos++;
    }
  }
  return acertos / alvo.length;
}

// Mensagem exibida quando NENHUM caminho de STT existe (sem Web Speech e sem motor instalado).
// `motivo` orienta o próximo passo (baixar o motor em Configurações → Motores).
function PronunciaIndisponivel({ motivo }: { motivo: string | null }) {
  return (
    <div className="revisao-pronuncia-erro">
      ⚠️ Reconhecimento de voz indisponível.
      {motivo && <div style={{ marginTop: '6px', fontSize: '0.9em' }}>{motivo}</div>}
    </div>
  );
}

function PronunciaFrase({ questao, respondida, aoConcluir }: PronunciaProps) {
  const {
    suportado, motivoIndisponivel, escutando, processando,
    transcricaoParcial, transcricaoFinal, estadoMotor, iniciar, parar, erro,
  } = useSTT({ idioma: 'zh-CN', continuo: false });
  const timeoutRef = useRef<number | null>(null);

  useEffect(() => {
    // Avalia a transcrição final quando a gravação para
    if (!escutando && transcricaoFinal && !respondida) {
      const pontuacao = calcularAcerto(questao.fraseOriginal, transcricaoFinal);
      if (pontuacao >= 0.75) {
        aoConcluir(true);
      } else {
        aoConcluir(false);
      }
    }
  }, [escutando, transcricaoFinal, respondida, questao.fraseOriginal, aoConcluir]);

  if (!suportado) {
    return <PronunciaIndisponivel motivo={motivoIndisponivel} />;
  }

  const transcricaoAtual = transcricaoFinal + (transcricaoParcial ? ' ' + transcricaoParcial : '');
  // No modo motor não há parciais: o estado vindo do Go ("Transcrevendo fala…") preenche o vão
  // entre soltar o botão e a transcrição chegar.
  const feedback = transcricaoAtual
    || (escutando ? 'Ouvindo...' : '')
    || estadoMotor
    || (processando ? 'Transcrevendo...' : '...');

  return (
    <div className="revisao-pronuncia-container">
      <div className="revisao-pronuncia-instrucao">
        Pressione e segure o microfone para falar a frase.
      </div>

      <div className="revisao-pronuncia-feedback">
        {feedback}
      </div>

      <button
        className={`revisao-btn-microfone ${escutando ? 'escutando' : ''}`}
        onMouseDown={() => iniciar()}
        onMouseUp={() => parar()}
        onMouseLeave={() => parar()}
        onTouchStart={(e) => { e.preventDefault(); iniciar(); }}
        onTouchEnd={(e) => { e.preventDefault(); parar(); }}
        disabled={respondida || processando}
      >
        <svg width="32" height="32" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
          <path d="M12 2a3 3 0 0 0-3 3v7a3 3 0 0 0 6 0V5a3 3 0 0 0-3-3Z"></path>
          <path d="M19 10v2a7 7 0 0 1-14 0v-2"></path>
          <line x1="12" y1="19" x2="12" y2="22"></line>
        </svg>
      </button>

      {erro && <div className="revisao-pronuncia-erro">Erro: {erro}</div>}
    </div>
  );
}

interface FilaItem {
  id: string;
  hanzi: string;
  pinyin: string;
  significados: string;
}

function PronunciaSequencia({ questao, respondida, aoConcluir, aoTocarAudio }: PronunciaProps) {
  // Inicializa a fila de caracteres com base na frase segmentada (pegando apenas os hanzis)
  const filaInicial = useMemo(() => {
    const itens: FilaItem[] = [];
    questao.fraseOriginalSegmentada?.forEach((p, idx) => {
      if (p.ehChines) {
        for (let i = 0; i < p.texto.length; i++) {
          itens.push({
            id: `${idx}-${i}`,
            hanzi: p.texto[i],
            pinyin: p.pinyin, // simplificação: usa o pinyin da palavra
            significados: p.significados?.join(', ') || ''
          });
        }
      }
    });
    return itens;
  }, [questao]);

  const [fila, setFila] = useState<FilaItem[]>(filaInicial);
  const [erroAtual, setErroAtual] = useState<FilaItem | null>(null);

  const {
    suportado, motivoIndisponivel, escutando, processando,
    transcricaoFinal, estadoMotor, iniciar, parar, limpar, erro,
  } = useSTT({ idioma: 'zh-CN', continuo: false });

  useEffect(() => {
    if (!escutando && transcricaoFinal && !respondida && fila.length > 0) {
      const alvo = fila[0].hanzi;
      const pontuacao = calcularAcerto(alvo, transcricaoFinal);
      // Consome a transcrição: sem isto, o avanço da fila re-dispararia este efeito e o PRÓXIMO
      // caractere seria avaliado contra a fala anterior (erro em cascata).
      limpar();

      if (pontuacao >= 1) {
        // Acertou o caractere atual
        setErroAtual(null);
        const novaFila = fila.slice(1);
        setFila(novaFila);
        if (novaFila.length === 0) {
          aoConcluir(true);
        }
      } else {
        // Errou o caractere
        const itemErro = fila[0];
        setErroAtual(itemErro);
        aoTocarAudio(itemErro.hanzi); // Toca o áudio em voz alta
        
        // Move o cartão pro fim da fila
        setTimeout(() => {
          setFila(prev => {
            const [primeiro, ...resto] = prev;
            return [...resto, primeiro];
          });
          setErroAtual(null);
        }, 2000); // 2 segundos para o usuário ver o erro
      }
    }
  }, [escutando, transcricaoFinal, respondida, fila, aoConcluir, aoTocarAudio]);

  if (!suportado) {
    return <PronunciaIndisponivel motivo={motivoIndisponivel} />;
  }

  if (fila.length === 0) {
    return null; // Já concluiu, aba de revisão lidará com o feedback geral
  }

  const alvo = fila[0];

  return (
    <div className="revisao-pronuncia-container">
      <div className="revisao-pronuncia-instrucao">
        Fale o caractere em destaque. Pressione para falar.
      </div>

      {(estadoMotor || processando) && (
        <div className="revisao-pronuncia-feedback">
          {estadoMotor || 'Transcrevendo...'}
        </div>
      )}

      <div className="revisao-pronuncia-sequencia">
        {fila.slice(0, 3).map((item, index) => (
          <div key={item.id} className={`revisao-cartao-pronuncia ${index === 0 ? 'destaque' : 'fila'} ${erroAtual?.id === item.id ? 'erro' : ''}`}>
            <span className="hanzi">{item.hanzi}</span>
            {index === 0 && erroAtual?.id === item.id && (
              <span className="pinyin-feedback">{item.pinyin}</span>
            )}
          </div>
        ))}
        {fila.length > 3 && <div className="revisao-cartao-pronuncia mais">+ {fila.length - 3}</div>}
      </div>

      <button
        className={`revisao-btn-microfone ${escutando ? 'escutando' : ''}`}
        onMouseDown={() => iniciar()}
        onMouseUp={() => parar()}
        onMouseLeave={() => parar()}
        onTouchStart={(e) => { e.preventDefault(); iniciar(); }}
        onTouchEnd={(e) => { e.preventDefault(); parar(); }}
        disabled={respondida || processando || erroAtual !== null}
      >
        <svg width="32" height="32" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
          <path d="M12 2a3 3 0 0 0-3 3v7a3 3 0 0 0 6 0V5a3 3 0 0 0-3-3Z"></path>
          <path d="M19 10v2a7 7 0 0 1-14 0v-2"></path>
          <line x1="12" y1="19" x2="12" y2="22"></line>
        </svg>
      </button>

      {erro && <div className="revisao-pronuncia-erro">Erro: {erro}</div>}
    </div>
  );
}
