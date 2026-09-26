package busca

import (
	"fmt"
	"strings"
	"testing"

	"wails_app/config"
	"wails_app/dicionario"
)

func entradaDeTeste(caractere, pinyin, definicao string) dicionario.DecomposicaoHanzi {
	return dicionario.DecomposicaoHanzi{Caractere: caractere, Pinyin: []string{pinyin}, Definicao: definicao}
}

func conjunto(chars ...string) map[string]bool {
	s := make(map[string]bool, len(chars))
	for _, c := range chars {
		s[c] = true
	}
	return s
}

// ----- Motor de busca (BuscarDeFontes) -----

func TestBuscarDeFontesPercorreCascataComCriterioEhMeta(t *testing.T) {
	fontes := []FonteBusca[int]{
		{Sequencia: SequenciaOrdenada([]int{1, 2, 3})},
		{Sequencia: SequenciaOrdenada([]int{4, 5, 6})},
	}
	aceitarPares := func(escolhidas []int, candidata int) bool { return candidata%2 == 0 }

	escolhidas := BuscarDeFontesSimples(3, nil, fontes, aceitarPares)

	esperados := []int{2, 4, 6}
	if len(escolhidas) != len(esperados) {
		t.Fatalf("esperava %v, veio %v", esperados, escolhidas)
	}
	for i, esperado := range esperados {
		if escolhidas[i] != esperado {
			t.Errorf("posição %d: esperava %d, veio %d (seleção: %v)", i, esperado, escolhidas[i], escolhidas)
		}
	}
}

func TestBuscarDeFontesContaIniciaisNaMeta(t *testing.T) {
	fontes := []FonteBusca[int]{{Sequencia: SequenciaOrdenada([]int{10, 20})}}

	escolhidas := BuscarDeFontesSimples(3, []int{1, 2}, fontes, nil)

	esperados := []int{1, 2, 10}
	if len(escolhidas) != len(esperados) {
		t.Fatalf("iniciais devem ocupar vagas da meta: esperava %v, veio %v", esperados, escolhidas)
	}
	for i, esperado := range esperados {
		if escolhidas[i] != esperado {
			t.Errorf("posição %d: esperava %d, veio %d", i, esperado, escolhidas[i])
		}
	}
}

func TestBuscarDeFontesRespeitaMaximoPorFonte(t *testing.T) {
	fontes := []FonteBusca[int]{
		{Sequencia: SequenciaOrdenada([]int{1, 2, 3}), Maximo: 1},
		{Sequencia: SequenciaOrdenada([]int{7, 8, 9})},
	}

	escolhidas := BuscarDeFontesSimples(3, nil, fontes, nil)

	esperados := []int{1, 7, 8}
	if len(escolhidas) != len(esperados) {
		t.Fatalf("esperava %v, veio %v", esperados, escolhidas)
	}
	for i, esperado := range esperados {
		if escolhidas[i] != esperado {
			t.Errorf("posição %d: esperava %d, veio %d (a fonte com teto 1 não pode dar 2 itens)", i, esperado, escolhidas[i])
		}
	}
}

func TestBuscarDeFontesNaoProduzFonteAposAhMeta(t *testing.T) {
	fallbackConsumido := false
	fontes := []FonteBusca[int]{
		{Sequencia: SequenciaOrdenada([]int{1, 2})},
		{Sequencia: SequenciaPreguicosa(func() []int {
			fallbackConsumido = true
			return []int{99}
		})},
	}

	escolhidas := BuscarDeFontesSimples(2, nil, fontes, nil)

	if len(escolhidas) != 2 {
		t.Fatalf("esperava 2 escolhidas, veio %v", escolhidas)
	}
	if fallbackConsumido {
		t.Error("a fonte de fallback não deveria ser produzida quando a meta já foi atingida")
	}
}

func TestBuscarDeFontesChanceZeroEhUmSempreTentam(t *testing.T) {
	for _, chance := range []float64{0, 1} {
		fontes := []FonteBusca[int]{{Sequencia: SequenciaOrdenada([]int{7}), Chance: chance}}
		escolhidas := BuscarDeFontesSimples(1, nil, fontes, nil)
		if len(escolhidas) != 1 || escolhidas[0] != 7 {
			t.Errorf("chance %v deveria sempre tentar a fonte, veio %v", chance, escolhidas)
		}
	}
}

func TestBuscarDeFontesChanceProbabilisticaVariaAhComposicao(t *testing.T) {
	const tentativas = 2000
	usos := 0
	for i := 0; i < tentativas; i++ {
		fontes := []FonteBusca[int]{
			{Sequencia: SequenciaOrdenada([]int{1}), Chance: 0.5, Maximo: 1},
			{Sequencia: SequenciaOrdenada([]int{2})},
		}
		escolhidas := BuscarDeFontesSimples(1, nil, fontes, nil)
		if escolhidas[0] == 1 {
			usos++
		}
	}
	// Faixa larga (30%–70%) para a invariante estatística não flutuar.
	if usos < tentativas*3/10 || usos > tentativas*7/10 {
		t.Errorf("fonte com chance 0.5 deveria ser usada em ~metade das buscas, veio %d/%d", usos, tentativas)
	}
}

func TestBuscarDeFontesConverteCandidataEmSelecao(t *testing.T) {
	fontes := []FonteBusca[dicionario.DecomposicaoHanzi]{
		{Sequencia: SequenciaOrdenada([]dicionario.DecomposicaoHanzi{entradaDeTeste("你", "ni3", "you")})},
	}

	opcoes := BuscarDeFontes(
		2,
		[]OpcaoRevisao{novaOpcao(entradaDeTeste("好", "hao3", "good"), true)},
		fontes,
		nil,
		novaOpcaoDistratora,
	)

	if len(opcoes) != 2 {
		t.Fatalf("esperava 2 opções, veio %d", len(opcoes))
	}
	if !opcoes[0].Correta || opcoes[0].Hanzi != "好" {
		t.Errorf("a inicial deveria seguir correta: %+v", opcoes[0])
	}
	if opcoes[1].Correta || opcoes[1].Hanzi != "你" {
		t.Errorf("a convertida deveria ser distratora de 你: %+v", opcoes[1])
	}
}

// ----- Sequências de candidatas -----

func TestSequenciaEmbaralhadaEhPermutacaoInterrompivel(t *testing.T) {
	lista := []int{1, 2, 3, 4, 5}

	vistos := make(map[int]bool)
	for v := range SequenciaEmbaralhada(lista) {
		if vistos[v] {
			t.Fatalf("elemento %d repetido na permutação", v)
		}
		vistos[v] = true
	}
	if len(vistos) != len(lista) {
		t.Fatalf("esperava percorrer os %d elementos, veio %d", len(lista), len(vistos))
	}

	consumidos := 0
	for range SequenciaEmbaralhada(lista) {
		consumidos++
		if consumidos == 2 {
			break
		}
	}
	if consumidos != 2 {
		t.Errorf("a sequência deveria parar quando o consumidor interrompe, veio %d", consumidos)
	}
}

