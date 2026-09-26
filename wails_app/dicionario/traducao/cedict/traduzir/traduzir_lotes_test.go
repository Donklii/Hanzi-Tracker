package main

import (
	"os"
	"testing"
)

// ----- Tradutor de lotes do CEDICT (funções puras, sem rede) -----

func TestExtrairRespostasNumeradasAceitaFormatosEhIgnoraProsa(t *testing.T) {
	resposta := "Aqui estão as traduções:\n" +
		"```\n" +
		"1. recorde mundial\n" +
		"2: de classe mundial/famoso\n" +
		"15) variante de 臺\n" +
		"linha solta sem número\n" +
		"```\n"

	respostas := extrairRespostasNumeradas(resposta)
	if len(respostas) != 3 {
		t.Fatalf("esperava 3 respostas numeradas, veio %d: %v", len(respostas), respostas)
	}
	if respostas[1] != "recorde mundial" || respostas[2] != "de classe mundial/famoso" || respostas[15] != "variante de 臺" {
		t.Fatalf("respostas extraídas erradas: %v", respostas)
	}
}


func TestLimparGlosaDevolvidaRemoveEcosDoModelo(t *testing.T) {
	casos := []struct {
		texto    string
		palavra  string
		esperado string
	}{
		{"recorde mundial", "世界纪录", "recorde mundial"},                  // resposta limpa passa intacta
		{"世界纪录 | recorde mundial", "世界纪录", "recorde mundial"},           // eco da palavra + separador
		{"世界纪录 | TRADUÇÃO: recorde mundial", "世界纪录", "recorde mundial"}, // eco do formato completo do lote
		{"  TRADUÇÃO: recorde mundial ", "世界纪录", "recorde mundial"},     // só o prefixo do marcador
	}
	for _, caso := range casos {
		if limpo := limparGlosaDevolvida(caso.texto, caso.palavra); limpo != caso.esperado {
			t.Fatalf("limparGlosaDevolvida(%q) devia dar %q, veio %q", caso.texto, caso.esperado, limpo)
		}
	}
}


func TestValidarSintaxeBasica(t *testing.T) {
	if !validarSintaxeBasica("recorde mundial/de classe mundial/histórico") {
		t.Fatalf("sintaxe simples deveria ser válida")
	}
	if !validarSintaxeBasica("bom/bem/certo ‖ gostar de") {
		t.Fatalf("grupos múltiplos deveriam ser válidos")
	}
	if validarSintaxeBasica("") {
		t.Fatalf("glosa vazia não pode ser válida")
	}
	if validarSintaxeBasica("bom ‖ ") {
		t.Fatalf("grupo vazio não pode ser válido")
	}
}


func TestTentarRespostasSemNumeracao(t *testing.T) {
	linhas := []linhaLote{
		{numero: 1, palavra: "世界", restoOriginal: "world", quantidadeLeituras: 1},
		{numero: 2, palavra: "好", restoOriginal: "good/well ‖ to be fond of/to like", quantidadeLeituras: 2},
	}

	respostaValida := "mundo\nbom/bem ‖ gostar de"
	res := tentarRespostasSemNumeracao(respostaValida, linhas)
	if len(res) != 2 || res[1] != "mundo" || res[2] != "bom/bem ‖ gostar de" {
		t.Fatalf("resposta sem numeração válida falhou: %v", res)
	}

	respostaInvalida := "mundo"
	if res := tentarRespostasSemNumeracao(respostaInvalida, linhas); res != nil {
		t.Fatalf("resposta com contagem errada tinha que retornar nil")
	}
}


func TestAchatarGruposInventadosSoNoGrupoUnico(t *testing.T) {
	// Caso real: original com 1 grupo, modelo separou as acepções com ‖ por conta própria.
	achatada := achatarGruposInventados("(forma presa) brutal/opressivo ‖ (literário) desastre/catástrofe", 1)
	if achatada != "/(forma presa) brutal/opressivo/(literário) desastre/catástrofe/" {
		t.Fatalf("esperava achatamento dos grupos inventados, veio %q", achatada)
	}

	multi := achatarGruposInventados("bom ‖ gostar de", 2)
	if multi != "bom ‖ gostar de" {
		t.Fatalf("glosa com leituras esperadas múltiplas não deve ser achatada, veio %q", multi)
	}
}


