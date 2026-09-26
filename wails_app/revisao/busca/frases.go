package busca

import (
	"math/rand/v2"
	"sort"
	"unicode"

	"wails_app/dicionario"
)

// ----- Seção: Busca de Frases Ideais -----
//
// Encontra A FRASE de uma questão. A atividade declara os critérios (CriteriosFraseIdeal) —
// normalmente traduzindo sua variante com CriteriosFraseDaVariante — e recebe a frase já filtrada e
// sorteada. O funil de filtros (obrigatórios e preferenciais) vive somente aqui: nenhuma atividade
// filtra frase por conta própria.

// TamanhoFraseConfortavel é o teto preferencial de caracteres da frase (cabe bem na tela).
const TamanhoFraseConfortavel = 20

// MaxPalavrasFraseFonetica é o limite preferencial de palavras da frase para atividades de fonética.
const MaxPalavrasFraseFonetica = 10

// Exigências de cobertura de vocabulário conhecido da frase.
const (
	// exigenciaCoberturaPreferencial tenta cobertura média, depois baixa, senão aceita qualquer frase.
	exigenciaCoberturaPreferencial = "preferencial"
	// exigenciaCoberturaAlta descarta frases abaixo da cobertura alta (falha se nenhuma sobrar).
	exigenciaCoberturaAlta = "alta_obrigatoria"
	// exigenciaFraseDeOrdenacao aplica a regra de frase válida para ordenação.
	exigenciaFraseDeOrdenacao = "valida_ordenacao"
	// exigenciaRequisitosMinimosFrase exige 80% das palavras no estudando ou aprendidas (desconsiderado na Jornada)
	// e 95% das palavras já vistas.
	exigenciaRequisitosMinimosFrase = "requisitos_minimos"
)

// CriteriosFraseIdeal descreve o que uma atividade exige da frase da sua questão.
type CriteriosFraseIdeal struct {
	// Palavra que a frase precisa conter inteira (o alvo da lacuna/destaque da questão).
	Palavra string

	// ExigirTodosConhecidos descarta frases com qualquer hanzi fora do vocabulário do usuário.
	ExigirTodosConhecidos bool

	// ExigenciaCobertura é uma das constantes exigencia* deste arquivo.
	ExigenciaCobertura string

	// MaxPalavras limita o número de palavras da frase (0 = sem limite de palavras).
	MaxPalavras int

	// SelecaoPorContagemDeEstudo pondera o sorteio final pela contagem absoluta de itens em estudo
	// na frase; o padrão (false) pondera pela proporção de vocabulário conhecido.
	SelecaoPorContagemDeEstudo bool

	// Assinatura identifica a frase no histórico da sessão (deduplicação entre questões).
	Assinatura func(fraseChines string) string
}

// CriteriosFraseDaVariante traduz a variante de uma questão nos critérios de frase que ela exige.
// Adicionar uma variante nova com frase = adicionar um caso aqui, e nada mais.
func CriteriosFraseDaVariante(variante, palavra string) CriteriosFraseIdeal {
	criterios := CriteriosFraseIdeal{
		Palavra:            palavra,
		ExigenciaCobertura: exigenciaCoberturaPreferencial,
		Assinatura:         func(chines string) string { return variante + ":" + palavra + ":" + chines },
	}

	switch variante {
	case VarianteOrdenacao, VarianteOrdenacaoTraducao:
		criterios.ExigenciaCobertura = exigenciaFraseDeOrdenacao
		// Assinatura compartilhada: a MESMA frase não vira duas questões de ordenação na sessão,
		// ainda que em variantes diferentes.
		criterios.Assinatura = func(chines string) string { return "ordenacao:" + chines }
	case VarianteFoneticaFrase:
		criterios.ExigenciaCobertura = exigenciaFraseDeOrdenacao
		criterios.MaxPalavras = MaxPalavrasFraseFonetica
		criterios.Assinatura = func(chines string) string { return "ordenacao:" + chines }
	case VarianteFoneticaTraducao, VarianteFoneticaFilaPinyin:
		criterios.ExigenciaCobertura = exigenciaCoberturaAlta
		criterios.MaxPalavras = MaxPalavrasFraseFonetica
		criterios.Assinatura = func(chines string) string { return variante + ":" + chines }
	case VariantePronunciaFrase:
		criterios.ExigenciaCobertura = exigenciaCoberturaAlta
		criterios.Assinatura = func(chines string) string { return variante + ":" + chines }
	case VariantePronunciaSequencia:
		criterios.ExigenciaCobertura = exigenciaCoberturaAlta
		criterios.ExigirTodosConhecidos = true
		criterios.SelecaoPorContagemDeEstudo = true
		criterios.Assinatura = func(chines string) string { return variante + ":" + chines }
	}

	return criterios
}

