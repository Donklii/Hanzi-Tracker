package main

import (
	"fmt"
	"math"
	"math/rand"
	"sort"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"wails_app/dicionario"
	"wails_app/progresso"
	"wails_app/revisao"
	"wails_app/segmentacao"
)

// ----- Seção vinda de: bindings_dicionario.go -----

// ----- Bindings de consulta ao dicionário (expostos ao frontend) -----
// Delegações finas ao GerenciadorDicionario (a.Dicionario), a fonte única de consulta de significado,
// leitura, decomposição e caracteres compostos. A regra de prioridade (MakeMeAHanzi para caractere
// isolado, CC-CEDICT para palavras) vive no gerenciador — ver wails_app/dicionario/gerenciador.go.

// LookupWord devolve as entradas de dicionário de uma palavra (MakeMeAHanzi na frente para caracteres
// isolados; CC-CEDICT para palavras compostas).
func (a *App) LookupWord(word string) []dicionario.EntradaDicionario {
	return a.Dicionario.BuscarPalavra(word)
}

// BuscarNoDicionarioGeral pesquisa globalmente (CC-CEDICT + MakeMeAHanzi) e embrulha o resultado no
// formato de card do frontend. A confiança é 1.0 (não veio de OCR).
func (a *App) BuscarNoDicionarioGeral(termo string) []FlashcardCard {
	consultas := a.Dicionario.BuscaGeral(termo)

	resultados := make([]FlashcardCard, 0, len(consultas))
	for _, c := range consultas {
		posicao := 0
		if a.Dicionario != nil && a.Dicionario.Banco != nil {
			freq := a.Dicionario.Banco.ObterFrequencia(c.Hanzi)
			if freq != nil {
				posicao = freq.Posicao
			}
		}
		card := FlashcardCard{
			Hanzi:          c.Hanzi,
			Pinyin:         c.Pinyin,
			Significados:   c.Significados,
			Confianca:      1.0,
			TipoHanzi:      c.TipoHanzi,
			PosicaoRanking: posicao,
			NivelHSK:       c.NivelHSK,
		}
		resultados = append(resultados, card)
	}
	return resultados
}

// BuscarPorPinyin devolve os hanzis que casam com o pinyin digitado, limitados para o dropdown da UI.
func (a *App) BuscarPorPinyin(pinyin string) []string {
	const maxSugestoes = 30
	res := a.Dicionario.BuscarPorPinyin(pinyin)
	if len(res) > maxSugestoes {
		return res[:maxSugestoes]
	}
	return res
}

// AvaliarTipoHanzi retorna o tipo do hanzi para o frontend ("Tradicional", "Simplificado" ou "Ambos").
func (a *App) AvaliarTipoHanzi(hanzi string) string {
	return a.Dicionario.TipoHanzi(hanzi)
}

type InformacaoExpansao struct {
	PosicaoRanking int    `json:"posicaoRanking"`
	Alternativa    string `json:"alternativa"`
	TipoHanzi      string `json:"tipoHanzi"`
	NivelHSK       int    `json:"nivelHSK,omitempty"`
}

// ObterInformacaoExpansao retorna informações adicionais de expansão do card (ranking, HSK e alternativa tradicional/simplificada).
func (a *App) ObterInformacaoExpansao(palavra string) InformacaoExpansao {
	info := InformacaoExpansao{
		PosicaoRanking: 0,
		Alternativa:    "",
		TipoHanzi:      "",
		NivelHSK:       0,
	}
	if a.Dicionario != nil && a.Dicionario.Banco != nil {
		freq := a.Dicionario.Banco.ObterFrequencia(palavra)
		if freq != nil {
			info.PosicaoRanking = freq.Posicao
		}
		info.Alternativa = a.Dicionario.Banco.ObterAlternativa(palavra)
		info.TipoHanzi = a.Dicionario.Banco.AvaliarTipoHanzi(palavra)
	}
	if nivel, ok := dicionario.TabelaHSK30[palavra]; ok {
		info.NivelHSK = nivel
	}
	return info
}

func (a *App) DecomposeCharacter(char string) *dicionario.DecomposicaoHanzi {
	return a.Dicionario.DecomporHanzi(char)
}

func (a *App) BuscarCaracteresCompostosPor(char string) []string {
	return a.Dicionario.CompostosPor(char)
}

