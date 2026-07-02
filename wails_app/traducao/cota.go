package traducao

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// CotaGratuitaCaracteresMes é o free tier documentado pelo Google (Cloud Translation API v2);
// só a BASE do %, ajustável se o preço mudar — não precisa de migração de dados.
const CotaGratuitaCaracteresMes = 500_000

// EstadoCota rastreia quantos caracteres já foram consumidos no mês corrente.
// Persistida em %APPDATA%\HanziTracker\cota_traducao.json (separado do Config principal
// para evitar condição de corrida com o frontend salvando configuracoes.json inteiro).
type EstadoCota struct {
	AnoMes           string `json:"anoMes"`           // "2026-07"; reseta quando o mês atual diverge
	CaracteresUsados int    `json:"caracteresUsados"`
}

// mu protege leitura/escrita concorrente do arquivo de cota (o loop de captura roda em goroutine).
var mu sync.Mutex

// anoMesAtual devolve o mês corrente no formato "YYYY-MM".
func anoMesAtual() string {
	return time.Now().Format("2006-01")
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
	return filepath.Join(dir, "cota_traducao.json"), nil
}

// CarregarCota lê o estado de cota do disco. Se o AnoMes salvo difere do mês atual, devolve
// zerado (e persiste o reset), garantindo que todo mês começa do zero.
func CarregarCota() EstadoCota {
	mu.Lock()
	defer mu.Unlock()

	return carregarCotaInterno()
}

// carregarCotaInterno é a versão sem lock (chamada quando o lock já foi adquirido).
func carregarCotaInterno() EstadoCota {
	mesAtual := anoMesAtual()

	caminho, err := caminhoCota()
	if err != nil {
		return EstadoCota{AnoMes: mesAtual, CaracteresUsados: 0}
	}

	data, err := os.ReadFile(caminho)
	if err != nil {
		// Arquivo não existe (first-run) ou erro de leitura: começa zerado.
		estado := EstadoCota{AnoMes: mesAtual, CaracteresUsados: 0}
		salvarCotaInterno(estado)
		return estado
	}

	var estado EstadoCota
	if err := json.Unmarshal(data, &estado); err != nil {
		estado = EstadoCota{AnoMes: mesAtual, CaracteresUsados: 0}
		salvarCotaInterno(estado)
		return estado
	}

	// Guard clause: mês mudou — reseta o contador.
	if estado.AnoMes != mesAtual {
		estado = EstadoCota{AnoMes: mesAtual, CaracteresUsados: 0}
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

// RegistrarUso incrementa o contador de caracteres usados e persiste. Chamar SÓ após chamada
// de API bem-sucedida (nunca em cache hit).
func RegistrarUso(caracteres int) error {
	mu.Lock()
	defer mu.Unlock()

	estado := carregarCotaInterno()
	estado.CaracteresUsados += caracteres
	return salvarCotaInterno(estado)
}

// PercentUsado calcula a porcentagem do free tier já consumida.
func PercentUsado(estado EstadoCota) float64 {
	if CotaGratuitaCaracteresMes <= 0 {
		return 100.0
	}
	return float64(estado.CaracteresUsados) / float64(CotaGratuitaCaracteresMes) * 100
}

// CotaExcedida verifica se o uso atual já atingiu ou ultrapassou o limite percentual configurado.
func CotaExcedida(limitePct float64) bool {
	mu.Lock()
	defer mu.Unlock()

	estado := carregarCotaInterno()
	return PercentUsado(estado) >= limitePct
}

// InfoCotaParaUI devolve dados formatados para exibição na UI (thread-safe).
func InfoCotaParaUI() (caracteresUsados int, cotaTotal int, percentual float64, anoMes string) {
	estado := CarregarCota()
	return estado.CaracteresUsados, CotaGratuitaCaracteresMes, PercentUsado(estado), estado.AnoMes
}

// ResetarCota força o reset da cota (para testes ou uso manual). Não é chamado em produção.
func ResetarCota() error {
	mu.Lock()
	defer mu.Unlock()

	return salvarCotaInterno(EstadoCota{AnoMes: anoMesAtual(), CaracteresUsados: 0})
}

// LimparArquivoCota remove o arquivo de cota do disco. Usado por ExcluirTudo.
func LimparArquivoCota() error {
	caminho, err := caminhoCota()
	if err != nil {
		return err
	}
	if err := os.Remove(caminho); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("falha ao remover cota_traducao.json: %w", err)
	}
	return nil
}
