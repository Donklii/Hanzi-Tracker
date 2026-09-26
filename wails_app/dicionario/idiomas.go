package dicionario

import (
	"embed"
	"io/fs"
	"sort"
	"strings"
)

// ----- Recursos de dicionário por idioma (embed compartilhado) -----
//
// Aqui fica SÓ o que o app lê em runtime, e tudo já pronto para uso: idiomas/ é produto, não insumo.
// As fontes cruas (cedict.u8, makemeahanzi.txt, lista de frequência) vivem em dicionario/fontes/,
// fora deste embed — elas alimentam a fusão em tempo de build e não têm por que pesar no binário
// (o //go:embed abaixo é recursivo: qualquer arquivo solto sob idiomas/ entra no executável).
//
// Um diretório por idioma, nenhum privilegiado na ORGANIZAÇÃO — inglês é só mais uma pasta. O que
// muda é o RUNTIME: quando o idioma escolhido não tem um recurso, cai-se para o inglês
// (IdiomaPadrao). Dados language-neutral (traços dos hanzi) NÃO ficam aqui — ver tracados.go.
//
//	Layout:
//
//	idiomas/
//	  compartilhado/
//	    compreensao/*.jsonl.gz    exercícios de compreensão agnósticos de idioma (ex: C3)
//	  en/
//	    dicionario.jsonl.gz       makemeahanzi + CC-CEDICT + frequência fundidos (ver banco.go)
//	    frases/*.tsv.gz           pares chinês↔inglês
//	  pt-BR/
//	    dicionario.jsonl.gz       idem, com definições/significados em português
//	    frases/*.tsv.gz           pares chinês↔português (Tatoeba, DeepSeek complementar/gerado)
//
// Para adicionar um idioma novo: ponha as fontes em fontes/<código>/, rode
// `go run ./dicionario/fusao <código>` para gerar idiomas/<código>/dicionario.jsonl.gz e inclua o
// código em IdiomasSuportados. Recurso que faltar cai no fallback.

//go:embed idiomas
var arquivosIdiomas embed.FS

// IdiomaPadrao é o idioma de fallback quando o escolhido não possui um recurso.
const IdiomaPadrao = "en"

const (
	recursoDicionario = "dicionario.jsonl.gz"
	subdirFrases      = "frases"
)

// IdiomasSuportados lista os códigos de idioma oferecidos ao usuário. Alimenta o select de idioma nas
// configurações e valida a escolha vinda do instalador. Um idioma pode entrar aqui só com frases
// próprias (traduzidas por frases/complementar) e ainda cair no inglês para o dicionário de caracteres —
// é o caso do "es", que não tem dicionario.jsonl.gz próprio e usa o fallback (ver lerRecursoIdioma).
var IdiomasSuportados = []string{"en", "pt-BR", "es"}

// IdiomaValido diz se o código é um idioma suportado (com pasta própria de recursos).
func IdiomaValido(idioma string) bool {
	for _, s := range IdiomasSuportados {
		if s == idioma {
			return true
		}
	}
	return false
}

// normalizarIdioma devolve um idioma sempre utilizável: o próprio se suportado, senão o fallback.
func normalizarIdioma(idioma string) string {
	if IdiomaValido(idioma) {
		return idioma
	}
	return IdiomaPadrao
}

// lerRecursoIdioma lê idiomas/<idioma>/<nome>, caindo para idiomas/en/<nome> quando o idioma escolhido
// não tem aquele recurso. Devolve também o idioma efetivamente usado (útil para log/diagnóstico).
func lerRecursoIdioma(idioma, nome string) ([]byte, string, error) {
	idioma = normalizarIdioma(idioma)

	dados, err := arquivosIdiomas.ReadFile("idiomas/" + idioma + "/" + nome)
	if err == nil {
		return dados, idioma, nil
	}
	if idioma == IdiomaPadrao {
		return nil, idioma, err
	}

	dados, err = arquivosIdiomas.ReadFile("idiomas/" + IdiomaPadrao + "/" + nome)
	return dados, IdiomaPadrao, err
}

// dirFrasesIdioma devolve o diretório de frases a usar para o idioma: o próprio se tiver ao menos um
// arquivo .tsv.gz, senão o do inglês (fallback). Alimenta a carga preguiçosa do GerenciadorFrases.
func dirFrasesIdioma(idioma string) string {
	idioma = normalizarIdioma(idioma)
	dir := "idiomas/" + idioma + "/" + subdirFrases
	if idioma != IdiomaPadrao && !temArquivoFrases(dir) {
		return "idiomas/" + IdiomaPadrao + "/" + subdirFrases
	}
	return dir
}

// temArquivoFrases diz se o diretório embarcado tem ao menos um arquivo de frases (.tsv.gz).
func temArquivoFrases(dir string) bool {
	entradas, err := fs.ReadDir(arquivosIdiomas, dir)
	if err != nil {
		return false
	}
	for _, e := range entradas {
		if !e.IsDir() && strings.HasSuffix(e.Name(), sufixoArquivoFrases) {
			return true
		}
	}
	return false
}

// listarArquivosFrases devolve, em ordem, os nomes dos arquivos .tsv.gz do diretório de frases dado.
func listarArquivosFrases(dir string) ([]string, error) {
	entradas, err := fs.ReadDir(arquivosIdiomas, dir)
	if err != nil {
		return nil, err
	}
	var nomes []string
	for _, e := range entradas {
		if e.IsDir() || !strings.HasSuffix(e.Name(), sufixoArquivoFrases) {
			continue
		}
		nomes = append(nomes, e.Name())
	}
	sort.Strings(nomes)
	return nomes, nil
}
