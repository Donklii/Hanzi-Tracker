package main

import (
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strconv"
	"syscall"
)

// encontrarPortaLivre pede ao SO uma porta TCP livre (bind em :0) e a devolve. A porta é repassada
// ao Python (que faz o bind nela) e ao app Wails (que conecta nela) via HANZITRACKER_OCR_PORT.
func encontrarPortaLivre() (int, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer listener.Close()
	return listener.Addr().(*net.TCPAddr).Port, nil
}

// ambienteFilho devolve o ambiente atual acrescido das variáveis que ambos os processos filhos
// (Python e Wails) precisam compartilhar:
//   - HANZITRACKER_OCR_PORT: a porta dinâmica do microserviço de OCR.
//   - HANZITRACKER_DATA_DIR: a pasta de dados REAL (%APPDATA%\HanziTracker). O Python usa isso em vez
//     do seu próprio %APPDATA%, que sob o Python da Microsoft Store é virtualizado para um sandbox.
func ambienteFilho(porta int) []string {
	env := append(os.Environ(), "HANZITRACKER_OCR_PORT="+strconv.Itoa(porta))
	if appData, err := os.UserConfigDir(); err == nil && appData != "" {
		env = append(env, "HANZITRACKER_DATA_DIR="+filepath.Join(appData, "HanziTracker"))
	}
	return env
}

// rodarNpm executa um comando npm dentro de um diretório, repassando a saída ao console.
// No Windows o npm é um .cmd, então precisa ser invocado via "cmd /c".
func rodarNpm(dir string, args ...string) error {
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.Command("cmd", append([]string{"/c", "npm"}, args...)...)
	} else {
		cmd = exec.Command("npm", args...)
	}
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// compilarFrontend instala as dependências (se faltarem) e compila o frontend React.
// Necessário no modo código-fonte porque o app Wails embute frontend/dist via //go:embed.
func compilarFrontend() error {
	dir := filepath.Join("wails_app", "frontend")

	// Guard clause: instala dependências apenas se node_modules não existir
	if _, err := os.Stat(filepath.Join(dir, "node_modules")); os.IsNotExist(err) {
		fmt.Println("Instalando dependências do frontend (npm install)...")
		if err := rodarNpm(dir, "install"); err != nil {
			return fmt.Errorf("npm install falhou: %w", err)
		}
	}

	fmt.Println("Compilando o frontend (npm run build)...")
	if err := rodarNpm(dir, "run", "build"); err != nil {
		return fmt.Errorf("npm run build falhou: %w", err)
	}
	return nil
}

