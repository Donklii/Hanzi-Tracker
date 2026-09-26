package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/shirou/gopsutil/v3/disk"
	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
	"wails_app/armazenamento"
	"wails_app/config"
	"wails_app/nuvem"
	"wails_app/ocr"
	"wails_app/progresso"
	"wails_app/stt"
	"wails_app/tts"
)

// ----- Seção vinda de: bindings_armazenamento.go -----

// ItemArmazenamento descreve uma categoria de dados em disco usada pelo app.
type ItemArmazenamento struct {
	Chave     string `json:"chave"`
	Rotulo    string `json:"rotulo"`
	Descricao string `json:"descricao"`
	Caminho   string `json:"caminho"`
	Bytes     int64  `json:"bytes"`
	Limpavel  bool   `json:"limpavel"`
	Perigoso  bool   `json:"perigoso"` // limpar apaga dados do usuário (ex.: vocabulário)
}

// StorageInfo resume o uso de disco do app e o espaço livre do volume.
type StorageInfo struct {
	Itens      []ItemArmazenamento `json:"itens"`
	TotalBytes int64               `json:"totalBytes"`
	DiscoLivre int64               `json:"discoLivre"`
	DiscoTotal int64               `json:"discoTotal"`
	PastaDados string              `json:"pastaDados"`
}

// ----- Métodos expostos ao frontend -----

