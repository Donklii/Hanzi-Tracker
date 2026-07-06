package motoresocr

import (
	_ "embed"
	"encoding/json"
	"runtime"

	"wails_app/baixador"
)

// ----- Manifesto de Motores de OCR (sidecars baixáveis) -----
// Catálogo dos MOTORES de OCR e do componente de overlay compartilhado, publicados como .zip em
// GitHub Releases deste repo e baixados sob demanda pelo Go para %APPDATA%\HanziTracker\motores_ocr\.
//
// Diferença para motores/<motor>/ModelosManifesto.py (PESOS, ex.: .onnx/.traineddata/.pth): lá são
// apenas os arquivos que o motor LÊ; aqui são os PRÓPRIOS executáveis do motor (server.py e afins,
// congelados por PyInstaller). Este catálogo vive
// no GO — e não no Python — porque o app precisa baixar o motor padrão ANTES de qualquer processo
// Python existir (bootstrap de first-run, Fase 5). O `Sha256` é OBRIGATÓRIO para binários: BaixarArquivo
// confere o hash após o download e recusa se divergir. Ver docs/PUBLICAR-MOTORES.md e Fase 5 no TODO.md.

// MotorOcrBaixavel é uma entrada do catálogo de motores: metadados para a UI "Gerenciar Motores"
// (Passo 6) + o artefato a baixar e o caminho do executável dentro do zip já extraído.
type MotorOcrBaixavel struct {
	Nome       string                    `json:"nome"`       // chave única / rótulo curto (ex.: "RapidOCR")
	Rotulo     string                    `json:"rotulo"`     // rótulo humano para a UI
	Descricao  string                    `json:"descricao"`  //
	Idiomas    []string                  `json:"idiomas"`    // códigos ISO (ex.: ["zh", "en"])
	Versao     string                    `json:"versao"`     // versão do motor (semver)
	Variante   string                    `json:"variante"`   // aceleração: "CPU", "DirectML", "CUDA" (ou combinação)
	Requisitos string                    `json:"requisitos"` // "" = nenhum; ex.: "GPU Nvidia + drivers CUDA"
	Padrao     bool                      `json:"padrao"`     // baixado automaticamente no bootstrap de first-run?
	Executavel string                    `json:"executavel"` // executável relativo à pasta de extração, na raiz do zip (".exe" no Windows; sem sufixo no Linux)
	Artefato   baixador.ArtefatoBaixavel `json:"artefato"`   //
}

// MotoresBaixaveis é o catálogo de MOTORES de OCR publicados (a tag/sha/tamanho de cada .zip vêm de
// artefatos_ocr.json, injetados por init — ver a seção no fim do arquivo):
//   - RapidOCR (padrão): sidecar congelado com DirectML embutido + fallback automático para CPU
//     (cobre Nvidia/AMD/Intel sem CUDA Toolkit). Já traz os pesos mobile embutidos — OCR sem download.
//   - Tesseract: dirige o tesseract.exe empacotado; já vem com tessdata_fast (chi_sim+eng) embutido,
//     e o tessdata_best é baixável em "Gerenciar Modelos". Só CPU.
//   - EasyOCR: PyTorch (CPU). NÃO embute pesos — exige baixar o modelo antes do primeiro uso.
//
// O `Nome` de cada entrada é o NOME DE CATÁLOGO (chave; injetado no sidecar via HANZITRACKER_MOTOR) e
// define a subpasta de pesos motores_ocr\<Nome>\modelos — precisa casar com os ModelosManifesto.py de cada motor.
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
		Executavel: baixador.NomeExecutavelSo("ocr_server"),
		// Url/Sha256/TamanhoBytes são injetados por init() a partir do artefatos_ocr*.json do SO (ver abaixo).
		Artefato: baixador.ArtefatoBaixavel{Nome: baixador.NomeZipArtefatoSo("ocr_server")},
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
		Executavel: baixador.NomeExecutavelSo("tesseract_server"),
		Artefato:   baixador.ArtefatoBaixavel{Nome: baixador.NomeZipArtefatoSo("tesseract_server")},
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
		Executavel: baixador.NomeExecutavelSo("easyocr_server"),
		Artefato:   baixador.ArtefatoBaixavel{Nome: baixador.NomeZipArtefatoSo("easyocr_server")},
	},
}

// ----- Injeção dos campos voláteis (tag/sha256/tamanho) a partir de artefatos_ocr*.json -----
// O que MUDA a cada release — a tag embutida na URL, o sha256 e o tamanho de cada zip — NÃO fica
// hardcoded no catálogo acima: vive em UM JSON POR SO, embutido no binário via go:embed. O workflow
// publicar-motores-ocr-windows.yml (Windows) reescreve artefatos_ocr.json e o publicar-motores-ocr-linux.yml
// reescreve artefatos_ocr_linux.json, cada um commitando o seu — assim uma release de motores de um
// SO nunca invalida as URLs do outro. Ver docs/PUBLICAR-MOTORES.md.

//go:embed artefatos_ocr.json
var artefatosOcrWindows []byte

//go:embed artefatos_ocr_linux.json
var artefatosOcrLinux []byte

// artefatosOcrBrutos é o manifesto do SO atual (ambos são embutidos; a escolha é por runtime.GOOS,
// sem build tags — o binário já é por-SO de qualquer forma, e os testes validam o JSON do runner).
var artefatosOcrBrutos = escolherArtefatosOcr()

func escolherArtefatosOcr() []byte {
	if runtime.GOOS == "windows" {
		return artefatosOcrWindows
	}
	return artefatosOcrLinux
}

// dadosArtefatoOcr é o par volátil (sha256 + tamanho) de um zip, chaveado pelo nome do arquivo.
type dadosArtefatoOcr struct {
	Sha256       string `json:"sha256"`
	TamanhoBytes int64  `json:"tamanhoBytes"`
}

// manifestoArtefatosOcr é o conteúdo de artefatos_ocr.json: a tag da release + os dados por zip.
type manifestoArtefatosOcr struct {
	Tag       string                      `json:"tag"`
	Artefatos map[string]dadosArtefatoOcr `json:"artefatos"`
}

// init injeta url (derivada da tag), sha256 e tamanho em cada entrada de MotoresBaixaveis a partir do
// JSON embutido. Um JSON malformado ou sem tag é bug de publicação (dado embutido no binário, pego
// pelos testes antes de qualquer release): falha alto com panic em vez de gerar URLs quebradas. Um
// sha256 vazio é aceito (estado pré-publicação do SO) — o download é recusado em runtime.
func init() {
	// O sidecar Tesseract empacota o tesseract.exe da instalação Windows (choco) — ainda não há build
	// Linux dele, então fora do Windows ele sai do catálogo (a UI nem o oferece).
	if runtime.GOOS != "windows" {
		delete(MotoresBaixaveis, "Tesseract")
	}

	var dados manifestoArtefatosOcr
	if err := json.Unmarshal(artefatosOcrBrutos, &dados); err != nil {
		panic("motoresocr: artefatos_ocr*.json inválido: " + err.Error())
	}
	if dados.Tag == "" {
		panic("motoresocr: artefatos_ocr*.json sem tag de release")
	}
	for chave, motor := range MotoresBaixaveis {
		art := dados.Artefatos[motor.Artefato.Nome]
		motor.Artefato.Url = baixador.BaseReleaseMotores + "/" + dados.Tag + "/" + motor.Artefato.Nome
		motor.Artefato.Sha256 = art.Sha256
		motor.Artefato.TamanhoBytes = art.TamanhoBytes
		MotoresBaixaveis[chave] = motor
	}
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
