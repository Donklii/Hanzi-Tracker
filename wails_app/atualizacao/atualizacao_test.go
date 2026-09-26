package atualizacao

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

// ----- Testes de Comparação de Versões -----

func TestCompararVersoes(t *testing.T) {
	testes := []struct {
		nome           string
		versaoA        string
		versaoB        string
		esperadoCmp    int
		esperadoValido bool
	}{
		{
			nome:           "1.10.0 maior que 1.9.9",
			versaoA:        "1.10.0",
			versaoB:        "1.9.9",
			esperadoCmp:    1,
			esperadoValido: true,
		},
		{
			nome:           "0.0.300 menor que 0.1.0",
			versaoA:        "0.0.300",
			versaoB:        "0.1.0",
			esperadoCmp:    -1,
			esperadoValido: true,
		},
		{
			nome:           "Versões idênticas",
			versaoA:        "1.2.3",
			versaoB:        "1.2.3",
			esperadoCmp:    0,
			esperadoValido: true,
		},
		{
			nome:           "Versão A malformada com 2 partes",
			versaoA:        "1.2",
			versaoB:        "1.2.0",
			esperadoCmp:    0,
			esperadoValido: false,
		},
		{
			nome:           "Versão B malformada com texto",
			versaoA:        "1.2.0",
			versaoB:        "1.2.x",
			esperadoCmp:    0,
			esperadoValido: false,
		},
		{
			nome:           "Versão com número negativo",
			versaoA:        "-1.0.0",
			versaoB:        "1.0.0",
			esperadoCmp:    0,
			esperadoValido: false,
		},
	}

	for _, tc := range testes {
		t.Run(tc.nome, func(t *testing.T) {
			cmp, valido := CompararVersoes(tc.versaoA, tc.versaoB)
			if valido != tc.esperadoValido {
				t.Fatalf("Esperado valido=%v, obtido=%v", tc.esperadoValido, valido)
			}
			if valido && cmp != tc.esperadoCmp {
				t.Fatalf("Esperado cmp=%d, obtido=%d", tc.esperadoCmp, cmp)
			}
		})
	}
}


// ----- Testes de Escolha da Melhor Release Estável -----

func TestEscolherMelhorReleaseEstavel(t *testing.T) {
	releases := []ReleaseGitHub{
		{TagName: "motores-ocr-v1", Draft: false, Prerelease: false},
		{TagName: "app-dev", Draft: false, Prerelease: false},
		{TagName: "app-v1.2", Draft: false, Prerelease: false},
		{TagName: "app-v1.9.0", Draft: false, Prerelease: false},
		{TagName: "app-v1.10.0", Draft: false, Prerelease: false},
		{TagName: "app-v1.11.0", Draft: true, Prerelease: false},
		{TagName: "app-v1.12.0", Draft: false, Prerelease: true},
	}

	tagEscolhida, ok := EscolherMelhorReleaseEstavel(releases)
	if !ok {
		t.Fatalf("Deveria ter encontrado uma release válida")
	}

	if tagEscolhida != "app-v1.10.0" {
		t.Fatalf("Esperado tag app-v1.10.0, mas obtido %s", tagEscolhida)
	}
}


func TestEscolherMelhorReleaseEstavelVazia(t *testing.T) {
	releases := []ReleaseGitHub{
		{TagName: "motores-v1", Draft: false, Prerelease: false},
		{TagName: "app-dev", Draft: false, Prerelease: false},
		{TagName: "app-vInvalida", Draft: false, Prerelease: false},
	}

	tagEscolhida, ok := EscolherMelhorReleaseEstavel(releases)
	if ok || tagEscolhida != "" {
		t.Fatalf("Não deveria ter encontrado tag estável válida, obtido: %s", tagEscolhida)
	}
}


// ----- Testes de Avaliação de Necessidade de Atualização (Regra C4) -----

