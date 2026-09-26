package taxonomia

// ----- Seção: Taxonomia compartilhada de classificação de frases (tema + dificuldade) -----
//
// Fonte ÚNICA da verdade dos rótulos de classificação, em chinês simplificado. Nasceu dentro de
// frases/classificar; foi promovida a pacote próprio quando frases/gerar passou a precisar da MESMA
// taxonomia (o gerador pede ao DeepSeek que já classifique cada frase criada). Deixar duas cópias do
// esquema é dívida garantida — elas divergem e a classificação do gerador sairia incompatível com a do
// classificador. Aqui mora o esquema e os utilitários de validação/casamento que ambos consomem.
//
// Os rótulos são em chinês (poupa tokens de saída do DeepSeek e é filtrável). A glosa em português ao
// lado de cada tema é só documentação — o dado gravado é sempre o rótulo chinês.

import (
	"sort"
	"strings"
)

// TEMAS é a taxonomia FECHADA de temas. Um por frase; 其他 ("outros") é a válvula de escape para o que
// não se encaixa em nenhum outro.
var TEMAS = []string{
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

// TEMA_OUTROS é a válvula de escape da taxonomia (último item de TEMAS). Serve de rede quando o modelo
// devolve um tema fora da lista, mas com o resto válido: cai aqui em vez de descartar a linha.
const TEMA_OUTROS = "其他"

// DIFICULDADES é a escala FECHADA de dificuldade, do mais fácil ao mais difícil.
// Mapeamento para o português: 入门→iniciante, 初级→fácil, 中级→médio, 高级→avançado.
var DIFICULDADES = []string{"入门", "初级", "中级", "高级"}


// ----- Validação e casamento -----

// TemaValido diz se o rótulo é um tema da taxonomia fechada.
func TemaValido(tema string) bool {
	return contem(TEMAS, tema)
}


// DificuldadeValida diz se o rótulo é uma dificuldade da escala fechada.
func DificuldadeValida(dificuldade string) bool {
	return contem(DIFICULDADES, dificuldade)
}


// TemasPorTamanho devolve TEMAS ordenado do rótulo mais longo ao mais curto, para o casamento por
// substring pegar o rótulo mais específico primeiro (defensivo — os atuais não são substring uns dos
// outros, mas a garantia evita surpresa se a taxonomia crescer).
func TemasPorTamanho() []string {
	copia := append([]string(nil), TEMAS...)
	sort.SliceStable(copia, func(i, j int) bool { return len(copia[i]) > len(copia[j]) })
	return copia
}


// PrimeiraOcorrencia devolve o primeiro rótulo de `rotulos` que aparece como substring em `linha`, ou
// "" se nenhum aparecer. Usada para extrair o rótulo válido de uma resposta ruidosa do modelo.
func PrimeiraOcorrencia(linha string, rotulos []string) string {
	for _, rotulo := range rotulos {
		if strings.Contains(linha, rotulo) {
			return rotulo
		}
	}
	return ""
}


// ----- Utilitários -----

func contem(lista []string, alvo string) bool {
	for _, item := range lista {
		if item == alvo {
			return true
		}
	}
	return false
}