// GetStorageInfo retorna o uso de disco por categoria e o espaço livre no volume.
func (a *App) GetStorageInfo() StorageInfo {
	var itens []ItemArmazenamento

	// Motores de OCR baixados (sidecars + os pesos de cada um, agora dentro de motores_ocr\<Motor>\modelos).
	// Só aparece quando há algum baixado.
	if b := armazenamento.TamanhoCaminho(ocr.PastaMotoresOcr()); b > 0 {
		itens = append(itens, ItemArmazenamento{
			Chave:     "motores_ocr",
			Rotulo:    "Motores de OCR",
			Descricao: "Programas de reconhecimento e seus modelos baixados. Limpar remove os motores inativos (mantém o ativo e o overlay).",
			Caminho:   ocr.PastaMotoresOcr(),
			Bytes:     b,
			Limpavel:  true,
		})
	}

	// Motores de voz (TTS) baixados (sidecars + os pesos de cada um, baixados do Hugging Face para
	// motores_tts\<Motor>\modelos\hf). Só aparece quando há algum baixado.
	if b := armazenamento.TamanhoCaminho(tts.PastaMotoresTts()); b > 0 {
		itens = append(itens, ItemArmazenamento{
			Chave:     "motores_tts",
			Rotulo:    "Motores de Voz",
			Descricao: "Programas de leitura em voz alta e seus modelos baixados. Limpar remove todos (a leitura volta a pedir download).",
			Caminho:   tts.PastaMotoresTts(),
			Bytes:     b,
			Limpavel:  true,
		})
	}

	// Motores de escuta (STT) baixados (sidecars + os pesos de cada um, baixados do Hugging Face
	// para motores_stt\<Motor>\modelos\hf). Só aparece quando há algum baixado.
	if b := armazenamento.TamanhoCaminho(stt.PastaMotoresStt()); b > 0 {
		itens = append(itens, ItemArmazenamento{
			Chave:     "motores_stt",
			Rotulo:    "Motores de Escuta",
			Descricao: "Programas de reconhecimento de fala e seus modelos baixados. Limpar remove todos (a revisão de pronúncia volta a pedir download).",
			Caminho:   stt.PastaMotoresStt(),
			Bytes:     b,
			Limpavel:  true,
		})
	}

	// Categorias de ambiente de desenvolvimento (modelos EasyOCR e cache de instalação) só fazem
	// sentido quando existem: no app distribuído já compilado normalmente nem aparecem, evitando
	// expor tecnicalidades ao usuário final.
	if b := armazenamento.TamanhoCaminho(armazenamento.PastaEasyOcr()); b > 0 {
		itens = append(itens, ItemArmazenamento{
			Chave:     "modelos_easyocr",
			Rotulo:    "Modelos do EasyOCR",
			Descricao: "Pesos do motor EasyOCR, baixados ao usá-lo.",
			Caminho:   armazenamento.PastaEasyOcr(),
			Bytes:     b,
			Limpavel:  true,
		})
	}
	if b := armazenamento.TamanhoCaminho(armazenamento.PastaCachePip()); b > 0 {
		itens = append(itens, ItemArmazenamento{
			Chave:     "cache_pip",
			Rotulo:    "Cache de instalação",
			Descricao: "Arquivos temporários de instalação de componentes. Seguro apagar.",
			Caminho:   armazenamento.PastaCachePip(),
			Bytes:     b,
			Limpavel:  true,
		})
	}

	tamanhoCacheTraducao, _, _ := progresso.TamanhoCacheTraducao()
	if tamanhoCacheTraducao > 0 {
		itens = append(itens, ItemArmazenamento{
			Chave:     "cache_traducao",
			Rotulo:    "Cache de Tradução",
			Descricao: "Textos traduzidos para não gastar cota em repetições.",
			Caminho:   armazenamento.CaminhoBanco() + " (Tabela interna)",
			Bytes:     tamanhoCacheTraducao,
			Limpavel:  true,
		})
	}

	tamanhoCacheTts, _, _ := progresso.TamanhoCacheTts()
	if tamanhoCacheTts > 0 {
		itens = append(itens, ItemArmazenamento{
			Chave:     "cache_tts",
			Rotulo:    "Cache de Áudio (Voz)",
			Descricao: "Falas já sintetizadas, para repetições saírem instantâneas e sem custo de CPU.",
			Caminho:   armazenamento.PastaCacheAudio(),
			Bytes:     tamanhoCacheTts,
			Limpavel:  true,
		})
	}

	itens = append(itens,
		ItemArmazenamento{
			Chave:     "logs",
			Rotulo:    "Logs de erro",
			Descricao: "Arquivos de log gerados quando algo falha.",
			Caminho:   armazenamento.PastaDados(),
			Bytes:     armazenamento.TamanhoLogs(),
			Limpavel:  true,
		},
		ItemArmazenamento{
			Chave:     "banco",
			Rotulo:    "Banco de vocabulário",
			Descricao: "Suas palavras vistas, em estudo e aprendidas. Apagar zera o progresso!",
			Caminho:   armazenamento.CaminhoBanco(),
			Bytes:     armazenamento.TamanhoCaminho(armazenamento.CaminhoBanco()),
			Limpavel:  true,
			Perigoso:  true,
		},
	)

	var total int64
	for _, it := range itens {
		total += it.Bytes
	}

	info := StorageInfo{
		Itens:      itens,
		TotalBytes: total,
		PastaDados: armazenamento.PastaDados(),
	}

	if uso, err := disk.Usage(armazenamento.PastaDados()); err == nil {
		info.DiscoLivre = int64(uso.Free)
		info.DiscoTotal = int64(uso.Total)
	}

	return info
}

// AbrirPastaDados abre a pasta de dados do app no gerenciador de arquivos do SO.
func (a *App) AbrirPastaDados() error {
	pasta := armazenamento.PastaDados()
	if err := os.MkdirAll(pasta, 0755); err != nil {
		return err
	}
	if runtime.GOOS == "windows" {
		return exec.Command("explorer", pasta).Start()
	}
	return exec.Command("xdg-open", pasta).Start()
}

