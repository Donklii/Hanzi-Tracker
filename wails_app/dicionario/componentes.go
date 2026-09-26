package dicionario

import (
	"strconv"
	"strings"
)

// ----- Seção: Componentes Desenháveis de um Hanzi -----
//
// Fatia os traços de um caractere pelos componentes da sua decomposição, para as atividades que
// pedem o redesenho de UMA parte do hanzi. A fonte é o par decomposição + correspondências do
// makemeahanzi: a decomposição é uma expressão IDS em notação prefixa (um operador ⿰/⿱/... seguido
// dos seus operandos) e cada entrada de Correspondencias é o CAMINHO na árvore dessa expressão até a
// folha que o traço de mesmo índice compõe — em 器 (⿳⿰口口犬⿰口口), o traço de caminho [2,1] é do
// último 口. Os índices devolvidos são posições em "strokes"/"medians" do banco de traçados
// (BancoTracados), que é ordenado igual: um índice serve aos dois arquivos.

// COMPONENTE_DESCONHECIDO é o marcador que o makemeahanzi usa quando não soube nomear a parte.
const COMPONENTE_DESCONHECIDO = "？"

// TRACOS_MINIMOS_COMPONENTE descarta o componente de traço único (o 丨 de 串, por exemplo): redesenhar
// um traço só não exercita nada.
const TRACOS_MINIMOS_COMPONENTE = 2

// aridadeIDS é o número de operandos de cada operador de descrição ideográfica (bloco Unicode IDC).
var aridadeIDS = map[rune]int{
	'⿰': 2, '⿱': 2, '⿴': 2, '⿵': 2, '⿶': 2, '⿷': 2, '⿸': 2, '⿹': 2, '⿺': 2, '⿻': 2,
	'⿲': 3, '⿳': 3,
}

// ComponenteHanzi é uma folha da decomposição de um caractere com os índices dos traços que a formam.
type ComponenteHanzi struct {
	Caractere string `json:"caractere"`
	Tracos    []int  `json:"tracos"`
}

// ComponentesDesenhaveis devolve os componentes do caractere que servem a uma atividade de redesenho
// parcial, na ordem em que o primeiro traço de cada um é escrito. Cada componente tem ao menos
// TRACOS_MINIMOS_COMPONENTE traços e deixa traços de fora — redesenhar o caractere inteiro é outra
// atividade. Vem vazio quando o caractere não tem dados curados PRÓPRIOS: a grafia tradicional herda
// os do simplificado, cujos traços são outros, então aqui NÃO se usa o fallback de BuscarCaractere.
func (b *Banco) ComponentesDesenhaveis(caractere string) []ComponenteHanzi {
	entrada, existe := b.caracteres[caractere]
	if !existe || len(entrada.Correspondencias) == 0 {
		return nil
	}

	folhas := folhasIDS(entrada.Decomposicao)
	if len(folhas) < 2 {
		return nil
	}

	tracosPorCaminho, ordemDosCaminhos, _ := agruparTracosPorCaminho(entrada.Correspondencias)

	var componentes []ComponenteHanzi
	for _, chave := range ordemDosCaminhos {
		tracos := tracosPorCaminho[chave]
		if len(tracos) < TRACOS_MINIMOS_COMPONENTE || len(tracos) >= len(entrada.Correspondencias) {
			continue
		}
		folha, temFolha := folhas[chave]
		if !temFolha || folha == COMPONENTE_DESCONHECIDO {
			continue
		}
		componentes = append(componentes, ComponenteHanzi{Caractere: folha, Tracos: tracos})
	}
	return componentes
}

