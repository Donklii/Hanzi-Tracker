package busca

import (
	"math/rand/v2"
	"sort"
	"unicode/utf8"

	"wails_app/dicionario"
)

// ----- Seção: Atividades Ideais (modos, variantes, elegibilidade e priorização) -----
//
// Responde ONDE praticar: quais modos existem, quais candidatos servem a cada modo, se um alvo tem
// insumos para uma atividade e qual atividade é a mais pendente para um alvo em foco. As demais
// partes do sistema consultam este módulo — nunca decidem elegibilidade por conta própria.

// Modos de revisão.
const (
	ModoSignificado = "significado"
	ModoFonetica    = "fonetica"
	ModoDesenho     = "desenho"
	ModoContexto    = "contexto"
	ModoPronuncia   = "pronuncia"
	ModoGeral       = "geral"
)

// ModosConcretos são os modos sorteáveis pela revisão geral (exclui ModoGeral, que os combina).
var ModosConcretos = []string{ModoSignificado, ModoFonetica, ModoDesenho, ModoContexto, ModoPronuncia}

// Variantes de atividade de cada modo. São a identidade da questão no contrato com o frontend
// (QuestaoRevisao.Variante) e nas assinaturas de deduplicação da sessão.
const (
	VarianteHanziParaSignificado          = "hanzi_para_significado"
	VarianteSignificadoParaHanzi          = "significado_para_hanzi"
	VarianteQuebraCabecaSignificado       = "quebracabeca_significado"
	VarianteImagemParaSignificado         = "imagem_para_significado"
	VarianteSignificadoParaImagem         = "significado_para_imagem"
	VarianteHanziFraseParaSignificado     = "hanzi_frase_para_significado"
	VarianteSignificadoParaHanziConhecido = "significado_para_hanzi_conhecido"

	VarianteAudioParaHanzi        = "audio_para_hanzi"
	VarianteHanziParaAudio        = "hanzi_para_audio"
	VarianteHanziParaPinyin       = "hanzi_para_pinyin"
	VarianteFoneticaFrase         = "fonetica_frase"
	VarianteFoneticaTraducao      = "fonetica_traducao"
	VarianteFoneticaFilaPinyin    = "fonetica_fila_pinyin"
	VarianteFoneticaPalavraPinyin = "fonetica_palavra_pinyin"
	VarianteQuebraCabecaFonetica  = "quebracabeca_fonetica"
	VarianteQuebraCabecaTrio      = "quebracabeca_trio"

	VarianteDesenhoContexto   = "desenho_contexto"
	VarianteDesenhoMemoria    = "desenho_memoria"
	VarianteDesenhoComponente = "desenho_componente"
	VarianteDesenhoGuiado     = "desenho_guiado"
	VarianteDesenhoMontagem   = "desenho_montagem"

	VarianteContexto         = "contexto"
	VarianteTraducaoContexto = "traducao_contexto"

	VarianteOrdenacao         = "ordenacao"
	VarianteOrdenacaoTraducao = "ordenacao_traducao"

	VariantePronunciaFrase     = "pronuncia_frase"
	VariantePronunciaSequencia = "pronuncia_sequencia"
	VariantePronunciaBaralho   = "pronuncia_baralho"
	VariantePronunciaTipo      = "pronuncia_tipo"

	VarianteCompreensao          = "compreensao"
	VarianteCompreensaoTraduzida = "compreensao_traduzida"
	VarianteRespostaDialogo      = "resposta_dialogo"
)

// Categorias de dificuldade das atividades de revisão.
const (
	DificuldadeIntroducao    = "introdução"
	DificuldadeIniciante     = "iniciante"
	DificuldadeIntermediario = "intermediário"
	DificuldadeAvancado      = "avançado"
)

// ----- Seção: Modos Permitidos e Candidatos por Modo -----

