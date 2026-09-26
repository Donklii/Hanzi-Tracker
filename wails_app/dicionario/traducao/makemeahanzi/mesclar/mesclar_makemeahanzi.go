package main

// ----- Seção: Mesclagem do dicionário MakeMeAHanzi -----
// Reconstrói o arquivo makemeahanzi.txt no idioma alvo (pt-BR ou es) a partir de dois insumos:
//   - a estrutura ORIGINAL (fontes/en/makemeahanzi.txt), da qual se preservam TODOS os campos
//     language-neutral (character, pinyin, decomposition, radical, etymology.type/phonetic/semantic, matches);
//   - as traduções dos lotes (traducao/makemeahanzi/lotes/lote_NNN_<idioma>.txt), das quais vêm APENAS
//     a definição e a dica (etymology.hint).
//
// A definição e a dica em inglês são DESCARTADAS — o arquivo derivado só carrega texto no idioma traduzido.
//
// Uso (executar da raiz do repositório):
//	go run wails_app/dicionario/traducao/makemeahanzi/mesclar/mesclar_makemeahanzi.go es    → gera fontes/es/makemeahanzi.txt
//	go run wails_app/dicionario/traducao/makemeahanzi/mesclar/mesclar_makemeahanzi.go pt    → gera fontes/pt-BR/makemeahanzi.txt

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ----- Constantes -----

const (
	CAMINHO_ORIGINAL   = "wails_app/dicionario/fontes/en/makemeahanzi.txt"
	PASTA_LOTES        = "wails_app/dicionario/traducao/makemeahanzi/lotes"
	MARCADOR_DEFINICAO = " | DEFINIÇÃO: "
	MARCADOR_DICA      = " | DICA: "
)

// configuracaoIdioma define caminhos específicos de cada idioma suportado.
type configuracaoIdioma struct {
	codigo       string
	sufixoLote   string
	caminhoSaida string
}

// traducao armazena a definição e a dica traduzidas de um caractere.
type traducao struct {
	definicao string
	dica      string
}


func main() {
	config, err := interpretarArgumentos(os.Args[1:])
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	traducoes, err := carregarTraducoes(PASTA_LOTES, config.sufixoLote)
	if err != nil {
		fmt.Printf("Erro ao carregar traduções dos lotes (%s): %v\n", config.codigo, err)
		os.Exit(1)
	}
	fmt.Printf("Carregadas %d traduções (%s) dos lotes.\n", len(traducoes), config.codigo)

	totalLinhas, comTraducao, semTraducao, err := mesclar(CAMINHO_ORIGINAL, config.caminhoSaida, traducoes)
	if err != nil {
		fmt.Printf("Erro na mesclagem (%s): %v\n", config.codigo, err)
		os.Exit(1)
	}

	fmt.Printf("Concluído: %d entradas gravadas em %s\n", totalLinhas, config.caminhoSaida)
	fmt.Printf("  %d com definição (%s), %d sem tradução (mantidas sem definição/dica)\n", comTraducao, config.codigo, semTraducao)
}


func carregarTraducoes(pasta, sufixoLote string) (map[string]traducao, error) {
	padrao := filepath.Join(pasta, fmt.Sprintf("lote_*%s.txt", sufixoLote))
	arquivos, err := filepath.Glob(padrao)
	if err != nil {
		return nil, err
	}
	if len(arquivos) == 0 {
		return nil, fmt.Errorf("nenhum arquivo lote_*%s.txt encontrado em %s", sufixoLote, pasta)
	}

	traducoes := make(map[string]traducao)
	for _, arquivo := range arquivos {
		if err := carregarArquivoLote(arquivo, traducoes); err != nil {
			return nil, fmt.Errorf("%s: %w", filepath.Base(arquivo), err)
		}
	}
	return traducoes, nil
}


func carregarArquivoLote(caminho string, traducoes map[string]traducao) error {
	arquivo, err := os.Open(caminho)
	if err != nil {
		return err
	}
	defer arquivo.Close()

	varredor := bufio.NewScanner(arquivo)
	varredor.Buffer(make([]byte, 1024*1024), 1024*1024)

	for varredor.Scan() {
		linha := varredor.Text()
		if strings.TrimSpace(linha) == "" || strings.HasPrefix(linha, "#") {
			continue
		}

		idxDef := strings.Index(linha, MARCADOR_DEFINICAO)
		if idxDef < 0 {
			return fmt.Errorf("linha sem marcador de definição: %q", linha)
		}
		caractere := linha[:idxDef]
		resto := linha[idxDef+len(MARCADOR_DEFINICAO):]

		definicao := resto
		dica := ""
		if idxDica := strings.Index(resto, MARCADOR_DICA); idxDica >= 0 {
			definicao = resto[:idxDica]
			dica = resto[idxDica+len(MARCADOR_DICA):]
		}

		traducoes[caractere] = traducao{
			definicao: strings.TrimSpace(definicao),
			dica:      strings.TrimSpace(dica),
		}
	}
	return varredor.Err()
}


