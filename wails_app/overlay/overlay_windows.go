//go:build windows

package overlay

import (
	"syscall"
	"unsafe"

	"github.com/lxn/win"
)

// Estruturas e constantes Win32 adicionais ou utilitárias

const (
	WM_APP_WAKEUP = win.WM_APP + 1
	LWA_COLORKEY  = 1
)

var (
	user32 = syscall.NewLazyDLL("user32.dll")
	gdi32  = syscall.NewLazyDLL("gdi32.dll")

	procPostThreadMessageW         = user32.NewProc("PostThreadMessageW")
	procSetLayeredWindowAttributes = user32.NewProc("SetLayeredWindowAttributes")
	procDrawTextW                  = user32.NewProc("DrawTextW")
	procFillRect                   = user32.NewProc("FillRect")

	procCreateSolidBrush = gdi32.NewProc("CreateSolidBrush")
	procCreateFontW      = gdi32.NewProc("CreateFontW")
)

func postThreadMessage(idThread uint32, msg uint32, wParam, lParam uintptr) bool {
	ret, _, _ := procPostThreadMessageW.Call(uintptr(idThread), uintptr(msg), wParam, lParam)
	return ret != 0
}

func setLayeredWindowAttributes(hwnd win.HWND, crKey win.COLORREF, bAlpha byte, dwFlags uint32) bool {
	ret, _, _ := procSetLayeredWindowAttributes.Call(uintptr(hwnd), uintptr(crKey), uintptr(bAlpha), uintptr(dwFlags))
	return ret != 0
}

func createSolidBrush(color win.COLORREF) win.HBRUSH {
	ret, _, _ := procCreateSolidBrush.Call(uintptr(color))
	return win.HBRUSH(ret)
}

func drawText(hdc win.HDC, text string, nCount int32, lpRect *win.RECT, uFormat uint32) int32 {
	ret, _, _ := procDrawTextW.Call(
		uintptr(hdc),
		uintptr(unsafe.Pointer(utf16PtrFromString(text))),
		uintptr(nCount),
		uintptr(unsafe.Pointer(lpRect)),
		uintptr(uFormat),
	)
	return int32(ret)
}
func utf16PtrFromString(s string) *uint16 {
	p, _ := syscall.UTF16PtrFromString(s)
	return p
}

func rgb(r, g, b byte) win.COLORREF {
	return win.RGB(r, g, b)
}

func getSystemMetrics(index int32) int {
	return int(win.GetSystemMetrics(index))
}

func createFont(tamanho int, negrito bool) win.HFONT {
	weight := win.FW_NORMAL
	if negrito {
		weight = win.FW_BOLD
	}
	ret, _, _ := procCreateFontW.Call(
		uintptr(int32(-tamanho)), 0, 0, 0,
		uintptr(int32(weight)), 0, 0, 0,
		uintptr(win.DEFAULT_CHARSET), uintptr(win.OUT_DEFAULT_PRECIS), uintptr(win.CLIP_DEFAULT_PRECIS),
		uintptr(win.CLEARTYPE_QUALITY), uintptr(win.DEFAULT_PITCH|win.FF_DONTCARE),
		uintptr(unsafe.Pointer(utf16PtrFromString("Segoe UI"))),
	)
	return win.HFONT(ret)
}

func fillRect(hdc win.HDC, rect *win.RECT, color win.COLORREF) {
	hbr := createSolidBrush(color)
	procFillRect.Call(uintptr(hdc), uintptr(unsafe.Pointer(rect)), uintptr(hbr))
	win.DeleteObject(win.HGDIOBJ(hbr))
}

func drawFrame(hdc win.HDC, rect *win.RECT, color win.COLORREF, espessura int32) {
	// FrameRect desenha apenas 1 px. Para espessura, vamos usar FillRect em 4 retângulos.
	// Top
	fillRect(hdc, &win.RECT{Left: rect.Left, Top: rect.Top, Right: rect.Right, Bottom: rect.Top + espessura}, color)
	// Bottom
	fillRect(hdc, &win.RECT{Left: rect.Left, Top: rect.Bottom - espessura, Right: rect.Right, Bottom: rect.Bottom}, color)
	// Left
	fillRect(hdc, &win.RECT{Left: rect.Left, Top: rect.Top, Right: rect.Left + espessura, Bottom: rect.Bottom}, color)
	// Right
	fillRect(hdc, &win.RECT{Left: rect.Right - espessura, Top: rect.Top, Right: rect.Right, Bottom: rect.Bottom}, color)
}

