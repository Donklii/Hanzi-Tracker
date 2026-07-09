//go:build !linux

package main

import (
	"fmt"
	"image"
	"image/draw"

	"wails_app/overlay"

	"github.com/kbinani/screenshot"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// censurarRetangulo pinta de preto sólido a interseção entre `r` (coordenadas da tela) e a
// imagem `img`, cujo pixel (0,0) corresponde a (origemX, origemY) na tela.
func censurarRetangulo(img *image.RGBA, origemX, origemY int, r image.Rectangle) {
	local := r.Sub(image.Pt(origemX, origemY)).Intersect(img.Bounds())
	if local.Empty() {
		return
	}
	draw.Draw(img, local, image.Black, image.Point{}, draw.Src)
}

// retanguloAppNaTela devolve o retângulo da janela principal do app, ou
// ok=false se ela estiver minimizada.
func (a *App) retanguloAppNaTela() (image.Rectangle, bool) {
	if runtime.WindowIsMinimised(a.ctx) {
		return image.Rectangle{}, false
	}
	x, y := runtime.WindowGetPosition(a.ctx)
	w, h := runtime.WindowGetSize(a.ctx)
	return image.Rect(x, y, x+w, y+h), true
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

// capturarMonitorCensurado tira o print do monitor alvo (com os highlights do overlay escondidos
// para não serem re-lidos pelo OCR) e aplica a censura das janelas do app antes de devolver.
func (a *App) capturarMonitorCensurado() (*image.RGBA, image.Rectangle, error) {
	bounds := a.limitesMonitorAlvo()

	var img *image.RGBA
	var err error
	overlay.OcultarDestaquesTemporariamente(func() {
		img, err = screenshot.CaptureRect(bounds)
		if err == nil {
			a.censurarAreasSensiveis(img, bounds.Min.X, bounds.Min.Y)
		}
	})

	if err != nil {
		return nil, bounds, fmt.Errorf("failed to capture screen: %w", err)
	}
	return img, bounds, nil
}
