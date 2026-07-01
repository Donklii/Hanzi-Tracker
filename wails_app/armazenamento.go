package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/shirou/gopsutil/v3/disk"
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

// ----- Caminhos -----

func pastaDados() string {
	appData, err := os.UserConfigDir()
	if err != nil {
		return ""
	}
	return filepath.Join(appData, "HanziTracker")
}

// pastaModelos é a raiz dos pesos de OCR (%APPDATA%\HanziTracker\modelos). Cada motor guarda os seus
// numa subpasta dedicada (ex.: modelos\RapidOCR) para não colidir com os pesos de outros motores.
// A aba de Armazenamento mede/limpa a raiz, englobando todos os motores.
func pastaModelos() string {
	return filepath.Join(pastaDados(), "modelos")
}

// pastaModelosRapidOcr é a subpasta dos pesos ONNX do motor RapidOCR (modelos\RapidOCR).
func pastaModelosRapidOcr() string {
	return filepath.Join(pastaModelos(), "RapidOCR")
}

func pastaEasyOcr() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".EasyOCR")
}

func pastaCachePip() string {
	local := os.Getenv("LOCALAPPDATA")
	if local == "" {
		return ""
	}
	return filepath.Join(local, "pip", "Cache")
}

func caminhoBanco() string {
	return filepath.Join(pastaDados(), "progresso.db")
}

// ----- Medição -----

// tamanhoCaminho soma o tamanho de um arquivo ou de todos os arquivos de uma pasta (recursivo).
func tamanhoCaminho(caminho string) int64 {
	if caminho == "" {
		return 0
	}
	info, err := os.Stat(caminho)
	if err != nil {
		return 0
	}
	if !info.IsDir() {
		return info.Size()
	}

	var total int64
	filepath.Walk(caminho, func(_ string, fi os.FileInfo, err error) error {
		if err == nil && fi != nil && !fi.IsDir() {
			total += fi.Size()
		}
		return nil
	})
	return total
}

// tamanhoLogs soma o tamanho dos arquivos .log na pasta de dados.
func tamanhoLogs() int64 {
	var total int64
	entradas, err := os.ReadDir(pastaDados())
	if err != nil {
		return 0
	}
	for _, e := range entradas {
		if e.IsDir() || !strings.HasSuffix(strings.ToLower(e.Name()), ".log") {
			continue
		}
		if info, err := e.Info(); err == nil {
			total += info.Size()
		}
	}
	return total
}

// ----- Métodos expostos ao frontend -----

// GetStorageInfo retorna o uso de disco por categoria e o espaço livre no volume.
func (a *App) GetStorageInfo() StorageInfo {
	itens := []ItemArmazenamento{
		{
			Chave:     "modelos_onnx",
			Rotulo:    "Modelos de OCR",
			Descricao: "Modelos de reconhecimento baixados sob demanda.",
			Caminho:   pastaModelos(),
			Bytes:     tamanhoCaminho(pastaModelos()),
			Limpavel:  true,
		},
	}

	// Categorias de ambiente de desenvolvimento (modelos EasyOCR e cache de instalação) só fazem
	// sentido quando existem: no app distribuído já compilado normalmente nem aparecem, evitando
	// expor tecnicalidades ao usuário final.
	if b := tamanhoCaminho(pastaEasyOcr()); b > 0 {
		itens = append(itens, ItemArmazenamento{
			Chave:     "modelos_easyocr",
			Rotulo:    "Modelos do EasyOCR",
			Descricao: "Pesos do motor EasyOCR, baixados ao usá-lo.",
			Caminho:   pastaEasyOcr(),
			Bytes:     b,
			Limpavel:  true,
		})
	}
	if b := tamanhoCaminho(pastaCachePip()); b > 0 {
		itens = append(itens, ItemArmazenamento{
			Chave:     "cache_pip",
			Rotulo:    "Cache de instalação",
			Descricao: "Arquivos temporários de instalação de componentes. Seguro apagar.",
			Caminho:   pastaCachePip(),
			Bytes:     b,
			Limpavel:  true,
		})
	}

	itens = append(itens,
		ItemArmazenamento{
			Chave:     "logs",
			Rotulo:    "Logs de erro",
			Descricao: "Arquivos de log gerados quando algo falha.",
			Caminho:   pastaDados(),
			Bytes:     tamanhoLogs(),
			Limpavel:  true,
		},
		ItemArmazenamento{
			Chave:     "banco",
			Rotulo:    "Banco de vocabulário",
			Descricao: "Suas palavras vistas, em estudo e aprendidas. Apagar zera o progresso!",
			Caminho:   caminhoBanco(),
			Bytes:     tamanhoCaminho(caminhoBanco()),
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
		PastaDados: pastaDados(),
	}

	if uso, err := disk.Usage(pastaDados()); err == nil {
		info.DiscoLivre = int64(uso.Free)
		info.DiscoTotal = int64(uso.Total)
	}

	return info
}

// AbrirPastaDados abre a pasta de dados do app no Explorer.
func (a *App) AbrirPastaDados() error {
	pasta := pastaDados()
	if err := os.MkdirAll(pasta, 0755); err != nil {
		return err
	}
	return exec.Command("explorer", pasta).Start()
}

// LimparArmazenamento apaga os dados de uma categoria pela chave.
func (a *App) LimparArmazenamento(chave string) error {
	switch chave {
	case "modelos_onnx":
		return limparPasta(pastaModelos())
	case "modelos_easyocr":
		return os.RemoveAll(pastaEasyOcr())
	case "cache_pip":
		return os.RemoveAll(pastaCachePip())
	case "logs":
		return limparLogs()
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

	if err := limparPasta(pastaModelos()); err != nil {
		erros = append(erros, fmt.Sprintf("modelos ONNX: %v", err))
	}
	if err := os.RemoveAll(pastaEasyOcr()); err != nil {
		erros = append(erros, fmt.Sprintf("modelos EasyOCR: %v", err))
	}
	if err := os.RemoveAll(pastaCachePip()); err != nil {
		erros = append(erros, fmt.Sprintf("cache pip: %v", err))
	}
	if err := limparLogs(); err != nil {
		erros = append(erros, fmt.Sprintf("logs: %v", err))
	}
	if err := progresso.LimparVocabulario(); err != nil {
		erros = append(erros, fmt.Sprintf("banco: %v", err))
	}

	if len(erros) > 0 {
		return fmt.Errorf("alguns itens não puderam ser apagados — %s", strings.Join(erros, "; "))
	}
	return nil
}

// ----- Auxiliares de remoção -----

// limparPasta apaga o conteúdo de uma pasta, mas mantém a pasta em si.
func limparPasta(pasta string) error {
	entradas, err := os.ReadDir(pasta)
	if err != nil {
		// Guard clause: pasta inexistente já está "limpa"
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	for _, e := range entradas {
		if err := os.RemoveAll(filepath.Join(pasta, e.Name())); err != nil {
			return err
		}
	}
	return nil
}

// limparLogs remove apenas os arquivos .log da pasta de dados.
func limparLogs() error {
	entradas, err := os.ReadDir(pastaDados())
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	for _, e := range entradas {
		if e.IsDir() || !strings.HasSuffix(strings.ToLower(e.Name()), ".log") {
			continue
		}
		if err := os.Remove(filepath.Join(pastaDados(), e.Name())); err != nil {
			return err
		}
	}
	return nil
}
