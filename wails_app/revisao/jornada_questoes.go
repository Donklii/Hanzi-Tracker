package revisao

import (
	"fmt"
	"math/rand/v2"

	"wails_app/dicionario"
	"wails_app/revisao/jornada"
)

// ----- Seção: Questões da Jornada -----
//
// A Jornada não tem gerador próprio: ela escolhe QUAIS palavras e em QUAL modo, e delega a montagem
// ao mesmo montarQuestao das demais sessões. O que muda é a origem dos alvos (as palavras do nível)
// e, principalmente, o UNIVERSO da sessão: alvos, distratores e peças de quebra-cabeça saem apenas
// do que a própria Jornada já ensinou até aquele nível (ver jornada.PalavrasAteNivel). O modo é
// DESCONECTADO das outras revisões — o vocabulário do usuário, o status "em estudo" e o grupo de
// foco não têm nenhuma influência aqui (ver prepararSessaoJornada). A dificuldade das atividades é
// progressiva por exposição da palavra no nível (escada introdução → avançado).

const (
	AparicoesPorPalavraJornada = 3
	QuestoesMinRevisaoJornada  = 8
	QuestoesMaxRevisaoJornada  = 16
)

var escadaDificuldadeJornada = []string{
	DificuldadeIntroducao,
	DificuldadeIniciante,
	DificuldadeIntermediario,
	DificuldadeAvancado,
}

var modosComVarianteIntroducao = map[string]bool{
	ModoSignificado: true,
	ModoFonetica:    true,
}

func totalQuestoesJornada(numPalavras int) int {
	total := numPalavras * AparicoesPorPalavraJornada
	if total < QuestoesMinRevisaoJornada {
		return QuestoesMinRevisaoJornada
	}
	if total > QuestoesMaxRevisaoJornada {
		return QuestoesMaxRevisaoJornada
	}
	return total
}

func dificuldadeDaExposicao(exposicao int) string {
	if exposicao < 0 {
		exposicao = 0
	}
	if exposicao >= len(escadaDificuldadeJornada) {
		exposicao = len(escadaDificuldadeJornada) - 1
	}
	return escadaDificuldadeJornada[exposicao]
}

// RevisoesEfetivas devolve as revisões do nível com os tipos que dependem de motor substituídos
// quando o motor está desligado: sem TTS, `fonetica` vira `palavras`; sem STT, `pronuncia` vira
// `palavras`. O COMPRIMENTO da lista é preservado, então os índices já salvos em jornada_progresso
// continuam válidos quando o usuário instalar o motor. É a fonte única dessa substituição — o
// binding da árvore e a geração de questões passam os dois por aqui.
func (r *GerenciadorRevisao) RevisoesEfetivas(nivel jornada.Nivel) []string {
	cfg := r.Config()
	temTts := cfg.MotorTtsAtivo != "" && cfg.MotorTtsAtivo != "nenhum"
	temStt := cfg.MotorSttAtivo != "" && cfg.MotorSttAtivo != "nenhum"

	efetivas := make([]string, len(nivel.Revisoes))
	for i, tipo := range nivel.Revisoes {
		switch {
		case tipo == jornada.RevisaoFonetica && !temTts:
			efetivas[i] = jornada.RevisaoPalavras
		case tipo == jornada.RevisaoPronuncia && !temStt:
			efetivas[i] = jornada.RevisaoPalavras
		default:
			efetivas[i] = tipo
		}
	}
	return efetivas
}

// ObterPrimeiraQuestaoJornada monta apenas a primeira questão da sessão da Jornada (para exibição em <10ms do skeleton correto).
func (r *GerenciadorRevisao) ObterPrimeiraQuestaoJornada(nivelId string, revIndex int) (*QuestaoRevisao, error) {
	questoes, err := r.ObterQuestoesJornada(nivelId, revIndex)
	if err != nil || len(questoes) == 0 {
		return nil, err
	}
	return &questoes[0], nil
}

// ObterQuestoesJornada monta a mini-sessão `revIndex` do nível: de 8 a 16 questões (por totalQuestoesJornada)
// com as palavras do nível, no(s) modo(s) do tipo daquela revisão.
func (r *GerenciadorRevisao) ObterQuestoesJornada(nivelId string, revIndex int) ([]QuestaoRevisao, error) {
	return r.ObterQuestoesJornadaComPrimeira(nivelId, revIndex, QuestaoRevisao{})
}

