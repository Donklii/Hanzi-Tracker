package util

import (
	"strings"

	"golang.design/x/hotkey"
)

// ParseHotkey converte um atalho textual ("ctrl+shift+e") num hotkey registrável. Os códigos de
// modificador e de tecla variam por SO (Virtual-Key codes no Windows, keysyms X11 no Linux), então o
// mapeamento fica em modificadorDe/teclaDe (hotkeys_windows.go / hotkeys_linux.go).
func ParseHotkey(hkStr string) *hotkey.Hotkey {
	parts := strings.Split(strings.ToLower(hkStr), "+")
	var mods []hotkey.Modifier
	var key hotkey.Key

	for _, p := range parts {
		p = strings.TrimSpace(p)
		if mod, ok := modificadorDe(p); ok {
			mods = append(mods, mod)
			continue
		}
		if k, ok := teclaDe(p); ok {
			key = k
		}
	}
	if key == 0 {
		return nil
	}
	return hotkey.New(mods, key)
}
