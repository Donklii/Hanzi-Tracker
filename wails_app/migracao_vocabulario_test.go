package main

import (
	"testing"

	"wails_app/dicionario"
)

// dicionarioDeTesteMigracao carrega o dicionário embarcado para exercitar a migração sem tocar o
// banco de progresso (as funções testadas são puras — só consultam o dicionário).
func dicionarioDeTesteMigracao(t *testing.T) *dicionario.GerenciadorDicionario {
	t.Helper()
	dic, err := dicionario.NovoGerenciadorDicionario(dicionario.IdiomaPadrao)
	if err != nil {
		t.Fatalf("falha ao carregar o dicionário: %v", err)
	}
	return dic
}

// TestDesfazerTrocaMeYaoConsertaCorrompidas garante que as grafias corrompidas pelo bug 么→幺 são
// remapeadas para a forma válida, e que grafias legítimas (inclusive o 幺 isolado, palavra real) NÃO
// são tocadas.
func TestDesfazerTrocaMeYaoConsertaCorrompidas(t *testing.T) {
	dic := dicionarioDeTesteMigracao(t)

	corrigidos := []struct{ corrompida, esperada string }{
		{"什幺", "什么"},
		{"怎幺", "怎么"},
		{"那幺", "那么"},
	}
	for _, c := range corrigidos {
		corrigida, ok := desfazerTrocaMeYao(dic, c.corrompida)
		if !ok || corrigida != c.esperada {
			t.Errorf("desfazerTrocaMeYao(%q) = (%q, %v), esperado (%q, true)", c.corrompida, corrigida, ok, c.esperada)
		}
	}

	// Grafias válidas não têm conserto (não são resíduo do bug): 什么 já existe; 幺 é palavra real.
	for _, valida := range []string{"什么", "幺", "好"} {
		if corrigida, ok := desfazerTrocaMeYao(dic, valida); ok {
			t.Errorf("desfazerTrocaMeYao(%q) não devia consertar grafia válida, veio %q", valida, corrigida)
		}
	}
}

// TestGrafiaReconhecida trava o reconhecimento que a migração usa: palavra e caractere do dicionário
// contam como reconhecidos; a grafia corrompida 什幺, não.
func TestGrafiaReconhecida(t *testing.T) {
	dic := dicionarioDeTesteMigracao(t)

	for _, valida := range []string{"什么", "好", "幺"} {
		if !grafiaReconhecida(dic, valida) {
			t.Errorf("grafiaReconhecida(%q) = false, esperado true", valida)
		}
	}
	if grafiaReconhecida(dic, "什幺") {
		t.Error("grafiaReconhecida(什幺) = true, esperado false (grafia inexistente)")
	}
}
