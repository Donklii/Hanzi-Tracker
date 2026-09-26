package notasversao

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"wails_app/dicionario"
)

// ----- Testes do Analisador de Notas (Parser C2) -----

func TestAnalisarNotaValida(t *testing.T) {
	conteudo := "\xef\xbb\xbf# Título Principal\r\n\r\n- Item antes de grupo\r\n  continuação do item antes de grupo\r\n\r\n## Novidades\r\n- Primeira novidade\r\n\tcontinuação com tab\r\n- Segunda novidade\r\n\r\n## Correções\r\n- Correção importante\r\n"

	titulo, grupos, err := analisarNota("2026-09-11-teste.md", []byte(conteudo))
	if err != nil {
		t.Fatalf("analisarNota falhou inesperadamente: %v", err)
	}

	if titulo != "Título Principal" {
		t.Errorf("título esperado 'Título Principal', obtido '%s'", titulo)
	}

	if len(grupos) != 3 {
		t.Fatalf("esperado 3 grupos, obtido %d", len(grupos))
	}

	if grupos[0].Titulo != "" || len(grupos[0].Itens) != 1 {
		t.Errorf("grupo sem subtítulo inválido: %+v", grupos[0])
	}
	if grupos[0].Itens[0] != "Item antes de grupo continuação do item antes de grupo" {
		t.Errorf("continuação não agregada corretamente: %s", grupos[0].Itens[0])
	}

	if grupos[1].Titulo != "Novidades" || len(grupos[1].Itens) != 2 {
		t.Errorf("grupo Novidades inválido: %+v", grupos[1])
	}
	if grupos[1].Itens[0] != "Primeira novidade continuação com tab" {
		t.Errorf("continuação com tab não agregada corretamente: %s", grupos[1].Itens[0])
	}

	if grupos[2].Titulo != "Correções" || len(grupos[2].Itens) != 1 {
		t.Errorf("grupo Correções inválido: %+v", grupos[2])
	}
}


func TestAnalisarNotaErros(t *testing.T) {
	casos := []struct {
		nome           string
		conteudo       string
		trechoEsperado string
	}{
		{
			nome:           "presenca de negrito",
			conteudo:       "# Título\n\n## Grupo\n- Item com **negrito** proibido",
			trechoEsperado: "formatação não permitida",
		},
		{
			nome:           "presenca de crase",
			conteudo:       "# Título\n\n## Grupo\n- Item com `crase` proibida",
			trechoEsperado: "formatação não permitida",
		},
		{
			nome:           "arquivo vazio",
			conteudo:       "   \n\r\n  ",
			trechoEsperado: "arquivo vazio ou sem título",
		},
		{
			nome:           "primeira linha nao e titulo",
			conteudo:       "## Grupo sem título\n- Item",
			trechoEsperado: "primeira linha com conteúdo deve ser '# <título>'",
		},
		{
			nome:           "titulo vazio",
			conteudo:       "# \n\n## Grupo\n- Item",
			trechoEsperado: "título não pode ser vazio",
		},
		{
			nome:           "titulo duplicado",
			conteudo:       "# Título 1\n\n- Item 1\n\n# Título 2\n- Item 2",
			trechoEsperado: "apenas um título principal",
		},
		{
			nome:           "grupo sem subtitulo",
			conteudo:       "# Título\n\n## \n- Item",
			trechoEsperado: "subtítulo do grupo não pode ser vazio",
		},
		{
			nome:           "grupo com prefixo errado",
			conteudo:       "# Título\n\n##SemEspaco\n- Item",
			trechoEsperado: "grupo deve iniciar com '## '",
		},
		{
			nome:           "grupo sem itens antes do proximo",
			conteudo:       "# Título\n\n## Grupo 1\n\n## Grupo 2\n- Item",
			trechoEsperado: "grupo anterior sem itens",
		},
		{
			nome:           "grupo sem itens no final",
			conteudo:       "# Título\n\n## Grupo 1\n- Item\n\n## Grupo 2",
			trechoEsperado: "grupo 'Grupo 2' sem itens",
		},
		{
			nome:           "item vazio",
			conteudo:       "# Título\n\n## Grupo\n- ",
			trechoEsperado: "item não pode ser vazio",
		},
		{
			nome:           "continuacao logo apos titulo",
			conteudo:       "# Título\n  continuação órfã",
			trechoEsperado: "linha de continuação sem item anterior",
		},
		{
			nome:           "continuacao logo apos grupo",
			conteudo:       "# Título\n\n## Grupo\n  continuação órfã",
			trechoEsperado: "linha de continuação sem item anterior",
		},
		{
			nome:           "linha invalida de texto solto",
			conteudo:       "# Título\n\nTexto solto sem marcador de item",
			trechoEsperado: "linha inválida",
		},
		{
			nome:           "nota sem nenhum item",
			conteudo:       "# Título Apenas",
			trechoEsperado: "nota sem itens",
		},
	}

	for _, caso := range casos {
		t.Run(caso.nome, func(t *testing.T) {
			_, _, err := analisarNota("arquivo_teste.md", []byte(caso.conteudo))
			if err == nil {
				t.Fatalf("esperava erro contendo '%s', mas obteve nil", caso.trechoEsperado)
			}

			if !strings.Contains(err.Error(), caso.trechoEsperado) {
				t.Errorf("erro '%s' não contém trecho esperado '%s'", err.Error(), caso.trechoEsperado)
			}
		})
	}
}


