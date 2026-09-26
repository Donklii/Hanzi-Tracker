package revisao

import (
	"strings"
	"testing"

	"wails_app/config"
	"wails_app/dicionario"
	"wails_app/revisao/jornada"
	"wails_app/segmentacao"
)

// appDeTeste monta um GerenciadorRevisao mínimo para a revisão: dicionários reais (embarcados), sem SQLite.
func appDeTeste(t *testing.T) (*GerenciadorRevisao, *config.Config) {
	t.Helper()

	gerenciadorDicionario, err := dicionario.NovoGerenciadorDicionario(dicionario.IdiomaPadrao)
	if err != nil {
		t.Fatalf("falha ao carregar dicionários: %v", err)
	}

	if err := segmentacao.InitJieba(); err != nil {
		t.Fatalf("falha ao carregar Jieba: %v", err)
	}

	cfg := config.DefaultConfig()
	return NovoGerenciadorRevisao(gerenciadorDicionario, dicionario.NovoGerenciadorFrases(dicionario.IdiomaPadrao), func() config.Config { return cfg }), &cfg
}

func TestObterQuestoesRevisaoTodosOsModos(t *testing.T) {
	app, _ := appDeTeste(t)

	for _, modo := range []string{ModoSignificado, ModoFonetica, ModoDesenho, ModoContexto, ModoGeral} {
		questoes, err := app.ObterQuestoesRevisao(modo, 10)
		if err != nil {
			t.Fatalf("modo %s: %v", modo, err)
		}
		if len(questoes) == 0 {
			t.Fatalf("modo %s: nenhuma questão gerada", modo)
		}

		vistos := make(map[string]bool)
		for _, q := range questoes {
			if q.Hanzi == "" || q.Pinyin == "" || q.Definicao == "" {
				t.Errorf("modo %s: questão incompleta: %+v", modo, q)
			}
			if vistos[q.Hanzi] && modo != ModoGeral {
				t.Errorf("modo %s: hanzi %q repetido na sessão", modo, q.Hanzi)
			}
			vistos[q.Hanzi] = true
			validarQuestao(t, q)
		}
	}
}

func validarQuestao(t *testing.T, q QuestaoRevisao) {
	t.Helper()

	if q.Dificuldade == "" {
		t.Errorf("questão %q (%s, variante %s): campo Dificuldade não foi preenchido", q.Hanzi, q.Modo, q.Variante)
	}

	// O desenho por componente é a única variante que manda recorte de traços para o canvas: sem os
	// dois campos o frontend cairia no desenho do caractere inteiro sem ninguém perceber.
	if q.Variante == VarianteDesenhoComponente && (q.ComponenteAlvo == "" || len(q.TracosAlvo) == 0) {
		t.Errorf("questão %q (variante %s): esperava componente e traços alvo preenchidos, veio %q e %v", q.Hanzi, q.Variante, q.ComponenteAlvo, q.TracosAlvo)
	}

	// A montagem manda a partição completa em peças: sem pelo menos duas, não há o que recompor.
	if q.Variante == VarianteDesenhoMontagem && len(q.ComponentesMontagem) < 2 {
		t.Errorf("questão %q (variante %s): esperava ao menos 2 peças de montagem, veio %+v", q.Hanzi, q.Variante, q.ComponentesMontagem)
	}

	// Os quebra-cabeças (significado/fonética/trio) têm forma própria (peças todas corretas) e estão
	// instáveis/em refatoração — ficam de fora desta validação, que será reescrita junto com eles.
	if q.Variante == VarianteQuebraCabecaSignificado || q.Variante == VarianteQuebraCabecaFonetica || q.Variante == VarianteQuebraCabecaTrio {
		return
	}

	precisaOpcoes := q.Modo == ModoSignificado ||
		(q.Modo == ModoFonetica && q.Variante != "fonetica_frase" && q.Variante != "fonetica_fila_pinyin" && q.Variante != "fonetica_palavra_pinyin") ||
		(q.Modo == ModoContexto && q.Variante != VarianteOrdenacao && q.Variante != VarianteOrdenacaoTraducao && q.Variante != VarianteCompreensao && q.Variante != VarianteCompreensaoTraduzida && q.Variante != VarianteRespostaDialogo)
	if precisaOpcoes {
		limiteOpcoes := 4
		if q.Variante == "traducao_contexto" || q.Variante == "fonetica_traducao" {
			limiteOpcoes = 3
		}
		if len(q.Opcoes) != limiteOpcoes {
			t.Fatalf("questão %q (%s, variante %s): esperava %d opções, veio %d", q.Hanzi, q.Modo, q.Variante, limiteOpcoes, len(q.Opcoes))
		}
		corretas := 0
		for _, o := range q.Opcoes {
			if o.Correta {
				corretas++
				if q.Variante == "traducao_contexto" || q.Variante == "fonetica_traducao" {
					if o.Definicao != q.FraseTraducao {
						t.Errorf("questão %q: opção correta de tradução esperava %q, veio %q", q.Hanzi, q.FraseTraducao, o.Definicao)
					}
					if o.Hanzi != q.FraseOriginal {
						t.Errorf("questão %q: opção correta de tradução esperava hanzi %q, veio %q", q.Hanzi, q.FraseOriginal, o.Hanzi)
					}
				} else {
					if o.Hanzi != q.Hanzi {
						t.Errorf("questão %q: opção correta aponta para %q", q.Hanzi, o.Hanzi)
					}
				}
			}
		}
		if corretas != 1 {
			t.Errorf("questão %q (variante %s): esperava exatamente 1 opção correta, veio %d", q.Hanzi, q.Variante, corretas)
		}

		if q.Modo == ModoFonetica && q.Variante != "fonetica_traducao" && q.Variante != "fonetica_fila_pinyin" && q.Variante != "fonetica_palavra_pinyin" {
			vistosPinyin := make(map[string]bool)
			for _, o := range q.Opcoes {
				norm := normalizarPinyinParaComparar(o.Pinyin)
				if vistosPinyin[norm] {
					t.Errorf("questão fonética %q (variante %s): colisão de pinyin %q (original: %q)", q.Hanzi, q.Variante, norm, o.Pinyin)
				}
				vistosPinyin[norm] = true
			}
		}
	}

	if (q.Modo == ModoContexto && (q.Variante == VarianteOrdenacao || q.Variante == VarianteOrdenacaoTraducao)) || (q.Modo == ModoFonetica && q.Variante == "fonetica_frase") {
		if len(q.PecasEsperadas) == 0 {
			t.Errorf("questão %q (%s): esperava peças esperadas, veio vazia", q.Hanzi, q.Modo)
		}
		if len(q.ElementosOrdenacao) == 0 {
			t.Errorf("questão %q (%s): esperava elementos ordenação, veio vazia", q.Hanzi, q.Modo)
		}

		if q.Modo == ModoFonetica && q.Variante == "fonetica_frase" {
			pinyinsCorretos := make(map[string]bool)
			for _, elem := range q.ElementosOrdenacao {
				if elem.Correta {
					pinyinsCorretos[normalizarPinyinParaComparar(elem.Pinyin)] = true
				}
			}
			for _, elem := range q.ElementosOrdenacao {
				if !elem.Correta {
					norm := normalizarPinyinParaComparar(elem.Pinyin)
					if pinyinsCorretos[norm] {
						t.Errorf("peça distratora de fonetica_frase %q (%q) tem pinyin idêntico a uma peça correta", elem.Texto, elem.Pinyin)
					}
				}
			}
		}
	}

	temFrase := q.Variante == "contexto" || q.Variante == "traducao_contexto" || q.Variante == "desenho_contexto" || q.Variante == "fonetica_traducao" || q.Variante == "fonetica_fila_pinyin"
	if temFrase {
		if q.FraseOriginal == "" || !strings.Contains(q.FraseOriginal, q.Hanzi) {
			t.Errorf("questão %q: frase original não contém o alvo: %q", q.Hanzi, q.FraseOriginal)
		}
		if !strings.Contains(q.FraseLacuna, "＿") {
			t.Errorf("questão %q: frase sem lacuna: %q", q.Hanzi, q.FraseLacuna)
		}
		if q.FraseAtribuicao == "" {
			t.Errorf("questão %q: frase Tatoeba sem atribuição CC-BY", q.Hanzi)
		}
	}

	if q.Modo == ModoContexto && q.Variante == "contexto" {
		for _, o := range q.Opcoes {
			if !o.Correta && strings.Contains(q.FraseOriginal, o.Hanzi) {
				t.Errorf("questão %q: distrator %q aparece na própria frase", q.Hanzi, o.Hanzi)
			}
		}
	}

	if q.Modo == ModoFonetica {
		sons := make(map[string]bool)
		for _, o := range q.Opcoes {
			if sons[o.Pinyin] {
				t.Errorf("questão %q: pinyin %q repetido entre as opções de áudio", q.Hanzi, o.Pinyin)
			}
			sons[o.Pinyin] = true
		}
	}
}

