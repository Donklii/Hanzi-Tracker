package busca

import (
	"sync"

	"wails_app/config"
	"wails_app/dicionario"
)

// ----- Seção: O Buscador -----
//
// Buscador é a interface central de busca por questões/distratores/atividades ideais: guarda as
// dependências fixas da sessão (dicionário, acervo de frases, configuração) e o estado mutável de
// uma sessão de revisão (status/visualizações do vocabulário do usuário, histórico de deduplicação
// entre questões). Todo módulo de atividade (significado, fonética, desenho, contexto, ordenação,
// pronúncia) consulta um *Buscador em vez de implementar sua própria lógica de seleção.

// PalavraTexto é o segmento mínimo que o Buscador precisa da decomposição de uma frase em palavras
// (usado só para contar quantas palavras de uma frase contêm hanzi desconhecido). A segmentação
// completa (com pinyin/significados) é responsabilidade do pacote revisao — injetada via Decompor.
type PalavraTexto struct {
	Texto    string
	EhChines bool
}

// Buscador é o motor com estado de uma sessão de revisão.
type Buscador struct {
	dicionario           *dicionario.GerenciadorDicionario
	frases               *dicionario.GerenciadorFrases
	compreensao          *dicionario.GerenciadorCompreensao
	config               func() config.Config
	decompor             func(texto string) []PalavraTexto
	temTracadoComponente func(palavra string) bool
	estatisticasPalavra  func(palavra string) map[string]int

	mapaStatus        map[string]string
	mapaStatusPalavra map[string]string
	mapaVisto         map[string]bool
	mapaVistoPalavra  map[string]bool
	mapaVisualizacoes map[string]int

	// focoSessao/mapaFoco guardam o grupo de foco vigente na sessão (lista na ordem persistida e
	// conjunto para consulta) — a categoria "em foco" das quotas do quebra-cabeça. metaStreak é a
	// meta de acertos consecutivos que "maximiza" uma área (define a pendência das aprendidas).
	focoSessao []string
	mapaFoco   map[string]bool
	metaStreak int

	// temaPreferido é o tema (rótulo chinês da taxonomia de frases) que a sessão prefere nas frases —
	// usado pela Jornada, cujos ramos são temáticos. Vazio = sem preferência.
	temaPreferido string

	// temaExigido e dificuldadeExigida são os filtros ESCOLHIDOS PELO USUÁRIO no painel da revisão.
	// Ao contrário de temaPreferido (preferencial, da Jornada), eles são OBRIGATÓRIOS: frase fora
	// do filtro é descartada. Vazio = sem filtro.
	temaExigido        string
	dificuldadeExigida string

	// dificuldadeAlvo é a categoria de dificuldade (introdução, iniciante, intermediário, avançado)
	// que a sessão prefere nas variantes das atividades — usado pela Jornada para progressão de
	// dificuldade por exposição. Vazio = sorteio livre de variante.
	dificuldadeAlvo string

	// escadaEstendida lista, por modo, as atividades EMPRESTADAS de outros modos nesta sessão, e
	// dificuldadeEmprestada guarda o degrau que cada uma passa a ocupar. É a Jornada, que trata
	// completar a lacuna da frase como o degrau avançado do significado: sem isso a escada do
	// significado não tem nada acima de "iniciante" e desaba toda no quebra-cabeça. Vazio (o padrão
	// das demais revisões) = cada modo com as suas variantes de sempre, na classificação de sempre.
	escadaEstendida       map[string][]string
	dificuldadeEmprestada map[string]string

	// variantesVetadas são as atividades que a sessão NÃO quer sortear na questão da vez — usado pela
	// Jornada para não repor no tabuleiro uma palavra que já esteve num. Vazio = nada vetado; um veto
	// que zeraria as opções é ignorado (a questão sai numa atividade vetada em vez de não sair).
	variantesVetadas map[string]bool

	// universoRestrito fecha as fontes de distratores/peças num conjunto de palavras. É a Jornada:
	// as questões dela só podem usar o que o próprio caminho já ensinou, sem tomar emprestado o
	// vocabulário do usuário. Vazio (o padrão das demais revisões) = dicionário inteiro liberado.
	universoRestrito      map[string]bool
	universoRestritoLista []string

	mu                sync.RWMutex
	historico         map[string]bool
	cacheEstatisticas map[string]map[string]int
}

