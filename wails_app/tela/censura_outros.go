//go:build !linux

package tela

import (
	"fmt"
	"image"
	"image/draw"

	"wails_app/overlay"

	"github.com/kbinani/screenshot"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// censurarRetangulo pinta de preto sólido a interseção entre `r` (coordenadas da tela) e a
// imagem `img`, cujo pixel (0,0) corresponde a (origemX, origemY) na tela. Devolve a região
// pintada em coordenadas locais da imagem (vazia quando não há interseção).
func censurarRetangulo(img *image.RGBA, origemX, origemY int, r image.Rectangle) image.Rectangle {
	local := r.Sub(image.Pt(origemX, origemY)).Intersect(img.Bounds())
	if local.Empty() {
		return image.Rectangle{}
	}
	draw.Draw(img, local, image.Black, image.Point{}, draw.Src)
	return local
}

// RetanguloAppNaTela devolve o retângulo da janela principal do app, ou
// ok=false se ela estiver minimizada.
func (gt *GerenciadorTela) RetanguloAppNaTela() (image.Rectangle, bool) {
	if runtime.WindowIsMinimised(gt.ctx) {
		return image.Rectangle{}, false
	}
	x, y := runtime.WindowGetPosition(gt.ctx)
	w, h := runtime.WindowGetSize(gt.ctx)
	return image.Rect(x, y, x+w, y+h), true
}

// CensurarAreasSensiveis apaga (preenche de preto) a área da janela principal do app e as áreas dos
// pop-ups do overlay dentro de `img`. Devolve as regiões pintadas, em coordenadas locais da imagem —
// o vigia congela o tick dos cards embaixo delas (o preto da censura não prova que a palavra saiu).
func (gt *GerenciadorTela) CensurarAreasSensiveis(img *image.RGBA, origemX, origemY int) []image.Rectangle {
	cfg := gt.obterConfig()
	if !cfg.CensurarJanelasDoApp {
		return nil
	}

	var censuradas []image.Rectangle
	if r, ok := gt.RetanguloAppNaTela(); ok {
		if local := censurarRetangulo(img, origemX, origemY, r); !local.Empty() {
			censuradas = append(censuradas, local)
		}
	}

	for _, r := range overlay.RetangulosParaCensura() {
		if local := censurarRetangulo(img, origemX, origemY, image.Rect(r.X0, r.Y0, r.X1, r.Y1)); !local.Empty() {
			censuradas = append(censuradas, local)
		}
	}
	return censuradas
}

// CapturarMonitorCensurado tira o print do monitor alvo, aplica a censura das janelas do app e
// devolve também as regiões censuradas (coordenadas locais do print) — a fonte de verdade do que
// foi de fato pintado NESTA captura, usada pelo vigia para congelar os cards embaixo delas.
func (gt *GerenciadorTela) CapturarMonitorCensurado() (*image.RGBA, image.Rectangle, []image.Rectangle, error) {
	cfg := gt.obterConfig()
	bounds := LimitesMonitorAlvo(cfg.MonitorAlvo)

	var img *image.RGBA
	var censuradas []image.Rectangle
	var err error
	overlay.ExecutarCapturaSemOverlays(func() {
		img, err = screenshot.CaptureRect(bounds)
		if err == nil {
			censuradas = gt.CensurarAreasSensiveis(img, bounds.Min.X, bounds.Min.Y)
		}
	})

	if err != nil {
		return nil, bounds, nil, fmt.Errorf("failed to capture screen: %w", err)
	}
	return img, bounds, censuradas, nil
}
