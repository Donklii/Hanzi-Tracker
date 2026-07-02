package overlay

// Rect representa um retângulo em coordenadas de tela.
type Rect struct {
	X0, Y0, X1, Y1 int
}

// Colide verifica se o retângulo 'a' colide com o retângulo 'b' usando uma margem extra.
func Colide(a, b Rect, margem int) bool {
	return !(a.X1+margem <= b.X0 ||
		b.X1+margem <= a.X0 ||
		a.Y1+margem <= b.Y0 ||
		b.Y1+margem <= a.Y0)
}

// AcharPosicao tenta encontrar um espaço livre (sem colisão com a lista `colocadas`)
// ao redor da posição preferida (preferX, preferY). O algoritmo empilha verticalmente.
func AcharPosicao(preferX, preferY, w, h int, colocadas []Rect, sw, sh int) (int, int, Rect, bool) {
	desloc := h + 8
	multiplicadores := []int{0, -1, 1, -2, 2, -3, 3, -4, 4, -5, 5}

	for _, mult := range multiplicadores {
		x := preferX
		if x < 0 {
			x = 0
		} else if x > sw-w {
			x = sw - w
		}

		y := preferY + (mult * desloc)
		if y < 0 {
			y = 0
		} else if y > sh-h {
			y = sh - h
		}

		rect := Rect{X0: x, Y0: y, X1: x + w, Y1: y + h}

		colidiu := false
		for _, r := range colocadas {
			if Colide(rect, r, 4) {
				colidiu = true
				break
			}
		}

		if !colidiu {
			return x, y, rect, true
		}
	}
	return 0, 0, Rect{}, false
}

// max retorna o maior entre dois inteiros.
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
