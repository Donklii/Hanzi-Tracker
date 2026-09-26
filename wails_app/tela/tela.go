package tela

import (
	"context"
	"wails_app/config"
)

type GerenciadorTela struct {
	ctx         context.Context
	obterConfig func() config.Config
}

func NovoGerenciadorTela(ctx context.Context, obterConfig func() config.Config) *GerenciadorTela {
	return &GerenciadorTela{
		ctx:         ctx,
		obterConfig: obterConfig,
	}
}
