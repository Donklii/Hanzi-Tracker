// ----- Seção: Revisão — Grade de 4 Opções -----
// tipoConteudo decide o que cada botão mostra:
//   'hanzi'     — o caractere grande
//   'definicao' — o significado
//   'audio'     — um botão ▶ que TOCA o som daquela opção (o clique no corpo do botão responde)
// Após respondida: a opção correta fica verde; a escolhida errada fica vermelha (com tremor).
import { main } from '../../wailsjs/go/models';

interface OpcoesRevisaoProps {
  opcoes: main.OpcaoRevisao[];
  tipoConteudo: 'hanzi' | 'definicao' | 'audio';
  respondida: boolean;
  indiceEscolhido: number | null;
  aoEscolher: (indice: number) => void;
  aoTocarAudio?: (hanzi: string) => void;
  hanziTocando?: string | null;
  hanziSintetizando?: string | null;
  vertical?: boolean;
}

export function OpcoesRevisao({ opcoes, tipoConteudo, respondida, indiceEscolhido, aoEscolher, aoTocarAudio, hanziTocando, hanziSintetizando, vertical }: OpcoesRevisaoProps) {
  const classesOpcao = (indice: number): string => {
    const classes = ['revisao-opcao'];
    if (tipoConteudo === 'hanzi') classes.push('hanzi');
    if (respondida && opcoes[indice].correta) classes.push('correta');
    if (respondida && indice === indiceEscolhido && !opcoes[indice].correta) classes.push('errada');
    return classes.join(' ');
  };

  const classesWrapper = `revisao-opcoes${vertical ? ' vertical' : ''}`;

  return (
    <div className={classesWrapper}>
      {opcoes.map((opcao, indice) => (
        <button
          key={indice}
          className={classesOpcao(indice)}
          disabled={respondida}
          onClick={() => aoEscolher(indice)}
        >
          {tipoConteudo === 'audio' ? (
            <>
              <span
                className="revisao-opcao-play"
                onClick={e => { e.stopPropagation(); aoTocarAudio && aoTocarAudio(opcao.hanzi); }}
              >
                {hanziSintetizando === opcao.hanzi ? '…' : hanziTocando === opcao.hanzi ? '🔊' : '▶'}
              </span>
              <span style={{ fontSize: '12px', color: 'var(--cor-texto-suave)' }}>Som {indice + 1}</span>
              {respondida && <span style={{ fontSize: '18px', fontFamily: 'var(--fonte-hanzi)' }}>{opcao.hanzi}</span>}
            </>
          ) : (
            <span>{tipoConteudo === 'hanzi' ? opcao.hanzi : opcao.definicao}</span>
          )}
        </button>
      ))}
    </div>
  );
}