// BuscarFraseIdeal aplica o funil de filtros sobre o acervo e sorteia a frase da questão.
// Passos OBRIGATÓRIOS (sem candidatas → falha): conter a palavra, todos conhecidos (se exigido),
// cumprir o filtro de tema/dificuldade do usuário, preservar o alvo na grafia de exibição,
// cumprir a exigência de cobertura e ser inédita na sessão. Passos PREFERENCIAIS (só valem se
// sobrar alguém): tamanho confortável, tema preferido da Jornada, compatibilidade com a grafia
// de exibição e cobertura maior.
func (b *Buscador) BuscarFraseIdeal(criterios CriteriosFraseIdeal) (dicionario.Frase, bool) {
	frases := b.frases.FrasesComPalavra(criterios.Palavra)
	if len(frases) == 0 {
		return dicionario.Frase{}, false
	}

	if criterios.ExigirTodosConhecidos {
		frases = b.frases.FiltrarTodosConhecidos(frases, b.mapaStatus)
		if len(frases) == 0 {
			return dicionario.Frase{}, false
		}
	}

	frases = b.filtrarPorFiltroDoUsuario(frases)
	if len(frases) == 0 {
		return dicionario.Frase{}, false
	}

	if curtas := b.frases.FiltrarPorTamanho(frases, TamanhoFraseConfortavel); len(curtas) > 0 {
		frases = curtas
	}

	if criterios.MaxPalavras > 0 {
		if poucasPalavras := b.frases.FiltrarPorMaxPalavras(frases, criterios.MaxPalavras, b.dicionario); len(poucasPalavras) > 0 {
			frases = poucasPalavras
		}
	}

	if doTema := b.filtrarPorTemaPreferido(frases); len(doTema) > 0 {
		frases = doTema
	}

	tipoExibicao := b.config().TipoHanziExibicao
	if tipoExibicao == "simplificado" || tipoExibicao == "tradicional" {
		compativeis := make([]dicionario.Frase, 0, len(frases))
		for _, f := range frases {
			if b.dicionario.FraseCompativelComTipo(f.Chines, tipoExibicao) {
				compativeis = append(compativeis, f)
			}
		}
		if len(compativeis) > 0 {
			frases = compativeis
		}

		frases = b.frases.FiltrarPreservandoAlvoNaConversao(frases, criterios.Palavra, b.dicionario, tipoExibicao)
		if len(frases) == 0 {
			return dicionario.Frase{}, false
		}
	}

	frases = b.filtrarPorExigenciaDeCobertura(frases, criterios.ExigenciaCobertura)
	if len(frases) == 0 {
		return dicionario.Frase{}, false
	}

	frases = b.filtrarFrasesIneditas(frases, criterios.Assinatura)
	if len(frases) == 0 {
		return dicionario.Frase{}, false
	}

	if criterios.SelecaoPorContagemDeEstudo {
		return b.frases.SelecionarSequenciaPonderada(frases, b.mapaStatus), true
	}
	return b.frases.SelecionarPonderada(frases, b.mapaStatus), true
}

// ----- Seção: Filtros do Funil -----

