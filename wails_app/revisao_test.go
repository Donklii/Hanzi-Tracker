package main

import (
	"strings"
	"testing"

	"wails_app/config"
	"wails_app/dicionario"
	"wails_app/segmentacao"
)

// appDeTeste monta um App mínimo para a revisão: dicionários reais (embarcados), sem SQLite —
// caracteresEmEstudo cai no caminho de erro e a seleção usa só o sorteio do dicionário.
func appDeTeste(t *testing.T) *App {
	t.Helper()

	banco := dicionario.NovoBancoMakeMeAHanzi()
	if err := banco.Carregar(); err != nil {
		t.Fatalf("falha ao carregar MakeMeAHanzi: %v", err)
	}

	cedict := dicionario.NovoCedict()
	if err := cedict.Carregar(); err != nil {
		t.Fatalf("falha ao carregar CEDICT: %v", err)
	}

	if err := segmentacao.InitJieba(); err != nil {
		t.Fatalf("falha ao carregar Jieba: %v", err)
	}

	return &App{
		Config:        config.DefaultConfig(),
		BancoHanzi:    banco,
		BancoFrases:   dicionario.NovoBancoFrases(),
		BancoTracados: dicionario.NovoBancoTracados(),
		Cedict:        cedict,
	}
}

func TestObterQuestoesRevisaoTodosOsModos(t *testing.T) {
	app := appDeTeste(t)

	for _, modo := range []string{ModoSignificado, ModoFonetica, ModoDesenho, ModoContexto, ModoGeral} {
		questoes, err := app.ObterQuestoesRevisao(modo, 10)
		if err != nil {
			t.Fatalf("modo %s: %v", modo, err)
		}
		if len(questoes) == 0 {
			t.Fatalf("modo %s: nenhuma questão gerada", modo)
		}

		vistos := make(map[string]bool)
		for _, q := range questoes {
			if q.Hanzi == "" || q.Pinyin == "" || q.Definicao == "" {
				t.Errorf("modo %s: questão incompleta: %+v", modo, q)
			}
			if vistos[q.Hanzi] && modo != ModoGeral {
				t.Errorf("modo %s: hanzi %q repetido na sessão", modo, q.Hanzi)
			}
			vistos[q.Hanzi] = true
			validarQuestao(t, q)
		}
	}
}

func validarQuestao(t *testing.T, q QuestaoRevisao) {
	t.Helper()

	precisaOpcoes := q.Modo == ModoSignificado || q.Modo == ModoFonetica || q.Modo == ModoContexto
	if precisaOpcoes {
		limiteOpcoes := 4
		if q.Variante == "traducao_contexto" {
			limiteOpcoes = 3
		}
		if len(q.Opcoes) != limiteOpcoes {
			t.Fatalf("questão %q (%s, variante %s): esperava %d opções, veio %d", q.Hanzi, q.Modo, q.Variante, limiteOpcoes, len(q.Opcoes))
		}
		corretas := 0
		for _, o := range q.Opcoes {
			if o.Correta {
				corretas++
				if q.Variante == "traducao_contexto" {
					if o.Definicao != q.FraseTraducao {
						t.Errorf("questão %q: opção correta de tradução esperava %q, veio %q", q.Hanzi, q.FraseTraducao, o.Definicao)
					}
					if o.Hanzi != q.FraseOriginal {
						t.Errorf("questão %q: opção correta de tradução esperava hanzi %q, veio %q", q.Hanzi, q.FraseOriginal, o.Hanzi)
					}
				} else {
					if o.Hanzi != q.Hanzi {
						t.Errorf("questão %q: opção correta aponta para %q", q.Hanzi, o.Hanzi)
					}
				}
			}
		}
		if corretas != 1 {
			t.Errorf("questão %q (variante %s): esperava exatamente 1 opção correta, veio %d", q.Hanzi, q.Variante, corretas)
		}
	}

	temFrase := q.Variante == "contexto" || q.Variante == "traducao_contexto" || q.Variante == "desenho_contexto"
	if temFrase {
		if q.FraseOriginal == "" || !strings.Contains(q.FraseOriginal, q.Hanzi) {
			t.Errorf("questão %q: frase original não contém o alvo: %q", q.Hanzi, q.FraseOriginal)
		}
		if !strings.Contains(q.FraseLacuna, "＿") {
			t.Errorf("questão %q: frase sem lacuna: %q", q.Hanzi, q.FraseLacuna)
		}
		if q.FraseAtribuicao == "" {
			t.Errorf("questão %q: frase Tatoeba sem atribuição CC-BY", q.Hanzi)
		}
	}

	if q.Modo == ModoContexto && q.Variante == "contexto" {
		for _, o := range q.Opcoes {
			if !o.Correta && strings.Contains(q.FraseOriginal, o.Hanzi) {
				t.Errorf("questão %q: distrator %q aparece na própria frase", q.Hanzi, o.Hanzi)
			}
		}
	}

	if q.Modo == ModoFonetica {
		sons := make(map[string]bool)
		for _, o := range q.Opcoes {
			if sons[o.Pinyin] {
				t.Errorf("questão %q: pinyin %q repetido entre as opções de áudio", q.Hanzi, o.Pinyin)
			}
			sons[o.Pinyin] = true
		}
	}
}

func TestObterDadosEscritaHanzi(t *testing.T) {
	app := appDeTeste(t)

	dados, err := app.ObterDadosEscritaHanzi("你")
	if err != nil {
		t.Fatalf("ObterDadosEscritaHanzi(你): %v", err)
	}
	if !strings.Contains(dados, "strokes") || !strings.Contains(dados, "medians") {
		t.Errorf("dados de escrita sem strokes/medians: %.80s…", dados)
	}

	if _, err := app.ObterDadosEscritaHanzi("☃"); err == nil {
		t.Error("esperava erro para caractere sem traçados")
	}
}
