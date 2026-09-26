package main

import (
	"strings"
	"testing"
)

// ----- Validador dos lotes traduzidos (regras por linha) -----

func TestProblemasDaLinhaAceitaContagemLivreEmLeituraUnica(t *testing.T) {
	original := "世界 | TRADUÇÃO: world/CL:個|个"

	if problemas := problemasDaLinha(original, "世界 | TRADUÇÃO: mundo"); len(problemas) != 0 {
		t.Fatalf("menos significados que o original é permitido, veio: %v", problemas)
	}
	if problemas := problemasDaLinha(original, "世界 | TRADUÇÃO: mundo/planeta/CL:個|个/globo"); len(problemas) != 0 {
		t.Fatalf("mais significados que o original é permitido, veio: %v", problemas)
	}
}


func TestProblemasDaLinhaDetectaPalavraTrocada(t *testing.T) {
	problemas := problemasDaLinha("上睑 | TRADUÇÃO: upper eyelid", "上眼睑 | TRADUÇÃO: pálpebra superior")
	if len(problemas) != 1 || !strings.Contains(problemas[0], "PALAVRA mudou") {
		t.Fatalf("esperava exatamente o problema de PALAVRA trocada, veio: %v", problemas)
	}
}


func TestProblemasDaLinhaConfereGruposDeLeitura(t *testing.T) {
	original := "好 | TRADUÇÃO: good/well ‖ to be fond of/to like"

	if problemas := problemasDaLinha(original, "好 | TRADUÇÃO: bom/bem/certo ‖ gostar de"); len(problemas) != 0 {
		t.Fatalf("grupos preservados com contagem livre tinham que passar, veio: %v", problemas)
	}
	if problemas := problemasDaLinha(original, "好 | TRADUÇÃO: bom ‖ bem ‖ gostar de"); len(problemas) == 0 {
		t.Fatalf("grupo ‖ a mais tinha que reprovar")
	}
	if problemas := problemasDaLinha("世界 | TRADUÇÃO: world", "世界 | TRADUÇÃO: mundo ‖ planeta"); len(problemas) == 0 {
		t.Fatalf("‖ inventado numa palavra de leitura única tinha que reprovar")
	}
}


func TestProblemasDaLinhaCaminhoLegadoPorContagemTotal(t *testing.T) {
	original := "好 | TRADUÇÃO: good/well ‖ to be fond of/to like"

	if problemas := problemasDaLinha(original, "好 | TRADUÇÃO: bom/bem/gostar de/ter tendência a"); len(problemas) != 0 {
		t.Fatalf("tradução legada (sem ‖, total igual) tinha que passar, veio: %v", problemas)
	}
	if problemas := problemasDaLinha(original, "好 | TRADUÇÃO: bom/gostar de"); len(problemas) == 0 {
		t.Fatalf("tradução legada com total diferente tinha que reprovar")
	}
}


func TestProblemasDaLinhaDetectaVaziosEhMalformadas(t *testing.T) {
	if problemas := problemasDaLinha("世界 | TRADUÇÃO: world", "世界 | TRADUÇÃO: mundo//planeta"); len(problemas) == 0 {
		t.Fatalf("significado vazio tinha que reprovar")
	}
	if problemas := problemasDaLinha("好 | TRADUÇÃO: good ‖ to like", "好 | TRADUÇÃO: bom ‖ "); len(problemas) == 0 {
		t.Fatalf("grupo vazio tinha que reprovar")
	}
	if problemas := problemasDaLinha("世界 | TRADUÇÃO: world", "linha sem o marcador"); len(problemas) != 1 || !strings.Contains(problemas[0], "malformada") {
		t.Fatalf("linha sem marcador tinha que reprovar como malformada, veio: %v", problemas)
	}
}


func TestInterpretarArgumentosValidacao(t *testing.T) {
	if _, _, _, err := interpretarArgumentosValidacao(nil); err == nil {
		t.Fatalf("sem argumentos tinha que dar erro de uso")
	}

	cod, sufixo, alvos, err := interpretarArgumentosValidacao([]string{"es", "001"})
	if err != nil || cod != "es" || sufixo != "_es" || len(alvos) != 1 || alvos[0] != "001" {
		t.Fatalf("esperava es 001, veio cod=%s sufixo=%s alvos=%v err=%v", cod, sufixo, alvos, err)
	}

	cod, sufixo, alvos, err = interpretarArgumentosValidacao([]string{"es", "all"})
	if err != nil || cod != "es" || sufixo != "_es" || len(alvos) != 1 || alvos[0] != "all" {
		t.Fatalf("esperava es all, veio cod=%s sufixo=%s alvos=%v err=%v", cod, sufixo, alvos, err)
	}

	cod, sufixo, alvos, err = interpretarArgumentosValidacao([]string{"es"})
	if err != nil || cod != "es" || sufixo != "_es" || len(alvos) != 1 || alvos[0] != "all" {
		t.Fatalf("esperava es default all, veio cod=%s sufixo=%s alvos=%v err=%v", cod, sufixo, alvos, err)
	}

	cod, sufixo, alvos, err = interpretarArgumentosValidacao([]string{"pt", "001", "002"})
	if err != nil || cod != "pt" || sufixo != "_pt" || len(alvos) != 2 || alvos[0] != "001" || alvos[1] != "002" {
		t.Fatalf("esperava pt [001 002], veio cod=%s sufixo=%s alvos=%v err=%v", cod, sufixo, alvos, err)
	}

	cod, sufixo, alvos, err = interpretarArgumentosValidacao([]string{"001"})
	if err != nil || cod != "pt" || sufixo != "_pt" || len(alvos) != 1 || alvos[0] != "001" {
		t.Fatalf("esperava fallback pt 001, veio cod=%s sufixo=%s alvos=%v err=%v", cod, sufixo, alvos, err)
	}

	cod, sufixo, alvos, err = interpretarArgumentosValidacao([]string{"all"})
	if err != nil || cod != "pt" || sufixo != "_pt" || len(alvos) != 1 || alvos[0] != "all" {
		t.Fatalf("esperava fallback pt all, veio cod=%s sufixo=%s alvos=%v err=%v", cod, sufixo, alvos, err)
	}

	if _, _, _, err := interpretarArgumentosValidacao([]string{"es", "4"}); err == nil {
		t.Fatalf("lote com menos de 3 dígitos tinha que dar erro")
	}
	if _, _, _, err := interpretarArgumentosValidacao([]string{"pt", "123"}); err == nil {
		t.Fatalf("lote fora da faixa 001-122 tinha que dar erro")
	}
	if _, _, _, err := interpretarArgumentosValidacao([]string{"es", "all", "001"}); err == nil {
		t.Fatalf("'all' combinado com outros argumentos tinha que dar erro")
	}
}
