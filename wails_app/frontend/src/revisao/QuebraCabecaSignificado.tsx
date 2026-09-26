// ----- Seção: Wrapper de Quebra-Cabeça de Significado -----
import { useMemo } from 'react';
import { JogoQuebraCabeca, ItemQuebraCabeca } from './JogoQuebraCabeca';
import { t } from '../i18n/i18n';

interface OpcaoQuestao {
  hanzi: string;
  pinyin: string;
  definicao: string;
}

interface QuebraCabecaSignificadoProps {
  questao: {
    opcoes: OpcaoQuestao[];
  };
  aoConcluir: (acertou: boolean, matchedIds: string[]) => void;
  aoTocarAudio?: (texto: string) => void;
}


function embaralharArray<T>(array: T[]): T[] {
  const copia = [...array];

  for (let i = copia.length - 1; i > 0; i--) {
    const j = Math.floor(Math.random() * (i + 1));
    [copia[i], copia[j]] = [copia[j], copia[i]];
  }

  return copia;
}

export function QuebraCabecaSignificado({ questao, aoConcluir, aoTocarAudio }: QuebraCabecaSignificadoProps) {
  const itensEsquerda = useMemo<ItemQuebraCabeca[]>(() => {
    return questao.opcoes.map((opt) => ({
      id: opt.hanzi,
      conteudo: (
        <div className="puzzle-hanzi-bloco">
          <div className="puzzle-hanzi-caractere">{opt.hanzi}</div>
          <div className="puzzle-hanzi-pinyin">{opt.pinyin}</div>
        </div>
      ),
    }));
  }, [questao.opcoes]);

  const itensDireita = useMemo<ItemQuebraCabeca[]>(() => {
    const itens = questao.opcoes.map((opt) => ({
      id: opt.hanzi,
      conteudo: (
        <div className="puzzle-significado-bloco">
          {opt.definicao}
        </div>
      ),
    }));

    return embaralharArray(itens);
  }, [questao.opcoes]);


  function tratarAcerto(id: string) {
    if (aoTocarAudio) {
      aoTocarAudio(id);
    }
  }

  return (
    <JogoQuebraCabeca
      itensEsquerda={itensEsquerda}
      itensDireita={itensDireita}
      combinacaoCorreta={(idEsq, idDir) => idEsq === idDir}
      vidasIniciais={2}
      instrucao={t('Arraste os Hanzis (esquerda) e conecte nos seus significados (direita)')}
      aoConcluir={aoConcluir}
      aoAcertarItem={tratarAcerto}
    />
  );
}
