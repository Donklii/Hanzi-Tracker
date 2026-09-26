package main

// ----- Seção: Validador de Lotes do MakeMeAHanzi -----
// Audita um ou todos os lotes traduzidos (lote_NNN_<idioma>.txt) contra o lote_NNN.txt original.
// Regras por linha:
//   - O CARACTERE (chave hanzi antes do marcador) tem que ser idêntico byte a byte.
//   - O marcador " | DEFINIÇÃO: " deve estar presente e a definição não pode ser vazia.
//   - A contagem de linhas e a ordem devem corresponder exatamente ao arquivo original.
//
// Uso (executar da raiz do repositório):
//	go run wails_app/dicionario/traducao/makemeahanzi/validar/validar_lotes.go es 001    → valida o lote 001 em espanhol
//	go run wails_app/dicionario/traducao/makemeahanzi/validar/validar_lotes.go es all    → valida todos os lotes em espanhol
//	go run wails_app/dicionario/traducao/makemeahanzi/validar/validar_lotes.go pt 001    → valida o lote 001 em português
//	go run wails_app/dicionario/traducao/makemeahanzi/validar/validar_lotes.go pt all    → valida todos os lotes em português

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
	PASTA_LOTES        = "wails_app/dicionario/traducao/makemeahanzi/lotes"
	MARCADOR_DEFINICAO = " | DEFINIÇÃO: "
	MARCADOR_DICA      = " | DICA: "
	PRIMEIRO_LOTE      = 1
	ULTIMO_LOTE        = 96
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
	charOrig, _, _, errO := dividirLinhaMakemeahanzi(linhaOriginal)
	if errO != nil {
		return []string{fmt.Sprintf("original malformada: %v", errO)}
	}
	charTrad, defTrad, _, errT := dividirLinhaMakemeahanzi(linhaTraduzida)
	if errT != nil {
		return []string{fmt.Sprintf("tradução malformada (%q): %v", linhaTraduzida, errT)}
	}

	var problemas []string
	if charOrig != charTrad {
		problemas = append(problemas, fmt.Sprintf("CARACTERE mudou — original %q, tradução %q", charOrig, charTrad))
	}
	if strings.TrimSpace(defTrad) == "" {
		problemas = append(problemas, fmt.Sprintf("(%s): definição vazia na tradução", charOrig))
	}

	return problemas
}

// ----- Seção: Interpretação de Argumentos e Parsing -----

func interpretarArgumentosValidacao(args []string) (codigoIdioma, sufixoArquivo string, alvos []string, err error) {
	if len(args) == 0 {
		return "", "", nil, fmt.Errorf("uso: validar_lotes.go <pt|es> <NNN|all>\n  ex.: go run .../validar_lotes.go es 001\n  ex.: go run .../validar_lotes.go es all\n  ex.: go run .../validar_lotes.go pt all")
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


func dividirLinhaMakemeahanzi(linha string) (caractere, definicao, dica string, err error) {
	idxDef := strings.Index(linha, MARCADOR_DEFINICAO)
	if idxDef < 0 {
		return "", "", "", fmt.Errorf("marcador %q não encontrado", MARCADOR_DEFINICAO)
	}
	caractere = strings.TrimSpace(linha[:idxDef])
	resto := linha[idxDef+len(MARCADOR_DEFINICAO):]

	definicao = resto
	if idxDica := strings.Index(resto, MARCADOR_DICA); idxDica >= 0 {
		definicao = resto[:idxDica]
		dica = strings.TrimSpace(resto[idxDica+len(MARCADOR_DICA):])
	}
	definicao = strings.TrimSpace(definicao)

	if caractere == "" {
		return "", "", "", fmt.Errorf("caractere vazio")
	}
	return caractere, definicao, dica, nil
}


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
