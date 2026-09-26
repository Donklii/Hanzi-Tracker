package revisao

import (
	"fmt"
	"math"
	"math/rand/v2"
	"sort"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"wails_app/config"
	"wails_app/dicionario"
	"wails_app/progresso"
	"wails_app/revisao/busca"
	"wails_app/segmentacao"
)

// ----- Seção: Constantes e Configurações -----

const (
	QuestoesPorSessaoPadrao = 10
	QuestoesPorSessaoMaximo = 50
	MetaAcertosConsecutivos = 3

	// chanceRepetirAlvoNoGeral é a fração das questões da revisão geral que repete um alvo já
	// praticado na sessão, noutra atividade (reforço espaçado dentro da própria sessão).
	chanceRepetirAlvoNoGeral = 0.2
)

// ----- Seção: Estruturas de Dados -----

type QuestaoRevisao struct {
	Modo     string `json:"modo"`
	Variante string `json:"variante"`
	Hanzi    string `json:"hanzi"`
	// PalavraFoco é a palavra do grupo de foco que originou a questão (o "principal" da atividade).
	// Para a maioria dos modos coincide com Hanzi; no desenho de palavra multi-hanzi, Hanzi vira o
	// hanzi componente desenhado, enquanto PalavraFoco continua sendo a palavra inteira — é ela que
	// contabiliza a conclusão do aprendizado.
	PalavraFoco string `json:"palavraFoco"`
	Pinyin      string `json:"pinyin"`
	Definicao   string `json:"definicao"`
	// ComponenteAlvo e TracosAlvo só existem na VarianteDesenhoComponente: o componente da
	// decomposição que o usuário redesenha e os índices dos traços dele dentro do caractere inteiro
	// (posições em "strokes"/"medians" do que ObterDadosEscritaHanzi(Hanzi) devolve).
	ComponenteAlvo string `json:"componenteAlvo"`
	TracosAlvo     []int  `json:"tracosAlvo"`
	// ComponentesMontagem e DistratoresMontagem só existem na VarianteDesenhoMontagem: a partição
	// COMPLETA dos traços do caractere pelas folhas da decomposição (ver dicionario.ParticaoEmComponentes)
	// — cada folha vira uma peça que o usuário arrasta ao lugar original — e alguns caracteres-componente
	// falsos, parecidos com os reais, embaralhados junto como distratores.
	ComponentesMontagem     []dicionario.ComponenteHanzi `json:"componentesMontagem"`
	DistratoresMontagem     []string                     `json:"distratoresMontagem"`
	EmEstudo                bool                         `json:"emEstudo"`
	EmFoco                  bool                         `json:"emFoco"`
	FraseLacuna             string                       `json:"fraseLacuna"`
	FraseOriginal           string                       `json:"fraseOriginal"`
	FraseOculta             string                       `json:"fraseOculta"`
	FraseLacunaSegmentada         []PalavraRevisao             `json:"fraseLacunaSegmentada"`
	FraseOriginalSegmentada       []PalavraRevisao             `json:"fraseOriginalSegmentada"`
	FraseOriginalLacunaSegmentada []PalavraRevisao             `json:"fraseOriginalLacunaSegmentada,omitempty"`
	FraseTraducao           string                       `json:"fraseTraducao"`
	FraseAtribuicao         string                       `json:"fraseAtribuicao"`
	FraseTema               string                       `json:"fraseTema"`
	FraseDificuldade        string                       `json:"fraseDificuldade"`
	Dificuldade             string                       `json:"dificuldade"`
	Opcoes                  []OpcaoRevisao               `json:"opcoes"`
	PilhaOrdenacao          []OpcaoRevisao               `json:"pilhaOrdenacao"`
	PecasEsperadas          []string                     `json:"pecasEsperadas"`
	ElementosOrdenacao      []ElementoOrdenacao          `json:"elementosOrdenacao"`
	PerguntaCompreensao           string                       `json:"perguntaCompreensao,omitempty"`
	PerguntaCompreensaoSegmentada []PalavraRevisao             `json:"perguntaCompreensaoSegmentada,omitempty"`
	PerguntaTraduzida             string                       `json:"perguntaTraduzida,omitempty"`
	ContextoTraduzido       string                       `json:"contextoTraduzido,omitempty"`
	IndiceRespostaCorreta   int                          `json:"indiceRespostaCorreta,omitempty"`
}

type PalavraRevisao struct {
	Texto        string   `json:"texto"`
	Pinyin       string   `json:"pinyin"`
	Significados []string `json:"significados"`
	EhChines     bool     `json:"ehChines"`
	EhLacuna     bool     `json:"ehLacuna,omitempty"`
	EhNaoVista   bool     `json:"ehNaoVista,omitempty"`
}

type alvoRevisao struct {
	entrada  dicionario.DecomposicaoHanzi
	emEstudo bool
	emFoco   bool
}

type GerenciadorRevisao struct {
	Config      func() config.Config
	Dicionario  *dicionario.GerenciadorDicionario
	Frases      *dicionario.GerenciadorFrases
	Compreensao *dicionario.GerenciadorCompreensao
	buscador    *busca.Buscador
}

// ----- Seção: Inicialização -----

func NovoGerenciadorRevisao(dic *dicionario.GerenciadorDicionario, frases *dicionario.GerenciadorFrases, cfg func() config.Config) *GerenciadorRevisao {
	decompor := func(texto string) []busca.PalavraTexto {
		segmentos := decomporTexto(dic, texto)
		convertido := make([]busca.PalavraTexto, len(segmentos))
		for i, s := range segmentos {
			convertido[i] = busca.PalavraTexto{Texto: s.Texto, EhChines: s.EhChines}
		}
		return convertido
	}
	temTracadoComponente := func(palavra string) bool {
		for _, h := range hanzisComponentes(palavra) {
			if dic.Tracados.Tem(h) {
				return true
			}
		}
		return false
	}
	estatisticasPalavra := func(palavra string) map[string]int {
		estatisticas, err := progresso.ObterEstatisticasPalavra(palavra)
		if err != nil || estatisticas == nil {
			return map[string]int{}
		}
		return estatisticas
	}

	return &GerenciadorRevisao{
		Config:     cfg,
		Dicionario: dic,
		Frases:     frases,
		buscador:   busca.Novo(dic, frases, cfg, decompor, temTracadoComponente, estatisticasPalavra),
	}
}