// ----- Testes do Sistema de Arquivos e Contrato C1 -----

func TestCarregarNotasFS_C1(t *testing.T) {
	t.Run("nome de arquivo fora do padrao", func(t *testing.T) {
		fsMap := fstest.MapFS{
			"invalido.md": &fstest.MapFile{Data: []byte("# T\n- I")},
		}

		_, err := carregarNotasFS(fsMap, "pt-BR")
		if err == nil || !strings.Contains(err.Error(), "fora do padrão") {
			t.Fatalf("esperava erro de nome fora do padrão, obtido: %v", err)
		}
	})

	t.Run("data invalida no nome", func(t *testing.T) {
		fsMap := fstest.MapFS{
			"2026-02-31-teste.md": &fstest.MapFile{Data: []byte("# T\n- I")},
		}

		_, err := carregarNotasFS(fsMap, "pt-BR")
		if err == nil || !strings.Contains(err.Error(), "data inválida") {
			t.Fatalf("esperava erro de data inválida, obtido: %v", err)
		}
	})

	t.Run("subpasta nao permitida", func(t *testing.T) {
		fsMap := fstest.MapFS{
			"subpasta/2026-09-11-teste.md": &fstest.MapFile{Data: []byte("# T\n- I")},
		}

		_, err := carregarNotasFS(fsMap, "pt-BR")
		if err == nil || !strings.Contains(err.Error(), "subpasta não permitida") {
			t.Fatalf("esperava erro de subpasta, obtido: %v", err)
		}
	})

	t.Run("traducao com idioma nao suportado", func(t *testing.T) {
		fsMap := fstest.MapFS{
			"2026-09-11-teste.md":    &fstest.MapFile{Data: []byte("# T\n- I")},
			"2026-09-11-teste.fr.md": &fstest.MapFile{Data: []byte("# T\n- I")},
		}

		_, err := carregarNotasFS(fsMap, "pt-BR")
		if err == nil || !strings.Contains(err.Error(), "idioma de tradução inválido") {
			t.Fatalf("esperava erro de idioma inválido, obtido: %v", err)
		}
	})

	t.Run("traducao redundante com pt-BR", func(t *testing.T) {
		fsMap := fstest.MapFS{
			"2026-09-11-teste.md":       &fstest.MapFile{Data: []byte("# T\n- I")},
			"2026-09-11-teste.pt-BR.md": &fstest.MapFile{Data: []byte("# T\n- I")},
		}

		_, err := carregarNotasFS(fsMap, "pt-BR")
		if err == nil || !strings.Contains(err.Error(), "idioma de tradução inválido") {
			t.Fatalf("esperava erro de sufixo pt-BR não permitido, obtido: %v", err)
		}
	})

	t.Run("traducao sem arquivo base pt-BR", func(t *testing.T) {
		fsMap := fstest.MapFS{
			"2026-09-11-teste.en.md": &fstest.MapFile{Data: []byte("# T\n- I")},
		}

		_, err := carregarNotasFS(fsMap, "en")
		if err == nil || !strings.Contains(err.Error(), "sem o arquivo pt-BR correspondente") {
			t.Fatalf("esperava erro de tradução sem base, obtido: %v", err)
		}
	})
}


