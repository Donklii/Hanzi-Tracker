package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"
	"wails_app/armazenamento"
	"wails_app/baixador"
	"wails_app/config"
	"wails_app/ocr"
	"wails_app/stt"
	"wails_app/tts"
)

// ----- Seção vinda de: bindings_motores_ocr.go -----

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

	lista := make([]MotorOcrInfo, 0, len(ocr.MotoresBaixaveis))
	for _, m := range ocr.MotoresBaixaveis {
		exe := ocr.CaminhoExecutavelMotor(m)
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
	m, ok := ocr.ObterMotorBaixavel(nome)
	if !ok {
		return fmt.Errorf("motor '%s' não encontrado no catálogo", nome)
	}
	// Guard clause: sha256 vazio = o zip deste SO ainda não tem release (ex.: Linux antes da primeira
	// release motores-ocr-linux-v*) — recusa antes de gastar centenas de MB num download inútil.
	if m.Artefato.Sha256 == "" {
		return fmt.Errorf("o motor '%s' ainda não foi publicado para este sistema operacional", m.Rotulo)
	}
	destino := ocr.PastaMotorOcr(m.Nome)
	if err := baixador.BaixarEExtrairArtefato(m.Artefato, destino, armazenamento.PastaDados(), func(msg string) { a.emitirProgressoMotor(m.Nome, msg) }); err != nil {
		a.emitirProgressoMotor(m.Nome, "⚠️ "+err.Error())
		return err
	}
	return nil
}

// RemoverMotor apaga a pasta de um motor instalado. Recusa remover o motor que está ATIVO (deixaria o
// app sem OCR no meio do uso) — troque para outro antes.
func (a *App) RemoverMotor(nome string) error {
	m, ok := ocr.ObterMotorBaixavel(nome)
	if !ok {
		return fmt.Errorf("motor '%s' não encontrado no catálogo", nome)
	}

	if a.motorOcr != nil {
		if abs, err := filepath.Abs(ocr.CaminhoExecutavelMotor(m)); err == nil {
			if filepath.Clean(abs) == filepath.Clean(a.motorOcr.ComandoAtivo()) {
				return fmt.Errorf("o motor '%s' está ativo; troque para outro motor antes de removê-lo", m.Rotulo)
			}
		}
	}

	pasta := ocr.PastaMotorOcr(m.Nome)
	if _, err := os.Stat(pasta); os.IsNotExist(err) {
		return nil // já removido
	}
	return os.RemoveAll(pasta)
}

