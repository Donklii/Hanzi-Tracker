package motorestts

import (
	"os"
	"path/filepath"

	"wails_app/armazenamento"
)

// ----- Caminhos -----

// PastaMotoresTts é a raiz dos motores de voz (%APPDATA%\HanziTracker\motores_tts). A pasta é SEPARADA
// de motores_ocr\ (OCR) porque a limpeza de motores de OCR preserva apenas o motor ativo — misturar os
// dois faria a limpeza de OCR apagar um motor de voz instalado. Como no OCR, cada motor guarda TANTO o
// executável QUANTO os pesos dele (em modelos\, o cache do Hugging Face vai em modelos\hf).
func PastaMotoresTts() string {
	return filepath.Join(armazenamento.PastaDados(), "motores_tts")
}

// PastaMotorTts é a subpasta de UM motor de voz (o zip é extraído aqui; o .exe fica na raiz dela e os
// pesos baixados do Hugging Face na primeira síntese vão para modelos\hf — ver kokoro_server.py).
func PastaMotorTts(nome string) string {
	return filepath.Join(PastaMotoresTts(), nome)
}

// CaminhoExecutavelMotorTts é o caminho completo do .exe de um motor de voz instalado.
func CaminhoExecutavelMotorTts(m MotorTtsBaixavel) string {
	return filepath.Join(PastaMotorTts(m.Nome), m.Executavel)
}

// ----- Resolução do motor a subir -----

// DescritorMotorTtsInstalado devolve como rodar um motor de voz JÁ BAIXADO no AppData (ok=false se
// ausente).
func DescritorMotorTtsInstalado(m MotorTtsBaixavel) (DescritorMotorTts, bool) {
	exe := CaminhoExecutavelMotorTts(m)
	info, err := os.Stat(exe)
	if err != nil || info.IsDir() {
		return DescritorMotorTts{}, false
	}
	abs, err := filepath.Abs(exe)
	if err != nil {
		abs = exe
	}
	return DescritorMotorTts{Nome: m.Rotulo + " (instalado)", Catalogo: m.Nome, Comando: abs}, true
}

// ResolverMotorTts resolve como subir o motor de voz `nome`, na ordem: (1) instalado no AppData;
// (2) sidecar em bundle ao lado do app (build local). Devolve ok=false quando o motor não está
// disponível — sinal para a UI pedir o download em "Gerenciar Motores de Voz". Diferente do OCR,
// NÃO há bootstrap automático: a feature é opcional e desligada por padrão, então o download é
// sempre uma ação explícita do usuário.
func ResolverMotorTts(nome string) (DescritorMotorTts, bool) {
	m, ok := ObterMotorTtsBaixavel(nome)
	if !ok {
		return DescritorMotorTts{}, false
	}
	if desc, ok := DescritorMotorTtsInstalado(m); ok {
		return desc, true
	}
	return resolverMotorTtsBundle(m)
}
