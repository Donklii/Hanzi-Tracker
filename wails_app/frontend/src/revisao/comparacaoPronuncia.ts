// ----- Comparação da fala do usuário (STT) com o alvo das atividades de pronúncia -----
// A transcrição do STT chega em hanzi e é decomposta em tokens com pinyin do dicionário
// (DecomporTextoRevisao). A comparação acontece sílaba a sílaba, nunca por substring de texto:
//   - polifônicos valem por qualquer leitura: o dicionário devolve "le, liǎo" para 了 e falar
//     "le" OU "liǎo" acerta — tanto no alvo quanto no que foi transcrito;
//   - se a transcrição contém o próprio hanzi do alvo, é acerto imediato (o STT já julgou o
//     áudio ao escolher o caractere);
//   - pares nebulosos clássicos (z/zh, c/ch, s/sh, n/l, f/h, -n/-ng) são tolerados, pois STT
//     e sotaques os confundem com frequência;
//   - tons são ignorados: a escolha de caractere do STT não preserva com confiança o tom que
//     o usuário de fato falou.

export interface TokenFalado {
  texto: string;
  pinyin: string;
  ehChines: boolean;
}

export interface AlvoPronuncia {
  hanzi: string;
  pinyin: string;
}

// Minúsculas, sem marcas de tom e só letras; preserva o trema (U+0308 após NFD) como "v"
// para não colidir lü/lu.
function normalizarSilaba(bruta: string): string {
  return bruta
    .toLowerCase()
    .normalize('NFD')
    .replace(/ü/g, 'v')
    .replace(/[̀-ͯ]/g, '')
    .replace(/[^a-z]/g, '');
}

// Ordem importa: dígrafos (zh/ch/sh) antes das iniciais de uma letra.
const INICIAIS = [
  'zh', 'ch', 'sh',
  'b', 'p', 'm', 'f', 'd', 't', 'n', 'l', 'g', 'k', 'h',
  'j', 'q', 'x', 'r', 'z', 'c', 's', 'y', 'w',
];

function decomporSilaba(silaba: string): { inicial: string; final: string } {
  for (const inicial of INICIAIS) {
    if (silaba.startsWith(inicial)) {
      return { inicial, final: silaba.slice(inicial.length) };
    }
  }
  return { inicial: '', final: silaba };
}

// Pares nebulosos de iniciais (mesma chave canônica = equivalentes na comparação).
const INICIAIS_EQUIVALENTES: Record<string, string> = {
  zh: 'z',
  ch: 'c',
  sh: 's',
  l: 'n',
  f: 'h',
};

function canonizarSilaba(silaba: string): string {
  const { inicial, final } = decomporSilaba(silaba);
  const inicialCanonica = INICIAIS_EQUIVALENTES[inicial] ?? inicial;
  const finalCanonica = final.replace(/ng$/, 'n'); // an/ang, en/eng, in/ing…
  return inicialCanonica + finalCanonica;
}

function silabasCasam(alvo: string, falada: string): boolean {
  if (alvo === falada) return true;
  return canonizarSilaba(alvo) === canonizarSilaba(falada);
}

// Divide o campo de pinyin do dicionário em leituras alternativas ("le, liǎo" → [[le],[liao]]);
// cada leitura vira a lista de sílabas normalizadas ("kè qi" → [[ke, qi]]).
export function extrairLeituras(pinyin: string): string[][] {
  if (!pinyin) return [];
  const leituras: string[][] = [];
  for (const trecho of pinyin.split(/[,，;；/]/)) {
    const silabas = trecho.trim().split(/\s+/).map(normalizarSilaba).filter(Boolean);
    if (silabas.length > 0) {
      leituras.push(silabas);
    }
  }
  return leituras;
}

// Converte os tokens transcritos em "slots" de sílaba na ordem falada. Cada slot carrega as
// sílabas alternativas daquele token naquela posição (polifônicos geram mais de uma opção).
function silabasFaladas(tokens: TokenFalado[]): string[][] {
  const slots: string[][] = [];
  for (const token of tokens) {
    if (!token.ehChines) continue;
    const leituras = extrairLeituras(token.pinyin);
    if (leituras.length === 0) continue;

    // Leituras com contagem de sílabas diferente da primeira não têm posição definida no slot
    const tamanho = leituras[0].length;
    const alternativas = leituras.filter(l => l.length === tamanho);
    for (let i = 0; i < tamanho; i++) {
      slots.push([...new Set(alternativas.map(l => l[i]))]);
    }
  }
  return slots;
}

// Diz se o alvo (palavra/caractere com pinyin do dicionário) foi pronunciado em algum trecho
// da transcrição: acerto direto pelo hanzi, ou qualquer leitura do alvo encontrada como
// sequência contígua de sílabas na fala.
export function pronunciaCasa(alvo: AlvoPronuncia, tokensFalados: TokenFalado[]): boolean {
  if (tokensFalados.length === 0) return false;

  const textoFalado = tokensFalados.map(t => t.texto).join('');
  if (alvo.hanzi && textoFalado.includes(alvo.hanzi)) {
    return true;
  }

  const slots = silabasFaladas(tokensFalados);
  if (slots.length === 0) return false;

  for (const leitura of extrairLeituras(alvo.pinyin)) {
    for (let inicio = 0; inicio + leitura.length <= slots.length; inicio++) {
      let casou = true;
      for (let j = 0; j < leitura.length; j++) {
        if (!slots[inicio + j].some(falada => silabasCasam(leitura[j], falada))) {
          casou = false;
          break;
        }
      }
      if (casou) return true;
    }
  }
  return false;
}
