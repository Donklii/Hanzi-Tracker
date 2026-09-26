package dicionario

import (
	"math/rand/v2"
	"sync"
)

// ----- Seção: Distratores de Componente (cartas falsas da montagem) -----
//
// A atividade de montagem (revisao) embaralha as peças reais do caractere com algumas cartas falsas.
// Um bom distrator é PARECIDO com um componente real — aqui "parecido" é ter a mesma contagem de
// traços na grafia isolada, ou seja, o mesmo tamanho/complexidade na carta. O pool reúne, uma vez,
// todos os caracteres que aparecem como folha em alguma decomposição E têm traçado próprio (a carta
// precisa desenhá-los), agrupados por contagem de traços para o sorteio ser barato.

// tentativasContagemDistrator é a ordem de busca ao redor da contagem-alvo: exata primeiro, depois as
// vizinhas, para o sorteio quase nunca ficar sem distrator quando a contagem exata está esgotada.
var tentativasContagemDistrator = []int{0, -1, 1, -2, 2}

type poolDistratores struct {
	umaVez      sync.Once
	porContagem map[int][]string // nº de traços da grafia isolada → componentes distintos com essa contagem
}

// SortearDistratoresMontagem devolve até `quantidade` caracteres-componente parecidos com os
// componentes reais (mesma contagem de traços — o tamanho da carta), sem repetir e sem coincidir com
// nenhum componente real. Cada distrator espelha um componente real distinto enquanto houver reais,
// depois recomeça a lista. Vem com menos itens (ou vazio) se o pool não tiver candidatos parecidos.
func (g *GerenciadorDicionario) SortearDistratoresMontagem(reais []ComponenteHanzi, quantidade int) []string {
	if quantidade <= 0 || len(reais) == 0 {
		return nil
	}
	g.distratores.umaVez.Do(g.montarPoolDistratores)

	excluir := make(map[string]bool, len(reais)+quantidade)
	for _, real := range reais {
		excluir[real.Caractere] = true
	}

	escolhidos := make([]string, 0, quantidade)
	for i := 0; i < quantidade; i++ {
		alvo := len(reais[i%len(reais)].Tracos)
		componente, achou := g.sortearComponentePorContagem(alvo, excluir)
		if !achou {
			continue
		}
		excluir[componente] = true
		escolhidos = append(escolhidos, componente)
	}
	return escolhidos
}

// montarPoolDistratores varre o dicionário uma vez e agrupa por contagem de traços todo componente-
// folha desenhável. Descarta o marcador de desconhecido e os de traço único (uma carta de um traço
// só seria distrator trivial). Roda sob o sync.Once do pool.
func (g *GerenciadorDicionario) montarPoolDistratores() {
	pool := make(map[int][]string)
	vistos := make(map[string]bool)

	for _, entrada := range g.Banco.caracteres {
		if entrada.Decomposicao == "" {
			continue
		}
		for _, folha := range folhasIDS(entrada.Decomposicao) {
			if folha == COMPONENTE_DESCONHECIDO || vistos[folha] {
				continue
			}
			vistos[folha] = true

			totalTracos, tem := g.Tracados.TotalTracos(folha)
			if !tem || totalTracos < TRACOS_MINIMOS_COMPONENTE {
				continue
			}
			pool[totalTracos] = append(pool[totalTracos], folha)
		}
	}
	g.distratores.porContagem = pool
}

// sortearComponentePorContagem sorteia um componente com a contagem de traços pedida (ou a vizinha
// mais próxima), pulando os já excluídos. Varre a partir de um ponto aleatório para não viciar na
// mesma folha sem alocar uma lista filtrada a cada chamada.
func (g *GerenciadorDicionario) sortearComponentePorContagem(contagem int, excluir map[string]bool) (string, bool) {
	for _, delta := range tentativasContagemDistrator {
		candidatos := g.distratores.porContagem[contagem+delta]
		if len(candidatos) == 0 {
			continue
		}
		inicio := rand.IntN(len(candidatos))
		for i := 0; i < len(candidatos); i++ {
			candidato := candidatos[(inicio+i)%len(candidatos)]
			if excluir[candidato] {
				continue
			}
			return candidato, true
		}
	}
	return "", false
}
