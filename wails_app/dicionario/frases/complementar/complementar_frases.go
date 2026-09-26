package main

// ----- Seção: Complementa o acervo de um idioma com as frases do inventário (traduzidas pelo Google) -----
//
// Passo de enriquecimento que traduz PARA um idioma-alvo todas as frases chinesas que o projeto conhece e
// que o alvo ainda não tem. A FONTE das frases é o inventário centralizado frases_conhecidas.tsv (a
// fonte-da-verdade das frases conhecidas, gerada por frases/inventario a partir de TODOS os bancos de
// todos os idiomas) — não os bancos do inglês direto. Assim a tradução é COMPLETA: pega inclusive as
// frases nativas de outro idioma (ex.: pt-BR do Tatoeba) que não existem no inglês. Se o inventário ainda
// não existir, ele é gerado antes de começar. O tema/dificuldade vêm prontos do inventário (dependem só
// do chinês); o crédito da frase chinesa (Tatoeba) é mantido e a atribuição passa a marcar o motor da
// tradução do idioma-alvo.
//
//   Fonte (lê tudo):   dicionario/frases/inventario/frases_conhecidas.tsv (chinês, atribuição, tema, dificuldade)
//   Alvo (checa quais chineses já existem, exceto a própria saída):
//                      idiomas/<alvo>/frases/*.tsv.gz  (col. 0 = chinês)
//   Cache versionado:  dicionario/frases/<alvo>/traducoes_google_complementares.tsv    (chinês <TAB> tradução)
//   Reparos (opcional):dicionario/frases/<alvo>/traducoes_deepseek_complementares.tsv  (chinês <TAB> tradução)
//   Saída embarcada:   idiomas/<alvo>/frases/frases_complementares.tsv.gz
//                      (chinês, tradução, atribuição, tema, dificuldade)
//
// QUALIDADE: o Google não traduz zh→idioma-alvo direto, ele PIVOTA pelo inglês, e o pivô erra frases
// elípticas ("是的，他喜歡" perde o verbo real). Por isso existe o passo frases/revisar, que julga cada
// tradução no DeepSeek e refaz as reprovadas traduzindo direto do chinês. Este script é o ESCRITOR ÚNICO
// do arquivo embarcado e prefere o reparo do DeepSeek quando ele existe, caindo no Google no resto — a
// atribuição de cada linha diz qual motor a produziu. Fluxo completo:
//
//	complementar (Google traduz tudo) → revisar (julga e refaz as erradas) → complementar (remonta)
//
// A 2ª rodada não gasta rede: o cache do Google já cobre tudo.
//
// A tradução é o passo caro (dezenas de milhares de frases): o cache é gravado a cada lote, então a
// execução é RESUMÍVEL — rerodar continua de onde parou e não repaga o que já traduziu. Com chave de API
// (GOOGLE_TRANSLATE_API_KEY no .env ou no ambiente) o motor traduz em LOTE, o único jeito
// viável para esse volume; sem chave, o endpoint livre traduz um a um e serve só para semear pouca coisa.
//
// ORDEM NO PIPELINE: rode DEPOIS de montar+classificar+espalhar+fundir. O arquivo de saída já carrega
// tema/dificuldade prontos; se fundir for reexecutado depois deste passo, ele reescreve os embarcados só
// com 3 colunas e apagaria a classificação daqui — então, ao reprocessar, rode fundir ANTES de
// complementar (ou complementar por último). Rodar da RAIZ do repo:
//
//	go run ./dicionario/frases/complementar [idioma]   # idioma-alvo (pt-BR, es…); padrão pt-BR
//
// Serve também para ONBOARDING de um idioma novo, que ainda não tem frase nenhuma: o alvo vazio faz todas
// as frases do inventário contarem como faltantes, e o corpus inteiro é traduzido de uma vez.

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"wails_app/dicionario/frases/idiomasalvo"
	"wails_app/dicionario/frases/tradutorgoogle"
)

