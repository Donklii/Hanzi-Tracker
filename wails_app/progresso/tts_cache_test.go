package progresso

import (
	"bytes"
	"strings"
	"testing"
)

// ----- Testes de Nomenclatura e Sanitização -----


func TestSanitizarNomeArquivoAudio(t *testing.T) {
	casos := []struct {
		entrada  string
		esperado string
	}{
		{"nǐ hǎo", "nǐ hǎo.wav"},
		{"mǎ", "mǎ.wav"},
		{"好", "好.wav"},
		{"a/b:c*d?e\"f<g>h|i", "a_b_c_d_e_f_g_h_i.wav"},
		{"", "_vazio.wav"},
		{"   ", "_vazio.wav"},
		{"CON", "_CON.wav"},
		{"aux", "_aux.wav"},
		{"com1", "_com1.wav"},
		{"teste.", "teste.wav"},
		{"teste ", "teste.wav"},
	}

	for _, caso := range casos {
		obtido := sanitizarNomeArquivoAudio(caso.entrada)
		if obtido != caso.esperado {
			t.Errorf("sanitizarNomeArquivoAudio(%q): esperado %q, obtido %q", caso.entrada, caso.esperado, obtido)
		}
	}
}


func TestSanitizarNomeArquivoAudioTruncamento(t *testing.T) {
	textoLongo := strings.Repeat("pinyin-muito-longo-com-bastante-caracteres-", 5)
	obtido := sanitizarNomeArquivoAudio(textoLongo)

	if len([]rune(obtido)) > 90 {
		t.Errorf("nome higienizado não deveria exceder limite de caracteres: tamanho %d", len([]rune(obtido)))
	}

	if !strings.HasSuffix(obtido, ".wav") {
		t.Errorf("nome higienizado deveria ter extensão .wav: obtido %q", obtido)
	}
}


func TestSanitizarNomeMotor(t *testing.T) {
	casos := []struct {
		entrada  string
		esperado string
	}{
		{"Kokoro-82M", "Kokoro-82M"},
		{"ChatTTS", "ChatTTS"},
		{"motor/com:caracteres*invalidos", "motor_com_caracteres_invalidos"},
		{"", "padrao"},
		{"   ", "padrao"},
	}

	for _, caso := range casos {
		obtido := sanitizarNomeMotor(caso.entrada)
		if obtido != caso.esperado {
			t.Errorf("sanitizarNomeMotor(%q): esperado %q, obtido %q", caso.entrada, caso.esperado, obtido)
		}
	}
}


// ----- Testes de Salvamento e Leitura do Cache -----


func TestSalvarEhBuscarAudioTts(t *testing.T) {
	_ = prepararBancoDeTeste(t)

	pinyin := "nǐ hǎo"
	motor := "Kokoro-82M"
	conteudoAudio := []byte("RIFFmockwavdata123456")

	// Cache miss inicial
	audio, achou, err := BuscarAudioTts(pinyin, motor)
	if err != nil {
		t.Fatalf("BuscarAudioTts antes de salvar: %v", err)
	}
	if achou {
		t.Fatalf("esperava cache miss, mas achou áudio inexistente")
	}
	if audio != nil {
		t.Fatalf("esperava slice nil em cache miss")
	}

	// Salva áudio no cache
	if err := SalvarAudioTts(pinyin, motor, conteudoAudio); err != nil {
		t.Fatalf("SalvarAudioTts: %v", err)
	}

	// Cache hit
	audio, achou, err = BuscarAudioTts(pinyin, motor)
	if err != nil {
		t.Fatalf("BuscarAudioTts após salvar: %v", err)
	}
	if !achou {
		t.Fatalf("esperava cache hit para o áudio salvo")
	}
	if !bytes.Equal(audio, conteudoAudio) {
		t.Fatalf("áudio lido difere do áudio gravado")
	}
}


func TestSalvarAudioTtsRejeitaVazio(t *testing.T) {
	_ = prepararBancoDeTeste(t)

	err := SalvarAudioTts("teste", "Kokoro-82M", []byte{})
	if err == nil {
		t.Fatalf("SalvarAudioTts deveria rejeitar áudio vazio")
	}
}


// ----- Testes de Medição e Limpeza do Cache -----


