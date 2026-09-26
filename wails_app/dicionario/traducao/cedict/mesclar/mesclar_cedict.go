package main

// ----- Seção: Mesclagem das traduções de volta no CEDICT -----
// Reconstrói o cedict.u8 no idioma alvo (pt-BR ou es) a partir de dois insumos:
//   - a estrutura ORIGINAL (fontes/en/cedict.u8), da qual seguem VERBATIM os campos
//     language-neutral: tradicional, simplificado e o pinyin da própria entrada;
//   - as traduções dos lotes (traducao/cedict/lotes/lote_NNN_<idioma>.txt), das quais vêm APENAS os
//     significados. As glosas em inglês são DESCARTADAS — o arquivo derivado só carrega o idioma traduzido.
//
// Os lotes são chaveados por SIMPLIFICADO e agrupam TODAS as linhas cruas da mesma palavra, então a
// mesclagem reparte a tradução de volta entre essas linhas.
//
// Uso (executar da raiz do repositório):
//	go run wails_app/dicionario/traducao/cedict/mesclar/mesclar_cedict.go es    → gera fontes/es/cedict.u8
//	go run wails_app/dicionario/traducao/cedict/mesclar/mesclar_cedict.go pt    → gera fontes/pt-BR/cedict.u8

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ----- Constantes -----

const (
	CAMINHO_ORIGINAL = "wails_app/dicionario/fontes/en/cedict.u8"
	PASTA_LOTES      = "wails_app/dicionario/traducao/cedict/lotes"

	MARCADOR           = " | TRADUÇÃO: "
	SEPARADOR_LEITURAS = "‖"
)

const AVISO_TRADUCAO_PT = "#\n" +
	"# ----- Tradução pt-BR (trabalho derivado) -----\n" +
	"# Os significados foram traduzidos automaticamente do inglês para o português (pt-BR) por LLM\n" +
	"# (DeepSeek V3). Tradicional, simplificado e pinyin seguem inalterados em relação ao CC-CEDICT\n" +
	"# original. Distribuído sob a mesma licença do original (CC BY-SA 4.0).\n" +
	"# Gerado por wails_app/dicionario/traducao/cedict/mesclar/mesclar_cedict.go\n" +
	"#\n"

const AVISO_TRADUCAO_ES = "#\n" +
	"# ----- Tradução Espanhol (trabalho derivado) -----\n" +
	"# Os significados foram traduzidos para o espanhol neutro por LLM.\n" +
	"# Tradicional, simplificado e pinyin seguem inalterados em relação ao CC-CEDICT original.\n" +
	"# Distribuído sob a mesma licença do original (CC BY-SA 4.0).\n" +
	"# Gerado por wails_app/dicionario/traducao/cedict/mesclar/mesclar_cedict.go\n" +
	"#\n"

// configuracaoIdioma define parâmetros específicos por idioma alvo.
type configuracaoIdioma struct {
	codigo        string
	sufixoLote    string
	caminhoSaida  string
	avisoTraducao string
}

// linhaCru é uma linha do CEDICT original. Tradicional, simplificado e pinyin são language-neutral e
// seguem verbatim para a saída; só os significados são trocados pela tradução.
type linhaCru struct {
	tradicional  string
	simplificado string
	pinyin       string
	significados []string
}

// estatisticas acumula os números do relatório final.
type estatisticas struct {
	palavrasPorGrupo int
	palavrasLegado   int
}


func main() {
	config, err := interpretarArgumentosMesclagem(os.Args[1:])
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	cabecalho, linhas, descartadas, err := lerOriginal(CAMINHO_ORIGINAL)
	if err != nil {
		fmt.Printf("Erro ao ler %s: %v\n", CAMINHO_ORIGINAL, err)
		os.Exit(1)
	}

	traducoes, err := carregarTraducoes(PASTA_LOTES, config.sufixoLote)
	if err != nil {
		fmt.Printf("Erro ao carregar os lotes traduzidos (%s): %v\n", config.codigo, err)
		os.Exit(1)
	}
	fmt.Printf("Lidas %d linhas cruas do CEDICT (%d fora do formato, descartadas) e %d palavras traduzidas dos lotes (%s).\n",
		len(linhas), descartadas, len(traducoes), config.codigo)

	significadosTraduzidos, stats, err := distribuirTraducoes(linhas, traducoes)
	if err != nil {
		fmt.Printf("Erro ao repartir as traduções entre as linhas cruas: %v\n", err)
		os.Exit(1)
	}

	if err := gravarSaida(config.caminhoSaida, config.avisoTraducao, cabecalho, linhas, significadosTraduzidos); err != nil {
		fmt.Printf("Erro ao gravar %s: %v\n", config.caminhoSaida, err)
		os.Exit(1)
	}

	fmt.Printf("Concluído: %d linhas gravadas em %s\n", len(linhas), config.caminhoSaida)
	fmt.Printf("  %d palavras repartidas por grupos %q (formato v2), %d pela contagem de significados (formato v1, legado)\n",
		stats.palavrasPorGrupo, SEPARADOR_LEITURAS, stats.palavrasLegado)
}


