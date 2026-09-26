package revisao

// ----- Seção: Modo de Fonética -----

// montarQuestaoFonetica monta a questão de escuta/leitura fonética (opções, frase ou quebra-cabeça).
func (r *GerenciadorRevisao) montarQuestaoFonetica(questao QuestaoRevisao, alvo alvoRevisao, pools poolsRevisao, sessaoModo string, varianteForcada string) (QuestaoRevisao, error) {
	if varianteForcada != "" {
		questao.Variante = varianteForcada
	} else {
		opcoesVariantes := []string{VarianteAudioParaHanzi, VarianteHanziParaAudio, VarianteHanziParaPinyin, VarianteFoneticaFrase, VarianteFoneticaTraducao, VarianteFoneticaFilaPinyin, VarianteFoneticaPalavraPinyin}
		if len(pools.Todos) >= 4 {
			if !r.buscador.HistoricoContem(VarianteQuebraCabecaFonetica + ":" + alvo.entrada.Caractere) {
				opcoesVariantes = append(opcoesVariantes, VarianteQuebraCabecaFonetica)
			}
		}
		questao.Variante = r.sortearVariante(opcoesVariantes...)
	}

	if questao.Variante == VarianteQuebraCabecaFonetica {
		pecas := r.buscarPecasQuebraCabecaFonetica(alvo.entrada, pools)
		if len(pecas) >= 4 {
			questao.Opcoes = pecas
			return questao, nil
		}
		questao.Variante = r.sortearVariante(VarianteAudioParaHanzi, VarianteHanziParaAudio, VarianteHanziParaPinyin)
	} else if questao.Variante == VarianteFoneticaFrase {
		if !r.preencherFrase(&questao) {
			questao.Variante = r.sortearVariante(VarianteAudioParaHanzi, VarianteHanziParaAudio, VarianteHanziParaPinyin)
		} else if err := r.gerarPilhaOrdenacao(&questao, pools); err != nil {
			questao.Variante = r.sortearVariante(VarianteAudioParaHanzi, VarianteHanziParaAudio, VarianteHanziParaPinyin)
		}
	} else if questao.Variante == VarianteFoneticaTraducao {
		if !r.preencherFrase(&questao) {
			questao.Variante = r.sortearVariante(VarianteAudioParaHanzi, VarianteHanziParaAudio, VarianteHanziParaPinyin)
		} else if err := r.buscarOpcoesTraducaoContexto(&questao); err != nil {
			questao.Variante = r.sortearVariante(VarianteAudioParaHanzi, VarianteHanziParaAudio, VarianteHanziParaPinyin)
		}
	} else if questao.Variante == VarianteFoneticaFilaPinyin {
		if !r.preencherFrase(&questao) {
			questao.Variante = r.sortearVariante(VarianteAudioParaHanzi, VarianteHanziParaAudio, VarianteHanziParaPinyin)
		}
	}

	if questao.Variante != VarianteFoneticaFrase && questao.Variante != VarianteQuebraCabecaFonetica && questao.Variante != VarianteFoneticaTraducao && questao.Variante != VarianteFoneticaFilaPinyin && questao.Variante != VarianteFoneticaPalavraPinyin {
		questao.Opcoes = r.buscarOpcoes(alvo.entrada, alvo.emFoco, pools, distintosPorPinyin)
	}

	return questao, nil
}