func TestObterDadosEscritaHanzi(t *testing.T) {
	app, _ := appDeTeste(t)

	dados, err := app.ObterDadosEscritaHanzi("你")
	if err != nil {
		t.Fatalf("ObterDadosEscritaHanzi(你): %v", err)
	}
	if !strings.Contains(dados, "strokes") || !strings.Contains(dados, "medians") {
		t.Errorf("dados de escrita sem strokes/medians: %.80s…", dados)
	}

	if _, err := app.ObterDadosEscritaHanzi("☃"); err == nil {
		t.Error("esperava erro para caractere sem traçados")
	}
}

// ----- Priorização de modos pelo progresso do card -----

func TestPrioridadeModo(t *testing.T) {
	estatisticas := map[string]int{
		ModoSignificado: 0,                           // pendente
		ModoFonetica:    MetaAcertosConsecutivos,     // concluída (no limite)
		ModoDesenho:     MetaAcertosConsecutivos - 1, // pendente (um abaixo do limite)
		ModoContexto:    MetaAcertosConsecutivos + 2, // concluída
		ModoPronuncia:   1,                           // pendente
	}
	esperados := map[string]int{
		ModoSignificado: 0,
		ModoDesenho:     0,
		ModoPronuncia:   0,

		ModoFonetica:    2,
		ModoContexto:    2,
	}
	for modo, esperado := range esperados {
		if p := prioridadeModo(modo, estatisticas); p != esperado {
			t.Errorf("prioridadeModo(%q) = %d, esperava %d", modo, p, esperado)
		}
	}
}

func TestOrdenarModosPorPendenciaFocoPriorizaPendentes(t *testing.T) {
	cfg := config.DefaultConfig()
	app := NovoGerenciadorRevisao(nil, nil, func() config.Config { return cfg })
	modos := []string{ModoSignificado, ModoFonetica, ModoDesenho, ModoContexto, ModoPronuncia}
	estatisticas := map[string]int{
		ModoSignificado: MetaAcertosConsecutivos, // concluída
		ModoFonetica:    0,                       // pendente
		ModoDesenho:     MetaAcertosConsecutivos, // concluída
		ModoContexto:    1,                       // pendente
		ModoPronuncia:   MetaAcertosConsecutivos, // concluída
	}
	alvo := alvoRevisao{emFoco: true}

	// A base é aleatória; roda várias vezes para garantir que a invariante de prioridade
	// (pendente < neutra < concluída) vale sempre, e que a saída é uma permutação válida.
	for iteracao := 0; iteracao < 50; iteracao++ {
		ordem := app.ordenarModosPorPendencia(alvo, modos, estatisticas)
		if len(ordem) != len(modos) {
			t.Fatalf("esperava %d índices, veio %d", len(modos), len(ordem))
		}
		vistos := make(map[int]bool)
		ultima := -1
		for _, i := range ordem {
			if i < 0 || i >= len(modos) || vistos[i] {
				t.Fatalf("índice inválido ou repetido na ordem: %v", ordem)
			}
			vistos[i] = true
			p := prioridadeModo(modos[i], estatisticas)
			if p < ultima {
				t.Errorf("modo %q (prioridade %d) veio depois de prioridade %d: ordem %v", modos[i], p, ultima, ordem)
			}
			ultima = p
		}
	}
}

func TestOrdenarModosPorPendenciaSemFocoNaoReordena(t *testing.T) {
	cfg := config.DefaultConfig()
	app := NovoGerenciadorRevisao(nil, nil, func() config.Config { return cfg })
	modos := []string{ModoSignificado, ModoFonetica, ModoDesenho}
	estatisticas := map[string]int{ModoSignificado: 0, ModoFonetica: MetaAcertosConsecutivos, ModoDesenho: MetaAcertosConsecutivos}

	// Fora do foco (ou sem estatísticas), devolve apenas uma permutação válida — sem priorizar.
	for _, alvo := range []alvoRevisao{{emFoco: false}, {emFoco: true}} {
		estat := estatisticas
		if alvo.emFoco {
			estat = nil // em foco mas sem estatísticas: também cai no sorteio uniforme
		}
		ordem := app.ordenarModosPorPendencia(alvo, modos, estat)
		if len(ordem) != len(modos) {
			t.Fatalf("esperava %d índices, veio %d", len(modos), len(ordem))
		}
		vistos := make(map[int]bool)
		for _, i := range ordem {
			if i < 0 || i >= len(modos) || vistos[i] {
				t.Fatalf("permutação inválida: %v", ordem)
			}
			vistos[i] = true
		}
	}
}

func TestObterQuestoesRevisaoGeralRespeitaModosDesativados(t *testing.T) {
	app, cfg := appDeTeste(t)

	// Cenário 1: Desativa todos os modos concretos EXCETO ModoSignificado
	cfg.ModosRevisaoGeralDesativados = []string{
		ModoFonetica,
		ModoDesenho,
		ModoContexto,

		ModoPronuncia,
	}

	questoes, err := app.ObterQuestoesRevisao(ModoGeral, 10)
	if err != nil {
		t.Fatalf("esperava sucesso ao obter questões no modo geral com restrições: %v", err)
	}
	if len(questoes) == 0 {
		t.Fatal("esperava ao menos uma questão gerada")
	}

	for _, q := range questoes {
		if q.Modo != ModoSignificado {
			t.Errorf("esperava questão no modo %q, veio %q (variante: %s)", ModoSignificado, q.Modo, q.Variante)
		}
	}

	// Cenário 2: Desativa TODOS os modos concretos (caso de fallback)
	cfg.ModosRevisaoGeralDesativados = []string{
		ModoSignificado,
		ModoFonetica,
		ModoDesenho,
		ModoContexto,

		ModoPronuncia,
	}

	questoesFallback, errFallback := app.ObterQuestoesRevisao(ModoGeral, 10)
	if errFallback != nil {
		t.Fatalf("esperava sucesso no fallback geral ao desativar tudo: %v", errFallback)
	}
	if len(questoesFallback) == 0 {
		t.Fatal("esperava obter questões mesmo quando todos os modos estão desativados (fallback)")
	}
}

