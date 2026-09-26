package main

// ----- Seção: Fusão das classificações (tema + dificuldade) nas frases embarcadas -----
//
// Passo FINAL do pipeline de frases. Pega os rótulos gerados por frases/classificar (e propagados por
// frases/espalhar) e os funde DENTRO dos próprios arquivos de frases embarcados, virando duas colunas
// novas por linha. Depois disso o app carrega tema/dificuldade junto da frase, sem join em runtime.
//
// Formato antes:  chinês <TAB> tradução <TAB> atribuição
// Formato depois: chinês <TAB> tradução <TAB> atribuição <TAB> tema <TAB> dificuldade
// Frase sem classificação (barrada na moderação, ou chinês sem par no espalhar) fica com as duas
// colunas VAZIAS — o app trata como "sem rótulo". As colunas extras são OPCIONAIS na leitura (o loader
// aceita 3 ou 5 colunas), então rodar a fusão é seguro e reversível.
//
// Idempotente: lê só as 3 colunas base de cada linha (descarta tema/dificuldade que já estejam lá) e
// reaplica a partir do classificacoes.tsv. Preserva a ORDEM original das linhas e regrava o gzip sem
// nome nem timestamp — os arquivos são versionados e precisam ser byte-a-byte reprodutíveis.
//
// Roda para os idiomas passados como argumento, ou para todos os suportados quando sem argumento.
// Rodar da RAIZ do módulo `wails_app`:
//
//	go run ./dicionario/frases/fundir [idioma...]
//
// Pré-requisito: frases/<idioma>/classificacoes.tsv já existir (rode antes classificar e/ou espalhar).

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const (
	moldeClassificacao = "dicionario/frases/%s/classificacoes.tsv"
	moldeDirFrases     = "dicionario/idiomas/%s/frases"

	sufixoFrases       = ".tsv.gz"
	colunasBaseFrase   = 3 // chinês, tradução, atribuição (tema/dificuldade são acrescentados aqui)
)

// idiomasPadrao são os idiomas fundidos quando nenhum é passado por argumento (espelha
// dicionario.IdiomasSuportados; hardcoded para o build tool não arrastar o embed inteiro).
var idiomasPadrao = []string{"en", "pt-BR"}

// classificacao é o par de rótulos de uma frase.
type classificacao struct {
	tema        string
	dificuldade string
}

func main() {
	idiomas := os.Args[1:]
	if len(idiomas) == 0 {
		idiomas = idiomasPadrao
	}

	for _, idioma := range idiomas {
		fundirIdioma(idioma)
	}
}

// fundirIdioma carrega as classificações do idioma e funde-as em cada arquivo de frases embarcado dele.
func fundirIdioma(idioma string) {
	classificacoes := lerClassificacoes(fmt.Sprintf(moldeClassificacao, idioma))
	fmt.Printf("== %s ==\nClassificações carregadas: %d\n", idioma, len(classificacoes))

	arquivos := listarArquivosFrases(fmt.Sprintf(moldeDirFrases, idioma))
	for _, caminho := range arquivos {
		total, comRotulo := fundirArquivo(caminho, classificacoes)
		fmt.Printf("  %s: %d frases, %d com rótulo\n", filepath.Base(caminho), total, comRotulo)
	}
}

// fundirArquivo reescreve um TSV gzipado de frases acrescentando as colunas tema/dificuldade, casando
// por texto chinês. Devolve o total de linhas e quantas receberam rótulo. Preserva a ordem original.
func fundirArquivo(caminho string, classificacoes map[string]classificacao) (total, comRotulo int) {
	linhas := lerLinhasFrases(caminho)

	fundidas := make([]string, 0, len(linhas))
	for _, linha := range linhas {
		campos := strings.Split(linha, "\t")
		if len(campos) < colunasBaseFrase || campos[0] == "" {
			continue // linha inválida no fonte — descarta, não propaga lixo
		}
		total++

		cl := classificacoes[campos[0]]
		if cl.tema != "" || cl.dificuldade != "" {
			comRotulo++
		}
		fundidas = append(fundidas, strings.Join([]string{campos[0], campos[1], campos[2], cl.tema, cl.dificuldade}, "\t"))
	}

	escreverFrasesGz(caminho, fundidas)
	return total, comRotulo
}

// ----- Seção: Leitura -----

// lerClassificacoes lê o classificacoes.tsv (chinês <TAB> tema <TAB> dificuldade) → mapa por chinês.
func lerClassificacoes(caminho string) map[string]classificacao {
	dados, err := os.ReadFile(caminho)
	if err != nil {
		abortar(fmt.Errorf("não foi possível ler %q (rode antes frases/classificar e/ou frases/espalhar): %w", caminho, err))
	}

	classificacoes := map[string]classificacao{}
	for _, linha := range strings.Split(string(dados), "\n") {
		campos := strings.Split(linha, "\t")
		if len(campos) != 3 || campos[0] == "" {
			continue
		}
		classificacoes[campos[0]] = classificacao{tema: campos[1], dificuldade: campos[2]}
	}
	return classificacoes
}

// lerLinhasFrases lê todas as linhas (não vazias) de um TSV gzipado de frases, na ordem do arquivo.
func lerLinhasFrases(caminho string) []string {
	dados, err := os.ReadFile(caminho)
	if err != nil {
		abortar(fmt.Errorf("não foi possível ler %q: %w", caminho, err))
	}
	gz, err := gzip.NewReader(bytes.NewReader(dados))
	if err != nil {
		abortar(fmt.Errorf("%q corrompido: %w", caminho, err))
	}
	defer gz.Close()

	var linhas []string
	varredor := bufio.NewScanner(gz)
	varredor.Buffer(make([]byte, 1024*1024), 1024*1024)
	for varredor.Scan() {
		linha := varredor.Text()
		if linha == "" {
			continue
		}
		linhas = append(linhas, linha)
	}
	abortar(varredor.Err())
	return linhas
}

// listarArquivosFrases devolve os .tsv.gz do diretório de frases do idioma, ordenados.
func listarArquivosFrases(dir string) []string {
	arquivos, err := filepath.Glob(filepath.Join(dir, "*"+sufixoFrases))
	if err != nil {
		abortar(err)
	}
	if len(arquivos) == 0 {
		abortar(fmt.Errorf("nenhum arquivo de frases (*%s) em %q", sufixoFrases, dir))
	}
	sort.Strings(arquivos)
	return arquivos
}

// ----- Seção: Escrita -----

// escreverFrasesGz regrava o arquivo de frases gzipado, na ordem dada. Cabeçalho gzip sem nome nem
// timestamp: o arquivo é versionado e precisa ser byte-a-byte reprodutível.
func escreverFrasesGz(caminho string, linhas []string) {
	arquivo, err := os.Create(caminho)
	abortar(err)
	defer arquivo.Close()

	gz, err := gzip.NewWriterLevel(arquivo, gzip.BestCompression)
	abortar(err)
	gz.Name = ""
	gz.ModTime = time.Time{}

	escritor := bufio.NewWriter(gz)
	for _, linha := range linhas {
		fmt.Fprintln(escritor, linha)
	}
	abortar(escritor.Flush())
	abortar(gz.Close())
}

// ----- Seção: Utilitários -----

func abortar(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, "erro:", err)
		os.Exit(1)
	}
}