// DefinirGerenciadorCompreensao associa o repositório de perguntas de compreensão ao gerenciador de revisão.
func (r *GerenciadorRevisao) DefinirGerenciadorCompreensao(comp *dicionario.GerenciadorCompreensao) {
	r.Compreensao = comp
	if r.buscador != nil {
		r.buscador.DefinirGerenciadorCompreensao(comp)
	}
}

// ----- Seção: Core da Revisão -----

// ObterDadosEscritaHanzi devolve o JSON de traçados do caractere para o Hanzi Writer.
func (r *GerenciadorRevisao) ObterDadosEscritaHanzi(caractere string) (string, error) {
	dados, existe := r.Dicionario.Tracados.Dados(caractere)
	if !existe {
		return "", fmt.Errorf("não há dados de traçado para %q", caractere)
	}
	return dados, nil
}

// prepararSessaoRevisao lê o vocabulário do usuário, monta os mapas da sessão e inicia a sessão no
// buscador. Devolve o mapaStatus por CARACTERE (para a seleção de alvos e os pools), o mapaStatus por
// PALAVRA INTEIRA (para alvos multi-hanzi que vêm prontos, como os da Jornada — indexar o mapa por
// caractere com uma chave de 2+ hanzi nunca bate) e o grupo de foco DOS ALVOS, vazio quando a
// priorização de estudo está desligada. Toda sessão de revisão (geral, por modo ou da Jornada) começa
// por aqui.
func (r *GerenciadorRevisao) prepararSessaoRevisao() (map[string]string, map[string]string, []string) {
	vocabulario, _ := progresso.GetAllVocab()

	// mapaStatus/mapaVisualizacoes indexam por CARACTERE na grafia de EXIBIÇÃO configurada, porque é
	// contra os candidatos sorteados do dicionário (que candidatosParaModo já traz nessa grafia) que
	// eles são cruzados na seleção de alvos. NÃO altera o vocabulário: o foco opera na grafia SALVA de
	// cada item — a identidade (ver bindings_estudo.GetVocab) — para o progresso não fundir as grafias.
	tipoExibicao := r.Config().TipoHanziExibicao
	converterParaExibicao := func(texto string) string {
		if tipoExibicao == "simplificado" || tipoExibicao == "tradicional" {
			return r.Dicionario.ConverterTexto(texto, tipoExibicao)
		}
		return texto
	}

	mapaStatus := make(map[string]string)
	mapaStatusPalavra := make(map[string]string, len(vocabulario))
	mapaVisualizacoes := make(map[string]int)
	for _, v := range vocabulario {
		palavraConvertida := converterParaExibicao(v.Hanzi)

		if v.Status == dicionario.StatusEstudo {
			mapaStatusPalavra[palavraConvertida] = dicionario.StatusEstudo
		} else if v.Status == dicionario.StatusAprendido && mapaStatusPalavra[palavraConvertida] != dicionario.StatusEstudo {
			mapaStatusPalavra[palavraConvertida] = dicionario.StatusAprendido
		}

		for _, caractereRuna := range palavraConvertida {
			ch := string(caractereRuna)
			if _, ehAbrev := dicionario.MapaAbrevParaCompleto[ch]; ehAbrev {
				continue
			}
			if v.Status == dicionario.StatusEstudo {
				mapaStatus[ch] = dicionario.StatusEstudo
			} else if v.Status == dicionario.StatusAprendido && mapaStatus[ch] != dicionario.StatusEstudo {
				mapaStatus[ch] = dicionario.StatusAprendido
			}

			if v.VezesVistaOcr > mapaVisualizacoes[ch] {
				mapaVisualizacoes[ch] = v.VezesVistaOcr
			}
		}
	}

	// mapaVisto marca os caracteres apenas "já vistos" (status visto e ainda NÃO promovidos a
	// estudo/aprendido) — a flag do pool de já vistas frequentes (ver busca.PalavrasVistasFrequentes).
	// Segundo passo, depois de mapaStatus completo, para respeitar a precedência estudo/aprendido > visto.
	mapaVisto := make(map[string]bool)
	for _, v := range vocabulario {
		if v.Status != dicionario.StatusVisto {
			continue
		}
		for _, caractereRuna := range converterParaExibicao(v.Hanzi) {
			ch := string(caractereRuna)
			if _, ehAbrev := dicionario.MapaAbrevParaCompleto[ch]; ehAbrev {
				continue
			}
			if mapaStatus[ch] == "" {
				mapaVisto[ch] = true
			}
		}
	}

	// O grupo de foco alimenta duas coisas: a seleção de alvos (só quando a priorização de estudo
	// está ativa) e a quota de peças "em foco" do quebra-cabeça (sempre). Com a priorização
	// desligada, o grupo persistido é apenas LIDO — sem sincronizar/rotacionar — para não reativar
	// os efeitos que o usuário desligou; nesse caso `foco` (dos alvos) permanece vazio.
	var foco, focoSessao []string
	if r.Config().PriorizarEstudoRevisao {
		foco, _ = r.sincronizarFocoRevisao(vocabulario)
		focoSessao = foco
	} else {
		focoSessao, _ = progresso.ObterFoco()
	}

	r.buscador.IniciarSessao(mapaStatus, mapaVisto, mapaVisualizacoes, focoSessao, MetaAcertosConsecutivos)
	r.buscador.DefinirMapasPalavra(mapaStatusPalavra, nil)

	return mapaStatus, mapaStatusPalavra, foco
}

