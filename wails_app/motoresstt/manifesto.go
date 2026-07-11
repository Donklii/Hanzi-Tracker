package motoresstt

import (
	_ "embed"
	"encoding/json"
	"runtime"

	"wails_app/baixador"
)

// ----- Manifesto de Motores de STT (sidecars baixáveis) -----
// Catálogo dos MOTORES de reconhecimento de fala (revisão de pronúncia), publicados como .zip em
// GitHub Releases deste repo e baixados sob demanda pelo Go para %APPDATA%\HanziTracker\motores_stt\.
// Espelha o manifesto dos motores de voz (motorestts); reaproveita baixador.ArtefatoBaixavel.
//
// Como no TTS, os sidecars de STT NÃO usam um ModelosManifesto.py — os pesos são baixados pelo
// PRÓPRIO sidecar do Hugging Face na primeira transcrição (ou no /api/stt/preparar), para
// modelos\<Motor>\hf (cache HF redirecionado pelo entry de cada motor). Aqui só ficam os
// executáveis congelados.
//
// O `Nome` de cada entrada é o NOME DE CATÁLOGO (chave; injetado no sidecar via HANZITRACKER_MOTOR)
// e precisa casar com Config.MotorSttAtivo e com o os.environ.setdefault do entry Python
// (paraformer_server.py).

// MotorSttBaixavel é uma entrada do catálogo de motores de STT: metadados para a UI + o artefato
// a baixar e o caminho do executável dentro do zip já extraído.
type MotorSttBaixavel struct {
	Nome       string                    `json:"nome"`       // chave única (ex.: "Paraformer-ZH")
	Rotulo     string                    `json:"rotulo"`     // rótulo humano para a UI
	Descricao  string                    `json:"descricao"`  //
	Versao     string                    `json:"versao"`     // versão do motor (semver)
	Requisitos string                    `json:"requisitos"` // "" = nenhum; ex.: "baixa ~240 MB de pesos no primeiro uso"
	Executavel string                    `json:"executavel"` // executável relativo à pasta de extração, na raiz do zip (".exe" no Windows; sem sufixo no Linux)
	Artefato   baixador.ArtefatoBaixavel `json:"artefato"`   //
}

// MotoresSttBaixaveis é o catálogo de MOTORES de STT publicados (a tag/sha/tamanho de cada .zip
// vêm de artefatos_stt*.json, injetados por init — ver a seção no fim do arquivo):
//   - Paraformer-ZH: reconhecimento de mandarim do FunASR rodando em onnxruntime via sherpa-onnx —
//     preciso em frases e em caracteres isolados, rápido em CPU e sem torch (sidecar leve). Também
//     é ele quem CAPTURA o microfone (a webview não tem acesso ao microfone no Linux). É offline
//     (não-streaming): os parciais em tempo real saem re-decodificando o áudio acumulado a cada
//     consulta de /api/stt/parcial.
//   - Zipformer-ZH-Streaming: reconhecimento de mandarim GENUINAMENTE streaming (transducer do
//     sherpa-onnx) — o fluxo de decodificação mantém estado e cada parcial custa só o trecho novo
//     de áudio, com latência menor e CPU constante. Em troca, tende a ser menos preciso que o
//     Paraformer, principalmente em caracteres isolados.
var MotoresSttBaixaveis = map[string]MotorSttBaixavel{
	"Paraformer-ZH": {
		Nome:   "Paraformer-ZH",
		Rotulo: "Paraformer-ZH (Mandarim)",
		Descricao: "Motor de reconhecimento de fala em mandarim (FunASR/sherpa-onnx), rápido em CPU e o mais preciso do catálogo — inclusive em caracteres isolados. " +
			"Baixa ~240 MB de pesos do Hugging Face no primeiro uso.",
		Versao:     "1.0.0",
		Requisitos: "Baixa ~240 MB de pesos na primeira transcrição.",
		Executavel: baixador.NomeExecutavelSo("paraformer_server"),
		// Url/Sha256/TamanhoBytes são injetados por init() a partir do artefatos_stt*.json do SO (ver abaixo).
		Artefato: baixador.ArtefatoBaixavel{Nome: baixador.NomeZipArtefatoSo("paraformer_server")},
	},
	"Zipformer-ZH-Streaming": {
		Nome:   "Zipformer-ZH-Streaming",
		Rotulo: "Zipformer-ZH Streaming (Mandarim, tempo real)",
		Descricao: "Motor de reconhecimento de fala em mandarim genuinamente streaming (sherpa-onnx): a transcrição parcial aparece com latência mínima enquanto você fala. " +
			"Menos preciso que o Paraformer-ZH em caracteres isolados. Baixa ~50 MB de pesos do Hugging Face no primeiro uso.",
		Versao:     "1.0.0",
		Requisitos: "Baixa ~50 MB de pesos na primeira transcrição.",
		Executavel: baixador.NomeExecutavelSo("zipformer_streaming_server"),
		Artefato:   baixador.ArtefatoBaixavel{Nome: baixador.NomeZipArtefatoSo("zipformer_streaming_server")},
	},
}