// ObterQuestoesJornadaComPrimeira monta a mini-sessão da Jornada preservando a primeira questão já gerada (para o skeleton).
func (r *GerenciadorRevisao) ObterQuestoesJornadaComPrimeira(nivelId string, revIndex int, primeira QuestaoRevisao) ([]QuestaoRevisao, error) {
	nivel, ramo, err := jornada.ObterNivel(nivelId)
	if err != nil {
		return nil, err
	}

	revisoes := r.RevisoesEfetivas(nivel)
	if revIndex < 0 || revIndex >= len(revisoes) {
		return nil, fmt.Errorf("revisão %d fora do nível %q (que tem %d revisões)", revIndex, nivelId, len(revisoes))
	}

	universo, err := jornada.PalavrasAteNivel(nivelId)
	if err != nil {
		return nil, err
	}

	mapaStatus := r.prepararSessaoJornada(universo)

	// As frases do ramo temático puxam para o tema dele (preferencial: sem frase do tema, a busca
	// segue com as demais).
	r.buscador.DefinirTemaPreferido(ramo.Tema)

	tipos := tiposDaRevisaoJornada(revisoes, revIndex)
	sessao := &sessaoJornada{
		gerenciador: r,
		nivelId:     nivelId,
		revIndex:    revIndex,
		revisaoTipo: revisoes[revIndex],
		palavras:    embaralharPalavras(nivel.Palavras),
		aparicoes:   make(map[string]int),
		noTabuleiro: make(map[string]bool),
		ultima:      make(map[string]string),
		universo:    universo,
		mapaStatus:  mapaStatus,
		candidatos:  make(map[string][]dicionario.DecomposicaoHanzi),
		pools:       make(map[string]poolsRevisao),
	}

	total := totalQuestoesJornada(len(nivel.Palavras))
	questoes := make([]QuestaoRevisao, 0, total)
	if primeira.Variante != "" {
		questoes = append(questoes, primeira)
	}
	for len(questoes) < total {
		modo := modoDaQuestaoJornada(tipos, len(questoes))

		questao, montou := sessao.montarProximaQuestao(modo)
		if !montou && modo != ModoSignificado {
			// Nenhuma palavra do nível rendeu no modo da vez (ex.: a ordenação exige frase quase toda
			// conhecida, o que raramente vale no começo da Jornada): a questão sai no significado em
			// vez de a mini-sessão encolher.
			questao, montou = sessao.montarProximaQuestao(ModoSignificado)
		}
		if !montou {
			break
		}
		questoes = append(questoes, questao)
	}

	if len(questoes) == 0 {
		return nil, fmt.Errorf("não foi possível montar questões para a revisão %d do nível %q", revIndex, nivelId)
	}
	return questoes, nil
}

// ----- Seção: Montagem das Questões do Nível -----

// prepararSessaoJornada inicia a sessão do buscador com o vocabulário da JORNADA no lugar do
// vocabulário do usuário: o que o caminho já ensinou até o nível conta como conhecido, não há grupo
// de foco e nenhuma palavra entra como "em estudo". Também fecha as fontes de distratores nesse
// universo (DefinirUniversoRestrito). Devolve o mapaStatus por caractere que a busca de frases usa
// para medir cobertura — aqui, "conhecido" = "a Jornada já ensinou".
func (r *GerenciadorRevisao) prepararSessaoJornada(universo []string) map[string]string {
	mapaStatus := make(map[string]string, len(universo)*2)
	for _, palavra := range universo {
		for _, componente := range hanzisComponentes(palavra) {
			mapaStatus[componente] = dicionario.StatusAprendido
		}
	}

	// Sem mapaVisto, sem visualizações de OCR e sem foco: os pools de "já vistas frequentes" e as
	// quotas do quebra-cabeça ficam vazios, e todo distrator passa a vir do universo da Jornada.
	r.buscador.IniciarSessao(mapaStatus, map[string]bool{}, map[string]int{}, nil, MetaAcertosConsecutivos)
	r.buscador.DefinirUniversoRestrito(universo)

	// Completar a lacuna da frase é, na Jornada, o degrau AVANÇADO do significado: exige a palavra
	// certa dentro de um contexto, não só o par hanzi↔glosa. Sem esse empréstimo a escada do
	// significado acaba em "iniciante" e toda aparição a partir da segunda desabava no quebra-cabeça.
	r.buscador.EstenderEscada(ModoSignificado, varianteGraduada{Variante: VarianteContexto, Dificuldade: DificuldadeAvancado})

	return mapaStatus
}

// sessaoJornada carrega o estado de montagem de UMA mini-sessão: as palavras do nível na ordem
// sorteada, o cursor cíclico sobre elas, o universo fechado da Jornada e os caches por modo (a
// varredura do dicionário é cara demais para repetir a cada tentativa).
type sessaoJornada struct {
	gerenciador *GerenciadorRevisao
	nivelId     string
	revIndex    int
	revisaoTipo string
	palavras    []string
	cursor      int
	aparicoes   map[string]int
	noTabuleiro map[string]bool
	ultima      map[string]string
	universo    []string
	mapaStatus  map[string]string
	candidatos  map[string][]dicionario.DecomposicaoHanzi
	pools       map[string]poolsRevisao
}

