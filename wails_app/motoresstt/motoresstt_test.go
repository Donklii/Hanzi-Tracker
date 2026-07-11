package motoresstt

import (
	"encoding/json"
	"regexp"
	"strings"
	"testing"

	"wails_app/config"
)

// ----- Catálogo de motores de STT (manifesto.go) -----

func TestObterMotorSttBaixavelExistente(t *testing.T) {
	for _, nome := range []string{"Paraformer-ZH", "Zipformer-ZH-Streaming"} {
		m, ok := ObterMotorSttBaixavel(nome)
		if !ok {
			t.Fatalf("esperava encontrar o motor %s no catálogo de STT", nome)
		}
		if m.Executavel == "" {
			t.Errorf("motor %s sem Executavel definido", nome)
		}
	}
}

func TestObterMotorSttBaixavelInexistente(t *testing.T) {
	if _, ok := ObterMotorSttBaixavel("MotorDeSttQueNaoExiste"); ok {
		t.Error("não deveria encontrar um motor de STT inexistente")
	}
}

// O motor de STT ativo (Config.MotorSttAtivo) só aceita nomes do catálogo — o default da config
// precisa existir nele, senão a revisão de pronúncia nasce quebrada para todo usuário novo.
func TestMotorSttPadraoDaConfigExisteNoCatalogo(t *testing.T) {
	padrao := config.DefaultConfig().MotorSttAtivo
	if _, ok := ObterMotorSttBaixavel(padrao); !ok {
		t.Errorf("o motor padrão da config (%q) não existe no catálogo de motores de STT", padrao)
	}
}

// ----- Integridade do catálogo -----

// Como no TTS, o sha256 pode estar VAZIO enquanto a release motores-stt-*-v1 não é publicada (o
// download é recusado em runtime nesse estado; ver manifesto.go). Quando preenchido, precisa ser
// um sha256 válido acompanhado do tamanho.
func TestCatalogoDeMotoresSttIntegro(t *testing.T) {
	if len(MotoresSttBaixaveis) == 0 {
		t.Fatal("catálogo de motores de STT vazio")
	}
	padraoSha256 := regexp.MustCompile(`^[0-9a-f]{64}$`)
	for nome, m := range MotoresSttBaixaveis {
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

// ----- Consistência com artefatos_stt*.json (fonte dos campos voláteis, reescrita pelo CI) -----
// init() injeta url/sha256/tamanho a partir do JSON embutido. Todo motor do catálogo precisa ter
// uma entrada correspondente lá, e a URL injetada precisa carregar a tag da release. Roda no CI
// ANTES do commit automático do manifesto, garantindo que o JSON reescrito casa com o catálogo.
func TestCatalogoSttCasaComArtefatosJSON(t *testing.T) {
	var dados manifestoArtefatosStt
	if err := json.Unmarshal(artefatosSttBrutos, &dados); err != nil {
		t.Fatalf("artefatos_stt.json inválido: %v", err)
	}
	if dados.Tag == "" {
		t.Fatal("artefatos_stt.json sem tag de release")
	}
	for nome, m := range MotoresSttBaixaveis {
		if _, ok := dados.Artefatos[m.Artefato.Nome]; !ok {
			t.Errorf("motor de STT %q (zip %q) sem entrada em artefatos_stt.json", nome, m.Artefato.Nome)
		}
		if !strings.Contains(m.Artefato.Url, "/"+dados.Tag+"/") {
			t.Errorf("motor de STT %q com URL sem a tag %q: %s", nome, dados.Tag, m.Artefato.Url)
		}
	}
}
