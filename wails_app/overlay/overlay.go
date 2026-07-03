//go:build windows

package overlay

import (
	"runtime"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"github.com/lxn/win"
)

var (
	threadID uint32
	actionQ  = make(chan func(), 100)
	ready    chan struct{}
	mu       sync.Mutex

	janelasTodos           []win.HWND
	janelasEstudos         []win.HWND
	janelasEstudosParciais []win.HWND
	janelaHover            win.HWND
	janelaHighlight        win.HWND
)

// Iniciar arranca a thread de interface (Win32) do overlay e registra a Window Class.
func Iniciar() {
	mu.Lock()
	if ready != nil {
		mu.Unlock()
		return
	}
	ready = make(chan struct{})
	mu.Unlock()

	go func() {
		runtime.LockOSThread()
		threadID = win.GetCurrentThreadId()

		// Registrar classe da janela
		var wc win.WNDCLASSEX
		wc.CbSize = uint32(unsafe.Sizeof(wc))
		wc.LpfnWndProc = syscall.NewCallback(wndProc)
		wc.HInstance = win.GetModuleHandle(nil)
		wc.HCursor = win.LoadCursor(0, (*uint16)(unsafe.Pointer(uintptr(win.IDC_ARROW))))
		wc.HbrBackground = win.HBRUSH(createSolidBrush(win.RGB(255, 0, 255))) // Cor chave Magenta
		wc.LpszClassName = utf16PtrFromString(className)
		win.RegisterClassEx(&wc)

		close(ready)

		var msg win.MSG
		for win.GetMessage(&msg, 0, 0, 0) != 0 {
			if msg.Message == WM_APP_WAKEUP {
				drainActions()
			}
			win.TranslateMessage(&msg)
			win.DispatchMessage(&msg)
		}
	}()
	<-ready
}

// Encerrar fecha as janelas e a thread de eventos.
func Encerrar() {
	if threadID != 0 {
		postThreadMessage(threadID, win.WM_QUIT, 0, 0)
	}
}

// execNaThread enfileira uma função para rodar na thread do Win32.
func execNaThread(fn func()) {
	actionQ <- fn
	if threadID != 0 {
		postThreadMessage(threadID, WM_APP_WAKEUP, 0, 0)
	}
}

func drainActions() {
	for {
		select {
		case fn := <-actionQ:
			fn()
		default:
			return
		}
	}
}

// limpaJanelas destrói um slice de janelas
func limpaJanelas(lista *[]win.HWND) {
	for _, h := range *lista {
		win.DestroyWindow(h)
	}
	*lista = nil
}

// ItemPopup descreve os dados necessários para exibir um card (similar ao JSON dict em Python).
type ItemPopup struct {
	Pinyin string
	Hanzi  string
	Sig    string
	X0, Y0 int
	X1, Y1 int
}

// Show exibe o card de hover na posição (x,y).
func Show(pinyin, hanzi, sig string, x, y int) {
	execNaThread(func() {
		if janelaHighlight != 0 {
			win.ShowWindow(janelaHighlight, win.SW_HIDE)
		}
		if janelaHover != 0 {
			win.DestroyWindow(janelaHover)
		}

		w, h := medirCard(pinyin, hanzi, sig, 1.0)
		
		finalX := x - (w / 2)
		finalY := y - h - 10
		if finalY < 0 {
			finalY = y + 30
		}

		janelaHover = createWindow(className, "HanziTrackerHover", finalX, finalY, w, h, true)
		windowDatas[janelaHover] = &WindowData{
			Type:   1,
			Pinyin: pinyin,
			Hanzi:  hanzi,
			Sig:    sig,
			Escala: 1.0,
		}

		win.ShowWindow(janelaHover, win.SW_SHOWNOACTIVATE)
		win.UpdateWindow(janelaHover)
	})
}

// Hide oculta o card de hover.
func Hide() {
	execNaThread(func() {
		if janelaHover != 0 {
			win.ShowWindow(janelaHover, win.SW_HIDE)
		}
		if janelaHighlight != 0 {
			win.ShowWindow(janelaHighlight, win.SW_HIDE)
		}
	})
}

