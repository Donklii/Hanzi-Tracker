package main

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"sync"
	"syscall"
	"time"
)

// DescritorMotorOcr descreve COMO subir o backend de OCR: o executável a rodar, seus argumentos e o
// diretório de trabalho. Abstrai "qual motor está ativo" para o app trocar de motor apenas apontando o
// descritor para outro executável baixado (ver Fase 5 no TODO.md e docs/CONTRATO-OCR.md).
type DescritorMotorOcr struct {
	Nome     string   // rótulo humano p/ logs (ex.: "RapidOCR (bundle)")
	Catalogo string   // NOME de catálogo do motor (chave em MotoresBaixaveis; "" = desconhecido). Define a subpasta de pesos modelos\<Catalogo> e é injetado no sidecar via HANZITRACKER_MOTOR.
	Comando  string   // executável: caminho do .exe congelado ou "python"
	Args     []string // argumentos (ex.: caminho do server.py no modo fonte)
	Dir      string   // diretório de trabalho ("" = herda do app)
}

// resolverMotorOcrBundle resolve um sidecar congelado empacotado AO LADO do app (instalação com bundle
// opcional / execução offline). NÃO há fallback para `python server.py`: todo motor é um executável,
// baixado no AppData ou em bundle. Devolve ok=false quando não há bundle — sinal de que o motor precisa
// ser baixado (o bootstrap decide isso; resolverMotorInicial também considera os motores já baixados no
// AppData). Os arquivos python_backend/*.py continuam sendo a fonte para congelar os sidecars (não são
// executados pelo app). Ver motores.go e BUILD.md §3.
func resolverMotorOcrBundle() (DescritorMotorOcr, bool) {
	// Sidecar congelado ao lado do app (PyInstaller onedir gera <nome>/<nome>.exe).
	candidatosExe := []string{
		filepath.Join("ocr_server", "ocr_server.exe"),
		filepath.Join("dist", "ocr_server", "ocr_server.exe"),
		"ocr_server.exe",
		filepath.Join("..", "ocr_server", "ocr_server.exe"),
	}
	for _, caminho := range candidatosExe {
		if info, err := os.Stat(caminho); err == nil && !info.IsDir() {
			abs, errAbs := filepath.Abs(caminho)
			if errAbs != nil {
				abs = caminho
			}
			return DescritorMotorOcr{Nome: "RapidOCR (bundle)", Catalogo: "RapidOCR", Comando: abs}, true
		}
	}
	return DescritorMotorOcr{}, false
}

// portaOcr devolve a porta do microserviço de OCR. Respeita a porta reservada pelo orquestrador
// (HANZITRACKER_OCR_PORT, o caminho normal de execução); se ausente (app rodando avulso), pede uma
// porta livre ao SO e a publica no ambiente para que enderecoBasePython() e o processo filho concordem.
func portaOcr() int {
	if p := os.Getenv("HANZITRACKER_OCR_PORT"); p != "" {
		if n, err := strconv.Atoi(p); err == nil {
			return n
		}
	}

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		os.Setenv("HANZITRACKER_OCR_PORT", "8080")
		return 8080
	}
	porta := listener.Addr().(*net.TCPAddr).Port
	listener.Close()
	os.Setenv("HANZITRACKER_OCR_PORT", strconv.Itoa(porta))
	return porta
}

// GerenciadorMotorOcr é o DONO do ciclo de vida do processo de OCR dentro do app: subir, derrubar e
// trocar (hot-swap) o motor em runtime. Mantém apenas UM motor ativo por vez, sempre na mesma porta
// dinâmica, para a troca (Fase 5) não depender do orquestrador. O encerramento derruba a árvore inteira
// via taskkill /T (o motor pode ter subprocessos), como o main.go faz com os demais filhos.
type GerenciadorMotorOcr struct {
	mutex     sync.Mutex
	processo  *exec.Cmd
	descritor DescritorMotorOcr
	porta     int
}

// NovoGerenciadorMotorOcr cria o gerenciador já resolvendo a porta do OCR (do orquestrador ou livre).
func NovoGerenciadorMotorOcr() *GerenciadorMotorOcr {
	return &GerenciadorMotorOcr{porta: portaOcr()}
}

// Iniciar sobe o backend descrito (derrubando antes um eventual motor já ativo). O processo herda o
// ambiente do app — inclui HANZITRACKER_OCR_PORT e HANZITRACKER_DATA_DIR (propagados pelo orquestrador).
func (g *GerenciadorMotorOcr) Iniciar(descritor DescritorMotorOcr) error {
	g.mutex.Lock()
	defer g.mutex.Unlock()
	return g.iniciarSemLock(descritor)
}

