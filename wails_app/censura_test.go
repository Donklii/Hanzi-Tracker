package main

import (
	"image"
	"image/color"
	"testing"
)

func TestCensurarRetanguloDentroDaImagem(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 100, 100))
	branco := color.RGBA{R: 255, G: 255, B: 255, A: 255}
	for y := 0; y < 100; y++ {
		for x := 0; x < 100; x++ {
			img.Set(x, y, branco)
		}
	}

	// origemX=500, origemY=300: a imagem representa um monitor cujo canto superior esquerdo fica em
	// (500,300) na tela. O retângulo a censurar vem em coordenadas ABSOLUTAS de tela: (510,310)-(530,330)
	// absoluto vira (10,10)-(30,30) local.
	censurarRetangulo(img, 500, 300, image.Rect(510, 310, 530, 330))

	dentro := img.RGBAAt(15, 15)
	if dentro.R != 0 || dentro.G != 0 || dentro.B != 0 {
		t.Errorf("pixel dentro do retângulo censurado deveria ser preto, veio %+v", dentro)
	}

	fora := img.RGBAAt(50, 50)
	if fora.R != 255 || fora.G != 255 || fora.B != 255 {
		t.Errorf("pixel fora do retângulo censurado não deveria mudar, veio %+v", fora)
	}
}

func TestCensurarRetanguloForaDaImagemNaoQuebra(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 50, 50))
	// Retângulo inteiramente fora da imagem: Intersect deve dar Empty() e a função deve só devolver,
	// sem pânico e sem alterar nada.
	censurarRetangulo(img, 0, 0, image.Rect(1000, 1000, 1100, 1100))
}