// LimparArmazenamento apaga os dados de uma categoria pela chave.
func (a *App) LimparArmazenamento(chave string) error {
	switch chave {
	case "motores_ocr":
		return a.limparMotores()
	case "motores_tts":
		return a.limparMotoresTts()
	case "motores_stt":
		return a.limparMotoresStt()
	case "modelos_easyocr":
		return os.RemoveAll(armazenamento.PastaEasyOcr())
	case "cache_pip":
		return os.RemoveAll(armazenamento.PastaCachePip())
	case "logs":
		return armazenamento.LimparLogs()
	case "cache_traducao":
		return progresso.LimparCacheTraducao()
	case "cache_tts":
		return progresso.LimparCacheTts()
	case "banco":
		return progresso.LimparVocabulario()
	default:
		return fmt.Errorf("categoria de armazenamento desconhecida: %s", chave)
	}
}

// ExcluirTudo apaga todos os dados baixados/gerados e zera o vocabulário (as preferências em
// configuracoes.json são preservadas). Equivale a um "reset de armazenamento".
func (a *App) ExcluirTudo() error {
	var erros []string

	if err := a.limparMotores(); err != nil {
		erros = append(erros, fmt.Sprintf("motores: %v", err))
	}
	if err := a.limparMotoresTts(); err != nil {
		erros = append(erros, fmt.Sprintf("motores de voz: %v", err))
	}
	if err := a.limparMotoresStt(); err != nil {
		erros = append(erros, fmt.Sprintf("motores de escuta: %v", err))
	}
	if err := os.RemoveAll(armazenamento.PastaEasyOcr()); err != nil {
		erros = append(erros, fmt.Sprintf("modelos EasyOCR: %v", err))
	}
	if err := os.RemoveAll(armazenamento.PastaCachePip()); err != nil {
		erros = append(erros, fmt.Sprintf("cache pip: %v", err))
	}
	if err := armazenamento.LimparLogs(); err != nil {
		erros = append(erros, fmt.Sprintf("logs: %v", err))
	}
	if err := progresso.LimparCacheTraducao(); err != nil {
		erros = append(erros, fmt.Sprintf("cache de tradução: %v", err))
	}
	if err := progresso.LimparCacheTts(); err != nil {
		erros = append(erros, fmt.Sprintf("cache de áudio TTS: %v", err))
	}
	if err := progresso.LimparVocabulario(); err != nil {
		erros = append(erros, fmt.Sprintf("banco: %v", err))
	}
	// As COTAS (tradução/Gemini) são preservadas de propósito: elas contabilizam consumo já feito nas
	// APIs externas neste período — apagar o contador não devolve o consumo, só cegaria o app para o
	// free tier restante.

	if len(erros) > 0 {
		return fmt.Errorf("alguns itens não puderam ser apagados — %s", strings.Join(erros, "; "))
	}
	return nil
}

// ----- Seção vinda de: bindings_hardware.go -----

// SystemHardware são os nomes reais de CPU e GPUs da máquina, para o select de hardware da UI.
// A lista de GPUs é informativa: a aceleração WebGPU usa o adaptador de vídeo padrão do sistema
// (o WebGpuExecutionProvider não expõe device_id), então escolher uma GPU específica só define a
// intenção CPU vs GPU — não qual placa executa.
type SystemHardware struct {
	Cpu  string   `json:"cpu"`
	Gpus []string `json:"gpus"`
}

// GetSystemHardware detecta os nomes de CPU/GPUs nativamente em Go, por SO (PowerShell/CIM no
// Windows; /proc/cpuinfo + lspci no Linux). Fica em arquivo próprio porque o app.go importa o
// runtime do Wails sem alias — aqui "runtime" é o da stdlib (runtime.GOOS, como em armazenamento.go).
func (a *App) GetSystemHardware() SystemHardware {
	var cpu string
	var gpus []string
	if runtime.GOOS == "windows" {
		cpu, gpus = hardwareWindows()
	} else {
		cpu, gpus = hardwareLinux()
	}

	if cpu == "" {
		cpu = "CPU"
	}
	if len(gpus) == 0 {
		gpus = append(gpus, "GPU (Detecção Falhou)")
	}
	return SystemHardware{Cpu: cpu, Gpus: gpus}
}

