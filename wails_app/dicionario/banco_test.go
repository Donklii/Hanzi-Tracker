package dicionario

import (
	"reflect"
	"testing"
)

// ----- Reconstrução das entradas fundidas (o inverso da compactação da fusão) -----

func TestReconstruirEntradasLeituraUnicaAchatada(t *testing.T) {
	// Caso mais comum (113.739 das 121.418): a 1ª leitura foi promovida ao topo e removida da lista.
	// O topo é a resposta inteira, e o tradicional do topo é a grafia dela (regra 1).
	banco := NovoBanco()
	entradas := banco.reconstruirEntradas(&entradaFundida{
		Simplificado: "下面",
		Tradicional:  []string{"下麵"},
		Definicao:    "abaixo; embaixo",
		Pinyin:       []string{"xià miàn"},
	})

	if len(entradas) != 1 {
		t.Fatalf("esperava 1 acepção, veio %d: %+v", len(entradas), entradas)
	}
	esperado := EntradaDicionario{
		Tradicional:  "下麵",
		Simplificado: "下面",
		Pinyin:       "xià miàn",
		Significados: []string{"abaixo", "embaixo"},
	}
	if !reflect.DeepEqual(entradas[0], esperado) {
		t.Fatalf("acepção reconstruída errada:\n veio     %+v\n esperado %+v", entradas[0], esperado)
	}
}

func TestReconstruirEntradasTopoVemAntesDasLeituras(t *testing.T) {
	// Caractere curado: o topo (definição do makemeahanzi) é a prioridade e vem primeiro; as leituras
	// do CEDICT vêm em seguida, cada uma com o seu pinyin.
	banco := NovoBanco()
	entradas := banco.reconstruirEntradas(&entradaFundida{
		Simplificado: "好",
		Definicao:    "bom, excelente; gostar de",
		Pinyin:       []string{"hǎo"},
		Decomposicao: "⿰女子",
		Leituras: []leituraFundida{
			{Significados: []string{"bom", "bem"}},
			{Pinyin: "hào", Significados: []string{"gostar de"}},
		},
	})

	if len(entradas) != 3 {
		t.Fatalf("esperava topo + 2 leituras, veio %d: %+v", len(entradas), entradas)
	}
	if entradas[0].Pinyin != "hǎo" || !reflect.DeepEqual(entradas[0].Significados, []string{"bom, excelente", "gostar de"}) {
		t.Fatalf("acepção do topo errada: %+v", entradas[0])
	}
	// Regra 2: leitura sem pinyin herda o do topo.
	if entradas[1].Pinyin != "hǎo" {
		t.Fatalf("leitura sem pinyin devia herdar o do topo, veio %q", entradas[1].Pinyin)
	}
	if entradas[2].Pinyin != "hào" {
		t.Fatalf("leitura com pinyin próprio devia mantê-lo, veio %q", entradas[2].Pinyin)
	}
	// Regra 4: sem tradicional no topo nem na leitura, a grafia é o próprio simplificado.
	for i, e := range entradas {
		if e.Tradicional != "好" {
			t.Fatalf("acepção %d devia ter grafia 好, veio %q", i, e.Tradicional)
		}
	}
}

// TestReconstruirEntradasPropagaTipoDaLeitura garante que o rótulo language-neutral gravado pela fusão
// (TIPO_LEITURA_*) chega ao consumidor: o topo de conteúdo não recebe rótulo e cada leitura mantém o seu.
func TestReconstruirEntradasPropagaTipoDaLeitura(t *testing.T) {
	banco := NovoBanco()
	entradas := banco.reconstruirEntradas(&entradaFundida{
		Simplificado: "伭",
		Definicao:    "cruel; impiedoso",
		Pinyin:       []string{"xián"},
		Leituras: []leituraFundida{
			{Significados: []string{"sobrenome Xian"}, Tipo: TIPO_LEITURA_SOBRENOME},
			{Pinyin: "xuán", Significados: []string{"variante de 玄"}, Tipo: TIPO_LEITURA_VARIANTE},
		},
	})

	if len(entradas) != 3 {
		t.Fatalf("esperava topo + 2 leituras, veio %d: %+v", len(entradas), entradas)
	}
	if entradas[0].Tipo != "" {
		t.Fatalf("o topo de conteúdo não podia receber rótulo, veio %q", entradas[0].Tipo)
	}
	if entradas[1].Tipo != TIPO_LEITURA_SOBRENOME || entradas[2].Tipo != TIPO_LEITURA_VARIANTE {
		t.Fatalf("os rótulos das leituras não chegaram: %q / %q", entradas[1].Tipo, entradas[2].Tipo)
	}
}

