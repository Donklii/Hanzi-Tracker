package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"wails_app/armazenamento"
	"wails_app/baixador"
	"wails_app/motoresstt"
)

// ----- Ciclo de vida dos MOTORES de STT (sidecars baixáveis) -----
// Expõe ao frontend o catálogo e o ciclo de vida (download/extração/remoção) donos do pacote
// motoresstt, em %APPDATA%\HanziTracker\motores_stt\<Motor>\. Espelha motores_tts.go.

// ----- API exposta ao frontend -----

// MotorSttInfo é o estado de um motor de STT para a UI "Gerenciar Motores de Escuta": catálogo +
// instalado/ativo. `publicado` indica se o artefato já tem release (sha256 preenchido) — a UI
// desabilita o download enquanto não houver.
type MotorSttInfo struct {
	Nome         string `json:"nome"`
	Rotulo       string `json:"rotulo"`
	Descricao    string `json:"descricao"`
	Versao       string `json:"versao"`
	Requisitos   string `json:"requisitos"`
	TamanhoBytes int64  `json:"tamanhoBytes"`
	Publicado    bool   `json:"publicado"`
	Instalado    bool   `json:"instalado"`
	Ativo        bool   `json:"ativo"`
}

// ListarMotoresStt devolve o catálogo de motores de STT com o estado instalado/ativo (ordenado por
// rótulo).
func (a *App) ListarMotoresStt() []MotorSttInfo {
	var ativoCmd string
	if a.motorStt != nil {
		ativoCmd = filepath.Clean(a.motorStt.ComandoAtivo())
	}

	lista := make([]MotorSttInfo, 0, len(motoresstt.MotoresSttBaixaveis))
	for _, m := range motoresstt.MotoresSttBaixaveis {
		exe := motoresstt.CaminhoExecutavelMotorStt(m)
		instalado := false
		if info, err := os.Stat(exe); err == nil && !info.IsDir() {
			instalado = true
		}
		// Um bundle local (builds/build_sidecars_stt_*.{sh,ps1}) também conta como instalado para a
		// UI: dá para usar o motor sem baixar nada.
		if !instalado {
			if _, ok := motoresstt.ResolverMotorStt(m.Nome); ok {
				instalado = true
			}
		}
		ativo := false
		if instalado && ativoCmd != "" && ativoCmd != "." {
			if desc, ok := motoresstt.ResolverMotorStt(m.Nome); ok && filepath.Clean(desc.Comando) == ativoCmd {
				ativo = true
			}
		}
		lista = append(lista, MotorSttInfo{
			Nome:         m.Nome,
			Rotulo:       m.Rotulo,
			Descricao:    m.Descricao,
			Versao:       m.Versao,
			Requisitos:   m.Requisitos,
			TamanhoBytes: m.Artefato.TamanhoBytes,
			Publicado:    m.Artefato.Sha256 != "",
			Instalado:    instalado,
			Ativo:        ativo,
		})
	}
	sort.Slice(lista, func(i, j int) bool { return lista[i].Rotulo < lista[j].Rotulo })
	return lista
}

// BaixarMotorStt baixa e instala um motor de STT do catálogo no AppData (progresso via o mesmo
// evento "motor_download_progresso" dos outros catálogos — a UI diferencia pelo nome).
func (a *App) BaixarMotorStt(nome string) error {
	m, ok := motoresstt.ObterMotorSttBaixavel(nome)
	if !ok {
		return fmt.Errorf("motor de STT '%s' não encontrado no catálogo", nome)
	}
	// Guard clause: sha256 vazio = o zip deste SO ainda não tem release (a UI já desabilita pelo
	// campo `publicado`; aqui é a defesa do backend contra um download inútil).
	if m.Artefato.Sha256 == "" {
		return fmt.Errorf("o motor de STT '%s' ainda não foi publicado para este sistema operacional", m.Rotulo)
	}
	destino := motoresstt.PastaMotorStt(m.Nome)
	if err := baixador.BaixarEExtrairArtefato(m.Artefato, destino, armazenamento.PastaDados(), func(msg string) { a.emitirProgressoMotor(m.Nome, msg) }); err != nil {
		a.emitirProgressoMotor(m.Nome, "⚠️ "+err.Error())
		return err
	}
	return nil
}

// RemoverMotorStt apaga a pasta de um motor de STT instalado. Se ele for o motor ativo, derruba o
// processo antes (como no TTS, ficar sem STT não quebra nada — a próxima escuta na revisão de
// pronúncia pede o download de novo).
func (a *App) RemoverMotorStt(nome string) error {
	m, ok := motoresstt.ObterMotorSttBaixavel(nome)
	if !ok {
		return fmt.Errorf("motor de STT '%s' não encontrado no catálogo", nome)
	}

	// O executável de um processo em execução fica travado no Windows: derruba antes de apagar.
	if a.motorStt != nil && a.motorStt.CatalogoAtivo() == m.Nome {
		a.motorStt.Encerrar()
	}

	pasta := motoresstt.PastaMotorStt(m.Nome)
	if _, err := os.Stat(pasta); os.IsNotExist(err) {
		return nil // já removido
	}
	return os.RemoveAll(pasta)
}

// limparMotoresStt remove TODOS os motores de STT baixados, derrubando o ativo antes (seu
// executável estaria em uso). É o que a aba de Armazenamento chama ao "Limpar" a categoria — sem
// STT o app segue funcionando (a revisão de pronúncia volta a pedir download).
func (a *App) limparMotoresStt() error {
	if a.motorStt != nil {
		a.motorStt.Encerrar()
	}

	raiz := motoresstt.PastaMotoresStt()
	entradas, err := os.ReadDir(raiz)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	var erros []string
	for _, e := range entradas {
		if err := os.RemoveAll(filepath.Join(raiz, e.Name())); err != nil {
			erros = append(erros, fmt.Sprintf("%s: %v", e.Name(), err))
		}
	}
	if len(erros) > 0 {
		return fmt.Errorf("alguns motores de STT não puderam ser removidos — %s", strings.Join(erros, "; "))
	}
	return nil
}
