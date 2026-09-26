package busca

import (
	"iter"
	"math/rand/v2"
	"strings"
	"unicode"
	"unicode/utf8"

	"wails_app/dicionario"
)

// ----- Seção: Busca de Peças de Ordenação -----
//
// Fornece as PEÇAS DISTRATORAS das atividades de ordenação (ordenar frase, ordenar tradução e
// fonética de frase): produtores de candidatas (hanzis aparentados por decomposição, palavras do
// mesmo tamanho, termos em inglês extraídos de definições) e os critérios de aceitação
// compartilhados. As atividades declaram as fontes e a meta; o motor central seleciona.

// ----- Seção: Critérios de Aceitação de Peças -----

// AceitarPecaDistratora monta o critério das peças distratoras chinesas: não repetir peça já
// escolhida, não conter nenhum hanzi da frase (estaria "correta" aos olhos do usuário) e não
// colidir o pinyin com o de uma peça correta.
func AceitarPecaDistratora(hanzisNaFrase map[string]bool, pinyinsCorretos map[string]bool) CriterioAceitacao[ElementoOrdenacao, ElementoOrdenacao] {
	return func(escolhidas []ElementoOrdenacao, candidata ElementoOrdenacao) bool {
		for _, e := range escolhidas {
			if e.Texto == candidata.Texto {
				return false
			}
		}
		for _, runa := range candidata.Texto {
			if unicode.Is(unicode.Han, runa) && hanzisNaFrase[string(runa)] {
				return false
			}
		}
		return !pinyinsCorretos[NormalizarPinyinParaComparar(candidata.Pinyin)]
	}
}

// AceitarPalavraInglesaDistratora monta o critério dos termos em inglês: não repetir (sem
// diferenciar maiúsculas) nenhuma palavra da tradução correta nem peça já escolhida.
func AceitarPalavraInglesaDistratora(palavrasOriginais map[string]bool) CriterioAceitacao[ElementoOrdenacao, string] {
	return func(escolhidas []ElementoOrdenacao, candidata string) bool {
		minuscula := strings.ToLower(candidata)
		if palavrasOriginais[minuscula] {
			return false
		}
		for _, e := range escolhidas {
			if strings.ToLower(e.Texto) == minuscula {
				return false
			}
		}
		return true
	}
}

// ----- Seção: Produtores de Peças Chinesas -----

// hanzisAparentadosDaFrase devolve (dedup, sem os da própria frase) os hanzis do dicionário que
// compartilham componentes de decomposição (radical, fonético ou semântico) com os hanzis da frase
// — a matéria-prima ideal de distratores: parecem plausíveis sem estarem na resposta.
func (b *Buscador) hanzisAparentadosDaFrase(hanzisNaFrase map[string]bool) []string {
	componentes := make(map[string]bool)
	for ch := range hanzisNaFrase {
		entrada := b.dicionario.DecomporHanzi(ch)
		if entrada == nil {
			continue
		}
		if entrada.Radical != "" {
			componentes[entrada.Radical] = true
		}
		if entrada.Etimologia.Fonetica != "" {
			componentes[entrada.Etimologia.Fonetica] = true
		}
		if entrada.Etimologia.Semantica != "" {
			componentes[entrada.Etimologia.Semantica] = true
		}
	}

	var aparentados []string
	vistos := make(map[string]bool)
	for comp := range componentes {
		if comp == "" {
			continue
		}
		for _, compHanzi := range b.dicionario.CompostosPor(comp) {
			if hanzisNaFrase[compHanzi] || vistos[compHanzi] {
				continue
			}
			vistos[compHanzi] = true
			aparentados = append(aparentados, compHanzi)
		}
	}
	return aparentados
}

