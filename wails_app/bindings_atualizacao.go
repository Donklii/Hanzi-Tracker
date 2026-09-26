package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"
	"wails_app/atualizacao"
)

// ----- DTOs e Tipos Exportados de Atualização -----

// EstadoAtualizacao encapsula a fotografia do estado atual da atualização automática exposta ao frontend.
type EstadoAtualizacao struct {
	VersaoAtual string `json:"versaoAtual"`
	CanalBuild  string `json:"canalBuild"`
	Commit      string `json:"commit"`
	BuildLocal  bool   `json:"buildLocal"`
	Fase        string `json:"fase"` // "normal" | "atualizando" | "falhou"
	VersaoAlvo  string `json:"versaoAlvo"`
	Erro        string `json:"erro"`
}

// ResultadoVerificacaoAtualizacao informa o resultado de uma checagem manual de atualização disparada pela UI.
type ResultadoVerificacaoAtualizacao struct {
	Disponivel bool   `json:"disponivel"`
	VersaoAlvo string `json:"versaoAlvo"`
	Motivo     string `json:"motivo"`
}


// ----- Métodos Exportados (Bindings do Wails) -----

// ObterEstadoAtualizacao aguarda a conclusão da verificação inicial no boot e devolve o estado atual.
func (a *App) ObterEstadoAtualizacao() EstadoAtualizacao {
	select {
	case <-a.sinalVerificacaoInicial:
	case <-time.After(15 * time.Second):
		a.definirFaseAtualizacao("normal", "", "")
		a.liberarSinalVerificacaoInicial()
	}

	a.estadoAtualizacaoMutex.RLock()
	defer a.estadoAtualizacaoMutex.RUnlock()

	commitCurto := atualizacao.Commit
	if len(commitCurto) > 7 {
		commitCurto = commitCurto[:7]
	}

	return EstadoAtualizacao{
		VersaoAtual: atualizacao.Versao,
		CanalBuild:  atualizacao.CanalDoBuild(),
		Commit:      commitCurto,
		BuildLocal:  atualizacao.BuildLocal(),
		Fase:        a.faseAtualizacao,
		VersaoAlvo:  a.versaoAlvoAtualizacao,
		Erro:        a.erroAtualizacao,
	}
}


// VerificarAtualizacao consulta a disponibilidade de nova versão para o canal informado via parâmetro.
func (a *App) VerificarAtualizacao(canal string) (ResultadoVerificacaoAtualizacao, error) {
	if atualizacao.BuildLocal() {
		return ResultadoVerificacaoAtualizacao{
			Disponivel: false,
			Motivo:     "Build local — atualização automática desativada",
		}, nil
	}

	if err := atualizacao.InstalacaoAtualizavel(); err != nil {
		return ResultadoVerificacaoAtualizacao{
			Disponivel: false,
			Motivo:     fmt.Sprintf("Instalação não atualizável: %v", err),
		}, nil
	}

	ctxBase := a.ctx
	if ctxBase == nil {
		ctxBase = context.Background()
	}
	ctx, cancel := context.WithTimeout(ctxBase, 15*time.Second)
	defer cancel()

	alvo, err := atualizacao.BuscarAlvo(ctx, canal)
	if err != nil {
		return ResultadoVerificacaoAtualizacao{}, err
	}

	if !atualizacao.PrecisaAtualizar(canal, alvo.Manifesto) {
		return ResultadoVerificacaoAtualizacao{
			Disponivel: false,
			VersaoAlvo: alvo.Manifesto.Versao,
			Motivo:     "Você já está na versão mais recente.",
		}, nil
	}

	return ResultadoVerificacaoAtualizacao{
		Disponivel: true,
		VersaoAlvo: alvo.Manifesto.Versao,
	}, nil
}