// ObterPrimeiraQuestaoRevisao monta rapidamente apenas a primeira questão da sessão no modo solicitado (para exibição em <10ms do skeleton correto).
func (r *GerenciadorRevisao) ObterPrimeiraQuestaoRevisao(modo string) (*QuestaoRevisao, error) {
	questoes, err := r.ObterQuestoesRevisao(modo, 1)
	if err != nil || len(questoes) == 0 {
		return nil, err
	}
	return &questoes[0], nil
}

// ObterQuestoesRevisao monta uma sessão de revisão no modo solicitado.
func (r *GerenciadorRevisao) ObterQuestoesRevisao(modo string, quantidade int) ([]QuestaoRevisao, error) {
	return r.ObterQuestoesRevisaoComPrimeira(modo, quantidade, QuestaoRevisao{})
}

// ObterQuestoesRevisaoComPrimeira monta uma sessão de revisão preservando a primeira questão já gerada (para o skeleton).
func (r *GerenciadorRevisao) ObterQuestoesRevisaoComPrimeira(modo string, quantidade int, primeira QuestaoRevisao) ([]QuestaoRevisao, error) {
	varianteForcada := ""
	modoBase := modo
	if strings.HasPrefix(modo, "exemplo:") {
		varianteForcada = strings.TrimPrefix(modo, "exemplo:")
		modoBase = busca.VarianteParaModoBase(varianteForcada)
		if modoBase == "" {
			modoBase = ModoSignificado
		}
	} else if vBase := busca.VarianteParaModoBase(modo); vBase != "" && modo != vBase && modo != ModoGeral && modo != ModoContexto {
		varianteForcada = modo
		modoBase = vBase
	}

	if modoBase == ModoFonetica && (r.Config().MotorTtsAtivo == "" || r.Config().MotorTtsAtivo == "nenhum") {
		return nil, fmt.Errorf("o modo fonética requer um motor TTS ativo nas configurações")
	}
	if modoBase == ModoPronuncia && (r.Config().MotorSttAtivo == "" || r.Config().MotorSttAtivo == "nenhum") {
		return nil, fmt.Errorf("o modo pronúncia requer um motor de reconhecimento de fala ativo nas configurações")
	}

	if varianteForcada == "" {
		if quantidade <= 0 || quantidade == QuestoesPorSessaoPadrao {
			if cfgQtd := r.Config().RevisaoQuantidadeQuestoes; cfgQtd >= 8 {
				quantidade = cfgQtd
			}
		}
	}

	if quantidade <= 0 {
		quantidade = QuestoesPorSessaoPadrao
	}
	if quantidade > QuestoesPorSessaoMaximo {
		quantidade = QuestoesPorSessaoMaximo
	}

	mapaStatus, _, foco := r.prepararSessaoRevisao()
	// Filtro de tema, dificuldade das atividades e frases escolhidos pelo usuário no painel de configuração da revisão.
	r.buscador.DefinirDificuldadeAlvo(r.Config().RevisaoFiltroDificuldade)
	r.buscador.DefinirFiltroFrases(r.Config().RevisaoFiltroTema, r.Config().RevisaoFiltroDificuldade)

	candidatos := r.candidatosParaModo(modoBase)
	if len(candidatos) < TotalOpcoesMultiplaEscolha {
		return nil, fmt.Errorf("não há caracteres suficientes no dicionário para o modo %q", modo)
	}

	pools := r.poolsDoModo(candidatos, mapaStatus)

	alvos := r.selecionarAlvos(candidatos, foco, mapaStatus, quantidade)

	var poolsPorModo map[string]poolsRevisao
	var modosPermitidos []string

	if modoBase == ModoGeral {
		modosPermitidos = r.modosPermitidosRevisao()
		poolsPorModo = make(map[string]poolsRevisao, len(modosPermitidos))
		for _, m := range modosPermitidos {
			poolsPorModo[m] = r.poolsDoModo(r.candidatosParaModo(m), mapaStatus)
		}
	}

	cacheEstatisticas := make(map[string]map[string]int)
	questoes := make([]QuestaoRevisao, 0, quantidade)
	if primeira.Variante != "" {
		questoes = append(questoes, primeira)
	}

	for _, alvo := range alvos {
		if len(questoes) >= quantidade {
			break
		}

		modoEvitar := ""
		if modoBase == ModoGeral && len(questoes) > 0 && rand.Float64() < chanceRepetirAlvoNoGeral {
			indicesFoco := make([]int, 0, len(questoes))
			for idx, q := range questoes {
				if q.EmFoco {
					indicesFoco = append(indicesFoco, idx)
				}
			}
			var qRef QuestaoRevisao
			if len(indicesFoco) > 0 {
				qRef = questoes[indicesFoco[rand.IntN(len(indicesFoco))]]
			} else {
				qRef = questoes[rand.IntN(len(questoes))]
			}

			var alvoRepetido alvoRevisao
			encontrou := false
			for _, c := range candidatos {
				if c.Caractere == qRef.Hanzi {
					alvoRepetido = alvoRevisao{entrada: c, emEstudo: qRef.EmEstudo, emFoco: qRef.EmFoco}
					encontrou = true
					break
				}
			}
			if encontrou {
				alvo = alvoRepetido
				modoEvitar = varianteParaModoBase(qRef.Variante)
			}
		}

		var questao QuestaoRevisao
		var err error

		if modoBase == ModoGeral {
			estatisticas := r.estatisticasSessao(alvo.entrada.Caractere, cacheEstatisticas)
			ordem := r.ordenarModosPorPendencia(alvo, modosPermitidos, estatisticas)
			sucesso := false
			for _, idx := range ordem {
				m := modosPermitidos[idx]
				if m == modoEvitar {
					continue
				}
				poolsQuestao := poolsPorModo[m]

				if !r.alvoElegivelNoModo(alvo, m, poolsQuestao.Todos) {
					continue
				}

				if r.Config().RevisaoFiltroDificuldade == "" {
					r.buscador.DefinirDificuldadeAlvo(dificuldadePorStreak(estatisticas[m]))
				}

				q, e := r.montarQuestao(m, alvo, poolsQuestao, modoBase, "")
				if e != nil {
					continue
				}
				if !r.registrarQuestaoInedita(&q) {
					continue
				}
				questao = q
				sucesso = true
				break
			}
			if !sucesso {
				err = fmt.Errorf("nenhum modo disponível ou inédito para o alvo")
			}
		} else {
			if r.Config().RevisaoFiltroDificuldade == "" {
				estatisticas := r.estatisticasSessao(alvo.entrada.Caractere, cacheEstatisticas)
				r.buscador.DefinirDificuldadeAlvo(dificuldadePorStreak(estatisticas[modoBase]))
			}
			questao, err = r.montarQuestao(modoBase, alvo, pools, modoBase, varianteForcada)
			if err == nil && !r.registrarQuestaoInedita(&questao) {
				err = fmt.Errorf("questão já usada")
			}
		}

		if err != nil {
			continue
		}
		questoes = append(questoes, questao)
	}

	if len(questoes) == 0 {
		return nil, fmt.Errorf("não foi possível montar questões para o modo %q", modo)
	}
	return questoes, nil
}