// Novo cria o Buscador para a sessão. `decompor` quebra uma frase em palavras/segmentos (usado na
// validação de frases de ordenação); `temTracadoComponente` diz se uma PALAVRA (possivelmente
// multi-hanzi) tem algum componente com traçado disponível (usado na elegibilidade do modo desenho);
// `estatisticasPalavra` devolve os streaks por área de uma palavra (usado na quota de aprendidas
// pendentes do quebra-cabeça). Todos são injetados porque dependem de lógica que vive nos pacotes
// revisao/progresso e o Buscador não deve importar de volta esses pacotes.
func Novo(dic *dicionario.GerenciadorDicionario, frases *dicionario.GerenciadorFrases, cfg func() config.Config, decompor func(string) []PalavraTexto, temTracadoComponente func(string) bool, estatisticasPalavra func(string) map[string]int) *Buscador {
	return &Buscador{
		dicionario:           dic,
		frases:               frases,
		config:               cfg,
		decompor:             decompor,
		temTracadoComponente: temTracadoComponente,
		estatisticasPalavra:  estatisticasPalavra,
		mapaStatus:           make(map[string]string),
		mapaVisto:            make(map[string]bool),
		mapaVisualizacoes:    make(map[string]int),
		mapaFoco:             make(map[string]bool),
		historico:            make(map[string]bool),
		cacheEstatisticas:    make(map[string]map[string]int),
	}
}

// IniciarSessao reseta o histórico de deduplicação e define os mapas do vocabulário do usuário para
// uma nova sessão. mapaStatus indexa por caractere o status estudo/aprendido; mapaVisto marca os
// caracteres apenas "já vistos" (status visto, sem terem sido promovidos a estudo/aprendido), usado
// pelo pool de já vistas frequentes (ver PalavrasVistasFrequentes); mapaVisualizacoes traz as
// visualizações de OCR por caractere; foco é o grupo de foco vigente (palavras na grafia salva);
// metaStreak é a meta de acertos consecutivos que conclui uma área.
func (b *Buscador) IniciarSessao(mapaStatus map[string]string, mapaVisto map[string]bool, mapaVisualizacoes map[string]int, foco []string, metaStreak int) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.mapaStatus = mapaStatus
	b.mapaStatusPalavra = nil
	b.mapaVisto = mapaVisto
	b.mapaVistoPalavra = nil
	b.mapaVisualizacoes = mapaVisualizacoes
	b.focoSessao = foco
	b.mapaFoco = make(map[string]bool, len(foco))
	for _, palavra := range foco {
		b.mapaFoco[palavra] = true
	}
	b.metaStreak = metaStreak
	b.temaPreferido = ""
	b.dificuldadeAlvo = ""
	b.temaExigido = ""
	b.dificuldadeExigida = ""
	b.escadaEstendida = nil
	b.dificuldadeEmprestada = nil
	b.variantesVetadas = nil
	b.universoRestrito = nil
	b.universoRestritoLista = nil
	b.historico = make(map[string]bool)
	b.cacheEstatisticas = make(map[string]map[string]int)
}

// ObterMapaStatus devolve o mapa de status da sessão corrente.
func (b *Buscador) ObterMapaStatus() map[string]string {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.mapaStatus
}

// DefinirGerenciadorCompreensao define o repositório de perguntas de compreensão no Buscador.
func (b *Buscador) DefinirGerenciadorCompreensao(comp *dicionario.GerenciadorCompreensao) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.compreensao = comp
}

// DefinirMapasPalavra registra os mapas de status e vistos por PALAVRA INTEIRA para validação de vocabulário.
func (b *Buscador) DefinirMapasPalavra(mapaStatusPalavra map[string]string, mapaVistoPalavra map[string]bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.mapaStatusPalavra = mapaStatusPalavra
	b.mapaVistoPalavra = mapaVistoPalavra
}

