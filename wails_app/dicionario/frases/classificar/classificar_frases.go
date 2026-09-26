package main

// ----- Seção: Classificação das frases (tema + dificuldade) via API do DeepSeek -----
//
// Enriquece o acervo de frases com dois rótulos por frase: o TEMA (do que ela trata) e a DIFICULDADE
// para quem estuda a língua. É um passo IRMÃO do de verificação (frases/verificar), com o mesmo
// esqueleto de HTTP/lotes/paralelismo/retomada, mas outra pergunta ao modelo.
//
// Para poupar tokens de SAÍDA, o modelo responde SÓ em chinês simplificado e SÓ com rótulos de listas
// fechadas — nada de justificativa nem tradução:
//   - TEMA: um rótulo da taxonomia fixa (ver TEMAS) — classificação fechada é filtrável; texto livre não.
//   - DIFICULDADE: 入门 / 初级 / 中级 / 高级  (= iniciante / fácil / médio / avançado).
// Uma linha por frase, no formato "N 主题 难度". O custo mora na ENTRADA (as frases), não na resposta.
//
// Entrada:  idiomas/<idioma>/frases/*.tsv.gz   (todas as frases embarcadas do idioma; col. 0 = chinês)
// Saída:    frases/<idioma>/classificacoes.tsv  (chinês <TAB> tema <TAB> dificuldade)
//
// Deduplica por texto chinês (uma classificação por frase, mesmo que apareça em vários arquivos). A
// saída é ordenada por chinês, para ser estável e determinística entre rodadas.
//
// Retomável e à prova de queda: cada lote concluído é ANEXADO à saída na hora (crash não perde o que
// já foi pago); uma classificacoes.tsv já existente é relida e as frases já classificadas são puladas
// (chave = texto chinês). Ao fim, o arquivo é reescrito ordenado. Passe "-refazer" para começar do zero.
//
// O objetivo desta classificação é ser PROPAGADA de graça a outros idiomas cujo chinês seja idêntico
// (ver frases/espalhar) — classifica-se o corpus grande (inglês) uma vez e espalha-se sem repagar.
//
// Rodar da RAIZ do repo (mesma pegadinha de módulo do resto do pipeline):
//
//	go run ./dicionario/frases/classificar [idioma] [-refazer]
//
// Chave da API: variável DEEPSEEK_API_KEY ou o mesmo arquivo gitignored do tradutor do CEDICT.

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"wails_app/dicionario/frases/taxonomia"
)

const (
	URL_API_DEEPSEEK   = "https://api.deepseek.com/chat/completions"
	MODELO_DEEPSEEK    = "deepseek-chat"
	VARIAVEL_CHAVE_API = "DEEPSEEK_API_KEY"

	idiomaPadrao       = "en"
	moldeFrasesIdioma  = "dicionario/idiomas/%s/frases"
	moldeClassificacao = "dicionario/frases/%s/classificacoes.tsv"

	FRASES_POR_REQUISICAO          = 100
	MAXIMO_REQUISICOES_SIMULTANEAS = 6
	MAXIMO_TENTATIVAS_HTTP         = 4
	MAXIMO_PASSADAS_POR_LOTE       = 3
	MAXIMO_TOKENS_RESPOSTA         = 4096
	TEMPERATURA                    = 0.0 // classificação determinística, não criação de texto
	TIMEOUT_REQUISICAO             = 4 * time.Minute

	PRECO_ENTRADA_POR_MILHAO = 0.28
	PRECO_SAIDA_POR_MILHAO   = 0.42
)

// A taxonomia (TEMAS, DIFICULDADES, TEMA_OUTROS) e seus validadores moram em frases/taxonomia — fonte
// única compartilhada com frases/gerar. Ver o pacote para os rótulos e as glosas em português.

// errConteudoBloqueado sinaliza um 400 "Content Exists Risk" da moderação do DeepSeek: alguma frase do
// lote foi recusada. É tratado isolando e pulando a(s) ofensora(s), não abortando a rodada (ver isolarBloqueadas).
var errConteudoBloqueado = errors.New("conteúdo recusado pela moderação do DeepSeek")

const INSTRUCOES_SISTEMA = `Você é um professor de chinês mandarim. Classifica frases em chinês por tema e por dificuldade para estudantes da língua. Responde apenas com os rótulos pedidos, em chinês, sem nenhum texto a mais.`

