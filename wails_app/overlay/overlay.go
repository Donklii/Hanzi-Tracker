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
	resumoCancel chan struct{}
	resumoMu     sync.Mutex
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
	janelaResumo           win.HWND
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
		wc.HCursor = win.LoadCursor(0, win.MAKEINTRESOURCE(win.IDC_ARROW))
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
	Pinyin     string
	Hanzi      string
	Sig        string
	X0, Y0     int
	X1, Y1     int
	SoTraducao bool // Se true, exibe apenas a tradução (Sig), sem Hanzi/Pinyin, no tamanho da linha de origem.
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

		janelaHover = createWindow(className, "HanziTrackerHover", finalX, finalY, w, h, true, 255)
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

// MostrarResumo exibe o resumo gerado pelo Gemini em um canto da tela.
// Se ttlSec for 0, o pop-up não tem TTL (fica na tela até ser sobrescrito ou ocultado).
// Se ttlSec for < 0, calcula o TTL automaticamente baseado no texto.
func MostrarResumo(titulo, texto, canto string, monX, monY, monW, monH int, ttlSec int) {
	resumoMu.Lock()
	if resumoCancel != nil {
		close(resumoCancel)
		resumoCancel = nil
	}

	if ttlSec < 0 {
		// Tempo de leitura: ~15 caracteres por segundo, mínimo de 10 segundos
		ttlSec = max(10, len(texto)/15)
	}

	var newCancel chan struct{}
	if ttlSec > 0 {
		newCancel = make(chan struct{})
		resumoCancel = newCancel
	}
	resumoMu.Unlock()

	execNaThread(func() {
		w := 420

		hdc := win.CreateDC(utf16PtrFromString("DISPLAY"), nil, nil, nil)
		fonteTitulo := createFont(15, true)
		fonteTexto := createFont(12, false)

		_, hTitulo := calcTextRect(hdc, titulo, fonteTitulo, w-(2*16))
		_, hTexto := calcTextRect(hdc, texto, fonteTexto, w-(2*16))

		win.DeleteObject(win.HGDIOBJ(fonteTitulo))
		win.DeleteObject(win.HGDIOBJ(fonteTexto))
		win.DeleteDC(hdc)

		h := 16 + hTitulo + 8 + hTexto + 16
		maxH := int(float64(monH) * 0.45)
		if h > maxH {
			h = maxH
		}

		var x, y int
		switch canto {
		case "superior-esquerdo":
			x = monX + 16
			y = monY + 16
		case "superior-direito":
			x = monX + monW - 16 - w
			y = monY + 16
		case "inferior-esquerdo":
			x = monX + 16
			y = monY + monH - 16 - h
		default: // "inferior-direito" e outros
			x = monX + monW - 16 - w
			y = monY + monH - 16 - h
		}

		if janelaResumo == 0 {
			janelaResumo = createWindow(className, "HanziTrackerResumo", x, y, w, h, true, 245)
			windowDatas[janelaResumo] = &WindowData{
				Type:   6,
				Hanzi:  titulo,
				Sig:    texto,
				Escala: 1.0,
			}
			win.ShowWindow(janelaResumo, win.SW_SHOWNOACTIVATE)
		} else {
			win.SetWindowPos(janelaResumo, 0, int32(x), int32(y), int32(w), int32(h), win.SWP_NOZORDER|win.SWP_NOACTIVATE|win.SWP_SHOWWINDOW)
			data := windowDatas[janelaResumo]
			data.Hanzi = titulo
			data.Sig = texto
			win.InvalidateRect(janelaResumo, nil, true)
		}
		win.UpdateWindow(janelaResumo)
	})

	if ttlSec > 0 {
		go func() {
			ticker := time.NewTicker(200 * time.Millisecond)
			defer ticker.Stop()

			remaining := time.Duration(ttlSec) * time.Second

			for {
				select {
				case <-newCancel:
					return // cancelado por outra chamada
				case <-ticker.C:
					isHovering := false
					var pt win.POINT
					if win.GetCursorPos(&pt) {
						mx, my := int(pt.X), int(pt.Y)
						hwnd := janelaResumo // leitura simples (não sincronizada rigorosamente, mas ok em Win32)
						if hwnd != 0 {
							var r win.RECT
							if win.GetWindowRect(hwnd, &r) {
								// Adiciona 15px de margem de tolerância ao redor do pop-up
								if mx >= int(r.Left)-15 && mx <= int(r.Right)+15 && my >= int(r.Top)-15 && my <= int(r.Bottom)+15 {
									isHovering = true
								}
							}
						} else {
							// janela ainda não criada, pausa o timer
							isHovering = true
						}
					}

					if !isHovering {
						remaining -= 200 * time.Millisecond
						if remaining <= 0 {
							OcultarResumo()
							return
						}
					}
				}
			}
		}()
	}
}

