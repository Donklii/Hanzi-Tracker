package main

import (
	"compress/gzip"
	"os"
	"path/filepath"
	"testing"

	"wails_app/dicionario"
)

// ----- Leitura da lista de frequência -----

// TestCarregarFrequenciasContagens cobre o caminho feliz: contagem por palavra e total do corpus
// (100 tokens aqui, para toda conta derivada ser conferível de cabeça).
func TestCarregarFrequenciasContagens(t *testing.T) {
	caminho := escreverListaFrequencia(t, "的 50\n我们 30\n好 20\n")

	contagens, totalTokens := carregarFrequencias(caminho)

	if totalTokens != 100 {
		t.Errorf("totalTokens = %d, esperado 100", totalTokens)
	}
	esperadas := map[string]int64{"的": 50, "我们": 30, "好": 20}
	for palavra, esperada := range esperadas {
		if contagens[palavra] != esperada {
			t.Errorf("contagem de %q = %d, esperada %d", palavra, contagens[palavra], esperada)
		}
	}
	if len(contagens) != len(esperadas) {
		t.Errorf("contagens tem %d palavras, esperado %d", len(contagens), len(esperadas))
	}
}

// TestCarregarFrequenciasIgnoraLinhasInvalidas garante que lixo na lista é descartado sem derrubar
// a fusão e, principalmente, sem entrar no total que serve de denominador.
func TestCarregarFrequenciasIgnoraLinhasInvalidas(t *testing.T) {
	caminho := escreverListaFrequencia(t, "的 60\nsemcontagem\n好 40\n 15\n汉 abc\n\n")

	contagens, totalTokens := carregarFrequencias(caminho)

	if totalTokens != 100 {
		t.Errorf("totalTokens = %d, esperado 100 (linhas inválidas não podem contar)", totalTokens)
	}
	if len(contagens) != 2 {
		t.Errorf("contagens tem %d palavras, esperado 2 (的 e 好)", len(contagens))
	}
	for _, invalida := range []string{"semcontagem", "汉", ""} {
		if _, existe := contagens[invalida]; existe {
			t.Errorf("linha inválida %q virou entrada de frequência", invalida)
		}
	}
}

// TestCarregarFrequenciasSomaPalavraRepetida cobre a borda de uma palavra aparecer duas vezes na
// lista: as contagens somam em vez de a última sobrescrever a primeira.
func TestCarregarFrequenciasSomaPalavraRepetida(t *testing.T) {
	caminho := escreverListaFrequencia(t, "好 30\n的 40\n好 30\n")

	contagens, totalTokens := carregarFrequencias(caminho)

	if totalTokens != 100 {
		t.Errorf("totalTokens = %d, esperado 100", totalTokens)
	}
	if contagens["好"] != 60 {
		t.Errorf("contagem de 好 = %d, esperada 60 (30+30 somados)", contagens["好"])
	}
}

// ----- Percentual e posição -----

// TestAplicarFrequenciasPercentualEhPosicao cobre o caminho feliz: percentual sobre o corpus e
// posição em ordem decrescente de uso, começando em 1.
func TestAplicarFrequenciasPercentualEhPosicao(t *testing.T) {
	entradas := entradasDeTeste("的", "我们", "好")
	contagens := map[string]int64{"的": 50, "我们": 30, "好": 20}

	comFrequencia, cobertura := aplicarFrequencias(entradas, contagens, 100)

	if comFrequencia != 3 {
		t.Errorf("comFrequencia = %d, esperado 3", comFrequencia)
	}
	if cobertura != 100 {
		t.Errorf("cobertura = %g, esperada 100", cobertura)
	}
	esperados := map[string]Frequencia{
		"的":  {Percentual: 50, Posicao: 1},
		"我们": {Percentual: 30, Posicao: 2},
		"好":  {Percentual: 20, Posicao: 3},
	}
	for palavra, esperado := range esperados {
		obtido := entradas[palavra].Frequencia
		if obtido == nil {
			t.Errorf("%q ficou sem frequência", palavra)
			continue
		}
		if *obtido != esperado {
			t.Errorf("frequência de %q = %+v, esperada %+v", palavra, *obtido, esperado)
		}
	}
}