// MODELO_PROMPT recebe, via %s (nesta ordem): lista de temas, lista de dificuldades e as linhas
// numeradas "N. <chinês>".
const MODELO_PROMPT = `Cada linha abaixo tem o formato "N. FRASE_EM_CHINÊS".

Para CADA frase, atribua DOIS rótulos:

1. TEMA — escolha exatamente UM desta lista (copie o rótulo em chinês, sem alterar). Use 其他 só quando nenhum outro servir:
%s

2. DIFICULDADE para quem estuda chinês — escolha exatamente UM, do mais fácil ao mais difícil:
%s

Responda UMA linha por frase, no formato EXATO:
N 主题 难度
Ou seja: o número N, um espaço, o rótulo do tema, um espaço, o rótulo da dificuldade. NADA MAIS — sem pontuação, sem tradução, sem explicação, sem parênteses.

Exemplo de resposta para 3 frases:
1 社交问候 入门
2 饮食 初级
3 工作职业 中级

FRASES:
%s`

// classificacao é o par de rótulos atribuído a uma frase.
type classificacao struct {
	tema        string
	dificuldade string
}

// classificador carrega o cliente HTTP, a chave, os contadores de token e o estado da saída
// (mapa em memória + arquivo de anexação para gravar cada lote na hora).
type classificador struct {
	chaveApi string
	cliente  *http.Client

	mu               sync.Mutex
	resultados       map[string]classificacao // chinês -> rótulos (pré-carregado com o que já existe)
	arquivo          *os.File                 // aberto em modo append; nil quando não há nada pendente
	bloqueadas       []string                 // frases recusadas pela moderação (puladas, ficam sem rótulo)
	naoClassificadas []string                 // frases que o modelo teimou em não classificar em formato válido
	tokensEntrada    int64
	tokensSaida      int64

	lotesFeitos int
	lotesTotal  int
}

func main() {
	idioma, refazer := lerArgumentos(os.Args[1:])

	chineses, err := lerChinesesUnicos(fmt.Sprintf(moldeFrasesIdioma, idioma))
	if err != nil {
		abortar(err)
	}
	fmt.Printf("Frases (chineses únicos) em %s: %d\n", idioma, len(chineses))

	caminho := fmt.Sprintf(moldeClassificacao, idioma)
	existentes := map[string]classificacao{}
	if !refazer {
		existentes = lerClassificacoesExistentes(caminho)
		if len(existentes) > 0 {
			fmt.Printf("Retomando: %d classificação(ões) já registrada(s) serão reaproveitadas\n", len(existentes))
		}
	}

	pendentes := semClassificacao(chineses, existentes)

	c := &classificador{
		chaveApi:   "", // preenchida abaixo só se houver o que classificar
		cliente:    &http.Client{Timeout: TIMEOUT_REQUISICAO},
		resultados: existentes,
	}

	if len(pendentes) == 0 {
		fmt.Println("Nada a classificar: todas as frases já têm rótulos.")
	} else {
		fmt.Printf("Classificando %d frase(s) com %s (%d por requisição, %d em paralelo)\n\n",
			len(pendentes), MODELO_DEEPSEEK, FRASES_POR_REQUISICAO, MAXIMO_REQUISICOES_SIMULTANEAS)

		chave, err := carregarChaveApi()
		abortar(err)
		c.chaveApi = chave

		abortar(c.abrirSaidaParaAnexar(caminho, refazer))
		abortar(c.classificar(pendentes))
		c.arquivo.Close()

		fmt.Printf("\nTokens: %d de entrada + %d de saída ≈ US$ %.4f\n", c.tokensEntrada, c.tokensSaida, c.custoEstimadoUsd())
		if len(c.bloqueadas) > 0 {
			fmt.Printf("Frases puladas pela moderação do DeepSeek (sem rótulo): %d\n", len(c.bloqueadas))
		}
		if len(c.naoClassificadas) > 0 {
			fmt.Printf("Frases puladas por resposta inválida do modelo (sem rótulo): %d\n", len(c.naoClassificadas))
		}
	}

	// Reescreve a saída ordenada por chinês (determinística), a partir de tudo que há em memória
	// (existentes + novos). Também normaliza um arquivo que veio só de anexações fora de ordem.
	gravarOrdenado(caminho, chineses, c.resultados)
	imprimirResumo(caminho, chineses, c.resultados)
}

// ----- Leitura das frases embarcadas -----

