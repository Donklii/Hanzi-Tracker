//go:build windows

package main

import (
	"os/exec"
	"strconv"
)

// prepararComandoFilho: no Windows o taskkill /T já derruba a árvore inteira por PID — não é
// preciso preparar nada no comando antes do Start.
func prepararComandoFilho(cmd *exec.Cmd) {}

// derrubarArvoreProcessos derruba o processo e toda a sua árvore via taskkill /T. É o caminho
// confiável no Windows: o `go run` cria um binário filho e o motor de OCR é neto do app — ambos só
// caem matando a árvore inteira, não apenas o processo direto.
func derrubarArvoreProcessos(pid int) error {
	return exec.Command("taskkill", "/F", "/T", "/PID", strconv.Itoa(pid)).Run()
}