func TestCarregarNotasFS_FallbackEOrdem(t *testing.T) {
	fsMap := fstest.MapFS{
		"2026-09-10-antiga.md": &fstest.MapFile{
			Data: []byte("# Antiga PT\n- Item antiga"),
		},
		"2026-09-11-nova-b.md": &fstest.MapFile{
			Data: []byte("# Nova B PT\n- Item B"),
		},
		"2026-09-11-nova-b.en.md": &fstest.MapFile{
			Data: []byte("# Nova B EN\n- Item B EN"),
		},
		"2026-09-11-nova-a.md": &fstest.MapFile{
			Data: []byte("# Nova A PT\n- Item A"),
		},
	}

	t.Run("ordenacao por data decrescente e empate por id decrescente", func(t *testing.T) {
		notas, err := carregarNotasFS(fsMap, "pt-BR")
		if err != nil {
			t.Fatalf("carregarNotasFS falhou: %v", err)
		}

		if len(notas) != 3 {
			t.Fatalf("esperado 3 notas, obtido %d", len(notas))
		}

		// Ordem esperada:
		// 1º: 2026-09-11-nova-b (mesma data que nova-a, mas 'nova-b' > 'nova-a')
		// 2º: 2026-09-11-nova-a
		// 3º: 2026-09-10-antiga
		if notas[0].Id != "2026-09-11-nova-b" {
			t.Errorf("1º esperado 2026-09-11-nova-b, obtido %s", notas[0].Id)
		}
		if notas[1].Id != "2026-09-11-nova-a" {
			t.Errorf("2º esperado 2026-09-11-nova-a, obtido %s", notas[1].Id)
		}
		if notas[2].Id != "2026-09-10-antiga" {
			t.Errorf("3º esperado 2026-09-10-antiga, obtido %s", notas[2].Id)
		}
	})

	t.Run("traducao e fallback", func(t *testing.T) {
		notasEN, err := carregarNotasFS(fsMap, "en")
		if err != nil {
			t.Fatalf("carregarNotasFS en falhou: %v", err)
		}

		// nova-b tem EN, nova-a não tem (deve usar fallback PT), antiga não tem (fallback PT)
		for _, n := range notasEN {
			if n.Id == "2026-09-11-nova-b" && n.Titulo != "Nova B EN" {
				t.Errorf("esperava título traduzido 'Nova B EN', obtido '%s'", n.Titulo)
			}
			if n.Id == "2026-09-11-nova-a" && n.Titulo != "Nova A PT" {
				t.Errorf("esperava fallback 'Nova A PT', obtido '%s'", n.Titulo)
			}
			if n.Id == "2026-09-10-antiga" && n.Titulo != "Antiga PT" {
				t.Errorf("esperava fallback 'Antiga PT', obtido '%s'", n.Titulo)
			}
		}

		// Ao solicitar es (sem nenhuma tradução es no fs), tudo deve cair em pt-BR
		notasES, err := carregarNotasFS(fsMap, "es")
		if err != nil {
			t.Fatalf("carregarNotasFS es falhou: %v", err)
		}
		for _, n := range notasES {
			if n.Id == "2026-09-11-nova-b" && n.Titulo != "Nova B PT" {
				t.Errorf("esperava fallback 'Nova B PT', obtido '%s'", n.Titulo)
			}
		}
	})
}


// ----- Testes do Estado C3 ("Já vista") -----