func TestReconstruirEntradasGrafiaDivergentePorLeitura(t *testing.T) {
	// 后: "rainha" NUNCA vira 後 — a exceção só sobrevive porque o tradicional está na leitura, e a
	// regra 3 (grafia única do topo) não pode se aplicar quando alguma leitura discorda.
	banco := NovoBanco()
	entradas := banco.reconstruirEntradas(&entradaFundida{
		Simplificado: "后",
		Tradicional:  []string{"後"},
		Definicao:    "rainha",
		Pinyin:       []string{"hòu"},
		Decomposicao: "⿸⺁口",
		Leituras: []leituraFundida{
			{Tradicional: "後", Significados: []string{"atrás", "depois"}},
		},
	})

	if len(entradas) != 2 {
		t.Fatalf("esperava topo + 1 leitura, veio %d", len(entradas))
	}
	if entradas[0].Tradicional != "后" {
		t.Fatalf("rainha não podia ganhar a grafia 後, veio %q", entradas[0].Tradicional)
	}
	if entradas[1].Tradicional != "後" {
		t.Fatalf("a leitura 'atrás' devia manter a grafia própria 後, veio %q", entradas[1].Tradicional)
	}
}

func TestGrafiaPadraoSoAdotaOhTradicionalQuandoNinguemDiscorda(t *testing.T) {
	// Regra 3: um tradicional no topo e nenhuma leitura com campo próprio → todas usam essa forma.
	uniforme := &entradaFundida{
		Simplificado: "下面",
		Tradicional:  []string{"下麵"},
		Leituras:     []leituraFundida{{Significados: []string{"abaixo"}}},
	}
	if grafia := grafiaPadrao(uniforme); grafia != "下麵" {
		t.Fatalf("esperava a grafia única 下麵, veio %q", grafia)
	}

	// Alguma leitura com grafia própria: a regra 3 não vale, cai na 4 (simplificado).
	divergente := &entradaFundida{
		Simplificado: "后",
		Tradicional:  []string{"後"},
		Leituras:     []leituraFundida{{Tradicional: "後"}},
	}
	if grafia := grafiaPadrao(divergente); grafia != "后" {
		t.Fatalf("com leitura discordante a grafia padrão devia ser o simplificado, veio %q", grafia)
	}

	// Mais de um tradicional (发 ← 發/髮): não há forma única para adotar.
	varios := &entradaFundida{Simplificado: "发", Tradicional: []string{"發", "髮"}}
	if grafia := grafiaPadrao(varios); grafia != "发" {
		t.Fatalf("com vários tradicionais a grafia padrão devia ser o simplificado, veio %q", grafia)
	}
}

func TestIndexarEntradaIndexaPelasDuasGrafias(t *testing.T) {
	banco := NovoBanco()
	banco.indexarEntrada(&entradaFundida{
		Simplificado: "发",
		Tradicional:  []string{"發"},
		Definicao:    "emitir",
		Pinyin:       []string{"fā"},
	})

	if len(banco.Buscar("发")) == 0 {
		t.Error("entrada não indexada pelo simplificado")
	}
	if len(banco.Buscar("發")) == 0 {
		t.Error("entrada não indexada pelo tradicional")
	}
	// O par alimenta a conversão caractere a caractere.
	if convertido := banco.ConverterTexto("发", "tradicional"); convertido != "發" {
		t.Errorf("ConverterTexto(发→tradicional) = %q, esperado 發", convertido)
	}
	if convertido := banco.ConverterTexto("發", "simplificado"); convertido != "发" {
		t.Errorf("ConverterTexto(發→simplificado) = %q, esperado 发", convertido)
	}
}

func TestIndexarEntradaSoGuardaCaractereQuandoHaDadoCurado(t *testing.T) {
	banco := NovoBanco()

	// Palavra só do CEDICT (sem decomposição): não há caractere curado para guardar.
	banco.indexarEntrada(&entradaFundida{Simplificado: "世界", Definicao: "mundo", Pinyin: []string{"shì jiè"}})
	if banco.BuscarCaractere("世界") != nil {
		t.Error("entrada sem decomposição não podia virar caractere curado")
	}
	if banco.TotalHanzis() != 0 {
		t.Errorf("TotalHanzis devia contar só os curados, veio %d", banco.TotalHanzis())
	}

	banco.indexarEntrada(&entradaFundida{
		Simplificado: "好",
		Definicao:    "bom",
		Pinyin:       []string{"hǎo"},
		Decomposicao: "⿰女子",
		Radical:      "女",
		Etimologia:   &etimologiaFundida{Tipo: "ideographic", Dica: "Uma mulher 女 com um filho 子"},
	})
	caractere := banco.BuscarCaractere("好")
	if caractere == nil {
		t.Fatal("entrada com decomposição devia virar caractere curado")
	}
	if caractere.Etimologia.Tipo != "ideographic" || caractere.Etimologia.Dica == "" {
		t.Errorf("etimologia não foi convertida das tags PT do arquivo: %+v", caractere.Etimologia)
	}
}

