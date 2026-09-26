package revisao

import (
	"fmt"
	"math/rand"
	"unicode"
	"wails_app/dicionario"
)

// avaliarCaracteresTexto calcula as porcentagens de caracteres chineses vistos e em estudo/aprendidos
// percorrendo cada rune do texto e verificando diretamente no mapaStatus do buscador.
// Isso é extremamente rápido (sem segmentação, sem alocação de mapas) e correto.
func (r *GerenciadorRevisao) avaliarCaracteresTexto(texto string) (pctVistas, pctEstudoAprendido float64, totalCaracteres int) {
	if r.buscador == nil {
		return 0, 0, 0
	}

	mapaStatus := r.buscador.ObterMapaStatus()

	qtdVistas := 0
	qtdEstudoAprendido := 0

	for _, ch := range texto {
		if !unicode.Is(unicode.Han, ch) {
			continue
		}
		totalCaracteres++
		s := string(ch)
		st := mapaStatus[s]
		if st != "" {
			qtdVistas++ // Qualquer status registrado = caractere visto
		}
		if st == dicionario.StatusEstudo || st == dicionario.StatusAprendido {
			qtdEstudoAprendido++
		}
	}

	if totalCaracteres == 0 {
		return 0, 0, 0
	}

	pctVistas = float64(qtdVistas) / float64(totalCaracteres)
	pctEstudoAprendido = float64(qtdEstudoAprendido) / float64(totalCaracteres)
	return
}

// ----- Seção: Modo de Compreensão de Texto -----

// preencherQuestaoCompreensao preenche a questão de múltipla escolha baseada em compreensão de leitura.
func (r *GerenciadorRevisao) preencherQuestaoCompreensao(questao QuestaoRevisao, alvo alvoRevisao, pools poolsRevisao) (QuestaoRevisao, error) {
	if questao.Variante == VarianteRespostaDialogo {
		return r.montarQuestaoRespostaDialogo(questao, alvo, pools)
	}

	if r.Compreensao == nil || r.Compreensao.TotalPerguntas() == 0 {
		return questao, fmt.Errorf("não há perguntas de compreensão no acervo para %q", questao.Hanzi)
	}

	candidatas := r.Compreensao.PerguntasComPalavra(alvo.entrada.Caractere)
	if len(candidatas) == 0 {
		runas := []rune(alvo.entrada.Caractere)
		if len(runas) > 0 {
			candidatas = r.Compreensao.PerguntasComCaractere(runas[0])
		}
	}
	if len(candidatas) == 0 {
		return questao, fmt.Errorf("nenhuma pergunta de compreensão contém %q", questao.Hanzi)
	}

	var candidatasFiltradas []dicionario.PerguntaCompreensao
	if r.buscador != nil {
		indices := rand.Perm(len(candidatas))
		avaliadas := 0

		for _, i := range indices {
			if avaliadas >= 300 {
				break
			}

			p := candidatas[i]
			avaliadas++

			// Avalia contexto, pergunta E opções (respostas) juntos para garantir que todos sejam compreensíveis
			textoCompleto := p.Contexto
			if p.Pergunta != "" {
				textoCompleto += "\n" + p.Pergunta
			}
			for _, opt := range p.Opcoes {
				if opt != "" {
					textoCompleto += "\n" + opt
				}
			}

			pctVistas, pctEstudoAprendido, totalCaracteres := r.avaliarCaracteresTexto(textoCompleto)
			if totalCaracteres == 0 {
				continue
			}

			// Exige no mínimo 95% de caracteres conhecidos (vistos) e 85% em estudo ou aprendidos
			if pctVistas >= 0.95-1e-9 && pctEstudoAprendido >= 0.85-1e-9 {
				candidatasFiltradas = append(candidatasFiltradas, p)
				if len(candidatasFiltradas) >= 15 {
					break
				}
			}
		}
	} else {
		candidatasFiltradas = candidatas
	}

	if len(candidatasFiltradas) == 0 {
		return questao, fmt.Errorf("nenhuma pergunta de compreensão atinge o mínimo de vocabulário para %q", questao.Hanzi)
	}

	var mapaStatus map[string]string
	if r.buscador != nil {
		mapaStatus = r.buscador.ObterMapaStatus()
	}
	escolhida := r.Compreensao.SelecionarPonderada(candidatasFiltradas, mapaStatus)

	r.aplicarPerguntaCompreensao(&questao, escolhida)
	return questao, nil
}

