package main

import (
	"fmt"
	"strings"

	"wails_app/dicionario"
	"wails_app/progresso"
)

// ----- Migração: conserto de grafias corrompidas pelo antigo bug de conversão 么→幺 -----
//
// Antes do conserto da colisão 么/幺 (ver dicionario/banco.go, removerConversoesSimplificadoEspurias),
// a conversão "para simplificado" reescrevia 么→幺 e podia gravar no vocabulário palavras que não
// existem (什么→什幺, 怎么→怎幺, 那么→那幺...). Esta migração roda uma vez no arranque, com o dicionário
// e o banco já prontos: para cada grafia salva que o dicionário NÃO reconhece, desfaz a troca (幺→么)
// e, se o resultado passar a existir, renomeia a entrada preservando o progresso. Grafias inválidas
// que NÃO casam com esse padrão são preservadas (podem ser palavras raras legítimas) e só reportadas —
// nunca removidas às cegas.

// O par exato que o bug trocava: a partícula 么 (U+4E48) virava 幺 (U+5E7A, "yao").
const (
	RUNA_CORROMPIDA = "幺"
	RUNA_CORRETA    = "么"
)

// corrigirVocabularioCorrompido varre o vocabulário e conserta as grafias corrompidas pelo bug 么→幺.
// Devolve quantas corrigiu e a lista das que ficaram inválidas sem conserto aplicável (só reportadas).
func corrigirVocabularioCorrompido(dic *dicionario.GerenciadorDicionario) (corrigidas int, invalidasSemConserto []string) {
	if dic == nil || dic.Banco == nil {
		return 0, nil
	}

	hanzis, err := progresso.ListarHanzisVocab()
	if err != nil {
		fmt.Printf("Aviso: migração de vocabulário não pôde listar as palavras: %v\n", err)
		return 0, nil
	}

	for _, hanzi := range hanzis {
		corrigido, temConserto := desfazerTrocaMeYao(dic, hanzi)
		if !temConserto {
			if !grafiaReconhecida(dic, hanzi) {
				invalidasSemConserto = append(invalidasSemConserto, hanzi)
			}
			continue
		}
		if err := progresso.RenomearHanziVocab(hanzi, corrigido); err != nil {
			fmt.Printf("Aviso: falha ao corrigir %q→%q no vocabulário: %v\n", hanzi, corrigido, err)
			continue
		}
		corrigidas++
	}
	return corrigidas, invalidasSemConserto
}

// desfazerTrocaMeYao decide o conserto de UMA grafia: devolve (corrigida, true) só quando a grafia é
// desconhecida pelo dicionário E desfazer a troca 幺→么 a torna reconhecida. Nos demais casos devolve
// ("", false) — grafia já válida ou resíduo que não casa com o padrão do bug. Função pura (sem banco).
func desfazerTrocaMeYao(dic *dicionario.GerenciadorDicionario, hanzi string) (string, bool) {
	if grafiaReconhecida(dic, hanzi) {
		return "", false
	}
	corrigido := strings.ReplaceAll(hanzi, RUNA_CORROMPIDA, RUNA_CORRETA)
	if corrigido == hanzi || !grafiaReconhecida(dic, corrigido) {
		return "", false
	}
	return corrigido, true
}

// grafiaReconhecida diz se o dicionário conhece a palavra/caractere (por qualquer grafia) ou se é uma
// abreviação visual de radical — os casos que NÃO são resíduo do bug e devem ser preservados.
func grafiaReconhecida(dic *dicionario.GerenciadorDicionario, hanzi string) bool {
	if _, ehAbrev := dicionario.MapaAbrevParaCompleto[hanzi]; ehAbrev {
		return true
	}
	if len(dic.Banco.Buscar(hanzi)) > 0 {
		return true
	}
	return dic.Banco.BuscarCaractere(hanzi) != nil
}
