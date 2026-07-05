package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"image"
	"image/draw"
	"image/png"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"

	"wails_app/overlay"
	"wails_app/traducao"

	"github.com/kbinani/screenshot"
	"github.com/wailsapp/wails/v2/pkg/runtime"
	"wails_app/baixador"
	"wails_app/config"
	"wails_app/dicionario"
	"wails_app/motoresocr"
	"wails_app/motorestts"
	"wails_app/progresso"
	"wails_app/segmentacao"
)

// LinhaTraduzida armazena a tradução de uma linha OCR inteira (antes da segmentação em palavras).
type LinhaTraduzida struct {
	Texto    string    `json:"texto"`
	Traducao string    `json:"traducao"` // "" se não traduzida (feature off, cota estourada ou erro de API)
	Caixa    []float64 `json:"caixa"`    // caixa da LINHA inteira (res.Caixa), NÃO repartida por palavra
}

// InfoCotaTraducao é o DTO exposto ao frontend com o estado de cota de tradução.
type InfoCotaTraducao struct {
	CaracteresUsados int     `json:"caracteresUsados"`
	CotaTotal        int     `json:"cotaTotal"`
	Percentual       float64 `json:"percentual"`
	AnoMes           string  `json:"anoMes"`
}

// App struct
type App struct {
	ctx           context.Context
	Config        config.Config
	Cedict        *dicionario.Cedict
	BancoHanzi    *dicionario.BancoMakeMeAHanzi
	BancoFrases   *dicionario.BancoFrases   // frases Tatoeba p/ revisão por contexto (carga preguiçosa)
	BancoTracados *dicionario.BancoTracados // traçados Hanzi Writer p/ revisão de desenho (carga preguiçosa)
	lastImageHash string
	mu            sync.RWMutex
	lastCards     []FlashcardCard
	lastLinhas    []LinhaTraduzida
	lastImagemPng []byte // última captura JÁ CENSURADA, guardada para o modo resumo do Gemini poder enviar a imagem

	popupsTodosVisivel bool

	// motorOcr é dono do ciclo de vida do processo de OCR (subir/derrubar/trocar). A posse migrou do
	// orquestrador (main.go) para o app para permitir trocar de motor em runtime (Fase 5, Passo 1).
	motorOcr *motoresocr.GerenciadorMotorOcr

	// motorTts é dono do ciclo de vida do processo de TTS (Kokoro-82M/ChatTTS). Criado
	// PREGUIÇOSAMENTE na primeira leitura em voz alta (garantirMotorTts) — nil até lá. ttsMutex
	// serializa as leituras e protege essa criação (ver tts.go).
	motorTts *motorestts.GerenciadorMotorTts
	ttsMutex sync.Mutex

	// Pré-carregamento do cache de TTS (ver tts_precache.go): sintetiza EM LOTE a fala de todas as
	// palavras dos dicionários. Um lote longo por vez — preCacheTtsAtivo barra reentrância e
	// preCacheTtsCancelar sinaliza o cancelamento cooperativo. Protegidos por preCacheTtsMutex.
	preCacheTtsMutex    sync.Mutex
	preCacheTtsAtivo    bool
	preCacheTtsCancelar chan struct{}

	// mapaStatusRevisao armazena o status de cada caractere (estudo/aprendido) durante uma sessão
	// de revisão, usado pela seleção ponderada de frases em preencherFrase.
	mapaStatusRevisao map[string]string
}

// NewApp creates a new App application struct
func NewApp() *App {
	cfg, err := config.LoadConfig()
	if err != nil {
		fmt.Printf("Erro ao carregar config, usando default: %v\n", err)
		cfg = config.DefaultConfig()
	}

	cedict := dicionario.NovoCedict()
	err = cedict.Carregar()
	if err != nil {
		fmt.Printf("Aviso: Falha ao carregar dicionário CC-CEDICT: %v\n", err)
	}

	err = segmentacao.InitJieba()
	if err != nil {
		fmt.Printf("Aviso: Falha ao carregar dicionário Jieba: %v\n", err)
	}

	bancoHanzi := dicionario.NovoBancoMakeMeAHanzi()
	err = bancoHanzi.Carregar()
	if err != nil {
		fmt.Printf("Aviso: Falha ao carregar dicionário MakeMeAHanzi: %v\n", err)
	}

	err = progresso.InitDB()
	if err != nil {
		fmt.Printf("Aviso: Falha ao inicializar banco de dados SQLite: %v\n", err)
	}

	return &App{
		Config:        cfg,
		Cedict:        cedict,
		BancoHanzi:    bancoHanzi,
		BancoFrases:   dicionario.NovoBancoFrases(),
		BancoTracados: dicionario.NovoBancoTracados(),
	}
}

// AddVocab
func (a *App) AddVocab(hanzi, pinyin, significado, status string) error {
	return progresso.AddOuUpdateVocab(hanzi, pinyin, significado, status)
}

// RemoveVocab
func (a *App) RemoveVocab(hanzi string) error {
	return progresso.RemoveVocab(hanzi)
}

// AvaliarTipoHanzi retorna o tipo do hanzi para o frontend ("Tradicional", "Simplificado" ou "Ambos")
func (a *App) AvaliarTipoHanzi(hanzi string) string {
	return a.Cedict.AvaliarTipoHanzi(hanzi)
}

func (a *App) GetVocab() ([]progresso.Vocab, error) {
	v, err := progresso.GetAllVocab()
	if err == nil {
		var filtrado []progresso.Vocab
		for i := range v {
			if _, ehAbrev := dicionario.MapaAbrevParaCompleto[v[i].Hanzi]; ehAbrev {
				continue // Oculta componentes e radicais avulsos do histórico
			}
			v[i].TipoHanzi = a.Cedict.AvaliarTipoHanzi(v[i].Hanzi)
			filtrado = append(filtrado, v[i])
		}
		v = filtrado
	}
	return v, err
}

// GetConfig returns the current configuration
func (a *App) GetConfig() (config.Config, error) {
	return a.Config, nil
}

// ShowHoverPopup exibe o card único perto do mouse.
func (a *App) ShowHoverPopup(pinyin, hanzi, sig string, x, y int) {
	overlay.Show(pinyin, hanzi, sig, x, y)
}

// HideHoverPopup oculta o card único.
func (a *App) HideHoverPopup() {
	overlay.Hide()
}

func contemHanzi(s string) bool {
	for _, r := range s {
		if unicode.Is(unicode.Han, r) {
			return true
		}
	}
	return false
}