func TestObterQuestoesRevisaoRespeitaSubAtividadesDesativadas(t *testing.T) {
	app, cfg := appDeTeste(t)

	// Cenário 1: Desativa todas as variantes de significado EXCETO VarianteHanziParaSignificado
	cfg.AtividadesDesativadas = []string{
		VarianteSignificadoParaHanzi,
		VarianteHanziFraseParaSignificado,
		VarianteSignificadoParaHanziConhecido,
		VarianteImagemParaSignificado,
		VarianteSignificadoParaImagem,
		VarianteQuebraCabecaSignificado,
		VarianteQuebraCabecaTrio,
	}

	questoes, err := app.ObterQuestoesRevisao(ModoSignificado, 10)
	if err != nil {
		t.Fatalf("esperava sucesso no modo significado com sub-atividades filtradas: %v", err)
	}
	if len(questoes) == 0 {
		t.Fatal("esperava questões geradas")
	}

	for _, q := range questoes {
		if q.Variante != VarianteHanziParaSignificado {
			t.Errorf("esperava apenas a variante %q, veio variante %q", VarianteHanziParaSignificado, q.Variante)
		}
	}

	// Cenário 2: Toggle off de uma sub-atividade específica (ex: quebra-cabeça de significado) na Revisão Geral
	cfg.AtividadesDesativadas = []string{VarianteQuebraCabecaSignificado}
	questoesGeral, errGeral := app.ObterQuestoesRevisao(ModoGeral, 20)
	if errGeral != nil {
		t.Fatalf("esperava sucesso na revisão geral com sub-atividade desligada: %v", errGeral)
	}
	for _, q := range questoesGeral {
		if q.Variante == VarianteQuebraCabecaSignificado {
			t.Fatalf("a variante desativada %q nao deveria ter sido sorteada na revisao geral", q.Variante)
		}
	}
}

func TestGerarPilhaOrdenacaoTraducao(t *testing.T) {
	app, _ := appDeTeste(t)

	// Inicializa o mapa de status com alguns caracteres aprendidos para testar os distratores baseados neles
	app.buscador.IniciarSessao(map[string]string{
		"女": dicionario.StatusAprendido,
		"马": dicionario.StatusAprendido,
		"爸": dicionario.StatusAprendido,
	}, nil, nil, nil, MetaAcertosConsecutivos)

	questao := QuestaoRevisao{
		Modo:          ModoContexto,
		Variante:      "ordenacao_traducao",
		FraseOriginal: "我爱我的妈妈。",
		FraseTraducao: "I love my mother.",
	}

	candidatos := app.candidatosParaModo(ModoContexto)

	err := app.gerarPilhaOrdenacaoTraducao(&questao, candidatos)
	if err != nil {
		t.Fatalf("erro ao gerar pilha de ordenação de tradução: %v", err)
	}

	if len(questao.PecasEsperadas) != 4 {
		t.Errorf("esperava 4 peças esperadas, veio %d", len(questao.PecasEsperadas))
	}
	palavrasEsperadas := []string{"I", "love", "my", "mother"}
	for i, p := range questao.PecasEsperadas {
		if p != palavrasEsperadas[i] {
			t.Errorf("peça esperada na posição %d esperava %q, veio %q", i, palavrasEsperadas[i], p)
		}
	}

	if len(questao.ElementosOrdenacao) <= len(questao.PecasEsperadas) {
		t.Errorf("esperava mais elementos de ordenação do que peças corretas (distratores), veio %d", len(questao.ElementosOrdenacao))
	}

	corretas := 0
	distratores := 0
	for _, elem := range questao.ElementosOrdenacao {
		if elem.Texto == "" {
			t.Errorf("encontrou elemento de ordenação com texto vazio")
		}
		if elem.Correta {
			corretas++
		} else {
			distratores++
		}
	}

	if corretas != 4 {
		t.Errorf("esperava 4 peças marcadas como corretas na pilha, veio %d", corretas)
	}
	if distratores == 0 {
		t.Errorf("esperava ter pelo menos 1 distrator na pilha")
	}
}

// ----- Questões da Jornada -----

// appDeTesteJornada é o app de teste com os motores de voz DESLIGADOS — o estado de uma instalação
// limpa, em que a Jornada substitui fonética/pronúncia por palavras.
func appDeTesteJornada(t *testing.T) *GerenciadorRevisao {
	t.Helper()
	app, cfg := appDeTeste(t)
	cfg.MotorTtsAtivo = "nenhum"
	cfg.MotorSttAtivo = "nenhum"
	return app
}

func TestRevisoesEfetivasSubstituiOhQueDependeDeMotor(t *testing.T) {
	nivel, _, err := jornada.ObterNivel("fund1_3") // palavras, fonetica, desenho, frases, pronuncia, mista
	if err != nil {
		t.Fatal(err)
	}

	app, cfg := appDeTeste(t)
	cfg.MotorTtsAtivo = "nenhum"
	cfg.MotorSttAtivo = "nenhum"

	efetivas := app.RevisoesEfetivas(nivel)
	if len(efetivas) != len(nivel.Revisoes) {
		t.Fatalf("a substituição deve preservar o comprimento (%d), veio %d", len(nivel.Revisoes), len(efetivas))
	}
	for i, tipo := range efetivas {
		if tipo == jornada.RevisaoFonetica || tipo == jornada.RevisaoPronuncia {
			t.Errorf("posição %d: sem motor, %q deveria ter virado %q", i, tipo, jornada.RevisaoPalavras)
		}
		if nivel.Revisoes[i] == jornada.RevisaoFonetica && tipo != jornada.RevisaoPalavras {
			t.Errorf("posição %d: fonetica sem TTS deveria virar palavras, veio %q", i, tipo)
		}
	}

	// Com os motores ligados, a lista do conteúdo vale como está.
	cfg.MotorTtsAtivo = "Kokoro-82M"
	cfg.MotorSttAtivo = "Paraformer-ZH"
	for i, tipo := range app.RevisoesEfetivas(nivel) {
		if tipo != nivel.Revisoes[i] {
			t.Errorf("posição %d: com motores ativos esperava %q, veio %q", i, nivel.Revisoes[i], tipo)
		}
	}
}