func TestTamanhoEhLimparCacheTts(t *testing.T) {
	_ = prepararBancoDeTeste(t)

	audio1 := []byte("audio-um-12345")
	audio2 := []byte("audio-dois-67890-abcdef")

	if err := SalvarAudioTts("palavra1", "Kokoro-82M", audio1); err != nil {
		t.Fatalf("SalvarAudioTts 1: %v", err)
	}
	if err := SalvarAudioTts("palavra2", "Kokoro-82M", audio2); err != nil {
		t.Fatalf("SalvarAudioTts 2: %v", err)
	}

	bytesTotal, contagem, err := TamanhoCacheTts()
	if err != nil {
		t.Fatalf("TamanhoCacheTts: %v", err)
	}
	if contagem != 2 {
		t.Fatalf("esperava 2 arquivos cacheados, obtido %d", contagem)
	}
	esperadoBytes := int64(len(audio1) + len(audio2))
	if bytesTotal != esperadoBytes {
		t.Fatalf("esperava %d bytes totais, obtido %d", esperadoBytes, bytesTotal)
	}

	// Limpar cache
	if err := LimparCacheTts(); err != nil {
		t.Fatalf("LimparCacheTts: %v", err)
	}

	bytesAposLimpar, contagemAposLimpar, err := TamanhoCacheTts()
	if err != nil {
		t.Fatalf("TamanhoCacheTts após limpar: %v", err)
	}
	if bytesAposLimpar != 0 || contagemAposLimpar != 0 {
		t.Fatalf("esperava cache zerado após limpeza, obtido %d bytes e %d arquivos", bytesAposLimpar, contagemAposLimpar)
	}

	// Busca deve dar cache miss
	_, achou, err := BuscarAudioTts("palavra1", "Kokoro-82M")
	if err != nil {
		t.Fatalf("BuscarAudioTts após limpar: %v", err)
	}
	if achou {
		t.Fatalf("áudio ainda encontrado após LimparCacheTts")
	}
}


// ----- Testes de Migração do Banco para Arquivos -----


func TestMigrarCacheTtsParaArquivos(t *testing.T) {
	_ = prepararBancoDeTeste(t)

	// Cria tabela tts_audio_cache antiga artificialmente para testar a migração
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS tts_audio_cache (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			pinyin TEXT NOT NULL,
			motor TEXT NOT NULL,
			audio BLOB NOT NULL,
			data_add DATETIME DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(pinyin, motor)
		);
	`)
	if err != nil {
		t.Fatalf("criar tabela antiga: %v", err)
	}

	audioExemplo := []byte("audio-legado-migrado-com-sucesso")
	_, err = db.Exec(
		"INSERT INTO tts_audio_cache (pinyin, motor, audio) VALUES (?, ?, ?)",
		"xiè xie", "Kokoro-82M", audioExemplo,
	)
	if err != nil {
		t.Fatalf("inserir dados antigos: %v", err)
	}

	// Executa migração
	if err := migrarCacheTtsParaArquivos(); err != nil {
		t.Fatalf("migrarCacheTtsParaArquivos: %v", err)
	}

	// 1. Tabela deve ter sido dropada do SQLite
	var existe int
	err = db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='tts_audio_cache'").Scan(&existe)
	if err != nil {
		t.Fatalf("verificar sqlite_master: %v", err)
	}
	if existe != 0 {
		t.Fatalf("tabela tts_audio_cache deveria ter sido dropada do SQLite")
	}

	// 2. Arquivo deve ter sido migrado e acessível via BuscarAudioTts
	audioLido, achou, err := BuscarAudioTts("xiè xie", "Kokoro-82M")
	if err != nil {
		t.Fatalf("BuscarAudioTts após migração: %v", err)
	}
	if !achou {
		t.Fatalf("áudio migrado não foi encontrado no novo sistema de arquivos")
	}
	if !bytes.Equal(audioLido, audioExemplo) {
		t.Fatalf("conteúdo do áudio migrado difere do original")
	}

	// 3. Idempotência: rodar novamente não deve quebrar nem emitir erro
	if err := migrarCacheTtsParaArquivos(); err != nil {
		t.Fatalf("migrarCacheTtsParaArquivos (segunda execução): %v", err)
	}
}