func hardwareWindows() (string, []string) {
	cpu := ""
	out, err := exec.Command("powershell", "-NoProfile", "-Command", "(Get-ItemProperty -Path 'HKLM:\\HARDWARE\\DESCRIPTION\\System\\CentralProcessor\\0').ProcessorNameString").Output()
	if err == nil {
		cpu = strings.TrimSpace(string(out))
	}

	var linhas []string
	out, err = exec.Command("powershell", "-NoProfile", "-Command", "Get-CimInstance Win32_VideoController | Select-Object -ExpandProperty Name").Output()
	if err == nil {
		linhas = strings.Split(string(out), "\n")
	}
	return cpu, filtrarGpus(linhas)
}

func hardwareLinux() (string, []string) {
	cpu := ""
	if data, err := os.ReadFile("/proc/cpuinfo"); err == nil {
		for _, linha := range strings.Split(string(data), "\n") {
			if strings.HasPrefix(linha, "model name") {
				if _, nome, ok := strings.Cut(linha, ":"); ok {
					cpu = strings.TrimSpace(nome)
				}
				break
			}
		}
	}

	// lspci lista "xx:yy.z VGA compatible controller: <nome> (rev xx)" — também "3D controller"
	// (GPUs dedicadas em notebooks) e "Display controller" (algumas iGPUs).
	var linhas []string
	if out, err := exec.Command("lspci").Output(); err == nil {
		for _, linha := range strings.Split(string(out), "\n") {
			for _, classe := range []string{"VGA compatible controller: ", "3D controller: ", "Display controller: "} {
				if _, nome, ok := strings.Cut(linha, classe); ok {
					if idx := strings.LastIndex(nome, " (rev "); idx > 0 {
						nome = nome[:idx]
					}
					linhas = append(linhas, nome)
					break
				}
			}
		}
	}
	return cpu, filtrarGpus(linhas)
}

// filtrarGpus limpa a lista de adaptadores: descarta vazios e virtuais (a menos que só haja
// virtuais) e remove duplicatas preservando a ordem.
func filtrarGpus(linhas []string) []string {
	var todas []string
	var filtradas []string
	for _, linha := range linhas {
		linha = strings.TrimSpace(linha)
		if linha == "" {
			continue
		}
		todas = append(todas, linha)

		linhaLower := strings.ToLower(linha)
		virtual := false
		for _, excl := range []string{"virtual", "parsec", "mirror", "remote"} {
			if strings.Contains(linhaLower, excl) {
				virtual = true
				break
			}
		}
		if !virtual {
			filtradas = append(filtradas, linha)
		}
	}

	usar := filtradas
	if len(filtradas) == 0 {
		usar = todas
	}

	var gpus []string
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
	return gpus
}

// ----- Seção vinda de: bindings_nuvem.go -----

// ----- Sincronização com o Google Drive (bindings e cola com o app) -----
// A lógica vive em wails_app/nuvem; aqui ficam a injeção das dependências (navegador do Wails,
// snapshot/troca do banco do progresso) e os métodos expostos ao frontend.

// iniciarNuvem cria o gerenciador de nuvem e liga o espelhamento automático de fundo.
func (a *App) iniciarNuvem(ctx context.Context) {
	a.nuvem = nuvem.NovoGerenciador(nuvem.Dependencias{
		AbrirNavegador:          func(url string) { wailsRuntime.BrowserOpenURL(a.ctx, url) },
		CaminhoBanco:            armazenamento.CaminhoBanco,
		ExportarSnapshot:        progresso.ExportarSnapshot,
		SubstituirBanco:         substituirBancoLocal,
		CaminhoConfiguracoes:    caminhoConfiguracoesLocais,
		SubstituirConfiguracoes: a.substituirConfiguracoesLocais,
	})
	go a.nuvem.LoopSincronizacao(ctx, 5*time.Minute)
}

