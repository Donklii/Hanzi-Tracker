package util

import (
	"testing"
)

func TestParseHotkey(t *testing.T) {
	testes := []struct {
		combo    string
		esperado bool
	}{
		{"ctrl+shift+e", true},
		{"ctrl+alt+t", true},
		{"ctrl+f1", true},
		{"ctrl+space", true},
		{"invalidkeycombo999", false},
		{"", false},
	}

	for _, tt := range testes {
		hk := ParseHotkey(tt.combo)
		if (hk != nil) != tt.esperado {
			t.Errorf("ParseHotkey(%q): esperado %v, obtido %v", tt.combo, tt.esperado, hk != nil)
		}
	}
}

func TestGerenciadorAtalhos(t *testing.T) {
	g := NovoGerenciadorAtalhos()

	handlers := map[string]func(){
		"test1": func() {},
	}

	novos := map[string]string{
		"test1": "ctrl+shift+f12",
	}

	_ = g.AtualizarAtalhos(novos, handlers)
	g.DesativarTodos()
}
