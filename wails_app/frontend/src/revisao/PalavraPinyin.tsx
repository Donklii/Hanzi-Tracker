import React, { useState, useEffect, useRef } from 'react';
import { main } from '../../wailsjs/go/models';
import { BotaoAudio } from './BotaoAudio';
import { extrairLeituras } from './comparacaoPronuncia';
import { tocarSomAcerto } from '../comum/sons';
import { t } from '../i18n/i18n';

// ----- Seção: Atividade Palavra Pinyin -----
// Variante "fonetica_palavra_pinyin" (Iniciante): Exibe uma única palavra em chinês.
// Acima da palavra, o pinyin é exibido censurado por `_ _ _` e revelado gradualmente a cada 2.0s
// (dica progressiva). O usuário digita o pinyin (ou hanzi) da palavra.
// Durante a digitação, o texto no input e o hanzi em destaque ficam verdes em tempo real (live highlighting).
// Não há falha irreversível; o envio correto conclui a questão com sucesso.

interface PalavraPinyinProps {
  questao: main.QuestaoRevisao;
  respondida: boolean;
  acertou: boolean | null;
  aoConcluir: (acertou: boolean) => void;
  aoTocarAudio: (texto: string) => void;
  hanziTocando: string | null;
  hanziSintetizando: string | null;
}

// ----- Seção: Auxiliares de Validação -----

function compactarPinyin(texto: string): string {
  return extrairLeituras(texto).reduce((acc, leitura) => acc + leitura.join(''), '');
}

