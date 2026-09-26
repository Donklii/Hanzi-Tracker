// Package busca centraliza o sistema de busca por questões, distratores e atividades ideais da
// revisão. Nenhuma atividade (significado, fonética, desenho, contexto, ordenação, pronúncia) tem
// função exclusiva para encontrar seus próprios candidatos: todas declaram critérios — fontes
// priorizadas, predicado de aceitação e meta — e chamam a interface deste pacote para obtê-los já
// filtrados e sorteados. Isso mantém a lógica de seleção robusta, testável isoladamente e num só
// lugar para manutenção, em vez de espalhada e duplicada por módulo de atividade.
//
// Módulos deste pacote:
//   - busca.go: o motor genérico (cascata de fontes + critério de aceitação) e os predicados de
//     distinção compartilhados entre atividades (pinyin, significado, hanzi).
//   - buscador.go: o Buscador, que guarda o estado da sessão (status/visualizações do vocabulário,
//     histórico de deduplicação) e expõe a API pública consumida pelo pacote revisao.
//   - atividades.go: quais modos/variantes existem, quais candidatos servem a cada modo, se um
//     alvo tem insumos para uma atividade e qual atividade é a mais pendente para um alvo em foco.
//   - frases.go: a frase ideal de uma questão (critérios por variante, funil de filtros).
//   - opcoes.go: alternativas de múltipla escolha, quebra-cabeça de significado e baralho de
//     pronúncia.
//   - pecas.go: peças distratoras de ordenação (hanzis aparentados, palavras, termos em inglês).
package busca

import (
	"iter"
	"math/rand/v2"
	"slices"
	"strings"

	"wails_app/dicionario"
)

// ----- Seção: Tipos de Resultado -----

// OpcaoRevisao é uma alternativa de múltipla escolha (ou peça de quebra-cabeça) de uma questão.
type OpcaoRevisao struct {
	Hanzi     string `json:"hanzi"`
	Pinyin    string `json:"pinyin"`
	Definicao string `json:"definicao"`
	Correta   bool   `json:"correta"`
}

// ElementoOrdenacao é uma peça (correta ou distratora) de uma questão de ordenação.
type ElementoOrdenacao struct {
	Texto     string `json:"texto"`
	Pinyin    string `json:"pinyin"`
	Definicao string `json:"definicao"`
	Correta   bool   `json:"correta"`
}

// ----- Seção: Motor Central de Busca -----
//
// O modelo é uma cascata de fontes: cada fonte entrega candidatas na ordem de preferência do
// produtor (ranqueada ou embaralhada), pode ser probabilística (Chance) e ter teto de aproveitamento
// (Maximo). O motor percorre as fontes na ordem declarada, aplica o critério de aceitação candidata
// a candidata e para assim que a meta é atingida — o resto da fonte atual e as fontes seguintes nem
// chegam a ser percorridos, então fallbacks caros são de fato preguiçosos.

// FonteBusca é uma origem priorizada de candidatas de uma busca.
type FonteBusca[C any] struct {
	// Sequencia entrega as candidatas na ordem de preferência do produtor. iter.Seq é preguiçoso:
	// fontes caras (consulta ao banco, varredura de todas as frases) só pagam custo se consumidas.
	Sequencia iter.Seq[C]

	// Chance é a probabilidade de a fonte ser tentada (0 ou 1 = sempre). Fontes probabilísticas
	// variam a composição entre questões (ex.: distrator "aprendido" em ~50% delas).
	Chance float64

	// Maximo limita quantos itens podem ser aproveitados desta fonte (0 = sem teto).
	Maximo int
}

// CriterioAceitacao decide se uma candidata entra na seleção, à luz das já escolhidas (unicidade
// de pinyin/glosa, ausência na frase, etc.). nil aceita qualquer candidata.
type CriterioAceitacao[R any, C any] func(escolhidas []R, candidata C) bool

