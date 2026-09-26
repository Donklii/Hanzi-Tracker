package revisao

import (
	"math"
	"testing"

	"wails_app/dicionario"
	"wails_app/revisao/jornada"
)

// ----- Guarda-corpo do conteúdo da Jornada -----
//
// Este teste vale por TODA a árvore (arvore.json) contra os dados REAIS do app: dicionário e acervo
// de frases. É o portão do plano de conteúdo — ramo novo que não passe aqui não vai render questão
// no app. Por isso as mensagens de falha apontam ramo, nível e palavra exatos.

// distanciaMinimaEntreNos é o espaço mínimo entre dois nós QUAISQUER do mapa, em pixels: abaixo
// disso os círculos (76px) e seus rótulos se sobrepõem na tela.
const distanciaMinimaEntreNos = 110.0

func TestConteudoJornadaValido(t *testing.T) {
	arvore, err := jornada.ObterArvore()
	if err != nil {
		t.Fatalf("ObterArvore: %v", err)
	}

	dic, err := dicionario.NovoGerenciadorDicionario(dicionario.IdiomaPadrao)
	if err != nil {
		t.Fatalf("falha ao carregar dicionários: %v", err)
	}
	frases := dicionario.NovoGerenciadorFrases(dicionario.IdiomaPadrao)

	type noDoMapa struct {
		id string
		x  float64
		y  float64
	}
	var nos []noDoMapa

	for _, ramo := range arvore.Ramos {
		if len(ramo.RotuloMapa) != 2 {
			t.Errorf("ramo %q (%s): sem rotuloMapa [x, y]", ramo.Id, ramo.Nome)
		}

		for _, nivel := range ramo.Niveis {
			validarPalavrasDoNivel(t, dic, frases, ramo, nivel)

			if len(nivel.Pos) != 2 {
				t.Errorf("nível %q (%s): sem pos [x, y]", nivel.Id, nivel.Titulo)
				continue
			}
			x, y := nivel.Pos[0], nivel.Pos[1]
			if x < 0 || x > arvore.Mapa.Largura || y < 0 || y > arvore.Mapa.Altura {
				t.Errorf("nível %q (%s): posição (%d, %d) fora do mapa %dx%d",
					nivel.Id, nivel.Titulo, x, y, arvore.Mapa.Largura, arvore.Mapa.Altura)
			}
			nos = append(nos, noDoMapa{id: nivel.Id, x: float64(x), y: float64(y)})
		}
	}

	for i := 0; i < len(nos); i++ {
		for j := i + 1; j < len(nos); j++ {
			distancia := math.Hypot(nos[i].x-nos[j].x, nos[i].y-nos[j].y)
			if distancia < distanciaMinimaEntreNos {
				t.Errorf("nós %q e %q estão a %.1fpx um do outro (mínimo %.0fpx): eles se sobrepõem no mapa",
					nos[i].id, nos[j].id, distancia, distanciaMinimaEntreNos)
			}
		}
	}
}

// validarPalavrasDoNivel confere que cada palavra do nível tem os insumos que as revisões dele pedem.
func validarPalavrasDoNivel(t *testing.T, dic *dicionario.GerenciadorDicionario, frases *dicionario.GerenciadorFrases, ramo jornada.Ramo, nivel jornada.Nivel) {
	t.Helper()

	precisaTracado := contemRevisao(nivel.Revisoes, jornada.RevisaoDesenho)
	precisaFrase := contemRevisao(nivel.Revisoes, jornada.RevisaoFrases)

	for _, palavra := range nivel.Palavras {
		pinyin, significados, _ := dic.Leitura(palavra)
		if pinyin == "" || len(significados) == 0 {
			t.Errorf("ramo %q, nível %q (%s), palavra %q: sem leitura no dicionário (pinyin=%q, %d significados)",
				ramo.Id, nivel.Id, nivel.Titulo, palavra, pinyin, len(significados))
		}

		// Mesma checagem de temTracadoComponente (NovoGerenciadorRevisao): a palavra é desenhável se
		// ALGUM hanzi componente tem traçado.
		if precisaTracado && !temTracadoEmAlgumComponente(dic, palavra) {
			t.Errorf("ramo %q, nível %q (%s), palavra %q: nenhum hanzi componente tem traçado, mas o nível tem revisão de desenho",
				ramo.Id, nivel.Id, nivel.Titulo, palavra)
		}

		if precisaFrase && len(frases.FrasesComPalavra(palavra)) == 0 {
			t.Errorf("ramo %q, nível %q (%s), palavra %q: nenhuma frase no acervo, mas o nível tem revisão de frases",
				ramo.Id, nivel.Id, nivel.Titulo, palavra)
		}
	}
}

func temTracadoEmAlgumComponente(dic *dicionario.GerenciadorDicionario, palavra string) bool {
	for _, hanzi := range hanzisComponentes(palavra) {
		if dic.Tracados.Tem(hanzi) {
			return true
		}
	}
	return false
}

// contemRevisao informa se o tipo aparece nas revisões do nível — direto ou dentro da `mista`, que
// pratica os tipos anteriores do próprio nível.
func contemRevisao(revisoes []string, tipo string) bool {
	for _, r := range revisoes {
		if r == tipo {
			return true
		}
	}
	return false
}
