package dicionario

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"fmt"
	"math/rand/v2"
	"strings"
	"sync"
	"unicode"
	"unicode/utf8"
)

// ----- Gerenciador de Frases (módulo irmão do GerenciadorDicionario) -----
//
// Repositório de pares de frase chinês→inglês usados pelos modos de revisão "por contexto" e
// "desenho guiado por contexto". Enquanto o GerenciadorDicionario centraliza significado/leitura/
// decomposição, este centraliza o acervo de frases — e é ESCALÁVEL por dados: todo arquivo colocado
// no diretório frases/ com o sufixo abaixo é carregado automaticamente, sem tocar em código.
//
// Como adicionar um novo corpus de frases:
//   1. Gere um TSV gzipado com ao menos TRÊS colunas por linha: chinês \t tradução \t atribuição.
//      Opcionalmente, mais duas colunas: tema \t dificuldade (fundidas por dicionario/frases/fundir).
//   2. Salve como frases/<nome>.tsv.gz (o padrão de embed abaixo o inclui na próxima compilação).
// A atribuição por frase é preservada e deve ser exibida ao usuário (licenças em LICENCAS-DADOS.md).
//
// Os arquivos são carregados na PRIMEIRA consulta (sync.Once): quem nunca abre a revisão não paga o
// custo de memória do acervo.

const (
	sufixoArquivoFrases = ".tsv.gz"
	colunasMinimasFrase = 3 // chinês, tradução, atribuição
	colunaTema          = 3 // opcional: tema da frase (fundido por dicionario/frases/fundir)
	colunaDificuldade   = 4 // opcional: grau de dificuldade da frase
)

type Frase struct {
	Chines     string `json:"chines"`
	Ingles     string `json:"ingles"`
	Atribuicao string `json:"atribuicao"`
	// Tema e Dificuldade vêm da classificação fundida no arquivo (colunas opcionais 4 e 5). Ficam
	// vazios em frases sem rótulo (adicionadas em runtime, ou sem classificação). Rótulos em chinês:
	// tema de taxonomia fechada; dificuldade em 入门/初级/中级/高级 (iniciante/fácil/médio/avançado).
	Tema        string `json:"tema"`
	Dificuldade string `json:"dificuldade"`
}

type GerenciadorFrases struct {
	// idioma escolhido para o acervo; frases desse idioma, com fallback para o inglês (ver dirFrasesIdioma).
	idioma string

	carregarUmaVez sync.Once
	erroCarga      error

	// mu protege frases/indice/textos contra corrida entre consultas (leitura) e AdicionarFrase
	// (escrita em runtime). A carga inicial roda dentro de sync.Once, antes de qualquer leitura.
	mu     sync.RWMutex
	frases []Frase
	indice map[rune][]int32 // caractere -> índices das frases que o contêm
	textos map[string]bool  // textos chineses já presentes (dedup de adições em runtime)

	// fontesExtras são consultadas UMA vez ao fim da carga preguiçosa, depois dos arquivos
	// embarcados. Servem para injetar frases de fora do embed (ex.: frases do usuário salvas no
	// SQLite) sem acoplar este pacote ao banco — o chamador registra um provedor. Ver RegistrarFonteExtra.
	fontesExtras []func() ([]Frase, error)
}

// NovoGerenciadorFrases cria o acervo para o idioma dado. As frases desse idioma são carregadas na
// primeira consulta; se o idioma ainda não tiver banco de frases próprio, cai para o inglês.
func NovoGerenciadorFrases(idioma string) *GerenciadorFrases {
	return &GerenciadorFrases{idioma: idioma}
}

// RegistrarFonteExtra adiciona um provedor de frases consultado na carga (após os arquivos
// embarcados). Deve ser chamado ANTES da primeira consulta ao acervo. Idempotência/dedup por texto
// é garantida por adicionarFraseInterna.
func (g *GerenciadorFrases) RegistrarFonteExtra(fn func() ([]Frase, error)) {
	g.fontesExtras = append(g.fontesExtras, fn)
}

// ----- Carga (preguiçosa, dirigida a dados) -----

