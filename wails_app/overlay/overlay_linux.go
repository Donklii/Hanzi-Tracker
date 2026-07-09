//go:build linux

// Implementação Linux do overlay (janelas nativas sobre a tela) usando X11 + Cairo + Pango via CGo.
// Substitui os no-ops de overlay_outros.go e replica a funcionalidade completa do overlay Windows:
// highlights (molduras coloridas), pop-ups hover, "mostrar tudo", resumo Gemini e censura de OCR.
//
// Requisitos de build: libx11-dev libxext-dev libcairo2-dev libpango1.0-dev
// (já instalados pelo Wails Linux: libgtk-3-dev e libwebkit2gtk-4.1-dev os puxam como dependência)
package overlay

/*
#cgo pkg-config: x11 xext cairo pangocairo

#include <X11/Xlib.h>
#include <X11/Xutil.h>
#include <X11/Xatom.h>
#include <X11/extensions/shape.h>
#include <cairo/cairo.h>
#include <cairo/cairo-xlib.h>
#include <pango/pangocairo.h>
#include <stdlib.h>
#include <string.h>

// ========================== State ==========================

static Display *ovl_dpy = NULL;
static int      ovl_scr;
static Window   ovl_root;
static Visual  *ovl_visual;
static Colormap ovl_cmap;
static int      ovl_depth;

// Atoms do window manager
static Atom a_wm_state, a_wm_state_above, a_wm_state_skip_taskbar, a_wm_state_skip_pager;
static Atom a_wm_type, a_wm_type_dock;

// ========================== Init / Cleanup ==========================

static int ovl_init(void) {
    ovl_dpy = XOpenDisplay(NULL);
    if (!ovl_dpy) return -1;

    ovl_scr  = DefaultScreen(ovl_dpy);
    ovl_root = RootWindow(ovl_dpy, ovl_scr);

    // Tenta visual ARGB de 32 bits para transparência real (requer compositor)
    XVisualInfo vinfo;
    if (XMatchVisualInfo(ovl_dpy, ovl_scr, 32, TrueColor, &vinfo)) {
        ovl_visual = vinfo.visual;
        ovl_depth  = 32;
        ovl_cmap   = XCreateColormap(ovl_dpy, ovl_root, ovl_visual, AllocNone);
    } else {
        ovl_visual = DefaultVisual(ovl_dpy, ovl_scr);
        ovl_depth  = DefaultDepth(ovl_dpy, ovl_scr);
        ovl_cmap   = DefaultColormap(ovl_dpy, ovl_scr);
    }

    a_wm_state              = XInternAtom(ovl_dpy, "_NET_WM_STATE", False);
    a_wm_state_above        = XInternAtom(ovl_dpy, "_NET_WM_STATE_ABOVE", False);
    a_wm_state_skip_taskbar = XInternAtom(ovl_dpy, "_NET_WM_STATE_SKIP_TASKBAR", False);
    a_wm_state_skip_pager   = XInternAtom(ovl_dpy, "_NET_WM_STATE_SKIP_PAGER", False);
    a_wm_type               = XInternAtom(ovl_dpy, "_NET_WM_WINDOW_TYPE", False);
    a_wm_type_dock          = XInternAtom(ovl_dpy, "_NET_WM_WINDOW_TYPE_DOCK", False);
    return 0;
}

static void ovl_cleanup(void) {
    if (ovl_dpy) { XCloseDisplay(ovl_dpy); ovl_dpy = NULL; }
}

// ========================== ARGB color helper ==========================

static unsigned long ovl_argb(unsigned char a, unsigned char r, unsigned char g, unsigned char b) {
    return ((unsigned long)a << 24) | ((unsigned long)r << 16) | ((unsigned long)g << 8) | b;
}

// ========================== Window management ==========================

static Window ovl_create_window(int x, int y, int w, int h, unsigned long bg) {
    if (!ovl_dpy || w <= 0 || h <= 0) return 0;

    XSetWindowAttributes attrs;
    memset(&attrs, 0, sizeof(attrs));
    attrs.override_redirect = True;
    attrs.colormap           = ovl_cmap;
    attrs.border_pixel       = 0;
    attrs.background_pixel   = bg;
    attrs.event_mask         = ExposureMask;

    unsigned long mask = CWOverrideRedirect | CWColormap | CWBorderPixel | CWBackPixel | CWEventMask;

    Window win = XCreateWindow(ovl_dpy, ovl_root, x, y,
        (unsigned int)w, (unsigned int)h, 0,
        ovl_depth, InputOutput, ovl_visual, mask, &attrs);

    // Tipo DOCK: sem entrada na taskbar
    XChangeProperty(ovl_dpy, win, a_wm_type, XA_ATOM, 32, PropModeReplace,
        (unsigned char *)&a_wm_type_dock, 1);

    // Always-on-top + skip taskbar/pager
    Atom states[3] = { a_wm_state_above, a_wm_state_skip_taskbar, a_wm_state_skip_pager };
    XChangeProperty(ovl_dpy, win, a_wm_state, XA_ATOM, 32, PropModeReplace,
        (unsigned char *)states, 3);

    // Click-through: forma de input vazia
    XShapeCombineRectangles(ovl_dpy, win, ShapeInput, 0, 0, NULL, 0, ShapeSet, Unsorted);

    return win;
}

static void ovl_shape_border(Window win, int w, int h, int border) {
    if (!ovl_dpy || !win || border <= 0) return;
    // 4 retângulos formando uma borda oca (YXBanded order)
    XRectangle rects[4];
    rects[0] = (XRectangle){0, 0, (unsigned short)w, (unsigned short)border};
    rects[1] = (XRectangle){0, (short)border, (unsigned short)border, (unsigned short)(h - 2*border)};
    rects[2] = (XRectangle){(short)(w - border), (short)border, (unsigned short)border, (unsigned short)(h - 2*border)};
    rects[3] = (XRectangle){0, (short)(h - border), (unsigned short)w, (unsigned short)border};
    XShapeCombineRectangles(ovl_dpy, win, ShapeBounding, 0, 0, rects, 4, ShapeSet, YXBanded);
}

static void ovl_map(Window win)    { if (ovl_dpy && win) XMapRaised(ovl_dpy, win); }
static void ovl_unmap(Window win)  { if (ovl_dpy && win) XUnmapWindow(ovl_dpy, win); }
static void ovl_destroy_win(Window win) { if (ovl_dpy && win) XDestroyWindow(ovl_dpy, win); }
static void ovl_flush(void)        { if (ovl_dpy) XFlush(ovl_dpy); }

static void ovl_move_resize(Window win, int x, int y, int w, int h) {
    if (ovl_dpy && win && w > 0 && h > 0)
        XMoveResizeWindow(ovl_dpy, win, x, y, (unsigned int)w, (unsigned int)h);
}

// ========================== Events ==========================

static int    ovl_pending(void)          { return ovl_dpy ? XPending(ovl_dpy) : 0; }
static void   ovl_next_event(XEvent *ev) { if (ovl_dpy) XNextEvent(ovl_dpy, ev); }
static int    ovl_ev_type(XEvent *ev)    { return ev->type; }
static Window ovl_ev_window(XEvent *ev)  { return ev->xany.window; }
static int    ovl_ev_expose_count(XEvent *ev) { return ev->xexpose.count; }

// ========================== Text measurement (Pango) ==========================

static void ovl_measure_text(const char *text, const char *family, int size_px, int is_bold,
                             int wrap_width, int *out_w, int *out_h) {
    if (!text || !text[0]) { if (out_w) *out_w = 0; if (out_h) *out_h = 0; return; }

    cairo_surface_t *srf = cairo_image_surface_create(CAIRO_FORMAT_ARGB32, 1, 1);
    cairo_t *cr = cairo_create(srf);
    PangoLayout *layout = pango_cairo_create_layout(cr);
    PangoFontDescription *desc = pango_font_description_new();

    pango_font_description_set_family(desc, family);
    pango_font_description_set_absolute_size(desc, size_px * PANGO_SCALE);
    if (is_bold) pango_font_description_set_weight(desc, PANGO_WEIGHT_BOLD);

    pango_layout_set_font_description(layout, desc);
    pango_layout_set_text(layout, text, -1);
    pango_layout_set_alignment(layout, PANGO_ALIGN_CENTER);

    if (wrap_width > 0) {
        pango_layout_set_width(layout, wrap_width * PANGO_SCALE);
        pango_layout_set_wrap(layout, PANGO_WRAP_WORD_CHAR);
    }

    int pw, ph;
    pango_layout_get_pixel_size(layout, &pw, &ph);
    if (out_w) *out_w = pw;
    if (out_h) *out_h = ph;

    pango_font_description_free(desc);
    g_object_unref(layout);
    cairo_destroy(cr);
    cairo_surface_destroy(srf);
}

// ========================== Drawing helpers (Cairo + Pango) ==========================

static void ovl_draw_text_at(cairo_t *cr, const char *text, const char *family,
                             int size_px, int is_bold,
                             double r, double g, double b,
                             int x, int y, int max_w, int wrap) {
    if (!text || !text[0]) return;

    PangoLayout *layout = pango_cairo_create_layout(cr);
    PangoFontDescription *desc = pango_font_description_new();

    pango_font_description_set_family(desc, family);
    pango_font_description_set_absolute_size(desc, size_px * PANGO_SCALE);
    if (is_bold) pango_font_description_set_weight(desc, PANGO_WEIGHT_BOLD);

    pango_layout_set_font_description(layout, desc);
    pango_layout_set_text(layout, text, -1);
    pango_layout_set_alignment(layout, PANGO_ALIGN_CENTER);

    if (max_w > 0) {
        pango_layout_set_width(layout, max_w * PANGO_SCALE);
        if (wrap) pango_layout_set_wrap(layout, PANGO_WRAP_WORD_CHAR);
    }

    cairo_set_source_rgb(cr, r, g, b);
    cairo_move_to(cr, x, y);
    pango_cairo_show_layout(cr, layout);

    pango_font_description_free(desc);
    g_object_unref(layout);
}

// ---- Card (pinyin + hanzi + sig) ----

static void ovl_draw_card(Window win, int w, int h,
                          const char *pinyin, const char *hanzi, const char *sig, double scale) {
    if (!ovl_dpy || !win) return;

    cairo_surface_t *srf = cairo_xlib_surface_create(ovl_dpy, win, ovl_visual, w, h);
    cairo_t *cr = cairo_create(srf);

    // Fundo #1a1a24
    cairo_set_source_rgba(cr, 0x1a/255.0, 0x1a/255.0, 0x24/255.0, 1.0);
    cairo_paint(cr);

    // Borda #ff9800
    cairo_set_source_rgb(cr, 0xff/255.0, 0x98/255.0, 0x00/255.0);
    cairo_set_line_width(cr, 1.0);
    cairo_rectangle(cr, 0.5, 0.5, w-1, h-1);
    cairo_stroke(cr);

    int szPy  = (int)(13*scale); if (szPy  < 8)  szPy  = 8;
    int szHz  = (int)(26*scale); if (szHz  < 11) szHz  = 11;
    int szSig = (int)(10*scale); if (szSig < 7)  szSig = 7;
    int pad   = (int)(12*scale); if (pad   < 4)  pad   = 4;
    int wrapW = w - 2*pad;
    int yPos  = pad;

    if (pinyin && pinyin[0]) {
        int th; ovl_measure_text(pinyin, "sans", szPy, 0, 0, NULL, &th);
        ovl_draw_text_at(cr, pinyin, "sans", szPy, 0,
                         0xff/255.0, 0x98/255.0, 0x00/255.0,
                         pad, yPos, wrapW, 0);
        int gap = (int)(5*scale); if (gap < 2) gap = 2;
        yPos += th + gap;
    }
    if (hanzi && hanzi[0]) {
        int th; ovl_measure_text(hanzi, "sans", szHz, 1, 0, NULL, &th);
        ovl_draw_text_at(cr, hanzi, "sans", szHz, 1,
                         1.0, 1.0, 1.0,
                         pad, yPos, wrapW, 0);
        yPos += th;
    }
    if (sig && sig[0]) {
        int gap = (int)(8*scale); if (gap < 3) gap = 3;
        yPos += gap;
        ovl_draw_text_at(cr, sig, "sans", szSig, 0,
                         0xcc/255.0, 0xcc/255.0, 0xcc/255.0,
                         pad, yPos, wrapW, 1);
    }

    cairo_destroy(cr);
    cairo_surface_destroy(srf);
    XFlush(ovl_dpy);
}

static void ovl_measure_card(const char *pinyin, const char *hanzi, const char *sig,
                             double scale, int *out_w, int *out_h) {
    int szPy  = (int)(13*scale); if (szPy  < 8)  szPy  = 8;
    int szHz  = (int)(26*scale); if (szHz  < 11) szHz  = 11;
    int szSig = (int)(10*scale); if (szSig < 7)  szSig = 7;
    int pad   = (int)(12*scale); if (pad   < 4)  pad   = 4;
    int wrapLimit = (int)(240*scale); if (wrapLimit < 80) wrapLimit = 80;

    int wPy=0, hPy=0, wHz=0, hHz=0, wSig=0, hSig=0;
    if (pinyin && pinyin[0]) ovl_measure_text(pinyin, "sans", szPy, 0, 0,         &wPy,  &hPy);
    if (hanzi  && hanzi[0])  ovl_measure_text(hanzi,  "sans", szHz, 1, 0,         &wHz,  &hHz);
    if (sig    && sig[0])    ovl_measure_text(sig,    "sans", szSig,0, wrapLimit, &wSig, &hSig);

    int wMax = wPy;
    if (wHz  > wMax) wMax = wHz;
    if (wSig > wMax) wMax = wSig;
    wMax += pad*2;
    if (wMax < wrapLimit) wMax = wrapLimit;

    int hTotal = 0;
    if (pinyin && pinyin[0]) { int gap=(int)(5*scale); if(gap<2) gap=2; hTotal += hPy + gap; }
    hTotal += hHz;
    if (sig && sig[0])       { int gap=(int)(8*scale); if(gap<3) gap=3; hTotal += hSig + gap; }
    hTotal += pad*2;

    *out_w = wMax;
    *out_h = hTotal;
}

// ---- Tradução-somente ----

static void ovl_draw_traducao(Window win, int w, int h, const char *sig) {
    if (!ovl_dpy || !win) return;
    cairo_surface_t *srf = cairo_xlib_surface_create(ovl_dpy, win, ovl_visual, w, h);
    cairo_t *cr = cairo_create(srf);

    cairo_set_source_rgba(cr, 0x1a/255.0, 0x1a/255.0, 0x24/255.0, 1.0);
    cairo_paint(cr);
    cairo_set_source_rgb(cr, 0xff/255.0, 0x98/255.0, 0x00/255.0);
    cairo_set_line_width(cr, 1.0);
    cairo_rectangle(cr, 0.5, 0.5, w-1, h-1);
    cairo_stroke(cr);

    ovl_draw_text_at(cr, sig, "sans", 11, 0,
                     0xcc/255.0, 0xcc/255.0, 0xcc/255.0,
                     6, 6, w-12, 1);

    cairo_destroy(cr);
    cairo_surface_destroy(srf);
    XFlush(ovl_dpy);
}

static void ovl_measure_traducao(const char *sig, int wrap_w, int *out_w, int *out_h) {
    int pad = 6;
    int wNat=0, hNat=0;
    ovl_measure_text(sig, "sans", 11, 0, 0, &wNat, &hNat);
    wNat += pad*2;

    int w = wrap_w;
    if (wNat > w) w = wNat;
    int wd = w - pad*2;
    if (wd < 40) { wd = 40; w = wd + pad*2; }

    int hSig=0;
    ovl_measure_text(sig, "sans", 11, 0, wd, NULL, &hSig);
    *out_w = w;
    *out_h = pad*2 + hSig;
}

// ---- Resumo (Gemini) ----

static void ovl_draw_resumo(Window win, int w, int h, const char *titulo, const char *texto) {
    if (!ovl_dpy || !win) return;
    cairo_surface_t *srf = cairo_xlib_surface_create(ovl_dpy, win, ovl_visual, w, h);
    cairo_t *cr = cairo_create(srf);

    cairo_set_source_rgba(cr, 0x1a/255.0, 0x1a/255.0, 0x24/255.0, 0.96);
    cairo_paint(cr);
    cairo_set_source_rgb(cr, 0xff/255.0, 0x98/255.0, 0x00/255.0);
    cairo_set_line_width(cr, 1.0);
    cairo_rectangle(cr, 0.5, 0.5, w-1, h-1);
    cairo_stroke(cr);

    int yPos = 16, textW = w - 32;
    if (titulo && titulo[0]) {
        int th; ovl_measure_text(titulo, "sans", 15, 1, 0, NULL, &th);
        ovl_draw_text_at(cr, titulo, "sans", 15, 1,
                         0xff/255.0, 0x98/255.0, 0x00/255.0,
                         16, yPos, textW, 0);
        yPos += th + 8;
    }
    if (texto && texto[0]) {
        ovl_draw_text_at(cr, texto, "sans", 12, 0,
                         0xcc/255.0, 0xcc/255.0, 0xcc/255.0,
                         16, yPos, textW, 1);
    }

    cairo_destroy(cr);
    cairo_surface_destroy(srf);
    XFlush(ovl_dpy);
}

static void ovl_measure_resumo(const char *titulo, const char *texto, int w, int *out_h) {
    int textW = w - 32, h = 16;
    if (titulo && titulo[0]) { int th; ovl_measure_text(titulo, "sans", 15, 1, textW, NULL, &th); h += th + 8; }
    if (texto  && texto[0])  { int th; ovl_measure_text(texto,  "sans", 12, 0, textW, NULL, &th); h += th; }
    h += 16;
    *out_h = h;
}
*/
import "C"