func main() {
	// Modo dev: "go run . dev" sobe o app via `wails dev` (hot reload do frontend + recompila o Go),
	// em vez de compilar o frontend e rodar o build embutido.
	modoDev := len(os.Args) > 1 && os.Args[1] == "dev"

	fmt.Println("Iniciando Hanzi Tracker (Go + Python)...")

	// Escolhe uma porta livre para o OCR e a compartilha com Python e Wails via ambiente.
	porta, err := encontrarPortaLivre()
	if err != nil {
		log.Fatalf("Falha ao encontrar uma porta livre para o OCR: %v", err)
	}
	fmt.Printf("Porta dinâmica do microserviço de OCR: %d\n", porta)
	envFilho := ambienteFilho(porta)

	// 1. O backend de OCR agora é iniciado e gerido pelo PRÓPRIO app Wails (app.go / GerenciadorMotorOcr),
	//    para permitir trocar de motor em runtime (hot-swap) direto pela UI. O orquestrador só reserva a
	//    porta e a pasta de dados (via envFilho, herdadas pelo app) — não sobe mais o processo de OCR aqui.
	//    Como o motor passa a ser descendente do app, o taskkill /T da árvore do Wails também o derruba.

	// 2. Iniciar a interface gráfica (Wails App)
	var wailsCmd *exec.Cmd

	// Ctrl+C/encerramento: derruba explicitamente a árvore de processos (backend de OCR, app e o popup.py neto).
	canalSinais := make(chan os.Signal, 1)
	signal.Notify(canalSinais, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-canalSinais
		fmt.Println("\nSinal de encerramento recebido. Derrubando processos...")
		// Derrubar a árvore do Wails também derruba o motor de OCR e o popup (netos do app).
		encerrarArvore(wailsCmd)
		os.Exit(0)
	}()

	wailsCmd = montarComandoWails(modoDev)
	wailsCmd.Stdout = os.Stdout
	wailsCmd.Stderr = os.Stderr
	// O app Wails (e, no modo dev/fonte, o binário que ele gera) herda a porta do OCR daqui.
	wailsCmd.Env = envFilho
	// Fora do Windows, põe o app num process group próprio para encerrarArvore alcançar os netos.
	prepararComandoFilho(wailsCmd)

	if err := wailsCmd.Start(); err != nil {
		fmt.Printf("Erro ao iniciar o aplicativo Wails: %v\n", err)
		if modoDev {
			fmt.Println("No modo dev é preciso ter o Wails CLI instalado: go install github.com/wailsapp/wails/v2/cmd/wails@latest")
		}
		os.Exit(1)
	}

	if err := wailsCmd.Wait(); err != nil {
		fmt.Printf("Aplicativo Wails encerrou com erro: %v\n", err)
	}

	// A janela foi fechada: garante a derrubada da árvore do Wails (motor de OCR e popup são netos do
	// app e caem junto). O próprio app já encerra o motor no shutdown; isto é a rede de segurança.
	encerrarArvore(wailsCmd)
	fmt.Println("Encerrando Hanzi Tracker...")
}

// encerrarArvore mata um processo e toda a sua árvore de descendentes pelo mecanismo do SO —
// taskkill /T no Windows, kill no process group no Linux (ver processos_windows.go/processos_outros.go).
func encerrarArvore(cmd *exec.Cmd) {
	// Guard clause: nada para encerrar
	if cmd == nil || cmd.Process == nil {
		return
	}

	if err := derrubarArvoreProcessos(cmd.Process.Pid); err != nil {
		// Fallback: mata ao menos o processo direto
		cmd.Process.Kill()
	}
}

// montarComandoWails decide como subir a interface: `wails dev` (modo dev), o executável
// compilado (se existir) ou `go run .` a partir do código-fonte (compilando o frontend antes).
func montarComandoWails(modoDev bool) *exec.Cmd {
	if modoDev {
		fmt.Println("Modo DEV: iniciando 'wails dev' (hot reload do frontend)...")
		args := []string{"dev"}
		if runtime.GOOS == "linux" {
			// Ubuntu 24.04+ só distribui o WebKitGTK 4.1 (o pacote 4.0 foi descontinuado),
			// então é preciso avisar o Wails para linkar contra ele.
			args = append(args, "-tags", "webkit2_41")
		}
		cmd := exec.Command("wails", args...)
		cmd.Dir = "wails_app"
		return cmd
	}

	fmt.Println("Rodando a partir do código fonte em wails_app...")

	// Compila o frontend antes do go run, pois o app embute frontend/dist via //go:embed.
	// Sem isso, mudanças no React não apareceriam (ou o embed falharia se dist não existir).
	if err := compilarFrontend(); err != nil {
		fmt.Printf("Falha ao compilar o frontend: %v\n", err)
		os.Exit(1)
	}

	// As tags "desktop,production" são exigidas pelo Wails: sem elas o build é recusado
	// ("Wails applications will not build without the correct build tags"). "production" faz o app
	// usar os assets embutidos (frontend/dist) em vez de tentar um servidor de dev (Vite).
	tags := "desktop,production"
	if runtime.GOOS == "linux" {
		// Ubuntu 24.04+ só distribui o WebKitGTK 4.1 (o pacote 4.0 foi descontinuado),
		// então é preciso avisar o Wails para linkar contra ele.
		tags += ",webkit2_41"
	}
	cmd := exec.Command("go", "run", "-tags", tags, ".")
	cmd.Dir = "wails_app"
	return cmd
}
