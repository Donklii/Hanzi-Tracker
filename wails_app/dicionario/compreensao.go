package dicionario

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"math/rand/v2"
	"strings"
	"sync"
	"unicode"
	"unicode/utf8"
)

// ----- Seção: Estruturas de Dados -----

// PerguntaCompreensao representa um exercício de compreensão de leitura com contexto e múltipla escolha.
type PerguntaCompreensao struct {
	Contexto              string   `json:"contexto"`
	Pergunta              string   `json:"pergunta"`
	Opcoes                []string `json:"opcoes"`
	IndiceRespostaCorreta int      `json:"indice_resposta_correta"`
	PerguntaTraduzida     string   `json:"pergunta_traduzida"`
	ContextoTraduzido     string   `json:"contexto_traduzido"`
	Tema                  string   `json:"tema"`
	Dificuldade           string   `json:"dificuldade"`
	Atribuicao            string   `json:"atribuicao"`
	Script                string   `json:"script"`
	Categoria             string   `json:"categoria,omitempty"`
}

// DialogoSimples representa um bate-volta de exatas duas falas.
type DialogoSimples struct {
	Linha1            string
	Linha2            string
	PalavrasUinicasL2 map[string]bool
	TraducaoL1        string
	TraducaoL2        string
}

// GerenciadorCompreensao é o repositório de perguntas de compreensão em memória.
type GerenciadorCompreensao struct {
	carregarUmaVez sync.Once
	erroCarga      error

	mu        sync.RWMutex
	perguntas []PerguntaCompreensao
	dialogos  []DialogoSimples
	
	indice          map[rune][]int32
	indiceDialogos  map[rune][]int32
	textos          map[string]bool

	fontesExtras []func() ([]PerguntaCompreensao, error)
}

// ----- Seção: Inicialização e Carga -----

// NovoGerenciadorCompreensao cria uma nova instância do repositório de compreensão.
func NovoGerenciadorCompreensao() *GerenciadorCompreensao {
	return &GerenciadorCompreensao{
		indice:         make(map[rune][]int32),
		indiceDialogos: make(map[rune][]int32),
		textos:         make(map[string]bool),
	}
}

// RegistrarFonteExtra adiciona um provedor de perguntas de compreensão (ex: SQLite).
func (g *GerenciadorCompreensao) RegistrarFonteExtra(fn func() ([]PerguntaCompreensao, error)) {
	g.fontesExtras = append(g.fontesExtras, fn)
}

func (g *GerenciadorCompreensao) garantirCarregado() error {
	g.carregarUmaVez.Do(func() {
		if g.indice == nil {
			g.indice = make(map[rune][]int32)
		}
		if g.textos == nil {
			g.textos = make(map[string]bool)
		}

		diretoriosCompreensao := []string{
			"idiomas/compartilhado/compreensao",
			"idiomas/en/compreensao",
		}
		for _, dirCompreensao := range diretoriosCompreensao {
			entradas, err := arquivosIdiomas.ReadDir(dirCompreensao)
			if err == nil {
				for _, entrada := range entradas {
					if !entrada.IsDir() && strings.HasSuffix(entrada.Name(), ".jsonl.gz") {
						caminho := dirCompreensao + "/" + entrada.Name()
						if err := g.carregarArquivo(caminho); err != nil {
							g.erroCarga = err
							return
						}
					}
				}
			}
		}

		for _, fonte := range g.fontesExtras {
			extras, err := fonte()
			if err != nil {
				fmt.Printf("Aviso: fonte extra de perguntas de compreensão falhou: %v\n", err)
				continue
			}
			for _, p := range extras {
				g.adicionarPerguntaInterna(p)
			}
		}
	})

	return g.erroCarga
}

// carregarArquivo lê um JSONL gzipado do embed e o carrega em memória
func (g *GerenciadorCompreensao) carregarArquivo(caminho string) error {
	dados, err := arquivosIdiomas.ReadFile(caminho)
	if err != nil {
		return fmt.Errorf("não foi possível ler o arquivo de compreensão %q: %w", caminho, err)
	}

	leitorGz, err := gzip.NewReader(bytes.NewReader(dados))
	if err != nil {
		return fmt.Errorf("arquivo de compreensão %q corrompido: %w", caminho, err)
	}
	defer leitorGz.Close()

	varredor := bufio.NewScanner(leitorGz)
	varredor.Buffer(make([]byte, 1024*1024), 1024*1024)

	for varredor.Scan() {
		linha := varredor.Bytes()
		if len(linha) == 0 {
			continue
		}
		var p PerguntaCompreensao
		if err := json.Unmarshal(linha, &p); err != nil {
			continue
		}
		g.adicionarPerguntaInterna(p)
	}

	return varredor.Err()
}