const (
	idiomaReferencia = "en"    // idioma de referência/fallback; não é alvo de tradução deste passo
	idiomaAlvoPadrao = "pt-BR"

	moldeFrasesIdioma  = "dicionario/idiomas/%s/frases"
	arquivoSaida       = "frases_complementares.tsv.gz"
	moldeCache         = "dicionario/frases/%s/traducoes_google_complementares.tsv"
	moldeCacheDeepSeek = "dicionario/frases/%s/traducoes_deepseek_complementares.tsv"

	// Inventário: fonte-da-verdade das frases chinesas conhecidas (chinês, atribuição, tema, dificuldade),
	// com a atribuição já priorizando o inglês. É de onde saem as frases a traduzir. Se não existir, é
	// gerado pelo subprocesso antes de começar. Caminho e pacote espelham frases/inventario e frases/gerar.
	caminhoInventario = "dicionario/frases/inventario/frases_conhecidas.tsv"
	pacoteInventario  = "./dicionario/frases/inventario"

	colInvChines      = 0
	colInvAtribuicao  = 1
	colInvTema        = 2
	colInvDificuldade = 3
	colInvMinimas     = 2 // chinês + atribuição; tema/dificuldade podem vir vazios

	// Sufixos da atribuição, com %s = pasta do idioma-alvo (pt-BR, es…). Marcam qual motor produziu a
	// tradução — o crédito da frase CHINESA (Tatoeba) é preservado à parte por tradutorgoogle.CreditoChines.
	MOLDE_SUFIXO_ATRIBUICAO_GOOGLE   = " — tradução %s via Google Tradutor"
	MOLDE_SUFIXO_ATRIBUICAO_DEEPSEEK = " — tradução %s via DeepSeek"

	LOTE_COM_CHAVE    = 100 // a API oficial aceita ~128 textos por requisição; 100 dá folga
	PAUSA_ENTRE_LOTES = 200 * time.Millisecond

	VARIAVEL_LIMITE = "COMPLEMENTAR_LIMITE" // teto de traduções NOVAS por execução (0 = sem teto); útil
	//                                         para espalhar o volume entre sessões/cotas — o cache retoma
)

// fraseExclusiva é uma frase chinesa do inventário candidata à tradução, com os metadados já prontos.
type fraseExclusiva struct {
	chines      string
	atribuicao  string
	tema        string
	dificuldade string
}

func main() {
	alvo := lerIdiomaAlvo(os.Args[1:])
	fmt.Printf("Idioma-alvo: %s (Google: %s)\n", alvo.Dir, alvo.CodigoGoogle)

	conhecidas := lerFrasesDoInventario()
	fmt.Printf("Frases no inventário (fonte): %d\n", len(conhecidas))

	jaNoAlvo, err := lerChinesesAlvo(fmt.Sprintf(moldeFrasesIdioma, alvo.Dir), arquivoSaida)
	abortar(err)
	fmt.Printf("Chineses já no alvo (%s): %d\n", alvo.Dir, len(jaNoAlvo))

	exclusivas := selecionarExclusivas(conhecidas, jaNoAlvo)
	fmt.Printf("A traduzir (sem par no %s): %d\n", alvo.Dir, len(exclusivas))

	cache, puladas := traduzirFaltantes(fmt.Sprintf(moldeCache, alvo.Dir), exclusivas, tradutorgoogle.Novo(), alvo.CodigoGoogle)

	reparos := tradutorgoogle.CarregarCache(fmt.Sprintf(moldeCacheDeepSeek, alvo.Dir))

	caminhoSaida := filepath.Join(fmt.Sprintf(moldeFrasesIdioma, alvo.Dir), arquivoSaida)
	total, pendentes, refeitas := montarArquivoFinal(caminhoSaida, exclusivas, cache, reparos, alvo.Dir)

	fmt.Printf("\n===== RESUMO =====\n")
	fmt.Printf("Saída embarcada: %s (%d frases)\n", caminhoSaida, total)
	fmt.Printf("Exclusivas: %d · embarcadas: %d · ainda sem tradução: %d · puladas (falha persistente): %d\n",
		len(exclusivas), total, pendentes, len(puladas))
	fmt.Printf("Motor: %d do Google · %d refeitas pelo DeepSeek (passo frases/revisar)\n", total-refeitas, refeitas)
	if pendentes > 0 {
		fmt.Printf("Rode de novo para traduzir e embarcar as %d restantes (o cache retoma de onde parou).\n", pendentes)
	}
}


