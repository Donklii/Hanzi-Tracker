// ----- Comparação da resposta DIGITADA (modo teclado das atividades de revisão) -----
// Nas atividades de contexto (escrever o hanzi que falta), fonética-frase e ordenação o usuário
// pode optar por DIGITAR livremente a resposta em vez de montar peças ou escolher opções — análogo
// ao "✏️ Prefiro desenhar a resposta". Ele digita em HANZI ou em PINYIN, e a correção segue o que
// veio: havendo qualquer caractere Han, comparamos a sequência de hanzis (ignorando pontuação e
// espaços); caso contrário tratamos como pinyin e comparamos as sílabas SEM tom (digitar tom é
// inviável), reaproveitando o normalizador de sílabas do módulo de pronúncia.

import { extrairLeituras } from './comparacaoPronuncia';
import { revisao } from '../../wailsjs/go/models';

// Faixa de caracteres Han (URO + Extensão A) — cobre o vocabulário do app.
const REGEX_HAN = /[㐀-鿿]/;
const REGEX_HAN_GLOBAL = /[㐀-鿿]/g;

export function contemHanzi(texto: string): boolean {
  return REGEX_HAN.test(texto);
}

// Só os caracteres Han, na ordem (descarta pontuação, espaços e latim).
function apenasHanzis(texto: string): string {
  const achados = texto.match(REGEX_HAN_GLOBAL);
  return achados ? achados.join('') : '';
}

// Concatena todas as sílabas normalizadas (sem tom) de um texto num único bloco comparável.
// "wǒ hěn hǎo", "wo3 hen3 hao3" e "wohenhao" convergem todos para "wohenhao".
function compactarPinyin(texto: string): string {
  return extrairLeituras(texto).reduce((acc, leitura) => acc + leitura.join(''), '');
}

// Cada leitura possível de um campo de pinyin, compactada (polifônicos: "le, liǎo" → ["le","liao"]).
function leiturasCompactas(pinyin: string): string[] {
  return extrairLeituras(pinyin).map(leitura => leitura.join('')).filter(Boolean);
}

export type ModoResposta = 'hanzi' | 'pinyin' | 'vazio';

export interface ResultadoDigitado {
  acertou: boolean;
  modo: ModoResposta;
}

// Núcleo: compara a entrada contra a sequência de hanzis esperada e as leituras de pinyin aceitas.
function avaliar(entrada: string, alvoHanzis: string, pinyinsAceitos: string[]): ResultadoDigitado {
  const limpa = entrada.trim();
  if (!limpa) return { acertou: false, modo: 'vazio' };

  if (contemHanzi(limpa)) {
    return { acertou: apenasHanzis(limpa) === apenasHanzis(alvoHanzis), modo: 'hanzi' };
  }

  const compacta = compactarPinyin(limpa);
  return { acertou: compacta !== '' && pinyinsAceitos.includes(compacta), modo: 'pinyin' };
}

// Contexto (escrever 1 hanzi): compara com questao.hanzi ou com o pinyin do alvo (polifônicos OK).
export function avaliarRespostaContexto(entrada: string, questao: revisao.QuestaoRevisao): ResultadoDigitado {
  return avaliar(entrada, questao.hanzi, leiturasCompactas(questao.pinyin));
}

// Frase (fonética-frase / ordenação): compara com a sequência de hanzis da frase ou com o pinyin
// de todos os tokens chineses concatenado. Usa a leitura primária de cada token (antes de vírgula)
// para não explodir polifônicos em várias "leituras da frase".
export function avaliarRespostaFrase(entrada: string, questao: revisao.QuestaoRevisao): ResultadoDigitado {
  const tokens = (questao.fraseOriginalSegmentada || []).filter((t: revisao.PalavraRevisao) => t.ehChines);
  const alvoHanzis = tokens.length ? tokens.map((t: revisao.PalavraRevisao) => t.texto).join('') : (questao.fraseOriginal || '');
  const pinyinFrase = tokens
    .map((t: revisao.PalavraRevisao) => (t.pinyin || '').split(/[,，;；/]/)[0])
    .join(' ');
  return avaliar(entrada, alvoHanzis, [compactarPinyin(pinyinFrase)].filter(Boolean));
}
