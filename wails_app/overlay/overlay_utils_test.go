package overlay

import (
	"testing"
)

func TestColide(t *testing.T) {
	tests := []struct {
		name     string
		a        Rect
		b        Rect
		margem   int
		expected bool
	}{
		{
			name:     "Sem colisão (longe)",
			a:        Rect{X0: 0, Y0: 0, X1: 10, Y1: 10},
			b:        Rect{X0: 20, Y0: 20, X1: 30, Y1: 30},
			margem:   4,
			expected: false,
		},
		{
			name:     "Colisão total (sobreposição)",
			a:        Rect{X0: 0, Y0: 0, X1: 20, Y1: 20},
			b:        Rect{X0: 10, Y0: 10, X1: 30, Y1: 30},
			margem:   4,
			expected: true,
		},
		{
			name:     "Sem colisão (exatamente lado a lado sem margem)",
			a:        Rect{X0: 0, Y0: 0, X1: 10, Y1: 10},
			b:        Rect{X0: 10, Y0: 0, X1: 20, Y1: 10},
			margem:   0,
			expected: false,
		},
		{
			name:     "Colisão com margem (lado a lado, mas margem faz colidir)",
			a:        Rect{X0: 0, Y0: 0, X1: 10, Y1: 10},
			b:        Rect{X0: 12, Y0: 0, X1: 20, Y1: 10},
			margem:   4,
			expected: true, // X1(10)+4=14 > X0(12), então não escapou
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Colide(tt.a, tt.b, tt.margem)
			if result != tt.expected {
				t.Errorf("Colide() = %v, esperado %v", result, tt.expected)
			}
		})
	}
}

func TestAcharPosicao(t *testing.T) {
	sw, sh := 1920, 1080
	tests := []struct {
		name        string
		preferX     int
		preferY     int
		w           int
		h           int
		colocadas   []Rect
		expectedOk  bool
		expectedY0  int // para verificar o comportamento de empilhamento
	}{
		{
			name:       "Livre na posição preferida",
			preferX:    100,
			preferY:    100,
			w:          50,
			h:          20,
			colocadas:  []Rect{},
			expectedOk: true,
			expectedY0: 100,
		},
		{
			name:       "Colisão na preferida, empilha para cima (-1 desloc)",
			preferX:    100,
			preferY:    100,
			w:          50,
			h:          20,
			colocadas:  []Rect{{X0: 100, Y0: 100, X1: 150, Y1: 120}},
			expectedOk: true,
			expectedY0: 100 - (20 + 8), // preferY - desloc
		},
		{
			name:       "Limite da tela (X e Y)",
			preferX:    2000, // Fora do limite
			preferY:    2000,
			w:          50,
			h:          20,
			colocadas:  []Rect{},
			expectedOk: true,
			expectedY0: 1080 - 20, // sh - h
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, y, _, ok := AcharPosicao(tt.preferX, tt.preferY, tt.w, tt.h, tt.colocadas, sw, sh)
			if ok != tt.expectedOk {
				t.Errorf("AcharPosicao() ok = %v, esperado %v", ok, tt.expectedOk)
			}
			if ok && y != tt.expectedY0 {
				t.Errorf("AcharPosicao() y = %v, esperado %v", y, tt.expectedY0)
			}
		})
	}
}