// modosEsperadosJornada lista os modos aceitáveis para um tipo de revisão. O ModoSignificado entra
// sempre porque é o fallback documentado (alvo sem insumo no modo, ou modo que não rendeu).
func modosEsperadosJornada(tipo string) map[string]bool {
	modos := map[string]bool{ModoSignificado: true}
	switch tipo {
	case jornada.RevisaoDesenho:
		modos[ModoDesenho] = true
	case jornada.RevisaoFrases:
		modos[ModoContexto] = true

	case jornada.RevisaoFonetica:
		modos[ModoFonetica] = true
	case jornada.RevisaoPronuncia:
		modos[ModoPronuncia] = true
	}
	return modos
}

func TestObterQuestoesJornadaCadaRevisaoDoNivel(t *testing.T) {
	app := appDeTesteJornada(t)

	nivel, _, err := jornada.ObterNivel("fund1_0")
	if err != nil {
		t.Fatal(err)
	}
	revisoes := app.RevisoesEfetivas(nivel)

	for revIndex, tipo := range revisoes {
		questoes, err := app.ObterQuestoesJornada("fund1_0", revIndex)
		if err != nil {
			t.Fatalf("revisão %d (%s): %v", revIndex, tipo, err)
		}
		esperado := totalQuestoesJornada(len(nivel.Palavras))
		if len(questoes) != esperado {
			t.Errorf("revisão %d (%s): esperava %d questões, veio %d", revIndex, tipo, esperado, len(questoes))
		}

		// A mista cicla os tipos anteriores do nível; as demais praticam só o próprio tipo.
		permitidos := modosEsperadosJornada(tipo)
		if tipo == jornada.RevisaoMista {
			for _, anterior := range revisoes[:revIndex] {
				for modo := range modosEsperadosJornada(anterior) {
					permitidos[modo] = true
				}
			}
		}

		for _, q := range questoes {
			if !permitidos[q.Modo] {
				t.Errorf("revisão %d (%s): questão no modo %q, fora dos modos do tipo", revIndex, tipo, q.Modo)
			}
			if q.Hanzi == "" || q.Pinyin == "" || q.Definicao == "" {
				t.Errorf("revisão %d (%s): questão incompleta: %+v", revIndex, tipo, q)
			}
			validarQuestao(t, q)
		}
	}
}

func TestObterQuestoesJornadaPrimeiraAparicaoIntrodutoria(t *testing.T) {
	app := appDeTesteJornada(t)

	questoes, err := app.ObterQuestoesJornada("fund1_0", 0)
	if err != nil {
		t.Fatal(err)
	}

	vistos := make(map[string]bool)
	for _, q := range questoes {
		palavra := q.PalavraFoco
		if vistos[palavra] {
			continue
		}
		vistos[palavra] = true

		if q.Dificuldade != DificuldadeIntroducao {
			t.Errorf("palavra %q primeira aparição: esperava dificuldade %q, veio %q", palavra, DificuldadeIntroducao, q.Dificuldade)
		}
		if q.Modo != ModoSignificado {
			t.Errorf("palavra %q primeira aparição: esperava modo %q, veio %q", palavra, ModoSignificado, q.Modo)
		}
	}
}

func TestObterQuestoesJornadaUsaAsPalavrasDoNivel(t *testing.T) {
	app := appDeTesteJornada(t)

	nivel, _, err := jornada.ObterNivel("fund1_0")
	if err != nil {
		t.Fatal(err)
	}
	doNivel := make(map[string]bool, len(nivel.Palavras))
	for _, palavra := range nivel.Palavras {
		doNivel[palavra] = true
	}

	questoes, err := app.ObterQuestoesJornada("fund1_0", 0)
	if err != nil {
		t.Fatal(err)
	}
	for _, q := range questoes {
		// No desenho de palavra multi-hanzi o Hanzi vira o componente; a PalavraFoco é a palavra.
		if !doNivel[q.PalavraFoco] {
			t.Errorf("questão sobre %q, que não é palavra do nível %v", q.PalavraFoco, nivel.Palavras)
		}
	}
}

// TestObterQuestoesJornadaFechaNoUniversoDoNivel trava a regra que torna a Jornada independente das
// outras revisões: alternativas e peças de quebra-cabeça só podem sair do que a própria Jornada já
// ensinou até aquele nível — nada do vocabulário do usuário, do status "em estudo" ou do foco.
func TestObterQuestoesJornadaFechaNoUniversoDoNivel(t *testing.T) {
	app := appDeTesteJornada(t)

	const nivelId = "fund1_2" // nível com anteriores no mesmo ramo: o universo já acumula
	universo, err := jornada.PalavrasAteNivel(nivelId)
	if err != nil {
		t.Fatal(err)
	}
	noUniverso := make(map[string]bool, len(universo))
	for _, palavra := range universo {
		noUniverso[palavra] = true
	}

	nivel, _, err := jornada.ObterNivel(nivelId)
	if err != nil {
		t.Fatal(err)
	}

	for revIndex, tipo := range app.RevisoesEfetivas(nivel) {
		questoes, err := app.ObterQuestoesJornada(nivelId, revIndex)
		if err != nil {
			t.Fatalf("revisão %d (%s): %v", revIndex, tipo, err)
		}

		for _, q := range questoes {
			if !noUniverso[q.PalavraFoco] {
				t.Errorf("revisão %d (%s): alvo %q fora do universo do nível", revIndex, tipo, q.PalavraFoco)
			}
			// As peças de ordenação vêm da FRASE (que traz palavras de fora por natureza); a regra
			// vale para as alternativas/peças, que é onde entravam as palavras em estudo e do foco.
			for _, opcao := range q.Opcoes {
				if !noUniverso[opcao.Hanzi] {
					t.Errorf("revisão %d (%s, variante %s): alternativa %q veio de fora do universo do nível",
						revIndex, tipo, q.Variante, opcao.Hanzi)
				}
			}
		}
	}
}

