package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type Config struct {
	IntervaloCapturaSegundos      int      `json:"intervaloCapturaSegundos"`
	ConfiancaMinimaOcr            float64  `json:"confiancaMinimaOcr"`
	ThreadsCpuOcr                 int      `json:"threadsCpuOcr"`
	HardwareSelecionado           string   `json:"hardwareSelecionado"`
	DispositivoOcr                string   `json:"dispositivoOcr"`
	ModeloOcr                     string   `json:"modeloOcr"`
	MotorOcrAtivo                 string   `json:"motorOcrAtivo"` // qual MOTOR (sidecar) subir no início; ver motores.go
	EscalaResolucaoOcr            int      `json:"escalaResolucaoOcr"`
	LimitarPorUsoCpu              bool     `json:"limitarPorUsoCpu"`
	UsoMaximoCpuPercent           float64  `json:"usoMaximoCpuPercent"`
	LimitarPorUsoGpu              bool     `json:"limitarPorUsoGpu"`
	UsoMaximoGpuPercent           float64  `json:"usoMaximoGpuPercent"`
	DistanciaMaximaHoverPx        int      `json:"distanciaMaximaHoverPx"`
	IntervaloAtualizacaoHoverMs   int      `json:"intervaloAtualizacaoHoverMs"`
	HabilitarPopupHover           bool     `json:"habilitarPopupHover"`
	TempoParadoPopupMs            int      `json:"tempoParadoPopupMs"`
	DestacarEstudoTela            bool     `json:"destacarEstudoTela"`
	DestacarEstudoParcialTela     bool     `json:"destacarEstudoParcialTela"`
	MonitorAlvo                   int      `json:"monitorAlvo"`
	AtalhoEscanear                string   `json:"atalhoEscanear"`
	AtalhoPopupTodos              string   `json:"atalhoPopupTodos"`
	AtalhoMarcarEstudo            string   `json:"atalhoMarcarEstudo"`
	AtalhoAlternarPopupHover      string   `json:"atalhoAlternarPopupHover"`
	TraducaoApiKey                string   `json:"traducaoApiKey"`
	TraducaoAtiva                 bool     `json:"traducaoAtiva"`
	TraducaoPausarPorCota         bool     `json:"traducaoPausarPorCota"`
	TraducaoLimiteCotaPercent     float64  `json:"traducaoLimiteCotaPercent"`
	TraducaoUsarCache             bool     `json:"traducaoUsarCache"`
	GeminiApiKey                  string   `json:"geminiApiKey"`
	GeminiAtivo                   bool     `json:"geminiAtivo"`
	GeminiPopupResumo             bool     `json:"geminiPopupResumo"`
	GeminiPopupLinha              bool     `json:"geminiPopupLinha"`
	GeminiCantoResumo             string   `json:"geminiCantoResumo"`
	GeminiEnviarImagem            bool     `json:"geminiEnviarImagem"`
	GeminiPausarPorCota           bool     `json:"geminiPausarPorCota"`
	GeminiLimiteRequisicoesDia    int      `json:"geminiLimiteRequisicoesDia"`
	GeminiModelo                  string   `json:"geminiModelo"`
	CensurarJanelasDoApp          bool     `json:"censurarJanelasDoApp"`
	HabilitarLeituraPinyin        bool     `json:"habilitarLeituraPinyin"`
	LerPinyinAoAbrirPopup         bool     `json:"lerPinyinAoAbrirPopup"`
	LerPinyinAoExpandirCard       bool     `json:"lerPinyinAoExpandirCard"`
	LerPinyinAoCompletarDesenho   bool     `json:"lerPinyinAoCompletarDesenho"`
	MotorTtsAtivo                 string   `json:"motorTtsAtivo"`
	MotorSttAtivo                 string   `json:"motorSttAtivo"`                 // motor de reconhecimento de fala da revisão de pronúncia; ver stt.go
	PriorizarEstudoRevisao        bool     `json:"priorizarEstudoRevisao"`        // revisão prioriza o grupo de foco dos hanzis em estudo (ver foco_revisao.go)
	TamanhoFocoRevisao            int      `json:"tamanhoFocoRevisao"`            // quantos caracteres em estudo ficam no grupo de foco por vez (modo manual)
	TamanhoFocoAutomatico         bool     `json:"tamanhoFocoAutomatico"`         // se true, o grupo cresce sozinho para caber todos os em estudo elegíveis (até o teto)
	SonsRevisao                   bool     `json:"sonsRevisao"`                   // jingles de acerto/erro/conclusão na revisão
	ModosRevisaoGeralDesativados  []string `json:"modosRevisaoGeralDesativados"`  // modalidades que o usuário desligou da revisão geral (vazio = todas ativas)
	AtividadesDesativadas         []string `json:"atividadesDesativadas"`         // sub-atividades (variantes) desligadas pelo usuário (vazio = todas ativas)
	RevisaoFiltroTema             string   `json:"revisaoFiltroTema"`             // rótulo chinês da taxonomia de frases (dicionario/frases/taxonomia); vazio = sem filtro
	RevisaoFiltroDificuldade      string   `json:"revisaoFiltroDificuldade"`      // rótulo chinês da taxonomia de frases (dicionario/frases/taxonomia); vazio = sem filtro
	RevisaoQuantidadeQuestoes     int      `json:"revisaoQuantidadeQuestoes"`     // quantidade de questões da revisão geral (mínimo 8, máximo 50, padrão 10)
	RevisarErradasAoFinal         bool     `json:"revisarErradasAoFinal"`         // se true, cria rodada de recuperação das questões erradas no final da revisão
	RevisaoIaVibe                 string   `json:"revisaoIaVibe"`                 // clima/vibe das frases geradas com IA
	TipoHanziGerado               string   `json:"tipoHanziGerado"`               // "ambos", "tradicional", "simplificado"
	TipoHanziExibicao             string   `json:"tipoHanziExibicao"`             // "ambos", "tradicional", "simplificado"
	RestringirHanziDesenho        bool     `json:"restringirHanziDesenho"`        // aplica a regra de exibição na busca por desenho
	MostrarSugestaoPalavrasVistas bool     `json:"mostrarSugestaoPalavrasVistas"` // pop-up que sugere estudar as palavras mais vistas pelo OCR (abas de seção/já vistas)
	VigiaCardsAtivo               bool     `json:"vigiaCardsAtivo"`               // vigia (1×/s entre scans) que apaga highlights fantasmas de palavras que saíram da tela (ver vigia_cards.go)
	AutoScanAtivo                 bool     `json:"autoScanAtivo"`                 // se true, o escaneamento de tela automático/periódico fica ativo
	RastrearPalavrasPerdidas      bool     `json:"rastrearPalavrasPerdidas"`      // vigia também procura os cards perdidos pela tela inteira e reativa onde achar (custoso em telas grandes)
	IdiomaTraducao                string   `json:"idiomaTraducao"`                // idioma das definições/dicas/frases do dicionário ("en", "pt-BR"); inglês é o fallback (ver dicionario/idiomas.go)
	CanalAtualizacao              string   `json:"canalAtualizacao"`              // canal de atualização ("estavel" ou "dev"); vazio = auto-detectado no boot
}

