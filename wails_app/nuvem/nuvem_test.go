package nuvem

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// ----- Google falso (OAuth + Drive) -----

// googleFalso emula os endpoints usados pelo pacote: token, userinfo, busca, upload resumable,
// download e revogação. O "Drive" é um único slot de arquivo em memória.
type googleFalso struct {
	mu sync.Mutex

	arquivoId       string    // "" = nuvem vazia
	conteudo        []byte    // conteúdo do backup remoto
	modificadoEm    time.Time // modifiedTime devolvido na busca
	renovacoes      int       // POSTs de refresh_token recebidos
	envios          int       // PUTs de conteúdo recebidos
	revogacoes      int       // POSTs de revogação recebidos
	proximoNumeroId int
}

func (f *googleFalso) instalar(t *testing.T, g *Gerenciador) {
	t.Helper()

	mux := http.NewServeMux()
	mux.HandleFunc("/token", func(w http.ResponseWriter, r *http.Request) {
		r.ParseForm()
		resposta := map[string]any{"access_token": "acesso-novo", "expires_in": 3600}
		if r.Form.Get("grant_type") == "authorization_code" {
			resposta["refresh_token"] = "refresh-de-teste"
		} else {
			f.mu.Lock()
			f.renovacoes++
			f.mu.Unlock()
		}
		json.NewEncoder(w).Encode(resposta)
	})
	mux.HandleFunc("/userinfo", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]string{"email": "donk@teste.com"})
	})
	mux.HandleFunc("/revoke", func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		f.revogacoes++
		f.mu.Unlock()
	})

	// Busca (GET /drive/files) e download (GET /drive/files/{id}?alt=media).
	mux.HandleFunc("/drive/files", func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		arquivos := []map[string]string{}
		if f.arquivoId != "" {
			arquivos = append(arquivos, map[string]string{
				"id":           f.arquivoId,
				"size":         fmt.Sprint(len(f.conteudo)),
				"modifiedTime": f.modificadoEm.Format(time.RFC3339),
			})
		}
		json.NewEncoder(w).Encode(map[string]any{"files": arquivos})
	})
	mux.HandleFunc("/drive/files/", func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		if strings.TrimPrefix(r.URL.Path, "/drive/files/") != f.arquivoId {
			http.NotFound(w, r)
			return
		}
		w.Write(f.conteudo)
	})

	// Upload resumable: abertura da sessão (POST cria / PATCH atualiza) e PUT do conteúdo.
	mux.HandleFunc("/upload/files", func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		f.proximoNumeroId++
		id := fmt.Sprintf("arquivo-%d", f.proximoNumeroId)
		f.mu.Unlock()
		w.Header().Set("Location", g.urls.upload+"/sessao/"+id)
	})
	mux.HandleFunc("/upload/files/", func(w http.ResponseWriter, r *http.Request) {
		id := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/upload/files/"), "/")
		w.Header().Set("Location", g.urls.upload+"/sessao/"+id)
	})
	mux.HandleFunc("/upload/sessao/", func(w http.ResponseWriter, r *http.Request) {
		corpo := make([]byte, r.ContentLength)
		r.Body.Read(corpo)
		id := strings.TrimPrefix(r.URL.Path, "/upload/sessao/")
		f.mu.Lock()
		f.arquivoId = id
		f.conteudo = corpo
		f.modificadoEm = time.Now()
		f.envios++
		f.mu.Unlock()
		json.NewEncoder(w).Encode(map[string]string{"id": id})
	})

	servidor := httptest.NewServer(mux)
	t.Cleanup(servidor.Close)
	g.urls = endpoints{
		autorizacao: servidor.URL + "/auth",
		token:       servidor.URL + "/token",
		drive:       servidor.URL + "/drive",
		upload:      servidor.URL + "/upload",
		userinfo:    servidor.URL + "/userinfo",
		revogacao:   servidor.URL + "/revoke",
	}
}

// ----- Montagem do gerenciador de teste -----

// ambienteDeTeste isola a pasta de dados num diretório temporário (mesmo truque dos testes de
// cota, redirecionando o os.UserConfigDir das duas plataformas).
func ambienteDeTeste(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("AppData", dir)
	return dir
}

