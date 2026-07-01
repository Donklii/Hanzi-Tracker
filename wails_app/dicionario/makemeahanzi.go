package dicionario

import (
	"bufio"
	"bytes"
	_ "embed"
	"encoding/json"
)


// ----- Banco de Dados Embarcado -----

//go:embed makemeahanzi_dictionary.txt
var dadosDicionario []byte


// ----- Mapa de Abreviações Visuais -----

// Caracteres do bloco CJK Radicals Supplement (U+2E80–U+2EFF)
// mapeados para seus caracteres CJK completos equivalentes.
var mapaAbrevParaCompleto = map[string]string{
	"⺀": "冫", // gelo
	"⺈": "刀", // faca
	"⺊": "卜", // adivinhação
	"⺌": "小", // pequeno (forma superior)
	"⺍": "小", // pequeno (variante)
	"⺗": "心", // coração (forma inferior)
	"⺮": "竹", // bambu (forma superior)
	"⺳": "网", // rede (forma superior)
	"⺼": "肉", // carne (parece 月)
}


// ----- Estruturas -----

type Etimologia struct {
	Tipo      string `json:"type,omitempty"`
	Fonetica  string `json:"phonetic,omitempty"`
	Semantica string `json:"semantic,omitempty"`
	Dica      string `json:"hint,omitempty"`
}

type DecomposicaoHanzi struct {
	Caractere    string     `json:"character"`
	Definicao    string     `json:"definition,omitempty"`
	Pinyin       []string   `json:"pinyin,omitempty"`
	Decomposicao string     `json:"decomposition"`
	Etimologia   Etimologia `json:"etymology,omitempty"`
	Radical      string     `json:"radical"`
	Abreviacoes  []string   `json:"abreviacoes,omitempty"`
}

type BancoMakeMeAHanzi struct {
	entradas map[string]DecomposicaoHanzi
}


// ----- Inicialização -----

func NovoBancoMakeMeAHanzi() *BancoMakeMeAHanzi {
	return &BancoMakeMeAHanzi{
		entradas: make(map[string]DecomposicaoHanzi),
	}
}


// ----- Métodos Públicos -----

func (b *BancoMakeMeAHanzi) Carregar() error {
	leitor := bytes.NewReader(dadosDicionario)
	varredor := bufio.NewScanner(leitor)

	for varredor.Scan() {
		linha := varredor.Text()
		if linha == "" {
			continue
		}

		var entrada DecomposicaoHanzi
		err := json.Unmarshal([]byte(linha), &entrada)

		if err == nil && entrada.Caractere != "" {
			b.entradas[entrada.Caractere] = entrada
		}
	}

	erroVarredor := varredor.Err()

	// ----- Segunda passagem: vincular abreviações aos caracteres completos -----
	reverso := make(map[string][]string)
	for abrev, completo := range mapaAbrevParaCompleto {
		reverso[completo] = append(reverso[completo], abrev)
	}

	for caractereCompleto, listaAbrev := range reverso {
		if entrada, existe := b.entradas[caractereCompleto]; existe {
			entrada.Abreviacoes = listaAbrev
			b.entradas[caractereCompleto] = entrada
		}
	}

	return erroVarredor
}


func (b *BancoMakeMeAHanzi) Buscar(hanzi string) *DecomposicaoHanzi {
	if entrada, existe := b.entradas[hanzi]; existe {
		return &entrada
	}

	return nil
}


// CaractereCompleto retorna o caractere CJK completo se o argumento for
// uma abreviação visual. Retorna string vazia caso contrário.
func (b *BancoMakeMeAHanzi) CaractereCompleto(abrev string) string {
	if completo, existe := mapaAbrevParaCompleto[abrev]; existe {
		return completo
	}
	return abrev
}

func (b *BancoMakeMeAHanzi) TotalHanzis() int {
	return len(b.entradas)
}