func DefaultConfig() Config {
	return Config{
		CanalAtualizacao:              "",
		IntervaloCapturaSegundos:      10,
		ConfiancaMinimaOcr:            0.5,
		ThreadsCpuOcr:                 4,
		HardwareSelecionado:           "CPU",
		DispositivoOcr:                "cpu",
		ModeloOcr:                     "RapidOCR",
		MotorOcrAtivo:                 "RapidOCR",
		EscalaResolucaoOcr:            100,
		LimitarPorUsoCpu:              false,
		UsoMaximoCpuPercent:           80.0,
		LimitarPorUsoGpu:              false,
		UsoMaximoGpuPercent:           80.0,
		DistanciaMaximaHoverPx:        220,
		IntervaloAtualizacaoHoverMs:   120,
		HabilitarPopupHover:           true,
		TempoParadoPopupMs:            500,
		DestacarEstudoTela:            true,
		DestacarEstudoParcialTela:     true,
		MonitorAlvo:                   0,
		AtalhoEscanear:                "ctrl+shift+e",
		AtalhoPopupTodos:              "ctrl+shift+t",
		AtalhoMarcarEstudo:            "ctrl+shift+m",
		AtalhoAlternarPopupHover:      "ctrl+shift+h",
		TraducaoApiKey:                "",
		TraducaoAtiva:                 false,
		TraducaoPausarPorCota:         true,
		TraducaoLimiteCotaPercent:     90,
		TraducaoUsarCache:             true,
		GeminiApiKey:                  "",
		GeminiAtivo:                   false,
		GeminiPopupResumo:             true,
		GeminiPopupLinha:              false,
		GeminiCantoResumo:             "inferior-direito",
		GeminiEnviarImagem:            false,
		GeminiPausarPorCota:           true,
		GeminiLimiteRequisicoesDia:    1500,
		GeminiModelo:                  "gemini-2.5-flash",
		CensurarJanelasDoApp:          true,
		HabilitarLeituraPinyin:        false,
		LerPinyinAoAbrirPopup:         false,
		LerPinyinAoExpandirCard:       false,
		LerPinyinAoCompletarDesenho:   true,
		MotorTtsAtivo:                 "Kokoro-82M",
		MotorSttAtivo:                 "Paraformer-ZH",
		PriorizarEstudoRevisao:        true,
		TamanhoFocoRevisao:            5,
		TamanhoFocoAutomatico:         false,
		SonsRevisao:                   true,
		ModosRevisaoGeralDesativados:  nil,
		RevisaoFiltroTema:             "",
		RevisaoFiltroDificuldade:      "",
		RevisaoQuantidadeQuestoes:     10,
		RevisarErradasAoFinal:         true,
		RevisaoIaVibe:                 "",
		TipoHanziGerado:               "ambos",
		TipoHanziExibicao:             "simplificado",
		RestringirHanziDesenho:        true,
		MostrarSugestaoPalavrasVistas: true,
		VigiaCardsAtivo:               true,
		AutoScanAtivo:                 true,
		RastrearPalavrasPerdidas:      false,
		IdiomaTraducao:                "en",
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

	// Configs salvas antes do grupo de foco existir desserializam o campo como 0.
	if cfg.TamanhoFocoRevisao <= 0 {
		cfg.TamanhoFocoRevisao = DefaultConfig().TamanhoFocoRevisao
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
