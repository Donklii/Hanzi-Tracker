package progresso

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"wails_app/armazenamento"
)

// ----- Cache de áudio TTS -----
// Armazena os arquivos WAV sintetizados em arquivos individuais no diretório dedicado em AppData
// (%APPDATA%\HanziTracker\cache_audio\<motor>\<pinyin>.wav). A chave é o PINYIN (não o hanzi) de
// propósito: hanzis homófonos (马/码/吗, todos "ma") têm a MESMA pronúncia, compartilhando o mesmo
// arquivo de áudio. A chave inclui o motor em subpastas dedicadas porque vozes diferentes não
// podem colidir. Salvar em arquivos desacopla o áudio do banco SQLite (progresso.db), mantendo o
// arquivo do banco enxuto para sincronização na nuvem e reduzindo a concorrência de locks.


// BuscarAudioTts procura um áudio já sintetizado para o par (pinyin, motor).
// Devolve os bytes do WAV, se achou, e um eventual erro de I/O.
func BuscarAudioTts(pinyin, motor string) (audio []byte, achou bool, err error) {
	caminho := caminhoArquivoAudio(pinyin, motor)

	dados, err := os.ReadFile(caminho)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, false, nil
		}
		return nil, false, err
	}

	return dados, true, nil
}


// SalvarAudioTts armazena um áudio sintetizado em arquivo individual no disco.
// A gravação é feita de forma atômica via arquivo temporário para evitar arquivos corrompidos.
func SalvarAudioTts(pinyin, motor string, audio []byte) error {
	if len(audio) == 0 {
		return fmt.Errorf("áudio vazio")
	}

	caminho := caminhoArquivoAudio(pinyin, motor)
	pasta := filepath.Dir(caminho)

	if err := os.MkdirAll(pasta, 0755); err != nil {
		return fmt.Errorf("falha ao criar pasta de cache de áudio: %w", err)
	}

	tempFile := caminho + ".tmp"
	if err := os.WriteFile(tempFile, audio, 0644); err != nil {
		return fmt.Errorf("falha ao gravar arquivo temporário de áudio: %w", err)
	}

	if err := os.Rename(tempFile, caminho); err != nil {
		_ = os.Remove(tempFile)
		return fmt.Errorf("falha ao mover áudio para destino definitivo: %w", err)
	}

	return nil
}


// LimparCacheTts apaga todos os áudios cacheados e o diretório de cache no disco.
func LimparCacheTts() error {
	pasta := armazenamento.PastaCacheAudio()
	if err := os.RemoveAll(pasta); err != nil {
		return err
	}
	return nil
}


// TamanhoCacheTts devolve o tamanho total em bytes e a contagem de arquivos de áudio cacheados.
func TamanhoCacheTts() (bytes int64, arquivos int, err error) {
	pasta := armazenamento.PastaCacheAudio()
	if _, errStat := os.Stat(pasta); errStat != nil {
		if os.IsNotExist(errStat) {
			return 0, 0, nil
		}
		return 0, 0, errStat
	}

	var totalBytes int64
	var contagem int

	err = filepath.Walk(pasta, func(_ string, info os.FileInfo, errWalk error) error {
		if errWalk != nil {
			return nil
		}
		if info != nil && !info.IsDir() && strings.HasSuffix(strings.ToLower(info.Name()), ".wav") {
			totalBytes += info.Size()
			contagem++
		}
		return nil
	})

	return totalBytes, contagem, err
}


// ----- Utilitários de Nomenclatura e Caminhos -----


// caminhoArquivoAudio devolve o caminho absoluto do arquivo WAV para o par (pinyin, motor).
func caminhoArquivoAudio(pinyin, motor string) string {
	pastaMotor := sanitizarNomeMotor(motor)
	nomeArquivo := sanitizarNomeArquivoAudio(pinyin)
	return filepath.Join(armazenamento.PastaCacheAudio(), pastaMotor, nomeArquivo)
}


// sanitizarNomeMotor garante um nome seguro de diretório para o motor.
func sanitizarNomeMotor(motor string) string {
	nome := strings.TrimSpace(motor)
	if nome == "" {
		return "padrao"
	}

	var builder strings.Builder
	for _, r := range nome {
		if r < 32 || r == '\\' || r == '/' || r == ':' || r == '*' || r == '?' || r == '"' || r == '<' || r == '>' || r == '|' {
			builder.WriteRune('_')
		} else {
			builder.WriteRune(r)
		}
	}

	limpo := strings.TrimRight(builder.String(), ". ")
	if limpo == "" {
		return "padrao"
	}
	return limpo
}


// sanitizarNomeArquivoAudio converte a chave pinyin em um nome de arquivo seguro para o SO,
// preservando os diacríticos de tom e a legibilidade no Explorer.
func sanitizarNomeArquivoAudio(pinyin string) string {
	nome := strings.TrimSpace(pinyin)
	if nome == "" {
		return "_vazio.wav"
	}

	var builder strings.Builder
	for _, r := range nome {
		if r < 32 || r == '\\' || r == '/' || r == ':' || r == '*' || r == '?' || r == '"' || r == '<' || r == '>' || r == '|' {
			builder.WriteRune('_')
		} else {
			builder.WriteRune(r)
		}
	}

	limpo := strings.TrimRight(builder.String(), ". ")
	if limpo == "" {
		limpo = "_"
	}

	maiusculo := strings.ToUpper(limpo)
	if maiusculo == "CON" || maiusculo == "PRN" || maiusculo == "AUX" || maiusculo == "NUL" ||
		(len(maiusculo) == 4 && (strings.HasPrefix(maiusculo, "COM") || strings.HasPrefix(maiusculo, "LPT")) && maiusculo[3] >= '1' && maiusculo[3] <= '9') {
		limpo = "_" + limpo
	}

	runas := []rune(limpo)
	if len(runas) > 80 {
		hash := sha256.Sum256([]byte(pinyin))
		limpo = string(runas[:70]) + "_" + hex.EncodeToString(hash[:4])
	}

	return limpo + ".wav"
}