// ----- Critérios de frase por variante -----

func TestCriteriosFraseDaVarianteFamiliaOrdenacao(t *testing.T) {
	for _, variante := range []string{VarianteOrdenacao, VarianteOrdenacaoTraducao} {
		criterios := CriteriosFraseDaVariante(variante, "好")
		if criterios.ExigenciaCobertura != exigenciaFraseDeOrdenacao {
			t.Errorf("%s: esperava exigência de frase de ordenação, veio %q", variante, criterios.ExigenciaCobertura)
		}
		if sig := criterios.Assinatura("你好"); sig != "ordenacao:你好" {
			t.Errorf("%s: a família de ordenação compartilha a assinatura da frase, veio %q", variante, sig)
		}
	}
}

func TestCriteriosFraseDaVarianteFoneticaLimitaPalavras(t *testing.T) {
	for _, variante := range []string{VarianteFoneticaFrase, VarianteFoneticaTraducao} {
		criterios := CriteriosFraseDaVariante(variante, "好")
		if criterios.MaxPalavras != MaxPalavrasFraseFonetica {
			t.Errorf("%s: esperava MaxPalavras %d, veio %d", variante, MaxPalavrasFraseFonetica, criterios.MaxPalavras)
		}
	}
}

func TestCriteriosFraseDaVariantePronuncia(t *testing.T) {
	frase := CriteriosFraseDaVariante(VariantePronunciaFrase, "好")
	if frase.ExigenciaCobertura != exigenciaCoberturaAlta || frase.ExigirTodosConhecidos || frase.SelecaoPorContagemDeEstudo {
		t.Errorf("pronuncia_frase: critérios inesperados: %+v", frase)
	}
	if sig := frase.Assinatura("你好"); sig != VariantePronunciaFrase+":你好" {
		t.Errorf("pronuncia_frase: assinatura por frase, veio %q", sig)
	}

	sequencia := CriteriosFraseDaVariante(VariantePronunciaSequencia, "好")
	if sequencia.ExigenciaCobertura != exigenciaCoberturaAlta || !sequencia.ExigirTodosConhecidos || !sequencia.SelecaoPorContagemDeEstudo {
		t.Errorf("pronuncia_sequencia: critérios inesperados: %+v", sequencia)
	}
}

func TestCriteriosFraseDaVariantePadraoIncluiAlvoNaAssinatura(t *testing.T) {
	criterios := CriteriosFraseDaVariante(VarianteContexto, "好")
	if criterios.ExigenciaCobertura != exigenciaCoberturaPreferencial || criterios.ExigirTodosConhecidos {
		t.Errorf("contexto: critérios inesperados: %+v", criterios)
	}
	if sig := criterios.Assinatura("你好"); sig != "contexto:好:你好" {
		t.Errorf("contexto: assinatura por variante+alvo+frase, veio %q", sig)
	}
}

// ----- Critérios de aceitação de peças -----

func TestAceitarPecaDistratora(t *testing.T) {
	aceitar := AceitarPecaDistratora(conjunto("你"), map[string]bool{"ni3": true})
	escolhidas := []ElementoOrdenacao{{Texto: "水", Pinyin: "shui3"}}

	casos := []struct {
		nome      string
		candidata ElementoOrdenacao
		esperado  bool
	}{
		{"hanzi presente na frase", ElementoOrdenacao{Texto: "你好", Pinyin: "ni3hao3"}, false},
		{"pinyin colide com peça correta", ElementoOrdenacao{Texto: "泥", Pinyin: "Ni 3"}, false},
		{"peça repetida", ElementoOrdenacao{Texto: "水", Pinyin: "shui3"}, false},
		{"peça válida", ElementoOrdenacao{Texto: "火", Pinyin: "huo3"}, true},
	}
	for _, caso := range casos {
		if got := aceitar(escolhidas, caso.candidata); got != caso.esperado {
			t.Errorf("%s: esperava %v, veio %v", caso.nome, caso.esperado, got)
		}
	}
}

func TestAceitarPalavraInglesaDistratora(t *testing.T) {
	aceitar := AceitarPalavraInglesaDistratora(map[string]bool{"love": true})
	escolhidas := []ElementoOrdenacao{{Texto: "Mother"}}

	casos := []struct {
		candidata string
		esperado  bool
	}{
		{"Love", false},   // palavra da tradução correta (sem diferenciar maiúsculas)
		{"mother", false}, // peça já escolhida
		{"water", true},
	}
	for _, caso := range casos {
		if got := aceitar(escolhidas, caso.candidata); got != caso.esperado {
			t.Errorf("%q: esperava %v, veio %v", caso.candidata, caso.esperado, got)
		}
	}
}

// ----- Pools da sessão -----

func TestPoolsDoModoSeparaPorStatus(t *testing.T) {
	candidatos := []dicionario.DecomposicaoHanzi{
		entradaDeTeste("一", "yi1", "one"),
		entradaDeTeste("二", "er4", "two"),
		entradaDeTeste("三", "san1", "three"),
	}
	mapaStatus := map[string]string{
		"一": dicionario.StatusEstudo,
		"二": dicionario.StatusAprendido,
	}

	pools := PoolsDoModo(candidatos, mapaStatus)

	if len(pools.Todos) != 3 {
		t.Errorf("Todos deveria ter os 3 candidatos, veio %d", len(pools.Todos))
	}
	if len(pools.EmEstudo) != 1 || pools.EmEstudo[0].Caractere != "一" {
		t.Errorf("EmEstudo esperava [一], veio %+v", pools.EmEstudo)
	}
	if len(pools.Aprendidos) != 1 || pools.Aprendidos[0].Caractere != "二" {
		t.Errorf("Aprendidos esperava [二], veio %+v", pools.Aprendidos)
	}
}

// ----- Planos de opções e cartas -----

func TestBuscarOpcoesCaractereMontaOpcoesUnicasComUmaCorreta(t *testing.T) {
	alvo := entradaDeTeste("一", "yi1", "one")
	pools := PoolsDoModo([]dicionario.DecomposicaoHanzi{
		alvo,
		entradaDeTeste("二", "er4", "two"),
		entradaDeTeste("三", "san1", "three"),
		entradaDeTeste("四", "si4", "four"),
		entradaDeTeste("五", "wu3", "five"),
		entradaDeTeste("六", "liu4", "six"),
	}, map[string]string{"二": dicionario.StatusEstudo, "三": dicionario.StatusAprendido})

	b := buscadorDeTeste(nil)
	for iteracao := 0; iteracao < 30; iteracao++ {
		opcoes := b.BuscarOpcoes(alvo, false, pools, DistintosPorSignificado)
		if len(opcoes) != TotalOpcoesMultiplaEscolha {
			t.Fatalf("esperava %d opções, veio %d", TotalOpcoesMultiplaEscolha, len(opcoes))
		}

		corretas := 0
		glosas := make(map[string]bool)
		for _, o := range opcoes {
			if o.Correta {
				corretas++
				if o.Hanzi != alvo.Caractere {
					t.Errorf("opção correta aponta para %q", o.Hanzi)
				}
			}
			if glosas[GlosaPrincipal(o.Definicao)] {
				t.Errorf("glosa repetida entre as opções: %+v", opcoes)
			}
			glosas[GlosaPrincipal(o.Definicao)] = true
		}
		if corretas != 1 {
			t.Fatalf("esperava exatamente 1 correta, veio %d", corretas)
		}
	}
}

