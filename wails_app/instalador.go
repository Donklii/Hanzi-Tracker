package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"wails_app/config"
	"wails_app/motoresocr"
	"wails_app/motorestts"
)

// ----- Escolha de motores feita na tela custom do instalador (ver nsis-instalador/project.nsi) -----
// O instalador NÃO embute nenhum motor: ele só grava QUAL motor de OCR/voz o usuário escolheu, e o
// app baixa sozinho esse motor no primeiro start pelo mecanismo de download-sob-demanda já existente
// (bootstrapMotorPadrao, em motores.go). Builds de dev (sem instalador) simplesmente não encontram o
// marcador e seguem com o comportamento padrão de sempre (RapidOCR).

// escolhaInstalador espelha o JSON gravado pela seção de instalação do NSIS.
type escolhaInstalador struct {
	MotorOcr string `json:"motorOcr"`
	MotorTts string `json:"motorTts"`
}

// caminhoEscolhaInstalador é o marcador escrito pelo instalador — removido após ser aplicado.
func caminhoEscolhaInstalador() (string, error) {
	appData, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(appData, "HanziTracker", "instalador_escolha.json"), nil
}

// aplicarEscolhaDoInstalador lê o marcador do instalador (se existir), grava a escolha na Config e
// apaga o marcador — deve rodar uma única vez, no startup, ANTES de resolver/bootstrapar o motor de
// OCR. Sem marcador (build de dev, ou já aplicado antes), é no-op silencioso.
func (a *App) aplicarEscolhaDoInstalador() {
	caminho, err := caminhoEscolhaInstalador()
	if err != nil {
		return
	}

	dados, err := os.ReadFile(caminho)
	if err != nil {
		return // sem marcador — nada a aplicar
	}
	defer os.Remove(caminho)

	var escolha escolhaInstalador
	if err := json.Unmarshal(dados, &escolha); err != nil {
		return
	}

	mudou := false
	if _, ok := motoresocr.ObterMotorBaixavel(escolha.MotorOcr); ok {
		a.Config.MotorOcrAtivo = escolha.MotorOcr
		mudou = true
	}

	// TTS é sempre aplicado (mesmo vazio): "" representa a escolha explícita de "nenhum agora",
	// diferente do padrão silencioso de DefaultConfig — respeita a decisão do usuário na instalação.
	if escolha.MotorTts == "" {
		a.Config.MotorTtsAtivo = ""
		mudou = true
	} else if _, ok := motorestts.ObterMotorTtsBaixavel(escolha.MotorTts); ok {
		a.Config.MotorTtsAtivo = escolha.MotorTts
		mudou = true
	}

	if mudou {
		if err := config.SaveConfig(a.Config); err != nil {
			fmt.Printf("Aviso: falha ao salvar a escolha de motores do instalador: %v\n", err)
		}
	}
}
