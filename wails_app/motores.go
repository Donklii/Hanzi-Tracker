package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"
	"wails_app/armazenamento"
	"wails_app/baixador"
	"wails_app/config"
	"wails_app/motoresocr"
)

// ----- Ciclo de vida dos MOTORES de OCR (sidecars baixáveis) — Fase 5, Passo 5 -----
// Expõe ao frontend o catálogo e o ciclo de vida (download/extração/troca/bootstrap) donos do
// pacote motoresocr, em %APPDATA%\HanziTracker\motores_ocr\<Motor>\. Ver docs/PUBLICAR-MOTORES.md.

// emitirProgressoMotor envia um evento de progresso de download/instalação de motor ao frontend.
func (a *App) emitirProgressoMotor(nome, mensagem string) {
	if a.ctx == nil {
		return
	}
	runtime.EventsEmit(a.ctx, "motor_download_progresso", map[string]interface{}{"nome": nome, "mensagem": mensagem})
}

// nomeMotorAtivo devolve o NOME de catálogo do motor em execução ("" se nenhum). Versão nil-safe de
// CatalogoAtivo para os métodos do App que rodam antes/fora do startup.
func (a *App) nomeMotorAtivo() string {
	if a.motorOcr == nil {
		return ""
	}
	return a.motorOcr.CatalogoAtivo()
}

// ----- API exposta ao frontend (Passo 6 consome) -----

// MotorOcrInfo é o estado de um motor para a UI "Gerenciar Motores": catálogo + instalado/ativo.
// `publicado` espelha o conceito dos motores de voz: indica se o artefato deste SO já tem release
// (sha256 preenchido no artefatos_ocr*.json) — sem release, o download é recusado.
type MotorOcrInfo struct {
	Nome         string   `json:"nome"`
	Rotulo       string   `json:"rotulo"`
	Descricao    string   `json:"descricao"`
	Idiomas      []string `json:"idiomas"`
	Versao       string   `json:"versao"`
	Variante     string   `json:"variante"`
	Requisitos   string   `json:"requisitos"`
	Padrao       bool     `json:"padrao"`
	TamanhoBytes int64    `json:"tamanhoBytes"`
	Publicado    bool     `json:"publicado"`
	Instalado    bool     `json:"instalado"`
	Ativo        bool     `json:"ativo"`
}

// ListarMotores devolve o catálogo de motores com o estado instalado/ativo (ordenado por rótulo).
func (a *App) ListarMotores() []MotorOcrInfo {
	var ativoCmd string
	if a.motorOcr != nil {
		ativoCmd = filepath.Clean(a.motorOcr.ComandoAtivo())
	}

	lista := make([]MotorOcrInfo, 0, len(motoresocr.MotoresBaixaveis))
	for _, m := range motoresocr.MotoresBaixaveis {
		exe := motoresocr.CaminhoExecutavelMotor(m)
		instalado := false
		if info, err := os.Stat(exe); err == nil && !info.IsDir() {
			instalado = true
		}
		ativo := false
		if instalado && ativoCmd != "" && ativoCmd != "." {
			if abs, err := filepath.Abs(exe); err == nil && filepath.Clean(abs) == ativoCmd {
				ativo = true
			}
		}
		lista = append(lista, MotorOcrInfo{
			Nome:         m.Nome,
			Rotulo:       m.Rotulo,
			Descricao:    m.Descricao,
			Idiomas:      m.Idiomas,
			Versao:       m.Versao,
			Variante:     m.Variante,
			Requisitos:   m.Requisitos,
			Padrao:       m.Padrao,
			TamanhoBytes: m.Artefato.TamanhoBytes,
			Publicado:    m.Artefato.Sha256 != "",
			Instalado:    instalado,
			Ativo:        ativo,
		})
	}
	sort.Slice(lista, func(i, j int) bool { return lista[i].Rotulo < lista[j].Rotulo })
	return lista
}

