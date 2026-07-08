package main

import (
	"fmt"
	"time"

	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/wailsapp/wails/v2/pkg/runtime"
	"wails_app/mouse"
	"wails_app/util"
)

func (a *App) StartBackgroundLoop() {
	go func() {
		// Atalhos globais configuráveis (todos opcionais).
		registrarAtalhoGlobal(a.Config.AtalhoEscanear, func() { runtime.EventsEmit(a.ctx, "trigger_scan") })
		registrarAtalhoGlobal(a.Config.AtalhoMarcarEstudo, func() { runtime.EventsEmit(a.ctx, "trigger_save") })
		registrarAtalhoGlobal(a.Config.AtalhoPopupTodos, a.alternarTodosPopups)
		registrarAtalhoGlobal(a.Config.AtalhoAlternarPopupHover, func() { runtime.EventsEmit(a.ctx, "toggle_popup_hover") })

		// Goroutine separada para rastrear o mouse velozmente
		go func() {
			ultimoX, ultimoY := -1, -1
			for {
				select {
				case <-a.ctx.Done():
					return
				default:
					cfg := a.Config
					x, y, err := mouse.GetCursorPos()
					// mouse parado = evento redundante
					if err == nil && (x != ultimoX || y != ultimoY) {
						runtime.EventsEmit(a.ctx, "mouse_pos", map[string]interface{}{
							"x": x,
							"y": y,
						})
						ultimoX, ultimoY = x, y
					}

					ms := cfg.IntervaloAtualizacaoHoverMs
					if ms < 16 {
						ms = 16 // Mínimo de ~60fps
					}
					time.Sleep(time.Duration(ms) * time.Millisecond)
				}
			}
		}()

		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()

		lastScan := time.Now()

		for {
			select {
			case <-a.ctx.Done():
				return
			case <-ticker.C:
				cfg := a.Config

				// Check Auto-Scan interval
				if time.Since(lastScan).Seconds() >= float64(cfg.IntervaloCapturaSegundos) {
					// Check CPU limit
					shouldScan := true
					if cfg.LimitarPorUsoCpu {
						percent, err := cpu.Percent(0, false)
						if err == nil && len(percent) > 0 {
							if percent[0] > cfg.UsoMaximoCpuPercent {
								fmt.Printf("CPU usage too high (%.1f%% > %.1f%%). Skipping scan.\n", percent[0], cfg.UsoMaximoCpuPercent)
								shouldScan = false
							}
						}
					}

					if shouldScan {
						lastScan = time.Now()
						// Send event to frontend to trigger scan visually, or do it directly.
						// It's better to tell the frontend to scan so UI updates correctly.
						runtime.EventsEmit(a.ctx, "trigger_scan")
					}
				}
			}
		}
	}()
}

// registrarAtalhoGlobal registra um atalho global do SO e dispara aoAcionar a cada acionamento.
// Atalho vazio ou não-parseável é ignorado (feature opcional); falha de registro (ex.: combinação
// já tomada por outro app) é apenas logada — o app segue funcional sem o atalho.
func registrarAtalhoGlobal(combo string, aoAcionar func()) {
	if combo == "" {
		return
	}
	hk := util.ParseHotkey(combo)
	if hk == nil {
		return
	}
	if err := hk.Register(); err != nil {
		fmt.Printf("Aviso: falha ao registrar o atalho global %q: %v\n", combo, err)
		return
	}

	go func() {
		for {
			<-hk.Keydown()
			aoAcionar()
		}
	}()
}
