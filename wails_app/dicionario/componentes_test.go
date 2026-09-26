package dicionario

import (
	"reflect"
	"testing"
)

// ----- Componentes desenháveis (fatiamento dos traços pela decomposição) -----

// bancoComCaractere monta um banco com UM caractere curado, sem tocar no arquivo embarcado.
func bancoComCaractere(entrada *entradaFundida) *Banco {
	banco := NovoBanco()
	banco.indexarEntrada(entrada)
	return banco
}

func TestComponentesDesenhaveisDecomposicaoSimples(t *testing.T) {
	// 好 = ⿰女子: os 3 primeiros traços são do 女, os 3 últimos do 子.
	banco := bancoComCaractere(&entradaFundida{
		Simplificado:     "好",
		Definicao:        "bom",
		Decomposicao:     "⿰女子",
		Radical:          "女",
		Correspondencias: [][]int{{0}, {0}, {0}, {1}, {1}, {1}},
	})

	componentes := banco.ComponentesDesenhaveis("好")
	esperado := []ComponenteHanzi{
		{Caractere: "女", Tracos: []int{0, 1, 2}},
		{Caractere: "子", Tracos: []int{3, 4, 5}},
	}
	if !reflect.DeepEqual(componentes, esperado) {
		t.Fatalf("componentes de 好 errados:\n veio     %+v\n esperado %+v", componentes, esperado)
	}
}

func TestComponentesDesenhaveisDecomposicaoAninhada(t *testing.T) {
	// 器 = ⿳⿰口口犬⿰口口: as correspondências são CAMINHOS na árvore, então os quatro 口 são quatro
	// componentes distintos — o de caminho [2,1] é o último, e não pode se confundir com o de [0,1].
	banco := bancoComCaractere(&entradaFundida{
		Simplificado: "器",
		Definicao:    "utensílio",
		Decomposicao: "⿳⿰口口犬⿰口口",
		Radical:      "口",
		Correspondencias: [][]int{
			{0, 0}, {0, 0}, {0, 0},
			{0, 1}, {0, 1}, {0, 1},
			{1}, {1}, {1}, {1},
			{2, 0}, {2, 0}, {2, 0},
			{2, 1}, {2, 1}, {2, 1},
		},
	})

	componentes := banco.ComponentesDesenhaveis("器")
	esperado := []ComponenteHanzi{
		{Caractere: "口", Tracos: []int{0, 1, 2}},
		{Caractere: "口", Tracos: []int{3, 4, 5}},
		{Caractere: "犬", Tracos: []int{6, 7, 8, 9}},
		{Caractere: "口", Tracos: []int{10, 11, 12}},
		{Caractere: "口", Tracos: []int{13, 14, 15}},
	}
	if !reflect.DeepEqual(componentes, esperado) {
		t.Fatalf("componentes de 器 errados:\n veio     %+v\n esperado %+v", componentes, esperado)
	}
}

func TestComponentesDesenhaveisDescartaTracoUnicoDesconhecidoEhNaoAtribuido(t *testing.T) {
	// Decomposição com um componente de traço único (丨), um desconhecido (？) e um traço que o
	// makemeahanzi não atribuiu (caminho vazio): sobra só o componente com traços de verdade.
	banco := bancoComCaractere(&entradaFundida{
		Simplificado:     "X",
		Definicao:        "fictício",
		Decomposicao:     "⿳丨？口",
		Radical:          "口",
		Correspondencias: [][]int{{0}, {1}, {1}, nil, {2}, {2}, {2}},
	})

	componentes := banco.ComponentesDesenhaveis("X")
	esperado := []ComponenteHanzi{{Caractere: "口", Tracos: []int{4, 5, 6}}}
	if !reflect.DeepEqual(componentes, esperado) {
		t.Fatalf("componentes errados:\n veio     %+v\n esperado %+v", componentes, esperado)
	}
}

