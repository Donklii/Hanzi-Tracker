package main

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

// ----- Ciclo de vida do PROCESSO do motor de TTS -----
// Irmão do GerenciadorMotorOcr (motor_ocr.go), com a própria porta e o próprio ciclo de vida: o
// motor de voz COEXISTE com o de OCR (dois processos ao mesmo tempo), então não dá para reaproveitar
// o gerenciador de OCR — que garante UM processo por vez na porta dele. O TTS sobe PREGUIÇOSAMENTE
// (só na primeira leitura em voz alta, ver garantirMotorTts em tts.go), não no startup.

// DescritorMotorTts descreve COMO subir o backend de TTS: o executável a rodar, seus argumentos e o
// diretório de trabalho. Trocar de motor (Kokoro-82M ↔ ChatTTS) é apontar o descritor para outro
// executável baixado.
type DescritorMotorTts struct {
	Nome     string   // rótulo humano p/ logs (ex.: "Kokoro-82M (instalado)")
	Catalogo string   // NOME de catálogo do motor (chave em MotoresTtsBaixaveis). Define a subpasta de pesos modelos\<Catalogo> e é injetado no sidecar via HANZITRACKER_MOTOR.
	Comando  string   // executável: caminho do .exe congelado
	Args     []string // argumentos ("" no caminho normal; reservado p/ variações futuras)
	Dir      string   // diretório de trabalho ("" = herda do app)
}

// resolverMotorTtsBundle resolve um sidecar de TTS congelado empacotado AO LADO do app (instalação
// com bundle opcional / build local via build_sidecars.ps1, antes da release publicada). Devolve
// ok=false quando não há bundle — sinal de que o motor precisa ser baixado. Espelha
// resolverMotorOcrBundle (motor_ocr.go).
func resolverMotorTtsBundle(m MotorTtsBaixavel) (DescritorMotorTts, bool) {
	base := strings.TrimSuffix(m.Executavel, filepath.Ext(m.Executavel))
	candidatosExe := []string{
		filepath.Join(base, m.Executavel),
		filepath.Join("dist", base, m.Executavel),
		m.Executavel,
		filepath.Join("..", base, m.Executavel),
		filepath.Join("..", "python_backend", "dist", base, m.Executavel),
	}
	for _, caminho := range candidatosExe {
		if info, err := os.Stat(caminho); err == nil && !info.IsDir() {
			abs, errAbs := filepath.Abs(caminho)
			if errAbs != nil {
				abs = caminho
			}
			return DescritorMotorTts{Nome: m.Rotulo + " (bundle)", Catalogo: m.Nome, Comando: abs}, true
		}
	}
	return DescritorMotorTts{}, false
}

// portaTts devolve a porta do microserviço de TTS. Como o TTS coexiste com o OCR, tem env própria
// (HANZITRACKER_TTS_PORT): se ausente, pede uma porta livre ao SO e a publica no ambiente para que
// enderecoBaseTts() e o processo filho concordem.
func portaTts() int {
	if p := os.Getenv("HANZITRACKER_TTS_PORT"); p != "" {
		if n, err := strconv.Atoi(p); err == nil {
			return n
		}
	}

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		os.Setenv("HANZITRACKER_TTS_PORT", "8090")
		return 8090
	}
	porta := listener.Addr().(*net.TCPAddr).Port
	listener.Close()
	os.Setenv("HANZITRACKER_TTS_PORT", strconv.Itoa(porta))
	return porta
}

// GerenciadorMotorTts é o DONO do ciclo de vida do processo de TTS dentro do app: subir, derrubar e
// trocar o motor em runtime. Mantém apenas UM motor de voz ativo por vez, sempre na mesma porta
// dinâmica. O encerramento derruba a árvore inteira via taskkill /T, como o gerenciador de OCR.
type GerenciadorMotorTts struct {
	mutex     sync.Mutex
	processo  *exec.Cmd
	descritor DescritorMotorTts
	porta     int
}

// NovoGerenciadorMotorTts cria o gerenciador já resolvendo a porta do TTS (da env ou livre).
func NovoGerenciadorMotorTts() *GerenciadorMotorTts {
	return &GerenciadorMotorTts{porta: portaTts()}
}

