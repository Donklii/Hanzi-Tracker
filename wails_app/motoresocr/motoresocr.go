// Package motoresocr é o DONO do ciclo de vida do processo de OCR dentro do app: subir, derrubar e
// trocar (hot-swap) o motor em runtime, além do catálogo de motores baixáveis (ver manifesto.go) e
// da resolução de qual motor subir (ver caminhos.go). O núcleo genérico de gerenciamento
// (porta/healthcheck/processo) vive no pacote sidecar, compartilhado com motorestts; aqui fica só o
// que é específico do OCR. Ver docs/CONTRATO-OCR.md.
package motoresocr

import (
	"os"
	"path/filepath"
	"time"

	"wails_app/sidecar"
)

// VersaoContrato é a versão do contrato da API de OCR que este app entende (ver docs/CONTRATO-OCR.md).
// O healthcheck recusa um sidecar cujo `versaoContrato` seja maior (contrato mais novo do que o app sabe
// falar), evitando engatar um motor incompatível.
const VersaoContrato = 1

// servicoOcr descreve o sidecar de OCR para o núcleo genérico: a porta dinâmica vem do orquestrador
// via HANZITRACKER_OCR_PORT (8080 é apenas o fallback de execução avulsa).
var servicoOcr = sidecar.Servico{
	Rotulo:         "OCR",
	EnvPorta:       "HANZITRACKER_OCR_PORT",
	PortaFallback:  8080,
	VersaoContrato: VersaoContrato,
}

// DescritorMotorOcr descreve COMO subir o backend de OCR (alias do descritor genérico de sidecar).
type DescritorMotorOcr = sidecar.Descritor

// GerenciadorMotorOcr é o DONO do ciclo de vida do processo de OCR (alias do gerenciador genérico).
type GerenciadorMotorOcr = sidecar.Gerenciador

// EnderecoBase devolve a base URL do microserviço Python de OCR.
func EnderecoBase() string {
	return servicoOcr.EnderecoBase()
}

// AguardarBackend aguarda o backend de OCR responder o healthcheck com contrato compatível.
func AguardarBackend(timeout time.Duration) error {
	return servicoOcr.AguardarBackend(timeout)
}

// NovoGerenciadorMotorOcr cria o gerenciador já resolvendo a porta do OCR (do orquestrador ou livre).
func NovoGerenciadorMotorOcr() *GerenciadorMotorOcr {
	return sidecar.NovoGerenciador(servicoOcr)
}

// resolverMotorOcrBundle resolve um sidecar congelado empacotado AO LADO do app (instalação com bundle
// opcional / execução offline). NÃO há fallback para `python server.py`: todo motor é um executável,
// baixado no AppData ou em bundle. Devolve ok=false quando não há bundle — sinal de que o motor precisa
// ser baixado (o bootstrap decide isso; ResolverMotorInicial também considera os motores já baixados no
// AppData). Os arquivos python_backend/*.py continuam sendo a fonte para congelar os sidecars (não são
// executados pelo app). Ver caminhos.go e BUILD.md §3.
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
