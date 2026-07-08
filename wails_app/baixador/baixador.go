// Package baixador reúne as primitivas de download/extração compartilhadas pelos catálogos de
// motores baixáveis (OCR e TTS) e pelos pesos de modelo: download atômico (.tmp + rename) com
// verificação sha256, extração de zip protegida contra Zip Slip e pré-checagem de espaço em disco.
package baixador

import (
	"archive/zip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/shirou/gopsutil/v3/disk"
)

// BaseReleaseMotores é o prefixo das URLs de download dos assets no GitHub Releases deste repo. O
// segmento seguinte é a TAG da release (ex.: "motores-v1"), que versiona os binários com URL imutável.
const BaseReleaseMotores = "https://github.com/Donklii/Hanzi-Tracker/releases/download"

// ArtefatoBaixavel descreve um único .zip publicado: onde baixar, o hash para conferência e o tamanho.
type ArtefatoBaixavel struct {
	Nome         string `json:"nome"`         // nome do arquivo (ex.: "ocr_server.zip")
	Url          string `json:"url"`          // URL HTTPS estável (asset de GitHub Release)
	Sha256       string `json:"sha256"`       // OBRIGATÓRIO — conferido após o download (BaixarArquivo)
	TamanhoBytes int64  `json:"tamanhoBytes"` // para pré-checagem de disco e barra de progresso
}

// VerificarEspacoDisco recusa a operação se o volume não tiver ao menos `bytesNecessarios` livres. Se
// não conseguir medir, não bloqueia (melhor tentar do que impedir sem motivo).
func VerificarEspacoDisco(caminhoRef string, bytesNecessarios int64) error {
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

// ExtrairZip descompacta `origem` em `destino`, protegendo contra Zip Slip (entradas com ../ que
// escapariam da pasta). O conteúdo do zip publicado tem o .exe na raiz (ver builds/build_sidecars_ocr_windows.ps1).
func ExtrairZip(origem, destino string) error {
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

// BaixarArquivo baixa uma URL para um caminho local de forma atômica (.tmp + rename), reportando o
// progresso via callback `onProgresso` (o chamador escolhe o evento — modelos vs. motores). Se
// `sha256Esperado` estiver preenchido, o hash é calculado durante o streaming e conferido antes de
// renomear; divergência aborta o download e apaga o .tmp (evita peso corrompido/adulterado). Vazio =
// verificação pulada (torna-se OBRIGATÓRIO ao baixar executáveis — ver BaixarEExtrairArtefato).
func BaixarArquivo(url, caminhoLocal, sha256Esperado string, onProgresso func(string)) error {
	if onProgresso == nil {
		onProgresso = func(string) {}
	}
	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("falha ao baixar %s: %w", filepath.Base(caminhoLocal), err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("download de %s retornou HTTP %d", filepath.Base(caminhoLocal), resp.StatusCode)
	}

	temp := caminhoLocal + ".tmp"
	f, err := os.Create(temp)
	if err != nil {
		return err
	}

	// Calcula o sha256 no mesmo passo da escrita (streaming), sem reler o arquivo do disco.
	hasher := sha256.New()

	total := resp.ContentLength
	nomeArq := filepath.Base(caminhoLocal)
	var baixado int64
	buf := make([]byte, 64*1024)
	ultimoPct := -1
	for {
		n, errRead := resp.Body.Read(buf)
		if n > 0 {
			if _, errWrite := f.Write(buf[:n]); errWrite != nil {
				f.Close()
				os.Remove(temp)
				return errWrite
			}
			hasher.Write(buf[:n])
			baixado += int64(n)
			if total > 0 {
				pct := int(baixado * 100 / total)
				if pct != ultimoPct {
					ultimoPct = pct
					onProgresso(fmt.Sprintf("Baixando %s (%d%%)…", nomeArq, pct))
				}
			}
		}
		if errRead == io.EOF {
			break
		}
		if errRead != nil {
			f.Close()
			os.Remove(temp)
			return errRead
		}
	}

	if err := f.Close(); err != nil {
		os.Remove(temp)
		return err
	}

	// Verificação de integridade: só quando o manifesto declara o hash esperado.
	if sha256Esperado != "" {
		hashObtido := hex.EncodeToString(hasher.Sum(nil))
		if !strings.EqualFold(hashObtido, sha256Esperado) {
			os.Remove(temp)
			return fmt.Errorf("integridade de %s falhou: sha256 esperado %s, obtido %s", nomeArq, sha256Esperado, hashObtido)
		}
		onProgresso(fmt.Sprintf("%s verificado (sha256 ✓)", nomeArq))
	}

	return os.Rename(temp, caminhoLocal)
}

// BaixarEExtrairArtefato baixa um .zip (com verificação sha256 OBRIGATÓRIA), extrai em `destino` e
// apaga o zip. Faz pré-checagem de disco antes (o download é pesado) usando `raizDados` como
// referência de volume (ex.: a pasta de dados do app). Reaproveita BaixarArquivo.
func BaixarEExtrairArtefato(art ArtefatoBaixavel, destino, raizDados string, onProgresso func(string)) error {
	// sha256 é OBRIGATÓRIO para binários (nos pesos ONNX é opcional; aqui, não).
	if art.Sha256 == "" {
		return fmt.Errorf("recusado: %s não declara sha256 (obrigatório para executáveis)", art.Nome)
	}

	// Pré-checagem de espaço: pico em disco = zip baixado + árvore extraída. Binários + libs do
	// onnxruntime comprimem bem, então a extração passa de 3x o zip; exigimos ~5x de folga.
	if err := VerificarEspacoDisco(raizDados, art.TamanhoBytes*5); err != nil {
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

	// 1) Baixa o zip para dentro do destino (verificação de integridade embutida em BaixarArquivo).
	zipLocal := filepath.Join(destino, art.Nome)
	onProgresso("Iniciando download…")
	if err := BaixarArquivo(art.Url, zipLocal, art.Sha256, onProgresso); err != nil {
		os.Remove(zipLocal)
		return err
	}

	// 2) Extrai (o .exe fica na raiz de `destino`) e descarta o zip.
	onProgresso("Extraindo…")
	if err := ExtrairZip(zipLocal, destino); err != nil {
		os.Remove(zipLocal)
		return fmt.Errorf("falha ao extrair %s: %w", art.Nome, err)
	}
	os.Remove(zipLocal)

	onProgresso("Concluído ✓")
	return nil
}
