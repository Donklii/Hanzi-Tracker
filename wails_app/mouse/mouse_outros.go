//go:build !windows && !linux

package mouse

import "errors"

// GetCursorPos não tem implementação fora do Windows: o rastreio global do mouse alimenta os pop-ups
// de hover do overlay, que também são Windows-only (ver overlay/overlay_outros.go). Devolver erro faz
// o loop de rastreio (loop.go) simplesmente não emitir eventos.
func GetCursorPos() (int, int, error) {
	return 0, 0, errors.New("posição global do mouse indisponível fora do Windows")
}
