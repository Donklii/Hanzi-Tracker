package main

// ----- Seção: Inventário centralizado das frases chinesas conhecidas -----
//
// Varre TODOS os bancos de frases embarcados de TODOS os idiomas e destila um único arquivo com as
// frases em chinês que o projeto já conhece — cada uma com sua classificação (tema + dificuldade).
// É a "fonte da verdade" do que já existe, pensada para o futuro gerador de frases via DeepSeek:
//   - dedup: antes de gravar frases novas, o gerador confere aqui e descarta as repetidas;
//   - padronização: a saída já vem no formato de classificação (chinês <TAB> atribuição <TAB> tema
//     <TAB> dificuldade) — as mesmas colunas do banco embarcado menos a tradução (que é por idioma).
//
// Por que ler dos .tsv.gz EMBARCADOS (e não dos classificacoes.tsv de trabalho): o que está embarcado
// é o que o app de fato conhece — os classificacoes.tsv são intermediários de build que podem estar
// à frente/atrás do que foi realmente empacotado. Classificação (col. 4/5) e atribuição (col. 3) já
// viajam em cada linha embarcada (ver dicionario/gerenciador_frases.go), então basta reaproveitá-las.
//
// Entrada:  dicionario/idiomas/<idioma>/frases/*.tsv.gz   (col. 0 = chinês, 2 = atribuição, 3 = tema, 4 = dificuldade)
// Saída:    dicionario/frases/inventario/frases_conhecidas.tsv   (chinês <TAB> atribuição <TAB> tema <TAB> dificuldade)
//
// Deduplica por texto chinês: a MESMA frase costuma aparecer em vários idiomas (a classificação é
// propagada por chinês idêntico em frases/espalhar), então uma única linha a representa. Se um idioma
// trouxer a frase SEM classificação e outro COM, a saída fica com a classificação. Divergências reais
// de rótulo entre arquivos são mantidas pelo primeiro visto (ordem de arquivo estável) e reportadas no
// resumo — são raras e indicam dado a revisar, não erro do extrator.
//
// ATRIBUIÇÃO: quando a mesma frase existe em vários idiomas, prevalece a do idioma canônico (en, ver
// idiomaPrioritario) — é a original do Tatoeba (chinês + tradução inglesa por IDs reais), enquanto as
// dos demais idiomas podem apontar tradução automática (ex.: "via Google Tradutor"). Frases que só
// existem fora do en ficam com a atribuição do idioma onde aparecem.
//
// Saída ordenada por chinês: determinística e estável entre rodadas (diff limpo, fácil de versionar).
//
// Rodar da RAIZ do repo (mesma pegadinha de módulo do resto do pipeline de frases):
//
//	go run ./dicionario/frases/inventario

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const (
	globBancosFrases = "dicionario/idiomas/*/frases/*.tsv.gz"
	caminhoSaida     = "dicionario/frases/inventario/frases_conhecidas.tsv"

	// idiomaPrioritario é o idioma cuja atribuição prevalece quando a mesma frase aparece em vários
	// (ver o cabeçalho: a do en é a original do Tatoeba, não uma tradução automática).
	idiomaPrioritario = "en"

	colChines      = 0
	colAtribuicao  = 2
	colTema        = 3
	colDificuldade = 4
	minColunas     = 3 // chinês, tradução, atribuição (tema/dificuldade são opcionais)
)

// fraseConhecida é uma frase chinesa única do acervo, com sua atribuição, sua classificação e o
// conjunto de idiomas cujos bancos a contêm (o rastro de idiomas alimenta só o resumo).
type fraseConhecida struct {
	chines           string
	atribuicao       string
	idiomaAtribuicao string // idioma de onde veio a atribuição vigente (para aplicar a prioridade)
	tema             string
	dificuldade      string
	idiomas          map[string]bool
}


func main() {
	arquivos, err := localizarBancos()
	abortar(err)
	fmt.Printf("Bancos de frases encontrados: %d\n", len(arquivos))

	inventario := map[string]*fraseConhecida{}
	conflitos := []string{}

	for _, caminho := range arquivos {
		idioma := idiomaDoCaminho(caminho)
		lidas, err := lerBanco(caminho, idioma, inventario, &conflitos)
		abortar(err)
		fmt.Printf("  %s [%s]: %d linhas\n", filepath.Base(caminho), idioma, lidas)
	}

	abortar(gravarOrdenado(caminhoSaida, inventario))
	imprimirResumo(caminhoSaida, inventario, conflitos)
}


// ----- Leitura dos bancos embarcados -----

