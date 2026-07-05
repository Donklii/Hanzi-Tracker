package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"wails_app/motorestts"
	"wails_app/progresso"
)

// Timeout longo de propósito: a primeira síntese de cada motor pode incluir o download dos
// pesos do Hugging Face (feito pelo sidecar) numa conexão lenta.
var clienteHttpTts = &http.Client{Timeout: 15 * time.Minute}

// ----- Leitura do pinyin em voz alta (TTS) -----
// Cliente do contrato de TTS (docs/CONTRATO-TTS.md) + o gatilho FalarPinyin exposto ao frontend.
// O motor de voz é um sidecar próprio (Kokoro-82M ou ChatTTS, ver pacote motorestts) que COEXISTE com
// o de OCR e sobe PREGUIÇOSAMENTE: só na primeira leitura em voz alta, nunca no startup — a feature
// é opcional e desligada por padrão.

// DespertarMotorTts garante que o motor de TTS esteja no ar (sobe o sidecar se não estiver).
// Útil para pré-aquecer o modelo em background antes da primeira chamada real de áudio.
func (a *App) DespertarMotorTts() {
	if a.Config.MotorTtsAtivo == "" {
		return
	}

	// Não bloqueamos a thread principal para não travar a interface do usuário
	// O TTS é thread-safe devido ao ttsMutex
	go func() {
		a.ttsMutex.Lock()
		defer a.ttsMutex.Unlock()

		_ = a.garantirMotorTts(a.Config.MotorTtsAtivo)
		a.emitirEstadoTts("") // Limpa o status
	}()
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
		a.motorTts = motorestts.NovoGerenciadorMotorTts()
	}

	// Motor certo já no ar? Healthcheck curto pega o caso de processo morto (crash) — aí re-sobe.
	if a.motorTts.CatalogoAtivo() == nome {
		if err := motorestts.AguardarBackend(2 * time.Second); err == nil {
			return nil
		}
	}

	desc, ok := motorestts.ResolverMotorTts(nome)
	if !ok {
		return fmt.Errorf("o motor de voz '%s' não está instalado — baixe-o em Configurações → Geral → Gerenciar Motores de Voz", nome)
	}

	a.emitirEstadoTts("Iniciando o motor de voz…")
	return a.motorTts.Trocar(desc, 60*time.Second)
}

// FalarPinyin é o gatilho de "ler o pinyin em voz alta": recebe o HANZI do card (não a string de
// pinyin romanizada — é o hanzi que garante pronúncia nativa correta) e o nome do motor de TTS
// selecionado, e devolve os bytes do WAV sintetizado em base64 (o frontend toca via <audio> — o
// popup nativo Win32 não tem áudio). Consulta primeiro o cache em SQLite, indexado pela PRONÚNCIA
// (pinyin do hanzi): hanzis homófonos compartilham um único áudio, então a leitura sai instantânea e
// sem custo de CPU mesmo na primeira vez que ESTE hanzi aparece, se um homófono já foi lido antes.
//
// A primeira chamada de cada motor pode demorar: sobe o sidecar, carrega o modelo e — só na
// primeiríssima vez — baixa os pesos do Hugging Face (~330 MB no Kokoro, ~1 GB no ChatTTS). O
// progresso é anunciado ao frontend via o evento "tts_estado".
func (a *App) FalarPinyin(hanzi string, motor string) (string, error) {
	if hanzi == "" {
		return "", fmt.Errorf("hanzi vazio")
	}
	if _, ok := motorestts.ObterMotorTtsBaixavel(motor); !ok {
		return "", fmt.Errorf("motor de TTS desconhecido: %s", motor)
	}

	// Serializa as leituras: duas sínteses simultâneas duplicariam o trabalho pesado de CPU e
	// tocariam áudios sobrepostos. O lock também protege a criação preguiçosa de a.motorTts.
	a.ttsMutex.Lock()
	defer a.ttsMutex.Unlock()

	// Chave de cache = pinyin do hanzi (ver traduzirHanziParaChaveTts): hanzis homófonos caem na
	// mesma chave e reaproveitam o mesmo áudio.
	chave := a.traduzirHanziParaChaveTts(hanzi)

	// Cache primeiro: se já sintetizamos ESTA PRONÚNCIA com este motor, nem precisa de sidecar.
	if audio, achou, err := progresso.BuscarAudioTts(chave, motor); err == nil && achou {
		return base64.StdEncoding.EncodeToString(audio), nil
	}

	if err := a.garantirMotorTts(motor); err != nil {
		a.emitirEstadoTts("")
		return "", err
	}

	a.emitirEstadoTts("Sintetizando fala… (a primeira vez pode baixar o modelo de voz)")
	defer a.emitirEstadoTts("")

	audio, err := a.sintetizarPinyin(hanzi)
	if err != nil {
		return "", err
	}

	if err := progresso.SalvarAudioTts(chave, motor, audio); err != nil {
		fmt.Printf("Aviso: falha ao salvar o áudio no cache de TTS: %v\n", err)
	}

	return base64.StdEncoding.EncodeToString(audio), nil
}

