package main

// ----- Manifesto de Motores de OCR (sidecars baixáveis) -----
// Catálogo dos MOTORES de OCR e do componente de overlay compartilhado, publicados como .zip em
// GitHub Releases deste repo e baixados sob demanda pelo Go para %APPDATA%\HanziTracker\motores\.
//
// Diferença para ocr/ModelosManifesto.py (PESOS ONNX): lá são apenas arquivos .onnx que o motor LÊ;
// aqui são os PRÓPRIOS executáveis do motor (server.py congelado por PyInstaller). Este catálogo vive
// no GO — e não no Python — porque o app precisa baixar o motor padrão ANTES de qualquer processo
// Python existir (bootstrap de first-run, Fase 5). O `Sha256` é OBRIGATÓRIO para binários: baixarArquivo
// confere o hash após o download e recusa se divergir. Ver docs/PUBLICAR-MOTORES.md e Fase 5 no TODO.md.

// _baseReleaseMotores é o prefixo das URLs de download dos assets no GitHub Releases deste repo. O
// segmento seguinte é a TAG da release (ex.: "motores-v1"), que versiona os binários com URL imutável.
const _baseReleaseMotores = "https://github.com/Donklii/Hanzi-Tracker/releases/download"

// ArtefatoBaixavel descreve um único .zip publicado: onde baixar, o hash para conferência e o tamanho.
type ArtefatoBaixavel struct {
	Nome         string `json:"nome"`         // nome do arquivo (ex.: "ocr_server.zip")
	Url          string `json:"url"`          // URL HTTPS estável (asset de GitHub Release)
	Sha256       string `json:"sha256"`       // OBRIGATÓRIO — conferido após o download (baixarArquivo)
	TamanhoBytes int64  `json:"tamanhoBytes"` // para pré-checagem de disco e barra de progresso
}

// MotorOcrBaixavel é uma entrada do catálogo de motores: metadados para a UI "Gerenciar Motores"
// (Passo 6) + o artefato a baixar e o caminho do .exe dentro do zip já extraído.
type MotorOcrBaixavel struct {
	Nome       string           `json:"nome"`       // chave única / rótulo curto (ex.: "RapidOCR")
	Rotulo     string           `json:"rotulo"`     // rótulo humano para a UI
	Descricao  string           `json:"descricao"`  //
	Idiomas    []string         `json:"idiomas"`    // códigos ISO (ex.: ["zh", "en"])
	Versao     string           `json:"versao"`     // versão do motor (semver)
	Variante   string           `json:"variante"`   // aceleração: "CPU", "DirectML", "CUDA" (ou combinação)
	Requisitos string           `json:"requisitos"` // "" = nenhum; ex.: "GPU Nvidia + drivers CUDA"
	Padrao     bool             `json:"padrao"`     // baixado automaticamente no bootstrap de first-run?
	Executavel string           `json:"executavel"` // .exe relativo à pasta de extração (o zip traz o exe na raiz — ex.: "ocr_server.exe")
	Artefato   ArtefatoBaixavel `json:"artefato"`   //
}

// PopupOverlayBaixavel é o COMPONENTE de overlay compartilhado por TODOS os motores (a janela de
// tradução dirigida pelo Go via stdin). NÃO é um motor de OCR: é baixado uma única vez no bootstrap,
// à parte do catálogo de motores. O .exe extraído (popup/popup.exe) é localizado por resolverComandoPopup.
var PopupOverlayBaixavel = ArtefatoBaixavel{
	Nome:         "popup.zip",
	Url:          _baseReleaseMotores + "/motores-v1/popup.zip",
	Sha256:       "c4b5cf9c98b0f8b6aeaefd0ca4b4b2bd7184955e526c5650b60a561f0c774030",
	TamanhoBytes: 11102791,
}

// MotoresBaixaveis é o catálogo de MOTORES de OCR publicados. Hoje só o RapidOCR padrão (o sidecar
// congelado com DirectML embutido + fallback automático para CPU — cobre Nvidia/AMD/Intel sem CUDA
// Toolkit). EasyOCR/Tesseract/PaddleOCR e uma variante RapidOCR-DirectML dedicada entram aqui conforme
// forem publicados (Passos 4 e 7 do TODO.md).
var MotoresBaixaveis = map[string]MotorOcrBaixavel{
	"RapidOCR": {
		Nome:   "RapidOCR",
		Rotulo: "RapidOCR (Padrão)",
		Descricao: "Motor padrão do Hanzi Tracker: RapidOCR sobre onnxruntime, leve e preciso. " +
			"Aceleração DirectML embutida (Nvidia/AMD/Intel, sem CUDA Toolkit) com fallback automático para CPU.",
		Idiomas:    []string{"zh", "en"},
		Versao:     "1.0.0",
		Variante:   "CPU/DirectML",
		Requisitos: "",
		Padrao:     true,
		Executavel: "ocr_server.exe",
		Artefato: ArtefatoBaixavel{
			Nome:         "ocr_server.zip",
			Url:          _baseReleaseMotores + "/motores-v1/ocr_server.zip",
			Sha256:       "50e0fb93c1a2260acd005909081f1552afe102eb4903c426537e3bf5e731ea31",
			TamanhoBytes: 121551807,
		},
	},
}

// MotorOcrPadrao devolve o motor a baixar no bootstrap de first-run (o marcado Padrao); ok=false se
// o catálogo não declarar um padrão. Só existe um hoje (RapidOCR).
func MotorOcrPadrao() (MotorOcrBaixavel, bool) {
	for _, m := range MotoresBaixaveis {
		if m.Padrao {
			return m, true
		}
	}
	return MotorOcrBaixavel{}, false
}

// ObterMotorBaixavel retorna o descritor de um motor do catálogo pelo nome (ok=false se não existir).
func ObterMotorBaixavel(nome string) (MotorOcrBaixavel, bool) {
	m, ok := MotoresBaixaveis[nome]
	return m, ok
}
