package main

import (
	"fmt"
	"strings"

	"wails_app/dicionario"
	"wails_app/dicionario/frases/taxonomia"
	"wails_app/gemini"
	"wails_app/progresso"
)

// ----- Seção: Geração de Perguntas de Compreensão com IA -----

const (
	QUANTIDADE_PADRAO_COMPREENSAO_IA = 5
	QUANTIDADE_MAXIMA_COMPREENSAO_IA = 30
	MAXIMO_ALVOS_COMPREENSAO_IA      = 12
	ATRIBUICAO_COMPREENSAO_IA        = "Gerado por IA (Gemini)"
)

// GerarPerguntasCompreensaoComIA gera novas questões de compreensão de múltipla escolha via Gemini.
func (a *App) GerarPerguntasCompreensaoComIA(quantidade int) (int, error) {
	if a.Config.GeminiApiKey == "" {
		return 0, fmt.Errorf("configure a chave de API do Gemini na aba Motores antes de gerar exercícios")
	}

	if a.Config.GeminiPausarPorCota && gemini.CotaExcedida(a.Config.GeminiLimiteRequisicoesDia) {
		return 0, fmt.Errorf("a cota diária de requisições do Gemini foi atingida")
	}

	if quantidade <= 0 {
		quantidade = QUANTIDADE_PADRAO_COMPREENSAO_IA
	}
	if quantidade > QUANTIDADE_MAXIMA_COMPREENSAO_IA {
		quantidade = QUANTIDADE_MAXIMA_COMPREENSAO_IA
	}

	palavrasAlvo, err := progresso.ObterFoco()
	if err != nil {
		return 0, err
	}

	if len(palavrasAlvo) == 0 {
		vocabs, err := progresso.GetAllVocab()
		if err != nil {
			return 0, err
		}
		for _, v := range vocabs {
			if v.Status == dicionario.StatusEstudo {
				palavrasAlvo = append(palavrasAlvo, v.Hanzi)
			}
		}
	}

	var alvos []gemini.AlvoRevisaoIa
	for _, p := range palavrasAlvo {
		if len(alvos) >= MAXIMO_ALVOS_COMPREENSAO_IA {
			break
		}
		pinyin, significados, _ := a.Dicionario.Leitura(p)
		if pinyin == "" || len(significados) == 0 {
			continue
		}
		alvos = append(alvos, gemini.AlvoRevisaoIa{
			Hanzi:     p,
			Pinyin:    pinyin,
			Definicao: strings.Join(significados, "; "),
		})
	}

	if len(alvos) == 0 {
		return 0, fmt.Errorf("é preciso ter palavras no grupo de foco ou marcadas como 'em estudo' antes de gerar perguntas")
	}

	tipoHanzi := a.Config.TipoHanziExibicao
	if tipoHanzi != "simplificado" && tipoHanzi != "tradicional" {
		tipoHanzi = "simplificado"
	}

	perguntasGeradas, err := gemini.GerarPerguntasCompreensao(
		a.Config.GeminiApiKey,
		a.Config.GeminiModelo,
		alvos,
		quantidade,
		a.Config.RevisaoFiltroTema,
		a.Config.RevisaoFiltroDificuldade,
		a.Config.RevisaoIaVibe,
		tipoHanzi,
	)
	if err != nil {
		return 0, err
	}

	_ = gemini.RegistrarRequisicao()

	ineditasCount := 0
	for _, p := range perguntasGeradas {
		if p.Contexto == "" || p.Pergunta == "" || len(p.Opcoes) != 4 {
			continue
		}

		temaFinal := a.Config.RevisaoFiltroTema
		if temaFinal == "" {
			temaFinal = p.Tema
			if !taxonomia.TemaValido(temaFinal) {
				temaFinal = taxonomia.TEMA_OUTROS
			}
		}

		dificuldadeFinal := a.Config.RevisaoFiltroDificuldade
		if dificuldadeFinal == "" {
			dificuldadeFinal = p.Dificuldade
			if !taxonomia.DificuldadeValida(dificuldadeFinal) {
				dificuldadeFinal = "中级"
			}
		}

		inedita, err := progresso.AddPerguntaCompreensao(
			p.Contexto,
			p.Pergunta,
			p.Opcoes,
			p.IndiceRespostaCorreta,
			p.PerguntaTraduzida,
			p.ContextoTraduzido,
			temaFinal,
			dificuldadeFinal,
			ATRIBUICAO_COMPREENSAO_IA,
			tipoHanzi,
		)
		if err != nil {
			return 0, err
		}

		if inedita && a.Compreensao != nil {
			a.Compreensao.AdicionarPergunta(dicionario.PerguntaCompreensao{
				Contexto:              p.Contexto,
				Pergunta:              p.Pergunta,
				Opcoes:                p.Opcoes,
				IndiceRespostaCorreta: p.IndiceRespostaCorreta,
				PerguntaTraduzida:     p.PerguntaTraduzida,
				ContextoTraduzido:     p.ContextoTraduzido,
				Tema:                  temaFinal,
				Dificuldade:           dificuldadeFinal,
				Atribuicao:            ATRIBUICAO_COMPREENSAO_IA,
				Script:                tipoHanzi,
			})
			ineditasCount++
		}
	}

	return ineditasCount, nil
}

// ObterPerguntasCompreensaoIA retorna todas as perguntas de compreensão salvas no banco SQLite.
func (a *App) ObterPerguntasCompreensaoIA() ([]progresso.PerguntaCompreensaoUsuario, error) {
	return progresso.GetPerguntasCompreensao()
}

// DescartarPerguntaCompreensaoIA exclui uma pergunta de compreensão do banco e do repositório em memória.
func (a *App) DescartarPerguntaCompreensaoIA(contexto string) (bool, error) {
	if contexto == "" {
		return false, nil
	}
	removido, err := progresso.RemoverPerguntaCompreensao(contexto)
	if err != nil {
		return false, err
	}
	if a.Compreensao != nil {
		a.Compreensao.RemoverPergunta(contexto)
	}
	return removido, nil
}
