package dicionario

import (
	"strings"
	"testing"
)

// ----- Carga do dicionário fundido por idioma e regras de fallback -----

// TestCarregarPtBrUsaOhDicionarioProprio confirma que pt-BR carrega o dicionário em português (e não
// cai para o inglês), usando 好 como sonda: "bom", nunca "good".
func TestCarregarPtBrUsaOhDicionarioProprio(t *testing.T) {
	banco := NovoBanco()
	if err := banco.Carregar("pt-BR"); err != nil {
		t.Fatalf("Carregar(pt-BR) falhou: %v", err)
	}

	caractere := banco.BuscarCaractere("好")
	if caractere == nil {
		t.Fatal("caractere 好 não encontrado no dicionário pt-BR")
	}
	if !strings.Contains(strings.ToLower(caractere.Definicao), "bom") {
		t.Errorf("definição pt-BR de 好 não parece portuguesa: %q", caractere.Definicao)
	}
	if strings.Contains(strings.ToLower(caractere.Definicao), "good") {
		t.Errorf("definição de 好 ainda está em inglês: %q", caractere.Definicao)
	}
}


// TestCarregarEnUsaOhDicionarioIngles garante que o inglês continua carregando a sua própria definição.
func TestCarregarEnUsaOhDicionarioIngles(t *testing.T) {
	banco := NovoBanco()
	if err := banco.Carregar("en"); err != nil {
		t.Fatalf("Carregar(en) falhou: %v", err)
	}

	caractere := banco.BuscarCaractere("好")
	if caractere == nil || !strings.Contains(strings.ToLower(caractere.Definicao), "good") {
		t.Errorf("definição en de 好 inesperada: %+v", caractere)
	}
}


// TestPalavraCompostaPtBrVemTraduzida trava o ganho da tradução do CEDICT: palavra composta (que só
// o CEDICT conhece, sem definição curada) também sai em português.
func TestPalavraCompostaPtBrVemTraduzida(t *testing.T) {
	banco := NovoBanco()
	if err := banco.Carregar("pt-BR"); err != nil {
		t.Fatalf("Carregar(pt-BR) falhou: %v", err)
	}

	entradas := banco.Buscar("世界")
	if len(entradas) == 0 {
		t.Fatal("palavra composta 世界 não encontrada no dicionário pt-BR")
	}
	juntos := strings.ToLower(strings.Join(entradas[0].Significados, " "))
	if !strings.Contains(juntos, "mundo") {
		t.Errorf("significados pt-BR de 世界 não parecem portugueses: %q", entradas[0].Significados)
	}
}


// TestFrasesPtBrCaiParaIngles confirma que o acervo de frases cai para o inglês quando pt-BR não tem
// banco de frases próprio.
func TestFrasesPtBrCaiParaIngles(t *testing.T) {
	g := NovoGerenciadorFrases("pt-BR")
	if total := g.TotalFrases(); total == 0 {
		t.Error("acervo de frases pt-BR (fallback en) veio vazio")
	}
}


// TestIdiomaFallbackDesconhecido garante que um idioma não suportado normaliza para o padrão (en).
func TestIdiomaFallbackDesconhecido(t *testing.T) {
	if IdiomaValido("xx-YY") {
		t.Error("idioma inexistente não deveria ser válido")
	}
	if idioma := normalizarIdioma("xx-YY"); idioma != IdiomaPadrao {
		t.Errorf("normalizarIdioma(desconhecido) = %q, esperado %q", idioma, IdiomaPadrao)
	}

	banco := NovoBanco()
	if err := banco.Carregar("xx-YY"); err != nil {
		t.Fatalf("Carregar(idioma desconhecido) deveria cair para en, mas falhou: %v", err)
	}
	if banco.TotalHanzis() == 0 {
		t.Error("Carregar com idioma desconhecido não carregou nada (fallback en falhou)")
	}
}
