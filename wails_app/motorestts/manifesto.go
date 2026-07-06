package motorestts

import (
	_ "embed"
	"encoding/json"
	"runtime"

	"wails_app/baixador"
)

// ----- Manifesto de Motores de TTS (sidecars baixáveis) -----
// Catálogo dos MOTORES de voz (leitura do pinyin em voz alta), publicados como .zip em GitHub
// Releases deste repo e baixados sob demanda pelo Go para %APPDATA%\HanziTracker\motores_tts\.
// Espelha o manifesto dos motores de OCR (motoresocr); reaproveita baixador.ArtefatoBaixavel.
//
// Diferença para os pesos: os sidecars de TTS NÃO usam um ModelosManifesto.py — os pesos são
// baixados pelo PRÓPRIO sidecar do Hugging Face na primeira síntese, para modelos\<Motor>\hf
// (cache HF redirecionado pelo entry de cada motor). Aqui só ficam os executáveis congelados.
//
// O `Nome` de cada entrada é o NOME DE CATÁLOGO (chave; injetado no sidecar via HANZITRACKER_MOTOR)
// e precisa casar com os valores do select "Motor de TTS" (Config.MotorTtsAtivo) e com o
// os.environ.setdefault dos entries Python (kokoro_server.py / chattts_server.py).

// MotorTtsBaixavel é uma entrada do catálogo de motores de voz: metadados para a UI + o artefato
// a baixar e o caminho do executável dentro do zip já extraído.
type MotorTtsBaixavel struct {
	Nome       string                    `json:"nome"`       // chave única (ex.: "Kokoro-82M")
	Rotulo     string                    `json:"rotulo"`     // rótulo humano para a UI
	Descricao  string                    `json:"descricao"`  //
	Versao     string                    `json:"versao"`     // versão do motor (semver)
	Requisitos string                    `json:"requisitos"` // "" = nenhum; ex.: "baixa ~1 GB de pesos no primeiro uso"
	Executavel string                    `json:"executavel"` // executável relativo à pasta de extração, na raiz do zip (".exe" no Windows; sem sufixo no Linux)
	Artefato   baixador.ArtefatoBaixavel `json:"artefato"`   //
}

// MotoresTtsBaixaveis é o catálogo de MOTORES de voz publicados (a tag/sha/tamanho de cada .zip vêm
// de artefatos_tts.json, injetados por init — ver a seção no fim do arquivo):
//   - Kokoro-82M: leve (~82M parâmetros), síntese rápida em CPU, vozes de mandarim de boa
//     qualidade. Baixa ~330 MB de pesos do Hugging Face no primeiro uso.
//   - ChatTTS: prosódia conversacional mais natural, porém mais pesado (~1 GB de pesos, síntese
//     mais lenta em CPU).
var MotoresTtsBaixaveis = map[string]MotorTtsBaixavel{
	"Kokoro-82M": {
		Nome:   "Kokoro-82M",
		Rotulo: "Kokoro-82M (Leve)",
		Descricao: "Motor de voz leve e rápido em CPU, com vozes de mandarim de boa qualidade. " +
			"Baixa ~330 MB de pesos do Hugging Face no primeiro uso.",
		Versao:     "1.0.0",
		Requisitos: "Baixa ~330 MB de pesos na primeira leitura em voz alta.",
		Executavel: baixador.NomeExecutavelSo("kokoro_server"),
		// Url/Sha256/TamanhoBytes são injetados por init() a partir do artefatos_tts*.json do SO (ver abaixo).
		Artefato: baixador.ArtefatoBaixavel{Nome: baixador.NomeZipArtefatoSo("kokoro_server")},
	},
	"ChatTTS": {
		Nome:   "ChatTTS",
		Rotulo: "ChatTTS (Natural, Pesado)",
		Descricao: "Motor de voz com prosódia conversacional mais natural, porém mais pesado. " +
			"Baixa ~1 GB de pesos do Hugging Face no primeiro uso; síntese mais lenta em CPU.",
		Versao:     "1.0.0",
		Requisitos: "Baixa ~1 GB de pesos na primeira leitura em voz alta.",
		Executavel: baixador.NomeExecutavelSo("chattts_server"),
		Artefato:   baixador.ArtefatoBaixavel{Nome: baixador.NomeZipArtefatoSo("chattts_server")},
	},
}

// ----- Injeção dos campos voláteis (tag/sha256/tamanho) a partir de artefatos_tts*.json -----
// Espelha o mecanismo do manifesto de OCR (motoresocr): o que MUDA a cada release — a tag na URL, o
// sha256 e o tamanho de cada zip — vive em UM JSON POR SO, embutido via go:embed. O workflow
// publicar-motores-tts-windows.yml (Windows) reescreve artefatos_tts.json e o publicar-motores-tts-linux.yml
// reescreve artefatos_tts_linux.json, cada um commitando o seu — assim uma release de motores de um
// SO nunca invalida as URLs do outro. Ver docs/PUBLICAR-MOTORES.md.

//go:embed artefatos_tts.json
var artefatosTtsWindows []byte

//go:embed artefatos_tts_linux.json
var artefatosTtsLinux []byte

// artefatosTtsBrutos é o manifesto do SO atual (ambos são embutidos; a escolha é por runtime.GOOS,
// sem build tags — o binário já é por-SO de qualquer forma, e os testes validam o JSON do runner).
var artefatosTtsBrutos = escolherArtefatosTts()

func escolherArtefatosTts() []byte {
	if runtime.GOOS == "windows" {
		return artefatosTtsWindows
	}
	return artefatosTtsLinux
}

// dadosArtefatoTts é o par volátil (sha256 + tamanho) de um zip, chaveado pelo nome do arquivo.
type dadosArtefatoTts struct {
	Sha256       string `json:"sha256"`
	TamanhoBytes int64  `json:"tamanhoBytes"`
}

// manifestoArtefatosTts é o conteúdo de artefatos_tts.json: a tag da release + os dados por zip.
type manifestoArtefatosTts struct {
	Tag       string                      `json:"tag"`
	Artefatos map[string]dadosArtefatoTts `json:"artefatos"`
}

// init injeta url (derivada da tag), sha256 e tamanho em cada entrada de MotoresTtsBaixaveis a partir
// do JSON embutido. JSON malformado ou sem tag é bug de publicação (pego pelos testes): falha com
// panic. Um sha256 vazio é aceito aqui (estado pré-publicação) — o download é recusado em runtime.
func init() {
	var dados manifestoArtefatosTts
	if err := json.Unmarshal(artefatosTtsBrutos, &dados); err != nil {
		panic("motorestts: artefatos_tts.json inválido: " + err.Error())
	}
	if dados.Tag == "" {
		panic("motorestts: artefatos_tts.json sem tag de release")
	}
	for chave, motor := range MotoresTtsBaixaveis {
		art := dados.Artefatos[motor.Artefato.Nome]
		motor.Artefato.Url = baixador.BaseReleaseMotores + "/" + dados.Tag + "/" + motor.Artefato.Nome
		motor.Artefato.Sha256 = art.Sha256
		motor.Artefato.TamanhoBytes = art.TamanhoBytes
		MotoresTtsBaixaveis[chave] = motor
	}
}

// ObterMotorTtsBaixavel retorna o descritor de um motor de voz do catálogo pelo nome (ok=false se
// não existir).
func ObterMotorTtsBaixavel(nome string) (MotorTtsBaixavel, bool) {
	m, ok := MotoresTtsBaixaveis[nome]
	return m, ok
}