// GetCotaTraducao retorna o estado atual da cota de tradução para exibição no frontend.
func (a *App) GetCotaTraducao() InfoCotaTraducao {
	usados, total, pct, anoMes := traducao.InfoCotaParaUI()
	return InfoCotaTraducao{
		CaracteresUsados: usados,
		CotaTotal:        total,
		Percentual:       pct,
		AnoMes:           anoMes,
	}
}

// alternarTodosPopups liga/desliga a exibição simultânea dos pop-ups de todos os cards atuais.
// Acionado pelo atalho "mostrar pop-up de tudo" (AtalhoPopupTodos).
func (a *App) alternarTodosPopups() {
	if a.popupsTodosVisivel {
		a.ocultarTodosPopups()
		a.popupsTodosVisivel = false
		return
	}
	a.mostrarTodosPopups()
	a.popupsTodosVisivel = true
}

// mostrarTodosPopups envia todos os cards do último scan ao overlay nativo para exibição simultânea.
// Quando a tradução está ativa e há linhas traduzidas, monta os pop-ups por LINHA (um pop-up por
// linha OCR com a tradução) em vez do modo padrão (um pop-up por palavra com pinyin/significado).
func (a *App) mostrarTodosPopups() {
	monitorID := a.Config.MonitorAlvo
	if monitorID >= screenshot.NumActiveDisplays() {
		monitorID = 0
	}
	bounds := screenshot.GetDisplayBounds(monitorID)
	offX := bounds.Min.X
	offY := bounds.Min.Y

	a.mu.RLock()
	linhasCopia := make([]LinhaTraduzida, len(a.lastLinhas))
	copy(linhasCopia, a.lastLinhas)
	cardsCopia := make([]FlashcardCard, len(a.lastCards))
	copy(cardsCopia, a.lastCards)
	a.mu.RUnlock()

	// Desvio para Gemini: se ativo, tem chave e temos linhas, tentamos exibir pelo Gemini primeiro.
	if a.Config.GeminiAtivo && a.Config.GeminiApiKey != "" && len(linhasCopia) > 0 {
		if a.Config.GeminiPopupResumo {
			if a.mostrarResumoGemini(linhasCopia, bounds.Min.X, bounds.Min.Y, bounds.Dx(), bounds.Dy()) {
				return
			}
		} else if a.Config.GeminiPopupLinha {
			if a.mostrarPopupsLinhaGemini(linhasCopia, offX, offY, bounds.Dx(), bounds.Dy()) {
				return
			}
		}
	}

	// Modo LINHA: se a tradução está ativa e há pelo menos uma linha com tradução não-vazia,
	// mostra um pop-up por LINHA (Hanzi = texto original, Sig = tradução, Pinyin = "").
	if !a.Config.GeminiAtivo && a.Config.TraducaoAtiva && a.Config.TraducaoApiKey != "" && len(linhasCopia) > 0 {
		a.traduzirLinhasPendentes(linhasCopia)

		var itens []overlay.ItemPopup
		temTraducao := false
		for i := range linhasCopia {
			l := &linhasCopia[i]
			if l.Traducao == "" {
				continue
			}
			temTraducao = true
			if len(l.Caixa) == 4 {
				itens = append(itens, overlay.ItemPopup{
					Pinyin:     "",
					Hanzi:      "",
					Sig:        l.Traducao,
					SoTraducao: true,
					X0:         int(l.Caixa[0]) + offX,
					Y0:         int(l.Caixa[1]) + offY,
					X1:         int(l.Caixa[2]) + offX,
					Y1:         int(l.Caixa[3]) + offY,
				})
			}
		}

		if temTraducao {
			overlay.MostrarTodos(itens, bounds.Dx(), bounds.Dy())
			return
		}
	}

	// Modo PALAVRA (padrão): um pop-up por palavra com pinyin/significado.
	var itens []overlay.ItemPopup
	for _, c := range cardsCopia {
		if len(c.Caixa) != 4 {
			continue
		}
		itens = append(itens, overlay.ItemPopup{
			Pinyin: c.Pinyin,
			Hanzi:  c.Hanzi,
			Sig:    strings.Join(c.Significados, ", "),
			X0:     int(c.Caixa[0]) + offX,
			Y0:     int(c.Caixa[1]) + offY,
			X1:     int(c.Caixa[2]) + offX,
			Y1:     int(c.Caixa[3]) + offY,
		})
	}

	overlay.MostrarTodos(itens, bounds.Dx(), bounds.Dy())
}

// ocultarTodosPopups remove todos os pop-ups exibidos pelo "mostrar pop-up de tudo".
func (a *App) ocultarTodosPopups() {
	overlay.OcultarTodos()
	overlay.OcultarResumo()
}

// traduzirLinhasPendentes preenche in place a Traducao das linhas com hanzi que ainda não a têm:
// primeiro pelo cache (se ligado) e depois com UMA chamada em LOTE à API — a cota é cobrada por
// caractere, então o lote custa o mesmo que N chamadas sequenciais e o pop-up aparece muito mais
// rápido. Falha de API apenas deixa as traduções vazias (o chamador ignora linhas sem tradução).
func (a *App) traduzirLinhasPendentes(linhas []LinhaTraduzida) {
	var pendentes []int
	for i := range linhas {
		l := &linhas[i]
		if !contemHanzi(l.Texto) || l.Traducao != "" {
			continue
		}

		if a.Config.TraducaoUsarCache {
			if cached, achou, err := progresso.BuscarTraducaoCache(l.Texto); err == nil && achou {
				l.Traducao = cached
				continue
			}
		}
		pendentes = append(pendentes, i)
	}

	// Guard clauses: nada a traduzir, ou a pausa por cota barrou a chamada de API.
	if len(pendentes) == 0 {
		return
	}
	if a.Config.TraducaoPausarPorCota && traducao.CotaExcedida(a.Config.TraducaoLimiteCotaPercent) {
		return
	}

	textos := make([]string, len(pendentes))
	totalCaracteres := 0
	for j, idx := range pendentes {
		textos[j] = linhas[idx].Texto
		totalCaracteres += len([]rune(linhas[idx].Texto))
	}

	traducoes, err := traducao.TraduzirLote(a.Config.TraducaoApiKey, textos, "pt")
	if err != nil {
		fmt.Printf("Aviso: tradução em lote falhou: %v\n", err)
		return
	}
	_ = traducao.RegistrarUso(totalCaracteres)

	for j, idx := range pendentes {
		if traducoes[j] == "" {
			continue
		}
		linhas[idx].Traducao = traducoes[j]
		if a.Config.TraducaoUsarCache {
			_ = progresso.SalvarTraducaoCache(linhas[idx].Texto, traducoes[j])
		}
	}
}

