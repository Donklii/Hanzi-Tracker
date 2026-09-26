package revisao

import (
	"testing"
	"wails_app/config"
	"wails_app/dicionario"
	"wails_app/segmentacao"
)

// ----- Seção: Testes do Modo de Compreensão na Revisão -----

func TestMontarQuestaoCompreensao(t *testing.T) {
	_ = segmentacao.InitJieba()
	dic, _ := dicionario.NovoGerenciadorDicionario(dicionario.IdiomaPadrao)
	frases := dicionario.NovoGerenciadorFrases("pt-BR")
	cfg := func() config.Config {
		return config.Config{
			TipoHanziExibicao: "simplificado",
		}
	}

	rev := NovoGerenciadorRevisao(dic, frases, cfg)
	comp := dicionario.NovoGerenciadorCompreensao()
	comp.TotalPerguntas()   // força a carga inicial do arquivo real
	comp.LimparParaTestes() // limpa tudo, inclusive a base real recém-carregada

	comp.AdicionarPergunta(dicionario.PerguntaCompreensao{
		Contexto:              "今天天气很好，我们去公园吧。",
		Pergunta:              "说话人想做什么？",
		Opcoes:                []string{"去休息", "去公园", "去工作", "去买东西"},
		IndiceRespostaCorreta: 1,
		PerguntaTraduzida:     "O que a pessoa quer fazer?",
		ContextoTraduzido:     "O tempo está muito bom hoje, vamos ao parque.",
		Tema:                  "天气自然",
		Dificuldade:           "初级",
	})

	rev.DefinirGerenciadorCompreensao(comp)
	rev.buscador = nil // desliga o filtro linguístico rigoroso para o teste unitário

	alvo := alvoRevisao{
		entrada: dicionario.DecomposicaoHanzi{
			Caractere: "好",
			Pinyin:    []string{"hǎo"},
			Definicao: "bom; bem",
		},
		emEstudo: true,
		emFoco:   true,
	}

	questao := QuestaoRevisao{Modo: ModoContexto, Variante: VarianteCompreensao, Hanzi: alvo.entrada.Caractere}
	questao, err := rev.preencherQuestaoCompreensao(questao, alvo, poolsRevisao{})
	if err != nil {
		t.Fatalf("falha inesperada ao montar questão de compreensão: %v", err)
	}

	if questao.Modo != ModoContexto {
		t.Errorf("modo esperado %s, obtido %s", ModoContexto, questao.Modo)
	}

	if questao.PerguntaCompreensao != "说话人想做什么？" {
		t.Errorf("pergunta diferente da esperada: %s", questao.PerguntaCompreensao)
	}

	if len(questao.Opcoes) != 4 {
		t.Fatalf("esperava 4 opções de resposta, veio: %d", len(questao.Opcoes))
	}

	if !questao.Opcoes[1].Correta {
		t.Errorf("esperava que a opção índice 1 fosse a correta")
	}
}
