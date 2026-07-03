package main

import (
	"regexp"
	"strings"
	"testing"

	"wails_app/config"
)

// ----- Catálogo de motores de voz (motores_tts_manifesto.go) -----

func TestObterMotorTtsBaixavelExistente(t *testing.T) {
	m, ok := ObterMotorTtsBaixavel("Kokoro-82M")
	if !ok {
		t.Fatal("esperava encontrar o motor Kokoro-82M no catálogo de voz")
	}
	if m.Executavel == "" {
		t.Error("motor Kokoro-82M sem Executavel definido")
	}
}

func TestObterMotorTtsBaixavelInexistente(t *testing.T) {
	if _, ok := ObterMotorTtsBaixavel("MotorDeVozQueNaoExiste"); ok {
		t.Error("não deveria encontrar um motor de voz inexistente")
	}
}

// O select "Motor de TTS" (Config.MotorTtsAtivo) só aceita nomes do catálogo — o default da config
// precisa existir nele, senão a leitura em voz alta nasce quebrada para todo usuário novo.
func TestMotorTtsPadraoDaConfigExisteNoCatalogo(t *testing.T) {
	padrao := config.DefaultConfig().MotorTtsAtivo
	if _, ok := ObterMotorTtsBaixavel(padrao); !ok {
		t.Errorf("o motor padrão da config (%q) não existe no catálogo de motores de voz", padrao)
	}
}

// ----- Integridade do catálogo -----

// Diferente do catálogo de OCR, o sha256 pode estar VAZIO enquanto a release motores-tts-v1 não é
// publicada (o download é recusado em runtime nesse estado; ver motores_tts_manifesto.go). Quando
// preenchido, precisa ser um sha256 válido acompanhado do tamanho — o mesmo rigor dos motores de OCR.
func TestCatalogoDeMotoresTtsIntegro(t *testing.T) {
	if len(MotoresTtsBaixaveis) == 0 {
		t.Fatal("catálogo de motores de voz vazio")
	}
	padraoSha256 := regexp.MustCompile(`^[0-9a-f]{64}$`)
	for nome, m := range MotoresTtsBaixaveis {
		if nome != m.Nome {
			t.Errorf("motor %q com Nome divergente da chave do mapa: %q", nome, m.Nome)
		}
		if !strings.HasPrefix(m.Artefato.Url, "https://") {
			t.Errorf("motor %q com URL não-https: %s", nome, m.Artefato.Url)
		}
		if m.Executavel == "" {
			t.Errorf("motor %q sem Executavel", nome)
		}
		if m.Artefato.Sha256 == "" {
			continue // pendente de publicação: aceito, o download é recusado em runtime
		}
		if !padraoSha256.MatchString(m.Artefato.Sha256) {
			t.Errorf("motor %q com sha256 inválido: %q", nome, m.Artefato.Sha256)
		}
		if m.Artefato.TamanhoBytes <= 0 {
			t.Errorf("motor %q publicado (sha256 preenchido) mas com TamanhoBytes inválido: %d", nome, m.Artefato.TamanhoBytes)
		}
	}
}
