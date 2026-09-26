package main

// ----- Seção: Propagação das classificações de frases entre idiomas -----
//
// Passo BARATO e sem API. A classificação (tema + dificuldade) é feita uma vez sobre o corpus grande
// — o inglês — pelo passo frases/classificar. Aqui ela é ESPALHADA para outro idioma cujas frases
// tenham o MESMO texto chinês: o rótulo depende só do chinês, então não há por que repagar o DeepSeek.
//
// A regra é literal: casa-se por igualdade EXATA do texto chinês (col. 0). Uma frase pt-BR cujo chinês
// não exista no corpus de origem simplesmente fica sem classificação (e é reportada no resumo) — nunca
// se adivinha rótulo aqui.
//
// Entrada de origem:  frases/<origem>/classificacoes.tsv     (chinês <TAB> tema <TAB> dificuldade)
// Frases do alvo:     idiomas/<alvo>/frases/*.tsv.gz          (col. 0 = chinês)
// Saída:              frases/<alvo>/classificacoes.tsv        (chinês <TAB> tema <TAB> dificuldade)
//
// Determinístico: a saída sai ordenada por chinês. Rodar da RAIZ do repo:
//
//	go run ./dicionario/frases/espalhar [origem] [alvo]     (padrão: en -> pt-BR)

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
	origemPadrao = "en"
	alvoPadrao   = "pt-BR"

	moldeClassificacao = "dicionario/frases/%s/classificacoes.tsv"
	moldeFrasesIdioma  = "dicionario/idiomas/%s/frases"
)

func main() {
	origem, alvo := lerArgumentos(os.Args[1:])
	if origem == alvo {
		abortar(fmt.Errorf("origem e alvo são o mesmo idioma (%q); nada a propagar", origem))
	}

	caminhoOrigem := fmt.Sprintf(moldeClassificacao, origem)
	classificacoes, err := lerClassificacoes(caminhoOrigem)
	if err != nil {
		abortar(err)
	}
	fmt.Printf("Classificações de origem (%s): %d\n", origem, len(classificacoes))

	chinesesAlvo, err := lerChinesesUnicos(fmt.Sprintf(moldeFrasesIdioma, alvo))
	if err != nil {
		abortar(err)
	}
	fmt.Printf("Frases (chineses únicos) no alvo (%s): %d\n", alvo, len(chinesesAlvo))

	caminhoAlvo := fmt.Sprintf(moldeClassificacao, alvo)
	casadas, semClassificacao := propagar(caminhoAlvo, chinesesAlvo, classificacoes)

	fmt.Printf("\n===== RESUMO =====\n")
	fmt.Printf("Saída: %s\n", caminhoAlvo)
	fmt.Printf("Propagadas (chinês idêntico): %d\n", casadas)
	fmt.Printf("Sem correspondência na origem: %d\n", len(semClassificacao))
	for i, ch := range semClassificacao {
		if i >= 10 {
			fmt.Printf("  … e mais %d\n", len(semClassificacao)-10)
			break
		}
		fmt.Printf("  %s\n", ch)
	}
}

// propagar escreve, ordenado por chinês, a classificação de cada frase do alvo que exista na origem.
// Devolve quantas casaram e a lista das que ficaram sem correspondência.
func propagar(caminhoAlvo string, chinesesAlvo []string, classificacoes map[string]string) (casadas int, semCorrespondencia []string) {
	if err := os.MkdirAll(filepath.Dir(caminhoAlvo), 0o755); err != nil {
		abortar(err)
	}

	var saida strings.Builder
	for _, ch := range chinesesAlvo {
		linha, ok := classificacoes[ch]
		if !ok {
			semCorrespondencia = append(semCorrespondencia, ch)
			continue
		}
		fmt.Fprintf(&saida, "%s\t%s\n", ch, linha)
		casadas++
	}
	abortar(os.WriteFile(caminhoAlvo, []byte(saida.String()), 0o644))
	return casadas, semCorrespondencia
}

// lerClassificacoes lê o classificacoes.tsv da origem e devolve chinês → "tema\tdificuldade" (os dois
// campos já juntos, prontos para reemitir). Linhas malformadas são ignoradas.
func lerClassificacoes(caminho string) (map[string]string, error) {
	dados, err := os.ReadFile(caminho)
	if err != nil {
		return nil, fmt.Errorf("não foi possível ler as classificações de origem %q (rode antes o passo frases/classificar): %w", caminho, err)
	}
	classificacoes := map[string]string{}
	for _, linha := range strings.Split(string(dados), "\n") {
		campos := strings.Split(linha, "\t")
		if len(campos) != 3 || campos[0] == "" || campos[1] == "" || campos[2] == "" {
			continue
		}
		classificacoes[campos[0]] = campos[1] + "\t" + campos[2]
	}
	return classificacoes, nil
}

// lerChinesesUnicos lê a coluna 0 (chinês) de todos os .tsv.gz do diretório de frases do idioma alvo e
// devolve os textos únicos, ordenados.
func lerChinesesUnicos(dir string) ([]string, error) {
	arquivos, err := filepath.Glob(filepath.Join(dir, "*.tsv.gz"))
	if err != nil {
		return nil, err
	}
	if len(arquivos) == 0 {
		return nil, fmt.Errorf("nenhum arquivo de frases (*.tsv.gz) em %q", dir)
	}
	sort.Strings(arquivos)

	vistos := map[string]bool{}
	for _, caminho := range arquivos {
		dados, err := os.ReadFile(caminho)
		if err != nil {
			return nil, fmt.Errorf("não foi possível ler %q: %w", caminho, err)
		}
		gz, err := gzip.NewReader(bytes.NewReader(dados))
		if err != nil {
			return nil, fmt.Errorf("%q corrompido: %w", caminho, err)
		}
		varredor := bufio.NewScanner(gz)
		varredor.Buffer(make([]byte, 1024*1024), 1024*1024)
		for varredor.Scan() {
			campos := strings.SplitN(varredor.Text(), "\t", 2)
			if len(campos) < 1 || campos[0] == "" {
				continue
			}
			vistos[campos[0]] = true
		}
		erroVarredura := varredor.Err()
		gz.Close()
		if erroVarredura != nil {
			return nil, erroVarredura
		}
	}

	chineses := make([]string, 0, len(vistos))
	for ch := range vistos {
		chineses = append(chineses, ch)
	}
	sort.Strings(chineses)
	return chineses, nil
}

func lerArgumentos(args []string) (origem, alvo string) {
	origem, alvo = origemPadrao, alvoPadrao
	if len(args) >= 1 && args[0] != "" {
		origem = args[0]
	}
	if len(args) >= 2 && args[1] != "" {
		alvo = args[1]
	}
	return origem, alvo
}

func abortar(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, "erro:", err)
		os.Exit(1)
	}
}
