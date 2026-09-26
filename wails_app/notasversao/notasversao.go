package notasversao

import (
	"bytes"
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"wails_app/armazenamento"
	"wails_app/dicionario"
)

// ----- Constantes e Variáveis -----

const (
	NOME_ARQUIVO_ESTADO = "notas_vistas.json"
	IDIOMA_PADRAO_NOTAS = "pt-BR"
	PREFIXO_TITULO      = "# "
	PREFIXO_GRUPO       = "## "
	PREFIXO_ITEM        = "- "
	FORMATO_DATA_NOTAS  = "2006-01-02"
)

type tipoLinha int

const (
	tipoLinhaNenhum tipoLinha = iota
	tipoLinhaTitulo
	tipoLinhaGrupo
	tipoLinhaItem
	tipoLinhaContinuacao
)

//go:embed notas
var fsNotas embed.FS

var padraoNomeNota = regexp.MustCompile(`^(\d{4}-\d{2}-\d{2})-([a-z0-9]+(?:-[a-z0-9]+)*)(?:\.([A-Za-z-]+))?\.md$`)

var muEstado sync.Mutex


// ----- DTOs e Tipos Exportados -----

// Nota representa as notas de versão de uma determinada publicação prontas para exibição.
type Nota struct {
	Id     string      `json:"id"`
	Data   string      `json:"data"`
	Titulo string      `json:"titulo"`
	Grupos []GrupoNota `json:"grupos"`
}

// GrupoNota agrupa itens de melhoria sob um subtítulo temático.
type GrupoNota struct {
	Titulo string   `json:"titulo"`
	Itens  []string `json:"itens"`
}

type estadoNotasVistas struct {
	Vistas []string `json:"vistas"`
}


// ----- Operações Públicas -----

// CarregarNotas carrega todas as notas na tradução solicitada (ou pt-BR por fallback), ordenadas por data decrescente.
func CarregarNotas(idioma string) ([]Nota, error) {
	subFS, err := fs.Sub(fsNotas, "notas")
	if err != nil {
		return nil, fmt.Errorf("falha ao acessar sistema de arquivos embutido de notas: %w", err)
	}

	return carregarNotasFS(subFS, idioma)
}


// NaoVistas devolve as notas que o usuário ainda não viu no idioma solicitado.
func NaoVistas(idioma string) ([]Nota, error) {
	subFS, err := fs.Sub(fsNotas, "notas")
	if err != nil {
		return nil, fmt.Errorf("falha ao acessar sistema de arquivos embutido de notas: %w", err)
	}

	caminho := filepath.Join(armazenamento.PastaDados(), NOME_ARQUIVO_ESTADO)
	return naoVistasNoCaminho(subFS, caminho, idioma)
}


// MarcarVistas registra os identificadores das notas fornecidas como vistas no arquivo de estado.
func MarcarVistas(ids []string) error {
	caminho := filepath.Join(armazenamento.PastaDados(), NOME_ARQUIVO_ESTADO)
	return marcarVistasNoCaminho(caminho, ids)
}


// ----- Lógica Interna de Notas -----