import (
	"fmt"
	"runtime"
	"sync"
	"time"
	"unsafe"
)

// ----- Tipos -----

// ItemPopup descreve os dados necessários para exibir um card (espelha a struct da versão Windows).
type ItemPopup struct {
	Pinyin     string
	Hanzi      string
	Sig        string
	X0, Y0     int
	X1, Y1     int
	SoTraducao bool
}



// dadosJanela armazena os dados para (re)desenhar uma janela popup.
type dadosJanela struct {
	tipo   int // 1=Hover, 2=Highlight, 3=TodosCards, 4=EstudoHighlights, 5=EstudoParcial, 6=Resumo, 7=TraducaoSomente
	pinyin string
	hanzi  string
	sig    string
	escala float64
}



// janelaLinux empacota uma janela X11 com metadados de posição e desenho.
type janelaLinux struct {
	xwin    C.Window
	x, y    int
	w, h    int
	visible bool
	data    *dadosJanela // nil para highlights (usam background_pixel + shape)
}

// ----- Estado -----

var (
	filaAcoes    = make(chan func(), 100)
	canalPronto    chan struct{}
	canalConcluido     chan struct{}
	inicializacaoOk     bool // true se o X11 inicializou corretamente
	mu         sync.Mutex
	todasJanelas = make(map[C.Window]*janelaLinux)

	janelaHover            *janelaLinux
	janelaDestaque        *janelaLinux
	janelaResumo           *janelaLinux
	janelasTodos           []*janelaLinux
	janelasEstudos         []*janelaLinux
	janelasEstudosParciais []*janelaLinux

	cancelarResumo chan struct{}
	resumoMu     sync.Mutex
)