// traduzirHanziParaChaveTts devolve a CHAVE de cache de áudio de um hanzi: o seu pinyin canônico
// (com tom, minúsculo, sílabas separadas por um espaço). É a "interface" entre a SÍNTESE (feita a
// partir do hanzi, que garante a pronúncia nativa) e o CACHE (indexado por pinyin): hanzis homófonos
// caem na MESMA chave e por isso compartilham um único WAV — 马/码/吗 (todos "mǎ"/"ma") guardam um só
// áudio, evitando re-sintetizar e re-salvar o mesmo som.
//
// A fonte da leitura canônica depende do tamanho:
//   - CARACTERE ISOLADO (1 rune): MakeMeAHanzi PRIMEIRO — é mais confiável para o pinyin de um
//     caractere único —, com CC-CEDICT como fallback.
//   - PALAVRA (2+ runes): CC-CEDICT (o MakeMeAHanzi só cataloga caracteres soltos).
//
// Quando nenhum dicionário conhece o hanzi, cai de volta no próprio hanzi como chave — o cache
// continua funcionando, só não deduplica (sem colisão possível: hanzi é CJK, pinyin é latino).
func (a *App) traduzirHanziParaChaveTts(hanzi string) string {
	// Caractere isolado: MakeMeAHanzi tem prioridade (pinyin mais confiável por caractere).
	if utf8.RuneCountInString(hanzi) == 1 && a.BancoHanzi != nil {
		if entrada := a.BancoHanzi.Buscar(hanzi); entrada != nil && len(entrada.Pinyin) > 0 {
			if chave := normalizarChavePinyin(entrada.Pinyin[0]); chave != "" {
				return chave
			}
		}
	}

	// Palavra (ou caractere isolado que o MakeMeAHanzi não conhece): CC-CEDICT.
	if a.Cedict != nil {
		if entradas := a.Cedict.Buscar(hanzi); len(entradas) > 0 {
			if chave := normalizarChavePinyin(entradas[0].Pinyin); chave != "" {
				return chave
			}
		}
	}

	return hanzi
}

// normalizarChavePinyin põe o pinyin numa forma canônica estável para servir de chave de cache:
// minúsculo, sem espaços nas pontas e com espaços internos colapsados a um só. PRESERVA os tons
// (diacríticos) — sem eles, sons diferentes (mā/mǎ) colidiriam numa chave só.
func normalizarChavePinyin(p string) string {
	return strings.Join(strings.Fields(strings.ToLower(p)), " ")
}

// sintetizarPinyin faz a chamada HTTP crua ao sidecar de TTS JÁ NO AR e devolve os bytes do WAV.
// NÃO consulta nem grava cache, NÃO sobe o motor e NÃO emite estado — é o núcleo compartilhado entre
// a leitura interativa (FalarPinyin) e o pré-carregamento em lote do cache (PreCarregarCacheTts). O
// chamador DEVE ter garantido o motor no ar (garantirMotorTts) sob a.ttsMutex.
func (a *App) sintetizarPinyin(hanzi string) ([]byte, error) {
	corpo, err := json.Marshal(map[string]string{"texto": hanzi})
	if err != nil {
		return nil, err
	}

	resp, err := clienteHttpTts.Post(motorestts.EnderecoBase()+"/api/tts", "application/json", bytes.NewReader(corpo))
	if err != nil {
		return nil, fmt.Errorf("falha ao sintetizar fala: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var respostaErro struct {
			Error string `json:"error"`
		}
		if errDecode := json.NewDecoder(resp.Body).Decode(&respostaErro); errDecode == nil && respostaErro.Error != "" {
			return nil, fmt.Errorf("motor de TTS: %s", respostaErro.Error)
		}
		return nil, fmt.Errorf("motor de TTS respondeu HTTP %d", resp.StatusCode)
	}

	audio, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("falha ao ler o áudio sintetizado: %w", err)
	}
	if len(audio) == 0 {
		return nil, fmt.Errorf("motor de TTS devolveu áudio vazio")
	}
	return audio, nil
}
