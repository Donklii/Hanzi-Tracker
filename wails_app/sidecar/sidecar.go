// Package sidecar é o núcleo genérico do ciclo de vida dos microserviços locais (motores de OCR e
// de TTS): resolução de porta dinâmica, healthcheck com verificação de contrato e o gerenciador de
// processo (subir/derrubar/trocar com hot-swap e reversão). Os pacotes motoresocr e motorestts eram
// espelhos um do outro; agora cada um só declara o SEU Servico (rótulo, env de porta, porta
// fallback, versão de contrato) e mantém o próprio catálogo/resolução de caminhos.
package sidecar

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"sync"
	"time"

	"wails_app/processos"
)

// Servico descreve uma família de sidecar (OCR ou TTS) para o núcleo genérico.
type Servico struct {
	Rotulo         string // p/ logs e mensagens de erro (ex.: "OCR", "TTS")
	EnvPorta       string // env compartilhada com o processo filho (ex.: "HANZITRACKER_OCR_PORT")
	PortaFallback  int    // usada quando a env está ausente e não dá para reservar uma porta livre
	VersaoContrato int    // versão máxima do contrato HTTP que o app entende
}

// Descritor descreve COMO subir um backend: o executável a rodar, seus argumentos e o diretório de
// trabalho. Trocar de motor é apontar o descritor para outro executável baixado.
type Descritor struct {
	Nome     string   // rótulo humano p/ logs (ex.: "RapidOCR (instalado)")
	Catalogo string   // NOME de catálogo do motor ("" = desconhecido). Define a subpasta de pesos e é injetado no sidecar via HANZITRACKER_MOTOR.
	Comando  string   // executável: caminho do binário congelado
	Args     []string // argumentos ("" no caminho normal; reservado p/ variações futuras)
	Dir      string   // diretório de trabalho ("" = herda do app)
}

// respostaHealth espelha o JSON de GET /api/health dos backends (mesmo contrato nos dois serviços).
type respostaHealth struct {
	Status         string `json:"status"`
	Servico        string `json:"servico"`
	Motor          string `json:"motor"`
	VersaoContrato int    `json:"versaoContrato"`
}

// EnderecoBase devolve a base URL do microserviço. A porta vem da env do serviço (definida pelo
// orquestrador ou por ResolverPorta); a PortaFallback só vale para execução avulsa.
//
// Usa 127.0.0.1 (loopback IPv4 explícito), NÃO "localhost". Os sidecars Python sobem via
// http.server.HTTPServer, que escuta só em IPv4 (address_family = AF_INET, bind em 0.0.0.0). Em
// muitas máquinas Windows "localhost" resolve para o IPv6 ::1 primeiro, então o Go discava
// [::1]:porta e levava "connection refused" (o Python não escuta em IPv6) — o erro que aparecia na
// versão instalada. 127.0.0.1 casa exatamente com o que o Python escuta e elimina a ambiguidade de
// resolução de nome/família de endereço.
func (s Servico) EnderecoBase() string {
	porta := os.Getenv(s.EnvPorta)
	if porta == "" {
		porta = strconv.Itoa(s.PortaFallback)
	}
	return "http://127.0.0.1:" + porta
}