func TestGlosaCompativelAceitaNovoEhLegado(t *testing.T) {
	original := linhaLote{palavra: "好", restoOriginal: "good/well ‖ to be fond of/to like", quantidadeLeituras: 2}

	if !glosaCompativel("bom/bem/certo ‖ gostar de", original) {
		t.Fatalf("tradução nova com grupos preservados tinha que ser compatível")
	}
	if glosaCompativel("bom ‖ bem ‖ gostar", original) {
		t.Fatalf("quantidade de grupos errada não podia ser compatível")
	}

	legada := linhaLote{palavra: "好", restoOriginal: "good/well ‖ to be fond of/to like", quantidadeLeituras: 2}
	if !glosaCompativel("bom/bem/gostar de/ter tendência a", legada) {
		t.Fatalf("tradução legada (sem ‖, total de 4 significados igual ao original) tinha que ser compatível")
	}

	simples := linhaLote{palavra: "世界", restoOriginal: "world", quantidadeLeituras: 1}
	if !glosaCompativel("mundo/planeta", simples) {
		t.Fatalf("contagem livre em leitura única tinha que ser compatível")
	}
}


func TestContarSignificadosTotaisSomaOsGrupos(t *testing.T) {
	if total := contarSignificadosTotais("good/well ‖ to be fond of/to like"); total != 4 {
		t.Fatalf("esperava 4 significados no total, veio %d", total)
	}
	if total := contarSignificadosTotais("world"); total != 1 {
		t.Fatalf("esperava 1 significado, veio %d", total)
	}
}


func TestDividirLinhaDoLote(t *testing.T) {
	palavra, glosas, err := dividirLinhaDoLote("久仰 | TRADUÇÃO: honorific: I've long looked forward to meeting you./It's an honor to meet you at last.")
	if err != nil || palavra != "久仰" {
		t.Fatalf("esperava palavra 久仰 sem erro, veio %q (%v)", palavra, err)
	}
	if glosas != "honorific: I've long looked forward to meeting you./It's an honor to meet you at last." {
		t.Fatalf("glosas erradas: %q", glosas)
	}
	if _, _, err := dividirLinhaDoLote("linha sem o marcador"); err == nil {
		t.Fatalf("linha sem marcador tinha que dar erro")
	}
}


func TestFatiarLinhasRespeitaOhTamanho(t *testing.T) {
	linhas := make([]linhaLote, 250)
	fatias := fatiarLinhas(linhas, 100)
	if len(fatias) != 3 || len(fatias[0]) != 100 || len(fatias[1]) != 100 || len(fatias[2]) != 50 {
		t.Fatalf("esperava fatias de 100/100/50, veio %d fatias", len(fatias))
	}
}


func TestObterConfiguracaoIdioma(t *testing.T) {
	cfgPt, err := obterConfiguracaoIdioma("pt")
	if err != nil || cfgPt.codigo != "pt" || cfgPt.sufixoArquivo != "_pt" {
		t.Fatalf("esperava config de pt válida, veio %+v (%v)", cfgPt, err)
	}

	cfgPtBr, err := obterConfiguracaoIdioma("pt-BR")
	if err != nil || cfgPtBr.codigo != "pt" || cfgPtBr.sufixoArquivo != "_pt" {
		t.Fatalf("esperava config de pt-BR normalizada, veio %+v (%v)", cfgPtBr, err)
	}

	cfgEs, err := obterConfiguracaoIdioma("es")
	if err != nil || cfgEs.codigo != "es" || cfgEs.sufixoArquivo != "_es" {
		t.Fatalf("esperava config de es válida, veio %+v (%v)", cfgEs, err)
	}

	if _, err := obterConfiguracaoIdioma("fr"); err == nil {
		t.Fatalf("idioma não suportado 'fr' devia retornar erro")
	}
}


func TestInterpretarArgumentos(t *testing.T) {
	if _, _, _, err := interpretarArgumentos(nil); err == nil {
		t.Fatalf("sem argumentos tinha que dar erro de uso")
	}

	cfg, todos, forcar, err := interpretarArgumentos([]string{"es"})
	if err != nil || cfg.codigo != "es" || forcar || len(todos) != 122 || todos[0] != "001" || todos[121] != "122" {
		t.Fatalf("esperava 122 lotes pendentes em es sem forçar, veio %d forcar=%v err=%v", len(todos), forcar, err)
	}

	cfg, alguns, forcar, err := interpretarArgumentos([]string{"pt", "004", "122"})
	if err != nil || cfg.codigo != "pt" || !forcar || len(alguns) != 2 || alguns[0] != "004" {
		t.Fatalf("esperava [004 122] em pt forçando, veio %v forcar=%v err=%v", alguns, forcar, err)
	}

	if _, _, _, err := interpretarArgumentos([]string{"es", "4"}); err == nil {
		t.Fatalf("número de lote com menos de 3 dígitos tinha que dar erro")
	}
	if _, _, _, err := interpretarArgumentos([]string{"pt", "123"}); err == nil {
		t.Fatalf("número de lote fora da faixa tinha que dar erro")
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