// registrarQuestaoInedita confere a assinatura da questão contra o histórico da sessão e a marca
// como usada. Devolve false se a questão já apareceu (o chamador tenta outra). Para quebra-cabeças,
// registra a atividade no histórico para TODAS as peças do tabuleiro, bloqueando que qualquer uma
// delas repita a atividade na mesma sessão.
func (r *GerenciadorRevisao) registrarQuestaoInedita(q *QuestaoRevisao) bool {
	if ehQuebraCabeca(q.Variante) {
		sigAlvo := assinaturaQuestao(q)
		if r.buscador.HistoricoContem(sigAlvo) || r.buscador.JaEsteveEmQuebraCabecaNaSessao(q.Hanzi) {
			return false
		}
		for _, opt := range q.Opcoes {
			if opt.Hanzi == "" {
				continue
			}
			for _, v := range VariantesQuebraCabeca {
				r.buscador.RegistrarSeInedita(v + ":" + opt.Hanzi)
			}
		}
		return true
	}
	return r.buscador.RegistrarSeInedita(assinaturaQuestao(q))
}

func assinaturaQuestao(q *QuestaoRevisao) string {
	if q.Variante == VarianteOrdenacao {
		return "ordenacao:" + q.FraseOriginal
	}
	if q.Variante == VariantePronunciaFrase || q.Variante == VariantePronunciaSequencia {
		return q.Variante + ":" + q.FraseOriginal
	}
	if q.Variante == VarianteFoneticaFrase || q.Variante == VarianteFoneticaTraducao || q.Variante == VarianteFoneticaFilaPinyin {
		return q.Variante + ":" + q.FraseOriginal
	}
	if q.Variante == VarianteContexto || q.Variante == VarianteTraducaoContexto || q.Variante == VarianteDesenhoContexto {
		return q.Variante + ":" + q.Hanzi + ":" + q.FraseOriginal
	}
	return q.Variante + ":" + q.Hanzi
}

// ----- Seção: Estatísticas e Priorização -----

// estatisticasSessao devolve (com cache por sessão) os streaks por área de uma palavra, com a
// regra híbrida do desenho aplicada.
func (r *GerenciadorRevisao) estatisticasSessao(palavra string, cache map[string]map[string]int) map[string]int {
	if s, ok := cache[palavra]; ok {
		return s
	}
	s, err := progresso.ObterEstatisticasPalavra(palavra)
	if err != nil || s == nil {
		s = map[string]int{}
	}
	// Para palavra multi-hanzi, o desenho vive nos hanzis componentes (o streak de desenho da própria
	// palavra fica sempre 0). A "pendência" de desenho passa a ser o MENOR streak entre os componentes,
	// para o desenho não ser eternamente sorteado como o mais atrasado nem impedir os demais modos.
	if utf8.RuneCountInString(palavra) > 1 {
		s["desenho"] = r.menorDesenhoComponentes(palavra)
	}
	cache[palavra] = s
	return s
}

// menorDesenhoComponentes devolve o menor streak de desenho entre os hanzis componentes da palavra
// (0 quando ela não tem componentes com estatística) — o desenho da palavra só conclui com o menor.
func (r *GerenciadorRevisao) menorDesenhoComponentes(palavra string) int {
	menor := -1
	for _, hanzi := range hanzisComponentes(palavra) {
		d := 0
		if st, err := progresso.ObterEstatisticasPalavra(hanzi); err == nil {
			d = st["desenho"]
		}
		if menor < 0 || d < menor {
			menor = d
		}
	}
	if menor < 0 {
		return 0
	}
	return menor
}

// dificuldadePorStreak traduz o streak de acertos da palavra num modo de atividade para o degrau
// de dificuldade-alvo da atividade.
func dificuldadePorStreak(streak int) string {
	switch {
	case streak <= 0:
		return busca.DificuldadeIntroducao
	case streak >= 1 && streak <= 3:
		return busca.DificuldadeIniciante
	case streak >= 4 && streak <= 6:
		return busca.DificuldadeIntermediario
	default:
		return busca.DificuldadeAvancado
	}
}

// ----- Seção: Seleção de Alvos -----

