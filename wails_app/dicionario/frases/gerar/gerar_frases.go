package main

// ----- Seção: Gerador de frases chinesas via DeepSeek + tradução/revisão multi-idioma -----
//
// Cria frases NOVAS em chinês com o DeepSeek e as integra ao acervo de cada idioma disponível. Fluxo:
//
//	1. GERAR    — pede N frases ao DeepSeek (tema e quantidade opcionais); volta o chinês SEM tradução,
//	              já com tema + dificuldade classificados pelo próprio modelo (taxonomia compartilhada).
//	2. DEDUP    — descarta as que o projeto já conhece, lendo o inventário frases_conhecidas.tsv (col. 0);
//	              se ele ainda não existir, roda frases/inventario para criá-lo antes.
//	3. TRADUZIR — para cada idioma, traduz as inéditas com o Google Tradutor (chinês → idioma-alvo).
//	4. REVISAR  — o DeepSeek dá um veredicto binário (对/错) por par chinês⇒tradução (mesmo padrão do
//	              frases/revisar), agora parametrizado pelo idioma-alvo.
//	5. CORRIGIR — as reprovadas são retraduzidas DIRETO do chinês pelo DeepSeek; as aprovadas ficam com
//	              a do Google. O Google é sempre a base garantida; o DeepSeek é a melhora quando dá.
//	6. GRAVAR   — anexa as novas a dicionario/idiomas/<idioma>/frases/frases_deepseek.tsv.gz (lê o que
//	              já existe, deduplica por chinês e reescreve ordenado — o loader do app pega .tsv.gz).
//	7. INVENTÁRIO — ao fim, roda frases/inventario para o frases_conhecidas.tsv refletir as frases recém-
//	              gravadas — é o que garante que a PRÓXIMA execução (passo 2) já as veja e não as repita.
//
// Temperatura: a GERAÇÃO usa temperatura alta (variedade entre execuções); revisão e correção usam 0
// (determinístico), igual ao resto do pipeline.
//
// Chaves de API: DeepSeek e Google, nos mesmos arquivos gitignored do resto do pipeline (ver deepseek/
// e tradutorgoogle/). Rodar da raiz do módulo wails_app:
//
//	go run ./dicionario/frases/gerar [quantidade] [tema]
//	  ex.: go run ./dicionario/frases/gerar 100
//	       go run ./dicionario/frases/gerar 50 饮食

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"wails_app/dicionario/frases/deepseek"
	"wails_app/dicionario/frases/idiomasalvo"
	"wails_app/dicionario/frases/taxonomia"
	"wails_app/dicionario/frases/tradutorgoogle"
)

const (
	moldeArquivoDeepSeek = "dicionario/idiomas/%s/frases/frases_deepseek.tsv.gz"

	// caminhoInventario é a fonte de dedup (as frases que o projeto já conhece, col. 0). pacoteInventario
	// é rodado como subprocesso para (re)gerá-lo: ao fim de cada execução e, sob demanda, quando o arquivo
	// ainda não existe. O caminho espelha o caminhoSaida de frases/inventario/inventario.go — devem casar.
	caminhoInventario = "dicionario/frases/inventario/frases_conhecidas.tsv"
	pacoteInventario  = "./dicionario/frases/inventario"

	QUANTIDADE_PADRAO   = 100
	TEMPERATURA_GERACAO = 1.3 // alta de propósito: sem variedade, toda execução repetiria e a dedup zeraria
	FRASES_POR_GERACAO  = 50  // frases pedidas por requisição de geração
	FATOR_TETO_GERACAO  = 4   // teto de lotes = ceil(quantidade/lote) × isto (evita loop se o modelo repetir)

	MINIMO_CARACTERES_FRASE = 2
	MAXIMO_CARACTERES_FRASE = 40 // frases curtas cabem melhor na revisão do app

	FRASES_POR_LOTE_GOOGLE = 100 // a API oficial aceita ~128 textos por requisição
	PARES_POR_VEREDICTO    = 100 // veredicto: saída minúscula (só 对/错), cabe lote grande
	FRASES_POR_CORRECAO    = 40  // correção: saída longa, lote menor para caber no teto de tokens

	MAXIMO_TOKENS_GERACAO   = 4096
	MAXIMO_TOKENS_VEREDICTO = 4096
	MAXIMO_TOKENS_CORRECAO  = 8192

	VEREDICTO_OK     = "ok"
	VEREDICTO_ERRADO = "errado"

	ATRIBUICAO_BASE          = "Frase em chinês gerada por DeepSeek (deepseek-chat)"
	SUFIXO_TRADUCAO_GOOGLE   = " — tradução %s via Google Tradutor"
	SUFIXO_TRADUCAO_DEEPSEEK = " — tradução %s via Google Tradutor, corrigida por DeepSeek"
)

