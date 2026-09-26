package busca

import (
	"math/rand/v2"
	"sort"
	"strings"
	"unicode/utf8"

	"wails_app/dicionario"
)

// ----- Seção: Busca de Opções e Cartas -----
//
// Monta as ALTERNATIVAS das questões: múltipla escolha (caractere ou palavra), peças do
// quebra-cabeça de significado e cartas do baralho de pronúncia. Cada plano declara suas fontes em
// cascata e critérios; o motor central (busca.go) seleciona.

const (
	// TotalOpcoesMultiplaEscolha é o número de alternativas das questões de múltipla escolha.
	TotalOpcoesMultiplaEscolha = 4
	// TotalOpcoesTraducaoContexto é o número de alternativas da variante de tradução por contexto.
	TotalOpcoesTraducaoContexto = 3
	// TotalPecasQuebraCabeca é o número de pares hanzi↔significado do quebra-cabeça.
	TotalPecasQuebraCabeca = 4
	// totalCartasBaralho é o número de cartas do baralho de pronúncia (alvo incluído).
	totalCartasBaralho = 6

	// chanceDistratorAprendido é a fração das questões que tenta incluir 1 distrator já aprendido.
	chanceDistratorAprendido = 0.50
	// chanceDistratorEmEstudo é a fração das questões que tenta incluir 1 distrator em estudo.
	chanceDistratorEmEstudo = 0.75

	// cartasBaralhoEmEstudo/Aprendidas é a composição-alvo do baralho além do alvo (~75% / ~25%).
	cartasBaralhoEmEstudo   = 4
	cartasBaralhoAprendidas = 2

	// maximoPalavrasMesmoTamanho limita o preenchimento com palavras sem hanzi em comum.
	maximoPalavrasMesmoTamanho = 12

	// TotalPoolVistas é o teto do pool de fallback das já vistas frequentes (ver PalavrasVistasFrequentes).
	TotalPoolVistas = 10
	// VagasLivresPoolVistas são as primeiras vagas do pool preenchidas só por frequência (sem exigir
	// a flag "visto"); da vaga seguinte em diante só entram caracteres já vistos, garantindo o mínimo
	// de vistas no pool quando há vistas suficientes no ranking.
	VagasLivresPoolVistas = 5
)

// Pools agrupa os bancos de candidatas da sessão — o insumo padrão das buscas de opções.
type Pools struct {
	Todos      []dicionario.DecomposicaoHanzi // todas as elegíveis ao modo da questão
	EmEstudo   []dicionario.DecomposicaoHanzi
	Aprendidos []dicionario.DecomposicaoHanzi

	// Vistas é o pool de fallback das já vistas frequentes: as TotalPoolVistas palavras do topo do
	// ranking de frequência ainda fora do estudo/aprendido, enviesado para as já vistas. NÃO é
	// preenchido por PoolsDoModo (que só separa por status) — o chamador o injeta com
	// PalavrasVistasFrequentes, pois depende do ranking de frequência do dicionário.
	Vistas []dicionario.DecomposicaoHanzi
}

// PoolsDoModo separa os candidatos de um modo pelos status do vocabulário da sessão. NÃO preenche
// Vistas (ver o campo): esse pool depende do ranking de frequência e é injetado pelo chamador.
func PoolsDoModo(candidatos []dicionario.DecomposicaoHanzi, mapaStatus map[string]string) Pools {
	pools := Pools{Todos: candidatos}
	for _, c := range candidatos {
		switch mapaStatus[c.Caractere] {
		case dicionario.StatusEstudo:
			pools.EmEstudo = append(pools.EmEstudo, c)
		case dicionario.StatusAprendido:
			pools.Aprendidos = append(pools.Aprendidos, c)
		}
	}
	return pools
}