func TestEstadoVistas_C3(t *testing.T) {
	pastaTemp := t.TempDir()
	caminhoEstado := filepath.Join(pastaTemp, "notas_vistas.json")

	fsMap := fstest.MapFS{
		"2026-09-10-v1.md": &fstest.MapFile{Data: []byte("# V1\n- Item 1")},
		"2026-09-11-v2.md": &fstest.MapFile{Data: []byte("# V2\n- Item 2")},
	}

	t.Run("primeira execucao marca todas as atuais e retorna lista vazia", func(t *testing.T) {
		naoVistas, err := naoVistasNoCaminho(fsMap, caminhoEstado, "pt-BR")
		if err != nil {
			t.Fatalf("naoVistasNoCaminho falhou: %v", err)
		}

		if len(naoVistas) != 0 {
			t.Fatalf("esperava lista vazia na primeira execução, obtido %d", len(naoVistas))
		}

		// Arquivo deve ter sido criado em disco
		estado, err := carregarEstadoVistas(caminhoEstado)
		if err != nil {
			t.Fatalf("carregarEstadoVistas falhou: %v", err)
		}

		if len(estado.Vistas) != 2 {
			t.Fatalf("esperava 2 notas salvas no estado, obtido %d", len(estado.Vistas))
		}
	})

	t.Run("nota nova no sistema de arquivos aparece em naoVistas", func(t *testing.T) {
		fsMapAtualizado := fstest.MapFS{
			"2026-09-10-v1.md": &fstest.MapFile{Data: []byte("# V1\n- Item 1")},
			"2026-09-11-v2.md": &fstest.MapFile{Data: []byte("# V2\n- Item 2")},
			"2026-09-12-v3.md": &fstest.MapFile{Data: []byte("# V3\n- Item 3")},
		}

		naoVistas, err := naoVistasNoCaminho(fsMapAtualizado, caminhoEstado, "pt-BR")
		if err != nil {
			t.Fatalf("naoVistasNoCaminho falhou: %v", err)
		}

		if len(naoVistas) != 1 {
			t.Fatalf("esperava 1 nota não vista, obtido %d", len(naoVistas))
		}

		if naoVistas[0].Id != "2026-09-12-v3" {
			t.Errorf("esperava id '2026-09-12-v3', obtido '%s'", naoVistas[0].Id)
		}
	})

	t.Run("marcarVistas faz a nota nova sumir de naoVistas", func(t *testing.T) {
		fsMapAtualizado := fstest.MapFS{
			"2026-09-10-v1.md": &fstest.MapFile{Data: []byte("# V1\n- Item 1")},
			"2026-09-11-v2.md": &fstest.MapFile{Data: []byte("# V2\n- Item 2")},
			"2026-09-12-v3.md": &fstest.MapFile{Data: []byte("# V3\n- Item 3")},
		}

		err := marcarVistasNoCaminho(caminhoEstado, []string{"2026-09-12-v3"})
		if err != nil {
			t.Fatalf("marcarVistasNoCaminho falhou: %v", err)
		}

		naoVistas, err := naoVistasNoCaminho(fsMapAtualizado, caminhoEstado, "pt-BR")
		if err != nil {
			t.Fatalf("naoVistasNoCaminho falhou: %v", err)
		}

		if len(naoVistas) != 0 {
			t.Fatalf("esperava 0 notas não vistas após marcação, obtido %d", len(naoVistas))
		}
	})

	t.Run("arquivo corrompido trata como primeira execucao", func(t *testing.T) {
		if err := os.WriteFile(caminhoEstado, []byte("{ json corrompido "), 0600); err != nil {
			t.Fatalf("falha ao corromper arquivo: %v", err)
		}

		fsMapAtualizado := fstest.MapFS{
			"2026-09-10-v1.md": &fstest.MapFile{Data: []byte("# V1\n- Item 1")},
			"2026-09-11-v2.md": &fstest.MapFile{Data: []byte("# V2\n- Item 2")},
		}

		naoVistas, err := naoVistasNoCaminho(fsMapAtualizado, caminhoEstado, "pt-BR")
		if err != nil {
			t.Fatalf("naoVistasNoCaminho falhou: %v", err)
		}

		if len(naoVistas) != 0 {
			t.Fatalf("esperava 0 notas não vistas no reset por corrupção, obtido %d", len(naoVistas))
		}

		estado, err := carregarEstadoVistas(caminhoEstado)
		if err != nil {
			t.Fatalf("arquivo recuperado não pôde ser lido: %v", err)
		}
		if len(estado.Vistas) != 2 {
			t.Fatalf("esperava 2 notas no estado recuperado, obtido %d", len(estado.Vistas))
		}
	})
}


// ----- Teste de Validação das Notas Embutidas Reais -----

func TestNotasEmbutidasReais(t *testing.T) {
	// Carregar em cada idioma do app analisa todos os arquivos: pt-BR lê as bases e cada idioma lê as
	// próprias traduções (sufixo inválido e tradução sem base falham em qualquer idioma).
	for _, idioma := range dicionario.IdiomasSuportados {
		t.Run("idioma_"+idioma, func(t *testing.T) {
			notas, err := CarregarNotas(idioma)
			if err != nil {
				t.Fatalf("falha ao carregar notas no idioma %s: %v", idioma, err)
			}

			if len(notas) == 0 {
				t.Fatalf("nenhuma nota embutida encontrada para idioma %s", idioma)
			}

			for _, nota := range notas {
				if !strings.HasPrefix(nota.Id, nota.Data+"-") {
					t.Errorf("nota %s: data %s não corresponde ao id", nota.Id, nota.Data)
				}
				if strings.TrimSpace(nota.Titulo) == "" || len(nota.Grupos) == 0 {
					t.Errorf("nota %s: sem título ou sem grupos", nota.Id)
				}
			}
		})
	}
}
