package main

import (
	"fmt"
	"math/rand/v2"
	"strings"
	"unicode"
	"unicode/utf8"

	"wails_app/dicionario"
	"wails_app/progresso"
	"wails_app/segmentacao"
)

// ----- Revisão de Hanzis -----
// Gera as questões dos 5 modos de revisão (significado, fonética, desenho, contexto e geral) no
// BACKEND: aqui o acesso ao dicionário MakeMeAHanzi (definições/pinyin), ao banco de frases do
// Tatoeba e aos traçados do Hanzi Writer é barato e síncrono — o frontend só renderiza a questão
// pronta e valida a interação. O áudio da revisão fonética NÃO vem aqui: o frontend pede via
// FalarPinyin (tts.go), que já resolve cache e sidecar.

// Modos de revisão (chaves compartilhadas com o frontend).
const (
	ModoSignificado = "significado"
	ModoFonetica    = "fonetica"
	ModoDesenho     = "desenho"
	ModoContexto    = "contexto"
	ModoGeral       = "geral"
)

// modosConcretos são os 4 modos "reais" usados pelo modo geral para sortear a modalidade de cada questão.
var modosConcretos = []string{ModoSignificado, ModoFonetica, ModoDesenho, ModoContexto}

// Quantidade de questões por sessão quando o frontend não especifica, e teto de segurança.
const (
	QuestoesPorSessaoPadrao = 10
	QuestoesPorSessaoMaximo = 50
)

// OpcaoRevisao é uma das 4 alternativas de uma questão de múltipla escolha.
type OpcaoRevisao struct {
	Hanzi     string `json:"hanzi"`
	Pinyin    string `json:"pinyin"`
	Definicao string `json:"definicao"`
	Correta   bool   `json:"correta"`
}

// QuestaoRevisao é uma questão pronta para renderização. O campo Variante diz qual das duas formas
// aleatórias do modo foi sorteada:
//   - significado: "hanzi_para_significado" | "significado_para_hanzi"
//   - fonetica:    "audio_para_hanzi"       | "hanzi_para_audio"
//   - desenho:     "desenho_contexto"       | "desenho_memoria"
//   - contexto:    "contexto" (opções E canvas — o usuário escolhe como responder)
type QuestaoRevisao struct {
	Modo      string `json:"modo"`
	Variante  string `json:"variante"`
	Hanzi     string `json:"hanzi"`
	Pinyin    string `json:"pinyin"`
	Definicao string `json:"definicao"`
	EmEstudo  bool   `json:"emEstudo"`

	// Frase de contexto (modos contexto e desenho_contexto; vazia nos demais). FraseLacuna troca o
	// hanzi-alvo por "＿" — inclusive no desenho guiado, senão bastaria copiar o caractere da frase.
	FraseLacuna             string           `json:"fraseLacuna"`
	FraseOriginal           string           `json:"fraseOriginal"`
	FraseLacunaSegmentada   []PalavraRevisao `json:"fraseLacunaSegmentada"`
	FraseOriginalSegmentada []PalavraRevisao `json:"fraseOriginalSegmentada"`
	FraseTraducao           string           `json:"fraseTraducao"`
	FraseAtribuicao         string           `json:"fraseAtribuicao"` // crédito CC-BY do Tatoeba — exibir junto da frase

	// Opcoes: 4 alternativas embaralhadas (vazio nos modos só-desenho).
	Opcoes []OpcaoRevisao `json:"opcoes"`
}

// PalavraRevisao representa um token da frase segmentada, com seu pinyin e significados caso seja chinês.
type PalavraRevisao struct {
	Texto        string   `json:"texto"`
	Pinyin       string   `json:"pinyin"`
	Significados []string `json:"significados"`
	EhChines     bool     `json:"ehChines"`
}

// alvoRevisao amarra a entrada do dicionário à origem dela (vocabulário em estudo ou sorteio).
type alvoRevisao struct {
	entrada  dicionario.DecomposicaoHanzi
	emEstudo bool
}

// ----- Binding: dados de escrita para o Hanzi Writer -----

// ObterDadosEscritaHanzi devolve o JSON cru de traçados do caractere, no formato que o motor
// Hanzi Writer espera no charDataLoader do frontend.
func (a *App) ObterDadosEscritaHanzi(caractere string) (string, error) {
	dados, existe := a.BancoTracados.Dados(caractere)
	if !existe {
		return "", fmt.Errorf("não há dados de traçado para %q", caractere)
	}
	return dados, nil
}

