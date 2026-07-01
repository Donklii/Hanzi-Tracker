package util

import (
	"strings"

	"golang.design/x/hotkey"
)

func ParseHotkey(hkStr string) *hotkey.Hotkey {
	parts := strings.Split(strings.ToLower(hkStr), "+")
	var mods []hotkey.Modifier
	var key hotkey.Key

	for _, p := range parts {
		p = strings.TrimSpace(p)
		switch p {
		case "ctrl":
			mods = append(mods, hotkey.ModCtrl)
		case "shift":
			mods = append(mods, hotkey.ModShift)
		case "alt":
			mods = append(mods, hotkey.ModAlt)
		case "win":
			mods = append(mods, hotkey.ModWin)
		default:
			if len(p) == 1 {
				// Mapeamento básico A-Z e 0-9
				char := p[0]
				if char >= 'a' && char <= 'z' {
					// hotkey.KeyA = 0x41
					key = hotkey.Key(char - 'a' + 0x41)
				} else if char >= '0' && char <= '9' {
					// hotkey.Key0 = 0x30
					key = hotkey.Key(char - '0' + 0x30)
				}
			}
		}
	}
	if key == 0 {
		return nil
	}
	return hotkey.New(mods, key)
}