// ----- Ciclo De Vida -----

// Iniciar arranca a thread de interface X11 do overlay.
func Iniciar() {
	mu.Lock()
	if canalPronto != nil {
		mu.Unlock()
		return
	}
	canalPronto = make(chan struct{})
	canalConcluido = make(chan struct{})
	mu.Unlock()

	go loopOverlay()
	<-canalPronto
}



// Encerrar fecha todas as janelas e a thread de eventos.
func Encerrar() {
	mu.Lock()
	if canalConcluido != nil {
		select {
		case <-canalConcluido:
			// já fechado
		default:
			close(canalConcluido)
		}
	}
	mu.Unlock()
}



func loopOverlay() {
	runtime.LockOSThread()

	if C.ovl_init() != 0 {
		fmt.Println("Aviso: falha ao inicializar overlay X11 (display não disponível?)")
		close(canalPronto)
		return
	}

	inicializacaoOk = true
	close(canalPronto)

	ticker := time.NewTicker(8 * time.Millisecond)
	defer ticker.Stop()
	defer func() {
		limparTodasJanelas()
		C.ovl_cleanup()
	}()

	for {
		select {
		case <-canalConcluido:
			return
		case fn := <-filaAcoes:
			fn()
			esvaziarAcoes()
			processarEventos()
		case <-ticker.C:
			processarEventos()
		}
	}
}