func TestComponentesDesenhaveisRecusaCoberturaTotalEhDecomposicaoInvalida(t *testing.T) {
	casos := []struct {
		nome    string
		entrada *entradaFundida
	}{
		{
			// Um componente só, cobrindo o caractere inteiro: isso é o desenho de memória, não a
			// atividade de componente.
			nome: "componente cobre todos os traços",
			entrada: &entradaFundida{
				Simplificado:     "一",
				Decomposicao:     "⿰女子",
				Correspondencias: [][]int{{0}, {0}, {0}},
			},
		},
		{
			// Decomposição truncada: o ⿰ pede 2 operandos e só tem 1.
			nome: "expressão IDS truncada",
			entrada: &entradaFundida{
				Simplificado:     "Y",
				Decomposicao:     "⿰女",
				Correspondencias: [][]int{{0}, {0}, {0}, {1}, {1}, {1}},
			},
		},
		{
			// Caractere sem correspondências (veio só do CEDICT ou de uma fusão antiga).
			nome: "sem correspondências",
			entrada: &entradaFundida{
				Simplificado: "Z",
				Decomposicao: "⿰女子",
			},
		},
	}

	for _, caso := range casos {
		banco := bancoComCaractere(caso.entrada)
		if componentes := banco.ComponentesDesenhaveis(caso.entrada.Simplificado); componentes != nil {
			t.Errorf("%s: esperava nenhum componente, veio %+v", caso.nome, componentes)
		}
	}
}

func TestComponentesDesenhaveisNaoUsaFallbackDoSimplificado(t *testing.T) {
	// 語 tem 14 traços e 语 tem 9: BuscarCaractere devolve os dados do simplificado para a grafia
	// tradicional, e usar esses índices no traçado do tradicional desenharia a parte errada.
	banco := bancoComCaractere(&entradaFundida{
		Simplificado:     "语",
		Tradicional:      []string{"語"},
		Decomposicao:     "⿰讠吾",
		Radical:          "讠",
		Correspondencias: [][]int{{0}, {0}, {1}, {1}, {1}, {1}, {1}, {1}, {1}},
	})

	if componentes := banco.ComponentesDesenhaveis("語"); componentes != nil {
		t.Errorf("esperava nenhum componente para a grafia tradicional, veio %+v", componentes)
	}
	if componentes := banco.ComponentesDesenhaveis("语"); len(componentes) != 2 {
		t.Errorf("esperava 2 componentes para 语, veio %+v", componentes)
	}
}

// ----- Partição em componentes (insumo da atividade de montagem) -----

func TestParticaoEmComponentesManteMesmoFolhaDeTracoUnico(t *testing.T) {
	// Diferente de ComponentesDesenhaveis, a partição não descarta a folha de traço único: na
	// montagem toda peça precisa existir, senão o caractere termina com buraco.
	banco := bancoComCaractere(&entradaFundida{
		Simplificado:     "旦",
		Definicao:        "amanhecer",
		Decomposicao:     "⿱日一",
		Radical:          "日",
		Correspondencias: [][]int{{0}, {0}, {0}, {0}, {1}},
	})

	componentes := banco.ParticaoEmComponentes("旦")
	esperado := []ComponenteHanzi{
		{Caractere: "日", Tracos: []int{0, 1, 2, 3}},
		{Caractere: "一", Tracos: []int{4}},
	}
	if !reflect.DeepEqual(componentes, esperado) {
		t.Fatalf("partição de 旦 errada:\n veio     %+v\n esperado %+v", componentes, esperado)
	}
}

func TestParticaoEmComponentesRecusaParticaoIncompleta(t *testing.T) {
	casos := []struct {
		nome    string
		entrada *entradaFundida
	}{
		{
			// Traço sem componente atribuído: a peça dele não existiria e a montagem terminaria furada.
			nome: "traço sem componente",
			entrada: &entradaFundida{
				Simplificado:     "A",
				Decomposicao:     "⿰女子",
				Correspondencias: [][]int{{0}, {0}, {0}, nil, {1}, {1}},
			},
		},
		{
			// Folha que o makemeahanzi não soube nomear: a peça não teria caractere para exibir.
			nome: "folha desconhecida",
			entrada: &entradaFundida{
				Simplificado:     "B",
				Decomposicao:     "⿱？口",
				Correspondencias: [][]int{{0}, {0}, {1}, {1}, {1}},
			},
		},
		{
			// Todos os traços numa folha só: peça única não é montagem nenhuma.
			nome: "folha única com todos os traços",
			entrada: &entradaFundida{
				Simplificado:     "C",
				Decomposicao:     "⿰女子",
				Correspondencias: [][]int{{0}, {0}, {0}},
			},
		},
		{
			// Decomposição truncada: o ⿰ pede 2 operandos e só tem 1.
			nome: "expressão IDS truncada",
			entrada: &entradaFundida{
				Simplificado:     "D",
				Decomposicao:     "⿰女",
				Correspondencias: [][]int{{0}, {0}, {0}, {1}, {1}, {1}},
			},
		},
		{
			nome: "sem correspondências",
			entrada: &entradaFundida{
				Simplificado: "E",
				Decomposicao: "⿰女子",
			},
		},
	}

	for _, caso := range casos {
		banco := bancoComCaractere(caso.entrada)
		if componentes := banco.ParticaoEmComponentes(caso.entrada.Simplificado); componentes != nil {
			t.Errorf("%s: esperava nenhuma partição, veio %+v", caso.nome, componentes)
		}
	}
}

