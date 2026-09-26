package busca

import (
	"os"
	"path/filepath"
	"strings"
	"sync"

	"wails_app/dicionario"
)

// ----- Seção: Cache de Imagens de Hanzi e Palavras -----

var (
	travaImagens      sync.RWMutex
	mapaImagensHanzi  map[string]bool
	imagensCarregadas bool
)

// Diretórios onde o aplicativo busca por imagens de Hanzis/palavras
var diretoriosImagens = []string{
	filepath.Join("frontend", "src", "revisao", "imagens_hanzi"),
	filepath.Join("wails_app", "frontend", "src", "revisao", "imagens_hanzi"),
	filepath.Join("..", "frontend", "src", "revisao", "imagens_hanzi"),
	filepath.Join("..", "..", "frontend", "src", "revisao", "imagens_hanzi"),
	filepath.Join("frontend", "public", "imagens_hanzi"),
	filepath.Join("wails_app", "frontend", "public", "imagens_hanzi"),
	filepath.Join("..", "frontend", "public", "imagens_hanzi"),
	filepath.Join("..", "..", "frontend", "public", "imagens_hanzi"),
}


// CarregarImagensDisponiveis varre os diretórios de imagens e registra todas as palavras que possuem
// arquivo de imagem (seja 1 hanzi isolado ou palavra multi-hanzi, como 苹果.png).
func CarregarImagensDisponiveis() map[string]bool {
	travaImagens.Lock()
	defer travaImagens.Unlock()

	mapaImagensHanzi = make(map[string]bool)

	for _, dir := range diretoriosImagens {
		entradas, err := os.ReadDir(dir)
		if err != nil {
			continue
		}

		for _, entrada := range entradas {
			if entrada.IsDir() {
				continue
			}

			nome := entrada.Name()
			extensao := filepath.Ext(nome)
			extensaoMinuscula := strings.ToLower(extensao)

			if extensaoMinuscula == ".png" || extensaoMinuscula == ".jpg" || extensaoMinuscula == ".jpeg" || extensaoMinuscula == ".webp" || extensaoMinuscula == ".svg" {
				palavra := strings.TrimSuffix(nome, extensao)
				if palavra != "" {
					mapaImagensHanzi[palavra] = true
				}
			}
		}
	}

	imagensCarregadas = true
	return mapaImagensHanzi
}


// TemImagemParaHanzi verifica se existe uma imagem atribuída à palavra (1 hanzi ou multi-hanzi).
func TemImagemParaHanzi(palavra string) bool {
	if palavra == "" {
		return false
	}

	travaImagens.RLock()
	carregado := imagensCarregadas
	tem := mapaImagensHanzi[palavra]
	travaImagens.RUnlock()

	if carregado {
		return tem
	}

	mapa := CarregarImagensDisponiveis()
	return mapa[palavra]
}


// RecarregarImagensDisponiveis força a atualização do cache de imagens.
func RecarregarImagensDisponiveis() map[string]bool {
	travaImagens.Lock()
	imagensCarregadas = false
	travaImagens.Unlock()
	return CarregarImagensDisponiveis()
}


// DistintosPorSignificadoEComImagem aceita apenas candidatas que possuem imagem atribuída e significados distintos.
func DistintosPorSignificadoEComImagem(escolhidas []OpcaoRevisao, candidata dicionario.DecomposicaoHanzi) bool {
	if !TemImagemParaHanzi(candidata.Caractere) {
		return false
	}
	return DistintosPorSignificado(escolhidas, candidata)
}
