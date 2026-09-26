package revisao

import (
	"testing"

	"wails_app/config"
	"wails_app/dicionario"
	"wails_app/progresso"
)

// nunca conclui: predicado padrão para os testes sem banco.
func nuncaConcluido(string) bool { return false }

func conjunto(chars ...string) map[string]bool {
	s := make(map[string]bool, len(chars))
	for _, c := range chars {
		s[c] = true
	}
	return s
}

// ----- reconciliarFoco -----

func TestReconciliarFocoEstavelQuandoNadaMuda(t *testing.T) {
	atuais := []string{"你", "好", "学"}
	elegiveis := conjunto("你", "好", "学")
	ordem := []string{"你", "好", "学"}

	mantidos, removidos, adicionados := reconciliarFoco(atuais, elegiveis, ordem, 3)
	if len(removidos) != 0 || len(adicionados) != 0 {
		t.Fatalf("grupo completo e elegível não deveria rotacionar: removidos=%v adicionados=%v", removidos, adicionados)
	}
	if len(mantidos) != 3 {
		t.Fatalf("esperava manter os 3, veio %v", mantidos)
	}
	for i, caractere := range atuais {
		if mantidos[i] != caractere {
			t.Errorf("mantidos deveria preservar a ordem de entrada: esperava %q na posição %d, veio %q", caractere, i, mantidos[i])
		}
	}
}

func TestReconciliarFocoRemoveIneligivelEPreencheDaOrdem(t *testing.T) {
	atuais := []string{"你", "好", "学"}
	elegiveis := conjunto("你", "学", "生", "水") // "好" perdeu a elegibilidade (concluído/saiu do estudo)
	ordem := []string{"水", "生"}               // ordem de preenchimento (já ponderada por recência)

	mantidos, removidos, adicionados := reconciliarFoco(atuais, elegiveis, ordem, 3)
	if len(removidos) != 1 || removidos[0] != "好" {
		t.Fatalf("esperava remover só %q, veio %v", "好", removidos)
	}
	if len(adicionados) != 1 || adicionados[0] != "水" {
		t.Fatalf("a vaga deveria ir para o 1º da ordem de preenchimento (%q), veio %v", "水", adicionados)
	}
	if len(mantidos) != 2 || mantidos[0] != "你" || mantidos[1] != "学" {
		t.Fatalf("mantidos esperados [你 学], veio %v", mantidos)
	}
}

func TestReconciliarFocoPreencheGrupoVazioNaOrdem(t *testing.T) {
	elegiveis := conjunto("你", "好", "学", "生", "水")
	ordem := []string{"你", "好", "学", "生", "水"}

	mantidos, removidos, adicionados := reconciliarFoco(nil, elegiveis, ordem, 3)
	if len(mantidos) != 0 || len(removidos) != 0 {
		t.Fatalf("grupo vazio não tem o que manter/remover: mantidos=%v removidos=%v", mantidos, removidos)
	}
	esperados := []string{"你", "好", "学"}
	if len(adicionados) != len(esperados) {
		t.Fatalf("esperava %v, veio %v", esperados, adicionados)
	}
	for i, caractere := range esperados {
		if adicionados[i] != caractere {
			t.Errorf("ordem de preenchimento: esperava %q na posição %d, veio %q", caractere, i, adicionados[i])
		}
	}
}

func TestReconciliarFocoRespeitaEncolhimentoDoTamanho(t *testing.T) {
	atuais := []string{"你", "好", "学", "生"}
	elegiveis := conjunto("你", "好", "学", "生")
	ordem := []string{"你", "好", "学", "生"}

	mantidos, removidos, adicionados := reconciliarFoco(atuais, elegiveis, ordem, 2)
	if len(mantidos) != 2 || mantidos[0] != "你" || mantidos[1] != "好" {
		t.Fatalf("encolhimento deveria manter os primeiros [你 好], veio %v", mantidos)
	}
	if len(removidos) != 2 {
		t.Fatalf("esperava remover os 2 excedentes, veio %v", removidos)
	}
	if len(adicionados) != 0 {
		t.Fatalf("grupo cheio não deveria receber ninguém, veio %v", adicionados)
	}
}