function leiturasAceitasPinyin(pinyinBruto: string): string[] {
  if (!pinyinBruto) return [];
  const pinyins = pinyinBruto.split(/[,，;；/]/);
  const aceitas: string[] = [];
  for (const p of pinyins) {
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

// ----- Seção: Componente Principal -----

export function PalavraPinyin({
  questao,
  respondida,
  acertou,
  aoConcluir,
  aoTocarAudio,
  hanziTocando,
  hanziSintetizando
}: PalavraPinyinProps) {
  const pinyinLimpo = questao.pinyin ? questao.pinyin.split(/[,，;；/]/)[0].trim() : '';
  const pinyinSemSimbolos = pinyinLimpo.replace(/[\s'`’-]/g, '');

  const [entrada, setEntrada] = useState('');
  const [dicaReveladaCount, setDicaReveladaCount] = useState(0);
  const [feedbackIncorreto, setFeedbackIncorreto] = useState(false);
  const inputRef = useRef<HTMLInputElement>(null);
  const backdropRef = useRef<HTMLDivElement>(null);

  // Reinicia estado ao mudar de questão
  useEffect(() => {
    setEntrada('');
    setDicaReveladaCount(0);
    setFeedbackIncorreto(false);
  }, [questao]);

  // Foco no campo de texto ao carregar
  useEffect(() => {
    if (!respondida) {
      // Toca automaticamente o áudio da palavra ao entrar
      aoTocarAudio(questao.hanzi);
      const timerFoco = setTimeout(() => inputRef.current?.focus(), 50);
      return () => clearTimeout(timerFoco);
    }
  }, [questao, respondida]);

  // Timer para revelação gradual da dica de pinyin (a cada 2.0s revela +1 caractere)
  useEffect(() => {
    if (respondida || !pinyinSemSimbolos) return;
    if (dicaReveladaCount >= pinyinSemSimbolos.length) return;

    const timerDica = setInterval(() => {
      setDicaReveladaCount(prev => Math.min(prev + 1, pinyinSemSimbolos.length));
    }, 2000);

    return () => clearInterval(timerDica);
  }, [dicaReveladaCount, pinyinSemSimbolos, respondida]);

  // Sincroniza a rolagem horizontal do backdrop com a caixa de texto
  function aoRolarInput(e: React.UIEvent<HTMLInputElement>) {
    if (backdropRef.current) {
      backdropRef.current.scrollLeft = e.currentTarget.scrollLeft;
    }
  }

  useEffect(() => {
    if (inputRef.current && backdropRef.current) {
      backdropRef.current.scrollLeft = inputRef.current.scrollLeft;
    }
  }, [entrada]);

  // Checa se a entrada atual casa com a resposta (live matching)
  function testarLiveMatch(textoInput: string): boolean {
    const limpa = textoInput.trim();
    if (!limpa) return false;

    if (contemHanzi(limpa)) {
      return apenasHanzis(limpa) === apenasHanzis(questao.hanzi);
    }

    const compacta = compactarPinyin(limpa);
    const aceitas = leiturasAceitasPinyin(questao.pinyin);
    return compacta !== '' && aceitas.includes(compacta);
  }

  const ehLiveCorreto = testarLiveMatch(entrada);

  // Executa validação ao enviar
  function validarSubmissao() {
    if (respondida) return;

    if (ehLiveCorreto) {
      setFeedbackIncorreto(false);
      aoConcluir(true);
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

  // Renderiza o pinyin censurado com dica progressiva
  function renderizarPinyinCensurado() {
    if (!pinyinLimpo) return null;
    if (respondida || ehLiveCorreto) {
      return <span className="fila-token-pinyin-concluido">{pinyinLimpo}</span>;
    }

    let contadorLetra = 0;
    return [...pinyinLimpo].map((char, i) => {
      const ehSimbolo = /[\s'`’-]/.test(char);
      if (ehSimbolo) return <span key={i} className="fila-pinyin-char espaco">{char}</span>;

      const revelado = contadorLetra < dicaReveladaCount;
      contadorLetra++;

      return (
        <span key={i} className={`fila-pinyin-char ${revelado ? 'revelado' : 'oculto'}`}>
          {revelado ? char : '_'}
        </span>
      );
    });
  }

  return (
    <div className="revisao-fila-pinyin-container">
      {/* Exibição da Palavra Única em Destaque 100% Centralizada */}
      <div className="revisao-frase" style={{ textAlign: 'center', margin: '20px 0' }}>
        <div style={{ display: 'grid', gridTemplateColumns: '1fr auto 1fr', alignItems: 'center', width: '100%' }}>
          <div style={{ justifySelf: 'end', paddingRight: '12px' }}>
            <BotaoAudio
              rotulo=""
              tocando={hanziTocando === questao.hanzi}
              carregando={hanziSintetizando === questao.hanzi}
              aoClicar={() => aoTocarAudio(questao.hanzi)}
            />
          </div>
          <div style={{ justifySelf: 'center' }}>
            <div className={`revisao-frase-token-bloco ${respondida || ehLiveCorreto ? 'concluido' : 'ativo'}`}>
              <div className="revisao-frase-token-pinyin">
                {renderizarPinyinCensurado()}
              </div>
              <div className="revisao-frase-token-hanzi" style={{ fontSize: '44px' }}>
                {questao.hanzi}
              </div>
            </div>
          </div>
          <div style={{ justifySelf: 'start' }} />
        </div>
      </div>

      {/* Caixa de Entrada de Texto com Destaque em Tempo Real */}
      {!respondida && (
        <div className="revisao-fila-painel-ativo">
          <div className={`revisao-rich-input-wrapper ${feedbackIncorreto ? 'incorreto' : ''}`}>
            <div ref={backdropRef} className="revisao-rich-input-backdrop">
              <span className={ehLiveCorreto ? 'texto-correto-verde' : 'texto-normal'}>
                {entrada}
              </span>
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
            {dicaReveladaCount >= pinyinSemSimbolos.length && pinyinSemSimbolos.length > 0 && (
              <button
                className="revisao-fila-pular-btn"
                onClick={() => {
                  aoConcluir(true);
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
            ✨ {t('Palavra concluída com sucesso!')}
          </div>
        </div>
      )}
    </div>
  );
}
