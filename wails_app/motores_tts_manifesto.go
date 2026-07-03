package main

// ----- Manifesto de Motores de TTS (sidecars baixáveis) -----
// Catálogo dos MOTORES de voz (leitura do pinyin em voz alta), publicados como .zip em GitHub
// Releases deste repo e baixados sob demanda pelo Go para %APPDATA%\HanziTracker\motores_tts\.
// Espelha o motores_manifesto.go dos motores de OCR; reaproveita ArtefatoBaixavel de lá.
//
// Diferença para os pesos: os sidecars de TTS NÃO usam um ModelosManifesto.py — os pesos são
// baixados pelo PRÓPRIO sidecar do Hugging Face na primeira síntese, para modelos\<Motor>\hf
// (cache HF redirecionado pelo entry de cada motor). Aqui só ficam os executáveis congelados.
//
// O `Nome` de cada entrada é o NOME DE CATÁLOGO (chave; injetado no sidecar via HANZITRACKER_MOTOR)
// e precisa casar com os valores do select "Motor de TTS" (Config.MotorTtsAtivo) e com o
// os.environ.setdefault dos entries Python (kokoro_server.py / chattts_server.py).

// MotorTtsBaixavel é uma entrada do catálogo de motores de voz: metadados para a UI + o artefato
// a baixar e o caminho do .exe dentro do zip já extraído.
type MotorTtsBaixavel struct {
	Nome       string           `json:"nome"`       // chave única (ex.: "Kokoro-82M")
	Rotulo     string           `json:"rotulo"`     // rótulo humano para a UI
	Descricao  string           `json:"descricao"`  //
	Versao     string           `json:"versao"`     // versão do motor (semver)
	Requisitos string           `json:"requisitos"` // "" = nenhum; ex.: "baixa ~1 GB de pesos no primeiro uso"
	Executavel string           `json:"executavel"` // .exe relativo à pasta de extração (o zip traz o exe na raiz)
	Artefato   ArtefatoBaixavel `json:"artefato"`   //
}

// MotoresTtsBaixaveis é o catálogo de MOTORES de voz publicados (release motores-tts-v1):
//   - Kokoro-82M: leve (~82M parâmetros), síntese rápida em CPU, vozes de mandarim de boa
//     qualidade. Baixa ~330 MB de pesos do Hugging Face no primeiro uso.
//   - ChatTTS: prosódia conversacional mais natural, porém mais pesado (~1 GB de pesos, síntese
//     mais lenta em CPU).
//
// ATENÇÃO (publicação pendente): o Sha256/TamanhoBytes de cada artefato só existem depois que a
// release motores-tts-v1 for publicada (empurre a tag; o CI congela os zips e imprime os hashes no
// resumo do job — ver docs/PUBLICAR-MOTORES.md). Enquanto Sha256 == "", o download é RECUSADO por
// baixarEExtrairArtefato (sha256 é obrigatório para executáveis) — preencha os dois campos com os
// valores do resumo do CI, como foi feito no motores_manifesto.go para o motores-v4.
var MotoresTtsBaixaveis = map[string]MotorTtsBaixavel{
	"Kokoro-82M": {
		Nome:   "Kokoro-82M",
		Rotulo: "Kokoro-82M (Leve)",
		Descricao: "Motor de voz leve e rápido em CPU, com vozes de mandarim de boa qualidade. " +
			"Baixa ~330 MB de pesos do Hugging Face no primeiro uso.",
		Versao:     "1.0.0",
		Requisitos: "Baixa ~330 MB de pesos na primeira leitura em voz alta.",
		Executavel: "kokoro_server.exe",
		Artefato: ArtefatoBaixavel{
			Nome:         "kokoro_server.zip",
			Url:          _baseReleaseMotores + "/motores-tts-v1/kokoro_server.zip",
			Sha256:       "a17c544ad7d86f2252a415b46759faa44b81401d9d9aba694f69bdc0955b6cb1",
			TamanhoBytes: 300637452,
		},
	},
	"ChatTTS": {
		Nome:   "ChatTTS",
		Rotulo: "ChatTTS (Natural, Pesado)",
		Descricao: "Motor de voz com prosódia conversacional mais natural, porém mais pesado. " +
			"Baixa ~1 GB de pesos do Hugging Face no primeiro uso; síntese mais lenta em CPU.",
		Versao:     "1.0.0",
		Requisitos: "Baixa ~1 GB de pesos na primeira leitura em voz alta.",
		Executavel: "chattts_server.exe",
		Artefato: ArtefatoBaixavel{
			Nome:         "chattts_server.zip",
			Url:          _baseReleaseMotores + "/motores-tts-v1/chattts_server.zip",
			Sha256:       "9d53d3ab939a6c3a6210b76c6c6d4ef4b00d2185c02e2f66c860f07e80815408",
			TamanhoBytes: 280536245,
		},
	},
}

// ObterMotorTtsBaixavel retorna o descritor de um motor de voz do catálogo pelo nome (ok=false se
// não existir).
func ObterMotorTtsBaixavel(nome string) (MotorTtsBaixavel, bool) {
	m, ok := MotoresTtsBaixaveis[nome]
	return m, ok
}
