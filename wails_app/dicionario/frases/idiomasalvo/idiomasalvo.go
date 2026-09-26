// Package idiomasalvo é a fonte ÚNICA dos idiomas-alvo do pipeline de frases: a pasta do acervo, o
// código do Google Tradutor e o nome em chinês injetado nos prompts do DeepSeek. Consumido por
// frases/gerar (itera todos), frases/complementar e frases/revisar (buscam um pelo código).
//
// Adicionar um idioma ao pipeline = acrescentar UMA entrada em Todos (mais registrar o código em
// dicionario.IdiomasSuportados, para o app aceitá-lo em runtime).
package idiomasalvo

// Idioma descreve um idioma-alvo do pipeline de frases.
//
//	Dir          nome da pasta do acervo (idiomas/<Dir>/) e do diretório de trabalho (frases/<Dir>/).
//	CodigoGoogle código de idioma da Cloud Translation API (pt, en, es, …).
//	NomeChines   nome do idioma EM chinês, injetado nos prompts de veredicto/tradução do DeepSeek
//	             (os prompts vão em chinês para poupar tokens de entrada — ver frases/revisar).
type Idioma struct {
	Dir          string
	CodigoGoogle string
	NomeChines   string
}

// Todos lista os idiomas-alvo do pipeline de frases, na ordem em que gerar os processa.
var Todos = []Idioma{
	{Dir: "en", CodigoGoogle: "en", NomeChines: "英语"},
	{Dir: "pt-BR", CodigoGoogle: "pt", NomeChines: "巴西葡萄牙语"},
	{Dir: "es", CodigoGoogle: "es", NomeChines: "西班牙语"},
}

// PorDir devolve o idioma-alvo de pasta `dir` (en, pt-BR, es…) e se ele existe no registro.
func PorDir(dir string) (Idioma, bool) {
	for _, i := range Todos {
		if i.Dir == dir {
			return i, true
		}
	}
	return Idioma{}, false
}
