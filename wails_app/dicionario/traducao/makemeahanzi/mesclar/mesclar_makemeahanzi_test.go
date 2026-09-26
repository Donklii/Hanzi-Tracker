package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// ----- Testes do Mesclador Makemeahanzi -----

func TestInterpretarArgumentos(t *testing.T) {
	if _, err := interpretarArgumentos(nil); err == nil {
		t.Fatalf("sem argumentos tinha que dar erro")
	}

	cfgPt, err := interpretarArgumentos([]string{"pt"})
	if err != nil || cfgPt.codigo != "pt" || cfgPt.sufixoLote != "_pt" || !strings.Contains(cfgPt.caminhoSaida, "pt-BR") {
		t.Fatalf("esperava config pt válida, veio %+v (%v)", cfgPt, err)
	}

	cfgPtBr, err := interpretarArgumentos([]string{"pt-BR"})
	if err != nil || cfgPtBr.codigo != "pt" || cfgPtBr.sufixoLote != "_pt" {
		t.Fatalf("esperava config pt-BR normalizada, veio %+v (%v)", cfgPtBr, err)
	}

	cfgEs, err := interpretarArgumentos([]string{"es"})
	if err != nil || cfgEs.codigo != "es" || cfgEs.sufixoLote != "_es" || !strings.Contains(cfgEs.caminhoSaida, "es") {
		t.Fatalf("esperava config es válida, veio %+v (%v)", cfgEs, err)
	}

	if _, err := interpretarArgumentos([]string{"en"}); err == nil {
		t.Fatalf("idioma inválido 'en' tinha que dar erro")
	}
}


func TestAplicarDicaAdicionaEhRemoveCorretamente(t *testing.T) {
	// Caso 1: sem etimologia prévia, adiciona dica
	campos := map[string]json.RawMessage{}
	aplicarDica(campos, "dica em português")
	if _, ok := campos["etymology"]; !ok {
		t.Fatalf("esperava etimologia criada com dica")
	}
	var etim map[string]string
	_ = json.Unmarshal(campos["etymology"], &etim)
	if etim["hint"] != "dica em português" {
		t.Fatalf("hint errado: %q", etim["hint"])
	}

	// Caso 2: com etimologia prévia preservando tipo/fonética, remove dica vazia
	camposComEtim := map[string]json.RawMessage{
		"etymology": json.RawMessage(`{"type":"ideographic","hint":"old english hint"}`),
	}
	aplicarDica(camposComEtim, "")
	var etimAtualizada map[string]string
	_ = json.Unmarshal(camposComEtim["etymology"], &etimAtualizada)
	if _, ok := etimAtualizada["hint"]; ok {
		t.Fatalf("hint deveria ter sido removido")
	}
	if etimAtualizada["type"] != "ideographic" {
		t.Fatalf("campo type deveria ser preservado: %q", etimAtualizada["type"])
	}

	// Caso 3: dica vazia e etimologia só continha hint -> remove etymology por completo
	camposSoHint := map[string]json.RawMessage{
		"etymology": json.RawMessage(`{"hint":"only hint"}`),
	}
	aplicarDica(camposSoHint, "")
	if _, ok := camposSoHint["etymology"]; ok {
		t.Fatalf("etymology deveria ter sido deletada se ficou vazia")
	}
}


func TestCarregarArquivoLoteLeEntradasValidas(t *testing.T) {
	dir := t.TempDir()
	caminhoLote := filepath.Join(dir, "lote_001_pt.txt")

	conteudo := "# Cabeçalho\n" +
		"一 | DEFINIÇÃO: um/único | DICA: linha horizontal representando o número um\n" +
		"二 | DEFINIÇÃO: dois\n" +
		"三 | DEFINIÇÃO: três | DICA: \n"

	if err := os.WriteFile(caminhoLote, []byte(conteudo), 0o644); err != nil {
		t.Fatalf("falha ao criar arquivo de teste: %v", err)
	}

	traducoes := make(map[string]traducao)
	if err := carregarArquivoLote(caminhoLote, traducoes); err != nil {
		t.Fatalf("erro ao ler lote: %v", err)
	}

	if len(traducoes) != 3 {
		t.Fatalf("esperava 3 traduções, veio %d", len(traducoes))
	}

	esperadoUm := traducao{definicao: "um/único", dica: "linha horizontal representando o número um"}
	if !reflect.DeepEqual(traducoes["一"], esperadoUm) {
		t.Fatalf("tradução de 一 incorreta: %+v", traducoes["一"])
	}

	esperadoDois := traducao{definicao: "dois", dica: ""}
	if !reflect.DeepEqual(traducoes["二"], esperadoDois) {
		t.Fatalf("tradução de 二 incorreta: %+v", traducoes["二"])
	}
}


func TestMesclarIntegraDefinicoesPreservandoCamposOriginais(t *testing.T) {
	dir := t.TempDir()
	caminhoOrigem := filepath.Join(dir, "makemeahanzi_en.txt")
	caminhoSaida := filepath.Join(dir, "saida", "makemeahanzi.txt")

	linha1 := `{"character":"一","definition":"one","pinyin":["yi1"],"radical":"一","etymology":{"type":"ideographic","hint":"one"}}`
	linha2 := `{"character":"二","definition":"two","pinyin":["er4"],"radical":"一"}`
	linhaSemTrad := `{"character":"⼁","pinyin":["shu4"],"radical":"⼁"}`

	conteudoOrigem := strings.Join([]string{linha1, linha2, linhaSemTrad}, "\n") + "\n"
	if err := os.WriteFile(caminhoOrigem, []byte(conteudoOrigem), 0o644); err != nil {
		t.Fatalf("falha ao escrever origem: %v", err)
	}

	traducoes := map[string]traducao{
		"一": {definicao: "um/único", dica: "dica um"},
		"二": {definicao: "dois", dica: ""},
	}

	total, comTraducao, semTraducao, err := mesclar(caminhoOrigem, caminhoSaida, traducoes)
	if err != nil {
		t.Fatalf("erro ao mesclar: %v", err)
	}

	if total != 3 || comTraducao != 2 || semTraducao != 1 {
		t.Fatalf("contagens inesperadas: total=%d, comTrad=%d, semTrad=%d", total, comTraducao, semTraducao)
	}

	dadosSaida, err := os.ReadFile(caminhoSaida)
	if err != nil {
		t.Fatalf("falha ao ler saída: %v", err)
	}

	linhasSaida := strings.Split(strings.TrimSpace(string(dadosSaida)), "\n")
	if len(linhasSaida) != 3 {
		t.Fatalf("esperava 3 linhas na saída, veio %d", len(linhasSaida))
	}

	var res1 map[string]any
	_ = json.Unmarshal([]byte(linhasSaida[0]), &res1)
	if res1["definition"] != "um/único" {
		t.Fatalf("definição esperada 'um/único', veio %v", res1["definition"])
	}
	if res1["character"] != "一" || res1["radical"] != "一" {
		t.Fatalf("campos neutros alterados: %+v", res1)
	}

	var resSemTrad map[string]any
	_ = json.Unmarshal([]byte(linhasSaida[2]), &resSemTrad)
	if _, ok := resSemTrad["definition"]; ok {
		t.Fatalf("entrada sem tradução não deveria ter campo definition")
	}
}
