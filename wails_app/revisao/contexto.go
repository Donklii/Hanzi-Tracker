package revisao

import (
	"fmt"
	"math/rand/v2"
	"strings"

	"wails_app/dicionario"
)

// ----- Seção: Modo de Contexto -----

// montarQuestaoContexto monta a questão de compreensão baseada em contexto ou tradução da frase.
// A frase e as alternativas vêm do módulo central de busca; aqui só se declaram a variante e o
// critério (distrator ausente da frase).
func (r *GerenciadorRevisao) montarQuestaoContexto(questao QuestaoRevisao, alvo alvoRevisao, pools poolsRevisao, varianteForcada string) (QuestaoRevisao, error) {
	var tentativas []string
	if varianteForcada != "" {
		tentativas = []string{varianteForcada}
	} else {
		opcoesVariantes := append(VariantesDoModo(ModoContexto), r.variantesExtras(ModoContexto)...)
		var desativadas []string
		if r.Config != nil {
			desativadas = r.Config().AtividadesDesativadas
		}

		tentativas = removerDesativadas(opcoesVariantes, desativadas)
		if len(tentativas) == 0 {
			tentativas = opcoesVariantes
		}
	}

	for len(tentativas) > 0 {
		varEscolhida := r.sortearVariante(tentativas...)
		q := questao
		q.Variante = varEscolhida

		if varEscolhida == VarianteOrdenacao || varEscolhida == VarianteOrdenacaoTraducao {
			res, err := r.preencherQuestaoOrdenacao(q, alvo, pools)
			if err == nil {
				return res, nil
			}
		} else if varEscolhida == VarianteCompreensao || varEscolhida == VarianteCompreensaoTraduzida || varEscolhida == VarianteRespostaDialogo {
			res, err := r.preencherQuestaoCompreensao(q, alvo, pools)
			if err == nil {
				return res, nil
			}
		} else if varEscolhida == VarianteTraducaoContexto {
			if r.preencherFrase(&q) {
				if err := r.buscarOpcoesTraducaoContexto(&q); err == nil {
					return q, nil
				}
			}
		} else {
			if err := r.preencherQuestaoLacuna(&q, alvo, pools); err == nil {
				return q, nil
			}
		}

		novas := make([]string, 0, len(tentativas)-1)
		for _, v := range tentativas {
			if v != varEscolhida {
				novas = append(novas, v)
			}
		}
		tentativas = novas
	}

	return questao, fmt.Errorf("não foi possível montar nenhuma variante de contexto para %q", alvo.entrada.Caractere)
}

// preencherQuestaoLacuna completa a questão de lacuna (VarianteContexto): a frase com a palavra
// escondida e as alternativas. É reusada pelo modo de significado, que na Jornada empresta esta
// atividade como o degrau avançado da escada dele (ver Buscador.EstenderEscada).
func (r *GerenciadorRevisao) preencherQuestaoLacuna(questao *QuestaoRevisao, alvo alvoRevisao, pools poolsRevisao) error {
	if !r.preencherFrase(questao) {
		return fmt.Errorf("nenhuma frase contém %q", questao.Hanzi)
	}

	// Distrator não pode aparecer na frase — estaria "correto" aos olhos do usuário.
	frase := questao.FraseOriginal
	aceitar := func(escolhidas []OpcaoRevisao, candidata dicionario.DecomposicaoHanzi) bool {
		return distintosPorHanzi(escolhidas, candidata) && !strings.Contains(frase, candidata.Caractere)
	}
	opcoes := r.buscarOpcoes(alvo.entrada, alvo.emFoco, pools, aceitar)
	questao.Opcoes = opcoes

	var pecasEsperadas []string
	for _, p := range questao.FraseOriginalSegmentada {
		if p.EhLacuna {
			pecasEsperadas = append(pecasEsperadas, p.Texto)
		}
	}
	if len(pecasEsperadas) == 0 {
		pecasEsperadas = []string{questao.Hanzi}
	}
	questao.PecasEsperadas = pecasEsperadas

	var elementos []ElementoOrdenacao
	for _, op := range opcoes {
		elementos = append(elementos, ElementoOrdenacao{
			Texto:     op.Hanzi,
			Pinyin:    op.Pinyin,
			Definicao: op.Definicao,
			Correta:   op.Correta,
		})
	}
	rand.Shuffle(len(elementos), func(i, j int) {
		elementos[i], elementos[j] = elementos[j], elementos[i]
	})
	questao.ElementosOrdenacao = elementos

	var pilhaLegada []OpcaoRevisao
	for _, elem := range elementos {
		pilhaLegada = append(pilhaLegada, OpcaoRevisao{
			Hanzi:     elem.Texto,
			Pinyin:    elem.Pinyin,
			Definicao: elem.Definicao,
			Correta:   elem.Correta,
		})
	}
	questao.PilhaOrdenacao = pilhaLegada

	return nil
}

// limparFraseDaQuestao desfaz o preenchimento de frase de uma questão que acabou saindo noutra
// atividade — sem isso ela viajaria com uma frase que o usuário não chega a ver.
func limparFraseDaQuestao(questao *QuestaoRevisao) {
	questao.FraseLacuna = ""
	questao.FraseOriginal = ""
	questao.FraseOculta = ""
	questao.FraseLacunaSegmentada = nil
	questao.FraseOriginalSegmentada = nil
	questao.FraseOriginalLacunaSegmentada = nil
	questao.FraseTraducao = ""
	questao.FraseAtribuicao = ""
	questao.FraseTema = ""
	questao.FraseDificuldade = ""
}

// buscarOpcoesTraducaoContexto monta as alternativas da tradução por contexto (1 correta e 2
// distratoras): as frases distratoras vêm do módulo central, ranqueadas por similaridade (hanzis
// compartilhados com a frase da questão).
func (r *GerenciadorRevisao) buscarOpcoesTraducaoContexto(questao *QuestaoRevisao) error {
	if r.Frases.TotalFrases() < TotalOpcoesTraducaoContexto {
		return fmt.Errorf("não há frases suficientes no banco de dados para gerar distratores")
	}

	fontes := []fonteBusca[dicionario.Frase]{
		{Sequencia: sequenciaPreguicosa(func() []dicionario.Frase {
			return r.frasesDistratorasPorSimilaridade(questao.FraseOriginal, questao.FraseTraducao)
		})},
	}
	distratoras := buscarDeFontesSimples(TotalOpcoesTraducaoContexto-1, nil, fontes, nil)
	if len(distratoras) < TotalOpcoesTraducaoContexto-1 {
		return fmt.Errorf("não há frases candidatas suficientes para gerar os distratores")
	}

	tipoExibicao := r.Config().TipoHanziExibicao
	opcoes := []OpcaoRevisao{{
		Hanzi:     questao.FraseOriginal,
		Definicao: questao.FraseTraducao,
		Correta:   true,
	}}
	for _, distratora := range distratoras {
		texto := distratora.Chines
		if tipoExibicao == "simplificado" || tipoExibicao == "tradicional" {
			texto = r.Dicionario.ConverterTexto(texto, tipoExibicao)
		}
		opcoes = append(opcoes, OpcaoRevisao{
			Hanzi:     texto,
			Definicao: distratora.Ingles,
			Correta:   false,
		})
	}

	rand.Shuffle(len(opcoes), func(i, j int) { opcoes[i], opcoes[j] = opcoes[j], opcoes[i] })
	questao.Opcoes = opcoes

	return nil
}
