package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// ----- Ciclo de vida dos MOTORES de TTS (sidecars baixáveis) -----
// Baixa/extrai/remove os motores de voz publicados (ver motores_tts_manifesto.go) em
// %APPDATA%\HanziTracker\motores_tts\<Motor>\. Espelha motores.go (motores de OCR), reaproveitando
// baixarEExtrairArtefato (escrita atômica + verificação sha256 + pré-checagem de disco). A pasta é
// SEPARADA de motores\ porque limparMotores() preserva apenas o motor de OCR ativo — misturar os
// dois faria a limpeza de OCR apagar um motor de voz instalado.

// ----- Caminhos -----

// pastaMotoresTts é a raiz dos executáveis dos motores de voz (%APPDATA%\HanziTracker\motores_tts).
func pastaMotoresTts() string {
	return filepath.Join(pastaDados(), "motores_tts")
}

// pastaMotorTts é a subpasta de UM motor de voz (o zip é extraído aqui; o .exe fica na raiz dela).
func pastaMotorTts(nome string) string {
	return filepath.Join(pastaMotoresTts(), nome)
}

// caminhoExecutavelMotorTts é o caminho completo do .exe de um motor de voz instalado.
func caminhoExecutavelMotorTts(m MotorTtsBaixavel) string {
	return filepath.Join(pastaMotorTts(m.Nome), m.Executavel)
}

// ----- Resolução do motor a subir -----

// descritorMotorTtsInstalado devolve como rodar um motor de voz JÁ BAIXADO no AppData (ok=false se
// ausente).
func descritorMotorTtsInstalado(m MotorTtsBaixavel) (DescritorMotorTts, bool) {
	exe := caminhoExecutavelMotorTts(m)
	info, err := os.Stat(exe)
	if err != nil || info.IsDir() {
		return DescritorMotorTts{}, false
	}
	abs, err := filepath.Abs(exe)
	if err != nil {
		abs = exe
	}
	return DescritorMotorTts{Nome: m.Rotulo + " (instalado)", Catalogo: m.Nome, Comando: abs}, true
}

// resolverMotorTts resolve como subir o motor de voz `nome`, na ordem: (1) instalado no AppData;
// (2) sidecar em bundle ao lado do app (build local). Devolve ok=false quando o motor não está
// disponível — sinal para a UI pedir o download em "Gerenciar Motores de Voz". Diferente do OCR,
// NÃO há bootstrap automático: a feature é opcional e desligada por padrão, então o download é
// sempre uma ação explícita do usuário.
func resolverMotorTts(nome string) (DescritorMotorTts, bool) {
	m, ok := ObterMotorTtsBaixavel(nome)
	if !ok {
		return DescritorMotorTts{}, false
	}
	if desc, ok := descritorMotorTtsInstalado(m); ok {
		return desc, true
	}
	return resolverMotorTtsBundle(m)
}

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

	lista := make([]MotorTtsInfo, 0, len(MotoresTtsBaixaveis))
	for _, m := range MotoresTtsBaixaveis {
		exe := caminhoExecutavelMotorTts(m)
		instalado := false
		if info, err := os.Stat(exe); err == nil && !info.IsDir() {
			instalado = true
		}
		// Um bundle local (build_sidecars.ps1) também conta como instalado para a UI: dá para usar
		// o motor sem baixar nada.
		if !instalado {
			if _, ok := resolverMotorTtsBundle(m); ok {
				instalado = true
			}
		}
		ativo := false
		if instalado && ativoCmd != "" && ativoCmd != "." {
			if desc, ok := resolverMotorTts(m.Nome); ok && filepath.Clean(desc.Comando) == ativoCmd {
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
	m, ok := ObterMotorTtsBaixavel(nome)
	if !ok {
		return fmt.Errorf("motor de voz '%s' não encontrado no catálogo", nome)
	}
	if err := a.baixarEExtrairArtefato(m.Artefato, pastaMotorTts(m.Nome), func(msg string) { a.emitirProgressoMotor(m.Nome, msg) }); err != nil {
		a.emitirProgressoMotor(m.Nome, "⚠️ "+err.Error())
		return err
	}
	return nil
}

// RemoverMotorTts apaga a pasta de um motor de voz instalado. Se ele for o motor ativo, derruba o
// processo antes (diferente do OCR, ficar sem TTS não quebra nada — a próxima leitura em voz alta
// pede o download de novo).
func (a *App) RemoverMotorTts(nome string) error {
	m, ok := ObterMotorTtsBaixavel(nome)
	if !ok {
		return fmt.Errorf("motor de voz '%s' não encontrado no catálogo", nome)
	}

	// O .exe de um processo em execução fica travado no Windows: derruba antes de apagar.
	if a.motorTts != nil && a.motorTts.CatalogoAtivo() == m.Nome {
		a.motorTts.Encerrar()
	}

	pasta := pastaMotorTts(m.Nome)
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

	raiz := pastaMotoresTts()
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
