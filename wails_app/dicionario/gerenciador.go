package dicionario

import (
	"fmt"
	"sort"
	"strings"
)

// ----- Gerenciador de Dicionário: fonte única de consulta (single source of truth) -----
//
// GerenciadorDicionario é o ÚNICO ponto de acesso às consultas de dicionário usadas por pop-ups,
// cards e revisões. Ele possui os dois bancos (o dicionário fundido e os traçados) e centraliza:
//   - busca de significado e leitura (pinyin) de palavras/hanzis;
//   - decomposição de palavras (segmentação por dicionário) e de hanzis (etimologia/radicais);
//   - caracteres compostos por um dado componente;
//   - classificação/conversão simplificado↔tradicional.
//
// PRIORIDADE (não replicar essa decisão fora daqui): quem manda é o campo "definicao" do topo da
// entrada fundida. A fusão já decidiu de onde ele veio — definição curada do makemeahanzi quando o
// caractere tem uma, senão a 1ª leitura do CEDICT promovida ao topo. Não existe mais escolha por
// FONTE em runtime; o que separa caractere de palavra composta é a contagem de hanzis, e só onde
// isso muda a resposta (ver DecomporHanzi e TemEntrada).
//
// Os bancos ficam expostos como campos públicos porque o gerenciador é o dono deles (padrão
// gerenciador-de-gerenciadores): consultas especializadas de um único banco (ex.: candidatos da
// revisão, dados de traçado) são feitas por eles diretamente, sempre a partir deste gerenciador.

// PrefixoDesenho marca uma busca originada do seletor "buscar por desenho" (ModalBuscaPorDesenho no
// frontend): o restante do termo são os caracteres literais a consultar, sem varredura textual.
const PrefixoDesenho = "[DESENHO]"
const PrefixoRanking = "[RANKING]"
const PrefixoHSK = "[HSK]"

type GerenciadorDicionario struct {
	Banco    *Banco
	Tracados *BancoTracados

	// distratores é o pool preguiçoso de componentes usado para sortear cartas falsas parecidas na
	// atividade de montagem (ver componentes_distratores.go).
	distratores poolDistratores
}

// ResultadoConsulta é o DTO neutro de uma busca no dicionário (sem campos de apresentação). O
// frontend/cards embrulham isso no seu próprio formato (ex.: FlashcardCard) quando precisam.
type ResultadoConsulta struct {
	Hanzi        string   `json:"hanzi"`
	Pinyin       string   `json:"pinyin"`
	Significados []string `json:"significados"`
	TipoHanzi    string   `json:"tipoHanzi"`
	NivelHSK     int      `json:"nivelHSK,omitempty"`
}

// ----- Inicialização -----

// NovoGerenciadorDicionario constrói o gerenciador e carrega o dicionário fundido do idioma. O banco
// de traçados é preguiçoso (só paga memória na revisão de desenho), então não é carregado aqui.
// Devolve o gerenciador já utilizável mesmo em caso de erro de carga, junto com o erro — o chamador
// decide se apenas avisa (o app segue degradado, sem dicionário).
func NovoGerenciadorDicionario(idioma string) (*GerenciadorDicionario, error) {
	gerenciador := &GerenciadorDicionario{
		Banco:    NovoBanco(),
		Tracados: NovoBancoTracados(),
	}

	if err := gerenciador.Banco.Carregar(idioma); err != nil {
		return gerenciador, fmt.Errorf("falha ao carregar o dicionário: %w", err)
	}
	return gerenciador, nil
}

// ----- Busca de significado e leitura -----

// BuscarPalavra devolve as acepções de uma palavra, já na ordem de prioridade do banco (a do topo
// primeiro — ver o cabeçalho).
func (g *GerenciadorDicionario) BuscarPalavra(palavra string) []EntradaDicionario {
	return g.Banco.Buscar(palavra)
}

// Leitura resolve a leitura (pinyin) e as acepções de um hanzi ou palavra. Devolve também a entrada
// casada (nil quando não há nenhuma) — o scan de OCR a usa para converter o card ao tipo configurado.
func (g *GerenciadorDicionario) Leitura(hanzi string) (pinyin string, significados []string, entrada *EntradaDicionario) {
	entradas := g.Banco.Buscar(hanzi)
	if len(entradas) == 0 {
		return "", nil, nil
	}
	return entradas[0].Pinyin, entradas[0].Significados, &entradas[0]
}

