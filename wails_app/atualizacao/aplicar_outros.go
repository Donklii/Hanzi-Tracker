//go:build !windows && !linux

package atualizacao

import "errors"

// ----- Aplicação da Atualização em Outras Plataformas -----

// Aplicar retorna erro para plataformas sem suporte a atualização automática.
func Aplicar(caminhoPacote string) error {
	return errors.New("sistema sem atualização automática")
}


// InstalacaoAtualizavel retorna erro para plataformas sem suporte a atualização automática.
func InstalacaoAtualizavel() error {
	return errors.New("sistema sem atualização automática")
}
