package main

// ----- Seção: Validador dos lotes do CEDICT -----
// Audita um ou todos os lotes traduzidos (lote_NNN_<idioma>.txt) contra o lote_NNN.txt original.
// Regras por linha:
//   - A PALAVRA (chave hanzi) tem que ser idêntica byte a byte.
//   - A quantidade de significados por grupo é LIVRE (o idioma alvo pode ter mais ou menos acepções que o inglês),
//     mas nenhum significado pode ser vazio.
//   - Palavras com mais de uma leitura (grupos "‖" no original) têm que preservar a quantidade de grupos.
//   - Traduções legadas (sem "‖") são aceitas se o total de significados for igual ao do original.
//
// Uso (executar da raiz do repositório):
//	go run wails_app/dicionario/traducao/cedict/validar/validar_lote.go es 001    → valida o lote 001 em espanhol
//	go run wails_app/dicionario/traducao/cedict/validar/validar_lote.go es all    → valida todos os lotes em espanhol
//	go run wails_app/dicionario/traducao/cedict/validar/validar_lote.go pt 001    → valida o lote 001 em português
//	go run wails_app/dicionario/traducao/cedict/validar/validar_lote.go pt all    → valida todos os lotes em português

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// ----- Constantes -----

const (
	MARCADOR           = " | TRADUÇÃO: "
	SEPARADOR_LEITURAS = "‖"
	PASTA_LOTES        = "wails_app/dicionario/traducao/cedict/lotes"
	PRIMEIRO_LOTE      = 1
	ULTIMO_LOTE        = 122
)


func main() {
	codigoIdioma, sufixoArquivo, alvos, err := interpretarArgumentosValidacao(os.Args[1:])
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	if len(alvos) == 1 && alvos[0] == "all" {
		total, semTraducao, comErro := 0, 0, 0
		for n := PRIMEIRO_LOTE; n <= ULTIMO_LOTE; n++ {
			nnn := fmt.Sprintf("%03d", n)
			caminhoTraduzido := filepath.Join(PASTA_LOTES, fmt.Sprintf("lote_%s%s.txt", nnn, sufixoArquivo))
			if _, err := os.Stat(caminhoTraduzido); os.IsNotExist(err) {
				semTraducao++
				continue
			}
			total++
			if !validarLote(PASTA_LOTES, nnn, sufixoArquivo, codigoIdioma) {
				comErro++
			}
		}
		fmt.Printf("\n===== RESUMO (%s) =====\n%d lotes traduzidos e conferidos, %d ainda sem lote_NNN%s.txt, %d com erro\n",
			strings.ToUpper(codigoIdioma), total-comErro, semTraducao, sufixoArquivo, comErro)
		if comErro > 0 || semTraducao > 0 {
			os.Exit(1)
		}
		return
	}

	todosValidos := true
	for _, nnn := range alvos {
		if !validarLote(PASTA_LOTES, nnn, sufixoArquivo, codigoIdioma) {
			todosValidos = false
		}
	}
	if !todosValidos {
		os.Exit(1)
	}
}


func validarLote(pasta, nnn, sufixoArquivo, codigoIdioma string) bool {
	original, err := lerLinhasDados(filepath.Join(pasta, fmt.Sprintf("lote_%s.txt", nnn)))
	if err != nil {
		fmt.Printf("lote %s: erro ao ler original: %v\n", nnn, err)
		return false
	}
	traduzido, err := lerLinhasDados(filepath.Join(pasta, fmt.Sprintf("lote_%s%s.txt", nnn, sufixoArquivo)))
	if err != nil {
		fmt.Printf("lote %s: erro ao ler tradução (%s): %v\n", nnn, codigoIdioma, err)
		return false
	}

	ok := true
	if len(original) != len(traduzido) {
		fmt.Printf("lote %s: FALHA — %d linhas no original, %d na tradução (linha faltando ou sobrando)\n", nnn, len(original), len(traduzido))
		ok = false
	}

	limite := len(original)
	if len(traduzido) < limite {
		limite = len(traduzido)
	}

	for i := 0; i < limite; i++ {
		problemas := problemasDaLinha(original[i], traduzido[i])
		if len(problemas) == 0 {
			continue
		}
		ok = false
		for _, problema := range problemas {
			fmt.Printf("lote %s linha %d: %s\n", nnn, i+1, problema)
		}
	}

	if ok {
		fmt.Printf("lote %s: OK (%d linhas conferidas em %s)\n", nnn, len(original), codigoIdioma)
	}
	return ok
}


