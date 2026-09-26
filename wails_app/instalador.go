package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"wails_app/config"
	"wails_app/dicionario"
	"wails_app/ocr"
	"wails_app/stt"
	"wails_app/tts"
)

// ----- Escolha de motores feita na tela custom do instalador (ver nsis-instalador/project.nsi) -----
// O instalador NÃO embute nenhum motor: ele só grava QUAL motor de OCR/voz o usuário escolheu, e o
// app baixa sozinho esse motor no primeiro start pelo mecanismo de download-sob-demanda já existente
// (bootstrapMotorPadrao, em motores.go). Builds de dev (sem instalador) simplesmente não encontram o
// marcador e seguem com o comportamento padrão de sempre (RapidOCR).

// escolhaInstalador espelha o JSON gravado pela seção de instalação do NSIS (uma página de escolha
// por família de motor: OCR, TTS e STT).
type escolhaInstalador struct {
	MotorOcr       string `json:"motorOcr"`
	MotorTts       string `json:"motorTts"`
	MotorStt       string `json:"motorStt"`
	IdiomaTraducao string `json:"idiomaTraducao"`
}

// caminhoEscolhaInstalador é o marcador gravado pelo instalador na pasta de dados do usuário
// (%APPDATA%\HanziTracker, a mesma das configurações). Não fica ao lado do executável nem em
// C:\ProgramData: o instalador roda como admin e o app não, e nessas pastas o app não teria
// permissão de apagar o marcador — ele seria reaplicado a cada início, desfazendo as trocas da UI.
func caminhoEscolhaInstalador() (string, error) {
	caminhoConfig, err := config.GetConfigPath()
	if err != nil {
		return "", err
	}
	return filepath.Join(filepath.Dir(caminhoConfig), "instalador_escolha.json"), nil
}

// aplicarEscolhaDoInstalador lê o arquivo gravado pelo instalador e, se válido, aplica como config
// de motor ativa no startup. Remove o arquivo para não re-gravar a cada início.
func (a *App) aplicarEscolhaDoInstalador() {
	caminho, err := caminhoEscolhaInstalador()
	if err != nil {
		return
	}
	dados, err := os.ReadFile(caminho)
	if err != nil {
		return // sem escolha do instalador (build de dev, ou já processado)
	}
	defer os.Remove(caminho)

	var escolha escolhaInstalador
	if err := json.Unmarshal(dados, &escolha); err != nil {
		fmt.Printf("Aviso: falha ao decodificar a escolha do instalador: %v\n", err)
		return
	}

	mudou := false

	// Idioma das traduções escolhido na instalação. Só aplica se for um idioma suportado; inválido/
	// vazio deixa o padrão (inglês, o fallback) intacto.
	if dicionario.IdiomaValido(escolha.IdiomaTraducao) {
		a.Config.IdiomaTraducao = escolha.IdiomaTraducao
		mudou = true
	}

	if _, ok := ocr.ObterMotorBaixavel(escolha.MotorOcr); ok {
		a.Config.MotorOcrAtivo = escolha.MotorOcr
		mudou = true
	}

	// TTS é sempre aplicado (mesmo vazio): "" representa a escolha explícita de "nenhum agora",
	// diferente do padrão silencioso de DefaultConfig — respeita a decisão do usuário na instalação.
	if escolha.MotorTts == "" {
		a.Config.MotorTtsAtivo = ""
		mudou = true
	} else if _, ok := tts.ObterMotorTtsBaixavel(escolha.MotorTts); ok {
		a.Config.MotorTtsAtivo = escolha.MotorTts
		mudou = true
	}

	// STT segue a mesma regra do TTS: "" = escolha explícita de "nenhum agora" (revisão de
	// pronúncia desligada até o usuário escolher um motor em Configurações → Motores).
	if escolha.MotorStt == "" {
		a.Config.MotorSttAtivo = ""
		mudou = true
	} else if _, ok := stt.ObterMotorSttBaixavel(escolha.MotorStt); ok {
		a.Config.MotorSttAtivo = escolha.MotorStt
		mudou = true
	}

	if mudou {
		if err := config.SaveConfig(a.Config); err != nil {
			fmt.Printf("Aviso: falha ao salvar a escolha de motores do instalador: %v\n", err)
		}
	}
}
