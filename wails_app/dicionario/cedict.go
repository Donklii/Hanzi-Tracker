package dicionario

import (
	"bufio"
	"bytes"
	_ "embed"
	"fmt"
	"strings"
)

//go:embed cedict_ts.u8
var cedictBytes []byte

type EntradaDicionario struct {
	Tradicional string
	Simplificado string
	Pinyin       string
	Significados []string
}

type Cedict struct {
	entradas      map[string][]EntradaDicionario
	pinyinIndex   map[string][]string
	simpParaTrad  map[rune]rune // simplificado → tradicional (caracteres que diferem)
	tradParaSimp  map[rune]rune // tradicional → simplificado (caracteres que diferem)
}

func NovoCedict() *Cedict {
	return &Cedict{
		entradas:     make(map[string][]EntradaDicionario),
		pinyinIndex:  make(map[string][]string),
		simpParaTrad: make(map[rune]rune),
		tradParaSimp: make(map[rune]rune),
	}
}

// Carregar lê o arquivo CC-CEDICT embarcado e popula o mapa em memória
func (c *Cedict) Carregar() error {
	if len(cedictBytes) == 0 {
		return fmt.Errorf("dicionário não foi embarcado corretamente")
	}

	scanner := bufio.NewScanner(bytes.NewReader(cedictBytes))
	for scanner.Scan() {
		linha := scanner.Text()
		if strings.HasPrefix(linha, "#") || strings.TrimSpace(linha) == "" {
			continue
		}

		// Example CEDICT format:
		// Tradicional Simplificado [pin1 yin1] /significado 1/significado 2/
		parts := strings.SplitN(linha, " [", 2)
		if len(parts) != 2 {
			continue
		}

		palavras := strings.Fields(parts[0])
		if len(palavras) != 2 {
			continue
		}
		tradicional, simplificado := palavras[0], palavras[1]

		// Monta mapas de conversão caractere-a-caractere (simplificado↔tradicional)
		if tradicional != simplificado {
			runesT := []rune(tradicional)
			runesS := []rune(simplificado)
			if len(runesT) == len(runesS) {
				for k := 0; k < len(runesT); k++ {
					if runesT[k] != runesS[k] {
						c.simpParaTrad[runesS[k]] = runesT[k]
						c.tradParaSimp[runesT[k]] = runesS[k]
					}
				}
			}
		}

		parts2 := strings.SplitN(parts[1], "] /", 2)
		if len(parts2) != 2 {
			continue
		}
		pinyinStr := parts2[0]
		significadosStr := strings.TrimSuffix(parts2[1], "/")
		significados := strings.Split(significadosStr, "/")

		entrada := EntradaDicionario{
			Tradicional:  tradicional,
			Simplificado: simplificado,
			Pinyin:       ConverterPinyin(pinyinStr),
			Significados: significados,
		}

		// Indexa tanto pelo tradicional quanto simplificado
		c.entradas[simplificado] = append(c.entradas[simplificado], entrada)
		if tradicional != simplificado {
			c.entradas[tradicional] = append(c.entradas[tradicional], entrada)
		}
        
		// Limpa o pinyin para busca reversa (remove tons e espaços)
		cleanPinyin := strings.ToLower(pinyinStr)
		for _, num := range []string{"1", "2", "3", "4", "5", " "} {
			cleanPinyin = strings.ReplaceAll(cleanPinyin, num, "")
		}
		
		// Adiciona no índice de pinyin se ainda não estiver presente para esse hanzi
		if cleanPinyin != "" {
			encontrado := false
			for _, h := range c.pinyinIndex[cleanPinyin] {
				if h == simplificado {
					encontrado = true
					break
				}
			}
			if !encontrado {
				c.pinyinIndex[cleanPinyin] = append(c.pinyinIndex[cleanPinyin], simplificado)
			}
		}
	}

	return scanner.Err()
}

func (c *Cedict) Buscar(palavra string) []EntradaDicionario {
	return c.entradas[palavra]
}