// BaixarMotor baixa e instala um motor do catálogo no AppData (progresso via "motor_download_progresso").
func (a *App) BaixarMotor(nome string) error {
	m, ok := motoresocr.ObterMotorBaixavel(nome)
	if !ok {
		return fmt.Errorf("motor '%s' não encontrado no catálogo", nome)
	}
	// Guard clause: sha256 vazio = o zip deste SO ainda não tem release (ex.: Linux antes da primeira
	// release motores-ocr-linux-v*) — recusa antes de gastar centenas de MB num download inútil.
	if m.Artefato.Sha256 == "" {
		return fmt.Errorf("o motor '%s' ainda não foi publicado para este sistema operacional", m.Rotulo)
	}
	destino := motoresocr.PastaMotorOcr(m.Nome)
	if err := baixador.BaixarEExtrairArtefato(m.Artefato, destino, armazenamento.PastaDados(), func(msg string) { a.emitirProgressoMotor(m.Nome, msg) }); err != nil {
		a.emitirProgressoMotor(m.Nome, "⚠️ "+err.Error())
		return err
	}
	return nil
}

// RemoverMotor apaga a pasta de um motor instalado. Recusa remover o motor que está ATIVO (deixaria o
// app sem OCR no meio do uso) — troque para outro antes.
func (a *App) RemoverMotor(nome string) error {
	m, ok := motoresocr.ObterMotorBaixavel(nome)
	if !ok {
		return fmt.Errorf("motor '%s' não encontrado no catálogo", nome)
	}

	if a.motorOcr != nil {
		if abs, err := filepath.Abs(motoresocr.CaminhoExecutavelMotor(m)); err == nil {
			if filepath.Clean(abs) == filepath.Clean(a.motorOcr.ComandoAtivo()) {
				return fmt.Errorf("o motor '%s' está ativo; troque para outro motor antes de removê-lo", m.Rotulo)
			}
		}
	}

	pasta := motoresocr.PastaMotorOcr(m.Nome)
	if _, err := os.Stat(pasta); os.IsNotExist(err) {
		return nil // já removido
	}
	return os.RemoveAll(pasta)
}

// limparMotores remove os motores INATIVOS baixados, preservando o motor ativo (seu .exe está em uso)
// e o overlay compartilhado (`_overlay`, em uso enquanto o app roda). É o que a aba de Armazenamento
// chama ao "Limpar" a categoria de motores — libera espaço sem derrubar o OCR atual.
func (a *App) limparMotores() error {
	raiz := motoresocr.PastaMotoresOcr()
	entradas, err := os.ReadDir(raiz)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	ativo := ""
	if a.motorOcr != nil {
		ativo = filepath.Clean(a.motorOcr.ComandoAtivo())
	}

	var erros []string
	for _, e := range entradas {
		sub := filepath.Join(raiz, e.Name())
		// Preserva a subpasta que contém o executável do motor ativo.
		if ativo != "" && ativo != "." && strings.HasPrefix(ativo, filepath.Clean(sub)+string(os.PathSeparator)) {
			continue
		}
		if err := os.RemoveAll(sub); err != nil {
			erros = append(erros, fmt.Sprintf("%s: %v", e.Name(), err))
		}
	}
	if len(erros) > 0 {
		return fmt.Errorf("alguns motores não puderam ser removidos — %s", strings.Join(erros, "; "))
	}
	return nil
}

// TrocarMotor faz o hot-swap para um motor JÁ instalado e persiste a escolha (usada no próximo início).
func (a *App) TrocarMotor(nome string) error {
	m, ok := motoresocr.ObterMotorBaixavel(nome)
	if !ok {
		return fmt.Errorf("motor '%s' não encontrado no catálogo", nome)
	}
	desc, instalado := motoresocr.DescritorMotorInstalado(m)
	if !instalado {
		return fmt.Errorf("o motor '%s' não está instalado; baixe-o primeiro", m.Rotulo)
	}
	if a.motorOcr == nil {
		return fmt.Errorf("gerenciador de motor indisponível")
	}

	if err := a.motorOcr.Trocar(desc, 30*time.Second); err != nil {
		return err
	}

	// Persiste como motor ativo (startup usa Config.MotorOcrAtivo).
	a.Config.MotorOcrAtivo = m.Nome
	if err := config.SaveConfig(a.Config); err != nil {
		fmt.Printf("Aviso: falha ao salvar o motor ativo: %v\n", err)
	}
	return nil
}