// ShowHighlight desenha uma borda ao redor de uma área específica
func (a *App) ShowHighlight(x0, y0, x1, y1 int) {
	monitorID := a.Config.MonitorAlvo
	if monitorID >= screenshot.NumActiveDisplays() {
		monitorID = 0
	}
	bounds := screenshot.GetDisplayBounds(monitorID)

	overlay.ShowHighlight(x0+bounds.Min.X, y0+bounds.Min.Y, x1+bounds.Min.X, y1+bounds.Min.Y)
}

// ShowEstudoHighlights envia molduras azuis para indicar palavras em estudo
func (a *App) ShowEstudoHighlights(boxes [][]float64) {
	monitorID := a.Config.MonitorAlvo
	if monitorID >= screenshot.NumActiveDisplays() {
		monitorID = 0
	}
	bounds := screenshot.GetDisplayBounds(monitorID)

	adjustedBoxes := make([][]float64, 0, len(boxes))
	for _, box := range boxes {
		if len(box) == 4 {
			adjustedBoxes = append(adjustedBoxes, []float64{
				box[0] + float64(bounds.Min.X),
				box[1] + float64(bounds.Min.Y),
				box[2] + float64(bounds.Min.X),
				box[3] + float64(bounds.Min.Y),
			})
		}
	}

	overlay.ShowEstudoHighlights(adjustedBoxes)
}

// ShowEstudoParcialHighlights envia molduras amarelas para indicar caracteres individuais em estudo dentro de palavras
func (a *App) ShowEstudoParcialHighlights(boxes [][]float64) {
	monitorID := a.Config.MonitorAlvo
	if monitorID >= screenshot.NumActiveDisplays() {
		monitorID = 0
	}
	bounds := screenshot.GetDisplayBounds(monitorID)

	adjustedBoxes := make([][]float64, 0, len(boxes))
	for _, box := range boxes {
		if len(box) == 4 {
			adjustedBoxes = append(adjustedBoxes, []float64{
				box[0] + float64(bounds.Min.X),
				box[1] + float64(bounds.Min.Y),
				box[2] + float64(bounds.Min.X),
				box[3] + float64(bounds.Min.Y),
			})
		}
	}

	overlay.ShowEstudoParcialHighlights(adjustedBoxes)
}

// SaveConfig saves the configuration and updates the App state
func (a *App) SaveConfig(newConfig config.Config) error {
	a.Config = newConfig
	return config.SaveConfig(newConfig)
}

// shutdown is called at termination
func (a *App) shutdown(ctx context.Context) {
	// Derruba o motor de OCR (e sua árvore) — o app é dono desse processo desde a migração da posse.
	if a.motorOcr != nil {
		a.motorOcr.Encerrar()
	}
	// Derruba o motor de TTS, se alguma leitura em voz alta o subiu nesta sessão (é preguiçoso).
	if a.motorTts != nil {
		a.motorTts.Encerrar()
	}
	overlay.Encerrar()
	progresso.LimparImagensSessao()
}

// breakIntoDictionaryWords usa Forward Maximum Matching para quebrar OOV (Out-Of-Vocabulary) em palavras válidas
func (a *App) breakIntoDictionaryWords(text string) []string {
	var result []string
	runes := []rune(text)

	for i := 0; i < len(runes); {
		matched := false
		// Tenta a maior substring possível a partir do índice 'i'
		for j := len(runes); j > i; j-- {
			sub := string(runes[i:j])

			isValid := false
			if utf8.RuneCountInString(sub) == 1 {
				isValid = true // Caracteres únicos sempre passam como fallback
			} else {
				entradas := a.Cedict.Buscar(sub)
				if len(entradas) > 0 {
					isValid = true
				}
			}

			if isValid {
				result = append(result, sub)
				i = j
				matched = true
				break
			}
		}

		if !matched {
			// Prevenção de loop infinito (embora tamanho 1 sempre dê match)
			result = append(result, string(runes[i:i+1]))
			i++
		}
	}

	return result
}

// startup is called when the app starts. The context is saved
// so we can call the runtime methods
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	a.StartBackgroundLoop()
	fmt.Println("Backend Go Inicializado.")

	// Aplica a escolha de motores feita na tela custom do instalador (ver instalador.go), ANTES de
	// resolver/bootstrapar o motor — é o que faz bootstrapMotorPadrao baixar o motor ESCOLHIDO em vez
	// do padrão do catálogo. No-op silencioso em builds de dev (sem instalador, sem marcador).
	a.aplicarEscolhaDoInstalador()

	// O app é dono do processo de OCR (subir/derrubar/trocar). Todo motor é um EXECUTÁVEL — baixado no
	// AppData ou empacotado num bundle ao lado do app; NÃO há mais fallback para `python server.py`
	// (tudo é modular/baixável). motoresocr.ResolverMotorInicial escolhe o motor preferido/padrão
	// instalado ou o bundle; se NADA existe (first-run), o bootstrap baixa+instala+ativa o RapidOCR
	// padrão sozinho.
	a.motorOcr = motoresocr.NovoGerenciadorMotorOcr()
	if desc, ok := motoresocr.ResolverMotorInicial(a.Config.MotorOcrAtivo); ok {
		if err := a.motorOcr.Iniciar(desc); err != nil {
			fmt.Printf("Aviso: falha ao subir o backend de OCR: %v\n", err)
		}

		// Espera o motor responder o healthcheck antes de anunciá-lo pronto. Roda em segundo plano
		// para não travar a UI; o frontend ouve "ocr_pronto"/"ocr_indisponivel".
		go func() {
			if err := motoresocr.AguardarBackend(30 * time.Second); err != nil {
				fmt.Printf("Aviso: motor de OCR indisponível: %v\n", err)
				runtime.EventsEmit(a.ctx, "ocr_indisponivel", err.Error())
				return
			}
			fmt.Println("Motor de OCR pronto (healthcheck ok).")
			runtime.EventsEmit(a.ctx, "ocr_pronto")
		}()

		// Inicializa o overlay embutido.
		overlay.Iniciar()
	} else {
		// First-run: nenhum motor instalado nem em bundle. Baixa o motor padrão (+ overlay) e ativa — tudo
		// em segundo plano; a UI acompanha por "motor_bootstrap_inicio"/"motor_download_progresso"/"ocr_pronto".
		fmt.Println("Nenhum motor de OCR encontrado — iniciando o bootstrap do motor padrão…")
		go a.bootstrapMotorPadrao()
	}
}