func TestReconciliarFocoComElegiveisInsuficientes(t *testing.T) {
	elegiveis := conjunto("你")
	ordem := []string{"你"}

	mantidos, _, adicionados := reconciliarFoco(nil, elegiveis, ordem, 5)
	if len(mantidos)+len(adicionados) != 1 {
		t.Fatalf("com 1 elegível o grupo deveria ficar com 1, veio mantidos=%v adicionados=%v", mantidos, adicionados)
	}
}

// ----- hanzisComponentes -----

func TestHanzisComponentes(t *testing.T) {
	var abrev string
	for k := range dicionario.MapaAbrevParaCompleto {
		abrev = k
		break
	}

	casos := []struct {
		palavra   string
		esperados []string
	}{
		{"好", []string{"好"}},         // caractere isolado → ele mesmo
		{"学生", []string{"学", "生"}},   // palavra multi-hanzi → componentes na ordem
		{"爸爸", []string{"爸"}},        // repetição some (dedup)
		{"a好b", []string{"好"}},       // não-Han ignorado
		{"好" + abrev, []string{"好"}}, // abreviação de radical ignorada
		{abrev, nil},                 // só radical avulso → vazio
	}
	for _, c := range casos {
		got := hanzisComponentes(c.palavra)
		if len(got) != len(c.esperados) {
			t.Fatalf("hanzisComponentes(%q) = %v, esperava %v", c.palavra, got, c.esperados)
		}
		for i := range c.esperados {
			if got[i] != c.esperados[i] {
				t.Errorf("hanzisComponentes(%q)[%d] = %q, esperava %q", c.palavra, i, got[i], c.esperados[i])
			}
		}
	}
}

// ----- palavrasFocoElegiveis -----

func TestPalavrasFocoElegiveisOrdemRecente(t *testing.T) {
	var abrev string
	for k := range dicionario.MapaAbrevParaCompleto {
		abrev = k
		break
	}

	// GetAllVocab devolve data_add DESC (mais recente primeiro); a fila de foco deve sair na mesma
	// ordem (recente → antigo). Palavras multi-hanzi são mantidas INTEIRAS (dedup por palavra).
	vocabulario := []progresso.Vocab{
		{Hanzi: "水", Status: dicionario.StatusEstudo},    // mais recente
		{Hanzi: abrev, Status: dicionario.StatusEstudo},  // radical avulso: ignorado (sem hanzi real)
		{Hanzi: "学生", Status: dicionario.StatusEstudo},   // palavra inteira
		{Hanzi: "好", Status: dicionario.StatusAprendido}, // aprendido: fora (não é estudo)
		{Hanzi: "你好", Status: dicionario.StatusEstudo},   // palavra inteira
		{Hanzi: "你", Status: dicionario.StatusEstudo},    // mais antigo
	}

	fila := palavrasFocoElegiveis(vocabulario, nuncaConcluido)
	esperados := []string{"水", "学生", "你好", "你"}
	if len(fila) != len(esperados) {
		t.Fatalf("esperava %v, veio %v", esperados, fila)
	}
	for i, palavra := range esperados {
		if fila[i] != palavra {
			t.Errorf("posição %d: esperava %q, veio %q (fila completa: %v)", i, palavra, fila[i], fila)
		}
	}
}

func TestPalavrasFocoElegiveisFiltraConcluidos(t *testing.T) {
	vocabulario := []progresso.Vocab{
		{Hanzi: "水", Status: dicionario.StatusEstudo},
		{Hanzi: "学生", Status: dicionario.StatusEstudo},
	}
	// A palavra "学生" já concluiu o aprendizado: não pode entrar no foco.
	concluido := func(p string) bool { return p == "学生" }

	fila := palavrasFocoElegiveis(vocabulario, concluido)
	esperados := []string{"水"}
	if len(fila) != len(esperados) {
		t.Fatalf("esperava %v (sem a concluída 学生), veio %v", esperados, fila)
	}
	for i, palavra := range esperados {
		if fila[i] != palavra {
			t.Errorf("posição %d: esperava %q, veio %q", i, palavra, fila[i])
		}
	}
}

// ----- sorteioPonderadoFoco -----

