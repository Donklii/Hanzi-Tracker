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

// ----- Ponte falsa + Drive falso -----

// lacreDeTeste é a credencial que a ponte falsa entrega ao app no fim do consentimento.
const lacreDeTeste = "lacre-de-teste"

// lacreRevogado faz a ponte falsa responder como se o usuário tivesse tirado o acesso no Google.
const lacreRevogado = "lacre-revogado"

// configuracoesLocais é o conteúdo do configuracoes.json de teste (JSON válido, como o real).
const configuracoesLocais = `{"idiomaTraducao":"pt-BR"}`

// arquivoFalso é uma entrada do Drive falso — arquivo ou pasta, conforme o mimeType.
type arquivoFalso struct {
	Id           string
	Nome         string
	PastaId      string
	MimeType     string
	Conteudo     []byte
	ModificadoEm time.Time
}

// servicosFalsos emula os dois lados com que o pacote conversa: a ponte do Hanzi Tracker (que troca
// lacre por access token) e a API do Drive, aqui um sistema de arquivos em memória com pastas.
type servicosFalsos struct {
	mu sync.Mutex

	arquivos        map[string]*arquivoFalso // id → arquivo/pasta
	proximoNumeroId int
	pastasCriadas   int // POSTs de criação de pasta recebidos
	renovacoes      int // pedidos de access token feitos à ponte
	envios          int // PUTs de conteúdo recebidos pelo Drive
	revogacoes      int // pedidos de revogação feitos à ponte
}

func novosServicosFalsos() *servicosFalsos {
	return &servicosFalsos{arquivos: map[string]*arquivoFalso{}}
}

func (f *servicosFalsos) instalar(t *testing.T, g *Gerenciador) {
	t.Helper()

	mux := http.NewServeMux()
	f.instalarPonte(mux)
	f.instalarDrive(mux, g)

	servidor := httptest.NewServer(mux)
	t.Cleanup(servidor.Close)
	g.urls = endpoints{
		iniciar: servidor.URL + "/auth/iniciar",
		acesso:  servidor.URL + "/auth/acesso",
		revogar: servidor.URL + "/auth/revogar",
		drive:   servidor.URL + "/drive",
		upload:  servidor.URL + "/upload",
	}
}

// instalarPonte responde pelo servidor central: consentimento, troca de lacre e revogação.
func (f *servicosFalsos) instalarPonte(mux *http.ServeMux) {
	// Iniciar faz o papel do servidor real: manda o navegador de volta ao loopback do app com o
	// lacre. O consentimento no Google acontece "dentro" da ponte e não aparece para o app.
	mux.HandleFunc("/auth/iniciar", func(w http.ResponseWriter, r *http.Request) {
		destino := fmt.Sprintf("http://127.0.0.1:%s/?%s", r.URL.Query().Get("porta"), url.Values{
			"nonce": {r.URL.Query().Get("nonce")},
			"lacre": {lacreDeTeste},
			"email": {"donk@teste.com"},
		}.Encode())
		http.Redirect(w, r, destino, http.StatusFound)
	})
	mux.HandleFunc("/auth/acesso", func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		f.renovacoes++
		f.mu.Unlock()

		var pedido struct {
			Lacre string `json:"lacre"`
		}
		json.NewDecoder(r.Body).Decode(&pedido)
		if pedido.Lacre == lacreRevogado {
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]string{"erro": "acesso revogado", "codigo": codigoRevogado})
			return
		}
		json.NewEncoder(w).Encode(map[string]any{
			"accessToken": "acesso-novo", "expiraEmSegundos": 3600, "email": "donk@teste.com",
		})
	})
	mux.HandleFunc("/auth/revogar", func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		f.revogacoes++
		f.mu.Unlock()
		w.WriteHeader(http.StatusNoContent)
	})
}

