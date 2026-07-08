package cota

import (
	"encoding/json"
	"os"
	"testing"
)

// prepararPastaDeTeste isola os arquivos de cota num diretório temporário, redirecionando o
// os.UserConfigDir das duas plataformas (XDG_CONFIG_HOME no Linux, AppData no Windows).
func prepararPastaDeTeste(t *testing.T) {
	t.Helper()

	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("AppData", dir)
}

func contadorDeTeste(t *testing.T, nome, periodo string) *Contador {
	t.Helper()
	return NovoContador(nome, func() string { return periodo }, nil)
}

// ----- Casos de sucesso -----

func TestRegistrarAcumulaEhPersiste(t *testing.T) {
	prepararPastaDeTeste(t)
	contador := contadorDeTeste(t, "teste", "2026-07")

	periodo, usado := contador.Carregar()
	if periodo != "2026-07" || usado != 0 {
		t.Fatalf("estado inicial: esperava (2026-07, 0), veio (%s, %d)", periodo, usado)
	}

	if err := contador.Registrar(10); err != nil {
		t.Fatalf("Registrar(10): %v", err)
	}
	if err := contador.Registrar(5); err != nil {
		t.Fatalf("Registrar(5): %v", err)
	}

	_, usado = contador.Carregar()
	if usado != 15 {
		t.Fatalf("esperava 15 usados, veio %d", usado)
	}

	// O arquivo unificado guarda a cota sob a própria chave, com os campos periodo/usado.
	caminho, err := caminhoNaPastaDados(nomeArquivoCotas)
	if err != nil {
		t.Fatalf("caminhoNaPastaDados: %v", err)
	}
	dados, err := os.ReadFile(caminho)
	if err != nil {
		t.Fatalf("cotas.json não foi gravado: %v", err)
	}
	var bruto map[string]registroCota
	if err := json.Unmarshal(dados, &bruto); err != nil {
		t.Fatalf("cotas.json inválido: %v", err)
	}
	if bruto["teste"].Periodo != "2026-07" || bruto["teste"].Usado != 15 {
		t.Fatalf("registro inesperado em cotas.json: %+v", bruto["teste"])
	}
}

func TestContadoresCompartilhamOhArquivoSemColidir(t *testing.T) {
	prepararPastaDeTeste(t)
	diaria := contadorDeTeste(t, "gemini", "2026-07-07")
	mensal := contadorDeTeste(t, "traducao", "2026-07")

	if err := diaria.Registrar(3); err != nil {
		t.Fatalf("Registrar na diária: %v", err)
	}
	if err := mensal.Registrar(1000); err != nil {
		t.Fatalf("Registrar na mensal: %v", err)
	}

	if _, usado := diaria.Carregar(); usado != 3 {
		t.Errorf("cota diária: esperava 3, veio %d", usado)
	}
	if _, usado := mensal.Carregar(); usado != 1000 {
		t.Errorf("cota mensal: esperava 1000, veio %d", usado)
	}
}

// ----- Casos de borda -----

func TestViradaDePeriodoZeraContador(t *testing.T) {
	prepararPastaDeTeste(t)

	antigo := contadorDeTeste(t, "teste", "2026-06")
	if err := antigo.Registrar(99); err != nil {
		t.Fatalf("Registrar no período antigo: %v", err)
	}

	// Mesma cota, período novo: o contador deve resetar (e persistir o reset).
	novo := contadorDeTeste(t, "teste", "2026-07")
	periodo, usado := novo.Carregar()
	if periodo != "2026-07" || usado != 0 {
		t.Fatalf("virada de período: esperava (2026-07, 0), veio (%s, %d)", periodo, usado)
	}
}

func TestArquivoCorrompidoComecaZerado(t *testing.T) {
	prepararPastaDeTeste(t)
	contador := contadorDeTeste(t, "teste", "2026-07")

	caminho, err := caminhoNaPastaDados(nomeArquivoCotas)
	if err != nil {
		t.Fatalf("caminhoNaPastaDados: %v", err)
	}
	if err := os.WriteFile(caminho, []byte("{corrompido"), 0644); err != nil {
		t.Fatalf("preparando arquivo corrompido: %v", err)
	}

	_, usado := contador.Carregar()
	if usado != 0 {
		t.Fatalf("arquivo corrompido: esperava 0 usados, veio %d", usado)
	}
}

func TestMigraArquivoAntigoPreservandoOhConsumo(t *testing.T) {
	prepararPastaDeTeste(t)

	// Formato antigo: um JSON por cota, com nomes de campo próprios.
	caminhoAntigo, err := caminhoNaPastaDados("cota_gemini.json")
	if err != nil {
		t.Fatalf("caminhoNaPastaDados: %v", err)
	}
	conteudoAntigo := []byte(`{"data": "2026-07-07", "requisicoesUsadas": 42}`)
	if err := os.WriteFile(caminhoAntigo, conteudoAntigo, 0644); err != nil {
		t.Fatalf("preparando arquivo antigo: %v", err)
	}

	contador := NovoContador("gemini",
		func() string { return "2026-07-07" },
		&MigracaoAntiga{NomeArquivo: "cota_gemini.json", CampoPeriodo: "data", CampoUsado: "requisicoesUsadas"})

	periodo, usado := contador.Carregar()
	if periodo != "2026-07-07" || usado != 42 {
		t.Fatalf("migração: esperava (2026-07-07, 42), veio (%s, %d)", periodo, usado)
	}

	// O arquivo antigo é consumido pela importação.
	if _, err := os.Stat(caminhoAntigo); !os.IsNotExist(err) {
		t.Error("o arquivo antigo deveria ter sido removido após a migração")
	}

	// Registrar continua do valor importado.
	if err := contador.Registrar(1); err != nil {
		t.Fatalf("Registrar pós-migração: %v", err)
	}
	if _, usado := contador.Carregar(); usado != 43 {
		t.Fatalf("pós-migração: esperava 43, veio %d", usado)
	}
}

func TestMigraArquivoAntigoDePeriodoVencidoComecaZerado(t *testing.T) {
	prepararPastaDeTeste(t)

	caminhoAntigo, err := caminhoNaPastaDados("cota_traducao.json")
	if err != nil {
		t.Fatalf("caminhoNaPastaDados: %v", err)
	}
	conteudoAntigo := []byte(`{"anoMes": "2026-05", "caracteresUsados": 12345}`)
	if err := os.WriteFile(caminhoAntigo, conteudoAntigo, 0644); err != nil {
		t.Fatalf("preparando arquivo antigo: %v", err)
	}

	contador := NovoContador("traducao",
		func() string { return "2026-07" },
		&MigracaoAntiga{NomeArquivo: "cota_traducao.json", CampoPeriodo: "anoMes", CampoUsado: "caracteresUsados"})

	periodo, usado := contador.Carregar()
	if periodo != "2026-07" || usado != 0 {
		t.Fatalf("migração de período vencido: esperava (2026-07, 0), veio (%s, %d)", periodo, usado)
	}
}
