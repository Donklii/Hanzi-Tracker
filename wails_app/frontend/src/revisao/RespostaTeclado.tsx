import { useState, useRef, useEffect } from 'react';
import { main } from '../../wailsjs/go/models';
import { BotaoAudio } from './BotaoAudio';
import { avaliarRespostaContexto, avaliarRespostaFrase, ModoResposta } from './comparacaoDigitada';
import { t } from '../i18n/i18n';

// ----- Seção: Revisão — Resposta por Teclado -----
// Alternativa de entrada para as atividades de contexto (escrever o hanzi), fonética-frase e
// ordenação: em vez de montar peças ou escolher opções, o usuário digita a resposta livremente em
// HANZI ou em PINYIN (a correção detecta o que veio — ver comparacaoDigitada.ts). Espelha o fluxo
// do "✏️ Prefiro desenhar a resposta": substitui a área de resposta e oferece um "↩ Voltar".

interface RespostaTecladoProps {
  questao: main.QuestaoRevisao;
  escopo: 'hanzi' | 'frase'; // hanzi = 1 caractere (contexto); frase = sentença inteira
  respondida: boolean;
  acertou: boolean | null;
  aoConcluir: (acertou: boolean) => void;
  aoTocarAudio: (texto: string) => void;
  hanziTocando: string | null;
  hanziSintetizando: string | null;
  aoVoltar: () => void;
}

export function RespostaTeclado({
  questao, escopo, respondida, acertou, aoConcluir,
  aoTocarAudio, hanziTocando, hanziSintetizando, aoVoltar,
}: RespostaTecladoProps) {
  const [entrada, setEntrada] = useState('');
  const [enviada, setEnviada] = useState<{ texto: string; modo: ModoResposta } | null>(null);
  const inputRef = useRef<HTMLInputElement>(null);
  const ehFrase = escopo === 'frase';

  // Cada questão nova reinicia o campo e devolve o foco ao input.
  useEffect(() => {
    setEntrada('');
    setEnviada(null);
    const t = window.setTimeout(() => inputRef.current?.focus(), 50);
    return () => window.clearTimeout(t);
  }, [questao]);

  function enviar() {
    if (respondida) return;
    const resultado = ehFrase
      ? avaliarRespostaFrase(entrada, questao)
      : avaliarRespostaContexto(entrada, questao);
    if (resultado.modo === 'vazio') return; // não envia em branco
    setEnviada({ texto: entrada.trim(), modo: resultado.modo });
    aoConcluir(resultado.acertou);
  }

  // Enter envia; stopPropagation evita que o atalho global (Enter avança a questão) dispare junto.
  function aoTeclar(e: React.KeyboardEvent<HTMLInputElement>) {
    if (e.key === 'Enter') {
      e.preventDefault();
      e.stopPropagation();
      enviar();
    }
  }

  return (
    <div className="revisao-teclado">
      {ehFrase && !respondida && (
        <BotaoAudio
          rotulo={t("Ouvir a frase")}
          tocando={hanziTocando === questao.fraseOriginal}
          carregando={hanziSintetizando === questao.fraseOriginal}
          aoClicar={() => aoTocarAudio(questao.fraseOriginal)}
        />
      )}

      {!respondida ? (
        <>
          <div className="revisao-teclado-linha">
            <input
              ref={inputRef}
              className="revisao-teclado-input"
              value={entrada}
              onChange={e => setEntrada(e.target.value)}
              onKeyDown={aoTeclar}
              placeholder={ehFrase ? t('Digite a frase em hanzi ou pinyin…') : t('Digite o hanzi ou o pinyin…')}
              autoComplete="off"
              autoCorrect="off"
              spellCheck={false}
              lang="zh"
            />
            <button className="revisao-teclado-enviar" onClick={enviar} disabled={!entrada.trim()}>
              {t('Responder')}
            </button>
          </div>
          <div className="revisao-teclado-dica">
            {t('Aceita caracteres (好) ou pinyin (hǎo, hao, hao3). Os tons são opcionais.')}
          </div>
          <button className="revisao-alternar-resposta" onClick={aoVoltar}>
            {t('↩ Voltar para {opcao}', { opcao: ehFrase ? t('a montagem') : t('as opções') })}
          </button>
        </>
      ) : (
        <div className="revisao-teclado-resultado">
          <div className="revisao-teclado-rotulo">{t('Sua resposta:')}</div>
          <div className={`revisao-teclado-enviada ${acertou ? 'acerto' : 'erro'}`}>
            <span>{enviada?.texto || '—'}</span>
            {enviada && (
              <span className="revisao-teclado-tag">{enviada.modo === 'hanzi' ? '汉字' : 'pīnyīn'}</span>
            )}
          </div>
          {ehFrase && (
            <FraseRevelada
              questao={questao}
              aoTocarAudio={aoTocarAudio}
              hanziTocando={hanziTocando}
              hanziSintetizando={hanziSintetizando}
            />
          )}
        </div>
      )}
    </div>
  );
}

interface FraseReveladaProps {
  questao: main.QuestaoRevisao;
  aoTocarAudio: (texto: string) => void;
  hanziTocando: string | null;
  hanziSintetizando: string | null;
}

// Revelação da frase correta (pinyin sobre cada hanzi) no escopo de frase — o banner de feedback
// só mostra o caractere-alvo, então a sentença completa precisa aparecer aqui.
function FraseRevelada({ questao, aoTocarAudio, hanziTocando, hanziSintetizando }: FraseReveladaProps) {
  const tokens = questao.fraseOriginalSegmentada || [];
  return (
    <div className="revisao-teclado-frase-correta">
      <div className="revisao-teclado-rotulo">{t('Frase correta:')}</div>
      <div className="revisao-teclado-frase-tokens">
        <BotaoAudio
          rotulo=""
          tocando={hanziTocando === questao.fraseOriginal}
          carregando={hanziSintetizando === questao.fraseOriginal}
          aoClicar={() => aoTocarAudio(questao.fraseOriginal)}
        />
        {tokens.map((t, i) =>
          t.ehChines && t.pinyin ? (
            <span key={i} className="revisao-teclado-token">
              <span className={`py ${t.ehNaoVista ? 'revisao-pinyin-nao-visto' : ''}`}>{t.pinyin}</span>
              <span className={`hz ${t.ehNaoVista ? 'revisao-palavra-nao-vista' : ''}`}>{t.texto}</span>
            </span>
          ) : (
            <span key={i} className="revisao-teclado-token-latim">{t.texto}</span>
          )
        )}
      </div>
      {questao.fraseTraducao && <div className="revisao-teclado-traducao">{questao.fraseTraducao}</div>}
    </div>
  );
}
