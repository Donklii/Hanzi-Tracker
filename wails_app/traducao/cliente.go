package traducao

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// clienteHTTP é o cliente compartilhado para todas as chamadas à API de tradução.
// Timeout de 5s para não travar/atrasar visivelmente o ciclo de captura.
var clienteHTTP = &http.Client{Timeout: 5 * time.Second}

// respostaTraducao espelha o JSON de resposta da Cloud Translation API v2 (Basic).
type respostaTraducao struct {
	Data struct {
		Translations []struct {
			TranslatedText string `json:"translatedText"`
		} `json:"translations"`
	} `json:"data"`
	Error *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

// corpoRequisicao é o body JSON enviado à API v2 (Basic). O campo `q` é repetível: a API aceita
// várias strings numa única requisição e devolve as traduções na MESMA ordem.
type corpoRequisicao struct {
	Q      []string `json:"q"`
	Source string   `json:"source"`
	Target string   `json:"target"`
	Format string   `json:"format"`
}

// Traduzir traduz UM texto (atalho para TraduzirLote com um item — ver contrato lá).
func Traduzir(apiKey, texto, idiomaAlvo string) (string, error) {
	if texto == "" {
		return "", nil
	}

	traducoes, err := TraduzirLote(apiKey, []string{texto}, idiomaAlvo)
	if err != nil {
		return "", err
	}
	return traducoes[0], nil
}

// TraduzirLote chama a Cloud Translation API v2 (Basic) com VÁRIOS textos numa única requisição —
// a cota é cobrada por caractere, então o lote custa o mesmo que N chamadas e evita N round-trips
// sequenciais no caminho do "pop-up de tudo". Devolve as traduções na mesma ordem dos textos.
// apiKey vem da config (textbox do usuário), NUNCA embutida no binário — ver
// docs/PUBLICAR-MOTORES.md e a regra de distribuição pública do projeto (nunca custear API de
// terceiros para todo mundo com uma chave só).
//
// idiomaAlvo pode começar fixo em "pt" (interface é PT-BR); deixar como parâmetro já prepara
// para tornar configurável depois, sem forçar isso agora.
//
// Erros (401/403/429/rede) devolvem err descritivo; o chamador NÃO deve derrubar o OCR,
// só pular a tradução dessas linhas.
func TraduzirLote(apiKey string, textos []string, idiomaAlvo string) ([]string, error) {
	// Guard clause: sem chave, sem tradução
	if apiKey == "" {
		return nil, fmt.Errorf("API key de tradução não configurada")
	}
	if len(textos) == 0 {
		return nil, nil
	}

	url := "https://translation.googleapis.com/language/translate/v2?key=" + apiKey

	corpo := corpoRequisicao{
		Q:      textos,
		Source: "zh",
		Target: idiomaAlvo,
		Format: "text",
	}
	bodyJSON, err := json.Marshal(corpo)
	if err != nil {
		return nil, fmt.Errorf("falha ao montar corpo da requisição de tradução: %w", err)
	}

	req, err := http.NewRequest("POST", url, bytes.NewReader(bodyJSON))
	if err != nil {
		return nil, fmt.Errorf("falha ao criar requisição de tradução: %w", err)
	}
	req.Header.Set("Content-Type", "application/json; charset=utf-8")

	resp, err := clienteHTTP.Do(req)
	if err != nil {
		return nil, fmt.Errorf("falha de rede ao chamar API de tradução: %w", err)
	}
	defer resp.Body.Close()

	bodyResp, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("falha ao ler resposta da API de tradução: %w", err)
	}

	var resultado respostaTraducao
	if err := json.Unmarshal(bodyResp, &resultado); err != nil {
		return nil, fmt.Errorf("falha ao decodificar resposta da API de tradução: %w", err)
	}

	// Guard clause: a API devolveu um erro explícito (chave inválida, cota estourada, etc.)
	if resultado.Error != nil {
		return nil, fmt.Errorf("API de tradução retornou erro %d: %s", resultado.Error.Code, resultado.Error.Message)
	}

	// Guard clause: resposta HTTP não-200 sem campo de erro estruturado
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API de tradução retornou HTTP %d: %s", resp.StatusCode, string(bodyResp))
	}

	// A API devolve uma tradução por texto enviado, na mesma ordem; qualquer divergência de
	// tamanho tornaria o pareamento texto↔tradução ambíguo.
	if len(resultado.Data.Translations) != len(textos) {
		return nil, fmt.Errorf("API de tradução devolveu %d traduções para %d textos", len(resultado.Data.Translations), len(textos))
	}

	traducoes := make([]string, len(textos))
	for i, t := range resultado.Data.Translations {
		traducoes[i] = t.TranslatedText
	}
	return traducoes, nil
}
