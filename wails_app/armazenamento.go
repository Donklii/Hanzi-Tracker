package main

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"github.com/shirou/gopsutil/v3/disk"
	"wails_app/armazenamento"
	"wails_app/motoresocr"
	"wails_app/motorestts"
	"wails_app/progresso"
)

// ItemArmazenamento descreve uma categoria de dados em disco usada pelo app.
type ItemArmazenamento struct {
	Chave     string `json:"chave"`
	Rotulo    string `json:"rotulo"`
	Descricao string `json:"descricao"`
	Caminho   string `json:"caminho"`
	Bytes     int64  `json:"bytes"`
	Limpavel  bool   `json:"limpavel"`
	Perigoso  bool   `json:"perigoso"` // limpar apaga dados do usuário (ex.: vocabulário)
}

// StorageInfo resume o uso de disco do app e o espaço livre do volume.
type StorageInfo struct {
	Itens      []ItemArmazenamento `json:"itens"`
	TotalBytes int64               `json:"totalBytes"`
	DiscoLivre int64               `json:"discoLivre"`
	DiscoTotal int64               `json:"discoTotal"`
	PastaDados string              `json:"pastaDados"`
}

// ----- Métodos expostos ao frontend -----

// GetStorageInfo retorna o uso de disco por categoria e o espaço livre no volume.
func (a *App) GetStorageInfo() StorageInfo {
	var itens []ItemArmazenamento

	// Motores de OCR baixados (sidecars + os pesos de cada um, agora dentro de motores_ocr\<Motor>\modelos).
	// Só aparece quando há algum baixado.
	if b := armazenamento.TamanhoCaminho(motoresocr.PastaMotoresOcr()); b > 0 {
		itens = append(itens, ItemArmazenamento{
			Chave:     "motores_ocr",
			Rotulo:    "Motores de OCR",
			Descricao: "Programas de reconhecimento e seus modelos baixados. Limpar remove os motores inativos (mantém o ativo e o overlay).",
			Caminho:   motoresocr.PastaMotoresOcr(),
			Bytes:     b,
			Limpavel:  true,
		})
	}

	// Motores de voz (TTS) baixados (sidecars + os pesos de cada um, baixados do Hugging Face para
	// motores_tts\<Motor>\modelos\hf). Só aparece quando há algum baixado.
	if b := armazenamento.TamanhoCaminho(motorestts.PastaMotoresTts()); b > 0 {
		itens = append(itens, ItemArmazenamento{
			Chave:     "motores_tts",
			Rotulo:    "Motores de Voz",
			Descricao: "Programas de leitura em voz alta e seus modelos baixados. Limpar remove todos (a leitura volta a pedir download).",
			Caminho:   motorestts.PastaMotoresTts(),
			Bytes:     b,
			Limpavel:  true,
		})
	}

	// Categorias de ambiente de desenvolvimento (modelos EasyOCR e cache de instalação) só fazem
	// sentido quando existem: no app distribuído já compilado normalmente nem aparecem, evitando
	// expor tecnicalidades ao usuário final.
	if b := armazenamento.TamanhoCaminho(armazenamento.PastaEasyOcr()); b > 0 {
		itens = append(itens, ItemArmazenamento{
			Chave:     "modelos_easyocr",
			Rotulo:    "Modelos do EasyOCR",
			Descricao: "Pesos do motor EasyOCR, baixados ao usá-lo.",
			Caminho:   armazenamento.PastaEasyOcr(),
			Bytes:     b,
			Limpavel:  true,
		})
	}
	if b := armazenamento.TamanhoCaminho(armazenamento.PastaCachePip()); b > 0 {
		itens = append(itens, ItemArmazenamento{
			Chave:     "cache_pip",
			Rotulo:    "Cache de instalação",
			Descricao: "Arquivos temporários de instalação de componentes. Seguro apagar.",
			Caminho:   armazenamento.PastaCachePip(),
			Bytes:     b,
			Limpavel:  true,
		})
	}

	tamanhoCacheTraducao, _, _ := progresso.TamanhoCacheTraducao()
	if tamanhoCacheTraducao > 0 {
		itens = append(itens, ItemArmazenamento{
			Chave:     "cache_traducao",
			Rotulo:    "Cache de Tradução",
			Descricao: "Textos traduzidos para não gastar cota em repetições.",
			Caminho:   armazenamento.CaminhoBanco() + " (Tabela interna)",
			Bytes:     tamanhoCacheTraducao,
			Limpavel:  true,
		})
	}

	tamanhoCacheTts, _, _ := progresso.TamanhoCacheTts()
	if tamanhoCacheTts > 0 {
		itens = append(itens, ItemArmazenamento{
			Chave:     "cache_tts",
			Rotulo:    "Cache de Áudio (Voz)",
			Descricao: "Falas já sintetizadas, para repetições saírem instantâneas e sem custo de CPU.",
			Caminho:   armazenamento.CaminhoBanco() + " (Tabela interna)",
			Bytes:     tamanhoCacheTts,
			Limpavel:  true,
		})
	}

	itens = append(itens,
		ItemArmazenamento{
			Chave:     "logs",
			Rotulo:    "Logs de erro",
			Descricao: "Arquivos de log gerados quando algo falha.",
			Caminho:   armazenamento.PastaDados(),
			Bytes:     armazenamento.TamanhoLogs(),
			Limpavel:  true,
		},
		ItemArmazenamento{
			Chave:     "banco",
			Rotulo:    "Banco de vocabulário",
			Descricao: "Suas palavras vistas, em estudo e aprendidas. Apagar zera o progresso!",
			Caminho:   armazenamento.CaminhoBanco(),
			Bytes:     armazenamento.TamanhoCaminho(armazenamento.CaminhoBanco()),
			Limpavel:  true,
			Perigoso:  true,
		},
	)

	var total int64
	for _, it := range itens {
		total += it.Bytes
	}

	info := StorageInfo{
		Itens:      itens,
		TotalBytes: total,
		PastaDados: armazenamento.PastaDados(),
	}

	if uso, err := disk.Usage(armazenamento.PastaDados()); err == nil {
		info.DiscoLivre = int64(uso.Free)
		info.DiscoTotal = int64(uso.Total)
	}

	return info
}