// ----- Binding: geração de questões -----

// ObterQuestoesRevisao monta uma sessão de questões para o modo pedido. Prioriza caracteres com
// status "estudo" no vocabulário (se a config permitir) e completa com sorteios do dicionário
// MakeMeAHanzi para evitar repetições quando há poucos caracteres em estudo.
//
// O modo "geral" mistura todos os 4 modos na mesma sessão, sorteando a modalidade de cada questão.
func (a *App) ObterQuestoesRevisao(modo string, quantidade int) ([]QuestaoRevisao, error) {
	if quantidade <= 0 {
		quantidade = QuestoesPorSessaoPadrao
	}
	if quantidade > QuestoesPorSessaoMaximo {
		quantidade = QuestoesPorSessaoMaximo
	}

	candidatos := a.candidatosParaModo(modo)
	if len(candidatos) < 4 {
		return nil, fmt.Errorf("não há caracteres suficientes no dicionário para o modo %q", modo)
	}

	vocabulario, _ := progresso.GetAllVocab()
	var emEstudo []string
	vistosEstudo := make(map[string]bool)

	mapaStatus := make(map[string]string)
	for _, v := range vocabulario {
		for _, r := range v.Hanzi {
			ch := string(r)
			if _, ehAbrev := dicionario.MapaAbrevParaCompleto[ch]; ehAbrev {
				continue
			}
			if v.Status == "estudo" {
				mapaStatus[ch] = "estudo"
				if !vistosEstudo[ch] {
					vistosEstudo[ch] = true
					emEstudo = append(emEstudo, ch)
				}
			} else if v.Status == "aprendido" && mapaStatus[ch] != "estudo" {
				mapaStatus[ch] = "aprendido"
			}
		}
	}
	rand.Shuffle(len(emEstudo), func(i, j int) { emEstudo[i], emEstudo[j] = emEstudo[j], emEstudo[i] })

	// Salva o mapaStatus no App para uso posterior na priorização de frases.
	a.mapaStatusRevisao = mapaStatus

	var candEstudo []dicionario.DecomposicaoHanzi
	var candAprendido []dicionario.DecomposicaoHanzi
	for _, c := range candidatos {
		if mapaStatus[c.Caractere] == "estudo" {
			candEstudo = append(candEstudo, c)
		} else if mapaStatus[c.Caractere] == "aprendido" {
			candAprendido = append(candAprendido, c)
		}
	}

	alvos := a.selecionarAlvos(candidatos, emEstudo, quantidade)

	// Para o modo geral, pré-computa os candidatos dos 4 modos concretos.
	var candidatosPorModo map[string][]dicionario.DecomposicaoHanzi
	if modo == ModoGeral {
		candidatosPorModo = make(map[string][]dicionario.DecomposicaoHanzi, len(modosConcretos))
		for _, m := range modosConcretos {
			candidatosPorModo[m] = a.candidatosParaModo(m)
		}
	}

	questoes := make([]QuestaoRevisao, 0, len(alvos))
	for _, alvo := range alvos {
		modoQuestao := modo
		candQuestao := candidatos

		if modo == ModoGeral {
			// Sorteia um modo concreto para esta questão, garantindo que o alvo é elegível.
			modoQuestao = a.sortearModoParaAlvo(alvo, candidatosPorModo)
			candQuestao = candidatosPorModo[modoQuestao]
		}

		// Recalcula distratores em estudo/aprendidos para o pool correto.
		var ceQuestao, caQuestao []dicionario.DecomposicaoHanzi
		if modo == ModoGeral {
			for _, c := range candQuestao {
				if mapaStatus[c.Caractere] == "estudo" {
					ceQuestao = append(ceQuestao, c)
				} else if mapaStatus[c.Caractere] == "aprendido" {
					caQuestao = append(caQuestao, c)
				}
			}
		} else {
			ceQuestao = candEstudo
			caQuestao = candAprendido
		}

		questao, err := a.montarQuestao(modoQuestao, alvo, candQuestao, ceQuestao, caQuestao)
		if err != nil {
			continue // alvo sem frase/distratores viáveis: pula sem derrubar a sessão
		}
		questoes = append(questoes, questao)
	}

	if len(questoes) == 0 {
		return nil, fmt.Errorf("não foi possível montar questões para o modo %q", modo)
	}
	return questoes, nil
}

