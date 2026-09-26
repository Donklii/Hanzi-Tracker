package revisao

import (
	"fmt"
	"math/rand/v2"
	"strings"
	"unicode/utf8"

	"wails_app/dicionario"
	"wails_app/progresso"
)

// ----- Seção: Modo de Desenho -----

// montarQuestaoDesenho monta a questão de traçado de escrita.
func (r *GerenciadorRevisao) montarQuestaoDesenho(questao QuestaoRevisao, alvo alvoRevisao, pools poolsRevisao, varianteForcada string) (QuestaoRevisao, error) {
	// Palavra multi-hanzi não pode ser desenhada de uma vez: escolhe-se o hanzi componente de MENOR
	// progresso de desenho (com traçado disponível). O acerto credita esse hanzi; a palavra
	// (PalavraFoco) só conclui o desenho quando TODOS os seus hanzis concluírem.
	if utf8.RuneCountInString(questao.Hanzi) > 1 {
		componente, ok := r.hanziDesenhoDaPalavra(questao.Hanzi)
		if !ok {
			return questao, fmt.Errorf("nenhum hanzi componente de %q tem traçado", questao.Hanzi)
		}
		questao.Hanzi = componente
		if pinyin, significados, _ := r.Dicionario.Leitura(componente); pinyin != "" {
			questao.Pinyin = pinyin
			if len(significados) > 0 {
				questao.Definicao = strings.Join(significados, "; ")
			}
		}
	}

	if varianteForcada != "" {
		questao.Variante = varianteForcada
	} else {
		questao.Variante = r.sortearVariante(VarianteDesenhoContexto, VarianteDesenhoMemoria, VarianteDesenhoComponente, VarianteDesenhoGuiado, VarianteDesenhoMontagem)
	}

	if questao.Variante == VarianteDesenhoComponente && !r.preencherComponenteDesenho(&questao) {
		questao.Variante = VarianteDesenhoMemoria
	}

	if questao.Variante == VarianteDesenhoMontagem && !r.preencherMontagemDesenho(&questao) {
		questao.Variante = VarianteDesenhoMemoria
	}

	if questao.Variante == VarianteDesenhoContexto && !r.preencherFrase(&questao) {
		questao.Variante = VarianteDesenhoMemoria
	}

	return questao, nil
}

// preencherComponenteDesenho sorteia o componente da decomposição que o usuário vai redesenhar e
// grava na questão os índices dos traços dele. Devolve false quando o caractere não se presta à
// atividade — sem decomposição própria (o caso de toda grafia tradicional, que herda a do
// simplificado), sem componente com traços suficientes, ou com índice de traço fora do banco de
// traçados. Aí a questão cai para o desenho de memória.
func (r *GerenciadorRevisao) preencherComponenteDesenho(questao *QuestaoRevisao) bool {
	componentes := r.Dicionario.Banco.ComponentesDesenhaveis(questao.Hanzi)
	if len(componentes) == 0 {
		return false
	}

	totalTracos, temTracados := r.Dicionario.Tracados.TotalTracos(questao.Hanzi)
	if !temTracados {
		return false
	}

	elegiveis := make([]dicionario.ComponenteHanzi, 0, len(componentes))
	for _, componente := range componentes {
		if !tracosDentroDoLimite(componente.Tracos, totalTracos) {
			continue
		}
		elegiveis = append(elegiveis, componente)
	}
	if len(elegiveis) == 0 {
		return false
	}

	escolhido := elegiveis[rand.IntN(len(elegiveis))]
	questao.ComponenteAlvo = escolhido.Caractere
	questao.TracosAlvo = escolhido.Tracos
	return true
}

// preencherMontagemDesenho grava na questão a partição completa dos traços do caractere pelas folhas
// da decomposição — as peças da atividade de montagem. Devolve false quando o caractere não se presta
// a ela: sem partição total (ver dicionario.ParticaoEmComponentes, incluindo toda grafia tradicional,
// que herda a decomposição do simplificado), sem traçados, ou com decomposição e traçados de
// contagens diferentes — as peças têm de recompor o caractere SEM sobra nem falta, senão a montagem
// termina com buraco. Aí a questão cai para o desenho de memória.
func (r *GerenciadorRevisao) preencherMontagemDesenho(questao *QuestaoRevisao) bool {
	componentes := r.Dicionario.Banco.ParticaoEmComponentes(questao.Hanzi)
	if len(componentes) == 0 {
		return false
	}

	totalTracos, temTracados := r.Dicionario.Tracados.TotalTracos(questao.Hanzi)
	if !temTracados {
		return false
	}

	// Os índices das peças são posições na lista de correspondências, todos distintos: se a soma bate
	// com o total do banco de traçados, a cobertura é exata e nenhum índice está fora da faixa.
	totalNasPecas := 0
	for _, componente := range componentes {
		totalNasPecas += len(componente.Tracos)
	}
	if totalNasPecas != totalTracos {
		return false
	}

	questao.ComponentesMontagem = componentes
	questao.DistratoresMontagem = r.Dicionario.SortearDistratoresMontagem(componentes, quantidadeDistratoresMontagem(len(componentes)))
	return true
}

// quantidadeDistratoresMontagem escala o número de cartas falsas com o de peças reais, até o teto:
// caracteres com mais componentes aguentam mais distratores sem virar bagunça.
func quantidadeDistratoresMontagem(numPecas int) int {
	if numPecas < DISTRATORES_MONTAGEM_MAXIMO {
		return numPecas
	}
	return DISTRATORES_MONTAGEM_MAXIMO
}

// DISTRATORES_MONTAGEM_MAXIMO limita quantas cartas falsas entram na mesa: distratores demais afogam
// as peças reais e cansam mais do que ensinam.
const DISTRATORES_MONTAGEM_MAXIMO = 3

// tracosDentroDoLimite confere que todo índice de traço do componente existe no banco de traçados:
// decomposição e traçados vêm de arquivos diferentes e um índice fora da faixa quebraria o canvas.
func tracosDentroDoLimite(tracos []int, totalTracos int) bool {
	for _, indice := range tracos {
		if indice < 0 || indice >= totalTracos {
			return false
		}
	}
	return true
}

// hanziDesenhoDaPalavra escolhe, entre os hanzis componentes que possuem traçado, o de MENOR streak
// de desenho (empate resolvido pela ordem na palavra). Devolve false se nenhum componente tem traçado.
func (r *GerenciadorRevisao) hanziDesenhoDaPalavra(palavra string) (string, bool) {
	melhor := ""
	menorStreak := 0
	for _, hanzi := range hanzisComponentes(palavra) {
		if !r.Dicionario.Tracados.Tem(hanzi) {
			continue
		}
		streak := 0
		if st, err := progresso.ObterEstatisticasPalavra(hanzi); err == nil {
			streak = st["desenho"]
		}
		if melhor == "" || streak < menorStreak {
			melhor = hanzi
			menorStreak = streak
		}
	}
	return melhor, melhor != ""
}
