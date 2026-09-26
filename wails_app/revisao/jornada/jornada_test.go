package jornada

import (
	"strconv"
	"strings"
	"testing"
)

// ----- Carga da árvore real -----

func TestObterArvoreCarregaEValida(t *testing.T) {
	arvore, err := ObterArvore()
	if err != nil {
		t.Fatalf("ObterArvore() falhou: %v", err)
	}
	if len(arvore.Ramos) == 0 {
		t.Fatal("árvore carregada sem ramos")
	}
	if arvore.Mapa.Largura <= 0 || arvore.Mapa.Altura <= 0 {
		t.Fatalf("mapa sem dimensões: %+v", arvore.Mapa)
	}

	for _, ramo := range arvore.Ramos {
		for iNivel, nivel := range ramo.Niveis {
			esperado := ramo.Id + "_" + strconv.Itoa(iNivel)
			if nivel.Id != esperado {
				t.Errorf("id do nível %d de %q = %q; esperado %q", iNivel, ramo.Id, nivel.Id, esperado)
			}
		}
	}
}

func TestObterNivel(t *testing.T) {
	nivel, ramo, err := ObterNivel("fund1_0")
	if err != nil {
		t.Fatalf("ObterNivel(fund1_0) falhou: %v", err)
	}
	if ramo.Id != "fund1" {
		t.Errorf("ramo = %q; esperado fund1", ramo.Id)
	}
	if len(nivel.Palavras) < minimoPalavrasPorNivel {
		t.Errorf("nível fund1_0 com %d palavras", len(nivel.Palavras))
	}

	if _, _, err := ObterNivel("nao_existe_9"); err == nil {
		t.Error("ObterNivel de nível inexistente deveria falhar")
	}
}

// ----- Universo de palavras da Jornada -----

func TestPalavrasAteNivel(t *testing.T) {
	primeiro, _, err := ObterNivel("fund1_0")
	if err != nil {
		t.Fatal(err)
	}

	// Primeiro nível do ramo raiz: o universo é só ele mesmo.
	universo, err := PalavrasAteNivel("fund1_0")
	if err != nil {
		t.Fatalf("PalavrasAteNivel(fund1_0): %v", err)
	}
	if len(universo) != len(primeiro.Palavras) {
		t.Errorf("esperava só as %d palavras do próprio nível, veio %d: %v", len(primeiro.Palavras), len(universo), universo)
	}

	// Nível seguinte: acumula os anteriores do mesmo ramo, sem alcançar os posteriores.
	segundo, ramo, err := ObterNivel("fund1_1")
	if err != nil {
		t.Fatal(err)
	}
	universo, err = PalavrasAteNivel("fund1_1")
	if err != nil {
		t.Fatal(err)
	}
	contem := conjuntoDe(universo)
	for _, palavra := range append(append([]string{}, primeiro.Palavras...), segundo.Palavras...) {
		if !contem[palavra] {
			t.Errorf("universo de fund1_1 deveria conter %q", palavra)
		}
	}
	if len(ramo.Niveis) > 2 {
		for _, palavra := range ramo.Niveis[2].Palavras {
			if contem[palavra] {
				t.Errorf("universo de fund1_1 NÃO pode conter %q, de um nível ainda não alcançado", palavra)
			}
		}
	}
}

func TestPalavrasAteNivelHerdaDosRamosAncestrais(t *testing.T) {
	filho, ramoFilho, err := ObterNivel("cotidiano1_0")
	if err != nil {
		t.Fatal(err)
	}
	if ramoFilho.Pai == "" {
		t.Fatalf("o teste pressupõe que %q tenha ramo pai", ramoFilho.Id)
	}

	universo, err := PalavrasAteNivel("cotidiano1_0")
	if err != nil {
		t.Fatal(err)
	}
	contem := conjuntoDe(universo)

	// Todo o ramo pai entra (ele foi concluído para chegar aqui), além do próprio nível.
	paiNivel, _, err := ObterNivel(ramoFilho.Pai + "_0")
	if err != nil {
		t.Fatal(err)
	}
	for _, palavra := range paiNivel.Palavras {
		if !contem[palavra] {
			t.Errorf("universo de cotidiano1_0 deveria herdar %q do ramo pai %q", palavra, ramoFilho.Pai)
		}
	}
	for _, palavra := range filho.Palavras {
		if !contem[palavra] {
			t.Errorf("universo de cotidiano1_0 deveria conter a própria palavra %q", palavra)
		}
	}

	// Ramo IRMÃO (mesmo pai, caminho paralelo que o usuário pode nem ter feito) fica de fora — só
	// contam as palavras exclusivas dele, já que a árvore pode repetir uma palavra entre ramos.
	arvore, _ := ObterArvore()
	doAncestral := make(map[string]bool)
	for _, ramo := range arvore.Ramos {
		if ramo.Id != ramoFilho.Pai {
			continue
		}
		for _, nivel := range ramo.Niveis {
			for _, palavra := range nivel.Palavras {
				doAncestral[palavra] = true
			}
		}
	}

	for _, irmao := range arvore.Ramos {
		if irmao.Id == ramoFilho.Id || irmao.Pai != ramoFilho.Pai {
			continue
		}
		for _, nivel := range irmao.Niveis {
			for _, palavra := range nivel.Palavras {
				if !doAncestral[palavra] && contem[palavra] {
					t.Errorf("universo de cotidiano1_0 não deveria conter %q, exclusiva do ramo irmão %q", palavra, irmao.Id)
				}
			}
		}
	}
}