// instalarDrive responde pela API do Drive: listar, criar pasta, upload resumable e download.
func (f *servicosFalsos) instalarDrive(mux *http.ServeMux, g *Gerenciador) {
	// Listagem (GET) e criação de pasta (POST) compartilham a rota /drive/files.
	mux.HandleFunc("/drive/files", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			f.criarPasta(w, r)
			return
		}
		f.listar(w, r)
	})
	mux.HandleFunc("/drive/files/", func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		arquivo := f.arquivos[strings.TrimPrefix(r.URL.Path, "/drive/files/")]
		if arquivo == nil {
			http.NotFound(w, r)
			return
		}
		w.Write(arquivo.Conteudo)
	})

	// Upload resumable: abertura da sessão (POST cria / PATCH atualiza) e PUT do conteúdo.
	mux.HandleFunc("/upload/files", func(w http.ResponseWriter, r *http.Request) {
		var metadados struct {
			Name    string   `json:"name"`
			Parents []string `json:"parents"`
		}
		json.NewDecoder(r.Body).Decode(&metadados)

		f.mu.Lock()
		pastaId := ""
		if len(metadados.Parents) > 0 {
			pastaId = metadados.Parents[0]
		}
		// Pasta de destino inexistente: é o 404 que o Drive real devolve quando o usuário a apagou.
		if pastaId != "" && f.arquivos[pastaId] == nil {
			f.mu.Unlock()
			http.NotFound(w, r)
			return
		}
		id := f.registrar(&arquivoFalso{Nome: metadados.Name, PastaId: pastaId})
		f.mu.Unlock()

		w.Header().Set("Location", g.urls.upload+"/sessao/"+id)
	})
	mux.HandleFunc("/upload/files/", func(w http.ResponseWriter, r *http.Request) {
		id := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/upload/files/"), "/")

		f.mu.Lock()
		existe := f.arquivos[id] != nil
		f.mu.Unlock()
		// Sobrescrever um arquivo que já não está no Drive também é 404.
		if !existe {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Location", g.urls.upload+"/sessao/"+id)
	})
	mux.HandleFunc("/upload/sessao/", func(w http.ResponseWriter, r *http.Request) {
		corpo := make([]byte, r.ContentLength)
		r.Body.Read(corpo)
		id := strings.TrimPrefix(r.URL.Path, "/upload/sessao/")

		f.mu.Lock()
		arquivo := f.arquivos[id]
		if arquivo == nil {
			f.mu.Unlock()
			http.NotFound(w, r)
			return
		}
		arquivo.Conteudo = corpo
		arquivo.ModificadoEm = time.Now()
		f.envios++
		f.mu.Unlock()

		json.NewEncoder(w).Encode(map[string]string{"id": id})
	})
}

// criarPasta atende o POST /drive/files com mimeType de pasta.
func (f *servicosFalsos) criarPasta(w http.ResponseWriter, r *http.Request) {
	var metadados struct {
		Name     string `json:"name"`
		MimeType string `json:"mimeType"`
	}
	json.NewDecoder(r.Body).Decode(&metadados)

	f.mu.Lock()
	f.pastasCriadas++
	id := f.registrar(&arquivoFalso{Nome: metadados.Name, MimeType: metadados.MimeType})
	f.mu.Unlock()

	json.NewEncoder(w).Encode(map[string]string{"id": id})
}

// listar responde o GET /drive/files entendendo as duas consultas que o pacote monta: pasta por
// nome+mimeType e filhos de uma pasta.
func (f *servicosFalsos) listar(w http.ResponseWriter, r *http.Request) {
	consulta := r.URL.Query().Get("q")

	f.mu.Lock()
	defer f.mu.Unlock()

	linhas := []map[string]string{}
	for _, arquivo := range f.arquivos {
		if !correspondeAhConsulta(arquivo, consulta) {
			continue
		}
		linhas = append(linhas, map[string]string{
			"id":           arquivo.Id,
			"name":         arquivo.Nome,
			"size":         fmt.Sprint(len(arquivo.Conteudo)),
			"modifiedTime": arquivo.ModificadoEm.Format(time.RFC3339),
		})
	}
	json.NewEncoder(w).Encode(map[string]any{"files": linhas})
}

// ----- Auxiliares do Drive falso -----

// registrar guarda o arquivo com um id novo (o chamador já segura o lock).
func (f *servicosFalsos) registrar(arquivo *arquivoFalso) string {
	f.proximoNumeroId++
	arquivo.Id = fmt.Sprintf("item-%d", f.proximoNumeroId)
	arquivo.ModificadoEm = time.Now()
	f.arquivos[arquivo.Id] = arquivo
	return arquivo.Id
}

// correspondeAhConsulta implementa só os predicados que o pacote usa, por substring.
func correspondeAhConsulta(arquivo *arquivoFalso, consulta string) bool {
	if strings.Contains(consulta, "mimeType = '"+mimeTypePasta+"'") {
		return arquivo.MimeType == mimeTypePasta && strings.Contains(consulta, "name = '"+arquivo.Nome+"'")
	}
	if arquivo.PastaId != "" && strings.Contains(consulta, "'"+arquivo.PastaId+"' in parents") {
		return true
	}
	return false
}