// encerrarNuvem dá à sincronização uma última chance de espelhar mudanças recentes, com prazo
// curto para não segurar o fechamento do app — o que não subir agora sobe na próxima abertura
// (o primeiro tique do LoopSincronizacao reenvia o banco se o mtime dele passou do último envio).
func (a *App) encerrarNuvem() {
	if a.nuvem == nil {
		return
	}

	feito := make(chan struct{})
	go func() {
		a.nuvem.SincronizarSeMudou()
		close(feito)
	}()
	select {
	case <-feito:
	case <-time.After(30 * time.Second):
		fmt.Println("Aviso: a sincronização final com o Drive não terminou a tempo — fica para a próxima abertura.")
	}
}

// substituirBancoLocal troca o arquivo do banco pelo recém-baixado da nuvem: fecha a conexão
// SQLite, move o novo por cima e reabre. Reabre mesmo se a troca falhar, para o app nunca ficar
// sem banco aberto.
func substituirBancoLocal(origem string) error {
	if err := progresso.FecharDB(); err != nil {
		return err
	}
	errTroca := os.Rename(origem, armazenamento.CaminhoBanco())
	if err := progresso.InitDB(); err != nil {
		return fmt.Errorf("falha ao reabrir o banco após a troca: %w", err)
	}
	return errTroca
}

// substituirConfiguracoesLocais põe o configuracoes.json vindo da nuvem no lugar e recarrega as
// preferências em memória — sem isso o app continuaria valendo as antigas até reiniciar e ainda
// as regravaria por cima na primeira alteração.
func (a *App) substituirConfiguracoesLocais(origem string) error {
	destino, err := config.GetConfigPath()
	if err != nil {
		return err
	}
	if err := os.Rename(origem, destino); err != nil {
		return err
	}

	novas, err := config.LoadConfig()
	if err != nil {
		return fmt.Errorf("configurações trocadas, mas falhou ao recarregá-las: %w", err)
	}
	a.Config = novas
	a.AtualizarAtalhosGlobais()
	return nil
}

// caminhoConfiguracoesLocais devolve o caminho do configuracoes.json ("" se a pasta de dados não
// puder ser resolvida — a sincronização trata como arquivo ausente).
func caminhoConfiguracoesLocais() string {
	caminho, err := config.GetConfigPath()
	if err != nil {
		return ""
	}
	return caminho
}

// ----- Métodos expostos ao frontend -----

// GetInfoNuvem retorna o estado da sincronização para a aba Armazenamento (sem tocar na rede).
func (a *App) GetInfoNuvem() nuvem.Info {
	return a.nuvem.Info()
}

// ConectarNuvem roda a autorização do Google (abre o navegador e espera o consentimento) e a
// verificação inicial: nuvem vazia = envia o banco local; backup existente = estado "conflito",
// aguardando ResolverConflitoNuvem.
func (a *App) ConectarNuvem() (nuvem.Info, error) {
	return a.nuvem.Conectar()
}

// ResolverConflitoNuvem aplica a escolha da primeira conexão ("manterLocal" | "usarNuvem").
func (a *App) ResolverConflitoNuvem(escolha string) (nuvem.Info, error) {
	return a.nuvem.ResolverConflito(escolha)
}

// SincronizarNuvem envia o banco para o Drive agora (botão "Sincronizar agora").
func (a *App) SincronizarNuvem() (nuvem.Info, error) {
	return a.nuvem.Sincronizar()
}

// DesconectarNuvem revoga o acesso e esquece a conexão (o backup continua no Drive do usuário).
func (a *App) DesconectarNuvem() (nuvem.Info, error) {
	return a.nuvem.Desconectar()
}

// ReiniciarAplicativo inicia uma nova instância do aplicativo e encerra a atual.
func (a *App) ReiniciarAplicativo() error {
	self, err := os.Executable()
	if err != nil {
		return err
	}

	cmd := exec.Command(self)
	cmd.Args = os.Args

	err = cmd.Start()
	if err != nil {
		return err
	}

	wailsRuntime.Quit(a.ctx)
	return nil
}
