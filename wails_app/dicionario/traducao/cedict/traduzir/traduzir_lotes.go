package main

// ----- Seção: Tradução dos Lotes do CEDICT via IA (DeepSeek / Gemini) -----
// Traduz as glosas de lote_NNN.txt (inglês) para lote_NNN_<idioma>.txt (ex.: _pt, _es)
// chamando DeepSeek (deepseek-chat) ou Google Gemini (gemini-2.5-flash) conforme configurado no .env.
// Os prompts utilizam instruções em chinês para máxima concisão e economia de tokens.
//
// Uso (executar da raiz do repositório):
//	go run wails_app/dicionario/traducao/cedict/traduzir/traduzir_lotes.go es            → todos os pendentes em espanhol
//	go run wails_app/dicionario/traducao/cedict/traduzir/traduzir_lotes.go es 001 002    → sobrescreve os lotes 001 e 002 em espanhol
//	go run wails_app/dicionario/traducao/cedict/traduzir/traduzir_lotes.go pt            → todos os pendentes em português
//	go run wails_app/dicionario/traducao/cedict/traduzir/traduzir_lotes.go pt 001 002    → sobrescreve os lotes 001 e 002 em português

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

// ----- Constantes e Provedores -----

const (
	PROVEDOR_DEEPSEEK = "deepseek"
	PROVEDOR_GEMINI   = "gemini"

	VARIAVEL_PROVEDOR_IA     = "PROVEDOR_IA"
	VARIAVEL_CHAVE_DEEPSEEK  = "DEEPSEEK_API_KEY"
	VARIAVEL_CHAVE_GEMINI    = "GEMINI_API_KEY"

	URL_API_DEEPSEEK = "https://api.deepseek.com/chat/completions"
	MODELO_DEEPSEEK  = "deepseek-chat"

	MODELO_GEMINI_PADRAO = "gemini-3.1-flash-lite"

	PASTA_LOTES        = "wails_app/dicionario/traducao/cedict/lotes"
	PRIMEIRO_LOTE      = 1
	ULTIMO_LOTE        = 122
	MARCADOR_TRADUCAO  = " | TRADUÇÃO: "
	SEPARADOR_LEITURAS = "‖"

	LINHAS_POR_REQUISICAO          = 100
	MAXIMO_REQUISICOES_SIMULTANEAS = 6
	MAXIMO_TENTATIVAS_HTTP         = 4
	MAXIMO_PASSADAS_POR_LINHA      = 3
	MAXIMO_TOKENS_RESPOSTA         = 8192
	TEMPERATURA_TRADUCAO           = 1.0
	TIMEOUT_REQUISICAO             = 4 * time.Minute

	PRECO_ENTRADA_DEEPSEEK_MILHAO = 0.28
	PRECO_SAIDA_DEEPSEEK_MILHAO   = 0.42

	PRECO_ENTRADA_GEMINI_MILHAO = 0.25
	PRECO_SAIDA_GEMINI_MILHAO   = 1.50
)

// ----- Configuração de Idioma e Prompts em Chinês -----

type configuracaoIdioma struct {
	codigo             string
	sufixoArquivo      string
	rotuloIdioma       string
	instrucoesSistema  string
	modeloPrompt       string
}

const INSTRUCOES_SISTEMA_PT = `你是汉语与巴西葡萄牙语（pt-BR）词典编纂专家，负责将中文词条释义精准翻译为自然地道的巴西葡萄牙语。`

const MODELO_PROMPT_REQUISICAO_PT = `每行格式为：“编号. 词语 [拼音] /英文义项1/英文义项2/...‖[多音拼音] /义项/...”。英文仅供语义参考，请直接根据中文原意翻译为巴西葡萄牙语。

规则：
1. 每行输出格式：“编号. 翻译”，必须严格保留编号，严禁在编号后重复中文词头。
2. 保持英文原有的结构与分组：多音字组间用“‖”分隔，同组内多个近义释义用“/”分隔。
3. 纯姓氏标记翻译为“(sobrenome)”，语法/方言标记如(coll.)译为(coloq.)，(idiom)译为(modismo)，(Taiwan)译为(Taiwan)。
4. 仅输出编号和翻译，严禁任何额外解释、问候或markdown格式。

待翻译词条：
%s`