func (a *App) CaractereCompleto(abrev string) string {
	return a.Dicionario.CaractereCompleto(abrev)
}

func (a *App) ObterTotalHanzisDicionario() int {
	return a.Dicionario.TotalHanzis()
}

// ----- Helpers de leitura compartilhados -----

// contemHanzi diz se a string tem ao menos um caractere Han (chinês).
func contemHanzi(s string) bool {
	for _, r := range s {
		if unicode.Is(unicode.Han, r) {
			return true
		}
	}
	return false
}

// ----- Seção vinda de: bindings_vocabulario.go -----

// ----- Bindings do vocabulário do usuário (banco de progresso) -----
// Adicionar/remover/listar palavras e o registro automático de "visto" feito pelo scan de OCR e
// pelos cliques no modal de detalhes.

// AddVocab grava/atualiza uma palavra com o status escolhido pelo usuário (estudo/aprendido). A grafia
// é gravada EXATAMENTE como veio do card (simplificada OU tradicional): ela é a identidade do item, e
// as duas grafias da mesma palavra coexistem como itens independentes nas seções (ver GetVocab).
// Normalizar para simplificado aqui — como se fazia antes — fundiria as duas numa só chave.
func (a *App) AddVocab(hanzi, pinyin, significado, status string) error {
	return progresso.AddOuUpdateVocab(hanzi, status)
}

// RemoveVocab remove uma palavra do vocabulário pela grafia exata informada.
func (a *App) RemoveVocab(hanzi string) error {
	return progresso.RemoveVocab(hanzi)
}

// GetVocab lista o vocabulário para a UI, ocultando componentes/radicais avulsos e preenchendo o
// tipo (simplificado/tradicional) de cada hanzi.
func (a *App) GetVocab() ([]progresso.Vocab, error) {
	v, err := progresso.GetAllVocab()
	if err != nil {
		return nil, err
	}

	var filtrado []progresso.Vocab
	for i := range v {
		if _, ehAbrev := dicionario.MapaAbrevParaCompleto[v[i].Hanzi]; ehAbrev {
			continue // Oculta componentes e radicais avulsos do histórico
		}

		// Preenchimento dinâmico
		pinyin, significados, _ := a.Dicionario.Leitura(v[i].Hanzi)
		v[i].Pinyin = pinyin
		v[i].Significado = strings.Join(significados, ", ")

		// A grafia salva é a identidade do item e é exibida como está — reescrevê-la para o tipo
		// configurado fundiria as formas simplificada e tradicional da mesma palavra na tela.
		v[i].TipoHanzi = a.Dicionario.TipoHanzi(v[i].Hanzi)
		
		posicao := 0
		if a.Dicionario != nil && a.Dicionario.Banco != nil {
			freq := a.Dicionario.Banco.ObterFrequencia(v[i].Hanzi)
			if freq != nil {
				posicao = freq.Posicao
			}
		}
		v[i].PosicaoRanking = posicao

		filtrado = append(filtrado, v[i])
	}
	return filtrado, nil
}

// MarcarVistoSilencioso registra uma palavra como 'vista' (sem mudar status existente), buscando
// pinyin/significado nos dicionários. Usado ao navegar pela decomposição no modal de detalhes.
func (a *App) MarcarVistoSilencioso(hanzi string) {
	progresso.RegistrarVisto(hanzi)
	a.registrarHanzisIndividuais(hanzi)
}

// registrarHanzisIndividuais desmembra uma palavra multi-caractere nos seus hanzis individuais
// e registra cada um como 'visto' com pinyin/significado próprios. Não se aplica a componentes
// de decomposição — apenas aos caracteres que formam a palavra no CEDICT.
func (a *App) registrarHanzisIndividuais(palavra string) {
	if utf8.RuneCountInString(palavra) <= 1 {
		return
	}

	for _, r := range palavra {
		if !unicode.Is(unicode.Han, r) {
			continue
		}

		progresso.RegistrarVisto(string(r))
	}
}