// BuscaGeral pesquisa por hanzi, pinyin ou significado. O prefixo PrefixoDesenho restringe a busca
// aos caracteres literais fornecidos, sem varredura textual.
func (g *GerenciadorDicionario) BuscaGeral(termo string) []ResultadoConsulta {
	if strings.HasPrefix(termo, PrefixoDesenho) {
		return g.buscarCaracteresLiterais(strings.TrimPrefix(termo, PrefixoDesenho))
	}
	if strings.HasPrefix(termo, PrefixoHSK) {
		return g.buscarPalavrasHSK(strings.TrimPrefix(termo, PrefixoHSK))
	}
	if strings.HasPrefix(termo, PrefixoRanking) {
		return g.buscarPalavrasPorRanking(strings.TrimPrefix(termo, PrefixoRanking))
	}

	var resultados []ResultadoConsulta
	vistos := make(map[string]bool)
	for _, e := range g.Banco.BuscarGeral(termo) {
		if vistos[e.Simplificado] {
			continue
		}
		vistos[e.Simplificado] = true
		resultados = append(resultados, g.montarResultado(e))

		// Se a versão tradicional for diferente da simplificada, adiciona ela também
		if e.Tradicional != "" && e.Tradicional != e.Simplificado {
			resultados = append(resultados, ResultadoConsulta{
				Hanzi:        e.Tradicional,
				Pinyin:       e.Pinyin,
				Significados: e.Significados,
				TipoHanzi:    "Tradicional",
			})
		}
	}
	return resultados
}

// buscarPalavrasHSK devolve todo o vocabulário oficial cobrado pelo HSK 3.0 (níveis 1 a 7),
// garantindo que apenas Chinês Simplificado seja retornado.
func (g *GerenciadorDicionario) buscarPalavrasHSK(termo string) []ResultadoConsulta {
	var resultados []ResultadoConsulta
	vistos := make(map[string]bool)

	termoLower := strings.ToLower(strings.TrimSpace(termo))

	for word, nivel := range TabelaHSK30 {
		if vistos[word] {
			continue
		}

		entradas := g.Banco.Buscar(word)
		if len(entradas) == 0 {
			continue
		}

		e := entradas[0]

		if termoLower != "" {
			combinaHanzi := strings.Contains(e.Simplificado, termoLower)
			combinaPinyin := strings.Contains(strings.ToLower(e.Pinyin), termoLower)
			combinaSignificado := false
			for _, sig := range e.Significados {
				if strings.Contains(strings.ToLower(sig), termoLower) {
					combinaSignificado = true
					break
				}
			}
			if !combinaHanzi && !combinaPinyin && !combinaSignificado {
				continue
			}
		}

		vistos[word] = true
		res := g.montarResultado(e)
		res.NivelHSK = nivel
		res.TipoHanzi = "Simplificado"
		resultados = append(resultados, res)
	}

	sort.Slice(resultados, func(i, j int) bool {
		if resultados[i].NivelHSK != resultados[j].NivelHSK {
			return resultados[i].NivelHSK < resultados[j].NivelHSK
		}
		freqI := g.Banco.ObterFrequencia(resultados[i].Hanzi)
		freqJ := g.Banco.ObterFrequencia(resultados[j].Hanzi)
		posI := 999999
		if freqI != nil && freqI.Posicao > 0 {
			posI = freqI.Posicao
		}
		posJ := 999999
		if freqJ != nil && freqJ.Posicao > 0 {
			posJ = freqJ.Posicao
		}
		if posI != posJ {
			return posI < posJ
		}
		return resultados[i].Hanzi < resultados[j].Hanzi
	})

	return resultados
}

// buscarPalavrasPorRanking encontra todos os verbetes que contêm o termo de pesquisa e ordena-os pelo
// ranking de frequência no corpus (posições mais baixas/mais frequentes primeiro).
func (g *GerenciadorDicionario) buscarPalavrasPorRanking(termo string) []ResultadoConsulta {
	var match []EntradaDicionario
	vistos := make(map[string]bool)

	for _, entradas := range g.Banco.entradas {
		for _, e := range entradas {
			if vistos[e.Simplificado] {
				continue
			}
			if termo == "" {
				// No ranking geral, só coletamos palavras que realmente possuem posição de frequência
				freq := g.Banco.ObterFrequencia(e.Simplificado)
				if freq == nil || freq.Posicao <= 0 {
					continue
				}
				vistos[e.Simplificado] = true
				match = append(match, e)
			} else {
				if strings.Contains(e.Simplificado, termo) || (e.Tradicional != "" && strings.Contains(e.Tradicional, termo)) {
					vistos[e.Simplificado] = true
					match = append(match, e)
				}
			}
		}
	}

	sort.Slice(match, func(i, j int) bool {
		freqI := g.Banco.ObterFrequencia(match[i].Simplificado)
		freqJ := g.Banco.ObterFrequencia(match[j].Simplificado)

		posI := 0
		if freqI != nil {
			posI = freqI.Posicao
		}
		posJ := 0
		if freqJ != nil {
			posJ = freqJ.Posicao
		}

		if posI > 0 && posJ > 0 {
			return posI < posJ
		}
		if posI > 0 {
			return true
		}
		if posJ > 0 {
			return false
		}
		return len(match[i].Simplificado) < len(match[j].Simplificado)
	})

	limite := 3000
	if termo == "" {
		limite = 15000
	}

	var resultados []ResultadoConsulta
	for _, e := range match {
		if len(resultados) >= limite {
			break
		}
		resultados = append(resultados, g.montarResultado(e))

		if e.Tradicional != "" && e.Tradicional != e.Simplificado {
			resultados = append(resultados, ResultadoConsulta{
				Hanzi:        e.Tradicional,
				Pinyin:       e.Pinyin,
				Significados: e.Significados,
				TipoHanzi:    "Tradicional",
			})
		}
	}
	return resultados
}

