package main

// ----- Seção: Montagem do acervo final de frases pt-BR (corrige as erradas via Google Tradutor) -----
//
// Terceiro e último passo do pipeline de frases sobre o corpus nativo pt-BR do Tatoeba (o primeiro
// passo, frases/gerar_tatoeba, foi removido depois de produzir frases/pt-BR/frases_brutas.tsv.gz — os
// despejos do Tatoeba não serão mais atualizados). Lê o veredicto de cada par
// (frases/<idioma>/veredictos.tsv, produzido por frases/verificar) e monta o arquivo EMBARCADO
// idiomas/<idioma>/frases/frases_tatoeba.tsv.gz:
//
//   - Par CORRETO (对): fica como está — tradução humana do Tatoeba + atribuição original.
//   - Par ERRADO (错): a tradução em português é REFEITA pelo Google Tradutor direto do chinês
//     (tradução automática é confiável para frase inteira, mais do que a glosa humana equivocada). A
//     frase chinesa continua sendo a do Tatoeba (CC-BY 2.0 FR), então a atribuição mantém o crédito do
//     chinês e passa a marcar que a tradução pt-BR é do Google.
//
// Cache versionado: cada tradução do Google vai para frases/<idioma>/traducoes_google.tsv
// (chinês <TAB> tradução). Reexecuções reaproveitam o cache e não repetem chamadas — a montagem do
// arquivo final fica reprodutível sem depender de rede. O motor de tradução vive no pacote compartilhado
// frases/tradutorgoogle (chave via GOOGLE_TRANSLATE_API_KEY no .env ou no ambiente).
//
// Rodar da RAIZ do repo:
//
//	go run ./dicionario/frases/montar [idioma]

import (
	"bufio"
	"compress/gzip"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"wails_app/dicionario/frases/tradutorgoogle"
)

const (
	idiomaPadrao   = "pt-BR"
	moldeVeredicto = "dicionario/frases/%s/veredictos.tsv"
	moldeCache     = "dicionario/frases/%s/traducoes_google.tsv"
	moldeSaida     = "dicionario/idiomas/%s/frases/frases_tatoeba.tsv.gz"

	VEREDICTO_OK = "ok"

	PAUSA_ENTRE_CHAMADAS = 200 * time.Millisecond // gentileza com o endpoint entre traduções novas
)

// veredicto é uma linha do veredictos.tsv já parseada.
type veredicto struct {
	chines     string
	traducao   string
	atribuicao string
	correto    bool
}

func main() {
	idioma := idiomaPadrao
	if len(os.Args) > 1 {
		idioma = os.Args[1]
	}

	veredictos, err := lerVeredictos(fmt.Sprintf(moldeVeredicto, idioma))
	abortar(err)
	corretos, errados := contar(veredictos)
	fmt.Printf("Veredictos: %d corretos + %d errados = %d\n", corretos, errados, len(veredictos))

	caminhoCache := fmt.Sprintf(moldeCache, idioma)
	cache := tradutorgoogle.CarregarCache(caminhoCache)
	fmt.Printf("Cache de traduções do Google: %d entrada(s)\n", len(cache))

	pendentes := chinesesSemTraducao(veredictos, cache)
	if len(pendentes) > 0 {
		tradutor := tradutorgoogle.Novo()
		fmt.Printf("Traduzindo %d frase(s) errada(s) via %s\n", len(pendentes), tradutor.NomeMotor())
		for i, chines := range pendentes {
			pt, err := tradutor.Traduzir(chines)
			abortar(err)
			cache[chines] = pt
			fmt.Printf("  [%d/%d] %s ⇒ %s\n", i+1, len(pendentes), chines, pt)
			abortar(tradutorgoogle.SalvarCache(caminhoCache, cache)) // grava a cada passo: interrupção não perde o já traduzido
			time.Sleep(PAUSA_ENTRE_CHAMADAS)
		}
	} else {
		fmt.Println("Nenhuma tradução nova a fazer (cache cobre todos os errados).")
	}

	total := montarArquivoFinal(fmt.Sprintf(moldeSaida, idioma), veredictos, cache)
	fmt.Printf("\n===== RESUMO =====\n")
	fmt.Printf("Saída embarcada: %s (%d frases)\n", fmt.Sprintf(moldeSaida, idioma), total)
	fmt.Printf("Corretas mantidas do Tatoeba: %d · Erradas refeitas pelo Google: %d\n", corretos, errados)
}