// ObterSugestoesEstudoOcr lista as palavras 'vistas' para o pop-up de sugestão de estudo.
// Aplica os mesmos pós-processos de GetVocab, preenche a posição no ranking e ordena
// com base em um cálculo inteligente que balanceia visualizações do OCR e frequência da palavra.
func (a *App) ObterSugestoesEstudoOcr() ([]progresso.Vocab, error) {
	sugestoes, err := progresso.ObterSugestoesEstudoOcr()
	if err != nil {
		return nil, err
	}

	var filtrado []progresso.Vocab
	for i := range sugestoes {
		if _, ehAbrev := dicionario.MapaAbrevParaCompleto[sugestoes[i].Hanzi]; ehAbrev {
			continue
		}

		// Preenchimento dinâmico
		pinyin, significados, _ := a.Dicionario.Leitura(sugestoes[i].Hanzi)
		sugestoes[i].Pinyin = pinyin
		sugestoes[i].Significado = strings.Join(significados, ", ")

		// Grafia salva = identidade: exibida como está (ver GetVocab).
		sugestoes[i].TipoHanzi = a.Dicionario.TipoHanzi(sugestoes[i].Hanzi)
		
		posicao := 0
		if a.Dicionario != nil && a.Dicionario.Banco != nil {
			freq := a.Dicionario.Banco.ObterFrequencia(sugestoes[i].Hanzi)
			if freq != nil {
				posicao = freq.Posicao
			}
		}
		sugestoes[i].PosicaoRanking = posicao

		filtrado = append(filtrado, sugestoes[i])
	}

	// Ordenação por cálculo inteligente de relevância (pontuação descrita por vezesVistaOcr vs posicaoRanking)
	sort.Slice(filtrado, func(i, j int) bool {
		scoreI := calcularPontuacaoSugestao(filtrado[i].VezesVistaOcr, filtrado[i].PosicaoRanking)
		scoreJ := calcularPontuacaoSugestao(filtrado[j].VezesVistaOcr, filtrado[j].PosicaoRanking)
		if scoreI == scoreJ {
			return filtrado[i].VezesVistaOcr > filtrado[j].VezesVistaOcr
		}
		return scoreI > scoreJ
	})

	return filtrado, nil
}

// calcularPontuacaoSugestao calcula a pontuação de prioridade da palavra para o pop-up de sugestões,
// balanceando as vezes que foi vista no OCR com a posição de frequência no ranking da língua.
func calcularPontuacaoSugestao(vezesVista int, posicaoRanking int) float64 {
	pos := posicaoRanking
	if pos <= 0 {
		pos = 30000 // Se não possuir posição no ranking, trata como palavra rara
	}
	// Fator de bônus por frequência: palavras comuns (posições mais baixas, ex: 1..1000) recebem
	// uma forte prioridade, mantendo também o peso do interesse demonstrado pelo usuário (vezesVista).
	fatorRanking := 10000.0 / float64(pos+100)
	return float64(vezesVista) * (1.0 + fatorRanking)
}

// OcultarSugestoesEstudoOcr silencia para sempre a sugestão de estudo das palavras marcadas com
// "não exibir novamente" no pop-up.
func (a *App) OcultarSugestoesEstudoOcr(hanzis []string) error {
	return progresso.OcultarSugestoesEstudoOcr(hanzis)
}

// RegistrarRespostaRevisao atualiza as estatísticas de acertos/erros de uma palavra na categoria correspondente.
func (a *App) RegistrarRespostaRevisao(hanzi, pinyin, significado, categoria string, acertou bool) error {
	return progresso.AtualizarAcertosSequencia(hanzi, categoria, acertou)
}

// RegistrarRespostaFrase credita, de uma vez, o acerto de todas as palavras chinesas de uma frase
// montada em uma ou mais categorias. Os modos de montagem premiam a frase inteira ao serem concluídos
// com sucesso: a ordenação dá +1 em "significado" e "contexto"; a fonética dá +1 em "fonetica". As
// gravações são sequenciais (um único goroutine) para não disputar o lock do SQLite, e cada palavra
// conta só uma vez por frase (dedup por texto).
func (a *App) RegistrarRespostaFrase(palavras []revisao.PalavraRevisao, categorias []string, acertou bool) error {
	vistos := make(map[string]bool)
	for _, p := range palavras {
		if !p.EhChines || p.Texto == "" || vistos[p.Texto] {
			continue
		}
		vistos[p.Texto] = true
		for _, categoria := range categorias {
			if err := progresso.AtualizarAcertosSequencia(p.Texto, categoria, acertou); err != nil {
				return err
			}
		}
	}
	return nil
}

