package main

import (
	"image"
	"wails_app/tela"
)

// GetMonitores retorna a lista de todos os monitores conectados
func (a *App) GetMonitores() []tela.Monitor {
	return tela.ListarMonitores()
}

// GetCaptureResolution retorna a resolução nativa do monitor capturado (display alvo),
// usada como teto/padrão do controle de Qualidade da Imagem do OCR.
func (a *App) GetCaptureResolution() tela.Resolucao {
	bounds := a.limitesMonitorAlvo()
	return tela.Resolucao{Largura: bounds.Dx(), Altura: bounds.Dy()}
}

// limitesMonitorAlvo devolve o retângulo (coordenadas absolutas da Root Window) do monitor de captura
// configurado, caindo no monitor 0 quando o alvo salvo não existe mais.
func (a *App) limitesMonitorAlvo() image.Rectangle {
	return tela.LimitesMonitorAlvo(a.Config.MonitorAlvo)
}