// localizarBancos devolve, em ordem estável, todos os .tsv.gz de frases de todos os idiomas. Ordem
// fixa torna a mesclagem (e a resolução de divergências pelo "primeiro visto") determinística.
func localizarBancos() ([]string, error) {
	arquivos, err := filepath.Glob(globBancosFrases)
	if err != nil {
		return nil, err
	}
	if len(arquivos) == 0 {
		return nil, fmt.Errorf("nenhum banco de frases (%s) encontrado — rode da raiz do repo", globBancosFrases)
	}
	sort.Strings(arquivos)
	return arquivos, nil
}


// idiomaDoCaminho extrai o rótulo de idioma de um caminho ".../idiomas/<idioma>/frases/...". Devolve
// "?" se o padrão não bater (não deveria, dado o glob de entrada) — o resumo ainda contabiliza.
func idiomaDoCaminho(caminho string) string {
	partes := strings.Split(filepath.ToSlash(caminho), "/")
	for i, p := range partes {
		if p == "idiomas" && i+1 < len(partes) {
			return partes[i+1]
		}
	}
	return "?"
}


// lerBanco lê um único .tsv.gz e mescla suas frases no inventário, deduplicando por chinês. Devolve
// quantas linhas úteis (com chinês) foram lidas do arquivo. Conflitos de classificação entre arquivos
// são anexados a *conflitos para o resumo.
func lerBanco(caminho, idioma string, inventario map[string]*fraseConhecida, conflitos *[]string) (int, error) {
	dados, err := os.ReadFile(caminho)
	if err != nil {
		return 0, fmt.Errorf("não foi possível ler %q: %w", caminho, err)
	}
	gz, err := gzip.NewReader(bytes.NewReader(dados))
	if err != nil {
		return 0, fmt.Errorf("%q corrompido: %w", caminho, err)
	}
	defer gz.Close()

	varredor := bufio.NewScanner(gz)
	varredor.Buffer(make([]byte, 1024*1024), 1024*1024)

	lidas := 0
	for varredor.Scan() {
		campos := strings.Split(varredor.Text(), "\t")
		if len(campos) < minColunas || campos[colChines] == "" {
			continue
		}
		lidas++
		mesclar(inventario, idioma, campos, conflitos)
	}
	if err := varredor.Err(); err != nil {
		return 0, fmt.Errorf("falha ao ler %q: %w", caminho, err)
	}
	return lidas, nil
}


// mesclar insere ou atualiza uma frase no inventário. Regras: registra o idioma de origem; prioriza a
// atribuição do idioma canônico; preenche a classificação quando a existente está vazia; e, se ambas
// estão preenchidas mas divergem, mantém a primeira (ordem de arquivo estável) e registra a divergência.
func mesclar(inventario map[string]*fraseConhecida, idioma string, campos []string, conflitos *[]string) {
	chines := campos[colChines]
	atribuicao := campos[colAtribuicao] // sempre presente: minColunas garante a coluna 2
	tema, dificuldade := "", ""
	if len(campos) > colTema {
		tema = campos[colTema]
	}
	if len(campos) > colDificuldade {
		dificuldade = campos[colDificuldade]
	}

	existente, ok := inventario[chines]
	if !ok {
		inventario[chines] = &fraseConhecida{
			chines:           chines,
			atribuicao:       atribuicao,
			idiomaAtribuicao: idioma,
			tema:             tema,
			dificuldade:      dificuldade,
			idiomas:          map[string]bool{idioma: true},
		}
		return
	}

	existente.idiomas[idioma] = true

	// Atribuição: a do idioma prioritário (en) prevalece. Uma vez fixada nele, nenhuma outra a troca;
	// enquanto não veio dele, vale a primeira vista (ordem de arquivo), substituída se o en aparecer.
	if idioma == idiomaPrioritario && existente.idiomaAtribuicao != idiomaPrioritario {
		existente.atribuicao = atribuicao
		existente.idiomaAtribuicao = idioma
	}

	// Classificação: completa o que faltava; sinaliza (sem sobrescrever) quando os dois divergem.
	if existente.tema == "" {
		existente.tema = tema
	} else if tema != "" && tema != existente.tema {
		*conflitos = append(*conflitos, fmt.Sprintf("%s  tema: %q vs %q", chines, existente.tema, tema))
	}
	if existente.dificuldade == "" {
		existente.dificuldade = dificuldade
	} else if dificuldade != "" && dificuldade != existente.dificuldade {
		*conflitos = append(*conflitos, fmt.Sprintf("%s  dificuldade: %q vs %q", chines, existente.dificuldade, dificuldade))
	}
}


// ----- Saída ordenada + resumo -----

