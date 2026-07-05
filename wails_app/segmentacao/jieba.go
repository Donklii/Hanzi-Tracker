package segmentacao

import (
	_ "embed"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/wangbin/jiebago"
)

//go:embed dict.txt
var dictBytes []byte

var seg jiebago.Segmenter

func InitJieba() error {
	// Write the embedded dict to a temp file because LoadDictionary requires a path
	tmpFile := filepath.Join(os.TempDir(), "jieba_dict.txt")
	err := os.WriteFile(tmpFile, dictBytes, 0644)
	if err != nil {
		return err
	}
	return seg.LoadDictionary(tmpFile)
}

// SegmentarTextoChines segmenta o texto em chinês e retorna palavras únicas.
func SegmentarTextoChines(texto string) []string {
	var palavrasFiltradas []string
	seen := make(map[string]bool)

	palavrasBrutas := seg.Cut(texto, true)

	for word := range palavrasBrutas {
		// Filter and keep only valid Chinese characters (CJK Ideographs: \u4e00 to \u9fff)
		var palavraLimpa strings.Builder
		for _, runeValue := range word {
			if runeValue >= '\u4e00' && runeValue <= '\u9fff' {
				palavraLimpa.WriteRune(runeValue)
			}
		}

		limpaStr := palavraLimpa.String()
		if limpaStr == "" {
			continue
		}

		if !seen[limpaStr] {
			seen[limpaStr] = true
			palavrasFiltradas = append(palavrasFiltradas, limpaStr)
		}
	}

	return palavrasFiltradas
}

// PalavraPosicionada represents a segmented word with its position
type PalavraPosicionada struct {
	Texto  string `json:"texto"`
	Inicio int    `json:"inicio"`
	Fim    int    `json:"fim"`
}

// SegmentarComPosicao segmenta preservando a ordem e o intervalo
func SegmentarComPosicao(texto string) []PalavraPosicionada {
	var posicionadas []PalavraPosicionada
	indice := 0

	ch := seg.Cut(texto, true)
	for token := range ch {
		tamanho := utf8.RuneCountInString(token)
		ehChinesa := false
		if token != "" {
			ehChinesa = true
			for _, c := range token {
				if c < '\u4e00' || c > '\u9fff' {
					ehChinesa = false
					break
				}
			}
		}

		if ehChinesa {
			posicionadas = append(posicionadas, PalavraPosicionada{
				Texto:  token,
				Inicio: indice,
				Fim:    indice + tamanho,
			})
		}
		indice += tamanho
	}

	return posicionadas
}

// TokenSegmentado contains a raw token from Jieba
type TokenSegmentado struct {
	Texto    string
	EhChines bool
}

// SegmentarTodosTokens segmenta o texto preservando todos os tokens (incluindo pontuação e espaços).
// Classifica cada token se é composto inteiramente por caracteres chineses ou não.
func SegmentarTodosTokens(texto string) []TokenSegmentado {
	var tokens []TokenSegmentado
	ch := seg.Cut(texto, true)
	for token := range ch {
		if token == "" {
			continue
		}
		ehChinesa := true
		for _, c := range token {
			if c < '\u4e00' || c > '\u9fff' {
				ehChinesa = false
				break
			}
		}
		tokens = append(tokens, TokenSegmentado{
			Texto:    token,
			EhChines: ehChinesa,
		})
	}
	return tokens
}
