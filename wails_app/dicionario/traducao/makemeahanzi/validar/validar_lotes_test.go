package main

import (
	"strings"
	"testing"
)

// ----- Testes do Validador MakeMeAHanzi -----

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

	if _, _, _, err := interpretarArgumentosValidacao([]string{"es", "4"}); err == nil {
		t.Fatalf("lote com menos de 3 dígitos tinha que dar erro")
	}
	if _, _, _, err := interpretarArgumentosValidacao([]string{"pt", "097"}); err == nil {
		t.Fatalf("lote fora da faixa 001-096 tinha que dar erro")
	}
}


func TestDividirLinhaMakemeahanzi(t *testing.T) {
	char, def, dica, err := dividirLinhaMakemeahanzi("一 | DEFINIÇÃO: um/único | DICA: linha horizontal")
	if err != nil || char != "一" || def != "um/único" || dica != "linha horizontal" {
		t.Fatalf("esperava 一 com definição e dica, veio char=%q def=%q dica=%q err=%v", char, def, dica, err)
	}

	char, def, dica, err = dividirLinhaMakemeahanzi("二 | DEFINIÇÃO: dois")
	if err != nil || char != "二" || def != "dois" || dica != "" {
		t.Fatalf("esperava 二 com definição sem dica, veio char=%q def=%q dica=%q err=%v", char, def, dica, err)
	}

	if _, _, _, err := dividirLinhaMakemeahanzi("linha sem marcador"); err == nil {
		t.Fatalf("linha sem marcador tinha que dar erro")
	}
}


func TestProblemasDaLinhaAceitaLinhasValidas(t *testing.T) {
	orig := "一 | DEFINIÇÃO: one | DICA: horizontal line"
	trad := "一 | DEFINIÇÃO: um | DICA: linha horizontal"

	if problemas := problemasDaLinha(orig, trad); len(problemas) != 0 {
		t.Fatalf("linha válida não deveria ter problemas, veio: %v", problemas)
	}
}


func TestProblemasDaLinhaDetectaDivergencias(t *testing.T) {
	orig := "一 | DEFINIÇÃO: one | DICA: horizontal line"
	tradCaractereTrocado := "二 | DEFINIÇÃO: um | DICA: linha horizontal"
	tradDefVazia := "一 | DEFINIÇÃO:  | DICA: linha horizontal"

	problemas := problemasDaLinha(orig, tradCaractereTrocado)
	if len(problemas) != 1 || !strings.Contains(problemas[0], "CARACTERE mudou") {
		t.Fatalf("esperava erro de caractere trocado, veio: %v", problemas)
	}

	problemas = problemasDaLinha(orig, tradDefVazia)
	if len(problemas) != 1 || !strings.Contains(problemas[0], "definição vazia") {
		t.Fatalf("esperava erro de definição vazia, veio: %v", problemas)
	}
}