func executarNaThread(fn func()) {
	if !inicializacaoOk {
		return
	}
	filaAcoes <- fn
}



func esvaziarAcoes() {
	for {
		select {
		case fn := <-filaAcoes:
			fn()
		default:
			return
		}
	}
}



func processarEventos() {
	for C.ovl_pending() > 0 {
		var ev C.XEvent
		C.ovl_next_event(&ev)

		if C.ovl_ev_type(&ev) == C.Expose && C.ovl_ev_expose_count(&ev) == 0 {
			win := C.ovl_ev_window(&ev)
			if lw, ok := todasJanelas[win]; ok && lw.data != nil {
				redesenharJanela(lw)
			}
		}
	}
}

// ----- Gerenciamento De Janelas (Interno) -----

func criarDestaque(x, y, w, h int, r, g, b byte, borda int, alpha byte) *janelaLinux {
	if w <= 0 {
		w = 1
	}
	if h <= 0 {
		h = 1
	}

	bg := C.ovl_argb(C.uchar(alpha), C.uchar(r), C.uchar(g), C.uchar(b))
	xwin := C.ovl_create_window(C.int(x), C.int(y), C.int(w), C.int(h), bg)
	if xwin == 0 {
		return nil
	}

	C.ovl_shape_border(xwin, C.int(w), C.int(h), C.int(borda))

	lw := &janelaLinux{xwin: xwin, x: x, y: y, w: w, h: h}
	todasJanelas[xwin] = lw
	return lw
}



