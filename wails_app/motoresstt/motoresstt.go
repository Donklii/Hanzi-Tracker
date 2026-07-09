// Package motoresstt é o DONO do ciclo de vida do processo de STT (reconhecimento de fala da
// revisão de pronúncia) dentro do app: subir, derrubar e trocar o motor em runtime, além do
// catálogo de motores baixáveis (ver manifesto.go) e da resolução de qual motor subir (ver
// caminhos.go). O núcleo genérico de gerenciamento (porta/healthcheck/processo) vive no pacote
// sidecar, compartilhado com motoresocr e motorestts; aqui fica só o que é específico do STT.
// O motor de STT COEXISTE com os de OCR e TTS (processos e portas separados) e sobe
// PREGUIÇOSAMENTE — só quando a revisão de pronúncia precisa dele, nunca no startup.
// Ver docs/CONTRATO-STT.md.
package motoresstt

import (
	"os"
	"path/filepath"
	"strings"
	"time"

	"wails_app/sidecar"
)

// VersaoContrato é a versão do contrato da API de STT que este app entende (ver
// docs/CONTRATO-STT.md). O healthcheck recusa um sidecar cujo `versaoContrato` seja maior.
const VersaoContrato = 1

// servicoStt descreve o sidecar de STT para o núcleo genérico: como o STT coexiste com OCR e TTS,
// tem env de porta própria (HANZITRACKER_STT_PORT; 8091 é apenas o fallback de execução avulsa).
var servicoStt = sidecar.Servico{
	Rotulo:         "STT",
	EnvPorta:       "HANZITRACKER_STT_PORT",
	PortaFallback:  8091,
	VersaoContrato: VersaoContrato,
}

// DescritorMotorStt descreve COMO subir o backend de STT (alias do descritor genérico de sidecar).
type DescritorMotorStt = sidecar.Descritor

// GerenciadorMotorStt é o DONO do ciclo de vida do processo de STT (alias do gerenciador genérico).
type GerenciadorMotorStt = sidecar.Gerenciador

// EnderecoBase devolve a base URL do microserviço de STT.
func EnderecoBase() string {
	return servicoStt.EnderecoBase()
}

// AguardarBackend aguarda o backend de STT responder o healthcheck com contrato compatível.
func AguardarBackend(timeout time.Duration) error {
	return servicoStt.AguardarBackend(timeout)
}

// NovoGerenciadorMotorStt cria o gerenciador já resolvendo a porta do STT (da env ou livre).
func NovoGerenciadorMotorStt() *GerenciadorMotorStt {
	return sidecar.NovoGerenciador(servicoStt)
}

// resolverMotorSttBundle resolve um sidecar de STT congelado empacotado AO LADO do app (build
// local via builds/build_sidecars_stt_*.{sh,ps1}, antes da release publicada). Devolve ok=false
// quando não há bundle — sinal de que o motor precisa ser baixado.
func resolverMotorSttBundle(m MotorSttBaixavel) (DescritorMotorStt, bool) {
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
			return DescritorMotorStt{Nome: m.Rotulo + " (bundle)", Catalogo: m.Nome, Comando: abs}, true
		}
	}
	return DescritorMotorStt{}, false
}
