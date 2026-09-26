// Package atualizacao gerencia a verificação, decisão e download de atualizações automáticas do aplicativo.
package atualizacao

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"wails_app/baixador"
)

// ----- Constantes do Sistema de Atualização -----

const (
	CANAL_ESTAVEL        = "estavel"
	CANAL_DEV            = "dev"
	TAG_DEV              = "app-dev"
	PREFIXO_TAG_ESTAVEL  = "app-v"
	USER_AGENT_APLICACAO = "HanziTracker-App"
	ARGUMENTO_PULAR      = "--pular-atualizacao"
)

// ----- Variáveis de Identidade do Build (preenchidas via ldflags na CI) -----

var (
	Versao    string
	Canal     string
	Commit    string
	DataBuild string
)

// Variáveis internas para configuração de endpoints (substituíveis em testes)
var (
	urlApiReleasesPadrao = "https://api.github.com/repos/Donklii/Hanzi-Tracker/releases?per_page=100"
	baseDownloadPadrao   = baixador.BaseReleaseMotores
)

// ----- Tipos e Estruturas de Dados -----

// Manifesto descreve os metadados do pacote de atualização publicado para um SO.
type Manifesto struct {
	Versao       string `json:"versao"`
	Commit       string `json:"commit"`
	DataBuild    string `json:"dataBuild"`
	Arquivo      string `json:"arquivo"`
	Sha256       string `json:"sha256"`
	TamanhoBytes int64  `json:"tamanhoBytes"`
}

// Alvo encapsula a tag de release encontrada e o manifesto associado baixado.
type Alvo struct {
	Tag       string
	Manifesto Manifesto
}

// IdentidadeBuild consolida as propriedades do build atual para avaliações puras de atualização.
type IdentidadeBuild struct {
	Versao    string
	Canal     string
	Commit    string
	DataBuild string
}

// ReleaseGitHub representa uma entrada mínima da API de Releases do GitHub.
type ReleaseGitHub struct {
	TagName    string `json:"tag_name"`
	Draft      bool   `json:"draft"`
	Prerelease bool   `json:"prerelease"`
}


// ----- Funções Principais de Identidade -----

// BuildLocal indica se a versão atual é de desenvolvimento local (sem Versao preenchida).
func BuildLocal() bool {
	return Versao == ""
}


// CanalDoBuild devolve o canal ativo deste binário ("dev" ou "estavel").
func CanalDoBuild() string {
	if Canal != "" {
		return Canal
	}
	return CANAL_ESTAVEL
}


// ObterIdentidadeAtual retorna os dados de identidade deste executável em execução.
func ObterIdentidadeAtual() IdentidadeBuild {
	return IdentidadeBuild{
		Versao:    Versao,
		Canal:     CanalDoBuild(),
		Commit:    Commit,
		DataBuild: DataBuild,
	}
}


// NomeManifestoSo devolve o nome do arquivo de manifesto de acordo com a plataforma em execução.
func NomeManifestoSo() string {
	if runtime.GOOS == "windows" {
		return "atualizacao-windows.json"
	}
	return "atualizacao-linux.json"
}


// ----- Busca de Alvo de Atualização -----

// BuscarAlvo busca a release-alvo adequada para o canal configurado e baixa o respectivo manifesto.
func BuscarAlvo(ctx context.Context, canalConfigurado string) (Alvo, error) {
	tagAlvo, err := resolverTagAlvo(ctx, canalConfigurado)
	if err != nil {
		return Alvo{}, err
	}

	manifesto, err := baixarManifesto(ctx, tagAlvo)
	if err != nil {
		return Alvo{}, err
	}

	return Alvo{
		Tag:       tagAlvo,
		Manifesto: manifesto,
	}, nil
}


func resolverTagAlvo(ctx context.Context, canalConfigurado string) (string, error) {
	if canalConfigurado == CANAL_DEV {
		return TAG_DEV, nil
	}

	if canalConfigurado != CANAL_ESTAVEL {
		return "", fmt.Errorf("canal de atualização desconhecido: %s", canalConfigurado)
	}

	return buscarMelhorTagEstavelNoGitHub(ctx)
}