// gravarOrdenado escreve o inventário inteiro ordenado por chinês, no formato chinês<TAB>atribuição
// <TAB>tema<TAB>dificuldade (tema/dificuldade podem vir vazios em frases sem classificação). Determinístico.
func gravarOrdenado(caminho string, inventario map[string]*fraseConhecida) error {
	if err := os.MkdirAll(filepath.Dir(caminho), 0o755); err != nil {
		return err
	}

	chineses := make([]string, 0, len(inventario))
	for ch := range inventario {
		chineses = append(chineses, ch)
	}
	sort.Strings(chineses)

	var saida strings.Builder
	for _, ch := range chineses {
		f := inventario[ch]
		fmt.Fprintf(&saida, "%s\t%s\t%s\t%s\n", f.chines, f.atribuicao, f.tema, f.dificuldade)
	}
	return os.WriteFile(caminho, []byte(saida.String()), 0o644)
}


// imprimirResumo mostra o tamanho do acervo, a cobertura de classificação, a distribuição por
// dificuldade/tema, a presença por idioma e eventuais divergências de rótulo entre arquivos.
func imprimirResumo(caminho string, inventario map[string]*fraseConhecida, conflitos []string) {
	classificadas, semClassificacao := 0, 0
	porDificuldade := map[string]int{}
	porTema := map[string]int{}
	porIdioma := map[string]int{}
	exclusivas := map[string]int{}
	porAtribuicao := map[string]int{}

	for _, f := range inventario {
		if f.tema != "" || f.dificuldade != "" {
			classificadas++
		} else {
			semClassificacao++
		}
		if f.dificuldade != "" {
			porDificuldade[f.dificuldade]++
		}
		if f.tema != "" {
			porTema[f.tema]++
		}
		porAtribuicao[f.idiomaAtribuicao]++
		for idioma := range f.idiomas {
			porIdioma[idioma]++
		}
		if len(f.idiomas) == 1 {
			for idioma := range f.idiomas {
				exclusivas[idioma]++
			}
		}
	}

	fmt.Printf("\n===== RESUMO =====\n")
	fmt.Printf("Saída: %s\n", caminho)
	fmt.Printf("Frases chinesas únicas: %d\n", len(inventario))
	fmt.Printf("  com classificação: %d\n", classificadas)
	fmt.Printf("  sem classificação: %d\n", semClassificacao)

	fmt.Printf("\nPresença por idioma (frases cujo banco daquele idioma contém):\n")
	for _, idioma := range chavesOrdenadas(porIdioma) {
		fmt.Printf("  %s: %d  (exclusivas desse idioma: %d)\n", idioma, porIdioma[idioma], exclusivas[idioma])
	}

	fmt.Printf("\nAtribuição por idioma de origem (%s priorizado):\n", idiomaPrioritario)
	for _, idioma := range chavesOrdenadas(porAtribuicao) {
		fmt.Printf("  %s: %d\n", idioma, porAtribuicao[idioma])
	}

	fmt.Printf("\nPor dificuldade:\n")
	for _, d := range chavesOrdenadas(porDificuldade) {
		fmt.Printf("  %s: %d\n", d, porDificuldade[d])
	}

	fmt.Printf("\nPor tema:\n")
	for _, t := range chavesOrdenadasPorValor(porTema) {
		fmt.Printf("  %s: %d\n", t, porTema[t])
	}

	if len(conflitos) > 0 {
		fmt.Printf("\n⚠ Divergências de classificação entre arquivos (mantida a 1ª vista): %d\n", len(conflitos))
		for i, c := range conflitos {
			if i >= 15 {
				fmt.Printf("  … e mais %d\n", len(conflitos)-15)
				break
			}
			fmt.Printf("  %s\n", c)
		}
	}
}


// ----- Utilitários -----

// chavesOrdenadas devolve as chaves do mapa em ordem alfabética (resumo estável entre rodadas).
func chavesOrdenadas(m map[string]int) []string {
	chaves := make([]string, 0, len(m))
	for k := range m {
		chaves = append(chaves, k)
	}
	sort.Strings(chaves)
	return chaves
}


// chavesOrdenadasPorValor devolve as chaves da maior contagem para a menor (desempate alfabético),
// para o resumo por tema sair do mais comum ao mais raro.
func chavesOrdenadasPorValor(m map[string]int) []string {
	chaves := make([]string, 0, len(m))
	for k := range m {
		chaves = append(chaves, k)
	}
	sort.Slice(chaves, func(i, j int) bool {
		if m[chaves[i]] != m[chaves[j]] {
			return m[chaves[i]] > m[chaves[j]]
		}
		return chaves[i] < chaves[j]
	})
	return chaves
}


func abortar(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, "erro:", err)
		os.Exit(1)
	}
}