func TestSorteioPonderadoFocoEhPermutacaoDentroDoCorte(t *testing.T) {
	candidatos := []string{"a", "b", "c", "d", "e"}
	ordem := sorteioPonderadoFoco(candidatos, nil)
	if len(ordem) != len(candidatos) {
		t.Fatalf("esperava %d elementos (todos cabem no corte), veio %d", len(candidatos), len(ordem))
	}
	visto := conjunto(ordem...)
	for _, c := range candidatos {
		if !visto[c] {
			t.Errorf("elemento %q sumiu do sorteio: %v", c, ordem)
		}
	}
}

func TestSorteioPonderadoFocoFavoreceAntigos(t *testing.T) {
	candidatos := []string{"recente", "b", "c", "d", "antigo"} // índice 0 = mais recente, índice n-1 = mais antigo (adicionado há mais tempo)
	const trials = 4000
	primeiroRecente, primeiroAntigo := 0, 0
	for i := 0; i < trials; i++ {
		ordem := sorteioPonderadoFoco(candidatos, nil)
		switch ordem[0] {
		case "recente":
			primeiroRecente++
		case "antigo":
			primeiroAntigo++
		}
	}
	if primeiroAntigo <= primeiroRecente {
		t.Errorf("o mais antigo (adicionado há mais tempo) deveria ser sorteado em 1º com mais frequência que o mais recente: antigo=%d recente=%d", primeiroAntigo, primeiroRecente)
	}
}

func TestSorteioPonderadoFocoFavoreceMaisVistos(t *testing.T) {
	// O mais recente (menor base de tempo em estudo) foi muito visto pelo OCR: o multiplicador de
	// visualizações deve fazê-lo sair em 1º com mais frequência que o mais antigo nunca visto.
	candidatos := []string{"recenteMuitoVisto", "b", "c", "d", "antigo"}
	vezesVista := map[string]int{"recenteMuitoVisto": 50}

	const trials = 4000
	primeiroMuitoVisto, primeiroAntigo := 0, 0
	for i := 0; i < trials; i++ {
		ordem := sorteioPonderadoFoco(candidatos, vezesVista)
		switch ordem[0] {
		case "recenteMuitoVisto":
			primeiroMuitoVisto++
		case "antigo":
			primeiroAntigo++
		}
	}
	if primeiroMuitoVisto <= primeiroAntigo {
		t.Errorf("o muito visto deveria ser sorteado em 1º com mais frequência que o antigo nunca visto: muitoVisto=%d antigo=%d", primeiroMuitoVisto, primeiroAntigo)
	}
}

func TestSorteioPonderadoFocoRestringeAosDezMaioresPesos(t *testing.T) {
	// 12 candidatos, todos sem visualizações: a ordem vem de data_add DESC (0=mais recente).
	// O peso favorece os adicionados há mais tempo (índice maior). Então os 2 mais recentes
	// (índices 0 e 1, "recente1" e "recente2") possuem os menores pesos e devem ficar fora do corte das 10 vagas.
	candidatos := []string{"recente1", "recente2", "c", "d", "e", "f", "g", "h", "i", "j", "k", "antigo"}

	ordem := sorteioPonderadoFoco(candidatos, nil)
	if len(ordem) != MaximoCandidatosSorteioFoco {
		t.Fatalf("esperava o corte em %d candidatos, veio %d: %v", MaximoCandidatosSorteioFoco, len(ordem), ordem)
	}
	visto := conjunto(ordem...)
	if visto["recente1"] || visto["recente2"] {
		t.Errorf("os 2 de menor peso (mais recentes) deveriam ficar fora do corte: %v", ordem)
	}

	// Com muitas visualizações, o mais recente sobe no ranking e entra no corte no lugar de outro.
	ordem = sorteioPonderadoFoco(candidatos, map[string]int{"recente1": 100})
	visto = conjunto(ordem...)
	if !visto["recente1"] {
		t.Errorf("o mais recente muito visto deveria entrar no corte: %v", ordem)
	}
	if visto["recente2"] {
		t.Errorf("o segundo mais recente (sem visualizações) segue com o menor peso e deveria ficar fora: %v", ordem)
	}
}

// ----- tamanhoFocoEfetivo -----