// fraseGerada é uma frase criada pelo DeepSeek: chinês + classificação, antes de qualquer tradução.
type fraseGerada struct {
	chines      string
	tema        string
	dificuldade string
}

// linhaFrase é uma linha final do acervo de um idioma (as 5 colunas do banco embarcado).
type linhaFrase struct {
	chines      string
	traducao    string
	atribuicao  string
	tema        string
	dificuldade string
}


// ----- Prompts (enviados em chinês; %s de idioma recebe o nomeChines do idioma-alvo) -----

// INSTRUCOES_GERACAO, em português: "Você é um professor de chinês mandarim que cria frases-exemplo
// idiomáticas, naturais e úteis para estudantes da língua."
const INSTRUCOES_GERACAO = `你是一位中文（普通话）老师，为语言学习者创作地道、自然、实用的中文例句。`

// MODELO_PROMPT_GERACAO recebe (nesta ordem): quantidade (%d), restrição de tema (%s, pode ser vazia),
// lista de temas (%s) e lista de dificuldades (%s). Em português, pede N frases só em chinês simplificado,
// variadas, curtas, cada uma rotulada com tema (da taxonomia) e dificuldade, no formato "frase | tema |
// dificuldade" — sem numeração, sem explicação, sem tradução.
const MODELO_PROMPT_GERACAO = `请创作 %d 个适合中文学习者的中文（普通话）例句。%s

要求：
- 只用简体中文，不要拼音，不要其他语言，也不要给出翻译。
- 句子要地道、自然、口语化，长度适中（大约 4 到 20 个汉字）。
- 尽量多样化：不同的主题、场景、句型和难度，不要重复，也不要和常见课本例句雷同。

给每个句子标注两个标签：
1. 主题——从下面这个封闭列表里选恰好一个（原样复制中文标签）：
%s
2. 难度（针对中文学习者）——从易到难选恰好一个：
%s

每个句子回答一行，格式严格为：
句子 | 主题 | 难度
也就是：中文句子，一个竖线"|"，主题标签，一个竖线"|"，难度标签。不要写编号，不要写解释，不要写译文。

示例：
今天天气很好，我们去公园吧。 | 天气自然 | 入门`

// INSTRUCOES_VEREDICTO recebe o nomeChines do idioma-alvo (2× %s). Em português: "Você é um revisor
// bilíngue de chinês mandarim e <idioma>. Sua tarefa é julgar se a tradução em <idioma> transmite
// fielmente o sentido da frase em chinês."
const INSTRUCOES_VEREDICTO = `你是一位精通中文（普通话）和%s的双语审校员。你的任务是判断%s译文是否忠实传达了中文原句的意思。`