// TodasPalavras devolve todas as formas escritas indexadas (simplificado e tradicional), sem
// repetição. É a fonte de hanzis do pré-carregamento do cache de TTS (ver tts_precache.go): cada
// forma vira uma síntese de fala.
func (c *Cedict) TodasPalavras() []string {
	palavras := make([]string, 0, len(c.entradas))
	for palavra := range c.entradas {
		palavras = append(palavras, palavra)
	}
	return palavras
}

// BuscarPorPinyin retorna uma lista de Hanzis simplificados que correspondem ao pinyin dado (sem tom/espaço)
func (c *Cedict) BuscarPorPinyin(pinyin string) []string {
    cleanPinyin := strings.ToLower(pinyin)
    for _, num := range []string{"1", "2", "3", "4", "5", " "} {
        cleanPinyin = strings.ReplaceAll(cleanPinyin, num, "")
    }
    return c.pinyinIndex[cleanPinyin]
}

// BuscarGeral pesquisa em todo o dicionário por hanzi, pinyin ou significado (limite de 50)
func (c *Cedict) BuscarGeral(termo string) []EntradaDicionario {
	if termo == "" {
		return nil
	}
	termoLower := strings.ToLower(termo)
	cleanTermo := termoLower
	for _, num := range []string{"1", "2", "3", "4", "5", " "} {
		cleanTermo = strings.ReplaceAll(cleanTermo, num, "")
	}

	var priority []EntradaDicionario
	var secondary []EntradaDicionario
	vistos := make(map[string]bool)

varredura:
	for _, entradas := range c.entradas {
		for _, e := range entradas {
			isPriority := false
			isSecondary := false

			// Hanzi
			if strings.Contains(e.Simplificado, termo) || strings.Contains(e.Tradicional, termo) {
				isPriority = true
			}
			// Pinyin
			if !isPriority {
				pinyinClean := RemoverTonsPinyin(strings.ToLower(e.Pinyin))
				pinyinClean = strings.ReplaceAll(pinyinClean, " ", "")
				if strings.Contains(pinyinClean, cleanTermo) {
					isPriority = true
				}
			}
			// Significado
			if !isPriority {
				for _, sig := range e.Significados {
					if strings.Contains(strings.ToLower(sig), termoLower) {
						isSecondary = true
						break
					}
				}
			}

			if isPriority || isSecondary {
				if !vistos[e.Simplificado] {
					vistos[e.Simplificado] = true
					if isPriority {
						priority = append(priority, e)
						if len(priority) >= 3000 {
							break varredura
						}
					} else {
						secondary = append(secondary, e)
					}
				}
			}
		}
	}

	var resultados []EntradaDicionario
	resultados = append(resultados, priority...)
	
	if len(resultados) < 3000 {
		for _, s := range secondary {
			resultados = append(resultados, s)
			if len(resultados) >= 3000 {
				break
			}
		}
	} else if len(resultados) > 3000 {
		resultados = resultados[:3000]
	}

	return resultados
}

// AvaliarTipoHanzi determina se um caractere é Tradicional, Simplificado ou Ambos
func (c *Cedict) AvaliarTipoHanzi(hanzi string) string {
	// Guard clause: dicionário ausente (ex.: App de teste sem o CEDICT carregado).
	if c == nil {
		return ""
	}

	entradas := c.entradas[hanzi]
	if len(entradas) == 0 {
		return ""
	}
	e := entradas[0]
	if e.Simplificado == hanzi && e.Tradicional == hanzi {
		return "Ambos"
	} else if e.Simplificado == hanzi {
		return "Simplificado"
	} else if e.Tradicional == hanzi {
		return "Tradicional"
	}
	return ""
}

// ConverterTexto converte um texto para simplificado ou tradicional, caractere a caractere,
// usando os mapas construídos a partir do CEDICT. Caracteres sem mapeamento (pontuação, caracteres
// comuns a ambas as formas) passam inalterados. O parâmetro alvo deve ser "simplificado" ou "tradicional".
func (c *Cedict) ConverterTexto(texto string, alvo string) string {
	if c == nil {
		return texto
	}
	var mapa map[rune]rune
	switch alvo {
	case "simplificado":
		mapa = c.tradParaSimp
	case "tradicional":
		mapa = c.simpParaTrad
	default:
		return texto
	}
	if len(mapa) == 0 {
		return texto
	}

	var sb strings.Builder
	sb.Grow(len(texto))
	for _, r := range texto {
		if convertido, ok := mapa[r]; ok {
			sb.WriteRune(convertido)
		} else {
			sb.WriteRune(r)
		}
	}
	return sb.String()
}

