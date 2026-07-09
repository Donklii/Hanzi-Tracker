// Package nuvem sincroniza o banco de progresso (progresso.db) com o Google Drive do usuário.
// O app continua salvando tudo localmente; a nuvem é um ESPELHO: depois de conectado, o banco é
// reenviado em segundo plano sempre que mudar (LoopSincronizacao) e numa última chance no shutdown.
//
// Primeira conexão: se o Drive já tem um backup (de outra máquina ou instalação anterior), nada é
// sincronizado até o usuário escolher — manter os dados locais (sobrescreve a nuvem) ou usar os da
// nuvem (sobrescreve o banco local). A escolha fica pendente em disco (sobrevive a reaberturas).
//
// Tudo em stdlib: OAuth 2.0 de app instalado com PKCE + redirect em loopback (oauth.go) e a API
// REST v3 do Drive (drive.go), escopo drive.file — o app só enxerga arquivos criados por ele.
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
	AbrirNavegador   func(url string)           // abre a tela de consentimento no navegador do usuário
	Credenciais      func() (id, secret string) // client id/secret OAuth colados pelo usuário na UI
	CaminhoBanco     func() string              // caminho do progresso.db local
	ExportarSnapshot func(destino string) error // cópia consistente do banco (VACUUM INTO)
	SubstituirBanco  func(origem string) error  // fecha o banco, põe `origem` no lugar e reabre
}

// estadoSalvo é o conteúdo de google_drive.json: o token OAuth e o ponto da sincronização.
type estadoSalvo struct {
	RefreshToken        string    `json:"refreshToken"`
	AccessToken         string    `json:"accessToken"`
	ExpiraEm            time.Time `json:"expiraEm"`
	Email               string    `json:"email"`
	RemotoId            string    `json:"remotoId,omitempty"` // id do backup no Drive ("" = ainda não criado)
	ConflitoPendente    bool      `json:"conflitoPendente,omitempty"`
	RemotoBytes         int64     `json:"remotoBytes,omitempty"`
	RemotoModificadoEm  time.Time `json:"remotoModificadoEm,omitempty"`
	UltimaSincronizacao time.Time `json:"ultimaSincronizacao,omitempty"`
}

// Info é o DTO do estado da nuvem para a UI (aba Armazenamento).
type Info struct {
	Estado              string `json:"estado"` // "nao_configurado" | "desconectado" | "conflito" | "conectado"
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
	g := &Gerenciador{dep: dep, urls: endpointsGoogle}
	g.token = carregarEstado()
	return g
}

// ----- Estado para a UI -----

// Info devolve o estado atual da sincronização (sem tocar na rede).
func (g *Gerenciador) Info() Info {
	g.mu.Lock()
	defer g.mu.Unlock()

	info := Info{Estado: "desconectado", Erro: g.ultimoErro}
	if !g.Configurado() {
		info.Estado = "nao_configurado"
		return info
	}
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
	if st, err := os.Stat(g.dep.CaminhoBanco()); err == nil {
		info.LocalBytes = st.Size()
	}
	return info
}

// ----- Conexão -----

// Conectar roda o fluxo OAuth completo (abre o navegador e espera o consentimento) e faz a
// verificação inicial: sem backup na nuvem, envia o banco local; com backup, deixa o CONFLITO
// pendente para o usuário resolver (ResolverConflito). Bloqueia até terminar ou estourar o tempo.
func (g *Gerenciador) Conectar() (Info, error) {
	if !g.Configurado() {
		return g.Info(), fmt.Errorf("preencha o Client ID e o Client Secret do Google na aba Armazenamento antes de conectar")
	}
	if err := g.reservar(); err != nil {
		return g.Info(), err
	}
	defer g.liberar()

	tok, err := g.autorizar()
	if err != nil {
		return g.Info(), err
	}

	remoto, err := g.procurarArquivoRemoto(tok)
	if err != nil {
		// Sem saber se há backup remoto não dá para sincronizar com segurança — a conexão é
		// descartada (o token não foi salvo) e o usuário tenta de novo.
		return g.Info(), fmt.Errorf("conectou ao Google, mas falhou ao consultar o Drive: %w", err)
	}

	if remoto == nil {
		// Nuvem vazia: o banco local vira o backup — conectado e sincronizado num passo só.
		if err := g.enviarBanco(tok); err != nil {
			return g.Info(), fmt.Errorf("conectou, mas falhou ao enviar o banco: %w", err)
		}
	} else {
		// Já existe backup: nada é tocado até o usuário escolher um dos lados.
		tok.ConflitoPendente = true
		tok.RemotoId = remoto.Id
		tok.RemotoBytes = remoto.Bytes
		tok.RemotoModificadoEm = remoto.ModificadoEm
	}

	g.definirToken(tok)
	return g.Info(), nil
}