func criarJanelaPopup(x, y, w, h int, data *dadosJanela) *janelaLinux {
	if w <= 0 {
		w = 1
	}
	if h <= 0 {
		h = 1
	}

	bg := C.ovl_argb(255, 0x1a, 0x1a, 0x24)
	xwin := C.ovl_create_window(C.int(x), C.int(y), C.int(w), C.int(h), bg)
	if xwin == 0 {
		return nil
	}

	lw := &janelaLinux{xwin: xwin, x: x, y: y, w: w, h: h, data: data}
	todasJanelas[xwin] = lw
	return lw
}



func mostrarJanela(lw *janelaLinux) {
	if lw == nil || lw.xwin == 0 {
		return
	}
	C.ovl_map(lw.xwin)
	lw.visible = true
	redesenharJanela(lw)
}



func esconderJanela(lw *janelaLinux) {
	if lw == nil || lw.xwin == 0 {
		return
	}
	C.ovl_unmap(lw.xwin)
	lw.visible = false
}



func destruirJanela(lw *janelaLinux) {
	if lw == nil || lw.xwin == 0 {
		return
	}
	delete(todasJanelas, lw.xwin)
	C.ovl_destroy_win(lw.xwin)
	lw.xwin = 0
	lw.visible = false
}



func destruirLista(lista *[]*janelaLinux) {
	for _, lw := range *lista {
		destruirJanela(lw)
	}
	*lista = nil
}



func redesenharJanela(lw *janelaLinux) {
	if lw == nil || lw.data == nil || lw.xwin == 0 {
		return
	}
	switch lw.data.tipo {
	case 1, 3: // Hover ou TodosCards
		cPy := C.CString(lw.data.pinyin)
		cHz := C.CString(lw.data.hanzi)
		cSg := C.CString(lw.data.sig)
		C.ovl_draw_card(lw.xwin, C.int(lw.w), C.int(lw.h), cPy, cHz, cSg, C.double(lw.data.escala))
		C.free(unsafe.Pointer(cPy))
		C.free(unsafe.Pointer(cHz))
		C.free(unsafe.Pointer(cSg))
	case 6: // Resumo
		cTi := C.CString(lw.data.hanzi)
		cTx := C.CString(lw.data.sig)
		C.ovl_draw_resumo(lw.xwin, C.int(lw.w), C.int(lw.h), cTi, cTx)
		C.free(unsafe.Pointer(cTi))
		C.free(unsafe.Pointer(cTx))
	case 7: // Tradução-somente
		cSg := C.CString(lw.data.sig)
		C.ovl_draw_traducao(lw.xwin, C.int(lw.w), C.int(lw.h), cSg)
		C.free(unsafe.Pointer(cSg))
	}
}



