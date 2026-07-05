package dicionario

import (
	"bufio"
	"bytes"
	"compress/gzip"
	_ "embed"
	"fmt"
	"strings"
	"sync"
)

// ----- Banco de Frases (Tatoeba) -----
// Pares de frases chinês→inglês do projeto Tatoeba (via manythings.org/anki, licença CC-BY 2.0 FR;
// a atribuição por frase é preservada e deve ser exibida ao usuário junto da frase).
// Alimenta os modos de revisão "por contexto" e "desenho guiado por contexto".
//
// O arquivo embarcado é um TSV gzipado ("chinês\tinglês\tatribuição") e só é carregado na PRIMEIRA
// consulta (sync.Once): quem nunca abre a revisão não paga o custo de memória do banco.

//go:embed frases_tatoeba.tsv.gz
var dadosFrases []byte

type Frase struct {
	Chines     string `json:"chines"`
	Ingles     string `json:"ingles"`
	Atribuicao string `json:"atribuicao"`
}

type BancoFrases struct {
	carregarUmaVez sync.Once
	erroCarga      error
	frases         []Frase
	indice         map[rune][]int32 // caractere -> índices das frases que o contêm
}

func NovoBancoFrases() *BancoFrases {
	return &BancoFrases{}
}

func (b *BancoFrases) garantirCarregado() error {
	b.carregarUmaVez.Do(func() {
		leitorGz, err := gzip.NewReader(bytes.NewReader(dadosFrases))
		if err != nil {
			b.erroCarga = fmt.Errorf("banco de frases corrompido: %w", err)
			return
		}
		defer leitorGz.Close()

		b.indice = make(map[rune][]int32)
		varredor := bufio.NewScanner(leitorGz)
		varredor.Buffer(make([]byte, 1024*1024), 1024*1024)

		for varredor.Scan() {
			campos := strings.Split(varredor.Text(), "\t")
			if len(campos) != 3 || campos[0] == "" {
				continue
			}

			idx := int32(len(b.frases))
			b.frases = append(b.frases, Frase{Chines: campos[0], Ingles: campos[1], Atribuicao: campos[2]})

			// Indexa cada caractere único da frase (pontuação também entra, mas nunca é consultada)
			vistos := make(map[rune]bool)
			for _, r := range campos[0] {
				if vistos[r] {
					continue
				}
				vistos[r] = true
				b.indice[r] = append(b.indice[r], idx)
			}
		}

		b.erroCarga = varredor.Err()
	})

	return b.erroCarga
}

// FrasesComCaractere devolve todas as frases que contêm o caractere dado.
func (b *BancoFrases) FrasesComCaractere(caractere rune) []Frase {
	if err := b.garantirCarregado(); err != nil {
		return nil
	}

	indices := b.indice[caractere]
	frases := make([]Frase, 0, len(indices))
	for _, idx := range indices {
		frases = append(frases, b.frases[idx])
	}
	return frases
}

// TemCaractere informa se existe ao menos uma frase contendo o caractere.
func (b *BancoFrases) TemCaractere(caractere rune) bool {
	if err := b.garantirCarregado(); err != nil {
		return false
	}
	return len(b.indice[caractere]) > 0
}

func (b *BancoFrases) TotalFrases() int {
	if err := b.garantirCarregado(); err != nil {
		return 0
	}
	return len(b.frases)
}
