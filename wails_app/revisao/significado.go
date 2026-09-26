package revisao

import "wails_app/revisao/jornada"

// ----- Seção: Modo de Significado -----

// montarQuestaoSignificado monta a questão para o modo de significado (múltipla escolha ou
// quebra-cabeça). Toda a caça de alternativas é delegada ao módulo central de busca; aqui só se
// declaram a variante e o critério (glosas principais distintas).
func (r *GerenciadorRevisao) montarQuestaoSignificado(questao QuestaoRevisao, alvo alvoRevisao, pools poolsRevisao, sessaoModo string, varianteForcada string) (QuestaoRevisao, error) {
	if varianteForcada != "" {
		questao.Variante = varianteForcada
	} else {
		opcoesVariantes := []string{VarianteHanziParaSignificado, VarianteSignificadoParaHanzi, VarianteHanziFraseParaSignificado, VarianteSignificadoParaHanziConhecido}
		if temImagemParaHanzi(alvo.entrada.Caractere) {
			opcoesVariantes = append(opcoesVariantes, VarianteImagemParaSignificado, VarianteSignificadoParaImagem)
		}
		if len(pools.Todos) >= 4 {
			jaTeveQuebraCabeca := r.buscador.HistoricoContem(VarianteQuebraCabecaSignificado+":"+alvo.entrada.Caractere) ||
				r.buscador.HistoricoContem(VarianteQuebraCabecaTrio+":"+alvo.entrada.Caractere)
			if !jaTeveQuebraCabeca {
				opcoesVariantes = append(opcoesVariantes, VarianteQuebraCabecaSignificado)
				if sessaoModo == ModoGeral || sessaoModo == jornada.RevisaoMista {
					opcoesVariantes = append(opcoesVariantes, VarianteQuebraCabecaTrio)
				}
			}
		}
		// Atividades que a sessão empresta para a escada do significado (na Jornada, a lacuna da frase
		// como degrau avançado). Fora dessas sessões a lista é vazia.
		opcoesVariantes = append(opcoesVariantes, r.variantesExtras(ModoSignificado)...)

		questao.Variante = r.sortearVariante(opcoesVariantes...)
	}

	if (questao.Variante == VarianteImagemParaSignificado || questao.Variante == VarianteSignificadoParaImagem) && !temImagemParaHanzi(alvo.entrada.Caractere) {
		questao.Variante = r.sortearVariante(VarianteHanziFraseParaSignificado, VarianteHanziParaSignificado, VarianteSignificadoParaHanziConhecido, VarianteSignificadoParaHanzi)
	}

	if questao.Variante == VarianteContexto {
		if err := r.preencherQuestaoLacuna(&questao, alvo, pools); err == nil && len(questao.Opcoes) == TotalOpcoesMultiplaEscolha {
			return questao, nil
		}
		// Palavra sem frase no acervo (ou sem distratores fora dela): a questão sai numa alternativa
		// simples, como nos demais fallbacks. A frase meio montada tem de sair junto — senão a questão
		// seguiria creditando visualizações de uma frase que o usuário não vai ver.
		limparFraseDaQuestao(&questao)
		questao.Variante = r.sortearVariante(VarianteHanziFraseParaSignificado, VarianteHanziParaSignificado, VarianteSignificadoParaHanziConhecido, VarianteSignificadoParaHanzi)
	}

	if questao.Variante == VarianteQuebraCabecaSignificado {
		pecas := r.buscarPecasQuebraCabeca(alvo.entrada, pools)
		if len(pecas) >= 4 {
			questao.Opcoes = pecas
			return questao, nil
		}
		questao.Variante = r.sortearVariante(VarianteHanziFraseParaSignificado, VarianteHanziParaSignificado, VarianteSignificadoParaHanziConhecido, VarianteSignificadoParaHanzi)
	} else if questao.Variante == VarianteQuebraCabecaTrio {
		pecas := r.buscarPecasQuebraCabecaTrio(alvo.entrada, pools)
		if len(pecas) >= 4 {
			questao.Opcoes = pecas
			return questao, nil
		}
		questao.Variante = r.sortearVariante(VarianteHanziFraseParaSignificado, VarianteHanziParaSignificado, VarianteSignificadoParaHanziConhecido, VarianteSignificadoParaHanzi)
	}

	if questao.Variante == VarianteSignificadoParaImagem {
		opcoesComImagem := r.buscarOpcoes(alvo.entrada, alvo.emFoco, pools, distintosPorSignificadoEComImagem)
		if len(opcoesComImagem) == TotalOpcoesMultiplaEscolha {
			questao.Opcoes = opcoesComImagem
			return questao, nil
		}
		// Se não houver distratores com imagem suficientes, reverte para alternativa de texto simples ou frase
		questao.Variante = r.sortearVariante(VarianteHanziFraseParaSignificado, VarianteSignificadoParaHanziConhecido, VarianteSignificadoParaHanzi, VarianteHanziParaSignificado)
	}

	if questao.Variante == VarianteHanziFraseParaSignificado {
		// Busca distratores priorizando palavras já conhecidas pelo usuário
		opcoesConhecidas := r.buscarOpcoes(alvo.entrada, alvo.emFoco, pools, r.distintosPorSignificadoConhecidos())
		if len(opcoesConhecidas) == TotalOpcoesMultiplaEscolha {
			questao.Opcoes = opcoesConhecidas
			return questao, nil
		}

		// Fallback para distratores gerais se o usuário ainda não tiver 3 palavras conhecidas
		questao.Opcoes = r.buscarOpcoes(alvo.entrada, alvo.emFoco, pools, distintosPorSignificado)
		return questao, nil
	}

	if questao.Variante == VarianteSignificadoParaHanziConhecido {
		opcoesConhecidas := r.buscarOpcoes(alvo.entrada, alvo.emFoco, pools, r.distintosPorHanziConhecidos())
		if len(opcoesConhecidas) == TotalOpcoesMultiplaEscolha {
			questao.Opcoes = opcoesConhecidas
			return questao, nil
		}
		// Fallback para distratores gerais se o usuário ainda não tiver 3 palavras conhecidas
		questao.Opcoes = r.buscarOpcoes(alvo.entrada, alvo.emFoco, pools, distintosPorHanzi)
		return questao, nil
	}

	questao.Opcoes = r.buscarOpcoes(alvo.entrada, alvo.emFoco, pools, distintosPorHanzi)
	return questao, nil
}