// lerChinesesUnicos lê a coluna 0 (chinês) de TODOS os .tsv.gz do diretório de frases do idioma e
// devolve os textos únicos, ordenados. Lê do sistema de arquivos (não do embed): é ferramenta de build.
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
		if err := lerChinesesArquivo(caminho, vistos); err != nil {
			return nil, err
		}
	}

	chineses := make([]string, 0, len(vistos))
	for ch := range vistos {
		chineses = append(chineses, ch)
	}
	sort.Strings(chineses)
	return chineses, nil
}

func lerChinesesArquivo(caminho string, vistos map[string]bool) error {
	dados, err := os.ReadFile(caminho)
	if err != nil {
		return fmt.Errorf("não foi possível ler %q: %w", caminho, err)
	}
	gz, err := gzip.NewReader(bytes.NewReader(dados))
	if err != nil {
		return fmt.Errorf("%q corrompido: %w", caminho, err)
	}
	defer gz.Close()

	varredor := bufio.NewScanner(gz)
	varredor.Buffer(make([]byte, 1024*1024), 1024*1024)
	for varredor.Scan() {
		campos := strings.SplitN(varredor.Text(), "\t", 2)
		if len(campos) < 1 || campos[0] == "" {
			continue
		}
		vistos[campos[0]] = true
	}
	return varredor.Err()
}

// ----- Retomada / pendências -----

func semClassificacao(chineses []string, existentes map[string]classificacao) []string {
	var pendentes []string
	for _, ch := range chineses {
		if _, ok := existentes[ch]; !ok {
			pendentes = append(pendentes, ch)
		}
	}
	return pendentes
}

// lerClassificacoesExistentes relê um classificacoes.tsv anterior para retomar sem repagar. Só aceita
// linhas com rótulos VÁLIDOS (nas listas fechadas); lixo é ignorado e será reclassificado.
func lerClassificacoesExistentes(caminho string) map[string]classificacao {
	dados, err := os.ReadFile(caminho)
	if err != nil {
		return map[string]classificacao{}
	}
	classificacoes := map[string]classificacao{}
	for _, linha := range strings.Split(string(dados), "\n") {
		campos := strings.Split(linha, "\t")
		if len(campos) != 3 || campos[0] == "" {
			continue
		}
		if taxonomia.TemaValido(campos[1]) && taxonomia.DificuldadeValida(campos[2]) {
			classificacoes[campos[0]] = classificacao{tema: campos[1], dificuldade: campos[2]}
		}
	}
	return classificacoes
}

// ----- Classificação em lote -----

// abrirSaidaParaAnexar prepara o arquivo de saída para receber cada lote na hora. Com -refazer,
// trunca (recomeço do zero); senão, anexa ao que já existe (as linhas antigas já estão em resultados).
func (c *classificador) abrirSaidaParaAnexar(caminho string, refazer bool) error {
	if err := os.MkdirAll(filepath.Dir(caminho), 0o755); err != nil {
		return err
	}
	flags := os.O_CREATE | os.O_WRONLY | os.O_APPEND
	if refazer {
		flags |= os.O_TRUNC
	}
	arq, err := os.OpenFile(caminho, flags, 0o644)
	if err != nil {
		return err
	}
	c.arquivo = arq
	return nil
}

// classificar fatia as pendências, dispara as requisições em paralelo e persiste cada lote concluído.
func (c *classificador) classificar(pendentes []string) error {
	fatias := fatiar(pendentes, FRASES_POR_REQUISICAO)
	c.lotesTotal = len(fatias)

	var wg sync.WaitGroup
	vagas := make(chan struct{}, MAXIMO_REQUISICOES_SIMULTANEAS)
	erros := make([]error, len(fatias))

	for f := range fatias {
		wg.Add(1)
		vagas <- struct{}{}
		go func(indice int) {
			defer wg.Done()
			defer func() { <-vagas }()

			parcial, err := c.classificarFatia(fatias[indice], 1)
			if err != nil {
				erros[indice] = err
				return
			}
			c.registrarLote(fatias[indice], parcial)
		}(f)
	}
	wg.Wait()

	for _, err := range erros {
		if err != nil {
			return err
		}
	}
	return nil
}