// ParticaoEmComponentes devolve TODOS os traços do caractere repartidos pelas folhas da decomposição,
// na ordem em que o primeiro traço de cada folha é escrito — o insumo da atividade de montagem, em
// que cada folha vira uma peça e o caractere inteiro é recomposto peça a peça. Diferente de
// ComponentesDesenhaveis, aqui a partição precisa ser TOTAL: qualquer traço sem componente atribuído
// ou de folha desconhecida deixaria uma peça impossível de nomear, então o caractere é recusado (vem
// vazio). Também vem vazio sem dados curados PRÓPRIOS — a grafia tradicional herda os do
// simplificado, cujos traços são outros, então NÃO se usa o fallback de BuscarCaractere.
func (b *Banco) ParticaoEmComponentes(caractere string) []ComponenteHanzi {
	entrada, existe := b.caracteres[caractere]
	if !existe || len(entrada.Correspondencias) == 0 {
		return nil
	}

	folhas := folhasIDS(entrada.Decomposicao)
	if len(folhas) < 2 {
		return nil
	}

	tracosPorCaminho, ordemDosCaminhos, tracosSemComponente := agruparTracosPorCaminho(entrada.Correspondencias)
	if tracosSemComponente > 0 || len(ordemDosCaminhos) < 2 {
		return nil
	}

	componentes := make([]ComponenteHanzi, 0, len(ordemDosCaminhos))
	for _, chave := range ordemDosCaminhos {
		folha, temFolha := folhas[chave]
		if !temFolha || folha == COMPONENTE_DESCONHECIDO {
			return nil
		}
		componentes = append(componentes, ComponenteHanzi{Caractere: folha, Tracos: tracosPorCaminho[chave]})
	}
	return componentes
}

// agruparTracosPorCaminho reparte os índices de traço pelo caminho de folha que cada um compõe,
// preservando a ordem de escrita (a ordem dos caminhos é a do primeiro traço de cada um). Também
// conta os traços que o makemeahanzi não atribuiu a componente nenhum — cada chamador decide se
// isso desqualifica o caractere.
func agruparTracosPorCaminho(correspondencias [][]int) (map[string][]int, []string, int) {
	tracosPorCaminho := make(map[string][]int, len(correspondencias))
	var ordemDosCaminhos []string
	tracosSemComponente := 0

	for indiceTraco, caminho := range correspondencias {
		if len(caminho) == 0 {
			tracosSemComponente++
			continue
		}
		chave := chaveCaminho(caminho)
		if _, jaVisto := tracosPorCaminho[chave]; !jaVisto {
			ordemDosCaminhos = append(ordemDosCaminhos, chave)
		}
		tracosPorCaminho[chave] = append(tracosPorCaminho[chave], indiceTraco)
	}
	return tracosPorCaminho, ordemDosCaminhos, tracosSemComponente
}

// folhasIDS mapeia o caminho de cada folha da expressão IDS (ver chaveCaminho) para o caractere dela.
// Devolve nil se a expressão estiver truncada — decomposição malformada não vira atividade.
func folhasIDS(decomposicao string) map[string]string {
	runas := []rune(decomposicao)
	folhas := make(map[string]string, len(runas))

	posicao := 0
	if !coletarFolhasIDS(runas, &posicao, nil, folhas) {
		return nil
	}
	return folhas
}

// coletarFolhasIDS consome UM nó da expressão a partir de posicao (que avança) e desce
// recursivamente pelos operandos. Devolve false quando os operandos acabam antes da aridade pedida.
func coletarFolhasIDS(runas []rune, posicao *int, caminho []int, folhas map[string]string) bool {
	if *posicao >= len(runas) {
		return false
	}

	runa := runas[*posicao]
	*posicao++

	aridade, ehOperador := aridadeIDS[runa]
	if !ehOperador {
		folhas[chaveCaminho(caminho)] = string(runa)
		return true
	}

	for operando := 0; operando < aridade; operando++ {
		// O append precisa copiar: os irmãos compartilhariam o array de baixo do caminho do pai.
		caminhoFilho := append(append([]int{}, caminho...), operando)
		if !coletarFolhasIDS(runas, posicao, caminhoFilho, folhas) {
			return false
		}
	}
	return true
}

// chaveCaminho serializa o caminho na árvore da decomposição ("0.1" = 2º operando do 1º operando).
func chaveCaminho(caminho []int) string {
	partes := make([]string, len(caminho))
	for i, indice := range caminho {
		partes[i] = strconv.Itoa(indice)
	}
	return strings.Join(partes, ".")
}
