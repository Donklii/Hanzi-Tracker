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

// teclaDe mapeia A-Z e 0-9 para keysyms X11 ('a' = 0x61, '0' = 0x30). Os keysyms são usados
// diretamente porque as constantes Key1..Key0 da lib estão deslocadas em uma posição no Linux
// (hotkey_x11.go define Key1 = 0x30, que é o keysym do '0').
func teclaDe(char byte) hotkey.Key {
	switch {
	case char >= 'a' && char <= 'z':
		return hotkey.Key(char - 'a' + 0x61)
	case char >= '0' && char <= '9':
		return hotkey.Key(char - '0' + 0x30)
	}
	return 0
}
