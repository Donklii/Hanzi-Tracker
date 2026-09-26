// ----- Seção: Wrapper de Quebra-Cabeça de 3 Colunas (Trio) -----
import { useMemo } from 'react';
import { JogoQuebraCabecaTrio, ItemQuebraCabeca } from './JogoQuebraCabecaTrio';
import { BotaoAudio } from './BotaoAudio';
import { t } from '../i18n/i18n';

interface OpcaoQuestao {
  hanzi: string;
  pinyin: string;
  definicao: string;
}

interface QuebraCabecaTrioProps {
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

export function QuebraCabecaTrio({
  questao,
  aoConcluir,
  aoTocarAudio,
  hanziTocando,
  hanziSintetizando,
}: QuebraCabecaTrioProps) {
  // Coluna 1: Hanzi
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

  // Coluna 2: Significado (embaralhado)
  const itensMeio = useMemo<ItemQuebraCabeca[]>(() => {
    const itens = questao.opcoes.map((opt) => ({
      id: opt.hanzi,
      conteudo: (
        <div className="puzzle-significado-bloco trio">
          {opt.definicao}
        </div>
      ),
    }));
    return embaralharArray(itens);
  }, [questao.opcoes]);

  // Coluna 3: Fonética / Pinyin com Botão de Áudio. A ORDEM embaralhada é sorteada uma única vez
  // por questão: o conteúdo depende do estado do áudio (tocando/sintetizando), e re-embaralhar
  // junto faria as peças trocarem de lugar sempre que um som começasse ou terminasse.
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

  // Só toca o som quando a conexão fecha o par significado↔som (mid_dir): tocar ao ligar
  // hanzi↔significado entregaria de graça a resposta da coluna de fonética.
  function tratarAcerto(id: string, conexao: 'esq_mid' | 'mid_dir') {
    if (conexao === 'mid_dir' && aoTocarAudio) {
      aoTocarAudio(id);
    }
  }

  return (
    <JogoQuebraCabecaTrio
      itensEsquerda={itensEsquerda}
      itensMeio={itensMeio}
      itensDireita={itensDireita}
      combinacaoCorreta={(idEsq, idAlvo) => idEsq === idAlvo}
      vidasIniciais={2}
      instrucao={t('Conecte os Hanzis (esquerda) aos Significados (meio) e Sons (direita)')}
      aoConcluir={aoConcluir}
      aoAcertarItem={tratarAcerto}
    />
  );
}