func TestBuscarOpcoesComAlvoEmFocoGaranteDistratorEmFoco(t *testing.T) {
	alvo := entradaDeTeste("一", "yi1", "one")
	distractorFoco := entradaDeTeste("二", "er4", "two")
	distractorForaFoco := entradaDeTeste("三", "san1", "three")
	distractorExtra := entradaDeTeste("四", "si4", "four")

	pools := PoolsDoModo([]dicionario.DecomposicaoHanzi{
		alvo,
		distractorFoco,
		distractorForaFoco,
		distractorExtra,
	}, map[string]string{
		"一": dicionario.StatusEstudo,
		"二": dicionario.StatusEstudo,
		"三": dicionario.StatusAprendido,
		"四": dicionario.StatusAprendido,
	})

	b := buscadorDeTeste(nil)
	b.IniciarSessao(
		map[string]string{"一": dicionario.StatusEstudo, "二": dicionario.StatusEstudo},
		nil, nil,
		[]string{"一", "二"}, // "一" (alvo) e "二" estão em foco
		3,
	)

	opcoes := b.BuscarOpcoes(alvo, true, pools, DistintosPorPinyin)
	if len(opcoes) != TotalOpcoesMultiplaEscolha {
		t.Fatalf("esperava %d opções, veio %d", TotalOpcoesMultiplaEscolha, len(opcoes))
	}

	temDistratorFoco := false
	for _, o := range opcoes {
		if !o.Correta && o.Hanzi == distractorFoco.Caractere {
			temDistratorFoco = true
			break
		}
	}

	if !temDistratorFoco {
		t.Errorf("esperava que ao menos um distrator fosse a palavra em foco %q, opções vieram: %+v", distractorFoco.Caractere, opcoes)
	}
}

// buscadorDeTeste monta um Buscador sem dependências reais: só o estado de sessão (status, foco,
// meta) e a função de estatísticas injetada importam para os planos testados aqui.
func buscadorDeTeste(estatisticas func(string) map[string]int) *Buscador {
	return Novo(nil, nil, nil, nil, nil, estatisticas)
}

func TestBuscarPecasQuebraCabecaTodasCorretasEhDistintas(t *testing.T) {
	alvo := entradaDeTeste("一", "yi1", "one")
	pools := PoolsDoModo([]dicionario.DecomposicaoHanzi{
		alvo,
		entradaDeTeste("二", "er4", "two"),
		entradaDeTeste("三", "san1", "three"),
		entradaDeTeste("四", "si4", "four"),
		entradaDeTeste("五", "wu3", "five"),
		entradaDeTeste("六", "liu4", "six"),
		entradaDeTeste("七", "qi1", "seven"),
	}, nil)

	pecas := buscadorDeTeste(nil).BuscarPecasQuebraCabeca(alvo, pools)
	if len(pecas) != TotalPecasQuebraCabeca {
		t.Fatalf("esperava %d peças, veio %d", TotalPecasQuebraCabeca, len(pecas))
	}
	glosas := make(map[string]bool)
	temAlvo := false
	for _, p := range pecas {
		if !p.Correta {
			t.Errorf("toda peça do quebra-cabeça é um par correto: %+v", p)
		}
		if glosas[GlosaPrincipal(p.Definicao)] {
			t.Errorf("glosa repetida no quebra-cabeça: %+v", pecas)
		}
		glosas[GlosaPrincipal(p.Definicao)] = true
		if p.Hanzi == alvo.Caractere {
			temAlvo = true
		}
	}
	if !temAlvo {
		t.Error("o alvo deveria estar entre as peças")
	}
}

func TestBuscarPecasQuebraCabecaIncompletoSemDistintosSuficientes(t *testing.T) {
	alvo := entradaDeTeste("一", "yi1", "one")
	pools := PoolsDoModo([]dicionario.DecomposicaoHanzi{
		alvo,
		entradaDeTeste("二", "er4", "one"), // glosa repetida do alvo: rejeitada
		entradaDeTeste("三", "san1", "three"),
	}, nil)

	pecas := buscadorDeTeste(nil).BuscarPecasQuebraCabeca(alvo, pools)
	if len(pecas) >= TotalPecasQuebraCabeca {
		t.Fatalf("sem candidatas distintas suficientes deveria vir incompleto, veio %d", len(pecas))
	}
}

// montarSessaoQuotasPecas prepara o cenário-padrão dos testes de quota: 焦/点 em foco (e estudo),
// 一/二/三 aprendidas pendentes, 四 aprendida com a área de significado maximizada e 五/六/七 em
// estudo fora do foco. Devolve o buscador com a sessão iniciada (meta 3) e os pools do modo.
func montarSessaoQuotasPecas(alvo dicionario.DecomposicaoHanzi) (*Buscador, Pools) {
	candidatos := []dicionario.DecomposicaoHanzi{
		entradaDeTeste("焦", "jiao1", "burnt"),
		entradaDeTeste("点", "dian3", "dot"),
		entradaDeTeste("一", "yi1", "one"),
		entradaDeTeste("二", "er4", "two"),
		entradaDeTeste("三", "san1", "three"),
		entradaDeTeste("四", "si4", "four"),
		entradaDeTeste("五", "wu3", "five"),
		entradaDeTeste("六", "liu4", "six"),
		entradaDeTeste("七", "qi1", "seven"),
	}
	if !contemCaractere(candidatos, alvo.Caractere) {
		candidatos = append(candidatos, alvo)
	}
	mapaStatus := map[string]string{
		"焦": dicionario.StatusEstudo,
		"点": dicionario.StatusEstudo,
		"一": dicionario.StatusAprendido,
		"二": dicionario.StatusAprendido,
		"三": dicionario.StatusAprendido,
		"四": dicionario.StatusAprendido,
		"五": dicionario.StatusEstudo,
		"六": dicionario.StatusEstudo,
		"七": dicionario.StatusEstudo,
	}

	estatisticas := func(palavra string) map[string]int {
		if palavra == "四" {
			return map[string]int{ModoSignificado: 3, ModoFonetica: 3}
		}
		return map[string]int{}
	}

	b := buscadorDeTeste(estatisticas)
	b.IniciarSessao(mapaStatus, nil, nil, []string{"焦", "点"}, 3)
	return b, PoolsDoModo(candidatos, mapaStatus)
}