// Resolucao representa a resolução (em px) do monitor de captura.
type Resolucao struct {
	Largura int `json:"largura"`
	Altura  int `json:"altura"`
}

// GetCaptureResolution retorna a resolução nativa do monitor capturado (display alvo),
// usada como teto/padrão do controle de Qualidade da Imagem do OCR.
func (a *App) GetCaptureResolution() Resolucao {
	alvo := a.Config.MonitorAlvo
	if alvo < 0 || alvo >= screenshot.NumActiveDisplays() {
		alvo = 0
	}
	bounds := screenshot.GetDisplayBounds(alvo)
	return Resolucao{Largura: bounds.Dx(), Altura: bounds.Dy()}
}

type Monitor struct {
	ID      int    `json:"id"`
	Nome    string `json:"nome"`
	Largura int    `json:"largura"`
	Altura  int    `json:"altura"`
	X       int    `json:"x"`
	Y       int    `json:"y"`
}

// getMonitorNamesWMI usa PowerShell para extrair o nome real dos monitores no Windows
func getMonitorNamesWMI() []string {
	out, err := exec.Command("powershell", "-NoProfile", "-Command", "Get-WmiObject -Namespace root\\wmi -Class WmiMonitorID | Select-Object -ExpandProperty InstanceName").Output()
	var names []string
	if err != nil {
		return names
	}
	lines := strings.Split(string(out), "\n")
	for _, l := range lines {
		l = strings.TrimSpace(l)
		if strings.Contains(l, "DISPLAY\\") {
			parts := strings.Split(l, "\\")
			if len(parts) > 1 {
				names = append(names, parts[1])
			}
		}
	}
	return names
}

// GetMonitores retorna a lista de todos os monitores conectados
func (a *App) GetMonitores() []Monitor {
	wmiNames := getMonitorNamesWMI()
	n := screenshot.NumActiveDisplays()
	var monitores []Monitor
	for i := 0; i < n; i++ {
		bounds := screenshot.GetDisplayBounds(i)
		nome := fmt.Sprintf("Monitor %d", i+1)
		if i == 0 && len(wmiNames) == 0 {
			nome = "Monitor Principal"
		}

		if i < len(wmiNames) {
			nome = wmiNames[i]
		}

		monitores = append(monitores, Monitor{
			ID:      i,
			Nome:    nome,
			Largura: bounds.Dx(),
			Altura:  bounds.Dy(),
			X:       bounds.Min.X,
			Y:       bounds.Min.Y,
		})
	}
	return monitores
}

// SystemHardware represents the real names of the machine's CPU and GPUs
type SystemHardware struct {
	Cpu  string   `json:"cpu"`
	Gpus []string `json:"gpus"`
}

// GetSystemHardware fetches the real hardware names natively in Go
func (a *App) GetSystemHardware() SystemHardware {
	cpu := "CPU"
	out, err := exec.Command("powershell", "-NoProfile", "-Command", "(Get-ItemProperty -Path 'HKLM:\\HARDWARE\\DESCRIPTION\\System\\CentralProcessor\\0').ProcessorNameString").Output()
	if err == nil {
		cpuName := strings.TrimSpace(string(out))
		if cpuName != "" {
			cpu = cpuName
		}
	}

	var gpus []string
	out, err = exec.Command("powershell", "-NoProfile", "-Command", "Get-CimInstance Win32_VideoController | Select-Object -ExpandProperty Name").Output()
	if err == nil {
		linhas := strings.Split(string(out), "\n")
		var filtradas []string
		var todas []string
		for _, linha := range linhas {
			linha = strings.TrimSpace(linha)
			if linha == "" {
				continue
			}
			todas = append(todas, linha)

			linhaLower := strings.ToLower(linha)
			isVirtual := false
			for _, excl := range []string{"virtual", "parsec", "mirror", "remote"} {
				if strings.Contains(linhaLower, excl) {
					isVirtual = true
					break
				}
			}
			if !isVirtual {
				filtradas = append(filtradas, linha)
			}
		}

		usar := filtradas
		if len(filtradas) == 0 {
			usar = todas
		}

		for _, g := range usar {
			existe := false
			for _, e := range gpus {
				if e == g {
					existe = true
					break
				}
			}
			if !existe {
				gpus = append(gpus, g)
			}
		}
	}

	if len(gpus) == 0 {
		gpus = append(gpus, "GPU (Detecção Falhou)")
	}

	return SystemHardware{Cpu: cpu, Gpus: gpus}
}

// ArquivoModelo é um arquivo (det/rec) que compõe um modelo, com a URL de download e o hash
// esperado. O `Sha256`, quando preenchido, é conferido após o download para garantir integridade;
// vazio = verificação pulada (ver ModelosManifesto.py).
type ArquivoModelo struct {
	Nome   string `json:"nome"`
	Url    string `json:"url"`
	Sha256 string `json:"sha256"`
}

// ModeloOcrInfo espelha o estado de um modelo retornado por /api/modelos
type ModeloOcrInfo struct {
	Nome         string          `json:"nome"`
	Rotulo       string          `json:"rotulo"`
	Descricao    string          `json:"descricao"`
	Idiomas      []string        `json:"idiomas"`
	Baixavel     bool            `json:"baixavel"`
	Embutido     bool            `json:"embutido"`
	Instalado    bool            `json:"instalado"`
	TamanhoBytes int64           `json:"tamanhoBytes"`
	Arquivos     []ArquivoModelo `json:"arquivos"`
}

// ListarModelos retorna o catálogo de modelos de OCR e seu estado (instalado/embutido)
func (a *App) ListarModelos() ([]ModeloOcrInfo, error) {
	resp, err := http.Get(motoresocr.EnderecoBase() + "/api/modelos")
	if err != nil {
		return nil, fmt.Errorf("falha ao buscar modelos do Python: %w", err)
	}
	defer resp.Body.Close()

	var modelos []ModeloOcrInfo
	if err := json.NewDecoder(resp.Body).Decode(&modelos); err != nil {
		return nil, fmt.Errorf("falha ao decodificar lista de modelos: %w", err)
	}
	return modelos, nil
}