// IniciarAtualizacao dispara manualmente o fluxo de atualização para o canal especificado.
func (a *App) IniciarAtualizacao(canal string) error {
	if a.atualizacaoEmAndamento.Load() {
		return errors.New("já existe uma atualização em andamento")
	}

	if atualizacao.BuildLocal() {
		return errors.New("atualização não permitida em build local")
	}

	if err := atualizacao.InstalacaoAtualizavel(); err != nil {
		return fmt.Errorf("instalação não atualizável: %w", err)
	}

	ctxBase := a.ctx
	if ctxBase == nil {
		ctxBase = context.Background()
	}
	ctx, cancel := context.WithTimeout(ctxBase, 15*time.Second)
	defer cancel()

	alvo, err := atualizacao.BuscarAlvo(ctx, canal)
	if err != nil {
		return fmt.Errorf("falha ao buscar atualização: %w", err)
	}

	if !atualizacao.PrecisaAtualizar(canal, alvo.Manifesto) {
		return errors.New("não há atualização disponível para o canal especificado")
	}

	a.definirFaseAtualizacao("atualizando", alvo.Manifesto.Versao, "")
	go a.executarAtualizacao(alvo)

	return nil
}


// ----- Orquestração Interna de Atualização -----

func (a *App) verificarAtualizacaoInicial(ctx context.Context) (bool, atualizacao.Alvo, string) {
	if atualizacao.BuildLocal() {
		return false, atualizacao.Alvo{}, "build local detectado — atualização desativada"
	}

	if atualizacao.TemArgumentoPularAtualizacao(os.Args) {
		return false, atualizacao.Alvo{}, "argumento --pular-atualizacao recebido"
	}

	if err := atualizacao.InstalacaoAtualizavel(); err != nil {
		return false, atualizacao.Alvo{}, fmt.Sprintf("instalação não atualizável: %v", err)
	}

	alvo, err := atualizacao.BuscarAlvo(ctx, a.Config.CanalAtualizacao)
	if err != nil {
		return false, atualizacao.Alvo{}, fmt.Sprintf("falha ao buscar versão alvo: %v", err)
	}

	if !atualizacao.PrecisaAtualizar(a.Config.CanalAtualizacao, alvo.Manifesto) {
		return false, atualizacao.Alvo{}, "aplicativo já está na versão mais recente"
	}

	return true, alvo, ""
}


func (a *App) executarAtualizacao(alvo atualizacao.Alvo) {
	if !a.atualizacaoEmAndamento.CompareAndSwap(false, true) {
		return
	}
	defer a.atualizacaoEmAndamento.Store(false)

	runtime.EventsEmit(a.ctx, "atualizacao_iniciada")

	caminhoPacote, err := atualizacao.Baixar(alvo, func(progresso string) {
		runtime.EventsEmit(a.ctx, "atualizacao_progresso", progresso)
	})
	if err != nil {
		a.tratarFalhaAtualizacao(err)
		return
	}

	runtime.EventsEmit(a.ctx, "atualizacao_progresso", "Instalando…")

	if err := atualizacao.Aplicar(caminhoPacote); err != nil {
		a.tratarFalhaAtualizacao(err)
		return
	}

	runtime.Quit(a.ctx)
}


func (a *App) tratarFalhaAtualizacao(err error) {
	msg := err.Error()
	fmt.Printf("Falha na atualização automática: %v\n", err)
	// Os serviços sobem ANTES de a falha ficar visível: o "Continuar" da tela monta o <App/>, que chama
	// bindings dependentes deles. No fluxo manual eles já estão de pé e isto é no-op (sync.Once).
	a.iniciarServicos()
	a.definirFaseAtualizacao("falhou", "", msg)
	runtime.EventsEmit(a.ctx, "atualizacao_falhou", msg)
}


func (a *App) definirFaseAtualizacao(fase, versaoAlvo, erro string) {
	a.estadoAtualizacaoMutex.Lock()
	defer a.estadoAtualizacaoMutex.Unlock()

	a.faseAtualizacao = fase
	if versaoAlvo != "" {
		a.versaoAlvoAtualizacao = versaoAlvo
	}
	a.erroAtualizacao = erro
}


func (a *App) liberarSinalVerificacaoInicial() {
	a.fecharSinalVerificacaoOnce.Do(func() {
		close(a.sinalVerificacaoInicial)
	})
}