// ObterEstatisticasPalavra retorna os acertos em sequência nas 5 categorias para uma palavra.
func (a *App) ObterEstatisticasPalavra(hanzi string) (map[string]int, error) {
	return progresso.ObterEstatisticasPalavra(hanzi)
}

// ObterSugestoesAprendidoLote verifica quais das palavras enviadas atingiram o critério para serem
// marcadas como aprendidas nas categorias habilitadas da revisão.
func (a *App) ObterSugestoesAprendidoLote(palavras []string, modos []string) ([]progresso.Vocab, error) {
	sugs, err := a.revisao.SugestoesAprendidoComModos(palavras, modos)
	if err != nil {
		return nil, err
	}
	for i := range sugs {
		pinyin, significados, _ := a.Dicionario.Leitura(sugs[i].Hanzi)
		sugs[i].Pinyin = pinyin
		sugs[i].Significado = strings.Join(significados, ", ")
	}
	return sugs, nil
}

// ConverterTexto converte o texto para o tipo solicitado ("simplificado" ou "tradicional")
func (a *App) ConverterTexto(texto, tipo string) string {
	return a.Dicionario.ConverterTexto(texto, tipo)
}

// SegmentarTexto quebra um texto misto em tokens chineses segmentados por dicionário e texto plano,
// classificando e agrupando as palavras de forma dinâmica.
func (a *App) SegmentarTexto(texto string) []string {
	if texto == "" {
		return nil
	}

	var resultado []string
	tokensBrutos := segmentacao.SegmentarTodosTokens(texto)
	for _, t := range tokensBrutos {
		if t.EhChines {
			resultado = append(resultado, a.Dicionario.SegmentarPorDicionario(t.Texto)...)
		} else {
			resultado = append(resultado, t.Texto)
		}
	}
	return resultado
}

// ----- Recomendações do Baralho na Seção de Estudo -----

type RecomendacaoBaralho struct {
	Hanzi          string `json:"hanzi"`
	Pinyin         string `json:"pinyin"`
	Significado    string `json:"significado"`
	Motivo         string `json:"motivo"`
	Revelada       bool   `json:"revelada"`
	NivelHSK       int    `json:"nivelHSK,omitempty"`
	PosicaoRanking int    `json:"posicaoRanking,omitempty"`
}

// enriquecerRecomendacao preenche HSK, ranking de frequência e formata significados.
func (a *App) enriquecerRecomendacao(c *RecomendacaoBaralho) {
	if c == nil || c.Hanzi == "" {
		return
	}
	if lvl, ok := dicionario.TabelaHSK30[c.Hanzi]; ok {
		c.NivelHSK = lvl
	}
	if a.Dicionario != nil && a.Dicionario.Banco != nil {
		if freq := a.Dicionario.Banco.ObterFrequencia(c.Hanzi); freq != nil && freq.Posicao > 0 {
			c.PosicaoRanking = freq.Posicao
		}
	}
	if c.Significado == "" && a.Dicionario != nil {
		_, sigs, _ := a.Dicionario.Leitura(c.Hanzi)
		if len(sigs) > 0 {
			c.Significado = strings.Join(sigs, ", ")
		}
	}
}

func (a *App) ObterRecomendacoesBaralho(forcarNovo bool) ([]RecomendacaoBaralho, error) {
	ultimasHanzisSet := make(map[string]bool)

	if !forcarNovo {
		salvas, dataMaisRecente, err := progresso.ObterRecomendacoesBaralhoBanco()
		if err == nil && len(salvas) == 3 && !dataMaisRecente.IsZero() {
			if time.Since(dataMaisRecente) < 24*time.Hour {
				res := make([]RecomendacaoBaralho, len(salvas))
				for i, item := range salvas {
					res[i] = RecomendacaoBaralho{
						Hanzi:       item.Hanzi,
						Pinyin:      item.Pinyin,
						Significado: item.Significado,
						Motivo:      item.Motivo,
						Revelada:    item.Revelada,
					}
					a.enriquecerRecomendacao(&res[i])
				}
				return res, nil
			}
		}
	} else {
		// Ao forçar novo sorteio, desconsiderar as Hanzis salvas no banco
		salvas, _, err := progresso.ObterRecomendacoesBaralhoBanco()
		if err == nil {
			for _, item := range salvas {
				ultimasHanzisSet[item.Hanzi] = true
			}
		}
	}

	// Adicionar o histórico recente em memória à lista de exclusão para evitar repetição contínua
	a.historicoRecomendacoesMutex.Lock()
	for _, h := range a.historicoRecomendacoes {
		ultimasHanzisSet[h] = true
	}
	a.historicoRecomendacoesMutex.Unlock()

	novas, err := a.gerarNovasRecomendacoesBaralho(ultimasHanzisSet)
	if err != nil {
		return nil, err
	}

	// Atualizar o histórico de exclusão recente com as novas recomendações
	a.historicoRecomendacoesMutex.Lock()
	for _, card := range novas {
		a.historicoRecomendacoes = append(a.historicoRecomendacoes, card.Hanzi)
	}
	if len(a.historicoRecomendacoes) > 30 {
		a.historicoRecomendacoes = a.historicoRecomendacoes[len(a.historicoRecomendacoes)-30:]
	}
	a.historicoRecomendacoesMutex.Unlock()

	return novas, nil
}

