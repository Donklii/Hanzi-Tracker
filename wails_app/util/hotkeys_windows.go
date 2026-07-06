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

// teclaDe mapeia A-Z e 0-9 para Virtual-Key codes ('A' = 0x41, '0' = 0x30).
func teclaDe(char byte) hotkey.Key {
	switch {
	case char >= 'a' && char <= 'z':
		return hotkey.Key(char - 'a' + 0x41)
	case char >= '0' && char <= '9':
		return hotkey.Key(char - '0' + 0x30)
	}
	return 0
}