const INSTRUCOES_SISTEMA_ES = `你是汉语与西班牙语（es）词典编纂专家，负责将中文词条释义精准翻译为自然地道的西班牙语。`

const MODELO_PROMPT_REQUISICAO_ES = `每行格式为：“编号. 词语 [拼音] /英文义项1/英文义项2/...‖[多音拼音] /义项/...”。英文仅供语义参考，请直接根据中文原意翻译为西班牙语。

规则：
1. 每行输出格式：“编号. 翻译”，必须严格保留编号，严禁在编号后重复中文词头。
2. 保持英文原有的结构与分组：多音字组间用“‖”分隔，同组内多个近义释义用“/”分隔。
3. 纯姓氏标记翻译为“(apellido)”，语法/方言标记如(coll.)译为(coloq.)，(idiom)译为(modismo)，(Taiwan)译为(Taiwán)。
4. 仅输出编号和翻译，严禁任何额外解释、问候或markdown格式。

待翻译词条：
%s`

// linhaLote armazena uma linha do lote original.
type linhaLote struct {
	numero             int
	palavra            string
	restoOriginal      string
	quantidadeLeituras int
}

// tradutor gerencia chamadas HTTP, concorrência e contagem de tokens.
type tradutor struct {
	provedor      string
	modelo        string
	chaveApi      string
	config        configuracaoIdioma
	cliente       *http.Client
	mu            sync.Mutex
	tokensEntrada int64
	tokensSaida   int64
}


func main() {
	configIdioma, lotes, forcar, err := interpretarArgumentos(os.Args[1:])
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	provedor, chave, modelo, err := obterProvedorConfigurado()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	t := &tradutor{
		provedor: provedor,
		modelo:   modelo,
		chaveApi: chave,
		config:   configIdioma,
		cliente:  &http.Client{Timeout: TIMEOUT_REQUISICAO},
	}

	var pendentes []string
	for _, nnn := range lotes {
		if !forcar && t.traducaoExistenteValida(nnn) {
			fmt.Printf("lote %s: %s existente e estruturalmente válido — pulado (passe o número explícito para refazer)\n", nnn, configIdioma.sufixoArquivo)
			continue
		}
		pendentes = append(pendentes, nnn)
	}
	if len(pendentes) == 0 {
		fmt.Printf("Nada a fazer: todos os lotes alvo já têm tradução válida em [%s].\n", configIdioma.codigo)
		return
	}

	fmt.Printf("Traduzindo %d lote(s) do CEDICT para [%s] via %s (%s) — %d linhas/req, %d reqs paralelas\n\n",
		len(pendentes), configIdioma.codigo, strings.ToUpper(provedor), modelo, LINHAS_POR_REQUISICAO, MAXIMO_REQUISICOES_SIMULTANEAS)

	var falhas []string
	for _, nnn := range pendentes {
		if err := t.processarLote(nnn); err != nil {
			fmt.Printf("lote %s: FALHA — %v\n", nnn, err)
			falhas = append(falhas, nnn)
		}
	}

	fmt.Printf("\n===== RESUMO (%s) =====\n", strings.ToUpper(configIdioma.codigo))
	fmt.Printf("%d lote(s) traduzido(s), %d com falha", len(pendentes)-len(falhas), len(falhas))
	if len(falhas) > 0 {
		fmt.Printf(" (%s)", strings.Join(falhas, ", "))
	}
	fmt.Printf("\nTokens: %d de entrada + %d de saída ≈ US$ %.2f\n", t.tokensEntrada, t.tokensSaida, t.custoEstimadoUsd())

	if len(falhas) > 0 {
		os.Exit(1)
	}
}