func mesclar(origem, saida string, traducoes map[string]traducao) (total, comTraducao, semTraducao int, err error) {
	entrada, err := os.Open(origem)
	if err != nil {
		return 0, 0, 0, err
	}
	defer entrada.Close()

	if err := os.MkdirAll(filepath.Dir(saida), 0o755); err != nil {
		return 0, 0, 0, err
	}
	arquivoSaida, err := os.Create(saida)
	if err != nil {
		return 0, 0, 0, err
	}
	defer arquivoSaida.Close()

	escritor := bufio.NewWriter(arquivoSaida)
	defer escritor.Flush()

	varredor := bufio.NewScanner(entrada)
	varredor.Buffer(make([]byte, 1024*1024), 1024*1024)

	for varredor.Scan() {
		linha := varredor.Text()
		if strings.TrimSpace(linha) == "" {
			continue
		}

		var campos map[string]json.RawMessage
		if err := json.Unmarshal([]byte(linha), &campos); err != nil {
			return 0, 0, 0, fmt.Errorf("linha inválida no original: %w", err)
		}

		var caractere string
		if raw, ok := campos["character"]; ok {
			_ = json.Unmarshal(raw, &caractere)
		}

		trad, temTrad := traducoes[caractere]
		if temTrad && trad.definicao != "" {
			comTraducao++
			campos["definition"] = jsonString(trad.definicao)
		} else {
			delete(campos, "definition")
			semTraducao++
		}

		aplicarDica(campos, trad.dica)

		saidaLinha, err := json.Marshal(campos)
		if err != nil {
			return 0, 0, 0, fmt.Errorf("falha ao serializar %q: %w", caractere, err)
		}
		escritor.Write(saidaLinha)
		escritor.WriteByte('\n')
		total++
	}

	if err := varredor.Err(); err != nil {
		return 0, 0, 0, err
	}
	return total, comTraducao, semTraducao, escritor.Flush()
}

// ----- Seção: Interpretação de Argumentos e Configuração -----

func interpretarArgumentos(args []string) (configuracaoIdioma, error) {
	if len(args) == 0 {
		return configuracaoIdioma{}, fmt.Errorf("uso: mesclar_makemeahanzi.go <pt|es>\n  ex.: go run .../mesclar_makemeahanzi.go es\n  ex.: go run .../mesclar_makemeahanzi.go pt")
	}

	idioma := strings.ToLower(strings.TrimSpace(args[0]))
	idioma = strings.ReplaceAll(idioma, "_", "-")

	switch idioma {
	case "pt", "pt-br":
		return configuracaoIdioma{
			codigo:       "pt",
			sufixoLote:   "_pt",
			caminhoSaida: "wails_app/dicionario/fontes/pt-BR/makemeahanzi.txt",
		}, nil
	case "es":
		return configuracaoIdioma{
			codigo:       "es",
			sufixoLote:   "_es",
			caminhoSaida: "wails_app/dicionario/fontes/es/makemeahanzi.txt",
		}, nil
	default:
		return configuracaoIdioma{}, fmt.Errorf("idioma não suportado %q: use 'pt' (ou 'pt-br') ou 'es'", args[0])
	}
}

// ----- Seção: Utilitários -----

func aplicarDica(campos map[string]json.RawMessage, dica string) {
	rawEtim, temEtim := campos["etymology"]

	if dica == "" && !temEtim {
		return
	}

	etim := map[string]json.RawMessage{}
	if temEtim {
		_ = json.Unmarshal(rawEtim, &etim)
	}

	if dica != "" {
		etim["hint"] = jsonString(dica)
	} else {
		delete(etim, "hint")
	}

	if len(etim) == 0 {
		delete(campos, "etymology")
		return
	}
	if novo, err := json.Marshal(etim); err == nil {
		campos["etymology"] = novo
	}
}


func jsonString(s string) json.RawMessage {
	b, _ := json.Marshal(s)
	return b
}
