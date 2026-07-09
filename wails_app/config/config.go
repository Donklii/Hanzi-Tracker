package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type Config struct {
	IntervaloCapturaSegundos    int     `json:"intervaloCapturaSegundos"`
	ConfiancaMinimaOcr          float64 `json:"confiancaMinimaOcr"`
	ThreadsCpuOcr               int     `json:"threadsCpuOcr"`
	HardwareSelecionado         string  `json:"hardwareSelecionado"`
	DispositivoOcr              string  `json:"dispositivoOcr"`
	ModeloOcr                   string  `json:"modeloOcr"`
	MotorOcrAtivo               string  `json:"motorOcrAtivo"` // qual MOTOR (sidecar) subir no início; ver motores.go
	EscalaResolucaoOcr          int     `json:"escalaResolucaoOcr"`
	LimitarPorUsoCpu            bool    `json:"limitarPorUsoCpu"`
	UsoMaximoCpuPercent         float64 `json:"usoMaximoCpuPercent"`
	LimitarPorUsoGpu            bool    `json:"limitarPorUsoGpu"`
	UsoMaximoGpuPercent         float64 `json:"usoMaximoGpuPercent"`
	DistanciaMaximaHoverPx      int     `json:"distanciaMaximaHoverPx"`
	IntervaloAtualizacaoHoverMs int     `json:"intervaloAtualizacaoHoverMs"`
	HabilitarPopupHover         bool    `json:"habilitarPopupHover"`
	TempoParadoPopupMs          int     `json:"tempoParadoPopupMs"`
	DestacarEstudoTela          bool    `json:"destacarEstudoTela"`
	DestacarEstudoParcialTela   bool    `json:"destacarEstudoParcialTela"`
	MonitorAlvo                 int     `json:"monitorAlvo"`
	AtalhoEscanear              string  `json:"atalhoEscanear"`
	AtalhoPopupTodos            string  `json:"atalhoPopupTodos"`
	AtalhoMarcarEstudo          string  `json:"atalhoMarcarEstudo"`
	AtalhoAlternarPopupHover    string  `json:"atalhoAlternarPopupHover"`
	TraducaoApiKey              string  `json:"traducaoApiKey"`
	TraducaoAtiva               bool    `json:"traducaoAtiva"`
	TraducaoPausarPorCota       bool    `json:"traducaoPausarPorCota"`
	TraducaoLimiteCotaPercent   float64 `json:"traducaoLimiteCotaPercent"`
	TraducaoUsarCache           bool    `json:"traducaoUsarCache"`
	GeminiApiKey                string  `json:"geminiApiKey"`
	GeminiAtivo                 bool    `json:"geminiAtivo"`
	GeminiPopupResumo           bool    `json:"geminiPopupResumo"`
	GeminiPopupLinha            bool    `json:"geminiPopupLinha"`
	GeminiCantoResumo           string  `json:"geminiCantoResumo"`
	GeminiEnviarImagem          bool    `json:"geminiEnviarImagem"`
	GeminiPausarPorCota         bool    `json:"geminiPausarPorCota"`
	GeminiLimiteRequisicoesDia  int     `json:"geminiLimiteRequisicoesDia"`
	GeminiModelo                string  `json:"geminiModelo"`
	CensurarJanelasDoApp        bool    `json:"censurarJanelasDoApp"`
	HabilitarLeituraPinyin      bool    `json:"habilitarLeituraPinyin"`
	LerPinyinAoAbrirPopup       bool    `json:"lerPinyinAoAbrirPopup"`
	LerPinyinAoExpandirCard     bool    `json:"lerPinyinAoExpandirCard"`
	LerPinyinAoCompletarDesenho bool    `json:"lerPinyinAoCompletarDesenho"`
	MotorTtsAtivo               string  `json:"motorTtsAtivo"`
	MotorSttAtivo               string  `json:"motorSttAtivo"` // motor de reconhecimento de fala da revisão de pronúncia; ver stt.go
	PriorizarEstudoRevisao      bool    `json:"priorizarEstudoRevisao"` // revisão sorteia primeiro os hanzis em estudo
	SonsRevisao                 bool    `json:"sonsRevisao"`            // jingles de acerto/erro/conclusão na revisão
	TipoHanziGerado             string  `json:"tipoHanziGerado"`        // "ambos", "tradicional", "simplificado"
	TipoHanziExibicao           string  `json:"tipoHanziExibicao"`      // "ambos", "tradicional", "simplificado"
	RestringirHanziDesenho      bool    `json:"restringirHanziDesenho"` // aplica a regra de exibição na busca por desenho
	DriveClientId               string  `json:"driveClientId"`          // credenciais OAuth da sincronização com o Google Drive,
	DriveClientSecret           string  `json:"driveClientSecret"`      // coladas pelo usuário na aba Armazenamento (ver wails_app/nuvem)
}

func DefaultConfig() Config {
	return Config{
		IntervaloCapturaSegundos:    10,
		ConfiancaMinimaOcr:          0.5,
		ThreadsCpuOcr:               4,
		HardwareSelecionado:         "CPU",
		DispositivoOcr:              "cpu",
		ModeloOcr:                   "RapidOCR",
		MotorOcrAtivo:               "RapidOCR",
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
		DestacarEstudoParcialTela:   true,
		MonitorAlvo:                 0,
		AtalhoEscanear:              "ctrl+shift+e",
		AtalhoPopupTodos:            "ctrl+shift+t",
		AtalhoMarcarEstudo:          "ctrl+shift+m",
		AtalhoAlternarPopupHover:    "ctrl+shift+h",
		TraducaoApiKey:              "",
		TraducaoAtiva:               false,
		TraducaoPausarPorCota:       true,
		TraducaoLimiteCotaPercent:   90,
		TraducaoUsarCache:           true,
		GeminiApiKey:                "",
		GeminiAtivo:                 false,
		GeminiPopupResumo:           true,
		GeminiPopupLinha:            false,
		GeminiCantoResumo:           "inferior-direito",
		GeminiEnviarImagem:          false,
		GeminiPausarPorCota:         true,
		GeminiLimiteRequisicoesDia:  1500,
		GeminiModelo:                "gemini-2.5-flash",
		CensurarJanelasDoApp:        true,
		HabilitarLeituraPinyin:      false,
		LerPinyinAoAbrirPopup:       false,
		LerPinyinAoExpandirCard:     false,
		LerPinyinAoCompletarDesenho: true,
		MotorTtsAtivo:               "Kokoro-82M",
		MotorSttAtivo:               "Paraformer-ZH",
		PriorizarEstudoRevisao:      true,
		SonsRevisao:                 true,
		TipoHanziGerado:             "ambos",
		TipoHanziExibicao:           "simplificado",
		RestringirHanziDesenho:      true,
		DriveClientId:               "",
		DriveClientSecret:           "",
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

	// Migração: "directml" e "cuda" são valores pré-WebGPU de DispositivoOcr. A intenção era
	// acelerar por GPU, então viram "webgpu" (o sidecar atual só conhece cpu/webgpu).
	if cfg.DispositivoOcr == "directml" || cfg.DispositivoOcr == "cuda" {
		cfg.DispositivoOcr = "webgpu"
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