func limparTodasJanelas() {
	destruirJanela(janelaHover)
	janelaHover = nil
	destruirJanela(janelaDestaque)
	janelaDestaque = nil
	destruirJanela(janelaResumo)
	janelaResumo = nil
	destruirLista(&janelasTodos)
	destruirLista(&janelasEstudos)
	destruirLista(&janelasEstudosParciais)
}

// ----- Api Publica -----

// MostrarHover exibe o card de hover na posição (x,y).
func MostrarHover(pinyin, hanzi, sig string, x, y int) {
	executarNaThread(func() {
		if janelaDestaque != nil {
			esconderJanela(janelaDestaque)
		}
		if janelaHover != nil {
			destruirJanela(janelaHover)
			janelaHover = nil
		}

		cPy := C.CString(pinyin)
		cHz := C.CString(hanzi)
		cSg := C.CString(sig)
		var cw, ch C.int
		C.ovl_measure_card(cPy, cHz, cSg, 1.0, &cw, &ch)
		C.free(unsafe.Pointer(cPy))
		C.free(unsafe.Pointer(cHz))
		C.free(unsafe.Pointer(cSg))

		w, h := int(cw), int(ch)
		finalX := x - w/2
		finalY := y - h - 10
		if finalY < 0 {
			finalY = y + 30
		}

		data := &dadosJanela{tipo: 1, pinyin: pinyin, hanzi: hanzi, sig: sig, escala: 1.0}
		janelaHover = criarJanelaPopup(finalX, finalY, w, h, data)
		if janelaHover != nil {
			mostrarJanela(janelaHover)
		}
	})
}



// OcultarHover oculta o card de hover.
func OcultarHover() {
	executarNaThread(func() {
		if janelaHover != nil {
			esconderJanela(janelaHover)
		}
		if janelaDestaque != nil {
			esconderJanela(janelaDestaque)
		}
	})
}



// MostrarDestaque exibe uma moldura vazada (verde) na posição selecionada.
func MostrarDestaque(x0, y0, x1, y1 int) {
	executarNaThread(func() {
		if janelaHover != nil {
			esconderJanela(janelaHover)
		}
		if janelaDestaque != nil {
			destruirJanela(janelaDestaque)
			janelaDestaque = nil
		}

		w := x1 - x0
		h := y1 - y0
		if w <= 0 {
			w = 1
		}
		if h <= 0 {
			h = 1
		}

		janelaDestaque = criarDestaque(x0, y0, w, h, 0, 255, 0, 3, 255) // verde, 3px
		if janelaDestaque != nil {
			mostrarJanela(janelaDestaque)
		}
	})
}



// MostrarDestaquesEstudo exibe várias molduras vazadas (azuis) simultaneamente.
func MostrarDestaquesEstudo(boxes [][]float64) {
	executarNaThread(func() {
		atualizarMolduras(&janelasEstudos, boxes, 0x21, 0x96, 0xf3, 2, 150, 4)
	})
}



// MostrarDestaquesEstudoParcial exibe várias molduras vazadas (amarelas) simultaneamente.
func MostrarDestaquesEstudoParcial(boxes [][]float64) {
	executarNaThread(func() {
		atualizarMolduras(&janelasEstudosParciais, boxes, 0xff, 0xeb, 0x3b, 2, 150, 5)
	})
}



// atualizarMolduras reutiliza janelas existentes para evitar flicker (espelha a versão Windows).
func atualizarMolduras(lista *[]*janelaLinux, boxes [][]float64, r, g, b byte, borda int, alpha byte, tipo int) {
	type item struct {
		lw   *janelaLinux
		used bool
	}
	existing := make([]*item, len(*lista))
	for i, lw := range *lista {
		existing[i] = &item{lw: lw}
	}

	var novas, extras []*janelaLinux

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

		// 1. Tenta janela na mesma posição/tamanho
		for _, it := range existing {
			if !it.used && it.lw.x == x0 && it.lw.y == y0 && it.lw.w == w && it.lw.h == h {
				it.used = true
				encontrou = true
				if !it.lw.visible {
					C.ovl_map(it.lw.xwin)
					it.lw.visible = true
				}
				novas = append(novas, it.lw)
				break
			}
		}

		// 2. Reutiliza qualquer janela não usada
		if !encontrou {
			for _, it := range existing {
				if !it.used {
					it.used = true
					encontrou = true
					it.lw.x, it.lw.y, it.lw.w, it.lw.h = x0, y0, w, h
					C.ovl_move_resize(it.lw.xwin, C.int(x0), C.int(y0), C.int(w), C.int(h))
					C.ovl_shape_border(it.lw.xwin, C.int(w), C.int(h), C.int(borda))
					if !it.lw.visible {
						C.ovl_map(it.lw.xwin)
						it.lw.visible = true
					}
					novas = append(novas, it.lw)
					break
				}
			}
		}

		// 3. Cria nova janela
		if !encontrou {
			lw := criarDestaque(x0, y0, w, h, r, g, b, borda, alpha)
			if lw != nil {
				C.ovl_map(lw.xwin)
				lw.visible = true
				novas = append(novas, lw)
			}
		}
	}

	// Esconde janelas que sobraram (pool)
	for _, it := range existing {
		if !it.used {
			if it.lw.visible {
				esconderJanela(it.lw)
			}
			extras = append(extras, it.lw)
		}
	}

	*lista = append(novas, extras...)
}



