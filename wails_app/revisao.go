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
	ModoOrdenacao   = "ordenacao"
	ModoPronuncia   = "pronuncia"
	ModoGeral       = "geral"
)

// modosConcretos são os 5 modos "reais" usados pelo modo geral para sortear a modalidade de cada questão.
var modosConcretos = []string{ModoSignificado, ModoFonetica, ModoDesenho, ModoContexto, ModoOrdenacao, ModoPronuncia}

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
	FraseOculta             string           `json:"fraseOculta"` // texto da frase com os hanzis trocados por _
	FraseLacunaSegmentada   []PalavraRevisao `json:"fraseLacunaSegmentada"`
	FraseOriginalSegmentada []PalavraRevisao `json:"fraseOriginalSegmentada"`
	FraseTraducao           string           `json:"fraseTraducao"`
	FraseAtribuicao         string           `json:"fraseAtribuicao"` // crédito CC-BY do Tatoeba — exibir junto da frase

	// Opcoes: 4 alternativas embaralhadas (vazio nos modos só-desenho e ordenacao).
	Opcoes []OpcaoRevisao `json:"opcoes"`

	// PilhaOrdenacao: hanzis corretos misturados com distratores, usados apenas no ModoOrdenacao
	PilhaOrdenacao []OpcaoRevisao `json:"pilhaOrdenacao"`
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
	if modo == ModoFonetica && (a.Config.MotorTtsAtivo == "" || a.Config.MotorTtsAtivo == "nenhum") {
		return nil, fmt.Errorf("o modo fonética requer um motor TTS ativo nas configurações")
	}
	if modo == ModoPronuncia && (a.Config.MotorSttAtivo == "" || a.Config.MotorSttAtivo == "nenhum") {
		return nil, fmt.Errorf("o modo pronúncia requer um motor de reconhecimento de fala ativo nas configurações")
	}

	if quantidade <= 0 {
		quantidade = QuestoesPorSessaoPadrao
	}
	if quantidade > QuestoesPorSessaoMaximo {
		quantidade = QuestoesPorSessaoMaximo
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

	candidatos := a.candidatosParaModo(modo)
	if len(candidatos) < 4 {
		return nil, fmt.Errorf("não há caracteres suficientes no dicionário para o modo %q", modo)
	}

	var candEstudo []dicionario.DecomposicaoHanzi
	var candAprendido []dicionario.DecomposicaoHanzi
	for _, c := range candidatos {
		if mapaStatus[c.Caractere] == "estudo" {
			candEstudo = append(candEstudo, c)
		} else if mapaStatus[c.Caractere] == "aprendido" {
			candAprendido = append(candAprendido, c)
		}
	}

	a.mu.Lock()
	a.historicoRevisao = make(map[string]bool)
	a.mu.Unlock()

	alvos := a.selecionarAlvos(candidatos, emEstudo, quantidade*3)

	// Para o modo geral, pré-computa os candidatos dos modos concretos permitidos.
	var candidatosPorModo map[string][]dicionario.DecomposicaoHanzi
	var modosPermitidos []string

	if modo == ModoGeral {
		for _, m := range modosConcretos {
			if m == ModoFonetica && (a.Config.MotorTtsAtivo == "" || a.Config.MotorTtsAtivo == "nenhum") {
				continue
			}
			if m == ModoPronuncia && (a.Config.MotorSttAtivo == "" || a.Config.MotorSttAtivo == "nenhum") {
				continue
			}
			modosPermitidos = append(modosPermitidos, m)
		}

		candidatosPorModo = make(map[string][]dicionario.DecomposicaoHanzi, len(modosPermitidos))
		for _, m := range modosPermitidos {
			candidatosPorModo[m] = a.candidatosParaModo(m)
		}
	}

	questoes := make([]QuestaoRevisao, 0, quantidade)
	for _, alvo := range alvos {
		if len(questoes) >= quantidade {
			break
		}

		modoEvitar := ""
		if modo == ModoGeral && len(questoes) > 0 && rand.Float32() < 0.2 {
			qRef := questoes[rand.IntN(len(questoes))]
			var alvoRepetido alvoRevisao
			encontrou := false
			for _, c := range candidatos {
				if c.Caractere == qRef.Hanzi {
					alvoRepetido = alvoRevisao{entrada: c}
					encontrou = true
					break
				}
			}
			if encontrou {
				alvo = alvoRepetido
				modoEvitar = varianteParaModoBase(qRef.Variante)
			}
		}

		candQuestao := candidatos
		var ceQuestao, caQuestao []dicionario.DecomposicaoHanzi
		var questao QuestaoRevisao
		var err error

		if modo == ModoGeral {
			ordem := rand.Perm(len(modosPermitidos))
			sucesso := false
			for _, i := range ordem {
				m := modosPermitidos[i]
				if m == modoEvitar {
					continue
				}
				candQuestao = candidatosPorModo[m]

				elegivel := false
				for _, c := range candQuestao {
					if c.Caractere == alvo.entrada.Caractere {
						elegivel = true
						break
					}
				}
				if !elegivel {
					continue
				}

				ceQuestao = nil
				caQuestao = nil
				for _, c := range candQuestao {
					if mapaStatus[c.Caractere] == "estudo" {
						ceQuestao = append(ceQuestao, c)
					} else if mapaStatus[c.Caractere] == "aprendido" {
						caQuestao = append(caQuestao, c)
					}
				}

				q, e := a.montarQuestao(m, alvo, candQuestao, ceQuestao, caQuestao)
				if e == nil {
					sig := assinaturaQuestao(&q)
					a.mu.RLock()
					usado := a.historicoRevisao[sig]
					a.mu.RUnlock()
					if !usado {
						questao = q
						err = nil
						sucesso = true
						a.mu.Lock()
						a.historicoRevisao[sig] = true
						a.mu.Unlock()
						break
					}
				}
			}
			if !sucesso {
				err = fmt.Errorf("nenhum modo disponível ou inédito para o alvo")
			}
		} else {
			ceQuestao = candEstudo
			caQuestao = candAprendido
			questao, err = a.montarQuestao(modo, alvo, candQuestao, ceQuestao, caQuestao)
			if err == nil {
				sig := assinaturaQuestao(&questao)
				a.mu.RLock()
				usado := a.historicoRevisao[sig]
				a.mu.RUnlock()
				if !usado {
					a.mu.Lock()
					a.historicoRevisao[sig] = true
					a.mu.Unlock()
				} else {
					err = fmt.Errorf("questão já usada")
				}
			}
		}

		if err != nil {
			continue // alvo sem frase/distratores viáveis, ou já repetido: pula
		}
		questoes = append(questoes, questao)
	}

	if len(questoes) == 0 {
		return nil, fmt.Errorf("não foi possível montar questões para o modo %q", modo)
	}
	return questoes, nil
}

