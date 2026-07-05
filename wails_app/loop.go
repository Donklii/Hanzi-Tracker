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
		// Tentar registrar o atalho de captura manual (ex: Ctrl+Shift+E) configurado
		if a.Config.AtalhoEscanear != "" {
			if hk := util.ParseHotkey(a.Config.AtalhoEscanear); hk != nil {
				if err := hk.Register(); err == nil {
					go func() {
						for {
							<-hk.Keydown()
							runtime.EventsEmit(a.ctx, "trigger_scan")
						}
					}()
				}
			}
		}

		// Tentar registrar atalho de marcar estudo rápido
		if a.Config.AtalhoMarcarEstudo != "" {
			if hk := util.ParseHotkey(a.Config.AtalhoMarcarEstudo); hk != nil {
				if err := hk.Register(); err == nil {
					go func() {
						for {
							<-hk.Keydown()
							runtime.EventsEmit(a.ctx, "trigger_save")
						}
					}()
				}
			}
		}

		// Tentar registrar atalho de "mostrar pop-up de tudo" (liga/desliga o overlay)
		if a.Config.AtalhoPopupTodos != "" {
			if hk := util.ParseHotkey(a.Config.AtalhoPopupTodos); hk != nil {
				if err := hk.Register(); err == nil {
					go func() {
						for {
							<-hk.Keydown()
							a.alternarTodosPopups()
						}
					}()
				}
			}
		}

		// Tentar registrar atalho de "ligar/desligar popup do mouse"
		if a.Config.AtalhoAlternarPopupHover != "" {
			if hk := util.ParseHotkey(a.Config.AtalhoAlternarPopupHover); hk != nil {
				if err := hk.Register(); err == nil {
					go func() {
						for {
							<-hk.Keydown()
							runtime.EventsEmit(a.ctx, "toggle_popup_hover")
						}
					}()
				}
			}
		}

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