// MostrarTodos posiciona inteligentemente e exibe os cards para um conjunto de itens.
func MostrarTodos(itens []ItemPopup, sw, sh int) {
	executarNaThread(func() {
		destruirLista(&janelasTodos)
		escalas := []float64{1.0, 0.8, 0.65, 0.5}
		var colocadas []Rect

		for _, item := range itens {
			if item.SoTraducao {
				larLinha := item.X1 - item.X0
				if larLinha < 60 {
					larLinha = 60
				}
				cSig := C.CString(item.Sig)
				var cw, ch C.int
				C.ovl_measure_traducao(cSig, C.int(larLinha), &cw, &ch)
				C.free(unsafe.Pointer(cSig))

				ww, hh := int(cw), int(ch)
				x := item.X0
				y := item.Y1 + 2
				if x+ww > sw {
					x = sw - ww
				}
				if x < 0 {
					x = 0
				}
				if y+hh > sh {
					y = item.Y0 - hh - 2
				}
				if y < 0 {
					y = 0
				}

				data := &dadosJanela{tipo: 7, sig: item.Sig, escala: 1.0}
				lw := criarJanelaPopup(x, y, ww, hh, data)
				if lw != nil {
					mostrarJanela(lw)
					janelasTodos = append(janelasTodos, lw)
					colocadas = append(colocadas, Rect{X0: x, Y0: y, X1: x + ww, Y1: y + hh})
				}
				continue
			}

			centroX := (item.X0 + item.X1) / 2
			colocado := false
			var ww, hh, preferX, preferY int

			for _, escala := range escalas {
				cPy := C.CString(item.Pinyin)
				cHz := C.CString(item.Hanzi)
				cSg := C.CString(item.Sig)
				var cw, ch C.int
				C.ovl_measure_card(cPy, cHz, cSg, C.double(escala), &cw, &ch)
				C.free(unsafe.Pointer(cPy))
				C.free(unsafe.Pointer(cHz))
				C.free(unsafe.Pointer(cSg))

				ww, hh = int(cw), int(ch)
				preferX = centroX - ww/2
				preferY = item.Y0 - hh - 6
				if preferY < 0 {
					preferY = item.Y1 + 6
				}

				x, y, rect, ok := AcharPosicao(preferX, preferY, ww, hh, colocadas, sw, sh)
				if ok {
					data := &dadosJanela{tipo: 3, pinyin: item.Pinyin, hanzi: item.Hanzi, sig: item.Sig, escala: escala}
					lw := criarJanelaPopup(x, y, ww, hh, data)
					if lw != nil {
						mostrarJanela(lw)
						janelasTodos = append(janelasTodos, lw)
						colocadas = append(colocadas, rect)
						colocado = true
					}
					break
				}
			}

			if !colocado {
				x := max(0, min(preferX, sw-ww))
				y := max(0, min(preferY, sh-hh))
				data := &dadosJanela{tipo: 3, pinyin: item.Pinyin, hanzi: item.Hanzi, sig: item.Sig, escala: 0.5}
				lw := criarJanelaPopup(x, y, ww, hh, data)
				if lw != nil {
					mostrarJanela(lw)
					janelasTodos = append(janelasTodos, lw)
					colocadas = append(colocadas, Rect{X0: x, Y0: y, X1: x + ww, Y1: y + hh})
				}
			}
		}
	})
}



// OcultarTodos remove as janelas abertas por MostrarTodos.
func OcultarTodos() {
	executarNaThread(func() {
		destruirLista(&janelasTodos)
	})
}