func varianteParaModoBase(variante string) string {
	switch variante {
	case "ordenacao":
		return ModoOrdenacao
	case "contexto", "desenho_contexto":
		return ModoContexto
	case "pronuncia_frase", "pronuncia_sequencia":
		return ModoPronuncia
	case "desenho_memoria":
		return ModoDesenho
	case "hanzi_para_audio", "audio_para_hanzi":
		return ModoFonetica
	default:
		return ModoSignificado
	}
}

func assinaturaQuestao(q *QuestaoRevisao) string {
	if q.Variante == "ordenacao" {
		return "ordenacao:" + q.FraseOriginal
	}
	if q.Variante == "pronuncia_frase" || q.Variante == "pronuncia_sequencia" {
		return q.Variante + ":" + q.FraseOriginal
	}
	if q.Variante == "contexto" || q.Variante == "desenho_contexto" {
		return q.Variante + ":" + q.Hanzi + ":" + q.FraseOriginal
	}
	return q.Variante + ":" + q.Hanzi
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
	case ModoOrdenacao, ModoPronuncia:
		return filtrar(todos, func(e dicionario.DecomposicaoHanzi) bool {
			r, _ := utf8.DecodeRuneInString(e.Caractere)
			return a.temFrase90(r)
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

	case ModoOrdenacao:
		questao.Variante = "ordenacao"
		if !a.preencherFrase(&questao) {
			return questao, fmt.Errorf("nenhuma frase contém %q", questao.Hanzi)
		}
		if err := a.gerarPilhaOrdenacao(&questao, candidatos); err != nil {
			return questao, err
		}

	case ModoPronuncia:
		questao.Variante = sortearVariante("pronuncia_frase", "pronuncia_sequencia")
		if !a.preencherFrase(&questao) {
			return questao, fmt.Errorf("nenhuma frase contém %q", questao.Hanzi)
		}

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

	// Filtra frases por porcentagem de caracteres já visualizados (aprendidos ou em estudo).
	var frases90, frases50, frases25 []dicionario.Frase
	for _, f := range frases {
		totalUnicos := 0
		conhecidos := 0
		vistos := make(map[rune]bool)

		for _, r := range f.Chines {
			if vistos[r] || !unicode.Is(unicode.Han, r) {
				continue
			}
			vistos[r] = true
			totalUnicos++
			if a.mapaStatusRevisao[string(r)] != "" {
				conhecidos++
			}
		}

		if totalUnicos > 0 {
			perc := float64(conhecidos) / float64(totalUnicos)
			if perc >= 0.9 {
				frases90 = append(frases90, f)
			}
			if perc >= 0.5 {
				frases50 = append(frases50, f)
			}
			if perc >= 0.25 {
				frases25 = append(frases25, f)
			}
		}
	}

	if questao.Variante == "ordenacao" || questao.Variante == "pronuncia_frase" || questao.Variante == "pronuncia_sequencia" {
		if len(frases90) > 0 {
			frases = frases90
		} else {
			return false // requisito rígido: se não houver 90%, descarta
		}
	} else {
		if len(frases50) > 0 {
			frases = frases50
		} else if len(frases25) > 0 {
			frases = frases25
		}
	}

	// Filtra frases inéditas para essa variante/hanzi nesta sessão
	var disponiveis []dicionario.Frase
	a.mu.RLock()
	historico := a.historicoRevisao
	a.mu.RUnlock()

	for _, f := range frases {
		sig := questao.Variante + ":" + questao.Hanzi + ":" + f.Chines
		if questao.Variante == "ordenacao" {
			sig = "ordenacao:" + f.Chines
		} else if questao.Variante == "pronuncia_frase" || questao.Variante == "pronuncia_sequencia" {
			sig = questao.Variante + ":" + f.Chines
		}
		if historico != nil && !historico[sig] {
			disponiveis = append(disponiveis, f)
		} else if historico == nil {
			disponiveis = append(disponiveis, f)
		}
	}

	if len(disponiveis) == 0 {
		return false
	}
	frases = disponiveis

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

	var oculta strings.Builder
	for _, r := range textoChinês {
		if unicode.Is(unicode.Han, r) {
			oculta.WriteString("＿")
		} else {
			oculta.WriteRune(r)
		}
	}
	questao.FraseOculta = oculta.String()

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

		var subTokens []string
		if a.temEntradaNoDicionario(t.Texto) {
			subTokens = append(subTokens, t.Texto)
		} else {
			subTokens = append(subTokens, a.quebrarEmPalavrasDoDicionario(t.Texto)...)
		}

		for _, st := range subTokens {
			pinyin, significados, _ := a.buscarLeituraHanzi(st)
			resultado = append(resultado, PalavraRevisao{
				Texto:        st,
				Pinyin:       pinyin,
				Significados: significados,
				EhChines:     true,
			})
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

// temFrase90 verifica se o caractere possui pelo menos uma frase onde 90% do vocabulário é conhecido.
func (a *App) temFrase90(alvo rune) bool {
	frases := a.BancoFrases.FrasesComCaractere(alvo)
	for _, f := range frases {
		totalUnicos := 0
		conhecidos := 0
		vistos := make(map[rune]bool)

		for _, r := range f.Chines {
			if vistos[r] || !unicode.Is(unicode.Han, r) {
				continue
			}
			vistos[r] = true
			totalUnicos++
			if a.mapaStatusRevisao[string(r)] != "" {
				conhecidos++
			}
		}

		if totalUnicos > 0 {
			perc := float64(conhecidos) / float64(totalUnicos)
			if perc >= 0.9 {
				return true
			}
		}
	}
	return false
}

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

// gerarPilhaOrdenacao cria a pilha de hanzis para a questão de ordenação da frase.
// A pilha conterá as ocorrências originais e ~1/3 de distratores, que compartilham radicais/fonéticos com as originais.
func (a *App) gerarPilhaOrdenacao(questao *QuestaoRevisao, candidatos []dicionario.DecomposicaoHanzi) error {
	hanzisNaFrase := make(map[string]dicionario.DecomposicaoHanzi)
	var ordemHanzis []string

	for _, r := range questao.FraseOriginal {
		if !unicode.Is(unicode.Han, r) {
			continue
		}
		ch := string(r)
		if _, existe := hanzisNaFrase[ch]; !existe {
			entrada := a.BancoHanzi.Buscar(ch)
			if entrada != nil {
				hanzisNaFrase[ch] = *entrada
			} else {
				hanzisNaFrase[ch] = dicionario.DecomposicaoHanzi{Caractere: ch}
			}
			ordemHanzis = append(ordemHanzis, ch)
		}
	}

	if len(hanzisNaFrase) == 0 {
		return fmt.Errorf("frase sem hanzis válidos")
	}

	// Calcula a quantidade de distratores para ser cerca de 1/3 da pilha total
	// Ex: Se frase tem 4 hanzis, 2 distratores -> total 6, distratores são 1/3.
	qtdDistratores := (len(hanzisNaFrase) + 1) / 2

	componentes := make(map[string]bool)
	for _, ch := range ordemHanzis {
		entrada := hanzisNaFrase[ch]
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

	var distratoresPlausiveis []dicionario.DecomposicaoHanzi
	vistosDistratores := make(map[string]bool)

	for comp := range componentes {
		if comp == "" {
			continue
		}
		compostos := a.BancoHanzi.BuscarCompostosPor(comp)
		for _, compHanzi := range compostos {
			if _, ehFrase := hanzisNaFrase[compHanzi]; ehFrase {
				continue
			}
			if !vistosDistratores[compHanzi] {
				vistosDistratores[compHanzi] = true
				entrada := a.BancoHanzi.Buscar(compHanzi)
				if entrada != nil && entrada.Definicao != "" && len(entrada.Pinyin) > 0 {
					distratoresPlausiveis = append(distratoresPlausiveis, *entrada)
				}
			}
		}
	}

	rand.Shuffle(len(distratoresPlausiveis), func(i, j int) {
		distratoresPlausiveis[i], distratoresPlausiveis[j] = distratoresPlausiveis[j], distratoresPlausiveis[i]
	})

	var pilha []OpcaoRevisao
	// Adiciona os corretos considerando ocorrências repetidas na frase
	for _, r := range questao.FraseOriginal {
		if !unicode.Is(unicode.Han, r) {
			continue
		}
		ch := string(r)
		entrada := hanzisNaFrase[ch]
		pinyin := ""
		if len(entrada.Pinyin) > 0 {
			pinyin = entrada.Pinyin[0]
		}
		pilha = append(pilha, OpcaoRevisao{
			Hanzi:     ch,
			Pinyin:    pinyin,
			Definicao: entrada.Definicao,
			Correta:   true,
		})
	}

	adicionados := 0
	for _, dist := range distratoresPlausiveis {
		if adicionados >= qtdDistratores {
			break
		}
		pilha = append(pilha, OpcaoRevisao{
			Hanzi:     dist.Caractere,
			Pinyin:    dist.Pinyin[0],
			Definicao: dist.Definicao,
			Correta:   false,
		})
		adicionados++
	}

	// Caso não tenha encontrado distratores suficientes por componente, completa aleatoriamente
	if adicionados < qtdDistratores {
		candidatosShuffled := make([]dicionario.DecomposicaoHanzi, len(candidatos))
		copy(candidatosShuffled, candidatos)
		rand.Shuffle(len(candidatosShuffled), func(i, j int) {
			candidatosShuffled[i], candidatosShuffled[j] = candidatosShuffled[j], candidatosShuffled[i]
		})
		for _, dist := range candidatosShuffled {
			if adicionados >= qtdDistratores {
				break
			}
			if _, ehFrase := hanzisNaFrase[dist.Caractere]; ehFrase {
				continue
			}
			if vistosDistratores[dist.Caractere] {
				continue
			}
			pilha = append(pilha, OpcaoRevisao{
				Hanzi:     dist.Caractere,
				Pinyin:    dist.Pinyin[0],
				Definicao: dist.Definicao,
				Correta:   false,
			})
			adicionados++
			vistosDistratores[dist.Caractere] = true
		}
	}

	rand.Shuffle(len(pilha), func(i, j int) {
		pilha[i], pilha[j] = pilha[j], pilha[i]
	})

	questao.PilhaOrdenacao = pilha
	return nil
}
