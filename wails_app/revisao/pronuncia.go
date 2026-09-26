package revisao

import (
	"fmt"

	"wails_app/dicionario"
)

// ----- Seção: Modo de Pronúncia -----

// montarQuestaoPronuncia monta a questão de pronúncia com validação de fala do microfone: frase,
// sequência (frase 100% conhecida) ou baralho de cartas — frase e cartas vêm do módulo central de
// busca.
func (r *GerenciadorRevisao) montarQuestaoPronuncia(questao QuestaoRevisao, alvo alvoRevisao, pools poolsRevisao, varianteForcada string) (QuestaoRevisao, error) {
	if varianteForcada != "" {
		questao.Variante = varianteForcada
	} else {
		questao.Variante = r.sortearVariante(VariantePronunciaFrase, VariantePronunciaSequencia, VariantePronunciaBaralho, VariantePronunciaTipo)
	}

	if questao.Variante == VariantePronunciaFrase || questao.Variante == VariantePronunciaSequencia {
		if !r.preencherFrase(&questao) {
			return questao, fmt.Errorf("nenhuma frase contém %q", questao.Hanzi)
		}
		return questao, nil
	}

	var cartas []dicionario.DecomposicaoHanzi
	if questao.Variante == VariantePronunciaTipo {
		cartas = buscarBaralhoPronunciaTipo(alvo.entrada, pools)
	} else {
		cartas = buscarBaralhoPronuncia(alvo.entrada, pools)
	}

	questao.FraseOriginalSegmentada = make([]PalavraRevisao, len(cartas))
	for i, entrada := range cartas {
		pinyin := ""
		if len(entrada.Pinyin) > 0 {
			pinyin = entrada.Pinyin[0]
		}
		questao.FraseOriginalSegmentada[i] = PalavraRevisao{
			Texto:        entrada.Caractere,
			Pinyin:       pinyin,
			Significados: []string{entrada.Definicao},
			EhChines:     true,
		}
	}

	return questao, nil
}
