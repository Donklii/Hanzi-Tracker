package nuvem

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"time"
)

// ----- API REST v3 do Google Drive (só o mínimo: procurar, criar pasta, enviar, baixar) -----
//
// O backup mora numa pasta dedicada na raiz do Drive do usuário, para ele achar e conseguir mover
// tudo de uma vez:
//
//	Hanzi Tracker/
//	├── progresso.db          (vocabulário, progresso e caches)
//	└── configuracoes.json    (preferências do app)

// Nomes do que o app cria no Drive do usuário.
const (
	nomePastaRemota          = "Hanzi Tracker"
	nomeArquivoBanco         = "progresso.db"
	nomeArquivoConfiguracoes = "configuracoes.json"
)

// mimeTypePasta é como o Drive representa uma pasta (a API não tem endpoint separado para isso).
const mimeTypePasta = "application/vnd.google-apps.folder"

// maximoArquivosNaPasta limita a listagem da pasta do backup: são dois arquivos, a folga cobre
// duplicatas que o usuário possa ter criado copiando arquivos para lá.
const maximoArquivosNaPasta = 20

// clienteArquivos cobre upload/download do banco inteiro, que pode ter dezenas de MB (os caches
// de áudio e tradução moram dentro do .db) — por isso o prazo bem mais largo que o clienteHTTP.
var clienteArquivos = &http.Client{Timeout: 10 * time.Minute}

// arquivoRemoto são os metadados de um arquivo do backup no Drive.
type arquivoRemoto struct {
	Id           string
	Bytes        int64
	ModificadoEm time.Time
}

// conteudoPasta é o que existe hoje na pasta do backup (campo nil = arquivo ainda não está lá).
type conteudoPasta struct {
	Banco         *arquivoRemoto
	Configuracoes *arquivoRemoto
}

// garantirPastaRemota devolve o id da pasta do backup, criando-a se ainda não existir. Com o
// escopo drive.file a busca só enxerga o que o próprio app criou, então não há risco de adotar uma
// pasta homônima do usuário.
func (g *Gerenciador) garantirPastaRemota(tok *estadoSalvo) (string, error) {
	// Guard clause: id já conhecido de uma sessão anterior.
	if tok.PastaId != "" {
		return tok.PastaId, nil
	}

	consulta := fmt.Sprintf("name = '%s' and mimeType = '%s' and trashed = false", nomePastaRemota, mimeTypePasta)
	achados, err := g.listarArquivos(tok, consulta)
	if err != nil {
		return "", fmt.Errorf("falha ao procurar a pasta %q no Drive: %w", nomePastaRemota, err)
	}
	if len(achados) > 0 {
		return achados[0].Id, nil
	}

	id, err := g.criarPastaRemota(tok)
	if err != nil {
		return "", fmt.Errorf("falha ao criar a pasta %q no Drive: %w", nomePastaRemota, err)
	}
	return id, nil
}

// listarPastaRemota diz o que já existe na pasta do backup (campos nil = ainda não enviado).
func (g *Gerenciador) listarPastaRemota(tok *estadoSalvo, pastaId string) (conteudoPasta, error) {
	achados, err := g.listarArquivos(tok, fmt.Sprintf("'%s' in parents and trashed = false", pastaId))
	if err != nil {
		return conteudoPasta{}, fmt.Errorf("falha ao listar a pasta do backup: %w", err)
	}

	var conteudo conteudoPasta
	for _, achado := range achados {
		// A listagem vem ordenada do mais recente para o mais antigo: a primeira ocorrência de cada
		// nome vence, e eventuais duplicatas antigas são ignoradas.
		switch achado.Nome {
		case nomeArquivoBanco:
			if conteudo.Banco == nil {
				conteudo.Banco = achado.metadados()
			}
		case nomeArquivoConfiguracoes:
			if conteudo.Configuracoes == nil {
				conteudo.Configuracoes = achado.metadados()
			}
		}
	}
	return conteudo, nil
}

// criarPastaRemota cria a pasta do backup na raiz do Drive do usuário.
func (g *Gerenciador) criarPastaRemota(tok *estadoSalvo) (string, error) {
	acesso, err := g.tokenAcesso(tok)
	if err != nil {
		return "", err
	}

	metadados, err := json.Marshal(map[string]string{"name": nomePastaRemota, "mimeType": mimeTypePasta})
	if err != nil {
		return "", err
	}
	req, err := http.NewRequest("POST", g.urls.drive+"/files", bytes.NewReader(metadados))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+acesso)
	req.Header.Set("Content-Type", "application/json; charset=utf-8")

	resp, err := clienteHTTP.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", erroDaApi("criar a pasta", resp)
	}

	var corpo struct {
		Id string `json:"id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&corpo); err != nil {
		return "", fmt.Errorf("resposta inválida da criação da pasta: %w", err)
	}
	return corpo.Id, nil
}

// arquivoListado é uma linha crua da listagem do Drive (a API manda o tamanho como string).
type arquivoListado struct {
	Id           string `json:"id"`
	Nome         string `json:"name"`
	Size         string `json:"size"`
	ModifiedTime string `json:"modifiedTime"`
}

// metadados converte a linha crua no formato usado pelo resto do pacote.
func (a arquivoListado) metadados() *arquivoRemoto {
	tamanho, _ := strconv.ParseInt(a.Size, 10, 64)
	modificado, _ := time.Parse(time.RFC3339, a.ModifiedTime)
	return &arquivoRemoto{Id: a.Id, Bytes: tamanho, ModificadoEm: modificado}
}

// listarArquivos roda uma consulta na API do Drive, do mais recente para o mais antigo.
func (g *Gerenciador) listarArquivos(tok *estadoSalvo, consulta string) ([]arquivoListado, error) {
	acesso, err := g.tokenAcesso(tok)
	if err != nil {
		return nil, err
	}

	parametros := url.Values{
		"q":        {consulta},
		"fields":   {"files(id,name,size,modifiedTime)"},
		"orderBy":  {"modifiedTime desc"},
		"pageSize": {strconv.Itoa(maximoArquivosNaPasta)},
	}
	req, err := http.NewRequest("GET", g.urls.drive+"/files?"+parametros.Encode(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+acesso)

	resp, err := clienteHTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, erroDaApi("consultar o Drive", resp)
	}

	var corpo struct {
		Files []arquivoListado `json:"files"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&corpo); err != nil {
		return nil, fmt.Errorf("resposta inválida da consulta ao Drive: %w", err)
	}
	return corpo.Files, nil
}

