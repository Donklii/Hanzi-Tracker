package main

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"wails_app/dicionario"
	"wails_app/progresso"
)

// ----- Bindings do vocabulário do usuário (banco de progresso) -----
// Adicionar/remover/listar palavras e o registro automático de "visto" feito pelo scan de OCR e
// pelos cliques no modal de detalhes.

// AddVocab grava/atualiza uma palavra com o status escolhido pelo usuário (estudo/aprendido).
func (a *App) AddVocab(hanzi, pinyin, significado, status string) error {
	return progresso.AddOuUpdateVocab(hanzi, pinyin, significado, status)
}

// RemoveVocab remove uma palavra do vocabulário.
func (a *App) RemoveVocab(hanzi string) error {
	return progresso.RemoveVocab(hanzi)
}

// GetVocab lista o vocabulário para a UI, ocultando componentes/radicais avulsos e preenchendo o
// tipo (simplificado/tradicional) de cada hanzi.
func (a *App) GetVocab() ([]progresso.Vocab, error) {
	v, err := progresso.GetAllVocab()
	if err != nil {
		return nil, err
	}

	var filtrado []progresso.Vocab
	for i := range v {
		if _, ehAbrev := dicionario.MapaAbrevParaCompleto[v[i].Hanzi]; ehAbrev {
			continue // Oculta componentes e radicais avulsos do histórico
		}
		v[i].TipoHanzi = a.Cedict.AvaliarTipoHanzi(v[i].Hanzi)
		filtrado = append(filtrado, v[i])
	}
	return filtrado, nil
}

// MarcarVistoSilencioso registra uma palavra como 'vista' (sem mudar status existente), buscando
// pinyin/significado nos dicionários. Usado ao navegar pela decomposição no modal de detalhes.
func (a *App) MarcarVistoSilencioso(hanzi string) {
	pinyin, significados, _ := a.buscarLeituraHanzi(hanzi)
	progresso.RegistrarVisto(hanzi, pinyin, strings.Join(significados, ", "))

	a.registrarHanzisIndividuais(hanzi)
}

// registrarHanzisIndividuais desmembra uma palavra multi-caractere nos seus hanzis individuais
// e registra cada um como 'visto' com pinyin/significado próprios. Não se aplica a componentes
// de decomposição — apenas aos caracteres que formam a palavra no CEDICT.
func (a *App) registrarHanzisIndividuais(palavra string) {
	if utf8.RuneCountInString(palavra) <= 1 {
		return
	}

	for _, r := range palavra {
		if !unicode.Is(unicode.Han, r) {
			continue
		}

		caractere := string(r)
		pinyin, significados, _ := a.buscarLeituraHanzi(caractere)
		progresso.RegistrarVisto(caractere, pinyin, strings.Join(significados, ", "))
	}
}

// RegistrarRespostaRevisao atualiza as estatísticas de acertos/erros de uma palavra na categoria correspondente.
func (a *App) RegistrarRespostaRevisao(hanzi, pinyin, significado, categoria string, acertou bool) error {
	return progresso.AtualizarAcertosSequencia(hanzi, pinyin, significado, categoria, acertou)
}

// ObterEstatisticasPalavra retorna os acertos em sequência nas 5 categorias para uma palavra.
func (a *App) ObterEstatisticasPalavra(hanzi string) (map[string]int, error) {
	return progresso.ObterEstatisticasPalavra(hanzi)
}

// ObterSugestoesAprendidoLote verifica quais das palavras enviadas atingiram o critério para serem marcadas como aprendidas.
func (a *App) ObterSugestoesAprendidoLote(hanzis []string) ([]progresso.Vocab, error) {
	return progresso.ObterSugestoesAprendidoLote(hanzis)
}