// BuscarDeFontes percorre as fontes em cascata e devolve a seleção (iniciais + aceitas) com até
// `meta` itens. `converter` transforma a candidata aceita no tipo da seleção (ex.: entrada de
// dicionário → opção de múltipla escolha).
func BuscarDeFontes[C any, R any](meta int, iniciais []R, fontes []FonteBusca[C], aceitar CriterioAceitacao[R, C], converter func(C) R) []R {
	escolhidas := append(make([]R, 0, meta), iniciais...)

	for _, fonte := range fontes {
		if len(escolhidas) >= meta {
			break
		}
		if fonte.Sequencia == nil {
			continue
		}
		if fonte.Chance > 0 && fonte.Chance < 1 && rand.Float64() > fonte.Chance {
			continue
		}

		aproveitadas := 0
		for candidata := range fonte.Sequencia {
			if len(escolhidas) >= meta {
				break
			}
			if fonte.Maximo > 0 && aproveitadas >= fonte.Maximo {
				break
			}
			if aceitar != nil && !aceitar(escolhidas, candidata) {
				continue
			}
			escolhidas = append(escolhidas, converter(candidata))
			aproveitadas++
		}
	}
	return escolhidas
}

// BuscarDeFontesSimples é BuscarDeFontes quando candidatas e seleção têm o mesmo tipo.
func BuscarDeFontesSimples[T any](meta int, iniciais []T, fontes []FonteBusca[T], aceitar CriterioAceitacao[T, T]) []T {
	return BuscarDeFontes(meta, iniciais, fontes, aceitar, func(candidata T) T { return candidata })
}

// ----- Seção: Sequências de Candidatas -----

// SequenciaOrdenada percorre a lista na ordem em que está (fontes já ranqueadas pelo produtor).
func SequenciaOrdenada[T any](lista []T) iter.Seq[T] {
	return slices.Values(lista)
}

// SequenciaEmbaralhada percorre a lista numa permutação aleatória nova, sorteada apenas quando a
// fonte é de fato consumida.
func SequenciaEmbaralhada[T any](lista []T) iter.Seq[T] {
	return func(entregar func(T) bool) {
		for _, i := range rand.Perm(len(lista)) {
			if !entregar(lista[i]) {
				return
			}
		}
	}
}

// SequenciaPreguicosa adia a produção da lista para o momento em que a fonte é consumida — fontes
// de fallback caras (consultas ao banco, varreduras) não pagam custo quando a busca satisfaz antes.
func SequenciaPreguicosa[T any](produzir func() []T) iter.Seq[T] {
	return func(entregar func(T) bool) {
		for _, item := range produzir() {
			if !entregar(item) {
				return
			}
		}
	}
}

// ----- Seção: Critérios de Distinção (predicados de aceitação compartilhados) -----

// CriterioDistrator é o critério de aceitação das alternativas de múltipla escolha.
type CriterioDistrator = CriterioAceitacao[OpcaoRevisao, dicionario.DecomposicaoHanzi]

// DistintosPorHanzi garante que o distrator não coincida com o caractere de nenhuma alternativa.
func DistintosPorHanzi(escolhidas []OpcaoRevisao, candidata dicionario.DecomposicaoHanzi) bool {
	for _, o := range escolhidas {
		if o.Hanzi == candidata.Caractere {
			return false
		}
	}

	return true
}

// DistintosPorSignificado garante que não haja nenhuma glosa principal repetida nas alternativas.
func DistintosPorSignificado(escolhidas []OpcaoRevisao, candidata dicionario.DecomposicaoHanzi) bool {
	glosaCandidata := GlosaPrincipal(candidata.Definicao)

	for _, o := range escolhidas {
		if GlosaPrincipal(o.Definicao) == glosaCandidata {
			return false
		}
	}

	return true
}

// DistintosPorPinyin garante que nenhuma sílaba (com tom) seja repetida nas alternativas.
func DistintosPorPinyin(escolhidas []OpcaoRevisao, candidata dicionario.DecomposicaoHanzi) bool {
	if len(candidata.Pinyin) == 0 || candidata.Pinyin[0] == "" {
		return false
	}

	pinyinCandidata := NormalizarPinyinParaComparar(candidata.Pinyin[0])

	for _, o := range escolhidas {
		if NormalizarPinyinParaComparar(o.Pinyin) == pinyinCandidata {
			return false
		}
	}

	return true
}

// GlosaPrincipal extrai a primeira acepção da definição ("you (informal)" de "you (informal); thou").
func GlosaPrincipal(definicao string) string {
	glosa, _, _ := strings.Cut(definicao, ";")

	return strings.ToLower(strings.TrimSpace(glosa))
}

// NormalizarPinyinParaComparar normaliza o pinyin para comparação (minúsculo, sem espaços e sem apóstrofos).
func NormalizarPinyinParaComparar(p string) string {
	p = strings.ToLower(p)
	p = strings.ReplaceAll(p, " ", "")
	p = strings.ReplaceAll(p, "'", "")
	return p
}
