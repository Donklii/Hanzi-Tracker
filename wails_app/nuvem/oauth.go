package nuvem

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

// ----- OAuth 2.0 de app instalado (loopback + PKCE) -----
// Fluxo padrão do Google para apps desktop: o app sobe um servidor HTTP temporário em
// 127.0.0.1:<porta aleatória>, abre a tela de consentimento no navegador do usuário e recebe o
// código de autorização no redirect de volta. O PKCE amarra o código a esta execução do app.

// endpoints agrupa as URLs do Google, trocadas por servidores falsos nos testes.
type endpoints struct {
	autorizacao string // tela de consentimento (navegador)
	token       string // troca/renovação de tokens
	drive       string // API de metadados do Drive (busca, download)
	upload      string // API de upload do Drive
	userinfo    string // e-mail da conta conectada
	revogacao   string // revogação do token ao desconectar
}

var endpointsGoogle = endpoints{
	autorizacao: "https://accounts.google.com/o/oauth2/v2/auth",
	token:       "https://oauth2.googleapis.com/token",
	drive:       "https://www.googleapis.com/drive/v3",
	upload:      "https://www.googleapis.com/upload/drive/v3",
	userinfo:    "https://openidconnect.googleapis.com/v1/userinfo",
	revogacao:   "https://oauth2.googleapis.com/revoke",
}

// escopos: drive.file limita o app aos arquivos QUE ELE criou; userinfo.email só mostra na UI
// qual conta está conectada.
const escoposOAuth = "https://www.googleapis.com/auth/drive.file https://www.googleapis.com/auth/userinfo.email"

// tempoLimiteAutorizacao é quanto o app espera o usuário concluir o consentimento no navegador.
const tempoLimiteAutorizacao = 3 * time.Minute

// clienteHTTP atende as chamadas curtas (tokens, busca, userinfo).
var clienteHTTP = &http.Client{Timeout: 30 * time.Second}

// paginaRetorno é o que o navegador mostra depois do consentimento.
const paginaRetorno = `<!DOCTYPE html><html lang="pt-BR"><meta charset="utf-8"><title>Hanzi Tracker</title>
<body style="font-family: sans-serif; text-align: center; padding-top: 15vh; background: #1b2636; color: #eee">
<h2>✅ Google Drive conectado ao Hanzi Tracker</h2><p>Pode fechar esta janela e voltar ao aplicativo.</p></body></html>`

// respostaToken é o JSON dos endpoints de token do Google (troca de código e renovação).
type respostaToken struct {
	AccessToken      string `json:"access_token"`
	RefreshToken     string `json:"refresh_token"`
	ExpiraEmSegundos int    `json:"expires_in"`
	Erro             string `json:"error"`
	ErroDescricao    string `json:"error_description"`
}

// autorizar roda o fluxo completo: loopback + navegador + troca do código + e-mail da conta.
// Devolve um estadoSalvo pronto (ainda sem os metadados de sincronização).
func (g *Gerenciador) autorizar() (*estadoSalvo, error) {
	clientId, clientSecret := g.credenciais()

	escutador, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, fmt.Errorf("falha ao abrir a porta local do OAuth: %w", err)
	}
	defer escutador.Close()
	redirect := "http://" + escutador.Addr().String()

	verificador := aleatorioBase64(32)
	estado := aleatorioBase64(16)
	somaVerificador := sha256.Sum256([]byte(verificador))
	desafio := base64.RawURLEncoding.EncodeToString(somaVerificador[:])

	codigoCh := make(chan string, 1)
	erroCh := make(chan error, 1)
	servidor := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		consulta := r.URL.Query()
		// Guard clauses: negado pelo usuário, ou redirect que não é o nosso (state diferente).
		if e := consulta.Get("error"); e != "" {
			http.Error(w, "Autorização negada.", http.StatusForbidden)
			erroCh <- fmt.Errorf("autorização negada no navegador: %s", e)
			return
		}
		if consulta.Get("code") == "" || consulta.Get("state") != estado {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		io.WriteString(w, paginaRetorno)
		codigoCh <- consulta.Get("code")
	})}
	go servidor.Serve(escutador)
	defer servidor.Close()

	g.dep.AbrirNavegador(g.urls.autorizacao + "?" + url.Values{
		"client_id":             {clientId},
		"redirect_uri":          {redirect},
		"response_type":         {"code"},
		"scope":                 {escoposOAuth},
		"state":                 {estado},
		"code_challenge":        {desafio},
		"code_challenge_method": {"S256"},
		"access_type":           {"offline"}, // pede refresh token (o app sincroniza sem navegador)
		"prompt":                {"consent"}, // garante refresh token mesmo em reconexões
	}.Encode())

	var codigo string
	select {
	case codigo = <-codigoCh:
	case err := <-erroCh:
		return nil, err
	case <-time.After(tempoLimiteAutorizacao):
		return nil, fmt.Errorf("tempo esgotado aguardando a autorização no navegador")
	}

	resposta, err := g.chamarEndpointToken(url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {codigo},
		"client_id":     {clientId},
		"client_secret": {clientSecret},
		"redirect_uri":  {redirect},
		"code_verifier": {verificador},
	})
	if err != nil {
		return nil, fmt.Errorf("falha ao trocar o código de autorização: %w", err)
	}
	if resposta.RefreshToken == "" {
		return nil, fmt.Errorf("o Google não devolveu um refresh token — remova o acesso do app em myaccount.google.com/permissions e conecte de novo")
	}

	tok := &estadoSalvo{
		RefreshToken: resposta.RefreshToken,
		AccessToken:  resposta.AccessToken,
		ExpiraEm:     time.Now().Add(time.Duration(resposta.ExpiraEmSegundos) * time.Second),
	}
	tok.Email, _ = g.buscarEmail(tok.AccessToken) // só informativo na UI; falha não impede conectar
	return tok, nil
}

