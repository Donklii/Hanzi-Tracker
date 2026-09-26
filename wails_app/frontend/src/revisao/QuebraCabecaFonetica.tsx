// ----- Seção: Wrapper de Quebra-Cabeça de Fonética -----
import { useMemo } from 'react';
import { JogoQuebraCabeca, ItemQuebraCabeca } from './JogoQuebraCabeca';
import { BotaoAudio } from './BotaoAudio';
import { t } from '../i18n/i18n';

interface OpcaoQuestao {
  hanzi: string;
  pinyin: string;
  definicao: string;
}

interface QuebraCabecaFoneticaProps {
  questao: {
    opcoes: OpcaoQuestao[];
  };
  aoConcluir: (acertou: boolean, matchedIds: string[]) => void;
  aoTocarAudio: (texto: string) => void;
  hanziTocando?: string | null;
  hanziSintetizando?: string | null;
}

function embaralharArray<T>(array: T[]): T[] {
  const copia = [...array];

  for (let i = copia.length - 1; i > 0; i--) {
    const j = Math.floor(Math.random() * (i + 1));
    [copia[i], copia[j]] = [copia[j], copia[i]];
  }

  return copia;
}

export function QuebraCabecaFonetica({
  questao,
  aoConcluir,
  aoTocarAudio,
  hanziTocando,
  hanziSintetizando,
}: QuebraCabecaFoneticaProps) {
  const itensEsquerda = useMemo<ItemQuebraCabeca[]>(() => {
    return questao.opcoes.map((opt) => ({
      id: opt.hanzi,
      conteudo: (
        <div className="puzzle-hanzi-bloco">
          <div className="puzzle-hanzi-caractere">{opt.hanzi}</div>
        </div>
      ),
    }));
  }, [questao.opcoes]);

  // A ORDEM embaralhada é sorteada uma única vez por questão: o conteúdo depende do estado do
  // áudio (tocando/sintetizando), e re-embaralhar junto faria as peças trocarem de lugar sempre
  // que um som começasse ou terminasse.
  const ordemDireita = useMemo(() => embaralharArray(questao.opcoes), [questao.opcoes]);

  const itensDireita = useMemo<ItemQuebraCabeca[]>(() => {
    return ordemDireita.map((opt) => ({
      id: opt.hanzi,
      conteudo: (
        <div className="puzzle-fonetica-bloco">
          <BotaoAudio
            rotulo=""
            tocando={hanziTocando === opt.hanzi}
            carregando={hanziSintetizando === opt.hanzi}
            aoClicar={() => aoTocarAudio(opt.hanzi)}
            titulo={t('Ouvir o som')}
          />
          <span className="puzzle-fonetica-pinyin">{opt.pinyin}</span>
        </div>
      ),
    }));
  }, [ordemDireita, hanziTocando, hanziSintetizando, aoTocarAudio]);

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
      instrucao={t('Arraste os Hanzis (esquerda) e conecte nos seus sons (direita)')}
      colunaDireitaCompacta
      aoConcluir={aoConcluir}
      aoAcertarItem={tratarAcerto}
    />
  );
}
