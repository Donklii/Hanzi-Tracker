// Package nuvem sincroniza os dados do usuário com o Google Drive dele, numa pasta dedicada:
//
//	Hanzi Tracker/
//	├── progresso.db          (vocabulário, progresso e caches)
//	└── configuracoes.json    (preferências do app)
//
// O app continua salvando tudo localmente; a nuvem é um ESPELHO: depois de conectado, os dois
// arquivos são reenviados em segundo plano sempre que qualquer um deles mudar (LoopSincronizacao)
// e numa última chance no shutdown.
//
// Primeira conexão: se o Drive já tem um backup (de outra máquina ou instalação anterior), nada é
// sincronizado até o usuário escolher — manter os dados locais (sobrescreve a nuvem) ou usar os da
// nuvem (sobrescreve o que está em disco). A escolha fica pendente em disco (sobrevive a
// reaberturas). O sinal de "já existe backup" é o progresso.db: as configurações o acompanham.
//
// As credenciais do Google ficam no servidor do Hanzi Tracker, não aqui: o app só guarda um LACRE
// (ponte.go) e o troca por um token de acesso curto a cada operação. Os bytes do banco continuam
// indo direto para a API REST v3 do Drive (drive.go), escopo drive.file — o app só enxerga
// arquivos criados por ele, e o servidor nunca vê o banco.
package nuvem

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"wails_app/armazenamento"
)

// nomeArquivoEstado guarda o token OAuth e o estado da sincronização na pasta de dados.
const nomeArquivoEstado = "google_drive.json"

// Escolhas do usuário na primeira conexão quando já existe um backup na nuvem.
const (
	EscolhaManterLocal = "manterLocal" // envia o banco local por cima do backup remoto
	EscolhaUsarNuvem   = "usarNuvem"   // baixa o backup remoto por cima do banco local
)

// Dependencias é a cola com o resto do app, injetada para o pacote não conhecer Wails/progresso.
type Dependencias struct {
	AbrirNavegador func(url string) // abre a tela de consentimento no navegador do usuário

	CaminhoBanco     func() string              // caminho do progresso.db local
	ExportarSnapshot func(destino string) error // cópia consistente do banco (VACUUM INTO)
	SubstituirBanco  func(origem string) error  // fecha o banco, põe `origem` no lugar e reabre

	CaminhoConfiguracoes    func() string             // caminho do configuracoes.json local
	SubstituirConfiguracoes func(origem string) error // põe `origem` no lugar e recarrega as preferências
}

// estadoSalvo é o conteúdo de google_drive.json: a credencial da conexão e o ponto da sincronização.
// Lacre é o refresh token cifrado pelo servidor — ilegível para o app e para quem ler o arquivo.
// Os três ids são do Drive e valem só enquanto o usuário não apagar a pasta de lá.
type estadoSalvo struct {
	Lacre       string    `json:"lacre"`
	AccessToken string    `json:"accessToken"`
	ExpiraEm    time.Time `json:"expiraEm"`
	Email       string    `json:"email"`

	PastaId               string `json:"pastaId,omitempty"`               // pasta "Hanzi Tracker" no Drive
	RemotoIdBanco         string `json:"remotoIdBanco,omitempty"`         // progresso.db ("" = ainda não enviado)
	RemotoIdConfiguracoes string `json:"remotoIdConfiguracoes,omitempty"` // configuracoes.json ("" = ainda não enviado)

	ConflitoPendente    bool      `json:"conflitoPendente,omitempty"`
	RemotoBytes         int64     `json:"remotoBytes,omitempty"`
	RemotoModificadoEm  time.Time `json:"remotoModificadoEm,omitempty"`
	UltimaSincronizacao time.Time `json:"ultimaSincronizacao,omitempty"`
}