// classificarFatia envia um conjunto de frases e devolve chinês→classificacao. Frases sem rótulo
// válido na resposta são reenviadas, até MAXIMO_PASSADAS_POR_LOTE — persistindo, derruba a rodada
// (nunca se grava um rótulo adivinhado).
func (c *classificador) classificarFatia(frases []string, passada int) (map[string]classificacao, error) {
	var lista strings.Builder
	for i, ch := range frases {
		fmt.Fprintf(&lista, "%d. %s\n", i+1, ch)
	}
	prompt := fmt.Sprintf(MODELO_PROMPT, strings.Join(taxonomia.TEMAS, " "), strings.Join(taxonomia.DIFICULDADES, " "), lista.String())

	resposta, err := c.chamarDeepSeek(prompt)
	if errors.Is(err, errConteudoBloqueado) {
		return c.isolarBloqueadas(frases)
	}
	if err != nil {
		return nil, err
	}

	brutos := extrairClassificacoes(resposta)
	resultado := map[string]classificacao{}
	var faltantes []string
	for i, ch := range frases {
		cl, ok := brutos[i+1]
		if !ok {
			faltantes = append(faltantes, ch)
			continue
		}
		resultado[ch] = cl
	}

	if len(faltantes) == 0 {
		return resultado, nil
	}
	if passada >= MAXIMO_PASSADAS_POR_LOTE {
		// Esgotadas as passadas, o resto é frase que o modelo teima em não classificar (resposta
		// fora de formato mesmo isolada). Pula e reporta — não se adivinha rótulo, nem se derruba
		// a rodada inteira por um punhado de frases.
		for _, ch := range faltantes {
			c.registrarNaoClassificada(ch)
		}
		return resultado, nil
	}

	reenvio, err := c.classificarFatia(faltantes, passada+1)
	if err != nil {
		return nil, err
	}
	for ch, cl := range reenvio {
		resultado[ch] = cl
	}
	return resultado, nil
}

// isolarBloqueadas trata um lote recusado pela moderação do DeepSeek ("Content Exists Risk"): divide o
// lote ao meio recursivamente até isolar a(s) frase(s) que, sozinha(s), disparam a recusa — e as PULA
// (ficam sem classificação, reportadas no fim). As demais do lote são classificadas normalmente. Custa
// ~log2(N) requisições extras só no lote afetado; os outros lotes seguem intactos.
func (c *classificador) isolarBloqueadas(frases []string) (map[string]classificacao, error) {
	if len(frases) == 1 {
		c.registrarBloqueada(frases[0])
		return map[string]classificacao{}, nil
	}
	meio := len(frases) / 2
	resultado, err := c.classificarFatia(frases[:meio], 1)
	if err != nil {
		return nil, err
	}
	segunda, err := c.classificarFatia(frases[meio:], 1)
	if err != nil {
		return nil, err
	}
	for ch, cl := range segunda {
		resultado[ch] = cl
	}
	return resultado, nil
}

// registrarBloqueada anota uma frase pulada pela moderação (sob trava) e avisa no console.
func (c *classificador) registrarBloqueada(chines string) {
	c.mu.Lock()
	c.bloqueadas = append(c.bloqueadas, chines)
	c.mu.Unlock()
	fmt.Printf("  ⚠ frase pulada (recusada pela moderação do DeepSeek): %s\n", chines)
}

// registrarNaoClassificada anota uma frase que o modelo não classificou em formato válido nem isolada.
func (c *classificador) registrarNaoClassificada(chines string) {
	c.mu.Lock()
	c.naoClassificadas = append(c.naoClassificadas, chines)
	c.mu.Unlock()
	fmt.Printf("  ⚠ frase pulada (sem classificação válida do modelo): %s\n", chines)
}

// registrarLote grava as linhas de um lote na saída (append) e as guarda em memória, sob trava.
// Persistir por lote deixa a rodada à prova de queda: um Ctrl-C não joga fora o que já foi pago.
func (c *classificador) registrarLote(frases []string, parcial map[string]classificacao) {
	c.mu.Lock()
	defer c.mu.Unlock()

	var linhas strings.Builder
	gravadas := 0
	for _, ch := range frases {
		cl, ok := parcial[ch]
		if !ok {
			continue // frase pulada (bloqueada/sem resposta válida) — NÃO entra no arquivo, fica sem rótulo
		}
		fmt.Fprintf(&linhas, "%s\t%s\t%s\n", ch, cl.tema, cl.dificuldade)
		c.resultados[ch] = cl
		gravadas++
	}
	if _, err := c.arquivo.WriteString(linhas.String()); err != nil {
		abortar(fmt.Errorf("falha ao gravar lote: %w", err))
	}

	c.lotesFeitos++
	fmt.Printf("[%d/%d] lote classificado (%d frases)\n", c.lotesFeitos, c.lotesTotal, gravadas)
}