func carregarNotasFS(sistemaArquivos fs.FS, idioma string) ([]Nota, error) {
	entradas, err := fs.ReadDir(sistemaArquivos, ".")
	if err != nil {
		return nil, fmt.Errorf("falha ao listar diretório de notas: %w", err)
	}

	mapaBase := make(map[string]string)
	mapaTraducoes := make(map[string]map[string]string)

	for _, entrada := range entradas {
		nome := entrada.Name()
		if entrada.IsDir() {
			return nil, fmt.Errorf("subpasta não permitida na pasta de notas: %s", nome)
		}

		partes := padraoNomeNota.FindStringSubmatch(nome)
		if partes == nil {
			return nil, fmt.Errorf("nome de arquivo de nota fora do padrão: %s", nome)
		}

		dataStr := partes[1]
		if _, errData := time.Parse(FORMATO_DATA_NOTAS, dataStr); errData != nil {
			return nil, fmt.Errorf("data inválida no arquivo %s: %w", nome, errData)
		}

		slug := partes[2]
		idNota := dataStr + "-" + slug
		sufixoIdioma := partes[3]

		if sufixoIdioma == "" {
			mapaBase[idNota] = nome
			continue
		}

		if !idiomaSuportadoParaTraducao(sufixoIdioma) {
			return nil, fmt.Errorf("idioma de tradução inválido ou não suportado no arquivo %s: %s", nome, sufixoIdioma)
		}

		if mapaTraducoes[idNota] == nil {
			mapaTraducoes[idNota] = make(map[string]string)
		}
		mapaTraducoes[idNota][sufixoIdioma] = nome
	}

	for idNota := range mapaTraducoes {
		if _, existe := mapaBase[idNota]; !existe {
			return nil, fmt.Errorf("tradução sem o arquivo pt-BR correspondente para a nota '%s'", idNota)
		}
	}

	notas := make([]Nota, 0, len(mapaBase))

	for idNota, arquivoBase := range mapaBase {
		arquivoEscolhido := arquivoBase
		if idioma != "" && idioma != IDIOMA_PADRAO_NOTAS {
			if traducoes, existe := mapaTraducoes[idNota]; existe {
				if arqTraducao, temTraducao := traducoes[idioma]; temTraducao {
					arquivoEscolhido = arqTraducao
				}
			}
		}

		conteudo, errLeitura := fs.ReadFile(sistemaArquivos, arquivoEscolhido)
		if errLeitura != nil {
			return nil, fmt.Errorf("erro ao ler arquivo %s: %w", arquivoEscolhido, errLeitura)
		}

		titulo, grupos, errParse := analisarNota(arquivoEscolhido, conteudo)
		if errParse != nil {
			return nil, errParse
		}

		dataNota := idNota[:10]
		notas = append(notas, Nota{
			Id:     idNota,
			Data:   dataNota,
			Titulo: titulo,
			Grupos: grupos,
		})
	}

	sort.Slice(notas, func(i, j int) bool {
		if notas[i].Data != notas[j].Data {
			return notas[i].Data > notas[j].Data
		}
		return notas[i].Id > notas[j].Id
	})

	return notas, nil
}


func naoVistasNoCaminho(sistemaArquivos fs.FS, caminhoArquivoEstado string, idioma string) ([]Nota, error) {
	muEstado.Lock()
	defer muEstado.Unlock()

	notas, err := carregarNotasFS(sistemaArquivos, idioma)
	if err != nil {
		return nil, err
	}

	estado, err := carregarEstadoVistas(caminhoArquivoEstado)
	if err != nil {
		todosIds := make([]string, 0, len(notas))
		for _, nota := range notas {
			todosIds = append(todosIds, nota.Id)
		}

		novoEstado := &estadoNotasVistas{Vistas: todosIds}
		if errSalvar := salvarEstadoVistas(caminhoArquivoEstado, novoEstado); errSalvar != nil {
			return nil, fmt.Errorf("falha ao salvar estado inicial de notas vistas: %w", errSalvar)
		}

		return []Nota{}, nil
	}

	vistasMap := make(map[string]bool, len(estado.Vistas))
	for _, id := range estado.Vistas {
		vistasMap[id] = true
	}

	var naoVistas []Nota
	for _, nota := range notas {
		if !vistasMap[nota.Id] {
			naoVistas = append(naoVistas, nota)
		}
	}

	return naoVistas, nil
}


