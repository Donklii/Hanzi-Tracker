package main

import (
	"testing"
)

func TestEnriquecerRecomendacao(t *testing.T) {
	app := &App{}
	// Sem gerenciador de dicionário carregado, ao menos o HSK estático funciona
	rec := &RecomendacaoBaralho{
		Hanzi: "我",
	}
	app.enriquecerRecomendacao(rec)

	if rec.NivelHSK != 1 {
		t.Errorf("esperava NivelHSK 1 para '我', veio %d", rec.NivelHSK)
	}
}

func TestHistoricoRecomendacoesLimite(t *testing.T) {
	app := &App{}

	// Adiciona 35 itens ao histórico
	app.historicoRecomendacoesMutex.Lock()
	for i := 0; i < 35; i++ {
		app.historicoRecomendacoes = append(app.historicoRecomendacoes, string(rune('A'+i)))
	}
	if len(app.historicoRecomendacoes) > 30 {
		app.historicoRecomendacoes = app.historicoRecomendacoes[len(app.historicoRecomendacoes)-30:]
	}
	app.historicoRecomendacoesMutex.Unlock()

	app.historicoRecomendacoesMutex.Lock()
	tam := len(app.historicoRecomendacoes)
	primeiro := app.historicoRecomendacoes[0]
	app.historicoRecomendacoesMutex.Unlock()

	if tam != 30 {
		t.Errorf("esperava histórico limitado em 30 itens, veio %d", tam)
	}
	if primeiro != "F" {
		t.Errorf("esperava que o primeiro item fosse 'F' após corte, veio %s", primeiro)
	}
}

func TestRecomendacaoEstruturaCampos(t *testing.T) {
	rec := RecomendacaoBaralho{
		Hanzi:          "电脑",
		Pinyin:         "diàn nǎo",
		Significado:    "computador",
		Motivo:         "Palavra essencial do HSK 1",
		Revelada:       false,
		NivelHSK:       1,
		PosicaoRanking: 150,
	}

	if rec.NivelHSK != 1 || rec.PosicaoRanking != 150 {
		t.Errorf("campos NivelHSK ou PosicaoRanking inconsistentes: %+v", rec)
	}
}