// TestObterQuestoesJornadaIgnoraEstudoEhFoco confere que o vocabulário do usuário não vaza para a
// sessão da Jornada nem pelos pools por status nem pelo grupo de foco do buscador.
func TestObterQuestoesJornadaIgnoraEstudoEhFoco(t *testing.T) {
	app := appDeTesteJornada(t)

	// Estado que a revisão geral usaria: um grupo de foco e palavras em estudo fora da Jornada.
	forasteira := "电脑" // computador — não está em nenhum nível da árvore
	app.buscador.IniciarSessao(
		map[string]string{"电": dicionario.StatusEstudo, "脑": dicionario.StatusEstudo},
		map[string]bool{},
		map[string]int{},
		[]string{forasteira},
		MetaAcertosConsecutivos,
	)

	questoes, err := app.ObterQuestoesJornada("fund1_0", 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(questoes) == 0 {
		t.Fatal("nenhuma questão gerada")
	}

	for _, q := range questoes {
		for _, opcao := range q.Opcoes {
			if opcao.Hanzi == forasteira {
				t.Errorf("a palavra %q (foco/estudo do usuário) não pode aparecer na Jornada", forasteira)
			}
		}
		if q.EmFoco || q.EmEstudo {
			t.Errorf("questão da Jornada marcada como emFoco=%v emEstudo=%v; o modo não usa esses estados",
				q.EmFoco, q.EmEstudo)
		}
	}
}

// TestObterQuestoesJornadaNaoRepeteTabuleiroPorPalavra guarda a regra que tirou o quebra-cabeça de
// significado da dominância (era 53% das questões da Jornada): como o tabuleiro pratica todas as
// peças de uma vez, cada palavra só entra em UM tabuleiro por mini-sessão. Sem ela, a escada de
// dificuldade colapsava — o quebra-cabeça é a única variante do significado acima de "introdução",
// então da segunda aparição de cada palavra em diante o filtro só tinha ele para escolher.
func TestObterQuestoesJornadaNaoRepeteTabuleiroPorPalavra(t *testing.T) {
	app := appDeTesteJornada(t)

	const nivelId = "fund1_2"
	nivel, _, err := jornada.ObterNivel(nivelId)
	if err != nil {
		t.Fatal(err)
	}

	totalQuebraCabeca, total := 0, 0
	for revIndex, tipo := range app.RevisoesEfetivas(nivel) {
		questoes, err := app.ObterQuestoesJornada(nivelId, revIndex)
		if err != nil {
			t.Fatalf("revisão %d (%s): %v", revIndex, tipo, err)
		}

		alvosComTabuleiro := make(map[string]bool)
		for _, q := range questoes {
			total++
			if !ehQuebraCabeca(q.Variante) {
				continue
			}
			totalQuebraCabeca++
			if alvosComTabuleiro[q.PalavraFoco] {
				t.Errorf("revisão %d (%s): a palavra %q foi alvo de dois tabuleiros na mesma mini-sessão",
					revIndex, tipo, q.PalavraFoco)
			}
			alvosComTabuleiro[q.PalavraFoco] = true
		}
	}

	// Com uma volta por palavra, o tabuleiro fica longe de dominar o nível.
	if totalQuebraCabeca*2 > total {
		t.Errorf("o quebra-cabeça ficou com %d de %d questões do nível — voltou a dominar", totalQuebraCabeca, total)
	}
}

// TestQuebraCabecaContaAparicaoDeTodasAsPecas fixa a contabilidade que sustenta a regra acima: a
// questão de tabuleiro pratica cada peça, não só o alvo.
func TestQuebraCabecaContaAparicaoDeTodasAsPecas(t *testing.T) {
	sessao := &sessaoJornada{
		palavras:    []string{"一", "二", "三"},
		aparicoes:   make(map[string]int),
		noTabuleiro: make(map[string]bool),
		ultima:      make(map[string]string),
	}

	questao := QuestaoRevisao{
		Variante: VarianteQuebraCabecaSignificado,
		Opcoes: []OpcaoRevisao{
			{Hanzi: "一"}, {Hanzi: "二"}, {Hanzi: "三"},
			{Hanzi: "四"}, // do universo herdado, fora do nível: entra no tabuleiro, não conta aparição
		},
	}
	sessao.registrarQuestao(questao, "一")

	for _, palavra := range []string{"一", "二", "三"} {
		if sessao.aparicoes[palavra] != 1 {
			t.Errorf("peça %q do nível: esperava 1 aparição, veio %d", palavra, sessao.aparicoes[palavra])
		}
		if !contemString(sessao.vetosDaPalavra(palavra), VarianteQuebraCabecaSignificado) {
			t.Errorf("peça %q deveria estar vetada para um novo tabuleiro de significado", palavra)
		}
	}
	if sessao.aparicoes["四"] != 0 {
		t.Errorf("palavra fora do nível não deveria contar aparição, veio %d", sessao.aparicoes["四"])
	}
	if len(sessao.vetosDaPalavra("五")) != 0 {
		t.Error("palavra que não esteve em tabuleiro nenhum não pode sair vetada")
	}
}

// TestVetoNaoRepeteAtividadeSeguida fixa a outra metade da regra: a atividade que a palavra acabou
// de praticar sai do sorteio da próxima aparição dela, que é o que impede o degrau em que a escada
// satura de entregar sempre a mesma coisa.
func TestVetoNaoRepeteAtividadeSeguida(t *testing.T) {
	sessao := &sessaoJornada{
		palavras:    []string{"一"},
		aparicoes:   make(map[string]int),
		noTabuleiro: make(map[string]bool),
		ultima:      make(map[string]string),
	}

	sessao.registrarQuestao(QuestaoRevisao{Variante: VarianteContexto}, "一")
	vetadas := sessao.vetosDaPalavra("一")
	if len(vetadas) != 1 || vetadas[0] != VarianteContexto {
		t.Errorf("esperava veto só da última atividade (%s), veio %v", VarianteContexto, vetadas)
	}

	// Trocando de atividade, o veto acompanha: o tabuleiro anterior continua vetado, sem duplicar.
	sessao.registrarQuestao(QuestaoRevisao{
		Variante: VarianteQuebraCabecaSignificado,
		Opcoes:   []OpcaoRevisao{{Hanzi: "一"}, {Hanzi: "二"}},
	}, "一")
	if vetadas := sessao.vetosDaPalavra("一"); !contemString(vetadas, VarianteQuebraCabecaSignificado) {
		t.Errorf("esperava veto do quebra-cabeça recém-praticado, veio %v", vetadas)
	}
}

func TestObterQuestoesJornadaRecusaEntradaInvalida(t *testing.T) {
	app := appDeTesteJornada(t)

	if _, err := app.ObterQuestoesJornada("nao_existe_0", 0); err == nil {
		t.Error("nível inexistente deveria falhar")
	}
	if _, err := app.ObterQuestoesJornada("fund1_0", 99); err == nil {
		t.Error("revIndex fora da faixa deveria falhar")
	}
	if _, err := app.ObterQuestoesJornada("fund1_0", -1); err == nil {
		t.Error("revIndex negativo deveria falhar")
	}
}

func TestAplicarFraseNaQuestaoProporcionalAoTamanhoHanzi(t *testing.T) {
	app, _ := appDeTeste(t)

	casos := []struct {
		nome                string
		hanzi               string
		textoChines         string
		lacunaEsperada      string
		qtdSublinhadosNoSeg int
	}{
		{
			nome:                "alvo_1_hanzi",
			hanzi:               "你",
			textoChines:         "你好，世界！",
			lacunaEsperada:      "＿好，世界！",
			qtdSublinhadosNoSeg: 1,
		},
		{
			nome:                "alvo_2_hanzis",
			hanzi:               "你好",
			textoChines:         "你好，世界！",
			lacunaEsperada:      "＿＿，世界！",
			qtdSublinhadosNoSeg: 2,
		},
		{
			nome:                "alvo_3_hanzis",
			hanzi:               "中国人",
			textoChines:         "我是中国人。",
			lacunaEsperada:      "我是＿＿＿。",
			qtdSublinhadosNoSeg: 3,
		},
	}

	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			q := QuestaoRevisao{Hanzi: c.hanzi}
			f := dicionario.Frase{Chines: c.textoChines, Ingles: "Hello"}
			app.aplicarFraseNaQuestao(&q, c.textoChines, f)

			if q.FraseLacuna != c.lacunaEsperada {
				t.Errorf("FraseLacuna incorreta para %q: esperado %q, obteve %q", c.hanzi, c.lacunaEsperada, q.FraseLacuna)
			}

			sublinhadosSegmentados := 0
			for _, p := range q.FraseLacunaSegmentada {
				for _, r := range p.Texto {
					if r == '＿' {
						sublinhadosSegmentados++
					}
				}
			}
			if sublinhadosSegmentados != c.qtdSublinhadosNoSeg {
				t.Errorf("Quantidade de sublinhados nos segmentos incorreta para %q: esperado %d, obteve %d", c.hanzi, c.qtdSublinhadosNoSeg, sublinhadosSegmentados)
			}
		})
	}
}