// lerIdiomaAlvo resolve o idioma-alvo do 1º argumento (padrão idiomaAlvoPadrao), buscando no registro
// compartilhado. Aborta se o código for desconhecido ou se for o idioma de referência (o inglês já é a
// base de tudo — não é alvo de tradução deste passo).
func lerIdiomaAlvo(args []string) idiomasalvo.Idioma {
	dir := idiomaAlvoPadrao
	if len(args) > 0 && strings.TrimSpace(args[0]) != "" {
		dir = strings.TrimSpace(args[0])
	}
	if dir == idiomaReferencia {
		abortar(fmt.Errorf("idioma-alvo %q é o idioma de referência — não se traduz para ele aqui; escolha outro (pt-BR, es…)", dir))
	}
	alvo, ok := idiomasalvo.PorDir(dir)
	if !ok {
		abortar(fmt.Errorf("idioma-alvo %q desconhecido — registre-o em frases/idiomasalvo antes", dir))
	}
	return alvo
}


// ----- Fonte das frases: o inventário centralizado -----

// lerFrasesDoInventario lê o inventário (frases_conhecidas.tsv) e devolve todas as frases conhecidas como
// candidatas à tradução, na ordem do arquivo (já ordenado e único por chinês). Se o inventário ainda não
// existe, roda frases/inventario para criá-lo antes — assim o onboarding de um idioma novo não depende de
// um passo manual.
func lerFrasesDoInventario() []fraseExclusiva {
	if _, err := os.Stat(caminhoInventario); os.IsNotExist(err) {
		fmt.Printf("Inventário ausente — gerando %s antes de traduzir.\n", caminhoInventario)
		gerarInventario()
	}

	dados, err := os.ReadFile(caminhoInventario)
	abortar(err)

	var frases []fraseExclusiva
	for _, linha := range strings.Split(string(dados), "\n") {
		campos := strings.Split(linha, "\t")
		if len(campos) < colInvMinimas || campos[colInvChines] == "" {
			continue
		}
		frases = append(frases, fraseExclusiva{
			chines:      campos[colInvChines],
			atribuicao:  campos[colInvAtribuicao],
			tema:        campoOpcional(campos, colInvTema),
			dificuldade: campoOpcional(campos, colInvDificuldade),
		})
	}
	return frases
}


// selecionarExclusivas descarta as frases cujo chinês já está no alvo, preservando a ordem do inventário
// (que já vem ordenado e único por chinês) — a saída fica determinística sem reordenar.
func selecionarExclusivas(conhecidas []fraseExclusiva, jaNoAlvo map[string]bool) []fraseExclusiva {
	var exclusivas []fraseExclusiva
	for _, f := range conhecidas {
		if jaNoAlvo[f.chines] {
			continue
		}
		exclusivas = append(exclusivas, f)
	}
	return exclusivas
}


// gerarInventario roda o extrator frases/inventario como subprocesso (herda o cwd = raiz do módulo, onde
// o caminho do pacote resolve). Sem inventário não há o que traduzir — falha barulhenta.
func gerarInventario() {
	comando := exec.Command("go", "run", pacoteInventario)
	comando.Stdout = os.Stdout
	comando.Stderr = os.Stderr
	abortar(comando.Run())
}


