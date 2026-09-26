package nuvem

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"time"
)

// ----- Ponte de credenciais: o OAuth mora no servidor do Hanzi Tracker -----
// Antes o app carregava as credenciais OAuth (que o próprio usuário tinha de criar no Google Cloud
// Console) e falava direto com o Google. Agora quem tem as credenciais é o servidor: o app abre o
// navegador nele, recebe de volta um LACRE (o refresh token cifrado, ilegível para o app) e troca
// esse lacre por um access token curto sempre que precisa mexer no Drive.
//
// O que continua igual: os bytes do banco vão do app DIRETO para a API do Drive (drive.go), no
// Drive da conta do próprio usuário. O servidor nunca vê o progresso.db.

// urlServidorPadrao é o endereço de produção da ponte, embutido no app.
const urlServidorPadrao = "https://hanzi-tracker.fellasdev.com"

// variavelServidor aponta o app para outra ponte durante o desenvolvimento (ex.: http://localhost:8080).
const variavelServidor = "HANZI_TRACKER_SERVIDOR"

// Rotas da ponte (espelham os caminhos de ponte.CaminhoX no servidor).
const (
	rotaIniciar = "/api/auth/iniciar"
	rotaAcesso  = "/api/auth/acesso"
	rotaRevogar = "/api/auth/revogar"
)

// codigoRevogado é o que a ponte devolve quando a conexão morreu de vez (acesso retirado no Google
// ou lacre inválido): o app apaga a conexão salva em vez de insistir.
const codigoRevogado = "revogado"

// margemRenovacaoToken é a antecedência com que o access token é trocado antes de expirar.
const margemRenovacaoToken = time.Minute

// tempoLimiteAutorizacao é quanto o app espera o usuário concluir o consentimento no navegador.
const tempoLimiteAutorizacao = 5 * time.Minute

// bytesNonce é o tamanho do valor sorteado que amarra a volta do navegador a esta tentativa.
const bytesNonce = 16

// clienteHTTP atende as chamadas curtas à ponte e à API de metadados do Drive.
var clienteHTTP = &http.Client{Timeout: 30 * time.Second}

// paginaRetorno é o que o navegador mostra quando o consentimento termina.
const paginaRetorno = `<!DOCTYPE html><html lang="pt-BR"><meta charset="utf-8"><title>Hanzi Tracker</title>
<body style="font-family: sans-serif; text-align: center; padding-top: 15vh; background: #1b2636; color: #eee">
<h2>✅ Google Drive conectado ao Hanzi Tracker</h2><p>Pode fechar esta janela e voltar ao aplicativo.</p></body></html>`

// endpoints agrupa os endereços usados pelo pacote, trocados por servidores falsos nos testes.
type endpoints struct {
	iniciar string // abre o consentimento (navegador)
	acesso  string // troca lacre por access token
	revogar string // desfaz a conexão no Google
	drive   string // API de metadados do Drive (busca, download)
	upload  string // API de upload do Drive
}

// endpointsPadrao monta os endereços a partir da base da ponte e da API pública do Drive.
func endpointsPadrao() endpoints {
	base := urlServidorPadrao
	if escolhido := os.Getenv(variavelServidor); escolhido != "" {
		base = escolhido
	}
	return endpoints{
		iniciar: base + rotaIniciar,
		acesso:  base + rotaAcesso,
		revogar: base + rotaRevogar,
		drive:   "https://www.googleapis.com/drive/v3",
		upload:  "https://www.googleapis.com/upload/drive/v3",
	}
}

// retornoAutorizacao é o que o navegador entrega de volta ao loopback do app.
type retornoAutorizacao struct {
	Lacre string
	Email string
	Erro  string
}

// respostaAcesso é o corpo devolvido por rotaAcesso.
type respostaAcesso struct {
	AccessToken      string `json:"accessToken"`
	ExpiraEmSegundos int    `json:"expiraEmSegundos"`
	Email            string `json:"email"`
}

// respostaErro é o corpo de qualquer falha da ponte.
type respostaErro struct {
	Erro   string `json:"erro"`
	Codigo string `json:"codigo"`
}

