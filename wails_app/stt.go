package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"wails_app/motoresstt"
)

// ----- Escuta do microfone e transcrição (STT) -----
// Cliente do contrato de STT (docs/CONTRATO-STT.md) + os gatilhos push-to-talk expostos ao
// frontend (revisão de pronúncia). A GRAVAÇÃO acontece no PRÓPRIO sidecar (sounddevice): a webview
// do Wails no Linux não tem SpeechRecognition nem getUserMedia, então o frontend só comanda
// iniciar/parar/cancelar. O motor de STT é um sidecar próprio (Paraformer-ZH, ver pacote
// motoresstt) que COEXISTE com os de OCR e TTS e sobe PREGUIÇOSAMENTE: só quando a revisão de
// pronúncia precisa escutar, nunca no startup.

// Timeout longo de propósito: o primeiro "parar" (ou o "preparar" do pré-aquecimento) pode incluir
// o download dos pesos do Hugging Face (feito pelo sidecar) numa conexão lenta.
var clienteHttpStt = &http.Client{Timeout: 15 * time.Minute}

// emitirEstadoStt envia ao frontend o estado atual da escuta/transcrição (mensagem na tela de
// pronúncia). Mensagem vazia = terminou/limpou.
func (a *App) emitirEstadoStt(mensagem string) {
	if a.ctx == nil {
		return
	}
	runtime.EventsEmit(a.ctx, "stt_estado", mensagem)
}

// garantirMotorStt deixa o motor de STT `nome` no ar e saudável: cria o gerenciador na primeira
// chamada, sobe/troca o sidecar quando o motor pedido difere do ativo e ressuscita um processo que
// morreu. DEVE ser chamado já com a.sttMutex adquirido.
func (a *App) garantirMotorStt(nome string) error {
	if a.motorStt == nil {
		a.motorStt = motoresstt.NovoGerenciadorMotorStt()
	}

	// Motor certo já no ar? Healthcheck curto pega o caso de processo morto (crash) — aí re-sobe.
	if a.motorStt.CatalogoAtivo() == nome {
		if err := motoresstt.AguardarBackend(2 * time.Second); err == nil {
			return nil
		}
	}

	desc, ok := motoresstt.ResolverMotorStt(nome)
	if !ok {
		return fmt.Errorf("o motor de escuta '%s' não está instalado — baixe-o em Configurações → Motores → Reconhecimento de Voz", nome)
	}

	a.emitirEstadoStt("Iniciando o motor de escuta…")
	return a.motorStt.Trocar(desc, 60*time.Second)
}

// DespertarMotorStt pré-aquece o motor de STT em segundo plano: sobe o sidecar e manda carregar o
// modelo (na primeiríssima vez, baixa ~240 MB de pesos do Hugging Face). O frontend chama ao
// entrar numa questão de pronúncia, para o primeiro push-to-talk sair sem essa espera. No-op
// silencioso quando o motor não está instalado — a UI já orienta o download.
func (a *App) DespertarMotorStt() {
	nome := a.Config.MotorSttAtivo
	if nome == "" {
		return
	}
	if _, ok := motoresstt.ResolverMotorStt(nome); !ok {
		return
	}

	// Não bloqueamos a thread principal para não travar a interface do usuário.
	go func() {
		a.sttMutex.Lock()
		defer a.sttMutex.Unlock()

		if err := a.garantirMotorStt(nome); err != nil {
			a.emitirEstadoStt("")
			return
		}
		a.emitirEstadoStt("Preparando o reconhecimento de voz… (a primeira vez baixa o modelo)")
		_, err := a.chamarSidecarStt("/api/stt/preparar")
		if err != nil {
			fmt.Printf("Aviso: falha ao preparar o motor de STT: %v\n", err)
		}
		a.emitirEstadoStt("") // limpa o status
	}()
}

