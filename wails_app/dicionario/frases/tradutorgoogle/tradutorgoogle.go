package tradutorgoogle

// ----- Seção: Motor compartilhado de tradução chinês → idioma-alvo via Google Tradutor -----
//
// Infraestrutura reutilizada pelos passos do pipeline de frases que precisam traduzir texto chinês com
// o Google (frases/montar e frases/complementar traduzem para pt-BR; frases/gerar traduz para cada
// idioma disponível). Fica num pacote próprio porque é consumida por mais de uma seção — a lógica de
// rede, backoff e cache mora num único lugar.
//
// O idioma-alvo é o CÓDIGO do Google (pt, en, …). Os métodos sem código (TraduzirLote/Traduzir) usam
// pt-BR por padrão (LINGUA_ALVO); os com sufixo ParaIdioma recebem o código explícito.
//
//   1. Cloud Translation API v2, se houver chave (variável GOOGLE_TRANSLATE_API_KEY ou no .env) —
//      o caminho oficial, recomendado e que aceita LOTE (vários textos numa só requisição).
//   2. Endpoint público translate.googleapis.com/translate_a (sem chave) — traduz um texto por vez;
//      serve só para semear poucos dados, não para volume.
//
// Os caminhos relativos (arquivo de chave) partem da RAIZ do repo, igual ao resto do pipeline.

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"sort"
	"strings"
	"time"
)

const (
	VARIAVEL_CHAVE_GOOGLE = "GOOGLE_TRANSLATE_API_KEY"

	URL_API_OFICIAL    = "https://translation.googleapis.com/language/translate/v2"
	URL_ENDPOINT_LIVRE = "https://translate.googleapis.com/translate_a/single"
	LINGUA_ORIGEM      = "zh-CN"
	LINGUA_ALVO        = "pt" // padrão dos métodos sem código de idioma explícito

	MAXIMO_TENTATIVAS_HTTP = 4
	TIMEOUT_REQUISICAO     = 1 * time.Minute
)

// Tradutor abstrai o motor de tradução (oficial com chave ou endpoint livre).
type Tradutor struct {
	chave   string // vazio → endpoint livre
	cliente *http.Client
}

// ----- Construção e identificação -----

func Novo() *Tradutor {
	return &Tradutor{
		chave:   carregarChaveGoogle(),
		cliente: &http.Client{Timeout: TIMEOUT_REQUISICAO},
	}
}

func (t *Tradutor) TemChave() bool {
	return t.chave != ""
}

func (t *Tradutor) NomeMotor() string {
	if t.chave != "" {
		return "Cloud Translation API v2 (com chave, em lote)"
	}
	return "endpoint público do Google Tradutor (sem chave, um por vez)"
}

// ----- Tradução -----

// TraduzirLote devolve a tradução pt-BR de cada chinês, na MESMA ordem da entrada (conveniência do
// idioma padrão). Ver TraduzirLoteParaIdioma para outro idioma-alvo.
func (t *Tradutor) TraduzirLote(chineses []string) ([]string, error) {
	return t.TraduzirLoteParaIdioma(chineses, LINGUA_ALVO)
}

// TraduzirLoteParaIdioma traduz cada chinês para linguaAlvo (código do Google: pt, en, …), na MESMA
// ordem da entrada. Com chave, resolve o lote inteiro numa única requisição oficial; sem chave, cai
// para o endpoint livre um a um. O chamador controla o tamanho do lote (a API oficial aceita até ~128).
func (t *Tradutor) TraduzirLoteParaIdioma(chineses []string, linguaAlvo string) ([]string, error) {
	if len(chineses) == 0 {
		return nil, nil
	}
	if t.chave != "" {
		return t.traduzirLoteOficial(chineses, linguaAlvo)
	}

	saida := make([]string, len(chineses))
	for i, chines := range chineses {
		traducao, err := t.traduzirUm(chines, linguaAlvo)
		if err != nil {
			return nil, err
		}
		saida[i] = traducao
	}
	return saida, nil
}