func TestTamanhoFocoEfetivoAutomatico(t *testing.T) {
	cfg := config.DefaultConfig()
	app := NovoGerenciadorRevisao(nil, nil, func() config.Config { return cfg })
	cfg.TamanhoFocoAutomatico = true

	if got := app.tamanhoFocoEfetivo(3); got != 3 {
		t.Errorf("auto com 3 elegíveis deveria ser 3, veio %d", got)
	}
	if got := app.tamanhoFocoEfetivo(TamanhoFocoMaximo + 5); got != TamanhoFocoMaximo {
		t.Errorf("auto deveria respeitar o teto %d, veio %d", TamanhoFocoMaximo, got)
	}
	if got := app.tamanhoFocoEfetivo(0); got != 0 {
		t.Errorf("auto sem elegíveis deveria ser 0, veio %d", got)
	}
}

func TestTamanhoFocoEfetivoManual(t *testing.T) {
	cfg := config.DefaultConfig()
	app := NovoGerenciadorRevisao(nil, nil, func() config.Config { return cfg })
	cfg.TamanhoFocoAutomatico = false
	cfg.TamanhoFocoRevisao = 5

	if got := app.tamanhoFocoEfetivo(3); got != 5 {
		t.Errorf("manual ignora a contagem de elegíveis: esperava 5, veio %d", got)
	}
	if got := app.tamanhoFocoEfetivo(50); got != 5 {
		t.Errorf("manual ignora a contagem de elegíveis: esperava 5, veio %d", got)
	}
}

// ----- selecionarAlvos com grupo de foco -----

func TestSelecionarAlvosProporcao60Por40NoBlocoInicial(t *testing.T) {
	cfg := config.DefaultConfig()
	app := NovoGerenciadorRevisao(nil, nil, func() config.Config { return cfg })

	candidatos := make([]dicionario.DecomposicaoHanzi, 0, 30)
	for _, caractere := range []rune("一二三四五六七八九十百千万水火木金土日月山口人") {
		candidatos = append(candidatos, dicionario.DecomposicaoHanzi{Caractere: string(caractere)})
	}
	foco := []string{"一", "二", "三", "四", "五"}
	mapaStatus := map[string]string{
		"一": dicionario.StatusEstudo, "二": dicionario.StatusEstudo, "三": dicionario.StatusEstudo,
		"四": dicionario.StatusEstudo, "五": dicionario.StatusEstudo,
		"六": dicionario.StatusAprendido, "七": dicionario.StatusAprendido,
	}

	const quantidade = 10
	alvos := app.selecionarAlvos(candidatos, foco, mapaStatus, quantidade)

	if len(alvos) > quantidade*3 {
		t.Fatalf("reserva estourou o teto de 3x a sessão: %d alvos", len(alvos))
	}
	if len(alvos) < quantidade {
		t.Fatalf("esperava ao menos %d alvos, veio %d", quantidade, len(alvos))
	}

	blocoInicial := alvos[:quantidade]
	qtdFoco, qtdFora := 0, 0
	cobertura := make(map[string]bool)
	for _, alvo := range blocoInicial {
		if alvo.emFoco {
			qtdFoco++
			if !alvo.emEstudo {
				t.Errorf("alvo em foco %q deveria estar marcado como em estudo", alvo.entrada.Caractere)
			}
			cobertura[alvo.entrada.Caractere] = true
		} else {
			qtdFora++
		}
	}

	if qtdFoco != 8 {
		t.Errorf("esperava 8 dos %d alvos iniciais em foco (mínimo 8 / 60%%), veio %d", quantidade, qtdFoco)
	}
	if qtdFora != 2 {
		t.Errorf("esperava 2 alvos de preenchimento fora do foco, veio %d", qtdFora)
	}
	for _, caractere := range foco {
		if !cobertura[caractere] {
			t.Errorf("caractere em foco %q ficou fora do bloco inicial da sessão", caractere)
		}
	}
}