func marcarVistasNoCaminho(caminhoArquivoEstado string, ids []string) error {
	if len(ids) == 0 {
		return nil
	}

	muEstado.Lock()
	defer muEstado.Unlock()

	estado, err := carregarEstadoVistas(caminhoArquivoEstado)
	if err != nil {
		estado = &estadoNotasVistas{Vistas: []string{}}
	}

	existentes := make(map[string]bool, len(estado.Vistas))
	for _, id := range estado.Vistas {
		existentes[id] = true
	}

	houveModificacao := false
	for _, id := range ids {
		idLimpo := strings.TrimSpace(id)
		if idLimpo == "" || existentes[idLimpo] {
			continue
		}
		estado.Vistas = append(estado.Vistas, idLimpo)
		existentes[idLimpo] = true
		houveModificacao = true
	}

	if !houveModificacao {
		return nil
	}

	return salvarEstadoVistas(caminhoArquivoEstado, estado)
}


func analisarNota(nomeArquivo string, conteudo []byte) (string, []GrupoNota, error) {
	if bytes.HasPrefix(conteudo, []byte{0xEF, 0xBB, 0xBF}) {
		conteudo = conteudo[3:]
	}

	linhasBrutas := strings.Split(string(conteudo), "\n")
	var titulo string
	var grupos []*GrupoNota
	var grupoAtual *GrupoNota
	ultimoTipo := tipoLinhaNenhum

	for indice, linhaBruta := range linhasBrutas {
		numeroLinha := indice + 1
		linha := strings.TrimSuffix(linhaBruta, "\r")

		if strings.Contains(linha, "**") || strings.ContainsRune(linha, '`') {
			return "", nil, fmt.Errorf("%s:%d: formatação não permitida (** ou crase)", nomeArquivo, numeroLinha)
		}

		if strings.TrimSpace(linha) == "" {
			continue
		}

		if titulo == "" {
			if !strings.HasPrefix(linha, PREFIXO_TITULO) || strings.HasPrefix(linha, PREFIXO_GRUPO) {
				return "", nil, fmt.Errorf("%s:%d: primeira linha com conteúdo deve ser '# <título>'", nomeArquivo, numeroLinha)
			}

			titulo = strings.TrimSpace(strings.TrimPrefix(linha, PREFIXO_TITULO))
			if titulo == "" {
				return "", nil, fmt.Errorf("%s:%d: título não pode ser vazio", nomeArquivo, numeroLinha)
			}

			ultimoTipo = tipoLinhaTitulo
			continue
		}

		if strings.HasPrefix(linha, PREFIXO_TITULO) && !strings.HasPrefix(linha, PREFIXO_GRUPO) {
			return "", nil, fmt.Errorf("%s:%d: apenas um título principal ('# <título>') é permitido", nomeArquivo, numeroLinha)
		}

		if strings.HasPrefix(linha, PREFIXO_GRUPO) {
			subtitulo := strings.TrimSpace(strings.TrimPrefix(linha, PREFIXO_GRUPO))
			if subtitulo == "" {
				return "", nil, fmt.Errorf("%s:%d: subtítulo do grupo não pode ser vazio", nomeArquivo, numeroLinha)
			}

			if grupoAtual != nil && len(grupoAtual.Itens) == 0 {
				return "", nil, fmt.Errorf("%s:%d: grupo anterior sem itens", nomeArquivo, numeroLinha)
			}

			grupoAtual = &GrupoNota{
				Titulo: subtitulo,
				Itens:  []string{},
			}
			grupos = append(grupos, grupoAtual)
			ultimoTipo = tipoLinhaGrupo
			continue
		}

		if strings.HasPrefix(linha, "##") {
			return "", nil, fmt.Errorf("%s:%d: grupo deve iniciar com '## '", nomeArquivo, numeroLinha)
		}

		if strings.HasPrefix(linha, PREFIXO_ITEM) {
			item := strings.TrimSpace(strings.TrimPrefix(linha, PREFIXO_ITEM))
			if item == "" {
				return "", nil, fmt.Errorf("%s:%d: item não pode ser vazio", nomeArquivo, numeroLinha)
			}

			if grupoAtual == nil {
				grupoAtual = &GrupoNota{
					Titulo: "",
					Itens:  []string{},
				}
				grupos = append(grupos, grupoAtual)
			}

			grupoAtual.Itens = append(grupoAtual.Itens, item)
			ultimoTipo = tipoLinhaItem
			continue
		}

		if strings.HasPrefix(linha, " ") || strings.HasPrefix(linha, "\t") {
			if ultimoTipo != tipoLinhaItem && ultimoTipo != tipoLinhaContinuacao {
				return "", nil, fmt.Errorf("%s:%d: linha de continuação sem item anterior", nomeArquivo, numeroLinha)
			}

			if grupoAtual == nil || len(grupoAtual.Itens) == 0 {
				return "", nil, fmt.Errorf("%s:%d: linha de continuação sem item disponível", nomeArquivo, numeroLinha)
			}

			ultimoIndice := len(grupoAtual.Itens) - 1
			grupoAtual.Itens[ultimoIndice] += " " + strings.TrimSpace(linha)
			ultimoTipo = tipoLinhaContinuacao
			continue
		}

		return "", nil, fmt.Errorf("%s:%d: linha inválida: %q", nomeArquivo, numeroLinha, linha)
	}

	if titulo == "" {
		return "", nil, fmt.Errorf("%s: arquivo vazio ou sem título", nomeArquivo)
	}

	if len(grupos) == 0 {
		return "", nil, fmt.Errorf("%s: nota sem itens", nomeArquivo)
	}

	totalItens := 0
	resultadoGrupos := make([]GrupoNota, 0, len(grupos))
	for _, g := range grupos {
		if len(g.Itens) == 0 {
			return "", nil, fmt.Errorf("%s: grupo '%s' sem itens", nomeArquivo, g.Titulo)
		}
		totalItens += len(g.Itens)
		resultadoGrupos = append(resultadoGrupos, *g)
	}

	if totalItens == 0 {
		return "", nil, fmt.Errorf("%s: nota sem itens", nomeArquivo)
	}

	return titulo, resultadoGrupos, nil
}