// pastaDoBackup devolve a pasta dedicada criada no Drive falso (nil se não existir).
func (f *servicosFalsos) pastaDoBackup() *arquivoFalso {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, arquivo := range f.arquivos {
		if arquivo.MimeType == mimeTypePasta && arquivo.Nome == nomePastaRemota {
			return arquivo
		}
	}
	return nil
}

// arquivoDoBackup devolve, por nome, o arquivo guardado dentro da pasta dedicada (nil se ausente).
func (f *servicosFalsos) arquivoDoBackup(nome string) *arquivoFalso {
	pasta := f.pastaDoBackup()
	if pasta == nil {
		return nil
	}

	f.mu.Lock()
	defer f.mu.Unlock()
	for _, arquivo := range f.arquivos {
		if arquivo.PastaId == pasta.Id && arquivo.Nome == nome {
			return arquivo
		}
	}
	return nil
}

// contarNoBackup diz quantos arquivos com aquele nome existem dentro da pasta dedicada — é o que
// denuncia duplicatas criadas por uma recuperação mal feita.
func (f *servicosFalsos) contarNoBackup(nome string) int {
	pasta := f.pastaDoBackup()
	if pasta == nil {
		return 0
	}

	f.mu.Lock()
	defer f.mu.Unlock()
	total := 0
	for _, arquivo := range f.arquivos {
		if arquivo.PastaId == pasta.Id && arquivo.Nome == nome {
			total++
		}
	}
	return total
}

// apagarPastaDoBackup simula o usuário apagando a pasta "Hanzi Tracker" do Drive dele, com tudo
// que havia dentro.
func (f *servicosFalsos) apagarPastaDoBackup() {
	pasta := f.pastaDoBackup()
	if pasta == nil {
		return
	}

	f.mu.Lock()
	defer f.mu.Unlock()
	for id, arquivo := range f.arquivos {
		if id == pasta.Id || arquivo.PastaId == pasta.Id {
			delete(f.arquivos, id)
		}
	}
}

// apagarDoBackup simula o usuário apagando um único arquivo de dentro da pasta dedicada.
func (f *servicosFalsos) apagarDoBackup(nome string) {
	arquivo := f.arquivoDoBackup(nome)
	if arquivo == nil {
		return
	}

	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.arquivos, arquivo.Id)
}

