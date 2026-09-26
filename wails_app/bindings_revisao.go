package main

import (
	"wails_app/progresso"
	"wails_app/revisao"
	"wails_app/revisao/jornada"
)

// ObterDadosEscritaHanzi delega a chamada para o GerenciadorRevisao.
func (a *App) ObterDadosEscritaHanzi(caractere string) (string, error) {
	return a.revisao.ObterDadosEscritaHanzi(caractere)
}

// ObterQuestoesRevisao delega a chamada para o GerenciadorRevisao.
func (a *App) ObterQuestoesRevisao(modo string, quantidade int) ([]revisao.QuestaoRevisao, error) {
	return a.revisao.ObterQuestoesRevisao(modo, quantidade)
}

// DecomporTextoRevisao delega a chamada para o GerenciadorRevisao.
func (a *App) DecomporTextoRevisao(texto string) []revisao.PalavraRevisao {
	return a.revisao.DecomporTextoRevisao(texto)
}

// ObterFocoRevisao delega a chamada para o GerenciadorRevisao.
func (a *App) ObterFocoRevisao() ([]revisao.ItemFocoRevisao, error) {
	return a.revisao.ObterFocoRevisao()
}

// AdicionarHanziFoco delega a chamada para o GerenciadorRevisao.
func (a *App) AdicionarHanziFoco(texto string) error {
	return a.revisao.AdicionarHanziFoco(texto)
}

// RemoverHanziFoco delega a chamada para o GerenciadorRevisao.
func (a *App) RemoverHanziFoco(texto string) error {
	return a.revisao.RemoverHanziFoco(texto)
}

// ObterProgressoRevisaoPalavras delega a chamada para o GerenciadorRevisao. O placar da revisão
// chama duas vezes (antes e depois da sessão) para exibir o avanço por palavra e por área.
func (a *App) ObterProgressoRevisaoPalavras(palavras []string) (revisao.ProgressoRevisaoPalavras, error) {
	return a.revisao.ProgressoPalavras(palavras)
}

// ----- Jornada de Revisão -----

// ObterArvoreJornada devolve a árvore da Jornada com as revisões de cada nível JÁ passadas por
// RevisoesEfetivas — o frontend recebe a lista que vai realmente praticar (sem motor de voz, os
// tipos que dependem dele já vêm substituídos), sem repetir a regra de substituição.
// Os ramos/níveis são COPIADOS antes da substituição: os slices de jornada.ObterArvore apontam para
// a árvore em cache do pacote, e escrever neles substituiria os tipos lá dentro para sempre (o
// usuário que instalasse um motor de voz depois continuaria sem fonética até reiniciar o app).
func (a *App) ObterArvoreJornada() (jornada.Arvore, error) {
	arvore, err := jornada.ObterArvore()
	if err != nil {
		return jornada.Arvore{}, err
	}

	ramos := make([]jornada.Ramo, len(arvore.Ramos))
	for iRamo, ramo := range arvore.Ramos {
		niveis := make([]jornada.Nivel, len(ramo.Niveis))
		for iNivel, nivel := range ramo.Niveis {
			nivel.Revisoes = a.revisao.RevisoesEfetivas(nivel)
			niveis[iNivel] = nivel
		}
		ramo.Niveis = niveis
		ramos[iRamo] = ramo
	}
	arvore.Ramos = ramos
	return arvore, nil
}

// ObterProgressoJornada devolve o mapa nivel_id -> revisões concluídas.
func (a *App) ObterProgressoJornada() (map[string]int, error) {
	return progresso.ObterProgressoJornada()
}

// ObterPrimeiraQuestaoRevisao devolve rapidamente apenas a primeira questão para definir o layout exato do skeleton.
func (a *App) ObterPrimeiraQuestaoRevisao(modo string) (revisao.QuestaoRevisao, error) {
	q, err := a.revisao.ObterPrimeiraQuestaoRevisao(modo)
	if err != nil || q == nil {
		return revisao.QuestaoRevisao{}, err
	}
	return *q, nil
}

// ObterQuestoesRevisaoComPrimeira monta a sessão de revisão preservando a primeira questão já gerada para o skeleton.
func (a *App) ObterQuestoesRevisaoComPrimeira(modo string, quantidade int, primeira revisao.QuestaoRevisao) ([]revisao.QuestaoRevisao, error) {
	return a.revisao.ObterQuestoesRevisaoComPrimeira(modo, quantidade, primeira)
}

// ObterQuestoesJornada delega a chamada para o GerenciadorRevisao.
func (a *App) ObterQuestoesJornada(nivelId string, revIndex int) ([]revisao.QuestaoRevisao, error) {
	return a.revisao.ObterQuestoesJornada(nivelId, revIndex)
}

// ObterQuestoesJornadaComPrimeira monta a sessão da Jornada preservando a primeira questão já gerada para o skeleton.
func (a *App) ObterQuestoesJornadaComPrimeira(nivelId string, revIndex int, primeira revisao.QuestaoRevisao) ([]revisao.QuestaoRevisao, error) {
	return a.revisao.ObterQuestoesJornadaComPrimeira(nivelId, revIndex, primeira)
}

// ObterPrimeiraQuestaoJornada devolve rapidamente apenas a primeira questão da Jornada para definir o layout exato do skeleton.
func (a *App) ObterPrimeiraQuestaoJornada(nivelId string, revIndex int) (revisao.QuestaoRevisao, error) {
	q, err := a.revisao.ObterPrimeiraQuestaoJornada(nivelId, revIndex)
	if err != nil || q == nil {
		return revisao.QuestaoRevisao{}, err
	}
	return *q, nil
}

// RegistrarRevisaoJornadaConcluida avança o progresso do nível em uma revisão (idempotente: só conta
// quando revIndex é a revisão que faltava).
func (a *App) RegistrarRevisaoJornadaConcluida(nivelId string, revIndex int) error {
	return progresso.RegistrarRevisaoJornada(nivelId, revIndex)
}