func (g *GerenciadorCompreensao) adicionarPerguntaInterna(p PerguntaCompreensao) {
	if p.Contexto == "" || g.textos[p.Contexto] {
		return
	}
	g.textos[p.Contexto] = true

	idx := int32(len(g.perguntas))
	g.perguntas = append(g.perguntas, p)

	vistos := make(map[rune]bool)
	for _, r := range p.Contexto {
		if vistos[r] {
			continue
		}
		vistos[r] = true
		g.indice[r] = append(g.indice[r], idx)
	}

	if p.Categoria != "d" && p.Categoria != "" {
		return
	}

	partes := strings.Split(p.Contexto, "\n")
	if len(partes) != 2 {
		return
	}

	idxD := int32(len(g.dialogos))
	
	palavrasL2 := make(map[string]bool)
	for _, r := range partes[1] {
		if unicode.Is(unicode.Han, r) {
			palavrasL2[string(r)] = true
		}
	}

	partesTrad := strings.Split(p.ContextoTraduzido, "\n")
	tradL1, tradL2 := "", ""
	if len(partesTrad) == 2 {
		tradL1 = partesTrad[0]
		tradL2 = partesTrad[1]
	} else {
		tradL1 = p.ContextoTraduzido
	}
	
	d := DialogoSimples{
		Linha1:            partes[0],
		Linha2:            partes[1],
		PalavrasUinicasL2: palavrasL2,
		TraducaoL1:        tradL1,
		TraducaoL2:        tradL2,
	}
	g.dialogos = append(g.dialogos, d)

	vistosD := make(map[rune]bool)
	for _, r := range p.Contexto {
		if vistosD[r] {
			continue
		}
		vistosD[r] = true
		g.indiceDialogos[r] = append(g.indiceDialogos[r], idxD)
	}
}

// AdicionarPergunta injeta uma pergunta de compreensão no repositório em runtime.
func (g *GerenciadorCompreensao) AdicionarPergunta(p PerguntaCompreensao) bool {
	if err := g.garantirCarregado(); err != nil {
		return false
	}
	g.mu.Lock()
	defer g.mu.Unlock()

	if g.textos[p.Contexto] {
		return false
	}
	g.adicionarPerguntaInterna(p)
	return true
}

// RemoverPergunta remove uma pergunta do acervo em memória pelo texto do contexto.
func (g *GerenciadorCompreensao) RemoverPergunta(contexto string) bool {
	if err := g.garantirCarregado(); err != nil {
		return false
	}
	g.mu.Lock()
	defer g.mu.Unlock()

	if !g.textos[contexto] {
		return false
	}

	delete(g.textos, contexto)

	pos := -1
	for i, p := range g.perguntas {
		if p.Contexto == contexto {
			pos = i
			break
		}
	}
	if pos != -1 {
		g.perguntas = append(g.perguntas[:pos], g.perguntas[pos+1:]...)
	}

	g.reindexarInterno()
	return true
}

func (g *GerenciadorCompreensao) reindexarInterno() {
	g.indice = make(map[rune][]int32)
	for i, p := range g.perguntas {
		vistos := make(map[rune]bool)
		for _, r := range p.Contexto {
			if vistos[r] {
				continue
			}
			vistos[r] = true
			g.indice[r] = append(g.indice[r], int32(i))
		}
	}
}

// LimparParaTestes limpa o acervo do repositório para evitar que testes unitários consumam a base real.
func (g *GerenciadorCompreensao) LimparParaTestes() {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.perguntas = nil
	g.dialogos = nil
	g.indice = make(map[rune][]int32)
	g.indiceDialogos = make(map[rune][]int32)
	g.textos = make(map[string]bool)
}

// ----- Seção: Consultas e Seleção -----