// tokenAcesso devolve um access token válido para `tok`, renovando com o refresh token quando
// falta menos de um minuto para expirar (e persistindo a renovação).
func (g *Gerenciador) tokenAcesso(tok *estadoSalvo) (string, error) {
	if time.Until(tok.ExpiraEm) > time.Minute {
		return tok.AccessToken, nil
	}

	clientId, clientSecret := g.credenciais()
	resposta, err := g.chamarEndpointToken(url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {tok.RefreshToken},
		"client_id":     {clientId},
		"client_secret": {clientSecret},
	})
	if err != nil {
		// invalid_grant = acesso revogado/expirado de vez: a conexão salva não vale mais nada.
		if strings.Contains(err.Error(), "invalid_grant") {
			g.mu.Lock()
			g.token = nil
			g.mu.Unlock()
			os.Remove(caminhoEstado())
			return "", fmt.Errorf("o acesso ao Google Drive foi revogado — conecte de novo")
		}
		return "", fmt.Errorf("falha ao renovar o acesso ao Google: %w", err)
	}

	tok.AccessToken = resposta.AccessToken
	tok.ExpiraEm = time.Now().Add(time.Duration(resposta.ExpiraEmSegundos) * time.Second)

	// Persiste a renovação por baixo do estado corrente, sem mexer nos metadados de sincronização.
	g.mu.Lock()
	if g.token != nil {
		g.token.AccessToken = tok.AccessToken
		g.token.ExpiraEm = tok.ExpiraEm
		salvarEstado(g.token)
	}
	g.mu.Unlock()
	return tok.AccessToken, nil
}

// chamarEndpointToken faz um POST de formulário ao endpoint de token e decodifica a resposta.
func (g *Gerenciador) chamarEndpointToken(formulario url.Values) (*respostaToken, error) {
	resp, err := clienteHTTP.PostForm(g.urls.token, formulario)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var resposta respostaToken
	if err := json.NewDecoder(resp.Body).Decode(&resposta); err != nil {
		return nil, fmt.Errorf("resposta inválida do endpoint de token: %w", err)
	}
	if resposta.Erro != "" {
		return nil, fmt.Errorf("%s (%s)", resposta.Erro, resposta.ErroDescricao)
	}
	if resposta.AccessToken == "" {
		return nil, fmt.Errorf("endpoint de token não devolveu um access token (HTTP %d)", resp.StatusCode)
	}
	return &resposta, nil
}

// buscarEmail consulta o e-mail da conta conectada (mostrado na UI).
func (g *Gerenciador) buscarEmail(accessToken string) (string, error) {
	req, err := http.NewRequest("GET", g.urls.userinfo, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := clienteHTTP.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var corpo struct {
		Email string `json:"email"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&corpo); err != nil {
		return "", err
	}
	return corpo.Email, nil
}

// revogarToken invalida o refresh token no Google (melhor esforço — desconectar local não depende).
func (g *Gerenciador) revogarToken(tok *estadoSalvo) {
	resp, err := clienteHTTP.PostForm(g.urls.revogacao, url.Values{"token": {tok.RefreshToken}})
	if err != nil {
		return
	}
	resp.Body.Close()
}

// aleatorioBase64 gera `n` bytes criptográficos em base64 URL-safe (state e verificador PKCE).
func aleatorioBase64(n int) string {
	b := make([]byte, n)
	rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}