// BaixarModelo baixa os arquivos de um modelo diretamente para o AppData REAL.
// O download é feito pelo Go (e não pelo Python) porque o Python da Microsoft Store virtualiza o
// %APPDATA% para um sandbox; o Go, sendo um processo normal, escreve no caminho real que ele e o
// Python leem. Emite progresso pelo evento "modelo_download_progresso".
func (a *App) BaixarModelo(nome string) error {
	modelos, err := a.ListarModelos()
	if err != nil {
		return err
	}

	var alvo *ModeloOcrInfo
	for i := range modelos {
		if modelos[i].Nome == nome {
			alvo = &modelos[i]
			break
		}
	}
	if alvo == nil {
		return fmt.Errorf("modelo '%s' não encontrado no catálogo", nome)
	}
	if !alvo.Baixavel {
		return fmt.Errorf("o modelo '%s' não é baixável", nome)
	}

	// O catálogo veio do motor ATIVO (ListarModelos consulta o processo dele), então os pesos vão para
	// a subpasta desse motor — a mesma que o Python monta a partir de HANZITRACKER_MOTOR.
	motorAtivo := a.nomeMotorAtivo()
	if motorAtivo == "" {
		return fmt.Errorf("nenhum motor de OCR ativo para receber o modelo '%s'", nome)
	}

	destino := motoresocr.PastaModelosMotor(motorAtivo)
	if err := os.MkdirAll(destino, 0755); err != nil {
		return fmt.Errorf("falha ao criar a pasta de modelos: %w", err)
	}

	a.emitirProgressoModelo(nome, "Iniciando download…")
	for _, arq := range alvo.Arquivos {
		caminho := filepath.Join(destino, arq.Nome)
		if _, err := os.Stat(caminho); err == nil {
			continue // já baixado
		}
		if err := a.baixarArquivoModelo(arq, destino, caminho, func(msg string) { a.emitirProgressoModelo(nome, msg) }); err != nil {
			a.emitirProgressoModelo(nome, "⚠️ "+err.Error())
			return err
		}
	}
	return nil
}

// baixarArquivoModelo baixa UM arquivo de peso para a pasta do motor ativo. Alguns catálogos (ex.:
// EasyOCR) publicam o peso ZIPADO — a URL termina em .zip mas `arq.Nome` é o arquivo final (.pth):
// nesse caso o sha256 confere o ZIP baixado, que é extraído no destino e descartado.
func (a *App) baixarArquivoModelo(arq ArquivoModelo, destino, caminho string, onProgresso func(string)) error {
	// Guard clause: peso publicado direto (o caso comum, ex.: .onnx e .traineddata).
	if !strings.EqualFold(filepath.Ext(arq.Url), ".zip") || strings.EqualFold(filepath.Ext(arq.Nome), ".zip") {
		return baixador.BaixarArquivo(arq.Url, caminho, arq.Sha256, onProgresso)
	}

	zipLocal := caminho + ".zip"
	if err := baixador.BaixarArquivo(arq.Url, zipLocal, arq.Sha256, onProgresso); err != nil {
		return err
	}
	defer os.Remove(zipLocal)

	onProgresso(fmt.Sprintf("Extraindo %s…", arq.Nome))
	if err := baixador.ExtrairZip(zipLocal, destino); err != nil {
		return fmt.Errorf("falha ao extrair o peso %s: %w", arq.Nome, err)
	}
	if _, err := os.Stat(caminho); err != nil {
		return fmt.Errorf("o zip baixado não continha o arquivo esperado (%s)", arq.Nome)
	}
	return nil
}

// RemoverModelo apaga os arquivos de um modelo do AppData real, preservando arquivos que ainda são
// usados por outro modelo do catálogo (ex.: o detector 'server' compartilhado).
func (a *App) RemoverModelo(nome string) error {
	modelos, err := a.ListarModelos()
	if err != nil {
		return err
	}

	usadosPorOutros := map[string]bool{}
	var alvo *ModeloOcrInfo
	for i := range modelos {
		if modelos[i].Nome == nome {
			alvo = &modelos[i]
			continue
		}
		for _, arq := range modelos[i].Arquivos {
			usadosPorOutros[arq.Nome] = true
		}
	}
	if alvo == nil {
		return fmt.Errorf("modelo '%s' não encontrado no catálogo", nome)
	}

	motorAtivo := a.nomeMotorAtivo()
	if motorAtivo == "" {
		return fmt.Errorf("nenhum motor de OCR ativo para remover o modelo '%s'", nome)
	}

	destino := motoresocr.PastaModelosMotor(motorAtivo)
	for _, arq := range alvo.Arquivos {
		if usadosPorOutros[arq.Nome] {
			continue // compartilhado: preserva
		}
		caminho := filepath.Join(destino, arq.Nome)
		if _, err := os.Stat(caminho); err == nil {
			if err := os.Remove(caminho); err != nil {
				return fmt.Errorf("falha ao remover %s: %w", arq.Nome, err)
			}
		}
	}
	return nil
}

// emitirProgressoModelo envia um evento de progresso de download ao frontend.
func (a *App) emitirProgressoModelo(nome, mensagem string) {
	runtime.EventsEmit(a.ctx, "modelo_download_progresso", map[string]interface{}{"nome": nome, "mensagem": mensagem})
}

// OcrResult represents the JSON structure from Python API
type OcrResult struct {
	Texto     string    `json:"texto"`
	Confianca float64   `json:"confianca"`
	Caixa     []float64 `json:"caixa"`
}

// FlashcardCard representa um cartão processado para o frontend
type FlashcardCard struct {
	Hanzi        string    `json:"hanzi"`
	Pinyin       string    `json:"pinyin"`
	Significados []string  `json:"significados"`
	Confianca    float64   `json:"confianca"`
	Caixa        []float64 `json:"caixa"`
	ImageId      int       `json:"imageId,omitempty"`
	TipoHanzi    string    `json:"tipoHanzi"`
}

// CaptureAndOCR takes a screenshot and sends it to the Python OCR service
func (a *App) GetLastCards() []FlashcardCard {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.lastCards
}

func (a *App) DecomposeCharacter(char string) *dicionario.DecomposicaoHanzi {
	return a.BancoHanzi.Buscar(char)
}

func (a *App) BuscarCaracteresCompostosPor(char string) []string {
	return a.BancoHanzi.BuscarCompostosPor(char)
}

func (a *App) CaractereCompleto(abrev string) string {
	return a.BancoHanzi.CaractereCompleto(abrev)
}

func (a *App) ObterTotalHanzisDicionario() int {
	return a.BancoHanzi.TotalHanzis()
}