// MODELO_PROMPT_VEREDICTO recebe o nomeChines (2×) e as linhas "N. <chinês> ⇒ <tradução>" (%s final).
// Em português: julga par a par se a tradução transmite fielmente o chinês; responde "N对"/"N错" (só o
// número e o caractere 对/错), errado quando há erro de sentido, certo quando só muda estilo/ordem.
const MODELO_PROMPT_VEREDICTO = `下面每一行的格式是"N. 中文句子 ⇒ %s译文"。

请逐条判断%s译文是否忠实传达了中文原句的意思。

每条回答一行，格式为"N对"（译文正确）或"N错"（译文错误）——重复编号N，并且只使用汉字"对"或"错"。不要写任何其他内容：不要多余的空格、标点、解释，也不要写出译文本身。

出现语义错误时判为错误（错）：主语/人称搞错（我/你/他/我们）、否定颠倒、数量或时态错误、动词搞错、遗漏或增添内容改变了意思，或译文对应的是另一个句子。只有文体差异、同义词、标点或自然语序不同时判为正确（对）——忠实的翻译不需要逐字直译。

输入：
%s`

// INSTRUCOES_CORRECAO recebe o nomeChines do idioma-alvo (2× %s). Em português: "Você é um tradutor de
// chinês mandarim para <idioma>. Traduz sempre direto do chinês. A saída é SEMPRE em <idioma> — nunca
// chinês, simplificado ou tradicional."
const INSTRUCOES_CORRECAO = `你是一位把中文（普通话）翻译成%s的翻译员。始终直接从中文翻译。输出必须始终是%s——绝不能是未翻译的中文，无论简体还是繁体。`

// MODELO_PROMPT_CORRECAO recebe o nomeChines (2×) e as linhas "N. <chinês>" (%s final). Em português:
// traduz cada frase para <idioma>; nunca devolve o chinês sem traduzir; responde "N. tradução", uma por
// linha, sem mais nada.
const MODELO_PROMPT_CORRECAO = `请将下面每个中文句子翻译成%s。

请直接从中文翻译。译文必须是%s。绝不能原样返回未翻译的中文——无论完全相同，还是仅仅转换了简繁体。这不算翻译，该条会被丢弃。

每条回答一行，格式为"N. 译文"——重复编号N，加句点，然后是译文。不要写任何其他内容：不要写中文，不要解释，也不要加多余的引号。保留原句对话的标点。

输入：
%s`


func main() {
	quantidade, tema := lerArgumentos(os.Args[1:])

	cliente, err := deepseek.Novo()
	abortar(err)
	tradutor := tradutorgoogle.Novo()

	fmt.Printf("=== Gerador de frases via DeepSeek ===\n")
	fmt.Printf("Meta: %d frase(s) inédita(s)%s | tradutor: %s\n", quantidade, descricaoTema(tema), tradutor.NomeMotor())

	conhecidas := lerConhecidas()
	fmt.Printf("Frases já conhecidas no projeto (dedup): %d\n\n", len(conhecidas))

	geradas := gerarNovas(cliente, tema, quantidade, conhecidas)
	if len(geradas) == 0 {
		abortar(errors.New("nenhuma frase inédita gerada — tente de novo ou reduza a quantidade"))
	}
	fmt.Printf("\nFrases inéditas geradas: %d\n", len(geradas))

	for _, idioma := range idiomasalvo.Todos {
		processarIdioma(cliente, tradutor, idioma, geradas)
	}

	atualizarInventario()

	entrada, saida := cliente.Tokens()
	fmt.Printf("\n===== FIM =====\n")
	fmt.Printf("DeepSeek: %d tokens de entrada + %d de saída ≈ US$ %.4f\n", entrada, saida, cliente.CustoEstimadoUsd())
}


// ----- Fase 1: geração + deduplicação -----