func TestAvaliarNecessidadeAtualizacao(t *testing.T) {
	testes := []struct {
		nome             string
		identidade       IdentidadeBuild
		canalConfigurado string
		alvo             Manifesto
		esperado         bool
	}{
		{
			nome: "Build local (Versao vazia) nunca atualiza",
			identidade: IdentidadeBuild{
				Versao:    "",
				Canal:     CANAL_ESTAVEL,
				Commit:    "abc1234",
				DataBuild: "2026-09-10T12:00:00Z",
			},
			canalConfigurado: CANAL_ESTAVEL,
			alvo: Manifesto{
				Versao:       "1.5.0",
				Commit:       "def5678",
				DataBuild:    "2026-09-10T13:00:00Z",
				Sha256:       "hash",
				TamanhoBytes: 1000,
			},
			esperado: false,
		},
		{
			nome: "Canal dev: commit igual não atualiza",
			identidade: IdentidadeBuild{
				Versao:    "0.0.10",
				Canal:     CANAL_DEV,
				Commit:    "mesmocommit40caracteres0000000000000000",
				DataBuild: "2026-09-10T10:00:00Z",
			},
			canalConfigurado: CANAL_DEV,
			alvo: Manifesto{
				Versao:       "0.0.11",
				Commit:       "mesmocommit40caracteres0000000000000000",
				DataBuild:    "2026-09-10T12:00:00Z",
				Sha256:       "hash",
				TamanhoBytes: 1000,
			},
			esperado: false,
		},
		{
			nome: "Canal dev: commit diferente e data anterior não atualiza",
			identidade: IdentidadeBuild{
				Versao:    "0.0.10",
				Canal:     CANAL_DEV,
				Commit:    "commitvelho1111111111111111111111111111",
				DataBuild: "2026-09-10T15:00:00Z",
			},
			canalConfigurado: CANAL_DEV,
			alvo: Manifesto{
				Versao:       "0.0.11",
				Commit:       "commitnovo22222222222222222222222222222",
				DataBuild:    "2026-09-10T12:00:00Z",
				Sha256:       "hash",
				TamanhoBytes: 1000,
			},
			esperado: false,
		},
		{
			nome: "Canal dev: commit diferente e data posterior atualiza com sucesso",
			identidade: IdentidadeBuild{
				Versao:    "0.0.10",
				Canal:     CANAL_DEV,
				Commit:    "commitvelho1111111111111111111111111111",
				DataBuild: "2026-09-10T10:00:00Z",
			},
			canalConfigurado: CANAL_DEV,
			alvo: Manifesto{
				Versao:       "0.0.11",
				Commit:       "commitnovo22222222222222222222222222222",
				DataBuild:    "2026-09-10T12:00:00Z",
				Sha256:       "hash",
				TamanhoBytes: 1000,
			},
			esperado: true,
		},
		{
			nome: "Canal dev: data malformada não atualiza",
			identidade: IdentidadeBuild{
				Versao:    "0.0.10",
				Canal:     CANAL_DEV,
				Commit:    "commitvelho1111111111111111111111111111",
				DataBuild: "2026-09-10T10:00:00Z",
			},
			canalConfigurado: CANAL_DEV,
			alvo: Manifesto{
				Versao:       "0.0.11",
				Commit:       "commitnovo22222222222222222222222222222",
				DataBuild:    "data-invalida",
				Sha256:       "hash",
				TamanhoBytes: 1000,
			},
			esperado: false,
		},
		{
			nome: "Canal estável (build estável): versão alvo maior atualiza",
			identidade: IdentidadeBuild{
				Versao:    "1.2.0",
				Canal:     CANAL_ESTAVEL,
				Commit:    "commitaaa111",
				DataBuild: "2026-09-10T10:00:00Z",
			},
			canalConfigurado: CANAL_ESTAVEL,
			alvo: Manifesto{
				Versao:       "1.3.0",
				Commit:       "commitbbb222",
				DataBuild:    "2026-09-10T12:00:00Z",
				Sha256:       "hash",
				TamanhoBytes: 1000,
			},
			esperado: true,
		},
		{
			nome: "Canal estável (build estável): versão alvo menor não atualiza",
			identidade: IdentidadeBuild{
				Versao:    "1.3.0",
				Canal:     CANAL_ESTAVEL,
				Commit:    "commitbbb222",
				DataBuild: "2026-09-10T12:00:00Z",
			},
			canalConfigurado: CANAL_ESTAVEL,
			alvo: Manifesto{
				Versao:       "1.2.0",
				Commit:       "commitaaa111",
				DataBuild:    "2026-09-10T10:00:00Z",
				Sha256:       "hash",
				TamanhoBytes: 1000,
			},
			esperado: false,
		},
		{
			nome: "Canal estável (build estável): versão alvo malformada não atualiza",
			identidade: IdentidadeBuild{
				Versao:    "1.2.0",
				Canal:     CANAL_ESTAVEL,
				Commit:    "commitaaa111",
				DataBuild: "2026-09-10T10:00:00Z",
			},
			canalConfigurado: CANAL_ESTAVEL,
			alvo: Manifesto{
				Versao:       "1.3",
				Commit:       "commitbbb222",
				DataBuild:    "2026-09-10T12:00:00Z",
				Sha256:       "hash",
				TamanhoBytes: 1000,
			},
			esperado: false,
		},
		{
			nome: "Canal estável (build dev): downgrade permitido se commit for diferente",
			identidade: IdentidadeBuild{
				Versao:    "0.0.50",
				Canal:     CANAL_DEV,
				Commit:    "commitdev12345",
				DataBuild: "2026-09-10T12:00:00Z",
			},
			canalConfigurado: CANAL_ESTAVEL,
			alvo: Manifesto{
				Versao:       "1.0.0",
				Commit:       "commitestavel67890",
				DataBuild:    "2026-09-01T10:00:00Z",
				Sha256:       "hash",
				TamanhoBytes: 1000,
			},
			esperado: true,
		},
		{
			nome: "Canal estável (build dev): commit igual não atualiza",
			identidade: IdentidadeBuild{
				Versao:    "0.0.50",
				Canal:     CANAL_DEV,
				Commit:    "commitidentico12345",
				DataBuild: "2026-09-10T12:00:00Z",
			},
			canalConfigurado: CANAL_ESTAVEL,
			alvo: Manifesto{
				Versao:       "1.0.0",
				Commit:       "commitidentico12345",
				DataBuild:    "2026-09-01T10:00:00Z",
				Sha256:       "hash",
				TamanhoBytes: 1000,
			},
			esperado: false,
		},
	}

	for _, tc := range testes {
		t.Run(tc.nome, func(t *testing.T) {
			resultado := AvaliarNecessidadeAtualizacao(tc.identidade, tc.canalConfigurado, tc.alvo)
			if resultado != tc.esperado {
				t.Fatalf("Esperado %v, mas obtido %v", tc.esperado, resultado)
			}
		})
	}
}