// MostrarResumo exibe o resumo gerado pelo Gemini em um canto da tela.
func MostrarResumo(titulo, texto, canto string, monX, monY, monW, monH int, ttlSec int) {
	resumoMu.Lock()
	if cancelarResumo != nil {
		close(cancelarResumo)
		cancelarResumo = nil
	}
	if ttlSec < 0 {
		ttlSec = max(10, len(texto)/15)
	}
	var newCancel chan struct{}
	if ttlSec > 0 {
		newCancel = make(chan struct{})
		cancelarResumo = newCancel
	}
	resumoMu.Unlock()

	executarNaThread(func() {
		w := 420

		cTi := C.CString(titulo)
		cTx := C.CString(texto)
		var ch C.int
		C.ovl_measure_resumo(cTi, cTx, C.int(w), &ch)
		C.free(unsafe.Pointer(cTi))
		C.free(unsafe.Pointer(cTx))

		hh := int(ch)
		maxH := int(float64(monH) * 0.45)
		if hh > maxH {
			hh = maxH
		}

		var x, y int
		switch canto {
		case "superior-esquerdo":
			x, y = monX+16, monY+16
		case "superior-direito":
			x, y = monX+monW-16-w, monY+16
		case "inferior-esquerdo":
			x, y = monX+16, monY+monH-16-hh
		default:
			x, y = monX+monW-16-w, monY+monH-16-hh
		}

		if janelaResumo != nil {
			janelaResumo.x, janelaResumo.y = x, y
			janelaResumo.w, janelaResumo.h = w, hh
			janelaResumo.data.hanzi = titulo
			janelaResumo.data.sig = texto
			C.ovl_move_resize(janelaResumo.xwin, C.int(x), C.int(y), C.int(w), C.int(hh))
			redesenharJanela(janelaResumo)
		} else {
			data := &dadosJanela{tipo: 6, hanzi: titulo, sig: texto, escala: 1.0}
			janelaResumo = criarJanelaPopup(x, y, w, hh, data)
			if janelaResumo != nil {
				mostrarJanela(janelaResumo)
			}
		}
	})

	if ttlSec > 0 {
		go func() {
			runtime.LockOSThread()
			defer runtime.UnlockOSThread()

			localDpy := C.XOpenDisplay(nil)
			if localDpy == nil {
				return
			}
			defer C.XCloseDisplay(localDpy)
			localRoot := C.XDefaultRootWindow(localDpy)

			ticker := time.NewTicker(200 * time.Millisecond)
			defer ticker.Stop()

			remaining := time.Duration(ttlSec) * time.Second

			for {
				select {
				case <-newCancel:
					return
				case <-ticker.C:
					isHovering := false
					lw := janelaResumo // leitura sem sync (tolerável, como na versão Windows)
					if lw != nil && lw.xwin != 0 {
						var rootRet, childRet C.Window
						var rootX, rootY, winX, winY C.int
						var mask C.uint
						C.XQueryPointer(localDpy, localRoot, &rootRet, &childRet, &rootX, &rootY, &winX, &winY, &mask)
						mx, my := int(rootX), int(rootY)
						if mx >= lw.x-15 && mx <= lw.x+lw.w+15 && my >= lw.y-15 && my <= lw.y+lw.h+15 {
							isHovering = true
						}
					} else {
						isHovering = true
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



// OcultarResumo oculta o pop-up de resumo.
func OcultarResumo() {
	resumoMu.Lock()
	if cancelarResumo != nil {
		close(cancelarResumo)
		cancelarResumo = nil
	}
	resumoMu.Unlock()

	executarNaThread(func() {
		if janelaResumo != nil {
			destruirJanela(janelaResumo)
			janelaResumo = nil
		}
	})
}



// OcultarDestaquesTemporariamente esconde os destaques (bordas), aguarda a atualização
// do compositor, roda a acao (print da tela) e os restaura.
func OcultarDestaquesTemporariamente(acao func()) {
	if !inicializacaoOk {
		acao()
		return
	}

	var escondidas []*janelaLinux
	feito := make(chan struct{})

	executarNaThread(func() {
		esconder := func(lw *janelaLinux) {
			if lw != nil && lw.visible {
				C.ovl_unmap(lw.xwin)
				lw.visible = false
				escondidas = append(escondidas, lw)
			}
		}
		esconder(janelaDestaque)
		for _, lw := range janelasEstudos {
			esconder(lw)
		}
		for _, lw := range janelasEstudosParciais {
			esconder(lw)
		}
		C.ovl_flush()
		feito <- struct{}{}
	})
	<-feito

	// Delay para o compositor atualizar o frame visualmente no Linux.
	// 150ms costuma ser suficiente para garantir que o unmap ocorra antes da captura.
	time.Sleep(150 * time.Millisecond)

	acao()

	executarNaThread(func() {
		for _, lw := range escondidas {
			if lw != nil && lw.xwin != 0 {
				C.ovl_map(lw.xwin)
				lw.visible = true
			}
		}
	})
}



// RetangulosVisiveis devolve, em coordenadas absolutas de tela, os retângulos de todas as
// janelas do overlay atualmente visíveis. Usado para censurar antes do OCR.
func RetangulosVisiveis() []Rect {
	if !inicializacaoOk {
		return nil
	}

	resultado := make(chan []Rect, 1)
	executarNaThread(func() {
		var rects []Rect
		coletar := func(lw *janelaLinux) {
			if lw != nil && lw.visible {
				rects = append(rects, Rect{X0: lw.x, Y0: lw.y, X1: lw.x + lw.w, Y1: lw.y + lw.h})
			}
		}
		coletar(janelaHover)
		coletar(janelaDestaque)
		coletar(janelaResumo)
		for _, lw := range janelasTodos {
			coletar(lw)
		}
		for _, lw := range janelasEstudos {
			coletar(lw)
		}
		for _, lw := range janelasEstudosParciais {
			coletar(lw)
		}
		resultado <- rects
	})
	return <-resultado
}

// ----- Utilitarios -----

// min retorna o menor entre dois inteiros (shadow do built-in, consistente com overlay.go Windows).
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