// lerChinesesAlvo devolve o conjunto de chineses já presentes no alvo, ignorando o próprio arquivo de
// saída — assim reexecutar não considera o que já embarcamos e o resultado continua determinístico. Um
// alvo vazio ou inexistente (idioma novo, sem frase nenhuma) devolve conjunto vazio, não erro: todas as
// frases do inglês passam a ser exclusivas.
func lerChinesesAlvo(dirAlvo, arquivoIgnorado string) (map[string]bool, error) {
	arquivos, err := filepath.Glob(filepath.Join(dirAlvo, "*.tsv.gz"))
	if err != nil {
		return nil, err
	}

	chineses := map[string]bool{}
	for _, caminho := range arquivos {
		if filepath.Base(caminho) == arquivoIgnorado {
			continue
		}
		linhas, err := lerLinhasGz(caminho)
		if err != nil {
			return nil, err
		}
		for _, linha := range linhas {
			campos := strings.SplitN(linha, "\t", 2)
			if len(campos) < 1 || campos[0] == "" {
				continue
			}
			chineses[campos[0]] = true
		}
	}
	return chineses, nil
}


// ----- Tradução (resumível, gravando o cache a cada lote) -----

// traduzirFaltantes garante uma tradução no cache para cada exclusiva ainda não traduzida. Devolve o
// cache atualizado e os chineses que falharam mesmo isolados (pulados nesta rodada, retomáveis depois).
func traduzirFaltantes(caminhoCache string, exclusivas []fraseExclusiva, tradutor *tradutorgoogle.Tradutor, codigoGoogle string) (map[string]string, []string) {
	cache := tradutorgoogle.CarregarCache(caminhoCache)

	var faltantes []string
	for _, f := range exclusivas {
		if cache[f.chines] == "" {
			faltantes = append(faltantes, f.chines) // exclusivas já vem única por chinês
		}
	}
	if len(faltantes) == 0 {
		fmt.Println("Cache cobre todas as exclusivas; nada a traduzir.")
		return cache, nil
	}
	if limite := lerLimite(); limite > 0 && len(faltantes) > limite {
		fmt.Printf("Teto de %d tradução(ões) nesta execução (faltam %d no total).\n", limite, len(faltantes))
		faltantes = faltantes[:limite]
	}

	tamanhoLote := 1
	if tradutor.TemChave() {
		tamanhoLote = LOTE_COM_CHAVE
	}
	fmt.Printf("Traduzindo %d frase(s) via %s (lote=%d)\n", len(faltantes), tradutor.NomeMotor(), tamanhoLote)

	var puladas []string
	for inicio := 0; inicio < len(faltantes); inicio += tamanhoLote {
		fim := inicio + tamanhoLote
		if fim > len(faltantes) {
			fim = len(faltantes)
		}
		lote := faltantes[inicio:fim]

		puladasDoLote := traduzirLoteNoCache(tradutor, lote, cache, codigoGoogle)
		puladas = append(puladas, puladasDoLote...)
		abortar(tradutorgoogle.SalvarCache(caminhoCache, cache)) // crash-safe: cada lote fica persistido

		fmt.Printf("  [%d/%d] processadas (%d puladas até aqui)\n", fim, len(faltantes), len(puladas))
		time.Sleep(PAUSA_ENTRE_LOTES)
	}
	return cache, puladas
}


// traduzirLoteNoCache traduz o lote para o idioma-alvo (código do Google) e grava no cache. Se o lote
// inteiro falhar (após os retries do tradutor), isola uma a uma para não deixar uma única frase
// problemática travar o corpus todo; as que insistirem em falhar são devolvidas como puladas.
func traduzirLoteNoCache(tradutor *tradutorgoogle.Tradutor, lote []string, cache map[string]string, codigoGoogle string) []string {
	traducoes, err := tradutor.TraduzirLoteParaIdioma(lote, codigoGoogle)
	if err == nil {
		for i, chines := range lote {
			cache[chines] = traducoes[i]
		}
		return nil
	}

	var puladas []string
	for _, chines := range lote {
		traducao, errUm := tradutor.TraduzirLoteParaIdioma([]string{chines}, codigoGoogle)
		if errUm != nil {
			puladas = append(puladas, chines)
			continue
		}
		cache[chines] = traducao[0]
	}
	return puladas
}