// TestAplicarFrequenciasIgnoraPalavraForaDoDicionario garante que token sem entrada não vira entrada
// nova (viria sem definição) e, sobretudo, que ele NÃO ocupa posição: o ranking é disputado só entre
// palavras que o dicionário conhece, então 好 é o 2º mesmo com "gucci" mais falado que ele.
func TestAplicarFrequenciasIgnoraPalavraForaDoDicionario(t *testing.T) {
	entradas := entradasDeTeste("的", "好")
	contagens := map[string]int64{"的": 50, "gucci": 30, "好": 20}

	comFrequencia, _ := aplicarFrequencias(entradas, contagens, 100)

	if comFrequencia != 2 {
		t.Errorf("comFrequencia = %d, esperado 2 (gucci não está no dicionário)", comFrequencia)
	}
	if _, existe := entradas["gucci"]; existe {
		t.Error("gucci virou entrada do dicionário — a frequência não pode criar entradas")
	}
	if entradas["好"].Frequencia.Posicao != 2 {
		t.Errorf("posição de 好 = %d, esperada 2 — token fora do dicionário roubou posição",
			entradas["好"].Frequencia.Posicao)
	}
}

// TestAplicarFrequenciasPercentualUsaCorpusInteiro trava a decisão mais fácil de quebrar sem
// perceber: o percentual divide pelo corpus INTEIRO, mesmo quando metade dele não casa com o
// dicionário. Dividir só pelo que casa faria 的 saltar de 25% para 50% e inflaria tudo em silêncio.
func TestAplicarFrequenciasPercentualUsaCorpusInteiro(t *testing.T) {
	entradas := entradasDeTeste("的", "我们")
	contagens := map[string]int64{"的": 25, "我们": 25, "gucci": 25, "remasterlng": 25}

	_, cobertura := aplicarFrequencias(entradas, contagens, 100)

	if entradas["的"].Frequencia.Percentual != 25 {
		t.Errorf("percentual de 的 = %g, esperado 25 — o denominador deixou de ser o corpus inteiro",
			entradas["的"].Frequencia.Percentual)
	}
	if cobertura != 50 {
		t.Errorf("cobertura = %g, esperada 50 (metade do corpus não casa com o dicionário)", cobertura)
	}
}

// TestAplicarFrequenciasEmpateDivideAhPosicao cobre o ranking de competição: contagens iguais dão a
// MESMA posição e a seguinte pula o tanto de empatados (1,1,3 — nunca 1,2,3).
func TestAplicarFrequenciasEmpateDivideAhPosicao(t *testing.T) {
	entradas := entradasDeTeste("的", "我们", "好")
	contagens := map[string]int64{"的": 40, "我们": 40, "好": 20}

	aplicarFrequencias(entradas, contagens, 100)

	if entradas["的"].Frequencia.Posicao != 1 || entradas["我们"].Frequencia.Posicao != 1 {
		t.Errorf("empatados deveriam dividir a 1ª posição: 的=%d 我们=%d",
			entradas["的"].Frequencia.Posicao, entradas["我们"].Frequencia.Posicao)
	}
	if entradas["好"].Frequencia.Posicao != 3 {
		t.Errorf("posição de 好 = %d, esperada 3 (o empate em 1º consome a 2ª)",
			entradas["好"].Frequencia.Posicao)
	}
}

// TestAplicarFrequenciasPosicaoSaiDaContagemCrua garante que a ordem vem da contagem, não do
// percentual gravado: estas duas contagens só diferem no 5º dígito significativo e colapsam no mesmo
// percentual arredondado, mas continuam sendo 1ª e 2ª — o arredondamento não pode inventar empate.
func TestAplicarFrequenciasPosicaoSaiDaContagemCrua(t *testing.T) {
	entradas := entradasDeTeste("的", "我们")
	contagens := map[string]int64{"的": 123460, "我们": 123450}

	aplicarFrequencias(entradas, contagens, 1000000)

	if entradas["的"].Frequencia.Percentual != entradas["我们"].Frequencia.Percentual {
		t.Fatalf("o teste perdeu o sentido: os percentuais deveriam colidir em %g, mas são %g e %g",
			12.35, entradas["的"].Frequencia.Percentual, entradas["我们"].Frequencia.Percentual)
	}
	if entradas["的"].Frequencia.Posicao != 1 || entradas["我们"].Frequencia.Posicao != 2 {
		t.Errorf("posições = 的:%d 我们:%d, esperadas 1 e 2 — o arredondamento inventou um empate",
			entradas["的"].Frequencia.Posicao, entradas["我们"].Frequencia.Posicao)
	}
}

