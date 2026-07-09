package main

import (
	"fmt"
	"image"
	"os/exec"
	"runtime"
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

// getDisplayBoundsX11 é um wrapper sobre screenshot.GetDisplayBounds que resolve o problema do X11 no Linux.
// A lib de captura retorna offsets pseudo-XRandR (onde o monitor à esquerda do principal tem coords negativas).
// Contudo, X11 (captura de tela, XQueryPointer, XCreateWindow) opera no espaço da Root Window,
// cuja origem (0,0) é estritamente o topo-esquerdo do conjunto inteiro de monitores.
// Essa função translaciona todos os bounds para o espaço positivo da Root Window.
func getDisplayBoundsX11(index int) image.Rectangle {
	bounds := screenshot.GetDisplayBounds(index)
	return traduzirParaAbsolutoX11(bounds)
}

// traduzirParaAbsolutoX11 converte um retângulo do sistema de coordenadas pseudo-XRandR
// (retornado pela captura ou por bibliotecas gráficas como GTK) para coordenadas absolutas
// da Root Window do X11. Retorna o próprio retângulo se não estiver no Linux.
func traduzirParaAbsolutoX11(r image.Rectangle) image.Rectangle {
	if runtime.GOOS != "linux" {
		return r
	}

	n := screenshot.NumActiveDisplays()
	minX, minY := 0, 0
	for i := 0; i < n; i++ {
		b := screenshot.GetDisplayBounds(i)
		if b.Min.X < minX {
			minX = b.Min.X
		}
		if b.Min.Y < minY {
			minY = b.Min.Y
		}
	}
	return image.Rect(
		r.Min.X-minX, r.Min.Y-minY,
		r.Max.X-minX, r.Max.Y-minY,
	)
}

// getMonitorNamesLinux tenta descobrir os nomes reais dos monitores via xrandr.
func getMonitorNamesLinux() []string {
	if runtime.GOOS != "linux" {
		return nil
	}
	out, err := exec.Command("xrandr", "--listmonitors").Output()
	if err != nil {
		return nil
	}
	
	var names []string
	lines := strings.Split(string(out), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		// Ignora a primeira linha "Monitors: N" e pega apenas as de monitores
		if len(line) > 0 && strings.Contains(line, ":") {
			parts := strings.Fields(line)
			if len(parts) > 0 {
				// O último token da linha do xrandr costuma ser o nome limpo (ex: "DP-1")
				names = append(names, parts[len(parts)-1])
			}
		}
	}
	return names
}

// GetMonitores retorna a lista de todos os monitores conectados
func (a *App) GetMonitores() []Monitor {
	wmiNames := getMonitorNamesWMI()
	linuxNames := getMonitorNamesLinux()
	
	n := screenshot.NumActiveDisplays()
	var monitores []Monitor
	for i := 0; i < n; i++ {
		bounds := getDisplayBoundsX11(i)
		nome := fmt.Sprintf("Monitor %d", i+1)
		
		if runtime.GOOS == "windows" && i < len(wmiNames) {
			nome = wmiNames[i]
		} else if runtime.GOOS == "linux" && i < len(linuxNames) {
			nome = linuxNames[i]
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

// limitesMonitorAlvo devolve o retângulo (coordenadas absolutas da Root Window) do monitor de captura
// configurado, caindo no monitor 0 quando o alvo salvo não existe mais (ex.: monitor desconectado).
func (a *App) limitesMonitorAlvo() image.Rectangle {
	alvo := a.Config.MonitorAlvo
	if alvo < 0 || alvo >= screenshot.NumActiveDisplays() {
		alvo = 0
	}
	return getDisplayBoundsX11(alvo)
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
