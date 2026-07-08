//go:build !windows

package main

import (
	"os/exec"
	"syscall"
)

// prepararComandoFilho põe o app num process group próprio — é o que permite derrubarArvoreProcessos
// matar a árvore inteira de uma vez (kill no grupo, pid negativo). Espelha wails_app/processos, que
// faz o mesmo com os sidecars.
func prepararComandoFilho(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

// derrubarArvoreProcessos mata o process group inteiro do app (equivalente ao taskkill /F /T do
// Windows) — sem isso, o binário gerado pelo `go run` e o motor de OCR ficariam órfãos.
func derrubarArvoreProcessos(pid int) error {
	return syscall.Kill(-pid, syscall.SIGKILL)
}