// garantirCarregado carrega, na primeira chamada, TODOS os arquivos de frases embarcados no
// diretório frases/. Falha barulhenta: se qualquer arquivo estiver corrompido, o erro é retido e
// propagado às consultas (que devolvem vazio), em vez de silenciar um acervo pela metade.
func (g *GerenciadorFrases) garantirCarregado() error {
	g.carregarUmaVez.Do(func() {
		g.indice = make(map[rune][]int32)
		g.textos = make(map[string]bool)

		dir := dirFrasesIdioma(g.idioma)
		nomes, err := listarArquivosFrases(dir)
		if err != nil {
			g.erroCarga = fmt.Errorf("não foi possível listar o diretório de frases: %w", err)
			return
		}

		for _, nome := range nomes {
			if err := g.carregarArquivo(dir, nome); err != nil {
				g.erroCarga = err
				return
			}
		}

		// Frases de fora do embed (ex.: salvas pelo usuário no SQLite). Falha de uma fonte extra
		// não invalida o acervo embarcado: registra e segue (a revisão ainda funciona sem elas).
		for _, fonte := range g.fontesExtras {
			extras, err := fonte()
			if err != nil {
				fmt.Printf("Aviso: fonte extra de frases falhou: %v\n", err)
				continue
			}
			for _, f := range extras {
				g.adicionarFraseInterna(f)
			}
		}
	})

	return g.erroCarga
}

// carregarArquivo lê um único TSV gzipado de frases e acrescenta suas linhas ao acervo, indexando os
// caracteres de cada frase. Os índices apontam para o slice compartilhado, então arquivos carregados
// depois apenas estendem o acervo — sem reindexar o que já entrou.
func (g *GerenciadorFrases) carregarArquivo(dir, nomeArquivo string) error {
	dados, err := arquivosIdiomas.ReadFile(dir + "/" + nomeArquivo)
	if err != nil {
		return fmt.Errorf("não foi possível ler o arquivo de frases %q: %w", nomeArquivo, err)
	}

	leitorGz, err := gzip.NewReader(bytes.NewReader(dados))
	if err != nil {
		return fmt.Errorf("arquivo de frases %q corrompido: %w", nomeArquivo, err)
	}
	defer leitorGz.Close()

	varredor := bufio.NewScanner(leitorGz)
	varredor.Buffer(make([]byte, 1024*1024), 1024*1024)

	for varredor.Scan() {
		campos := strings.Split(varredor.Text(), "\t")
		if len(campos) < colunasMinimasFrase || campos[0] == "" {
			continue
		}
		frase := Frase{Chines: campos[0], Ingles: campos[1], Atribuicao: campos[2]}
		if len(campos) > colunaTema {
			frase.Tema = campos[colunaTema]
		}
		if len(campos) > colunaDificuldade {
			frase.Dificuldade = campos[colunaDificuldade]
		}
		g.adicionarFraseInterna(frase)
	}

	if err := varredor.Err(); err != nil {
		return fmt.Errorf("falha ao ler o arquivo de frases %q: %w", nomeArquivo, err)
	}
	return nil
}

// adicionarFraseInterna acrescenta uma frase ao acervo e a indexa por caractere, deduplicando pelo
// texto chinês. NÃO trava — é chamada durante a carga (sob sync.Once) e por AdicionarFrase (que já
// segura o lock de escrita). Ignora frases com texto chinês vazio ou já presente.
func (g *GerenciadorFrases) adicionarFraseInterna(f Frase) {
	if f.Chines == "" || g.textos[f.Chines] {
		return
	}
	g.textos[f.Chines] = true

	idx := int32(len(g.frases))
	g.frases = append(g.frases, f)

	// Indexa cada caractere único da frase (pontuação também entra, mas nunca é consultada).
	vistos := make(map[rune]bool)
	for _, r := range f.Chines {
		if vistos[r] {
			continue
		}
		vistos[r] = true
		g.indice[r] = append(g.indice[r], idx)
	}
}