// AguardarBackend aguarda o backend responder GET /api/health com status "ok" e um contrato
// compatível, tentando repetidamente até `timeout`. Devolve nil quando o motor está pronto, ou um
// erro descritivo (timeout ou contrato incompatível).
func (s Servico) AguardarBackend(timeout time.Duration) error {
	cliente := &http.Client{Timeout: 2 * time.Second}
	prazo := time.Now().Add(timeout)
	var ultimoErro error

	for time.Now().Before(prazo) {
		resp, err := cliente.Get(s.EnderecoBase() + "/api/health")
		if err != nil {
			ultimoErro = err // ainda subindo: aguarda e re-tenta
			time.Sleep(300 * time.Millisecond)
			continue
		}

		var saude respostaHealth
		errDecode := json.NewDecoder(resp.Body).Decode(&saude)
		resp.Body.Close()
		if errDecode != nil {
			ultimoErro = errDecode
			time.Sleep(300 * time.Millisecond)
			continue
		}

		// Guard clause: contrato mais novo do que o app sabe falar — não engata (motor incompatível).
		if saude.VersaoContrato > s.VersaoContrato {
			return fmt.Errorf("motor de %s fala o contrato v%d, mas o app só entende até v%d — atualize o app",
				s.Rotulo, saude.VersaoContrato, s.VersaoContrato)
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
	return fmt.Errorf("backend de %s não ficou pronto em %s: %w", s.Rotulo, timeout, ultimoErro)
}

// ResolverPorta devolve a porta do microserviço. Respeita a porta já reservada na env do serviço
// (o caminho normal); se ausente (app rodando avulso), pede uma porta livre ao SO e a publica no
// ambiente para que EnderecoBase() e o processo filho concordem.
func (s Servico) ResolverPorta() int {
	if p := os.Getenv(s.EnvPorta); p != "" {
		if n, err := strconv.Atoi(p); err == nil {
			return n
		}
	}

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		os.Setenv(s.EnvPorta, strconv.Itoa(s.PortaFallback))
		return s.PortaFallback
	}
	porta := listener.Addr().(*net.TCPAddr).Port
	listener.Close()
	os.Setenv(s.EnvPorta, strconv.Itoa(porta))
	return porta
}

// Gerenciador é o DONO do ciclo de vida do processo de um sidecar: subir, derrubar e trocar
// (hot-swap) o motor em runtime. Mantém apenas UM motor ativo por vez, sempre na mesma porta
// dinâmica. O encerramento derruba a árvore inteira (processos.DerrubarArvore — o motor pode ter
// subprocessos).
type Gerenciador struct {
	servico   Servico
	mutex     sync.Mutex
	processo  *exec.Cmd
	descritor Descritor
	porta     int
}

// NovoGerenciador cria o gerenciador de um serviço, já resolvendo a porta dele (da env ou livre).
func NovoGerenciador(servico Servico) *Gerenciador {
	return &Gerenciador{servico: servico, porta: servico.ResolverPorta()}
}

// Iniciar sobe o backend descrito (derrubando antes um eventual motor já ativo). O processo herda o
// ambiente do app — inclui a env de porta do serviço e HANZITRACKER_DATA_DIR.
func (g *Gerenciador) Iniciar(descritor Descritor) error {
	g.mutex.Lock()
	defer g.mutex.Unlock()
	return g.iniciarSemLock(descritor)
}

// Encerrar derruba o motor ativo e toda a sua árvore (processos.DerrubarArvore). Idempotente.
func (g *Gerenciador) Encerrar() {
	g.mutex.Lock()
	defer g.mutex.Unlock()
	g.encerrarSemLock()
}

// Trocar faz o hot-swap: derruba o motor atual e sobe o novo na MESMA porta, aguardando o
// healthcheck. Se o motor novo não responder, reverte para o anterior (fallback) para o app não
// ficar sem o serviço. Toda a operação é atômica sob o mutex, então um Encerrar() concorrente
// (ex.: fechamento do app durante a troca) espera ela terminar antes de derrubar o motor resultante.
func (g *Gerenciador) Trocar(descritor Descritor, timeout time.Duration) error {
	g.mutex.Lock()
	defer g.mutex.Unlock()

	anterior := g.descritor
	tinhaAnterior := g.processo != nil

	if err := g.iniciarSemLock(descritor); err != nil {
		return err
	}
	if err := g.servico.AguardarBackend(timeout); err == nil {
		return nil
	}

	// Falha do motor novo: reverte ao anterior, se havia um.
	if tinhaAnterior {
		if errRev := g.iniciarSemLock(anterior); errRev == nil {
			return fmt.Errorf("motor de %s %q não respondeu ao healthcheck; revertido para %q",
				g.servico.Rotulo, descritor.Nome, anterior.Nome)
		}
	}
	return fmt.Errorf("motor de %s %q não respondeu ao healthcheck e não foi possível reverter",
		g.servico.Rotulo, descritor.Nome)
}

// CatalogoAtivo devolve o NOME de catálogo do motor em execução ("" se nenhum/desconhecido). É o
// que define a subpasta de pesos do motor e como os chamadores sabem se o motor pedido já está no ar.
func (g *Gerenciador) CatalogoAtivo() string {
	g.mutex.Lock()
	defer g.mutex.Unlock()
	if g.processo == nil {
		return ""
	}
	return g.descritor.Catalogo
}

// ComandoAtivo devolve o caminho do executável do motor em execução ("" se nenhum). Comparar por
// caminho (e não por rótulo) é o jeito robusto de a UI saber QUAL motor do catálogo está ativo e de
// a remoção recusar apagar o motor que está rodando.
func (g *Gerenciador) ComandoAtivo() string {
	g.mutex.Lock()
	defer g.mutex.Unlock()
	if g.processo == nil {
		return ""
	}
	return g.descritor.Comando
}

func (g *Gerenciador) iniciarSemLock(descritor Descritor) error {
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
	processos.PrepararSidecar(cmd)
	cmd.Env = os.Environ() // herda a env de porta do serviço + HANZITRACKER_DATA_DIR
	// Informa ao sidecar o seu nome de catálogo: o Python monta a subpasta de pesos a partir desta
	// env (obterNomeMotor), garantindo que ele LÊ exatamente onde o Go BAIXA, sem duplicar a
	// constante nos dois lados.
	if descritor.Catalogo != "" {
		cmd.Env = append(cmd.Env, "HANZITRACKER_MOTOR="+descritor.Catalogo)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("falha ao iniciar o backend de %s (%s): %w", g.servico.Rotulo, descritor.Nome, err)
	}
	g.processo = cmd
	g.descritor = descritor
	fmt.Printf("Backend de %s iniciado: %s (porta %d)\n", g.servico.Rotulo, descritor.Nome, g.porta)
	return nil
}

func (g *Gerenciador) encerrarSemLock() {
	// Guard clause: nada rodando.
	if g.processo == nil || g.processo.Process == nil {
		return
	}

	pid := g.processo.Process.Pid
	if err := processos.DerrubarArvore(pid); err != nil {
		g.processo.Process.Kill() // fallback: ao menos o processo direto
	}
	g.processo.Wait() // reap: libera os recursos do processo encerrado
	g.processo = nil
}
