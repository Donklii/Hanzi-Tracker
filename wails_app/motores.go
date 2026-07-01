package main

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/shirou/gopsutil/v3/disk"
	"github.com/wailsapp/wails/v2/pkg/runtime"
	"wails_app/config"
)

// ----- Ciclo de vida dos MOTORES de OCR (sidecars baixáveis) — Fase 5, Passo 5 -----
// Baixa/extrai/remove/troca os motores publicados (ver motores_manifesto.go) em
// %APPDATA%\HanziTracker\motores\<Motor>\, e faz o bootstrap de first-run (baixar o motor padrão
// quando nada existe localmente). O download reaproveita baixarArquivo (escrita atômica + verificação
// sha256). O overlay compartilhado (popup) mora em motores\_overlay\. Ver docs/PUBLICAR-MOTORES.md.

// ----- Caminhos -----

// pastaMotores é a raiz dos executáveis dos motores (%APPDATA%\HanziTracker\motores). Fica separada de
// modelos\ (pesos ONNX) — a aba de Armazenamento pode medi-la à parte.
func pastaMotores() string {
	return filepath.Join(pastaDados(), "motores")
}

// pastaMotor é a subpasta de UM motor (o zip é extraído aqui; o .exe fica na raiz dela).
func pastaMotor(nome string) string {
	return filepath.Join(pastaMotores(), nome)
}

// caminhoExecutavelMotor é o caminho completo do .exe de um motor instalado (pastaMotor + Executavel).
func caminhoExecutavelMotor(m MotorOcrBaixavel) string {
	return filepath.Join(pastaMotor(m.Nome), m.Executavel)
}

// pastaOverlay guarda o overlay compartilhado (não é um motor; nome reservado com "_" para não colidir
// com um motor de mesmo nome).
func pastaOverlay() string {
	return filepath.Join(pastaMotores(), "_overlay")
}

// caminhoExecutavelOverlay é o .exe do overlay extraído (o popup.zip traz popup.exe na raiz).
func caminhoExecutavelOverlay() string {
	return filepath.Join(pastaOverlay(), "popup.exe")
}

// ----- Resolução do motor a subir -----

// descritorMotorInstalado devolve como rodar um motor JÁ BAIXADO no AppData (ok=false se ausente).
func descritorMotorInstalado(m MotorOcrBaixavel) (DescritorMotorOcr, bool) {
	exe := caminhoExecutavelMotor(m)
	info, err := os.Stat(exe)
	if err != nil || info.IsDir() {
		return DescritorMotorOcr{}, false
	}
	abs, err := filepath.Abs(exe)
	if err != nil {
		abs = exe
	}
	return DescritorMotorOcr{Nome: m.Rotulo + " (instalado)", Comando: abs}, true
}

// resolverMotorInicial escolhe o motor a subir na inicialização, na ordem: (1) motor preferido do
// usuário (último ativo) instalado no AppData; (2) motor padrão do catálogo instalado no AppData;
// (3) sidecar em bundle ao lado do app. Devolve ok=false quando NADA foi encontrado — sinal de
// first-run: o app deve chamar bootstrapMotorPadrao (baixa+instala+ativa o RapidOCR padrão).
func resolverMotorInicial(nomePreferido string) (DescritorMotorOcr, bool) {
	if nomePreferido != "" {
		if m, ok := ObterMotorBaixavel(nomePreferido); ok {
			if desc, ok := descritorMotorInstalado(m); ok {
				return desc, true
			}
		}
	}
	if padrao, ok := MotorOcrPadrao(); ok {
		if desc, ok := descritorMotorInstalado(padrao); ok {
			return desc, true
		}
	}
	return resolverMotorOcrBundle()
}

// ----- Download + extração -----

// verificarEspacoDisco recusa a operação se o volume não tiver ao menos `bytesNecessarios` livres. Se
// não conseguir medir, não bloqueia (melhor tentar do que impedir sem motivo).
func verificarEspacoDisco(caminhoRef string, bytesNecessarios int64) error {
	uso, err := disk.Usage(caminhoRef)
	if err != nil {
		return nil
	}
	if int64(uso.Free) < bytesNecessarios {
		faltam := (bytesNecessarios - int64(uso.Free)) / (1024 * 1024)
		return fmt.Errorf("espaço em disco insuficiente: livre %d MB, necessário ~%d MB (faltam ~%d MB)",
			int64(uso.Free)/(1024*1024), bytesNecessarios/(1024*1024), faltam)
	}
	return nil
}