// MostrarTodos posiciona inteligentemente e exibe os cards para um conjunto de itens.
func MostrarTodos(itens []ItemPopup, sw, sh int) {
	execNaThread(func() {
		limpaJanelas(&janelasTodos)
		escalas := []float64{1.0, 0.8, 0.65, 0.5}
		var colocadas []Rect

		for _, item := range itens {
			centroX := (item.X0 + item.X1) / 2
			colocado := false
			var w, h, preferX, preferY int

			for _, escala := range escalas {
				w, h = medirCard(item.Pinyin, item.Hanzi, item.Sig, escala)
				preferX = centroX - w/2
				preferY = item.Y0 - h - 6
				if preferY < 0 {
					preferY = item.Y1 + 6
				}

				x, y, rect, ok := AcharPosicao(preferX, preferY, w, h, colocadas, sw, sh)
				if ok {
					hwnd := createWindow(className, "HanziTrackerCard", x, y, w, h, true)
					windowDatas[hwnd] = &WindowData{
						Type:   3,
						Pinyin: item.Pinyin,
						Hanzi:  item.Hanzi,
						Sig:    item.Sig,
						Escala: escala,
					}
					win.ShowWindow(hwnd, win.SW_SHOWNOACTIVATE)
					janelasTodos = append(janelasTodos, hwnd)
					colocadas = append(colocadas, rect)
					colocado = true
					break
				}
			}

			if !colocado {
				// Aceita sobreposição se nem na menor escala encontrou lugar
				x := max(0, min(preferX, sw-w))
				y := max(0, min(preferY, sh-h))
				hwnd := createWindow(className, "HanziTrackerCard", x, y, w, h, true)
				windowDatas[hwnd] = &WindowData{
					Type:   3,
					Pinyin: item.Pinyin,
					Hanzi:  item.Hanzi,
					Sig:    item.Sig,
					Escala: 0.5,
				}
				win.ShowWindow(hwnd, win.SW_SHOWNOACTIVATE)
				janelasTodos = append(janelasTodos, hwnd)
				colocadas = append(colocadas, Rect{X0: x, Y0: y, X1: x + w, Y1: y + h})
			}
		}
	})
}

// OcultarTodos remove as janelas abertas por MostrarTodos.
func OcultarTodos() {
	execNaThread(func() {
		limpaJanelas(&janelasTodos)
	})
}

// ShowHighlight exibe uma moldura vazada (verde) na posição selecionada.
func ShowHighlight(x0, y0, x1, y1 int) {
	execNaThread(func() {
		if janelaHover != 0 {
			win.ShowWindow(janelaHover, win.SW_HIDE)
		}
		if janelaHighlight != 0 {
			win.DestroyWindow(janelaHighlight)
		}

		w := x1 - x0
		h := y1 - y0
		if w <= 0 {
			w = 1
		}
		if h <= 0 {
			h = 1
		}

		janelaHighlight = createWindow(className, "HanziTrackerHighlight", x0, y0, w, h, true)
		windowDatas[janelaHighlight] = &WindowData{Type: 2}
		win.ShowWindow(janelaHighlight, win.SW_SHOWNOACTIVATE)
		win.UpdateWindow(janelaHighlight)
	})
}

// ShowEstudoHighlights exibe várias molduras vazadas (azuis) simultaneamente.
func ShowEstudoHighlights(boxes [][]float64) {
	execNaThread(func() {
		limpaJanelas(&janelasEstudos)
		for _, box := range boxes {
			if len(box) != 4 {
				continue
			}
			x0, y0, x1, y1 := int(box[0]), int(box[1]), int(box[2]), int(box[3])
			w := x1 - x0
			h := y1 - y0
			if w <= 0 {
				w = 1
			}
			if h <= 0 {
				h = 1
			}

			hwnd := createWindow(className, "HanziTrackerEstudo", x0, y0, w, h, true)
			windowDatas[hwnd] = &WindowData{Type: 4}
			win.ShowWindow(hwnd, win.SW_SHOWNOACTIVATE)
			janelasEstudos = append(janelasEstudos, hwnd)
		}
	})
}