// chaveTabuleiro identifica "esta palavra já esteve num tabuleiro desta atividade" (ver noTabuleiro).
func chaveTabuleiro(variante, palavra string) string {
	return variante + ":" + palavra
}

// vetosDaPalavra lista as atividades que a palavra não deve repetir na questão da vez:
//
//   - a última que ela praticou, para duas aparições seguidas não saírem na mesma atividade — sem
//     isso, o degrau em que a escada satura ("avançado", de longe o mais frequente) entregaria
//     sempre a mesma coisa, que é como o quebra-cabeça dominava antes;
//   - os tabuleiros em que ela já entrou, porque ali a decisão é com VÁRIAS palavras de uma vez:
//     quem foi peça já treinou o pareamento, e num universo fechado (4-5 palavras do nível mais as
//     herdadas) o segundo tabuleiro devolveria quase as mesmas peças.
//
// É preferência, não regra: vetar tudo o que o modo oferece é ignorado pelo Buscador.
func (s *sessaoJornada) vetosDaPalavra(palavra string) []string {
	var vetadas []string
	if ultima := s.ultima[palavra]; ultima != "" {
		vetadas = append(vetadas, ultima)
	}

	temNoTabuleiro := func(v string) bool {
		return s.noTabuleiro[chaveTabuleiro(v, palavra)]
	}

	esteveEmSignificado := temNoTabuleiro(VarianteQuebraCabecaSignificado) ||
		temNoTabuleiro(VarianteQuebraCabecaTrio) ||
		s.ultima[palavra] == VarianteQuebraCabecaSignificado ||
		s.ultima[palavra] == VarianteQuebraCabecaTrio

	if esteveEmSignificado {
		for _, v := range []string{VarianteQuebraCabecaSignificado, VarianteQuebraCabecaTrio} {
			if !contemString(vetadas, v) {
				vetadas = append(vetadas, v)
			}
		}
	}

	if temNoTabuleiro(VarianteQuebraCabecaFonetica) || s.ultima[palavra] == VarianteQuebraCabecaFonetica {
		if !contemString(vetadas, VarianteQuebraCabecaFonetica) {
			vetadas = append(vetadas, VarianteQuebraCabecaFonetica)
		}
	}

	return vetadas
}

func contemString(lista []string, item string) bool {
	for _, v := range lista {
		if v == item {
			return true
		}
	}
	return false
}

// registrarQuestao contabiliza a questão montada. Numa atividade de tabuleiro TODAS as peças são
// pares corretos — a questão pratica cada uma delas, não só o alvo —, então todas contam aparição
// (a escada de dificuldade delas avança junto) e ficam marcadas como já vistas naquele tabuleiro.
func (s *sessaoJornada) registrarQuestao(questao QuestaoRevisao, alvo string) {
	s.ultima[alvo] = questao.Variante

	if !ehQuebraCabeca(questao.Variante) {
		s.aparicoes[alvo]++
		return
	}

	doNivel := make(map[string]bool, len(s.palavras))
	for _, palavra := range s.palavras {
		doNivel[palavra] = true
	}

	for _, opcao := range questao.Opcoes {
		s.noTabuleiro[chaveTabuleiro(questao.Variante, opcao.Hanzi)] = true
		s.ultima[opcao.Hanzi] = questao.Variante
		if doNivel[opcao.Hanzi] {
			s.aparicoes[opcao.Hanzi]++
		}
	}
	// O alvo entra pelas peças; se por algum motivo não estiver entre elas, a aparição dele não pode
	// se perder — sem isso o cursor voltaria a ele na mesma dificuldade indefinidamente.
	if !doNivel[alvo] || s.aparicoes[alvo] == 0 {
		s.aparicoes[alvo]++
	}
}