// ModosPermitidos lista os modos concretos que a revisão geral pode sortear: remove os desativados
// na configuração, os que possuem todas as sub-atividades desligadas e os sem motor (TTS/STT). Se o usuário desativar todos, o fallback devolve todos
// os modos com motor disponível.
func (b *Buscador) ModosPermitidos() []string {
	desativados := make(map[string]bool)
	for _, m := range b.config().ModosRevisaoGeralDesativados {
		desativados[m] = true
	}

	motorIndisponivel := func(m string) bool {
		if m == ModoFonetica && (b.config().MotorTtsAtivo == "" || b.config().MotorTtsAtivo == "nenhum") {
			return true
		}
		if m == ModoPronuncia && (b.config().MotorSttAtivo == "" || b.config().MotorSttAtivo == "nenhum") {
			return true
		}
		return false
	}

	subAtividadesDesativadas := b.config().AtividadesDesativadas

	var permitidos []string
	for _, m := range ModosConcretos {
		if desativados[m] || motorIndisponivel(m) || todasSubAtividadesDesativadas(m, subAtividadesDesativadas) {
			continue
		}
		permitidos = append(permitidos, m)
	}
	if len(permitidos) == 0 {
		for _, m := range ModosConcretos {
			if motorIndisponivel(m) {
				continue
			}
			permitidos = append(permitidos, m)
		}
	}
	return permitidos
}

// VariantesDoModo devolve as sub-atividades (variantes) associadas a um modo concreto de revisão.
func VariantesDoModo(modo string) []string {
	switch modo {
	case ModoSignificado:
		return []string{
			VarianteHanziParaSignificado,
			VarianteSignificadoParaHanzi,
			VarianteHanziFraseParaSignificado,
			VarianteSignificadoParaHanziConhecido,
			VarianteImagemParaSignificado,
			VarianteSignificadoParaImagem,
			VarianteQuebraCabecaSignificado,
			VarianteQuebraCabecaTrio,
		}
	case ModoFonetica:
		return []string{
			VarianteAudioParaHanzi,
			VarianteHanziParaAudio,
			VarianteHanziParaPinyin,
			VarianteFoneticaFrase,
			VarianteFoneticaTraducao,
			VarianteFoneticaFilaPinyin,
			VarianteFoneticaPalavraPinyin,
			VarianteQuebraCabecaFonetica,
			VarianteQuebraCabecaTrio,
		}
	case ModoDesenho:
		return []string{
			VarianteDesenhoContexto,
			VarianteDesenhoMemoria,
			VarianteDesenhoComponente,
			VarianteDesenhoGuiado,
			VarianteDesenhoMontagem,
		}
	case ModoContexto:
		return []string{
			VarianteContexto,
			VarianteTraducaoContexto,
			VarianteOrdenacao,
			VarianteOrdenacaoTraducao,
			VarianteCompreensao,
			VarianteCompreensaoTraduzida,
			VarianteRespostaDialogo,
		}
	case ModoPronuncia:
		return []string{
			VariantePronunciaFrase,
			VariantePronunciaSequencia,
			VariantePronunciaBaralho,
			VariantePronunciaTipo,
		}
	default:
		return nil
	}
}

func todasSubAtividadesDesativadas(modo string, desativadas []string) bool {
	if len(desativadas) == 0 {
		return false
	}
	vars := VariantesDoModo(modo)
	if len(vars) == 0 {
		return false
	}
	mapa := make(map[string]bool, len(desativadas))
	for _, d := range desativadas {
		mapa[d] = true
	}
	for _, v := range vars {
		if !mapa[v] {
			return false
		}
	}
	return true
}

