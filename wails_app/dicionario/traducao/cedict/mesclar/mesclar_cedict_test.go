package main

import (
	"reflect"
	"strings"
	"testing"
)

// ----- Mesclagem das traduções do CEDICT (funções puras, sem disco) -----

func TestPartirLinhaCruQuebraOhFormatoDoCedict(t *testing.T) {
	cru, ok := partirLinhaCru("上級 上级 [shang4 ji2] /higher authorities/superiors/CL:個|个[ge4]/")
	if !ok {
		t.Fatalf("linha bem formada tinha que ser aceita")
	}
	if cru.tradicional != "上級" || cru.simplificado != "上级" || cru.pinyin != "shang4 ji2" {
		t.Fatalf("campos language-neutral errados: %+v", cru)
	}
	if !reflect.DeepEqual(cru.significados, []string{"higher authorities", "superiors", "CL:個|个[ge4]"}) {
		t.Fatalf("significados errados: %q", cru.significados)
	}

	if _, ok := partirLinhaCru("# comentário"); ok {
		t.Fatalf("linha sem o formato do CEDICT não podia ser aceita")
	}
	if _, ok := partirLinhaCru("世界 [shi4 jie4] /world/"); ok {
		t.Fatalf("linha sem a forma simplificada não podia ser aceita")
	}
	if _, ok := partirLinhaCru("好 好 [hao3] //"); ok {
		t.Fatalf("linha sem significado nenhum não podia ser aceita (o extrator também a pulou)")
	}
}


func TestDistribuirPorPalavraUsaOhCaminhoPorGrupos(t *testing.T) {
	// 好 com duas linhas cruas (hǎo "bom" e hào "gostar de") e tradução v2 com os grupos preservados.
	linhas := []linhaCru{
		{simplificado: "好", pinyin: "hao3", significados: []string{"good", "well"}},
		{simplificado: "好", pinyin: "hao4", significados: []string{"to be fond of", "to like"}},
	}

	porLinha, legado, err := distribuirPorPalavra(linhas, []int{0, 1}, []string{"bom/bem/certo", "gostar de"})
	if err != nil || legado {
		t.Fatalf("esperava caminho por grupos sem erro, veio legado=%v err=%v", legado, err)
	}
	// Contagem LIVRE por grupo: 3 significados na primeira leitura contra 2 do inglês, e 1 na segunda.
	if !reflect.DeepEqual(porLinha[0], []string{"bom", "bem", "certo"}) || !reflect.DeepEqual(porLinha[1], []string{"gostar de"}) {
		t.Fatalf("grupos repartidos errado: %q", porLinha)
	}

	// Leitura única: tudo vai para a única linha crua, com quantidade livre.
	simples := []linhaCru{{simplificado: "世界", significados: []string{"world"}}}
	porLinha, legado, err = distribuirPorPalavra(simples, []int{0}, []string{"mundo/planeta"})
	if err != nil || legado {
		t.Fatalf("leitura única não podia dar erro nem cair no legado, veio legado=%v err=%v", legado, err)
	}
	if !reflect.DeepEqual(porLinha[0], []string{"mundo", "planeta"}) {
		t.Fatalf("leitura única repartida errado: %q", porLinha)
	}
}


func TestDistribuirPorPalavraUsaOhCaminhoLegadoPorContagem(t *testing.T) {
	// Formato v1: multi-leitura traduzida ANTES do separador existir — a lista veio achatada, e o que
	// permite repartir é a contagem total ter sido preservada (2 + 2 = 4).
	linhas := []linhaCru{
		{simplificado: "好", pinyin: "hao3", significados: []string{"good", "well"}},
		{simplificado: "好", pinyin: "hao4", significados: []string{"to be fond of", "to like"}},
	}

	porLinha, legado, err := distribuirPorPalavra(linhas, []int{0, 1}, []string{"bom/bem/gostar de/ter tendência a"})
	if err != nil || !legado {
		t.Fatalf("esperava caminho legado sem erro, veio legado=%v err=%v", legado, err)
	}
	if !reflect.DeepEqual(porLinha[0], []string{"bom", "bem"}) || !reflect.DeepEqual(porLinha[1], []string{"gostar de", "ter tendência a"}) {
		t.Fatalf("recorte por contagem errado: %q", porLinha)
	}

	if _, _, err := distribuirPorPalavra(linhas, []int{0, 1}, []string{"bom/bem/gostar de"}); err == nil {
		t.Fatalf("total de significados diferente do original tinha que dar erro (não dá para repartir)")
	}
}


func TestDistribuirPorPalavraRecusaQuantidadeDeGruposErrada(t *testing.T) {
	linhas := []linhaCru{
		{simplificado: "好", significados: []string{"good"}},
		{simplificado: "好", significados: []string{"to like"}},
	}

	if _, _, err := distribuirPorPalavra(linhas, []int{0, 1}, []string{"bom", "gostar", "sobrando"}); err == nil {
		t.Fatalf("3 grupos para 2 linhas cruas tinha que dar erro")
	}

	// Grupo ‖ inventado numa palavra de leitura única: não há linha crua para o segundo grupo.
	simples := []linhaCru{{simplificado: "虐", significados: []string{"brutal"}}}
	if _, _, err := distribuirPorPalavra(simples, []int{0}, []string{"brutal", "desastre"}); err == nil {
		t.Fatalf("grupo ‖ sobrando em leitura única tinha que dar erro")
	}
}


