package dicionario

import (
	"strings"
	"testing"
)

// ----- Carga e consultas do acervo de frases -----

func TestGerenciadorFrasesCarregaAcervo(t *testing.T) {
	gerenciador := NovoGerenciadorFrases(IdiomaPadrao)

	total := gerenciador.TotalFrases()
	if total == 0 {
		t.Fatal("esperava frases carregadas do diretório frases/")
	}

	if len(gerenciador.ObterTodasFrases()) != total {
		t.Fatalf("ObterTodasFrases (%d) diverge de TotalFrases (%d)", len(gerenciador.ObterTodasFrases()), total)
	}
}

func TestFrasesComCaractereContemOAlvo(t *testing.T) {
	gerenciador := NovoGerenciadorFrases(IdiomaPadrao)

	// 我 ("eu") é frequente o bastante para aparecer no acervo do Tatoeba.
	const alvo = '我'
	if !gerenciador.TemCaractere(alvo) {
		t.Fatalf("esperava ao menos uma frase com o caractere %q", string(alvo))
	}

	frases := gerenciador.FrasesComCaractere(alvo)
	if len(frases) == 0 {
		t.Fatalf("TemCaractere disse sim, mas FrasesComCaractere veio vazio para %q", string(alvo))
	}

	for _, f := range frases {
		if !strings.ContainsRune(f.Chines, alvo) {
			t.Fatalf("frase indexada não contém o alvo %q: %q", string(alvo), f.Chines)
		}
	}
}

func TestCaractereInexistenteNaoTemFrase(t *testing.T) {
	gerenciador := NovoGerenciadorFrases(IdiomaPadrao)

	// Símbolo especial '§' não aparece no lado chinês das frases.
	const alvo = '§'
	if gerenciador.TemCaractere(alvo) {
		t.Fatalf("não deveria haver frase indexada pelo caractere %q", string(alvo))
	}
	if frases := gerenciador.FrasesComCaractere(alvo); len(frases) != 0 {
		t.Fatalf("esperava nenhuma frase para %q, veio %d", string(alvo), len(frases))
	}
}

// TestFrasesCarregamTemaEDificuldade garante que a classificação fundida nos arquivos (colunas 4 e 5)
// chega aos campos Tema/Dificuldade da Frase, que os dois rótulos andam juntos (nunca só um) e que a
// dificuldade fica restrita ao conjunto fechado esperado.
func TestFrasesCarregamTemaEDificuldade(t *testing.T) {
	gerenciador := NovoGerenciadorFrases(IdiomaPadrao)

	dificuldadesValidas := map[string]bool{"入门": true, "初级": true, "中级": true, "高级": true}

	comRotulo := 0
	for _, f := range gerenciador.ObterTodasFrases() {
		if f.Tema == "" && f.Dificuldade == "" {
			continue // frase sem classificação (barrada na moderação, ou sem par) — permitido
		}
		comRotulo++
		if f.Tema == "" || f.Dificuldade == "" {
			t.Fatalf("frase %q com rótulo pela metade: tema=%q dificuldade=%q", f.Chines, f.Tema, f.Dificuldade)
		}
		if !dificuldadesValidas[f.Dificuldade] {
			t.Fatalf("frase %q com dificuldade fora do conjunto esperado: %q", f.Chines, f.Dificuldade)
		}
	}

	if comRotulo == 0 {
		t.Fatal("esperava frases com tema/dificuldade fundidos, nenhuma veio rotulada")
	}
}

// ----- Filtragem e seleção (frases construídas, determinístico) -----

func TestFiltrarPorTamanho(t *testing.T) {
	gerenciador := NovoGerenciadorFrases(IdiomaPadrao)

	curta := Frase{Chines: "你好"}
	longa := Frase{Chines: strings.Repeat("我", 25)}
	frases := []Frase{curta, longa}

	filtradas := gerenciador.FiltrarPorTamanho(frases, 20)
	if len(filtradas) != 1 || filtradas[0].Chines != curta.Chines {
		t.Fatalf("esperava só a frase curta, veio %+v", filtradas)
	}
}