// entradaFoco monta a entrada de revisão para um item do grupo de foco que não está no banco de
// candidatos single-char (tipicamente uma palavra multi-hanzi), a partir da leitura do dicionário.
func (r *GerenciadorRevisao) entradaFoco(item string) *dicionario.DecomposicaoHanzi {
	pinyin, significados, _ := r.Dicionario.Leitura(item)
	if pinyin == "" || len(significados) == 0 {
		return nil
	}
	return &dicionario.DecomposicaoHanzi{
		Caractere: item,
		Pinyin:    []string{pinyin},
		Definicao: strings.Join(significados, "; "),
	}
}

func (r *GerenciadorRevisao) selecionarAlvos(candidatos []dicionario.DecomposicaoHanzi, foco []string, mapaStatus map[string]string, quantidade int) []alvoRevisao {
	porCaractere := make(map[string]dicionario.DecomposicaoHanzi, len(candidatos))
	for _, c := range candidatos {
		porCaractere[c.Caractere] = c
	}

	totalAlvos := quantidade * 3
	usados := make(map[string]bool, totalAlvos)

	var elegiveisFoco []dicionario.DecomposicaoHanzi
	for _, item := range foco {
		if entrada, ok := porCaractere[item]; ok {
			elegiveisFoco = append(elegiveisFoco, entrada)
			usados[item] = true
		} else if utf8.RuneCountInString(item) > 1 {
			// Palavra multi-hanzi não está no banco single-char: entra pela leitura do dicionário.
			// (Caractere isolado fora dos candidatos do modo continua sendo ignorado, como antes.)
			if entrada := r.entradaFoco(item); entrada != nil {
				elegiveisFoco = append(elegiveisFoco, *entrada)
				usados[item] = true
			}
		}
	}
	rand.Shuffle(len(elegiveisFoco), func(i, j int) { elegiveisFoco[i], elegiveisFoco[j] = elegiveisFoco[j], elegiveisFoco[i] })

	metaFocoBloco, metaFocoTotal := 0, 0
	if len(elegiveisFoco) > 0 && r.Config().PriorizarEstudoRevisao {
		focoDesejado := int(math.Round(float64(quantidade) * 0.60))
		if focoDesejado < 8 {
			focoDesejado = 8
		}
		if focoDesejado > quantidade {
			focoDesejado = quantidade
		}
		metaFocoBloco = focoDesejado
		metaFocoTotal = focoDesejado * 3
	}

	alvosFoco := make([]alvoRevisao, metaFocoTotal)
	for i := range alvosFoco {
		alvosFoco[i] = alvoRevisao{entrada: elegiveisFoco[i%len(elegiveisFoco)], emEstudo: true, emFoco: true}
	}

	var estudoRestante, aprendidos []dicionario.DecomposicaoHanzi
	for _, c := range candidatos {
		switch mapaStatus[c.Caractere] {
		case dicionario.StatusEstudo:
			if !usados[c.Caractere] {
				estudoRestante = append(estudoRestante, c)
			}
		case dicionario.StatusAprendido:
			if !usados[c.Caractere] {
				aprendidos = append(aprendidos, c)
			}
		}
	}
	rand.Shuffle(len(estudoRestante), func(i, j int) { estudoRestante[i], estudoRestante[j] = estudoRestante[j], estudoRestante[i] })

	// Ordena palavras aprendidas por prioridade de revisão: leva em conta o progresso acumulado
	// (menor progresso = mais prioridade) e o tempo desde a última prática (mais tempo sem
	// praticar = mais prioridade). O score final combina as duas variáveis:
	//   prioridade = progresso - bonus_por_tempo_sem_praticar
	// Palavras nunca praticadas recebem prioridade máxima (score = -999).
	type aprendidoComProgresso struct {
		entrada    dicionario.DecomposicaoHanzi
		prioridade float64
		ruido      float64
	}
	agora := time.Now()
	aprendidosProgresso := make([]aprendidoComProgresso, len(aprendidos))
	for i, c := range aprendidos {
		prog := 0
		if st, err := progresso.ObterEstatisticasPalavra(c.Caractere); err == nil && st != nil {
			for _, s := range st {
				prog += s
			}
		}

		// Calcula bônus por tempo sem praticar: cada dia completo sem praticar subtrai 1 do score.
		// Palavras nunca praticadas (ultima_pratica NULL) recebem score mínimo para garantir
		// que apareçam primeiro.
		var score float64
		ultima, err := progresso.ObterUltimaPratica(c.Caractere)
		if err == nil && !ultima.IsZero() {
			diasSemPraticar := agora.Sub(ultima).Hours() / 24.0
			score = float64(prog) - diasSemPraticar
		} else {
			score = -999.0
		}

		aprendidosProgresso[i] = aprendidoComProgresso{entrada: c, prioridade: score, ruido: rand.Float64()}
	}
	sort.Slice(aprendidosProgresso, func(i, j int) bool {
		if aprendidosProgresso[i].prioridade != aprendidosProgresso[j].prioridade {
			return aprendidosProgresso[i].prioridade < aprendidosProgresso[j].prioridade
		}
		return aprendidosProgresso[i].ruido < aprendidosProgresso[j].ruido
	})
	for i, ap := range aprendidosProgresso {
		aprendidos[i] = ap.entrada
	}

	permGeral := rand.Perm(len(candidatos))

	qtdEstudoDesejada := int(math.Round(float64(quantidade) * 0.30))
	qtdAprendidoDesejada := int(math.Round(float64(quantidade) * 0.10))
	if !r.Config().PriorizarEstudoRevisao {
		qtdEstudoDesejada = int(math.Round(float64(quantidade) * 0.75))
		qtdAprendidoDesejada = int(math.Round(float64(quantidade) * 0.25))
	}
	if qtdAprendidoDesejada < 1 && len(aprendidos) > 0 {
		qtdAprendidoDesejada = 1
	}

	totalEstudoAlvos := qtdEstudoDesejada * 3
	totalAprendidoAlvos := qtdAprendidoDesejada * 3

	iEstudo, iAprendido, iGeral := 0, 0, 0
	countEstudo, countAprendido := 0, 0

	proximoEstudo := func() (dicionario.DecomposicaoHanzi, bool) {
		for iEstudo < len(estudoRestante) {
			entrada := estudoRestante[iEstudo]
			iEstudo++
			if !usados[entrada.Caractere] {
				return entrada, true
			}
		}
		return dicionario.DecomposicaoHanzi{}, false
	}
	proximoAprendido := func() (dicionario.DecomposicaoHanzi, bool) {
		for iAprendido < len(aprendidos) {
			entrada := aprendidos[iAprendido]
			iAprendido++
			if !usados[entrada.Caractere] {
				return entrada, true
			}
		}
		return dicionario.DecomposicaoHanzi{}, false
	}
	proximoGeral := func() (dicionario.DecomposicaoHanzi, bool) {
		for iGeral < len(permGeral) {
			entrada := candidatos[permGeral[iGeral]]
			iGeral++
			if !usados[entrada.Caractere] {
				return entrada, true
			}
		}
		return dicionario.DecomposicaoHanzi{}, false
	}

	preenchimento := make([]alvoRevisao, 0, totalAlvos-len(alvosFoco))
	for len(alvosFoco)+len(preenchimento) < totalAlvos {
		var entrada dicionario.DecomposicaoHanzi
		var ok bool

		if countEstudo < totalEstudoAlvos {
			entrada, ok = proximoEstudo()
			if ok {
				countEstudo++
			}
		}
		if !ok && countAprendido < totalAprendidoAlvos {
			entrada, ok = proximoAprendido()
			if ok {
				countAprendido++
			}
		}
		if !ok {
			entrada, ok = proximoEstudo()
		}
		if !ok {
			entrada, ok = proximoAprendido()
		}
		if !ok {
			entrada, ok = proximoGeral()
		}
		if !ok {
			break
		}
		usados[entrada.Caractere] = true
		preenchimento = append(preenchimento, alvoRevisao{entrada: entrada, emEstudo: mapaStatus[entrada.Caractere] == dicionario.StatusEstudo})
	}

	if metaFocoBloco > len(alvosFoco) {
		metaFocoBloco = len(alvosFoco)
	}
	restoBloco := quantidade - metaFocoBloco
	if restoBloco > len(preenchimento) {
		restoBloco = len(preenchimento)
	}

	bloco := make([]alvoRevisao, 0, metaFocoBloco+restoBloco)
	bloco = append(bloco, alvosFoco[:metaFocoBloco]...)
	bloco = append(bloco, preenchimento[:restoBloco]...)
	rand.Shuffle(len(bloco), func(i, j int) { bloco[i], bloco[j] = bloco[j], bloco[i] })

	reserva := append(alvosFoco[metaFocoBloco:], preenchimento[restoBloco:]...)
	return append(bloco, reserva...)
}

