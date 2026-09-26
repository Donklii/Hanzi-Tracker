package main

// ----- Seção: Verificação das frases pt-BR do Tatoeba via API do DeepSeek -----
//
// Segundo passo do pipeline de frases sobre o corpus nativo pt-BR do Tatoeba (o primeiro passo,
// frases/gerar_tatoeba, casava os pares crus a partir dos despejos do Tatoeba; foi removido depois de
// produzir frases/pt-BR/frases_brutas.tsv.gz — os despejos não serão mais atualizados). As frases do
// Tatoeba são colaborativas e NÃO revisadas: acontece de a tradução em português trair o sentido do chinês
// (pessoa trocada, negação invertida, tradução de outra frase). Este comando pede ao DeepSeek um
// veredicto binário por par — e SÓ isso.
//
// Para poupar tokens de SAÍDA, o modelo responde apenas em chinês: 对 (correta) ou 错 (errada), um
// por linha. Nada de justificativa nem tradução — a correção das erradas é outro passo (montar, via
// Google Tradutor, mais confiável para frase inteira). A entrada carrega o par inteiro; o custo mora
// nela, não na resposta.
//
// Entrada:  frases/<idioma>/frases_brutas.tsv.gz  (chinês <TAB> português <TAB> atribuição)
// Saída:    frases/<idioma>/veredictos.tsv         (chinês <TAB> português <TAB> atribuição <TAB> ok|errado)
//
// O acervo em runtime guarda só a PRIMEIRA frase por texto chinês (GerenciadorFrases deduplica por
// chinês). Então aqui as brutas são COLAPSADAS a um par por chinês — o de menor id de tradução (o
// mais antigo) — antes de ir ao modelo: não se paga para julgar frases que o app nunca mostraria.
//
// Retomável: um veredictos.tsv já existente é relido e os pares já julgados são pulados (chave =
// chinês+português). Assim uma rodada interrompida continua de onde parou sem repagar. Passe "-refazer"
// para reavaliar tudo do zero.
//
// Rodar da RAIZ do repo (mesma pegadinha de módulo do resto do pipeline):
//
//	go run ./dicionario/frases/verificar [idioma] [-refazer]
//
// Chave da API: variável DEEPSEEK_API_KEY ou o mesmo arquivo gitignored do tradutor do CEDICT.

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	URL_API_DEEPSEEK   = "https://api.deepseek.com/chat/completions"
	MODELO_DEEPSEEK    = "deepseek-chat"
	VARIAVEL_CHAVE_API = "DEEPSEEK_API_KEY"

	idiomaPadrao   = "pt-BR"
	moldeBrutas    = "dicionario/frases/%s/frases_brutas.tsv.gz"
	moldeVeredicto = "dicionario/frases/%s/veredictos.tsv"

	VEREDICTO_OK     = "ok"
	VEREDICTO_ERRADO = "errado"

	PARES_POR_REQUISICAO           = 100
	MAXIMO_REQUISICOES_SIMULTANEAS = 6
	MAXIMO_TENTATIVAS_HTTP         = 4
	MAXIMO_PASSADAS_POR_PAR        = 3
	MAXIMO_TOKENS_RESPOSTA         = 4096
	TEMPERATURA                    = 0.0 // julgamento determinístico, não criação de texto
	TIMEOUT_REQUISICAO             = 4 * time.Minute

	PRECO_ENTRADA_POR_MILHAO = 0.28
	PRECO_SAIDA_POR_MILHAO   = 0.42
)

const INSTRUCOES_SISTEMA = `Você é um revisor bilíngue de chinês mandarim e português do Brasil. Sua tarefa é julgar se a tradução em português transmite fielmente o sentido da frase em chinês.`