// DefinirUniversoRestrito limita as fontes de palavras da sessão à lista dada (ver universoRestrito).
// Lista vazia devolve a busca ao dicionário inteiro. Chamar depois de IniciarSessao, que zera.
func (b *Buscador) DefinirUniversoRestrito(palavras []string) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if len(palavras) == 0 {
		b.universoRestrito = nil
		b.universoRestritoLista = nil
		return
	}

	b.universoRestrito = make(map[string]bool, len(palavras))
	b.universoRestritoLista = make([]string, 0, len(palavras))
	for _, palavra := range palavras {
		if b.universoRestrito[palavra] {
			continue
		}
		b.universoRestrito[palavra] = true
		b.universoRestritoLista = append(b.universoRestritoLista, palavra)
	}
}

// palavrasDisponiveis é a fonte de palavras das buscas por distrator: o universo restrito da sessão,
// quando houver, ou todas as palavras do dicionário.
func (b *Buscador) palavrasDisponiveis() []string {
	b.mu.RLock()
	restrito := b.universoRestritoLista
	b.mu.RUnlock()
	if len(restrito) > 0 {
		return restrito
	}
	return b.dicionario.Banco.TodasPalavras()
}

// temUniversoRestrito informa se a sessão está fechada num conjunto de palavras.
func (b *Buscador) temUniversoRestrito() bool {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return len(b.universoRestrito) > 0
}

// DefinirTemaPreferido registra o tema que a sessão prefere nas frases (ver BuscarFraseIdeal). É um
// PREFERENCIAL: sem frase do tema, a busca segue com as demais candidatas. Chamar depois de
// IniciarSessao, que zera a preferência.
func (b *Buscador) DefinirTemaPreferido(tema string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.temaPreferido = tema
}

// DefinirFiltroFrases registra o tema e a dificuldade que o usuário escolheu no painel de configuração
// da revisão (ver BuscarFraseIdeal). Ao contrário de DefinirTemaPreferido, este filtro é OBRIGATÓRIO:
// frases que não casem são descartadas. Chamar depois de IniciarSessao, que zera o filtro.
func (b *Buscador) DefinirFiltroFrases(tema, dificuldade string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.temaExigido = tema
	b.dificuldadeExigida = dificuldade
}

// DefinirDificuldadeAlvo registra a categoria de dificuldade que a sessão prefere sortear nas variantes
// (ver SortearVarianteSessao). Vazio = sem preferência. Chamar depois de IniciarSessao, que zera o alvo.
func (b *Buscador) DefinirDificuldadeAlvo(dificuldade string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.dificuldadeAlvo = dificuldade
}

// DefinirVariantesVetadas registra as atividades que a sessão não quer sortear daqui em diante (ver
// variantesVetadas). Sem argumentos, limpa o veto. Chamar depois de IniciarSessao, que zera.
func (b *Buscador) DefinirVariantesVetadas(variantes ...string) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if len(variantes) == 0 {
		b.variantesVetadas = nil
		return
	}
	b.variantesVetadas = make(map[string]bool, len(variantes))
	for _, variante := range variantes {
		b.variantesVetadas[variante] = true
	}
}

// EstenderEscada empresta atividades de outros modos para a escada de dificuldade de `modo` nesta
// sessão (ver escadaEstendida): elas passam a ser sorteáveis ali, no degrau declarado. Chamar depois
// de IniciarSessao, que zera.
func (b *Buscador) EstenderEscada(modo string, variantes ...VarianteGraduada) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.escadaEstendida == nil {
		b.escadaEstendida = make(map[string][]string)
		b.dificuldadeEmprestada = make(map[string]string)
	}
	for _, graduada := range variantes {
		b.escadaEstendida[modo] = append(b.escadaEstendida[modo], graduada.Variante)
		b.dificuldadeEmprestada[graduada.Variante] = graduada.Dificuldade
	}
}

// VariantesExtras devolve as atividades que a sessão emprestou para a escada do modo (ver
// EstenderEscada). Fora dessas sessões é sempre vazio, e o modo sorteia só as variantes dele.
func (b *Buscador) VariantesExtras(modo string) []string {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.escadaEstendida[modo]
}

// DificuldadeDaVariante devolve o degrau da atividade NESTA sessão: o emprestado, quando a escada foi
// estendida, ou a classificação fixa de ObterDificuldadeVariante.
func (b *Buscador) DificuldadeDaVariante(variante string) string {
	b.mu.RLock()
	emprestada := b.dificuldadeEmprestada[variante]
	b.mu.RUnlock()
	if emprestada != "" {
		return emprestada
	}
	return ObterDificuldadeVariante(variante)
}

