package progresso

import (
	"testing"
	"time"
)

// TestRegistrarRevisaoJornadaEsperaOhLockDoBanco reproduz o que acontece ao concluir uma revisão da
// Jornada: o registro do progresso e as leituras do placar chegam JUNTOS, cada um em sua goroutine
// (os bindings do Wails são concorrentes), e o pool abre uma conexão para cada um. Sem busy_timeout
// no DSN o escritor não espera o leitor: leva SQLITE_BUSY na hora e o progresso se perde em silêncio.
func TestRegistrarRevisaoJornadaEsperaOhLockDoBanco(t *testing.T) {
	prepararBancoDeTeste(t)

	// Leitor com transação aberta segura o lock compartilhado até o rollback.
	tx, err := db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	var total int
	if err := tx.QueryRow("SELECT COUNT(*) FROM vocabulario").Scan(&total); err != nil {
		tx.Rollback()
		t.Fatal(err)
	}

	resultado := make(chan error, 1)
	go func() { resultado <- RegistrarRevisaoJornada("fund1_0", 0) }()

	// O leitor só libera depois de a escrita já ter começado a esperar.
	time.Sleep(300 * time.Millisecond)
	tx.Rollback()

	if err := <-resultado; err != nil {
		t.Fatalf("a escrita deveria ESPERAR o leitor terminar, veio: %v", err)
	}

	progresso, err := ObterProgressoJornada()
	if err != nil {
		t.Fatal(err)
	}
	if progresso["fund1_0"] != 1 {
		t.Fatalf("esperava a revisão registrada mesmo com o banco ocupado, veio %d", progresso["fund1_0"])
	}
}

func TestRegistrarRevisaoJornadaAvancaEmSequencia(t *testing.T) {
	prepararBancoDeTeste(t)

	// Caso de sucesso: revisões 0 e 1 concluídas em ordem levam o contador a 2.
	if err := RegistrarRevisaoJornada("fund1_0", 0); err != nil {
		t.Fatalf("RegistrarRevisaoJornada(fund1_0, 0): %v", err)
	}
	if err := RegistrarRevisaoJornada("fund1_0", 1); err != nil {
		t.Fatalf("RegistrarRevisaoJornada(fund1_0, 1): %v", err)
	}

	progresso, err := ObterProgressoJornada()
	if err != nil {
		t.Fatalf("ObterProgressoJornada: %v", err)
	}
	if progresso["fund1_0"] != 2 {
		t.Fatalf("esperava 2 revisões concluídas em fund1_0, veio %d", progresso["fund1_0"])
	}
}

func TestRegistrarRevisaoJornadaEhIdempotente(t *testing.T) {
	prepararBancoDeTeste(t)

	if err := RegistrarRevisaoJornada("fund1_0", 0); err != nil {
		t.Fatal(err)
	}
	if err := RegistrarRevisaoJornada("fund1_0", 1); err != nil {
		t.Fatal(err)
	}

	// Repetir uma revisão já concluída não avança (usuário refez a revisão 0).
	if err := RegistrarRevisaoJornada("fund1_0", 0); err != nil {
		t.Fatal(err)
	}
	// Pular um índice também não avança (chamada fora de ordem não pode desbloquear o nível).
	if err := RegistrarRevisaoJornada("fund1_0", 3); err != nil {
		t.Fatal(err)
	}

	progresso, err := ObterProgressoJornada()
	if err != nil {
		t.Fatal(err)
	}
	if progresso["fund1_0"] != 2 {
		t.Fatalf("esperava o contador parado em 2, veio %d", progresso["fund1_0"])
	}
}

func TestObterProgressoJornadaSeparaNiveis(t *testing.T) {
	prepararBancoDeTeste(t)

	// Caso de borda: banco sem nenhuma revisão registrada devolve mapa vazio, não erro.
	progresso, err := ObterProgressoJornada()
	if err != nil {
		t.Fatalf("ObterProgressoJornada em banco vazio: %v", err)
	}
	if len(progresso) != 0 {
		t.Fatalf("esperava mapa vazio, veio %+v", progresso)
	}

	if err := RegistrarRevisaoJornada("fund1_0", 0); err != nil {
		t.Fatal(err)
	}
	if err := RegistrarRevisaoJornada("cotidiano1_2", 0); err != nil {
		t.Fatal(err)
	}

	progresso, err = ObterProgressoJornada()
	if err != nil {
		t.Fatal(err)
	}
	if progresso["fund1_0"] != 1 || progresso["cotidiano1_2"] != 1 {
		t.Fatalf("cada nível deveria ter contador próprio, veio %+v", progresso)
	}
}