func TestPrecisaAtualizarPularAtualizacao(t *testing.T) {
	pular := TemArgumentoPularAtualizacao([]string{"caminho/binario", "--pular-atualizacao"})
	if !pular {
		t.Fatalf("Esperado verdadeiro para flag --pular-atualizacao")
	}

	naoPular := TemArgumentoPularAtualizacao([]string{"caminho/binario", "--outro-argumento"})
	if naoPular {
		t.Fatalf("Esperado falso quando não há flag --pular-atualizacao")
	}
}


// ----- Testes de BuscarAlvo com Servidor Mock -----

func TestBuscarAlvoDevComServidorMock(t *testing.T) {
	manifestoMock := Manifesto{
		Versao:       "0.0.99",
		Commit:       "sha40dev00000000000000000000000000000000",
		DataBuild:    "2026-09-10T12:00:00Z",
		Arquivo:      "HanziTracker-amd64-installer.exe",
		Sha256:       "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
		TamanhoBytes: 123456,
	}

	servidor := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == fmt.Sprintf("/%s/%s", TAG_DEV, NomeManifestoSo()) {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(manifestoMock)
			return
		}
		http.NotFound(w, r)
	}))
	defer servidor.Close()

	originalBaseDownload := baseDownloadPadrao
	baseDownloadPadrao = servidor.URL
	defer func() { baseDownloadPadrao = originalBaseDownload }()

	alvo, err := BuscarAlvo(context.Background(), CANAL_DEV)
	if err != nil {
		t.Fatalf("Erro inesperado ao buscar alvo dev: %v", err)
	}

	if alvo.Tag != TAG_DEV {
		t.Fatalf("Esperado tag %s, obtido %s", TAG_DEV, alvo.Tag)
	}
	if alvo.Manifesto.Commit != manifestoMock.Commit {
		t.Fatalf("Esperado commit %s, obtido %s", manifestoMock.Commit, alvo.Manifesto.Commit)
	}
}


func TestBuscarAlvoEstavelComServidorMock(t *testing.T) {
	releasesMock := []ReleaseGitHub{
		{TagName: "app-v1.9.0", Draft: false, Prerelease: false},
		{TagName: "app-v1.10.0", Draft: false, Prerelease: false},
	}

	manifestoMock := Manifesto{
		Versao:       "1.10.0",
		Commit:       "sha40estavel00000000000000000000000000000",
		DataBuild:    "2026-09-10T12:00:00Z",
		Arquivo:      "HanziTracker-amd64-installer.exe",
		Sha256:       "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
		TamanhoBytes: 123456,
	}

	servidor := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/releases" {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(releasesMock)
			return
		}
		if r.URL.Path == fmt.Sprintf("/app-v1.10.0/%s", NomeManifestoSo()) {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(manifestoMock)
			return
		}
		http.NotFound(w, r)
	}))
	defer servidor.Close()

	originalApiReleases := urlApiReleasesPadrao
	originalBaseDownload := baseDownloadPadrao
	urlApiReleasesPadrao = servidor.URL + "/releases"
	baseDownloadPadrao = servidor.URL
	defer func() {
		urlApiReleasesPadrao = originalApiReleases
		baseDownloadPadrao = originalBaseDownload
	}()

	alvo, err := BuscarAlvo(context.Background(), CANAL_ESTAVEL)
	if err != nil {
		t.Fatalf("Erro inesperado ao buscar alvo estável: %v", err)
	}

	if alvo.Tag != "app-v1.10.0" {
		t.Fatalf("Esperado tag app-v1.10.0, obtido %s", alvo.Tag)
	}
	if alvo.Manifesto.Versao != "1.10.0" {
		t.Fatalf("Esperado versao 1.10.0, obtido %s", alvo.Manifesto.Versao)
	}
}


func TestBaixarSha256Vazio(t *testing.T) {
	alvo := Alvo{
		Tag: "app-dev",
		Manifesto: Manifesto{
			Sha256: "",
		},
	}

	_, err := Baixar(alvo, nil)
	if err == nil {
		t.Fatalf("Deveria ter recusado manifesto com sha256 vazio")
	}
}