func (r *GerenciadorRevisao) aplicarPerguntaCompreensao(questao *QuestaoRevisao, p dicionario.PerguntaCompreensao) {
	textoContexto := p.Contexto
	textoPergunta := p.Pergunta
	tipoExibicao := r.Config().TipoHanziExibicao
	if tipoExibicao == "simplificado" || tipoExibicao == "tradicional" {
		textoContexto = r.Dicionario.ConverterTexto(textoContexto, tipoExibicao)
		textoPergunta = r.Dicionario.ConverterTexto(textoPergunta, tipoExibicao)
	}

	questao.FraseOriginal = textoContexto
	questao.FraseOriginalSegmentada = r.DecomporTextoRevisao(textoContexto)
	questao.PerguntaCompreensao = textoPergunta
	if textoPergunta != "" {
		questao.PerguntaCompreensaoSegmentada = r.DecomporTextoRevisao(textoPergunta)
	}
	questao.PerguntaTraduzida = p.PerguntaTraduzida
	questao.ContextoTraduzido = p.ContextoTraduzido
	questao.IndiceRespostaCorreta = p.IndiceRespostaCorreta
	questao.FraseTema = p.Tema
	questao.FraseDificuldade = p.Dificuldade
	questao.FraseAtribuicao = p.Atribuicao

	var opcoes []OpcaoRevisao
	for i, opt := range p.Opcoes {
		optTexto := opt
		if tipoExibicao == "simplificado" || tipoExibicao == "tradicional" {
			optTexto = r.Dicionario.ConverterTexto(optTexto, tipoExibicao)
		}
		opcoes = append(opcoes, OpcaoRevisao{
			Hanzi:     optTexto,
			Definicao: optTexto,
			Correta:   i == p.IndiceRespostaCorreta,
		})
	}

	questao.Opcoes = opcoes
}

// montarQuestaoRespostaDialogo seleciona um diálogo simples (A e B) e gera distratores para a fala B.
func (r *GerenciadorRevisao) montarQuestaoRespostaDialogo(questao QuestaoRevisao, alvo alvoRevisao, pools poolsRevisao) (QuestaoRevisao, error) {
	if r.Compreensao == nil {
		return questao, fmt.Errorf("repositório de compreensão não carregado")
	}

	candidatas := r.Compreensao.DialogosComPalavra(alvo.entrada.Caractere)
	if len(candidatas) == 0 {
		runas := []rune(alvo.entrada.Caractere)
		if len(runas) > 0 {
			candidatas = r.Compreensao.DialogosComCaractere(runas[0])
		}
	}
	if len(candidatas) == 0 {
		return questao, fmt.Errorf("nenhum diálogo contém %q", questao.Hanzi)
	}

	var candidatasFiltradas []dicionario.DialogoSimples
	if r.buscador != nil {
		indices := rand.Perm(len(candidatas))
		avaliadas := 0

		for _, i := range indices {
			if avaliadas >= 300 {
				break
			}

			d := candidatas[i]
			avaliadas++

			// Avalia as duas falas juntas
			pctVistas, pctEstudoAprendido, totalCaracteres := r.avaliarCaracteresTexto(d.Linha1 + "\n" + d.Linha2)
			if totalCaracteres == 0 {
				continue
			}

			// Exige no mínimo 95% de caracteres conhecidos (vistos) e 85% em estudo ou aprendidos
			if pctVistas >= 0.95-1e-9 && pctEstudoAprendido >= 0.85-1e-9 {
				candidatasFiltradas = append(candidatasFiltradas, d)
				if len(candidatasFiltradas) >= 15 {
					break
				}
			}
		}
	} else {
		candidatasFiltradas = candidatas
	}

	if len(candidatasFiltradas) == 0 {
		return questao, fmt.Errorf("nenhum diálogo atinge o mínimo de vocabulário para %q", questao.Hanzi)
	}

	dialogoCorreto := candidatasFiltradas[rand.Intn(len(candidatasFiltradas))]

	// Buscar 1 distrator validando que também respeite o mínimo de vocabulário (95% vistos, 85% estudo/aprendido)
	validadorDistrator := func(texto string) bool {
		if r.buscador == nil {
			return true
		}
		pctVistas, pctEstudoAprendido, total := r.avaliarCaracteresTexto(texto)
		if total == 0 {
			return true
		}
		return pctVistas >= 0.95-1e-9 && pctEstudoAprendido >= 0.85-1e-9
	}

	distratores := r.Compreensao.BuscarDistratoresDialogo(dialogoCorreto, 1, validadorDistrator)

	// Construir a questão
	questao.FraseOriginal = dialogoCorreto.Linha1
	questao.FraseOriginalSegmentada = r.DecomporTextoRevisao(dialogoCorreto.Linha1)
	questao.PerguntaCompreensao = dialogoCorreto.Linha2
	questao.PerguntaCompreensaoSegmentada = r.DecomporTextoRevisao(dialogoCorreto.Linha2)
	questao.ContextoTraduzido = dialogoCorreto.TraducaoL1

	opcoesTextos := append(distratores, dialogoCorreto.Linha2)
	rand.Shuffle(len(opcoesTextos), func(i, j int) {
		opcoesTextos[i], opcoesTextos[j] = opcoesTextos[j], opcoesTextos[i]
	})

	var opcoes []OpcaoRevisao
	tipoExibicao := r.Config().TipoHanziExibicao
	for _, optTexto := range opcoesTextos {
		if tipoExibicao == "simplificado" || tipoExibicao == "tradicional" {
			optTexto = r.Dicionario.ConverterTexto(optTexto, tipoExibicao)
		}
		opcoes = append(opcoes, OpcaoRevisao{
			Hanzi:     optTexto,
			Definicao: optTexto,
			Correta:   optTexto == dialogoCorreto.Linha2,
		})
	}
	questao.Opcoes = opcoes

	return questao, nil
}