func (a *App) VirarCartaBaralho(hanzi string, revelada bool) error {
	return progresso.AtualizarReveladaBaralhoBanco(hanzi, revelada)
}

func (a *App) gerarNovasRecomendacoesBaralho(ultimasHanzisSet map[string]bool) ([]RecomendacaoBaralho, error) {
	vocabs, err := progresso.GetAllVocab()
	if err != nil {
		vocabs = nil
	}

	excluidasSet := make(map[string]bool)
	aprendidasSet := make(map[string]bool)
	emEstudoSet := make(map[string]bool)

	for h := range ultimasHanzisSet {
		excluidasSet[h] = true
	}

	for _, v := range vocabs {
		if v.Status == "estudo" || v.Status == "aprendido" {
			excluidasSet[v.Hanzi] = true
		}
		if v.Status == "aprendido" {
			aprendidasSet[v.Hanzi] = true
		}
		if v.Status == "estudo" {
			emEstudoSet[v.Hanzi] = true
		}
	}

	aprendidasExpandidasSet := make(map[string]bool)
	for h := range aprendidasSet {
		aprendidasExpandidasSet[h] = true
		if comp, ok := dicionario.MapaAbrevParaCompleto[h]; ok {
			aprendidasExpandidasSet[comp] = true
		}
		for abrev, comp := range dicionario.MapaAbrevParaCompleto {
			if comp == h || abrev == h {
				aprendidasExpandidasSet[abrev] = true
				aprendidasExpandidasSet[comp] = true
			}
		}
		if a.Dicionario != nil {
			if completo := a.Dicionario.CaractereCompleto(h); completo != "" {
				aprendidasExpandidasSet[completo] = true
			}
		}
	}

	usados := make(map[string]bool)
	var resultado []RecomendacaoBaralho

	// Helper para criar e adicionar card recomendada
	adicionarCard := func(hanzi, motivo string) bool {
		if hanzi == "" || usados[hanzi] {
			return false
		}
		usados[hanzi] = true
		pinyin, sigs, _ := a.Dicionario.Leitura(hanzi)
		card := RecomendacaoBaralho{
			Hanzi:       hanzi,
			Pinyin:      pinyin,
			Significado: strings.Join(sigs, ", "),
			Motivo:      motivo,
			Revelada:    false,
		}
		a.enriquecerRecomendacao(&card)
		resultado = append(resultado, card)
		return true
	}

	topFrequentes := a.Dicionario.TopPalavrasFrequentes(500)
	candPalavras := a.Dicionario.TopPalavrasFrequentes(3000)

	// =========================================================================
	// Slot 1: Progressão HSK (Adequado ao nível do usuário com amostragem inteligente)
	// =========================================================================
	contagemHskUsuario := make(map[int]int)
	for _, v := range vocabs {
		if v.Status == "aprendido" || v.Status == "estudo" {
			if lvl, ok := dicionario.TabelaHSK30[v.Hanzi]; ok && lvl > 0 {
				contagemHskUsuario[lvl]++
			}
		}
	}

	// Descobrir nível-alvo (HSK 1 até 6)
	nivelAlvo := 1
	for lvl := 1; lvl <= 6; lvl++ {
		limiar := 35
		if lvl == 1 {
			limiar = 30
		} else if lvl == 2 {
			limiar = 50
		} else if lvl >= 3 {
			limiar = 70
		}
		if contagemHskUsuario[lvl] >= limiar {
			nivelAlvo = lvl + 1
		} else {
			nivelAlvo = lvl
			break
		}
	}
	if nivelAlvo > 7 {
		nivelAlvo = 7
	}

	// Coleta palavras elegíveis do nível-alvo (ou níveis vizinhos se escasso)
	type candidatoSlot struct {
		palavra string
		pos     int
	}
	var candidatosHsk []candidatoSlot

	tentarNivel := func(lvl int) {
		for word, n := range dicionario.TabelaHSK30 {
			if n != lvl || excluidasSet[word] || usados[word] {
				continue
			}
			pos := 999999
			if freq := a.Dicionario.Banco.ObterFrequencia(word); freq != nil && freq.Posicao > 0 {
				pos = freq.Posicao
			}
			candidatosHsk = append(candidatosHsk, candidatoSlot{palavra: word, pos: pos})
		}
	}

	tentarNivel(nivelAlvo)
	if len(candidatosHsk) < 5 && nivelAlvo > 1 {
		tentarNivel(nivelAlvo - 1)
	}
	if len(candidatosHsk) < 5 && nivelAlvo < 7 {
		tentarNivel(nivelAlvo + 1)
	}

	if len(candidatosHsk) > 0 {
		sort.Slice(candidatosHsk, func(i, j int) bool {
			return candidatosHsk[i].pos < candidatosHsk[j].pos
		})
		limitePool := 35
		if len(candidatosHsk) < limitePool {
			limitePool = len(candidatosHsk)
		}
		idxSorteado := int(float64(limitePool) * (1.0 - math.Sqrt(rand.Float64())))
		if idxSorteado >= limitePool {
			idxSorteado = limitePool - 1
		}
		escolhido := candidatosHsk[idxSorteado].palavra
		nivelPalavra := dicionario.TabelaHSK30[escolhido]
		motivo := fmt.Sprintf("Palavra essencial do HSK %d (alta frequência)", nivelPalavra)
		adicionarCard(escolhido, motivo)
	}

	// =========================================================================
	// Slot 2: Efeito Lego (Composta por caracteres que o usuário já aprendeu)
	// =========================================================================
	type candidatoComposto struct {
		palavra     string
		componentes []string
		pos         int
	}
	var candidatosCompostos []candidatoComposto

	for _, w := range candPalavras {
		if excluidasSet[w] || usados[w] {
			continue
		}

		runas := []rune(w)
		if len(runas) > 1 {
			var comps []string
			todosConhecidos := true
			for _, r := range runas {
				charStr := string(r)
				charCompleto := a.Dicionario.CaractereCompleto(charStr)
				if aprendidasExpandidasSet[charStr] || aprendidasExpandidasSet[charCompleto] {
					comps = append(comps, charStr)
				} else {
					dec := a.Dicionario.DecomporHanzi(charStr)
					if dec != nil && dec.Decomposicao != "" {
						subConhecida := true
						for _, subR := range dec.Decomposicao {
							if unicode.Is(unicode.Han, subR) {
								subStr := string(subR)
								subCompl := a.Dicionario.CaractereCompleto(subStr)
								if !aprendidasExpandidasSet[subStr] && !aprendidasExpandidasSet[subCompl] {
									subConhecida = false
									break
								}
							}
						}
						if subConhecida {
							comps = append(comps, charStr)
						} else {
							todosConhecidos = false
							break
						}
					} else {
						todosConhecidos = false
						break
					}
				}
			}

			if todosConhecidos && len(comps) > 0 {
				pos := 999999
				if freq := a.Dicionario.Banco.ObterFrequencia(w); freq != nil && freq.Posicao > 0 {
					pos = freq.Posicao
				}
				candidatosCompostos = append(candidatosCompostos, candidatoComposto{
					palavra:     w,
					componentes: comps,
					pos:         pos,
				})
				if len(candidatosCompostos) >= 40 {
					break
				}
			}
		}
	}

	if len(candidatosCompostos) > 0 {
		sort.Slice(candidatosCompostos, func(i, j int) bool {
			return candidatosCompostos[i].pos < candidatosCompostos[j].pos
		})
		limitePool := 20
		if len(candidatosCompostos) < limitePool {
			limitePool = len(candidatosCompostos)
		}
		idxSorteado := rand.Intn(limitePool)
		escolhido := candidatosCompostos[idxSorteado]
		motivo := "Composta por caracteres que você já aprendeu: " + strings.Join(escolhido.componentes, ", ")
		adicionarCard(escolhido.palavra, motivo)
	} else {
		// Fallback inteligente: se o usuário ainda não tem caracteres suficientes para formar palavras compostas,
		// sugere um caractere fundamental de altíssima produtividade no idioma chinês
		fundamentais := []string{
			"人", "口", "日", "月", "木", "水", "火", "土", "金", "山",
			"手", "心", "女", "子", "大", "小", "车", "门", "目", "田",
			"白", "马", "言", "力", "方", "生", "中", "天", "文", "年",
		}
		var fundamentalEscolhido string
		for _, f := range fundamentais {
			if !excluidasSet[f] && !usados[f] {
				fundamentalEscolhido = f
				break
			}
		}
		if fundamentalEscolhido != "" {
			adicionarCard(fundamentalEscolhido, "Caractere fundamental: base estrutural de dezenas de palavras no chinês")
		}
	}

	// =========================================================================
	// Slot 3: Imersão Real (OCR) ou Vocabulário Frequente do Cotidiano
	// =========================================================================
	type candidatoOcr struct {
		palavra string
		vistas  int
	}
	var candidatosOcr []candidatoOcr

	for _, v := range vocabs {
		if v.Status == "visto" && v.VezesVistaOcr > 0 && !excluidasSet[v.Hanzi] && !usados[v.Hanzi] {
			candidatosOcr = append(candidatosOcr, candidatoOcr{palavra: v.Hanzi, vistas: v.VezesVistaOcr})
		}
	}

	if len(candidatosOcr) > 0 {
		sort.Slice(candidatosOcr, func(i, j int) bool {
			return candidatosOcr[i].vistas > candidatosOcr[j].vistas
		})
		limitePool := 12
		if len(candidatosOcr) < limitePool {
			limitePool = len(candidatosOcr)
		}
		idxSorteado := rand.Intn(limitePool)
		escolhido := candidatosOcr[idxSorteado]
		motivo := fmt.Sprintf("Vista %d vezes nos seus escaneamentos", escolhido.vistas)
		adicionarCard(escolhido.palavra, motivo)
	} else {
		// Fallback de imersão: vocabulário conversacional de altíssima frequência no dia a dia
		var candidatosFreq []string
		for _, w := range topFrequentes {
			if !excluidasSet[w] && !usados[w] {
				candidatosFreq = append(candidatosFreq, w)
				if len(candidatosFreq) >= 25 {
					break
				}
			}
		}
		if len(candidatosFreq) > 0 {
			escolhido := candidatosFreq[rand.Intn(len(candidatosFreq))]
			adicionarCard(escolhido, "Vocabulário frequente em conversas e mídias do dia a dia")
		}
	}

	// =========================================================================
	// Garantia de 3 Cartas (Fallback Geral se algum slot não produziu)
	// =========================================================================
	if len(resultado) < 3 {
		for _, w := range topFrequentes {
			if !excluidasSet[w] && !usados[w] {
				if adicionarCard(w, "Sugerida pelo baralho de vocabulário") {
					if len(resultado) >= 3 {
						break
					}
				}
			}
		}
	}

	// Se mesmo assim faltar (ex.: histórico de exclusão rigoroso demais), relaxa as exclusões do histórico recente
	if len(resultado) < 3 {
		for _, w := range topFrequentes {
			if !emEstudoSet[w] && !aprendidasSet[w] && !usados[w] {
				if adicionarCard(w, "Sugerida pelo baralho de vocabulário") {
					if len(resultado) >= 3 {
						break
					}
				}
			}
		}
	}

	// Persistir no banco de dados SQLite
	var itensBanco []progresso.ItemRecomendacaoBaralho
	for _, item := range resultado {
		itensBanco = append(itensBanco, progresso.ItemRecomendacaoBaralho{
			Hanzi:       item.Hanzi,
			Pinyin:      item.Pinyin,
			Significado: item.Significado,
			Motivo:      item.Motivo,
			Revelada:    item.Revelada,
			DataAdd:     time.Now(),
		})
	}
	_ = progresso.SalvarRecomendacoesBaralhoBanco(itensBanco)

	return resultado, nil
}