// semearBackup põe no Drive falso uma pasta com banco (e, opcionalmente, configurações), como se
// outra máquina já tivesse sincronizado.
func (f *servicosFalsos) semearBackup(banco, configuracoes string) {
	f.mu.Lock()
	defer f.mu.Unlock()

	pasta := &arquivoFalso{Nome: nomePastaRemota, MimeType: mimeTypePasta}
	f.registrar(pasta)
	f.registrar(&arquivoFalso{Nome: nomeArquivoBanco, PastaId: pasta.Id, Conteudo: []byte(banco)})
	if configuracoes != "" {
		f.registrar(&arquivoFalso{Nome: nomeArquivoConfiguracoes, PastaId: pasta.Id, Conteudo: []byte(configuracoes)})
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

// caminhosLocais são os dois arquivos que a sincronização espelha, no ambiente de teste.
type caminhosLocais struct {
	Banco         string
	Configuracoes string
}

// gerenciadorDeTeste monta um Gerenciador apontando para os serviços falsos, com um "banco" que é
// um arquivo de texto simples e um "navegador" que segue o redirect da ponte sozinho.
func gerenciadorDeTeste(t *testing.T, falso *servicosFalsos) (*Gerenciador, caminhosLocais) {
	t.Helper()
	dir := ambienteDeTeste(t)

	locais := caminhosLocais{
		Banco:         filepath.Join(dir, "HanziTracker", "progresso.db"),
		Configuracoes: filepath.Join(dir, "HanziTracker", "configuracoes.json"),
	}
	if err := os.MkdirAll(filepath.Dir(locais.Banco), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(locais.Banco, []byte("banco-local"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(locais.Configuracoes, []byte(configuracoesLocais), 0644); err != nil {
		t.Fatal(err)
	}

	g := NovoGerenciador(Dependencias{
		// Faz o papel do usuário no navegador: abre a ponte, que redireciona de volta ao loopback.
		AbrirNavegador: func(enderecoPonte string) { go http.Get(enderecoPonte) },
		CaminhoBanco:   func() string { return locais.Banco },
		ExportarSnapshot: func(destino string) error {
			dados, err := os.ReadFile(locais.Banco)
			if err != nil {
				return err
			}
			return os.WriteFile(destino, dados, 0600)
		},
		SubstituirBanco: func(origem string) error {
			return os.Rename(origem, locais.Banco)
		},
		CaminhoConfiguracoes: func() string { return locais.Configuracoes },
		SubstituirConfiguracoes: func(origem string) error {
			return os.Rename(origem, locais.Configuracoes)
		},
	})
	falso.instalar(t, g)
	return g, locais
}

// ----- Primeira conexão -----

func TestConectarComNuvemVaziaEnviaOhsDadosLocais(t *testing.T) {
	falso := novosServicosFalsos()
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
	if info.UltimaSincronizacao == "" {
		t.Fatal("a primeira sincronização deveria ficar registrada")
	}

	banco := falso.arquivoDoBackup(nomeArquivoBanco)
	if banco == nil {
		t.Fatalf("o banco deveria estar dentro da pasta %q", nomePastaRemota)
	}
	if string(banco.Conteudo) != "banco-local" {
		t.Fatalf("o backup remoto deveria ser o banco local, veio %q", banco.Conteudo)
	}
}

func TestPastaDedicadaEhCriadaNaPrimeiraConexao(t *testing.T) {
	falso := novosServicosFalsos()
	g, _ := gerenciadorDeTeste(t, falso)
	if _, err := g.Conectar(); err != nil {
		t.Fatal(err)
	}

	pasta := falso.pastaDoBackup()
	if pasta == nil {
		t.Fatalf("a pasta %q deveria ter sido criada no Drive", nomePastaRemota)
	}
	if pasta.PastaId != "" {
		t.Fatal("a pasta do backup deve ficar na raiz do Drive, para o usuário achar")
	}
	if falso.pastasCriadas != 1 {
		t.Fatalf("esperava exatamente 1 pasta criada, houve %d", falso.pastasCriadas)
	}
}

func TestPastaExistenteEhReaproveitadaEmVezDeDuplicada(t *testing.T) {
	falso := novosServicosFalsos()
	falso.semearBackup("banco-da-nuvem", configuracoesLocais)
	g, _ := gerenciadorDeTeste(t, falso)

	if _, err := g.Conectar(); err != nil {
		t.Fatal(err)
	}
	if falso.pastasCriadas != 0 {
		t.Fatalf("a pasta já existia; não deveria ter sido criada outra (%d criadas)", falso.pastasCriadas)
	}
}

func TestConfiguracoesSobemJuntoComOhBanco(t *testing.T) {
	falso := novosServicosFalsos()
	g, _ := gerenciadorDeTeste(t, falso)
	if _, err := g.Conectar(); err != nil {
		t.Fatal(err)
	}

	configuracoes := falso.arquivoDoBackup(nomeArquivoConfiguracoes)
	if configuracoes == nil {
		t.Fatalf("o %s deveria ter subido junto com o banco", nomeArquivoConfiguracoes)
	}
	if string(configuracoes.Conteudo) != configuracoesLocais {
		t.Fatalf("as configurações remotas deveriam ser as locais, veio %q", configuracoes.Conteudo)
	}
}

func TestConectarComBackupRemotoFicaEmConflito(t *testing.T) {
	falso := novosServicosFalsos()
	falso.semearBackup("banco-da-nuvem", configuracoesLocais)
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
	esperado := int64(len("banco-da-nuvem") + len(configuracoesLocais))
	if info.RemotoBytes != esperado {
		t.Fatalf("esperava a soma dos dois arquivos remotos (%d), veio %d", esperado, info.RemotoBytes)
	}
}

func TestResolverConflitoManterLocalSobrescreveAhNuvem(t *testing.T) {
	falso := novosServicosFalsos()
	falso.semearBackup("banco-da-nuvem", `{"idiomaTraducao":"en"}`)
	g, _ := gerenciadorDeTeste(t, falso)
	if _, err := g.Conectar(); err != nil {
		t.Fatal(err)
	}
	idBancoAntes := falso.arquivoDoBackup(nomeArquivoBanco).Id

	info, err := g.ResolverConflito(EscolhaManterLocal)
	if err != nil {
		t.Fatalf("ResolverConflito: %v", err)
	}
	if info.Estado != "conectado" {
		t.Fatalf("esperava estado conectado, veio %q", info.Estado)
	}
	if conteudo := string(falso.arquivoDoBackup(nomeArquivoBanco).Conteudo); conteudo != "banco-local" {
		t.Fatalf("a nuvem deveria ter sido sobrescrita pelo banco local, veio %q", conteudo)
	}
	if conteudo := string(falso.arquivoDoBackup(nomeArquivoConfiguracoes).Conteudo); conteudo != configuracoesLocais {
		t.Fatalf("as configurações remotas deveriam ter sido sobrescritas, veio %q", conteudo)
	}
	// A atualização deve reaproveitar os arquivos remotos existentes, não criar segundos.
	if idDepois := falso.arquivoDoBackup(nomeArquivoBanco).Id; idDepois != idBancoAntes {
		t.Fatalf("esperava atualizar o arquivo remoto existente (%s), criou %s", idBancoAntes, idDepois)
	}
}

func TestResolverConflitoUsarNuvemRestauraBancoEhConfiguracoes(t *testing.T) {
	falso := novosServicosFalsos()
	configuracoesDaNuvem := `{"idiomaTraducao":"es"}`
	falso.semearBackup("banco-da-nuvem", configuracoesDaNuvem)
	g, locais := gerenciadorDeTeste(t, falso)
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
	if conteudo := lerArquivo(t, locais.Banco); conteudo != "banco-da-nuvem" {
		t.Fatalf("o banco local deveria ter virado o da nuvem, veio %q", conteudo)
	}
	if conteudo := lerArquivo(t, locais.Configuracoes); conteudo != configuracoesDaNuvem {
		t.Fatalf("as configurações locais deveriam ter virado as da nuvem, veio %q", conteudo)
	}
	if falso.envios != 0 {
		t.Fatalf("usar a nuvem não deveria enviar nada; houve %d envios", falso.envios)
	}
}

func TestBackupSemConfiguracoesPreservaAhsLocais(t *testing.T) {
	falso := novosServicosFalsos()
	// Backup criado por uma versão anterior: só o banco está lá.
	falso.semearBackup("banco-da-nuvem", "")
	g, locais := gerenciadorDeTeste(t, falso)
	if _, err := g.Conectar(); err != nil {
		t.Fatal(err)
	}

	if _, err := g.ResolverConflito(EscolhaUsarNuvem); err != nil {
		t.Fatalf("ResolverConflito: %v", err)
	}
	if conteudo := lerArquivo(t, locais.Configuracoes); conteudo != configuracoesLocais {
		t.Fatalf("sem configurações na nuvem, as locais deveriam continuar valendo, veio %q", conteudo)
	}
}

func TestResolverConflitoSemConflitoPendenteFalha(t *testing.T) {
	falso := novosServicosFalsos()
	g, _ := gerenciadorDeTeste(t, falso)

	if _, err := g.ResolverConflito(EscolhaManterLocal); err == nil {
		t.Fatal("esperava erro ao resolver conflito sem conflito pendente")
	}
}

// ----- Sincronização contínua -----

func TestSincronizarSeMudouEnviaSoQuandoAlgoMudar(t *testing.T) {
	falso := novosServicosFalsos()
	g, locais := gerenciadorDeTeste(t, falso)
	if _, err := g.Conectar(); err != nil {
		t.Fatal(err)
	}
	enviosAposConectar := falso.envios

	// Nada tocado desde a conexão: nada a fazer.
	g.SincronizarSeMudou()
	if falso.envios != enviosAposConectar {
		t.Fatalf("nada mudou, mas houve envio (%d → %d)", enviosAposConectar, falso.envios)
	}

	envelhecerParaOhFuturo(t, locais.Banco, []byte("banco-local-v2"))
	g.SincronizarSeMudou()
	if conteudo := string(falso.arquivoDoBackup(nomeArquivoBanco).Conteudo); conteudo != "banco-local-v2" {
		t.Fatalf("a nuvem deveria ter o banco novo, veio %q", conteudo)
	}
}

func TestMudancaSoNasConfiguracoesTambemSincroniza(t *testing.T) {
	falso := novosServicosFalsos()
	g, locais := gerenciadorDeTeste(t, falso)
	if _, err := g.Conectar(); err != nil {
		t.Fatal(err)
	}

	// O banco fica intocado: quem muda é só o configuracoes.json.
	novasConfiguracoes := `{"idiomaTraducao":"es"}`
	envelhecerParaOhFuturo(t, locais.Configuracoes, []byte(novasConfiguracoes))

	g.SincronizarSeMudou()
	if conteudo := string(falso.arquivoDoBackup(nomeArquivoConfiguracoes).Conteudo); conteudo != novasConfiguracoes {
		t.Fatalf("mudar só as configurações deveria disparar o envio, veio %q", conteudo)
	}
}

func TestConfiguracoesIlegiveisNaoSobem(t *testing.T) {
	falso := novosServicosFalsos()
	g, locais := gerenciadorDeTeste(t, falso)
	if _, err := g.Conectar(); err != nil {
		t.Fatal(err)
	}

	// JSON truncado, como se o arquivo fosse lido no meio de uma gravação do app.
	if err := os.WriteFile(locais.Configuracoes, []byte(`{"idiomaTradu`), 0644); err != nil {
		t.Fatal(err)
	}

	if _, err := g.Sincronizar(); err == nil {
		t.Fatal("esperava erro ao sincronizar com o configuracoes.json corrompido")
	}
	if conteudo := string(falso.arquivoDoBackup(nomeArquivoConfiguracoes).Conteudo); conteudo != configuracoesLocais {
		t.Fatalf("as configurações da nuvem não podiam ser contaminadas, veio %q", conteudo)
	}
	// O banco é enviado antes e não deve ser penalizado pela falha das configurações.
	if falso.arquivoDoBackup(nomeArquivoBanco) == nil {
		t.Fatal("o banco deveria ter subido mesmo com as configurações fora")
	}
}

// ----- Recuperação quando o usuário mexe no Drive por fora -----

func TestPastaApagadaNoDriveEhRecriadaNaSincronizacao(t *testing.T) {
	falso := novosServicosFalsos()
	g, locais := gerenciadorDeTeste(t, falso)
	if _, err := g.Conectar(); err != nil {
		t.Fatal(err)
	}

	// O usuário apaga a pasta inteira do Drive; o app segue com os ids antigos salvos em disco.
	falso.apagarPastaDoBackup()
	envelhecerParaOhFuturo(t, locais.Banco, []byte("banco-local-v2"))

	g.SincronizarSeMudou()

	if info := g.Info(); info.Erro != "" {
		t.Fatalf("a sincronização deveria ter se recuperado sozinha, veio o erro %q", info.Erro)
	}
	banco := falso.arquivoDoBackup(nomeArquivoBanco)
	if banco == nil {
		t.Fatal("o banco deveria ter sido reenviado para a pasta recriada")
	}
	if string(banco.Conteudo) != "banco-local-v2" {
		t.Fatalf("esperava o banco novo na pasta recriada, veio %q", banco.Conteudo)
	}
	if falso.arquivoDoBackup(nomeArquivoConfiguracoes) == nil {
		t.Fatal("as configurações também deveriam ter voltado para a pasta recriada")
	}
	if falso.pastasCriadas != 2 {
		t.Fatalf("esperava a pasta original mais a recriada (2), houve %d", falso.pastasCriadas)
	}

	// Os ids novos precisam ficar salvos: a próxima sincronização não pode recriar a pasta de novo.
	envelhecerParaOhFuturo(t, locais.Banco, []byte("banco-local-v3"))
	g.SincronizarSeMudou()
	if falso.pastasCriadas != 2 {
		t.Fatalf("a recuperação deveria ter persistido o id da pasta nova (%d criadas)", falso.pastasCriadas)
	}
	if conteudo := string(falso.arquivoDoBackup(nomeArquivoBanco).Conteudo); conteudo != "banco-local-v3" {
		t.Fatalf("a sincronização seguinte deveria ter enviado o banco novo, veio %q", conteudo)
	}
}

func TestArquivoApagadoNoDriveNaoDuplicaAhPastaNemOhsIrmaos(t *testing.T) {
	falso := novosServicosFalsos()
	g, locais := gerenciadorDeTeste(t, falso)
	if _, err := g.Conectar(); err != nil {
		t.Fatal(err)
	}

	// Só o banco é apagado: a pasta e o configuracoes.json continuam no Drive.
	falso.apagarDoBackup(nomeArquivoBanco)
	envelhecerParaOhFuturo(t, locais.Banco, []byte("banco-local-v2"))

	g.SincronizarSeMudou()

	if info := g.Info(); info.Erro != "" {
		t.Fatalf("a sincronização deveria ter se recuperado sozinha, veio o erro %q", info.Erro)
	}
	if conteudo := string(falso.arquivoDoBackup(nomeArquivoBanco).Conteudo); conteudo != "banco-local-v2" {
		t.Fatalf("o banco apagado deveria ter sido recriado com o conteúdo novo, veio %q", conteudo)
	}
	// A pasta ainda existia: recriá-la espalharia o backup em dois lugares.
	if falso.pastasCriadas != 1 {
		t.Fatalf("a pasta continuava no Drive; não deveria ter sido criada outra (%d criadas)", falso.pastasCriadas)
	}
	// E o arquivo que sobreviveu tem de ser sobrescrito no lugar, não ganhar uma cópia ao lado.
	if total := falso.contarNoBackup(nomeArquivoConfiguracoes); total != 1 {
		t.Fatalf("o configuracoes.json que sobrou deveria ser reaproveitado, mas há %d cópias", total)
	}
	if total := falso.contarNoBackup(nomeArquivoBanco); total != 1 {
		t.Fatalf("esperava um único progresso.db na pasta, há %d", total)
	}
}

func TestFalhaQueNaoEhSumicoNaoRefazAhDescoberta(t *testing.T) {
	falso := novosServicosFalsos()
	g, locais := gerenciadorDeTeste(t, falso)
	if _, err := g.Conectar(); err != nil {
		t.Fatal(err)
	}

	// Configurações corrompidas: é falha do lado local, nada sumiu do Drive. Redescobrir aqui só
	// gastaria chamadas à API e arriscaria duplicar.
	if err := os.WriteFile(locais.Configuracoes, []byte(`{"idiomaTradu`), 0644); err != nil {
		t.Fatal(err)
	}
	envelhecerParaOhFuturo(t, locais.Banco, []byte("banco-local-v2"))

	if _, err := g.Sincronizar(); err == nil {
		t.Fatal("esperava erro ao sincronizar com o configuracoes.json corrompido")
	}
	if falso.pastasCriadas != 1 {
		t.Fatalf("uma falha local não deveria disparar a redescoberta da pasta (%d criadas)", falso.pastasCriadas)
	}
}

func TestSincronizarComConflitoPendenteFalha(t *testing.T) {
	falso := novosServicosFalsos()
	falso.semearBackup("banco-da-nuvem", configuracoesLocais)
	g, _ := gerenciadorDeTeste(t, falso)
	if _, err := g.Conectar(); err != nil {
		t.Fatal(err)
	}

	if _, err := g.Sincronizar(); err == nil {
		t.Fatal("esperava erro ao sincronizar com o conflito da primeira conexão pendente")
	}
}

// ----- Credencial e persistência -----

func TestAutorizacaoLevaAhPortaDeLoopbackEhOhNonce(t *testing.T) {
	falso := novosServicosFalsos()
	g, _ := gerenciadorDeTeste(t, falso)

	// Captura a URL da ponte para conferir os parâmetros que o app manda.
	var enderecoVisto string
	abrirOriginal := g.dep.AbrirNavegador
	g.dep.AbrirNavegador = func(u string) {
		enderecoVisto = u
		abrirOriginal(u)
	}

	if _, err := g.Conectar(); err != nil {
		t.Fatal(err)
	}
	analisada, err := url.Parse(enderecoVisto)
	if err != nil {
		t.Fatal(err)
	}
	if analisada.Query().Get("porta") == "" {
		t.Fatal("o app precisa informar à ponte a porta do loopback dele")
	}
	if analisada.Query().Get("nonce") == "" {
		t.Fatal("o app precisa mandar o nonce que confere na volta")
	}
	// Nada de credencial do Google saindo do app: quem as tem é o servidor.
	if analisada.Query().Get("client_id") != "" || analisada.Query().Get("client_secret") != "" {
		t.Fatal("o app não deve mais enviar credenciais OAuth")
	}
}

func TestApenasOhLacreEhGravadoEmDisco(t *testing.T) {
	falso := novosServicosFalsos()
	g, _ := gerenciadorDeTeste(t, falso)
	if _, err := g.Conectar(); err != nil {
		t.Fatal(err)
	}

	var estado map[string]any
	if err := json.Unmarshal([]byte(lerArquivo(t, caminhoEstado())), &estado); err != nil {
		t.Fatal(err)
	}
	if estado["lacre"] != lacreDeTeste {
		t.Fatalf("esperava o lacre persistido, veio %v", estado["lacre"])
	}
	// O refresh token do Google fica no servidor: o disco do usuário não pode ter um.
	if _, tem := estado["refreshToken"]; tem {
		t.Fatal("o estado salvo não pode mais guardar refresh token")
	}
	if estado["pastaId"] == nil || estado["pastaId"] == "" {
		t.Fatal("o id da pasta do Drive deveria ser lembrado entre sessões")
	}
}

func TestTokenExpiradoEhRenovadoNaPonte(t *testing.T) {
	falso := novosServicosFalsos()
	g, _ := gerenciadorDeTeste(t, falso)
	if _, err := g.Conectar(); err != nil {
		t.Fatal(err)
	}
	renovacoesAposConectar := falso.renovacoes

	// Envelhece o token à força e sincroniza de novo: a renovação deve acontecer sozinha.
	g.mu.Lock()
	g.token.ExpiraEm = time.Now().Add(-time.Hour)
	g.mu.Unlock()

	if _, err := g.Sincronizar(); err != nil {
		t.Fatalf("Sincronizar com token vencido: %v", err)
	}
	if falso.renovacoes != renovacoesAposConectar+1 {
		t.Fatalf("esperava exatamente 1 renovação nova, houve %d", falso.renovacoes-renovacoesAposConectar)
	}
}

func TestAcessoRevogadoNaPonteEsqueceAhConexao(t *testing.T) {
	falso := novosServicosFalsos()
	g, _ := gerenciadorDeTeste(t, falso)
	if _, err := g.Conectar(); err != nil {
		t.Fatal(err)
	}

	// Usuário tirou o acesso do app em myaccount.google.com: a ponte passa a recusar o lacre.
	g.mu.Lock()
	g.token.Lacre = lacreRevogado
	g.token.ExpiraEm = time.Now().Add(-time.Hour)
	salvarEstado(g.token)
	g.mu.Unlock()

	if _, err := g.Sincronizar(); err == nil {
		t.Fatal("esperava erro ao sincronizar com o acesso revogado")
	}
	if info := g.Info(); info.Estado != "desconectado" {
		t.Fatalf("uma conexão revogada deveria virar desconectado, veio %q", info.Estado)
	}
	if _, err := os.Stat(caminhoEstado()); !os.IsNotExist(err) {
		t.Fatal("o estado salvo deveria ter sido apagado ao descobrir a revogação")
	}
}

func TestConexaoSobreviveAhReabertura(t *testing.T) {
	falso := novosServicosFalsos()
	g, locais := gerenciadorDeTeste(t, falso)
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

	// E a sincronização continua funcionando sem novo consentimento nem pasta nova.
	envelhecerParaOhFuturo(t, locais.Banco, []byte("banco-local-v2"))
	reaberto.SincronizarSeMudou()
	if conteudo := string(falso.arquivoDoBackup(nomeArquivoBanco).Conteudo); conteudo != "banco-local-v2" {
		t.Fatalf("a sincronização após reabrir deveria ter enviado o banco novo, veio %q", conteudo)
	}
	if falso.pastasCriadas != 1 {
		t.Fatalf("reabrir não deveria criar outra pasta (%d criadas)", falso.pastasCriadas)
	}
}

func TestDesconectarEsqueceAhConexaoEhRevogaNaPonte(t *testing.T) {
	falso := novosServicosFalsos()
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
		t.Fatalf("esperava 1 revogação na ponte, houve %d", falso.revogacoes)
	}
	if _, err := os.Stat(caminhoEstado()); !os.IsNotExist(err) {
		t.Fatal("o arquivo de estado deveria ter sido apagado")
	}
	// O backup na nuvem é preservado (são os dados do usuário, não do app).
	if falso.arquivoDoBackup(nomeArquivoBanco) == nil {
		t.Fatal("desconectar não deveria apagar o backup remoto")
	}
}

// ----- Auxiliares dos testes -----

// envelhecerParaOhFuturo reescreve o arquivo e joga o mtime para a frente, para a comparação por
// mtime enxergar a mudança mesmo em relógio de baixa resolução.
func envelhecerParaOhFuturo(t *testing.T, caminho string, conteudo []byte) {
	t.Helper()

	if err := os.WriteFile(caminho, conteudo, 0644); err != nil {
		t.Fatal(err)
	}
	depois := time.Now().Add(2 * time.Second)
	if err := os.Chtimes(caminho, depois, depois); err != nil {
		t.Fatal(err)
	}
}

func lerArquivo(t *testing.T, caminho string) string {
	t.Helper()

	dados, err := os.ReadFile(caminho)
	if err != nil {
		t.Fatal(err)
	}
	return string(dados)
}