// FraseContemTipo verifica se uma frase contém caracteres exclusivamente do tipo oposto ao desejado.
// Retorna true se a frase for compatível com o tipo alvo (i.e., não contém caracteres do tipo oposto).
func (c *Cedict) FraseCompativelComTipo(frase string, tipoDesejado string) bool {
	if c == nil || tipoDesejado == "ambos" || tipoDesejado == "" {
		return true
	}
	for _, r := range frase {
		tipo := c.AvaliarTipoHanzi(string(r))
		if tipoDesejado == "simplificado" && tipo == "Tradicional" {
			return false
		}
		if tipoDesejado == "tradicional" && tipo == "Simplificado" {
			return false
		}
	}
	return true
}

func RemoverTonsPinyin(p string) string {
	replacements := map[rune]rune{
		'ā': 'a', 'á': 'a', 'ǎ': 'a', 'à': 'a',
		'ē': 'e', 'é': 'e', 'ě': 'e', 'è': 'e',
		'ī': 'i', 'í': 'i', 'ǐ': 'i', 'ì': 'i',
		'ō': 'o', 'ó': 'o', 'ǒ': 'o', 'ò': 'o',
		'ū': 'u', 'ú': 'u', 'ǔ': 'u', 'ù': 'u',
		'ü': 'u', 'ǖ': 'u', 'ǘ': 'u', 'ǚ': 'u', 'ǜ': 'u',
		'v': 'u',
	}
	
	var sb strings.Builder
	for _, r := range p {
		if val, ok := replacements[r]; ok {
			sb.WriteRune(val)
		} else {
			sb.WriteRune(r)
		}
	}
	return sb.String()
}

var tones = map[rune][]rune{
	'a': {'a', 'ā', 'á', 'ǎ', 'à'},
	'e': {'e', 'ē', 'é', 'ě', 'è'},
	'i': {'i', 'ī', 'í', 'ǐ', 'ì'},
	'o': {'o', 'ō', 'ó', 'ǒ', 'ò'},
	'u': {'u', 'ū', 'ú', 'ǔ', 'ù'},
	'v': {'ü', 'ǖ', 'ǘ', 'ǚ', 'ǜ'},
}

func ConverterPinyin(pinyinNum string) string {
	parts := strings.Split(pinyinNum, " ")
	for i, part := range parts {
		part = strings.ReplaceAll(part, "u:", "v")
		if len(part) == 0 {
			continue
		}
		
		lastChar := part[len(part)-1]
		if lastChar >= '1' && lastChar <= '5' {
			toneIdx := int(lastChar - '0')
			if toneIdx == 5 {
				toneIdx = 0
			}
			word := part[:len(part)-1]
			
			vowels := []rune{'a', 'e', 'o', 'i', 'u', 'v'}
			targetVowel := rune(0)
			
			if strings.ContainsRune(word, 'a') {
				targetVowel = 'a'
			} else if strings.ContainsRune(word, 'e') {
				targetVowel = 'e'
			} else if strings.Contains(word, "ou") {
				targetVowel = 'o'
			} else {
				for j := len(word) - 1; j >= 0; j-- {
					c := rune(word[j])
					isVowel := false
					for _, v := range vowels {
						if c == v {
							isVowel = true
							break
						}
					}
					if isVowel {
						targetVowel = c
						break
					}
				}
			}
			
			if targetVowel != 0 {
				word = strings.Replace(word, string(targetVowel), string(tones[targetVowel][toneIdx]), 1)
			}
			parts[i] = word
		} else {
			parts[i] = strings.ReplaceAll(part, "v", "ü")
		}
	}
	return strings.Join(parts, " ")
}