// limparMotores remove os motores INATIVOS baixados, preservando o motor ativo (seu .exe está em uso)
// e o overlay compartilhado (`_overlay`, em uso enquanto o app roda). É o que a aba de Armazenamento
// chama ao "Limpar" a categoria de motores — libera espaço sem derrubar o OCR atual.
func (a *App) limparMotores() error {
	raiz := ocr.PastaMotoresOcr()
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
	m, ok := ocr.ObterMotorBaixavel(nome)
	if !ok {
		return fmt.Errorf("motor '%s' não encontrado no catálogo", nome)
	}
	desc, instalado := ocr.DescritorMotorInstalado(m)
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
	escolhido, ok := ocr.ObterMotorBaixavel(a.Config.MotorOcrAtivo)
	if !ok {
		escolhido, ok = ocr.MotorOcrPadrao()
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
	desc, ok := ocr.DescritorMotorInstalado(escolhido)
	if !ok {
		runtime.EventsEmit(a.ctx, "ocr_indisponivel", "motor baixado, mas o executável não foi encontrado")
		return
	}
	if err := a.motorOcr.Iniciar(desc); err != nil {
		runtime.EventsEmit(a.ctx, "ocr_indisponivel", err.Error())
		return
	}
	if err := ocr.AguardarBackend(30 * time.Second); err != nil {
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

// ----- Seção vinda de: bindings_modelos_ocr.go -----

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
	resp, err := http.Get(ocr.EnderecoBase() + "/api/modelos")
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

	destino := ocr.PastaModelosMotor(motorAtivo)
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

	destino := ocr.PastaModelosMotor(motorAtivo)
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

// ----- Seção vinda de: bindings_motores_tts.go -----

// ----- Ciclo de vida dos MOTORES de TTS (sidecars baixáveis) -----
// Expõe ao frontend o catálogo e o ciclo de vida (download/extração/remoção) donos do pacote
// motorestts, em %APPDATA%\HanziTracker\motores_tts\<Motor>\.

// ----- API exposta ao frontend -----

// MotorTtsInfo é o estado de um motor de voz para a UI "Gerenciar Motores de Voz": catálogo +
// instalado/ativo. `publicado` indica se o artefato já tem release (sha256 preenchido) — a UI
// desabilita o download enquanto não houver.
type MotorTtsInfo struct {
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

// ListarMotoresTts devolve o catálogo de motores de voz com o estado instalado/ativo (ordenado por
// rótulo).
func (a *App) ListarMotoresTts() []MotorTtsInfo {
	var ativoCmd string
	if a.motorTts != nil {
		ativoCmd = filepath.Clean(a.motorTts.ComandoAtivo())
	}

	lista := make([]MotorTtsInfo, 0, len(tts.MotoresTtsBaixaveis))
	for _, m := range tts.MotoresTtsBaixaveis {
		exe := tts.CaminhoExecutavelMotorTts(m)
		instalado := false
		if info, err := os.Stat(exe); err == nil && !info.IsDir() {
			instalado = true
		}
		// Um bundle local (builds/build_sidecars_tts_windows.ps1) também conta como instalado para a UI: dá para usar
		// o motor sem baixar nada.
		if !instalado {
			if _, ok := tts.ResolverMotorTts(m.Nome); ok {
				instalado = true
			}
		}
		ativo := false
		if instalado && ativoCmd != "" && ativoCmd != "." {
			if desc, ok := tts.ResolverMotorTts(m.Nome); ok && filepath.Clean(desc.Comando) == ativoCmd {
				ativo = true
			}
		}
		lista = append(lista, MotorTtsInfo{
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

// BaixarMotorTts baixa e instala um motor de voz do catálogo no AppData (progresso via o mesmo
// evento "motor_download_progresso" dos motores de OCR — a UI diferencia pelo nome).
func (a *App) BaixarMotorTts(nome string) error {
	m, ok := tts.ObterMotorTtsBaixavel(nome)
	if !ok {
		return fmt.Errorf("motor de voz '%s' não encontrado no catálogo", nome)
	}
	// Guard clause: sha256 vazio = o zip deste SO ainda não tem release (a UI já desabilita pelo
	// campo `publicado`; aqui é a defesa do backend contra um download inútil).
	if m.Artefato.Sha256 == "" {
		return fmt.Errorf("o motor de voz '%s' ainda não foi publicado para este sistema operacional", m.Rotulo)
	}
	destino := tts.PastaMotorTts(m.Nome)
	if err := baixador.BaixarEExtrairArtefato(m.Artefato, destino, armazenamento.PastaDados(), func(msg string) { a.emitirProgressoMotor(m.Nome, msg) }); err != nil {
		a.emitirProgressoMotor(m.Nome, "⚠️ "+err.Error())
		return err
	}
	return nil
}

// RemoverMotorTts apaga a pasta de um motor de voz instalado. Se ele for o motor ativo, derruba o
// processo antes (diferente do OCR, ficar sem TTS não quebra nada — a próxima leitura em voz alta
// pede o download de novo).
func (a *App) RemoverMotorTts(nome string) error {
	m, ok := tts.ObterMotorTtsBaixavel(nome)
	if !ok {
		return fmt.Errorf("motor de voz '%s' não encontrado no catálogo", nome)
	}

	// O .exe de um processo em execução fica travado no Windows: derruba antes de apagar.
	if a.motorTts != nil && a.motorTts.CatalogoAtivo() == m.Nome {
		a.motorTts.Encerrar()
	}

	pasta := tts.PastaMotorTts(m.Nome)
	if _, err := os.Stat(pasta); os.IsNotExist(err) {
		return nil // já removido
	}
	return os.RemoveAll(pasta)
}

// limparMotoresTts remove TODOS os motores de voz baixados, derrubando o ativo antes (seu .exe
// estaria em uso). É o que a aba de Armazenamento chama ao "Limpar" a categoria — sem TTS o app
// segue funcionando (a leitura em voz alta volta a pedir download).
func (a *App) limparMotoresTts() error {
	if a.motorTts != nil {
		a.motorTts.Encerrar()
	}

	raiz := tts.PastaMotoresTts()
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
		return fmt.Errorf("alguns motores de voz não puderam ser removidos — %s", strings.Join(erros, "; "))
	}
	return nil
}

// ----- Seção vinda de: bindings_motores_stt.go -----

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

	lista := make([]MotorSttInfo, 0, len(stt.MotoresSttBaixaveis))
	for _, m := range stt.MotoresSttBaixaveis {
		exe := stt.CaminhoExecutavelMotorStt(m)
		instalado := false
		if info, err := os.Stat(exe); err == nil && !info.IsDir() {
			instalado = true
		}
		// Um bundle local (builds/build_sidecars_stt_*.{sh,ps1}) também conta como instalado para a
		// UI: dá para usar o motor sem baixar nada.
		if !instalado {
			if _, ok := stt.ResolverMotorStt(m.Nome); ok {
				instalado = true
			}
		}
		ativo := false
		if instalado && ativoCmd != "" && ativoCmd != "." {
			if desc, ok := stt.ResolverMotorStt(m.Nome); ok && filepath.Clean(desc.Comando) == ativoCmd {
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
	m, ok := stt.ObterMotorSttBaixavel(nome)
	if !ok {
		return fmt.Errorf("motor de STT '%s' não encontrado no catálogo", nome)
	}
	// Guard clause: sha256 vazio = o zip deste SO ainda não tem release (a UI já desabilita pelo
	// campo `publicado`; aqui é a defesa do backend contra um download inútil).
	if m.Artefato.Sha256 == "" {
		return fmt.Errorf("o motor de STT '%s' ainda não foi publicado para este sistema operacional", m.Rotulo)
	}
	destino := stt.PastaMotorStt(m.Nome)
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
	m, ok := stt.ObterMotorSttBaixavel(nome)
	if !ok {
		return fmt.Errorf("motor de STT '%s' não encontrado no catálogo", nome)
	}

	// O executável de um processo em execução fica travado no Windows: derruba antes de apagar.
	if a.motorStt != nil && a.motorStt.CatalogoAtivo() == m.Nome {
		a.motorStt.Encerrar()
	}

	pasta := stt.PastaMotorStt(m.Nome)
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

	raiz := stt.PastaMotoresStt()
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