// Info é o DTO do estado da nuvem para a UI (aba Armazenamento).
type Info struct {
	Estado              string `json:"estado"` // "desconectado" | "conflito" | "conectado"
	Email               string `json:"email"`
	UltimaSincronizacao string `json:"ultimaSincronizacao"` // RFC3339 ("" = nunca sincronizou)
	RemotoBytes         int64  `json:"remotoBytes"`
	RemotoModificadoEm  string `json:"remotoModificadoEm"` // RFC3339 ("" = desconhecido)
	LocalBytes          int64  `json:"localBytes"`
	Erro                string `json:"erro"` // última falha de sincronização ("" = nenhuma)
}

// Gerenciador é o dono da conexão com o Google Drive e da sincronização do banco.
type Gerenciador struct {
	dep  Dependencias
	urls endpoints

	// mu protege token/ultimoErro/ocupado. As operações de rede rodam FORA do lock; `ocupado`
	// garante uma operação de nuvem por vez (a UI e o loop de fundo podem disparar em paralelo).
	mu         sync.Mutex
	ocupado    bool
	token      *estadoSalvo
	ultimoErro string
}

// NovoGerenciador cria o gerenciador e recarrega a conexão salva em disco, se houver.
func NovoGerenciador(dep Dependencias) *Gerenciador {
	g := &Gerenciador{dep: dep, urls: endpointsPadrao()}
	g.token = carregarEstado()
	return g
}

// ----- Estado para a UI -----

// Info devolve o estado atual da sincronização (sem tocar na rede).
func (g *Gerenciador) Info() Info {
	g.mu.Lock()
	defer g.mu.Unlock()

	info := Info{Estado: "desconectado", Erro: g.ultimoErro}
	if g.token == nil {
		return info
	}

	info.Estado = "conectado"
	if g.token.ConflitoPendente {
		info.Estado = "conflito"
	}
	info.Email = g.token.Email
	info.RemotoBytes = g.token.RemotoBytes
	if !g.token.UltimaSincronizacao.IsZero() {
		info.UltimaSincronizacao = g.token.UltimaSincronizacao.Format(time.RFC3339)
	}
	if !g.token.RemotoModificadoEm.IsZero() {
		info.RemotoModificadoEm = g.token.RemotoModificadoEm.Format(time.RFC3339)
	}
	info.LocalBytes = tamanhoDoArquivo(g.dep.CaminhoBanco()) + tamanhoDoArquivo(g.dep.CaminhoConfiguracoes())
	return info
}

// ----- Conexão -----

// Conectar roda o fluxo OAuth completo (abre o navegador e espera o consentimento) e faz a
// verificação inicial: sem backup na nuvem, envia o banco local; com backup, deixa o CONFLITO
// pendente para o usuário resolver (ResolverConflito). Bloqueia até terminar ou estourar o tempo.
func (g *Gerenciador) Conectar() (Info, error) {
	if err := g.reservar(); err != nil {
		return g.Info(), err
	}
	defer g.liberar()

	tok, err := g.autorizar()
	if err != nil {
		return g.Info(), err
	}

	pastaId, err := g.garantirPastaRemota(tok)
	if err != nil {
		// Sem a pasta não dá para sincronizar — a conexão é descartada (o lacre não foi salvo) e o
		// usuário tenta de novo.
		return g.Info(), fmt.Errorf("conectou ao Google, mas falhou ao preparar a pasta no Drive: %w", err)
	}
	tok.PastaId = pastaId

	remoto, err := g.listarPastaRemota(tok, pastaId)
	if err != nil {
		return g.Info(), fmt.Errorf("conectou ao Google, mas falhou ao consultar o Drive: %w", err)
	}

	if remoto.Banco == nil {
		// Nuvem vazia: os dados locais viram o backup — conectado e sincronizado num passo só.
		// Um configuracoes.json solto na pasta (sem banco) é sobrescrito junto, não vira conflito.
		if remoto.Configuracoes != nil {
			tok.RemotoIdConfiguracoes = remoto.Configuracoes.Id
		}
		if err := g.enviarArquivos(tok); err != nil {
			return g.Info(), fmt.Errorf("conectou, mas falhou ao enviar os dados: %w", err)
		}
	} else {
		// Já existe backup: nada é tocado até o usuário escolher um dos lados.
		tok.ConflitoPendente = true
		tok.RemotoIdBanco = remoto.Banco.Id
		tok.RemotoBytes = remoto.Banco.Bytes
		tok.RemotoModificadoEm = remoto.Banco.ModificadoEm
		if remoto.Configuracoes != nil {
			tok.RemotoIdConfiguracoes = remoto.Configuracoes.Id
			tok.RemotoBytes += remoto.Configuracoes.Bytes
		}
	}

	g.definirToken(tok)
	return g.Info(), nil
}

