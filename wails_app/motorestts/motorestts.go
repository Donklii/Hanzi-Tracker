// Package motorestts é o DONO do ciclo de vida do processo de TTS dentro do app: subir, derrubar e
// trocar o motor de voz em runtime, além do catálogo de motores baixáveis (ver manifesto.go) e da
// resolução de qual motor subir (ver caminhos.go). O núcleo genérico de gerenciamento
// (porta/healthcheck/processo) vive no pacote sidecar, compartilhado com motoresocr; aqui fica só o
// que é específico do TTS. O motor de voz COEXISTE com o de OCR (dois processos ao mesmo tempo,
// portas separadas) e sobe PREGUIÇOSAMENTE — só na primeira leitura em voz alta, nunca no startup
// (a feature é opcional e desligada por padrão). Ver docs/CONTRATO-TTS.md.
package motorestts

import (
	"os"
	"path/filepath"
	"strings"
	"time"

	"wails_app/sidecar"
)

// VersaoContrato é a versão do contrato da API de TTS que este app entende (ver
// docs/CONTRATO-TTS.md). O healthcheck recusa um sidecar cujo `versaoContrato` seja maior.
const VersaoContrato = 1

// servicoTts descreve o sidecar de TTS para o núcleo genérico: como o TTS coexiste com o OCR, tem
// env de porta própria (HANZITRACKER_TTS_PORT; 8090 é apenas o fallback de execução avulsa).
var servicoTts = sidecar.Servico{
	Rotulo:         "TTS",
	EnvPorta:       "HANZITRACKER_TTS_PORT",
	PortaFallback:  8090,
	VersaoContrato: VersaoContrato,
}

// DescritorMotorTts descreve COMO subir o backend de TTS (alias do descritor genérico de sidecar).
type DescritorMotorTts = sidecar.Descritor

// GerenciadorMotorTts é o DONO do ciclo de vida do processo de TTS (alias do gerenciador genérico).
type GerenciadorMotorTts = sidecar.Gerenciador

// EnderecoBase devolve a base URL do microserviço de TTS.
func EnderecoBase() string {
	return servicoTts.EnderecoBase()
}

// AguardarBackend aguarda o backend de TTS responder o healthcheck com contrato compatível.
func AguardarBackend(timeout time.Duration) error {
	return servicoTts.AguardarBackend(timeout)
}

// NovoGerenciadorMotorTts cria o gerenciador já resolvendo a porta do TTS (da env ou livre).
func NovoGerenciadorMotorTts() *GerenciadorMotorTts {
	return sidecar.NovoGerenciador(servicoTts)
}

// resolverMotorTtsBundle resolve um sidecar de TTS congelado empacotado AO LADO do app (instalação
// com bundle opcional / build local via builds/build_sidecars_tts_windows.ps1, antes da release
// publicada). Devolve ok=false quando não há bundle — sinal de que o motor precisa ser baixado.
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