// buscarCaracteresLiterais consulta um a um os caracteres desenhados pelo usuário.
func (g *GerenciadorDicionario) buscarCaracteresLiterais(caracteres string) []ResultadoConsulta {
	var resultados []ResultadoConsulta
	vistos := make(map[string]bool)

	for _, r := range caracteres {
		caractere := string(r)
		if caractere == " " {
			continue
		}
		for _, e := range g.Banco.Buscar(caractere) {
			if vistos[e.Simplificado] {
				continue
			}
			vistos[e.Simplificado] = true
			resultados = append(resultados, g.montarResultado(e))

			// Se a versão tradicional for diferente da simplificada, adiciona ela também
			if e.Tradicional != "" && e.Tradicional != e.Simplificado {
				resultados = append(resultados, ResultadoConsulta{
					Hanzi:        e.Tradicional,
					Pinyin:       e.Pinyin,
					Significados: e.Significados,
					TipoHanzi:    "Tradicional",
				})
			}
		}
	}
	return resultados
}

// ----- Decomposição de palavras e hanzis -----

// SegmentarPorDicionario quebra um texto OOV (Out-Of-Vocabulary) em palavras válidas do dicionário
// via Forward Maximum Matching (tenta sempre a maior substring que tenha entrada).
func (g *GerenciadorDicionario) SegmentarPorDicionario(texto string) []string {
	var resultado []string
	runas := []rune(texto)

	for i := 0; i < len(runas); {
		casou := false
		for j := len(runas); j > i; j-- {
			sub := string(runas[i:j])
			if !g.TemEntrada(sub) {
				continue
			}
			resultado = append(resultado, sub)
			i = j
			casou = true
			break
		}

		if casou {
			continue
		}
		// Prevenção de loop infinito (embora caractere isolado sempre dê match).
		resultado = append(resultado, string(runas[i:i+1]))
		i++
	}

	return resultado
}

// DecomporHanzi devolve a decomposição/etimologia de um caractere (nil se desconhecido).
func (g *GerenciadorDicionario) DecomporHanzi(caractere string) *DecomposicaoHanzi {
	return g.Banco.BuscarCaractere(caractere)
}

// CompostosPor devolve os caracteres que usam o dado como componente de decomposição.
func (g *GerenciadorDicionario) CompostosPor(componente string) []string {
	return g.Banco.BuscarCompostosPor(componente)
}

// TopPalavrasFrequentes devolve as palavras mais frequentes do dicionário.
func (g *GerenciadorDicionario) TopPalavrasFrequentes(limite int) []string {
	if g == nil || g.Banco == nil {
		return nil
	}
	return g.Banco.ObterTopPalavrasFrequentes(limite)
}

// ----- Utilitários de classificação, pinyin e conversão -----

// TemEntrada diz se a palavra vale como token/card próprio: caractere isolado sempre passa (fallback
// universal); palavra composta precisa de entrada no dicionário.
func (g *GerenciadorDicionario) TemEntrada(palavra string) bool {
	if ehCaractereUnico(palavra) {
		return true
	}
	return len(g.Banco.Buscar(palavra)) > 0
}

// TipoHanzi classifica o hanzi como "Tradicional", "Simplificado", "Ambos" ou "" (desconhecido).
func (g *GerenciadorDicionario) TipoHanzi(hanzi string) string {
	return g.Banco.AvaliarTipoHanzi(hanzi)
}

// CaractereCompleto resolve uma abreviação visual/radical para o caractere CJK completo equivalente.
func (g *GerenciadorDicionario) CaractereCompleto(abrev string) string {
	return g.Banco.CaractereCompleto(abrev)
}

// BuscarPorPinyin devolve os hanzis simplificados que casam com o pinyin dado (sem tom/espaço).
func (g *GerenciadorDicionario) BuscarPorPinyin(pinyin string) []string {
	return g.Banco.BuscarPorPinyin(pinyin)
}

// ConverterTexto converte um texto entre "simplificado" e "tradicional" caractere a caractere.
func (g *GerenciadorDicionario) ConverterTexto(texto string, alvo string) string {
	return g.Banco.ConverterTexto(texto, alvo)
}

// FraseCompativelComTipo diz se a frase não contém caracteres exclusivos do tipo oposto ao desejado.
func (g *GerenciadorDicionario) FraseCompativelComTipo(frase string, tipoDesejado string) bool {
	return g.Banco.FraseCompativelComTipo(frase, tipoDesejado)
}

// TotalHanzis devolve a quantidade de caracteres com dados curados (decomposição/radical).
func (g *GerenciadorDicionario) TotalHanzis() int {
	return g.Banco.TotalHanzis()
}

// montarResultado embrulha uma acepção no DTO neutro de consulta.
func (g *GerenciadorDicionario) montarResultado(e EntradaDicionario) ResultadoConsulta {
	return ResultadoConsulta{
		Hanzi:        e.Simplificado,
		Pinyin:       e.Pinyin,
		Significados: e.Significados,
		TipoHanzi:    g.TipoHanzi(e.Simplificado),
	}
}
