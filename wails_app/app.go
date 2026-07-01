package main

import (
	"bytes"
	"context"
	"crypto/md5"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"
	"unicode/utf8"

	"github.com/kbinani/screenshot"
	"github.com/wailsapp/wails/v2/pkg/runtime"
	"wails_app/config"
	"wails_app/dicionario"
	"wails_app/progresso"
	"wails_app/segmentacao"
)

// enderecoBasePython devolve a base URL do microserviço Python de OCR. A porta é definida
// dinamicamente pelo orquestrador (main.go) e repassada via HANZITRACKER_OCR_PORT; o 8080 é apenas
// um fallback para quando o app é executado avulso, fora do orquestrador.
func enderecoBasePython() string {
	porta := os.Getenv("HANZITRACKER_OCR_PORT")
	if porta == "" {
		porta = "8080"
	}
	return "http://localhost:" + porta
}

// VersaoContratoOcr é a versão do contrato da API de OCR que este app entende (ver docs/CONTRATO-OCR.md).
// O healthcheck recusa um sidecar cujo `versaoContrato` seja maior (contrato mais novo do que o app sabe
// falar), evitando engatar um motor incompatível.
const VersaoContratoOcr = 1

// RespostaHealth espelha o JSON de GET /api/health do backend de OCR.
type RespostaHealth struct {
	Status         string `json:"status"`
	Servico        string `json:"servico"`
	Motor          string `json:"motor"`
	VersaoContrato int    `json:"versaoContrato"`
}

// aguardarBackendOcr aguarda o backend de OCR responder GET /api/health com status "ok" e um contrato
// compatível, tentando repetidamente até `timeout`. Devolve nil quando o motor está pronto, ou um erro
// descritivo (timeout ou contrato incompatível). É a base para o app só marcar o motor como pronto
// depois que o sidecar realmente subiu (Fase 5, Passo 1).
func aguardarBackendOcr(timeout time.Duration) error {
	cliente := &http.Client{Timeout: 2 * time.Second}
	prazo := time.Now().Add(timeout)
	var ultimoErro error

	for time.Now().Before(prazo) {
		resp, err := cliente.Get(enderecoBasePython() + "/api/health")
		if err != nil {
			ultimoErro = err // ainda subindo: aguarda e re-tenta
			time.Sleep(300 * time.Millisecond)
			continue
		}

		var saude RespostaHealth
		errDecode := json.NewDecoder(resp.Body).Decode(&saude)
		resp.Body.Close()
		if errDecode != nil {
			ultimoErro = errDecode
			time.Sleep(300 * time.Millisecond)
			continue
		}

		// Guard clause: contrato mais novo do que o app sabe falar — não engata (evita motor incompatível).
		if saude.VersaoContrato > VersaoContratoOcr {
			return fmt.Errorf("motor de OCR fala o contrato v%d, mas o app só entende até v%d — atualize o app", saude.VersaoContrato, VersaoContratoOcr)
		}

		if saude.Status == "ok" {
			return nil
		}
		ultimoErro = fmt.Errorf("motor respondeu status %q", saude.Status)
		time.Sleep(300 * time.Millisecond)
	}

	if ultimoErro == nil {
		ultimoErro = fmt.Errorf("sem resposta")
	}
	return fmt.Errorf("backend de OCR não ficou pronto em %s: %w", timeout, ultimoErro)
}

// App struct
type App struct {
	ctx           context.Context
	Config        config.Config
	Cedict        *dicionario.Cedict
	BancoHanzi    *dicionario.BancoMakeMeAHanzi
	lastImageHash string
	lastCards     []FlashcardCard

	popupCmd           *exec.Cmd
	popupStdin         io.WriteCloser
	popupsTodosVisivel bool

	// motorOcr é dono do ciclo de vida do processo de OCR (subir/derrubar/trocar). A posse migrou do
	// orquestrador (main.go) para o app para permitir trocar de motor em runtime (Fase 5, Passo 1).
	motorOcr *GerenciadorMotorOcr
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
		Config:     cfg,
		Cedict:     cedict,
		BancoHanzi: bancoHanzi,
	}
}