func (t *tradutor) processarLote(nnn string) error {
	inicio := time.Now()
	cabecalho, linhas, err := lerLote(t.caminhoLote(nnn, false))
	if err != nil {
		return err
	}
	if len(linhas) == 0 {
		return fmt.Errorf("lote sem linhas de dados")
	}

	fatias := fatiarLinhas(linhas, LINHAS_POR_REQUISICAO)
	traducoes := make(map[int]string, len(linhas))
	var muTraducoes sync.Mutex
	var wg sync.WaitGroup
	vagas := make(chan struct{}, MAXIMO_REQUISICOES_SIMULTANEAS)
	erros := make([]error, len(fatias))

	for f := range fatias {
		wg.Add(1)
		vagas <- struct{}{}
		go func(indiceFatia int) {
			defer wg.Done()
			defer func() { <-vagas }()

			resultado, err := t.traduzirLinhas(fatias[indiceFatia], 1)
			if err != nil {
				erros[indiceFatia] = err
				return
			}
			muTraducoes.Lock()
			for numero, glosa := range resultado {
				traducoes[numero] = glosa
			}
			muTraducoes.Unlock()
		}(f)
	}
	wg.Wait()

	for _, err := range erros {
		if err != nil {
			return err
		}
	}

	if err := t.gravarLoteTraduzido(nnn, cabecalho, linhas, traducoes); err != nil {
		return err
	}
	fmt.Printf("lote %s: traduzido em %s (%d linhas, %d requisições)\n", nnn, time.Since(inicio).Round(time.Second), len(linhas), len(fatias))

	return t.rodarVerificadorOficial(nnn)
}


func (t *tradutor) traduzirLinhas(linhas []linhaLote, passada int) (map[int]string, error) {
	var prompt strings.Builder
	for _, l := range linhas {
		fmt.Fprintf(&prompt, "%d. %s\n", l.numero, l.restoOriginal)
	}

	resposta, err := t.chamarIA(fmt.Sprintf(t.config.modeloPrompt, prompt.String()))
	if err != nil {
		return nil, err
	}

	porNumero := make(map[int]linhaLote, len(linhas))
	for _, l := range linhas {
		porNumero[l.numero] = l
	}

	traducoes := make(map[int]string, len(linhas))
	for numero, texto := range extrairRespostasNumeradas(resposta) {
		linha, pedida := porNumero[numero]
		if !pedida {
			continue
		}
		glosa := limparGlosaDevolvida(texto, linha.palavra)
		glosa = achatarGruposInventados(glosa, linha.quantidadeLeituras)
		if glosaCompativel(glosa, linha) {
			traducoes[numero] = glosa
		}
	}

	if len(traducoes) == 0 && len(linhas) > 0 {
		traducoes = tentarRespostasSemNumeracao(resposta, linhas)
	}

	var invalidas []linhaLote
	for _, l := range linhas {
		if _, ok := traducoes[l.numero]; !ok {
			invalidas = append(invalidas, l)
		}
	}
	if len(invalidas) == 0 {
		return traducoes, nil
	}
	if passada >= MAXIMO_PASSADAS_POR_LINHA {
		return nil, fmt.Errorf("%d linha(s) sem tradução válida após %d passadas (ex.: linha %d %q)",
			len(invalidas), passada, invalidas[0].numero, invalidas[0].palavra)
	}

	reenvio, err := t.traduzirLinhas(invalidas, passada+1)
	if err != nil {
		return nil, err
	}
	for numero, glosa := range reenvio {
		traducoes[numero] = glosa
	}
	return traducoes, nil
}


func (t *tradutor) chamarIA(prompt string) (string, error) {
	if t.provedor == PROVEDOR_GEMINI {
		return t.chamarGemini(prompt)
	}
	return t.chamarDeepSeek(prompt)
}


