package motoresocr

import (
	"encoding/json"
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

// ----- Integridade do catálogo: sha256 obrigatório e URL HTTPS -----

func TestCatalogoDeMotoresIntegro(t *testing.T) {
	if len(MotoresBaixaveis) == 0 {
		t.Fatal("catálogo de motores vazio")
	}
	for nome, m := range MotoresBaixaveis {
		if m.Artefato.Sha256 == "" {
			t.Errorf("motor %q sem sha256 (obrigatório para binários)", nome)
		}
		if !strings.HasPrefix(m.Artefato.Url, "https://") {
			t.Errorf("motor %q com URL não-https: %s", nome, m.Artefato.Url)
		}
		if m.Artefato.TamanhoBytes <= 0 {
			t.Errorf("motor %q com TamanhoBytes inválido: %d", nome, m.Artefato.TamanhoBytes)
		}
		if m.Executavel == "" {
			t.Errorf("motor %q sem Executavel", nome)
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