func TestSelecionarAlvosPreenchimentoPriorizaEstudoDepoisAprendido(t *testing.T) {
	cfg := config.DefaultConfig()
	app := NovoGerenciadorRevisao(nil, nil, func() config.Config { return cfg })

	var candidatos []dicionario.DecomposicaoHanzi
	mapaStatus := map[string]string{}
	for _, c := range []rune("一二三") { // em estudo
		candidatos = append(candidatos, dicionario.DecomposicaoHanzi{Caractere: string(c)})
		mapaStatus[string(c)] = dicionario.StatusEstudo
	}
	for _, c := range []rune("四五") { // aprendido
		candidatos = append(candidatos, dicionario.DecomposicaoHanzi{Caractere: string(c)})
		mapaStatus[string(c)] = dicionario.StatusAprendido
	}
	for _, c := range []rune("六七八九十") { // nem estudo nem aprendido
		candidatos = append(candidatos, dicionario.DecomposicaoHanzi{Caractere: string(c)})
	}

	const quantidade = 4 // = caracteres em estudo (3) + 1; menor que estudo+aprendido (5)
	alvos := app.selecionarAlvos(candidatos, nil, mapaStatus, quantidade)

	if len(alvos) < quantidade {
		t.Fatalf("esperava ao menos %d alvos, veio %d", quantidade, len(alvos))
	}

	blocoInicial := alvos[:quantidade]
	qtdEstudo := 0
	for _, alvo := range blocoInicial {
		status := mapaStatus[alvo.entrada.Caractere]
		if status != dicionario.StatusEstudo && status != dicionario.StatusAprendido {
			t.Errorf("bloco inicial trouxe %q sem status estudo/aprendido, havendo candidatos dessas filas disponíveis", alvo.entrada.Caractere)
		}
		if status == dicionario.StatusEstudo {
			qtdEstudo++
		}
	}
	if qtdEstudo != 3 {
		t.Errorf("esperava os 3 caracteres em estudo preenchendo antes do aprendido, veio %d", qtdEstudo)
	}
}

func TestSelecionarAlvosFocoIgnoraIneligiveis(t *testing.T) {
	cfg := config.DefaultConfig()
	app := NovoGerenciadorRevisao(nil, nil, func() config.Config { return cfg })

	candidatos := []dicionario.DecomposicaoHanzi{{Caractere: "一"}, {Caractere: "二"}, {Caractere: "三"}}
	foco := []string{"一", "龍"} // "龍" não está entre os candidatos do modo

	alvos := app.selecionarAlvos(candidatos, foco, map[string]string{"一": dicionario.StatusEstudo}, 3)
	for _, alvo := range alvos {
		if alvo.entrada.Caractere == "龍" {
			t.Fatalf("caractere fora dos candidatos não deveria virar alvo")
		}
	}
	temFoco := false
	for _, alvo := range alvos {
		if alvo.emFoco && alvo.entrada.Caractere == "一" {
			temFoco = true
		}
	}
	if !temFoco {
		t.Fatalf("o caractere elegível do foco deveria estar entre os alvos: %+v", alvos)
	}
}

func TestAdicionarERemoverHanziFocoAjustaTamanhoConfigurado(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.TamanhoFocoAutomatico = false
	cfg.TamanhoFocoRevisao = 5
	app := NovoGerenciadorRevisao(nil, nil, func() config.Config { return cfg })

	if !app.Config().TamanhoFocoAutomatico {
		novoTamanho := app.tamanhoFocoConfigurado() - 1
		if novoTamanho < 1 {
			novoTamanho = 1
		}
		cfg.TamanhoFocoRevisao = novoTamanho
	}
	if app.Config().TamanhoFocoRevisao != 4 {
		t.Errorf("esperava tamanho de foco 4 após remoção, veio %d", app.Config().TamanhoFocoRevisao)
	}

	if !app.Config().TamanhoFocoAutomatico {
		novoTamanho := app.tamanhoFocoConfigurado() + 1
		if novoTamanho > TamanhoFocoMaximo {
			novoTamanho = TamanhoFocoMaximo
		}
		cfg.TamanhoFocoRevisao = novoTamanho
	}
	if app.Config().TamanhoFocoRevisao != 5 {
		t.Errorf("esperava tamanho de foco 5 após adição, veio %d", app.Config().TamanhoFocoRevisao)
	}
}

func TestSugestoesAprendidoComModos(t *testing.T) {
	cfg := config.DefaultConfig()
	app := NovoGerenciadorRevisao(nil, nil, func() config.Config { return cfg })

	// Testando r.aprendizadoConcluidoComModos
	// Quando modosPermitidos inclui apenas significado e fonética
	modosRestritos := []string{ModoSignificado, ModoFonetica}

	// Sem acertos nas categorias
	if app.aprendizadoConcluidoComModos("字", modosRestritos) {
		t.Errorf("palavra sem acertos não deveria ser considerada concluída")
	}
}