func (t *tradutor) chamarDeepSeek(prompt string) (string, error) {
	corpo, err := json.Marshal(map[string]any{
		"model": MODELO_DEEPSEEK,
		"messages": []map[string]string{
			{"role": "system", "content": t.config.instrucoesSistema},
			{"role": "user", "content": prompt},
		},
		"temperature": TEMPERATURA_TRADUCAO,
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
		requisicao.Header.Set("Authorization", "Bearer "+t.chaveApi)

		resposta, err := t.cliente.Do(requisicao)
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
			ultimoErro = fmt.Errorf("API DeepSeek devolveu %s", resposta.Status)
			continue
		}
		if resposta.StatusCode != http.StatusOK {
			return "", fmt.Errorf("API DeepSeek devolveu %s: %s", resposta.Status, resumoCorpo(corpoResposta))
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
			ultimoErro = fmt.Errorf("resposta inesperada do DeepSeek: %s", resumoCorpo(corpoResposta))
			continue
		}

		t.mu.Lock()
		t.tokensEntrada += dados.Usage.PromptTokens
		t.tokensSaida += dados.Usage.CompletionTokens
		t.mu.Unlock()
		return dados.Choices[0].Message.Content, nil
	}
	return "", fmt.Errorf("desisti após %d tentativas no DeepSeek: %w", MAXIMO_TENTATIVAS_HTTP, ultimoErro)
}


func (t *tradutor) chamarGemini(prompt string) (string, error) {
	corpo, err := json.Marshal(map[string]any{
		"contents": []map[string]any{
			{
				"role": "user",
				"parts": []map[string]string{
					{"text": prompt},
				},
			},
		},
		"systemInstruction": map[string]any{
			"parts": []map[string]string{
				{"text": t.config.instrucoesSistema},
			},
		},
		"generationConfig": map[string]any{
			"temperature":     TEMPERATURA_TRADUCAO,
			"maxOutputTokens": MAXIMO_TOKENS_RESPOSTA,
		},
	})
	if err != nil {
		return "", err
	}

	var ultimoErro error
	for tentativa := 1; tentativa <= MAXIMO_TENTATIVAS_HTTP; tentativa++ {
		if tentativa > 1 {
			time.Sleep(time.Duration(5<<(tentativa-2)) * time.Second)
		}

		urlGemini := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent", t.modelo)
		requisicao, err := http.NewRequest(http.MethodPost, urlGemini, bytes.NewReader(corpo))
		if err != nil {
			return "", err
		}
		requisicao.Header.Set("Content-Type", "application/json")
		requisicao.Header.Set("x-goog-api-key", t.chaveApi)

		resposta, err := t.cliente.Do(requisicao)
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
			ultimoErro = fmt.Errorf("API Gemini devolveu %s", resposta.Status)
			continue
		}
		if resposta.StatusCode != http.StatusOK {
			return "", fmt.Errorf("API Gemini devolveu %s: %s", resposta.Status, resumoCorpo(corpoResposta))
		}

		var dados struct {
			Candidates []struct {
				Content struct {
					Parts []struct {
						Text string `json:"text"`
					} `json:"parts"`
				} `json:"content"`
			} `json:"candidates"`
			UsageMetadata struct {
				PromptTokenCount     int64 `json:"promptTokenCount"`
				CandidatesTokenCount int64 `json:"candidatesTokenCount"`
			} `json:"usageMetadata"`
			Error *struct {
				Code    int    `json:"code"`
				Message string `json:"message"`
			} `json:"error"`
		}
		if err := json.Unmarshal(corpoResposta, &dados); err != nil {
			ultimoErro = fmt.Errorf("resposta inesperada do Gemini: %s", resumoCorpo(corpoResposta))
			continue
		}
		if dados.Error != nil {
			return "", fmt.Errorf("API do Gemini retornou erro %d: %s", dados.Error.Code, dados.Error.Message)
		}
		if len(dados.Candidates) == 0 || len(dados.Candidates[0].Content.Parts) == 0 {
			ultimoErro = fmt.Errorf("Gemini não devolveu texto: %s", resumoCorpo(corpoResposta))
			continue
		}

		t.mu.Lock()
		t.tokensEntrada += dados.UsageMetadata.PromptTokenCount
		t.tokensSaida += dados.UsageMetadata.CandidatesTokenCount
		t.mu.Unlock()

		return dados.Candidates[0].Content.Parts[0].Text, nil
	}
	return "", fmt.Errorf("desisti após %d tentativas no Gemini: %w", MAXIMO_TENTATIVAS_HTTP, ultimoErro)
}