func problemasDaLinha(linhaOriginal, linhaTraduzida string) []string {
	palavraOrig, gruposOrig, errO := dividirLinha(linhaOriginal)
	if errO != nil {
		return []string{fmt.Sprintf("original malformada: %v", errO)}
	}
	palavraTrad, gruposTrad, errT := dividirLinha(linhaTraduzida)
	if errT != nil {
		return []string{fmt.Sprintf("tradução malformada (%q): %v", linhaTraduzida, errT)}
	}

	var problemas []string
	if palavraOrig != palavraTrad {
		problemas = append(problemas, fmt.Sprintf("PALAVRA mudou — original %q, tradução %q", palavraOrig, palavraTrad))
	}

	if len(gruposTrad) > 1 || len(gruposOrig) == 1 {
		if len(gruposTrad) != len(gruposOrig) {
			problemas = append(problemas, fmt.Sprintf("(%s): quantidade de grupos ‖ mudou — original %d, tradução %d", palavraOrig, len(gruposOrig), len(gruposTrad)))
		}
	} else {
		totalOrig := contarSignificados(gruposOrig)
		totalTrad := contarSignificados(gruposTrad)
		if totalOrig != totalTrad {
			problemas = append(problemas, fmt.Sprintf("(%s): multi-leitura sem ‖ (legado) com total de significados diferente — original %d, tradução %d", palavraOrig, totalOrig, totalTrad))
		}
	}

	if vazio := primeiroVazio(gruposTrad); vazio != "" {
		problemas = append(problemas, fmt.Sprintf("(%s): %s", palavraOrig, vazio))
	}

	return problemas
}

// ----- Seção: Interpretação de Argumentos -----

func interpretarArgumentosValidacao(args []string) (codigoIdioma, sufixoArquivo string, alvos []string, err error) {
	if len(args) == 0 {
		return "", "", nil, fmt.Errorf("uso: validar_lote.go <pt|es> <NNN|all>\n  ex.: go run .../validar_lote.go es 001\n  ex.: go run .../validar_lote.go es all\n  ex.: go run .../validar_lote.go pt all")
	}

	primeiro := strings.ToLower(strings.TrimSpace(args[0]))
	primeiro = strings.ReplaceAll(primeiro, "_", "-")

	var resto []string
	if primeiro == "pt" || primeiro == "pt-br" || primeiro == "es" {
		codigoIdioma = "pt"
		sufixoArquivo = "_pt"
		if primeiro == "es" {
			codigoIdioma = "es"
			sufixoArquivo = "_es"
		}
		resto = args[1:]
		if len(resto) == 0 {
			return codigoIdioma, sufixoArquivo, []string{"all"}, nil
		}
	} else {
		codigoIdioma = "pt"
		sufixoArquivo = "_pt"
		resto = args
	}

	for _, arg := range resto {
		if arg == "all" {
			if len(resto) > 1 {
				return "", "", nil, fmt.Errorf("o argumento 'all' deve ser usado isoladamente")
			}
			alvos = append(alvos, "all")
			continue
		}
		n, err := strconv.Atoi(arg)
		if err != nil || len(arg) != 3 || n < PRIMEIRO_LOTE || n > ULTIMO_LOTE {
			return "", "", nil, fmt.Errorf("argumento inválido %q: use 'all' ou números de 3 dígitos entre %03d e %03d", arg, PRIMEIRO_LOTE, ULTIMO_LOTE)
		}
		alvos = append(alvos, fmt.Sprintf("%03d", n))
	}

	return codigoIdioma, sufixoArquivo, alvos, nil
}

// ----- Seção: Utilitários -----

func lerLinhasDados(caminho string) ([]string, error) {
	arquivo, err := os.Open(caminho)
	if err != nil {
		return nil, err
	}
	defer arquivo.Close()

	varredor := bufio.NewScanner(arquivo)
	varredor.Buffer(make([]byte, 1024*1024), 1024*1024)

	var linhas []string
	for varredor.Scan() {
		linha := varredor.Text()
		if strings.HasPrefix(linha, "#") || strings.TrimSpace(linha) == "" {
			continue
		}
		linhas = append(linhas, linha)
	}
	return linhas, varredor.Err()
}


func dividirLinha(linha string) (string, []string, error) {
	idx := strings.Index(linha, MARCADOR)
	if idx < 0 {
		return "", nil, fmt.Errorf("marcador %q não encontrado", MARCADOR)
	}
	palavra := linha[:idx]
	resto := linha[idx+len(MARCADOR):]
	if palavra == "" || resto == "" {
		return "", nil, fmt.Errorf("palavra ou tradução vazia")
	}
	grupos := strings.Split(resto, SEPARADOR_LEITURAS)
	for i := range grupos {
		grupos[i] = strings.TrimSpace(grupos[i])
	}
	return palavra, grupos, nil
}


func contarSignificados(grupos []string) int {
	total := 0
	for _, grupo := range grupos {
		total += len(strings.Split(grupo, "/"))
	}
	return total
}


func primeiroVazio(grupos []string) string {
	for _, grupo := range grupos {
		if grupo == "" {
			return "grupo de leitura vazio na tradução"
		}
		for _, significado := range strings.Split(grupo, "/") {
			if strings.TrimSpace(significado) == "" {
				return "significado vazio na tradução"
			}
		}
	}
	return ""
}
