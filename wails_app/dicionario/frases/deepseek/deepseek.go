package deepseek

// ----- Seção: Cliente compartilhado da API do DeepSeek -----
//
// Esqueleto HTTP reutilizado pelos passos do pipeline que conversam com o DeepSeek. Ficou num pacote
// próprio quando o mesmo cliente já aparecia em três scripts (frases/verificar, frases/classificar e
// traducao/cedict/traduzir) e um quarto ia nascer — a regra do DRY manda extrair na terceira repetição.
//
// Cobre o que todos os chamadores precisavam igual: chave, retentativa com backoff em 429/5xx,
// leitura da resposta e contabilidade de tokens/custo. O que é específico de cada passo (prompt,
// parsing da resposta, lotes, retomada) continua no script, que é onde varia de verdade.
//
// Temperatura: Conversar usa 0 (TEMPERATURA) — julgamento/tradução determinística, base dos arquivos
// versionados estáveis entre rodadas. A geração de frases NOVAS (frases/gerar) precisa do oposto:
// temperatura > 0 via ConversarComTemperatura, senão toda execução devolveria as mesmas frases e a
// deduplicação zeraria tudo depois da primeira rodada.
//
// Chave da API: variável DEEPSEEK_API_KEY ou o arquivo gitignored do tradutor do CEDICT.
// Os caminhos relativos partem da RAIZ do repo, igual ao resto do pipeline.

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

// ErrConteudoBloqueado sinaliza que a moderação da API rejeitou a requisição (400 "Content Exists Risk").
// NÃO é retentável — a mesma entrada vai reprovar de novo — então o chamador deve isolar e pular o
// conteúdo ofensor em vez de tentar de novo com o lote inteiro.
var ErrConteudoBloqueado = errors.New("conteúdo bloqueado pela moderação da API")

const (
	URL_API            = "https://api.deepseek.com/chat/completions"
	MODELO             = "deepseek-chat"
	VARIAVEL_CHAVE_API = "DEEPSEEK_API_KEY"

	MAXIMO_TENTATIVAS_HTTP = 4
	TEMPERATURA            = 0.0
	TIMEOUT_REQUISICAO     = 4 * time.Minute

	PRECO_ENTRADA_POR_MILHAO = 0.28
	PRECO_SAIDA_POR_MILHAO   = 0.42
)

// Cliente guarda a chave, o cliente HTTP e a contabilidade de tokens (segura para uso concorrente).
type Cliente struct {
	chaveApi      string
	cliente       *http.Client
	mu            sync.Mutex
	tokensEntrada int64
	tokensSaida   int64
}


// ----- Construção -----

func Novo() (*Cliente, error) {
	chave, err := carregarChaveApi()
	if err != nil {
		return nil, err
	}
	return &Cliente{chaveApi: chave, cliente: &http.Client{Timeout: TIMEOUT_REQUISICAO}}, nil
}


// ----- Conversa -----

// Conversar manda um par instruções-de-sistema + prompt e devolve o conteúdo da resposta, em
// temperatura 0 (determinística). Repete com backoff em erro de rede/429/5xx; status de erro definitivo
// (4xx que não 429) devolve erro na hora.
func (c *Cliente) Conversar(instrucoesSistema, prompt string, maximoTokens int) (string, error) {
	return c.ConversarComTemperatura(instrucoesSistema, prompt, maximoTokens, TEMPERATURA)
}


// ConversarComTemperatura é como Conversar, mas com temperatura explícita — para a geração de frases,
// que precisa de variedade entre execuções (a 0 repetiria as mesmas frases). Julgamento e tradução
// continuam determinísticos usando Conversar (temperatura 0).
func (c *Cliente) ConversarComTemperatura(instrucoesSistema, prompt string, maximoTokens int, temperatura float64) (string, error) {
	corpo, err := json.Marshal(map[string]any{
		"model": MODELO,
		"messages": []map[string]string{
			{"role": "system", "content": instrucoesSistema},
			{"role": "user", "content": prompt},
		},
		"temperature": temperatura,
		"max_tokens":  maximoTokens,
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

		conteudo, erroTentativa, fatal := c.tentar(corpo)
		if fatal != nil {
			return "", fatal
		}
		if erroTentativa != nil {
			ultimoErro = erroTentativa
			continue
		}
		return conteudo, nil
	}
	return "", fmt.Errorf("desisti após %d tentativas: %w", MAXIMO_TENTATIVAS_HTTP, ultimoErro)
}


// tentar faz uma requisição. Devolve (conteúdo, erro-retentável, erro-fatal): só o fatal interrompe o laço.
func (c *Cliente) tentar(corpo []byte) (conteudo string, retentavel, fatal error) {
	requisicao, err := http.NewRequest(http.MethodPost, URL_API, bytes.NewReader(corpo))
	if err != nil {
		return "", nil, err
	}
	requisicao.Header.Set("Content-Type", "application/json")
	requisicao.Header.Set("Authorization", "Bearer "+c.chaveApi)

	resposta, err := c.cliente.Do(requisicao)
	if err != nil {
		return "", err, nil
	}
	corpoResposta, err := io.ReadAll(resposta.Body)
	resposta.Body.Close()
	if err != nil {
		return "", err, nil
	}

	if resposta.StatusCode == http.StatusBadRequest && bytes.Contains(corpoResposta, []byte("Content Exists Risk")) {
		return "", nil, ErrConteudoBloqueado
	}
	if resposta.StatusCode == http.StatusTooManyRequests || resposta.StatusCode >= 500 {
		return "", fmt.Errorf("API devolveu %s", resposta.Status), nil
	}
	if resposta.StatusCode != http.StatusOK {
		return "", nil, fmt.Errorf("API devolveu %s: %s", resposta.Status, ResumoCorpo(corpoResposta))
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
		return "", fmt.Errorf("resposta inesperada da API: %s", ResumoCorpo(corpoResposta)), nil
	}

	c.mu.Lock()
	c.tokensEntrada += dados.Usage.PromptTokens
	c.tokensSaida += dados.Usage.CompletionTokens
	c.mu.Unlock()
	return dados.Choices[0].Message.Content, nil, nil
}


// ----- Contabilidade -----

func (c *Cliente) Tokens() (entrada, saida int64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.tokensEntrada, c.tokensSaida
}


func (c *Cliente) CustoEstimadoUsd() float64 {
	entrada, saida := c.Tokens()
	return float64(entrada)/1e6*PRECO_ENTRADA_POR_MILHAO + float64(saida)/1e6*PRECO_SAIDA_POR_MILHAO
}


// ----- Utilitários -----

func ResumoCorpo(corpo []byte) string {
	texto := strings.TrimSpace(string(corpo))
	if len(texto) > 300 {
		texto = texto[:300] + "…"
	}
	return texto
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
