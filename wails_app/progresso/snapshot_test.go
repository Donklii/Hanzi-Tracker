package progresso

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"
)

// prepararBancoDeTeste isola o banco num diretório temporário, redirecionando o os.UserConfigDir
// das duas plataformas (XDG_CONFIG_HOME no Linux, AppData no Windows), e o inicializa.
func prepararBancoDeTeste(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("AppData", dir)
	if err := os.MkdirAll(filepath.Join(dir, "HanziTracker"), 0755); err != nil {
		t.Fatal(err)
	}

	if err := InitDB(); err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	t.Cleanup(func() { FecharDB() })

	return dir
}

func TestExportarSnapshotCopiaOsDados(t *testing.T) {
	dir := prepararBancoDeTeste(t)

	if err := AddOuUpdateVocab("你好", "nǐ hǎo", "olá", "estudo"); err != nil {
		t.Fatalf("AddOuUpdateVocab: %v", err)
	}

	destino := filepath.Join(dir, "snapshot.db")
	if err := ExportarSnapshot(destino); err != nil {
		t.Fatalf("ExportarSnapshot: %v", err)
	}

	// O snapshot é um banco independente e completo: abre e contém a palavra gravada.
	copia, err := sql.Open("sqlite", destino)
	if err != nil {
		t.Fatalf("abrir snapshot: %v", err)
	}
	defer copia.Close()

	var total int
	if err := copia.QueryRow("SELECT COUNT(*) FROM vocabulario WHERE hanzi = '你好'").Scan(&total); err != nil {
		t.Fatalf("consultar snapshot: %v", err)
	}
	if total != 1 {
		t.Fatalf("esperava 1 registro no snapshot, veio %d", total)
	}
}

func TestExportarSnapshotSobrescreveDestinoExistente(t *testing.T) {
	dir := prepararBancoDeTeste(t)

	destino := filepath.Join(dir, "snapshot.db")
	if err := os.WriteFile(destino, []byte("sobra de execução anterior"), 0644); err != nil {
		t.Fatal(err)
	}

	if err := ExportarSnapshot(destino); err != nil {
		t.Fatalf("ExportarSnapshot sobre destino existente: %v", err)
	}
}

func TestFecharDbPermiteSubstituirEhReabrirOhArquivo(t *testing.T) {
	dir := prepararBancoDeTeste(t)

	if err := AddOuUpdateVocab("猫", "māo", "gato", "aprendido"); err != nil {
		t.Fatal(err)
	}

	// Simula o "usar dados da nuvem": snapshot vira o banco baixado, que substitui o local.
	baixado := filepath.Join(dir, "baixado.db")
	if err := ExportarSnapshot(baixado); err != nil {
		t.Fatal(err)
	}
	if err := FecharDB(); err != nil {
		t.Fatalf("FecharDB: %v", err)
	}

	caminhoBanco := filepath.Join(dir, "HanziTracker", "progresso.db")
	if err := os.Rename(baixado, caminhoBanco); err != nil {
		t.Fatalf("substituir o arquivo do banco: %v", err)
	}

	if err := InitDB(); err != nil {
		t.Fatalf("reabrir com InitDB: %v", err)
	}

	lista, err := GetAllVocab()
	if err != nil {
		t.Fatalf("GetAllVocab após a troca: %v", err)
	}
	if len(lista) != 1 || lista[0].Hanzi != "猫" {
		t.Fatalf("esperava o vocabulário do banco substituto, veio %+v", lista)
	}
}