func lerOriginal(caminho string) ([]string, []linhaCru, int, error) {
	arquivo, err := os.Open(caminho)
	if err != nil {
		return nil, nil, 0, err
	}
	defer arquivo.Close()

	varredor := bufio.NewScanner(arquivo)
	varredor.Buffer(make([]byte, 1024*1024), 1024*1024)

	var cabecalho []string
	var linhas []linhaCru
	descartadas := 0

	for varredor.Scan() {
		linha := varredor.Text()
		if strings.HasPrefix(linha, "#") {
			if len(linhas) == 0 {
				cabecalho = append(cabecalho, linha)
			}
			continue
		}
		if strings.TrimSpace(linha) == "" {
			continue
		}

		cru, ok := partirLinhaCru(linha)
		if !ok {
			descartadas++
			continue
		}
		linhas = append(linhas, cru)
	}

	if err := varredor.Err(); err != nil {
		return nil, nil, 0, err
	}
	if len(linhas) == 0 {
		return nil, nil, 0, fmt.Errorf("nenhuma linha de dados reconhecida em %s", caminho)
	}
	return cabecalho, linhas, descartadas, nil
}


func carregarTraducoes(pasta, sufixoLote string) (map[string][]string, error) {
	padrao := filepath.Join(pasta, fmt.Sprintf("lote_???%s.txt", sufixoLote))
	arquivos, err := filepath.Glob(padrao)
	if err != nil {
		return nil, err
	}
	if len(arquivos) == 0 {
		return nil, fmt.Errorf("nenhum arquivo lote_???%s.txt encontrado em %s", sufixoLote, pasta)
	}

	traducoes := map[string][]string{}
	for _, arquivo := range arquivos {
		if err := carregarArquivoLote(arquivo, traducoes); err != nil {
			return nil, fmt.Errorf("%s: %w", filepath.Base(arquivo), err)
		}
	}
	return traducoes, nil
}


func carregarArquivoLote(caminho string, traducoes map[string][]string) error {
	arquivo, err := os.Open(caminho)
	if err != nil {
		return err
	}
	defer arquivo.Close()

	varredor := bufio.NewScanner(arquivo)
	varredor.Buffer(make([]byte, 1024*1024), 1024*1024)

	for varredor.Scan() {
		linha := varredor.Text()
		if strings.HasPrefix(linha, "#") || strings.TrimSpace(linha) == "" {
			continue
		}

		idx := strings.Index(linha, MARCADOR)
		if idx < 0 {
			return fmt.Errorf("linha sem o marcador %q: %q", MARCADOR, linha)
		}
		palavra := linha[:idx]
		glosas := linha[idx+len(MARCADOR):]
		if palavra == "" || strings.TrimSpace(glosas) == "" {
			return fmt.Errorf("palavra ou tradução vazia: %q", linha)
		}
		if _, repetida := traducoes[palavra]; repetida {
			return fmt.Errorf("palavra %q aparece traduzida em mais de um lote", palavra)
		}
		traducoes[palavra] = dividirGrupos(glosas)
	}
	return varredor.Err()
}


func distribuirTraducoes(linhas []linhaCru, traducoes map[string][]string) ([][]string, estatisticas, error) {
	indices := agruparIndicesPorSimplificado(linhas)
	significadosTraduzidos := make([][]string, len(linhas))
	feitas := map[string]bool{}
	var stats estatisticas

	for _, linha := range linhas {
		if feitas[linha.simplificado] {
			continue
		}
		feitas[linha.simplificado] = true

		grupos, temTraducao := traducoes[linha.simplificado]
		if !temTraducao {
			return nil, stats, fmt.Errorf("palavra %q não tem tradução em nenhum lote", linha.simplificado)
		}

		desta := indices[linha.simplificado]
		porLinha, legado, err := distribuirPorPalavra(linhas, desta, grupos)
		if err != nil {
			return nil, stats, err
		}
		if legado {
			stats.palavrasLegado++
		} else {
			stats.palavrasPorGrupo++
		}

		for i, idx := range desta {
			significadosTraduzidos[idx] = porLinha[i]
		}
	}

	return significadosTraduzidos, stats, nil
}


