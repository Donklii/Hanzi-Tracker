//go:build windows

package atualizacao

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
)

// ----- Constantes e Scripts de Instalação -----

const scriptPowershellAtualizacao = `
Wait-Process -Id $env:HT_PID -Timeout 120 -ErrorAction SilentlyContinue
$ok = $false
try {
  $p = Start-Process -FilePath $env:HT_INSTALADOR -ArgumentList ('/S /D=' + $env:HT_DIR) -Verb RunAs -Wait -PassThru
  $ok = ($p.ExitCode -eq 0)
} catch {}
Remove-Item -LiteralPath $env:HT_INSTALADOR -Force -ErrorAction SilentlyContinue
if ($ok) { Start-Process -FilePath $env:HT_EXE } else { Start-Process -FilePath $env:HT_EXE -ArgumentList '--pular-atualizacao' }
`


// ----- Aplicação da Atualização no Windows -----

// Aplicar executa o script auxiliar de atualização desacoplado e fecha o app para substituição de binários.
func Aplicar(caminhoPacote string) error {
	if err := InstalacaoAtualizavel(); err != nil {
		return err
	}

	executavel, err := os.Executable()
	if err != nil {
		return fmt.Errorf("falha ao identificar executável corrente: %w", err)
	}

	caminhoPacoteAbs, err := filepath.Abs(caminhoPacote)
	if err != nil {
		return fmt.Errorf("falha ao resolver caminho absoluto do instalador: %w", err)
	}

	diretorio := filepath.Dir(executavel)

	cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-WindowStyle", "Hidden", "-Command", scriptPowershellAtualizacao)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	cmd.Env = append(os.Environ(),
		fmt.Sprintf("HT_PID=%d", os.Getpid()),
		fmt.Sprintf("HT_INSTALADOR=%s", caminhoPacoteAbs),
		fmt.Sprintf("HT_DIR=%s", diretorio),
		fmt.Sprintf("HT_EXE=%s", executavel),
	)

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("falha ao iniciar processo auxiliar de atualização: %w", err)
	}

	return nil
}


// InstalacaoAtualizavel valida se a instalação atual pode receber atualizações automáticas via NSIS.
func InstalacaoAtualizavel() error {
	executavel, err := os.Executable()
	if err != nil {
		return fmt.Errorf("não foi possível obter o caminho do executável: %w", err)
	}

	diretorio := filepath.Dir(executavel)
	caminhoUninstall := filepath.Join(diretorio, "uninstall.exe")

	if _, err := os.Stat(caminhoUninstall); os.IsNotExist(err) {
		return errors.New("aplicativo não foi instalado via instalador NSIS (uninstall.exe ausente)")
	}

	return nil
}