// SortearVarianteSessao aplica o estado da sessão às opções da atividade — primeiro o veto e o
// filtro de sub-atividades desativadas pelo usuário, depois o filtro por dificuldade — e sorteia uma variante.
func (b *Buscador) SortearVarianteSessao(opcoes ...string) string {
	b.mu.RLock()
	alvo := b.dificuldadeAlvo
	vetadas := b.variantesVetadas
	emprestadas := b.dificuldadeEmprestada
	var desativadas []string
	if b.config != nil {
		desativadas = b.config().AtividadesDesativadas
	}
	b.mu.RUnlock()

	dificuldadeDe := func(variante string) string {
		if emprestada := emprestadas[variante]; emprestada != "" {
			return emprestada
		}
		return ObterDificuldadeVariante(variante)
	}
	opcoesValidas := RemoverDesativadas(removerVetadas(opcoes, vetadas), desativadas)
	return SortearVariante(filtrarVariantesPorDificuldadeCom(alvo, opcoesValidas, dificuldadeDe)...)
}

// removerVetadas descarta as opções vetadas na sessão. Se o veto não deixar nenhuma opção de pé,
// devolve a lista original: vetar é uma preferência, não pode impedir a questão de existir.
func removerVetadas(opcoes []string, vetadas map[string]bool) []string {
	if len(vetadas) == 0 {
		return opcoes
	}
	permitidas := make([]string, 0, len(opcoes))
	for _, opcao := range opcoes {
		if !vetadas[opcao] {
			permitidas = append(permitidas, opcao)
		}
	}
	if len(permitidas) == 0 {
		return opcoes
	}
	return permitidas
}

// RemoverDesativadas descarta as sub-atividades desativadas pelo usuário nas configurações.
func RemoverDesativadas(opcoes []string, desativadas []string) []string {
	if len(desativadas) == 0 {
		return opcoes
	}
	mapa := make(map[string]bool, len(desativadas))
	for _, d := range desativadas {
		mapa[d] = true
	}
	permitidas := make([]string, 0, len(opcoes))
	for _, opcao := range opcoes {
		if !mapa[opcao] {
			permitidas = append(permitidas, opcao)
		}
	}
	if len(permitidas) == 0 {
		return opcoes
	}
	return permitidas
}

// streaksDaPalavra devolve (com cache por sessão) os streaks por área da palavra via a função de
// estatísticas injetada. Sem função injetada (testes sem banco), devolve mapa vazio — nesse caso
// nenhuma área conta como concluída, o que mantém a palavra elegível na dúvida.
func (b *Buscador) streaksDaPalavra(palavra string) map[string]int {
	b.mu.RLock()
	streaks, ok := b.cacheEstatisticas[palavra]
	b.mu.RUnlock()
	if ok {
		return streaks
	}

	streaks = map[string]int{}
	if b.estatisticasPalavra != nil {
		if s := b.estatisticasPalavra(palavra); s != nil {
			streaks = s
		}
	}

	b.mu.Lock()
	b.cacheEstatisticas[palavra] = streaks
	b.mu.Unlock()
	return streaks
}

// RegistrarSeInedita confere a assinatura contra o histórico da sessão e a marca como usada. Devolve
// false se a assinatura já havia sido registrada antes (o chamador tenta outra alternativa).
func (b *Buscador) RegistrarSeInedita(assinatura string) bool {
	b.mu.RLock()
	usada := b.historico[assinatura]
	b.mu.RUnlock()
	if usada {
		return false
	}
	b.mu.Lock()
	b.historico[assinatura] = true
	b.mu.Unlock()
	return true
}

// HistoricoContem verifica se a assinatura já foi registrada no histórico da sessão.
func (b *Buscador) HistoricoContem(assinatura string) bool {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.historico[assinatura]
}

// JaEsteveEmQuebraCabecaNaSessao verifica se o caractere já participou de um tabuleiro de quebra-cabeça na sessão.
func (b *Buscador) JaEsteveEmQuebraCabecaNaSessao(caractere string) bool {
	b.mu.RLock()
	defer b.mu.RUnlock()
	for _, v := range VariantesQuebraCabeca {
		if b.historico[v+":"+caractere] {
			return true
		}
	}
	return false
}