// PalavrasVistasFrequentes monta o pool de fallback das já vistas frequentes a partir dos candidatos
// do modo: descarta as já promovidas a estudo/aprendido e as sem posição no ranking de frequência,
// ordena o restante por frequência (mais comum primeiro) e recorta com recortarVistasFrequentes —
// as primeiras vagas por frequência pura e as seguintes só para já vistas.
func (b *Buscador) PalavrasVistasFrequentes(candidatos []dicionario.DecomposicaoHanzi) []dicionario.DecomposicaoHanzi {
	type ranqueada struct {
		entrada dicionario.DecomposicaoHanzi
		posicao int
	}

	var elegiveis []ranqueada
	for _, c := range candidatos {
		if b.mapaStatus[c.Caractere] == dicionario.StatusEstudo || b.mapaStatus[c.Caractere] == dicionario.StatusAprendido {
			continue
		}
		freq := b.dicionario.Banco.ObterFrequencia(c.Caractere)
		if freq == nil || freq.Posicao <= 0 {
			continue
		}
		elegiveis = append(elegiveis, ranqueada{c, freq.Posicao})
	}

	sort.SliceStable(elegiveis, func(i, j int) bool { return elegiveis[i].posicao < elegiveis[j].posicao })

	ordenadas := make([]dicionario.DecomposicaoHanzi, len(elegiveis))
	for i, e := range elegiveis {
		ordenadas[i] = e.entrada
	}
	return recortarVistasFrequentes(ordenadas, func(ch string) bool { return b.mapaVisto[ch] })
}

// recortarVistasFrequentes recorta a lista já ordenada por frequência em até TotalPoolVistas itens:
// as VagasLivresPoolVistas primeiras entram por frequência pura (já vistas ou não); da vaga seguinte
// em diante só entram as que ehVisto aprova. Assim o pool fica com no mínimo (TotalPoolVistas -
// VagasLivresPoolVistas) já vistas quando há vistas suficientes no ranking, sem abrir mão de encabeçá-lo
// com as mais frequentes.
func recortarVistasFrequentes(ordenadas []dicionario.DecomposicaoHanzi, ehVisto func(string) bool) []dicionario.DecomposicaoHanzi {
	selecionadas := make([]dicionario.DecomposicaoHanzi, 0, TotalPoolVistas)
	for _, c := range ordenadas {
		if len(selecionadas) >= TotalPoolVistas {
			break
		}
		if len(selecionadas) >= VagasLivresPoolVistas && !ehVisto(c.Caractere) {
			continue
		}
		selecionadas = append(selecionadas, c)
	}
	return selecionadas
}

// ----- Seção: Planos de Múltipla Escolha -----

// BuscarOpcoes monta as alternativas de múltipla escolha para qualquer alvo: palavra multi-hanzi
// usa o plano de palavras parecidas; caractere isolado usa os bancos da sessão por status. Se o alvo
// estiver em foco (ou `emFoco` for verdadeiro), garante a inclusão de ao menos um distrator em foco (se houver).
func (b *Buscador) BuscarOpcoes(alvo dicionario.DecomposicaoHanzi, emFoco bool, pools Pools, aceitar CriterioDistrator) []OpcaoRevisao {
	if utf8.RuneCountInString(alvo.Caractere) > 1 {
		return b.buscarOpcoesPalavra(alvo, emFoco, pools, aceitar)
	}
	return b.buscarOpcoesCaractere(alvo, emFoco, pools, aceitar)
}

// buscarOpcoesCaractere busca as alternativas para alvo de um caractere: mistura probabilística
// dos bancos por status (chanceDistratorAprendido/EmEstudo variam a composição entre questões) com
// preenchimento do banco geral do modo. Se o alvo for em foco, prioriza um distrator do foco.
func (b *Buscador) buscarOpcoesCaractere(alvo dicionario.DecomposicaoHanzi, emFoco bool, pools Pools, aceitar CriterioDistrator) []OpcaoRevisao {
	var fontes []FonteBusca[dicionario.DecomposicaoHanzi]

	if (emFoco || (b != nil && b.mapaFoco[alvo.Caractere])) && b != nil {
		fontes = append(fontes, FonteBusca[dicionario.DecomposicaoHanzi]{
			Sequencia: SequenciaEmbaralhada(b.pecasCandidatasDoFoco(pools)),
			Chance:    1.0,
			Maximo:    1,
		})
	}

	// Universo fechado (Jornada): o pool mistura caracteres soltos e palavras de 2–3 hanzis, então a
	// alternativa do MESMO tamanho do alvo vem primeiro — senão o alvo de 1 hanzi apareceria entre
	// três palavras compridas e se entregaria pelo formato.
	if b.temUniversoRestrito() {
		fontes = append(fontes, FonteBusca[dicionario.DecomposicaoHanzi]{
			Sequencia: SequenciaEmbaralhada(entradasComTamanhoDe(pools.Todos, alvo.Caractere)),
		})
	}

	fontes = append(fontes,
		FonteBusca[dicionario.DecomposicaoHanzi]{Sequencia: SequenciaEmbaralhada(pools.Aprendidos), Chance: chanceDistratorAprendido, Maximo: 1},
		FonteBusca[dicionario.DecomposicaoHanzi]{Sequencia: SequenciaEmbaralhada(pools.EmEstudo), Chance: chanceDistratorEmEstudo, Maximo: 1},
		FonteBusca[dicionario.DecomposicaoHanzi]{Sequencia: SequenciaEmbaralhada(pools.Vistas)},
		FonteBusca[dicionario.DecomposicaoHanzi]{Sequencia: SequenciaEmbaralhada(pools.Todos)},
	)

	opcoes := BuscarDeFontes(
		TotalOpcoesMultiplaEscolha,
		[]OpcaoRevisao{novaOpcao(alvo, true)},
		fontes,
		aceitarDistratorForaDoAlvo(alvo, aceitar),
		novaOpcaoDistratora,
	)

	rand.Shuffle(len(opcoes), func(i, j int) { opcoes[i], opcoes[j] = opcoes[j], opcoes[i] })
	return opcoes
}

