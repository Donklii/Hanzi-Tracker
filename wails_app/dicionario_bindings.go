package main

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"wails_app/dicionario"
)

// ----- Bindings de consulta aos dicionários embarcados (CC-CEDICT + MakeMeAHanzi) -----
// Métodos de busca expostos ao frontend e os helpers de leitura compartilhados pelo scan de OCR
// (app.go), pelo registro de vocabulário (vocabulario.go) e pela revisão (revisao.go).

// LookupWord devolve as entradas de dicionário de uma palavra, com o MakeMeAHanzi na frente para
// caracteres isolados (definição por caractere mais rica que a do CEDICT).
func (a *App) LookupWord(word string) []dicionario.EntradaDicionario {
	entradas := a.Cedict.Buscar(word)

	if utf8.RuneCountInString(word) == 1 {
		if dec := a.BancoHanzi.Buscar(word); dec != nil && dec.Definicao != "" {
			pinyin := ""
			if len(dec.Pinyin) > 0 {
				pinyin = strings.Join(dec.Pinyin, ", ")
			}

			entry := dicionario.EntradaDicionario{
				Simplificado: word,
				Tradicional:  word,
				Pinyin:       pinyin,
				Significados: []string{dec.Definicao},
			}

			return append([]dicionario.EntradaDicionario{entry}, entradas...)
		}
	}

	return entradas
}

// BuscarNoDicionarioGeral realiza uma pesquisa global combinando CEDICT e MakeMeAHanzi
func (a *App) BuscarNoDicionarioGeral(termo string) []FlashcardCard {
	var resultados []FlashcardCard
	vistos := make(map[string]bool)

	if strings.HasPrefix(termo, "[DESENHO]") {
		caracteres := strings.TrimPrefix(termo, "[DESENHO]")
		for _, c := range caracteres {
			charStr := string(c)
			if charStr == " " {
				continue
			}
			entradas := a.LookupWord(charStr)
			for _, e := range entradas {
				if !vistos[e.Simplificado] {
					vistos[e.Simplificado] = true
					resultados = append(resultados, FlashcardCard{
						Hanzi:        e.Simplificado,
						Pinyin:       e.Pinyin,
						Significados: e.Significados,
						Confianca:    1.0,
						TipoHanzi:    a.Cedict.AvaliarTipoHanzi(e.Simplificado),
					})
				}
			}
		}
		return resultados
	}

	// Busca primeiro no MakeMeAHanzi (para garantir caracteres únicos de etimologia)
	if a.BancoHanzi != nil {
		entradasMake := a.BancoHanzi.BuscarGeral(termo)
		for _, e := range entradasMake {
			if !vistos[e.Caractere] {
				vistos[e.Caractere] = true

				sigs := strings.Split(e.Definicao, ";")
				for i := range sigs {
					sigs[i] = strings.TrimSpace(sigs[i])
				}

				resultados = append(resultados, FlashcardCard{
					Hanzi:        e.Caractere,
					Pinyin:       strings.Join(e.Pinyin, ", "),
					Significados: sigs,
					Confianca:    1.0,
					TipoHanzi:    a.Cedict.AvaliarTipoHanzi(e.Caractere),
				})
			}
		}
	}

	// Busca no CEDICT
	if a.Cedict != nil {
		entradas := a.Cedict.BuscarGeral(termo)
		for _, e := range entradas {
			if !vistos[e.Simplificado] {
				vistos[e.Simplificado] = true
				resultados = append(resultados, FlashcardCard{
					Hanzi:        e.Simplificado,
					Pinyin:       e.Pinyin,
					Significados: e.Significados,
					Confianca:    1.0,
					TipoHanzi:    a.Cedict.AvaliarTipoHanzi(e.Simplificado),
				})
			}
		}
	}

	return resultados
}

// BuscarPorPinyin devolve os hanzis que casam com o pinyin digitado, limitados para o dropdown da UI.
func (a *App) BuscarPorPinyin(pinyin string) []string {
	const maxSugestoes = 30
	res := a.Cedict.BuscarPorPinyin(pinyin)
	if len(res) > maxSugestoes {
		return res[:maxSugestoes]
	}
	return res
}

// AvaliarTipoHanzi retorna o tipo do hanzi para o frontend ("Tradicional", "Simplificado" ou "Ambos")
func (a *App) AvaliarTipoHanzi(hanzi string) string {
	return a.Cedict.AvaliarTipoHanzi(hanzi)
}

func (a *App) DecomposeCharacter(char string) *dicionario.DecomposicaoHanzi {
	return a.BancoHanzi.Buscar(char)
}

func (a *App) BuscarCaracteresCompostosPor(char string) []string {
	return a.BancoHanzi.BuscarCompostosPor(char)
}

func (a *App) CaractereCompleto(abrev string) string {
	return a.BancoHanzi.CaractereCompleto(abrev)
}

func (a *App) ObterTotalHanzisDicionario() int {
	return a.BancoHanzi.TotalHanzis()
}

// ----- Helpers de leitura compartilhados -----

// buscarLeituraHanzi resolve a leitura (pinyin) e as acepções de um hanzi: para caractere isolado o
// MakeMeAHanzi tem prioridade (pinyin por caractere mais confiável); o CC-CEDICT é o fallback e a
// fonte para palavras. Quando a leitura veio do CEDICT, devolve também a entrada casada (nil caso
// contrário) — o scan de OCR a usa para converter o card ao tipo de hanzi configurado.
func (a *App) buscarLeituraHanzi(hanzi string) (pinyin string, significados []string, entradaCedict *dicionario.EntradaDicionario) {
	if utf8.RuneCountInString(hanzi) == 1 {
		if dec := a.BancoHanzi.Buscar(hanzi); dec != nil && dec.Definicao != "" {
			return strings.Join(dec.Pinyin, ", "), []string{dec.Definicao}, nil
		}
	}

	entradas := a.Cedict.Buscar(hanzi)
	if len(entradas) == 0 {
		return "", nil, nil
	}
	return entradas[0].Pinyin, entradas[0].Significados, &entradas[0]
}

// temEntradaNoDicionario diz se a palavra vale como card/token próprio: caracteres isolados sempre
// passam (fallback universal); palavras compostas precisam de entrada no CC-CEDICT.
func (a *App) temEntradaNoDicionario(palavra string) bool {
	if utf8.RuneCountInString(palavra) == 1 {
		return true
	}
	return len(a.Cedict.Buscar(palavra)) > 0
}

// quebrarEmPalavrasDoDicionario usa Forward Maximum Matching para quebrar OOV (Out-Of-Vocabulary)
// em palavras válidas do dicionário.
func (a *App) quebrarEmPalavrasDoDicionario(text string) []string {
	var result []string
	runes := []rune(text)

	for i := 0; i < len(runes); {
		matched := false
		// Tenta a maior substring possível a partir do índice 'i'
		for j := len(runes); j > i; j-- {
			sub := string(runes[i:j])
			if !a.temEntradaNoDicionario(sub) {
				continue
			}
			result = append(result, sub)
			i = j
			matched = true
			break
		}

		if !matched {
			// Prevenção de loop infinito (embora tamanho 1 sempre dê match)
			result = append(result, string(runes[i:i+1]))
			i++
		}
	}

	return result
}

// contemHanzi diz se a string tem ao menos um caractere Han (chinês).
func contemHanzi(s string) bool {
	for _, r := range s {
		if unicode.Is(unicode.Han, r) {
			return true
		}
	}
	return false
}