// ----- Leitura de veredictos -----

func lerVeredictos(caminho string) ([]veredicto, error) {
	dados, err := os.ReadFile(caminho)
	if err != nil {
		return nil, fmt.Errorf("não foi possível ler os veredictos %q (rode 'verificar' antes): %w", caminho, err)
	}
	var veredictos []veredicto
	for _, linha := range strings.Split(string(dados), "\n") {
		if strings.TrimSpace(linha) == "" {
			continue
		}
		campos := strings.Split(linha, "\t")
		if len(campos) != 4 {
			return nil, fmt.Errorf("linha de veredicto malformada: %q", linha)
		}
		veredictos = append(veredictos, veredicto{
			chines:     campos[0],
			traducao:   campos[1],
			atribuicao: campos[2],
			correto:    campos[3] == VEREDICTO_OK,
		})
	}
	return veredictos, nil
}


func contar(veredictos []veredicto) (corretos, errados int) {
	for _, v := range veredictos {
		if v.correto {
			corretos++
		} else {
			errados++
		}
	}
	return corretos, errados
}


// chinesesSemTraducao devolve, na ordem dos veredictos, os chineses errados que ainda não estão no cache.
func chinesesSemTraducao(veredictos []veredicto, cache map[string]string) []string {
	var pendentes []string
	visto := map[string]bool{}
	for _, v := range veredictos {
		if v.correto || cache[v.chines] != "" || visto[v.chines] {
			continue
		}
		visto[v.chines] = true
		pendentes = append(pendentes, v.chines)
	}
	return pendentes
}


// ----- Montagem do arquivo final embarcado -----

// montarArquivoFinal escreve o TSV gzipado embarcado, na ordem dos veredictos. Cabeçalho gzip sem nome
// nem timestamp: o arquivo é versionado e precisa ser byte-a-byte reprodutível.
func montarArquivoFinal(caminho string, veredictos []veredicto, cache map[string]string) int {
	abortar(os.MkdirAll(filepath.Dir(caminho), 0o755))
	arquivo, err := os.Create(caminho)
	abortar(err)
	defer arquivo.Close()

	gz, err := gzip.NewWriterLevel(arquivo, gzip.BestCompression)
	abortar(err)
	gz.Name = ""
	gz.ModTime = time.Time{}
	escritor := bufio.NewWriter(gz)

	total := 0
	for _, v := range veredictos {
		traducao, atribuicao := v.traducao, v.atribuicao
		if !v.correto {
			pt := cache[v.chines]
			if pt == "" {
				abortar(fmt.Errorf("frase errada %q sem tradução no cache (bug)", v.chines))
			}
			traducao = pt
			atribuicao = atribuicaoCorrigida(v.atribuicao)
		}
		fmt.Fprintf(escritor, "%s\t%s\t%s\n", v.chines, traducao, atribuicao)
		total++
	}
	abortar(escritor.Flush())
	abortar(gz.Close())
	return total
}


// atribuicaoCorrigida mantém só o crédito da frase CHINESA (a que continua sendo do Tatoeba) e marca
// que a tradução pt-BR foi refeita pelo Google — o autor português original já não é usado.
func atribuicaoCorrigida(original string) string {
	return tradutorgoogle.CreditoChines(original) + " — tradução pt-BR refeita via Google Tradutor"
}


// ----- Utilitários -----

func abortar(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, "erro:", err)
		os.Exit(1)
	}
}