// entradasComTamanhoDe recorta as entradas com a mesma quantidade de hanzis do alvo (excluindo o
// próprio alvo). Usado quando a sessão tem universo fechado e o pool mistura tamanhos.
func entradasComTamanhoDe(entradas []dicionario.DecomposicaoHanzi, alvo string) []dicionario.DecomposicaoHanzi {
	tamanhoAlvo := utf8.RuneCountInString(alvo)
	mesmoTamanho := make([]dicionario.DecomposicaoHanzi, 0, len(entradas))
	for _, entrada := range entradas {
		if entrada.Caractere == alvo || utf8.RuneCountInString(entrada.Caractere) != tamanhoAlvo {
			continue
		}
		mesmoTamanho = append(mesmoTamanho, entrada)
	}
	return mesmoTamanho
}

// buscarOpcoesPalavra busca as alternativas para alvo palavra multi-hanzi. Os distratores preferem
// palavras com a MESMA quantidade de hanzis que diferem do alvo em exatamente um (compartilhando os
// demais), priorizando quando o hanzi divergente tem decomposição parecida com o que substitui; o
// preenchimento cai para palavras do mesmo tamanho sem hanzi em comum. Se o alvo for em foco, prioriza
// um distrator do foco.
func (b *Buscador) buscarOpcoesPalavra(alvo dicionario.DecomposicaoHanzi, emFoco bool, pools Pools, aceitar CriterioDistrator) []OpcaoRevisao {
	var fontes []FonteBusca[dicionario.DecomposicaoHanzi]

	if (emFoco || (b != nil && b.mapaFoco[alvo.Caractere])) && b != nil {
		fontes = append(fontes, FonteBusca[dicionario.DecomposicaoHanzi]{
			Sequencia: SequenciaEmbaralhada(b.pecasCandidatasDoFoco(pools)),
			Chance:    1.0,
			Maximo:    1,
		})
	}

	fontes = append(fontes,
		FonteBusca[dicionario.DecomposicaoHanzi]{Sequencia: SequenciaPreguicosa(func() []dicionario.DecomposicaoHanzi { return b.palavrasQueDiferemPorUmHanzi(alvo.Caractere) })},
		FonteBusca[dicionario.DecomposicaoHanzi]{Sequencia: SequenciaPreguicosa(func() []dicionario.DecomposicaoHanzi { return b.palavrasMesmoTamanho(alvo.Caractere) })},
	)

	// Universo fechado (Jornada): as duas fontes acima exigem mesmo tamanho do alvo e podem não
	// completar as alternativas dentro de um conjunto pequeno — o pool do modo (que ali JÁ é o
	// universo) fecha a conta em vez de deixar a questão com menos opções.
	if b.temUniversoRestrito() {
		fontes = append(fontes, FonteBusca[dicionario.DecomposicaoHanzi]{Sequencia: SequenciaEmbaralhada(pools.Todos)})
	}

	opcoes := BuscarDeFontes(
		TotalOpcoesMultiplaEscolha,
		[]OpcaoRevisao{novaOpcao(alvo, true)},
		fontes,
		aceitarDistratorForaDoAlvo(alvo, aceitar),
		novaOpcaoDistratora,
	)

	rand.Shuffle(len(opcoes), func(i, j int) { opcoes[i], opcoes[j] = opcoes[j], opcoes[i] })
	return opcoes
}

