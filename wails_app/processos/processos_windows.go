//go:build windows

// Package processos concentra o que muda por SO ao subir/derrubar os sidecars (motores de OCR/TTS):
// esconder a janela de console e matar a árvore inteira de processos do motor.
package processos

import (
	"os/exec"
	"strconv"
	"syscall"
)

// PrepararSidecar esconde a janela de console do sidecar (o motor é um serviço sem UI).
func PrepararSidecar(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
}

// DerrubarArvore derruba o processo e toda a sua árvore via taskkill /T (o motor pode ter
// subprocessos).
func DerrubarArvore(pid int) error {
	kill := exec.Command("taskkill", "/F", "/T", "/PID", strconv.Itoa(pid))
	kill.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	return kill.Run()
}