func TestDistribuirTraducoesCasaOhIndiceDeCadaLinhaCrua(t *testing.T) {
	// As duas leituras de 好 NÃO são adjacentes: o agrupamento é por palavra, não por vizinhança.
	linhas := []linhaCru{
		{simplificado: "好", pinyin: "hao3", significados: []string{"good"}},
		{simplificado: "世界", pinyin: "shi4 jie4", significados: []string{"world"}},
		{simplificado: "好", pinyin: "hao4", significados: []string{"to like"}},
	}
	traducoes := map[string][]string{
		"好":  {"bom", "gostar de"},
		"世界": {"mundo"},
	}

	significadosPt, stats, err := distribuirTraducoes(linhas, traducoes)
	if err != nil {
		t.Fatalf("distribuição não podia falhar: %v", err)
	}
	if !reflect.DeepEqual(significadosPt[0], []string{"bom"}) || !reflect.DeepEqual(significadosPt[2], []string{"gostar de"}) {
		t.Fatalf("grupos de 好 não casaram com as linhas cruas certas: %q", significadosPt)
	}
	if !reflect.DeepEqual(significadosPt[1], []string{"mundo"}) {
		t.Fatalf("世界 repartida errado: %q", significadosPt[1])
	}
	if stats.palavrasPorGrupo != 2 || stats.palavrasLegado != 0 {
		t.Fatalf("estatísticas erradas: %+v", stats)
	}

	if _, _, err := distribuirTraducoes(linhas, map[string][]string{"好": {"bom", "gostar de"}}); err == nil {
		t.Fatalf("palavra sem tradução em lote nenhum tinha que dar erro")
	}
}


func TestFormatarLinhaCruDevolveOhFormatoDoCedict(t *testing.T) {
	linha := linhaCru{tradicional: "上級", simplificado: "上级", pinyin: "shang4 ji2"}
	texto := formatarLinhaCru(linha, []string{"autoridades superiores", "superiores", "subordinados"})

	esperado := "上級 上级 [shang4 ji2] /autoridades superiores/superiores/subordinados/\n"
	if texto != esperado {
		t.Fatalf("linha remontada errada:\n veio     %q\n esperado %q", texto, esperado)
	}

	// A saída tem que voltar a passar pelo mesmo parsing da entrada (ida e volta sem perda).
	cru, ok := partirLinhaCru(strings.TrimSuffix(texto, "\n"))
	if !ok || cru.tradicional != "上級" || cru.simplificado != "上级" || cru.pinyin != "shang4 ji2" || len(cru.significados) != 3 {
		t.Fatalf("linha remontada não sobreviveu ao parsing de volta: %+v (ok=%v)", cru, ok)
	}
}


func TestDividirGruposEhPartirSignificadosLimpamOhEspaco(t *testing.T) {
	if grupos := dividirGrupos("bom/bem ‖ gostar de"); !reflect.DeepEqual(grupos, []string{"bom/bem", "gostar de"}) {
		t.Fatalf("grupos mal separados: %q", grupos)
	}
	if significados := partirSignificados("bom / bem/certo"); !reflect.DeepEqual(significados, []string{"bom", "bem", "certo"}) {
		t.Fatalf("significados mal separados: %q", significados)
	}
}


func TestInterpretarArgumentosMesclagem(t *testing.T) {
	if _, err := interpretarArgumentosMesclagem(nil); err == nil {
		t.Fatalf("sem argumentos tinha que dar erro")
	}

	cfgPt, err := interpretarArgumentosMesclagem([]string{"pt"})
	if err != nil || cfgPt.codigo != "pt" || cfgPt.sufixoLote != "_pt" || !strings.Contains(cfgPt.caminhoSaida, "pt-BR") {
		t.Fatalf("esperava config pt válida, veio %+v (%v)", cfgPt, err)
	}

	cfgPtBr, err := interpretarArgumentosMesclagem([]string{"pt-BR"})
	if err != nil || cfgPtBr.codigo != "pt" || cfgPtBr.sufixoLote != "_pt" {
		t.Fatalf("esperava config pt-BR normalizada, veio %+v (%v)", cfgPtBr, err)
	}

	cfgEs, err := interpretarArgumentosMesclagem([]string{"es"})
	if err != nil || cfgEs.codigo != "es" || cfgEs.sufixoLote != "_es" || !strings.Contains(cfgEs.caminhoSaida, "es") {
		t.Fatalf("esperava config es válida, veio %+v (%v)", cfgEs, err)
	}

	if _, err := interpretarArgumentosMesclagem([]string{"en"}); err == nil {
		t.Fatalf("idioma inválido 'en' tinha que dar erro")
	}
}
