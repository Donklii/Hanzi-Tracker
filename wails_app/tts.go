package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"wails_app/progresso"
)

// ----- Leitura do pinyin em voz alta (TTS) -----
// Cliente do contrato de TTS (docs/CONTRATO-TTS.md) + o gatilho FalarPinyin exposto ao frontend.
// O motor de voz é um sidecar próprio (Kokoro-82M ou ChatTTS, ver motores_tts.go) que COEXISTE com
// o de OCR e sobe PREGUIÇOSAMENTE: só na primeira leitura em voz alta, nunca no startup — a feature
// é opcional e desligada por padrão.

// enderecoBaseTts devolve a base URL do microserviço de TTS. A porta é resolvida por portaTts()
// (motor_tts.go) na criação do gerenciador e publicada em HANZITRACKER_TTS_PORT; o 8090 é apenas um
// fallback para execução avulsa.
func enderecoBaseTts() string {
	porta := os.Getenv("HANZITRACKER_TTS_PORT")
	if porta == "" {
		porta = "8090"
	}
	return "http://localhost:" + porta
}

// VersaoContratoTts é a versão do contrato da API de TTS que este app entende (ver
// docs/CONTRATO-TTS.md). O healthcheck recusa um sidecar cujo `versaoContrato` seja maior.
const VersaoContratoTts = 1

// aguardarBackendTts aguarda o backend de TTS responder GET /api/health com status "ok" e um
// contrato compatível, tentando repetidamente até `timeout`. Espelha aguardarBackendOcr (app.go);
// reaproveita RespostaHealth — o JSON de health dos dois contratos tem o mesmo formato.
func aguardarBackendTts(timeout time.Duration) error {
	cliente := &http.Client{Timeout: 2 * time.Second}
	prazo := time.Now().Add(timeout)
	var ultimoErro error

	for time.Now().Before(prazo) {
		resp, err := cliente.Get(enderecoBaseTts() + "/api/health")
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

		// Guard clause: contrato mais novo do que o app sabe falar — não engata.
		if saude.VersaoContrato > VersaoContratoTts {
			return fmt.Errorf("motor de TTS fala o contrato v%d, mas o app só entende até v%d — atualize o app", saude.VersaoContrato, VersaoContratoTts)
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
	return fmt.Errorf("backend de TTS não ficou pronto em %s: %w", timeout, ultimoErro)
}

// emitirEstadoTts envia ao frontend o estado atual da síntese (barra de status). Mensagem vazia =
// terminou/limpou.
func (a *App) emitirEstadoTts(mensagem string) {
	if a.ctx == nil {
		return
	}
	runtime.EventsEmit(a.ctx, "tts_estado", mensagem)
}

// garantirMotorTts deixa o motor de voz `nome` no ar e saudável: cria o gerenciador na primeira
// chamada, sobe/troca o sidecar quando o motor pedido difere do ativo e ressuscita um processo que
// morreu. DEVE ser chamado já com a.ttsMutex adquirido (feito em FalarPinyin).
func (a *App) garantirMotorTts(nome string) error {
	if a.motorTts == nil {
		a.motorTts = NovoGerenciadorMotorTts()
	}

	// Motor certo já no ar? Healthcheck curto pega o caso de processo morto (crash) — aí re-sobe.
	if a.motorTts.CatalogoAtivo() == nome {
		if err := aguardarBackendTts(2 * time.Second); err == nil {
			return nil
		}
	}

	desc, ok := resolverMotorTts(nome)
	if !ok {
		return fmt.Errorf("o motor de voz '%s' não está instalado — baixe-o em Configurações → Geral → Gerenciar Motores de Voz", nome)
	}

	a.emitirEstadoTts("Iniciando o motor de voz…")
	return a.motorTts.Trocar(desc, 60*time.Second)
}

// FalarPinyin é o gatilho de "ler o pinyin em voz alta": recebe o HANZI do card (não a string de
// pinyin romanizada — é o hanzi que garante pronúncia nativa correta) e o nome do motor de TTS
// selecionado, e devolve os bytes do WAV sintetizado em base64 (o frontend toca via <audio> — o
// popup nativo Win32 não tem áudio). Consulta primeiro o cache em SQLite: sínteses repetidas do
// mesmo hanzi saem instantâneas e sem custo de CPU.
//
// A primeira chamada de cada motor pode demorar: sobe o sidecar, carrega o modelo e — só na
// primeiríssima vez — baixa os pesos do Hugging Face (~330 MB no Kokoro, ~1 GB no ChatTTS). O
// progresso é anunciado ao frontend via o evento "tts_estado".
func (a *App) FalarPinyin(hanzi string, motor string) (string, error) {
	if hanzi == "" {
		return "", fmt.Errorf("hanzi vazio")
	}
	if _, ok := ObterMotorTtsBaixavel(motor); !ok {
		return "", fmt.Errorf("motor de TTS desconhecido: %s", motor)
	}

	// Serializa as leituras: duas sínteses simultâneas duplicariam o trabalho pesado de CPU e
	// tocariam áudios sobrepostos. O lock também protege a criação preguiçosa de a.motorTts.
	a.ttsMutex.Lock()
	defer a.ttsMutex.Unlock()

	// Cache primeiro: se já sintetizamos este hanzi com este motor, nem precisa de sidecar.
	if audio, achou, err := progresso.BuscarAudioTts(hanzi, motor); err == nil && achou {
		return base64.StdEncoding.EncodeToString(audio), nil
	}

	if err := a.garantirMotorTts(motor); err != nil {
		a.emitirEstadoTts("")
		return "", err
	}

	a.emitirEstadoTts("Sintetizando fala… (a primeira vez pode baixar o modelo de voz)")
	defer a.emitirEstadoTts("")

	corpo, err := json.Marshal(map[string]string{"texto": hanzi})
	if err != nil {
		return "", err
	}

	// Timeout longo de propósito: a primeira síntese de cada motor pode incluir o download dos
	// pesos do Hugging Face (feito pelo sidecar) numa conexão lenta.
	cliente := &http.Client{Timeout: 15 * time.Minute}
	resp, err := cliente.Post(enderecoBaseTts()+"/api/tts", "application/json", bytes.NewReader(corpo))
	if err != nil {
		return "", fmt.Errorf("falha ao sintetizar fala: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var respostaErro struct {
			Error string `json:"error"`
		}
		if errDecode := json.NewDecoder(resp.Body).Decode(&respostaErro); errDecode == nil && respostaErro.Error != "" {
			return "", fmt.Errorf("motor de TTS: %s", respostaErro.Error)
		}
		return "", fmt.Errorf("motor de TTS respondeu HTTP %d", resp.StatusCode)
	}

	audio, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("falha ao ler o áudio sintetizado: %w", err)
	}
	if len(audio) == 0 {
		return "", fmt.Errorf("motor de TTS devolveu áudio vazio")
	}

	if err := progresso.SalvarAudioTts(hanzi, motor, audio); err != nil {
		fmt.Printf("Aviso: falha ao salvar o áudio no cache de TTS: %v\n", err)
	}

	return base64.StdEncoding.EncodeToString(audio), nil
}