// gerarNovas pede frases ao DeepSeek em lotes até juntar `quantidade` inéditas (fora de `conhecidas` e
// sem repetir entre si) ou esgotar o teto de lotes. Devolve as inéditas na ordem em que surgiram.
func gerarNovas(cliente *deepseek.Cliente, tema string, quantidade int, conhecidas map[string]bool) []fraseGerada {
	coletadas := map[string]bool{}
	var ordenadas []fraseGerada
	teto := (quantidade+FRASES_POR_GERACAO-1)/FRASES_POR_GERACAO*FATOR_TETO_GERACAO + 1

	for lote := 1; len(ordenadas) < quantidade && lote <= teto; lote++ {
		pedir := quantidade - len(ordenadas)
		if pedir > FRASES_POR_GERACAO {
			pedir = FRASES_POR_GERACAO
		}

		antes := len(ordenadas)
		for _, f := range gerarLote(cliente, tema, pedir) {
			if conhecidas[f.chines] || coletadas[f.chines] {
				continue
			}
			coletadas[f.chines] = true
			ordenadas = append(ordenadas, f)
		}
		fmt.Printf("[geração %d/%d] pedidas %d · inéditas novas: %d · total %d/%d\n",
			lote, teto, pedir, len(ordenadas)-antes, len(ordenadas), quantidade)
	}

	if len(ordenadas) > quantidade {
		ordenadas = ordenadas[:quantidade]
	}
	if len(ordenadas) < quantidade {
		fmt.Printf("⚠ meta não atingida: só %d de %d inéditas (o modelo passou a repetir demais)\n", len(ordenadas), quantidade)
	}
	return ordenadas
}


// gerarLote pede um lote ao DeepSeek e devolve as frases válidas extraídas. Erro de moderação pula o
// lote (fica sem essas frases); erro definitivo derruba a rodada (falha barulhenta).
func gerarLote(cliente *deepseek.Cliente, tema string, quantidade int) []fraseGerada {
	restricao := ""
	if tema != "" {
		restricao = fmt.Sprintf(" 请让所有句子都围绕这个主题：%s。", tema)
	}
	prompt := fmt.Sprintf(MODELO_PROMPT_GERACAO, quantidade, restricao,
		strings.Join(taxonomia.TEMAS, " "), strings.Join(taxonomia.DIFICULDADES, " "))

	resposta, err := cliente.ConversarComTemperatura(INSTRUCOES_GERACAO, prompt, MAXIMO_TOKENS_GERACAO, TEMPERATURA_GERACAO)
	if errors.Is(err, deepseek.ErrConteudoBloqueado) {
		fmt.Println("  ⚠ lote de geração recusado pela moderação — pulado")
		return nil
	}
	abortar(err)
	return extrairFrasesGeradas(resposta)
}


// padraoNumeroInicial remove uma numeração acidental no início da frase ("12.", "12、", "12)"...).
var padraoNumeroInicial = regexp.MustCompile(`^\s*\d{1,4}\s*[.、):：]\s*`)

// extrairFrasesGeradas varre a resposta ("frase | tema | dificuldade" por linha) e devolve as frases
// válidas. Descarta linhas fora do formato, frases sem chinês/curtas/longas ou com letra latina (pinyin/
// eco), e sem dificuldade válida; tema fora da taxonomia cai em 其他.
func extrairFrasesGeradas(resposta string) []fraseGerada {
	temasOrdenados := taxonomia.TemasPorTamanho()
	var frases []fraseGerada
	for _, linha := range strings.Split(resposta, "\n") {
		partes := strings.Split(strings.ReplaceAll(linha, "｜", "|"), "|")
		if len(partes) < 3 {
			continue
		}
		chines := strings.TrimSpace(padraoNumeroInicial.ReplaceAllString(partes[0], ""))
		if !fraseChinesaValida(chines) {
			continue
		}
		dificuldade := taxonomia.PrimeiraOcorrencia(partes[len(partes)-1], taxonomia.DIFICULDADES)
		if dificuldade == "" {
			continue
		}
		tema := taxonomia.PrimeiraOcorrencia(partes[len(partes)-2], temasOrdenados)
		if tema == "" {
			tema = taxonomia.TEMA_OUTROS
		}
		frases = append(frases, fraseGerada{chines: chines, tema: tema, dificuldade: dificuldade})
	}
	return frases
}


