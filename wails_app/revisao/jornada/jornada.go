package jornada

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"sync"
)

// ----- Árvore da Jornada -----
// Conteúdo da Jornada de Revisão: ramos temáticos com níveis sequenciais, cada nível com as palavras
// que ele ensina e a sequência de revisões (mini-sessões) que o usuário conclui em ordem.
//
// O conteúdo é DADO embarcado (arvore.json). As palavras vêm em chinês simplificado e SEM pinyin ou
// significado: esses vêm do dicionário em runtime, para não duplicar (e envelhecer) o dado curado.
// Este pacote não depende de nenhum outro pacote do app — só carrega e valida a árvore.

//go:embed arvore.json
var dadosArvore []byte

// ----- Tipos -----

type Mapa struct {
	Largura int `json:"largura"`
	Altura  int `json:"altura"`
}

type Arvore struct {
	Mapa  Mapa   `json:"mapa"`
	Ramos []Ramo `json:"ramos"`
}

type Ramo struct {
	Id          string  `json:"id"`
	Nome        string  `json:"nome"`
	Icone       string  `json:"icone"`
	Dificuldade string  `json:"dificuldade"`
	Pai         string  `json:"pai"` // vazio = raiz da árvore
	Tema        string  `json:"tema"`
	RotuloMapa  []int   `json:"rotuloMapa"` // [x, y] do rótulo do ramo no mapa
	Niveis      []Nivel `json:"niveis"`
}

type Nivel struct {
	Id       string   `json:"id"` // derivado na carga: "<ramoId>_<indice>"
	Titulo   string   `json:"titulo"`
	Pos      []int    `json:"pos"` // [x, y] do nó no mapa
	Palavras []string `json:"palavras"`
	Revisoes []string `json:"revisoes"`
}

// ----- Vocabulário fechado do conteúdo -----

// Tipos de revisão aceitos em Nivel.Revisoes. O mapeamento para os modos reais da revisão é feito no
// pacote revisao (RevisoesEfetivas / ObterQuestoesJornada), não aqui.
const (
	RevisaoPalavras  = "palavras"
	RevisaoFonetica  = "fonetica"
	RevisaoDesenho   = "desenho"
	RevisaoFrases    = "frases"
	RevisaoPronuncia = "pronuncia"
	RevisaoMista     = "mista"
)

var revisoesValidas = map[string]bool{
	RevisaoPalavras:  true,
	RevisaoFonetica:  true,
	RevisaoDesenho:   true,
	RevisaoFrases:    true,
	RevisaoPronuncia: true,
	RevisaoMista:     true,
}

var dificuldadesValidas = map[string]bool{
	"iniciante":     true,
	"intermediario": true,
	"avancado":      true,
}

// TemasValidos é a taxonomia fechada de temas das frases, copiada de TEMAS em
// dicionario/frases/classificar/classificar_frases.go (aquele arquivo é um programa standalone —
// package main —, então não dá para importar a lista; ao mexer lá, mexer aqui também).
var TemasValidos = []string{
	"日常生活", // cotidiano
	"家庭",   // família
	"工作职业", // trabalho / profissão
	"学习教育", // estudo / educação
	"饮食",   // comida / bebida
	"购物",   // compras
	"旅行交通", // viagem / transporte
	"健康身体", // saúde / corpo
	"情感",   // emoções / sentimentos
	"时间日期", // tempo / data
	"天气自然", // clima / natureza
	"社交问候", // social / saudações
	"兴趣娱乐", // interesses / lazer
	"科技",   // tecnologia
	"数字量词", // números / quantidades
	"地点方位", // lugares / direções
	"语言文化", // língua / cultura
	"其他",   // outros
}

const minimoPalavrasPorNivel = 3

// ----- Carga -----

var (
	carregarUmaVez sync.Once
	arvoreCache    Arvore
	erroCarga      error
)

// ObterArvore devolve a árvore embarcada, carregada e validada uma única vez. Conteúdo inválido é
// erro barulhento: a Jornada inteira depende dele, então não há degradação silenciosa.
func ObterArvore() (Arvore, error) {
	carregarUmaVez.Do(func() {
		var arvore Arvore
		if err := json.Unmarshal(dadosArvore, &arvore); err != nil {
			erroCarga = fmt.Errorf("arvore.json inválido: %w", err)
			return
		}
		preencherIdsNiveis(&arvore)
		if err := validarArvore(arvore); err != nil {
			erroCarga = err
			return
		}
		arvoreCache = arvore
	})

	return arvoreCache, erroCarga
}

// ObterNivel devolve o nível e o ramo a que ele pertence.
func ObterNivel(nivelId string) (Nivel, Ramo, error) {
	arvore, err := ObterArvore()
	if err != nil {
		return Nivel{}, Ramo{}, err
	}

	for _, ramo := range arvore.Ramos {
		for _, nivel := range ramo.Niveis {
			if nivel.Id == nivelId {
				return nivel, ramo, nil
			}
		}
	}

	return Nivel{}, Ramo{}, fmt.Errorf("nível %q não existe na árvore da jornada", nivelId)
}