// filtrarPorExigenciaDeCobertura aplica a política de cobertura declarada nos critérios: as
// obrigatórias podem zerar a lista (o chamador falha); a preferencial degrada média → baixa → todas.
func (b *Buscador) filtrarPorExigenciaDeCobertura(frases []dicionario.Frase, exigencia string) []dicionario.Frase {
	switch exigencia {
	case exigenciaFraseDeOrdenacao:
		validas := make([]dicionario.Frase, 0, len(frases))
		for _, f := range frases {
			if b.verificarRequisitosMinimosFrase(f) || b.verificarFraseValidaOrdenacao(f) {
				validas = append(validas, f)
			}
		}
		if len(validas) > 0 {
			return validas
		}
		return frases

	case exigenciaCoberturaAlta:
		validas := make([]dicionario.Frase, 0, len(frases))
		for _, f := range frases {
			if b.verificarRequisitosMinimosFrase(f) {
				validas = append(validas, f)
			}
		}
		if len(validas) > 0 {
			return validas
		}
		return b.frases.FiltrarPorCobertura(frases, b.mapaStatus, dicionario.CoberturaAlta)

	default:
		validas := make([]dicionario.Frase, 0, len(frases))
		for _, f := range frases {
			if b.verificarRequisitosMinimosFrase(f) {
				validas = append(validas, f)
			}
		}
		if len(validas) > 0 {
			return validas
		}
		if frasesMedia := b.frases.FiltrarPorCobertura(frases, b.mapaStatus, dicionario.CoberturaMedia); len(frasesMedia) > 0 {
			return frasesMedia
		}
		if frasesBaixa := b.frases.FiltrarPorCobertura(frases, b.mapaStatus, dicionario.CoberturaBaixa); len(frasesBaixa) > 0 {
			return frasesBaixa
		}
		return frases
	}
}

// filtrarPorFiltroDoUsuario descarta frases que não casem com o tema e/ou dificuldade exigidos pelo
// usuário no painel de configuração da revisão (ver DefinirFiltroFrases). É um passo OBRIGATÓRIO.
// Tema e dificuldade são filtrados independentemente (filtro vazio não restringe aquela dimensão).
func (b *Buscador) filtrarPorFiltroDoUsuario(frases []dicionario.Frase) []dicionario.Frase {
	b.mu.RLock()
	temaExigido := b.temaExigido
	dificuldadeExigida := b.dificuldadeExigida
	b.mu.RUnlock()

	if temaExigido == "" && dificuldadeExigida == "" {
		return frases
	}

	filtradas := make([]dicionario.Frase, 0, len(frases))
	for _, f := range frases {
		if temaExigido != "" && f.Tema != temaExigido {
			continue
		}
		if dificuldadeExigida != "" && f.Dificuldade != dificuldadeExigida {
			continue
		}
		filtradas = append(filtradas, f)
	}
	return filtradas
}

// filtrarPorTemaPreferido devolve as frases do tema preferido da sessão (ver DefinirTemaPreferido).
// Devolve lista vazia quando não há preferência ou nenhuma candidata é do tema — o chamador, então,
// segue com as candidatas originais (é um passo PREFERENCIAL).
func (b *Buscador) filtrarPorTemaPreferido(frases []dicionario.Frase) []dicionario.Frase {
	b.mu.RLock()
	tema := b.temaPreferido
	b.mu.RUnlock()
	if tema == "" {
		return nil
	}

	doTema := make([]dicionario.Frase, 0, len(frases))
	for _, f := range frases {
		if f.Tema == tema {
			doTema = append(doTema, f)
		}
	}
	return doTema
}

// filtrarFrasesIneditas descarta as frases cuja assinatura já apareceu no histórico da sessão.
func (b *Buscador) filtrarFrasesIneditas(frases []dicionario.Frase, assinatura func(string) string) []dicionario.Frase {
	b.mu.RLock()
	historico := b.historico
	b.mu.RUnlock()
	if historico == nil {
		return frases
	}

	ineditas := make([]dicionario.Frase, 0, len(frases))
	for _, f := range frases {
		if historico[assinatura(f.Chines)] {
			continue
		}
		ineditas = append(ineditas, f)
	}
	return ineditas
}