// ----- Seção: Plano do Quebra-Cabeça (significado, fonética e trio) -----
//
// A composição das peças segue quotas mínimas de ~25% (PecasQuotaMinima) por categoria do
// vocabulário, na ordem de prioridade: palavras do grupo de foco, aprendidas com a sequência de
// acertos da(s) área(s) da atividade ainda não maximizada e em estudo fora do foco (o foco já tem
// quota própria e é sempre um subconjunto do estudo). O alvo abate a quota da própria categoria
// (como no baralho de pronúncia) e o preenchimento restante cai para as já vistas frequentes e o
// banco geral do modo. Quando uma categoria não tem candidatas aprovadas suficientes, a vaga
// transborda naturalmente para as fontes seguintes da cascata.

// PecasQuotaMinima é a quota mínima de peças por categoria do vocabulário (⌈25%⌉ das peças).
const PecasQuotaMinima = (TotalPecasQuebraCabeca + 3) / 4

// BuscarPecasQuebraCabeca busca os pares do quebra-cabeça: o alvo + entradas de glosa principal
// distinta, todas marcadas corretas (cada hanzi é pareado ao próprio significado). Pode devolver
// menos que TotalPecasQuebraCabeca — o chamador decide o fallback.
func (b *Buscador) BuscarPecasQuebraCabeca(alvo dicionario.DecomposicaoHanzi, pools Pools) []OpcaoRevisao {
	return b.buscarPecasQuebraCabecaComposto(alvo, pools, DistintosPorSignificado, []string{ModoSignificado})
}

// BuscarPecasQuebraCabecaFonetica busca os pares do quebra-cabeça de fonética (distintos por som).
func (b *Buscador) BuscarPecasQuebraCabecaFonetica(alvo dicionario.DecomposicaoHanzi, pools Pools) []OpcaoRevisao {
	return b.buscarPecasQuebraCabecaComposto(alvo, pools, DistintosPorPinyin, []string{ModoFonetica})
}

// BuscarPecasQuebraCabecaTrio busca os trios do quebra-cabeça (distintos por significado e som).
// A pendência de aprendida considera as duas áreas praticadas: basta uma abaixo da meta.
func (b *Buscador) BuscarPecasQuebraCabecaTrio(alvo dicionario.DecomposicaoHanzi, pools Pools) []OpcaoRevisao {
	distintosDuplo := func(escolhidas []OpcaoRevisao, candidata dicionario.DecomposicaoHanzi) bool {
		return DistintosPorSignificado(escolhidas, candidata) && DistintosPorPinyin(escolhidas, candidata)
	}
	return b.buscarPecasQuebraCabecaComposto(alvo, pools, distintosDuplo, []string{ModoSignificado, ModoFonetica})
}

// buscarPecasQuebraCabecaComposto monta as peças de qualquer variante de quebra-cabeça: declara a
// cascata com as quotas por categoria e delega a seleção ao motor central.
func (b *Buscador) buscarPecasQuebraCabecaComposto(alvo dicionario.DecomposicaoHanzi, pools Pools, aceitar CriterioDistrator, areasAtividade []string) []OpcaoRevisao {
	quotaFoco, quotaAprendidas, quotaEstudo := b.quotasPecasQuebraCabeca(alvo, areasAtividade)

	fontes := []FonteBusca[dicionario.DecomposicaoHanzi]{
		{Sequencia: SequenciaPreguicosa(func() []dicionario.DecomposicaoHanzi { return b.pecasCandidatasDoFoco(pools) }), Maximo: quotaFoco},
		{Sequencia: SequenciaPreguicosa(func() []dicionario.DecomposicaoHanzi { return b.aprendidasComAreaPendente(pools, areasAtividade) }), Maximo: quotaAprendidas},
		{Sequencia: SequenciaPreguicosa(func() []dicionario.DecomposicaoHanzi { return b.emEstudoForaDoFoco(pools) }), Maximo: quotaEstudo},
		{Sequencia: SequenciaEmbaralhada(pools.Vistas)},
		{Sequencia: SequenciaEmbaralhada(pools.Todos)},
	}

	pecas := BuscarDeFontes(
		TotalPecasQuebraCabeca,
		[]OpcaoRevisao{novaOpcao(alvo, true)},
		fontes,
		b.aceitarPecaQuebraCabecaForaDoAlvo(alvo, aceitar),
		func(entrada dicionario.DecomposicaoHanzi) OpcaoRevisao { return novaOpcao(entrada, true) },
	)

	rand.Shuffle(len(pecas), func(i, j int) { pecas[i], pecas[j] = pecas[j], pecas[i] })
	return pecas
}

