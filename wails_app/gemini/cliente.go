package gemini

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// clienteHTTP é o cliente compartilhado para todas as chamadas à API do Gemini.
// Timeout de 60s porque a geração de conteúdo com imagem pode demorar.
var clienteHTTP = &http.Client{Timeout: 60 * time.Second}

type part struct {
	Text       string      `json:"text,omitempty"`
	InlineData *inlineData `json:"inline_data,omitempty"`
}

type inlineData struct {
	MimeType string `json:"mime_type"`
	Data     string `json:"data"` // base64
}

type content struct {
	Parts []part `json:"parts"`
}

type geminiRequest struct {
	Contents []content `json:"contents"`
}

type geminiResponse struct {
	Candidates []struct {
		Content struct {
			Parts []struct {
				Text string `json:"text"`
			} `json:"parts"`
		} `json:"content"`
	} `json:"candidates"`
	Error *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

// chamarGemini faz a requisição à API do Gemini.
func chamarGemini(apiKey, modelo, textoPrompt string, imagemPng []byte) (string, error) {
	// Guard clauses iniciais
	if apiKey == "" {
		return "", fmt.Errorf("API key do Gemini não configurada")
	}
	if textoPrompt == "" {
		return "", nil
	}
	if modelo == "" {
		modelo = "gemini-2.5-flash"
	}

	url := "https://generativelanguage.googleapis.com/v1beta/models/" + modelo + ":generateContent"

	reqParts := []part{{Text: textoPrompt}}
	if imagemPng != nil {
		base64Data := base64.StdEncoding.EncodeToString(imagemPng)
		reqParts = append(reqParts, part{
			InlineData: &inlineData{
				MimeType: "image/png",
				Data:     base64Data,
			},
		})
	}

	corpo := geminiRequest{
		Contents: []content{
			{Parts: reqParts},
		},
	}

	bodyJSON, err := json.Marshal(corpo)
	if err != nil {
		return "", fmt.Errorf("falha ao montar corpo da requisição do Gemini: %w", err)
	}

	req, err := http.NewRequest("POST", url, bytes.NewReader(bodyJSON))
	if err != nil {
		return "", fmt.Errorf("falha ao criar requisição do Gemini: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-goog-api-key", apiKey) // A chave NUNCA vai na URL

	resp, err := clienteHTTP.Do(req)
	if err != nil {
		return "", fmt.Errorf("falha de rede ao chamar API do Gemini: %w", err)
	}
	defer resp.Body.Close()

	bodyResp, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("falha ao ler resposta da API do Gemini: %w", err)
	}

	var resultado geminiResponse
	if err := json.Unmarshal(bodyResp, &resultado); err != nil {
		return "", fmt.Errorf("falha ao decodificar resposta da API do Gemini: %w", err)
	}

	// Se a API devolveu um erro explícito
	if resultado.Error != nil {
		return "", fmt.Errorf("API do Gemini retornou erro %d: %s", resultado.Error.Code, resultado.Error.Message)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("API do Gemini retornou HTTP %d: %s", resp.StatusCode, string(bodyResp))
	}

	if len(resultado.Candidates) == 0 {
		return "", fmt.Errorf("API do Gemini não retornou candidates")
	}

	var builder strings.Builder
	for _, p := range resultado.Candidates[0].Content.Parts {
		builder.WriteString(p.Text)
	}

	textoFinal := builder.String()
	if textoFinal == "" {
		return "", fmt.Errorf("API do Gemini retornou texto vazio")
	}

	return textoFinal, nil
}

// TraduzirLinhas traduz um conjunto de linhas via Gemini, mantendo o contexto.
func TraduzirLinhas(apiKey, modelo string, linhas []string) (map[int]string, error) {
	if len(linhas) == 0 {
		return nil, nil
	}

	var linhasNumeradas strings.Builder
	for i, linha := range linhas {
		linhasNumeradas.WriteString(fmt.Sprintf("%d: %s\n", i+1, linha))
	}

	prompt := fmt.Sprintf(`Traduza para o português do Brasil cada linha numerada abaixo, extraída por OCR de uma tela em chinês. Use as demais linhas como contexto para manter o sentido.
Responda APENAS com as traduções, uma por linha, no formato exato "N: tradução", mantendo os mesmos números, sem qualquer texto adicional.

%s`, linhasNumeradas.String())

	resposta, err := chamarGemini(apiKey, modelo, prompt, nil)
	if err != nil {
		return nil, err
	}

	re := regexp.MustCompile(`(?m)^\s*(\d+)\s*[:：]\s*(.+)$`)
	matches := re.FindAllStringSubmatch(resposta, -1)

	resultado := make(map[int]string)
	for _, match := range matches {
		if len(match) == 3 {
			num, errNum := strconv.Atoi(match[1])
			if errNum == nil && num >= 1 && num <= len(linhas) {
				resultado[num] = strings.TrimSpace(match[2])
			}
		}
	}

	return resultado, nil
}

// ResumirTela gera um resumo da tela atual baseado no OCR e, opcionalmente, na imagem.
func ResumirTela(apiKey, modelo string, linhas []string, imagemPng []byte) (string, error) {
	if len(linhas) == 0 && imagemPng == nil {
		return "", nil
	}

	var linhasNumeradas strings.Builder
	for i, linha := range linhas {
		linhasNumeradas.WriteString(fmt.Sprintf("%d: %s\n", i+1, linha))
	}

	prompt := fmt.Sprintf(`O texto numerado abaixo foi extraído por OCR de uma tela em chinês (provavelmente um jogo ou aplicativo).
Responda em português do Brasil com um resumo curto e coerente do que a tela mostra (2 a 5 frases). Responda APENAS o resumo, sem preâmbulo.

%s`, linhasNumeradas.String())

	if imagemPng != nil {
		prompt += "\nA captura de tela está anexada; use-a como contexto adicional."
	}

	resposta, err := chamarGemini(apiKey, modelo, prompt, imagemPng)
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(resposta), nil
}
