package main

import (
	"wails_app/notasversao"
)

// ----- Métodos Exportados de Notas de Versão (Bindings do Wails) -----

// ObterNotasVersaoNaoVistas devolve as notas de versão que o usuário ainda não visualizou.
func (a *App) ObterNotasVersaoNaoVistas() ([]notasversao.Nota, error) {
	idioma := a.Config.IdiomaTraducao
	return notasversao.NaoVistas(idioma)
}


// MarcarNotasVersaoVistas registra os identificadores das notas exibidas como já vistas.
func (a *App) MarcarNotasVersaoVistas(ids []string) error {
	return notasversao.MarcarVistas(ids)
}


// ObterNotasVersao devolve o histórico completo de notas de versão sem alterar o estado de visualização.
func (a *App) ObterNotasVersao() ([]notasversao.Nota, error) {
	idioma := a.Config.IdiomaTraducao
	return notasversao.CarregarNotas(idioma)
}