func contarCategoriasPecas(pecas []OpcaoRevisao) (foco, aprendidasPendentes, estudoForaFoco int, temMaximizada bool) {
	ehFoco := conjunto("焦", "点")
	ehAprendidaPendente := conjunto("一", "二", "三")
	ehEstudoForaFoco := conjunto("五", "六", "七")
	for _, p := range pecas {
		switch {
		case ehFoco[p.Hanzi]:
			foco++
		case ehAprendidaPendente[p.Hanzi]:
			aprendidasPendentes++
		case ehEstudoForaFoco[p.Hanzi]:
			estudoForaFoco++
		case p.Hanzi == "四":
			temMaximizada = true
		}
	}
	return foco, aprendidasPendentes, estudoForaFoco, temMaximizada
}

func TestBuscarPecasQuebraCabecaQuotasPorCategoria(t *testing.T) {
	alvo := entradaDeTeste("焦", "jiao1", "burnt")
	b, pools := montarSessaoQuotasPecas(alvo)

	for iteracao := 0; iteracao < 30; iteracao++ {
		pecas := b.BuscarPecasQuebraCabeca(alvo, pools)
		if len(pecas) != TotalPecasQuebraCabeca {
			t.Fatalf("esperava %d peças, veio %d", TotalPecasQuebraCabeca, len(pecas))
		}

		foco, aprendidasPendentes, estudoForaFoco, _ := contarCategoriasPecas(pecas)
		// alvo (焦) em foco abate a quota de foco: sobra alvo + 1 aprendida pendente + 1 em estudo +
		// 1 vaga livre (fallback). Cada categoria vem com pelo menos a quota mínima.
		if foco < PecasQuotaMinima {
			t.Errorf("esperava ao menos %d peças em foco (alvo incluído), veio %d: %+v", PecasQuotaMinima, foco, pecas)
		}
		if aprendidasPendentes < PecasQuotaMinima {
			t.Errorf("esperava ao menos %d aprendidas pendentes, veio %d: %+v", PecasQuotaMinima, aprendidasPendentes, pecas)
		}
		if estudoForaFoco < PecasQuotaMinima {
			t.Errorf("esperava ao menos %d em estudo fora do foco, veio %d: %+v", PecasQuotaMinima, estudoForaFoco, pecas)
		}
	}
}

func TestBuscarPecasQuebraCabecaAlvoSemCategoriaNaoAbateQuota(t *testing.T) {
	alvo := entradaDeTeste("零", "ling2", "zero") // fora de foco/aprendido/estudo
	b, pools := montarSessaoQuotasPecas(alvo)

	for iteracao := 0; iteracao < 30; iteracao++ {
		pecas := b.BuscarPecasQuebraCabeca(alvo, pools)
		if len(pecas) != TotalPecasQuebraCabeca {
			t.Fatalf("esperava %d peças, veio %d", TotalPecasQuebraCabeca, len(pecas))
		}

		foco, aprendidasPendentes, estudoForaFoco, temMaximizada := contarCategoriasPecas(pecas)
		// alvo (零) fora de qualquer categoria não abate quota: as 4 vagas são alvo + 1 foco +
		// 1 aprendida pendente + 1 em estudo, preenchidas exatamente (sem fallback).
		if foco != PecasQuotaMinima || aprendidasPendentes != PecasQuotaMinima {
			t.Errorf("alvo sem categoria deveria manter as quotas cheias de foco/aprendidas (%d/%d), veio %d/%d",
				PecasQuotaMinima, PecasQuotaMinima, foco, aprendidasPendentes)
		}
		if estudoForaFoco != PecasQuotaMinima {
			t.Errorf("esperava exatamente %d peça em estudo fora do foco, veio %d: %+v", PecasQuotaMinima, estudoForaFoco, pecas)
		}
		// Sem vaga livre de fallback, a aprendida de área maximizada (四) nunca entra.
		if temMaximizada {
			t.Errorf("a aprendida com streak maximizado não deveria entrar: %+v", pecas)
		}
	}
}

func TestBuscarBaralhoPronunciaComposicaoEhUnicidade(t *testing.T) {
	alvo := entradaDeTeste("一", "yi1", "one")
	emEstudo := []dicionario.DecomposicaoHanzi{
		alvo,
		entradaDeTeste("二", "er4", "two"),
		entradaDeTeste("三", "san1", "three"),
		entradaDeTeste("四", "si4", "four"),
		entradaDeTeste("五", "wu3", "five"),
	}
	aprendidos := []dicionario.DecomposicaoHanzi{
		entradaDeTeste("六", "liu4", "six"),
		entradaDeTeste("七", "qi1", "seven"),
		entradaDeTeste("八", "ba1", "eight"),
	}
	pools := Pools{Todos: append(append([]dicionario.DecomposicaoHanzi{}, emEstudo...), aprendidos...), EmEstudo: emEstudo, Aprendidos: aprendidos}

	ehEstudo := conjunto("一", "二", "三", "四", "五")

	for iteracao := 0; iteracao < 30; iteracao++ {
		cartas := BuscarBaralhoPronuncia(alvo, pools)
		if len(cartas) != totalCartasBaralho {
			t.Fatalf("esperava %d cartas, veio %d", totalCartasBaralho, len(cartas))
		}

		vistos := make(map[string]bool)
		qtdEstudo, temAlvo := 0, false
		for _, carta := range cartas {
			if vistos[carta.Caractere] {
				t.Fatalf("carta repetida no baralho: %q", carta.Caractere)
			}
			vistos[carta.Caractere] = true
			if carta.Caractere == alvo.Caractere {
				temAlvo = true
			}
			if ehEstudo[carta.Caractere] {
				qtdEstudo++
			}
		}
		if !temAlvo {
			t.Fatal("o alvo deveria estar no baralho")
		}
		// Alvo em estudo: composição-alvo = alvo + 3 em estudo + 2 aprendidas.
		if qtdEstudo != cartasBaralhoEmEstudo {
			t.Errorf("esperava %d cartas em estudo (alvo incluído), veio %d", cartasBaralhoEmEstudo, qtdEstudo)
		}
	}
}

// ----- Pool de já vistas frequentes (recortarVistasFrequentes) -----

// listaOrdenada monta uma lista de candidatos (em ordem de frequência) com caracteres "c0","c1",…
func listaOrdenada(n int) []dicionario.DecomposicaoHanzi {
	lista := make([]dicionario.DecomposicaoHanzi, n)
	for i := range lista {
		lista[i] = entradaDeTeste(fmt.Sprintf("c%d", i), "", "")
	}
	return lista
}

