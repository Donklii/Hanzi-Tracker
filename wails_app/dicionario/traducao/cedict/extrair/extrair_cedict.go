package main

// ----- Seção: Extração dos lotes de tradução do CEDICT -----
// Lê o cedict.u8 (inglês) e gera cedict_en.txt + lotes/lote_NNN.txt para tradução via LLM.
// Cada linha de lote agrupa por SIMPLIFICADO (a chave de fusão de dicionario/fusao) os significados
// de todas as linhas cruas do CEDICT. Linhas cruas diferentes da mesma palavra (leituras/formas
// distintas) viram grupos separados por " ‖ ": a mesclagem futura redistribui os grupos às linhas
// originais SEM depender de contagem exata de significados — a quantidade de "/" por grupo é livre.
// O pinyin entre colchetes imediatamente após hanzi citado nas glosas (ex.: 臺[tai2]) é removido:
// não precisa de tradução e gastaria token na ida e na volta (o modelo ecoa referências verbatim).
// Colchetes que NÃO seguem hanzi (ex.: "Taiwan pr. [xx]") são conteúdo e permanecem.
//
// Rodar da RAIZ do repo:
//
//	go run wails_app/dicionario/traducao/cedict/extrair/extrair_cedict.go

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// ----- Constantes -----

const (
	LINHAS_POR_LOTE    = 1000
	SEPARADOR_LEITURAS = " ‖ " // entre grupos de significados vindos de linhas cruas diferentes da mesma palavra
)

const CABECALHO_LOTE = "# FORMATO (não traduzir esta linha): PALAVRA | TRADUÇÃO: <significados separados por \"/\"; leituras diferentes separadas por \" ‖ \">\n" +
	"# Traduza os significados diretamente do chinês para o português; a QUANTIDADE de significados por grupo é livre, mas os grupos \"‖\" devem manter a mesma quantidade e a mesma ordem.\n" +
	"# NUNCA altere a PALAVRA antes do \" | TRADUÇÃO:\" — ela é a chave usada para juntar a tradução de volta ao dicionário original.\n" +
	"# Responda com o mesmo formato, mesma ordem, uma linha por palavra, sem pular nenhuma.\n"

// grupoCedict junta, por SIMPLIFICADO, os significados de cada linha crua do CEDICT que compartilha
// a palavra (um grupo por linha crua, na ordem de aparição). Pinyin e tradicional da própria palavra
// ficam de fora do lote (não precisam de tradução e só gastariam token); eles continuam disponíveis
// no cedict.u8 original para a futura mesclagem por simplificado+ordem.
type grupoCedict struct {
	simplificado         string
	significadosPorLinha [][]string
}


func main() {
	caminhoEntrada := "wails_app/dicionario/fontes/en/cedict.u8"
	caminhoSaida := "wails_app/dicionario/traducao/cedict/cedict_en.txt"
	pastaLotes := "wails_app/dicionario/traducao/cedict/lotes"

	grupos, totalLinhasCru, err := extrairGrupos(caminhoEntrada)
	if err != nil {
		fmt.Printf("Erro ao ler %s: %v\n", caminhoEntrada, err)
		os.Exit(1)
	}

	if err := gravarArquivoUnico(caminhoSaida, grupos); err != nil {
		fmt.Printf("Erro ao gravar %s: %v\n", caminhoSaida, err)
		os.Exit(1)
	}

	numLotes, err := gravarLotes(pastaLotes, grupos)
	if err != nil {
		fmt.Printf("Erro ao gravar lotes: %v\n", err)
		os.Exit(1)
	}

	multiLeitura := 0
	for _, g := range grupos {
		if len(g.significadosPorLinha) > 1 {
			multiLeitura++
		}
	}

	fmt.Printf("Concluído: %d linhas cruas do CEDICT agrupadas em %d palavras simplificadas, exportadas para %s\n", totalLinhasCru, len(grupos), caminhoSaida)
	fmt.Printf("%d palavras com mais de uma leitura (grupos separados por %q)\n", multiLeitura, strings.TrimSpace(SEPARADOR_LEITURAS))
	fmt.Printf("Gerados %d lotes de até %d linhas em %s\n", numLotes, LINHAS_POR_LOTE, pastaLotes)
}