// quotasPecasQuebraCabeca resolve as quotas de cada categoria: partem da quota mínima e o alvo
// abate a da sua categoria mais específica, já que ele ocupa uma das vagas da seleção — foco
// primeiro (toda palavra em foco também está em estudo), depois aprendida pendente, depois estudo.
func (b *Buscador) quotasPecasQuebraCabeca(alvo dicionario.DecomposicaoHanzi, areasAtividade []string) (quotaFoco, quotaAprendidas, quotaEstudo int) {
	quotaFoco, quotaAprendidas, quotaEstudo = PecasQuotaMinima, PecasQuotaMinima, PecasQuotaMinima
	switch {
	case b.mapaFoco[alvo.Caractere]:
		quotaFoco--
	case b.mapaStatus[alvo.Caractere] == dicionario.StatusAprendido && b.temAreaPendente(alvo.Caractere, areasAtividade):
		quotaAprendidas--
	case b.mapaStatus[alvo.Caractere] == dicionario.StatusEstudo:
		quotaEstudo--
	}
	return quotaFoco, quotaAprendidas, quotaEstudo
}

// pecasCandidatasDoFoco monta (embaralhadas) as entradas das palavras do grupo de foco da sessão:
// caractere presente nos candidatos do modo usa a entrada do banco da questão; palavra multi-hanzi
// entra pela leitura do dicionário (o mesmo caminho da entrada de alvo em foco da revisão).
func (b *Buscador) pecasCandidatasDoFoco(pools Pools) []dicionario.DecomposicaoHanzi {
	porCaractere := make(map[string]dicionario.DecomposicaoHanzi, len(pools.Todos))
	for _, c := range pools.Todos {
		porCaractere[c.Caractere] = c
	}

	var candidatas []dicionario.DecomposicaoHanzi
	for _, palavra := range b.focoSessao {
		if entrada, ok := porCaractere[palavra]; ok {
			candidatas = append(candidatas, entrada)
			continue
		}
		if b.dicionario != nil && utf8.RuneCountInString(palavra) > 1 {
			if entrada := b.entradaPalavraCedict(palavra); entrada != nil {
				candidatas = append(candidatas, *entrada)
			}
		}
	}
	rand.Shuffle(len(candidatas), func(i, j int) { candidatas[i], candidatas[j] = candidatas[j], candidatas[i] })
	return candidatas
}

// aprendidasComAreaPendente filtra (embaralhadas) as aprendidas do modo cuja sequência de acertos
// ainda não atingiu a meta em alguma das áreas praticadas pela atividade.
func (b *Buscador) aprendidasComAreaPendente(pools Pools, areas []string) []dicionario.DecomposicaoHanzi {
	var pendentes []dicionario.DecomposicaoHanzi
	for _, c := range pools.Aprendidos {
		if b.temAreaPendente(c.Caractere, areas) {
			pendentes = append(pendentes, c)
		}
	}
	rand.Shuffle(len(pendentes), func(i, j int) { pendentes[i], pendentes[j] = pendentes[j], pendentes[i] })
	return pendentes
}

// temAreaPendente diz se alguma das áreas da palavra ainda está com streak abaixo da meta da sessão.
func (b *Buscador) temAreaPendente(palavra string, areas []string) bool {
	streaks := b.streaksDaPalavra(palavra)
	for _, area := range areas {
		if streaks[area] < b.metaStreak {
			return true
		}
	}
	return false
}

// emEstudoForaDoFoco filtra (embaralhadas) as em estudo do modo que não estão no grupo de foco —
// o foco tem quota própria; separar evita contar a mesma palavra em duas categorias.
func (b *Buscador) emEstudoForaDoFoco(pools Pools) []dicionario.DecomposicaoHanzi {
	var estudo []dicionario.DecomposicaoHanzi
	for _, c := range pools.EmEstudo {
		if !b.mapaFoco[c.Caractere] {
			estudo = append(estudo, c)
		}
	}
	rand.Shuffle(len(estudo), func(i, j int) { estudo[i], estudo[j] = estudo[j], estudo[i] })
	return estudo
}

// ----- Seção: Plano do Baralho de Pronúncia -----