// ----- Arredondamento -----

// TestArredondarSignificativos confirma que o corte é por dígitos SIGNIFICATIVOS (preserva a escala
// da cauda), não por casas decimais — que zeraria toda palavra rara.
func TestArredondarSignificativos(t *testing.T) {
	casos := []struct {
		nome     string
		valor    float64
		digitos  int
		esperado float64
	}{
		{"topo do ranking", 4.628938204103, DIGITOS_FREQUENCIA, 4.629},
		{"arredonda para cima", 0.65274999, 4, 0.6527},
		{"cauda mantém a escala", 0.00000116975, DIGITOS_FREQUENCIA, 0.00000117},
		{"já curto fica igual", 25, DIGITOS_FREQUENCIA, 25},
		{"zero segue zero", 0, DIGITOS_FREQUENCIA, 0},
	}
	for _, caso := range casos {
		t.Run(caso.nome, func(t *testing.T) {
			if obtido := arredondarSignificativos(caso.valor, caso.digitos); obtido != caso.esperado {
				t.Errorf("arredondarSignificativos(%g, %d) = %g, esperado %g",
					caso.valor, caso.digitos, obtido, caso.esperado)
			}
		})
	}
}

// ----- Remoção do pinyin colado ao hanzi nas glosas do CEDICT -----

// TestRemoverPinyinAposHanziSoLimpaDepoisDeHanzi cobre os quatro casos que separam ruído de conteúdo:
// colchete colado a hanzi sai, colchete solto fica, a corrida "trad|simp" é respeitada, e colchetes
// em sequência somem todos.
func TestRemoverPinyinAposHanziSoLimpaDepoisDeHanzi(t *testing.T) {
	casos := []struct {
		nome     string
		entrada  string
		esperado string
	}{
		{"referência simples", "variant of 臺[tai2]", "variant of 臺"},
		{"corrida trad|simp", "CL:個|个[ge4]", "CL:個|个"},
		{"forma completa da citação", "這裡|这里[zhe4 li3]", "這裡|这里"},
		{"colchete solto é conteúdo", "Taiwan pr. [tai2]", "Taiwan pr. [tai2]"},
		{"colchetes em sequência", "臺[tai2][wan1]", "臺"},
		{"glosa sem hanzi passa intacta", "aim, goal; of", "aim, goal; of"},
	}
	for _, caso := range casos {
		t.Run(caso.nome, func(t *testing.T) {
			if obtido := removerPinyinAposHanzi(caso.entrada); obtido != caso.esperado {
				t.Errorf("removerPinyinAposHanzi(%q) = %q, esperado %q", caso.entrada, obtido, caso.esperado)
			}
		})
	}
}

// TestRemoverPinyinDosSignificadosLimpaCadaAcepcao garante que a limpeza cai sobre cada significado da
// leitura, não só sobre o primeiro.
func TestRemoverPinyinDosSignificadosLimpaCadaAcepcao(t *testing.T) {
	limpos := removerPinyinDosSignificados([]string{"variant of 臺[tai2]", "CL:個|个[ge4]", "here"})

	esperado := []string{"variant of 臺", "CL:個|个", "here"}
	for i := range esperado {
		if limpos[i] != esperado[i] {
			t.Errorf("significado %d = %q, esperado %q", i, limpos[i], esperado[i])
		}
	}
}

// ----- Prioridade negativa da leitura secundária (variant of / surname) -----

// TestLeituraEhVarianteExigeTodosOsSignificados: só é variante quem NÃO carrega conteúdo próprio —
// todos os significados têm que ser ponteiros. As formas "old/archaic variant of" também contam.
func TestLeituraEhVarianteExigeTodosOsSignificados(t *testing.T) {
	casos := []struct {
		nome         string
		significados []string
		esperado     bool
	}{
		{"variante pura", []string{"variant of 這裡|这里"}, true},
		{"variante antiga também conta", []string{"old variant of 於|于"}, true},
		{"tem conteúdo junto não é variante", []string{"variant of X", "to like"}, false},
		{"conteúdo real não é variante", []string{"here"}, false},
		{"leitura vazia não é variante", nil, false},
	}
	for _, caso := range casos {
		t.Run(caso.nome, func(t *testing.T) {
			if obtido := leituraEhVariante(caso.significados); obtido != caso.esperado {
				t.Errorf("leituraEhVariante(%q) = %v, esperado %v", caso.significados, obtido, caso.esperado)
			}
		})
	}
}

