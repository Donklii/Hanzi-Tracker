//go:build !windows && !linux

// Fora do Windows o overlay (janelas Win32 topmost desenhadas por cima do jogo) não existe: tudo aqui
// é no-op para o app compilar e funcionar sem os pop-ups sobre a tela. As funções que devolvem dados
// respondem "nada visível"; OcultarDestaquesTemporariamente ainda executa a ação recebida, porque o
// chamador depende dela (é o print da tela do scan de OCR).
package overlay

// ItemPopup descreve os dados necessários para exibir um card (espelha a struct da versão Windows).
type ItemPopup struct {
	Pinyin     string
	Hanzi      string
	Sig        string
	X0, Y0     int
	X1, Y1     int
	SoTraducao bool
}

func Iniciar()  {}
func Encerrar() {}

func MostrarHover(pinyin, hanzi, sig string, x, y int) {}
func OcultarHover()                                    {}

func MostrarResumo(titulo, texto, canto string, monX, monY, monW, monH int, ttlSec int) {}
func OcultarResumo()                                                                    {}

func MostrarTodos(itens []ItemPopup, sw, sh int) {}
func OcultarTodos()                              {}

func MostrarDestaque(x0, y0, x1, y1 int)              {}
func MostrarDestaquesEstudo(boxes [][]float64)        {}
func MostrarDestaquesEstudoParcial(boxes [][]float64) {}

// OcultarDestaquesTemporariamente não tem o que esconder, mas a ação (captura de tela do scan)
// precisa rodar mesmo assim.
func OcultarDestaquesTemporariamente(acao func()) {
	acao()
}

// RetangulosVisiveis devolve nil: sem janelas de overlay, não há área para censurar antes do OCR.
func RetangulosVisiveis() []Rect {
	return nil
}
