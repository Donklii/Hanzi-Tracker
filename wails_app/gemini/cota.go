package gemini

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// EstadoCota rastreia quantas requisições já foram consumidas no dia corrente.
// Persistida em %APPDATA%\HanziTracker\cota_gemini.json.
type EstadoCota struct {
	Data              string `json:"data"` // "2026-07-04"; reseta quando o dia atual diverge
	RequisicoesUsadas int    `json:"requisicoesUsadas"`
}

// mu protege leitura/escrita concorrente do arquivo de cota.
var mu sync.Mutex

// dataAtual devolve o dia corrente no formato "YYYY-MM-DD".
func dataAtual() string {
	return time.Now().Format("2006-01-02")
}

// caminhoCota devolve o caminho do arquivo de persistência da cota.
func caminhoCota() (string, error) {
	appData, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(appData, "HanziTracker")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}
	return filepath.Join(dir, "cota_gemini.json"), nil
}

// CarregarCota lê o estado de cota do disco. Se a Data salva difere da data atual, devolve
// zerado (e persiste o reset), garantindo que todo dia começa do zero.
func CarregarCota() EstadoCota {
	mu.Lock()
	defer mu.Unlock()

	return carregarCotaInterno()
}

// carregarCotaInterno é a versão sem lock (chamada quando o lock já foi adquirido).
func carregarCotaInterno() EstadoCota {
	dataHoje := dataAtual()

	caminho, err := caminhoCota()
	if err != nil {
		return EstadoCota{Data: dataHoje, RequisicoesUsadas: 0}
	}

	data, err := os.ReadFile(caminho)
	if err != nil {
		// Arquivo não existe (first-run) ou erro de leitura: começa zerado.
		estado := EstadoCota{Data: dataHoje, RequisicoesUsadas: 0}
		salvarCotaInterno(estado)
		return estado
	}

	var estado EstadoCota
	if err := json.Unmarshal(data, &estado); err != nil {
		estado = EstadoCota{Data: dataHoje, RequisicoesUsadas: 0}
		salvarCotaInterno(estado)
		return estado
	}

	// Guard clause: dia mudou — reseta o contador.
	if estado.Data != dataHoje {
		estado = EstadoCota{Data: dataHoje, RequisicoesUsadas: 0}
		salvarCotaInterno(estado)
	}

	return estado
}

// salvarCotaInterno persiste o estado no disco (sem lock — chamada internamente).
func salvarCotaInterno(estado EstadoCota) error {
	caminho, err := caminhoCota()
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(estado, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(caminho, data, 0644)
}

// RegistrarRequisicao incrementa o contador de requisições usadas e persiste. Chamar SÓ após chamada
// de API bem-sucedida.
func RegistrarRequisicao() error {
	mu.Lock()
	defer mu.Unlock()

	estado := carregarCotaInterno()
	estado.RequisicoesUsadas += 1
	return salvarCotaInterno(estado)
}

// CotaExcedida verifica se o uso atual já atingiu ou ultrapassou o limite configurado.
func CotaExcedida(limiteRequisicoes int) bool {
	mu.Lock()
	defer mu.Unlock()

	estado := carregarCotaInterno()
	return estado.RequisicoesUsadas >= limiteRequisicoes
}

// InfoCotaParaUI devolve dados formatados para exibição na UI (thread-safe).
func InfoCotaParaUI() (requisicoesUsadas int, data string) {
	estado := CarregarCota()
	return estado.RequisicoesUsadas, estado.Data
}

// LimparArquivoCota remove o arquivo de cota do disco. Usado por ExcluirTudo.
func LimparArquivoCota() error {
	caminho, err := caminhoCota()
	if err != nil {
		return err
	}
	if err := os.Remove(caminho); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("falha ao remover cota_gemini.json: %w", err)
	}
	return nil
}
