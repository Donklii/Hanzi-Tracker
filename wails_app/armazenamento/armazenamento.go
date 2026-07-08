// Package armazenamento reúne os caminhos de dados do app (%APPDATA%\HanziTracker\...) e as funções
// puras de medição/limpeza de disco, usadas tanto pela aba de Armazenamento (App, em armazenamento.go
// na raiz) quanto pelos pacotes de motores baixáveis (motoresocr, motorestts).
package armazenamento

import (
	"os"
	"path/filepath"
	"strings"
)

// ----- Caminhos -----

func PastaDados() string {
	appData, err := os.UserConfigDir()
	if err != nil {
		return ""
	}
	return filepath.Join(appData, "HanziTracker")
}

func PastaEasyOcr() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".EasyOCR")
}

func PastaCachePip() string {
	local := os.Getenv("LOCALAPPDATA")
	if local == "" {
		return ""
	}
	return filepath.Join(local, "pip", "Cache")
}

func CaminhoBanco() string {
	return filepath.Join(PastaDados(), "progresso.db")
}

// ----- Medição -----

// TamanhoCaminho soma o tamanho de um arquivo ou de todos os arquivos de uma pasta (recursivo).
func TamanhoCaminho(caminho string) int64 {
	if caminho == "" {
		return 0
	}
	info, err := os.Stat(caminho)
	if err != nil {
		return 0
	}
	if !info.IsDir() {
		return info.Size()
	}

	var total int64
	filepath.Walk(caminho, func(_ string, fi os.FileInfo, err error) error {
		if err == nil && fi != nil && !fi.IsDir() {
			total += fi.Size()
		}
		return nil
	})
	return total
}

// TamanhoLogs soma o tamanho dos arquivos .log na pasta de dados.
func TamanhoLogs() int64 {
	var total int64
	entradas, err := os.ReadDir(PastaDados())
	if err != nil {
		return 0
	}
	for _, e := range entradas {
		if e.IsDir() || !strings.HasSuffix(strings.ToLower(e.Name()), ".log") {
			continue
		}
		if info, err := e.Info(); err == nil {
			total += info.Size()
		}
	}
	return total
}

// ----- Limpeza -----

// LimparLogs remove apenas os arquivos .log da pasta de dados.
func LimparLogs() error {
	entradas, err := os.ReadDir(PastaDados())
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	for _, e := range entradas {
		if e.IsDir() || !strings.HasSuffix(strings.ToLower(e.Name()), ".log") {
			continue
		}
		if err := os.Remove(filepath.Join(PastaDados(), e.Name())); err != nil {
			return err
		}
	}
	return nil
}