func TestRecortarVistasFrequentesVagasLivresDepoisSoVisto(t *testing.T) {
	ordenadas := listaOrdenada(15)
	// Vistas só a partir do índice 5: c7, c9, c11, c13, c14 (as demais nunca vistas).
	vistas := conjunto("c7", "c9", "c11", "c13", "c14")

	got := recortarVistasFrequentes(ordenadas, func(ch string) bool { return vistas[ch] })

	if len(got) != TotalPoolVistas {
		t.Fatalf("esperava %d itens no pool, veio %d: %v", TotalPoolVistas, len(got), textos(got))
	}
	// As VagasLivresPoolVistas primeiras são as mais frequentes, independentemente de já vistas.
	for i := 0; i < VagasLivresPoolVistas; i++ {
		if got[i].Caractere != fmt.Sprintf("c%d", i) {
			t.Errorf("vaga livre %d esperava c%d, veio %q (pool: %v)", i, i, got[i].Caractere, textos(got))
		}
	}
	// Da VagasLivresPoolVistas em diante, só entram já vistas.
	for i := VagasLivresPoolVistas; i < len(got); i++ {
		if !vistas[got[i].Caractere] {
			t.Errorf("vaga %d deveria ser de já vista, veio %q (pool: %v)", i, got[i].Caractere, textos(got))
		}
	}
	esperadas := []string{"c0", "c1", "c2", "c3", "c4", "c7", "c9", "c11", "c13", "c14"}
	for i, e := range esperadas {
		if got[i].Caractere != e {
			t.Errorf("posição %d esperava %q, veio %q (pool: %v)", i, e, got[i].Caractere, textos(got))
		}
	}
}

func TestRecortarVistasFrequentesSemVistasParaNasVagasLivres(t *testing.T) {
	ordenadas := listaOrdenada(12)
	got := recortarVistasFrequentes(ordenadas, func(string) bool { return false })

	// Sem nenhuma já vista, o pool para nas vagas livres (as mais frequentes).
	if len(got) != VagasLivresPoolVistas {
		t.Fatalf("sem vistas o pool deveria ter só as %d vagas livres, veio %d: %v", VagasLivresPoolVistas, len(got), textos(got))
	}
}

func TestRecortarVistasFrequentesMenosCandidatosQueTeto(t *testing.T) {
	ordenadas := listaOrdenada(3)
	got := recortarVistasFrequentes(ordenadas, func(string) bool { return false })

	if len(got) != 3 {
		t.Fatalf("com 3 candidatos o pool deveria ter 3 (todos em vaga livre), veio %d", len(got))
	}
}

func textos(entradas []dicionario.DecomposicaoHanzi) []string {
	out := make([]string, len(entradas))
	for i, e := range entradas {
		out[i] = e.Caractere
	}
	return out
}

func TestExtrairInicialPinyin(t *testing.T) {
	testes := []struct {
		pinyin   string
		esperado string
	}{
		{"zhang1", "zh"},
		{"chā", "ch"},
		{"shū", "sh"},
		{"bā", "b"},
		{"pín", "p"},
		{"ā", ""},
		{"ōu", ""},
	}

	for _, tt := range testes {
		obtido := extrairInicialPinyin(tt.pinyin)
		if obtido != tt.esperado {
			t.Errorf("extrairInicialPinyin(%q) = %q, esperava %q", tt.pinyin, obtido, tt.esperado)
		}
	}
}

func TestBuscarBaralhoPronunciaTipoFiltraMesmaInicial(t *testing.T) {
	alvo := entradaDeTeste("张", "zhang1", "folha")
	emEstudo := []dicionario.DecomposicaoHanzi{
		alvo,
		entradaDeTeste("这", "zhe4", "este"),
		entradaDeTeste("知", "zhi1", "saber"),
		entradaDeTeste("中", "zhong1", "centro"),
		entradaDeTeste("主", "zhu3", "senhor"),
		entradaDeTeste("照", "zhao4", "iluminar"),
		entradaDeTeste("你", "ni3", "você"),
		entradaDeTeste("好", "hao3", "bom"),
	}
	pools := Pools{Todos: emEstudo, EmEstudo: emEstudo}

	cartas := BuscarBaralhoPronunciaTipo(alvo, pools)
	if len(cartas) == 0 {
		t.Fatalf("esperava cartas no baralho de pronúncia por tipo, veio vazio")
	}

	for _, carta := range cartas {
		inicial := extrairInicialPinyin(carta.Pinyin[0])
		if inicial != "zh" {
			t.Errorf("carta %s (%s) tem inicial %q, esperava %q", carta.Caractere, carta.Pinyin[0], inicial, "zh")
		}
	}
}

// ----- Tema preferido nas frases (Jornada) -----

// buscadorComFrasesDeTema monta um Buscador com um acervo real acrescido das frases de teste (via
// fonte extra). As frases usam uma palavra inexistente no acervo Tatoeba, para que as candidatas da
// busca sejam exatamente as injetadas aqui.
func buscadorComFrasesDeTema(t *testing.T) *Buscador {
	t.Helper()

	acervo := dicionario.NovoGerenciadorFrases(dicionario.IdiomaPadrao)
	acervo.RegistrarFonteExtra(func() ([]dicionario.Frase, error) {
		return []dicionario.Frase{
			{Chines: "我喜欢" + palavraSoDoTeste + "。", Ingles: "Eu gosto disso.", Atribuicao: "teste", Tema: "日常生活", Dificuldade: "入门"},
			{Chines: "他买了" + palavraSoDoTeste + "。", Ingles: "Ele comprou isso.", Atribuicao: "teste", Tema: "购物", Dificuldade: "入门"},
		}, nil
	})

	buscador := Novo(nil, acervo, func() config.Config { return config.Config{} }, nil, nil, nil)
	buscador.IniciarSessao(map[string]string{}, map[string]bool{}, map[string]int{}, nil, 3)
	return buscador
}

// palavraSoDoTeste é uma sequência de hanzis raros que não aparece no acervo real de frases.
const palavraSoDoTeste = "圕龘"

func TestBuscarFraseIdealPrefereOhTemaDaSessao(t *testing.T) {
	buscador := buscadorComFrasesDeTema(t)
	criterios := CriteriosFraseDaVariante(VarianteContexto, palavraSoDoTeste)

	// Sem preferência: qualquer uma das duas serve (só confere que a busca acha frase).
	if _, ok := buscador.BuscarFraseIdeal(criterios); !ok {
		t.Fatal("sem tema preferido, a busca deveria achar uma das frases de teste")
	}

	// Com o tema preferido: a frase escolhida é a daquele tema.
	buscador.IniciarSessao(map[string]string{}, map[string]bool{}, map[string]int{}, nil, 3)
	buscador.DefinirTemaPreferido("购物")
	frase, ok := buscador.BuscarFraseIdeal(criterios)
	if !ok {
		t.Fatal("com tema preferido, a busca deveria achar a frase do tema")
	}
	if frase.Tema != "购物" {
		t.Errorf("esperava a frase do tema 购物, veio tema %q (%s)", frase.Tema, frase.Chines)
	}
}