var padraoHan = regexp.MustCompile(`\p{Han}`)
var padraoLatina = regexp.MustCompile(`\p{Latin}`)

// fraseChinesaValida exige texto de tamanho razoável, com ao menos um caractere Han e SEM letra latina
// (rejeita pinyin, eco em latim e nomes estrangeiros — a saída fica puramente chinesa).
func fraseChinesaValida(texto string) bool {
	n := utf8.RuneCountInString(texto)
	if n < MINIMO_CARACTERES_FRASE || n > MAXIMO_CARACTERES_FRASE {
		return false
	}
	return padraoHan.MatchString(texto) && !padraoLatina.MatchString(texto)
}


// ----- Fase 2-6: por idioma (traduz, revisa, corrige, grava) -----

// processarIdioma traduz as frases geradas para um idioma, revisa/corrige as traduções e anexa o
// resultado ao frases_deepseek.tsv.gz do idioma.
func processarIdioma(cliente *deepseek.Cliente, tradutor *tradutorgoogle.Tradutor, idioma idiomasalvo.Idioma, geradas []fraseGerada) {
	fmt.Printf("\n===== Idioma %s =====\n", idioma.Dir)
	chineses := chinesesDe(geradas)

	traducoes := traduzirTudo(tradutor, idioma, chineses)
	fmt.Printf("Traduzidas pelo Google: %d\n", len(traducoes))

	errados := revisar(cliente, idioma, chineses, traducoes)
	corrigidas := corrigir(cliente, idioma, errados)
	fmt.Printf("Revisão do DeepSeek: %d reprovada(s), %d corrigida(s)\n", len(errados), len(corrigidas))

	linhas := montarLinhas(idioma, geradas, traducoes, corrigidas)
	caminho := fmt.Sprintf(moldeArquivoDeepSeek, idioma.Dir)
	adicionadas := gravarIncremental(caminho, linhas)
	fmt.Printf("Gravadas %d frase(s) nova(s) em %s\n", adicionadas, caminho)
}


// traduzirTudo traduz todo o chinês para o idioma-alvo com o Google, em lotes, devolvendo chinês→tradução.
func traduzirTudo(tradutor *tradutorgoogle.Tradutor, idioma idiomasalvo.Idioma, chineses []string) map[string]string {
	traducoes := map[string]string{}
	for _, lote := range fatiar(chineses, FRASES_POR_LOTE_GOOGLE) {
		saida, err := tradutor.TraduzirLoteParaIdioma(lote, idioma.CodigoGoogle)
		abortar(err)
		for i, ch := range lote {
			traducoes[ch] = saida[i]
		}
	}
	return traducoes
}


// revisar pede ao DeepSeek um veredicto binário por par chinês⇒tradução e devolve o conjunto de chineses
// REPROVADOS. Par sem veredicto válido é tratado como aprovado (não se corrige o que não foi julgado).
func revisar(cliente *deepseek.Cliente, idioma idiomasalvo.Idioma, chineses []string, traducoes map[string]string) map[string]bool {
	errados := map[string]bool{}
	instrucoes := fmt.Sprintf(INSTRUCOES_VEREDICTO, idioma.NomeChines, idioma.NomeChines)

	for _, lote := range fatiar(chineses, PARES_POR_VEREDICTO) {
		var entradas strings.Builder
		for i, ch := range lote {
			fmt.Fprintf(&entradas, "%d. %s ⇒ %s\n", i+1, ch, traducoes[ch])
		}
		prompt := fmt.Sprintf(MODELO_PROMPT_VEREDICTO, idioma.NomeChines, idioma.NomeChines, entradas.String())

		resposta, err := cliente.Conversar(instrucoes, prompt, MAXIMO_TOKENS_VEREDICTO)
		if errors.Is(err, deepseek.ErrConteudoBloqueado) {
			fmt.Println("  ⚠ lote de revisão recusado pela moderação — mantém tradução do Google")
			continue
		}
		abortar(err)

		veredictos := extrairVeredictos(resposta)
		for i, ch := range lote {
			if veredictos[i+1] == VEREDICTO_ERRADO {
				errados[ch] = true
			}
		}
	}
	return errados
}


