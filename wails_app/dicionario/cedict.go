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
	entradas    map[string][]EntradaDicionario
	pinyinIndex map[string][]string
}

func NovoCedict() *Cedict {
	return &Cedict{
		entradas:    make(map[string][]EntradaDicionario),
		pinyinIndex: make(map[string][]string),
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

// BuscarPorPinyin retorna uma lista de Hanzis simplificados que correspondem ao pinyin dado (sem tom/espaço)
func (c *Cedict) BuscarPorPinyin(pinyin string) []string {
    cleanPinyin := strings.ToLower(pinyin)
    for _, num := range []string{"1", "2", "3", "4", "5", " "} {
        cleanPinyin = strings.ReplaceAll(cleanPinyin, num, "")
    }
    return c.pinyinIndex[cleanPinyin]
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