// ----- Injeção dos campos voláteis (tag/sha256/tamanho) a partir de artefatos_stt*.json -----
// Espelha o mecanismo dos manifestos de OCR e TTS: o que MUDA a cada release — a tag na URL, o
// sha256 e o tamanho de cada zip — vive em UM JSON POR SO, embutido via go:embed. O workflow
// publicar-motores-stt-windows.yml (Windows) reescreve artefatos_stt.json e o
// publicar-motores-stt-linux.yml reescreve artefatos_stt_linux.json, cada um commitando o seu —
// assim uma release de motores de um SO nunca invalida as URLs do outro. Ver docs/PUBLICAR-MOTORES.md.

//go:embed artefatos_stt.json
var artefatosSttWindows []byte

//go:embed artefatos_stt_linux.json
var artefatosSttLinux []byte

// artefatosSttBrutos é o manifesto do SO atual (ambos são embutidos; a escolha é por runtime.GOOS,
// sem build tags — o binário já é por-SO de qualquer forma, e os testes validam o JSON do runner).
var artefatosSttBrutos = escolherArtefatosStt()

func escolherArtefatosStt() []byte {
	if runtime.GOOS == "windows" {
		return artefatosSttWindows
	}
	return artefatosSttLinux
}

// dadosArtefatoStt é o par volátil (sha256 + tamanho) de um zip, chaveado pelo nome do arquivo.
type dadosArtefatoStt struct {
	Sha256       string `json:"sha256"`
	TamanhoBytes int64  `json:"tamanhoBytes"`
}

// manifestoArtefatosStt é o conteúdo de artefatos_stt*.json: a tag da release + os dados por zip.
type manifestoArtefatosStt struct {
	Tag       string                      `json:"tag"`
	Artefatos map[string]dadosArtefatoStt `json:"artefatos"`
}

// init injeta url (derivada da tag), sha256 e tamanho em cada entrada de MotoresSttBaixaveis a
// partir do JSON embutido. JSON malformado ou sem tag é bug de publicação (pego pelos testes):
// falha com panic. Um sha256 vazio é aceito aqui (estado pré-publicação) — o download é recusado
// em runtime.
func init() {
	var dados manifestoArtefatosStt
	if err := json.Unmarshal(artefatosSttBrutos, &dados); err != nil {
		panic("motoresstt: artefatos_stt.json inválido: " + err.Error())
	}
	if dados.Tag == "" {
		panic("motoresstt: artefatos_stt.json sem tag de release")
	}
	for chave, motor := range MotoresSttBaixaveis {
		art := dados.Artefatos[motor.Artefato.Nome]
		motor.Artefato.Url = baixador.BaseReleaseMotores + "/" + dados.Tag + "/" + motor.Artefato.Nome
		motor.Artefato.Sha256 = art.Sha256
		motor.Artefato.TamanhoBytes = art.TamanhoBytes
		MotoresSttBaixaveis[chave] = motor
	}
}

// ObterMotorSttBaixavel retorna o descritor de um motor de STT do catálogo pelo nome (ok=false se
// não existir).
func ObterMotorSttBaixavel(nome string) (MotorSttBaixavel, bool) {
	m, ok := MotoresSttBaixaveis[nome]
	return m, ok
}