// AddVocab
func (a *App) AddVocab(hanzi, pinyin, significado, status string) error {
	return progresso.AddOuUpdateVocab(hanzi, pinyin, significado, status)
}

// GetVocab
func (a *App) GetVocab() ([]progresso.Vocab, error) {
	return progresso.GetAllVocab()
}

// GetConfig returns the current configuration
func (a *App) GetConfig() (config.Config, error) {
	return a.Config, nil
}

// ShowHoverPopup sends a message to popup.py to show the window
// NOTA: x, y aqui já são coordenadas globais do desktop virtual (vindas de mouse.GetCursorPos),
// portanto NÃO devemos somar o offset do monitor.
func (a *App) ShowHoverPopup(pinyin, hanzi, sig string, x, y int) {
	if a.popupStdin != nil {
		data := map[string]interface{}{
			"action":       "show",
			"pinyin":       pinyin,
			"hanzi":        hanzi,
			"significados": sig,
			"x":            x,
			"y":            y,
		}
		b, _ := json.Marshal(data)
		a.popupStdin.Write(append(b, '\n'))
	}
}

// HideHoverPopup hides the floating popup window
func (a *App) HideHoverPopup() {
	if a.popupStdin != nil {
		data := map[string]interface{}{
			"action": "hide",
		}
		b, _ := json.Marshal(data)
		a.popupStdin.Write(append(b, '\n'))
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

// mostrarTodosPopups envia todos os cards do último scan ao popup.py para exibição simultânea.
// O popup.py posiciona cada um sobre sua caixa e resolve sobreposições (empilhando e/ou
// reduzindo o tamanho conforme necessário).
func (a *App) mostrarTodosPopups() {
	if a.popupStdin == nil {
		return
	}

	monitorID := a.Config.MonitorAlvo
	if monitorID >= screenshot.NumActiveDisplays() {
		monitorID = 0
	}
	bounds := screenshot.GetDisplayBounds(monitorID)
	offX := float64(bounds.Min.X)
	offY := float64(bounds.Min.Y)

	itens := make([]map[string]interface{}, 0, len(a.lastCards))
	for _, c := range a.lastCards {
		if len(c.Caixa) != 4 {
			continue
		}
		itens = append(itens, map[string]interface{}{
			"pinyin":       c.Pinyin,
			"hanzi":        c.Hanzi,
			"significados": strings.Join(c.Significados, ", "),
			"x0":           c.Caixa[0] + offX,
			"y0":           c.Caixa[1] + offY,
			"x1":           c.Caixa[2] + offX,
			"y1":           c.Caixa[3] + offY,
		})
	}

	b, _ := json.Marshal(map[string]interface{}{"action": "show_all", "itens": itens})
	a.popupStdin.Write(append(b, '\n'))
}

// ocultarTodosPopups remove todos os pop-ups exibidos pelo "mostrar pop-up de tudo".
func (a *App) ocultarTodosPopups() {
	if a.popupStdin == nil {
		return
	}
	b, _ := json.Marshal(map[string]interface{}{"action": "hide_all"})
	a.popupStdin.Write(append(b, '\n'))
}

// ShowHighlight sends a message to popup.py to draw a bounding box
func (a *App) ShowHighlight(x0, y0, x1, y1 int) {
	if a.popupStdin != nil {
		monitorID := a.Config.MonitorAlvo
		if monitorID >= screenshot.NumActiveDisplays() {
			monitorID = 0
		}
		bounds := screenshot.GetDisplayBounds(monitorID)

		data := map[string]interface{}{
			"action": "highlight",
			"x0":     x0 + bounds.Min.X,
			"y0":     y0 + bounds.Min.Y,
			"x1":     x1 + bounds.Min.X,
			"y1":     y1 + bounds.Min.Y,
		}
		b, _ := json.Marshal(data)
		a.popupStdin.Write(append(b, '\n'))
	}
}

// ShowEstudoHighlights sends a message to popup.py to draw multiple study highlights
func (a *App) ShowEstudoHighlights(boxes [][]float64) {
	if a.popupStdin != nil {
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

		data := map[string]interface{}{
			"action": "estudo_highlights",
			"boxes":  adjustedBoxes,
		}
		b, _ := json.Marshal(data)
		a.popupStdin.Write(append(b, '\n'))
	}
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
	if a.popupCmd != nil && a.popupCmd.Process != nil {
		a.popupCmd.Process.Kill()
	}
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

	// O app é dono do processo de OCR (subir/derrubar/trocar). Todo motor é um EXECUTÁVEL — baixado no
	// AppData ou empacotado num bundle ao lado do app; NÃO há mais fallback para `python server.py`
	// (tudo é modular/baixável). resolverMotorInicial escolhe o motor preferido/padrão instalado ou o
	// bundle; se NADA existe (first-run), o bootstrap baixa+instala+ativa o RapidOCR padrão sozinho.
	a.motorOcr = NovoGerenciadorMotorOcr()
	if desc, ok := resolverMotorInicial(a.Config.MotorOcrAtivo); ok {
		if err := a.motorOcr.Iniciar(desc); err != nil {
			fmt.Printf("Aviso: falha ao subir o backend de OCR: %v\n", err)
		}

		// Espera o motor responder o healthcheck antes de anunciá-lo pronto. Roda em segundo plano
		// para não travar a UI; o frontend ouve "ocr_pronto"/"ocr_indisponivel".
		go func() {
			if err := aguardarBackendOcr(30 * time.Second); err != nil {
				fmt.Printf("Aviso: motor de OCR indisponível: %v\n", err)
				runtime.EventsEmit(a.ctx, "ocr_indisponivel", err.Error())
				return
			}
			fmt.Println("Motor de OCR pronto (healthcheck ok).")
			runtime.EventsEmit(a.ctx, "ocr_pronto")
		}()

		// Garante o overlay (pode faltar se o motor foi baixado avulso, sem ele) — baixa se preciso e sobe.
		go a.garantirEIniciarOverlay()
	} else {
		// First-run: nenhum motor instalado nem em bundle. Baixa o motor padrão (+ overlay) e ativa — tudo
		// em segundo plano; a UI acompanha por "motor_bootstrap_inicio"/"motor_download_progresso"/"ocr_pronto".
		fmt.Println("Nenhum motor de OCR encontrado — iniciando o bootstrap do motor padrão…")
		go a.bootstrapMotorPadrao()
	}
}

// iniciarOverlay sobe o processo de overlay (popup) se ainda não estiver rodando e houver um executável
// resolvível. É idempotente: chamado no startup e de novo após o bootstrap (quando o overlay só passa a
// existir depois de baixado). O Go dirige o overlay por stdin; a janela fica oculta (HideWindow).
func (a *App) iniciarOverlay() {
	if a.popupCmd != nil {
		return // já rodando
	}
	cmd, ok := resolverComandoPopup()
	if !ok {
		return // sem overlay disponível (ex.: first-run antes do bootstrap concluir)
	}
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	if err := cmd.Start(); err != nil {
		return
	}
	a.popupCmd = cmd
	a.popupStdin = stdin
	fmt.Println("Overlay (popup) iniciado.")
}

// garantirEIniciarOverlay baixa o overlay compartilhado se ainda não estiver instalado e o sobe. Roda
// em segundo plano (faz I/O de rede); iniciarOverlay é idempotente (não sobe o overlay duas vezes).
func (a *App) garantirEIniciarOverlay() {
	if err := a.garantirOverlay(); err != nil {
		fmt.Printf("Aviso: overlay indisponível: %v\n", err)
		return
	}
	a.iniciarOverlay()
}

// resolverComandoPopup decide como subir o overlay (popup), devolvendo ok=false quando NENHUM
// executável é encontrado (ex.: first-run antes de o bootstrap baixar o overlay). Ordem: (1) overlay
// baixado no AppData (motores\_overlay\popup.exe); (2) overlay em bundle ao lado do app. NÃO há fallback
// para `python popup.py`: o overlay é sempre um executável (baixado ou em bundle). Ver motores.go.
func resolverComandoPopup() (*exec.Cmd, bool) {
	// 1) Overlay baixado sob demanda no AppData (Fase 5).
	if exe := caminhoExecutavelOverlay(); exe != "" {
		if info, err := os.Stat(exe); err == nil && !info.IsDir() {
			return exec.Command(exe), true
		}
	}

	// 2) Overlay congelado ao lado do app (instalação com bundle / offline).
	candidatosExe := []string{
		filepath.Join("popup", "popup.exe"),
		filepath.Join("dist", "popup", "popup.exe"),
		"popup.exe",
	}
	for _, caminho := range candidatosExe {
		if info, err := os.Stat(caminho); err == nil && !info.IsDir() {
			return exec.Command(caminho), true
		}
	}
	return nil, false
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
	resp, err := http.Get(enderecoBasePython() + "/api/modelos")
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

	destino := pastaModelosRapidOcr()
	if err := os.MkdirAll(destino, 0755); err != nil {
		return fmt.Errorf("falha ao criar a pasta de modelos: %w", err)
	}

	a.emitirProgressoModelo(nome, "Iniciando download…")
	for _, arq := range alvo.Arquivos {
		caminho := filepath.Join(destino, arq.Nome)
		if _, err := os.Stat(caminho); err == nil {
			continue // já baixado
		}
		if err := a.baixarArquivo(arq.Url, caminho, arq.Sha256, func(msg string) { a.emitirProgressoModelo(nome, msg) }); err != nil {
			a.emitirProgressoModelo(nome, "⚠️ "+err.Error())
			return err
		}
	}
	return nil
}

// baixarArquivo baixa uma URL para um caminho local de forma atômica (.tmp + rename), reportando o
// progresso via callback `onProgresso` (o chamador escolhe o evento — modelos vs. motores). Se
// `sha256Esperado` estiver preenchido, o hash é calculado durante o streaming e conferido antes de
// renomear; divergência aborta o download e apaga o .tmp (evita peso corrompido/adulterado). Vazio =
// verificação pulada (torna-se OBRIGATÓRIO ao baixar executáveis — ver motores.go).
func (a *App) baixarArquivo(url, caminhoLocal, sha256Esperado string, onProgresso func(string)) error {
	if onProgresso == nil {
		onProgresso = func(string) {}
	}
	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("falha ao baixar %s: %w", filepath.Base(caminhoLocal), err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("download de %s retornou HTTP %d", filepath.Base(caminhoLocal), resp.StatusCode)
	}

	temp := caminhoLocal + ".tmp"
	f, err := os.Create(temp)
	if err != nil {
		return err
	}

	// Calcula o sha256 no mesmo passo da escrita (streaming), sem reler o arquivo do disco.
	hasher := sha256.New()

	total := resp.ContentLength
	nomeArq := filepath.Base(caminhoLocal)
	var baixado int64
	buf := make([]byte, 64*1024)
	ultimoPct := -1
	for {
		n, errRead := resp.Body.Read(buf)
		if n > 0 {
			if _, errWrite := f.Write(buf[:n]); errWrite != nil {
				f.Close()
				os.Remove(temp)
				return errWrite
			}
			hasher.Write(buf[:n])
			baixado += int64(n)
			if total > 0 {
				pct := int(baixado * 100 / total)
				if pct != ultimoPct {
					ultimoPct = pct
					onProgresso(fmt.Sprintf("Baixando %s (%d%%)…", nomeArq, pct))
				}
			}
		}
		if errRead == io.EOF {
			break
		}
		if errRead != nil {
			f.Close()
			os.Remove(temp)
			return errRead
		}
	}

	if err := f.Close(); err != nil {
		os.Remove(temp)
		return err
	}

	// Verificação de integridade: só quando o manifesto declara o hash esperado.
	if sha256Esperado != "" {
		hashObtido := hex.EncodeToString(hasher.Sum(nil))
		if !strings.EqualFold(hashObtido, sha256Esperado) {
			os.Remove(temp)
			return fmt.Errorf("integridade de %s falhou: sha256 esperado %s, obtido %s", nomeArq, sha256Esperado, hashObtido)
		}
		onProgresso(fmt.Sprintf("%s verificado (sha256 ✓)", nomeArq))
	}

	return os.Rename(temp, caminhoLocal)
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

	destino := pastaModelosRapidOcr()
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
}

// CaptureAndOCR takes a screenshot and sends it to the Python OCR service
func (a *App) GetLastCards() []FlashcardCard {
	return a.lastCards
}

func (a *App) DecomposeCharacter(char string) *dicionario.DecomposicaoHanzi {
	return a.BancoHanzi.Buscar(char)
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
}

func (a *App) CaptureAndOCR() ([]FlashcardCard, error) {
	// Capture the target display
	alvo := a.Config.MonitorAlvo
	if alvo < 0 || alvo >= screenshot.NumActiveDisplays() {
		alvo = 0
	}
	bounds := screenshot.GetDisplayBounds(alvo)
	img, err := screenshot.CaptureRect(bounds)
	if err != nil {
		return nil, fmt.Errorf("failed to capture screen: %w", err)
	}

	// Encode to PNG bytes
	var buf bytes.Buffer
	err = png.Encode(&buf, img)
	if err != nil {
		return nil, fmt.Errorf("failed to encode PNG: %w", err)
	}

	// Fingerprint check
	hash := fmt.Sprintf("%x", md5.Sum(buf.Bytes()))
	if a.lastImageHash == hash {
		return a.lastCards, nil
	}
	a.lastImageHash = hash

	// A tela mudou: o overlay de "pop-up de tudo" ficou obsoleto, então o ocultamos.
	if a.popupsTodosVisivel {
		a.ocultarTodosPopups()
		a.popupsTodosVisivel = false
	}

	// Send to Python API
	req, err := http.NewRequest("POST", enderecoBasePython()+"/api/ocr", &buf)
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

	client := &http.Client{}
	resp, err := client.Do(req)
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
	for _, res := range results {
		// Pular palavras com confiança abaixo da configurada
		if res.Confianca < a.Config.ConfiancaMinimaOcr {
			continue
		}

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

			var base64Img string
			if len(caixaCard) == 4 {
				// Expand um pouquinho o crop pra dar respiro visual (padding de 10px)
				rect := image.Rect(int(caixaCard[0])-10, int(caixaCard[1])-10, int(caixaCard[2])+10, int(caixaCard[3])+10)
				// Certifica que não ultrapassa os limites da imagem
				rect = rect.Intersect(img.Bounds())

				cropped := img.SubImage(rect)
				var bufImg bytes.Buffer
				if err := png.Encode(&bufImg, cropped); err == nil {
					base64Img = base64.StdEncoding.EncodeToString(bufImg.Bytes())
				}
			}

			imgId := 0
			if base64Img != "" {
				if id, err := progresso.SalvarImagemSessao(base64Img); err == nil {
					imgId = id
				}
			}

			cards = append(cards, FlashcardCard{
				Hanzi:        p,
				Pinyin:       pinyin,
				Significados: significados,
				Confianca:    res.Confianca,
				Caixa:        caixaCard,
				ImageId:      imgId,
			})

			// Salvar no histórico de "Já Vistas"
			sigStr := ""
			if len(significados) > 0 {
				sigStr = significados[0]
				for i := 1; i < len(significados); i++ {
					sigStr += ", " + significados[i]
				}
			}
			progresso.RegistrarVisto(p, pinyin, sigStr)
		}
	}

	a.lastCards = cards
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
