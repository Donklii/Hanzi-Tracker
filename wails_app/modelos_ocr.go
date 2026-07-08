package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/wailsapp/wails/v2/pkg/runtime"
	"wails_app/baixador"
	"wails_app/motoresocr"
)

// ----- Ciclo de vida dos MODELOS (pesos) de OCR do motor ativo -----
// Expõe ao frontend o catálogo de pesos servido pelo sidecar ativo (/api/modelos) e o
// download/remoção deles no AppData real. Irmão de motores.go (que cuida dos MOTORES/sidecars).

// ArquivoModelo é um arquivo (det/rec) que compõe um modelo, com a URL de download e o hash
// esperado. O `Sha256`, quando preenchido, é conferido após o download para garantir integridade;
// vazio = verificação pulada (ver ModelosManifesto.py).
type ArquivoModelo struct {
	Nome   string `json:"nome"`
	Url    string `json:"url"`
	Sha256 string `json:"sha256"`
}

// ModeloOcrInfo espelha o estado de um modelo retornado por /api/modelos
type ModeloOcrInfo struct {
	Nome         string          `json:"nome"`
	Rotulo       string          `json:"rotulo"`
	Descricao    string          `json:"descricao"`
	Idiomas      []string        `json:"idiomas"`
	Baixavel     bool            `json:"baixavel"`
	Embutido     bool            `json:"embutido"`
	Instalado    bool            `json:"instalado"`
	TamanhoBytes int64           `json:"tamanhoBytes"`
	Arquivos     []ArquivoModelo `json:"arquivos"`
}

// ListarModelos retorna o catálogo de modelos de OCR e seu estado (instalado/embutido)
func (a *App) ListarModelos() ([]ModeloOcrInfo, error) {
	resp, err := http.Get(motoresocr.EnderecoBase() + "/api/modelos")
	if err != nil {
		return nil, fmt.Errorf("falha ao buscar modelos do Python: %w", err)
	}
	defer resp.Body.Close()

	var modelos []ModeloOcrInfo
	if err := json.NewDecoder(resp.Body).Decode(&modelos); err != nil {
		return nil, fmt.Errorf("falha ao decodificar lista de modelos: %w", err)
	}
	return modelos, nil
}

// BaixarModelo baixa os arquivos de um modelo diretamente para o AppData REAL.
// O download é feito pelo Go (e não pelo Python) porque o Python da Microsoft Store virtualiza o
// %APPDATA% para um sandbox; o Go, sendo um processo normal, escreve no caminho real que ele e o
// Python leem. Emite progresso pelo evento "modelo_download_progresso".
func (a *App) BaixarModelo(nome string) error {
	modelos, err := a.ListarModelos()
	if err != nil {
		return err
	}

	var alvo *ModeloOcrInfo
	for i := range modelos {
		if modelos[i].Nome == nome {
			alvo = &modelos[i]
			break
		}
	}
	if alvo == nil {
		return fmt.Errorf("modelo '%s' não encontrado no catálogo", nome)
	}
	if !alvo.Baixavel {
		return fmt.Errorf("o modelo '%s' não é baixável", nome)
	}

	// O catálogo veio do motor ATIVO (ListarModelos consulta o processo dele), então os pesos vão para
	// a subpasta desse motor — a mesma que o Python monta a partir de HANZITRACKER_MOTOR.
	motorAtivo := a.nomeMotorAtivo()
	if motorAtivo == "" {
		return fmt.Errorf("nenhum motor de OCR ativo para receber o modelo '%s'", nome)
	}

	destino := motoresocr.PastaModelosMotor(motorAtivo)
	if err := os.MkdirAll(destino, 0755); err != nil {
		return fmt.Errorf("falha ao criar a pasta de modelos: %w", err)
	}

	a.emitirProgressoModelo(nome, "Iniciando download…")
	for _, arq := range alvo.Arquivos {
		caminho := filepath.Join(destino, arq.Nome)
		if _, err := os.Stat(caminho); err == nil {
			continue // já baixado
		}
		if err := a.baixarArquivoModelo(arq, destino, caminho, func(msg string) { a.emitirProgressoModelo(nome, msg) }); err != nil {
			a.emitirProgressoModelo(nome, "⚠️ "+err.Error())
			return err
		}
	}
	return nil
}

// baixarArquivoModelo baixa UM arquivo de peso para a pasta do motor ativo. Alguns catálogos (ex.:
// EasyOCR) publicam o peso ZIPADO — a URL termina em .zip mas `arq.Nome` é o arquivo final (.pth):
// nesse caso o sha256 confere o ZIP baixado, que é extraído no destino e descartado.
func (a *App) baixarArquivoModelo(arq ArquivoModelo, destino, caminho string, onProgresso func(string)) error {
	// Guard clause: peso publicado direto (o caso comum, ex.: .onnx e .traineddata).
	if !strings.EqualFold(filepath.Ext(arq.Url), ".zip") || strings.EqualFold(filepath.Ext(arq.Nome), ".zip") {
		return baixador.BaixarArquivo(arq.Url, caminho, arq.Sha256, onProgresso)
	}

	zipLocal := caminho + ".zip"
	if err := baixador.BaixarArquivo(arq.Url, zipLocal, arq.Sha256, onProgresso); err != nil {
		return err
	}
	defer os.Remove(zipLocal)

	onProgresso(fmt.Sprintf("Extraindo %s…", arq.Nome))
	if err := baixador.ExtrairZip(zipLocal, destino); err != nil {
		return fmt.Errorf("falha ao extrair o peso %s: %w", arq.Nome, err)
	}
	if _, err := os.Stat(caminho); err != nil {
		return fmt.Errorf("o zip baixado não continha o arquivo esperado (%s)", arq.Nome)
	}
	return nil
}

// RemoverModelo apaga os arquivos de um modelo do AppData real, preservando arquivos que ainda são
// usados por outro modelo do catálogo (ex.: o detector 'server' compartilhado).
func (a *App) RemoverModelo(nome string) error {
	modelos, err := a.ListarModelos()
	if err != nil {
		return err
	}

	usadosPorOutros := map[string]bool{}
	var alvo *ModeloOcrInfo
	for i := range modelos {
		if modelos[i].Nome == nome {
			alvo = &modelos[i]
			continue
		}
		for _, arq := range modelos[i].Arquivos {
			usadosPorOutros[arq.Nome] = true
		}
	}
	if alvo == nil {
		return fmt.Errorf("modelo '%s' não encontrado no catálogo", nome)
	}

	motorAtivo := a.nomeMotorAtivo()
	if motorAtivo == "" {
		return fmt.Errorf("nenhum motor de OCR ativo para remover o modelo '%s'", nome)
	}

	destino := motoresocr.PastaModelosMotor(motorAtivo)
	for _, arq := range alvo.Arquivos {
		if usadosPorOutros[arq.Nome] {
			continue // compartilhado: preserva
		}
		caminho := filepath.Join(destino, arq.Nome)
		if _, err := os.Stat(caminho); err == nil {
			if err := os.Remove(caminho); err != nil {
				return fmt.Errorf("falha ao remover %s: %w", arq.Nome, err)
			}
		}
	}
	return nil
}

// emitirProgressoModelo envia um evento de progresso de download ao frontend.
func (a *App) emitirProgressoModelo(nome, mensagem string) {
	runtime.EventsEmit(a.ctx, "modelo_download_progresso", map[string]interface{}{"nome": nome, "mensagem": mensagem})
}
