// ----- Seção: Configurações — licenças de terceiros (atribuições exigidas pelos autores) -----
//
// Registro do que cada origem exige em troca do uso dos dados. NÃO é uma lista de agradecimentos:
// cada campo aqui existe porque uma licença o exige, e sumir com um deles quebra a licença.
//
//   - Arphic PL §1: o ARPHICPL.TXT tem que ser retido INALTERADO em todas as cópias; §2(a) exige
//     avisar COMO e QUANDO mudamos.
//   - Aviso do Unicode (arquivo "LGPL" do Make Me a Hanzi), cláusula (c): aviso claro de que os
//     dados foram modificados, no software E na documentação.
//   - LGPL-3.0 §4(b)/(c): acompanhar a GPL-3.0 e, como o app exibe créditos em execução, incluir o
//     aviso de copyright junto deles e dizer onde ler as licenças.
//   - CC BY-SA 4.0 §3(a)(1): autor, aviso de copyright, licença, ISENÇÃO DE GARANTIA, URI da origem
//     e indicação das modificações.
//
// Os textos de licença são importados VERBATIM (`?raw`) e nunca passam por tradução — são o
// documento legal. Só `papel`, `derivacao` e `modificacoes` são texto nosso e traduzível.
import textoArphic from './textos/ARPHICPL.TXT?raw';
import textoMakemeahanziLgpl from './textos/makemeahanzi-LGPL.txt?raw';
import textoGpl from './textos/GPL-3.0.txt?raw';
import textoCcBySa from './textos/CC-BY-SA-4.0.txt?raw';
import textoCcBy20Fr from './textos/CC-BY-2.0-FR.txt?raw';

export interface TextoLicenca {
  rotulo: string;
  texto: string;
}

export interface Atribuicao {
  nome: string;
  papel: string; // o que fornece ao app (traduzível)
  copyright: string; // avisos de copyright — verbatim, nunca traduzidos
  derivacao: string; // de onde a origem tirou os dados (traduzível)
  licenca: string;
  origem: string;
  modificacoes: string; // como e quando alteramos (traduzível) — exigido por Arphic §2(a) e Unicode (c)
  textos: TextoLicenca[];
}