// BuscarBaralhoPronuncia seleciona as cartas do baralho: o alvo + palavras dos bancos da sessão na
// composição-alvo (~75% em estudo / ~25% aprendidas, ajustada pelo status do próprio alvo),
// completando em cascata (estudo → aprendido → geral) até completar o baralho.
func BuscarBaralhoPronuncia(alvo dicionario.DecomposicaoHanzi, pools Pools) []dicionario.DecomposicaoHanzi {
	metaEstudo, metaAprendido := cartasBaralhoEmEstudo, cartasBaralhoAprendidas
	if contemCaractere(pools.EmEstudo, alvo.Caractere) {
		metaEstudo-- // o alvo já ocupa uma vaga de estudo
	} else if contemCaractere(pools.Aprendidos, alvo.Caractere) {
		metaAprendido-- // o alvo já ocupa uma vaga de aprendida
	} else {
		metaAprendido-- // alvo fora dos bancos: sobram 5 vagas (4 estudo + 1 aprendida)
	}

	fontes := []FonteBusca[dicionario.DecomposicaoHanzi]{
		{Sequencia: SequenciaEmbaralhada(pools.EmEstudo), Maximo: metaEstudo},
		{Sequencia: SequenciaEmbaralhada(pools.Aprendidos), Maximo: metaAprendido},
		{Sequencia: SequenciaEmbaralhada(pools.EmEstudo)},
		{Sequencia: SequenciaEmbaralhada(pools.Aprendidos)},
		{Sequencia: SequenciaEmbaralhada(pools.Vistas)},
		{Sequencia: SequenciaEmbaralhada(pools.Todos)},
	}

	aceitar := func(escolhidas []dicionario.DecomposicaoHanzi, candidata dicionario.DecomposicaoHanzi) bool {
		if candidata.Caractere == "" || len(candidata.Pinyin) == 0 {
			return false
		}
		return !contemCaractere(escolhidas, candidata.Caractere)
	}

	cartas := BuscarDeFontesSimples(totalCartasBaralho, []dicionario.DecomposicaoHanzi{alvo}, fontes, aceitar)

	rand.Shuffle(len(cartas), func(i, j int) { cartas[i], cartas[j] = cartas[j], cartas[i] })
	return cartas
}

var listaIniciaisPinyin = []string{
	"zh", "ch", "sh",
	"b", "p", "d", "t", "g", "k", "z", "c", "s", "j", "q", "x", "r", "f", "h", "l", "m", "n", "w", "y",
}

func extrairInicialPinyin(pinyin string) string {
	if len(pinyin) == 0 {
		return ""
	}
	pinyinLimpo := dicionario.RemoverTonsPinyin(strings.ToLower(pinyin))
	pinyinLimpo = strings.TrimSpace(pinyinLimpo)
	silabas := strings.Fields(pinyinLimpo)
	if len(silabas) == 0 {
		return ""
	}
	silaba := silabas[0]

	for _, inicial := range listaIniciaisPinyin {
		if strings.HasPrefix(silaba, inicial) {
			return inicial
		}
	}
	return ""
}

// BuscarBaralhoPronunciaTipo seleciona as cartas do baralho pertencentes ao MESMO tipo de pronúncia
// (mesma consoante inicial) do alvo, variando tom e vogais.
func BuscarBaralhoPronunciaTipo(alvo dicionario.DecomposicaoHanzi, pools Pools) []dicionario.DecomposicaoHanzi {
	pinyinAlvo := ""
	if len(alvo.Pinyin) > 0 {
		pinyinAlvo = alvo.Pinyin[0]
	}
	inicialAlvo := extrairInicialPinyin(pinyinAlvo)

	fontes := []FonteBusca[dicionario.DecomposicaoHanzi]{
		{Sequencia: SequenciaEmbaralhada(pools.EmEstudo)},
		{Sequencia: SequenciaEmbaralhada(pools.Aprendidos)},
		{Sequencia: SequenciaEmbaralhada(pools.Vistas)},
		{Sequencia: SequenciaEmbaralhada(pools.Todos)},
	}

	aceitar := func(escolhidas []dicionario.DecomposicaoHanzi, candidata dicionario.DecomposicaoHanzi) bool {
		if candidata.Caractere == "" || len(candidata.Pinyin) == 0 {
			return false
		}
		if extrairInicialPinyin(candidata.Pinyin[0]) != inicialAlvo {
			return false
		}
		return !contemCaractere(escolhidas, candidata.Caractere)
	}

	cartas := BuscarDeFontesSimples(totalCartasBaralho, []dicionario.DecomposicaoHanzi{alvo}, fontes, aceitar)

	rand.Shuffle(len(cartas), func(i, j int) { cartas[i], cartas[j] = cartas[j], cartas[i] })
	return cartas
}

