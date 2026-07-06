package motoresocr

import (
	"encoding/json"
	"regexp"
	"strings"
	"testing"
)

// ----- Catálogo de motores (manifesto.go) -----

func TestObterMotorBaixavelExistente(t *testing.T) {
	m, ok := ObterMotorBaixavel("RapidOCR")
	if !ok {
		t.Fatal("esperava encontrar o motor RapidOCR no catálogo")
	}
	if m.Executavel == "" {
		t.Error("motor RapidOCR sem Executavel definido")
	}
}

func TestObterMotorBaixavelInexistente(t *testing.T) {
	if _, ok := ObterMotorBaixavel("MotorQueNaoExiste"); ok {
		t.Error("não deveria encontrar um motor inexistente")
	}
}

func TestMotorOcrPadrao(t *testing.T) {
	m, ok := MotorOcrPadrao()
	if !ok {
		t.Fatal("esperava um motor padrão no catálogo")
	}
	if !m.Padrao {
		t.Error("o motor devolvido por MotorOcrPadrao não está marcado como Padrao")
	}
}

// ----- Integridade do catálogo: URL HTTPS + sha256 válido quando publicado -----

// O sha256 pode estar VAZIO enquanto o artefato do motor não é publicado PARA O SO deste runner
// (ex.: os zips Linux antes da primeira release motores-ocr-linux-v*; o download é recusado em
// runtime nesse estado — mesmo modelo dos motores de voz). Quando preenchido, precisa ser um sha256
// válido acompanhado do tamanho.
func TestCatalogoDeMotoresIntegro(t *testing.T) {
	if len(MotoresBaixaveis) == 0 {
		t.Fatal("catálogo de motores vazio")
	}
	padraoSha256 := regexp.MustCompile(`^[0-9a-f]{64}$`)
	for nome, m := range MotoresBaixaveis {
		if !strings.HasPrefix(m.Artefato.Url, "https://") {
			t.Errorf("motor %q com URL não-https: %s", nome, m.Artefato.Url)
		}
		if m.Executavel == "" {
			t.Errorf("motor %q sem Executavel", nome)
		}
		if m.Artefato.Sha256 == "" {
			continue // pendente de publicação neste SO: aceito, o download é recusado em runtime
		}
		if !padraoSha256.MatchString(m.Artefato.Sha256) {
			t.Errorf("motor %q com sha256 inválido: %q", nome, m.Artefato.Sha256)
		}
		if m.Artefato.TamanhoBytes <= 0 {
			t.Errorf("motor %q publicado (sha256 preenchido) mas com TamanhoBytes inválido: %d", nome, m.Artefato.TamanhoBytes)
		}
	}
}

// ----- Consistência com artefatos_ocr.json (fonte dos campos voláteis, reescrita pelo CI) -----
// init() injeta url/sha256/tamanho a partir do JSON embutido. Todo motor do catálogo precisa ter uma
// entrada correspondente lá, e a URL injetada precisa carregar a tag da release — senão o motor
// nasce sem sha256/URL válida e o download é recusado em runtime. Este teste roda no CI ANTES do
// commit automático do manifesto, garantindo que o JSON reescrito casa com o catálogo.
func TestCatalogoCasaComArtefatosJSON(t *testing.T) {
	var dados manifestoArtefatosOcr
	if err := json.Unmarshal(artefatosOcrBrutos, &dados); err != nil {
		t.Fatalf("artefatos_ocr.json inválido: %v", err)
	}
	if dados.Tag == "" {
		t.Fatal("artefatos_ocr.json sem tag de release")
	}
	for nome, m := range MotoresBaixaveis {
		if _, ok := dados.Artefatos[m.Artefato.Nome]; !ok {
			t.Errorf("motor %q (zip %q) sem entrada em artefatos_ocr.json", nome, m.Artefato.Nome)
		}
		if !strings.Contains(m.Artefato.Url, "/"+dados.Tag+"/") {
			t.Errorf("motor %q com URL sem a tag %q: %s", nome, dados.Tag, m.Artefato.Url)
		}
	}
}
