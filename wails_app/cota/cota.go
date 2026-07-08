// Package cota implementa os contadores de uso persistidos num ÚNICO JSON
// (%APPDATA%\HanziTracker\cotas.json) com reset automático por período, compartilhado pelos
// rastreadores de cota de tradução (mensal, por caractere) e do Gemini (diário, por requisição).
// Cada consumidor declara a SUA chave no arquivo e a SUA função de período — o resto (load, reset
// quando o período vira, save e a migração dos arquivos antigos por-cota) vive uma única vez aqui.
package cota

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

// nomeArquivoCotas é o arquivo unificado de todas as cotas (antes era um JSON por cota).
const nomeArquivoCotas = "cotas.json"

// mu serializa o acesso ao arquivo unificado: só uma cota é consumida por vez no app, mas a UI lê
// os contadores em paralelo ao uso — o lock evita ler o arquivo no meio de uma escrita.
var mu sync.Mutex

// registroCota é o valor de uma cota dentro de cotas.json ({"periodo": ..., "usado": ...}).
type registroCota struct {
	Periodo string `json:"periodo"`
	Usado   int    `json:"usado"`
}

// MigracaoAntiga descreve o arquivo do formato anterior (um JSON por cota, com nomes de campo
// próprios) a importar na primeira leitura, preservando o consumo já contabilizado no período.
type MigracaoAntiga struct {
	NomeArquivo  string // ex.: "cota_gemini.json"
	CampoPeriodo string // ex.: "data", "anoMes"
	CampoUsado   string // ex.: "requisicoesUsadas", "caracteresUsados"
}

// Contador é uma cota nomeada dentro do arquivo unificado, zerada automaticamente quando o período
// corrente (dia/mês) diverge do gravado.
type Contador struct {
	nome         string // chave em cotas.json (ex.: "gemini", "traducao")
	periodoAtual func() string
	migracao     *MigracaoAntiga // nil = sem formato antigo a importar
}

// NovoContador cria o contador de uma cota. `migracao` importa o arquivo do formato antigo na
// primeira leitura (nil quando não há).
func NovoContador(nome string, periodoAtual func() string, migracao *MigracaoAntiga) *Contador {
	return &Contador{nome: nome, periodoAtual: periodoAtual, migracao: migracao}
}

// Carregar lê o estado da cota. Se o período salvo difere do atual, devolve zerado (e persiste o
// reset), garantindo que todo período começa do zero.
func (c *Contador) Carregar() (periodo string, usado int) {
	mu.Lock()
	defer mu.Unlock()

	_, reg := c.carregarSemLock()
	return reg.Periodo, reg.Usado
}

// Registrar incrementa o contador de uso e persiste. Chamar SÓ após chamada de API bem-sucedida
// (nunca em cache hit).
func (c *Contador) Registrar(quantidade int) error {
	mu.Lock()
	defer mu.Unlock()

	cotas, reg := c.carregarSemLock()
	reg.Usado += quantidade
	cotas[c.nome] = reg
	return salvarCotasSemLock(cotas)
}

// carregarSemLock devolve o mapa completo de cotas e o registro desta cota já normalizado
// (importado do formato antigo se preciso; zerado se o período virou). Persiste qualquer
// normalização feita, para o arquivo refletir sempre o estado devolvido.
func (c *Contador) carregarSemLock() (map[string]registroCota, registroCota) {
	cotas := lerCotasSemLock()

	reg, existe := cotas[c.nome]
	if !existe {
		reg = c.importarCotaAntiga()
	}

	// Guard clause: período virou (ou cota recém-criada/importada de outro período) — zera.
	if reg.Periodo != c.periodoAtual() {
		reg = registroCota{Periodo: c.periodoAtual(), Usado: 0}
	}

	if cotas[c.nome] != reg {
		cotas[c.nome] = reg
		_ = salvarCotasSemLock(cotas)
	}
	return cotas, reg
}

// importarCotaAntiga lê (e apaga) o arquivo do formato anterior desta cota, devolvendo o registro
// importado — zerado no período atual quando não há nada a importar (first-run ou arquivo inválido).
func (c *Contador) importarCotaAntiga() registroCota {
	zerado := registroCota{Periodo: c.periodoAtual(), Usado: 0}
	if c.migracao == nil {
		return zerado
	}

	caminho, err := caminhoNaPastaDados(c.migracao.NomeArquivo)
	if err != nil {
		return zerado
	}
	dados, err := os.ReadFile(caminho)
	if err != nil {
		return zerado // sem arquivo antigo — nada a importar
	}
	// O arquivo antigo é consumido pela importação (senão seria re-importado a cada leitura futura
	// do cotas.json recém-apagado/corrompido, ressuscitando valores velhos).
	defer os.Remove(caminho)

	var antigo map[string]any
	if err := json.Unmarshal(dados, &antigo); err != nil {
		return zerado
	}

	periodo, _ := antigo[c.migracao.CampoPeriodo].(string)
	usado, _ := antigo[c.migracao.CampoUsado].(float64) // números JSON decodificam como float64
	if periodo == "" {
		return zerado
	}
	return registroCota{Periodo: periodo, Usado: int(usado)}
}

// lerCotasSemLock carrega o mapa de cotas do disco; arquivo ausente ou corrompido vira mapa vazio
// (cada cota renasce zerada no próprio período — comportamento do first-run).
func lerCotasSemLock() map[string]registroCota {
	cotas := map[string]registroCota{}

	caminho, err := caminhoNaPastaDados(nomeArquivoCotas)
	if err != nil {
		return cotas
	}
	dados, err := os.ReadFile(caminho)
	if err != nil {
		return cotas
	}
	_ = json.Unmarshal(dados, &cotas)
	return cotas
}

// salvarCotasSemLock persiste o mapa de cotas no disco.
func salvarCotasSemLock(cotas map[string]registroCota) error {
	caminho, err := caminhoNaPastaDados(nomeArquivoCotas)
	if err != nil {
		return err
	}
	dados, err := json.MarshalIndent(cotas, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(caminho, dados, 0644)
}

// caminhoNaPastaDados resolve um nome de arquivo dentro da pasta de dados do app, criando-a se
// preciso.
func caminhoNaPastaDados(nome string) (string, error) {
	appData, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(appData, "HanziTracker")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}
	return filepath.Join(dir, nome), nil
}
