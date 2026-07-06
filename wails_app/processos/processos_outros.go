//go:build !windows

// Package processos concentra o que muda por SO ao subir/derrubar os sidecars (motores de OCR/TTS):
// esconder a janela de console e matar a árvore inteira de processos do motor.
package processos

import (
	"os/exec"
	"syscall"
)

// PrepararSidecar põe o sidecar num process group próprio — é o que permite DerrubarArvore matar a
// árvore inteira de uma vez (kill no grupo, pid negativo).
func PrepararSidecar(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

// DerrubarArvore mata o process group inteiro do sidecar (equivalente ao taskkill /F /T do Windows).
func DerrubarArvore(pid int) error {
	return syscall.Kill(-pid, syscall.SIGKILL)
}