// gerenciadorDeTeste monta um Gerenciador apontando para o Google falso, com um "banco" que é um
// arquivo de texto simples e um "navegador" que autoriza sozinho (segue o redirect com o código).
func gerenciadorDeTeste(t *testing.T, falso *googleFalso) (*Gerenciador, string) {
	t.Helper()
	dir := ambienteDeTeste(t)

	caminhoBanco := filepath.Join(dir, "HanziTracker", "progresso.db")
	if err := os.MkdirAll(filepath.Dir(caminhoBanco), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(caminhoBanco, []byte("banco-local"), 0644); err != nil {
		t.Fatal(err)
	}

	g := NovoGerenciador(Dependencias{
		AbrirNavegador: func(urlConsentimento string) {
			// Faz o papel do usuário no navegador: extrai o redirect e volta com um código.
			analisada, err := url.Parse(urlConsentimento)
			if err != nil {
				t.Errorf("URL de consentimento inválida: %v", err)
				return
			}
			redirect := analisada.Query().Get("redirect_uri")
			estado := analisada.Query().Get("state")
			go http.Get(redirect + "/?state=" + url.QueryEscape(estado) + "&code=codigo-de-teste")
		},
		Credenciais:  func() (string, string) { return "cliente-de-teste", "segredo-de-teste" },
		CaminhoBanco: func() string { return caminhoBanco },
		ExportarSnapshot: func(destino string) error {
			dados, err := os.ReadFile(caminhoBanco)
			if err != nil {
				return err
			}
			return os.WriteFile(destino, dados, 0600)
		},
		SubstituirBanco: func(origem string) error {
			return os.Rename(origem, caminhoBanco)
		},
	})
	falso.instalar(t, g)
	return g, caminhoBanco
}

// ----- Primeira conexão -----

func TestConectarComNuvemVaziaEnviaOhBancoLocal(t *testing.T) {
	falso := &googleFalso{}
	g, _ := gerenciadorDeTeste(t, falso)

	info, err := g.Conectar()
	if err != nil {
		t.Fatalf("Conectar: %v", err)
	}
	if info.Estado != "conectado" {
		t.Fatalf("esperava estado conectado, veio %q", info.Estado)
	}
	if info.Email != "donk@teste.com" {
		t.Fatalf("esperava o e-mail da conta, veio %q", info.Email)
	}
	if string(falso.conteudo) != "banco-local" {
		t.Fatalf("o backup remoto deveria ser o banco local, veio %q", falso.conteudo)
	}
	if info.UltimaSincronizacao == "" {
		t.Fatal("a primeira sincronização deveria ficar registrada")
	}
}

func TestConectarComBackupRemotoFicaEmConflito(t *testing.T) {
	falso := &googleFalso{arquivoId: "arquivo-antigo", conteudo: []byte("banco-da-nuvem"), modificadoEm: time.Now()}
	g, _ := gerenciadorDeTeste(t, falso)

	info, err := g.Conectar()
	if err != nil {
		t.Fatalf("Conectar: %v", err)
	}
	if info.Estado != "conflito" {
		t.Fatalf("esperava estado conflito, veio %q", info.Estado)
	}
	if falso.envios != 0 {
		t.Fatalf("nada deveria ser enviado antes da escolha do usuário; houve %d envios", falso.envios)
	}
	if info.RemotoBytes != int64(len("banco-da-nuvem")) {
		t.Fatalf("esperava o tamanho do backup remoto na UI, veio %d", info.RemotoBytes)
	}
}

func TestResolverConflitoManterLocalSobrescreveAhNuvem(t *testing.T) {
	falso := &googleFalso{arquivoId: "arquivo-antigo", conteudo: []byte("banco-da-nuvem"), modificadoEm: time.Now()}
	g, _ := gerenciadorDeTeste(t, falso)
	if _, err := g.Conectar(); err != nil {
		t.Fatal(err)
	}

	info, err := g.ResolverConflito(EscolhaManterLocal)
	if err != nil {
		t.Fatalf("ResolverConflito: %v", err)
	}
	if info.Estado != "conectado" {
		t.Fatalf("esperava estado conectado, veio %q", info.Estado)
	}
	if string(falso.conteudo) != "banco-local" {
		t.Fatalf("a nuvem deveria ter sido sobrescrita pelo banco local, veio %q", falso.conteudo)
	}
	// A atualização deve reaproveitar o arquivo remoto existente, não criar um segundo.
	if falso.arquivoId != "arquivo-antigo" {
		t.Fatalf("esperava atualizar o arquivo remoto existente, criou %q", falso.arquivoId)
	}
}

func TestResolverConflitoUsarNuvemSobrescreveOhBancoLocal(t *testing.T) {
	falso := &googleFalso{arquivoId: "arquivo-antigo", conteudo: []byte("banco-da-nuvem"), modificadoEm: time.Now()}
	g, caminhoBanco := gerenciadorDeTeste(t, falso)
	if _, err := g.Conectar(); err != nil {
		t.Fatal(err)
	}

	info, err := g.ResolverConflito(EscolhaUsarNuvem)
	if err != nil {
		t.Fatalf("ResolverConflito: %v", err)
	}
	if info.Estado != "conectado" {
		t.Fatalf("esperava estado conectado, veio %q", info.Estado)
	}
	dados, err := os.ReadFile(caminhoBanco)
	if err != nil {
		t.Fatal(err)
	}
	if string(dados) != "banco-da-nuvem" {
		t.Fatalf("o banco local deveria ter virado o da nuvem, veio %q", dados)
	}
	if falso.envios != 0 {
		t.Fatalf("usar a nuvem não deveria enviar nada; houve %d envios", falso.envios)
	}
}

func TestResolverConflitoSemConflitoPendenteFalha(t *testing.T) {
	falso := &googleFalso{}
	g, _ := gerenciadorDeTeste(t, falso)

	if _, err := g.ResolverConflito(EscolhaManterLocal); err == nil {
		t.Fatal("esperava erro ao resolver conflito sem conflito pendente")
	}
}

// ----- Sincronização contínua -----

func TestSincronizarSeMudouEnviaSoQuandoOhBancoMudar(t *testing.T) {
	falso := &googleFalso{}
	g, caminhoBanco := gerenciadorDeTeste(t, falso)
	if _, err := g.Conectar(); err != nil {
		t.Fatal(err)
	}
	enviosAposConectar := falso.envios

	// Banco intocado desde a conexão: nada a fazer.
	g.SincronizarSeMudou()
	if falso.envios != enviosAposConectar {
		t.Fatalf("banco não mudou, mas houve envio (%d → %d)", enviosAposConectar, falso.envios)
	}

	// Banco alterado (mtime no futuro garante diferença mesmo em relógio de baixa resolução).
	os.WriteFile(caminhoBanco, []byte("banco-local-v2"), 0644)
	depois := time.Now().Add(2 * time.Second)
	os.Chtimes(caminhoBanco, depois, depois)

	g.SincronizarSeMudou()
	if falso.envios != enviosAposConectar+1 {
		t.Fatalf("esperava 1 envio novo após a mudança, houve %d", falso.envios-enviosAposConectar)
	}
	if string(falso.conteudo) != "banco-local-v2" {
		t.Fatalf("a nuvem deveria ter o banco novo, veio %q", falso.conteudo)
	}
}

func TestSincronizarComConflitoPendenteFalha(t *testing.T) {
	falso := &googleFalso{arquivoId: "arquivo-antigo", conteudo: []byte("banco-da-nuvem"), modificadoEm: time.Now()}
	g, _ := gerenciadorDeTeste(t, falso)
	if _, err := g.Conectar(); err != nil {
		t.Fatal(err)
	}

	if _, err := g.Sincronizar(); err == nil {
		t.Fatal("esperava erro ao sincronizar com o conflito da primeira conexão pendente")
	}
}

// ----- Token e persistência -----

func TestTokenExpiradoEhRenovadoAntesDeUsar(t *testing.T) {
	falso := &googleFalso{}
	g, _ := gerenciadorDeTeste(t, falso)
	if _, err := g.Conectar(); err != nil {
		t.Fatal(err)
	}

	// Envelhece o token à força e sincroniza de novo: a renovação deve acontecer sozinha.
	g.mu.Lock()
	g.token.ExpiraEm = time.Now().Add(-time.Hour)
	g.mu.Unlock()

	if _, err := g.Sincronizar(); err != nil {
		t.Fatalf("Sincronizar com token vencido: %v", err)
	}
	if falso.renovacoes != 1 {
		t.Fatalf("esperava exatamente 1 renovação de token, houve %d", falso.renovacoes)
	}
}

func TestConexaoSobreviveAhReabertura(t *testing.T) {
	falso := &googleFalso{}
	g, caminhoBanco := gerenciadorDeTeste(t, falso)
	if _, err := g.Conectar(); err != nil {
		t.Fatal(err)
	}

	// "Reabre o app": um gerenciador novo lendo a mesma pasta de dados.
	reaberto := NovoGerenciador(g.dep)
	reaberto.urls = g.urls
	info := reaberto.Info()
	if info.Estado != "conectado" {
		t.Fatalf("esperava continuar conectado após reabrir, veio %q", info.Estado)
	}
	if info.Email != "donk@teste.com" {
		t.Fatalf("esperava o e-mail persistido, veio %q", info.Email)
	}

	// E a sincronização continua funcionando sem novo consentimento.
	os.WriteFile(caminhoBanco, []byte("banco-local-v2"), 0644)
	depois := time.Now().Add(2 * time.Second)
	os.Chtimes(caminhoBanco, depois, depois)
	reaberto.SincronizarSeMudou()
	if string(falso.conteudo) != "banco-local-v2" {
		t.Fatalf("a sincronização após reabrir deveria ter enviado o banco novo, veio %q", falso.conteudo)
	}
}

func TestDesconectarEsqueceAhConexaoEhRevogaOhToken(t *testing.T) {
	falso := &googleFalso{}
	g, _ := gerenciadorDeTeste(t, falso)
	if _, err := g.Conectar(); err != nil {
		t.Fatal(err)
	}

	info, err := g.Desconectar()
	if err != nil {
		t.Fatalf("Desconectar: %v", err)
	}
	if info.Estado != "desconectado" {
		t.Fatalf("esperava estado desconectado, veio %q", info.Estado)
	}
	if falso.revogacoes != 1 {
		t.Fatalf("esperava 1 revogação no Google, houve %d", falso.revogacoes)
	}
	if _, err := os.Stat(caminhoEstado()); !os.IsNotExist(err) {
		t.Fatal("o arquivo de estado deveria ter sido apagado")
	}
	// O backup na nuvem é preservado (é o backup do usuário, não um dado do app).
	if falso.arquivoId == "" {
		t.Fatal("desconectar não deveria apagar o backup remoto")
	}
}

func TestSemCredenciaisFicaNaoConfigurado(t *testing.T) {
	falso := &googleFalso{}
	g, _ := gerenciadorDeTeste(t, falso)
	// Usuário ainda não colou as credenciais na UI.
	g.dep.Credenciais = func() (string, string) { return "", "" }

	if info := g.Info(); info.Estado != "nao_configurado" {
		t.Fatalf("esperava nao_configurado sem credenciais, veio %q", info.Estado)
	}
	if _, err := g.Conectar(); err == nil {
		t.Fatal("Conectar sem credenciais deveria falhar")
	}
}

func TestCredenciaisDaUiChegamNaAutorizacao(t *testing.T) {
	falso := &googleFalso{}
	g, _ := gerenciadorDeTeste(t, falso)

	// Captura a URL de consentimento para conferir o client_id usado.
	var urlVista string
	abrirOriginal := g.dep.AbrirNavegador
	g.dep.AbrirNavegador = func(u string) {
		urlVista = u
		abrirOriginal(u)
	}

	if _, err := g.Conectar(); err != nil {
		t.Fatal(err)
	}
	analisada, err := url.Parse(urlVista)
	if err != nil {
		t.Fatal(err)
	}
	if id := analisada.Query().Get("client_id"); id != "cliente-de-teste" {
		t.Fatalf("esperava o client_id fornecido pela UI na autorização, veio %q", id)
	}
}
