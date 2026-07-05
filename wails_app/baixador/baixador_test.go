package baixador

import (
	"archive/zip"
	"os"
	"path/filepath"
	"testing"
)

// ----- Extração de zip com proteção Zip Slip -----

func TestExtrairZipGolden(t *testing.T) {
	dir := t.TempDir()
	zipPath := filepath.Join(dir, "teste.zip")
	criarZipDeTeste(t, zipPath, map[string]string{"ocr_server.exe": "conteudo"})

	destino := filepath.Join(dir, "saida")
	if err := ExtrairZip(zipPath, destino); err != nil {
		t.Fatalf("ExtrairZip falhou no caso válido: %v", err)
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
	if err := ExtrairZip(zipPath, destino); err == nil {
		t.Error("ExtrairZip deveria recusar uma entrada com ../ (Zip Slip)")
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
	if err := ExtrairZip(zipPath, destino); err != nil {
		t.Fatalf("ExtrairZip deveria lidar com separador backslash: %v", err)
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