func (t *tradutor) gravarLoteTraduzido(nnn string, cabecalho []string, linhas []linhaLote, traducoes map[int]string) error {
	var saida strings.Builder
	for _, l := range cabecalho {
		saida.WriteString(l)
		saida.WriteByte('\n')
	}
	for _, l := range linhas {
		glosa, ok := traducoes[l.numero]
		if !ok {
			return fmt.Errorf("linha %d (%s) ficou sem tradução", l.numero, l.palavra)
		}
		saida.WriteString(l.palavra)
		saida.WriteString(MARCADOR_TRADUCAO)
		saida.WriteString(glosa)
		saida.WriteByte('\n')
	}
	return os.WriteFile(t.caminhoLote(nnn, true), []byte(saida.String()), 0o644)
}


func (t *tradutor) rodarVerificadorOficial(nnn string) error {
	comando := exec.Command("go", "run", "wails_app/dicionario/traducao/cedict/validar/validar_lote.go", t.config.codigo, nnn)
	comando.Stdout = os.Stdout
	comando.Stderr = os.Stderr
	if err := comando.Run(); err != nil {
		return fmt.Errorf("verificador oficial (validar_lote.go %s %s) reprovou o lote", t.config.codigo, nnn)
	}
	return nil
}


func (t *tradutor) traducaoExistenteValida(nnn string) bool {
	_, original, err := lerLote(t.caminhoLote(nnn, false))
	if err != nil {
		return false
	}
	_, traduzido, err := lerLote(t.caminhoLote(nnn, true))
	if err != nil || len(original) != len(traduzido) {
		return false
	}
	for i := range original {
		if original[i].palavra != traduzido[i].palavra {
			return false
		}
		if !glosaCompativel(traduzido[i].restoOriginal, original[i]) {
			return false
		}
	}
	return true
}


func (t *tradutor) caminhoLote(nnn string, traduzido bool) string {
	sufixo := ""
	if traduzido {
		sufixo = t.config.sufixoArquivo
	}
	return fmt.Sprintf("%s/lote_%s%s.txt", PASTA_LOTES, nnn, sufixo)
}


func (t *tradutor) custoEstimadoUsd() float64 {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.provedor == PROVEDOR_GEMINI {
		return float64(t.tokensEntrada)/1e6*PRECO_ENTRADA_GEMINI_MILHAO + float64(t.tokensSaida)/1e6*PRECO_SAIDA_GEMINI_MILHAO
	}
	return float64(t.tokensEntrada)/1e6*PRECO_ENTRADA_DEEPSEEK_MILHAO + float64(t.tokensSaida)/1e6*PRECO_SAIDA_DEEPSEEK_MILHAO
}

// ----- Seção: Interpretação de Argumentos e Configuração de Ambiente -----