// corrigir retraduz, direto do chinês, as reprovadas, e devolve chinês→tradução das que voltaram
// válidas. Moderação/eco pula a frase (fica com a tradução do Google, que já é a base garantida).
func corrigir(cliente *deepseek.Cliente, idioma idiomasalvo.Idioma, errados map[string]bool) map[string]string {
	if len(errados) == 0 {
		return map[string]string{}
	}
	instrucoes := fmt.Sprintf(INSTRUCOES_CORRECAO, idioma.NomeChines, idioma.NomeChines)
	corrigidas := map[string]string{}

	for _, lote := range fatiar(chavesOrdenadas(errados), FRASES_POR_CORRECAO) {
		var entradas strings.Builder
		for i, ch := range lote {
			fmt.Fprintf(&entradas, "%d. %s\n", i+1, ch)
		}
		prompt := fmt.Sprintf(MODELO_PROMPT_CORRECAO, idioma.NomeChines, idioma.NomeChines, entradas.String())

		resposta, err := cliente.Conversar(instrucoes, prompt, MAXIMO_TOKENS_CORRECAO)
		if errors.Is(err, deepseek.ErrConteudoBloqueado) {
			fmt.Println("  ⚠ lote de correção recusado pela moderação — mantém tradução do Google")
			continue
		}
		abortar(err)

		traducoes := extrairTraducoes(resposta)
		for i, ch := range lote {
			if t, ok := traducoes[i+1]; ok {
				corrigidas[ch] = t
			}
		}
	}
	return corrigidas
}


// montarLinhas junta, por frase gerada, a melhor tradução (correção do DeepSeek se houver, senão a do
// Google) e monta a linha final do acervo com a atribuição correspondente.
func montarLinhas(idioma idiomasalvo.Idioma, geradas []fraseGerada, traducoes, corrigidas map[string]string) []linhaFrase {
	var linhas []linhaFrase
	for _, f := range geradas {
		traducao := traducoes[f.chines]
		atribuicao := ATRIBUICAO_BASE + fmt.Sprintf(SUFIXO_TRADUCAO_GOOGLE, idioma.Dir)
		if corrigida, ok := corrigidas[f.chines]; ok {
			traducao = corrigida
			atribuicao = ATRIBUICAO_BASE + fmt.Sprintf(SUFIXO_TRADUCAO_DEEPSEEK, idioma.Dir)
		}
		if strings.TrimSpace(traducao) == "" {
			fmt.Printf("  ⚠ sem tradução para %q — frase pulada neste idioma\n", f.chines)
			continue
		}
		linhas = append(linhas, linhaFrase{
			chines:      f.chines,
			traducao:    traducao,
			atribuicao:  atribuicao,
			tema:        f.tema,
			dificuldade: f.dificuldade,
		})
	}
	return linhas
}


// ----- Leitura do que já existe (dedup) e escrita incremental -----

// lerConhecidas devolve o conjunto de todo texto chinês que o projeto já conhece, lido da coluna 0 do
// inventário (frases_conhecidas.tsv). Se o inventário ainda não existe, roda frases/inventario para
// criá-lo. O passo 7 o reatualiza ao fim de cada execução, então ele já cobre os frases_deepseek das
// rodadas anteriores — é o que garante que uma frase nunca se repita entre execuções.
func lerConhecidas() map[string]bool {
	if _, err := os.Stat(caminhoInventario); os.IsNotExist(err) {
		fmt.Printf("Inventário ausente — gerando %s pela primeira vez.\n", caminhoInventario)
		atualizarInventario()
	}

	dados, err := os.ReadFile(caminhoInventario)
	abortar(err) // sem inventário não há como deduplicar — falha barulhenta
	conhecidas := map[string]bool{}
	for _, linha := range strings.Split(string(dados), "\n") {
		campos := strings.SplitN(linha, "\t", 2)
		if campos[0] == "" {
			continue
		}
		conhecidas[campos[0]] = true
	}
	return conhecidas
}