func TestFiltrarPorMaxPalavras(t *testing.T) {
	gerenciador := NovoGerenciadorFrases(IdiomaPadrao)

	curta := Frase{Chines: "你好世界"} // 2 palavras
	longa := Frase{Chines: "你好世界！我很喜欢学习中文。"} // muitas palavras

	filtradas := gerenciador.FiltrarPorMaxPalavras([]Frase{curta, longa}, 4, nil)
	if len(filtradas) != 1 || filtradas[0].Chines != curta.Chines {
		t.Fatalf("esperava só a frase com até 4 palavras, veio %+v", filtradas)
	}
}

func TestFiltrarTodosConhecidos(t *testing.T) {
	gerenciador := NovoGerenciadorFrases(IdiomaPadrao)

	status := map[string]string{"你": StatusEstudo, "好": StatusAprendido}
	todosConhecidos := Frase{Chines: "你好"}
	comDesconhecido := Frase{Chines: "你好吗"} // 吗 não está no status
	semHan := Frase{Chines: "abc"}

	filtradas := gerenciador.FiltrarTodosConhecidos([]Frase{todosConhecidos, comDesconhecido, semHan}, status)
	if len(filtradas) != 1 || filtradas[0].Chines != todosConhecidos.Chines {
		t.Fatalf("esperava só a frase com todos conhecidos, veio %+v", filtradas)
	}
}

func TestFiltrarPorCobertura(t *testing.T) {
	gerenciador := NovoGerenciadorFrases(IdiomaPadrao)

	// 你 conhecido, 好 desconhecido -> cobertura 0.5.
	status := map[string]string{"你": StatusEstudo}
	frases := []Frase{{Chines: "你好"}}

	if metade := gerenciador.FiltrarPorCobertura(frases, status, CoberturaMedia); len(metade) != 1 {
		t.Fatalf("cobertura 0.5 deveria passar no piso médio (0.5), veio %d", len(metade))
	}
	if alta := gerenciador.FiltrarPorCobertura(frases, status, CoberturaAlta); len(alta) != 0 {
		t.Fatalf("cobertura 0.5 não deveria passar no piso alto (0.9), veio %d", len(alta))
	}
}

// TestFiltrarPreservandoAlvoNaConversaoDescartaAlvoQueDesaparece cobre o alvo que estuda uma grafia
// (o tradicional 發) enquanto a exibição força a outra (simplificado): a frase vira 出发 e o alvo 發
// some do texto convertido, então deve ser descartada.
func TestFiltrarPreservandoAlvoNaConversaoDescartaAlvoQueDesaparece(t *testing.T) {
	gerenciadorFrases := NovoGerenciadorFrases(IdiomaPadrao)
	gerenciadorDicionario := gerenciadorDeTeste(t)

	// Pré-condição do teste: confirma que a conversão realmente engole o alvo (documenta o porquê).
	convertida := gerenciadorDicionario.ConverterTexto("發", "simplificado")
	if strings.Contains(convertida, "發") {
		t.Fatalf("pré-condição do teste falhou: esperava que a conversão removesse 發, resultado: %q", convertida)
	}

	frases := []Frase{{Chines: "他明天出發。"}}
	filtradas := gerenciadorFrases.FiltrarPreservandoAlvoNaConversao(frases, "發", gerenciadorDicionario, "simplificado")
	if len(filtradas) != 0 {
		t.Fatalf("esperava a frase descartada (alvo desaparece na conversão), veio %+v", filtradas)
	}
}

// TestFiltrarPreservandoAlvoNaConversaoMantemAlvoEstavel garante o caso normal: um caractere que a
// conversão não altera (ex.: 好, idêntico em simplificado e tradicional) mantém a frase na lista.
func TestFiltrarPreservandoAlvoNaConversaoMantemAlvoEstavel(t *testing.T) {
	gerenciadorFrases := NovoGerenciadorFrases(IdiomaPadrao)
	gerenciadorDicionario := gerenciadorDeTeste(t)

	frases := []Frase{{Chines: "你好吗？"}}
	filtradas := gerenciadorFrases.FiltrarPreservandoAlvoNaConversao(frases, "好", gerenciadorDicionario, "simplificado")
	if len(filtradas) != 1 {
		t.Fatalf("esperava a frase mantida (alvo estável na conversão), veio %+v", filtradas)
	}
}