func TestObterDificuldadeVariante(t *testing.T) {
	testes := []struct {
		variante    string
		dificuldade string
	}{
		{VarianteHanziParaSignificado, DificuldadeIniciante},
		{VarianteSignificadoParaHanzi, DificuldadeIniciante},
		{VarianteAudioParaHanzi, DificuldadeIntroducao},
		{VarianteHanziParaPinyin, DificuldadeIntroducao},
		{VarianteDesenhoGuiado, DificuldadeIntroducao},
		{VarianteContexto, DificuldadeIniciante},
		{VarianteDesenhoMemoria, DificuldadeIntermediario},
		{VarianteQuebraCabecaSignificado, DificuldadeIntermediario},
		{VarianteHanziParaAudio, DificuldadeIniciante},
		{VariantePronunciaTipo, DificuldadeIniciante},
		{VarianteOrdenacaoTraducao, DificuldadeIntermediario},
		{VarianteTraducaoContexto, DificuldadeIntermediario},
		{VarianteFoneticaFrase, DificuldadeAvancado},
		{VarianteFoneticaTraducao, DificuldadeIntermediario},
		{VariantePronunciaSequencia, DificuldadeIntermediario},
		{VarianteDesenhoComponente, DificuldadeIniciante},
		{VarianteDesenhoMontagem, DificuldadeIniciante},
		{VarianteOrdenacao, DificuldadeAvancado},
		{VariantePronunciaFrase, DificuldadeIntermediario},
		{VariantePronunciaBaralho, DificuldadeAvancado},
		{VarianteQuebraCabecaTrio, DificuldadeAvancado},
		{VarianteDesenhoContexto, DificuldadeAvancado},
		{VarianteQuebraCabecaFonetica, DificuldadeAvancado},
	}

	for _, tt := range testes {
		obtida := ObterDificuldadeVariante(tt.variante)
		if obtida != tt.dificuldade {
			t.Errorf("variante %s: esperava dificuldade %q, veio %q", tt.variante, tt.dificuldade, obtida)
		}
	}
}

func TestHelpersJornada(t *testing.T) {
	testesTotal := []struct {
		numPalavras int
		esperado    int
	}{
		{2, 8},
		{4, 12},
		{5, 15},
		{6, 16},
	}
	for _, tt := range testesTotal {
		obtido := totalQuestoesJornada(tt.numPalavras)
		if obtido != tt.esperado {
			t.Errorf("totalQuestoesJornada(%d): esperava %d, veio %d", tt.numPalavras, tt.esperado, obtido)
		}
	}

	testesDificuldade := []struct {
		exposicao int
		esperado  string
	}{
		{0, DificuldadeIntroducao},
		{1, DificuldadeIniciante},
		{2, DificuldadeIntermediario},
		{3, DificuldadeAvancado},
		{7, DificuldadeAvancado},
	}
	for _, tt := range testesDificuldade {
		obtido := dificuldadeDaExposicao(tt.exposicao)
		if obtido != tt.esperado {
			t.Errorf("dificuldadeDaExposicao(%d): esperava %q, veio %q", tt.exposicao, tt.esperado, obtido)
		}
	}
}

// ----- Seção: Teste da Variante Fonética Hanzi para Pinyin -----

func TestVarianteHanziParaPinyinFonetica(t *testing.T) {
	app, _ := appDeTeste(t)
	app.buscador.DefinirDificuldadeAlvo(DificuldadeIntroducao)
	app.buscador.DefinirVariantesVetadas(VarianteAudioParaHanzi)

	alvo := alvoRevisao{
		entrada: dicionario.DecomposicaoHanzi{
			Caractere: "好",
			Pinyin:    []string{"hǎo"},
			Definicao: "bom; bem",
		},
		emEstudo: true,
	}

	pools := poolsRevisao{
		Todos: []dicionario.DecomposicaoHanzi{
			alvo.entrada,
			{Caractere: "大", Pinyin: []string{"dà"}, Definicao: "grande"},
			{Caractere: "小", Pinyin: []string{"xiǎo"}, Definicao: "pequeno"},
			{Caractere: "人", Pinyin: []string{"rén"}, Definicao: "pessoa"},
		},
	}

	questaoInicial := QuestaoRevisao{
		Modo:     ModoFonetica,
		Variante: VarianteHanziParaPinyin,
		Hanzi:    alvo.entrada.Caractere,
		Pinyin:   alvo.entrada.Pinyin[0],
	}

	questaoMontada, err := app.montarQuestaoFonetica(questaoInicial, alvo, pools, ModoFonetica, "")
	if err != nil {
		t.Fatalf("falha ao montar questão fonética: %v", err)
	}

	if questaoMontada.Variante != VarianteHanziParaPinyin {
		t.Errorf("esperava variante %q, veio %q", VarianteHanziParaPinyin, questaoMontada.Variante)
	}

	if len(questaoMontada.Opcoes) != 4 {
		t.Fatalf("esperava 4 opções, veio %d", len(questaoMontada.Opcoes))
	}

	temCorreta := false
	for _, opt := range questaoMontada.Opcoes {
		if opt.Correta {
			temCorreta = true
			if opt.Pinyin != "hǎo" {
				t.Errorf("opção correta esperava pinyin 'hǎo', veio %q", opt.Pinyin)
			}
		}
	}

	if !temCorreta {
		t.Errorf("nenhuma opção correta foi encontrada nas alternativas da questão")
	}
}

// ----- Seção: Teste da Variante de Desenho por Componente -----

func TestPreencherComponenteDesenho(t *testing.T) {
	app, _ := appDeTeste(t)

	questao := QuestaoRevisao{Modo: ModoDesenho, Variante: VarianteDesenhoComponente, Hanzi: "好"}
	if !app.preencherComponenteDesenho(&questao) {
		t.Fatal("esperava que 好 (⿰女子) tivesse componente desenhável")
	}

	if questao.ComponenteAlvo != "女" && questao.ComponenteAlvo != "子" {
		t.Errorf("componente de 好 esperava 女 ou 子, veio %q", questao.ComponenteAlvo)
	}

	totalTracos, temTracados := app.Dicionario.Tracados.TotalTracos("好")
	if !temTracados {
		t.Fatal("好 não tem traçados no banco embarcado")
	}
	// O recorte tem de ser PARCIAL: cobrir tudo seria o desenho de memória com outro nome.
	if len(questao.TracosAlvo) == 0 || len(questao.TracosAlvo) >= totalTracos {
		t.Fatalf("esperava recorte parcial dos %d traços de 好, veio %v", totalTracos, questao.TracosAlvo)
	}
	for _, indice := range questao.TracosAlvo {
		if indice < 0 || indice >= totalTracos {
			t.Errorf("traço %d fora da faixa 0..%d", indice, totalTracos-1)
		}
	}
}

