package main

import (
	"bufio"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	"wails_app/dicionario"
)

// Estrutura do JSON do C3
// A raiz é um array de documentos.
// Documento = [Contexto, Perguntas, ID]
// Contexto = Array de strings
// Perguntas = Array de objetos {question, choice, answer}
// ID = String

const (
	maxCaracteresContexto = 200
)

func main() {
	fmt.Println("Iniciando processamento do dataset C3...")
	arquivosEntrada := []string{
		"../../fontes/compreensao/c3/d-train.json",
		"../../fontes/compreensao/c3/m-train.json",
		"../../fontes/compreensao/c3/d-dev.json",
		"../../fontes/compreensao/c3/m-dev.json",
	}

	caminhoSaida := "../../idiomas/compartilhado/compreensao/c3.jsonl.gz"

	if err := os.MkdirAll(filepath.Dir(caminhoSaida), 0755); err != nil {
		fmt.Printf("Erro ao criar diretorio de saida: %v\n", err)
		os.Exit(1)
	}

	arquivoSaida, err := os.Create(caminhoSaida)
	if err != nil {
		fmt.Printf("Erro ao criar arquivo de saida: %v\n", err)
		os.Exit(1)
	}
	defer arquivoSaida.Close()

	escritorGz, err := gzip.NewWriterLevel(arquivoSaida, gzip.BestCompression)
	if err != nil {
		fmt.Printf("Erro ao criar escritor gzip: %v\n", err)
		os.Exit(1)
	}
	// Limpar metadados para versionamento determinístico
	escritorGz.Name = ""
	escritorGz.ModTime = time.Time{}
	defer escritorGz.Close()

	bufonado := bufio.NewWriterSize(escritorGz, 1<<20)
	codificador := json.NewEncoder(bufonado)
	codificador.SetEscapeHTML(false)

	totalLidos := 0
	totalSalvos := 0

	for _, arquivo := range arquivosEntrada {
		fmt.Printf("Processando %s...\n", arquivo)
		dados, err := os.ReadFile(arquivo)
		if err != nil {
			fmt.Printf("Aviso: arquivo nao encontrado %s: %v\n", arquivo, err)
			continue
		}

		nomeBase := filepath.Base(arquivo)
		categoria := "m"
		if strings.HasPrefix(nomeBase, "d-") {
			categoria = "d"
		}

		// O JSON raiz é uma matriz de arrays de interfaces
		var documentos [][]interface{}
		if err := json.Unmarshal(dados, &documentos); err != nil {
			fmt.Printf("Erro ao decodificar %s: %v\n", arquivo, err)
			continue
		}

		for _, doc := range documentos {
			if len(doc) < 2 {
				continue
			}

			// Contexto: Array de strings
			linhasContextoObj, ok := doc[0].([]interface{})
			if !ok {
				continue
			}
			var linhasContexto []string
			for _, l := range linhasContextoObj {
				if s, ok := l.(string); ok {
					linhasContexto = append(linhasContexto, s)
				}
			}
			contextoStr := strings.Join(linhasContexto, "\n")
			
			// Remover pontuações desnecessárias ou trim
			contextoStr = strings.TrimSpace(contextoStr)
			
			if contextoStr == "" {
				continue
			}
			
			// Filtrar por tamanho
			if utf8.RuneCountInString(contextoStr) > maxCaracteresContexto {
				continue
			}

			// Perguntas
			perguntasObj, ok := doc[1].([]interface{})
			if !ok {
				continue
			}

			for _, pObj := range perguntasObj {
				pMapa, ok := pObj.(map[string]interface{})
				if !ok {
					continue
				}

				perguntaTxt, _ := pMapa["question"].(string)
				respostaTxt, _ := pMapa["answer"].(string)
				opcoesObj, ok := pMapa["choice"].([]interface{})
				if !ok || len(opcoesObj) != 4 { // Exigir 4 opções
					continue
				}

				var opcoes []string
				indiceResposta := -1
				for i, opObj := range opcoesObj {
					if opStr, ok := opObj.(string); ok {
						opcoes = append(opcoes, opStr)
						if opStr == respostaTxt {
							indiceResposta = i
						}
					}
				}

				if indiceResposta == -1 || len(opcoes) != 4 || perguntaTxt == "" {
					continue
				}
				
				totalLidos++

				novaPergunta := dicionario.PerguntaCompreensao{
					Contexto:              contextoStr,
					Pergunta:              perguntaTxt,
					Opcoes:                opcoes,
					IndiceRespostaCorreta: indiceResposta,
					Atribuicao:            "Dataset C3 (Chinese Machine Reading Comprehension)",
					Categoria:             categoria,
				}

				if err := codificador.Encode(novaPergunta); err != nil {
					fmt.Printf("Erro ao encodar pergunta: %v\n", err)
					continue
				}
				totalSalvos++
			}
		}
	}

	bufonado.Flush()
	fmt.Printf("Processamento concluido! %d perguntas validadas e salvas de um total lido compativel de %d\n", totalSalvos, totalLidos)
}
