//go:build linux

package util

import "golang.design/x/hotkey"

// modificadorDe mapeia o nome do modificador para a máscara X11 (Alt = Mod1, Super/Win = Mod4 — a lib
// não define ModAlt/ModWin fora do Windows).
func modificadorDe(nome string) (hotkey.Modifier, bool) {
	switch nome {
	case "ctrl":
		return hotkey.ModCtrl, true
	case "shift":
		return hotkey.ModShift, true
	case "alt":
		return hotkey.Mod1, true
	case "win":
		return hotkey.Mod4, true
	}
	return 0, false
}

// teclaDe mapeia A-Z, 0-9 e teclas especiais para keysyms X11 no Linux.
func teclaDe(nome string) (hotkey.Key, bool) {
	if len(nome) == 1 {
		char := nome[0]
		switch {
		case char >= 'a' && char <= 'z':
			return hotkey.Key(char - 'a' + 0x61), true
		case char >= '0' && char <= '9':
			return hotkey.Key(char - '0' + 0x30), true
		}
	}

	switch nome {
	case "f1":
		return hotkey.Key(0xFFBE), true
	case "f2":
		return hotkey.Key(0xFFBF), true
	case "f3":
		return hotkey.Key(0xFFC0), true
	case "f4":
		return hotkey.Key(0xFFC1), true
	case "f5":
		return hotkey.Key(0xFFC2), true
	case "f6":
		return hotkey.Key(0xFFC3), true
	case "f7":
		return hotkey.Key(0xFFC4), true
	case "f8":
		return hotkey.Key(0xFFC5), true
	case "f9":
		return hotkey.Key(0xFFC6), true
	case "f10":
		return hotkey.Key(0xFFC7), true
	case "f11":
		return hotkey.Key(0xFFC8), true
	case "f12":
		return hotkey.Key(0xFFC9), true
	case "space", "espaço", "espaco":
		return hotkey.Key(0x20), true
	case "tab":
		return hotkey.Key(0xFF09), true
	case "enter", "return":
		return hotkey.Key(0xFF0D), true
	case "escape", "esc":
		return hotkey.Key(0xFF1B), true
	case "backspace":
		return hotkey.Key(0xFF08), true
	case "delete", "del":
		return hotkey.Key(0xFFFF), true
	case "left":
		return hotkey.Key(0xFF51), true
	case "up":
		return hotkey.Key(0xFF52), true
	case "right":
		return hotkey.Key(0xFF53), true
	case "down":
		return hotkey.Key(0xFF54), true
	}
	return 0, false
}