func TestBuscarPorPinyinAceitaComEhSemTom(t *testing.T) {
	banco := NovoBanco()
	banco.indexarEntrada(&entradaFundida{Simplificado: "好", Definicao: "bom", Pinyin: []string{"hǎo"}})

	if hanzis := banco.BuscarPorPinyin("hao"); len(hanzis) != 1 || hanzis[0] != "好" {
		t.Errorf("BuscarPorPinyin(hao) = %v, esperado [好]", hanzis)
	}
	if hanzis := banco.BuscarPorPinyin("hǎo"); len(hanzis) != 1 || hanzis[0] != "好" {
		t.Errorf("BuscarPorPinyin(hǎo) = %v, esperado [好]", hanzis)
	}
	if hanzis := banco.BuscarPorPinyin("hao3"); len(hanzis) != 1 || hanzis[0] != "好" {
		t.Errorf("BuscarPorPinyin(hao3) = %v, esperado [好]", hanzis)
	}
}

func TestEhCaractereUnicoSeparaPalavraDeCaractere(t *testing.T) {
	if !ehCaractereUnico("好") {
		t.Error("好 é um caractere isolado")
	}
	if ehCaractereUnico("世界") {
		t.Error("世界 é palavra composta, não caractere isolado")
	}
	if ehCaractereUnico("") {
		t.Error("string vazia não é caractere")
	}
}

// ----- Pinyin -----

func TestConverterPinyinAcentuaAhVogalCerta(t *testing.T) {
	casos := []struct{ numerado, esperado string }{
		{"hao3", "hǎo"},          // 'a' vence, mesmo não sendo a última vogal
		{"hao3 chi1", "hǎo chī"}, // várias sílabas
		{"gou3", "gǒu"},          // "ou" acentua no 'o'
		{"lv4", "lǜ"},            // v → ü
		{"nu:3", "nǚ"},           // "u:" do CEDICT → ü
		{"ma5", "ma"},            // tom neutro fica sem acento
		{"xian1", "xiān"},        // 'a' vence 'i'
		{"n", "n"},               // sílaba sem vogal passa intacta
	}

	for _, caso := range casos {
		if convertido := ConverterPinyin(caso.numerado); convertido != caso.esperado {
			t.Errorf("ConverterPinyin(%q) = %q, esperado %q", caso.numerado, convertido, caso.esperado)
		}
	}
}

func TestRemoverTonsPinyinNormalizaAcentoEhU(t *testing.T) {
	if semTom := RemoverTonsPinyin("hǎo chī"); semTom != "hao chi" {
		t.Errorf("RemoverTonsPinyin(hǎo chī) = %q, esperado 'hao chi'", semTom)
	}
	if semTom := RemoverTonsPinyin("nǚ"); semTom != "nu" {
		t.Errorf("RemoverTonsPinyin(nǚ) = %q, esperado 'nu'", semTom)
	}
}

func TestObterFrequencia(t *testing.T) {
	banco := NovoBanco()
	e := &entradaFundida{
		Simplificado: "下面",
		Definicao:    "abaixo",
		Frequencia: &Frequencia{
			Percentual: 0.1234,
			Posicao:    500,
		},
	}
	banco.indexarEntrada(e)

	freq := banco.ObterFrequencia("下面")
	if freq == nil || freq.Posicao != 500 {
		t.Errorf("esperava frequencia com posicao 500, veio %+v", freq)
	}
}

func TestObterAlternativa(t *testing.T) {
	banco := NovoBanco()
	e := &entradaFundida{
		Simplificado: "学",
		Tradicional:  []string{"學"},
		Definicao:    "estudar",
	}
	banco.indexarEntrada(e)

	alt := banco.ObterAlternativa("学")
	if alt != "學" {
		t.Errorf("esperava alternativa de '学' ser '學', veio %q", alt)
	}

	alt2 := banco.ObterAlternativa("學")
	if alt2 != "学" {
		t.Errorf("esperava alternativa de '學' ser '学', veio %q", alt2)
	}

	alt3 := banco.ObterAlternativa("人")
	if alt3 != "" {
		t.Errorf("esperava alternativa de '人' ser vazia, veio %q", alt3)
	}
}