// verificarFraseValidaOrdenacao verifica se uma frase cumpre os requisitos de cobertura de ordenação.
// - Pelo menos 90% dos hanzis devem ser conhecidos ("estudo" ou "aprendido").
// - Os 10% restantes de hanzis devem ter sido vistos no mínimo 10 vezes no OCR.
// - No máximo 1 palavra pode ser composta por esses hanzis restantes/desconhecidos.
func (b *Buscador) verificarFraseValidaOrdenacao(f dicionario.Frase) bool {
	vistos := make(map[rune]bool)
	var hanzisUnicos []rune
	for _, runa := range f.Chines {
		if !unicode.Is(unicode.Han, runa) {
			continue
		}
		if !vistos[runa] {
			vistos[runa] = true
			hanzisUnicos = append(hanzisUnicos, runa)
		}
	}

	if len(hanzisUnicos) == 0 {
		return false
	}

	conhecidosCount := 0
	var restantes []rune
	for _, runa := range hanzisUnicos {
		ch := string(runa)
		status := b.mapaStatus[ch]
		if status == dicionario.StatusEstudo || status == dicionario.StatusAprendido {
			conhecidosCount++
		} else {
			restantes = append(restantes, runa)
		}
	}

	percentConhecidos := float64(conhecidosCount) / float64(len(hanzisUnicos))
	if percentConhecidos < 0.90 {
		return false
	}

	if len(restantes) == 0 {
		return true
	}

	for _, runa := range restantes {
		ch := string(runa)
		if b.mapaVisualizacoes[ch] < 10 {
			return false
		}
	}

	palavras := b.decompor(f.Chines)
	palavrasComRestantes := 0
	for _, p := range palavras {
		if !p.EhChines {
			continue
		}
		contemRestante := false
		for _, runa := range p.Texto {
			if !unicode.Is(unicode.Han, runa) {
				continue
			}
			ch := string(runa)
			status := b.mapaStatus[ch]
			if status != dicionario.StatusEstudo && status != dicionario.StatusAprendido {
				contemRestante = true
				break
			}
		}
		if contemRestante {
			palavrasComRestantes++
		}
	}

	if palavrasComRestantes > 1 {
		return false
	}

	return true
}

// verificarRequisitosMinimosFrase verifica se uma frase cumpre os requerimentos mínimos de vocabulário:
// - Pelo menos 95% das palavras da frase já foram vistas.
// - Pelo menos 80% das palavras da frase estão como estudando ou aprendidas (desconsiderado no modo Jornada).
func (b *Buscador) verificarRequisitosMinimosFrase(f dicionario.Frase) bool {
	var palavrasChines []string
	if b.decompor != nil {
		palavras := b.decompor(f.Chines)
		for _, p := range palavras {
			if p.EhChines {
				palavrasChines = append(palavrasChines, p.Texto)
			}
		}
	} else {
		for _, r := range f.Chines {
			if unicode.Is(unicode.Han, r) {
				palavrasChines = append(palavrasChines, string(r))
			}
		}
	}

	totalPalavras := len(palavrasChines)
	if totalPalavras == 0 {
		return false
	}

	qtdEstudoAprendido := 0
	qtdVistas := 0
	ehJornada := b.temUniversoRestrito()

	for _, pal := range palavrasChines {
		if ehJornada {
			if b.PalavraEhVista(pal) {
				qtdVistas++
			}
			if b.PalavraEhEstudoOuAprendida(pal) {
				qtdEstudoAprendido++
			}
		} else {
			if b.PalavraEhEstudoOuAprendida(pal) {
				qtdEstudoAprendido++
				qtdVistas++
			} else if b.PalavraEhVista(pal) {
				qtdVistas++
			}
		}
	}

	pctVistas := float64(qtdVistas) / float64(totalPalavras)
	if pctVistas < 0.95-1e-9 {
		return false
	}

	if !ehJornada {
		pctEstudoAprendido := float64(qtdEstudoAprendido) / float64(totalPalavras)
		if pctEstudoAprendido < 0.80-1e-9 {
			return false
		}
	}

	return true
}