func TestParticaoEmComponentesNaoUsaFallbackDoSimplificado(t *testing.T) {
	// Mesma regra de ComponentesDesenhaveis: a grafia tradicional herda os dados do simplificado,
	// cujos traços são outros — a partição para ela tem de vir vazia.
	banco := bancoComCaractere(&entradaFundida{
		Simplificado:     "语",
		Tradicional:      []string{"語"},
		Decomposicao:     "⿰讠吾",
		Radical:          "讠",
		Correspondencias: [][]int{{0}, {0}, {1}, {1}, {1}, {1}, {1}, {1}, {1}},
	})

	if componentes := banco.ParticaoEmComponentes("語"); componentes != nil {
		t.Errorf("esperava nenhuma partição para a grafia tradicional, veio %+v", componentes)
	}
	if componentes := banco.ParticaoEmComponentes("语"); len(componentes) != 2 {
		t.Errorf("esperava 2 peças para 语, veio %+v", componentes)
	}
}

func TestParticaoEmComponentesBateComOsTracadosEmbarcados(t *testing.T) {
	// Guarda do contrato entre os DOIS arquivos embarcados: quando há partição, a soma das peças tem
	// de bater EXATAMENTE com o total do banco de traçados — sem sobra nem falta, senão a montagem
	// termina com buraco no canvas.
	banco := NovoBanco()
	if err := banco.Carregar(IdiomaPadrao); err != nil {
		t.Fatalf("falha ao carregar o dicionário: %v", err)
	}
	tracados := NovoBancoTracados()

	comParticao := 0
	for _, entrada := range banco.CandidatosRevisao() {
		componentes := banco.ParticaoEmComponentes(entrada.Caractere)
		if len(componentes) == 0 {
			continue
		}
		totalTracos, temTracados := tracados.TotalTracos(entrada.Caractere)
		if !temTracados {
			continue
		}

		comParticao++
		totalNasPecas := 0
		for _, componente := range componentes {
			totalNasPecas += len(componente.Tracos)
		}
		if totalNasPecas != totalTracos {
			t.Fatalf("%s: partição soma %d traços, mas o banco de traçados tem %d",
				entrada.Caractere, totalNasPecas, totalTracos)
		}
	}

	if comParticao == 0 {
		t.Fatal("nenhum candidato da revisão tem partição completa — a atividade de montagem nunca sairia")
	}
	t.Logf("candidatos da revisão com partição completa: %d", comParticao)
}

func TestComponentesDesenhaveisBatemComOsTracadosEmbarcados(t *testing.T) {
	// Guarda do contrato entre os DOIS arquivos embarcados: todo índice devolvido tem de existir no
	// banco de traçados, senão o canvas do frontend recorta fora da faixa.
	banco := NovoBanco()
	if err := banco.Carregar(IdiomaPadrao); err != nil {
		t.Fatalf("falha ao carregar o dicionário: %v", err)
	}
	tracados := NovoBancoTracados()

	comComponentes := 0
	for _, entrada := range banco.CandidatosRevisao() {
		componentes := banco.ComponentesDesenhaveis(entrada.Caractere)
		if len(componentes) == 0 {
			continue
		}
		totalTracos, temTracados := tracados.TotalTracos(entrada.Caractere)
		if !temTracados {
			continue
		}

		comComponentes++
		for _, componente := range componentes {
			for _, indice := range componente.Tracos {
				if indice < 0 || indice >= totalTracos {
					t.Fatalf("%s: componente %q aponta para o traço %d, mas o caractere tem %d traços",
						entrada.Caractere, componente.Caractere, indice, totalTracos)
				}
			}
		}
	}

	if comComponentes == 0 {
		t.Fatal("nenhum candidato da revisão tem componente desenhável — a atividade nunca sairia")
	}
	t.Logf("candidatos da revisão com componente desenhável: %d", comComponentes)
}
