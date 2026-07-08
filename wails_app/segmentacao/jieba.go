package segmentacao

import (
	_ "embed"
	"os"
	"path/filepath"
	"strings"

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
		// Mantém apenas os caracteres chineses de cada token (descarta pontuação/latinos misturados)
		var palavraLimpa strings.Builder
		for _, runeValue := range word {
			if ehRuneChinesa(runeValue) {
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
			if !ehRuneChinesa(c) {
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

// ehRuneChinesa diz se a rune está no bloco principal de ideogramas CJK (U+4E00–U+9FFF).
func ehRuneChinesa(r rune) bool {
	return r >= '一' && r <= '鿿'
}
