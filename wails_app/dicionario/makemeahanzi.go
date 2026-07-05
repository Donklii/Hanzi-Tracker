package dicionario

import (
	"bufio"
	"bytes"
	_ "embed"
	"encoding/json"
	"strings"
)


// ----- Banco de Dados Embarcado -----

//go:embed makemeahanzi_dictionary.txt
var dadosDicionario []byte


// ----- Mapa de Abreviações Visuais -----

// Caracteres do bloco CJK Radicals Supplement e outros componentes visuais comuns
// mapeados para seus caracteres CJK completos equivalentes.
var MapaAbrevParaCompleto = map[string]string{
	// CJK Radicals Supplement
	"⺀": "冫", // gelo
	"⺈": "刀", // faca
	"⺊": "卜", // adivinhação
	"⺌": "小", // pequeno (forma superior)
	"⺍": "小", // pequeno (variante)
	"⺗": "心", // coração (forma inferior)
	"⺮": "竹", // bambu (forma superior)
	"⺳": "网", // rede (forma superior)
	"⺼": "肉", // carne (parece 月)
	
	// Componentes e Radicais Avulsos (para não virarem cards separados)
	"氵": "水",
	"冫": "冰",
	"亻": "人",
	"艹": "草",
	"扌": "手",
	"阝": "邑", // ou 阜, mapeado para 邑 convencionalmente
	"犭": "犬",
	"忄": "心",
	"辶": "走",
	"廴": "建",
	"彳": "行",
	"刂": "刀",
	"灬": "火",
	"糹": "糸",
	"纟": "糸",
	"釒": "金",
	"钅": "金",
	"飠": "食",
	"饣": "食",
	"衤": "衣",
	"礻": "示",
	"疒": "病",
	"罒": "网",
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

	// candidatosRevisao: subconjunto com definição E pinyin, excluindo abreviações visuais.
	// É o universo de sorteio da revisão de hanzis (alvos e distratores) — cacheado no Carregar
	// para as consultas aleatórias não varrerem o mapa inteiro a cada questão.
	candidatosRevisao []DecomposicaoHanzi
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
	for abrev, completo := range MapaAbrevParaCompleto {
		reverso[completo] = append(reverso[completo], abrev)
	}

	for caractereCompleto, listaAbrev := range reverso {
		if entrada, existe := b.entradas[caractereCompleto]; existe {
			entrada.Abreviacoes = listaAbrev
			b.entradas[caractereCompleto] = entrada
		}
	}

	// ----- Terceira passagem: cachear os candidatos da revisão -----
	for caractere, entrada := range b.entradas {
		if _, ehAbrev := MapaAbrevParaCompleto[caractere]; ehAbrev {
			continue
		}
		if entrada.Definicao == "" || len(entrada.Pinyin) == 0 || entrada.Pinyin[0] == "" {
			continue
		}
		b.candidatosRevisao = append(b.candidatosRevisao, entrada)
	}

	return erroVarredor
}

// CandidatosRevisao devolve os caracteres elegíveis para a revisão (com definição e pinyin).
// O slice retornado é compartilhado — o chamador não deve modificá-lo.
func (b *BancoMakeMeAHanzi) CandidatosRevisao() []DecomposicaoHanzi {
	return b.candidatosRevisao
}


func (b *BancoMakeMeAHanzi) Buscar(hanzi string) *DecomposicaoHanzi {
	if entrada, existe := b.entradas[hanzi]; existe {
		return &entrada
	}

	return nil
}


// CaractereCompleto retorna o caractere CJK completo se o argumento for
// uma abreviação visual ou componente. Retorna string vazia (ou próprio caractere) caso contrário.
func (b *BancoMakeMeAHanzi) CaractereCompleto(abrev string) string {
	if completo, existe := MapaAbrevParaCompleto[abrev]; existe {
		return completo
	}
	return abrev
}

func (b *BancoMakeMeAHanzi) TotalHanzis() int {
	return len(b.entradas)
}

// TodosCaracteres devolve todos os caracteres indexados, EXCETO as abreviações visuais de radical
// (bloco CJK Radicals Supplement), que não são palavras faláveis. Alimenta o pré-carregamento do
// cache de TTS (ver tts_precache.go) junto com o CC-CEDICT.
func (b *BancoMakeMeAHanzi) TodosCaracteres() []string {
	caracteres := make([]string, 0, len(b.entradas))
	for caractere := range b.entradas {
		if _, ehAbrev := MapaAbrevParaCompleto[caractere]; ehAbrev {
			continue
		}
		caracteres = append(caracteres, caractere)
	}
	return caracteres
}

// BuscarCompostosPor retorna até 100 caracteres que utilizam o caractere dado como componente
func (b *BancoMakeMeAHanzi) BuscarCompostosPor(componente string) []string {
	var resultados []string
	if componente == "" {
		return resultados
	}
	for char, entrada := range b.entradas {
		if char != componente && strings.Contains(entrada.Decomposicao, componente) {
			resultados = append(resultados, char)
			if len(resultados) >= 100 {
				break
			}
		}
	}
	return resultados
}

// BuscarGeral realiza a busca na base do MakeMeAHanzi por hanzi, pinyin ou significado
func (b *BancoMakeMeAHanzi) BuscarGeral(termo string) []DecomposicaoHanzi {
	var resultados []DecomposicaoHanzi
	if termo == "" {
		return resultados
	}

	termoLower := strings.ToLower(termo)
	cleanTermo := termoLower
	for _, num := range []string{"1", "2", "3", "4", "5", " "} {
		cleanTermo = strings.ReplaceAll(cleanTermo, num, "")
	}

	var priority []DecomposicaoHanzi
	var secondary []DecomposicaoHanzi

	for char, e := range b.entradas {
		if _, ehAbrev := MapaAbrevParaCompleto[char]; ehAbrev {
			continue // Ignora componentes e abreviações na busca geral
		}

		isPriority := false
		isSecondary := false

		// Hanzi
		if strings.Contains(e.Caractere, termo) {
			isPriority = true
		}

		// Pinyin
		if !isPriority {
			for _, p := range e.Pinyin {
				pinyinClean := RemoverTonsPinyin(strings.ToLower(p))
				pinyinClean = strings.ReplaceAll(pinyinClean, " ", "")
				if strings.Contains(pinyinClean, cleanTermo) {
					isPriority = true
					break
				}
			}
		}

		// Definição
		if !isPriority {
			if strings.Contains(strings.ToLower(e.Definicao), termoLower) {
				isSecondary = true
			}
		}

		if isPriority {
			priority = append(priority, e)
		} else if isSecondary {
			secondary = append(secondary, e)
		}
	}

	resultados = append(resultados, priority...)
	resultados = append(resultados, secondary...)
	return resultados
}