func (a *App) BuscarPorPinyin(pinyin string) []string {
	res := a.Cedict.BuscarPorPinyin(pinyin)
	if len(res) > 30 {
		return res[:30]
	}
	return res
}

// BuscarNoDicionarioGeral realiza uma pesquisa global combinando CEDICT e MakeMeAHanzi
func (a *App) BuscarNoDicionarioGeral(termo string) []FlashcardCard {
	var resultados []FlashcardCard
	vistos := make(map[string]bool)

	if strings.HasPrefix(termo, "[DESENHO]") {
		caracteres := strings.TrimPrefix(termo, "[DESENHO]")
		for _, c := range caracteres {
			charStr := string(c)
			if charStr == " " {
				continue
			}
			entradas := a.LookupWord(charStr)
			for _, e := range entradas {
				if !vistos[e.Simplificado] {
					vistos[e.Simplificado] = true
					resultados = append(resultados, FlashcardCard{
						Hanzi:        e.Simplificado,
						Pinyin:       e.Pinyin,
						Significados: e.Significados,
						Confianca:    1.0,
						TipoHanzi:    a.Cedict.AvaliarTipoHanzi(e.Simplificado),
					})
				}
			}
		}
		return resultados
	}

	// Busca primeiro no MakeMeAHanzi (para garantir caracteres únicos de etimologia)
	if a.BancoHanzi != nil {
		entradasMake := a.BancoHanzi.BuscarGeral(termo)
		for _, e := range entradasMake {
			if !vistos[e.Caractere] {
				vistos[e.Caractere] = true
				
				sigs := strings.Split(e.Definicao, ";")
				for i := range sigs {
					sigs[i] = strings.TrimSpace(sigs[i])
				}
				
				resultados = append(resultados, FlashcardCard{
					Hanzi:        e.Caractere,
					Pinyin:       strings.Join(e.Pinyin, ", "),
					Significados: sigs,
					Confianca:    1.0,
					TipoHanzi:    a.Cedict.AvaliarTipoHanzi(e.Caractere),
				})
			}
		}
	}

	// Busca no CEDICT
	if a.Cedict != nil {
		entradas := a.Cedict.BuscarGeral(termo)
		for _, e := range entradas {
			if !vistos[e.Simplificado] {
				vistos[e.Simplificado] = true
				resultados = append(resultados, FlashcardCard{
					Hanzi:        e.Simplificado,
					Pinyin:       e.Pinyin,
					Significados: e.Significados,
					Confianca:    1.0,
					TipoHanzi:    a.Cedict.AvaliarTipoHanzi(e.Simplificado),
				})
			}
		}
	}

	return resultados
}

func (a *App) LookupWord(word string) []dicionario.EntradaDicionario {
	entradas := a.Cedict.Buscar(word)

	if utf8.RuneCountInString(word) == 1 {
		if dec := a.BancoHanzi.Buscar(word); dec != nil && dec.Definicao != "" {
			pinyin := ""
			if len(dec.Pinyin) > 0 {
				pinyin = strings.Join(dec.Pinyin, ", ")
			}

			entry := dicionario.EntradaDicionario{
				Simplificado: word,
				Tradicional:  word,
				Pinyin:       pinyin,
				Significados: []string{dec.Definicao},
			}

			return append([]dicionario.EntradaDicionario{entry}, entradas...)
		}
	}

	return entradas
}

func (a *App) MarcarVistoSilencioso(hanzi string) {
	pinyin, significados := "", []string{}

	if utf8.RuneCountInString(hanzi) == 1 {
		if dec := a.BancoHanzi.Buscar(hanzi); dec != nil && dec.Definicao != "" {
			if len(dec.Pinyin) > 0 {
				pinyin = strings.Join(dec.Pinyin, ", ")
			}
			significados = []string{dec.Definicao}
		}
	}

	if len(significados) == 0 {
		entradas := a.Cedict.Buscar(hanzi)
		if len(entradas) > 0 {
			pinyin = entradas[0].Pinyin
			significados = entradas[0].Significados
		}
	}

	sigStr := ""
	if len(significados) > 0 {
		sigStr = significados[0]
		for i := 1; i < len(significados); i++ {
			sigStr += ", " + significados[i]
		}
	}
	progresso.RegistrarVisto(hanzi, pinyin, sigStr)

	a.registrarHanzisIndividuais(hanzi)
}


// registrarHanzisIndividuais desmembra uma palavra multi-caractere nos seus hanzis individuais
// e registra cada um como 'visto' com pinyin/significado próprios. Não se aplica a componentes
// de decomposição — apenas aos caracteres que formam a palavra no CEDICT.
func (a *App) registrarHanzisIndividuais(palavra string) {
	if utf8.RuneCountInString(palavra) <= 1 {
		return
	}

	for _, r := range palavra {
		if !unicode.Is(unicode.Han, r) {
			continue
		}

		caractere := string(r)
		pinyinChar, significadosChar := "", []string{}

		if dec := a.BancoHanzi.Buscar(caractere); dec != nil && dec.Definicao != "" {
			if len(dec.Pinyin) > 0 {
				pinyinChar = strings.Join(dec.Pinyin, ", ")
			}
			significadosChar = []string{dec.Definicao}
		}

		if len(significadosChar) == 0 {
			entradas := a.Cedict.Buscar(caractere)
			if len(entradas) > 0 {
				pinyinChar = entradas[0].Pinyin
				significadosChar = entradas[0].Significados
			}
		}

		sigStr := ""
		if len(significadosChar) > 0 {
			sigStr = significadosChar[0]
			for i := 1; i < len(significadosChar); i++ {
				sigStr += ", " + significadosChar[i]
			}
		}

		progresso.RegistrarVisto(caractere, pinyinChar, sigStr)
	}
}

// codificadorPng é o encoder compartilhado das capturas e dos crops. BestSpeed porque os PNGs ou
// vão ao sidecar por localhost ou viram crops minúsculos de card — em ambos os casos o tamanho
// extra é irrelevante e a compressão padrão gastava CPU visível a cada scan.
var codificadorPng = png.Encoder{CompressionLevel: png.BestSpeed}
var clienteHttpOcr = &http.Client{}