// CandidatosParaModo devolve as entradas do dicionário aptas ao modo, já na grafia de exibição
// configurada: desenho exige traçados, contexto exige frase + traçados, ordenação/pronúncia exigem
// frase com alta cobertura de vocabulário conhecido.
func (b *Buscador) CandidatosParaModo(modo string) []dicionario.DecomposicaoHanzi {
	todos := b.dicionario.Banco.CandidatosRevisao()

	if b.config().TipoHanziExibicao == "tradicional" {
		// Converte cada candidato para sua forma tradicional
		tradTodos := make([]dicionario.DecomposicaoHanzi, 0, len(todos))
		for _, e := range todos {
			tradChar := b.dicionario.ConverterTexto(e.Caractere, "tradicional")
			if tradChar != "" {
				copia := e
				copia.Caractere = tradChar
				tradTodos = append(tradTodos, copia)
			}
		}
		todos = tradTodos
	}

	if b.config().TipoHanziExibicao == "simplificado" || b.config().TipoHanziExibicao == "tradicional" {
		todos = filtrar(todos, func(e dicionario.DecomposicaoHanzi) bool {
			tipo := b.dicionario.TipoHanzi(e.Caractere)
			if b.config().TipoHanziExibicao == "simplificado" {
				return tipo != "Tradicional"
			}
			return tipo != "Simplificado"
		})
	}

	switch modo {
	case ModoDesenho:
		return filtrar(todos, func(e dicionario.DecomposicaoHanzi) bool {
			simp := b.dicionario.ConverterTexto(e.Caractere, "simplificado")
			return b.dicionario.Tracados.Tem(e.Caractere) || b.dicionario.Tracados.Tem(simp)
		})
	case ModoContexto:
		var desativadas []string
		if b.config != nil {
			desativadas = b.config().AtividadesDesativadas
		}
		varianteAtiva := func(v string) bool {
			for _, d := range desativadas {
				if d == v {
					return false
				}
			}
			return true
		}
		temContexto := varianteAtiva(VarianteContexto) || varianteAtiva(VarianteTraducaoContexto)
		temOrdenacao := varianteAtiva(VarianteOrdenacao) || varianteAtiva(VarianteOrdenacaoTraducao)
		temCompreensao := varianteAtiva(VarianteCompreensao) || varianteAtiva(VarianteCompreensaoTraduzida)
		temDialogo := varianteAtiva(VarianteRespostaDialogo)

		return filtrar(todos, func(e dicionario.DecomposicaoHanzi) bool {
			caractereRuna, _ := utf8.DecodeRuneInString(e.Caractere)
			simp := b.dicionario.ConverterTexto(e.Caractere, "simplificado")
			condContext := temContexto && b.frases.TemCaractere(caractereRuna) && (b.dicionario.Tracados.Tem(e.Caractere) || b.dicionario.Tracados.Tem(simp))
			condOrdenacao := temOrdenacao && b.frases.TemFraseComCobertura(caractereRuna, b.mapaStatus, dicionario.CoberturaAlta)
			condCompreensao := temCompreensao && b.compreensao != nil && len(b.compreensao.PerguntasComCaractere(caractereRuna)) > 0
			condDialogo := temDialogo && b.compreensao != nil && len(b.compreensao.DialogosComCaractere(caractereRuna)) > 0
			return condContext || condOrdenacao || condCompreensao || condDialogo
		})
	case ModoPronuncia:
		return filtrar(todos, func(e dicionario.DecomposicaoHanzi) bool {
			caractereRuna, _ := utf8.DecodeRuneInString(e.Caractere)
			return b.frases.TemFraseComCobertura(caractereRuna, b.mapaStatus, dicionario.CoberturaAlta)
		})
	case ModoGeral:
		return todos
	default:
		return todos
	}
}

func filtrar(entradas []dicionario.DecomposicaoHanzi, manter func(dicionario.DecomposicaoHanzi) bool) []dicionario.DecomposicaoHanzi {
	filtradas := make([]dicionario.DecomposicaoHanzi, 0, len(entradas))
	for _, e := range entradas {
		if manter(e) {
			filtradas = append(filtradas, e)
		}
	}
	return filtradas
}

// ----- Seção: Elegibilidade de Alvo por Atividade -----

// AlvoElegivelNoModo decide se a palavra-alvo pode gerar questão no modo. Para caractere isolado é
// a pertença ao banco de candidatos do modo; para palavra multi-hanzi delega à checagem específica
// por palavra.
func (b *Buscador) AlvoElegivelNoModo(palavra, modo string, candidatosDoModo []dicionario.DecomposicaoHanzi) bool {
	if utf8.RuneCountInString(palavra) > 1 {
		return b.palavraElegivelNoModo(palavra, modo)
	}
	for _, c := range candidatosDoModo {
		if c.Caractere == palavra {
			return true
		}
	}
	return false
}

// palavraElegivelNoModo checa se uma palavra multi-hanzi tem insumos para o modo: traçado de algum
// componente (desenho), frase que a contenha (contexto/ordenação/pronúncia) ou leitura de dicionário
// (significado/fonética).
func (b *Buscador) palavraElegivelNoModo(palavra, modo string) bool {
	switch modo {
	case ModoDesenho:
		return b.temTracadoComponente(palavra)
	case ModoContexto, ModoPronuncia:
		return len(b.frases.FrasesComPalavra(palavra)) > 0
	case ModoSignificado, ModoFonetica:
		pinyin, significados, _ := b.dicionario.Leitura(palavra)
		return pinyin != "" && len(significados) > 0
	default:
		return false
	}
}