// MODELO_PROMPT recebe (via %s) as linhas numeradas "N. <chinês> ⇒ <português>".
const MODELO_PROMPT = `Cada linha abaixo tem o formato "N. FRASE_EM_CHINÊS ⇒ tradução em português".

Julgue, entrada por entrada, se a tradução em português transmite FIELMENTE o sentido da frase em chinês.

Responda UMA linha por entrada, no formato "N对" quando a tradução estiver correta ou "N错" quando estiver errada — repita o número N e use APENAS o caractere chinês 对 (correta) ou 错 (errada). Não escreva mais nada: nem espaços extras, nem pontuação, nem explicação, nem a tradução.

Considere ERRADA (错) quando houver erro de SENTIDO: pessoa/sujeito trocado (eu/você/ele/nós), negação invertida, número ou tempo verbal errado, omissão ou acréscimo que mude o sentido, ou tradução que corresponde a outra frase. Considere CORRETA (对) quando só houver diferença de estilo, sinônimos, pontuação ou ordem natural do português — tradução fiel não precisa ser literal.

ENTRADAS:
%s`

// par é um par chinês→português já colapsado (um por chinês), com os dados para reconstruir a linha.
type par struct {
	chines     string
	traducao   string
	atribuicao string
	idCmn      int // para ordenar a saída de forma estável (espelha a ordem das brutas)
	idTraducao int // menor id vence o colapso por chinês
}

// verificador carrega o cliente HTTP, a chave e os contadores de tokens.
type verificador struct {
	chaveApi      string
	cliente       *http.Client
	mu            sync.Mutex
	tokensEntrada int64
	tokensSaida   int64
}

func main() {
	idioma, refazer := lerArgumentos(os.Args[1:])

	pares, err := lerBrutasColapsadas(fmt.Sprintf(moldeBrutas, idioma))
	if err != nil {
		abortar(err)
	}
	fmt.Printf("Frases brutas colapsadas a um par por chinês: %d\n", len(pares))

	caminhoVeredicto := fmt.Sprintf(moldeVeredicto, idioma)
	veredictos := map[string]string{}
	if !refazer {
		veredictos = lerVeredictosExistentes(caminhoVeredicto)
		if len(veredictos) > 0 {
			fmt.Printf("Retomando: %d veredicto(s) já registrado(s) serão reaproveitados\n", len(veredictos))
		}
	}

	pendentes := paresSemVeredicto(pares, veredictos)
	if len(pendentes) == 0 {
		fmt.Println("Nada a julgar: todos os pares já têm veredicto.")
	} else {
		fmt.Printf("Julgando %d par(es) com %s (%d por requisição, %d em paralelo)\n\n",
			len(pendentes), MODELO_DEEPSEEK, PARES_POR_REQUISICAO, MAXIMO_REQUISICOES_SIMULTANEAS)

		chave, err := carregarChaveApi()
		abortar(err)
		v := &verificador{chaveApi: chave, cliente: &http.Client{Timeout: TIMEOUT_REQUISICAO}}

		novos, err := v.julgarPares(pendentes)
		abortar(err)
		for chave, veredicto := range novos {
			veredictos[chave] = veredicto
		}
		fmt.Printf("\nTokens: %d de entrada + %d de saída ≈ US$ %.4f\n", v.tokensEntrada, v.tokensSaida, v.custoEstimadoUsd())
	}

	okCount, erradoCount := gravarVeredictos(caminhoVeredicto, pares, veredictos)
	fmt.Printf("\n===== RESUMO =====\n")
	fmt.Printf("Saída: %s\n", caminhoVeredicto)
	fmt.Printf("Corretas (对): %d · Erradas (错): %d · Total: %d\n", okCount, erradoCount, okCount+erradoCount)
}

// ----- Leitura das brutas e colapso por chinês -----

var padraoAtribuicao = regexp.MustCompile(`#(\d+) \([^)]*\) & #(\d+) \(`)

