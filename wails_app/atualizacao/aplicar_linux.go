//go:build linux

package atualizacao

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"
)

// ----- Constantes do Ambiente Linux -----

const (
	CAMINHO_BINARIO_LINUX_PADRAO = "/usr/bin/hanzitracker"
	SCRIPT_RELANCADOR_LINUX      = `while kill -0 "$HT_PID" 2>/dev/null; do sleep 0.3; done; exec /usr/bin/hanzitracker`
)


// ----- Aplicação da Atualização no Linux -----

// Aplicar instala o pacote .deb síncronamente via pkexec e inicia o relançador destacado.
func Aplicar(caminhoPacote string) error {
	if err := InstalacaoAtualizavel(); err != nil {
		return err
	}

	cmdDpkg := exec.Command("pkexec", "dpkg", "-i", caminhoPacote)
	saida, err := cmdDpkg.CombinedOutput()
	if err != nil {
		return fmt.Errorf("falha ao instalar pacote .deb: %w (detalhes: %s)", err, strings.TrimSpace(string(saida)))
	}

	_ = os.Remove(caminhoPacote)

	cmdRelancador := exec.Command("sh", "-c", SCRIPT_RELANCADOR_LINUX)
	cmdRelancador.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	cmdRelancador.Env = append(os.Environ(), fmt.Sprintf("HT_PID=%d", os.Getpid()))

	if err := cmdRelancador.Start(); err != nil {
		return fmt.Errorf("falha ao iniciar processo de relançamento: %w", err)
	}

	return nil
}


// InstalacaoAtualizavel verifica se o binário está no local padrão e se o pkexec está disponível.
func InstalacaoAtualizavel() error {
	executavel, err := os.Executable()
	if err != nil {
		return fmt.Errorf("não foi possível determinar o executável: %w", err)
	}

	if executavel != CAMINHO_BINARIO_LINUX_PADRAO {
		return fmt.Errorf("instalação não atualizável: executável em %s, esperado %s", executavel, CAMINHO_BINARIO_LINUX_PADRAO)
	}

	if _, err := exec.LookPath("pkexec"); err != nil {
		return errors.New("pkexec não encontrado no PATH")
	}

	return nil
}
