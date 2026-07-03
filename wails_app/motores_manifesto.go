package main

// ----- Manifesto de Motores de OCR (sidecars baixáveis) -----
// Catálogo dos MOTORES de OCR e do componente de overlay compartilhado, publicados como .zip em
// GitHub Releases deste repo e baixados sob demanda pelo Go para %APPDATA%\HanziTracker\motores\.
//
// Diferença para motores/<motor>/ModelosManifesto.py (PESOS, ex.: .onnx/.traineddata/.pth): lá são
// apenas os arquivos que o motor LÊ; aqui são os PRÓPRIOS executáveis do motor (server.py e afins,
// congelados por PyInstaller). Este catálogo vive
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

// MotoresBaixaveis é o catálogo de MOTORES de OCR publicados (release motores-v4):
//   - RapidOCR (padrão): sidecar congelado com DirectML embutido + fallback automático para CPU
//     (cobre Nvidia/AMD/Intel sem CUDA Toolkit). Já traz os pesos mobile embutidos — OCR sem download.
//   - Tesseract: dirige o tesseract.exe empacotado; já vem com tessdata_fast (chi_sim+eng) embutido,
//     e o tessdata_best é baixável em "Gerenciar Modelos". Só CPU.
//   - EasyOCR: PyTorch (CPU). NÃO embute pesos — exige baixar o modelo antes do primeiro uso.
//
// O `Nome` de cada entrada é o NOME DE CATÁLOGO (chave; injetado no sidecar via HANZITRACKER_MOTOR) e
// define a subpasta de pesos modelos\<Nome> — precisa casar com os ModelosManifesto.py de cada motor.
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
			Sha256:       "290de20931bf8cc4c9a4887685df1884f468a8ba79e5fea626037ae555054875",
			TamanhoBytes: 126066360,
		},
	},
	"Tesseract": {
		Nome:   "Tesseract",
		Rotulo: "Tesseract",
		Descricao: "Motor Tesseract (CPU): dirige o tesseract.exe empacotado. Já vem com os pesos rápidos " +
			"(tessdata_fast, chi_sim+eng) — o tessdata_best, mais preciso, é baixável em Gerenciar Modelos.",
		Idiomas:    []string{"zh", "en"},
		Versao:     "1.0.0",
		Variante:   "CPU",
		Requisitos: "",
		Padrao:     false,
		Executavel: "tesseract_server.exe",
		Artefato: ArtefatoBaixavel{
			Nome:         "tesseract_server.zip",
			Url:          _baseReleaseMotores + "/motores-v1/tesseract_server.zip",
			Sha256:       "3904412594afa69f7e4fab01af4142202c5761224ecf41778971e59ab0022b9f",
			TamanhoBytes: 104055930,
		},
	},
	"EasyOCR": {
		Nome:   "EasyOCR",
		Rotulo: "EasyOCR",
		Descricao: "Motor EasyOCR (PyTorch, CPU): detector CRAFT + reconhecedor zh_sim_g2. NÃO embute pesos — " +
			"baixe o modelo em Gerenciar Modelos antes do primeiro uso.",
		Idiomas:    []string{"zh", "en"},
		Versao:     "1.0.0",
		Variante:   "CPU",
		Requisitos: "Requer baixar o modelo (~93 MB) em Gerenciar Modelos antes do primeiro uso.",
		Padrao:     false,
		Executavel: "easyocr_server.exe",
		Artefato: ArtefatoBaixavel{
			Nome:         "easyocr_server.zip",
			Url:          _baseReleaseMotores + "/motores-v1/easyocr_server.zip",
			Sha256:       "90baf98b9f1314ca2bdaf3f5ef383a97aa0647579cc44b27938b57d5e43d20a4",
			TamanhoBytes: 253364151,
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