func interpretarArgumentos(args []string) (config configuracaoIdioma, lotes []string, forcar bool, err error) {
	if len(args) == 0 {
		return config, nil, false, fmt.Errorf("uso: traduzir_lotes.go <pt|es> [NNN ...]\n  ex.: go run .../traduzir_lotes.go es        (todos os pendentes)\n  ex.: go run .../traduzir_lotes.go es 001    (sobrescreve o lote 001)")
	}

	config, err = obterConfiguracaoIdioma(args[0])
	if err != nil {
		return config, nil, false, err
	}

	argsLotes := args[1:]
	if len(argsLotes) == 0 {
		for n := PRIMEIRO_LOTE; n <= ULTIMO_LOTE; n++ {
			lotes = append(lotes, fmt.Sprintf("%03d", n))
		}
		return config, lotes, false, nil
	}

	for _, arg := range argsLotes {
		n, err := strconv.Atoi(arg)
		if err != nil || len(arg) != 3 || n < PRIMEIRO_LOTE || n > ULTIMO_LOTE {
			return config, nil, false, fmt.Errorf("argumento de lote inválido %q: use números de 3 dígitos entre %03d e %03d, ou nenhum número para todos os pendentes", arg, PRIMEIRO_LOTE, ULTIMO_LOTE)
		}
		lotes = append(lotes, fmt.Sprintf("%03d", n))
	}
	return config, lotes, true, nil
}


func obterConfiguracaoIdioma(idioma string) (configuracaoIdioma, error) {
	normalizado := strings.ToLower(strings.TrimSpace(idioma))
	normalizado = strings.ReplaceAll(normalizado, "_", "-")

	switch normalizado {
	case "pt", "pt-br":
		return configuracaoIdioma{
			codigo:             "pt",
			sufixoArquivo:      "_pt",
			rotuloIdioma:       "português do Brasil",
			instrucoesSistema:  INSTRUCOES_SISTEMA_PT,
			modeloPrompt:       MODELO_PROMPT_REQUISICAO_PT,
		}, nil
	case "es":
		return configuracaoIdioma{
			codigo:             "es",
			sufixoArquivo:      "_es",
			rotuloIdioma:       "espanhol",
			instrucoesSistema:  INSTRUCOES_SISTEMA_ES,
			modeloPrompt:       MODELO_PROMPT_REQUISICAO_ES,
		}, nil
	default:
		return configuracaoIdioma{}, fmt.Errorf("idioma não suportado %q: use 'pt' (ou 'pt-br') ou 'es'", idioma)
	}
}


