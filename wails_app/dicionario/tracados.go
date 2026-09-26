package dicionario

import (
	"bufio"
	"bytes"
	"compress/gzip"
	_ "embed"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
)

// ----- Banco de Traçados (Hanzi Writer) -----
// Dados de traçado (strokes/medians em SVG) do projeto hanzi-writer-data, derivado do Make Me a
// Hanzi (Arphic Public License — ver LICENCAS-DADOS.md). Alimenta o canvas interativo da revisão
// de desenho: o frontend pede o JSON cru de um caractere via ObterDadosEscritaHanzi e o entrega ao
// motor Hanzi Writer (charDataLoader).
//
// O arquivo embarcado é um TSV gzipado ("caractere\tjson") de ~40 MB descomprimido, por isso o
// carregamento é PREGUIÇOSO (sync.Once): a memória só é paga quando a revisão de desenho é usada.
//
// Fica em fontes/ (junto das outras origens externas cruas), mesmo sendo embarcado direto sem
// passar por fusão: é a única fonte que já chega no formato exato que o app consome, byte a byte —
// não há produto derivado equivalente em idiomas/ para gerar (ver LICENCAS-DADOS.md).

//go:embed fontes/hanzi_tracados.tsv.gz
var dadosTracados []byte

type BancoTracados struct {
	carregarUmaVez sync.Once
	erroCarga      error
	porCaractere   map[string]string // caractere -> JSON cru no formato do Hanzi Writer
}

func NovoBancoTracados() *BancoTracados {
	return &BancoTracados{}
}

func (b *BancoTracados) garantirCarregado() error {
	b.carregarUmaVez.Do(func() {
		leitorGz, err := gzip.NewReader(bytes.NewReader(dadosTracados))
		if err != nil {
			b.erroCarga = fmt.Errorf("banco de traçados corrompido: %w", err)
			return
		}
		defer leitorGz.Close()

		b.porCaractere = make(map[string]string, 10000)
		varredor := bufio.NewScanner(leitorGz)
		varredor.Buffer(make([]byte, 4*1024*1024), 4*1024*1024)

		for varredor.Scan() {
			caractere, json, achou := strings.Cut(varredor.Text(), "\t")
			if !achou || caractere == "" || json == "" {
				continue
			}
			b.porCaractere[caractere] = json
		}

		b.erroCarga = varredor.Err()
	})

	return b.erroCarga
}

// Dados devolve o JSON cru de traçados do caractere, no formato esperado pelo Hanzi Writer.
func (b *BancoTracados) Dados(caractere string) (string, bool) {
	if err := b.garantirCarregado(); err != nil {
		return "", false
	}
	json, existe := b.porCaractere[caractere]
	return json, existe
}

// TotalTracos devolve quantos traços o caractere tem. Serve para validar índices de traço vindos de
// OUTRO arquivo (as correspondências da decomposição — ver Banco.ComponentesDesenhaveis) antes de
// mandá-los ao frontend: um índice fora da faixa quebraria o canvas.
func (b *BancoTracados) TotalTracos(caractere string) (int, bool) {
	dados, existe := b.Dados(caractere)
	if !existe {
		return 0, false
	}

	var forma struct {
		Tracos []json.RawMessage `json:"strokes"`
	}
	if err := json.Unmarshal([]byte(dados), &forma); err != nil {
		return 0, false
	}
	return len(forma.Tracos), true
}

// Tem informa se há dados de traçado para o caractere (usado ao filtrar candidatos da revisão).
func (b *BancoTracados) Tem(caractere string) bool {
	if err := b.garantirCarregado(); err != nil {
		return false
	}
	_, existe := b.porCaractere[caractere]
	return existe
}

func (b *BancoTracados) TotalCaracteres() int {
	if err := b.garantirCarregado(); err != nil {
		return 0
	}
	return len(b.porCaractere)
}