// ----- Seção: Produtores de Palavras Distratoras -----

// palavrasQueDiferemPorUmHanzi devolve palavras do dicionário com a mesma quantidade de hanzis do
// alvo que diferem dele em EXATAMENTE um hanzi (compartilham os demais), ordenadas pela semelhança
// de decomposição entre o hanzi divergente e o que ele substitui (mais parecido primeiro).
func (b *Buscador) palavrasQueDiferemPorUmHanzi(alvo string) []dicionario.DecomposicaoHanzi {
	runasAlvo := []rune(alvo)
	n := len(runasAlvo)
	if n < 2 {
		return nil
	}

	type candidata struct {
		entrada dicionario.DecomposicaoHanzi
		score   int
	}
	var lista []candidata
	vistos := make(map[string]bool)

	for _, p := range b.palavrasDisponiveis() {
		if p == alvo || vistos[p] {
			continue
		}
		runasP := []rune(p)
		if len(runasP) != n {
			continue
		}

		posDivergente := -1
		divergencias := 0
		for i := 0; i < n; i++ {
			if runasP[i] != runasAlvo[i] {
				divergencias++
				posDivergente = i
			}
		}
		if divergencias != 1 {
			continue
		}
		vistos[p] = true

		entrada := b.entradaPalavraCedict(p)
		if entrada == nil {
			continue
		}
		score := b.similaridadeDecomposicao(string(runasP[posDivergente]), string(runasAlvo[posDivergente]))
		lista = append(lista, candidata{*entrada, score})
	}

	// Desempate aleatório antes da ordenação estável por score (mais parecido primeiro).
	rand.Shuffle(len(lista), func(i, j int) { lista[i], lista[j] = lista[j], lista[i] })
	sort.SliceStable(lista, func(i, j int) bool { return lista[i].score > lista[j].score })

	resultado := make([]dicionario.DecomposicaoHanzi, 0, len(lista))
	for _, c := range lista {
		resultado = append(resultado, c.entrada)
	}
	return resultado
}

// palavrasMesmoTamanho devolve, embaralhadas, algumas palavras do dicionário com a mesma quantidade
// de hanzis do alvo (sem exigir hanzi em comum) — só como preenchimento quando os distratores ideais
// não completam as alternativas.
func (b *Buscador) palavrasMesmoTamanho(alvo string) []dicionario.DecomposicaoHanzi {
	n := len([]rune(alvo))
	todas := b.palavrasDisponiveis()
	var resultado []dicionario.DecomposicaoHanzi
	for _, idx := range rand.Perm(len(todas)) {
		p := todas[idx]
		if p == alvo || len([]rune(p)) != n {
			continue
		}
		if entrada := b.entradaPalavraCedict(p); entrada != nil {
			resultado = append(resultado, *entrada)
			if len(resultado) >= maximoPalavrasMesmoTamanho {
				break
			}
		}
	}
	return resultado
}

// entradaPalavraCedict monta a entrada de revisão de uma palavra a partir do dicionário, ou nil se
// não houver pinyin/significado.
func (b *Buscador) entradaPalavraCedict(palavra string) *dicionario.DecomposicaoHanzi {
	entradas := b.dicionario.Banco.Buscar(palavra)
	if len(entradas) == 0 || entradas[0].Pinyin == "" || len(entradas[0].Significados) == 0 {
		return nil
	}
	return &dicionario.DecomposicaoHanzi{
		Caractere: palavra,
		Pinyin:    []string{entradas[0].Pinyin},
		Definicao: strings.Join(entradas[0].Significados, "; "),
	}
}

// similaridadeDecomposicao pontua o quanto dois hanzis se parecem estruturalmente: +2 se dividem o
// mesmo radical, +1 por componente etimológico (fonético/semântico) em comum. Quanto maior, mais
// plausível é o distrator (o hanzi divergente "imita" o que substitui).
func (b *Buscador) similaridadeDecomposicao(a, outro string) int {
	da := b.dicionario.DecomporHanzi(a)
	db := b.dicionario.DecomporHanzi(outro)
	if da == nil || db == nil {
		return 0
	}
	score := 0
	if da.Radical != "" && da.Radical == db.Radical {
		score += 2
	}
	componentesA := map[string]bool{}
	if da.Etimologia.Fonetica != "" {
		componentesA[da.Etimologia.Fonetica] = true
	}
	if da.Etimologia.Semantica != "" {
		componentesA[da.Etimologia.Semantica] = true
	}
	if db.Etimologia.Fonetica != "" && componentesA[db.Etimologia.Fonetica] {
		score++
	}
	if db.Etimologia.Semantica != "" && componentesA[db.Etimologia.Semantica] {
		score++
	}
	return score
}