func TestBuscarFraseIdealTemaSemCorrespondenciaNaoZeraAhBusca(t *testing.T) {
	buscador := buscadorComFrasesDeTema(t)
	buscador.DefinirTemaPreferido("科技") // nenhuma frase de teste tem esse tema

	frase, ok := buscador.BuscarFraseIdeal(CriteriosFraseDaVariante(VarianteContexto, palavraSoDoTeste))
	if !ok {
		t.Fatal("tema sem correspondência deveria degradar para as demais candidatas, não zerar a busca")
	}
	if !strings.Contains(frase.Chines, palavraSoDoTeste) {
		t.Errorf("frase devolvida não contém a palavra buscada: %q", frase.Chines)
	}
}

func TestIniciarSessaoZeraOhTemaPreferido(t *testing.T) {
	buscador := buscadorComFrasesDeTema(t)
	buscador.DefinirTemaPreferido("购物")
	buscador.IniciarSessao(map[string]string{}, map[string]bool{}, map[string]int{}, nil, 3)

	if buscador.temaPreferido != "" {
		t.Errorf("IniciarSessao deveria zerar o tema preferido, veio %q", buscador.temaPreferido)
	}
}

func TestBuscarFraseIdealFiltroObrigatorioSucesso(t *testing.T) {
	buscador := buscadorComFrasesDeTema(t)
	criterios := CriteriosFraseDaVariante(VarianteContexto, palavraSoDoTeste)

	buscador.DefinirFiltroFrases("购物", "")
	frase, ok := buscador.BuscarFraseIdeal(criterios)
	if !ok {
		t.Fatal("com filtro de tema 购物, a busca deveria achar a frase correspondente")
	}
	if frase.Tema != "购物" {
		t.Errorf("esperava a frase do tema 购物, veio tema %q (%s)", frase.Tema, frase.Chines)
	}
}

func TestBuscarFraseIdealFiltroObrigatorioSemCorrespondenciaFalha(t *testing.T) {
	buscador := buscadorComFrasesDeTema(t)
	buscador.DefinirFiltroFrases("科技", "") // nenhuma frase de teste tem esse tema

	_, ok := buscador.BuscarFraseIdeal(CriteriosFraseDaVariante(VarianteContexto, palavraSoDoTeste))
	if ok {
		t.Fatal("o filtro do usuário é obrigatório: tema sem correspondência deveria falhar a busca (ok == false)")
	}
}

func TestIniciarSessaoZeraFiltroFrases(t *testing.T) {
	buscador := buscadorComFrasesDeTema(t)
	buscador.DefinirFiltroFrases("购物", "入门")
	buscador.IniciarSessao(map[string]string{}, map[string]bool{}, map[string]int{}, nil, 3)

	if buscador.temaExigido != "" || buscador.dificuldadeExigida != "" {
		t.Errorf("IniciarSessao deveria zerar temaExigido e dificuldadeExigida, veio tema=%q dif=%q", buscador.temaExigido, buscador.dificuldadeExigida)
	}
}

func TestFiltrarVariantesPorDificuldade(t *testing.T) {
	opcoesBase := []string{VarianteHanziFraseParaSignificado, VarianteSignificadoParaHanziConhecido, VarianteQuebraCabecaSignificado}

	resultadoVazio := filtrarVariantesPorDificuldade("", opcoesBase)
	if len(resultadoVazio) != len(opcoesBase) {
		t.Errorf("alvo vazio deveria devolver todas as opções, veio %v", resultadoVazio)
	}

	opcoesOrdenacao := []string{VarianteOrdenacao, VarianteOrdenacaoTraducao}
	resIniciante := filtrarVariantesPorDificuldade(DificuldadeIniciante, opcoesOrdenacao)
	if len(resIniciante) != 1 || resIniciante[0] != VarianteOrdenacaoTraducao {
		t.Errorf("esperava [ordenacao_traducao] (menor acima), veio %v", resIniciante)
	}

	resAvancado := filtrarVariantesPorDificuldade(DificuldadeAvancado, opcoesBase)
	if len(resAvancado) != 1 || resAvancado[0] != VarianteQuebraCabecaSignificado {
		t.Errorf("esperava [quebracabeca_significado] (maior abaixo), veio %v", resAvancado)
	}

	resIntroducao := filtrarVariantesPorDificuldade(DificuldadeIntroducao, opcoesBase)
	esperadasIntro := []string{VarianteHanziFraseParaSignificado, VarianteSignificadoParaHanziConhecido}
	if len(resIntroducao) != 2 || resIntroducao[0] != esperadasIntro[0] || resIntroducao[1] != esperadasIntro[1] {
		t.Errorf("esperava %v (exatas), veio %v", esperadasIntro, resIntroducao)
	}
}

func TestSortearVarianteSessaoEDificuldadeAlvo(t *testing.T) {
	buscador := &Buscador{}
	buscador.DefinirDificuldadeAlvo(DificuldadeAvancado)
	res := buscador.SortearVarianteSessao(VarianteHanziFraseParaSignificado, VarianteQuebraCabecaSignificado)
	if res != VarianteQuebraCabecaSignificado {
		t.Errorf("com alvo avançado, esperava sempre quebracabeca_significado, veio %q", res)
	}

	buscador.IniciarSessao(map[string]string{}, map[string]bool{}, map[string]int{}, nil, 3)
	if buscador.dificuldadeAlvo != "" {
		t.Errorf("IniciarSessao deveria zerar a dificuldade alvo, veio %q", buscador.dificuldadeAlvo)
	}
}

func TestSortearVarianteSessaoRespeitaOhVeto(t *testing.T) {
	buscador := &Buscador{}

	// O veto tem de vencer a dificuldade: com alvo avançado o quebra-cabeça seria a única exata, e é
	// justamente esse o caso que fazia a Jornada repetir a mesma atividade sessão inteira.
	buscador.DefinirDificuldadeAlvo(DificuldadeAvancado)
	buscador.DefinirVariantesVetadas(VarianteQuebraCabecaSignificado)
	if res := buscador.SortearVarianteSessao(VarianteHanziParaSignificado, VarianteQuebraCabecaSignificado); res != VarianteHanziParaSignificado {
		t.Errorf("com o quebra-cabeça vetado, esperava hanzi_para_significado, veio %q", res)
	}

	// Vetar tudo não pode deixar a questão sem atividade.
	if res := buscador.SortearVarianteSessao(VarianteQuebraCabecaSignificado); res != VarianteQuebraCabecaSignificado {
		t.Errorf("veto que zera as opções deveria ser ignorado, veio %q", res)
	}

	buscador.DefinirVariantesVetadas()
	if res := buscador.SortearVarianteSessao(VarianteQuebraCabecaSignificado); res != VarianteQuebraCabecaSignificado {
		t.Errorf("sem argumentos o veto deveria ser limpo, veio %q", res)
	}

	buscador.DefinirVariantesVetadas(VarianteQuebraCabecaSignificado)
	buscador.IniciarSessao(map[string]string{}, map[string]bool{}, map[string]int{}, nil, 3)
	if buscador.variantesVetadas != nil {
		t.Errorf("IniciarSessao deveria zerar o veto, veio %v", buscador.variantesVetadas)
	}
}