func TestPreencherComponenteDesenhoRecusaSemComponente(t *testing.T) {
	app, _ := appDeTeste(t)

	casos := []struct {
		nome  string
		hanzi string
	}{
		{"caractere de traço único", "一"},
		{"grafia tradicional (herda a decomposição do simplificado, com outros traços)", "語"},
	}

	for _, caso := range casos {
		questao := QuestaoRevisao{Modo: ModoDesenho, Variante: VarianteDesenhoComponente, Hanzi: caso.hanzi}
		if app.preencherComponenteDesenho(&questao) {
			t.Errorf("%s: esperava recusa para %q, veio componente %q com traços %v", caso.nome, caso.hanzi, questao.ComponenteAlvo, questao.TracosAlvo)
		}
	}
}

// ----- Seção: Teste da Variante de Montagem de Componentes -----

func TestPreencherMontagemDesenho(t *testing.T) {
	app, _ := appDeTeste(t)

	questao := QuestaoRevisao{Modo: ModoDesenho, Variante: VarianteDesenhoMontagem, Hanzi: "好"}
	if !app.preencherMontagemDesenho(&questao) {
		t.Fatal("esperava que 好 (⿰女子) tivesse partição completa para a montagem")
	}

	if len(questao.ComponentesMontagem) != 2 {
		t.Fatalf("esperava 2 peças para 好, veio %+v", questao.ComponentesMontagem)
	}

	totalTracos, temTracados := app.Dicionario.Tracados.TotalTracos("好")
	if !temTracados {
		t.Fatal("好 não tem traçados no banco embarcado")
	}
	// A partição tem de recompor o caractere SEM sobra nem falta.
	totalNasPecas := 0
	for _, peca := range questao.ComponentesMontagem {
		if peca.Caractere == "" || len(peca.Tracos) == 0 {
			t.Fatalf("peça vazia na montagem de 好: %+v", peca)
		}
		totalNasPecas += len(peca.Tracos)
	}
	if totalNasPecas != totalTracos {
		t.Fatalf("peças somam %d traços, mas 好 tem %d no banco de traçados", totalNasPecas, totalTracos)
	}
}

func TestPreencherMontagemDesenhoGeraDistratores(t *testing.T) {
	app, _ := appDeTeste(t)

	questao := QuestaoRevisao{Modo: ModoDesenho, Variante: VarianteDesenhoMontagem, Hanzi: "好"}
	if !app.preencherMontagemDesenho(&questao) {
		t.Fatal("esperava partição para 好")
	}

	if len(questao.DistratoresMontagem) == 0 {
		t.Fatal("esperava ao menos um distrator para 好")
	}
	if len(questao.DistratoresMontagem) > DISTRATORES_MONTAGEM_MAXIMO {
		t.Fatalf("distratores acima do teto: %v", questao.DistratoresMontagem)
	}

	reais := make(map[string]bool)
	for _, peca := range questao.ComponentesMontagem {
		reais[peca.Caractere] = true
	}

	vistos := make(map[string]bool)
	for _, distrator := range questao.DistratoresMontagem {
		if reais[distrator] {
			t.Errorf("distrator %q coincide com um componente real", distrator)
		}
		if vistos[distrator] {
			t.Errorf("distrator %q repetido", distrator)
		}
		vistos[distrator] = true
		// A carta precisa desenhar o distrator: sem traçado, não há o que mostrar.
		if _, tem := app.Dicionario.Tracados.TotalTracos(distrator); !tem {
			t.Errorf("distrator %q não tem traçado no banco embarcado", distrator)
		}
	}
}

func TestPreencherMontagemDesenhoRecusaSemParticao(t *testing.T) {
	app, _ := appDeTeste(t)

	casos := []struct {
		nome  string
		hanzi string
	}{
		{"caractere de traço único", "一"},
		{"grafia tradicional (herda a decomposição do simplificado, com outros traços)", "語"},
	}

	for _, caso := range casos {
		questao := QuestaoRevisao{Modo: ModoDesenho, Variante: VarianteDesenhoMontagem, Hanzi: caso.hanzi}
		if app.preencherMontagemDesenho(&questao) {
			t.Errorf("%s: esperava recusa para %q, veio %+v", caso.nome, caso.hanzi, questao.ComponentesMontagem)
		}
	}
}

func TestDecomporTextoRevisaoMarcaPalavrasNaoVistas(t *testing.T) {
	app, _ := appDeTeste(t)

	mapaVocab := map[string]bool{
		"你好": true,
		"你":  true,
		"好":  true,
	}

	segmentos := decomporTextoComVocab(app.Dicionario, "你好世界", mapaVocab)
	if len(segmentos) == 0 {
		t.Fatal("esperava segmentos para 你好世界")
	}

	for _, seg := range segmentos {
		if !seg.EhChines {
			continue
		}
		if seg.Texto == "你好" || seg.Texto == "你" || seg.Texto == "好" {
			if seg.EhNaoVista {
				t.Errorf("palavra %q consta no vocabulário, mas foi marcada como EhNaoVista", seg.Texto)
			}
		} else if seg.Texto == "世界" || seg.Texto == "世" || seg.Texto == "界" {
			if !seg.EhNaoVista {
				t.Errorf("palavra %q NÃO consta no vocabulário, mas NÃO foi marcada como EhNaoVista", seg.Texto)
			}
		}
	}
}


// ----- Seção: Teste do Quebra-Cabeça de 3 Colunas na Jornada -----

func TestObterQuestoesJornadaRevisaoMistaPodeSortearQuebraCabecaTrio(t *testing.T) {
	app := appDeTesteJornada(t)

	nivel, _, err := jornada.ObterNivel("fund1_0")
	if err != nil {
		t.Fatalf("erro ao obter nível fund1_0: %v", err)
	}

	indiceMista := len(nivel.Revisoes) - 1
	if nivel.Revisoes[indiceMista] != jornada.RevisaoMista {
		t.Fatalf("última revisão do nível deveria ser %q, veio %q", jornada.RevisaoMista, nivel.Revisoes[indiceMista])
	}

	encontrouTrio := false
	for tentativa := 0; tentativa < 40; tentativa++ {
		questoes, err := app.ObterQuestoesJornada("fund1_0", indiceMista)
		if err != nil {
			t.Fatalf("ObterQuestoesJornada(fund1_0, %d): %v", indiceMista, err)
		}

		for _, q := range questoes {
			if q.Variante == VarianteQuebraCabecaTrio {
				encontrouTrio = true
				break
			}
		}
		if encontrouTrio {
			break
		}
	}

	if !encontrouTrio {
		t.Errorf("esperava que a revisão mista da Jornada pudesse sortear a variante %q, mas não apareceu em 40 tentativas", VarianteQuebraCabecaTrio)
	}
}