// ResolverConflito aplica a escolha do usuário da primeira conexão: EscolhaManterLocal envia os
// dados locais por cima do backup, EscolhaUsarNuvem baixa o backup por cima dos dados locais.
func (g *Gerenciador) ResolverConflito(escolha string) (Info, error) {
	if err := g.reservar(); err != nil {
		return g.Info(), err
	}
	defer g.liberar()

	tok := g.tokenAtual()
	if tok == nil || !tok.ConflitoPendente {
		return g.Info(), fmt.Errorf("não há conflito de sincronização pendente")
	}

	switch escolha {
	case EscolhaManterLocal:
		if err := g.enviarArquivos(tok); err != nil {
			return g.Info(), err
		}
	case EscolhaUsarNuvem:
		if err := g.baixarArquivos(tok); err != nil {
			return g.Info(), err
		}
	default:
		return g.Info(), fmt.Errorf("escolha de conflito desconhecida: %q", escolha)
	}

	tok.ConflitoPendente = false
	g.definirToken(tok)
	return g.Info(), nil
}

// Desconectar revoga o acesso (melhor esforço) e esquece a conexão. O backup na nuvem NÃO é
// apagado — continua no Drive do usuário.
func (g *Gerenciador) Desconectar() (Info, error) {
	if err := g.reservar(); err != nil {
		return g.Info(), err
	}
	defer g.liberar()

	if tok := g.tokenAtual(); tok != nil {
		g.revogarToken(tok)
	}

	g.mu.Lock()
	g.token = nil
	g.ultimoErro = ""
	g.mu.Unlock()
	os.Remove(caminhoEstado())
	return g.Info(), nil
}

// ----- Sincronização -----

// Sincronizar envia agora um snapshot do banco e as configurações para a nuvem (botão
// "Sincronizar agora").
func (g *Gerenciador) Sincronizar() (Info, error) {
	if err := g.reservar(); err != nil {
		return g.Info(), err
	}
	defer g.liberar()

	tok := g.tokenAtual()
	if tok == nil {
		return g.Info(), fmt.Errorf("o Google Drive não está conectado")
	}
	if tok.ConflitoPendente {
		return g.Info(), fmt.Errorf("resolva o conflito da primeira conexão antes de sincronizar")
	}

	if err := g.enviarArquivos(tok); err != nil {
		return g.Info(), err
	}
	g.definirToken(tok)
	return g.Info(), nil
}

// SincronizarSeMudou reenvia os dados só se o banco OU as configurações mudaram desde a última
// sincronização (comparação por mtime). É o passo do loop de fundo e do shutdown; falhas não são
// fatais — ficam em Info().Erro e a próxima passada tenta de novo.
func (g *Gerenciador) SincronizarSeMudou() {
	g.mu.Lock()
	pronto := g.token != nil && !g.token.ConflitoPendente && !g.ocupado
	ultima := time.Time{}
	if g.token != nil {
		ultima = g.token.UltimaSincronizacao
	}
	g.mu.Unlock()
	if !pronto {
		return
	}

	if !g.algoMudouDesde(ultima) {
		return
	}
	g.Sincronizar()
}

