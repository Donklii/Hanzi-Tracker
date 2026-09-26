package dicionario

import "strings"

// ----- Pinyin: conversão numérica→acentuada, remoção de tons e normalização para busca -----
//
// O CC-CEDICT grava o pinyin com tom NUMÉRICO ("hao3"); o app exibe acentuado ("hǎo"). A fusão já
// converte na geração (ConverterPinyin), então o arquivo fundido chega aqui acentuado — as funções
// abaixo servem à BUSCA (normalizar o que o usuário digita) e seguem exportadas porque a fusão
// também as usa.

// vogaisAcentuadasPorTom mapeia a vogal base para as suas formas por tom, na ordem
// neutro/1º/2º/3º/4º — o índice é o próprio dígito de tom do CEDICT (5 = neutro, tratado como 0).
var vogaisAcentuadasPorTom = map[rune][]rune{
	'a': {'a', 'ā', 'á', 'ǎ', 'à'},
	'e': {'e', 'ē', 'é', 'ě', 'è'},
	'i': {'i', 'ī', 'í', 'ǐ', 'ì'},
	'o': {'o', 'ō', 'ó', 'ǒ', 'ò'},
	'u': {'u', 'ū', 'ú', 'ǔ', 'ù'},
	'v': {'ü', 'ǖ', 'ǘ', 'ǚ', 'ǜ'},
}

// vogaisSemTom desfaz o acento de tom, e ainda normaliza ü→u e v→u (o CEDICT escreve "u:" para ü, e
// quem digita costuma usar "v"). É o que permite casar "nu" com "nǚ" numa busca.
var vogaisSemTom = map[rune]rune{
	'ā': 'a', 'á': 'a', 'ǎ': 'a', 'à': 'a',
	'ē': 'e', 'é': 'e', 'ě': 'e', 'è': 'e',
	'ī': 'i', 'í': 'i', 'ǐ': 'i', 'ì': 'i',
	'ō': 'o', 'ó': 'o', 'ǒ': 'o', 'ò': 'o',
	'ū': 'u', 'ú': 'u', 'ǔ': 'u', 'ù': 'u',
	'ü': 'u', 'ǖ': 'u', 'ǘ': 'u', 'ǚ': 'u', 'ǜ': 'u',
	'v': 'u',
}


// ConverterPinyin transforma o pinyin de tom numérico do CEDICT ("hao3 chi1") no acentuado
// ("hǎo chī"). Usada pela fusão na geração do dicionário — o app já lê o resultado pronto.
func ConverterPinyin(pinyinNumerado string) string {
	silabas := strings.Split(pinyinNumerado, " ")

	for i, silaba := range silabas {
		silaba = strings.ReplaceAll(silaba, "u:", "v")
		if len(silaba) == 0 {
			continue
		}

		tom := silaba[len(silaba)-1]
		if tom < '1' || tom > '5' {
			silabas[i] = strings.ReplaceAll(silaba, "v", "ü")
			continue
		}

		indiceTom := int(tom - '0')
		if indiceTom == 5 {
			indiceTom = 0 // tom neutro usa a vogal sem acento
		}
		corpo := silaba[:len(silaba)-1]

		vogal := vogalAcentuavel(corpo)
		if vogal != 0 {
			corpo = strings.Replace(corpo, string(vogal), string(vogaisAcentuadasPorTom[vogal][indiceTom]), 1)
		}
		silabas[i] = corpo
	}

	return strings.Join(silabas, " ")
}


// RemoverTonsPinyin devolve o pinyin sem acento de tom (e com ü/v normalizados para u).
func RemoverTonsPinyin(pinyin string) string {
	var construtor strings.Builder
	construtor.Grow(len(pinyin))

	for _, r := range pinyin {
		if semTom, existe := vogaisSemTom[r]; existe {
			construtor.WriteRune(semTom)
			continue
		}
		construtor.WriteRune(r)
	}
	return construtor.String()
}

// ----- Utilitários -----

// vogalAcentuavel devolve qual vogal da sílaba recebe o acento de tom, pela regra do pinyin: 'a' e
// 'e' sempre ganham; em "ou" o acento vai no 'o'; senão vai na ÚLTIMA vogal. Devolve 0 se não houver
// vogal (ex.: "n", "ng" — interjeições do CEDICT).
func vogalAcentuavel(silaba string) rune {
	if strings.ContainsRune(silaba, 'a') {
		return 'a'
	}
	if strings.ContainsRune(silaba, 'e') {
		return 'e'
	}
	if strings.Contains(silaba, "ou") {
		return 'o'
	}

	for i := len(silaba) - 1; i >= 0; i-- {
		if _, ehVogal := vogaisAcentuadasPorTom[rune(silaba[i])]; ehVogal {
			return rune(silaba[i])
		}
	}
	return 0
}


// limparPinyinParaBusca normaliza pinyin para comparação: minúsculo, sem acento de tom, sem os
// dígitos de tom e sem espaços. É o formato das chaves do índice reverso de pinyin, e o ÚNICO ponto
// de normalização — indexar e buscar têm que passar pela mesma função, senão o índice fica gravado
// numa forma ("hao") e consultado noutra ("hǎo"), e nada casa.
//
// Absorve as três grafias que chegam aqui: a acentuada do dicionário fundido (hǎo), a numerada do
// CEDICT cru (hao3) e a que o usuário digita (hao).
func limparPinyinParaBusca(pinyin string) string {
	pinyin = RemoverTonsPinyin(strings.ToLower(pinyin))
	for _, remover := range []string{"1", "2", "3", "4", "5", " "} {
		pinyin = strings.ReplaceAll(pinyin, remover, "")
	}
	return pinyin
}