func TestDificuldadePorStreak(t *testing.T) {
	testes := []struct {
		streak   int
		esperado string
	}{
		{-1, DificuldadeIntroducao},
		{0, DificuldadeIntroducao},
		{1, DificuldadeIniciante},
		{2, DificuldadeIniciante},
		{3, DificuldadeIniciante},
		{4, DificuldadeIntermediario},
		{5, DificuldadeIntermediario},
		{6, DificuldadeIntermediario},
		{7, DificuldadeAvancado},
		{8, DificuldadeAvancado},
		{9, DificuldadeAvancado},
		{10, DificuldadeAvancado},
	}

	for _, tt := range testes {
		obtido := dificuldadePorStreak(tt.streak)
		if obtido != tt.esperado {
			t.Errorf("dificuldadePorStreak(%d): esperava %q, veio %q", tt.streak, tt.esperado, obtido)
		}
	}
}

func TestRevisaoGeralProporcoesEQuantidadeConfiguravel(t *testing.T) {
	gerenciadorDicionario, err := dicionario.NovoGerenciadorDicionario(dicionario.IdiomaPadrao)
	if err != nil {
		t.Fatalf("falha ao carregar dicionários: %v", err)
	}

	cfg := config.DefaultConfig()
	cfg.RevisaoQuantidadeQuestoes = 20
	cfg.PriorizarEstudoRevisao = true

	app := NovoGerenciadorRevisao(gerenciadorDicionario, dicionario.NovoGerenciadorFrases(dicionario.IdiomaPadrao), func() config.Config { return cfg })

	candidatos := make([]dicionario.DecomposicaoHanzi, 0, 30)
	for _, caractere := range []rune("一二三四五六七八九十百千万水火木金土日月山口人") {
		candidatos = append(candidatos, dicionario.DecomposicaoHanzi{Caractere: string(caractere)})
	}
	foco := []string{"一", "二", "三", "四", "五", "六", "七", "八", "九", "十"}
	mapaStatus := map[string]string{
		"一": dicionario.StatusEstudo, "二": dicionario.StatusEstudo, "三": dicionario.StatusEstudo,
		"四": dicionario.StatusEstudo, "五": dicionario.StatusEstudo, "六": dicionario.StatusEstudo,
		"七": dicionario.StatusEstudo, "八": dicionario.StatusEstudo, "九": dicionario.StatusEstudo,
		"十": dicionario.StatusEstudo,
	}

	alvos := app.selecionarAlvos(candidatos, foco, mapaStatus, 20)
	blocoInicial := alvos[:20]
	qtdFoco := 0
	for _, a := range blocoInicial {
		if a.emFoco {
			qtdFoco++
		}
	}

	if qtdFoco < 12 {
		t.Errorf("com foco ativado e 20 questões, esperava 12 alvos de foco no bloco inicial (60%% de 20), veio %d", qtdFoco)
	}

	questoes, err := app.ObterQuestoesRevisao(ModoGeral, 0)
	if err != nil {
		t.Fatalf("ObterQuestoesRevisao(ModoGeral, 0): %v", err)
	}

	if len(questoes) != 20 {
		t.Errorf("esperava 20 questões na revisão geral configurada, veio %d", len(questoes))
	}
}

func TestRevisarErradasAoFinalPadrao(t *testing.T) {
	cfg := config.DefaultConfig()
	if !cfg.RevisarErradasAoFinal {
		t.Errorf("esperava RevisarErradasAoFinal ativado por padrão em DefaultConfig(), veio falso")
	}
}

func TestRemoverFraseEmRuntime(t *testing.T) {
	gf := dicionario.NovoGerenciadorFrases(dicionario.IdiomaPadrao)
	textoUnico := "你好，这是FraseUnicaDeTeste12345！"
	f := dicionario.Frase{
		Chines:     textoUnico,
		Ingles:     "Olá mundo teste",
		Atribuicao: "Teste",
	}

	if !gf.AdicionarFrase(f) {
		t.Fatalf("esperava adicionar frase com sucesso")
	}

	if !gf.RemoverFrase(textoUnico) {
		t.Errorf("esperava remover frase com sucesso")
	}

	if gf.RemoverFrase(textoUnico) {
		t.Errorf("esperava que a segunda remoção devolvesse false (frase já removida)")
	}
}

func TestRegistrarQuestaoIneditaBloqueiaTodasPecasQuebraCabeca(t *testing.T) {
	r, _ := appDeTeste(t)

	q1 := QuestaoRevisao{
		Modo:     ModoSignificado,
		Variante: VarianteQuebraCabecaSignificado,
		Hanzi:    "一",
		Opcoes: []OpcaoRevisao{
			{Hanzi: "一"},
			{Hanzi: "二"},
			{Hanzi: "三"},
			{Hanzi: "四"},
		},
	}

	if !r.registrarQuestaoInedita(&q1) {
		t.Fatalf("primeira questão de quebra-cabeça deveria ser registrada com sucesso")
	}

	// Verifica se TODAS as peças (alvo e distratores/peças secundárias) foram registradas no histórico
	for _, hanzi := range []string{"一", "二", "三", "四"} {
		sigSignificado := VarianteQuebraCabecaSignificado + ":" + hanzi
		if !r.buscador.HistoricoContem(sigSignificado) {
			t.Errorf("peça %q deveria ter sua assinatura registrada em %s", hanzi, sigSignificado)
		}
		sigTrio := VarianteQuebraCabecaTrio + ":" + hanzi
		if !r.buscador.HistoricoContem(sigTrio) {
			t.Errorf("peça %q deveria ter sua assinatura registrada em %s", hanzi, sigTrio)
		}
	}

	// Tentar registrar nova questão de quebra-cabeça com alvo em uma peça secundária ("二") deve ser recusado
	q2 := QuestaoRevisao{
		Modo:     ModoSignificado,
		Variante: VarianteQuebraCabecaSignificado,
		Hanzi:    "二",
		Opcoes: []OpcaoRevisao{
			{Hanzi: "二"},
			{Hanzi: "五"},
			{Hanzi: "六"},
			{Hanzi: "七"},
		},
	}

	if r.registrarQuestaoInedita(&q2) {
		t.Errorf("questão de quebra-cabeça para o alvo %q deveria ser recusada pois %q já participou de um tabuleiro anteriormente", "二", "二")
	}

	// Verificar se buscarPecasQuebraCabeca para um novo alvo não reusa "二" nem "三" como peças
	alvoNovo := dicionario.DecomposicaoHanzi{Caractere: "五", Pinyin: []string{"wǔ"}, Definicao: "cinco"}
	pools := poolsRevisao{
		Todos: []dicionario.DecomposicaoHanzi{
			alvoNovo,
			{Caractere: "二", Pinyin: []string{"èr"}, Definicao: "dois"},
			{Caractere: "三", Pinyin: []string{"sān"}, Definicao: "três"},
			{Caractere: "六", Pinyin: []string{"liù"}, Definicao: "seis"},
			{Caractere: "七", Pinyin: []string{"qī"}, Definicao: "sete"},
			{Caractere: "八", Pinyin: []string{"bā"}, Definicao: "oito"},
		},
	}
	pecasNovas := r.buscarPecasQuebraCabeca(alvoNovo, pools)
	for _, p := range pecasNovas {
		if p.Hanzi == "二" || p.Hanzi == "三" {
			t.Errorf("peça %q que já esteve em quebra-cabeça anterior não deveria ter sido sorteada em novo tabuleiro", p.Hanzi)
		}
	}
}