// Traduzir é a conveniência de um texto só para o idioma padrão (usada por quem traduz poucas frases).
func (t *Tradutor) Traduzir(chines string) (string, error) {
	traducoes, err := t.TraduzirLote([]string{chines})
	if err != nil {
		return "", err
	}
	return traducoes[0], nil
}

// traduzirLoteOficial manda o lote inteiro à Cloud Translation API v2, com backoff em erro de rede/429/5xx.
func (t *Tradutor) traduzirLoteOficial(chineses []string, linguaAlvo string) ([]string, error) {
	var ultimoErro error
	for tentativa := 1; tentativa <= MAXIMO_TENTATIVAS_HTTP; tentativa++ {
		if tentativa > 1 {
			time.Sleep(time.Duration(2<<(tentativa-2)) * time.Second)
		}
		traducoes, err := t.tentarLoteOficial(chineses, linguaAlvo)
		if err != nil {
			ultimoErro = err
			continue
		}
		return traducoes, nil
	}
	return nil, fmt.Errorf("falha ao traduzir lote de %d após %d tentativas: %w", len(chineses), MAXIMO_TENTATIVAS_HTTP, ultimoErro)
}

func (t *Tradutor) tentarLoteOficial(chineses []string, linguaAlvo string) ([]string, error) {
	corpo, _ := json.Marshal(map[string]any{
		"q":      chineses,
		"source": LINGUA_ORIGEM,
		"target": linguaAlvo,
		"format": "text",
	})
	req, err := http.NewRequest(http.MethodPost, URL_API_OFICIAL+"?key="+url.QueryEscape(t.chave), bytes.NewReader(corpo))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	corpoResp, status, err := t.fazer(req)
	if err != nil {
		return nil, err
	}
	if status != http.StatusOK {
		return nil, fmt.Errorf("API oficial devolveu %d: %s", status, resumo(corpoResp))
	}

	var dados struct {
		Data struct {
			Translations []struct {
				TranslatedText string `json:"translatedText"`
			} `json:"translations"`
		} `json:"data"`
	}
	if err := json.Unmarshal(corpoResp, &dados); err != nil {
		return nil, fmt.Errorf("resposta inesperada da API oficial: %s", resumo(corpoResp))
	}
	if len(dados.Data.Translations) != len(chineses) {
		return nil, fmt.Errorf("API oficial devolveu %d traduções para %d textos", len(dados.Data.Translations), len(chineses))
	}

	saida := make([]string, len(chineses))
	for i, traducao := range dados.Data.Translations {
		pt := strings.TrimSpace(traducao.TranslatedText)
		if pt == "" {
			return nil, fmt.Errorf("tradução vazia para %q", chineses[i])
		}
		saida[i] = pt
	}
	return saida, nil
}

// traduzirUm usa o endpoint livre (um texto), com backoff em erro de rede/429/5xx.
func (t *Tradutor) traduzirUm(chines, linguaAlvo string) (string, error) {
	var ultimoErro error
	for tentativa := 1; tentativa <= MAXIMO_TENTATIVAS_HTTP; tentativa++ {
		if tentativa > 1 {
			time.Sleep(time.Duration(2<<(tentativa-2)) * time.Second)
		}
		traducao, err := t.tentarUmLivre(chines, linguaAlvo)
		if err != nil {
			ultimoErro = err
			continue
		}
		traducao = strings.TrimSpace(traducao)
		if traducao == "" {
			ultimoErro = fmt.Errorf("tradução vazia para %q", chines)
			continue
		}
		return traducao, nil
	}
	return "", fmt.Errorf("falha ao traduzir %q após %d tentativas: %w", chines, MAXIMO_TENTATIVAS_HTTP, ultimoErro)
}