// lerBrutasColapsadas lê o TSV bruto e devolve UM par por texto chinês: o de menor id de tradução
// (o mais antigo). Ordena por id da frase chinesa para a saída ser estável entre rodadas.
func lerBrutasColapsadas(caminho string) ([]par, error) {
	dados, err := os.ReadFile(caminho)
	if err != nil {
		return nil, fmt.Errorf("não foi possível ler as brutas %q: %w", caminho, err)
	}
	gz, err := gzip.NewReader(bytes.NewReader(dados))
	if err != nil {
		return nil, fmt.Errorf("brutas %q corrompidas: %w", caminho, err)
	}
	defer gz.Close()

	porChines := map[string]par{}
	varredor := bufio.NewScanner(gz)
	varredor.Buffer(make([]byte, 1024*1024), 1024*1024)
	for varredor.Scan() {
		campos := strings.Split(varredor.Text(), "\t")
		if len(campos) != 3 || campos[0] == "" {
			continue
		}
		idCmn, idTraducao := idsDaAtribuicao(campos[2])
		novo := par{chines: campos[0], traducao: campos[1], atribuicao: campos[2], idCmn: idCmn, idTraducao: idTraducao}
		atual, existe := porChines[campos[0]]
		if !existe || novo.idTraducao < atual.idTraducao {
			porChines[campos[0]] = novo
		}
	}
	if err := varredor.Err(); err != nil {
		return nil, err
	}

	pares := make([]par, 0, len(porChines))
	for _, p := range porChines {
		pares = append(pares, p)
	}
	sort.Slice(pares, func(i, j int) bool {
		if pares[i].idCmn != pares[j].idCmn {
			return pares[i].idCmn < pares[j].idCmn
		}
		return pares[i].idTraducao < pares[j].idTraducao
	})
	return pares, nil
}

// idsDaAtribuicao extrai (id da frase chinesa, id da tradução) da atribuição do Tatoeba. Zero quando
// não casa — não deve ocorrer com atribuição gerada por nós, mas mantém o colapso estável se ocorrer.
func idsDaAtribuicao(atribuicao string) (idCmn, idTraducao int) {
	m := padraoAtribuicao.FindStringSubmatch(atribuicao)
	if m == nil {
		return 0, 0
	}
	idCmn, _ = strconv.Atoi(m[1])
	idTraducao, _ = strconv.Atoi(m[2])
	return idCmn, idTraducao
}

// chaveVeredicto identifica um par de forma estável entre rodadas (para retomar sem repagar).
func chaveVeredicto(p par) string { return p.chines + "\t" + p.traducao }

func paresSemVeredicto(pares []par, veredictos map[string]string) []par {
	var pendentes []par
	for _, p := range pares {
		if _, ok := veredictos[chaveVeredicto(p)]; !ok {
			pendentes = append(pendentes, p)
		}
	}
	return pendentes
}

// ----- Julgamento em lote -----

// julgarPares fatia os pendentes, dispara as requisições em paralelo e devolve chaveVeredicto→ok|errado.
func (v *verificador) julgarPares(pendentes []par) (map[string]string, error) {
	fatias := fatiar(pendentes, PARES_POR_REQUISICAO)
	resultado := map[string]string{}
	var muResultado sync.Mutex
	var wg sync.WaitGroup
	vagas := make(chan struct{}, MAXIMO_REQUISICOES_SIMULTANEAS)
	erros := make([]error, len(fatias))

	for f := range fatias {
		wg.Add(1)
		vagas <- struct{}{}
		go func(indice int) {
			defer wg.Done()
			defer func() { <-vagas }()

			parcial, err := v.julgarFatia(fatias[indice], 1)
			if err != nil {
				erros[indice] = err
				return
			}
			muResultado.Lock()
			for chave, veredicto := range parcial {
				resultado[chave] = veredicto
			}
			muResultado.Unlock()
		}(f)
	}
	wg.Wait()

	for _, err := range erros {
		if err != nil {
			return nil, err
		}
	}
	return resultado, nil
}

// julgarFatia envia um conjunto de pares e devolve chaveVeredicto→ok|errado. Pares sem veredicto
// válido na resposta são reenviados, até MAXIMO_PASSADAS_POR_PAR — persistindo, derruba a rodada
// (nunca se grava um veredicto adivinhado).
func (v *verificador) julgarFatia(pares []par, passada int) (map[string]string, error) {
	var prompt strings.Builder
	for i, p := range pares {
		fmt.Fprintf(&prompt, "%d. %s ⇒ %s\n", i+1, p.chines, p.traducao)
	}

	resposta, err := v.chamarDeepSeek(fmt.Sprintf(MODELO_PROMPT, prompt.String()))
	if err != nil {
		return nil, err
	}

	brutos := extrairVeredictos(resposta)
	resultado := map[string]string{}
	var faltantes []par
	for i, p := range pares {
		veredicto, ok := brutos[i+1]
		if !ok {
			faltantes = append(faltantes, p)
			continue
		}
		resultado[chaveVeredicto(p)] = veredicto
	}

	if len(faltantes) == 0 {
		return resultado, nil
	}
	if passada >= MAXIMO_PASSADAS_POR_PAR {
		return nil, fmt.Errorf("%d par(es) sem veredicto válido após %d passadas (ex.: %q)",
			len(faltantes), passada, faltantes[0].chines)
	}

	reenvio, err := v.julgarFatia(faltantes, passada+1)
	if err != nil {
		return nil, err
	}
	for chave, veredicto := range reenvio {
		resultado[chave] = veredicto
	}
	return resultado, nil
}