// TestFiltrarPreservandoAlvoNaConversaoNoOpQuandoAmbos garante que o filtro não mexe nas frases
// quando a exibição não força um tipo específico (nada é convertido, então nada pode ser "engolido").
func TestFiltrarPreservandoAlvoNaConversaoNoOpQuandoAmbos(t *testing.T) {
	gerenciadorFrases := NovoGerenciadorFrases(IdiomaPadrao)
	gerenciadorDicionario := gerenciadorDeTeste(t)

	frases := []Frase{{Chines: "他明天出發。"}}
	filtradas := gerenciadorFrases.FiltrarPreservandoAlvoNaConversao(frases, "發", gerenciadorDicionario, "ambos")
	if len(filtradas) != len(frases) {
		t.Fatalf("esperava no-op para tipoAlvo=ambos, veio %+v", filtradas)
	}
}

// TestAdicionarFrase cobre a injeção de uma frase em runtime (geração com IA salva pelo usuário):
// ela entra no acervo, fica consultável, e re-adicionar a mesma não duplica.
func TestAdicionarFrase(t *testing.T) {
	gerenciador := NovoGerenciadorFrases(IdiomaPadrao)

	total0 := gerenciador.TotalFrases()
	const sentenca = "ZQX獨門測試句子甲乙丙。" // sintética: 'Q' latino não ocorre no acervo chinês

	fraseTeste := Frase{
		Chines:      sentenca,
		Ingles:      "synthetic test sentence",
		Atribuicao:  "teste",
		Tema:        "日常生活",
		Dificuldade: "入门",
	}

	if !gerenciador.AdicionarFrase(fraseTeste) {
		t.Fatal("esperava que a frase sintética fosse inédita (inserida)")
	}
	if got := gerenciador.TotalFrases(); got != total0+1 {
		t.Fatalf("esperava total %d após inserir, veio %d", total0+1, got)
	}

	encontrada := false
	for _, f := range gerenciador.FrasesComCaractere('Q') {
		if f.Chines == sentenca {
			encontrada = true
			if f.Ingles != "synthetic test sentence" || f.Atribuicao != "teste" || f.Tema != "日常生活" || f.Dificuldade != "入门" {
				t.Errorf("campos da frase adicionada divergiram: %+v", f)
			}
		}
	}
	if !encontrada {
		t.Fatal("frase adicionada não apareceu na consulta por caractere")
	}

	if gerenciador.AdicionarFrase(Frase{Chines: sentenca, Ingles: "outra", Atribuicao: "outra"}) {
		t.Error("re-adicionar a mesma frase deveria devolver false (dedup)")
	}
	if got := gerenciador.TotalFrases(); got != total0+1 {
		t.Fatalf("dedup falhou: total mudou para %d", got)
	}
}

func TestSelecionarPonderadaCasosTriviais(t *testing.T) {
	gerenciador := NovoGerenciadorFrases(IdiomaPadrao)

	unica := Frase{Chines: "你好"}
	if escolhida := gerenciador.SelecionarPonderada([]Frase{unica}, nil); escolhida.Chines != unica.Chines {
		t.Fatalf("com uma única frase deveria devolvê-la, veio %+v", escolhida)
	}

	// Sem status (mapa vazio): ainda deve devolver uma das frases fornecidas.
	frases := []Frase{{Chines: "你好"}, {Chines: "谢谢"}}
	escolhida := gerenciador.SelecionarPonderada(frases, map[string]string{})
	if escolhida.Chines != "你好" && escolhida.Chines != "谢谢" {
		t.Fatalf("escolha fora do conjunto fornecido: %+v", escolhida)
	}
}
