package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type Config struct {
	IntervaloCapturaSegundos     int     `json:"intervaloCapturaSegundos"`
	ConfiancaMinimaOcr           float64 `json:"confiancaMinimaOcr"`
	ThreadsCpuOcr                int     `json:"threadsCpuOcr"`
	HardwareSelecionado          string  `json:"hardwareSelecionado"`
	DispositivoOcr               string  `json:"dispositivoOcr"`
	ModeloOcr                    string  `json:"modeloOcr"`
	EscalaResolucaoOcr           int     `json:"escalaResolucaoOcr"`
	LimitarPorUsoCpu             bool    `json:"limitarPorUsoCpu"`
	UsoMaximoCpuPercent          float64 `json:"usoMaximoCpuPercent"`
	LimitarPorUsoGpu             bool    `json:"limitarPorUsoGpu"`
	UsoMaximoGpuPercent          float64 `json:"usoMaximoGpuPercent"`
	DistanciaMaximaHoverPx       int     `json:"distanciaMaximaHoverPx"`
	IntervaloAtualizacaoHoverMs  int     `json:"intervaloAtualizacaoHoverMs"`
	HabilitarPopupHover          bool    `json:"habilitarPopupHover"`
	TempoParadoPopupMs           int     `json:"tempoParadoPopupMs"`
	DestacarEstudoTela           bool    `json:"destacarEstudoTela"`
	MonitorAlvo                  int     `json:"monitorAlvo"`
	AtalhoEscanear               string  `json:"atalhoEscanear"`
	AtalhoPopupTodos             string  `json:"atalhoPopupTodos"`
	AtalhoMarcarEstudo           string  `json:"atalhoMarcarEstudo"`
	AtalhoAlternarPopupHover     string  `json:"atalhoAlternarPopupHover"`
}

func DefaultConfig() Config {
	return Config{
		IntervaloCapturaSegundos:    10,
		ConfiancaMinimaOcr:          0.5,
		ThreadsCpuOcr:               4,
		HardwareSelecionado:         "CPU",
		DispositivoOcr:              "cpu",
		ModeloOcr:                   "RapidOCR",
		EscalaResolucaoOcr:          100,
		LimitarPorUsoCpu:            false,
		UsoMaximoCpuPercent:         80.0,
		LimitarPorUsoGpu:            false,
		UsoMaximoGpuPercent:         80.0,
		DistanciaMaximaHoverPx:      220,
		IntervaloAtualizacaoHoverMs: 120,
		HabilitarPopupHover:         true,
		TempoParadoPopupMs:          500,
		DestacarEstudoTela:          true,
		MonitorAlvo:                 0,
		AtalhoEscanear:              "ctrl+shift+e",
		AtalhoPopupTodos:            "ctrl+shift+t",
		AtalhoMarcarEstudo:          "ctrl+shift+m",
		AtalhoAlternarPopupHover:    "ctrl+shift+h",
	}
}

func GetConfigPath() (string, error) {
	appData, err := os.UserConfigDir() // %AppData% no Windows
	if err != nil {
		return "", err
	}
	dir := filepath.Join(appData, "HanziTracker")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}
	return filepath.Join(dir, "configuracoes.json"), nil
}

func LoadConfig() (Config, error) {
	cfg := DefaultConfig()
	path, err := GetConfigPath()
	if err != nil {
		return cfg, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			// Salva o default caso não exista
			SaveConfig(cfg)
			return cfg, nil
		}
		return cfg, err
	}

	if err := json.Unmarshal(data, &cfg); err != nil {
		return cfg, fmt.Errorf("falha ao parsear config: %w", err)
	}

	return cfg, nil
}

func SaveConfig(cfg Config) error {
	path, err := GetConfigPath()
	if err != nil {
		return err
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
}