// censurarRetangulo pinta de preto sólido a interseção entre `r` (coordenadas ABSOLUTAS de tela) e a
// imagem `img`, cujo pixel (0,0) corresponde a (origemX, origemY) na tela — o canto superior esquerdo
// do monitor alvo (screenshot.GetDisplayBounds(alvo).Min). Retângulos parcial ou totalmente fora da
// imagem são recortados/ignorados automaticamente pelo Intersect — não precisa de nenhum tratamento
// especial para "janela em outro monitor" ou "pop-up fora da área capturada".
func censurarRetangulo(img *image.RGBA, origemX, origemY int, r image.Rectangle) {
	local := r.Sub(image.Pt(origemX, origemY)).Intersect(img.Bounds())
	if local.Empty() {
		return
	}
	draw.Draw(img, local, image.Black, image.Point{}, draw.Src)
}

// retanguloAppNaTela devolve o retângulo (coordenadas ABSOLUTAS de tela) da janela principal do app, ou
// ok=false se ela estiver minimizada (nesse caso não há nada visível a censurar).
func (a *App) retanguloAppNaTela() (image.Rectangle, bool) {
	if runtime.WindowIsMinimised(a.ctx) {
		return image.Rectangle{}, false
	}
	x, y := runtime.WindowGetPosition(a.ctx)
	w, h := runtime.WindowGetSize(a.ctx)
	return image.Rect(x, y, x+w, y+h), true
}

// censurarAreasSensiveis apaga (preenche de preto) a área da janela principal do app e as áreas dos
// pop-ups do overlay (hover, destaques e "mostrar tudo") dentro de `img`, quando `CensurarJanelasDoApp`
// estiver ligado em Config. `img` é a captura do monitor alvo; origemX/origemY é o canto superior
// esquerdo desse monitor na tela (screenshot.GetDisplayBounds(alvo).Min).
func (a *App) censurarAreasSensiveis(img *image.RGBA, origemX, origemY int) {
	if !a.Config.CensurarJanelasDoApp {
		return
	}

	if r, ok := a.retanguloAppNaTela(); ok {
		censurarRetangulo(img, origemX, origemY, r)
	}

	for _, r := range overlay.RetangulosVisiveis() {
		censurarRetangulo(img, origemX, origemY, image.Rect(r.X0, r.Y0, r.X1, r.Y1))
	}
}

