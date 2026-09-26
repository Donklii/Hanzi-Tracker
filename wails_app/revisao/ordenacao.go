package revisao

import (
	"fmt"
	"maps"
	"math/rand/v2"
	"slices"
	"strings"
	"unicode/utf8"

	"wails_app/dicionario"
)

// ----- Seção: Modo de Ordenação -----

// chancePecaPalavraInteira é a probabilidade de uma palavra multi-hanzi da frase virar UMA peça
// inteira em vez de uma peça por hanzi.
const chancePecaPalavraInteira = 0.5

// preencherQuestaoOrdenacao preenche a questão de ordenar blocos de palavras em inglês ou hanzis chineses.
func (r *GerenciadorRevisao) preencherQuestaoOrdenacao(questao QuestaoRevisao, alvo alvoRevisao, pools poolsRevisao) (QuestaoRevisao, error) {
	if !r.preencherFrase(&questao) {
		return questao, fmt.Errorf("nenhuma frase contém %q", questao.Hanzi)
	}

	if questao.Variante == VarianteOrdenacao {
		if err := r.gerarPilhaOrdenacao(&questao, pools); err != nil {
			return questao, err
		}
	} else {
		if err := r.gerarPilhaOrdenacaoTraducao(&questao, pools.Todos); err != nil {
			return questao, err
		}
	}

	return questao, nil
}

// gerarPilhaOrdenacao cria a pilha de peças para a questão de ordenação/fonética-frase: as peças
// corretas vêm da própria frase (palavra inteira ou quebrada em hanzis, ao acaso) e as distratoras
// são buscadas no módulo central — por tamanho de peça, com fallback nas já vistas frequentes e
// depois no banco geral do modo.
func (r *GerenciadorRevisao) gerarPilhaOrdenacao(questao *QuestaoRevisao, pools poolsRevisao) error {
	palavras := r.DecomporTextoRevisao(questao.FraseOriginal)

	var pecasCorretas []ElementoOrdenacao
	var pecasEsperadas []string

	for _, p := range palavras {
		if !p.EhChines {
			continue
		}

		runesPalavra := []rune(p.Texto)
		if len(runesPalavra) <= 1 || rand.Float64() < chancePecaPalavraInteira {
			def := ""
			if len(p.Significados) > 0 {
				def = strings.Join(p.Significados, "; ")
			}
			pecasCorretas = append(pecasCorretas, ElementoOrdenacao{
				Texto:     p.Texto,
				Pinyin:    p.Pinyin,
				Definicao: def,
				Correta:   true,
			})
			pecasEsperadas = append(pecasEsperadas, p.Texto)
		} else {
			for _, runePalavra := range runesPalavra {
				ch := string(runePalavra)
				pin, sigs, _ := r.Dicionario.Leitura(ch)
				def := ""
				if len(sigs) > 0 {
					def = strings.Join(sigs, "; ")
				}
				pecasCorretas = append(pecasCorretas, ElementoOrdenacao{
					Texto:     ch,
					Pinyin:    pin,
					Definicao: def,
					Correta:   true,
				})
				pecasEsperadas = append(pecasEsperadas, ch)
			}
		}
	}

	if len(pecasCorretas) == 0 {
		return fmt.Errorf("frase sem hanzis válidos")
	}

	questao.PecasEsperadas = pecasEsperadas

	// ----- Busca das peças distratoras -----
	qtdDistratores := (len(pecasCorretas) + 1) / 2

	pinyinsCorretos := make(map[string]bool)
	for _, p := range pecasCorretas {
		pinyinsCorretos[normalizarPinyinParaComparar(p.Pinyin)] = true
	}
	hanzisNaFrase := hanzisDaFrase(questao.FraseOriginal)
	aceitar := aceitarPecaDistratora(hanzisNaFrase, pinyinsCorretos)

	// Distribui a meta entre os tamanhos de peça presentes (mínimo 1 por tamanho), cada tamanho com
	// sua fonte ideal: hanzis aparentados por decomposição para peças de 1 hanzi, palavras do mesmo
	// tamanho para as demais.
	tamanhosPecas := make(map[int]int)
	for _, p := range pecasCorretas {
		tamanhosPecas[utf8.RuneCountInString(p.Texto)]++
	}
	metaPorTamanho := qtdDistratores / len(tamanhosPecas)
	if metaPorTamanho < 1 {
		metaPorTamanho = 1
	}

	var pecasDistratoras []ElementoOrdenacao
	for _, tamanho := range slices.Sorted(maps.Keys(tamanhosPecas)) {
		if len(pecasDistratoras) >= qtdDistratores {
			break
		}
		meta := min(metaPorTamanho, qtdDistratores-len(pecasDistratoras))

		fonte := fonteBusca[ElementoOrdenacao]{Sequencia: r.sequenciaPecasPalavrasDoTamanho(tamanho)}
		if tamanho == 1 {
			fonte = fonteBusca[ElementoOrdenacao]{Sequencia: sequenciaPreguicosa(func() []ElementoOrdenacao {
				return r.pecasAparentadasDaFrase(hanzisNaFrase)
			})}
		}

		pecasDistratoras = buscarDeFontesSimples(len(pecasDistratoras)+meta, pecasDistratoras, []fonteBusca[ElementoOrdenacao]{fonte}, aceitar)
	}

	// Completa a meta com as já vistas frequentes e, por fim, o banco geral do modo quando as fontes
	// por tamanho não bastaram.
	fontesFinais := []fonteBusca[ElementoOrdenacao]{
		{Sequencia: sequenciaPecasDeEntradas(pools.Vistas)},
		{Sequencia: sequenciaPecasDeEntradas(pools.Todos)},
	}
	pecasDistratoras = buscarDeFontesSimples(qtdDistratores, pecasDistratoras, fontesFinais, aceitar)

	aplicarPilhaOrdenacao(questao, pecasCorretas, pecasDistratoras)
	return nil
}

