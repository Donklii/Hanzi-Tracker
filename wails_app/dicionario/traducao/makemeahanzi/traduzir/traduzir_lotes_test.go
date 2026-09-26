package main

import (
	"os"
	"reflect"
	"testing"
)

// ----- Testes do Tradutor MakeMeAHanzi -----

func TestExtrairRespostasNumeradas(t *testing.T) {
	resposta := "Aqui estão as traduções:\n" +
		"```\n" +
		"1. DEFINIÇÃO: gelo | DICA: \n" +
		"2: DEFINIÇÃO: coração; mente | DICA: sentimento\n" +
		"15) DEFINIÇÃO: bambu | DICA: duas hastes de bambu\n" +
		"linha solta sem número\n" +
		"```\n"

	respostas := extrairRespostasNumeradas(resposta)
	if len(respostas) != 3 {
		t.Fatalf("esperava 3 respostas numeradas, veio %d: %v", len(respostas), respostas)
	}
	if respostas[1] != "DEFINIÇÃO: gelo | DICA:" || respostas[2] != "DEFINIÇÃO: coração; mente | DICA: sentimento" || respostas[15] != "DEFINIÇÃO: bambu | DICA: duas hastes de bambu" {
		t.Fatalf("respostas extraídas incorretas: %v", respostas)
	}
}


func TestExtrairDefinicaoEhDica(t *testing.T) {
	casos := []struct {
		texto     string
		caractere string
		esperado  resultadoTraducao
		comErro   bool
	}{
		{
			texto:     "DEFINIÇÃO: gelo | DICA:",
			caractere: "⺀",
			esperado:  resultadoTraducao{definicao: "gelo", dica: ""},
			comErro:   false,
		},
		{
			texto:     "⺀ | DEFINIÇÃO: gelo | DICA: água congelada",
			caractere: "⺀",
			esperado:  resultadoTraducao{definicao: "gelo", dica: "água congelada"},
			comErro:   false,
		},
		{
			texto:     "bambu; flauta | DICA: haste de bambu",
			caractere: "⺮",
			esperado:  resultadoTraducao{definicao: "bambu; flauta", dica: "haste de bambu"},
			comErro:   false,
		},
		{
			texto:     "DEFINIÇÃO:  | DICA: vazia",
			caractere: "一",
			esperado:  resultadoTraducao{},
			comErro:   true,
		},
	}

	for _, c := range casos {
		res, err := extrairDefinicaoEhDica(c.texto, c.caractere)
		if c.comErro && err == nil {
			t.Fatalf("esperava erro para %q, veio sucesso: %+v", c.texto, res)
		}
		if !c.comErro && err != nil {
			t.Fatalf("erro inesperado para %q: %v", c.texto, err)
		}
		if !c.comErro && !reflect.DeepEqual(res, c.esperado) {
			t.Fatalf("resultado divergente para %q: esperado %+v, veio %+v", c.texto, c.esperado, res)
		}
	}
}


func TestDividirLinhaMakemeahanzi(t *testing.T) {
	char, def, dica, err := dividirLinhaMakemeahanzi("⺀ | DEFINIÇÃO: ice | DICA: ")
	if err != nil || char != "⺀" || def != "ice" || dica != "" {
		t.Fatalf("divisão com dica vazia falhou: char=%q def=%q dica=%q err=%v", char, def, dica, err)
	}

	char, def, dica, err = dividirLinhaMakemeahanzi("⺮ | DEFINIÇÃO: bamboo | DICA: stalks of bamboo")
	if err != nil || char != "⺮" || def != "bamboo" || dica != "stalks of bamboo" {
		t.Fatalf("divisão com dica falhou: char=%q def=%q dica=%q err=%v", char, def, dica, err)
	}

	if _, _, _, err := dividirLinhaMakemeahanzi("linha sem marcador"); err == nil {
		t.Fatalf("linha sem marcador tinha que retornar erro")
	}
}