// padraoVeredicto casa "N对"/"N. 错" etc.: o número da entrada seguido de 对/對 (correta) ou 错/錯 (errada).
var padraoVeredicto = regexp.MustCompile(`(\d{1,4})\s*[.\s:)]*\s*(对|對|错|錯)`)

// extrairVeredictos varre a resposta e devolve numero→ok|errado. Ignora qualquer prosa fora do padrão.
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
		} else {
			veredictos[numero] = VEREDICTO_ERRADO
		}
	}
	return veredictos
}

// ----- Saída -----

// gravarVeredictos escreve todos os pares (na ordem colapsada) com seu veredicto. Todo par precisa ter
// um veredicto no mapa — ausência aqui é bug no controle de passadas, não caso esperado.
func gravarVeredictos(caminho string, pares []par, veredictos map[string]string) (okCount, erradoCount int) {
	var saida strings.Builder
	for _, p := range pares {
		veredicto := veredictos[chaveVeredicto(p)]
		if veredicto == "" {
			abortar(fmt.Errorf("par %q ficou sem veredicto (bug)", p.chines))
		}
		if veredicto == VEREDICTO_OK {
			okCount++
		} else {
			erradoCount++
		}
		fmt.Fprintf(&saida, "%s\t%s\t%s\t%s\n", p.chines, p.traducao, p.atribuicao, veredicto)
	}
	abortar(os.WriteFile(caminho, []byte(saida.String()), 0o644))
	return okCount, erradoCount
}

// lerVeredictosExistentes relê um veredictos.tsv anterior para retomar sem repagar (chave = chinês+pt).
func lerVeredictosExistentes(caminho string) map[string]string {
	dados, err := os.ReadFile(caminho)
	if err != nil {
		return map[string]string{}
	}
	veredictos := map[string]string{}
	for _, linha := range strings.Split(string(dados), "\n") {
		campos := strings.Split(linha, "\t")
		if len(campos) != 4 {
			continue
		}
		if campos[3] == VEREDICTO_OK || campos[3] == VEREDICTO_ERRADO {
			veredictos[campos[0]+"\t"+campos[1]] = campos[3]
		}
	}
	return veredictos
}

// ----- HTTP / DeepSeek (mesmo esqueleto do tradutor do CEDICT) -----

func (v *verificador) chamarDeepSeek(prompt string) (string, error) {
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
		requisicao.Header.Set("Authorization", "Bearer "+v.chaveApi)

		resposta, err := v.cliente.Do(requisicao)
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

		v.mu.Lock()
		v.tokensEntrada += dados.Usage.PromptTokens
		v.tokensSaida += dados.Usage.CompletionTokens
		v.mu.Unlock()
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

func fatiar(pares []par, tamanho int) [][]par {
	var fatias [][]par
	for inicio := 0; inicio < len(pares); inicio += tamanho {
		fim := inicio + tamanho
		if fim > len(pares) {
			fim = len(pares)
		}
		fatias = append(fatias, pares[inicio:fim])
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

func (v *verificador) custoEstimadoUsd() float64 {
	v.mu.Lock()
	defer v.mu.Unlock()
	return float64(v.tokensEntrada)/1e6*PRECO_ENTRADA_POR_MILHAO + float64(v.tokensSaida)/1e6*PRECO_SAIDA_POR_MILHAO
}

func abortar(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, "erro:", err)
		os.Exit(1)
	}
}