// AdicionarFrase injeta uma frase no acervo em runtime (ex.: frase gerada com IA que o usuário
// salvou). Devolve true se a frase era inédita. Thread-safe; a frase passa a valer nas consultas
// desta sessão imediatamente (a persistência entre sessões é responsabilidade do chamador).
func (g *GerenciadorFrases) AdicionarFrase(f Frase) bool {
	if err := g.garantirCarregado(); err != nil {
		return false
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.textos[f.Chines] {
		return false
	}
	g.adicionarFraseInterna(f)
	return true
}

// RemoverFrase remove uma frase do acervo em runtime (ex.: frase descartada pelo usuário).
func (g *GerenciadorFrases) RemoverFrase(chines string) bool {
	if err := g.garantirCarregado(); err != nil {
		return false
	}
	g.mu.Lock()
	defer g.mu.Unlock()

	if !g.textos[chines] {
		return false
	}

	delete(g.textos, chines)

	pos := -1
	for i, f := range g.frases {
		if f.Chines == chines {
			pos = i
			break
		}
	}
	if pos != -1 {
		g.frases = append(g.frases[:pos], g.frases[pos+1:]...)
	}

	g.indice = make(map[rune][]int32)
	for i, f := range g.frases {
		vistos := make(map[rune]bool)
		for _, r := range f.Chines {
			if vistos[r] {
				continue
			}
			vistos[r] = true
			g.indice[r] = append(g.indice[r], int32(i))
		}
	}
	return true
}

// ----- Consultas -----

// FrasesComCaractere devolve todas as frases que contêm o caractere dado.
func (g *GerenciadorFrases) FrasesComCaractere(caractere rune) []Frase {
	if err := g.garantirCarregado(); err != nil {
		return nil
	}

	g.mu.RLock()
	defer g.mu.RUnlock()
	indices := g.indice[caractere]
	frases := make([]Frase, 0, len(indices))
	for _, idx := range indices {
		frases = append(frases, g.frases[idx])
	}
	return frases
}

// FrasesComPalavra devolve as frases que contêm a palavra inteira (substring). Reaproveita o índice
// por caractere (frases com o 1º hanzi da palavra) e filtra por substring — para caractere isolado
// equivale a FrasesComCaractere.
func (g *GerenciadorFrases) FrasesComPalavra(palavra string) []Frase {
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
	frases := make([]Frase, 0, len(indices))
	for _, idx := range indices {
		if strings.Contains(g.frases[idx].Chines, palavra) {
			frases = append(frases, g.frases[idx])
		}
	}
	return frases
}

// TemCaractere informa se existe ao menos uma frase contendo o caractere.
func (g *GerenciadorFrases) TemCaractere(caractere rune) bool {
	if err := g.garantirCarregado(); err != nil {
		return false
	}
	g.mu.RLock()
	defer g.mu.RUnlock()
	return len(g.indice[caractere]) > 0
}

// ObterTodasFrases devolve uma cópia de todas as frases do acervo (cópia para não expor o slice
// interno a corridas com AdicionarFrase).
func (g *GerenciadorFrases) ObterTodasFrases() []Frase {
	if err := g.garantirCarregado(); err != nil {
		return nil
	}
	g.mu.RLock()
	defer g.mu.RUnlock()
	copia := make([]Frase, len(g.frases))
	copy(copia, g.frases)
	return copia
}

func (g *GerenciadorFrases) TotalFrases() int {
	if err := g.garantirCarregado(); err != nil {
		return 0
	}
	g.mu.RLock()
	defer g.mu.RUnlock()
	return len(g.frases)
}

// ----- Filtragem e seleção para a revisão -----
//
// Responsabilidade unificada aqui: dado um conjunto de frases (e o status de conhecimento do usuário
// na sessão), este módulo é quem decide QUAIS frases servem e QUAL escolher. A revisão só orquestra
// (qual filtro por variante) — não reimplementa a lógica de filtragem.
//
// O status de conhecimento é um mapa caractere→status: StatusEstudo, StatusAprendido ou "" (ainda
// desconhecido). É montado pela revisão a partir do vocabulário do usuário.

const (
	StatusVisto     = "visto"
	StatusEstudo    = "estudo"
	StatusAprendido = "aprendido"
)

// Faixas de cobertura de vocabulário conhecido de uma frase (fração dos caracteres Han únicos que o
// usuário já viu). Usadas como piso ao filtrar frases por familiaridade.
const (
	CoberturaAlta  = 0.9
	CoberturaMedia = 0.5
	CoberturaBaixa = 0.25
)

// Pesos da seleção ponderada. O peso base garante que frases sem vocabulário conhecido ainda possam
// aparecer; os demais favorecem frases com mais vocabulário que o usuário já domina/estuda.
const (
	pesoFraseBase            = 10
	pesoFraseAprendido       = 400
	pesoFraseEstudo          = 200
	pesoFraseSequenciaEstudo = 20
)

// FiltrarPreservandoAlvoNaConversao devolve as frases cujo texto, depois de convertido para tipoAlvo
// pelo GerenciadorDicionario informado, AINDA contém literalmente o caractere-alvo.
//
// Existe porque a grafia de exibição pode divergir da grafia estudada: quem estuda o tradicional 發
// e tem a exibição forçada em "simplificado" veria a frase convertida para 发, e a lacuna/destaque da
// questão apontaria para um caractere (發) que já não está no texto convertido. Este filtro descarta
// essas frases. No-op (devolve as frases inalteradas) quando tipoAlvo não é "simplificado" nem
// "tradicional".
func (g *GerenciadorFrases) FiltrarPreservandoAlvoNaConversao(frases []Frase, alvo string, dic *GerenciadorDicionario, tipoAlvo string) []Frase {
	if tipoAlvo != "simplificado" && tipoAlvo != "tradicional" {
		return frases
	}

	filtradas := make([]Frase, 0, len(frases))
	for _, f := range frases {
		if strings.Contains(dic.ConverterTexto(f.Chines, tipoAlvo), alvo) {
			filtradas = append(filtradas, f)
		}
	}
	return filtradas
}

// FiltrarPorTamanho devolve as frases com no máximo maxCaracteres caracteres (as mais curtas cabem
// melhor na tela). Devolve slice vazio se nenhuma couber — o chamador decide se cai no fallback.
func (g *GerenciadorFrases) FiltrarPorTamanho(frases []Frase, maxCaracteres int) []Frase {
	filtradas := make([]Frase, 0, len(frases))
	for _, f := range frases {
		if utf8.RuneCountInString(f.Chines) <= maxCaracteres {
			filtradas = append(filtradas, f)
		}
	}
	return filtradas
}

// FiltrarPorMaxPalavras devolve as frases com no máximo maxPalavras palavras em chinês.
func (g *GerenciadorFrases) FiltrarPorMaxPalavras(frases []Frase, maxPalavras int, dic *GerenciadorDicionario) []Frase {
	filtradas := make([]Frase, 0, len(frases))
	for _, f := range frases {
		if g.ContarPalavras(f.Chines, dic) <= maxPalavras {
			filtradas = append(filtradas, f)
		}
	}
	return filtradas
}

// ContarPalavras conta a quantidade de palavras chinesas em um texto.
func (g *GerenciadorFrases) ContarPalavras(texto string, dic *GerenciadorDicionario) int {
	if dic != nil {
		tokens := dic.SegmentarPorDicionario(texto)
		contagem := 0
		for _, t := range tokens {
			for _, r := range t {
				if unicode.Is(unicode.Han, r) {
					contagem++
					break
				}
			}
		}
		if contagem > 0 {
			return contagem
		}
	}
	contagem := 0
	for _, r := range texto {
		if unicode.Is(unicode.Han, r) {
			contagem++
		}
	}
	return contagem
}

// FiltrarTodosConhecidos devolve apenas as frases em que TODO caractere Han já é conhecido
// (StatusEstudo ou StatusAprendido). Exige ao menos um caractere Han na frase.
func (g *GerenciadorFrases) FiltrarTodosConhecidos(frases []Frase, status map[string]string) []Frase {
	filtradas := make([]Frase, 0, len(frases))
	for _, f := range frases {
		temHan := false
		todosConhecidos := true
		for _, r := range f.Chines {
			if !unicode.Is(unicode.Han, r) {
				continue
			}
			temHan = true
			if !ehConhecido(status[string(r)]) {
				todosConhecidos = false
				break
			}
		}
		if temHan && todosConhecidos {
			filtradas = append(filtradas, f)
		}
	}
	return filtradas
}

// FiltrarPorCobertura devolve as frases cuja fração de caracteres Han já vistos (com qualquer status)
// é >= minCobertura. Frases sem nenhum caractere Han são descartadas.
func (g *GerenciadorFrases) FiltrarPorCobertura(frases []Frase, status map[string]string, minCobertura float64) []Frase {
	filtradas := make([]Frase, 0, len(frases))
	for _, f := range frases {
		unicos, conhecidos := contarConhecimento(f.Chines, status)
		if unicos == 0 {
			continue
		}
		if float64(conhecidos)/float64(unicos) >= minCobertura {
			filtradas = append(filtradas, f)
		}
	}
	return filtradas
}

// TemFraseComCobertura diz se o caractere tem ao menos uma frase cuja cobertura de vocabulário
// conhecido atinge minCobertura. Alimenta a seleção de alvos das variantes de ordenação/pronúncia.
func (g *GerenciadorFrases) TemFraseComCobertura(caractere rune, status map[string]string, minCobertura float64) bool {
	for _, f := range g.FrasesComCaractere(caractere) {
		unicos, conhecidos := contarConhecimento(f.Chines, status)
		if unicos > 0 && float64(conhecidos)/float64(unicos) >= minCobertura {
			return true
		}
	}
	return false
}

// SelecionarPonderada escolhe uma frase por roleta ponderada, dando peso proporcional à PROPORÇÃO de
// vocabulário conhecido (estudo/aprendido) sobre o total de hanzis únicos da frase.
func (g *GerenciadorFrases) SelecionarPonderada(frases []Frase, status map[string]string) Frase {
	if len(frases) == 1 || len(status) == 0 {
		return frases[rand.IntN(len(frases))]
	}

	pesos := make([]int, len(frases))
	pesoTotal := 0
	for i, f := range frases {
		pontos := 0
		unicos := 0
		vistos := make(map[rune]bool)
		for _, r := range f.Chines {
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

	return frases[sortearPorRoleta(pesos, pesoTotal)]
}

// SelecionarSequenciaPonderada escolhe uma frase dando peso à CONTAGEM absoluta de palavras sob
// estudo (flat), em vez da proporção — favorece frases que exercitam mais itens em estudo.
func (g *GerenciadorFrases) SelecionarSequenciaPonderada(frases []Frase, status map[string]string) Frase {
	if len(frases) == 1 || len(status) == 0 {
		return frases[rand.IntN(len(frases))]
	}

	pesos := make([]int, len(frases))
	pesoTotal := 0
	for i, f := range frases {
		contagemEstudo := 0
		vistos := make(map[rune]bool)
		for _, r := range f.Chines {
			if vistos[r] || !unicode.Is(unicode.Han, r) {
				continue
			}
			vistos[r] = true
			if status[string(r)] == StatusEstudo {
				contagemEstudo++
			}
		}

		peso := pesoFraseBase + contagemEstudo*pesoFraseSequenciaEstudo
		pesos[i] = peso
		pesoTotal += peso
	}

	return frases[sortearPorRoleta(pesos, pesoTotal)]
}

// ----- Helpers internos de filtragem -----

// contarConhecimento conta os caracteres Han ÚNICOS de uma frase e quantos deles já têm status.
func contarConhecimento(chines string, status map[string]string) (unicos, conhecidos int) {
	vistos := make(map[rune]bool)
	for _, r := range chines {
		if vistos[r] || !unicode.Is(unicode.Han, r) {
			continue
		}
		vistos[r] = true
		unicos++
		if status[string(r)] != "" {
			conhecidos++
		}
	}
	return unicos, conhecidos
}

// sortearPorRoleta devolve o índice sorteado por roleta ponderada. Assume pesoTotal > 0 (garantido
// pelo peso base positivo de cada frase).
func sortearPorRoleta(pesos []int, pesoTotal int) int {
	sorteio := rand.IntN(pesoTotal)
	for i, peso := range pesos {
		sorteio -= peso
		if sorteio < 0 {
			return i
		}
	}
	return len(pesos) - 1 // fallback (não deve ocorrer)
}

// ehConhecido diz se um status representa um caractere já dominado ou em estudo.
func ehConhecido(status string) bool {
	return status == StatusEstudo || status == StatusAprendido
}