// PalavrasAteNivel devolve o vocabulário que a Jornada já ENSINOU quando o usuário chega ao nível:
// as palavras dos ramos ancestrais (o caminho percorrido até aqui, na ordem da raiz para baixo), as
// dos níveis anteriores do mesmo ramo e as do próprio nível. É o universo FECHADO das questões da
// Jornada — ela não empresta palavras do vocabulário do usuário (nem "em estudo", nem do foco), para
// o modo ser independente das outras revisões.
func PalavrasAteNivel(nivelId string) ([]string, error) {
	arvore, err := ObterArvore()
	if err != nil {
		return nil, err
	}

	ramoPorId := make(map[string]Ramo, len(arvore.Ramos))
	for _, ramo := range arvore.Ramos {
		ramoPorId[ramo.Id] = ramo
	}

	nivel, ramo, err := ObterNivel(nivelId)
	if err != nil {
		return nil, err
	}

	// Cadeia de ancestrais, do mais distante para o mais próximo (o ramo do nível fica de fora: os
	// níveis dele entram até o atual, logo abaixo). Guarda contra pai cíclico em conteúdo inválido.
	var ancestrais []Ramo
	visitados := map[string]bool{ramo.Id: true}
	for paiId := ramo.Pai; paiId != "" && !visitados[paiId]; {
		pai, existe := ramoPorId[paiId]
		if !existe {
			break
		}
		visitados[paiId] = true
		ancestrais = append([]Ramo{pai}, ancestrais...)
		paiId = pai.Pai
	}

	var palavras []string
	jaIncluida := make(map[string]bool)
	adicionar := func(lista []string) {
		for _, palavra := range lista {
			if jaIncluida[palavra] {
				continue
			}
			jaIncluida[palavra] = true
			palavras = append(palavras, palavra)
		}
	}

	for _, ancestral := range ancestrais {
		for _, n := range ancestral.Niveis {
			adicionar(n.Palavras)
		}
	}
	for _, n := range ramo.Niveis {
		adicionar(n.Palavras)
		if n.Id == nivel.Id {
			break
		}
	}

	return palavras, nil
}

// preencherIdsNiveis deriva o id de cada nível a partir do ramo e da posição (não vem do JSON).
func preencherIdsNiveis(arvore *Arvore) {
	for iRamo := range arvore.Ramos {
		ramo := &arvore.Ramos[iRamo]
		for iNivel := range ramo.Niveis {
			ramo.Niveis[iNivel].Id = fmt.Sprintf("%s_%d", ramo.Id, iNivel)
		}
	}
}

// ----- Validação -----

func validarArvore(arvore Arvore) error {
	if arvore.Mapa.Largura <= 0 || arvore.Mapa.Altura <= 0 {
		return fmt.Errorf("mapa da jornada sem dimensões válidas (largura=%d altura=%d)", arvore.Mapa.Largura, arvore.Mapa.Altura)
	}
	if len(arvore.Ramos) == 0 {
		return fmt.Errorf("árvore da jornada sem ramos")
	}

	idsRamo := make(map[string]bool, len(arvore.Ramos))
	for _, ramo := range arvore.Ramos {
		if ramo.Id == "" {
			return fmt.Errorf("ramo sem id")
		}
		if idsRamo[ramo.Id] {
			return fmt.Errorf("ramo %q duplicado", ramo.Id)
		}
		idsRamo[ramo.Id] = true
	}

	for _, ramo := range arvore.Ramos {
		if err := validarRamo(ramo, idsRamo); err != nil {
			return err
		}
	}

	return nil
}

func validarRamo(ramo Ramo, idsRamo map[string]bool) error {
	if ramo.Pai != "" && !idsRamo[ramo.Pai] {
		return fmt.Errorf("ramo %q aponta para o pai inexistente %q", ramo.Id, ramo.Pai)
	}
	if ramo.Pai == ramo.Id {
		return fmt.Errorf("ramo %q é pai de si mesmo", ramo.Id)
	}
	if !dificuldadesValidas[ramo.Dificuldade] {
		return fmt.Errorf("ramo %q com dificuldade inválida %q", ramo.Id, ramo.Dificuldade)
	}
	if ramo.Tema != "" && !temaValido(ramo.Tema) {
		return fmt.Errorf("ramo %q com tema %q fora da taxonomia de frases", ramo.Id, ramo.Tema)
	}
	if len(ramo.RotuloMapa) != 2 {
		return fmt.Errorf("ramo %q sem rotuloMapa [x, y]", ramo.Id)
	}
	if len(ramo.Niveis) == 0 {
		return fmt.Errorf("ramo %q sem níveis", ramo.Id)
	}

	for _, nivel := range ramo.Niveis {
		if err := validarNivel(nivel); err != nil {
			return err
		}
	}

	return nil
}

func validarNivel(nivel Nivel) error {
	if nivel.Titulo == "" {
		return fmt.Errorf("nível %q sem título", nivel.Id)
	}
	if len(nivel.Pos) != 2 {
		return fmt.Errorf("nível %q sem pos [x, y]", nivel.Id)
	}
	if len(nivel.Palavras) < minimoPalavrasPorNivel {
		return fmt.Errorf("nível %q tem %d palavra(s); o mínimo é %d", nivel.Id, len(nivel.Palavras), minimoPalavrasPorNivel)
	}
	if len(nivel.Revisoes) == 0 {
		return fmt.Errorf("nível %q sem revisões", nivel.Id)
	}

	totalMistas := 0
	for _, tipo := range nivel.Revisoes {
		if !revisoesValidas[tipo] {
			return fmt.Errorf("nível %q com tipo de revisão inválido %q", nivel.Id, tipo)
		}
		if tipo == RevisaoMista {
			totalMistas++
		}
	}
	if totalMistas != 1 {
		return fmt.Errorf("nível %q tem %d revisão(ões) %q; deve ter exatamente 1", nivel.Id, totalMistas, RevisaoMista)
	}
	if nivel.Revisoes[len(nivel.Revisoes)-1] != RevisaoMista {
		return fmt.Errorf("nível %q não termina com a revisão %q", nivel.Id, RevisaoMista)
	}

	return nil
}

func temaValido(tema string) bool {
	for _, valido := range TemasValidos {
		if valido == tema {
			return true
		}
	}
	return false
}