// ----- Seção: Priorização de Atividade por Pendência -----

// OrdenarModosPorPendencia devolve os índices dos modos permitidos na ordem de tentativa: base
// aleatória e, para alvo em foco com estatísticas, estável por prioridade (pendente < neutra <
// concluída) — a atividade mais atrasada do alvo é tentada primeiro. `meta` é o número de acertos
// consecutivos que conclui uma área.
func OrdenarModosPorPendencia(emFoco bool, modosPermitidos []string, estatisticas map[string]int, meta int) []int {
	ordem := rand.Perm(len(modosPermitidos))
	if !emFoco || estatisticas == nil {
		return ordem
	}
	sort.SliceStable(ordem, func(i, j int) bool {
		return PrioridadeModo(modosPermitidos[ordem[i]], estatisticas, meta) < PrioridadeModo(modosPermitidos[ordem[j]], estatisticas, meta)
	})
	return ordem
}

// PrioridadeModo classifica a pendência de um modo para um alvo: 0 = área pendente (streak abaixo
// da meta), 1 = sem categoria de progresso (neutra), 2 = área concluída.
func PrioridadeModo(modo string, estatisticas map[string]int, meta int) int {
	streak, temCategoria := estatisticas[modo]
	if !temCategoria {
		return 1
	}
	if streak < meta {
		return 0
	}
	return 2
}

// VarianteGraduada é uma atividade com o degrau de dificuldade que ela ocupa numa escada. Serve para
// uma sessão emprestar a atividade de um modo para a escada de outro (ver Buscador.EstenderEscada).
type VarianteGraduada struct {
	Variante    string
	Dificuldade string
}

// VariantesQuebraCabeca são as atividades de tabuleiro: em vez de um alvo com distratores, elas
// põem VÁRIAS palavras corretas na tela e todas são praticadas na mesma questão. É o que justifica
// tratá-las à parte na contabilidade de uma sessão (ver EhQuebraCabeca).
var VariantesQuebraCabeca = []string{VarianteQuebraCabecaSignificado, VarianteQuebraCabecaFonetica, VarianteQuebraCabecaTrio}

// EhQuebraCabeca diz se a variante é uma atividade de tabuleiro (ver VariantesQuebraCabeca).
func EhQuebraCabeca(variante string) bool {
	for _, v := range VariantesQuebraCabeca {
		if v == variante {
			return true
		}
	}
	return false
}

// ----- Seção: Utilitários de Modo e Variante -----

// VarianteParaModoBase traduz a variante de uma questão para o modo concreto que a originou — usado
// na revisão geral para evitar repetir a mesma atividade num alvo repetido.
func VarianteParaModoBase(variante string) string {
	switch variante {
	case VarianteContexto, VarianteTraducaoContexto, VarianteDesenhoContexto, VarianteOrdenacao, VarianteOrdenacaoTraducao, VarianteCompreensao, VarianteCompreensaoTraduzida, VarianteRespostaDialogo:
		return ModoContexto
	case VariantePronunciaFrase, VariantePronunciaSequencia, VariantePronunciaBaralho, VariantePronunciaTipo:
		return ModoPronuncia
	case VarianteFoneticaFrase, VarianteFoneticaTraducao, VarianteFoneticaFilaPinyin, VarianteFoneticaPalavraPinyin, VarianteHanziParaAudio, VarianteAudioParaHanzi, VarianteHanziParaPinyin, VarianteQuebraCabecaFonetica:
		return ModoFonetica
	case VarianteDesenhoMemoria, VarianteDesenhoComponente, VarianteDesenhoGuiado, VarianteDesenhoMontagem:
		return ModoDesenho
	case VarianteQuebraCabecaSignificado, VarianteHanziParaSignificado, VarianteSignificadoParaHanzi, VarianteHanziFraseParaSignificado, VarianteSignificadoParaHanziConhecido, VarianteImagemParaSignificado, VarianteSignificadoParaImagem:
		return ModoSignificado
	case VarianteQuebraCabecaTrio:
		return ModoGeral
	default:
		return ""
	}
}

