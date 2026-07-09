//go:build linux

package main

import (
	"fmt"
	"image"
	"image/draw"
	"os"

	"wails_app/overlay"

	"github.com/kbinani/screenshot"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// isWayland detecta se a sessão do desktop atual é Wayland, onde o comportamento
// de capturas e posições de janelas GTK difere consideravelmente do X11.
func isWayland() bool {
	return os.Getenv("XDG_SESSION_TYPE") == "wayland" || os.Getenv("WAYLAND_DISPLAY") != ""
}

// censurarRetangulo pinta de preto sólido a interseção entre `r` (coordenadas absolutas de tela) e a
// imagem `img`, cujo pixel (0,0) corresponde a (origemX, origemY) na tela absoluta.
func censurarRetangulo(img *image.RGBA, origemX, origemY int, r image.Rectangle) {
	// A imagem capturada pelo kbinani no Linux sempre tem Bounds().Min == (0,0).
	// origemX e origemY são as coordenadas ABSOLUTAS (Root Window) do monitor.
	// r é a coordenada ABSOLUTA do retângulo a ser censurado.
	deslocamento := image.Pt(-origemX, -origemY)
	local := r.Add(deslocamento).Intersect(img.Bounds())
	if local.Empty() {
		return
	}
	draw.Draw(img, local, image.Black, image.Point{}, draw.Src)
}

// retanguloAppNaTela devolve o retângulo (coordenadas ABSOLUTAS) da janela principal do app.
func (a *App) retanguloAppNaTela() (image.Rectangle, bool) {
	if runtime.WindowIsMinimised(a.ctx) {
		return image.Rectangle{}, false
	}
	if isWayland() {
		// O Wayland bloqueia a obtenção da posição absoluta real da janela por segurança.
		// Wails (via GTK) sempre nos retornará (0, 0), o que faria o retângulo apagar
		// cegamente uma grande área no canto superior esquerdo do print.
		return image.Rectangle{}, false
	}
	
	x, y := runtime.WindowGetPosition(a.ctx)
	w, h := runtime.WindowGetSize(a.ctx)
	
	// A janela GTK (Wails) reporta a posição no Linux com coordenadas pseudo-XRandR no X11.
	// Traduzimos para o espaço Absoluto da Root Window do X11 (para alinhar com os overlays e o print).
	r := traduzirParaAbsolutoX11(image.Rect(x, y, x+w, y+h))
	return r, true
}

// censurarAreasSensiveis apaga (preenche de preto) a área da janela principal do app e as áreas dos
// pop-ups do overlay dentro de `img`.
func (a *App) censurarAreasSensiveis(img *image.RGBA, origemX, origemY int) {
	if !a.Config.CensurarJanelasDoApp {
		return
	}

	if r, ok := a.retanguloAppNaTela(); ok {
		censurarRetangulo(img, origemX, origemY, r)
	}

	for _, r := range overlay.RetangulosVisiveis() {
		censurarRetangulo(img, origemX, origemY, image.Rect(r.X0, r.Y0, r.X1, r.Y1))
	}
}

// capturarMonitorCensurado tira o print do monitor alvo e aplica a censura das janelas do app.
func (a *App) capturarMonitorCensurado() (*image.RGBA, image.Rectangle, error) {
	// Bounds absolutos são usados para a matemática de censura e retorno (OCR usa relativos a ele)
	boundsAbsolutos := a.limitesMonitorAlvo()
	
	alvo := a.Config.MonitorAlvo
	if alvo < 0 || alvo >= screenshot.NumActiveDisplays() {
		alvo = 0
	}
	
	// A biblioteca kbinani/screenshot no Linux bifurca dependendo de Wayland/X11:
	// No Wayland, ela usa D-Bus/XDG Portal, que EXIGE as coordenadas absolutas (Root Window).
	// No X11, ela usa XCB e simula XRandR, exigindo as coordenadas pseudo-XRandR.
	capturaBounds := screenshot.GetDisplayBounds(alvo)
	if isWayland() {
		capturaBounds = boundsAbsolutos
	}

	var img *image.RGBA
	var err error
	overlay.OcultarDestaquesTemporariamente(func() {
		img, err = screenshot.CaptureRect(capturaBounds)
		if err == nil {
			a.censurarAreasSensiveis(img, boundsAbsolutos.Min.X, boundsAbsolutos.Min.Y)
		}
	})

	if err != nil {
		return nil, boundsAbsolutos, fmt.Errorf("failed to capture screen: %w", err)
	}
	return img, boundsAbsolutos, nil
}