func drawTextCentered(hdc win.HDC, text string, rect *win.RECT, color win.COLORREF, hfont win.HFONT, wrap bool) int {
	if text == "" {
		return 0
	}
	win.SetTextColor(hdc, color)
	win.SetBkMode(hdc, win.TRANSPARENT)
	win.SelectObject(hdc, win.HGDIOBJ(hfont))

	flags := uint32(win.DT_CENTER)
	if wrap {
		flags |= win.DT_WORDBREAK
	} else {
		flags |= win.DT_SINGLELINE | win.DT_VCENTER
	}

	altura := drawText(hdc, text, -1, rect, flags)
	return int(altura)
}

func calcTextRect(hdc win.HDC, text string, hfont win.HFONT, wrapWidth int) (win.RECT, int) {
	if text == "" {
		return win.RECT{}, 0
	}
	win.SelectObject(hdc, win.HGDIOBJ(hfont))
	rect := win.RECT{0, 0, int32(wrapWidth), 0}
	flags := uint32(win.DT_CALCRECT | win.DT_CENTER)
	if wrapWidth > 0 {
		flags |= win.DT_WORDBREAK
	} else {
		flags |= win.DT_SINGLELINE
	}
	altura := drawText(hdc, text, -1, &rect, flags)
	return rect, int(altura)
}

func createWindow(className, title string, x, y, w, h int, isTopmost bool) win.HWND {
	dwExStyle := uint32(win.WS_EX_LAYERED | win.WS_EX_TOOLWINDOW)
	if isTopmost {
		dwExStyle |= win.WS_EX_TOPMOST
	}
	dwStyle := uint32(win.WS_POPUP)

	hwnd := win.CreateWindowEx(
		dwExStyle,
		utf16PtrFromString(className),
		utf16PtrFromString(title),
		dwStyle,
		int32(x), int32(y), int32(w), int32(h),
		0, 0, win.GetModuleHandle(nil), nil,
	)

	// LWA_COLORKEY para a cor magenta (Transparente)
	setLayeredWindowAttributes(hwnd, win.RGB(255, 0, 255), 0, LWA_COLORKEY)
	return hwnd
}

// Struct para dados de cada janela
type WindowData struct {
	Type   int // 1=Hover, 2=Highlight, 3=TodosCards, 4=EstudoHighlights
	Pinyin string
	Hanzi  string
	Sig    string
	Escala float64
}

var (
	className = "HanziTrackerOverlay"
	windowDatas = make(map[win.HWND]*WindowData)
)

func medirCard(pinyin, hanzi, sig string, escala float64) (w, h int) {
	hdc := win.GetDC(0)
	defer win.ReleaseDC(0, hdc)

	szPy := max(8, int(13*escala))
	szHz := max(11, int(26*escala))
	szSig := max(7, int(10*escala))
	wrap := max(80, int(240*escala))
	pad := max(4, int(12*escala))

	fPy := createFont(szPy, false)
	fHz := createFont(szHz, true)
	fSig := createFont(szSig, false)
	defer win.DeleteObject(win.HGDIOBJ(fPy))
	defer win.DeleteObject(win.HGDIOBJ(fHz))
	defer win.DeleteObject(win.HGDIOBJ(fSig))

	rPy, hPy := calcTextRect(hdc, pinyin, fPy, 0)
	rHz, hHz := calcTextRect(hdc, hanzi, fHz, 0)

	limite := max(20, int(70*escala))
	runes := []rune(sig)
	if len(runes) > limite {
		sig = string(runes[:limite-1]) + "…"
	}
	rSig, hSig := calcTextRect(hdc, sig, fSig, wrap)

	wMax := 0
	for _, r := range []win.RECT{rPy, rHz, rSig} {
		largura := int(r.Right - r.Left)
		if largura > wMax {
			wMax = largura
		}
	}
	wMax += pad * 2
	if wMax < wrap {
		wMax = wrap
	}

	hTotal := 0
	if pinyin != "" {
		hTotal += hPy + max(2, int(5*escala))
	}
	hTotal += hHz
	if sig != "" {
		hTotal += hSig + max(3, int(8*escala))
	}
	hTotal += pad * 2

	return wMax, hTotal
}