// AbrirPastaDados abre a pasta de dados do app no gerenciador de arquivos do SO.
func (a *App) AbrirPastaDados() error {
	pasta := armazenamento.PastaDados()
	if err := os.MkdirAll(pasta, 0755); err != nil {
		return err
	}
	if runtime.GOOS == "windows" {
		return exec.Command("explorer", pasta).Start()
	}
	return exec.Command("xdg-open", pasta).Start()
}

// LimparArmazenamento apaga os dados de uma categoria pela chave.
func (a *App) LimparArmazenamento(chave string) error {
	switch chave {
	case "motores_ocr":
		return a.limparMotores()
	case "motores_tts":
		return a.limparMotoresTts()
	case "modelos_easyocr":
		return os.RemoveAll(armazenamento.PastaEasyOcr())
	case "cache_pip":
		return os.RemoveAll(armazenamento.PastaCachePip())
	case "logs":
		return armazenamento.LimparLogs()
	case "cache_traducao":
		return progresso.LimparCacheTraducao()
	case "cache_tts":
		return progresso.LimparCacheTts()
	case "banco":
		return progresso.LimparVocabulario()
	default:
		return fmt.Errorf("categoria de armazenamento desconhecida: %s", chave)
	}
}

// ExcluirTudo apaga todos os dados baixados/gerados e zera o vocabulário (as preferências em
// configuracoes.json são preservadas). Equivale a um "reset de armazenamento".
func (a *App) ExcluirTudo() error {
	var erros []string

	if err := a.limparMotores(); err != nil {
		erros = append(erros, fmt.Sprintf("motores: %v", err))
	}
	if err := a.limparMotoresTts(); err != nil {
		erros = append(erros, fmt.Sprintf("motores de voz: %v", err))
	}
	if err := os.RemoveAll(armazenamento.PastaEasyOcr()); err != nil {
		erros = append(erros, fmt.Sprintf("modelos EasyOCR: %v", err))
	}
	if err := os.RemoveAll(armazenamento.PastaCachePip()); err != nil {
		erros = append(erros, fmt.Sprintf("cache pip: %v", err))
	}
	if err := armazenamento.LimparLogs(); err != nil {
		erros = append(erros, fmt.Sprintf("logs: %v", err))
	}
	if err := progresso.LimparCacheTraducao(); err != nil {
		erros = append(erros, fmt.Sprintf("cache de tradução: %v", err))
	}
	if err := progresso.LimparCacheTts(); err != nil {
		erros = append(erros, fmt.Sprintf("cache de áudio TTS: %v", err))
	}
	if err := progresso.LimparVocabulario(); err != nil {
		erros = append(erros, fmt.Sprintf("banco: %v", err))
	}

	if len(erros) > 0 {
		return fmt.Errorf("alguns itens não puderam ser apagados — %s", strings.Join(erros, "; "))
	}
	return nil
}