// gerarPilhaOrdenacaoTraducao cria a pilha de peças para a variante de tradução da frase
// (ordenacao_traducao): as peças corretas são as palavras da tradução e as distratoras são termos
// em inglês buscados no módulo central (definições de hanzis-semente aparentados, com fallback em
// palavras de outras frases).
func (r *GerenciadorRevisao) gerarPilhaOrdenacaoTraducao(questao *QuestaoRevisao, candidatos []dicionario.DecomposicaoHanzi) error {
	palavrasTraducao := quebrarFraseEmPalavras(questao.FraseTraducao)
	if len(palavrasTraducao) == 0 {
		return fmt.Errorf("tradução da frase vazia ou sem palavras válidas")
	}

	var pecasCorretas []ElementoOrdenacao
	for _, pal := range palavrasTraducao {
		pecasCorretas = append(pecasCorretas, ElementoOrdenacao{
			Texto:   pal,
			Correta: true,
		})
	}
	questao.PecasEsperadas = palavrasTraducao

	// ----- Busca das peças distratoras -----
	qtdDistratores := (len(pecasCorretas) + 1) / 2
	hanzisNaFrase := hanzisDaFrase(questao.FraseOriginal)

	palavrasOriginais := make(map[string]bool)
	for _, p := range palavrasTraducao {
		palavrasOriginais[strings.ToLower(p)] = true
	}

	sementes := r.sementesDistratorasDaTraducao(hanzisNaFrase, candidatos, qtdDistratores)
	fontes := []fonteBusca[string]{
		{Sequencia: r.sequenciaPalavrasInglesasDeSementes(sementes)},
		{Sequencia: r.sequenciaPalavrasInglesasDeFrases()},
	}

	pecasDistratoras := buscarDeFontes(
		qtdDistratores,
		nil,
		fontes,
		aceitarPalavraInglesaDistratora(palavrasOriginais),
		func(palavra string) ElementoOrdenacao { return ElementoOrdenacao{Texto: palavra} },
	)

	aplicarPilhaOrdenacao(questao, pecasCorretas, pecasDistratoras)
	return nil
}

// aplicarPilhaOrdenacao publica na questão a pilha final (corretas + distratoras, embaralhadas) e
// o espelho legado em PilhaOrdenacao.
func aplicarPilhaOrdenacao(questao *QuestaoRevisao, pecasCorretas, pecasDistratoras []ElementoOrdenacao) {
	todos := append(pecasCorretas, pecasDistratoras...)
	rand.Shuffle(len(todos), func(i, j int) {
		todos[i], todos[j] = todos[j], todos[i]
	})
	questao.ElementosOrdenacao = todos

	var pilhaLegada []OpcaoRevisao
	for _, elem := range todos {
		pilhaLegada = append(pilhaLegada, OpcaoRevisao{
			Hanzi:     elem.Texto,
			Pinyin:    elem.Pinyin,
			Definicao: elem.Definicao,
			Correta:   elem.Correta,
		})
	}
	questao.PilhaOrdenacao = pilhaLegada
}
