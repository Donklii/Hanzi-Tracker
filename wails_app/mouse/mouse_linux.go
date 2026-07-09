//go:build linux

// GetCursorPos retorna a posição global do cursor via XQueryPointer (Xlib). Abre e fecha uma conexão
// X11 a cada chamada para evitar problemas de thread-safety — o overhead (~0.2ms) é desprezível com
// o intervalo de polling de 120ms do loop.go.
package mouse

/*
#cgo pkg-config: x11
#include <X11/Xlib.h>
*/
import "C"
import "fmt"

func GetCursorPos() (int, int, error) {
	dpy := C.XOpenDisplay(nil)
	if dpy == nil {
		return 0, 0, fmt.Errorf("falha ao abrir display X11 para consultar posição do cursor")
	}
	defer C.XCloseDisplay(dpy)

	root := C.XDefaultRootWindow(dpy)
	var rootRet, childRet C.Window
	var rootX, rootY, winX, winY C.int
	var mask C.uint

	ret := C.XQueryPointer(dpy, root, &rootRet, &childRet, &rootX, &rootY, &winX, &winY, &mask)
	if ret == 0 {
		return 0, 0, fmt.Errorf("XQueryPointer falhou")
	}
	return int(rootX), int(rootY), nil
}