// PerguntasComCaractere devolve as perguntas de compreensão cujo contexto contém o caractere dado.
func (g *GerenciadorCompreensao) PerguntasComCaractere(caractere rune) []PerguntaCompreensao {
	if err := g.garantirCarregado(); err != nil {
		return nil
	}

	g.mu.RLock()
	defer g.mu.RUnlock()

	indices := g.indice[caractere]
	res := make([]PerguntaCompreensao, 0, len(indices))
	for _, idx := range indices {
		res = append(res, g.perguntas[idx])
	}
	return res
}

// PerguntasComPalavra devolve as perguntas cujo contexto contém a palavra inteira.
func (g *GerenciadorCompreensao) PerguntasComPalavra(palavra string) []PerguntaCompreensao {
	if palavra == "" {
		return nil
	}
	if err := g.garantirCarregado(); err != nil {
		return nil
	}

	primeira, _ := utf8.DecodeRuneInString(palavra)

	g.mu.RLock()
	defer g.mu.RUnlock()

	indices := g.indice[primeira]
	res := make([]PerguntaCompreensao, 0, len(indices))
	for _, idx := range indices {
		if strings.Contains(g.perguntas[idx].Contexto, palavra) {
			res = append(res, g.perguntas[idx])
		}
	}
	return res
}

// TotalPerguntas devolve a quantidade total de perguntas no repositório.
func (g *GerenciadorCompreensao) TotalPerguntas() int {
	if err := g.garantirCarregado(); err != nil {
		return 0
	}
	g.mu.RLock()
	defer g.mu.RUnlock()
	return len(g.perguntas)
}

// SelecionarPonderada escolhe uma pergunta por roleta ponderada de acordo com o status dos caracteres.
func (g *GerenciadorCompreensao) SelecionarPonderada(perguntas []PerguntaCompreensao, status map[string]string) PerguntaCompreensao {
	if len(perguntas) == 0 {
		return PerguntaCompreensao{}
	}
	if len(perguntas) == 1 || len(status) == 0 {
		return perguntas[rand.IntN(len(perguntas))]
	}

	pesos := make([]int, len(perguntas))
	pesoTotal := 0
	for i, p := range perguntas {
		pontos := 0
		unicos := 0
		vistos := make(map[rune]bool)
		for _, r := range p.Contexto {
			if vistos[r] || !unicode.Is(unicode.Han, r) {
				continue
			}
			vistos[r] = true
			unicos++
			switch status[string(r)] {
			case StatusAprendido:
				pontos += pesoFraseAprendido
			case StatusEstudo:
				pontos += pesoFraseEstudo
			}
		}

		peso := pesoFraseBase
		if unicos > 0 {
			peso += pontos / unicos
		}
		pesos[i] = peso
		pesoTotal += peso
	}

	return perguntas[sortearPorRoleta(pesos, pesoTotal)]
}

// DialogosComCaractere devolve os diálogos cujo contexto contém o caractere dado.
func (g *GerenciadorCompreensao) DialogosComCaractere(caractere rune) []DialogoSimples {
	if err := g.garantirCarregado(); err != nil {
		return nil
	}

	g.mu.RLock()
	defer g.mu.RUnlock()

	indices, ok := g.indiceDialogos[caractere]
	if !ok {
		return nil
	}

	res := make([]DialogoSimples, len(indices))
	for i, idx := range indices {
		res[i] = g.dialogos[idx]
	}
	return res
}

// DialogosComPalavra devolve os diálogos cujo contexto contém a palavra.
func (g *GerenciadorCompreensao) DialogosComPalavra(palavra string) []DialogoSimples {
	if err := g.garantirCarregado(); err != nil {
		return nil
	}

	runas := []rune(palavra)
	if len(runas) == 0 {
		return nil
	}

	var possiveis []DialogoSimples
	if len(runas) == 1 {
		possiveis = g.DialogosComCaractere(runas[0])
	} else {
		g.mu.RLock()
		indices, ok := g.indiceDialogos[runas[0]]
		if ok {
			for _, idx := range indices {
				possiveis = append(possiveis, g.dialogos[idx])
			}
		}
		g.mu.RUnlock()
	}

	if len(runas) <= 1 {
		return possiveis
	}

	filtradas := make([]DialogoSimples, 0, len(possiveis))
	for _, d := range possiveis {
		if strings.Contains(d.Linha1, palavra) || strings.Contains(d.Linha2, palavra) {
			filtradas = append(filtradas, d)
		}
	}
	return filtradas
}