// algoMudouDesde diz se algum dos arquivos sincronizados foi tocado depois de `momento`.
func (g *Gerenciador) algoMudouDesde(momento time.Time) bool {
	for _, caminho := range []string{g.dep.CaminhoBanco(), g.dep.CaminhoConfiguracoes()} {
		st, err := os.Stat(caminho)
		if err != nil {
			continue
		}
		if st.ModTime().After(momento) {
			return true
		}
	}
	return false
}

// LoopSincronizacao espelha os dados em segundo plano: um primeiro tique logo após abrir (apanha
// mudanças da sessão anterior que ficaram sem envio) e depois a cada `intervalo`, até o ctx cair.
func (g *Gerenciador) LoopSincronizacao(ctx context.Context, intervalo time.Duration) {
	temporizador := time.NewTimer(30 * time.Second)
	defer temporizador.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-temporizador.C:
		}
		g.SincronizarSeMudou()
		temporizador.Reset(intervalo)
	}
}

// ----- Passos internos (rodam já reservados) -----

// enviarArquivos sobe o banco e as configurações para a pasta do backup, atualizando os metadados
// de sincronização em `tok` (que o chamador persiste com definirToken).
//
// Os ids do Drive guardados em `tok` valem só enquanto o usuário não apagar nada de lá. Quando o
// Drive responde "não encontrado", eles apontam para o que já não existe: o destino é redescoberto
// e o envio refeito uma vez, para uma pasta apagada por engano não deixar a sincronização quebrada
// para sempre.
func (g *Gerenciador) enviarArquivos(tok *estadoSalvo) error {
	snapshot := filepath.Join(armazenamento.PastaDados(), "progresso.db.envio.tmp")
	if err := g.dep.ExportarSnapshot(snapshot); err != nil {
		return g.registrarErro(fmt.Errorf("falha ao preparar o snapshot do banco: %w", err))
	}
	defer os.Remove(snapshot)

	err := g.subirOhBackup(tok, snapshot)
	if ehNaoEncontrado(err) {
		if erroRedescoberta := g.redescobrirDestino(tok); erroRedescoberta != nil {
			return g.registrarErro(erroRedescoberta)
		}
		err = g.subirOhBackup(tok, snapshot)
	}
	if err != nil {
		return g.registrarErro(err)
	}

	tok.UltimaSincronizacao = time.Now()
	tok.RemotoModificadoEm = tok.UltimaSincronizacao
	g.limparErro()
	return nil
}

// subirOhBackup envia os dois arquivos para a pasta apontada por `tok` e atualiza os ids e o
// tamanho remoto. O erro volta cru: quem decide se vale redescobrir o destino é enviarArquivos.
func (g *Gerenciador) subirOhBackup(tok *estadoSalvo, snapshot string) error {
	idBanco, err := g.enviarArquivo(tok, snapshot, tok.RemotoIdBanco, tok.PastaId, nomeArquivoBanco)
	if err != nil {
		return fmt.Errorf("falha ao enviar o banco para o Drive: %w", err)
	}
	tok.RemotoIdBanco = idBanco
	tok.RemotoBytes = tamanhoDoArquivo(snapshot) // o que subiu foi o snapshot, não o arquivo vivo

	// As configurações só sobem quando o arquivo local está íntegro: mandar um JSON truncado (lido
	// no meio de uma gravação do app) contaminaria as preferências das outras máquinas.
	caminhoConfiguracoes := g.dep.CaminhoConfiguracoes()
	if err := conferirJsonIntegro(caminhoConfiguracoes); err != nil {
		return fmt.Errorf("banco enviado, mas as configurações ficaram de fora: %w", err)
	}

	idConfiguracoes, err := g.enviarArquivo(tok, caminhoConfiguracoes, tok.RemotoIdConfiguracoes, tok.PastaId, nomeArquivoConfiguracoes)
	if err != nil {
		return fmt.Errorf("banco enviado, mas falhou ao enviar as configurações: %w", err)
	}
	tok.RemotoIdConfiguracoes = idConfiguracoes
	tok.RemotoBytes += tamanhoDoArquivo(caminhoConfiguracoes)
	return nil
}