// padraoNumero pega o número da entrada no início da linha ("12", "12.", "12)"...).
var padraoNumero = regexp.MustCompile(`^\s*(\d{1,6})`)

// extrairClassificacoes varre a resposta e devolve numero→classificacao. Uma linha só conta se tiver
// número, um tema válido E uma dificuldade válida; qualquer prosa fora disso é ignorada.
func extrairClassificacoes(resposta string) map[int]classificacao {
	temasOrdenados := taxonomia.TemasPorTamanho()
	classificacoes := map[int]classificacao{}
	for _, linha := range strings.Split(resposta, "\n") {
		m := padraoNumero.FindStringSubmatch(linha)
		if m == nil {
			continue
		}
		numero, err := strconv.Atoi(m[1])
		if err != nil {
			continue
		}
		dificuldade := taxonomia.PrimeiraOcorrencia(linha, taxonomia.DIFICULDADES)
		if dificuldade == "" {
			continue // sem dificuldade válida não há como salvar a linha; vira faltante e é reenviada
		}
		tema := taxonomia.PrimeiraOcorrencia(linha, temasOrdenados)
		if tema == "" {
			tema = taxonomia.TEMA_OUTROS // modelo escolheu um tema fora da taxonomia → válvula de escape
		}
		classificacoes[numero] = classificacao{tema: tema, dificuldade: dificuldade}
	}
	return classificacoes
}

// ----- Saída ordenada + resumo -----

// gravarOrdenado reescreve a saída inteira ordenada por chinês (a mesma ordem de `chineses`), a partir
// do mapa em memória. Só entram frases que têm classificação — determinístico e estável entre rodadas.
func gravarOrdenado(caminho string, chineses []string, resultados map[string]classificacao) {
	var saida strings.Builder
	for _, ch := range chineses {
		cl, ok := resultados[ch]
		if !ok {
			continue
		}
		fmt.Fprintf(&saida, "%s\t%s\t%s\n", ch, cl.tema, cl.dificuldade)
	}
	abortar(os.WriteFile(caminho, []byte(saida.String()), 0o644))
}

// imprimirResumo mostra a cobertura e a distribuição por dificuldade e por tema.
func imprimirResumo(caminho string, chineses []string, resultados map[string]classificacao) {
	classificadas := 0
	porDificuldade := map[string]int{}
	porTema := map[string]int{}
	for _, ch := range chineses {
		cl, ok := resultados[ch]
		if !ok {
			continue
		}
		classificadas++
		porDificuldade[cl.dificuldade]++
		porTema[cl.tema]++
	}

	fmt.Printf("\n===== RESUMO =====\n")
	fmt.Printf("Saída: %s\n", caminho)
	fmt.Printf("Classificadas: %d de %d frases\n", classificadas, len(chineses))

	fmt.Printf("\nPor dificuldade:\n")
	for _, d := range taxonomia.DIFICULDADES {
		fmt.Printf("  %s: %d\n", d, porDificuldade[d])
	}

	fmt.Printf("\nPor tema:\n")
	for _, t := range taxonomia.TEMAS {
		if porTema[t] > 0 {
			fmt.Printf("  %s: %d\n", t, porTema[t])
		}
	}
}

// ----- HTTP / DeepSeek (mesmo esqueleto do verificar) -----

