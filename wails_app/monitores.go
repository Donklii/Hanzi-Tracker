package main

import (
	"fmt"
	"image"
	"os/exec"
	"strings"

	"github.com/kbinani/screenshot"
)

// ----- Monitores e resolução de captura -----
// Bindings de enumeração de monitores para a UI e o helper central de bounds do monitor alvo,
// usado por captura, highlights e pop-ups (todos precisam converter coordenadas locais↔absolutas).

// Monitor descreve um display conectado para o select de monitor da UI.
type Monitor struct {
	ID      int    `json:"id"`
	Nome    string `json:"nome"`
	Largura int    `json:"largura"`
	Altura  int    `json:"altura"`
	X       int    `json:"x"`
	Y       int    `json:"y"`
}

// Resolucao representa a resolução (em px) do monitor de captura.
type Resolucao struct {
	Largura int `json:"largura"`
	Altura  int `json:"altura"`
}

// GetMonitores retorna a lista de todos os monitores conectados
func (a *App) GetMonitores() []Monitor {
	wmiNames := getMonitorNamesWMI()
	n := screenshot.NumActiveDisplays()
	var monitores []Monitor
	for i := 0; i < n; i++ {
		bounds := screenshot.GetDisplayBounds(i)
		nome := fmt.Sprintf("Monitor %d", i+1)
		if i == 0 && len(wmiNames) == 0 {
			nome = "Monitor Principal"
		}

		if i < len(wmiNames) {
			nome = wmiNames[i]
		}

		monitores = append(monitores, Monitor{
			ID:      i,
			Nome:    nome,
			Largura: bounds.Dx(),
			Altura:  bounds.Dy(),
			X:       bounds.Min.X,
			Y:       bounds.Min.Y,
		})
	}
	return monitores
}

// GetCaptureResolution retorna a resolução nativa do monitor capturado (display alvo),
// usada como teto/padrão do controle de Qualidade da Imagem do OCR.
func (a *App) GetCaptureResolution() Resolucao {
	bounds := a.limitesMonitorAlvo()
	return Resolucao{Largura: bounds.Dx(), Altura: bounds.Dy()}
}

// limitesMonitorAlvo devolve o retângulo (coordenadas absolutas de tela) do monitor de captura
// configurado, caindo no monitor 0 quando o alvo salvo não existe mais (ex.: monitor desconectado).
func (a *App) limitesMonitorAlvo() image.Rectangle {
	alvo := a.Config.MonitorAlvo
	if alvo < 0 || alvo >= screenshot.NumActiveDisplays() {
		alvo = 0
	}
	return screenshot.GetDisplayBounds(alvo)
}

// getMonitorNamesWMI usa PowerShell para extrair o nome real dos monitores no Windows
func getMonitorNamesWMI() []string {
	out, err := exec.Command("powershell", "-NoProfile", "-Command", "Get-WmiObject -Namespace root\\wmi -Class WmiMonitorID | Select-Object -ExpandProperty InstanceName").Output()
	var names []string
	if err != nil {
		return names
	}
	lines := strings.Split(string(out), "\n")
	for _, l := range lines {
		l = strings.TrimSpace(l)
		if strings.Contains(l, "DISPLAY\\") {
			parts := strings.Split(l, "\\")
			if len(parts) > 1 {
				names = append(names, parts[1])
			}
		}
	}
	return names
}