// redescobrirDestino refaz os ids do Drive depois de um "não encontrado": esquece o que estava
// salvo, resolve a pasta de novo e readota os arquivos que ainda restam lá dentro.
//
// É aqui que mora o cuidado de não duplicar. A pasta só nasce quando a busca por nome não acha
// nenhuma (garantirPastaRemota), então apagar só um arquivo não gera uma segunda pasta; e cada
// arquivo só é criado do zero quando não há homônimo na pasta para sobrescrever, então o que
// sobreviveu é atualizado no lugar em vez de ganhar uma cópia ao lado.
func (g *Gerenciador) redescobrirDestino(tok *estadoSalvo) error {
	tok.PastaId = ""
	tok.RemotoIdBanco = ""
	tok.RemotoIdConfiguracoes = ""

	pastaId, err := g.garantirPastaRemota(tok)
	if err != nil {
		return fmt.Errorf("o backup sumiu do Drive e falhou ao preparar a pasta de novo: %w", err)
	}
	tok.PastaId = pastaId

	remoto, err := g.listarPastaRemota(tok, pastaId)
	if err != nil {
		return fmt.Errorf("o backup sumiu do Drive e falhou ao consultar a pasta: %w", err)
	}
	if remoto.Banco != nil {
		tok.RemotoIdBanco = remoto.Banco.Id
	}
	if remoto.Configuracoes != nil {
		tok.RemotoIdConfiguracoes = remoto.Configuracoes.Id
	}
	return nil
}

// baixarArquivos traz o backup da nuvem por cima dos dados locais: o banco (via SubstituirBanco,
// que fecha e reabre a conexão SQLite) e as configurações. Cada arquivo é baixado para um
// temporário primeiro — se a rede cair no meio, o que está em disco fica intacto.
func (g *Gerenciador) baixarArquivos(tok *estadoSalvo) error {
	temporarioBanco := filepath.Join(armazenamento.PastaDados(), "progresso.db.nuvem.tmp")
	if err := g.baixarArquivo(tok, tok.RemotoIdBanco, temporarioBanco); err != nil {
		os.Remove(temporarioBanco)
		return g.registrarErro(fmt.Errorf("falha ao baixar o backup da nuvem: %w", err))
	}
	if err := g.dep.SubstituirBanco(temporarioBanco); err != nil {
		os.Remove(temporarioBanco)
		return g.registrarErro(fmt.Errorf("falha ao trocar o banco local pelo da nuvem: %w", err))
	}

	if err := g.baixarConfiguracoes(tok); err != nil {
		return err
	}

	// Local e nuvem acabaram de ficar idênticos.
	tok.UltimaSincronizacao = time.Now()
	g.limparErro()
	return nil
}

// baixarConfiguracoes traz o configuracoes.json da nuvem. Um backup criado antes desta versão (ou
// por uma máquina que falhou ao enviar as preferências) não tem o arquivo: nesse caso as
// configurações locais continuam valendo, e é o backup que se completa na próxima sincronização.
func (g *Gerenciador) baixarConfiguracoes(tok *estadoSalvo) error {
	if tok.RemotoIdConfiguracoes == "" {
		return nil
	}

	temporario := filepath.Join(armazenamento.PastaDados(), "configuracoes.json.nuvem.tmp")
	if err := g.baixarArquivo(tok, tok.RemotoIdConfiguracoes, temporario); err != nil {
		os.Remove(temporario)
		return g.registrarErro(fmt.Errorf("banco restaurado, mas falhou ao baixar as configurações: %w", err))
	}
	// Um JSON corrompido na nuvem deixaria o app sem preferências válidas ao reabrir.
	if err := conferirJsonIntegro(temporario); err != nil {
		os.Remove(temporario)
		return g.registrarErro(fmt.Errorf("banco restaurado, mas as configurações da nuvem vieram ilegíveis: %w", err))
	}

	if err := g.dep.SubstituirConfiguracoes(temporario); err != nil {
		os.Remove(temporario)
		return g.registrarErro(fmt.Errorf("banco restaurado, mas falhou ao aplicar as configurações da nuvem: %w", err))
	}
	return nil
}