// ObterDificuldadeVariante devolve a categoria de dificuldade (introdução, iniciante, intermediário ou avançado)
// associada a uma variante de atividade de revisão.
func ObterDificuldadeVariante(variante string) string {
	switch variante {
	case VarianteAudioParaHanzi, VarianteHanziParaPinyin, VarianteDesenhoGuiado, VarianteImagemParaSignificado, VarianteSignificadoParaImagem, VarianteHanziFraseParaSignificado, VarianteSignificadoParaHanziConhecido:
		return DificuldadeIntroducao

	case VarianteHanziParaSignificado, VarianteSignificadoParaHanzi, VarianteContexto, VarianteDesenhoMontagem, VarianteDesenhoComponente, VarianteHanziParaAudio, VariantePronunciaTipo, VarianteFoneticaPalavraPinyin, VarianteCompreensaoTraduzida:
		return DificuldadeIniciante

	case VarianteOrdenacaoTraducao, VarianteTraducaoContexto, VarianteFoneticaTraducao, VariantePronunciaSequencia, VarianteFoneticaFilaPinyin, VarianteDesenhoMemoria, VarianteQuebraCabecaSignificado, VariantePronunciaFrase:
		return DificuldadeIntermediario

	case VarianteOrdenacao, VarianteFoneticaFrase, VariantePronunciaBaralho, VarianteQuebraCabecaTrio, VarianteDesenhoContexto, VarianteQuebraCabecaFonetica, VarianteCompreensao, VarianteRespostaDialogo:
		return DificuldadeAvancado

	default:
		return DificuldadeIniciante
	}
}

var nivelDificuldade = map[string]int{
	DificuldadeIntroducao:    0,
	DificuldadeIniciante:     1,
	DificuldadeIntermediario: 2,
	DificuldadeAvancado:      3,
	"入门":                   0,
	"初级":                   1,
	"中级":                   2,
	"高级":                   3,
	"facil":                  1,
	"fácil":                  1,
	"medio":                  2,
	"médio":                  2,
	"intermediario":          2,
	"avancado":               3,
}

// filtrarVariantesPorDificuldade escolhe as variantes de opções que melhor casam com a dificuldade-alvo.
// Se o alvo for vazio, devolve todas as opções sem filtrar.
func filtrarVariantesPorDificuldade(alvo string, opcoes []string) []string {
	return filtrarVariantesPorDificuldadeCom(alvo, opcoes, ObterDificuldadeVariante)
}

// filtrarVariantesPorDificuldadeCom é o filtro com a classificação injetada: a sessão pode reclassificar
// uma atividade ao emprestá-la para a escada de outro modo (ver Buscador.EstenderEscada).
func filtrarVariantesPorDificuldadeCom(alvo string, opcoes []string, dificuldadeDe func(string) string) []string {
	if alvo == "" || len(opcoes) == 0 {
		return opcoes
	}

	alvoNivel, existe := nivelDificuldade[alvo]
	if !existe {
		return opcoes
	}

	var exatas []string
	maxAbaixo := -1
	var maiorAbaixo []string
	minAcima := 999
	var menorAcima []string

	for _, opt := range opcoes {
		difStr := dificuldadeDe(opt)
		difNivel := nivelDificuldade[difStr]

		if difNivel == alvoNivel {
			exatas = append(exatas, opt)
			continue
		}

		if difNivel < alvoNivel {
			if difNivel > maxAbaixo {
				maxAbaixo = difNivel
				maiorAbaixo = []string{opt}
			} else if difNivel == maxAbaixo {
				maiorAbaixo = append(maiorAbaixo, opt)
			}
			continue
		}

		if difNivel > alvoNivel {
			if difNivel < minAcima {
				minAcima = difNivel
				menorAcima = []string{opt}
			} else if difNivel == minAcima {
				menorAcima = append(menorAcima, opt)
			}
		}
	}

	if len(exatas) > 0 {
		return exatas
	}
	if len(maiorAbaixo) > 0 && len(menorAcima) > 0 {
		if rand.Float64() < 0.5 {
			return maiorAbaixo
		}
		return menorAcima
	}
	if len(maiorAbaixo) > 0 {
		return maiorAbaixo
	}
	if len(menorAcima) > 0 {
		return menorAcima
	}

	return opcoes
}

// SortearVariante escolhe uma variante ao acaso entre as opções dadas.
func SortearVariante(opcoes ...string) string {
	if len(opcoes) == 0 {
		return ""
	}
	return opcoes[rand.IntN(len(opcoes))]
}
