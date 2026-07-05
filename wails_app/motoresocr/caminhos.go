package motoresocr

import (
	"os"
	"path/filepath"

	"wails_app/armazenamento"
)

// ----- Caminhos -----

// PastaMotoresOcr é a raiz dos motores de OCR (%APPDATA%\HanziTracker\motores_ocr). O nome espelha
// motores_tts\ (motorestts) para as duas famílias de motor ficarem coerentes no AppData. Cada motor
// vive numa subpasta própria que guarda TANTO o executável QUANTO os pesos dele (em modelos\), então
// não há mais uma pasta modelos\ separada duplicando os nomes dos motores.
func PastaMotoresOcr() string {
	return filepath.Join(armazenamento.PastaDados(), "motores_ocr")
}

// PastaMotorOcr é a subpasta de UM motor de OCR (o zip é extraído aqui; o .exe fica na raiz dela e os
// pesos vão para a subpasta modelos\ — ver PastaModelosMotor).
func PastaMotorOcr(nome string) string {
	return filepath.Join(PastaMotoresOcr(), nome)
}

// PastaModelosMotor é a subpasta de pesos de UM motor (motores_ocr\<Motor>\modelos, ex.:
// motores_ocr\RapidOCR\modelos). Fica DENTRO da pasta do próprio motor para não repetir o nome do
// motor numa árvore modelos\ paralela. O mesmo <Motor> é injetado no sidecar via HANZITRACKER_MOTOR
// (ver iniciarSemLock), então o Python lê os pesos exatamente onde o Go os baixa — deve casar com
// obterPastaModelos() do GerenciadorModelosModule.py.
func PastaModelosMotor(nome string) string {
	return filepath.Join(PastaMotorOcr(nome), "modelos")
}

// CaminhoExecutavelMotor é o caminho completo do .exe de um motor instalado (PastaMotorOcr + Executavel).
func CaminhoExecutavelMotor(m MotorOcrBaixavel) string {
	return filepath.Join(PastaMotorOcr(m.Nome), m.Executavel)
}

// ----- Resolução do motor a subir -----

// DescritorMotorInstalado devolve como rodar um motor JÁ BAIXADO no AppData (ok=false se ausente).
func DescritorMotorInstalado(m MotorOcrBaixavel) (DescritorMotorOcr, bool) {
	exe := CaminhoExecutavelMotor(m)
	info, err := os.Stat(exe)
	if err != nil || info.IsDir() {
		return DescritorMotorOcr{}, false
	}
	abs, err := filepath.Abs(exe)
	if err != nil {
		abs = exe
	}
	return DescritorMotorOcr{Nome: m.Rotulo + " (instalado)", Catalogo: m.Nome, Comando: abs}, true
}

// ResolverMotorInicial escolhe o motor a subir na inicialização, na ordem: (1) motor preferido do
// usuário (último ativo) instalado no AppData; (2) motor padrão do catálogo instalado no AppData;
// (3) sidecar em bundle ao lado do app. Devolve ok=false quando NADA foi encontrado — sinal de
// first-run: o app deve chamar bootstrapMotorPadrao (baixa+instala+ativa o RapidOCR padrão).
func ResolverMotorInicial(nomePreferido string) (DescritorMotorOcr, bool) {
	if nomePreferido != "" {
		if m, ok := ObterMotorBaixavel(nomePreferido); ok {
			if desc, ok := DescritorMotorInstalado(m); ok {
				return desc, true
			}
		}
	}
	if padrao, ok := MotorOcrPadrao(); ok {
		if desc, ok := DescritorMotorInstalado(padrao); ok {
			return desc, true
		}
	}
	return resolverMotorOcrBundle()
}