// MostrarTodos posiciona inteligentemente e exibe os cards para um conjunto de itens.
func MostrarTodos(itens []ItemPopup, sw, sh int) {
	execNaThread(func() {
		limpaJanelas(&janelasTodos)
		escalas := []float64{1.0, 0.8, 0.65, 0.5}
		var colocadas []Rect

		for _, item := range itens {
			// Modo tradução-somente: popup com largura da linha de origem, logo abaixo dela.
			if item.SoTraducao {
				larLinha := item.X1 - item.X0
				if larLinha < 60 {
					larLinha = 60
				}
				w, h := medirCardTraducao(item.Sig, larLinha)
				x := item.X0
				y := item.Y1 + 2 // logo abaixo da linha
				// Garante que não sai da tela
				if x+w > sw {
					x = sw - w
				}
				if x < 0 {
					x = 0
				}
				if y+h > sh {
					y = item.Y0 - h - 2 // acima da linha se não couber abaixo
				}
				if y < 0 {
					y = 0
				}

				hwnd := createWindow(className, "HanziTrackerCard", x, y, w, h, true, 255)
				windowDatas[hwnd] = &WindowData{
					Type:   7, // Tradução-somente
					Sig:    item.Sig,
					Escala: 1.0,
				}
				win.ShowWindow(hwnd, win.SW_SHOWNOACTIVATE)
				janelasTodos = append(janelasTodos, hwnd)
				colocadas = append(colocadas, Rect{X0: x, Y0: y, X1: x + w, Y1: y + h})
				continue
			}

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
					hwnd := createWindow(className, "HanziTrackerCard", x, y, w, h, true, 255)
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
				hwnd := createWindow(className, "HanziTrackerCard", x, y, w, h, true, 255)
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

// OcultarResumo oculta o pop-up de resumo.
func OcultarResumo() {
	resumoMu.Lock()
	if resumoCancel != nil {
		close(resumoCancel)
		resumoCancel = nil
	}
	resumoMu.Unlock()

	execNaThread(func() {
		if janelaResumo != 0 {
			win.DestroyWindow(janelaResumo)
			janelaResumo = 0
		}
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

		janelaHighlight = createWindow(className, "HanziTrackerHighlight", x0, y0, w, h, true, 255)
		windowDatas[janelaHighlight] = &WindowData{Type: 2}
		win.ShowWindow(janelaHighlight, win.SW_SHOWNOACTIVATE)
		win.UpdateWindow(janelaHighlight)
	})
}

// atualizarMolduras reutiliza as janelas (HWNDs) existentes para evitar flicker (piscar).
func atualizarMolduras(lista *[]win.HWND, boxes [][]float64, windowName string, tipo int) {
	type WinItem struct {
		hwnd win.HWND
		r    win.RECT
		used bool
	}

	itens := make([]*WinItem, len(*lista))
	for i, h := range *lista {
		var r win.RECT
		win.GetWindowRect(h, &r)
		itens[i] = &WinItem{hwnd: h, r: r, used: false}
	}

	var novasJanelas []win.HWND
	var extras []win.HWND

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

		encontrou := false
		// 1. Tenta achar uma janela exatamente na mesma posição e tamanho
		for _, item := range itens {
			if !item.used && int(item.r.Left) == x0 && int(item.r.Top) == y0 && int(item.r.Right) == x0+w && int(item.r.Bottom) == y0+h {
				item.used = true
				encontrou = true
				if !win.IsWindowVisible(item.hwnd) {
					win.ShowWindow(item.hwnd, win.SW_SHOWNOACTIVATE)
				}
				novasJanelas = append(novasJanelas, item.hwnd)
				break
			}
		}

		// 2. Se não achou exata, pega qualquer uma não usada e reposiciona
		if !encontrou {
			for _, item := range itens {
				if !item.used {
					item.used = true
					encontrou = true
					win.SetWindowPos(item.hwnd, 0, int32(x0), int32(y0), int32(w), int32(h), win.SWP_NOZORDER|win.SWP_NOACTIVATE|win.SWP_SHOWWINDOW)
					win.InvalidateRect(item.hwnd, nil, true)
					win.UpdateWindow(item.hwnd)
					novasJanelas = append(novasJanelas, item.hwnd)
					break
				}
			}
		}

		// 3. Se não tem mais janelas livres, cria uma nova
		if !encontrou {
			alpha := byte(255)
			if tipo == 4 || tipo == 5 {
				alpha = 150 // Semi-transparente para as caixas de estudo automáticas
			}
			hwnd := createWindow(className, windowName, x0, y0, w, h, true, alpha)
			windowDatas[hwnd] = &WindowData{Type: tipo}
			win.ShowWindow(hwnd, win.SW_SHOWNOACTIVATE)
			novasJanelas = append(novasJanelas, hwnd)
		}
	}

	// Esconde as janelas que sobraram em vez de destruir, mantendo no pool
	for _, item := range itens {
		if !item.used {
			if win.IsWindowVisible(item.hwnd) {
				win.ShowWindow(item.hwnd, win.SW_HIDE)
			}
			extras = append(extras, item.hwnd)
		}
	}

	*lista = append(novasJanelas, extras...)
}

// ShowEstudoHighlights exibe várias molduras vazadas (azuis) simultaneamente.
func ShowEstudoHighlights(boxes [][]float64) {
	execNaThread(func() {
		atualizarMolduras(&janelasEstudos, boxes, "HanziTrackerEstudo", 4)
	})
}

// ShowEstudoParcialHighlights exibe várias molduras vazadas (amarelas) simultaneamente.
func ShowEstudoParcialHighlights(boxes [][]float64) {
	execNaThread(func() {
		atualizarMolduras(&janelasEstudosParciais, boxes, "HanziTrackerEstudoParcial", 5)
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
		coletar(janelaResumo)
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