// montarProximaQuestao tenta montar uma questão no modo pedido, percorrendo as palavras do nível a
// partir do cursor (uma volta completa no máximo). Falha de montagem num alvo — ex.: sem frase
// inédita para a palavra — apenas passa para o próximo.
func (s *sessaoJornada) montarProximaQuestao(modo string) (QuestaoRevisao, bool) {
	r := s.gerenciador

	for tentativa := 0; tentativa < len(s.palavras); tentativa++ {
		palavra := s.palavras[s.cursor%len(s.palavras)]
		s.cursor++

		entrada := r.entradaFoco(palavra)
		if entrada == nil {
			fmt.Printf("Aviso: palavra %q do nível %q não tem leitura no dicionário; pulando\n", palavra, s.nivelId)
			continue
		}

		// emEstudo/emFoco ficam FALSOS de propósito: na Jornada o alvo vale pelo lugar dele na
		// árvore, não pelo que o usuário marcou nas outras revisões.
		alvo := alvoRevisao{entrada: *entrada}

		exposicao := s.revIndex + s.aparicoes[palavra]
		modoDoAlvo := modo
		if exposicao == 0 && !modosComVarianteIntroducao[modo] {
			modoDoAlvo = ModoSignificado
		}

		// Alvo sem insumo para o modo (ex.: palavra sem traçado no desenho) cai para o significado,
		// que só depende da leitura do dicionário — a questão sai, ainda que noutra atividade.
		if !r.alvoElegivelNoModo(alvo, modoDoAlvo, s.candidatosDoModo(modoDoAlvo)) {
			modoDoAlvo = ModoSignificado
		}

		sessaoModo := modoDoAlvo
		if s.revisaoTipo == jornada.RevisaoMista {
			sessaoModo = jornada.RevisaoMista
		}

		r.buscador.DefinirDificuldadeAlvo(dificuldadeDaExposicao(exposicao))
		r.buscador.DefinirVariantesVetadas(s.vetosDaPalavra(palavra)...)

		questao, err := r.montarQuestao(modoDoAlvo, alvo, s.poolsDoModo(modoDoAlvo), sessaoModo, "")
		if err != nil {
			continue
		}
		s.registrarQuestao(questao, palavra)
		return questao, true
	}

	return QuestaoRevisao{}, false
}

func (s *sessaoJornada) candidatosDoModo(modo string) []dicionario.DecomposicaoHanzi {
	if candidatos, ok := s.candidatos[modo]; ok {
		return candidatos
	}
	candidatos := s.gerenciador.candidatosParaModo(modo)
	s.candidatos[modo] = candidatos
	return candidatos
}

// poolsDoModo monta os pools da Jornada: Todos é o UNIVERSO do nível (nada de banco geral), e as
// listas por status do usuário ficam vazias — é isso que impede uma palavra "em estudo" ou do grupo
// de foco de entrar como distrator/peça, já que todas as fontes com quota do quebra-cabeça leem
// justamente EmEstudo/Aprendidos/Vistas e o grupo de foco (vazio nesta sessão).
func (s *sessaoJornada) poolsDoModo(modo string) poolsRevisao {
	if pools, ok := s.pools[modo]; ok {
		return pools
	}

	// A entrada do banco de candidatos do modo é mais rica (traz decomposição); só as palavras fora
	// dele — tipicamente as multi-hanzi — entram pela leitura do dicionário.
	porCaractere := make(map[string]dicionario.DecomposicaoHanzi)
	for _, candidato := range s.candidatosDoModo(modo) {
		porCaractere[candidato.Caractere] = candidato
	}

	entradas := make([]dicionario.DecomposicaoHanzi, 0, len(s.universo))
	for _, palavra := range s.universo {
		if entrada, existe := porCaractere[palavra]; existe {
			entradas = append(entradas, entrada)
			continue
		}
		if entrada := s.gerenciador.entradaFoco(palavra); entrada != nil {
			entradas = append(entradas, *entrada)
		}
	}

	pools := poolsRevisao{Todos: entradas}
	s.pools[modo] = pools
	return pools
}

// ----- Seção: Tipo de Conteúdo → Modo Real -----

// tiposDaRevisaoJornada devolve os tipos de conteúdo que a revisão `revIndex` do nível pratica: um só
// para as revisões comuns; para a `mista` (sempre a última), todos os tipos anteriores do próprio
// nível, que ela cicla. Nível cuja única revisão é a mista pratica as palavras.
func tiposDaRevisaoJornada(revisoes []string, revIndex int) []string {
	if revisoes[revIndex] != jornada.RevisaoMista {
		return []string{revisoes[revIndex]}
	}
	anteriores := revisoes[:revIndex]
	if len(anteriores) == 0 {
		return []string{jornada.RevisaoPalavras}
	}
	return anteriores
}

// modoDaQuestaoJornada traduz o tipo de conteúdo da questão `indice` no modo real da revisão. O tipo
// `frases` alterna entre contexto e ordenação, para a mini-sessão não repetir a mesma atividade.
func modoDaQuestaoJornada(tipos []string, indice int) string {
	switch tipos[indice%len(tipos)] {
	case jornada.RevisaoFonetica:
		return ModoFonetica
	case jornada.RevisaoDesenho:
		return ModoDesenho
	case jornada.RevisaoPronuncia:
		return ModoPronuncia
	case jornada.RevisaoFrases:
		return ModoContexto
	default:
		return ModoSignificado
	}
}

func embaralharPalavras(palavras []string) []string {
	embaralhadas := make([]string, len(palavras))
	copy(embaralhadas, palavras)
	rand.Shuffle(len(embaralhadas), func(i, j int) {
		embaralhadas[i], embaralhadas[j] = embaralhadas[j], embaralhadas[i]
	})
	return embaralhadas
}