// ----- Persistência e Utilitários -----

func salvarEstadoVistas(caminho string, estado *estadoNotasVistas) error {
	dados, err := json.MarshalIndent(estado, "", "  ")
	if err != nil {
		return fmt.Errorf("falha ao serializar estado de notas vistas: %w", err)
	}

	pasta := filepath.Dir(caminho)
	if errPasta := os.MkdirAll(pasta, 0755); errPasta != nil {
		return fmt.Errorf("falha ao criar diretório %s: %w", pasta, errPasta)
	}

	temporario := caminho + ".tmp"
	if errEscrita := os.WriteFile(temporario, dados, 0600); errEscrita != nil {
		return fmt.Errorf("falha ao escrever arquivo temporário %s: %w", temporario, errEscrita)
	}

	if errRename := os.Rename(temporario, caminho); errRename != nil {
		_ = os.Remove(caminho)
		if errRename2 := os.Rename(temporario, caminho); errRename2 != nil {
			return fmt.Errorf("falha ao renomear arquivo temporário para %s: %w", caminho, errRename2)
		}
	}

	return nil
}


func carregarEstadoVistas(caminho string) (*estadoNotasVistas, error) {
	dados, err := os.ReadFile(caminho)
	if err != nil {
		return nil, err
	}

	var estado estadoNotasVistas
	if errUnmarshal := json.Unmarshal(dados, &estado); errUnmarshal != nil {
		return nil, fmt.Errorf("conteúdo corrompido em %s: %w", caminho, errUnmarshal)
	}

	if estado.Vistas == nil {
		estado.Vistas = []string{}
	}

	return &estado, nil
}


func idiomaSuportadoParaTraducao(idioma string) bool {
	if idioma == IDIOMA_PADRAO_NOTAS {
		return false
	}

	for _, suportado := range dicionario.IdiomasSuportados {
		if suportado == idioma {
			return true
		}
	}

	return false
}