// tentarUmLivre chama translate_a/single, cuja resposta é um array JSON aninhado:
// [ [ [trecho_traduzido, trecho_original, ...], ... ], ... ]. Concatena todos os trechos traduzidos.
func (t *Tradutor) tentarUmLivre(chines, linguaAlvo string) (string, error) {
	parametros := url.Values{}
	parametros.Set("client", "gtx")
	parametros.Set("sl", LINGUA_ORIGEM)
	parametros.Set("tl", linguaAlvo)
	parametros.Set("dt", "t")
	parametros.Set("q", chines)

	req, err := http.NewRequest(http.MethodGet, URL_ENDPOINT_LIVRE+"?"+parametros.Encode(), nil)
	if err != nil {
		return "", err
	}
	corpoResp, status, err := t.fazer(req)
	if err != nil {
		return "", err
	}
	if status != http.StatusOK {
		return "", fmt.Errorf("endpoint livre devolveu %d: %s", status, resumo(corpoResp))
	}

	var raiz []json.RawMessage
	if err := json.Unmarshal(corpoResp, &raiz); err != nil || len(raiz) == 0 {
		return "", fmt.Errorf("resposta inesperada do endpoint livre: %s", resumo(corpoResp))
	}
	var trechos [][]json.RawMessage
	if err := json.Unmarshal(raiz[0], &trechos); err != nil {
		return "", fmt.Errorf("estrutura inesperada do endpoint livre: %s", resumo(corpoResp))
	}
	var texto strings.Builder
	for _, trecho := range trechos {
		if len(trecho) == 0 {
			continue
		}
		var parte string
		if err := json.Unmarshal(trecho[0], &parte); err == nil {
			texto.WriteString(parte)
		}
	}
	return texto.String(), nil
}

func (t *Tradutor) fazer(req *http.Request) ([]byte, int, error) {
	resp, err := t.cliente.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	corpo, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, err
	}
	if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
		return corpo, resp.StatusCode, fmt.Errorf("status %d", resp.StatusCode)
	}
	return corpo, resp.StatusCode, nil
}

// ----- Cache versionado de traduções (chinês <TAB> tradução) -----

// CarregarCache lê um cache chinês → tradução. Arquivo ausente devolve mapa vazio (não é erro: é a
// primeira execução). Linhas malformadas são ignoradas.
func CarregarCache(caminho string) map[string]string {
	cache := map[string]string{}
	dados, err := os.ReadFile(caminho)
	if err != nil {
		return cache
	}
	for _, linha := range strings.Split(string(dados), "\n") {
		campos := strings.Split(linha, "\t")
		if len(campos) == 2 && campos[0] != "" && campos[1] != "" {
			cache[campos[0]] = campos[1]
		}
	}
	return cache
}

// SalvarCache grava o cache ordenado por chinês, para o arquivo versionado ser estável entre gravações.
func SalvarCache(caminho string, cache map[string]string) error {
	chaves := make([]string, 0, len(cache))
	for k := range cache {
		chaves = append(chaves, k)
	}
	sort.Strings(chaves)

	var saida strings.Builder
	for _, k := range chaves {
		fmt.Fprintf(&saida, "%s\t%s\n", k, cache[k])
	}
	return os.WriteFile(caminho, []byte(saida.String()), 0o644)
}

// ----- Utilitários -----

var padraoCreditoChines = regexp.MustCompile(`^(.*tatoeba\.org #\d+ \([^)]*\))`)

// CreditoChines extrai de uma atribuição do Tatoeba apenas o crédito da frase CHINESA (o que sobrevive
// quando a tradução pt-BR passa a ser de máquina). Sem o padrão reconhecido, devolve a atribuição inteira.
func CreditoChines(atribuicao string) string {
	if m := padraoCreditoChines.FindStringSubmatch(atribuicao); m != nil {
		return m[1]
	}
	return atribuicao
}

func carregarChaveGoogle() string {
	if chave := obterVariavelAmbiente(VARIAVEL_CHAVE_GOOGLE); chave != "" {
		return chave
	}
	if chave := obterVariavelAmbiente("GOOGLE_API_KEY"); chave != "" {
		return chave
	}
	return ""
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

func resumo(corpo []byte) string {
	texto := strings.TrimSpace(string(corpo))
	if len(texto) > 300 {
		texto = texto[:300] + "…"
	}
	return texto
}
