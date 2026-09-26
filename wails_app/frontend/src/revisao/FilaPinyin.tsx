import React, { useState, useEffect, useRef } from 'react';
import { revisao } from '../../wailsjs/go/models';
import { BotaoAudio } from './BotaoAudio';
import { extrairLeituras } from './comparacaoPronuncia';
import { tocarSomAcerto } from '../comum/sons';
import { t } from '../i18n/i18n';

// ----- Seção: Atividade Fila de Pinyin -----
// Variante "fonetica_fila_pinyin" (Iniciante): A frase em chinês é exibida no padrão de sentença das
// atividades de revisão. O pinyin da frase inteira é exibido censurado acima dos hanzis e é revelado
// gradualmente caractere a caractere ao longo do tempo (dica progressiva).
// A dica de revelação de letras é sempre focada no índice efetivo pendente (indiceEfetivo), considerando
// tanto as palavras já enviadas quanto as palavras digitadas corretamente na caixa de texto em tempo real,
// garantindo que a dica avance imediatamente para a próxima palavra não escrita.
// É possível submeter partes da frase de uma só vez (ou a frase inteira).
// Durante a digitação, tanto o texto na caixa de entrada quanto os Hanzis na frase acima que casam com a
// sequência assumem a cor verde em tempo real (live highlighting).
// A validação ocorre estritamente de forma sequencial (da esquerda para a direita). Se um envio contiver
// partes corretas e incorretas, a parte correta é validada e a parte incorreta é descartada.

interface FilaPinyinProps {
  questao: revisao.QuestaoRevisao;
  respondida: boolean;
  acertou: boolean | null;
  aoConcluir: (acertou: boolean) => void;
  aoTocarAudio: (texto: string) => void;
  hanziTocando: string | null;
  hanziSintetizando: string | null;
  AoClicarNoCartao?: (item: any) => void;
}

// ----- Seção: Auxiliares de Pinyin e Validação -----

function compactarPinyin(texto: string): string {
  return extrairLeituras(texto).reduce((acc, leitura) => acc + leitura.join(''), '');
}

function leiturasAceitasToken(token: revisao.PalavraRevisao): string[] {
  if (!token.pinyin) return [];
  const pinyinsBrutos = token.pinyin.split(/[,，;；/]/);
  const aceitas: string[] = [];
  for (const p of pinyinsBrutos) {
    const c = compactarPinyin(p);
    if (c) aceitas.push(c);
  }
  return aceitas;
}

function contemHanzi(texto: string): boolean {
  return /[㐀-鿿]/.test(texto);
}

function apenasHanzis(texto: string): string {
  const achados = texto.match(/[㐀-鿿]/g);
  return achados ? achados.join('') : '';
}

