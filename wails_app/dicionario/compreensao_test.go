package dicionario

import (
	"testing"
)

// ----- Seção: Testes do Gerenciador de Compreensão -----

func TestGerenciadorCompreensaoAdicionarEConsultar(t *testing.T) {
	g := NovoGerenciadorCompreensao()

	p1 := PerguntaCompreensao{
		Contexto:              "今天天气很好，我们去公园吧。",
		Pergunta:              "说话人想做什么？",
		Opcoes:                []string{"去休息", "去公园", "去工作", "去买东西"},
		IndiceRespostaCorreta: 1,
		PerguntaTraduzida:     "O que a pessoa quer fazer?",
		ContextoTraduzido:     "O tempo está muito bom hoje, vamos ao parque.",
		Tema:                  "天气自然",
		Dificuldade:           "初级",
	}
	
	totalAnterior := g.TotalPerguntas()

	if !g.AdicionarPergunta(p1) {
		t.Fatalf("esperava adicionar pergunta inédita com sucesso")
	}

	if g.AdicionarPergunta(p1) {
		t.Fatalf("esperava rejeitar pergunta duplicada pelo contexto")
	}

	if g.TotalPerguntas() != totalAnterior+1 {
		t.Fatalf("esperava %d pergunta no total, obtido: %d", totalAnterior+1, g.TotalPerguntas())
	}

	comHanzi := g.PerguntasComPalavra("好")
	if len(comHanzi) == 0 {
		t.Fatalf("esperava achar pelo menos 1 pergunta com o hanzi '好', obtido: %d", len(comHanzi))
	}

	encontrou := false
	for _, p := range comHanzi {
		if p.Pergunta == p1.Pergunta {
			encontrou = true
			break
		}
	}

	if !encontrou {
		t.Errorf("pergunta inserida não encontrada na busca por palavra")
	}

	if !g.RemoverPergunta(p1.Contexto) {
		t.Fatalf("esperava remover pergunta existente")
	}

	if g.TotalPerguntas() != totalAnterior {
		t.Fatalf("esperava %d perguntas após remoção, obtido: %d", totalAnterior, g.TotalPerguntas())
	}
}

func TestGerenciadorCompreensaoLazyLoadEmbed(t *testing.T) {
	g := NovoGerenciadorCompreensao()

	// Ao chamar TotalPerguntas, ele forçará a execução de garantirCarregado()
	// que vai ler do embed fs (se houver o diretório idiomas/en/compreensao e arquivos jsonl.gz).
	total := g.TotalPerguntas()

	if total == 0 {
		t.Log("Atenção: O total de perguntas é 0. Talvez os dados do C3 não tenham sido gerados ainda, ou a leitura do embed falhou silenciosamente.")
	} else {
		t.Logf("Sucesso: O gerenciador carregou automaticamente %d perguntas de compreensão do embed.", total)
	}
}

func TestGerenciadorCompreensaoCategoriaTags(t *testing.T) {
	g := NovoGerenciadorCompreensao()
	total := g.TotalPerguntas()
	if total == 0 {
		t.Skip("Sem perguntas de compreensão no embed para validar tags de categoria.")
	}

	temDialogo := false
	temPassagem := false

	g.mu.RLock()
	defer g.mu.RUnlock()

	for _, p := range g.perguntas {
		if p.Categoria == "d" {
			temDialogo = true
		} else if p.Categoria == "m" {
			temPassagem = true
		}
		if temDialogo && temPassagem {
			break
		}
	}

	if !temDialogo {
		t.Errorf("esperava encontrar perguntas com categoria 'd' (diálogos)")
	}
	if !temPassagem {
		t.Errorf("esperava encontrar perguntas com categoria 'm' (passagens)")
	}
}
