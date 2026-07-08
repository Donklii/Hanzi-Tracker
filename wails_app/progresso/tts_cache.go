package progresso

import (
	"database/sql"
	"errors"
	"fmt"
)

// ----- Cache de áudio TTS -----
// Espelha o traducao_cache.go: tabela dentro do progresso.db (não é arquivo próprio) que guarda o
// WAV sintetizado por (pinyin, motor). A chave é o PINYIN (não o hanzi) de propósito: hanzis
// homófonos (马/码/吗, todos "ma") têm a MESMA pronúncia, então compartilham um único áudio — a
// tradução hanzi→pinyin é feita pelo chamador (App.traduzirHanziParaChaveTts). Sínteses repetidas —
// hover no mesmo card, revisão — saem instantâneas e sem custo de CPU do torch. A chave inclui o
// motor porque Kokoro e ChatTTS têm vozes diferentes: trocar o select não pode servir o áudio do
// motor antigo.

// BuscarAudioTts procura um áudio já sintetizado para o par (pinyin, motor).
// Devolve os bytes do WAV, se achou, e um eventual erro de banco.
func BuscarAudioTts(pinyin, motor string) (audio []byte, achou bool, err error) {
	if db == nil {
		return nil, false, fmt.Errorf("DB não inicializado")
	}

	err = db.QueryRow(
		"SELECT audio FROM tts_audio_cache WHERE pinyin = ? AND motor = ?",
		pinyin, motor,
	).Scan(&audio)

	if err != nil {
		// sql.ErrNoRows não é um erro de fato — só indica cache miss.
		if errors.Is(err, sql.ErrNoRows) {
			return nil, false, nil
		}
		return nil, false, err
	}

	return audio, true, nil
}

// SalvarAudioTts armazena um áudio sintetizado no cache, indexado pela pronúncia (pinyin, motor).
// Usa INSERT OR IGNORE para que a constraint UNIQUE sirva de rede de segurança contra duplicatas. O
// chamador DEVE ter verificado BuscarAudioTts ANTES.
func SalvarAudioTts(pinyin, motor string, audio []byte) error {
	if db == nil {
		return fmt.Errorf("DB não inicializado")
	}

	_, err := db.Exec(
		"INSERT OR IGNORE INTO tts_audio_cache (pinyin, motor, audio) VALUES (?, ?, ?)",
		pinyin, motor, audio,
	)
	return err
}

// LimparCacheTts apaga todos os áudios cacheados e recupera o espaço em disco.
func LimparCacheTts() error {
	if db == nil {
		return fmt.Errorf("DB não inicializado")
	}
	if _, err := db.Exec("DELETE FROM tts_audio_cache"); err != nil {
		return err
	}
	// Recupera o espaço em disco liberado pelos registros apagados.
	_, _ = db.Exec("VACUUM")
	return nil
}

// TamanhoCacheTts devolve uma estimativa do tamanho em bytes e a contagem de áudios cacheados.
// Como o cache é uma tabela dentro de progresso.db (não um arquivo próprio), o tamanho é
// aproximado via SUM(LENGTH(...)).
func TamanhoCacheTts() (bytes int64, linhas int, err error) {
	if db == nil {
		return 0, 0, fmt.Errorf("DB não inicializado")
	}

	// Contagem de linhas
	err = db.QueryRow("SELECT COUNT(*) FROM tts_audio_cache").Scan(&linhas)
	if err != nil {
		return 0, 0, err
	}

	// Guard clause: tabela vazia — sem necessidade de calcular SUM
	if linhas == 0 {
		return 0, 0, nil
	}

	// Tamanho aproximado: soma dos comprimentos dos blobs de áudio (domina o tamanho da tabela)
	err = db.QueryRow(
		"SELECT COALESCE(SUM(LENGTH(audio)), 0) FROM tts_audio_cache",
	).Scan(&bytes)
	if err != nil {
		return 0, linhas, err
	}

	return bytes, linhas, nil
}