// autorizar roda o consentimento pelo navegador e devolve um estadoSalvo com o lacre da conexão.
// O app sobe um servidor HTTP temporário em 127.0.0.1:<porta aleatória>; a ponte devolve o
// navegador para lá quando o Google termina.
func (g *Gerenciador) autorizar() (*estadoSalvo, error) {
	escutador, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, fmt.Errorf("falha ao abrir a porta local da autorização: %w", err)
	}
	defer escutador.Close()

	nonce, err := aleatorioBase64(bytesNonce)
	if err != nil {
		return nil, err
	}

	retornoCh := make(chan retornoAutorizacao, 1)
	servidor := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		consulta := r.URL.Query()
		// Guard clause: chamada que não é a volta desta tentativa.
		if consulta.Get("nonce") != nonce {
			http.NotFound(w, r)
			return
		}
		if motivo := consulta.Get("erro"); motivo != "" {
			http.Error(w, "Autorização não concluída: "+motivo, http.StatusForbidden)
			retornoCh <- retornoAutorizacao{Erro: motivo}
			return
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		io.WriteString(w, paginaRetorno)
		retornoCh <- retornoAutorizacao{Lacre: consulta.Get("lacre"), Email: consulta.Get("email")}
	})}
	go servidor.Serve(escutador)
	defer servidor.Close()

	porta := escutador.Addr().(*net.TCPAddr).Port
	g.dep.AbrirNavegador(g.urls.iniciar + "?" + url.Values{
		"porta": {strconv.Itoa(porta)},
		"nonce": {nonce},
	}.Encode())

	var retorno retornoAutorizacao
	select {
	case retorno = <-retornoCh:
	case <-time.After(tempoLimiteAutorizacao):
		return nil, fmt.Errorf("tempo esgotado aguardando a autorização no navegador")
	}

	if retorno.Erro != "" {
		return nil, fmt.Errorf("a autorização não foi concluída: %s", retorno.Erro)
	}
	if retorno.Lacre == "" {
		return nil, fmt.Errorf("o servidor de sincronização não devolveu a credencial da conexão")
	}
	return &estadoSalvo{Lacre: retorno.Lacre, Email: retorno.Email}, nil
}

// tokenAcesso devolve um access token válido do Drive para `tok`, pedindo um novo à ponte quando o
// atual está por expirar (e persistindo a renovação).
func (g *Gerenciador) tokenAcesso(tok *estadoSalvo) (string, error) {
	if time.Until(tok.ExpiraEm) > margemRenovacaoToken {
		return tok.AccessToken, nil
	}

	resposta, err := g.pedirAcesso(tok.Lacre)
	if err != nil {
		return "", err
	}

	tok.AccessToken = resposta.AccessToken
	tok.ExpiraEm = time.Now().Add(time.Duration(resposta.ExpiraEmSegundos) * time.Second)
	if tok.Email == "" {
		tok.Email = resposta.Email
	}

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

// pedirAcesso troca o lacre por um access token na ponte. Um lacre recusado (acesso revogado no
// Google) apaga a conexão salva: insistir só geraria o mesmo erro para sempre.
func (g *Gerenciador) pedirAcesso(lacre string) (*respostaAcesso, error) {
	resp, err := g.postarLacre(g.urls.acesso, lacre)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		falha := lerErroDaPonte(resp)
		if falha.Codigo == codigoRevogado {
			g.esquecerConexao()
			return nil, fmt.Errorf("o acesso ao Google Drive foi revogado — conecte de novo")
		}
		return nil, fmt.Errorf("o servidor de sincronização recusou a credencial: %s", falha.Erro)
	}

	var resposta respostaAcesso
	if err := json.NewDecoder(resp.Body).Decode(&resposta); err != nil {
		return nil, fmt.Errorf("resposta inválida do servidor de sincronização: %w", err)
	}
	if resposta.AccessToken == "" {
		return nil, fmt.Errorf("o servidor de sincronização não devolveu um token de acesso")
	}
	return &resposta, nil
}

// revogarToken desfaz a conexão no Google pela ponte (melhor esforço — desconectar localmente não
// depende disso dar certo).
func (g *Gerenciador) revogarToken(tok *estadoSalvo) {
	resp, err := g.postarLacre(g.urls.revogar, tok.Lacre)
	if err != nil {
		return
	}
	resp.Body.Close()
}

// ----- Utilitários -----

// postarLacre faz o POST JSON com o lacre no corpo, formato comum às rotas de acesso e revogação.
func (g *Gerenciador) postarLacre(endereco, lacre string) (*http.Response, error) {
	corpo, err := json.Marshal(map[string]string{"lacre": lacre})
	if err != nil {
		return nil, err
	}

	resp, err := clienteHTTP.Post(endereco, "application/json", bytes.NewReader(corpo))
	if err != nil {
		return nil, fmt.Errorf("falha ao falar com o servidor de sincronização: %w", err)
	}
	return resp, nil
}

// esquecerConexao apaga a conexão salva quando ela deixa de valer.
func (g *Gerenciador) esquecerConexao() {
	g.mu.Lock()
	g.token = nil
	g.mu.Unlock()
	os.Remove(caminhoEstado())
}

// lerErroDaPonte decodifica o corpo de erro da ponte, com uma mensagem de reserva se ele vier
// ilegível (proxy no meio do caminho, por exemplo).
func lerErroDaPonte(resp *http.Response) respostaErro {
	var falha respostaErro
	if err := json.NewDecoder(io.LimitReader(resp.Body, 4096)).Decode(&falha); err != nil || falha.Erro == "" {
		falha.Erro = fmt.Sprintf("HTTP %d", resp.StatusCode)
	}
	return falha
}

// aleatorioBase64 gera `n` bytes criptográficos em base64 URL-safe.
func aleatorioBase64(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("falha ao sortear o identificador da autorização: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