func (a *App) CaptureAndOCR() ([]FlashcardCard, error) {
	// Capture the target display
	alvo := a.Config.MonitorAlvo
	if alvo < 0 || alvo >= screenshot.NumActiveDisplays() {
		alvo = 0
	}
	bounds := screenshot.GetDisplayBounds(alvo)

	var img *image.RGBA
	var err error

	overlay.OcultarHighlightsTemporariamente(func() {
		img, err = screenshot.CaptureRect(bounds)
		if err == nil {
			// Censura a área da janela do app e dos pop-ups do overlay ANTES de codificar/enviar ao OCR —
			// precisa vir antes do fingerprint (hash) logo abaixo, senão o hash não refletiria a censura.
			a.censurarAreasSensiveis(img, bounds.Min.X, bounds.Min.Y)
		}
	})

	if err != nil {
		return nil, fmt.Errorf("failed to capture screen: %w", err)
	}

	// Fingerprint nos PIXELS CRUS (FNV-1a), ANTES de codificar: no auto-scan o caso comum é a tela
	// não ter mudado, e hashear os bytes do RGBA é muito mais barato que pagar a compressão PNG do
	// frame inteiro só para descobrir isso. A censura já foi aplicada acima, então o hash a reflete.
	hasher := fnv.New64a()
	_, _ = hasher.Write(img.Pix)
	hash := fmt.Sprintf("%x", hasher.Sum64())
	if a.lastImageHash == hash {
		a.mu.RLock()
		cards := a.lastCards
		a.mu.RUnlock()
		return cards, nil
	}
	a.lastImageHash = hash

	// Encode to PNG bytes. BestSpeed: a imagem vai ao sidecar por localhost, então o payload maior
	// não custa nada — e a compressão padrão gastava dezenas de ms de CPU por scan.
	var buf bytes.Buffer
	err = codificadorPng.Encode(&buf, img)
	if err != nil {
		return nil, fmt.Errorf("failed to encode PNG: %w", err)
	}
	imagemPng := append([]byte(nil), buf.Bytes()...)

	// A tela mudou: o overlay de "pop-up de tudo" ficou obsoleto, então o ocultamos.
	if a.popupsTodosVisivel {
		a.ocultarTodosPopups()
		a.popupsTodosVisivel = false
	}

	// Send to Python API
	req, err := http.NewRequest("POST", motoresocr.EnderecoBase()+"/api/ocr", &buf)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/octet-stream")

	// Inject Config from App
	req.Header.Set("X-Ocr-Model", a.Config.ModeloOcr)
	req.Header.Set("X-Ocr-Device", a.Config.DispositivoOcr)
	req.Header.Set("X-Ocr-Hardware", a.Config.HardwareSelecionado)
	req.Header.Set("X-Ocr-Threads", fmt.Sprintf("%d", a.Config.ThreadsCpuOcr))

	maxSidePct := a.Config.EscalaResolucaoOcr
	if maxSidePct <= 0 || maxSidePct > 100 {
		maxSidePct = 100
	}
	realMaxSide := 0
	if maxSidePct < 100 {
		w := bounds.Dx()
		h := bounds.Dy()
		maior := w
		if h > w {
			maior = h
		}
		realMaxSide = int(float64(maior) * (float64(maxSidePct) / 100.0))
	}
	req.Header.Set("X-Ocr-Max-Side", fmt.Sprintf("%d", realMaxSide))

	resp, err := clienteHttpOcr.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to call Python API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error (%d): %s", resp.StatusCode, string(body))
	}

	var results []OcrResult
	if err := json.NewDecoder(resp.Body).Decode(&results); err != nil {
		return nil, fmt.Errorf("failed to decode JSON response: %w", err)
	}

	// Process strings using Jieba and CEDICT
	var cards []FlashcardCard
	var linhas []LinhaTraduzida

	// Crops dos cards deste scan, gravados em disco de uma vez no fim do loop (uma transação por
	// scan, não um fsync por palavra — ver progresso.SalvarImagensSessaoLote).
	var cropsPendentes []string
	var indicesCardsComCrop []int

	// Pré-calcula se a tradução deve ser tentada neste scan (evita repetir a verificação no loop).
	// A tradução em si ocorre apenas em mostrarTodosPopups().

	for _, res := range results {
		// Pular palavras com confiança abaixo da configurada
		if res.Confianca < a.Config.ConfiancaMinimaOcr {
			continue
		}

		// ----- Tradução da LINHA inteira (antes da segmentação em palavras) -----
		linhas = append(linhas, LinhaTraduzida{
			Texto:    res.Texto,
			Traducao: "", // Preenchido sob demanda em mostrarTodosPopups()
			Caixa:    res.Caixa,
		})

		palavras := segmentacao.SegmentarTextoChines(res.Texto)

		var refinedPalavras []string
		for _, p := range palavras {
			// Verifica se a palavra tem significado como um todo
			hasMeaning := false
			if utf8.RuneCountInString(p) == 1 {
				hasMeaning = true // Sempre aceitamos caracteres únicos
			} else {
				entradas := a.Cedict.Buscar(p)
				if len(entradas) > 0 {
					hasMeaning = true
				}
			}

			if hasMeaning {
				refinedPalavras = append(refinedPalavras, p)
			} else {
				// FMM split para preservar compostos
				refinedPalavras = append(refinedPalavras, a.breakIntoDictionaryWords(p)...)
			}
		}

		// O OCR devolve a caixa da LINHA inteira; como cada palavra dela vira um card,
		// repartimos a largura da linha proporcionalmente à quantidade de caracteres de
		// cada palavra. Assim cada card recebe uma sub-caixa horizontal aproximada e o
		// hover consegue identificar qual card está realmente sob o mouse.
		totalRunes := 0
		for _, p := range refinedPalavras {
			totalRunes += utf8.RuneCountInString(p)
		}
		offsetRunes := 0

		for _, p := range refinedPalavras {
			// Busca no dicionário
			pinyin, significados := "", []string{}

			if utf8.RuneCountInString(p) == 1 {
				if dec := a.BancoHanzi.Buscar(p); dec != nil && dec.Definicao != "" {
					if len(dec.Pinyin) > 0 {
						pinyin = strings.Join(dec.Pinyin, ", ")
					}
					significados = []string{dec.Definicao}
				}
			}

			if len(significados) == 0 {
				entradas := a.Cedict.Buscar(p)
				if len(entradas) > 0 {
					pinyin = entradas[0].Pinyin
					significados = entradas[0].Significados
					
					// Conversão de acordo com a configuração de Tipo de Hanzi Gerado
					tipoGeradoOcr := a.Config.TipoHanziGerado
					if a.Config.TipoHanziExibicao != "" && a.Config.TipoHanziExibicao != "ambos" {
						tipoGeradoOcr = a.Config.TipoHanziExibicao
					}
					
					if tipoGeradoOcr == "simplificado" && entradas[0].Simplificado != "" {
						p = entradas[0].Simplificado
					} else if tipoGeradoOcr == "tradicional" && entradas[0].Tradicional != "" {
						p = entradas[0].Tradicional
					}
				}
			}

			// Sub-caixa aproximada deste card dentro da linha (repartição proporcional por caracteres).
			// Detecta a orientação da linha (horizontal vs vertical) pelo aspect ratio do bounding box
			// e distribui proporcionalmente ao longo do eixo correto.
			caixaCard := res.Caixa
			nRunes := utf8.RuneCountInString(p)
			if len(res.Caixa) == 4 && totalRunes > 0 {
				x0 := res.Caixa[0]
				y0 := res.Caixa[1]
				largura := res.Caixa[2] - res.Caixa[0]
				altura := res.Caixa[3] - res.Caixa[1]

				fracInicio := float64(offsetRunes) / float64(totalRunes)
				fracFim := float64(offsetRunes+nRunes) / float64(totalRunes)

				if altura > largura {
					// Texto vertical: distribui ao longo de Y, mantém X inteiro
					inicioY := y0 + altura*fracInicio
					fimY := y0 + altura*fracFim
					caixaCard = []float64{x0, inicioY, res.Caixa[2], fimY}
				} else {
					// Texto horizontal (ou quadrado): distribui ao longo de X, mantém Y inteiro
					inicioX := x0 + largura*fracInicio
					fimX := x0 + largura*fracFim
					caixaCard = []float64{inicioX, y0, fimX, res.Caixa[3]}
				}
			}
			offsetRunes += nRunes

			// Ignorar cartões de caracteres individuais que não têm nenhum significado (como lixo de OCR)
			// ou que são componentes visuais/radicais avulsos (como 氵, 冫, 亻, etc).
			_, ehAbrev := dicionario.MapaAbrevParaCompleto[p]
			if utf8.RuneCountInString(p) == 1 && (len(significados) == 0 || ehAbrev) {
				continue
			}

			var base64Img string
			if len(caixaCard) == 4 {
				// Expand um pouquinho o crop pra dar respiro visual (padding de 10px)
				rect := image.Rect(int(caixaCard[0])-10, int(caixaCard[1])-10, int(caixaCard[2])+10, int(caixaCard[3])+10)
				// Certifica que não ultrapassa os limites da imagem
				rect = rect.Intersect(img.Bounds())

				cropped := img.SubImage(rect)
				var bufImg bytes.Buffer
				if err := codificadorPng.Encode(&bufImg, cropped); err == nil {
					base64Img = base64.StdEncoding.EncodeToString(bufImg.Bytes())
				}
			}

			cards = append(cards, FlashcardCard{
				Hanzi:        p,
				Pinyin:       pinyin,
				Significados: significados,
				Confianca:    res.Confianca,
				Caixa:        caixaCard,
				TipoHanzi:    a.Cedict.AvaliarTipoHanzi(p),
			})
			if base64Img != "" {
				cropsPendentes = append(cropsPendentes, base64Img)
				indicesCardsComCrop = append(indicesCardsComCrop, len(cards)-1)
			}

			// Salvar no histórico de "Já Vistas"
			sigStr := ""
			if len(significados) > 0 {
				sigStr = significados[0]
				for i := 1; i < len(significados); i++ {
					sigStr += ", " + significados[i]
				}
			}
			progresso.RegistrarVisto(p, pinyin, sigStr)
			a.registrarHanzisIndividuais(p)
		}
	}

	// Grava os crops do scan em disco de uma vez e preenche os ids nos cards correspondentes.
	if ids, err := progresso.SalvarImagensSessaoLote(cropsPendentes); err == nil {
		for i, indice := range indicesCardsComCrop {
			cards[indice].ImageId = ids[i]
		}
	}

	a.mu.Lock()
	a.lastCards = cards
	a.lastLinhas = linhas
	a.lastImagemPng = imagemPng
	a.mu.Unlock()
	return cards, nil
}

// GetSessionImage returns the base64 image from SQLite
func (a *App) GetSessionImage(id int) string {
	base64, err := progresso.GetImagemSessao(id)
	if err != nil {
		return ""
	}
	return base64
}
