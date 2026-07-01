package main

import (
	"archive/zip"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ----- Catálogo de motores (motores_manifesto.go) -----

func TestObterMotorBaixavelExistente(t *testing.T) {
	m, ok := ObterMotorBaixavel("RapidOCR")
	if !ok {
		t.Fatal("esperava encontrar o motor RapidOCR no catálogo")
	}
	if m.Executavel == "" {
		t.Error("motor RapidOCR sem Executavel definido")
	}
}

func TestObterMotorBaixavelInexistente(t *testing.T) {
	if _, ok := ObterMotorBaixavel("MotorQueNaoExiste"); ok {
		t.Error("não deveria encontrar um motor inexistente")
	}
}

func TestMotorOcrPadrao(t *testing.T) {
	m, ok := MotorOcrPadrao()
	if !ok {
		t.Fatal("esperava um motor padrão no catálogo")
	}
	if !m.Padrao {
		t.Error("o motor devolvido por MotorOcrPadrao não está marcado como Padrao")
	}
}

// ----- Integridade do catálogo: sha256 obrigatório e URL HTTPS -----

func TestCatalogoDeMotoresIntegro(t *testing.T) {
	if len(MotoresBaixaveis) == 0 {
		t.Fatal("catálogo de motores vazio")
	}
	for nome, m := range MotoresBaixaveis {
		if m.Artefato.Sha256 == "" {
			t.Errorf("motor %q sem sha256 (obrigatório para binários)", nome)
		}
		if !strings.HasPrefix(m.Artefato.Url, "https://") {
			t.Errorf("motor %q com URL não-https: %s", nome, m.Artefato.Url)
		}
		if m.Artefato.TamanhoBytes <= 0 {
			t.Errorf("motor %q com TamanhoBytes inválido: %d", nome, m.Artefato.TamanhoBytes)
		}
		if m.Executavel == "" {
			t.Errorf("motor %q sem Executavel", nome)
		}
	}

	// O overlay compartilhado também é um binário baixado: exige sha256 + HTTPS.
	if PopupOverlayBaixavel.Sha256 == "" || !strings.HasPrefix(PopupOverlayBaixavel.Url, "https://") {
		t.Error("PopupOverlayBaixavel com sha256 vazio ou URL não-https")
	}
}

// ----- Extração de zip com proteção Zip Slip (motores.go) -----

func TestExtrairZipGolden(t *testing.T) {
	dir := t.TempDir()
	zipPath := filepath.Join(dir, "teste.zip")
	criarZipDeTeste(t, zipPath, map[string]string{"ocr_server.exe": "conteudo"})

	destino := filepath.Join(dir, "saida")
	if err := extrairZip(zipPath, destino); err != nil {
		t.Fatalf("extrairZip falhou no caso válido: %v", err)
	}

	dados, err := os.ReadFile(filepath.Join(destino, "ocr_server.exe"))
	if err != nil {
		t.Fatalf("arquivo esperado não foi extraído: %v", err)
	}
	if string(dados) != "conteudo" {
		t.Errorf("conteúdo extraído incorreto: %q", string(dados))
	}
}

func TestExtrairZipRecusaZipSlip(t *testing.T) {
	dir := t.TempDir()
	zipPath := filepath.Join(dir, "malicioso.zip")
	criarZipDeTeste(t, zipPath, map[string]string{"../fuga.txt": "malicioso"})

	destino := filepath.Join(dir, "saida")
	if err := extrairZip(zipPath, destino); err == nil {
		t.Error("extrairZip deveria recusar uma entrada com ../ (Zip Slip)")
	}
}

// TestExtrairZipComBackslash cobre a regressão do Compress-Archive: nomes com separador "\" e entrada de
// diretório com trailing "\". Sem a normalização, o "diretório" vira arquivo vazio e a extração quebra.
func TestExtrairZipComBackslash(t *testing.T) {
	dir := t.TempDir()
	zipPath := filepath.Join(dir, "backslash.zip")

	f, err := os.Create(zipPath)
	if err != nil {
		t.Fatal(err)
	}
	w := zip.NewWriter(f)
	if _, err := w.Create(`_internal\numpy\`); err != nil { // entrada de diretório com backslash
		t.Fatal(err)
	}
	arq, err := w.Create(`_internal\numpy\fft\core.pyd`) // arquivo aninhado com backslash
	if err != nil {
		t.Fatal(err)
	}
	if _, err := arq.Write([]byte("binario")); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	f.Close()

	destino := filepath.Join(dir, "saida")
	if err := extrairZip(zipPath, destino); err != nil {
		t.Fatalf("extrairZip deveria lidar com separador backslash: %v", err)
	}

	info, err := os.Stat(filepath.Join(destino, "_internal", "numpy"))
	if err != nil || !info.IsDir() {
		t.Errorf("_internal/numpy deveria ser um diretório (não arquivo vazio): err=%v", err)
	}
	if _, err := os.Stat(filepath.Join(destino, "_internal", "numpy", "fft", "core.pyd")); err != nil {
		t.Errorf("arquivo aninhado não foi extraído: %v", err)
	}
}

// criarZipDeTeste gera um .zip em `caminho` com os arquivos {nome: conteúdo} informados.
func criarZipDeTeste(t *testing.T, caminho string, arquivos map[string]string) {
	t.Helper()
	f, err := os.Create(caminho)
	if err != nil {
		t.Fatalf("falha ao criar o zip de teste: %v", err)
	}
	defer f.Close()

	w := zip.NewWriter(f)
	for nome, conteudo := range arquivos {
		entrada, err := w.Create(nome)
		if err != nil {
			t.Fatalf("falha ao criar entrada %q: %v", nome, err)
		}
		if _, err := entrada.Write([]byte(conteudo)); err != nil {
			t.Fatalf("falha ao escrever entrada %q: %v", nome, err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatalf("falha ao fechar o zip de teste: %v", err)
	}
}