func obterProvedorConfigurado() (provedor, chave, modelo string, err error) {
	provedorConfig := strings.ToLower(strings.TrimSpace(obterVariavelAmbiente(VARIAVEL_PROVEDOR_IA)))
	if provedorConfig == "" {
		provedorConfig = strings.ToLower(strings.TrimSpace(obterVariavelAmbiente("PROVEDOR_TRADUCAO")))
	}
	if provedorConfig == "" {
		provedorConfig = PROVEDOR_DEEPSEEK
	}

	switch provedorConfig {
	case "gemini", "google":
		chave = obterVariavelAmbiente(VARIAVEL_CHAVE_GEMINI)
		if chave == "" {
			return "", "", "", fmt.Errorf("chave da API do Gemini não encontrada: defina %s no ambiente ou no arquivo .env", VARIAVEL_CHAVE_GEMINI)
		}
		modelo = strings.TrimSpace(obterVariavelAmbiente("GEMINI_MODEL"))
		if modelo == "" {
			modelo = strings.TrimSpace(obterVariavelAmbiente("MODELO_GEMINI"))
		}
		if modelo == "" {
			modelo = MODELO_GEMINI_PADRAO
		}
		return PROVEDOR_GEMINI, chave, modelo, nil
	case "deepseek":
		chave = obterVariavelAmbiente(VARIAVEL_CHAVE_DEEPSEEK)
		if chave == "" {
			return "", "", "", fmt.Errorf("chave da API do DeepSeek não encontrada: defina %s no ambiente ou no arquivo .env", VARIAVEL_CHAVE_DEEPSEEK)
		}
		modelo = strings.TrimSpace(obterVariavelAmbiente("DEEPSEEK_MODEL"))
		if modelo == "" {
			modelo = MODELO_DEEPSEEK
		}
		return PROVEDOR_DEEPSEEK, chave, modelo, nil
	default:
		return "", "", "", fmt.Errorf("provedor de IA desconhecido %q: use 'deepseek' ou 'gemini' no .env", provedorConfig)
	}
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

// ----- Seção: Manipulação de Arquivos e Parsing -----

func lerLote(caminho string) (cabecalho []string, linhas []linhaLote, err error) {
	conteudo, err := os.ReadFile(caminho)
	if err != nil {
		return nil, nil, err
	}

	todas := strings.Split(strings.ReplaceAll(string(conteudo), "\r\n", "\n"), "\n")
	numero := 0
	for _, bruta := range todas {
		if strings.HasPrefix(bruta, "#") {
			cabecalho = append(cabecalho, bruta)
			continue
		}
		if strings.TrimSpace(bruta) == "" {
			continue
		}
		palavra, resto, err := dividirLinhaDoLote(bruta)
		if err != nil {
			return nil, nil, fmt.Errorf("%s: %w", caminho, err)
		}
		numero++
		linhas = append(linhas, linhaLote{
			numero:             numero,
			palavra:            palavra,
			restoOriginal:      resto,
			quantidadeLeituras: contarGrupos(resto),
		})
	}
	return cabecalho, linhas, nil
}


func dividirLinhaDoLote(linha string) (palavra, resto string, err error) {
	idx := strings.Index(linha, MARCADOR_TRADUCAO)
	if idx < 0 {
		return "", "", fmt.Errorf("marcador %q não encontrado na linha: %q", MARCADOR_TRADUCAO, linha)
	}
	palavra = strings.TrimSpace(linha[:idx])
	resto = strings.TrimSpace(linha[idx+len(MARCADOR_TRADUCAO):])
	if palavra == "" {
		return "", "", fmt.Errorf("palavra vazia na linha: %q", linha)
	}
	return palavra, resto, nil
}



func contarSignificados(grupo string) int {
	grupo = strings.TrimSpace(grupo)
	if strings.HasPrefix(grupo, "[") {
		if fecho := strings.Index(grupo, "]"); fecho >= 0 {
			grupo = strings.TrimSpace(grupo[fecho+1:])
		}
	}
	partes := strings.Split(grupo, "/")
	total := 0
	for _, p := range partes {
		if strings.TrimSpace(p) != "" {
			total++
		}
	}
	return total
}


func fatiarLinhas(linhas []linhaLote, tamanho int) [][]linhaLote {
	var fatias [][]linhaLote
	for inicio := 0; inicio < len(linhas); inicio += tamanho {
		fim := inicio + tamanho
		if fim > len(linhas) {
			fim = len(linhas)
		}
		fatias = append(fatias, linhas[inicio:fim])
	}
	return fatias
}


var padraoLinhaNumerada = regexp.MustCompile(`^\s*(\d{1,4})\s*[.:)]\s*(.+)$`)

func extrairRespostasNumeradas(resposta string) map[int]string {
	respostas := make(map[int]string)
	for _, linha := range strings.Split(resposta, "\n") {
		casamento := padraoLinhaNumerada.FindStringSubmatch(linha)
		if casamento == nil {
			continue
		}
		numero, err := strconv.Atoi(casamento[1])
		if err != nil {
			continue
		}
		respostas[numero] = strings.TrimSpace(casamento[2])
	}
	return respostas
}


func tentarRespostasSemNumeracao(resposta string, linhas []linhaLote) map[int]string {
	var candidatas []string
	for _, l := range strings.Split(resposta, "\n") {
		t := strings.TrimSpace(l)
		if t == "" || strings.HasPrefix(t, "```") || strings.HasPrefix(t, "#") {
			continue
		}
		candidatas = append(candidatas, t)
	}
	if len(candidatas) != len(linhas) {
		return nil
	}

	respostas := make(map[int]string, len(linhas))
	for i, l := range linhas {
		glosa := limparGlosaDevolvida(candidatas[i], l.palavra)
		glosa = achatarGruposInventados(glosa, l.quantidadeLeituras)
		if !glosaCompativel(glosa, l) {
			return nil
		}
		respostas[l.numero] = glosa
	}
	return respostas
}


func limparGlosaDevolvida(texto, palavra string) string {
	glosa := strings.TrimSpace(texto)
	glosa = strings.TrimPrefix(glosa, "```")
	glosa = strings.TrimSuffix(glosa, "```")
	glosa = strings.TrimSpace(glosa)

	if strings.HasPrefix(glosa, palavra) {
		glosa = strings.TrimSpace(strings.TrimPrefix(glosa, palavra))
	}
	if strings.HasPrefix(glosa, "|") {
		glosa = strings.TrimSpace(strings.TrimPrefix(glosa, "|"))
	}
	if strings.HasPrefix(glosa, "TRADUÇÃO:") {
		glosa = strings.TrimSpace(strings.TrimPrefix(glosa, "TRADUÇÃO:"))
	}
	if strings.HasPrefix(glosa, "TRADUCAO:") {
		glosa = strings.TrimSpace(strings.TrimPrefix(glosa, "TRADUCAO:"))
	}

	grupos := strings.Split(glosa, SEPARADOR_LEITURAS)
	for i, g := range grupos {
		partes := strings.Split(g, "/")
		var limpos []string
		for _, p := range partes {
			p = strings.TrimSpace(p)
			if p != "" {
				limpos = append(limpos, p)
			}
		}
		grupos[i] = strings.Join(limpos, "/")
	}
	return strings.TrimSpace(strings.Join(grupos, " ‖ "))
}


func achatarGruposInventados(glosa string, leiturasOriginais int) string {
	if leiturasOriginais != 1 || !strings.Contains(glosa, SEPARADOR_LEITURAS) {
		return glosa
	}
	pedacos := strings.Split(glosa, SEPARADOR_LEITURAS)
	var todos []string
	for _, p := range pedacos {
		p = strings.TrimSpace(p)
		p = strings.Trim(p, "/")
		if p != "" {
			todos = append(todos, p)
		}
	}
	if len(todos) == 0 {
		return ""
	}
	return "/" + strings.Join(todos, "/") + "/"
}


func glosaCompativel(glosa string, l linhaLote) bool {
	if strings.TrimSpace(glosa) == "" {
		return false
	}
	if !validarSintaxeBasica(glosa) {
		return false
	}
	gruposTrad := contarGrupos(glosa)
	if gruposTrad > 1 || l.quantidadeLeituras == 1 {
		return gruposTrad == l.quantidadeLeituras
	}
	return contarSignificadosTotais(glosa) == contarSignificadosTotais(l.restoOriginal)
}


func validarSintaxeBasica(glosa string) bool {
	for _, g := range strings.Split(glosa, SEPARADOR_LEITURAS) {
		g = strings.TrimSpace(g)
		if g == "" {
			return false
		}
		if contarSignificados(g) == 0 {
			return false
		}
	}
	return true
}


func contarGrupos(glosa string) int {
	if !strings.Contains(glosa, SEPARADOR_LEITURAS) {
		return 1
	}
	return len(strings.Split(glosa, SEPARADOR_LEITURAS))
}


func contarSignificadosTotais(glosa string) int {
	total := 0
	for _, g := range strings.Split(glosa, SEPARADOR_LEITURAS) {
		total += contarSignificados(g)
	}
	return total
}


func resumoCorpo(corpo []byte) string {
	texto := strings.TrimSpace(string(corpo))
	if len(texto) > 300 {
		texto = texto[:300] + "…"
	}
	return texto
}