// ----- Seção: Montagem de Questões (Delegador) -----

func (r *GerenciadorRevisao) montarQuestao(modo string, alvo alvoRevisao, pools poolsRevisao, sessaoModo string, varianteForcada string) (QuestaoRevisao, error) {
	pinyinAlvo := ""
	if len(alvo.entrada.Pinyin) > 0 {
		pinyinAlvo = alvo.entrada.Pinyin[0]
	}

	questao := QuestaoRevisao{
		Modo:        modo,
		Hanzi:       alvo.entrada.Caractere,
		PalavraFoco: alvo.entrada.Caractere,
		Pinyin:      pinyinAlvo,
		Definicao:   alvo.entrada.Definicao,
		EmEstudo:    alvo.emEstudo,
		EmFoco:      alvo.emFoco,
	}

	var err error
	switch modo {
	case ModoSignificado:
		questao, err = r.montarQuestaoSignificado(questao, alvo, pools, sessaoModo, varianteForcada)
	case ModoFonetica:
		questao, err = r.montarQuestaoFonetica(questao, alvo, pools, sessaoModo, varianteForcada)
	case ModoDesenho:
		questao, err = r.montarQuestaoDesenho(questao, alvo, pools, varianteForcada)
	case ModoContexto:
		questao, err = r.montarQuestaoContexto(questao, alvo, pools, varianteForcada)
	case ModoPronuncia:
		questao, err = r.montarQuestaoPronuncia(questao, alvo, pools, varianteForcada)
	default:
		return questao, fmt.Errorf("modo de revisão desconhecido: %q", modo)
	}

	if err != nil {
		return questao, err
	}

	questao.Dificuldade = r.dificuldadeDaVariante(questao.Variante)

	if ehQuebraCabeca(questao.Variante) {
		if len(questao.Opcoes) < 4 {
			return questao, fmt.Errorf("não há peças suficientes para o quebra-cabeça de %q", questao.Hanzi)
		}
	} else {
		precisaOpcoes := modo == ModoSignificado || (modo == ModoFonetica && questao.Variante != VarianteFoneticaFrase && questao.Variante != VarianteFoneticaFilaPinyin && questao.Variante != VarianteFoneticaPalavraPinyin) || (modo == ModoContexto && questao.Variante != VarianteOrdenacao && questao.Variante != VarianteOrdenacaoTraducao)
		if precisaOpcoes {
			limiteOpcoes := TotalOpcoesMultiplaEscolha
			if questao.Variante == VarianteTraducaoContexto || questao.Variante == VarianteFoneticaTraducao {
				limiteOpcoes = TotalOpcoesTraducaoContexto
			}
			if questao.Variante == VarianteRespostaDialogo {
				limiteOpcoes = 2
			}
			if len(questao.Opcoes) != limiteOpcoes {
				return questao, fmt.Errorf("não há distratores suficientes para %q (modo %s, variante %s)", questao.Hanzi, questao.Modo, questao.Variante)
			}
		}
	}

	if questao.FraseOriginal != "" {
		r.registrarVisualizacoesDaFrase(questao.FraseOriginal)
	}

	return questao, nil
}