func TestSortearVarianteSessaoRespeitaSubAtividadesDesativadas(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.AtividadesDesativadas = []string{VarianteQuebraCabecaSignificado}

	buscador := Novo(nil, nil, func() config.Config { return cfg }, nil, nil, nil)

	// Com VarianteQuebraCabecaSignificado desativada na config, o sorteio deve retornar apenas a ativa
	for i := 0; i < 20; i++ {
		res := buscador.SortearVarianteSessao(VarianteHanziParaSignificado, VarianteQuebraCabecaSignificado)
		if res != VarianteHanziParaSignificado {
			t.Fatalf("esperava apenas a variante ativa %q, veio %q na iteracao %d", VarianteHanziParaSignificado, res, i)
		}
	}
}

func TestModosPermitidosExcluiModoSemSubAtividadesAtivas(t *testing.T) {
	cfg := config.DefaultConfig()
	// Desativa todas as sub-atividades do modo de ordenação
	cfg.AtividadesDesativadas = VariantesDoModo(ModoPronuncia)

	buscador := Novo(nil, nil, func() config.Config { return cfg }, nil, nil, nil)

	permitidos := buscador.ModosPermitidos()
	for _, m := range permitidos {
		if m == ModoPronuncia {
			t.Fatalf("ModosPermitidos nao deveria incluir o modo %q pois todas as suas sub-atividades estao desativadas", ModoPronuncia)
		}
	}
}

func TestSortearVarianteComRotuloTaxonomiaEFallback(t *testing.T) {
	buscador := &Buscador{}

	// Caso 1: Rótulo chinês "高级" (Avançado, nível 3) escolhe a exata "desenho_contexto" (nível 3) sobre "desenho_memoria" (nível 1)
	buscador.DefinirDificuldadeAlvo("高级")
	if res := buscador.SortearVarianteSessao(VarianteDesenhoMemoria, VarianteDesenhoContexto); res != VarianteDesenhoContexto {
		t.Errorf("com alvo 高级, esperava variante_desenho_contexto (exata), veio %q", res)
	}

	// Caso 2: Fallback para atividade mais fácil — com alvo "高级" (nível 3) e opções nível 0 (audio_para_hanzi) e nível 1 (contexto), escolhe a nível 1
	if res := buscador.SortearVarianteSessao(VarianteAudioParaHanzi, VarianteContexto); res != VarianteContexto {
		t.Errorf("com alvo 高级 e sem variante nível 3, esperava fallback para a mais fácil disponível (contexto - nível 1), veio %q", res)
	}

	// Caso 3: Alvo vazio ("Todas") não restringe nenhuma opção
	buscador.DefinirDificuldadeAlvo("")
	opcoes := []string{VarianteHanziParaSignificado, VarianteDesenhoContexto}
	respostas := make(map[string]bool)
	for i := 0; i < 50; i++ {
		respostas[buscador.SortearVarianteSessao(opcoes...)] = true
	}
	if len(respostas) < 2 {
		t.Errorf("com alvo vazio (Todas), esperava sorteio livre entre todas as variantes, veio apenas %v", respostas)
	}
}

func TestSortearVarianteFallbackEntreAbaixoEAcima(t *testing.T) {
	buscador := &Buscador{}
	// Com alvo nível 2 (intermediário) e opções nível 0 (introdução) e nível 3 (avançado),
	// o fallback deve sorteá-las alternando entre a abaixo (nível 0) e a acima (nível 3).
	buscador.DefinirDificuldadeAlvo(DificuldadeIntermediario)
	opcoes := []string{VarianteHanziParaSignificado, VarianteDesenhoContexto}
	respostas := make(map[string]bool)
	for i := 0; i < 100; i++ {
		respostas[buscador.SortearVarianteSessao(opcoes...)] = true
	}
	if len(respostas) < 2 {
		t.Errorf("com alvo nível 2 sem variante exata, o fallback deveria sortear entre abaixo (nível 0) e acima (nível 3), veio apenas %v", respostas)
	}
}

// ----- Seção: Testes de Requerimentos Mínimos de Frase -----

func TestVerificarRequisitosMinimosFrase(t *testing.T) {
	decomporMock := func(texto string) []PalavraTexto {
		if texto == "你好世界" {
			return []PalavraTexto{
				{Texto: "你好", EhChines: true},
				{Texto: "世界", EhChines: true},
			}
		}
		if texto == "你好世界中国" {
			return []PalavraTexto{
				{Texto: "你好", EhChines: true},
				{Texto: "世界", EhChines: true},
				{Texto: "中国", EhChines: true},
				{Texto: "朋友", EhChines: true},
				{Texto: "学习", EhChines: true},
			}
		}
		return nil
	}

	buscador := Novo(nil, nil, func() config.Config { return config.Config{} }, decomporMock, nil, nil)

	// Cenário 1: Modo normal.
	// Vocabulário: "你好" (estudo), "世界" (visto) -> 50% estudo/aprendido, 100% visto.
	mapaStatus := map[string]string{"你好": dicionario.StatusEstudo}
	mapaVisto := map[string]bool{"世界": true}
	buscador.IniciarSessao(mapaStatus, mapaVisto, nil, nil, 3)

	fraseCurta := dicionario.Frase{Chines: "你好世界"}
	if buscador.verificarRequisitosMinimosFrase(fraseCurta) {
		t.Errorf("esperava falhar no modo normal por ter apenas 50%% estudo/aprendido (<80%%)")
	}

	// Promove "世界" a estudo -> 100% estudo/aprendido, 100% visto.
	mapaStatus["世界"] = dicionario.StatusEstudo
	buscador.IniciarSessao(mapaStatus, mapaVisto, nil, nil, 3)
	if !buscador.verificarRequisitosMinimosFrase(fraseCurta) {
		t.Errorf("esperava passar no modo normal com 100%% estudo/aprendido e 100%% visto")
	}

	// Cenário 2: Modo Jornada (desconsidera regra dos 80% estudo/aprendido).
	// Vocabulário da Jornada com 5 palavras: 1 estudo ("你好"), 4 vistas -> 20% estudo, 100% visto.
	buscador.IniciarSessao(map[string]string{"你好": dicionario.StatusEstudo}, map[string]bool{"世界": true, "中国": true, "朋友": true, "学习": true}, nil, nil, 3)
	buscador.DefinirUniversoRestrito([]string{"你好", "世界", "中国", "朋友", "学习"})

	fraseLonga := dicionario.Frase{Chines: "你好世界中国"}
	if !buscador.verificarRequisitosMinimosFrase(fraseLonga) {
		t.Errorf("na Jornada deveria passar mesmo com apenas 20%% estudo/aprendido, pois 100%% das palavras foram vistas")
	}

	// Frase com palavra não vista (menos de 95% visto) deve falhar mesmo na Jornada.
	buscador.IniciarSessao(map[string]string{}, map[string]bool{"你好": true}, nil, nil, 3)
	buscador.DefinirUniversoRestrito([]string{"你好"})
	if buscador.verificarRequisitosMinimosFrase(fraseLonga) {
		t.Errorf("esperava falhar na Jornada pois menos de 95%% das palavras foram vistas")
	}
}