function contarLetrasPinyinClean(token: revisao.PalavraRevisao): number {
  if (!token.pinyin) return 0;
  const pLimpo = token.pinyin.split(/[,，;；/]/)[0].replace(/[\s'`’-]/g, '');
  return pLimpo.length;
}

export interface InputTokenStatus {
  texto: string;
  ehCorreto: boolean;
}

export interface ResultadoMatching {
  numTokensValidados: number;
  inputStatusTokens: InputTokenStatus[];
}

// Valida sequencialmente as palavras digitadas a partir do índice atual da frase.
export function conferirPrefixoMatching(
  entrada: string,
  tokens: revisao.PalavraRevisao[],
  startIdx: number
): ResultadoMatching {
  if (!entrada || startIdx >= tokens.length) {
    return { numTokensValidados: 0, inputStatusTokens: [] };
  }

  // 1. Tenta correspondência por palavras separadas por espaço (ou hanzi)
  const palavrasEntrada = entrada.split(/(\s+)/);
  const inputStatusTokens: InputTokenStatus[] = [];

  let tokenIndexActual = startIdx;
  let numValidados = 0;

  for (let i = 0; i < palavrasEntrada.length; i++) {
    const palavra = palavrasEntrada[i];

    if (/^\s+$/.test(palavra)) {
      inputStatusTokens.push({ texto: palavra, ehCorreto: false });
      continue;
    }

    if (!palavra.trim()) continue;

    if (tokenIndexActual < tokens.length) {
      const tokenAlvo = tokens[tokenIndexActual];
      let casou = false;

      if (contemHanzi(palavra)) {
        casou = apenasHanzis(palavra) === apenasHanzis(tokenAlvo.texto);
      } else {
        const compactaInput = compactarPinyin(palavra);
        const aceitas = leiturasAceitasToken(tokenAlvo);
        casou = compactaInput !== '' && aceitas.includes(compactaInput);
      }

      if (casou) {
        numValidados++;
        tokenIndexActual++;
        inputStatusTokens.push({ texto: palavra, ehCorreto: true });
        continue;
      }
    }

    inputStatusTokens.push({ texto: palavra, ehCorreto: false });
  }

  if (numValidados > 0) {
    // Mantém o espaço entre palavras validadas verde no input
    for (let k = 0; k < inputStatusTokens.length; k++) {
      if (inputStatusTokens[k].ehCorreto) {
        if (k > 0 && /^\s+$/.test(inputStatusTokens[k - 1].texto)) {
          inputStatusTokens[k - 1].ehCorreto = true;
        }
      }
    }

    return {
      numTokensValidados: numValidados,
      inputStatusTokens
    };
  }

  // 2. Fallback: Correspondência de pinyin contínuo sem espaços (ex: "nihao" ou "nihaoma")
  if (entrada.trim() && !contemHanzi(entrada.trim())) {
    const entradaCompactaTotal = compactarPinyin(entrada.trim());
    if (entradaCompactaTotal) {
      let pinyinAcumulado = '';
      let maiorMatchIndex = -1;

      for (let j = startIdx; j < tokens.length; j++) {
        const aceitas = leiturasAceitasToken(tokens[j]);
        if (aceitas.length === 0) break;
        pinyinAcumulado += aceitas[0];

        if (entradaCompactaTotal === pinyinAcumulado || entradaCompactaTotal.startsWith(pinyinAcumulado)) {
          maiorMatchIndex = j;
        } else {
          break;
        }
      }

      if (maiorMatchIndex >= startIdx) {
        const validadosContinuo = (maiorMatchIndex - startIdx) + 1;
        let tamanhoMatch = 0;
        for (let m = startIdx; m <= maiorMatchIndex; m++) {
          const aceitas = leiturasAceitasToken(tokens[m]);
          tamanhoMatch += aceitas[0] ? aceitas[0].length : 0;
        }

        let charsAcumulados = 0;
        let splitPos = entrada.length;
        for (let idxChar = 0; idxChar < entrada.length; idxChar++) {
          if (/[a-zA-Z1-5]/.test(entrada[idxChar])) {
            charsAcumulados++;
          }
          if (charsAcumulados === tamanhoMatch) {
            splitPos = idxChar + 1;
            break;
          }
        }

        const parteVerde = entrada.slice(0, splitPos);
        const parteRestante = entrada.slice(splitPos);

        const statusTokensContinuo: InputTokenStatus[] = [
          { texto: parteVerde, ehCorreto: true }
        ];
        if (parteRestante) {
          statusTokensContinuo.push({ texto: parteRestante, ehCorreto: false });
        }

        return {
          numTokensValidados: validadosContinuo,
          inputStatusTokens: statusTokensContinuo
        };
      }
    }
  }

  return {
    numTokensValidados: 0,
    inputStatusTokens: [{ texto: entrada, ehCorreto: false }]
  };
}

// ----- Seção: Componente Principal -----

export function FilaPinyin({
  questao,
  respondida,
  acertou,
  aoConcluir,
  aoTocarAudio,
  hanziTocando,
  hanziSintetizando,
  AoClicarNoCartao
}: FilaPinyinProps) {
  const tokens = (questao.fraseOriginalSegmentada || []).filter((t: revisao.PalavraRevisao) => t.ehChines && t.texto);

  const [indiceAtual, setIndiceAtual] = useState(0);
  const [entrada, setEntrada] = useState('');
  const [dicaReveladaLetrasCount, setDicaReveladaLetrasCount] = useState(0);
  const [concluidos, setConcluidos] = useState<boolean[]>(new Array(tokens.length).fill(false));
  const [feedbackIncorreto, setFeedbackIncorreto] = useState(false);
  const inputRef = useRef<HTMLInputElement>(null);
  const backdropRef = useRef<HTMLDivElement>(null);

  // Analisa o texto digitado atualmente para aplicar a cor verde e avançar o índice efetivo em tempo real
  const matchingAtual = conferirPrefixoMatching(entrada, tokens, indiceAtual);
  const indiceEfetivo = indiceAtual + matchingAtual.numTokensValidados;

  // Sincroniza a rolagem horizontal do backdrop com a caixa de texto
  function aoRolarInput(e: React.UIEvent<HTMLInputElement>) {
    if (backdropRef.current) {
      backdropRef.current.scrollLeft = e.currentTarget.scrollLeft;
    }
  }

  // Sincroniza a posição da rolagem sempre que a entrada muda
  useEffect(() => {
    if (inputRef.current && backdropRef.current) {
      backdropRef.current.scrollLeft = inputRef.current.scrollLeft;
    }
  }, [entrada]);

  // Reinicia o estado a cada nova questão
  useEffect(() => {
    setIndiceAtual(0);
    setEntrada('');
    setDicaReveladaLetrasCount(0);
    setConcluidos(new Array(tokens.length).fill(false));
    setFeedbackIncorreto(false);
  }, [questao]);

  // Ao avançar o índice efetivo (seja por envio ou por digitar a palavra correta live no input), reseta a contagem de dica
  useEffect(() => {
    setDicaReveladaLetrasCount(0);
  }, [indiceEfetivo]);

  // Foco no campo de texto ao carregar ou avançar
  useEffect(() => {
    if (!respondida) {
      const timerFoco = setTimeout(() => inputRef.current?.focus(), 50);
      return () => clearTimeout(timerFoco);
    }
  }, [indiceAtual, respondida]);

  // Total de letras pendentes a partir do índice efetivo atual (primeira palavra não escrita nem enviada)
  const totalLetrasPendentes = tokens.reduce((acc: number, t: revisao.PalavraRevisao, idx: number) => {
    if (idx < indiceEfetivo || concluidos[idx]) return acc;
    return acc + contarLetrasPinyinClean(t);
  }, 0);

  // Timer para revelar gradualmente os caracteres do pinyin a partir da palavra efetivamente pendente (a cada 2.0s)
  useEffect(() => {
    if (respondida || indiceEfetivo >= tokens.length || totalLetrasPendentes === 0) return;
    if (dicaReveladaLetrasCount >= totalLetrasPendentes) return;

    const timerDica = setInterval(() => {
      setDicaReveladaLetrasCount(prev => prev + 1);
    }, 2000);

    return () => clearInterval(timerDica);
  }, [indiceEfetivo, dicaReveladaLetrasCount, totalLetrasPendentes, respondida]);

  // Executa a validação ao submeter
  function validarSubmissao() {
    if (respondida || indiceAtual >= tokens.length) return;

    const limpa = entrada.trim();
    if (!limpa) return;

    const resultado = conferirPrefixoMatching(entrada, tokens, indiceAtual);

    if (resultado.numTokensValidados > 0) {
      const novosConcluidos = [...concluidos];
      for (let k = 0; k < resultado.numTokensValidados; k++) {
        const idxConcluido = indiceAtual + k;
        novosConcluidos[idxConcluido] = true;
      }
      setConcluidos(novosConcluidos);
      setFeedbackIncorreto(false);

      const novoIndice = indiceAtual + resultado.numTokensValidados;
      setIndiceAtual(novoIndice);
      setEntrada('');
      setDicaReveladaLetrasCount(0);

      if (novoIndice >= tokens.length) {
        aoConcluir(true);
      } else {
        tocarSomAcerto();
      }
    } else {
      setFeedbackIncorreto(true);
      setTimeout(() => setFeedbackIncorreto(false), 1200);
    }
  }

  function aoTeclar(e: React.KeyboardEvent<HTMLInputElement>) {
    if (e.key === 'Enter') {
      e.preventDefault();
      e.stopPropagation();
      validarSubmissao();
    }
  }

  // Renderiza o pinyin censurado ou revelado de um token específico
  function renderizarPinyinDoToken(token: revisao.PalavraRevisao, tokenIdx: number) {
    const pinyinOriginal = token.pinyin ? token.pinyin.split(/[,，;；/]/)[0].trim() : '';
    if (!pinyinOriginal) return null;

    // Se a palavra já foi concluída, respondida ou é anterior ao índice efetivo (escrita live no input): exibe o pinyin verde completo
    const ehConcluidoOuLive = concluidos[tokenIdx] || respondida || tokenIdx < indiceEfetivo;
    if (ehConcluidoOuLive) {
      return <span className="fila-token-pinyin-concluido">{pinyinOriginal}</span>;
    }

    // Soma a quantidade de letras pendentes dos tokens não concluídos entre indiceEfetivo e tokenIdx
    let letrasPendentesAntes = 0;
    for (let k = indiceEfetivo; k < tokenIdx; k++) {
      if (!concluidos[k]) {
        letrasPendentesAntes += contarLetrasPinyinClean(tokens[k]);
      }
    }

    let contadorLetrasToken = 0;
    return [...pinyinOriginal].map((char, charIdx) => {
      const ehSimbolo = /[\s'`’-]/.test(char);
      if (ehSimbolo) {
        return <span key={charIdx} className="fila-pinyin-char espaco">{char}</span>;
      }

      const indiceAbsolutoLetra = letrasPendentesAntes + contadorLetrasToken;
      contadorLetrasToken++;
      const revelado = indiceAbsolutoLetra < dicaReveladaLetrasCount;

      return (
        <span key={charIdx} className={`fila-pinyin-char ${revelado ? 'revelado' : 'oculto'}`}>
          {revelado ? char : '_'}
        </span>
      );
    });
  }

  return (
    <div className="revisao-fila-pinyin-container">
      {/* Tradução da Frase */}
      {questao.fraseTraducao && (
        <div className="revisao-fila-traducao">
          <span>{questao.fraseTraducao}</span>
        </div>
      )}

      {/* Frase Inteira no Padrão das Outras Atividades */}
      <div className="revisao-frase" style={{ textAlign: 'center', margin: '16px 0' }}>
        <div style={{ display: 'flex', gap: '10px', alignItems: 'center', justifyContent: 'center' }}>
          <BotaoAudio
            rotulo=""
            tocando={hanziTocando === questao.fraseOriginal}
            carregando={hanziSintetizando === questao.fraseOriginal}
            aoClicar={() => aoTocarAudio(questao.fraseOriginal)}
          />
          <div className="revisao-frase-tokens-container">
            {tokens.map((token: revisao.PalavraRevisao, idx: number) => {
              const ehConcluidoOuLive = concluidos[idx] || respondida || idx < indiceEfetivo;
              const ehAtivo = idx === indiceEfetivo && !respondida && !ehConcluidoOuLive;

              return (
                <div
                  key={idx}
                  className={`revisao-frase-token-bloco ${ehConcluidoOuLive ? 'concluido' : ''} ${ehAtivo ? 'ativo' : ''}`}
                  onClick={() => {
                    if (AoClicarNoCartao) {
                      AoClicarNoCartao({ Hanzi: token.texto, Pinyin: token.pinyin, significados: token.significados });
                    }
                  }}
                >
                  <div className="revisao-frase-token-pinyin">
                    {renderizarPinyinDoToken(token, idx)}
                  </div>
                  <div className="revisao-frase-token-hanzi">
                    {token.texto}
                  </div>
                </div>
              );
            })}
          </div>
        </div>
      </div>

      {/* Caixa de Texto com Destaque em Tempo Real e Sincronia de Cursor */}
      {!respondida && (
        <div className="revisao-fila-painel-ativo">
          <div className={`revisao-rich-input-wrapper ${feedbackIncorreto ? 'incorreto' : ''}`}>
            <div ref={backdropRef} className="revisao-rich-input-backdrop">
              {matchingAtual.inputStatusTokens.map((item, idx) => (
                <span key={idx} className={item.ehCorreto ? 'texto-correto-verde' : 'texto-normal'}>
                  {item.texto}
                </span>
              ))}
            </div>
            <input
              ref={inputRef}
              className="revisao-rich-input-real"
              value={entrada}
              onChange={e => setEntrada(e.target.value)}
              onScroll={aoRolarInput}
              onKeyDown={aoTeclar}
              placeholder={t('Digite o pinyin ou o hanzi…')}
              autoComplete="off"
              autoCorrect="off"
              spellCheck={false}
              lang="zh"
            />
          </div>

          <div style={{ display: 'flex', gap: '10px', width: '100%', maxWidth: '440px', justifyContent: 'center' }}>
            <button
              className="revisao-teclado-enviar"
              onClick={validarSubmissao}
              disabled={!entrada.trim()}
            >
              {t('Confirmar')}
            </button>
            {dicaReveladaLetrasCount >= totalLetrasPendentes && totalLetrasPendentes > 0 && (
              <button
                className="revisao-fila-pular-btn"
                onClick={() => {
                  const novos = [...concluidos];
                  novos[indiceAtual] = true;
                  setConcluidos(novos);
                  const prox = indiceAtual + 1;
                  setIndiceAtual(prox);
                  setEntrada('');
                  setDicaReveladaLetrasCount(0);
                  if (prox >= tokens.length) aoConcluir(true);
                }}
              >
                {t('Avançar ➔')}
              </button>
            )}
          </div>

          {feedbackIncorreto && (
            <div className="revisao-fila-feedback-incorreto">
              {t('Tente novamente ou aguarde a dica de pinyin…')}
            </div>
          )}
        </div>
      )}

      {/* Feedback de Conclusão */}
      {respondida && (
        <div className="revisao-fila-sucesso">
          <div className="revisao-fila-sucesso-texto">
            ✨ {t('Frase concluída com sucesso!')}
          </div>
        </div>
      )}
    </div>
  );
}
