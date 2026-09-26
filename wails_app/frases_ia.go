package main

import (
	"fmt"
	"strings"

	"wails_app/dicionario"
	"wails_app/dicionario/frases/taxonomia"
	"wails_app/gemini"
	"wails_app/progresso"
)

// ----- Seção: Geração de Frases com IA -----

const (
	QUANTIDADE_PADRAO_FRASES_IA = 10
	QUANTIDADE_MAXIMA_FRASES_IA = 40
	MAXIMO_ALVOS_FRASES_IA      = 12
	ATRIBUICAO_FRASE_IA         = "Gerada por IA (Gemini)"
)

func (a *App) GerarFrasesComIA(quantidade int) (int, error) {
	if a.Config.GeminiApiKey == "" {
		return 0, fmt.Errorf("configure a chave de API do Gemini na aba Motores antes de gerar frases")
	}

	if a.Config.GeminiPausarPorCota && gemini.CotaExcedida(a.Config.GeminiLimiteRequisicoesDia) {
		return 0, fmt.Errorf("a cota diária de requisições do Gemini foi atingida")
	}

	if quantidade <= 0 {
		quantidade = QUANTIDADE_PADRAO_FRASES_IA
	}
	if quantidade > QUANTIDADE_MAXIMA_FRASES_IA {
		quantidade = QUANTIDADE_MAXIMA_FRASES_IA
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
		if len(alvos) >= MAXIMO_ALVOS_FRASES_IA {
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
		return 0, fmt.Errorf("é preciso ter palavras no grupo de foco ou marcadas como 'em estudo' antes de gerar frases")
	}

	tipoHanzi := a.Config.TipoHanziExibicao
	if tipoHanzi != "simplificado" && tipoHanzi != "tradicional" {
		tipoHanzi = "simplificado"
	}

	frasesGeradas, err := gemini.GerarFrasesRevisao(
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
	for _, f := range frasesGeradas {
		if f.Frase == "" || f.Traducao == "" {
			continue
		}

		temaFinal := a.Config.RevisaoFiltroTema
		if temaFinal == "" {
			temaFinal = f.Tema
			if !taxonomia.TemaValido(temaFinal) {
				temaFinal = taxonomia.TEMA_OUTROS
			}
		}

		dificuldadeFinal := a.Config.RevisaoFiltroDificuldade
		if dificuldadeFinal == "" {
			dificuldadeFinal = f.Dificuldade
			if !taxonomia.DificuldadeValida(dificuldadeFinal) {
				dificuldadeFinal = "中级"
			}
		}

		inedita, err := progresso.AddFraseUsuario(
			f.Frase,
			f.Traducao,
			ATRIBUICAO_FRASE_IA,
			temaFinal,
			dificuldadeFinal,
		)
		if err != nil {
			return 0, err
		}

		if inedita {
			a.Frases.AdicionarFrase(dicionario.Frase{
				Chines:      f.Frase,
				Ingles:      f.Traducao,
				Atribuicao:  ATRIBUICAO_FRASE_IA,
				Tema:        temaFinal,
				Dificuldade: dificuldadeFinal,
			})
			ineditasCount++
		}
	}

	return ineditasCount, nil
}

// ObterFrasesIA retorna todas as frases geradas por IA salvas no banco de dados.
func (a *App) ObterFrasesIA() ([]progresso.FraseUsuario, error) {
	return progresso.GetFrasesUsuario()
}

// DescartarFraseIA remove uma frase gerada por IA tanto do banco de dados quanto do acervo em memória.
func (a *App) DescartarFraseIA(chines string) (bool, error) {
	if chines == "" {
		return false, nil
	}
	removido, err := progresso.RemoverFraseUsuario(chines)
	if err != nil {
		return false, err
	}
	if a.Frases != nil {
		a.Frases.RemoverFrase(chines)
	}
	return removido, nil
}