func conjuntoDe(palavras []string) map[string]bool {
	conjunto := make(map[string]bool, len(palavras))
	for _, palavra := range palavras {
		conjunto[palavra] = true
	}
	return conjunto
}

// ----- Validação -----

// arvoreValida devolve uma árvore mínima que passa em todas as regras, para os casos abaixo a
// deformarem em um ponto só.
func arvoreValida() Arvore {
	return Arvore{
		Mapa: Mapa{Largura: 2000, Altura: 2100},
		Ramos: []Ramo{
			{
				Id: "raiz", Nome: "Raiz", Icone: "fundamentos",
				Dificuldade: "iniciante", Pai: "", Tema: "",
				RotuloMapa: []int{100, 100},
				Niveis: []Nivel{
					{Id: "raiz_0", Titulo: "Um", Pos: []int{100, 200},
						Palavras: []string{"你好", "谢谢", "再见"},
						Revisoes: []string{"palavras", "mista"}},
				},
			},
			{
				Id: "filho", Nome: "Filho", Icone: "cotidiano",
				Dificuldade: "intermediario", Pai: "raiz", Tema: "日常生活",
				RotuloMapa: []int{300, 100},
				Niveis: []Nivel{
					{Id: "filho_0", Titulo: "Dois", Pos: []int{300, 200},
						Palavras: []string{"起床", "洗澡", "早饭"},
						Revisoes: []string{"palavras", "desenho", "mista"}},
				},
			},
		},
	}
}

func TestValidarArvoreAceitaArvoreValida(t *testing.T) {
	if err := validarArvore(arvoreValida()); err != nil {
		t.Fatalf("árvore válida recusada: %v", err)
	}
}

func TestValidarArvoreRecusaInvalidas(t *testing.T) {
	casos := []struct {
		nome    string
		deforma func(*Arvore)
		trecho  string
	}{
		{"id de ramo duplicado", func(a *Arvore) { a.Ramos[1].Id = "raiz" }, "duplicado"},
		{"pai inexistente", func(a *Arvore) { a.Ramos[1].Pai = "fantasma" }, "pai inexistente"},
		{"mista fora do fim", func(a *Arvore) {
			a.Ramos[1].Niveis[0].Revisoes = []string{"palavras", "mista", "desenho"}
		}, "não termina"},
		{"sem mista", func(a *Arvore) { a.Ramos[0].Niveis[0].Revisoes = []string{"palavras"} }, "deve ter exatamente 1"},
		{"duas mistas", func(a *Arvore) {
			a.Ramos[0].Niveis[0].Revisoes = []string{"mista", "palavras", "mista"}
		}, "deve ter exatamente 1"},
		{"tipo de revisão inválido", func(a *Arvore) {
			a.Ramos[0].Niveis[0].Revisoes = []string{"cantar", "mista"}
		}, "tipo de revisão inválido"},
		{"ramo sem níveis", func(a *Arvore) { a.Ramos[1].Niveis = nil }, "sem níveis"},
		{"poucas palavras", func(a *Arvore) { a.Ramos[0].Niveis[0].Palavras = []string{"你好", "谢谢"} }, "o mínimo é"},
		{"dificuldade inválida", func(a *Arvore) { a.Ramos[0].Dificuldade = "difícil" }, "dificuldade inválida"},
		{"tema fora da taxonomia", func(a *Arvore) { a.Ramos[1].Tema = "颜色" }, "fora da taxonomia"},
		{"nível sem pos", func(a *Arvore) { a.Ramos[0].Niveis[0].Pos = nil }, "sem pos"},
		{"ramo sem rotuloMapa", func(a *Arvore) { a.Ramos[0].RotuloMapa = nil }, "sem rotuloMapa"},
	}

	for _, caso := range casos {
		t.Run(caso.nome, func(t *testing.T) {
			arvore := arvoreValida()
			caso.deforma(&arvore)
			err := validarArvore(arvore)
			if err == nil {
				t.Fatalf("árvore inválida (%s) foi aceita", caso.nome)
			}
			if !strings.Contains(err.Error(), caso.trecho) {
				t.Errorf("erro = %q; esperava conter %q", err.Error(), caso.trecho)
			}
		})
	}
}