func (g *GerenciadorMotorOcr) iniciarSemLock(descritor DescritorMotorOcr) error {
	// Guard clause: um motor por vez — derruba o atual antes de subir o novo.
	if g.processo != nil && g.processo.Process != nil {
		g.encerrarSemLock()
	}

	cmd := exec.Command(descritor.Comando, descritor.Args...)
	if descritor.Dir != "" {
		cmd.Dir = descritor.Dir
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	cmd.Env = os.Environ() // herda HANZITRACKER_OCR_PORT + HANZITRACKER_DATA_DIR
	// Informa ao sidecar o seu nome de catálogo: o Python monta a subpasta de pesos
	// modelos\<Catalogo> a partir desta env (obterNomeMotor), garantindo que ele LÊ exatamente onde o
	// Go BAIXA (pastaModelosMotor) sem duplicar a constante nos dois lados.
	if descritor.Catalogo != "" {
		cmd.Env = append(cmd.Env, "HANZITRACKER_MOTOR="+descritor.Catalogo)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("falha ao iniciar o backend de OCR (%s): %w", descritor.Nome, err)
	}
	g.processo = cmd
	g.descritor = descritor
	fmt.Printf("Backend de OCR iniciado: %s (porta %d)\n", descritor.Nome, g.porta)
	return nil
}

// Encerrar derruba o motor ativo e toda a sua árvore (taskkill /T). Idempotente.
func (g *GerenciadorMotorOcr) Encerrar() {
	g.mutex.Lock()
	defer g.mutex.Unlock()
	g.encerrarSemLock()
}

func (g *GerenciadorMotorOcr) encerrarSemLock() {
	// Guard clause: nada rodando.
	if g.processo == nil || g.processo.Process == nil {
		return
	}

	pid := g.processo.Process.Pid
	kill := exec.Command("taskkill", "/F", "/T", "/PID", strconv.Itoa(pid))
	kill.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	if err := kill.Run(); err != nil {
		g.processo.Process.Kill() // fallback: ao menos o processo direto
	}
	g.processo.Wait() // reap: libera os recursos do processo encerrado
	g.processo = nil
}

// MotorAtivo devolve o nome do motor em execução ("" se nenhum). Usado por logs/UI de motores.
func (g *GerenciadorMotorOcr) MotorAtivo() string {
	g.mutex.Lock()
	defer g.mutex.Unlock()
	if g.processo == nil {
		return ""
	}
	return g.descritor.Nome
}

// CatalogoAtivo devolve o NOME de catálogo do motor em execução ("" se nenhum/desconhecido). É o que
// define a subpasta de pesos do motor (modelos\<Catalogo>) — ver BaixarModelo/RemoverModelo no app.go.
func (g *GerenciadorMotorOcr) CatalogoAtivo() string {
	g.mutex.Lock()
	defer g.mutex.Unlock()
	if g.processo == nil {
		return ""
	}
	return g.descritor.Catalogo
}

// ComandoAtivo devolve o caminho do executável do motor em execução ("" se nenhum). Comparar por
// caminho (e não por rótulo) é o jeito robusto de a UI saber QUAL motor do catálogo está ativo e de
// RemoverMotor recusar apagar o motor que está rodando. Ver motores.go.
func (g *GerenciadorMotorOcr) ComandoAtivo() string {
	g.mutex.Lock()
	defer g.mutex.Unlock()
	if g.processo == nil {
		return ""
	}
	return g.descritor.Comando
}

// Trocar faz o hot-swap: derruba o motor atual e sobe o novo na MESMA porta, aguardando o healthcheck.
// Se o motor novo não responder, reverte para o anterior (fallback) para o app não ficar sem OCR. Toda
// a operação é atômica sob o mutex, então um Encerrar() concorrente (ex.: fechamento do app durante a
// troca) espera ela terminar antes de derrubar o motor resultante. É a primitiva que a UI de "Gerenciar
// Motores" (Fase 5) vai acionar depois de baixar um sidecar.
func (g *GerenciadorMotorOcr) Trocar(descritor DescritorMotorOcr, timeout time.Duration) error {
	g.mutex.Lock()
	defer g.mutex.Unlock()

	anterior := g.descritor
	tinhaAnterior := g.processo != nil

	if err := g.iniciarSemLock(descritor); err != nil {
		return err
	}
	if err := aguardarBackendOcr(timeout); err == nil {
		return nil
	}

	// Falha do motor novo: reverte ao anterior, se havia um.
	if tinhaAnterior {
		if errRev := g.iniciarSemLock(anterior); errRev == nil {
			return fmt.Errorf("motor %q não respondeu ao healthcheck; revertido para %q", descritor.Nome, anterior.Nome)
		}
	}
	return fmt.Errorf("motor %q não respondeu ao healthcheck e não foi possível reverter", descritor.Nome)
}
