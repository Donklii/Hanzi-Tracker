package motoresstt

import (
	"os"
	"path/filepath"

	"wails_app/armazenamento"
)

// ----- Caminhos -----

// PastaMotoresStt é a raiz dos motores de STT (%APPDATA%\HanziTracker\motores_stt). A pasta é
// SEPARADA de motores_ocr\ e motores_tts\ pelo mesmo motivo que separa aquelas duas: cada
// categoria tem a própria política de limpeza na aba Armazenamento. Como nos outros, cada motor
// guarda TANTO o executável QUANTO os pesos dele (em modelos\, o cache do Hugging Face vai em
// modelos\hf).
func PastaMotoresStt() string {
	return filepath.Join(armazenamento.PastaDados(), "motores_stt")
}

// PastaMotorStt é a subpasta de UM motor de STT (o zip é extraído aqui; o executável fica na raiz
// dela e os pesos baixados do Hugging Face na primeira transcrição vão para modelos\hf — ver
// paraformer_server.py).
func PastaMotorStt(nome string) string {
	return filepath.Join(PastaMotoresStt(), nome)
}

// CaminhoExecutavelMotorStt é o caminho completo do executável de um motor de STT instalado.
func CaminhoExecutavelMotorStt(m MotorSttBaixavel) string {
	return filepath.Join(PastaMotorStt(m.Nome), m.Executavel)
}

// ----- Resolução do motor a subir -----

// DescritorMotorSttInstalado devolve como rodar um motor de STT JÁ BAIXADO no AppData (ok=false se
// ausente).
func DescritorMotorSttInstalado(m MotorSttBaixavel) (DescritorMotorStt, bool) {
	exe := CaminhoExecutavelMotorStt(m)
	info, err := os.Stat(exe)
	if err != nil || info.IsDir() {
		return DescritorMotorStt{}, false
	}
	abs, err := filepath.Abs(exe)
	if err != nil {
		abs = exe
	}
	return DescritorMotorStt{Nome: m.Rotulo + " (instalado)", Catalogo: m.Nome, Comando: abs}, true
}

// ResolverMotorStt resolve como subir o motor de STT `nome`, na ordem: (1) instalado no AppData;
// (2) sidecar em bundle ao lado do app (build local). Devolve ok=false quando o motor não está
// disponível — sinal para a UI pedir o download em "Gerenciar Motores de Escuta". Como no TTS,
// NÃO há bootstrap automático: o download é sempre uma ação explícita do usuário.
func ResolverMotorStt(nome string) (DescritorMotorStt, bool) {
	m, ok := ObterMotorSttBaixavel(nome)
	if !ok {
		return DescritorMotorStt{}, false
	}
	if desc, ok := DescritorMotorSttInstalado(m); ok {
		return desc, true
	}
	return resolverMotorSttBundle(m)
}