// TestLeituraEhSobrenomeExigeSoSobrenome: só é sobrenome quem NÃO carrega conteúdo próprio — todos os
// significados têm que ser "surname" mais o nome (até MAX_PALAVRAS_ALEM_DO_SOBRENOME palavras).
func TestLeituraEhSobrenomeExigeSoSobrenome(t *testing.T) {
	casos := []struct {
		nome         string
		significados []string
		esperado     bool
	}{
		{"sobrenome simples", []string{"surname Wang"}, true},
		{"sobrenome só a palavra", []string{"surname"}, true},
		{"sobrenome de dois nomes cabe no limite", []string{"surname Ouyang Xiu"}, true},
		{"tem conteúdo junto não é sobrenome", []string{"king", "surname Wang"}, false},
		{"palavras demais depois de surname", []string{"surname of a mythical emperor"}, false},
		{"surname no meio não conta", []string{"two-character surname Sima"}, false},
		{"conteúdo real não é sobrenome", []string{"beautiful"}, false},
		{"leitura vazia não é sobrenome", nil, false},
	}
	for _, caso := range casos {
		t.Run(caso.nome, func(t *testing.T) {
			if obtido := leituraEhSobrenome(caso.significados); obtido != caso.esperado {
				t.Errorf("leituraEhSobrenome(%q) = %v, esperado %v", caso.significados, obtido, caso.esperado)
			}
		})
	}
}

// TestLeituraEhSecundariaUneVarianteEhSobrenome: a leitura que cede prioridade é a variante OU a de
// sobrenome; conteúdo próprio não cede.
func TestClassificarLeituraDistingueVarianteEhSobrenome(t *testing.T) {
	casos := []struct {
		nome         string
		significados []string
		esperado     string
	}{
		{"variante", []string{"variant of 這裡|这里"}, dicionario.TIPO_LEITURA_VARIANTE},
		{"sobrenome", []string{"surname Wang"}, dicionario.TIPO_LEITURA_SOBRENOME},
		{"conteúdo não recebe rótulo", []string{"here"}, ""},
		{"mistura com conteúdo é conteúdo", []string{"surname Wang", "king"}, ""},
	}
	for _, caso := range casos {
		t.Run(caso.nome, func(t *testing.T) {
			if obtido := classificarLeitura(caso.significados); obtido != caso.esperado {
				t.Errorf("classificarLeitura(%q) = %q, esperado %q", caso.significados, obtido, caso.esperado)
			}
		})
	}
}

// TestCompactarEntradaSobrenomeCedeAscensao espelha a regra da variante para o sobrenome: a 1ª leitura
// só diz "surname", a 2ª tem conteúdo — quem sobe ao topo é a 2ª, e o sobrenome fica aninhado.
func TestCompactarEntradaSobrenomeCedeAscensao(t *testing.T) {
	e := &EntradaFundida{
		Simplificado: "X",
		Leituras: []Leitura{
			{Pinyin: "wáng", Significados: []string{"surname Wang"}},
			{Pinyin: "wáng", Significados: []string{"king; monarch"}},
		},
	}

	c := compactarEntrada(e, []string{dicionario.TIPO_LEITURA_SOBRENOME, ""})

	if e.Definicao != "king; monarch" {
		t.Fatalf("definição do topo = %q, esperada \"king; monarch\" (o sobrenome não podia subir)", e.Definicao)
	}
	if c.promovidas != 1 || c.promocoesDesviadas != 1 {
		t.Fatalf("contadores = %+v, esperava promovidas=1 e promocoesDesviadas=1", c)
	}
	if len(e.Leituras) != 1 || len(e.Leituras[0].Significados) != 1 || e.Leituras[0].Significados[0] != "surname Wang" {
		t.Fatalf("o sobrenome tinha que sobrar aninhado, veio %+v", e.Leituras)
	}
	if e.Leituras[0].Tipo != dicionario.TIPO_LEITURA_SOBRENOME {
		t.Fatalf("a leitura aninhada tinha que carregar o rótulo de sobrenome, veio %q", e.Leituras[0].Tipo)
	}
}

