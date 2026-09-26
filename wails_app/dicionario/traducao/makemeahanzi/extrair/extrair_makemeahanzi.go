package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

const linhasPorLote = 100

// entradaMakemeahanzi espelha só os campos que interessam do makemeahanzi_dictionary.txt.
type entradaMakemeahanzi struct {
	Character  string `json:"character"`
	Definition string `json:"definition"`
	Etymology  struct {
		Hint string `json:"hint"`
	} `json:"etymology"`
}

type linhaExtraida struct {
	character  string
	definition string
	hint       string
}

const cabecalhoLote = "# FORMATO (não traduzir esta linha): CARACTERE | DEFINIÇÃO: <texto em inglês> | DICA: <contexto/etimologia em inglês>\n" +
	"# Traduza apenas o texto após \"DEFINIÇÃO:\" e após \"DICA:\" para português. DICA é só uma dica de contexto, pode ficar vazia.\n" +
	"# Responda com o mesmo formato, mesma ordem, uma linha por caractere, sem pular nenhuma.\n"

// formatarLinha grava cada entrada com as colunas rotuladas explicitamente, para não depender
// de espaço/tab como separador (uma LLM pode confundir isso com parte da definição).
func formatarLinha(l linhaExtraida) string {
	return fmt.Sprintf("%s | DEFINIÇÃO: %s | DICA: %s\n", l.character, l.definition, l.hint)
}

func main() {
	caminhoEntrada := "wails_app/dicionario/fontes/en/makemeahanzi.txt"
	caminhoSaida := "wails_app/dicionario/traducao/makemeahanzi_en.txt"
	pastaLotes := "wails_app/dicionario/traducao/lotes"

	linhas, semDefinicao, err := extrairLinhas(caminhoEntrada)
	if err != nil {
		fmt.Printf("Erro ao ler %s: %v\n", caminhoEntrada, err)
		os.Exit(1)
	}

	if err := gravarArquivoUnico(caminhoSaida, linhas); err != nil {
		fmt.Printf("Erro ao gravar %s: %v\n", caminhoSaida, err)
		os.Exit(1)
	}

	numLotes, err := gravarLotes(pastaLotes, linhas)
	if err != nil {
		fmt.Printf("Erro ao gravar lotes: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Concluído: %d entradas exportadas para %s (%d sem definição, ignoradas)\n", len(linhas), caminhoSaida, semDefinicao)
	fmt.Printf("Gerados %d lotes de até %d linhas em %s\n", numLotes, linhasPorLote, pastaLotes)
}

// extrairLinhas lê o dicionário linha a linha e retorna character/definition/hint de cada entrada com definição.
func extrairLinhas(caminho string) ([]linhaExtraida, int, error) {
	arquivo, err := os.Open(caminho)
	if err != nil {
		return nil, 0, err
	}
	defer arquivo.Close()

	varredor := bufio.NewScanner(arquivo)
	varredor.Buffer(make([]byte, 1024*1024), 1024*1024)

	var linhas []linhaExtraida
	semDefinicao := 0

	for varredor.Scan() {
		var entrada entradaMakemeahanzi
		if err := json.Unmarshal(varredor.Bytes(), &entrada); err != nil {
			continue
		}

		if entrada.Definition == "" {
			semDefinicao++
			continue
		}

		linhas = append(linhas, linhaExtraida{
			character:  entrada.Character,
			definition: entrada.Definition,
			hint:       entrada.Etymology.Hint,
		})
	}

	if err := varredor.Err(); err != nil {
		return nil, 0, err
	}

	return linhas, semDefinicao, nil
}

// gravarArquivoUnico grava todas as linhas extraídas num único arquivo de referência.
func gravarArquivoUnico(caminho string, linhas []linhaExtraida) error {
	arquivo, err := os.Create(caminho)
	if err != nil {
		return err
	}
	defer arquivo.Close()

	escritor := bufio.NewWriter(arquivo)
	defer escritor.Flush()

	for _, l := range linhas {
		escritor.WriteString(formatarLinha(l))
	}

	return escritor.Flush()
}

// gravarLotes divide as linhas em arquivos de até linhasPorLote cada, para envio individual a uma LLM.
func gravarLotes(pasta string, linhas []linhaExtraida) (int, error) {
	if err := os.MkdirAll(pasta, 0755); err != nil {
		return 0, err
	}

	antigos, err := filepath.Glob(filepath.Join(pasta, "lote_*.txt"))
	if err != nil {
		return 0, err
	}
	for _, antigo := range antigos {
		if err := os.Remove(antigo); err != nil {
			return 0, err
		}
	}

	numLotes := (len(linhas) + linhasPorLote - 1) / linhasPorLote

	for i := 0; i < numLotes; i++ {
		inicio := i * linhasPorLote
		fim := inicio + linhasPorLote
		if fim > len(linhas) {
			fim = len(linhas)
		}

		caminhoLote := filepath.Join(pasta, fmt.Sprintf("lote_%03d.txt", i+1))
		arquivo, err := os.Create(caminhoLote)
		if err != nil {
			return 0, err
		}

		escritor := bufio.NewWriter(arquivo)
		escritor.WriteString(cabecalhoLote)
		for _, l := range linhas[inicio:fim] {
			escritor.WriteString(formatarLinha(l))
		}
		if err := escritor.Flush(); err != nil {
			arquivo.Close()
			return 0, err
		}
		arquivo.Close()
	}

	return numLotes, nil
}