// IniciarEscutaStt começa a captura do microfone no sidecar (push-to-talk: o frontend chama ao
// PRESSIONAR o botão). Sobe o motor se preciso — na primeira escuta da sessão isso inclui o boot
// do sidecar (segundos); o pré-aquecimento (DespertarMotorStt) normalmente já pagou esse custo.
func (a *App) IniciarEscutaStt() error {
	nome := a.Config.MotorSttAtivo
	if _, ok := motoresstt.ObterMotorSttBaixavel(nome); !ok {
		return fmt.Errorf("motor de STT desconhecido: %s", nome)
	}

	a.sttMutex.Lock()
	defer a.sttMutex.Unlock()

	if err := a.garantirMotorStt(nome); err != nil {
		a.emitirEstadoStt("")
		return err
	}

	a.emitirEstadoStt("")
	_, err := a.chamarSidecarStt("/api/stt/iniciar")
	return err
}

// PararEscutaStt para a captura e devolve o texto transcrito (o frontend chama ao SOLTAR o botão).
// A primeira transcrição de cada sessão pode demorar: carga do modelo e — só na primeiríssima
// vez — download dos pesos do Hugging Face (~240 MB). O progresso é anunciado ao frontend via o
// evento "stt_estado".
func (a *App) PararEscutaStt() (string, error) {
	a.sttMutex.Lock()
	defer a.sttMutex.Unlock()

	// Guard clause: nenhum motor no ar = nenhuma escuta em andamento (ex.: iniciar falhou).
	if a.motorStt == nil || a.motorStt.CatalogoAtivo() == "" {
		return "", fmt.Errorf("nenhuma escuta em andamento")
	}

	a.emitirEstadoStt("Transcrevendo fala… (a primeira vez pode baixar o modelo)")
	defer a.emitirEstadoStt("")

	corpo, err := a.chamarSidecarStt("/api/stt/parar")
	if err != nil {
		return "", err
	}

	var resposta struct {
		Texto string `json:"texto"`
	}
	if err := json.Unmarshal(corpo, &resposta); err != nil {
		return "", fmt.Errorf("resposta inválida do motor de STT: %w", err)
	}
	return resposta.Texto, nil
}

// CancelarEscutaStt descarta uma gravação em andamento sem transcrever (soltar o botão fora da
// área, troca de questão, desmontagem da tela). Idempotente: sem motor no ar, é um no-op.
func (a *App) CancelarEscutaStt() error {
	a.sttMutex.Lock()
	defer a.sttMutex.Unlock()

	// Guard clause: nada rodando — nada a cancelar.
	if a.motorStt == nil || a.motorStt.CatalogoAtivo() == "" {
		return nil
	}
	_, err := a.chamarSidecarStt("/api/stt/cancelar")
	return err
}

// chamarSidecarStt faz um POST cru a um endpoint do sidecar de STT JÁ NO AR e devolve o corpo da
// resposta. O chamador DEVE ter garantido o motor no ar (garantirMotorStt) sob a.sttMutex.
func (a *App) chamarSidecarStt(endpoint string) ([]byte, error) {
	resp, err := clienteHttpStt.Post(motoresstt.EnderecoBase()+endpoint, "application/json", bytes.NewReader(nil))
	if err != nil {
		return nil, fmt.Errorf("falha ao falar com o motor de escuta: %w", err)
	}
	defer resp.Body.Close()

	var corpo bytes.Buffer
	if _, err := corpo.ReadFrom(resp.Body); err != nil {
		return nil, fmt.Errorf("falha ao ler a resposta do motor de escuta: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		var respostaErro struct {
			Error string `json:"error"`
		}
		if errDecode := json.Unmarshal(corpo.Bytes(), &respostaErro); errDecode == nil && respostaErro.Error != "" {
			return nil, fmt.Errorf("motor de STT: %s", respostaErro.Error)
		}
		return nil, fmt.Errorf("motor de STT respondeu HTTP %d", resp.StatusCode)
	}
	return corpo.Bytes(), nil
}