// ----- Seção: Aplicação de Frase na Questão -----

// preencherFrase busca no módulo central a frase ideal para a variante da questão (ver
// busca.CriteriosFraseDaVariante) e a aplica: lacuna, versão oculta, segmentações e tradução.
// Devolve false quando nenhuma frase do acervo cumpre os critérios.
func (r *GerenciadorRevisao) preencherFrase(questao *QuestaoRevisao) bool {
	criterios := busca.CriteriosFraseDaVariante(questao.Variante, questao.Hanzi)
	escolhida, ok := r.buscador.BuscarFraseIdeal(criterios)
	if !ok {
		return false
	}

	textoChines := escolhida.Chines
	tipoExibicao := r.Config().TipoHanziExibicao
	if tipoExibicao == "simplificado" || tipoExibicao == "tradicional" {
		textoChines = r.Dicionario.ConverterTexto(textoChines, tipoExibicao)
	}

	r.aplicarFraseNaQuestao(questao, textoChines, escolhida)
	return true
}

func (r *GerenciadorRevisao) aplicarFraseNaQuestao(questao *QuestaoRevisao, textoChines string, frase dicionario.Frase) {
	questao.FraseOriginal = textoChines

	runesOriginal := []rune(textoChines)
	runesHanzi := []rune(questao.Hanzi)
	lenHanzi := len(runesHanzi)

	targetStart := -1
	if lenHanzi > 0 && len(runesOriginal) >= lenHanzi {
		for i := 0; i <= len(runesOriginal)-lenHanzi; i++ {
			match := true
			for j := 0; j < lenHanzi; j++ {
				if runesOriginal[i+j] != runesHanzi[j] {
					match = false
					break
				}
			}
			if match {
				targetStart = i
				break
			}
		}
	}

	mascara := strings.Repeat("＿", lenHanzi)
	if targetStart != -1 {
		targetEnd := targetStart + lenHanzi
		var sb strings.Builder
		sb.WriteString(string(runesOriginal[:targetStart]))
		sb.WriteString(mascara)
		sb.WriteString(string(runesOriginal[targetEnd:]))
		questao.FraseLacuna = sb.String()
	} else {
		questao.FraseLacuna = strings.Replace(textoChines, questao.Hanzi, mascara, 1)
	}

	questao.FraseTraducao = frase.Ingles
	questao.FraseAtribuicao = frase.Atribuicao
	questao.FraseTema = frase.Tema
	questao.FraseDificuldade = frase.Dificuldade

	var oculta strings.Builder
	for _, runa := range textoChines {
		if unicode.Is(unicode.Han, runa) {
			oculta.WriteString("＿")
		} else {
			oculta.WriteRune(runa)
		}
	}
	questao.FraseOculta = oculta.String()

	brutosSegmentados := r.DecomporTextoRevisao(questao.FraseOriginal)
	segOriginal, segLacuna := r.refinarSegmentacaoParaLacuna(brutosSegmentados, targetStart, lenHanzi, questao.Hanzi)

	questao.FraseOriginalSegmentada = brutosSegmentados
	questao.FraseOriginalLacunaSegmentada = segOriginal
	questao.FraseLacunaSegmentada = segLacuna
}

// refinarSegmentacaoParaLacuna ajusta a segmentação de dicionário para isolar o alvo da lacuna.
// Se uma palavra composta de dicionário contém um hanzi alvo como lacuna, ela é dividida em
// sub-palavras (prefixo, lacuna, sufixo) preservando pinyin e significados de cada parte.
func (r *GerenciadorRevisao) refinarSegmentacaoParaLacuna(brutos []PalavraRevisao, targetStart, lenHanzi int, hanziAlvo string) ([]PalavraRevisao, []PalavraRevisao) {
	targetEnd := targetStart + lenHanzi
	var segOriginal []PalavraRevisao
	var segLacuna []PalavraRevisao

	currentRuneIdx := 0
	for _, p := range brutos {
		pRunes := []rune(p.Texto)
		pRuneCount := len(pRunes)
		tokenStart := currentRuneIdx
		tokenEnd := currentRuneIdx + pRuneCount
		currentRuneIdx = tokenEnd

		tStart := targetStart
		tEnd := targetEnd
		if tStart == -1 && strings.Contains(p.Texto, hanziAlvo) {
			idxRuna := strings.Index(p.Texto, hanziAlvo)
			prefixRunes := []rune(p.Texto[:idxRuna])
			tStart = tokenStart + len(prefixRunes)
			tEnd = tStart + utf8.RuneCountInString(hanziAlvo)
		}

		temIntersecao := tStart != -1 && tokenStart < tEnd && tokenEnd > tStart

		if !temIntersecao {
			segOriginal = append(segOriginal, p)
			segLacuna = append(segLacuna, p)
			continue
		}

		if tokenStart == tStart && tokenEnd == tEnd {
			pOrig := p
			pOrig.EhLacuna = true
			segOriginal = append(segOriginal, pOrig)

			pLacuna := p
			pLacuna.Texto = strings.Repeat("＿", len(pRunes))
			pLacuna.Pinyin = ""
			pLacuna.Significados = nil
			pLacuna.EhLacuna = true
			segLacuna = append(segLacuna, pLacuna)
			continue
		}

		pinyinFields := strings.Fields(p.Pinyin)
		obterLeituraRuna := func(idxNoToken int) string {
			if idxNoToken < len(pinyinFields) {
				return pinyinFields[idxNoToken]
			}
			ch := string(pRunes[idxNoToken])
			pin, _, _ := r.Dicionario.Leitura(ch)
			return pin
		}

		var prefixRunes, lacunaRunes, suffixRunes []rune
		var prefixPinyins, lacunaPinyins, suffixPinyins []string

		for i, runa := range pRunes {
			pos := tokenStart + i
			pin := obterLeituraRuna(i)
			if pos < tStart {
				prefixRunes = append(prefixRunes, runa)
				if pin != "" {
					prefixPinyins = append(prefixPinyins, pin)
				}
			} else if pos >= tStart && pos < tEnd {
				lacunaRunes = append(lacunaRunes, runa)
				if pin != "" {
					lacunaPinyins = append(lacunaPinyins, pin)
				}
			} else {
				suffixRunes = append(suffixRunes, runa)
				if pin != "" {
					suffixPinyins = append(suffixPinyins, pin)
				}
			}
		}

		if len(prefixRunes) > 0 {
			pPre := PalavraRevisao{
				Texto:      string(prefixRunes),
				Pinyin:     strings.Join(prefixPinyins, " "),
				EhChines:   p.EhChines,
				EhNaoVista: p.EhNaoVista,
			}
			segOriginal = append(segOriginal, pPre)
			segLacuna = append(segLacuna, pPre)
		}

		if len(lacunaRunes) > 0 {
			pLacOriginal := PalavraRevisao{
				Texto:      string(lacunaRunes),
				Pinyin:     strings.Join(lacunaPinyins, " "),
				EhChines:   p.EhChines,
				EhLacuna:   true,
				EhNaoVista: p.EhNaoVista,
			}
			pLacLacuna := PalavraRevisao{
				Texto:      strings.Repeat("＿", len(lacunaRunes)),
				Pinyin:     "",
				EhChines:   p.EhChines,
				EhLacuna:   true,
				EhNaoVista: p.EhNaoVista,
			}
			segOriginal = append(segOriginal, pLacOriginal)
			segLacuna = append(segLacuna, pLacLacuna)
		}

		if len(suffixRunes) > 0 {
			pSuf := PalavraRevisao{
				Texto:      string(suffixRunes),
				Pinyin:     strings.Join(suffixPinyins, " "),
				EhChines:   p.EhChines,
				EhNaoVista: p.EhNaoVista,
			}
			segOriginal = append(segOriginal, pSuf)
			segLacuna = append(segLacuna, pSuf)
		}
	}

	return segOriginal, segLacuna
}