// gravarIncremental lê o frases_deepseek.tsv.gz existente do idioma (se houver), acrescenta as linhas
// novas (dedup por chinês), reescreve ordenado e gzipado. Devolve quantas foram efetivamente novas.
func gravarIncremental(caminho string, novas []linhaFrase) int {
	existentes := lerAcervoExistente(caminho)
	adicionadas := 0
	for _, l := range novas {
		if _, ok := existentes[l.chines]; ok {
			continue
		}
		existentes[l.chines] = l
		adicionadas++
	}
	abortar(escreverAcervo(caminho, existentes))
	return adicionadas
}


// lerAcervoExistente lê um frases_deepseek.tsv.gz num mapa chinês→linha. Arquivo ausente devolve mapa
// vazio (primeira execução do idioma) — não é erro.
func lerAcervoExistente(caminho string) map[string]linhaFrase {
	acervo := map[string]linhaFrase{}
	dados, err := os.ReadFile(caminho)
	if err != nil {
		return acervo
	}
	gz, err := gzip.NewReader(bytes.NewReader(dados))
	abortar(err)
	defer gz.Close()

	varredor := bufio.NewScanner(gz)
	varredor.Buffer(make([]byte, 1024*1024), 1024*1024)
	for varredor.Scan() {
		campos := strings.Split(varredor.Text(), "\t")
		if len(campos) < 5 || campos[0] == "" {
			continue
		}
		acervo[campos[0]] = linhaFrase{
			chines:      campos[0],
			traducao:    campos[1],
			atribuicao:  campos[2],
			tema:        campos[3],
			dificuldade: campos[4],
		}
	}
	return acervo
}


// escreverAcervo grava o acervo inteiro ordenado por chinês, gzipado, no formato de 5 colunas do banco
// embarcado (chinês, tradução, atribuição, tema, dificuldade) — determinístico e estável entre rodadas.
func escreverAcervo(caminho string, acervo map[string]linhaFrase) error {
	if err := os.MkdirAll(filepath.Dir(caminho), 0o755); err != nil {
		return err
	}

	chineses := make([]string, 0, len(acervo))
	for ch := range acervo {
		chineses = append(chineses, ch)
	}
	sort.Strings(chineses)

	var buffer bytes.Buffer
	gz := gzip.NewWriter(&buffer)
	for _, ch := range chineses {
		l := acervo[ch]
		if _, err := fmt.Fprintf(gz, "%s\t%s\t%s\t%s\t%s\n", l.chines, l.traducao, l.atribuicao, l.tema, l.dificuldade); err != nil {
			return err
		}
	}
	if err := gz.Close(); err != nil {
		return err
	}
	return os.WriteFile(caminho, buffer.Bytes(), 0o644)
}


// atualizarInventario roda o extrator frases/inventario como subprocesso, para o frases_conhecidas.tsv
// refletir as frases recém-gravadas. NÃO é fatal: as frases já estão no acervo, e o dedup da próxima
// execução lê os bancos embarcados direto (não o inventário) — uma falha aqui só deixa o inventário
// desatualizado, então avisa e segue. O subprocesso herda o cwd (raiz do módulo wails_app), onde o
// caminho do pacote resolve, igual à invocação do próprio gerador.
func atualizarInventario() {
	fmt.Printf("\nRodando o inventário (%s)...\n", pacoteInventario)
	comando := exec.Command("go", "run", pacoteInventario)
	comando.Stdout = os.Stdout
	comando.Stderr = os.Stderr
	if err := comando.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "⚠ inventário não atualizado: %v — rode manualmente: go run %s\n", err, pacoteInventario)
	}
}


