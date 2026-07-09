package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"
	"wails_app/armazenamento"
	"wails_app/nuvem"
	"wails_app/progresso"
)

// ----- Sincronização com o Google Drive (bindings e cola com o app) -----
// A lógica vive em wails_app/nuvem; aqui ficam a injeção das dependências (navegador do Wails,
// snapshot/troca do banco do progresso) e os métodos expostos ao frontend.

// iniciarNuvem cria o gerenciador de nuvem e liga o espelhamento automático de fundo.
func (a *App) iniciarNuvem(ctx context.Context) {
	a.nuvem = nuvem.NovoGerenciador(nuvem.Dependencias{
		AbrirNavegador: func(url string) { runtime.BrowserOpenURL(a.ctx, url) },
		// As credenciais OAuth são as que o usuário colou na aba Armazenamento — lidas na hora,
		// para valerem assim que salvas, sem reiniciar o app.
		Credenciais:      func() (string, string) { return a.Config.DriveClientId, a.Config.DriveClientSecret },
		CaminhoBanco:     armazenamento.CaminhoBanco,
		ExportarSnapshot: progresso.ExportarSnapshot,
		SubstituirBanco:  substituirBancoLocal,
	})
	go a.nuvem.LoopSincronizacao(ctx, 5*time.Minute)
}

// encerrarNuvem dá à sincronização uma última chance de espelhar mudanças recentes, com prazo
// curto para não segurar o fechamento do app — o que não subir agora sobe na próxima abertura
// (o primeiro tique do LoopSincronizacao reenvia o banco se o mtime dele passou do último envio).
func (a *App) encerrarNuvem() {
	if a.nuvem == nil {
		return
	}

	feito := make(chan struct{})
	go func() {
		a.nuvem.SincronizarSeMudou()
		close(feito)
	}()
	select {
	case <-feito:
	case <-time.After(30 * time.Second):
		fmt.Println("Aviso: a sincronização final com o Drive não terminou a tempo — fica para a próxima abertura.")
	}
}

// substituirBancoLocal troca o arquivo do banco pelo recém-baixado da nuvem: fecha a conexão
// SQLite, move o novo por cima e reabre. Reabre mesmo se a troca falhar, para o app nunca ficar
// sem banco aberto.
func substituirBancoLocal(origem string) error {
	if err := progresso.FecharDB(); err != nil {
		return err
	}
	errTroca := os.Rename(origem, armazenamento.CaminhoBanco())
	if err := progresso.InitDB(); err != nil {
		return fmt.Errorf("falha ao reabrir o banco após a troca: %w", err)
	}
	return errTroca
}

// ----- Métodos expostos ao frontend -----

// GetInfoNuvem retorna o estado da sincronização para a aba Armazenamento (sem tocar na rede).
func (a *App) GetInfoNuvem() nuvem.Info {
	return a.nuvem.Info()
}

// ConectarNuvem roda a autorização do Google (abre o navegador e espera o consentimento) e a
// verificação inicial: nuvem vazia = envia o banco local; backup existente = estado "conflito",
// aguardando ResolverConflitoNuvem.
func (a *App) ConectarNuvem() (nuvem.Info, error) {
	return a.nuvem.Conectar()
}

// ResolverConflitoNuvem aplica a escolha da primeira conexão ("manterLocal" | "usarNuvem").
func (a *App) ResolverConflitoNuvem(escolha string) (nuvem.Info, error) {
	return a.nuvem.ResolverConflito(escolha)
}

// SincronizarNuvem envia o banco para o Drive agora (botão "Sincronizar agora").
func (a *App) SincronizarNuvem() (nuvem.Info, error) {
	return a.nuvem.Sincronizar()
}

// DesconectarNuvem revoga o acesso e esquece a conexão (o backup continua no Drive do usuário).
func (a *App) DesconectarNuvem() (nuvem.Info, error) {
	return a.nuvem.Desconectar()
}