// extrairGrupos lê o CC-CEDICT linha a linha (mesmo parsing de dicionario/cedict.go) e agrupa os
// significados por simplificado, preservando a ordem de primeira aparição e mantendo os significados
// de cada linha crua como um grupo separado.
func extrairGrupos(caminho string) ([]grupoCedict, int, error) {
	arquivo, err := os.Open(caminho)
	if err != nil {
		return nil, 0, err
	}
	defer arquivo.Close()

	varredor := bufio.NewScanner(arquivo)
	varredor.Buffer(make([]byte, 1024*1024), 1024*1024)

	indice := map[string]int{}
	var grupos []grupoCedict
	totalLinhasCru := 0

	for varredor.Scan() {
		linha := varredor.Text()
		if strings.HasPrefix(linha, "#") || strings.TrimSpace(linha) == "" {
			continue
		}

		parts := strings.SplitN(linha, " [", 2)
		if len(parts) != 2 {
			continue
		}
		palavras := strings.Fields(parts[0])
		if len(palavras) != 2 {
			continue
		}
		simplificado := palavras[1]

		parts2 := strings.SplitN(parts[1], "] /", 2)
		if len(parts2) != 2 {
			continue
		}
		significadosStr := strings.TrimSuffix(parts2[1], "/")
		if significadosStr == "" {
			continue
		}
		significados := strings.Split(removerPinyinAposHanzi(significadosStr), "/")

		totalLinhasCru++
		if i, ok := indice[simplificado]; ok {
			grupos[i].significadosPorLinha = append(grupos[i].significadosPorLinha, significados)
			continue
		}
		indice[simplificado] = len(grupos)
		grupos = append(grupos, grupoCedict{simplificado: simplificado, significadosPorLinha: [][]string{significados}})
	}

	if err := varredor.Err(); err != nil {
		return nil, 0, err
	}

	return grupos, totalLinhasCru, nil
}


// gravarArquivoUnico grava todos os grupos extraídos num único arquivo de referência.
func gravarArquivoUnico(caminho string, grupos []grupoCedict) error {
	if err := os.MkdirAll(filepath.Dir(caminho), 0755); err != nil {
		return err
	}
	arquivo, err := os.Create(caminho)
	if err != nil {
		return err
	}
	defer arquivo.Close()

	escritor := bufio.NewWriter(arquivo)
	defer escritor.Flush()

	for _, g := range grupos {
		escritor.WriteString(formatarLinha(g))
	}

	return escritor.Flush()
}


// gravarLotes divide os grupos em arquivos de até LINHAS_POR_LOTE cada, para envio individual a uma LLM.
func gravarLotes(pasta string, grupos []grupoCedict) (int, error) {
	if err := os.MkdirAll(pasta, 0755); err != nil {
		return 0, err
	}

	antigos, err := filepath.Glob(filepath.Join(pasta, "lote_???.txt"))
	if err != nil {
		return 0, err
	}
	for _, antigo := range antigos {
		if err := os.Remove(antigo); err != nil {
			return 0, err
		}
	}

	numLotes := (len(grupos) + LINHAS_POR_LOTE - 1) / LINHAS_POR_LOTE

	for i := 0; i < numLotes; i++ {
		inicio := i * LINHAS_POR_LOTE
		fim := inicio + LINHAS_POR_LOTE
		if fim > len(grupos) {
			fim = len(grupos)
		}

		caminhoLote := filepath.Join(pasta, fmt.Sprintf("lote_%03d.txt", i+1))
		arquivo, err := os.Create(caminhoLote)
		if err != nil {
			return 0, err
		}

		escritor := bufio.NewWriter(arquivo)
		escritor.WriteString(CABECALHO_LOTE)
		for _, g := range grupos[inicio:fim] {
			escritor.WriteString(formatarLinha(g))
		}
		if err := escritor.Flush(); err != nil {
			arquivo.Close()
			return 0, err
		}
		arquivo.Close()
	}

	return numLotes, nil
}

// ----- Utilitários -----

// formatarLinha grava cada grupo com a coluna rotulada explicitamente, para não depender de
// espaço/tab como separador (uma LLM pode confundir isso com parte da tradução).
func formatarLinha(g grupoCedict) string {
	partes := make([]string, len(g.significadosPorLinha))
	for i, significados := range g.significadosPorLinha {
		partes[i] = strings.Join(significados, "/")
	}
	return fmt.Sprintf("%s | TRADUÇÃO: %s\n", g.simplificado, strings.Join(partes, SEPARADOR_LEITURAS))
}


var padraoPinyinAposHanzi = regexp.MustCompile(`(\p{Han})\[[^\]]*\]`)

// removerPinyinAposHanzi apaga o pinyin entre colchetes que segue imediatamente um hanzi citado numa
// glosa (ex.: "variant of 臺[tai2]" → "variant of 臺"). Colchetes sem hanzi imediatamente antes
// (ex.: "Taiwan pr. [xia4]") são conteúdo da glosa e ficam intactos. O laço cobre colchetes em
// sequência ("臺[tai2][xxx]"), que o ReplaceAll sozinho deixaria escapar.
func removerPinyinAposHanzi(texto string) string {
	for {
		novo := padraoPinyinAposHanzi.ReplaceAllString(texto, "${1}")
		if novo == texto {
			return novo
		}
		texto = novo
	}
}