// ----- Montagem do arquivo final embarcado -----

// montarArquivoFinal escreve o TSV gzipado embarcado (chinês, pt-BR, atribuição, tema, dificuldade), na
// ordem já ordenada por chinês. Cabeçalho gzip sem nome nem timestamp: o arquivo é versionado e precisa
// ser byte-a-byte reprodutível. A tradução do DeepSeek (reparo do passo frases/revisar) tem PRIORIDADE
// sobre a do Google, e a atribuição diz qual motor produziu cada linha. Devolve quantas foram escritas,
// quantas ficaram sem tradução e quantas vieram do reparo.
func montarArquivoFinal(caminho string, exclusivas []fraseExclusiva, cache, reparos map[string]string, dirAlvo string) (total, pendentes, refeitas int) {
	abortar(os.MkdirAll(filepath.Dir(caminho), 0o755))
	arquivo, err := os.Create(caminho)
	abortar(err)
	defer arquivo.Close()

	sufixoGoogle := fmt.Sprintf(MOLDE_SUFIXO_ATRIBUICAO_GOOGLE, dirAlvo)
	sufixoDeepSeek := fmt.Sprintf(MOLDE_SUFIXO_ATRIBUICAO_DEEPSEEK, dirAlvo)

	gz, err := gzip.NewWriterLevel(arquivo, gzip.BestCompression)
	abortar(err)
	gz.Name = ""
	gz.ModTime = time.Time{}
	escritor := bufio.NewWriter(gz)

	for _, f := range exclusivas {
		pt, sufixo := cache[f.chines], sufixoGoogle
		if reparo := reparos[f.chines]; reparo != "" {
			pt, sufixo = reparo, sufixoDeepSeek
			refeitas++
		}
		if pt == "" {
			pendentes++ // ainda não traduzida — fica de fora desta rodada, entra quando o cache cobrir
			continue
		}
		atribuicao := tradutorgoogle.CreditoChines(f.atribuicao) + sufixo
		fmt.Fprintf(escritor, "%s\t%s\t%s\t%s\t%s\n", f.chines, pt, atribuicao, f.tema, f.dificuldade)
		total++
	}
	abortar(escritor.Flush())
	abortar(gz.Close())
	return total, pendentes, refeitas
}


// ----- Utilitários -----

func lerLinhasGz(caminho string) ([]string, error) {
	dados, err := os.ReadFile(caminho)
	if err != nil {
		return nil, fmt.Errorf("não foi possível ler %q: %w", caminho, err)
	}
	gz, err := gzip.NewReader(bytes.NewReader(dados))
	if err != nil {
		return nil, fmt.Errorf("%q corrompido: %w", caminho, err)
	}
	defer gz.Close()

	var linhas []string
	varredor := bufio.NewScanner(gz)
	varredor.Buffer(make([]byte, 1024*1024), 1024*1024)
	for varredor.Scan() {
		linhas = append(linhas, varredor.Text())
	}
	if err := varredor.Err(); err != nil {
		return nil, err
	}
	return linhas, nil
}


// lerLimite lê o teto de traduções novas por execução (VARIAVEL_LIMITE). Valor ausente ou inválido = 0
// (sem teto), o comportamento padrão.
func lerLimite() int {
	bruto := strings.TrimSpace(os.Getenv(VARIAVEL_LIMITE))
	if bruto == "" {
		return 0
	}
	limite, err := strconv.Atoi(bruto)
	if err != nil || limite < 0 {
		return 0
	}
	return limite
}


func campoOpcional(campos []string, indice int) string {
	if len(campos) > indice {
		return campos[indice]
	}
	return ""
}


func abortar(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, "erro:", err)
		os.Exit(1)
	}
}