func limparPrefixoFala(texto string) string {
	texto = strings.TrimSpace(texto)
	runas := []rune(texto)
	for i, r := range runas {
		if r == ':' || r == '：' {
			if i > 0 && i <= 5 {
				return strings.TrimSpace(string(runas[i+1:]))
			}
			break
		}
	}
	return texto
}

func absInt(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

// BuscarDistratoresDialogo busca outros diálogos para servirem de opções incorretas,
// garantindo proximidade no número de caracteres da resposta, similaridade vocabular e sem alternativas duplicadas.
func (g *GerenciadorCompreensao) BuscarDistratoresDialogo(correta DialogoSimples, quantidade int, validador ...func(texto string) bool) []string {
	if err := g.garantirCarregado(); err != nil || len(g.dialogos) == 0 {
		return nil
	}

	g.mu.RLock()
	defer g.mu.RUnlock()

	fnValidador := func(texto string) bool {
		if len(validador) > 0 && validador[0] != nil {
			return validador[0](texto)
		}
		return true
	}

	total := len(g.dialogos)
	candidatos := make([]string, 0, quantidade)
	vistos := make(map[string]bool)

	corretaLimpa := limparPrefixoFala(correta.Linha2)
	vistos[corretaLimpa] = true

	tamanhoCorreta := utf8.RuneCountInString(corretaLimpa)
	totalCorreta := len(correta.PalavrasUinicasL2)
	offset := rand.IntN(total)

	adicionarDistrator := func(texto string, aplicarValidador bool) bool {
		limpo := limparPrefixoFala(texto)
		if vistos[limpo] || limpo == "" {
			return false
		}
		if aplicarValidador && !fnValidador(texto) {
			return false
		}
		vistos[limpo] = true
		candidatos = append(candidatos, texto)
		return len(candidatos) == quantidade
	}

	// 1. Prioridade alta: mesma faixa de tamanho (diferença <= 3 caracteres) E similaridade vocabular (>= 30%)
	if totalCorreta > 0 {
		for i := 0; i < total; i++ {
			idx := (offset + i) % total
			d := g.dialogos[idx]

			tamanhoCand := utf8.RuneCountInString(limparPrefixoFala(d.Linha2))
			if absInt(tamanhoCand-tamanhoCorreta) > 3 {
				continue
			}

			intersecao := 0
			for palavra := range d.PalavrasUinicasL2 {
				if correta.PalavrasUinicasL2[palavra] {
					intersecao++
				}
			}

			similaridade := float64(intersecao) / float64(totalCorreta)
			if similaridade >= 0.3 {
				if adicionarDistrator(d.Linha2, true) {
					return candidatos
				}
			}
		}
	}

	// 2. Prioridade média: mesma faixa de tamanho (diferença <= 4 caracteres)
	for i := 0; i < total; i++ {
		idx := (offset + i) % total
		d := g.dialogos[idx]
		tamanhoCand := utf8.RuneCountInString(limparPrefixoFala(d.Linha2))
		if absInt(tamanhoCand-tamanhoCorreta) <= 4 {
			if adicionarDistrator(d.Linha2, true) {
				return candidatos
			}
		}
	}

	// 3. Prioridade baixa: diferença de tamanho <= 8 caracteres
	for i := 0; i < total; i++ {
		idx := (offset + i) % total
		d := g.dialogos[idx]
		tamanhoCand := utf8.RuneCountInString(limparPrefixoFala(d.Linha2))
		if absInt(tamanhoCand-tamanhoCorreta) <= 8 {
			if adicionarDistrator(d.Linha2, true) {
				return candidatos
			}
		}
	}

	// 4. Se não preencheu com o validador ativo, faz uma passada de fallback permitindo qualquer distrator válido
	if len(candidatos) < quantidade {
		for i := 0; i < total; i++ {
			idx := (offset + i) % total
			d := g.dialogos[idx]
			if adicionarDistrator(d.Linha2, false) {
				return candidatos
			}
		}
	}

	return candidatos
}