// extrairZip descompacta `origem` em `destino`, protegendo contra Zip Slip (entradas com ../ que
// escapariam da pasta). O conteúdo do zip publicado tem o .exe na raiz (ver build_sidecars.ps1).
func extrairZip(origem, destino string) error {
	r, err := zip.OpenReader(origem)
	if err != nil {
		return err
	}
	defer r.Close()

	destAbs, err := filepath.Abs(destino)
	if err != nil {
		return err
	}

	for _, f := range r.File {
		// Alguns zipadores (ex.: Compress-Archive do PowerShell) gravam os nomes com "\" em vez de "/",
		// violando a spec ZIP. Normaliza para "/" para detectar diretório (trailing slash) e montar o
		// caminho — sem isso, uma entrada de diretório vira arquivo vazio e a extração quebra depois.
		nome := strings.ReplaceAll(f.Name, `\`, "/")
		ehDir := strings.HasSuffix(nome, "/")
		alvo := filepath.Join(destino, filepath.FromSlash(nome))

		// Zip Slip: garante que o alvo fica DENTRO de destino.
		alvoAbs, err := filepath.Abs(alvo)
		if err != nil {
			return err
		}
		if alvoAbs != destAbs && !strings.HasPrefix(alvoAbs, destAbs+string(os.PathSeparator)) {
			return fmt.Errorf("entrada de zip fora do destino (%s): %s", destino, f.Name)
		}

		if ehDir {
			if err := os.MkdirAll(alvo, 0755); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(alvo), 0755); err != nil {
			return err
		}

		rc, err := f.Open()
		if err != nil {
			return err
		}
		out, err := os.OpenFile(alvo, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0755)
		if err != nil {
			rc.Close()
			return err
		}
		_, errCopy := io.Copy(out, rc)
		out.Close()
		rc.Close()
		if errCopy != nil {
			return errCopy
		}
	}
	return nil
}

// baixarEExtrairArtefato baixa um .zip (com verificação sha256 OBRIGATÓRIA), extrai em `destino` e
// apaga o zip. Faz pré-checagem de disco antes (o download é pesado). Reaproveita baixarArquivo.
func (a *App) baixarEExtrairArtefato(art ArtefatoBaixavel, destino string, onProgresso func(string)) error {
	// sha256 é OBRIGATÓRIO para binários (nos pesos ONNX é opcional; aqui, não).
	if art.Sha256 == "" {
		return fmt.Errorf("recusado: %s não declara sha256 (obrigatório para executáveis)", art.Nome)
	}

	// Pré-checagem de espaço: pico em disco = zip baixado + árvore extraída. Binários + DLLs do
	// onnxruntime/DirectML comprimem bem, então a extração passa de 3x o zip; exigimos ~5x de folga.
	if err := verificarEspacoDisco(pastaDados(), art.TamanhoBytes*5); err != nil {
		return err
	}

	// Extração idempotente: limpa resíduo de uma tentativa anterior (ex.: download/extração parcial que
	// falhou) antes de recriar o destino — senão um arquivo/diretório meio-criado colide com a nova
	// extração (ex.: um "diretório" que ficou como arquivo vazio).
	if err := os.RemoveAll(destino); err != nil {
		return fmt.Errorf("falha ao limpar %s antes de extrair: %w", destino, err)
	}
	if err := os.MkdirAll(destino, 0755); err != nil {
		return fmt.Errorf("falha ao criar %s: %w", destino, err)
	}

	// 1) Baixa o zip para dentro do destino (verificação de integridade embutida em baixarArquivo).
	zipLocal := filepath.Join(destino, art.Nome)
	onProgresso("Iniciando download…")
	if err := a.baixarArquivo(art.Url, zipLocal, art.Sha256, onProgresso); err != nil {
		os.Remove(zipLocal)
		return err
	}

	// 2) Extrai (o .exe fica na raiz de `destino`) e descarta o zip.
	onProgresso("Extraindo…")
	if err := extrairZip(zipLocal, destino); err != nil {
		os.Remove(zipLocal)
		return fmt.Errorf("falha ao extrair %s: %w", art.Nome, err)
	}
	os.Remove(zipLocal)

	onProgresso("Concluído ✓")
	return nil
}

// emitirProgressoMotor envia um evento de progresso de download/instalação de motor ao frontend.
func (a *App) emitirProgressoMotor(nome, mensagem string) {
	if a.ctx == nil {
		return
	}
	runtime.EventsEmit(a.ctx, "motor_download_progresso", map[string]interface{}{"nome": nome, "mensagem": mensagem})
}

// ----- API exposta ao frontend (Passo 6 consome) -----

// MotorOcrInfo é o estado de um motor para a UI "Gerenciar Motores": catálogo + instalado/ativo.
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
	Instalado    bool     `json:"instalado"`
	Ativo        bool     `json:"ativo"`
}

// ListarMotores devolve o catálogo de motores com o estado instalado/ativo (ordenado por rótulo).
func (a *App) ListarMotores() []MotorOcrInfo {
	var ativoCmd string
	if a.motorOcr != nil {
		ativoCmd = filepath.Clean(a.motorOcr.ComandoAtivo())
	}

	lista := make([]MotorOcrInfo, 0, len(MotoresBaixaveis))
	for _, m := range MotoresBaixaveis {
		exe := caminhoExecutavelMotor(m)
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
			Instalado:    instalado,
			Ativo:        ativo,
		})
	}
	sort.Slice(lista, func(i, j int) bool { return lista[i].Rotulo < lista[j].Rotulo })
	return lista
}

// BaixarMotor baixa e instala um motor do catálogo no AppData (progresso via "motor_download_progresso").
func (a *App) BaixarMotor(nome string) error {
	m, ok := ObterMotorBaixavel(nome)
	if !ok {
		return fmt.Errorf("motor '%s' não encontrado no catálogo", nome)
	}
	if err := a.baixarEExtrairArtefato(m.Artefato, pastaMotor(m.Nome), func(msg string) { a.emitirProgressoMotor(m.Nome, msg) }); err != nil {
		a.emitirProgressoMotor(m.Nome, "⚠️ "+err.Error())
		return err
	}
	return nil
}

// RemoverMotor apaga a pasta de um motor instalado. Recusa remover o motor que está ATIVO (deixaria o
// app sem OCR no meio do uso) — troque para outro antes.
func (a *App) RemoverMotor(nome string) error {
	m, ok := ObterMotorBaixavel(nome)
	if !ok {
		return fmt.Errorf("motor '%s' não encontrado no catálogo", nome)
	}

	if a.motorOcr != nil {
		if abs, err := filepath.Abs(caminhoExecutavelMotor(m)); err == nil {
			if filepath.Clean(abs) == filepath.Clean(a.motorOcr.ComandoAtivo()) {
				return fmt.Errorf("o motor '%s' está ativo; troque para outro motor antes de removê-lo", m.Rotulo)
			}
		}
	}

	pasta := pastaMotor(m.Nome)
	if _, err := os.Stat(pasta); os.IsNotExist(err) {
		return nil // já removido
	}
	return os.RemoveAll(pasta)
}

// limparMotores remove os motores INATIVOS baixados, preservando o motor ativo (seu .exe está em uso)
// e o overlay compartilhado (`_overlay`, em uso enquanto o app roda). É o que a aba de Armazenamento
// chama ao "Limpar" a categoria de motores — libera espaço sem derrubar o OCR atual.
func (a *App) limparMotores() error {
	raiz := pastaMotores()
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
		if e.Name() == "_overlay" {
			continue // overlay compartilhado — em uso enquanto o app roda
		}
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
	m, ok := ObterMotorBaixavel(nome)
	if !ok {
		return fmt.Errorf("motor '%s' não encontrado no catálogo", nome)
	}
	desc, instalado := descritorMotorInstalado(m)
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

// garantirOverlay baixa o overlay compartilhado se ele ainda não estiver instalado no AppData.
func (a *App) garantirOverlay() error {
	if info, err := os.Stat(caminhoExecutavelOverlay()); err == nil && !info.IsDir() {
		return nil // já instalado
	}
	return a.baixarEExtrairArtefato(PopupOverlayBaixavel, pastaOverlay(), func(msg string) { a.emitirProgressoMotor("overlay", msg) })
}

// bootstrapMotorPadrao roda no first-run (nenhum motor local/instalado): baixa o motor padrão + o
// overlay, sobe o motor e anuncia os eventos de estado. Deve rodar em uma goroutine (faz I/O de rede).
func (a *App) bootstrapMotorPadrao() {
	padrao, ok := MotorOcrPadrao()
	if !ok {
		runtime.EventsEmit(a.ctx, "ocr_indisponivel", "nenhum motor padrão declarado no catálogo")
		return
	}

	runtime.EventsEmit(a.ctx, "motor_bootstrap_inicio", padrao.Rotulo)
	fmt.Printf("Bootstrap: baixando o motor padrão (%s)…\n", padrao.Rotulo)

	// 1) Motor padrão.
	if err := a.BaixarMotor(padrao.Nome); err != nil {
		fmt.Printf("Bootstrap falhou (motor): %v\n", err)
		runtime.EventsEmit(a.ctx, "ocr_indisponivel", "falha ao baixar o motor padrão: "+err.Error())
		return
	}

	// 2) Overlay compartilhado (não bloqueia o OCR se falhar).
	if err := a.garantirOverlay(); err != nil {
		fmt.Printf("Aviso: overlay indisponível após bootstrap: %v\n", err)
	}

	// 3) Sobe o motor recém-instalado e espera o healthcheck.
	desc, ok := descritorMotorInstalado(padrao)
	if !ok {
		runtime.EventsEmit(a.ctx, "ocr_indisponivel", "motor baixado, mas o executável não foi encontrado")
		return
	}
	if err := a.motorOcr.Iniciar(desc); err != nil {
		runtime.EventsEmit(a.ctx, "ocr_indisponivel", err.Error())
		return
	}
	if err := aguardarBackendOcr(30 * time.Second); err != nil {
		runtime.EventsEmit(a.ctx, "ocr_indisponivel", err.Error())
		return
	}

	// 4) Sobe o overlay agora que ele existe (o startup pulou por não haver executável).
	a.iniciarOverlay()

	a.Config.MotorOcrAtivo = padrao.Nome
	if err := config.SaveConfig(a.Config); err != nil {
		fmt.Printf("Aviso: falha ao salvar o motor ativo: %v\n", err)
	}

	fmt.Println("Bootstrap concluído: motor de OCR pronto.")
	runtime.EventsEmit(a.ctx, "motor_bootstrap_fim", padrao.Rotulo)
	runtime.EventsEmit(a.ctx, "ocr_pronto")
}