// TestCompactarEntradaVarianteCedeAscensao é o coração da regra: a 1ª leitura só diz "variant of", a
// 2ª tem conteúdo — quem sobe ao topo é a 2ª, e a variante fica aninhada.
func TestCompactarEntradaVarianteCedeAscensao(t *testing.T) {
	e := &EntradaFundida{
		Simplificado: "X",
		Leituras: []Leitura{
			{Pinyin: "biàn", Significados: []string{"variant of Y"}},
			{Pinyin: "zhèng", Significados: []string{"real meaning"}},
		},
	}

	c := compactarEntrada(e, []string{dicionario.TIPO_LEITURA_VARIANTE, ""})

	if e.Definicao != "real meaning" {
		t.Fatalf("definição do topo = %q, esperada \"real meaning\" (a variante não podia subir)", e.Definicao)
	}
	if len(e.Pinyin) != 1 || e.Pinyin[0] != "zhèng" {
		t.Fatalf("pinyin do topo = %v, esperado [zhèng]", e.Pinyin)
	}
	if c.promovidas != 1 || c.promocoesDesviadas != 1 {
		t.Fatalf("contadores = %+v, esperava promovidas=1 e promocoesDesviadas=1", c)
	}
	if len(e.Leituras) != 1 || len(e.Leituras[0].Significados) != 1 || e.Leituras[0].Significados[0] != "variant of Y" {
		t.Fatalf("a variante tinha que sobrar aninhada, veio %+v", e.Leituras)
	}
	if e.Leituras[0].Tipo != dicionario.TIPO_LEITURA_VARIANTE {
		t.Fatalf("a leitura aninhada tinha que carregar o rótulo de variante, veio %q", e.Leituras[0].Tipo)
	}
}

// TestCompactarEntradaTodasVariantesPromoveAhPrimeira: quando não há leitura de conteúdo, promove a
// primeira mesmo — é tudo que a palavra tem, e não conta como desvio.
func TestCompactarEntradaTodasVariantesPromoveAhPrimeira(t *testing.T) {
	e := &EntradaFundida{
		Simplificado: "X",
		Leituras: []Leitura{
			{Pinyin: "a", Significados: []string{"variant of Y"}},
			{Pinyin: "b", Significados: []string{"old variant of Z"}},
		},
	}

	c := compactarEntrada(e, []string{dicionario.TIPO_LEITURA_VARIANTE, dicionario.TIPO_LEITURA_VARIANTE})

	if e.Definicao != "variant of Y" {
		t.Fatalf("definição do topo = %q, esperada \"variant of Y\" (a 1ª leitura)", e.Definicao)
	}
	if c.promovidas != 1 || c.promocoesDesviadas != 0 {
		t.Fatalf("contadores = %+v, esperava promovidas=1 e promocoesDesviadas=0", c)
	}
	if len(e.Leituras) != 1 || e.Leituras[0].Tipo != dicionario.TIPO_LEITURA_VARIANTE {
		t.Fatalf("a 2ª variante tinha que sobrar aninhada e rotulada, veio %+v", e.Leituras)
	}
}

// TestCompactarEntradaSemFlagsMantemAhPrimeira trava a compatibilidade: sem informação de variante
// (idioma desalinhado ou palavra fora do original), o comportamento antigo — promover a 1ª — segue.
func TestCompactarEntradaSemFlagsMantemAhPrimeira(t *testing.T) {
	e := &EntradaFundida{
		Simplificado: "X",
		Leituras: []Leitura{
			{Pinyin: "a", Significados: []string{"first"}},
			{Pinyin: "b", Significados: []string{"second"}},
		},
	}

	c := compactarEntrada(e, nil)

	if e.Definicao != "first" || c.promocoesDesviadas != 0 {
		t.Fatalf("sem flags, a 1ª leitura tinha que subir sem desvio: def=%q %+v", e.Definicao, c)
	}
}