// ----- Miudezas de estado -----

// reservar garante uma operação de nuvem por vez (conectar/sincronizar/resolver/desconectar).
func (g *Gerenciador) reservar() error {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.ocupado {
		return fmt.Errorf("já há uma operação de nuvem em andamento")
	}
	g.ocupado = true
	return nil
}

func (g *Gerenciador) liberar() {
	g.mu.Lock()
	g.ocupado = false
	g.mu.Unlock()
}

// tokenAtual devolve uma CÓPIA do token para a operação em curso mexer fora do lock.
func (g *Gerenciador) tokenAtual() *estadoSalvo {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.token == nil {
		return nil
	}
	copia := *g.token
	return &copia
}

// definirToken adota `tok` como estado corrente e o persiste.
func (g *Gerenciador) definirToken(tok *estadoSalvo) {
	g.mu.Lock()
	g.token = tok
	g.mu.Unlock()
	salvarEstado(tok)
}

// registrarErro guarda a falha para a UI (Info().Erro) e a devolve para o chamador propagar.
func (g *Gerenciador) registrarErro(err error) error {
	g.mu.Lock()
	g.ultimoErro = err.Error()
	g.mu.Unlock()
	return err
}

func (g *Gerenciador) limparErro() {
	g.mu.Lock()
	g.ultimoErro = ""
	g.mu.Unlock()
}

// ----- Utilitários -----

// tamanhoDoArquivo devolve o tamanho em bytes (0 quando o arquivo não existe ou não pode ser lido).
func tamanhoDoArquivo(caminho string) int64 {
	st, err := os.Stat(caminho)
	if err != nil {
		return 0
	}
	return st.Size()
}

// conferirJsonIntegro recusa um arquivo que não seja um JSON completo — pega tanto arquivo lido no
// meio de uma gravação quanto download interrompido.
func conferirJsonIntegro(caminho string) error {
	dados, err := os.ReadFile(caminho)
	if err != nil {
		return fmt.Errorf("não foi possível ler %s: %w", filepath.Base(caminho), err)
	}
	if !json.Valid(dados) {
		return fmt.Errorf("%s não está em JSON válido", filepath.Base(caminho))
	}
	return nil
}

// ----- Persistência do estado -----

func caminhoEstado() string {
	return filepath.Join(armazenamento.PastaDados(), nomeArquivoEstado)
}

// carregarEstado lê google_drive.json; qualquer problema (não existe, corrompido) vale como
// "desconectado" — o usuário só precisa conectar de novo.
func carregarEstado() *estadoSalvo {
	dados, err := os.ReadFile(caminhoEstado())
	if err != nil {
		return nil
	}
	var estado estadoSalvo
	if err := json.Unmarshal(dados, &estado); err != nil || estado.Lacre == "" {
		return nil
	}
	return &estado
}

// salvarEstado persiste o estado com escrita atômica (temp + rename) e permissão restrita ao
// usuário — o arquivo carrega a credencial da conexão com a conta Google.
func salvarEstado(estado *estadoSalvo) error {
	dados, err := json.MarshalIndent(estado, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(armazenamento.PastaDados(), 0755); err != nil {
		return err
	}

	temporario := caminhoEstado() + ".tmp"
	if err := os.WriteFile(temporario, dados, 0600); err != nil {
		return err
	}
	return os.Rename(temporario, caminhoEstado())
}