func (c *classificador) chamarDeepSeek(prompt string) (string, error) {
	corpo, err := json.Marshal(map[string]any{
		"model": MODELO_DEEPSEEK,
		"messages": []map[string]string{
			{"role": "system", "content": INSTRUCOES_SISTEMA},
			{"role": "user", "content": prompt},
		},
		"temperature": TEMPERATURA,
		"max_tokens":  MAXIMO_TOKENS_RESPOSTA,
		"stream":      false,
	})
	if err != nil {
		return "", err
	}

	var ultimoErro error
	for tentativa := 1; tentativa <= MAXIMO_TENTATIVAS_HTTP; tentativa++ {
		if tentativa > 1 {
			time.Sleep(time.Duration(5<<(tentativa-2)) * time.Second)
		}

		requisicao, err := http.NewRequest(http.MethodPost, URL_API_DEEPSEEK, bytes.NewReader(corpo))
		if err != nil {
			return "", err
		}
		requisicao.Header.Set("Content-Type", "application/json")
		requisicao.Header.Set("Authorization", "Bearer "+c.chaveApi)

		resposta, err := c.cliente.Do(requisicao)
		if err != nil {
			ultimoErro = err
			continue
		}
		corpoResposta, err := io.ReadAll(resposta.Body)
		resposta.Body.Close()
		if err != nil {
			ultimoErro = err
			continue
		}

		if resposta.StatusCode == http.StatusTooManyRequests || resposta.StatusCode >= 500 {
			ultimoErro = fmt.Errorf("API devolveu %s", resposta.Status)
			continue
		}
		// Moderação: 400 "Content Exists Risk" — não adianta repetir; sinaliza para isolar a frase ofensora.
		if resposta.StatusCode == http.StatusBadRequest && bytes.Contains(corpoResposta, []byte("Content Exists Risk")) {
			return "", errConteudoBloqueado
		}
		if resposta.StatusCode != http.StatusOK {
			return "", fmt.Errorf("API devolveu %s: %s", resposta.Status, resumoCorpo(corpoResposta))
		}

		var dados struct {
			Choices []struct {
				Message struct {
					Content string `json:"content"`
				} `json:"message"`
			} `json:"choices"`
			Usage struct {
				PromptTokens     int64 `json:"prompt_tokens"`
				CompletionTokens int64 `json:"completion_tokens"`
			} `json:"usage"`
		}
		if err := json.Unmarshal(corpoResposta, &dados); err != nil || len(dados.Choices) == 0 {
			ultimoErro = fmt.Errorf("resposta inesperada da API: %s", resumoCorpo(corpoResposta))
			continue
		}

		c.mu.Lock()
		c.tokensEntrada += dados.Usage.PromptTokens
		c.tokensSaida += dados.Usage.CompletionTokens
		c.mu.Unlock()
		return dados.Choices[0].Message.Content, nil
	}
	return "", fmt.Errorf("desisti após %d tentativas: %w", MAXIMO_TENTATIVAS_HTTP, ultimoErro)
}

// ----- Utilitários -----

func lerArgumentos(args []string) (idioma string, refazer bool) {
	idioma = idiomaPadrao
	for _, arg := range args {
		if arg == "-refazer" || arg == "--refazer" {
			refazer = true
			continue
		}
		idioma = arg
	}
	return idioma, refazer
}

func carregarChaveApi() (string, error) {
	if chave := obterVariavelAmbiente(VARIAVEL_CHAVE_API); chave != "" {
		return chave, nil
	}
	return "", fmt.Errorf("chave da API não encontrada: defina %s no ambiente ou no arquivo .env na raiz do repositório", VARIAVEL_CHAVE_API)
}


func obterVariavelAmbiente(nomeVar string) string {
	if valor := strings.TrimSpace(os.Getenv(nomeVar)); valor != "" {
		return valor
	}

	caminhos := []string{
		".env",
		"../.env",
		"../../.env",
		"../../../.env",
		"../../../../.env",
		"../../../../../.env",
	}

	for _, caminho := range caminhos {
		conteudo, err := os.ReadFile(caminho)
		if err != nil {
			continue
		}
		for _, linha := range strings.Split(string(conteudo), "\n") {
			linha = strings.TrimSpace(linha)
			if linha == "" || strings.HasPrefix(linha, "#") {
				continue
			}
			partes := strings.SplitN(linha, "=", 2)
			if len(partes) != 2 {
				continue
			}
			chave := strings.TrimSpace(partes[0])
			valor := strings.Trim(strings.TrimSpace(partes[1]), "\"'")
			if chave == nomeVar && valor != "" {
				return valor
			}
		}
	}
	return ""
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

func resumoCorpo(corpo []byte) string {
	texto := strings.TrimSpace(string(corpo))
	if len(texto) > 300 {
		texto = texto[:300] + "…"
	}
	return texto
}

func (c *classificador) custoEstimadoUsd() float64 {
	c.mu.Lock()
	defer c.mu.Unlock()
	return float64(c.tokensEntrada)/1e6*PRECO_ENTRADA_POR_MILHAO + float64(c.tokensSaida)/1e6*PRECO_SAIDA_POR_MILHAO
}

func abortar(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, "erro:", err)
		os.Exit(1)
	}
}