func TestVarianteImagemParaSignificado(t *testing.T) {
	if ObterDificuldadeVariante(VarianteImagemParaSignificado) != DificuldadeIntroducao {
		t.Errorf("esperava DificuldadeIntroducao para VarianteImagemParaSignificado, veio %q", ObterDificuldadeVariante(VarianteImagemParaSignificado))
	}

	if ObterDificuldadeVariante(VarianteSignificadoParaImagem) != DificuldadeIntroducao {
		t.Errorf("esperava DificuldadeIntroducao para VarianteSignificadoParaImagem, veio %q", ObterDificuldadeVariante(VarianteSignificadoParaImagem))
	}

	if VarianteParaModoBase(VarianteImagemParaSignificado) != ModoSignificado {
		t.Errorf("esperava ModoSignificado para VarianteImagemParaSignificado, veio %q", VarianteParaModoBase(VarianteImagemParaSignificado))
	}

	if VarianteParaModoBase(VarianteSignificadoParaImagem) != ModoSignificado {
		t.Errorf("esperava ModoSignificado para VarianteSignificadoParaImagem, veio %q", VarianteParaModoBase(VarianteSignificadoParaImagem))
	}

	// Testar detecção de imagens para caractere isolado e palavra multi-hanzi
	if !TemImagemParaHanzi("水") {
		t.Errorf("esperava TemImagemParaHanzi('水') == true")
	}

	if !TemImagemParaHanzi("苹果") {
		t.Errorf("esperava TemImagemParaHanzi('苹果') == true (palavra multi-hanzi)")
	}

	if TemImagemParaHanzi("palavra_inexistente_999") {
		t.Errorf("esperava TemImagemParaHanzi('palavra_inexistente_999') == false")
	}

	// Testar o filtro de distratores com imagem
	candidataComImagem := dicionario.DecomposicaoHanzi{Caractere: "水", Definicao: "água"}
	candidataSemImagem := dicionario.DecomposicaoHanzi{Caractere: "inexistente_123", Definicao: "teste"}

	if !DistintosPorSignificadoEComImagem(nil, candidataComImagem) {
		t.Errorf("esperava DistintosPorSignificadoEComImagem == true para palavra com imagem")
	}

	if DistintosPorSignificadoEComImagem(nil, candidataSemImagem) {
		t.Errorf("esperava DistintosPorSignificadoEComImagem == false para palavra sem imagem")
	}

	// Testar VarianteHanziFraseParaSignificado
	if ObterDificuldadeVariante(VarianteHanziFraseParaSignificado) != DificuldadeIntroducao {
		t.Errorf("esperava DificuldadeIntroducao para VarianteHanziFraseParaSignificado, veio %q", ObterDificuldadeVariante(VarianteHanziFraseParaSignificado))
	}

	mapaStatus := map[string]string{"水": dicionario.StatusAprendido}
	mapaVisto := map[string]bool{}
	criterioConhecidos := DistintosPorSignificadoConhecidos(mapaStatus, mapaVisto)

	if !criterioConhecidos(nil, candidataComImagem) {
		t.Errorf("esperava DistintosPorSignificadoConhecidos == true para palavra aprendida")
	}

	if criterioConhecidos(nil, candidataSemImagem) {
		t.Errorf("esperava DistintosPorSignificadoConhecidos == false para palavra nao aprendida/vista")
	}

	// Testar VarianteSignificadoParaHanziConhecido
	if ObterDificuldadeVariante(VarianteSignificadoParaHanziConhecido) != DificuldadeIntroducao {
		t.Errorf("esperava DificuldadeIntroducao para VarianteSignificadoParaHanziConhecido, veio %q", ObterDificuldadeVariante(VarianteSignificadoParaHanziConhecido))
	}

	if VarianteParaModoBase(VarianteSignificadoParaHanziConhecido) != ModoSignificado {
		t.Errorf("esperava ModoSignificado para VarianteSignificadoParaHanziConhecido, veio %q", VarianteParaModoBase(VarianteSignificadoParaHanziConhecido))
	}

	criterioHanziConhecidos := DistintosPorHanziConhecidos(mapaStatus, mapaVisto)
	if !criterioHanziConhecidos(nil, candidataComImagem) {
		t.Errorf("esperava DistintosPorHanziConhecidos == true para palavra aprendida")
	}

	if criterioHanziConhecidos(nil, candidataSemImagem) {
		t.Errorf("esperava DistintosPorHanziConhecidos == false para palavra nao aprendida/vista")
	}
}

func TestVarianteFoneticaFilaPinyin(t *testing.T) {
	if ObterDificuldadeVariante(VarianteFoneticaFilaPinyin) != DificuldadeIntermediario {
		t.Errorf("esperava DificuldadeIntermediario para VarianteFoneticaFilaPinyin, veio %q", ObterDificuldadeVariante(VarianteFoneticaFilaPinyin))
	}

	if VarianteParaModoBase(VarianteFoneticaFilaPinyin) != ModoFonetica {
		t.Errorf("esperava ModoFonetica para VarianteFoneticaFilaPinyin, veio %q", VarianteParaModoBase(VarianteFoneticaFilaPinyin))
	}

	criterios := CriteriosFraseDaVariante(VarianteFoneticaFilaPinyin, "好")
	if criterios.MaxPalavras != MaxPalavrasFraseFonetica {
		t.Errorf("esperava MaxPalavras %d, veio %d", MaxPalavrasFraseFonetica, criterios.MaxPalavras)
	}
}

func TestVarianteFoneticaPalavraPinyin(t *testing.T) {
	if ObterDificuldadeVariante(VarianteFoneticaPalavraPinyin) != DificuldadeIniciante {
		t.Errorf("esperava DificuldadeIniciante para VarianteFoneticaPalavraPinyin, veio %q", ObterDificuldadeVariante(VarianteFoneticaPalavraPinyin))
	}

	if VarianteParaModoBase(VarianteFoneticaPalavraPinyin) != ModoFonetica {
		t.Errorf("esperava ModoFonetica para VarianteFoneticaPalavraPinyin, veio %q", VarianteParaModoBase(VarianteFoneticaPalavraPinyin))
	}
}