// DecomporTextoRevisao devolve a segmentação em palavras de um texto (com pinyin/significados),
// marcada com EhNaoVista se a palavra não constar no vocabulário do usuário.
func (r *GerenciadorRevisao) DecomporTextoRevisao(texto string) []PalavraRevisao {
	mapaVocab := make(map[string]bool)
	vocabulario, err := progresso.GetAllVocab()
	if err == nil {
		tipoExibicao := ""
		if r != nil && r.Config != nil {
			tipoExibicao = r.Config().TipoHanziExibicao
		}
		for _, v := range vocabulario {
			mapaVocab[v.Hanzi] = true
			if r != nil && r.Dicionario != nil && tipoExibicao != "" {
				mapaVocab[r.Dicionario.ConverterTexto(v.Hanzi, tipoExibicao)] = true
			}
		}
	}
	return decomporTextoComVocab(r.Dicionario, texto, mapaVocab)
}

func decomporTexto(dic *dicionario.GerenciadorDicionario, texto string) []PalavraRevisao {
	return decomporTextoComVocab(dic, texto, nil)
}

func decomporTextoComVocab(dic *dicionario.GerenciadorDicionario, texto string, mapaVocab map[string]bool) []PalavraRevisao {
	tokensRaw := segmentacao.SegmentarTodosTokens(texto)
	var resultado []PalavraRevisao

	for _, t := range tokensRaw {
		if !t.EhChines {
			resultado = append(resultado, PalavraRevisao{
				Texto:    t.Texto,
				EhChines: false,
			})
			continue
		}

		var subTokens []string
		if dic != nil && dic.TemEntrada(t.Texto) {
			subTokens = append(subTokens, t.Texto)
		} else if dic != nil {
			subTokens = append(subTokens, dic.SegmentarPorDicionario(t.Texto)...)
		} else {
			subTokens = append(subTokens, t.Texto)
		}

		for _, st := range subTokens {
			pinyin := ""
			var significados []string
			if dic != nil {
				pinyin, significados, _ = dic.Leitura(st)
			}

			ehNaoVista := false
			if mapaVocab != nil {
				ehNaoVista = !verificarPalavraVista(st, mapaVocab)
			}

			resultado = append(resultado, PalavraRevisao{
				Texto:        st,
				Pinyin:       pinyin,
				Significados: significados,
				EhChines:     true,
				EhNaoVista:   ehNaoVista,
			})
		}
	}

	return resultado
}

func verificarPalavraVista(palavra string, mapaVocab map[string]bool) bool {
	if mapaVocab == nil {
		return true
	}
	if mapaVocab[palavra] {
		return true
	}
	todomasVistos := true
	for _, runa := range palavra {
		if !mapaVocab[string(runa)] {
			todomasVistos = false
			break
		}
	}
	return todomasVistos
}

func (r *GerenciadorRevisao) registrarVisualizacoesDaFrase(frase string) {
	if frase == "" {
		return
	}

	palavras := r.DecomporTextoRevisao(frase)
	ocorrencias := make(map[string]int)
	for _, p := range palavras {
		if !p.EhChines {
			continue
		}
		ocorrencias[p.Texto]++
	}

	if len(ocorrencias) == 0 {
		return
	}

	// Garante que todas as palavras estejam criadas antes do incremento
	for palavra := range ocorrencias {
		_ = progresso.RegistrarVisto(palavra)
	}

	_ = progresso.IncrementarVisualizacoesVocab(ocorrencias)
}
