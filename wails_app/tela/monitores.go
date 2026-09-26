package tela

import (
	"fmt"
	"image"
	"os/exec"
	"runtime"
	"strings"

	"github.com/kbinani/screenshot"
)

// ----- Monitores e resolução de captura -----

type Monitor struct {
	ID      int    `json:"id"`
	Nome    string `json:"nome"`
	Largura int    `json:"largura"`
	Altura  int    `json:"altura"`
	X       int    `json:"x"`
	Y       int    `json:"y"`
}

type Resolucao struct {
	Largura int `json:"largura"`
	Altura  int `json:"altura"`
}

// GetDisplayBoundsX11 é um wrapper sobre screenshot.GetDisplayBounds que resolve o problem do X11 no Linux.
func GetDisplayBoundsX11(index int) image.Rectangle {
	bounds := screenshot.GetDisplayBounds(index)
	return TraduzirParaAbsolutoX11(bounds)
}

// TraduzirParaAbsolutoX11 converte um retângulo do sistema de coordenadas pseudo-XRandR
// para coordenadas absolutas da Root Window do X11.
func TraduzirParaAbsolutoX11(r image.Rectangle) image.Rectangle {
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

// ListarMonitores retorna a lista de todos os monitores conectados.
func ListarMonitores() []Monitor {
	wmiNames := getMonitorNamesWMI()
	linuxNames := getMonitorNamesLinux()
	
	n := screenshot.NumActiveDisplays()
	var monitores []Monitor
	for i := 0; i < n; i++ {
		bounds := GetDisplayBoundsX11(i)
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

// LimitesMonitorAlvo devolve o retângulo do monitor alvo configurado.
func LimitesMonitorAlvo(monitorAlvo int) image.Rectangle {
	alvo := monitorAlvo
	if alvo < 0 || alvo >= screenshot.NumActiveDisplays() {
		alvo = 0
	}
	return GetDisplayBoundsX11(alvo)
}

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
		if len(line) > 0 && strings.Contains(line, ":") {
			parts := strings.Fields(line)
			if len(parts) > 0 {
				names = append(names, parts[len(parts)-1])
			}
		}
	}
	return names
}

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
