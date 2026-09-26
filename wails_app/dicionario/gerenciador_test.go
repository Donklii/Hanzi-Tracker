package dicionario

import (
	"strings"
	"testing"
)

// gerenciadorDeTeste carrega o gerenciador real (dados embarcados) para exercitar a prioridade e a
// decomposição ponta a ponta.
func gerenciadorDeTeste(t *testing.T) *GerenciadorDicionario {
	t.Helper()

	gerenciador, err := NovoGerenciadorDicionario(IdiomaPadrao)
	if err != nil {
		t.Fatalf("falha ao carregar o gerenciador de dicionário: %v", err)
	}
	return gerenciador
}

// ----- Busca de significado e leitura -----

// TestBuscarPalavraPoeAhDefinicaoDoTopoNaFrente trava a regra de prioridade: quem responde primeiro é
// o campo "definicao" do topo da entrada fundida, seja ele curado ou promovido do CEDICT — o banco
// não escolhe mais por fonte.
func TestBuscarPalavraPoeAhDefinicaoDoTopoNaFrente(t *testing.T) {
	gerenciador := gerenciadorDeTeste(t)

	entradas := gerenciador.BuscarPalavra("好")
	if len(entradas) == 0 {
		t.Fatal("esperava entradas para o caractere 好")
	}

	primeira := entradas[0]
	if primeira.Simplificado != "好" || primeira.Tradicional != "好" {
		t.Fatalf("esperava a acepção do topo na frente, veio %+v", primeira)
	}
	if primeira.Pinyin == "" || len(primeira.Significados) == 0 {
		t.Fatalf("acepção do topo sem leitura/significado: %+v", primeira)
	}
}


// TestBuscarPalavraCompostaVemDoCedict garante que palavra composta (que só o CEDICT conhece)
// continua respondendo, agora pela leitura promovida ao topo.
func TestBuscarPalavraCompostaVemDoCedict(t *testing.T) {
	gerenciador := gerenciadorDeTeste(t)

	entradas := gerenciador.BuscarPalavra("世界")
	if len(entradas) == 0 {
		t.Fatal("esperava entradas para a palavra composta 世界")
	}
	if entradas[0].Pinyin == "" || len(entradas[0].Significados) == 0 {
		t.Fatalf("palavra composta sem leitura/significado: %+v", entradas[0])
	}
}


// TestLeituraDevolveAhEntradaCasada: a entrada casada volta para QUALQUER palavra com acepção,
// inclusive caractere isolado — é ela que deixa o scan de OCR converter o card ao tipo configurado.
// Antes o caractere isolado vinha do makemeahanzi (sem grafia) e devolvia nil, então não convertia.
func TestLeituraDevolveAhEntradaCasada(t *testing.T) {
	gerenciador := gerenciadorDeTeste(t)

	for _, palavra := range []string{"好", "你好"} {
		pinyin, significados, entrada := gerenciador.Leitura(palavra)
		if pinyin == "" || len(significados) == 0 {
			t.Fatalf("leitura de %q vazia: pinyin=%q significados=%v", palavra, pinyin, significados)
		}
		if entrada == nil {
			t.Fatalf("leitura de %q devia devolver a entrada casada", palavra)
		}
		if entrada.Simplificado != palavra {
			t.Fatalf("entrada casada de %q veio de outra palavra: %+v", palavra, entrada)
		}
	}

	if pinyin, significados, entrada := gerenciador.Leitura("zzq"); pinyin != "" || significados != nil || entrada != nil {
		t.Fatalf("palavra inexistente devia devolver tudo vazio, veio %q %v %+v", pinyin, significados, entrada)
	}
}

// ----- Decomposição e caracteres compostos -----

func TestSegmentarPorDicionarioPreservaCompostos(t *testing.T) {
	gerenciador := gerenciadorDeTeste(t)

	tokens := gerenciador.SegmentarPorDicionario("你好世界")
	if juntos := strings.Join(tokens, ""); juntos != "你好世界" {
		t.Fatalf("segmentação perdeu/alterou caracteres: %q", juntos)
	}
	if len(tokens) < 2 {
		t.Fatalf("esperava a frase quebrada em ao menos duas palavras, veio %v", tokens)
	}
}


func TestDecomporHanziEhCompostosPor(t *testing.T) {
	gerenciador := gerenciadorDeTeste(t)

	if dec := gerenciador.DecomporHanzi("好"); dec == nil || dec.Caractere != "好" {
		t.Fatalf("esperava decomposição de 好, veio %+v", dec)
	}
	// Palavra composta não tem dado curado de caractere.
	if dec := gerenciador.DecomporHanzi("世界"); dec != nil {
		t.Fatalf("palavra composta não devia ter decomposição, veio %+v", dec)
	}

	// 口 é componente frequente: deve haver caracteres compostos por ele.
	if compostos := gerenciador.CompostosPor("口"); len(compostos) == 0 {
		t.Fatal("esperava caracteres compostos pelo componente 口")
	}
}

// ----- Utilitários de classificação -----

func TestTemEntrada(t *testing.T) {
	gerenciador := gerenciadorDeTeste(t)

	// Caractere isolado sempre passa (fallback universal), mesmo sem entrada no dicionário.
	if !gerenciador.TemEntrada("好") {
		t.Fatal("caractere isolado deveria ter entrada")
	}

	// String multi-caractere sem entrada não passa.
	if gerenciador.TemEntrada("zzq") {
		t.Fatal("string sem entrada no dicionário não deveria passar")
	}
}


// TestConverterTextoUsaOhParDeGrafias trava a conversão simplificado↔tradicional montada a partir
// da lista "tradicional" de cada entrada fundida.
func TestConverterTextoUsaOhParDeGrafias(t *testing.T) {
	gerenciador := gerenciadorDeTeste(t)

	if convertido := gerenciador.ConverterTexto("发", "tradicional"); convertido == "发" {
		t.Error("发 deveria ter uma forma tradicional (發/髮)")
	}
	if convertido := gerenciador.ConverterTexto("學", "simplificado"); convertido != "学" {
		t.Errorf("ConverterTexto(學→simplificado) = %q, esperado 学", convertido)
	}
	// Alvo desconhecido devolve o texto intacto.
	if convertido := gerenciador.ConverterTexto("学", "xx"); convertido != "学" {
		t.Errorf("alvo inválido devia devolver o texto intacto, veio %q", convertido)
	}
}


// TestConverterTextoNaoCorrompeParticulaMe trava a correção da colisão 么/幺 do CC-CEDICT: como o
// dicionário lista 么 como forma tradicional de 幺, a conversão char-a-char reescrevia 么→幺 e
// corrompia 什么→什幺, 怎么→怎幺, 那么→那幺... A partícula 么 já é simplificada e tem de sair intacta,
// sem estragar as conversões legítimas (發→发) nem a direção oposta (么→麼).
func TestConverterTextoNaoCorrompeParticulaMe(t *testing.T) {
	gerenciador := gerenciadorDeTeste(t)

	casos := []struct {
		texto, alvo, esperado string
	}{
		{"什么", "simplificado", "什么"},
		{"么", "simplificado", "么"},
		{"怎么", "simplificado", "怎么"},
		{"那么", "simplificado", "那么"},
		{"什么", "tradicional", "什麼"},
		{"么", "tradicional", "麼"},
		{"發", "simplificado", "发"},
	}
	for _, c := range casos {
		convertido := gerenciador.ConverterTexto(c.texto, c.alvo)
		if convertido != c.esperado {
			t.Errorf("ConverterTexto(%q, %q) = %q, esperado %q", c.texto, c.alvo, convertido, c.esperado)
		}
	}
}