// ShowEstudoParcialHighlights exibe várias molduras vazadas (amarelas) simultaneamente.
func ShowEstudoParcialHighlights(boxes [][]float64) {
	execNaThread(func() {
		limpaJanelas(&janelasEstudosParciais)
		for _, box := range boxes {
			if len(box) != 4 {
				continue
			}
			x0, y0, x1, y1 := int(box[0]), int(box[1]), int(box[2]), int(box[3])
			w := x1 - x0
			h := y1 - y0
			if w <= 0 {
				w = 1
			}
			if h <= 0 {
				h = 1
			}

			hwnd := createWindow(className, "HanziTrackerEstudoParcial", x0, y0, w, h, true)
			windowDatas[hwnd] = &WindowData{Type: 5}
			win.ShowWindow(hwnd, win.SW_SHOWNOACTIVATE)
			janelasEstudosParciais = append(janelasEstudosParciais, hwnd)
		}
	})
}

// OcultarHighlightsTemporariamente esconde os highlights (borders), aguarda a renderização
// para garantir que saiam da tela, roda a acao (print da tela) e os restaura. 
func OcultarHighlightsTemporariamente(acao func()) {
	if threadID == 0 {
		acao()
		return
	}

	var escondidas []win.HWND
	done := make(chan struct{})
	
	execNaThread(func() {
		esconder := func(h win.HWND) {
			if h != 0 && win.IsWindowVisible(h) {
				win.ShowWindow(h, win.SW_HIDE)
				escondidas = append(escondidas, h)
			}
		}
		esconder(janelaHighlight)
		for _, h := range janelasEstudos {
			esconder(h)
		}
		for _, h := range janelasEstudosParciais {
			esconder(h)
		}
		done <- struct{}{}
	})
	<-done // Aguarda chamadas completarem

	// Pequeno delay para o Desktop Window Manager (DWM) atualizar o frame visualmente
	time.Sleep(30 * time.Millisecond)

	acao()

	execNaThread(func() {
		for _, h := range escondidas {
			if h != 0 {
				win.ShowWindow(h, win.SW_SHOWNOACTIVATE)
			}
		}
	})
}

// RetangulosVisiveis devolve, em coordenadas ABSOLUTAS de tela, os retângulos de todas as janelas do
// overlay atualmente visíveis (hover, destaques verdes/azuis e os cards do "mostrar tudo"). Usado para
// censurar essas áreas antes de enviar a captura de tela ao OCR — sem isso, o próprio pop-up (sempre
// topmost) poderia ser lido de volta pelo OCR no scan seguinte.
func RetangulosVisiveis() []Rect {
	// threadID só é != 0 depois de Iniciar(); sem thread de overlay rodando não existe nenhuma janela
	// criada, e enfileirar em execNaThread aqui travaria para sempre esperando uma função que nunca
	// seria executada (ninguém chamaria drainActions). Por isso este guard vem ANTES de tudo.
	if threadID == 0 {
		return nil
	}

	resultado := make(chan []Rect, 1)
	execNaThread(func() {
		var rects []Rect
		coletar := func(h win.HWND) {
			if h == 0 || !win.IsWindowVisible(h) {
				return
			}
			var r win.RECT
			if win.GetWindowRect(h, &r) {
				rects = append(rects, Rect{X0: int(r.Left), Y0: int(r.Top), X1: int(r.Right), Y1: int(r.Bottom)})
			}
		}

		coletar(janelaHover)
		coletar(janelaHighlight)
		for _, h := range janelasTodos {
			coletar(h)
		}
		for _, h := range janelasEstudos {
			coletar(h)
		}
		for _, h := range janelasEstudosParciais {
			coletar(h)
		}

		resultado <- rects
	})
	return <-resultado
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