// ----- Seção: Utilitários de Opção -----

// aceitarDistratorForaDoAlvo compõe o critério da atividade com a exclusão do próprio alvo.
func aceitarDistratorForaDoAlvo(alvo dicionario.DecomposicaoHanzi, aceitar CriterioDistrator) CriterioDistrator {
	return func(escolhidas []OpcaoRevisao, candidata dicionario.DecomposicaoHanzi) bool {
		return candidata.Caractere != alvo.Caractere && aceitar(escolhidas, candidata)
	}
}

// aceitarPecaQuebraCabecaForaDoAlvo compõe o critério do quebra-cabeça excluindo o alvo e palavras que já foram peças na sessão.
func (b *Buscador) aceitarPecaQuebraCabecaForaDoAlvo(alvo dicionario.DecomposicaoHanzi, aceitar CriterioDistrator) CriterioDistrator {
	return func(escolhidas []OpcaoRevisao, candidata dicionario.DecomposicaoHanzi) bool {
		if candidata.Caractere == alvo.Caractere {
			return false
		}
		if b.JaEsteveEmQuebraCabecaNaSessao(candidata.Caractere) {
			return false
		}
		return aceitar(escolhidas, candidata)
	}
}

// NovaOpcao monta uma estrutura de OpcaoRevisao.
func NovaOpcao(entrada dicionario.DecomposicaoHanzi, correta bool) OpcaoRevisao {
	return novaOpcao(entrada, correta)
}

func novaOpcao(entrada dicionario.DecomposicaoHanzi, correta bool) OpcaoRevisao {
	return OpcaoRevisao{
		Hanzi:     entrada.Caractere,
		Pinyin:    entrada.Pinyin[0],
		Definicao: entrada.Definicao,
		Correta:   correta,
	}
}

func novaOpcaoDistratora(entrada dicionario.DecomposicaoHanzi) OpcaoRevisao {
	return novaOpcao(entrada, false)
}

func contemCaractere(entradas []dicionario.DecomposicaoHanzi, caractere string) bool {
	for _, e := range entradas {
		if e.Caractere == caractere {
			return true
		}
	}
	return false
}

// DistintosPorSignificadoConhecidos aceita candidatas que possuem significados distintos e pertencem
// aos pools de palavras já conhecidas/estudadas/vistas pelo usuário.
func DistintosPorSignificadoConhecidos(mapaStatus map[string]string, mapaVisto map[string]bool) CriterioDistrator {
	return func(escolhidas []OpcaoRevisao, candidata dicionario.DecomposicaoHanzi) bool {
		status := mapaStatus[candidata.Caractere]
		ehConhecido := status == dicionario.StatusAprendido || status == dicionario.StatusEstudo || mapaVisto[candidata.Caractere]
		if !ehConhecido {
			return false
		}
		return DistintosPorSignificado(escolhidas, candidata)
	}
}

// DistintosPorSignificadoConhecidos devolve o critério de distrator de palavras conhecidas vinculado ao Buscador.
func (b *Buscador) DistintosPorSignificadoConhecidos() CriterioDistrator {
	return DistintosPorSignificadoConhecidos(b.mapaStatus, b.mapaVisto)
}

// DistintosPorHanziConhecidos aceita candidatas que possuem Hanzis distintos e pertencem
// aos pools de palavras já conhecidas/estudadas/vistas pelo usuário.
func DistintosPorHanziConhecidos(mapaStatus map[string]string, mapaVisto map[string]bool) CriterioDistrator {
	return func(escolhidas []OpcaoRevisao, candidata dicionario.DecomposicaoHanzi) bool {
		status := mapaStatus[candidata.Caractere]
		ehConhecido := status == dicionario.StatusAprendido || status == dicionario.StatusEstudo || mapaVisto[candidata.Caractere]
		if !ehConhecido {
			return false
		}
		return DistintosPorHanzi(escolhidas, candidata)
	}
}

// DistintosPorHanziConhecidos devolve o critério de distrator de Hanzis de palavras conhecidas vinculado ao Buscador.
func (b *Buscador) DistintosPorHanziConhecidos() CriterioDistrator {
	return DistintosPorHanziConhecidos(b.mapaStatus, b.mapaVisto)
}
