package nuvem

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"time"
)

// ----- API REST v3 do Google Drive (só o mínimo: procurar, enviar, baixar) -----

// nomeArquivoRemoto é o nome do backup no Drive do usuário (fica visível na raiz do Drive).
const nomeArquivoRemoto = "HanziTracker-progresso.db"

// clienteArquivos cobre upload/download do banco inteiro, que pode ter dezenas de MB (os caches
// de áudio e tradução moram dentro do .db) — por isso o prazo bem mais largo que o clienteHTTP.
var clienteArquivos = &http.Client{Timeout: 10 * time.Minute}

// arquivoRemoto são os metadados do backup no Drive.
type arquivoRemoto struct {
	Id           string
	Bytes        int64
	ModificadoEm time.Time
}

// procurarArquivoRemoto busca o backup no Drive (nil = nuvem vazia, primeira conexão de todas).
// Com o escopo drive.file a busca só enxerga arquivos criados pelo próprio app.
func (g *Gerenciador) procurarArquivoRemoto(tok *estadoSalvo) (*arquivoRemoto, error) {
	acesso, err := g.tokenAcesso(tok)
	if err != nil {
		return nil, err
	}

	consulta := url.Values{
		"q":        {fmt.Sprintf("name = '%s' and trashed = false", nomeArquivoRemoto)},
		"fields":   {"files(id,size,modifiedTime)"},
		"orderBy":  {"modifiedTime desc"},
		"pageSize": {"1"},
	}
	req, err := http.NewRequest("GET", g.urls.drive+"/files?"+consulta.Encode(), nil)
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
		return nil, erroDaApi("buscar o backup", resp)
	}

	var corpo struct {
		Files []struct {
			Id           string `json:"id"`
			Size         string `json:"size"` // a API do Drive manda o tamanho como string
			ModifiedTime string `json:"modifiedTime"`
		} `json:"files"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&corpo); err != nil {
		return nil, fmt.Errorf("resposta inválida da busca no Drive: %w", err)
	}

	// Guard clause: nuvem vazia.
	if len(corpo.Files) == 0 {
		return nil, nil
	}

	achado := corpo.Files[0]
	tamanho, _ := strconv.ParseInt(achado.Size, 10, 64)
	modificado, _ := time.Parse(time.RFC3339, achado.ModifiedTime)
	return &arquivoRemoto{Id: achado.Id, Bytes: tamanho, ModificadoEm: modificado}, nil
}

// enviarArquivo sobe `caminho` para o Drive em upload "resumable" (obrigatório acima de 5 MB;
// aqui sempre num único PUT — se a conexão cair, a próxima sincronização reenvia do zero).
// Com idExistente vazio cria o arquivo; senão sobrescreve o conteúdo mantendo o mesmo id.
func (g *Gerenciador) enviarArquivo(tok *estadoSalvo, caminho, idExistente string) (string, error) {
	acesso, err := g.tokenAcesso(tok)
	if err != nil {
		return "", err
	}

	// Passo 1: abrir a sessão de upload (devolve a URL de envio no header Location).
	var abertura *http.Request
	if idExistente == "" {
		metadados, _ := json.Marshal(map[string]string{"name": nomeArquivoRemoto})
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

// erroDaApi resume uma resposta de erro da API do Drive num erro legível.
func erroDaApi(acao string, resp *http.Response) error {
	corpo, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
	return fmt.Errorf("falha ao %s (HTTP %d): %s", acao, resp.StatusCode, bytes.TrimSpace(corpo))
}