func TestFatiarLinhas(t *testing.T) {
	linhas := make([]linhaLote, 120)
	fatias := fatiarLinhas(linhas, 50)
	if len(fatias) != 3 || len(fatias[0]) != 50 || len(fatias[1]) != 50 || len(fatias[2]) != 20 {
		t.Fatalf("esperava 3 fatias (50, 50, 20), veio %d fatias", len(fatias))
	}
}


func TestObterConfiguracaoIdioma(t *testing.T) {
	cfgPt, err := obterConfiguracaoIdioma("pt")
	if err != nil || cfgPt.codigo != "pt" || cfgPt.sufixoArquivo != "_pt" {
		t.Fatalf("configuração de pt falhou: %+v (%v)", cfgPt, err)
	}

	cfgPtBr, err := obterConfiguracaoIdioma("pt-BR")
	if err != nil || cfgPtBr.codigo != "pt" || cfgPtBr.sufixoArquivo != "_pt" {
		t.Fatalf("configuração de pt-BR falhou: %+v (%v)", cfgPtBr, err)
	}

	cfgEs, err := obterConfiguracaoIdioma("es")
	if err != nil || cfgEs.codigo != "es" || cfgEs.sufixoArquivo != "_es" {
		t.Fatalf("configuração de es falhou: %+v (%v)", cfgEs, err)
	}

	if _, err := obterConfiguracaoIdioma("fr"); err == nil {
		t.Fatalf("idioma inválido 'fr' tinha que dar erro")
	}
}


func TestInterpretarArgumentos(t *testing.T) {
	if _, _, _, err := interpretarArgumentos(nil); err == nil {
		t.Fatalf("sem argumentos tinha que dar erro")
	}

	cfg, todos, forcar, err := interpretarArgumentos([]string{"es"})
	if err != nil || cfg.codigo != "es" || forcar || len(todos) != 96 || todos[0] != "001" || todos[95] != "096" {
		t.Fatalf("esperava 96 lotes pendentes em es, veio %d forcar=%v err=%v", len(todos), forcar, err)
	}

	cfg, alguns, forcar, err := interpretarArgumentos([]string{"pt", "001", "096"})
	if err != nil || cfg.codigo != "pt" || !forcar || len(alguns) != 2 || alguns[0] != "001" {
		t.Fatalf("esperava [001 096] forçando, veio %v forcar=%v err=%v", alguns, forcar, err)
	}

	if _, _, _, err := interpretarArgumentos([]string{"es", "1"}); err == nil {
		t.Fatalf("lote com menos de 3 dígitos tinha que dar erro")
	}
	if _, _, _, err := interpretarArgumentos([]string{"pt", "097"}); err == nil {
		t.Fatalf("lote fora da faixa 001-096 tinha que dar erro")
	}
}


func TestObterProvedorConfigurado(t *testing.T) {
	os.Setenv("PROVEDOR_IA", "deepseek")
	os.Setenv("DEEPSEEK_API_KEY", "chave-teste-deepseek")
	prov, chave, modelo, err := obterProvedorConfigurado()
	if err != nil || prov != "deepseek" || chave != "chave-teste-deepseek" || modelo != "deepseek-chat" {
		t.Fatalf("esperava deepseek configurado, veio prov=%s chave=%s modelo=%s err=%v", prov, chave, modelo, err)
	}

	os.Setenv("PROVEDOR_IA", "gemini")
	os.Setenv("GEMINI_API_KEY", "chave-teste-gemini")
	prov, chave, modelo, err = obterProvedorConfigurado()
	if err != nil || prov != "gemini" || chave != "chave-teste-gemini" || modelo != "gemini-3.1-flash-lite" {
		t.Fatalf("esperava gemini configurado, veio prov=%s chave=%s modelo=%s err=%v", prov, chave, modelo, err)
	}

	// Restaurar estado
	os.Unsetenv("PROVEDOR_IA")
}