export const ATRIBUICOES: Atribuicao[] = [
  {
    nome: 'Make Me a Hanzi',
    papel: 'Definições, pinyin, decomposição, etimologia e radical dos caracteres.',
    copyright: 'Unihan: Copyright © 1991–2009 Unicode, Inc. Todos os direitos reservados. · CJKlib: Christoph Burgmer',
    derivacao: 'O arquivo dictionary.txt do Make Me a Hanzi, derivado das bases Unihan e CJKlib.',
    licenca: 'LGPL-3.0 ou posterior, com o aviso de copyright e permissão do Unicode',
    origem: 'https://github.com/skishore/makemeahanzi',
    modificacoes:
      'Modificado por nós: as definições e dicas foram traduzidas para o português em 2026-07-14 e os registros foram reestruturados em 2026-07-17, ao serem fundidos com o CC-CEDICT e a lista de frequência num único dicionário. Nessa fusão, caracteres exclusivamente tradicionais deixaram de ter registro próprio e passaram a constar como variantes do simplificado.',
    textos: [
      { rotulo: 'Aviso do Unicode + LGPL-3.0 (arquivo "LGPL" do Make Me a Hanzi)', texto: textoMakemeahanziLgpl },
      { rotulo: 'GNU GPL-3.0 (incorporada pela LGPL-3.0)', texto: textoGpl },
    ],
  },
  {
    nome: 'Make Me a Hanzi / Arphic — traçados',
    papel: 'Traços e animações de escrita dos caracteres.',
    copyright: 'Copyright © 1999 Arphic Technology Co., Ltd. Todos os direitos reservados, exceto onde a licença especifica.',
    derivacao:
      'As fontes AR PL KaitiM GB e AR PL UKai, da Arphic Technology, extraídas pelo Make Me a Hanzi (arquivo graphics.txt) e redistribuídas pelo projeto hanzi-writer-data.',
    licenca: 'Arphic Public License',
    origem: 'https://github.com/chanind/hanzi-writer-data',
    modificacoes:
      'Modificado por nós: os traçados foram convertidos para TSV comprimido em 2026-07-03, para o app carregá-los sob demanda. Os traços em si não foram alterados.',
    textos: [{ rotulo: 'Arphic Public License (ARPHICPL.TXT)', texto: textoArphic }],
  },
  {
    nome: 'CC-CEDICT',
    papel: 'Grafias tradicional e simplificada, pinyin e significados das palavras.',
    copyright: 'CC-CEDICT · MDBG',
    derivacao: 'O dicionário chinês-inglês colaborativo CC-CEDICT, mantido pelo MDBG.',
    licenca: 'CC BY-SA 4.0',
    origem: 'https://www.mdbg.net/chinese/dictionary?page=cc-cedict',
    modificacoes:
      'Modificado por nós: os significados foram traduzidos para o português em 2026-07-16 e os registros foram reestruturados em 2026-07-17, ao serem fundidos com o Make Me a Hanzi e a lista de frequência num único dicionário. As grafias e o pinyin não foram alterados.',
    textos: [{ rotulo: 'CC BY-SA 4.0', texto: textoCcBySa }],
  },
  {
    nome: 'FrequencyWords',
    papel: 'Frequência de uso das palavras — o percentual e a posição no ranking.',
    copyright: 'Hermit Dave',
    derivacao: 'A lista zh_cn do FrequencyWords, contada a partir do corpus de legendas OpenSubtitles 2018 (via OPUS).',
    licenca: 'CC BY-SA 4.0',
    origem: 'https://github.com/hermitdave/FrequencyWords',
    modificacoes:
      'A lista é distribuída aqui sem nenhuma alteração de conteúdo (apenas comprimida). Em 2026-07-17 passamos a derivar dela o percentual de uso e a posição no ranking de cada palavra, que é o que o app mostra.',
    textos: [{ rotulo: 'CC BY-SA 4.0', texto: textoCcBySa }],
  },
  {
    nome: 'Tatoeba',
    papel: 'Frases de exemplo em chinês, usadas na revisão por contexto e no desenho guiado por contexto.',
    copyright:
      'Contribuidores do Tatoeba (tatoeba.org) — obra coletiva de milhares de autores independentes. A atribuição de cada frase (autor da frase chinesa e, quando aplicável, do par original) é exibida junto dela na revisão, como a licença exige.',
    derivacao:
      'Despejos públicos do Tatoeba (cmn-eng e cmn-por). O acervo em inglês veio pré-casado de manythings.org/anki; o acervo em português foi casado por nós a partir dos despejos brutos (per_language/cmn e per_language/por) e do arquivo de links do próprio Tatoeba.',
    licenca: 'CC BY 2.0 France (Paternité 2.0 France)',
    origem: 'https://tatoeba.org',
    modificacoes:
      'Modificado por nós: os pares chinês–português foram montados em 2026-07-20 a partir dos despejos brutos (nenhum par pré-pronto existe para este idioma). Como as traduções do Tatoeba são colaborativas e não revisadas, cada par pt-BR foi julgado por um modelo de linguagem; as ~8% cuja tradução trocava o sentido da frase chinesa (114 de 1.398) tiveram o português refeito por tradução automática direta do chinês — a frase chinesa em si nunca foi alterada, e a atribuição de cada uma dessas frases marca a substituição.',
    textos: [{ rotulo: 'CC BY 2.0 France (Paternité 2.0 France)', texto: textoCcBy20Fr }],
  },
];

// AVISO_COMPARTILHA_IGUAL cobre o ShareAlike da CC BY-SA 4.0: o dicionário que o app embarca é obra
// derivada das três origens somadas, então precisa dizer sob que licença ele próprio sai.
export const AVISO_COMPARTILHA_IGUAL =
  'O dicionário embarcado neste app é uma obra derivada das origens acima somadas e é distribuído sob a CC BY-SA 4.0 e a LGPL-3.0. Os dados de traçado permanecem sob a Arphic Public License.';

// AVISO_SEM_GARANTIA cobre a isenção de garantia que a CC BY-SA 4.0 (§3(a)(1)(A)(iv)) e a Arphic PL
// (§7) exigem que acompanhe a atribuição.
export const AVISO_SEM_GARANTIA =
  'Estes dados são fornecidos "como estão", sem garantia de nenhum tipo. Os textos completos das licenças, abaixo, trazem a isenção de garantia e a limitação de responsabilidade de cada origem.';