// TestCompactarEntradaVarianteAindaEvitaGrafiaDivergente reproduz o caso 这里: a leitura de conteúdo
// escolhida tem grafia própria e as grafias divergem, então a promoção é evitada e tudo fica aninhado
// — a regra da variante não atropela a proteção da associação tradicional↔leitura.
func TestCompactarEntradaVarianteAindaEvitaGrafiaDivergente(t *testing.T) {
	e := &EntradaFundida{
		Simplificado: "这里",
		Tradicional:  []string{"這裏", "這裡"},
		Leituras: []Leitura{
			{Pinyin: "zhè lǐ", Tradicional: "這裏", Significados: []string{"variant of 這裡"}},
			{Pinyin: "zhè lǐ", Tradicional: "這裡", Significados: []string{"here"}},
		},
	}

	c := compactarEntrada(e, []string{dicionario.TIPO_LEITURA_VARIANTE, ""})

	if e.Definicao != "" || c.promovidas != 0 || c.promocoesEvitadas != 1 {
		t.Fatalf("这里 tinha que ficar aninhado sem topo: def=%q %+v", e.Definicao, c)
	}
	if len(e.Leituras) != 2 {
		t.Fatalf("as duas leituras tinham que sobreviver aninhadas, veio %d", len(e.Leituras))
	}
	// Mesmo com a promoção evitada (early return), o rótulo tem que ter sido gravado antes.
	if e.Leituras[0].Tipo != dicionario.TIPO_LEITURA_VARIANTE || e.Leituras[1].Tipo != "" {
		t.Fatalf("os rótulos tinham que sobreviver ao early return: %q / %q", e.Leituras[0].Tipo, e.Leituras[1].Tipo)
	}
}

// ----- Compactação: remoção de redundâncias -----

// TestCompactarEntradaRemovePinyinRedundanteIgnorandoCaixa cobre o caso 邷: o topo curado do makemeahanzi
// capitaliza o pinyin de lugar ("Wǎ") e a leitura do CEDICT o traz minúsculo ("wǎ") — é a mesma sílaba,
// então o pinyin da leitura é redundante e sai (a reconstrução o repõe do topo). Uma leitura de OUTRO
// tom ("wà") não pode ser tocada, senão a compactação fundiria leituras distintas.
func TestCompactarEntradaRemovePinyinRedundanteIgnorandoCaixa(t *testing.T) {
	e := &EntradaFundida{
		Simplificado: "邷",
		Definicao:    "old place name",
		Pinyin:       []string{"Wǎ"},
		Leituras: []Leitura{
			{Pinyin: "wǎ", Significados: []string{"to grab"}},
			{Pinyin: "wà", Significados: []string{"outra leitura"}},
		},
	}

	c := compactarEntrada(e, nil)

	if c.pinyinsLimpos != 1 {
		t.Fatalf("pinyinsLimpos = %d, esperado 1 (só o wǎ difere apenas na caixa)", c.pinyinsLimpos)
	}
	if len(e.Leituras) != 2 {
		t.Fatalf("as duas leituras tinham que sobreviver (significados próprios), veio %d", len(e.Leituras))
	}
	if e.Leituras[0].Pinyin != "" {
		t.Fatalf("o pinyin redundante wǎ tinha que sair, veio %q", e.Leituras[0].Pinyin)
	}
	if e.Leituras[1].Pinyin != "wà" {
		t.Fatalf("a leitura de tom distinto não podia perder o pinyin, veio %q", e.Leituras[1].Pinyin)
	}
}

// ----- Apoio -----

// entradasDeTeste monta o mapa de entradas da fusão com as palavras dadas, cada uma só com a chave
// simplificada — é tudo que a frequência olha.
func entradasDeTeste(palavras ...string) map[string]*EntradaFundida {
	entradas := make(map[string]*EntradaFundida, len(palavras))
	for _, palavra := range palavras {
		entradas[palavra] = &EntradaFundida{Simplificado: palavra}
	}
	return entradas
}

// escreverListaFrequencia grava um .txt.gz temporário no formato da lista do OpenSubtitles
// ("palavra<espaço>contagem") e devolve seu caminho.
func escreverListaFrequencia(t *testing.T, conteudo string) string {
	t.Helper()

	caminho := filepath.Join(t.TempDir(), "frequencia.txt.gz")
	arquivo, err := os.Create(caminho)
	if err != nil {
		t.Fatalf("criar lista de frequência temporária: %v", err)
	}
	defer arquivo.Close()

	escritorGz := gzip.NewWriter(arquivo)
	if _, err := escritorGz.Write([]byte(conteudo)); err != nil {
		t.Fatalf("escrever lista de frequência temporária: %v", err)
	}
	if err := escritorGz.Close(); err != nil {
		t.Fatalf("fechar gzip da lista temporária: %v", err)
	}
	return caminho
}