// ResolverConflito aplica a escolha do usuário da primeira conexão: EscolhaManterLocal envia o
// banco local por cima do backup, EscolhaUsarNuvem baixa o backup por cima do banco local.
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
		if err := g.enviarBanco(tok); err != nil {
			return g.Info(), err
		}
	case EscolhaUsarNuvem:
		if err := g.baixarBanco(tok); err != nil {
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

// Sincronizar envia um snapshot do banco local para a nuvem agora (botão "Sincronizar agora").
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

	if err := g.enviarBanco(tok); err != nil {
		return g.Info(), err
	}
	g.definirToken(tok)
	return g.Info(), nil
}

// SincronizarSeMudou reenvia o banco só se ele mudou desde a última sincronização (comparação por
// mtime). É o passo do loop de fundo e do shutdown; falhas não são fatais — ficam em Info().Erro
// e a próxima passada tenta de novo.
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

	st, err := os.Stat(g.dep.CaminhoBanco())
	if err != nil || !st.ModTime().After(ultima) {
		return
	}
	g.Sincronizar()
}

// LoopSincronizacao espelha o banco em segundo plano: um primeiro tique logo após abrir (apanha
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

// enviarBanco tira um snapshot consistente do banco e o sobe para a nuvem, atualizando os
// metadados de sincronização em `tok` (que o chamador persiste com definirToken).
func (g *Gerenciador) enviarBanco(tok *estadoSalvo) error {
	snapshot := filepath.Join(armazenamento.PastaDados(), "progresso.db.envio.tmp")
	if err := g.dep.ExportarSnapshot(snapshot); err != nil {
		return g.registrarErro(fmt.Errorf("falha ao preparar o snapshot do banco: %w", err))
	}
	defer os.Remove(snapshot)

	id, err := g.enviarArquivo(tok, snapshot, tok.RemotoId)
	if err != nil {
		return g.registrarErro(fmt.Errorf("falha ao enviar o banco para o Drive: %w", err))
	}

	tok.RemotoId = id
	tok.UltimaSincronizacao = time.Now()
	tok.RemotoModificadoEm = tok.UltimaSincronizacao
	if st, err := os.Stat(snapshot); err == nil {
		tok.RemotoBytes = st.Size() // o que subiu foi o snapshot (compactado), não o arquivo vivo
	}
	g.limparErro()
	return nil
}

// baixarBanco baixa o backup remoto e o põe no lugar do banco local (via SubstituirBanco, que
// fecha e reabre a conexão SQLite). Baixa para um temporário primeiro — se a rede cair no meio,
// o banco local fica intacto.
func (g *Gerenciador) baixarBanco(tok *estadoSalvo) error {
	temporario := filepath.Join(armazenamento.PastaDados(), "progresso.db.nuvem.tmp")
	if err := g.baixarArquivo(tok, tok.RemotoId, temporario); err != nil {
		os.Remove(temporario)
		return g.registrarErro(fmt.Errorf("falha ao baixar o backup da nuvem: %w", err))
	}

	if err := g.dep.SubstituirBanco(temporario); err != nil {
		os.Remove(temporario)
		return g.registrarErro(fmt.Errorf("falha ao trocar o banco local pelo da nuvem: %w", err))
	}

	// Local e nuvem acabaram de ficar idênticos.
	tok.UltimaSincronizacao = time.Now()
	g.limparErro()
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
	if err := json.Unmarshal(dados, &estado); err != nil || estado.RefreshToken == "" {
		return nil
	}
	return &estado
}

// salvarEstado persiste o estado com escrita atômica (temp + rename) e permissão restrita ao
// usuário — o arquivo carrega o refresh token da conta Google.
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
