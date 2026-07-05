package main

import (
	"fmt"
	"strings"
	"time"
	"wails_app/gemini"
	"wails_app/overlay"
	"wails_app/progresso"
)

// InfoCotaGemini é o DTO exposto ao frontend com o estado de cota de requisições do Gemini.
type InfoCotaGemini struct {
	RequisicoesUsadas int    `json:"requisicoesUsadas"`
	Data              string `json:"data"`
}

// GetCotaGemini retorna o estado atual da cota do Gemini para exibição no frontend.
func (a *App) GetCotaGemini() InfoCotaGemini {
	usados, data := gemini.InfoCotaParaUI()
	return InfoCotaGemini{
		RequisicoesUsadas: usados,
		Data:              data,
	}
}

// mostrarResumoGemini exibe um pop-up de resumo da tela gerado pelo Gemini.
func (a *App) mostrarResumoGemini(linhas []LinhaTraduzida, monX, monY, monW, monH int) bool {
	var textos []string
	for _, l := range linhas {
		if contemHanzi(l.Texto) {
			textos = append(textos, l.Texto)
		}
	}

	if len(textos) == 0 {
		return false
	}

	titulo := "Resumo da Tela (Gemini)"
	canto := a.Config.GeminiCantoResumo

	if a.Config.GeminiPausarPorCota && gemini.CotaExcedida(a.Config.GeminiLimiteRequisicoesDia) {
		overlay.MostrarResumo(titulo, "Cota diária do Gemini atingida — aumente o limite nas configurações ou aguarde amanhã.", canto, monX, monY, monW, monH, -1)
		return true
	}

	var imagemPng []byte
	if a.Config.GeminiEnviarImagem {
		a.mu.RLock()
		if len(a.lastImagemPng) > 0 {
			imagemPng = append([]byte(nil), a.lastImagemPng...)
		}
		a.mu.RUnlock()
	}

	// Inicia rotina para enviar e aguardar resposta
	go func() {
		resumoChan := make(chan string, 1)
		errChan := make(chan error, 1)

		go func() {
			resumo, err := gemini.ResumirTela(a.Config.GeminiApiKey, a.Config.GeminiModelo, textos, imagemPng)
			if err != nil {
				errChan <- err
			} else {
				resumoChan <- resumo
			}
		}()

		dots := 1
		direction := 1
		ticker := time.NewTicker(500 * time.Millisecond)
		defer ticker.Stop()

		// Estado inicial
		overlay.MostrarResumo(titulo, "Enviando imagem e textos para o Gemini...", canto, monX, monY, monW, monH, 0)

		for {
			select {
			case err := <-errChan:
				overlay.MostrarResumo(titulo, "Erro ao consultar o Gemini: "+err.Error(), canto, monX, monY, monW, monH, -1)
				return
			case res := <-resumoChan:
				_ = gemini.RegistrarRequisicao()
				overlay.MostrarResumo(titulo, res, canto, monX, monY, monW, monH, -1)
				return
			case <-ticker.C:
				dots += direction
				if dots == 4 {
					dots = 2
					direction = -1
				} else if dots == 0 {
					dots = 2
					direction = 1
				}
				overlay.MostrarResumo(titulo, "Esperando o Gemini responder" + strings.Repeat(".", dots), canto, monX, monY, monW, monH, 0)
			}
		}
	}()

	return true
}

// mostrarPopupsLinhaGemini exibe traduções por linha usando a API do Gemini.
func (a *App) mostrarPopupsLinhaGemini(linhas []LinhaTraduzida, offX, offY, sw, sh int) bool {
	var linhasComHanzi []int
	for i, l := range linhas {
		if contemHanzi(l.Texto) {
			linhasComHanzi = append(linhasComHanzi, i)
		}
	}

	if len(linhasComHanzi) == 0 {
		return false
	}

	var textosPendentes []string
	var pendentesMap = make(map[int]int) // mapeia index de textosPendentes (1-based) -> index em linhas

	for _, idx := range linhasComHanzi {
		l := &linhas[idx]
		
		if a.Config.TraducaoUsarCache {
			if cached, achou, err := progresso.BuscarTraducaoCache(l.Texto); err == nil && achou {
				l.Traducao = cached
				continue
			}
		}

		textosPendentes = append(textosPendentes, l.Texto)
		pendentesMap[len(textosPendentes)] = idx
	}

	if len(textosPendentes) > 0 {
		podeChamarAPI := true
		if a.Config.GeminiPausarPorCota {
			if gemini.CotaExcedida(a.Config.GeminiLimiteRequisicoesDia) {
				podeChamarAPI = false
			}
		}

		if podeChamarAPI {
			mapaTraducoes, err := gemini.TraduzirLinhas(a.Config.GeminiApiKey, a.Config.GeminiModelo, textosPendentes)
			if err == nil {
				_ = gemini.RegistrarRequisicao()
				for numL, trad := range mapaTraducoes {
					if idxLinha, ok := pendentesMap[numL]; ok {
						l := &linhas[idxLinha]
						l.Traducao = trad
						if a.Config.TraducaoUsarCache {
							_ = progresso.SalvarTraducaoCache(l.Texto, trad)
						}
					}
				}
			} else {
				fmt.Printf("Aviso: TraduzirLinhas com Gemini falhou: %v\n", err)
			}
		}
	}

	var itens []overlay.ItemPopup
	for _, idx := range linhasComHanzi {
		l := linhas[idx]
		if l.Traducao != "" && len(l.Caixa) == 4 {
			itens = append(itens, overlay.ItemPopup{
				Pinyin:     "",
				Hanzi:      "",
				Sig:        l.Traducao,
				SoTraducao: true,
				X0:         int(l.Caixa[0]) + offX,
				Y0:         int(l.Caixa[1]) + offY,
				X1:         int(l.Caixa[2]) + offX,
				Y1:         int(l.Caixa[3]) + offY,
			})
		}
	}

	if len(itens) == 0 {
		return false
	}

	overlay.MostrarTodos(itens, sw, sh)
	return true
}
