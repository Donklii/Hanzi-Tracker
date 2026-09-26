package util

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"golang.design/x/hotkey"
)

type itemAtalhoGlobal struct {
	combo   string
	hk      *hotkey.Hotkey
	cancel  context.CancelFunc
}

// GerenciadorAtalhos controla o registro e desregistro dinâmico de atalhos globais no SO.
type GerenciadorAtalhos struct {
	mu            sync.Mutex
	atalhosAtivos map[string]*itemAtalhoGlobal
}

// NovoGerenciadorAtalhos cria uma nova instância do gerenciador de atalhos globais.
func NovoGerenciadorAtalhos() *GerenciadorAtalhos {
	return &GerenciadorAtalhos{
		atalhosAtivos: make(map[string]*itemAtalhoGlobal),
	}
}

// AtualizarAtalhos ajusta reativamente a lista de atalhos globais registrados no SO.
// Chaves que mudaram de combinação ou foram limpas são desregistradas imediatamente.
// Devolve um mapa de erros por nome de atalho (caso algum falhe ao registrar no SO).
func (g *GerenciadorAtalhos) AtualizarAtalhos(novosAtalhos map[string]string, handlers map[string]func()) map[string]string {
	g.mu.Lock()
	defer g.mu.Unlock()

	erros := make(map[string]string)

	// 1. Remover ou atualizar atalhos existentes
	for nome, item := range g.atalhosAtivos {
		novoComboNormalized := strings.ToLower(strings.TrimSpace(novosAtalhos[nome]))
		oldComboNormalized := strings.ToLower(strings.TrimSpace(item.combo))

		// Se mudou ou foi removido ou não tem mais handler
		_, handlerExiste := handlers[nome]
		if novoComboNormalized != oldComboNormalized || !handlerExiste || novoComboNormalized == "" {
			g.desregistrarItem(nome)
		}
	}

	// 2. Registrar novas combinações
	for nome, combo := range novosAtalhos {
		comboNormalized := strings.ToLower(strings.TrimSpace(combo))
		if comboNormalized == "" {
			continue
		}
		handler, handlerExiste := handlers[nome]
		if !handlerExiste || handler == nil {
			continue
		}

		// Se já estiver registrado exatamente como solicitado, mantém
		if item, ok := g.atalhosAtivos[nome]; ok && strings.ToLower(strings.TrimSpace(item.combo)) == comboNormalized {
			continue
		}

		hk := ParseHotkey(comboNormalized)
		if hk == nil {
			erros[nome] = fmt.Sprintf("Combinação inválida ou não suportada: %q", combo)
			continue
		}

		if err := hk.Register(); err != nil {
			erros[nome] = fmt.Sprintf("Não foi possível registrar o atalho global %q no SO: %v", combo, err)
			continue
		}

		ctx, cancel := context.WithCancel(context.Background())
		item := &itemAtalhoGlobal{
			combo:  comboNormalized,
			hk:     hk,
			cancel: cancel,
		}
		g.atalhosAtivos[nome] = item

		// Goroutine para escutar acionamento da tecla
		go func(h func(), hot *hotkey.Hotkey, c context.Context) {
			for {
				select {
				case <-c.Done():
					return
				case _, ok := <-hot.Keydown():
					if !ok {
						return
					}
					h()
				}
			}
		}(handler, hk, ctx)
	}

	return erros
}

// DesativarTodos remove todos os atalhos globais registrados no SO.
func (g *GerenciadorAtalhos) DesativarTodos() {
	g.mu.Lock()
	defer g.mu.Unlock()

	for nome := range g.atalhosAtivos {
		g.desregistrarItem(nome)
	}
}

func (g *GerenciadorAtalhos) desregistrarItem(nome string) {
	item, ok := g.atalhosAtivos[nome]
	if !ok {
		return
	}
	if item.cancel != nil {
		item.cancel()
	}
	if item.hk != nil {
		_ = item.hk.Unregister()
	}
	delete(g.atalhosAtivos, nome)
}
