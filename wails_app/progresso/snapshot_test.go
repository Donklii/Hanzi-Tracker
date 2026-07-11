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

func TestEstatisticasESequencias(t *testing.T) {
	prepararBancoDeTeste(t)

	// Caso de sucesso: incrementando os acertos nas categorias
	err := AtualizarAcertosSequencia("猫", "māo", "gato", "significado", true)
	if err != nil {
		t.Fatalf("Erro ao atualizar acertos: %v", err)
	}
	err = AtualizarAcertosSequencia("猫", "māo", "gato", "significado", true)
	if err != nil {
		t.Fatalf("Erro ao atualizar acertos: %v", err)
	}

	stats, err := ObterEstatisticasPalavra("猫")
	if err != nil {
		t.Fatalf("Erro ao obter estatísticas: %v", err)
	}
	if stats["significado"] != 2 {
		t.Errorf("Esperava acertos_sequencia_significado = 2, veio %d", stats["significado"])
	}

	// Caso de falha/reset: errar reseta o streak para 0
	err = AtualizarAcertosSequencia("猫", "māo", "gato", "significado", false)
	if err != nil {
		t.Fatalf("Erro ao atualizar acertos: %v", err)
	}

	stats, err = ObterEstatisticasPalavra("猫")
	if err != nil {
		t.Fatalf("Erro ao obter estatísticas: %v", err)
	}
	if stats["significado"] != 0 {
		t.Errorf("Esperava acertos_sequencia_significado resetado para 0, veio %d", stats["significado"])
	}

	// Testar sugestões para aprendido
	// Preenche todas as 5 categorias com streak >= 3
	categorias := []string{"significado", "fonetica", "desenho", "contexto", "pronuncia"}
	for _, cat := range categorias {
		for i := 0; i < 3; i++ {
			err = AtualizarAcertosSequencia("狗", "gǒu", "cachorro", cat, true)
			if err != nil {
				t.Fatalf("Erro ao incrementar categoria %s: %v", cat, err)
			}
		}
	}

	// Outra palavra que não atinge todos os requisitos (só significado)
	for i := 0; i < 3; i++ {
		err = AtualizarAcertosSequencia("鸟", "niǎo", "pássaro", "significado", true)
		if err != nil {
			t.Fatalf("Erro ao incrementar categoria significado para 鸟: %v", err)
		}
	}

	sugestoes, err := ObterSugestoesAprendidoLote([]string{"狗", "鸟"})
	if err != nil {
		t.Fatalf("Erro ao obter sugestões: %v", err)
	}

	// Deve sugerir apenas "狗"
	if len(sugestoes) != 1 || sugestoes[0].Hanzi != "狗" {
		t.Errorf("Esperava sugerir apenas a palavra '狗', veio: %+v", sugestoes)
	}
}
