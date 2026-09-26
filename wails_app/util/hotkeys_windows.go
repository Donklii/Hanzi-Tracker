//go:build windows

package util

import "golang.design/x/hotkey"

// modificadorDe mapeia o nome do modificador para o código do Windows.
func modificadorDe(nome string) (hotkey.Modifier, bool) {
	switch nome {
	case "ctrl":
		return hotkey.ModCtrl, true
	case "shift":
		return hotkey.ModShift, true
	case "alt":
		return hotkey.ModAlt, true
	case "win":
		return hotkey.ModWin, true
	}
	return 0, false
}

// teclaDe mapeia A-Z, 0-9 e teclas especiais para Virtual-Key codes do Windows.
func teclaDe(nome string) (hotkey.Key, bool) {
	if len(nome) == 1 {
		char := nome[0]
		switch {
		case char >= 'a' && char <= 'z':
			return hotkey.Key(char - 'a' + 0x41), true
		case char >= '0' && char <= '9':
			return hotkey.Key(char - '0' + 0x30), true
		}
	}

	switch nome {
	case "f1":
		return hotkey.Key(0x70), true
	case "f2":
		return hotkey.Key(0x71), true
	case "f3":
		return hotkey.Key(0x72), true
	case "f4":
		return hotkey.Key(0x73), true
	case "f5":
		return hotkey.Key(0x74), true
	case "f6":
		return hotkey.Key(0x75), true
	case "f7":
		return hotkey.Key(0x76), true
	case "f8":
		return hotkey.Key(0x77), true
	case "f9":
		return hotkey.Key(0x78), true
	case "f10":
		return hotkey.Key(0x79), true
	case "f11":
		return hotkey.Key(0x7A), true
	case "f12":
		return hotkey.Key(0x7B), true
	case "space", "espaço", "espaco":
		return hotkey.Key(0x20), true
	case "tab":
		return hotkey.Key(0x09), true
	case "enter", "return":
		return hotkey.Key(0x0D), true
	case "escape", "esc":
		return hotkey.Key(0x1B), true
	case "backspace":
		return hotkey.Key(0x08), true
	case "delete", "del":
		return hotkey.Key(0x2E), true
	case "left":
		return hotkey.Key(0x25), true
	case "up":
		return hotkey.Key(0x26), true
	case "right":
		return hotkey.Key(0x27), true
	case "down":
		return hotkey.Key(0x28), true
	}
	return 0, false
}