// PecasAparentadasDaFrase transforma os hanzis aparentados em peças candidatas (embaralhadas),
// descartando os sem leitura ou sem definição no banco de decomposição.
func (b *Buscador) PecasAparentadasDaFrase(hanzisNaFrase map[string]bool) []ElementoOrdenacao {
	var pecas []ElementoOrdenacao
	for _, ch := range b.hanzisAparentadosDaFrase(hanzisNaFrase) {
		entrada := b.dicionario.DecomporHanzi(ch)
		if entrada == nil || entrada.Definicao == "" || len(entrada.Pinyin) == 0 || entrada.Pinyin[0] == "" {
			continue
		}
		pecas = append(pecas, ElementoOrdenacao{Texto: ch, Pinyin: entrada.Pinyin[0], Definicao: entrada.Definicao})
	}
	rand.Shuffle(len(pecas), func(i, j int) { pecas[i], pecas[j] = pecas[j], pecas[i] })
	return pecas
}

// SequenciaPecasPalavrasDoTamanho entrega, em ordem aleatória e sob demanda, palavras do dicionário
// com `tamanho` hanzis já como peças — a consulta ao banco só acontece para as efetivamente
// percorridas pela busca.
func (b *Buscador) SequenciaPecasPalavrasDoTamanho(tamanho int) iter.Seq[ElementoOrdenacao] {
	return func(entregar func(ElementoOrdenacao) bool) {
		todas := b.dicionario.Banco.TodasPalavras()
		for _, idx := range rand.Perm(len(todas)) {
			palavra := todas[idx]
			if utf8.RuneCountInString(palavra) != tamanho {
				continue
			}
			entradas := b.dicionario.Banco.Buscar(palavra)
			if len(entradas) == 0 || entradas[0].Pinyin == "" {
				continue
			}
			peca := ElementoOrdenacao{
				Texto:     palavra,
				Pinyin:    entradas[0].Pinyin,
				Definicao: strings.Join(entradas[0].Significados, "; "),
			}
			if !entregar(peca) {
				return
			}
		}
	}
}

// SequenciaPecasDeEntradas entrega, em ordem aleatória e sob demanda, entradas do banco da sessão
// como peças (fallback final quando as fontes ideais não completam a meta).
func SequenciaPecasDeEntradas(entradas []dicionario.DecomposicaoHanzi) iter.Seq[ElementoOrdenacao] {
	return func(entregar func(ElementoOrdenacao) bool) {
		for _, i := range rand.Perm(len(entradas)) {
			entrada := entradas[i]
			if len(entrada.Pinyin) == 0 || entrada.Pinyin[0] == "" {
				continue
			}
			peca := ElementoOrdenacao{Texto: entrada.Caractere, Pinyin: entrada.Pinyin[0], Definicao: entrada.Definicao}
			if !entregar(peca) {
				return
			}
		}
	}
}

// ----- Seção: Produtores de Termos em Inglês (ordenação de tradução) -----

// SementesDistratorasDaTraducao devolve os hanzis-semente, na ordem de preferência, de onde os
// termos distratores em inglês são extraídos. A cascata para de agregar camadas assim que a
// quantidade desejada é atingida:
//  1. aparentados da frase já aprendidos;
//  2. aparentados da frase em estudo;
//  3. aparentados da frase quaisquer;
//  4. outros hanzis aprendidos (embaralhados);
//  5. outros hanzis em estudo (embaralhados);
//  6. candidatos aleatórios do modo.
func (b *Buscador) SementesDistratorasDaTraducao(hanzisNaFrase map[string]bool, candidatos []dicionario.DecomposicaoHanzi, desejadas int) []string {
	aparentados := b.hanzisAparentadosDaFrase(hanzisNaFrase)

	var sementes []string
	vistos := make(map[string]bool)
	adicionar := func(hanzi string) {
		if vistos[hanzi] {
			return
		}
		vistos[hanzi] = true
		sementes = append(sementes, hanzi)
	}

	for _, ch := range aparentados {
		if b.mapaStatus[ch] == dicionario.StatusAprendido {
			adicionar(ch)
		}
	}

	if len(sementes) < desejadas {
		for _, ch := range aparentados {
			if b.mapaStatus[ch] == dicionario.StatusEstudo {
				adicionar(ch)
			}
		}
	}

	if len(sementes) < desejadas {
		for _, ch := range aparentados {
			adicionar(ch)
		}
	}

	if len(sementes) < desejadas {
		var aprendidosExtras []string
		for ch, status := range b.mapaStatus {
			if status == dicionario.StatusAprendido && !hanzisNaFrase[ch] && !vistos[ch] {
				aprendidosExtras = append(aprendidosExtras, ch)
			}
		}
		rand.Shuffle(len(aprendidosExtras), func(i, j int) { aprendidosExtras[i], aprendidosExtras[j] = aprendidosExtras[j], aprendidosExtras[i] })
		for _, ch := range aprendidosExtras {
			adicionar(ch)
		}
	}

	if len(sementes) < desejadas {
		var estudoExtras []string
		for ch, status := range b.mapaStatus {
			if status == dicionario.StatusEstudo && !hanzisNaFrase[ch] && !vistos[ch] {
				estudoExtras = append(estudoExtras, ch)
			}
		}
		rand.Shuffle(len(estudoExtras), func(i, j int) { estudoExtras[i], estudoExtras[j] = estudoExtras[j], estudoExtras[i] })
		for _, ch := range estudoExtras {
			adicionar(ch)
		}
	}

	if len(sementes) < desejadas {
		for _, i := range rand.Perm(len(candidatos)) {
			if len(sementes) >= desejadas {
				break
			}
			entrada := candidatos[i]
			if !hanzisNaFrase[entrada.Caractere] {
				adicionar(entrada.Caractere)
			}
		}
	}

	return sementes
}