func buscarMelhorTagEstavelNoGitHub(ctx context.Context) (string, error) {
	requisicao, err := http.NewRequestWithContext(ctx, http.MethodGet, urlApiReleasesPadrao, nil)
	if err != nil {
		return "", fmt.Errorf("falha ao criar requisição de releases: %w", err)
	}
	requisicao.Header.Set("User-Agent", USER_AGENT_APLICACAO)
	requisicao.Header.Set("Accept", "application/vnd.github+json")

	resposta, err := http.DefaultClient.Do(requisicao)
	if err != nil {
		return "", fmt.Errorf("falha ao consultar API de releases do GitHub: %w", err)
	}
	defer resposta.Body.Close()

	if resposta.StatusCode != http.StatusOK {
		return "", fmt.Errorf("API do GitHub retornou status HTTP %d", resposta.StatusCode)
	}

	var releases []ReleaseGitHub
	decodificador := json.NewDecoder(resposta.Body)
	if err := decodificador.Decode(&releases); err != nil {
		return "", fmt.Errorf("falha ao decodificar releases do GitHub: %w", err)
	}

	tagEncontrada, ok := EscolherMelhorReleaseEstavel(releases)
	if !ok {
		return "", errors.New("nenhuma release estável válida encontrada")
	}

	return tagEncontrada, nil
}


func baixarManifesto(ctx context.Context, tag string) (Manifesto, error) {
	urlManifesto := fmt.Sprintf("%s/%s/%s", baseDownloadPadrao, tag, NomeManifestoSo())
	requisicao, err := http.NewRequestWithContext(ctx, http.MethodGet, urlManifesto, nil)
	if err != nil {
		return Manifesto{}, fmt.Errorf("falha ao criar requisição do manifesto: %w", err)
	}
	requisicao.Header.Set("User-Agent", USER_AGENT_APLICACAO)

	resposta, err := http.DefaultClient.Do(requisicao)
	if err != nil {
		return Manifesto{}, fmt.Errorf("falha ao baixar manifesto de %s: %w", urlManifesto, err)
	}
	defer resposta.Body.Close()

	if resposta.StatusCode != http.StatusOK {
		return Manifesto{}, fmt.Errorf("download do manifesto retornou status HTTP %d", resposta.StatusCode)
	}

	var manifesto Manifesto
	decodificador := json.NewDecoder(resposta.Body)
	if err := decodificador.Decode(&manifesto); err != nil {
		return Manifesto{}, fmt.Errorf("falha ao decodificar JSON do manifesto: %w", err)
	}

	return manifesto, nil
}


// ----- Decisão de Atualização (Regra C4) -----

// PrecisaAtualizar verifica se o executável em execução deve ser atualizado para o manifesto alvo.
// Não olha o --pular-atualizacao: a flag só vale para a verificação automática do boot (quem a
// checa é verificarAtualizacaoInicial). O "Verificar agora" da UI tem de funcionar também na sessão
// reaberta com ela depois de um UAC negado.
func PrecisaAtualizar(canalConfigurado string, alvo Manifesto) bool {
	if BuildLocal() {
		return false
	}

	return AvaliarNecessidadeAtualizacao(ObterIdentidadeAtual(), canalConfigurado, alvo)
}


// AvaliarNecessidadeAtualizacao é a função pura que executa as regras da tabela C4.
func AvaliarNecessidadeAtualizacao(identidade IdentidadeBuild, canalConfigurado string, alvo Manifesto) bool {
	if identidade.Versao == "" {
		return false
	}

	canalBuild := identidade.Canal
	if canalBuild == "" {
		canalBuild = CANAL_ESTAVEL
	}

	if canalConfigurado == CANAL_DEV {
		return avaliarAtualizacaoDev(identidade, alvo)
	}

	if canalConfigurado == CANAL_ESTAVEL {
		if canalBuild == CANAL_ESTAVEL {
			return avaliarAtualizacaoEstavelParaEstavel(identidade, alvo)
		}
		if canalBuild == CANAL_DEV {
			return avaliarDowngradeDevParaEstavel(identidade, alvo)
		}
	}

	return false
}


func avaliarAtualizacaoDev(identidade IdentidadeBuild, alvo Manifesto) bool {
	if alvo.Commit == "" || identidade.Commit == "" {
		return false
	}
	if strings.EqualFold(alvo.Commit, identidade.Commit) {
		return false
	}

	tempoAlvo, errAlvo := time.Parse(time.RFC3339, alvo.DataBuild)
	if errAlvo != nil {
		return false
	}

	tempoMeu, errMeu := time.Parse(time.RFC3339, identidade.DataBuild)
	if errMeu != nil {
		return false
	}

	return tempoAlvo.After(tempoMeu)
}


