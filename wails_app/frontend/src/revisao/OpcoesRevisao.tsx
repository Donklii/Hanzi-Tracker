// ----- Seção: Revisão — Grade de 4 Opções -----
// tipoConteudo decide o que cada botão mostra:
//   'hanzi'        — o caractere grande
//   'definicao'    — o significado
//   'pinyin'       — a transcrição pinyin
//   'audio'        — um botão ▶ que TOCA o som daquela opção (o clique no corpo do botão responde)
//   'imagem_hanzi' — imagem do Hanzi em miniatura + o Hanzi/palavra abaixo
// Após respondida: a opção correta fica verde; a escolhida errada fica vermelha (com tremor).
import { main } from '../../wailsjs/go/models';
import { t } from '../i18n/i18n';
import { obterUrlImagemHanzi } from './obterImagemHanzi';

interface OpcoesRevisaoProps {
  opcoes: main.OpcaoRevisao[];
  tipoConteudo: 'hanzi' | 'definicao' | 'audio' | 'pinyin' | 'imagem_hanzi';
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
    if (tipoConteudo === 'imagem_hanzi') classes.push('imagem-hanzi');
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
              <span style={{ fontSize: '12px', color: 'var(--cor-texto-suave)' }}>{t('Som {numero}', { numero: indice + 1 })}</span>
              {respondida && <span style={{ fontSize: '18px', fontFamily: 'var(--fonte-hanzi)' }}>{opcao.hanzi}</span>}
            </>
          ) : tipoConteudo === 'imagem_hanzi' ? (
            <div className="revisao-opcao-imagem-conteudo">
              {obterUrlImagemHanzi(opcao.hanzi) && (
                <img
                  src={obterUrlImagemHanzi(opcao.hanzi)!}
                  alt={opcao.hanzi}
                  className="revisao-opcao-imagem-thumb"
                />
              )}
              <span className="revisao-opcao-hanzi-sub">{opcao.hanzi}</span>
            </div>
          ) : (
            <span>{tipoConteudo === 'hanzi' ? opcao.hanzi : tipoConteudo === 'pinyin' ? opcao.pinyin : opcao.definicao}</span>
          )}
        </button>
      ))}
    </div>
  );
}