// ----- Bootstrap de first-run -----

// bootstrapMotorPadrao roda no first-run (nenhum motor local/instalado): baixa o motor ESCOLHIDO + o
// overlay, sobe o motor e anuncia os eventos de estado. Deve rodar em uma goroutine (faz I/O de rede).
//
// O motor a baixar é a.Config.MotorOcrAtivo quando ele nomeia uma entrada válida do catálogo — é o
// caso comum em produção, pois aplicarEscolhaDoInstalador (instalador.go) já preenche esse campo com a
// escolha feita na tela custom do instalador ANTES do startup chegar aqui. Sem essa escolha (build de
// dev, ou marcador ausente), cai no motor marcado Padrao no catálogo (RapidOCR), como sempre foi.
func (a *App) bootstrapMotorPadrao() {
	escolhido, ok := motoresocr.ObterMotorBaixavel(a.Config.MotorOcrAtivo)
	if !ok {
		escolhido, ok = motoresocr.MotorOcrPadrao()
	}
	if !ok {
		runtime.EventsEmit(a.ctx, "ocr_indisponivel", "nenhum motor padrão declarado no catálogo")
		return
	}

	// Guard clause: sha256 vazio = o zip deste SO ainda não tem release (ex.: Linux antes da primeira
	// release motores-ocr-linux-v*). O app segue no ar (dicionário, revisão, progresso), só sem OCR.
	if escolhido.Artefato.Sha256 == "" {
		runtime.EventsEmit(a.ctx, "ocr_indisponivel",
			fmt.Sprintf("o motor %s ainda não foi publicado para este sistema operacional", escolhido.Rotulo))
		return
	}

	runtime.EventsEmit(a.ctx, "motor_bootstrap_inicio", escolhido.Rotulo)
	fmt.Printf("Bootstrap: baixando o motor escolhido (%s)…\n", escolhido.Rotulo)

	// 1) Baixa o motor escolhido (instalador) ou o padrão do catálogo (dev).
	if err := a.BaixarMotor(escolhido.Nome); err != nil {
		fmt.Printf("Bootstrap falhou (motor): %v\n", err)
		runtime.EventsEmit(a.ctx, "ocr_indisponivel", "falha ao baixar o motor: "+err.Error())
		return
	}

	// 2) Sobe o motor recém-instalado e espera o healthcheck.
	desc, ok := motoresocr.DescritorMotorInstalado(escolhido)
	if !ok {
		runtime.EventsEmit(a.ctx, "ocr_indisponivel", "motor baixado, mas o executável não foi encontrado")
		return
	}
	if err := a.motorOcr.Iniciar(desc); err != nil {
		runtime.EventsEmit(a.ctx, "ocr_indisponivel", err.Error())
		return
	}
	if err := motoresocr.AguardarBackend(30 * time.Second); err != nil {
		runtime.EventsEmit(a.ctx, "ocr_indisponivel", err.Error())
		return
	}

	a.Config.MotorOcrAtivo = escolhido.Nome
	if err := config.SaveConfig(a.Config); err != nil {
		fmt.Printf("Aviso: falha ao salvar o motor ativo: %v\n", err)
	}

	fmt.Println("Bootstrap concluído: motor de OCR pronto.")
	runtime.EventsEmit(a.ctx, "motor_bootstrap_fim", escolhido.Rotulo)
	runtime.EventsEmit(a.ctx, "ocr_pronto")
}