// SequenciaPalavrasInglesasDeSementes extrai, sob demanda, as palavras em inglês das definições
// dos hanzis-semente (na ordem das sementes): quebra as acepções em ",", ";", remove o prefixo
// verbal "to " e a pontuação de cada palavra.
func (b *Buscador) SequenciaPalavrasInglesasDeSementes(sementes []string) iter.Seq[string] {
	return func(entregar func(string) bool) {
		for _, ch := range sementes {
			def := ""
			if entrada := b.dicionario.DecomporHanzi(ch); entrada != nil && entrada.Definicao != "" {
				def = entrada.Definicao
			} else if _, sigs, _ := b.dicionario.Leitura(ch); len(sigs) > 0 {
				def = strings.Join(sigs, "; ")
			}
			if def == "" {
				continue
			}

			termos := strings.FieldsFunc(def, func(runa rune) bool { return runa == ',' || runa == ';' })
			for _, termo := range termos {
				termo = strings.TrimSpace(termo)
				if strings.HasPrefix(strings.ToLower(termo), "to ") {
					termo = termo[3:]
				}

				for _, palavra := range strings.Fields(termo) {
					palavra = strings.TrimFunc(palavra, unicode.IsPunct)
					if palavra == "" {
						continue
					}
					if !entregar(palavra) {
						return
					}
				}
			}
		}
	}
}

// SequenciaPalavrasInglesasDeFrases entrega, sob demanda, palavras das traduções de outras frases
// do acervo (embaralhadas) — o fallback final dos termos distratores em inglês.
func (b *Buscador) SequenciaPalavrasInglesasDeFrases() iter.Seq[string] {
	return func(entregar func(string) bool) {
		todasFrases := b.frases.ObterTodasFrases()
		for _, i := range rand.Perm(len(todasFrases)) {
			for _, palavra := range QuebrarFraseEmPalavras(todasFrases[i].Ingles) {
				if !entregar(palavra) {
					return
				}
			}
		}
	}
}

// ----- Seção: Utilitários de Peça -----

// HanzisDaFrase coleta o conjunto de hanzis presentes na frase.
func HanzisDaFrase(frase string) map[string]bool {
	hanzis := make(map[string]bool)
	for _, runa := range frase {
		if unicode.Is(unicode.Han, runa) {
			hanzis[string(runa)] = true
		}
	}
	return hanzis
}

// QuebrarFraseEmPalavras limpa a pontuação e quebra a frase em palavras.
func QuebrarFraseEmPalavras(frase string) []string {
	trocador := strings.NewReplacer(".", "", ",", "", "!", "", "?", "", ";", "", ":", "", "\"", "", "(", "", ")", "")
	limpa := trocador.Replace(frase)

	return strings.Fields(limpa)
}