func avaliarAtualizacaoEstavelParaEstavel(identidade IdentidadeBuild, alvo Manifesto) bool {
	comparacao, ok := CompararVersoes(alvo.Versao, identidade.Versao)
	if !ok {
		return false
	}
	return comparacao > 0
}


func avaliarDowngradeDevParaEstavel(identidade IdentidadeBuild, alvo Manifesto) bool {
	if alvo.Commit == "" || identidade.Commit == "" {
		return false
	}
	return !strings.EqualFold(alvo.Commit, identidade.Commit)
}


// ----- Download do Pacote -----

// Baixar efetua a checagem de integridade, espaço em disco e download seguro do pacote de atualização.
func Baixar(alvo Alvo, onProgresso func(string)) (string, error) {
	if strings.TrimSpace(alvo.Manifesto.Sha256) == "" {
		return "", errors.New("sha256 do pacote está vazio no manifesto")
	}

	pastaDestino := filepath.Join(os.TempDir(), "HanziTracker-atualizacao")
	if err := os.MkdirAll(pastaDestino, 0755); err != nil {
		return "", fmt.Errorf("falha ao criar pasta temporária de atualização: %w", err)
	}

	if err := baixador.VerificarEspacoDisco(os.TempDir(), alvo.Manifesto.TamanhoBytes*2); err != nil {
		return "", err
	}

	caminhoPacote := filepath.Join(pastaDestino, alvo.Manifesto.Arquivo)
	urlPacote := fmt.Sprintf("%s/%s/%s", baseDownloadPadrao, alvo.Tag, alvo.Manifesto.Arquivo)

	if err := baixador.BaixarArquivo(urlPacote, caminhoPacote, alvo.Manifesto.Sha256, onProgresso); err != nil {
		return "", err
	}

	return caminhoPacote, nil
}


// ----- Funções Utilitárias -----

// TemArgumentoPularAtualizacao verifica se a flag --pular-atualizacao consta nos argumentos.
func TemArgumentoPularAtualizacao(argumentos []string) bool {
	for _, arg := range argumentos {
		if arg == ARGUMENTO_PULAR {
			return true
		}
	}
	return false
}


// EscolherMelhorReleaseEstavel filtra e elege a maior tag numérica de release estável disponível.
func EscolherMelhorReleaseEstavel(releases []ReleaseGitHub) (string, bool) {
	var melhorTag string
	var melhorVersao string

	for _, release := range releases {
		if release.Draft || release.Prerelease {
			continue
		}
		if !strings.HasPrefix(release.TagName, PREFIXO_TAG_ESTAVEL) {
			continue
		}

		candidataVersao := strings.TrimPrefix(release.TagName, PREFIXO_TAG_ESTAVEL)
		if _, ok := analisarVersaoNumerica(candidataVersao); !ok {
			continue
		}

		if melhorTag == "" {
			melhorTag = release.TagName
			melhorVersao = candidataVersao
			continue
		}

		resultado, ok := CompararVersoes(candidataVersao, melhorVersao)
		if ok && resultado > 0 {
			melhorTag = release.TagName
			melhorVersao = candidataVersao
		}
	}

	if melhorTag == "" {
		return "", false
	}

	return melhorTag, true
}


// CompararVersoes compara duas versões semânticas no formato X.Y.Z estritamente numérico.
// Retorna 1 se versaoA > versaoB, -1 se versaoA < versaoB, 0 se iguais, e false se alguma for inválida.
func CompararVersoes(versaoA, versaoB string) (int, bool) {
	partesA, okA := analisarVersaoNumerica(versaoA)
	partesB, okB := analisarVersaoNumerica(versaoB)
	if !okA || !okB {
		return 0, false
	}

	for i := 0; i < 3; i++ {
		if partesA[i] > partesB[i] {
			return 1, true
		}
		if partesA[i] < partesB[i] {
			return -1, true
		}
	}

	return 0, true
}


func analisarVersaoNumerica(versao string) ([]int, bool) {
	partes := strings.Split(strings.TrimSpace(versao), ".")
	if len(partes) != 3 {
		return nil, false
	}

	numeros := make([]int, 3)
	for i, parte := range partes {
		if parte == "" {
			return nil, false
		}
		val, err := strconv.Atoi(parte)
		if err != nil || val < 0 {
			return nil, false
		}
		numeros[i] = val
	}

	return numeros, true
}