// PalavraEhEstudoOuAprendida verifica se a palavra (ou todos os seus caracteres) possui status estudo ou aprendida.
func (b *Buscador) PalavraEhEstudoOuAprendida(palavra string) bool {
	if b.mapaStatusPalavra != nil {
		st := b.mapaStatusPalavra[palavra]
		if st == dicionario.StatusEstudo || st == dicionario.StatusAprendido {
			return true
		}
	}
	if st := b.mapaStatus[palavra]; st == dicionario.StatusEstudo || st == dicionario.StatusAprendido {
		return true
	}

	temHan := false
	for _, r := range palavra {
		if !unicode.Is(unicode.Han, r) {
			continue
		}
		temHan = true
		ch := string(r)
		st := b.mapaStatus[ch]
		if st != dicionario.StatusEstudo && st != dicionario.StatusAprendido {
			return false
		}
	}
	return temHan
}

// PalavraEhVista verifica se a palavra (ou todos os seus caracteres) já foi vista pelo usuário.
func (b *Buscador) PalavraEhVista(palavra string) bool {
	if b.PalavraEhEstudoOuAprendida(palavra) {
		return true
	}
	if b.mapaVistoPalavra != nil && b.mapaVistoPalavra[palavra] {
		return true
	}
	if b.mapaVisto[palavra] {
		return true
	}
	if b.universoRestrito != nil && b.universoRestrito[palavra] {
		return true
	}

	temHan := false
	for _, r := range palavra {
		if !unicode.Is(unicode.Han, r) {
			continue
		}
		temHan = true
		ch := string(r)
		ehVistaRuna := b.mapaStatus[ch] != "" || b.mapaVisto[ch] || b.mapaVisualizacoes[ch] > 0 || (b.universoRestrito != nil && b.universoRestrito[ch])
		if !ehVistaRuna {
			return false
		}
	}
	return temHan
}

// ----- Seção: Produtores de Frases Distratoras -----

// FrasesDistratorasPorSimilaridade devolve as frases do acervo candidatas a distrator de tradução,
// ranqueadas da mais parecida para a menos parecida (nº de hanzis compartilhados com a frase-alvo;
// empates embaralhados), excluindo a própria frase e frases de tradução idêntica.
func (b *Buscador) FrasesDistratorasPorSimilaridade(fraseChines, fraseTraducao string) []dicionario.Frase {
	hanzisAlvo := make(map[rune]bool)
	for _, runa := range fraseChines {
		if unicode.Is(unicode.Han, runa) {
			hanzisAlvo[runa] = true
		}
	}

	type candidataDistratora struct {
		frase          dicionario.Frase
		compartilhados int
	}

	var candidatas []candidataDistratora
	for _, f := range b.frases.ObterTodasFrases() {
		if f.Chines == fraseChines || f.Ingles == fraseTraducao {
			continue
		}

		compartilhados := 0
		vistos := make(map[rune]bool)
		for _, runa := range f.Chines {
			if !unicode.Is(unicode.Han, runa) || vistos[runa] {
				continue
			}
			vistos[runa] = true
			if hanzisAlvo[runa] {
				compartilhados++
			}
		}

		candidatas = append(candidatas, candidataDistratora{frase: f, compartilhados: compartilhados})
	}

	// Embaralha para que os empates em hanzis compartilhados tenham chances iguais; a ordenação
	// estável decrescente preserva o sorteio dentro de cada faixa.
	rand.Shuffle(len(candidatas), func(i, j int) { candidatas[i], candidatas[j] = candidatas[j], candidatas[i] })
	sort.SliceStable(candidatas, func(i, j int) bool { return candidatas[i].compartilhados > candidatas[j].compartilhados })

	frases := make([]dicionario.Frase, 0, len(candidatas))
	for _, c := range candidatas {
		frases = append(frases, c.frase)
	}
	return frases
}