// sortearModoParaAlvo escolhe um modo concreto aleatório que aceite o alvo dado.
// Tenta os 4 modos em ordem aleatória e cai em "significado" como último recurso.
func (a *App) sortearModoParaAlvo(alvo alvoRevisao, candidatosPorModo map[string][]dicionario.DecomposicaoHanzi) string {
	ordem := rand.Perm(len(modosConcretos))
	for _, i := range ordem {
		m := modosConcretos[i]
		for _, c := range candidatosPorModo[m] {
			if c.Caractere == alvo.entrada.Caractere {
				return m
			}
		}
	}
	return ModoSignificado // fallback seguro: significado aceita qualquer caractere
}

// ----- Seleção de candidatos e alvos -----

// candidatosParaModo filtra o universo de caracteres pelo que o modo exige além de definição e
// pinyin (já garantidos por CandidatosRevisao): traçados para desenhar, frase para contextualizar.
// O modo "geral" usa o universo inteiro (cada questão será re-filtrada pelo modo concreto sorteado).
func (a *App) candidatosParaModo(modo string) []dicionario.DecomposicaoHanzi {
	todos := a.BancoHanzi.CandidatosRevisao()

	// Filtra pelo tipo de Hanzi selecionado nas configurações
	if a.Config.TipoHanziExibicao == "simplificado" || a.Config.TipoHanziExibicao == "tradicional" {
		todos = filtrar(todos, func(e dicionario.DecomposicaoHanzi) bool {
			tipo := a.Cedict.AvaliarTipoHanzi(e.Caractere)
			if a.Config.TipoHanziExibicao == "simplificado" {
				return tipo != "Tradicional"
			}
			return tipo != "Simplificado"
		})
	}

	switch modo {
	case ModoDesenho:
		return filtrar(todos, func(e dicionario.DecomposicaoHanzi) bool {
			return a.BancoTracados.Tem(e.Caractere)
		})
	case ModoContexto:
		return filtrar(todos, func(e dicionario.DecomposicaoHanzi) bool {
			r, _ := utf8.DecodeRuneInString(e.Caractere)
			return a.BancoFrases.TemCaractere(r) && a.BancoTracados.Tem(e.Caractere)
		})
	case ModoGeral:
		return todos // o universo completo — cada questão será filtrada individualmente
	default: // significado e fonética usam o universo inteiro
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

// selecionarAlvos escolhe os caracteres-alvo da sessão: primeiro os em estudo (embaralhados),
// depois completa com sorteios do dicionário, sem repetir caractere dentro da sessão.
func (a *App) selecionarAlvos(candidatos []dicionario.DecomposicaoHanzi, emEstudo []string, quantidade int) []alvoRevisao {
	porCaractere := make(map[string]dicionario.DecomposicaoHanzi, len(candidatos))
	for _, c := range candidatos {
		porCaractere[c.Caractere] = c
	}

	alvos := make([]alvoRevisao, 0, quantidade)
	usados := make(map[string]bool, quantidade)

	if a.Config.PriorizarEstudoRevisao {
		for _, caractere := range emEstudo {
			if len(alvos) >= quantidade {
				break
			}
			entrada, elegivel := porCaractere[caractere]
			if !elegivel || usados[caractere] {
				continue
			}
			usados[caractere] = true
			alvos = append(alvos, alvoRevisao{entrada: entrada, emEstudo: true})
		}
	}

	for _, i := range rand.Perm(len(candidatos)) {
		if len(alvos) >= quantidade {
			break
		}
		entrada := candidatos[i]
		if usados[entrada.Caractere] {
			continue
		}
		usados[entrada.Caractere] = true
		alvos = append(alvos, alvoRevisao{entrada: entrada})
	}

	// Mistura os em-estudo com os sorteados — sem isso a sessão começaria sempre pelos de estudo.
	rand.Shuffle(len(alvos), func(i, j int) { alvos[i], alvos[j] = alvos[j], alvos[i] })
	return alvos
}

// ----- Montagem das questões -----

func (a *App) montarQuestao(modo string, alvo alvoRevisao, candidatos, candEstudo, candAprendido []dicionario.DecomposicaoHanzi) (QuestaoRevisao, error) {
	questao := QuestaoRevisao{
		Modo:      modo,
		Hanzi:     alvo.entrada.Caractere,
		Pinyin:    alvo.entrada.Pinyin[0],
		Definicao: alvo.entrada.Definicao,
		EmEstudo:  alvo.emEstudo,
	}

	switch modo {
	case ModoSignificado:
		questao.Variante = sortearVariante("hanzi_para_significado", "significado_para_hanzi")
		questao.Opcoes = montarOpcoes(alvo.entrada, candidatos, candEstudo, candAprendido, distintosPorSignificado)

	case ModoFonetica:
		questao.Variante = sortearVariante("audio_para_hanzi", "hanzi_para_audio")
		questao.Opcoes = montarOpcoes(alvo.entrada, candidatos, candEstudo, candAprendido, distintosPorPinyin)

	case ModoDesenho:
		questao.Variante = sortearVariante("desenho_contexto", "desenho_memoria")
		if questao.Variante == "desenho_contexto" && !a.preencherFrase(&questao) {
			questao.Variante = "desenho_memoria" // sem frase para este caractere: cai na outra forma
		}

	case ModoContexto:
		questao.Variante = "contexto"
		if !a.preencherFrase(&questao) {
			return questao, fmt.Errorf("nenhuma frase contém %q", questao.Hanzi)
		}
		frase := questao.FraseOriginal
		questao.Opcoes = montarOpcoes(alvo.entrada, candidatos, candEstudo, candAprendido, func(escolhidas []OpcaoRevisao, candidata dicionario.DecomposicaoHanzi) bool {
			// Distrator não pode aparecer na frase — estaria "correto" aos olhos do usuário.
			return distintosPorHanzi(escolhidas, candidata) && !strings.Contains(frase, candidata.Caractere)
		})

	default:
		return questao, fmt.Errorf("modo de revisão desconhecido: %q", modo)
	}

	precisaOpcoes := modo == ModoSignificado || modo == ModoFonetica || modo == ModoContexto
	if precisaOpcoes && len(questao.Opcoes) != 4 {
		return questao, fmt.Errorf("não há distratores suficientes para %q", questao.Hanzi)
	}
	return questao, nil
}

func sortearVariante(a, b string) string {
	if rand.IntN(2) == 0 {
		return a
	}
	return b
}

// preencherFrase sorteia uma frase que contenha o hanzi-alvo (preferindo curtas, que cabem melhor
// na tela) e preenche os campos de frase da questão. Filtra e converte a frase de acordo com a
// configuração TipoHanziExibicao: "simplificado" ou "tradicional" descartam frases incompatíveis
// e convertem o texto; "ambos" mantém a frase original.
//
// Priorização: frases contendo mais caracteres aprendidos/em estudo ganham peso maior na seleção
// aleatória, favorecendo frases com vocabulário que o usuário já conhece.
// Devolve false se não houver nenhuma frase disponível.
func (a *App) preencherFrase(questao *QuestaoRevisao) bool {
	alvo, _ := utf8.DecodeRuneInString(questao.Hanzi)
	frases := a.BancoFrases.FrasesComCaractere(alvo)
	if len(frases) == 0 {
		return false
	}

	tipoExibicao := a.Config.TipoHanziExibicao

	const tamanhoConfortavel = 20 // em caracteres chineses; acima disso a lacuna fica difícil de achar
	curtas := make([]dicionario.Frase, 0, len(frases))
	for _, f := range frases {
		if utf8.RuneCountInString(f.Chines) <= tamanhoConfortavel {
			curtas = append(curtas, f)
		}
	}
	if len(curtas) > 0 {
		frases = curtas
	}

	// Filtra frases compatíveis com o tipo de Hanzi configurado (ignora quando "ambos")
	if tipoExibicao == "simplificado" || tipoExibicao == "tradicional" {
		compativeis := make([]dicionario.Frase, 0, len(frases))
		for _, f := range frases {
			if a.Cedict.FraseCompativelComTipo(f.Chines, tipoExibicao) {
				compativeis = append(compativeis, f)
			}
		}
		// Se houver frases compatíveis, usa só elas; caso contrário, mantém todas
		// (serão convertidas abaixo)
		if len(compativeis) > 0 {
			frases = compativeis
		}
	}

	// Seleção ponderada: frases com mais caracteres aprendidos/em estudo ganham peso maior.
	escolhida := a.selecionarFrasePonderada(frases)

	// Converte a frase para o tipo de Hanzi configurado (quando não é "ambos")
	textoChinês := escolhida.Chines
	if tipoExibicao == "simplificado" || tipoExibicao == "tradicional" {
		textoChinês = a.Cedict.ConverterTexto(textoChinês, tipoExibicao)
	}

	questao.FraseOriginal = textoChinês
	questao.FraseLacuna = strings.Replace(textoChinês, questao.Hanzi, "＿", 1)
	questao.FraseTraducao = escolhida.Ingles
	questao.FraseAtribuicao = escolhida.Atribuicao

	// Segmenta a frase para habilitar o popup de tooltip por palavra no frontend
	questao.FraseOriginalSegmentada = a.decomporTextoRevisao(questao.FraseOriginal)

	// Cria a versão com lacuna baseada na segmentada original
	questao.FraseLacunaSegmentada = make([]PalavraRevisao, len(questao.FraseOriginalSegmentada))
	for i, p := range questao.FraseOriginalSegmentada {
		if strings.Contains(p.Texto, questao.Hanzi) {
			p.Texto = strings.Replace(p.Texto, questao.Hanzi, "＿", 1)
			// Remove pinyin e significado do token que contém o caractere oculto (pois ele é a resposta)
			p.Pinyin = ""
			p.Significados = nil
		}
		questao.FraseLacunaSegmentada[i] = p
	}

	return true
}

// decomporTextoRevisao segmenta uma frase e preenche pinyin/significado dos tokens chineses.
func (a *App) decomporTextoRevisao(texto string) []PalavraRevisao {
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

		hasMeaning := false
		if utf8.RuneCountInString(t.Texto) == 1 {
			hasMeaning = true
		} else {
			if len(a.Cedict.Buscar(t.Texto)) > 0 {
				hasMeaning = true
			}
		}

		var subTokens []string
		if hasMeaning {
			subTokens = append(subTokens, t.Texto)
		} else {
			subTokens = append(subTokens, a.breakIntoDictionaryWords(t.Texto)...)
		}

		for _, st := range subTokens {
			tr := PalavraRevisao{
				Texto:    st,
				EhChines: true,
			}

			// Priorizar MakeMeAHanzi para caracteres isolados
			usouMakeMeAHanzi := false
			if utf8.RuneCountInString(st) == 1 {
				dec := a.BancoHanzi.Buscar(st)
				if dec != nil && dec.Definicao != "" {
					if len(dec.Pinyin) > 0 {
						tr.Pinyin = strings.Join(dec.Pinyin, ", ")
					}
					tr.Significados = []string{dec.Definicao}
					usouMakeMeAHanzi = true
				}
			}

			if !usouMakeMeAHanzi {
				entradas := a.Cedict.Buscar(st)
				if len(entradas) > 0 {
					tr.Pinyin = entradas[0].Pinyin
					tr.Significados = entradas[0].Significados
				}
			}

			resultado = append(resultado, tr)
		}
	}

	return resultado
}

const (
	pesoFraseBase      = 10
	pesoFraseAprendido = 400
	pesoFraseEstudo    = 200
)

// selecionarFrasePonderada escolhe uma frase usando seleção ponderada. O peso base garante que
// frases sem vocabulário conhecido ainda possam aparecer. Frases com maior proporção de vocabulário conhecido
// do usuário (estudando ou aprendido) em relação ao total de hanzis únicos são proporcionalmente mais prováveis.
func (a *App) selecionarFrasePonderada(frases []dicionario.Frase) dicionario.Frase {
	if len(frases) == 1 || len(a.mapaStatusRevisao) == 0 {
		return frases[rand.IntN(len(frases))]
	}

	pesos := make([]int, len(frases))
	pesoTotal := 0
	for i, f := range frases {
		pontosConhecimento := 0
		totalUnicos := 0
		vistos := make(map[rune]bool)
		
		for _, r := range f.Chines {
			if vistos[r] || !unicode.Is(unicode.Han, r) {
				continue
			}
			vistos[r] = true
			totalUnicos++
			switch a.mapaStatusRevisao[string(r)] {
			case "aprendido":
				pontosConhecimento += pesoFraseAprendido
			case "estudo":
				pontosConhecimento += pesoFraseEstudo
			}
		}

		var pesoFinal int
		if totalUnicos > 0 {
			pesoFinal = pesoFraseBase + (pontosConhecimento / totalUnicos)
		} else {
			pesoFinal = pesoFraseBase
		}

		pesos[i] = pesoFinal
		pesoTotal += pesoFinal
	}

	// Roleta ponderada
	sorteio := rand.IntN(pesoTotal)
	for i, peso := range pesos {
		sorteio -= peso
		if sorteio < 0 {
			return frases[i]
		}
	}
	return frases[len(frases)-1] // fallback (não deve ocorrer)
}

// ----- Distratores -----

// criterioDistrator decide se a candidata pode entrar no conjunto de opções já escolhidas.
// Cada modo usa um critério: as opções precisam ser DISTINGUÍVEIS entre si naquilo que o usuário
// compara (som na fonética, glosa no significado, o próprio caractere no contexto).
type criterioDistrator func(escolhidas []OpcaoRevisao, candidata dicionario.DecomposicaoHanzi) bool

// montarOpcoes monta as 4 alternativas (alvo + 3 distratores sorteados) já embaralhadas.
// Devolve menos de 4 se o critério esgotar os candidatos — o chamador descarta a questão.
func montarOpcoes(alvo dicionario.DecomposicaoHanzi, candidatos, candEstudo, candAprendido []dicionario.DecomposicaoHanzi, aceitar criterioDistrator) []OpcaoRevisao {
	opcoes := []OpcaoRevisao{novaOpcao(alvo, true)}

	tentarAdicionar := func(lista []dicionario.DecomposicaoHanzi) bool {
		for _, i := range rand.Perm(len(lista)) {
			candidata := lista[i]
			if candidata.Caractere != alvo.Caractere && aceitar(opcoes, candidata) {
				opcoes = append(opcoes, novaOpcao(candidata, false))
				return true
			}
		}
		return false
	}

	// 1º distrator: 50% de chance de ser um hanzi aprendido
	if rand.Float64() <= 0.50 {
		_ = tentarAdicionar(candAprendido)
	}
	if len(opcoes) < 2 {
		_ = tentarAdicionar(candidatos)
	}

	// 2º distrator: 75% de chance de ser um hanzi em estudo
	if rand.Float64() <= 0.75 {
		_ = tentarAdicionar(candEstudo)
	}
	if len(opcoes) < 3 {
		_ = tentarAdicionar(candidatos)
	}

	// 3º distrator: aleatório
	_ = tentarAdicionar(candidatos)

	// Preenchimento de segurança, caso alguma lista estivesse vazia ou sem candidatos válidos
	for len(opcoes) < 4 {
		if !tentarAdicionar(candidatos) {
			break // Esgotou os candidatos gerais também
		}
	}

	rand.Shuffle(len(opcoes), func(i, j int) { opcoes[i], opcoes[j] = opcoes[j], opcoes[i] })
	return opcoes
}

func novaOpcao(entrada dicionario.DecomposicaoHanzi, correta bool) OpcaoRevisao {
	return OpcaoRevisao{
		Hanzi:     entrada.Caractere,
		Pinyin:    entrada.Pinyin[0],
		Definicao: entrada.Definicao,
		Correta:   correta,
	}
}

// distintosPorSignificado: nenhuma glosa principal repetida — senão haveria duas opções "certas".
func distintosPorSignificado(escolhidas []OpcaoRevisao, candidata dicionario.DecomposicaoHanzi) bool {
	glosaCandidata := glosaPrincipal(candidata.Definicao)
	for _, o := range escolhidas {
		if glosaPrincipal(o.Definicao) == glosaCandidata {
			return false
		}
	}
	return true
}

// distintosPorPinyin: nenhuma sílaba (com tom) repetida — homófonos como 他/她/它 soariam idênticos
// no áudio e a questão não teria resposta única.
func distintosPorPinyin(escolhidas []OpcaoRevisao, candidata dicionario.DecomposicaoHanzi) bool {
	for _, o := range escolhidas {
		if o.Pinyin == candidata.Pinyin[0] {
			return false
		}
	}
	return true
}

func distintosPorHanzi(escolhidas []OpcaoRevisao, candidata dicionario.DecomposicaoHanzi) bool {
	for _, o := range escolhidas {
		if o.Hanzi == candidata.Caractere {
			return false
		}
	}
	return true
}

// glosaPrincipal extrai a primeira acepção da definição ("you (informal)" de "you (informal); thou").
func glosaPrincipal(definicao string) string {
	glosa, _, _ := strings.Cut(definicao, ";")
	return strings.ToLower(strings.TrimSpace(glosa))
}