// ----- Parsing das respostas do DeepSeek (veredicto + correção) -----

// padraoVeredicto casa "N对"/"N. 错" etc.: o número da entrada seguido de 对/對 (correta) ou 错/錯 (errada).
var padraoVeredicto = regexp.MustCompile(`(\d{1,4})\s*[.\s:)]*\s*(对|對|错|錯)`)

// extrairVeredictos varre a resposta e devolve numero→ok|errado. Ignora prosa fora do padrão.
func extrairVeredictos(resposta string) map[int]string {
	veredictos := map[int]string{}
	for _, linha := range strings.Split(resposta, "\n") {
		m := padraoVeredicto.FindStringSubmatch(linha)
		if m == nil {
			continue
		}
		numero, err := strconv.Atoi(m[1])
		if err != nil {
			continue
		}
		if m[2] == "对" || m[2] == "對" {
			veredictos[numero] = VEREDICTO_OK
			continue
		}
		veredictos[numero] = VEREDICTO_ERRADO
	}
	return veredictos
}


// padraoTraducao casa "N. tradução" (número, separador e o texto).
var padraoTraducao = regexp.MustCompile(`^\s*(\d{1,4})\s*[.:)\]]\s*(.+?)\s*$`)

// extrairTraducoes varre a resposta e devolve numero→tradução. Rejeita "tradução" sem nenhuma letra
// latina (chinês ecoado sem traduzir): pt-BR e inglês são escrita latina, então isso é lixo e a entrada
// vira faltante (fica com a tradução do Google).
func extrairTraducoes(resposta string) map[int]string {
	traducoes := map[int]string{}
	for _, linha := range strings.Split(resposta, "\n") {
		m := padraoTraducao.FindStringSubmatch(linha)
		if m == nil {
			continue
		}
		numero, err := strconv.Atoi(m[1])
		if err != nil {
			continue
		}
		texto := strings.TrimSpace(m[2])
		if texto == "" || !padraoLatina.MatchString(texto) {
			continue
		}
		traducoes[numero] = texto
	}
	return traducoes
}


// ----- Utilitários -----

// lerArgumentos lê [quantidade] [tema] em qualquer ordem: o primeiro argumento numérico é a quantidade,
// qualquer outro é o tema. Quantidade ausente/inválida cai no padrão.
func lerArgumentos(args []string) (quantidade int, tema string) {
	quantidade = QUANTIDADE_PADRAO
	achouQuantidade := false
	for _, arg := range args {
		if n, err := strconv.Atoi(arg); err == nil && !achouQuantidade {
			quantidade = n
			achouQuantidade = true
			continue
		}
		tema = arg
	}
	if quantidade < 1 {
		quantidade = QUANTIDADE_PADRAO
	}
	return quantidade, tema
}


func chinesesDe(geradas []fraseGerada) []string {
	chineses := make([]string, len(geradas))
	for i, f := range geradas {
		chineses[i] = f.chines
	}
	return chineses
}


func chavesOrdenadas(conjunto map[string]bool) []string {
	chaves := make([]string, 0, len(conjunto))
	for ch := range conjunto {
		chaves = append(chaves, ch)
	}
	sort.Strings(chaves)
	return chaves
}


func fatiar(itens []string, tamanho int) [][]string {
	var fatias [][]string
	for inicio := 0; inicio < len(itens); inicio += tamanho {
		fim := inicio + tamanho
		if fim > len(itens) {
			fim = len(itens)
		}
		fatias = append(fatias, itens[inicio:fim])
	}
	return fatias
}


func descricaoTema(tema string) string {
	if tema == "" {
		return ""
	}
	return fmt.Sprintf(" sobre \"%s\"", tema)
}


func abortar(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, "erro:", err)
		os.Exit(1)
	}
}