// Iniciar sobe o backend descrito (derrubando antes um eventual motor já ativo). O processo herda o
// ambiente do app — inclui HANZITRACKER_TTS_PORT e HANZITRACKER_DATA_DIR.
func (g *GerenciadorMotorTts) Iniciar(descritor DescritorMotorTts) error {
	g.mutex.Lock()
	defer g.mutex.Unlock()
	return g.iniciarSemLock(descritor)
}

func (g *GerenciadorMotorTts) iniciarSemLock(descritor DescritorMotorTts) error {
	// Guard clause: um motor de voz por vez — derruba o atual antes de subir o novo.
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
	cmd.Env = os.Environ() // herda HANZITRACKER_TTS_PORT + HANZITRACKER_DATA_DIR
	// Informa ao sidecar o seu nome de catálogo: o Python monta a subpasta de pesos
	// modelos\<Catalogo>\hf a partir desta env (obterNomeMotor), garantindo que os pesos baixados
	// do Hugging Face moram no AppData do app.
	if descritor.Catalogo != "" {
		cmd.Env = append(cmd.Env, "HANZITRACKER_MOTOR="+descritor.Catalogo)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("falha ao iniciar o backend de TTS (%s): %w", descritor.Nome, err)
	}
	g.processo = cmd
	g.descritor = descritor
	fmt.Printf("Backend de TTS iniciado: %s (porta %d)\n", descritor.Nome, g.porta)
	return nil
}

// Encerrar derruba o motor ativo e toda a sua árvore (taskkill /T). Idempotente.
func (g *GerenciadorMotorTts) Encerrar() {
	g.mutex.Lock()
	defer g.mutex.Unlock()
	g.encerrarSemLock()
}

func (g *GerenciadorMotorTts) encerrarSemLock() {
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

// CatalogoAtivo devolve o NOME de catálogo do motor de voz em execução ("" se nenhum/desconhecido).
// É como garantirMotorTts (tts.go) sabe se o motor pedido já está no ar ou se precisa trocar.
func (g *GerenciadorMotorTts) CatalogoAtivo() string {
	g.mutex.Lock()
	defer g.mutex.Unlock()
	if g.processo == nil {
		return ""
	}
	return g.descritor.Catalogo
}

// ComandoAtivo devolve o caminho do executável do motor de voz em execução ("" se nenhum). Comparar
// por caminho é o jeito robusto de a UI saber QUAL motor do catálogo está ativo e de RemoverMotorTts
// recusar apagar o motor que está rodando.
func (g *GerenciadorMotorTts) ComandoAtivo() string {
	g.mutex.Lock()
	defer g.mutex.Unlock()
	if g.processo == nil {
		return ""
	}
	return g.descritor.Comando
}

// Trocar faz o hot-swap: derruba o motor de voz atual e sobe o novo na MESMA porta, aguardando o
// healthcheck. Se o motor novo não responder, reverte para o anterior (fallback). Toda a operação é
// atômica sob o mutex, então um Encerrar() concorrente (ex.: fechamento do app durante a troca)
// espera ela terminar antes de derrubar o motor resultante.
func (g *GerenciadorMotorTts) Trocar(descritor DescritorMotorTts, timeout time.Duration) error {
	g.mutex.Lock()
	defer g.mutex.Unlock()

	anterior := g.descritor
	tinhaAnterior := g.processo != nil

	if err := g.iniciarSemLock(descritor); err != nil {
		return err
	}
	if err := aguardarBackendTts(timeout); err == nil {
		return nil
	}

	// Falha do motor novo: reverte ao anterior, se havia um.
	if tinhaAnterior {
		if errRev := g.iniciarSemLock(anterior); errRev == nil {
			return fmt.Errorf("motor de TTS %q não respondeu ao healthcheck; revertido para %q", descritor.Nome, anterior.Nome)
		}
	}
	return fmt.Errorf("motor de TTS %q não respondeu ao healthcheck e não foi possível reverter", descritor.Nome)
}