// enviarArquivo sobe `caminho` para o Drive em upload "resumable" (obrigatório acima de 5 MB;
// aqui sempre num único PUT — se a conexão cair, a próxima sincronização reenvia do zero).
// Com idExistente vazio cria o arquivo com o nome dado dentro de pastaId; senão sobrescreve o
// conteúdo mantendo o mesmo id (e, portanto, o mesmo nome e a mesma pasta).
func (g *Gerenciador) enviarArquivo(tok *estadoSalvo, caminho, idExistente, pastaId, nome string) (string, error) {
	acesso, err := g.tokenAcesso(tok)
	if err != nil {
		return "", err
	}

	// Passo 1: abrir a sessão de upload (devolve a URL de envio no header Location).
	var abertura *http.Request
	if idExistente == "" {
		metadados, _ := json.Marshal(map[string]any{"name": nome, "parents": []string{pastaId}})
		abertura, err = http.NewRequest("POST", g.urls.upload+"/files?uploadType=resumable", bytes.NewReader(metadados))
	} else {
		abertura, err = http.NewRequest("PATCH", g.urls.upload+"/files/"+idExistente+"?uploadType=resumable", bytes.NewReader([]byte("{}")))
	}
	if err != nil {
		return "", err
	}
	abertura.Header.Set("Authorization", "Bearer "+acesso)
	abertura.Header.Set("Content-Type", "application/json; charset=utf-8")

	resp, err := clienteHTTP.Do(abertura)
	if err != nil {
		return "", err
	}
	if resp.StatusCode != http.StatusOK {
		defer resp.Body.Close()
		return "", erroDaApi("abrir a sessão de upload", resp)
	}
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	urlEnvio := resp.Header.Get("Location")
	if urlEnvio == "" {
		return "", fmt.Errorf("o Drive não devolveu a URL da sessão de upload")
	}

	// Passo 2: enviar o conteúdo inteiro.
	arquivo, err := os.Open(caminho)
	if err != nil {
		return "", err
	}
	defer arquivo.Close()
	info, err := arquivo.Stat()
	if err != nil {
		return "", err
	}

	envio, err := http.NewRequest("PUT", urlEnvio, arquivo)
	if err != nil {
		return "", err
	}
	envio.ContentLength = info.Size()

	respEnvio, err := clienteArquivos.Do(envio)
	if err != nil {
		return "", err
	}
	defer respEnvio.Body.Close()
	if respEnvio.StatusCode != http.StatusOK && respEnvio.StatusCode != http.StatusCreated {
		return "", erroDaApi("enviar o banco", respEnvio)
	}

	var corpo struct {
		Id string `json:"id"`
	}
	if err := json.NewDecoder(respEnvio.Body).Decode(&corpo); err != nil {
		return "", fmt.Errorf("resposta inválida do upload: %w", err)
	}
	return corpo.Id, nil
}

// baixarArquivo baixa o conteúdo do backup para `destino` (permissão restrita: são dados do usuário).
func (g *Gerenciador) baixarArquivo(tok *estadoSalvo, id, destino string) error {
	acesso, err := g.tokenAcesso(tok)
	if err != nil {
		return err
	}

	req, err := http.NewRequest("GET", g.urls.drive+"/files/"+id+"?alt=media", nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+acesso)

	resp, err := clienteArquivos.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return erroDaApi("baixar o backup", resp)
	}

	arquivo, err := os.OpenFile(destino, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	if _, err := io.Copy(arquivo, resp.Body); err != nil {
		arquivo.Close()
		return err
	}
	return arquivo.Close()
}

// ----- Erros da API -----

// erroApi é uma resposta de erro do Drive com a situação HTTP preservada, para o chamador
// distinguir "o que eu procurava sumiu" de "deu ruim" (ver ehNaoEncontrado).
type erroApi struct {
	Acao     string
	Situacao int
	Corpo    string
}

func (e *erroApi) Error() string {
	return fmt.Sprintf("falha ao %s (HTTP %d): %s", e.Acao, e.Situacao, e.Corpo)
}

// erroDaApi resume uma resposta de erro da API do Drive num erro legível.
func erroDaApi(acao string, resp *http.Response) error {
	corpo, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
	return &erroApi{Acao: acao, Situacao: resp.StatusCode, Corpo: string(bytes.TrimSpace(corpo))}
}

// ehNaoEncontrado diz se o erro veio de um id que o Drive não conhece mais — pasta ou arquivo que
// o usuário apagou do Drive dele por fora do app.
func ehNaoEncontrado(err error) bool {
	var falha *erroApi
	return errors.As(err, &falha) && falha.Situacao == http.StatusNotFound
}