func desenharCard(hwnd win.HWND, hdc win.HDC, data *WindowData) {
	var rect win.RECT
	win.GetClientRect(hwnd, &rect)

	// Fundo (Simula tk.Frame com fundo #1a1a24 e borda #ff9800)
	bgCor := win.RGB(0x1a, 0x1a, 0x24)     // #1a1a24
	bordaCor := win.RGB(0xff, 0x98, 0x00) // #ff9800

	fillRect(hdc, &rect, bgCor)
	drawFrame(hdc, &rect, bordaCor, 1)

	szPy := max(8, int(13*data.Escala))
	szHz := max(11, int(26*data.Escala))
	szSig := max(7, int(10*data.Escala))
	pad := max(4, int(12*data.Escala))

	fPy := createFont(szPy, false)
	fHz := createFont(szHz, true)
	fSig := createFont(szSig, false)
	defer win.DeleteObject(win.HGDIOBJ(fPy))
	defer win.DeleteObject(win.HGDIOBJ(fHz))
	defer win.DeleteObject(win.HGDIOBJ(fSig))

	yAtual := int32(pad)

	if data.Pinyin != "" {
		rPy, hPy := calcTextRect(hdc, data.Pinyin, fPy, 0)
		rPy.Left = 0
		rPy.Right = rect.Right
		rPy.Top = yAtual
		rPy.Bottom = yAtual + int32(hPy)
		drawTextCentered(hdc, data.Pinyin, &rPy, win.RGB(0xff, 0x98, 0x00), fPy, false)
		yAtual += int32(hPy) + int32(max(2, int(5*data.Escala)))
	}

	rHz, hHz := calcTextRect(hdc, data.Hanzi, fHz, 0)
	rHz.Left = 0
	rHz.Right = rect.Right
	rHz.Top = yAtual
	rHz.Bottom = yAtual + int32(hHz)
	drawTextCentered(hdc, data.Hanzi, &rHz, win.RGB(0xff, 0xff, 0xff), fHz, false)
	yAtual += int32(hHz)

	if data.Sig != "" {
		yAtual += int32(max(3, int(8*data.Escala)))
		
		limite := max(20, int(70*data.Escala))
		runes := []rune(data.Sig)
		if len(runes) > limite {
			data.Sig = string(runes[:limite-1]) + "…"
		}
		
		rSig := win.RECT{Left: int32(pad), Top: yAtual, Right: rect.Right - int32(pad), Bottom: rect.Bottom}
		drawTextCentered(hdc, data.Sig, &rSig, win.RGB(0xcc, 0xcc, 0xcc), fSig, true)
	}
}

func wndProc(hwnd win.HWND, msg uint32, wParam, lParam uintptr) uintptr {
	switch msg {
	case win.WM_PAINT:
		var ps win.PAINTSTRUCT
		hdc := win.BeginPaint(hwnd, &ps)

		data := windowDatas[hwnd]
		if data != nil {
			switch data.Type {
			case 1, 3: // Hover ou TodosCards
				desenharCard(hwnd, hdc, data)
			case 2: // Highlight (Borda Verde)
				var r win.RECT
				win.GetClientRect(hwnd, &r)
				// Fundo já é a cor-chave, fica transparente. Desenhamos só a moldura.
				drawFrame(hdc, &r, win.RGB(0, 255, 0), 3)
			case 4: // EstudoHighlights (Borda Azul)
				var r win.RECT
				win.GetClientRect(hwnd, &r)
				drawFrame(hdc, &r, win.RGB(0x21, 0x96, 0xf3), 3) // #2196f3
			case 5: // EstudoParcialHighlights (Borda Amarela)
				var r win.RECT
				win.GetClientRect(hwnd, &r)
				drawFrame(hdc, &r, win.RGB(0xff, 0xeb, 0x3b), 3) // #FFEB3B
			}
		}

		win.EndPaint(hwnd, &ps)
		return 0
	case win.WM_DESTROY:
		delete(windowDatas, hwnd)
		return 0
	case win.WM_NCHITTEST:
		// Se precisar que o clique vaze, HTTRANSPARENT
		// return win.HTTRANSPARENT
	}
	return win.DefWindowProc(hwnd, msg, wParam, lParam)
}