func distribuirPorPalavra(linhas []linhaCru, indices []int, grupos []string) ([][]string, bool, error) {
	simplificado := linhas[indices[0]].simplificado

	if len(grupos) > 1 || len(indices) == 1 {
		if len(grupos) != len(indices) {
			return nil, false, fmt.Errorf("palavra %q: %d linha(s) crua(s) no CEDICT mas %d grupo(s) %q na tradução",
				simplificado, len(indices), len(grupos), SEPARADOR_LEITURAS)
		}
		porLinha := make([][]string, len(grupos))
		for i, grupo := range grupos {
			porLinha[i] = partirSignificados(grupo)
		}
		return porLinha, false, nil
	}

	todos := partirSignificados(grupos[0])
	total := 0
	for _, idx := range indices {
		total += len(linhas[idx].significados)
	}
	if len(todos) != total {
		return nil, true, fmt.Errorf("palavra %q (legado, sem %q): %d significados no original mas %d na tradução — sem como repartir entre as %d leituras",
			simplificado, SEPARADOR_LEITURAS, total, len(todos), len(indices))
	}

	porLinha := make([][]string, len(indices))
	corte := 0
	for i, idx := range indices {
		quantidade := len(linhas[idx].significados)
		porLinha[i] = todos[corte : corte+quantidade]
		corte += quantidade
	}
	return porLinha, true, nil
}


func gravarSaida(caminho, avisoTraducao string, cabecalho []string, linhas []linhaCru, significadosTraduzidos [][]string) error {
	if err := os.MkdirAll(filepath.Dir(caminho), 0o755); err != nil {
		return err
	}
	arquivo, err := os.Create(caminho)
	if err != nil {
		return err
	}
	defer arquivo.Close()

	escritor := bufio.NewWriter(arquivo)
	defer escritor.Flush()

	for _, linha := range cabecalho {
		escritor.WriteString(linha + "\n")
	}
	escritor.WriteString(avisoTraducao)

	for i, linha := range linhas {
		escritor.WriteString(formatarLinhaCru(linha, significadosTraduzidos[i]))
	}
	return escritor.Flush()
}

// ----- Seção: Interpretação de Argumentos e Configuração -----

func interpretarArgumentosMesclagem(args []string) (configuracaoIdioma, error) {
	if len(args) == 0 {
		return configuracaoIdioma{}, fmt.Errorf("uso: mesclar_cedict.go <pt|es>\n  ex.: go run .../mesclar_cedict.go es\n  ex.: go run .../mesclar_cedict.go pt")
	}

	idioma := strings.ToLower(strings.TrimSpace(args[0]))
	idioma = strings.ReplaceAll(idioma, "_", "-")

	switch idioma {
	case "pt", "pt-br":
		return configuracaoIdioma{
			codigo:        "pt",
			sufixoLote:    "_pt",
			caminhoSaida:  "wails_app/dicionario/fontes/pt-BR/cedict.u8",
			avisoTraducao: AVISO_TRADUCAO_PT,
		}, nil
	case "es":
		return configuracaoIdioma{
			codigo:        "es",
			sufixoLote:    "_es",
			caminhoSaida:  "wails_app/dicionario/fontes/es/cedict.u8",
			avisoTraducao: AVISO_TRADUCAO_ES,
		}, nil
	default:
		return configuracaoIdioma{}, fmt.Errorf("idioma não suportado %q: use 'pt' (ou 'pt-br') ou 'es'", args[0])
	}
}

// ----- Seção: Utilitários -----

func partirLinhaCru(linha string) (linhaCru, bool) {
	partes := strings.SplitN(linha, " [", 2)
	if len(partes) != 2 {
		return linhaCru{}, false
	}
	palavras := strings.Fields(partes[0])
	if len(palavras) != 2 {
		return linhaCru{}, false
	}
	resto := strings.SplitN(partes[1], "] /", 2)
	if len(resto) != 2 {
		return linhaCru{}, false
	}
	significados := strings.TrimSuffix(resto[1], "/")
	if significados == "" {
		return linhaCru{}, false
	}

	return linhaCru{
		tradicional:  palavras[0],
		simplificado: palavras[1],
		pinyin:       resto[0],
		significados: strings.Split(significados, "/"),
	}, true
}


func formatarLinhaCru(linha linhaCru, significados []string) string {
	return fmt.Sprintf("%s %s [%s] /%s/\n", linha.tradicional, linha.simplificado, linha.pinyin, strings.Join(significados, "/"))
}


func agruparIndicesPorSimplificado(linhas []linhaCru) map[string][]int {
	indices := map[string][]int{}
	for i, linha := range linhas {
		indices[linha.simplificado] = append(indices[linha.simplificado], i)
	}
	return indices
}


func dividirGrupos(glosas string) []string {
	grupos := strings.Split(glosas, SEPARADOR_LEITURAS)
	for i := range grupos {
		grupos[i] = strings.TrimSpace(grupos[i])
	}
	return grupos
}


func partirSignificados(grupo string) []string {
	significados := strings.Split(grupo, "/")
	for i := range significados {
		significados[i] = strings.TrimSpace(significados[i])
	}
	return significados
}
