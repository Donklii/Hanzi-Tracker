package main

import (
	"fmt"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"wails_app/motorestts"
	"wails_app/progresso"
)

// ----- Pré-carregamento do cache de áudio (TTS) -----
// Sintetiza EM LOTE a fala de todas as palavras dos dicionários embarcados (CC-CEDICT +
// MakeMeAHanzi) e grava cada WAV no cache de TTS (progresso.db, por hanzi+motor), para que a leitura
// em voz alta de qualquer card saia instantânea e sem custo de CPU depois. É uma operação LONGA
// (dezenas de milhares de sínteses no torch), então:
//   - roda em SEGUNDO PLANO (goroutine), o gatilho volta na hora;
//   - PULA o que já está em cache — resumível entre execuções e barato de re-rodar;
//   - pode ser CANCELADA (para no próximo item);
//   - reporta o andamento ao frontend pelo evento "tts_precache_progresso".

// maxFalhasSeguidasPreCache aborta o lote quando o motor morre no meio: sem isso, o loop varreria
// dezenas de milhares de itens fazendo chamadas HTTP que falham na hora.
const maxFalhasSeguidasPreCache = 20

// ProgressoPreCacheTts é o DTO do andamento do lote enviado ao frontend a cada punhado de itens.
type ProgressoPreCacheTts struct {
	Total        int    `json:"total"`        // palavras únicas a processar (dos dois dicionários)
	Processados  int    `json:"processados"`  // já visitados (cache hit + síntese + falha)
	Sintetizados int    `json:"sintetizados"` // sintetizados agora e gravados no cache
	JaEmCache    int    `json:"jaEmCache"`    // pulados por já existirem no cache
	Falhas       int    `json:"falhas"`       // erros de síntese/gravação em itens individuais
	EmAndamento  bool   `json:"emAndamento"`  // false = terminou, cancelou ou abortou
	Mensagem     string `json:"mensagem"`     // texto pronto para a UI
}

// emitirProgressoPreCacheTts publica o andamento do lote no frontend.
func (a *App) emitirProgressoPreCacheTts(prog ProgressoPreCacheTts) {
	if a.ctx == nil {
		return
	}
	runtime.EventsEmit(a.ctx, "tts_precache_progresso", prog)
}

// PreCarregarCacheTts dispara, em SEGUNDO PLANO, a síntese em lote de todas as palavras dos
// dicionários no motor de voz `motor`, gravando cada áudio no cache. Retorna na hora; o andamento
// chega ao frontend pelo evento "tts_precache_progresso". Erra se já houver um lote em curso ou se o
// motor for desconhecido.
func (a *App) PreCarregarCacheTts(motor string) error {
	if _, ok := motorestts.ObterMotorTtsBaixavel(motor); !ok {
		return fmt.Errorf("motor de TTS desconhecido: %s", motor)
	}

	a.preCacheTtsMutex.Lock()
	if a.preCacheTtsAtivo {
		a.preCacheTtsMutex.Unlock()
		return fmt.Errorf("já existe um pré-carregamento de áudio em andamento")
	}
	cancelar := make(chan struct{})
	a.preCacheTtsAtivo = true
	a.preCacheTtsCancelar = cancelar
	a.preCacheTtsMutex.Unlock()

	go a.executarPreCacheTts(motor, cancelar)
	return nil
}

// PararPreCacheTts sinaliza o cancelamento cooperativo do lote em curso (idempotente). O lote para
// no próximo item e emite o progresso final.
func (a *App) PararPreCacheTts() {
	a.preCacheTtsMutex.Lock()
	defer a.preCacheTtsMutex.Unlock()
	if a.preCacheTtsAtivo && a.preCacheTtsCancelar != nil {
		close(a.preCacheTtsCancelar)
		a.preCacheTtsCancelar = nil // evita double close se PararPreCacheTts for chamado de novo
	}
}

// executarPreCacheTts é o corpo do lote (roda na goroutine disparada por PreCarregarCacheTts).
func (a *App) executarPreCacheTts(motor string, cancelar <-chan struct{}) {
	// Ao terminar (fim, cancelamento ou aborto), libera o lote para um próximo disparo.
	defer func() {
		a.preCacheTtsMutex.Lock()
		a.preCacheTtsAtivo = false
		a.preCacheTtsCancelar = nil
		a.preCacheTtsMutex.Unlock()
	}()

	palavras := a.coletarPalavrasParaTts()
	total := len(palavras)
	prog := ProgressoPreCacheTts{Total: total, EmAndamento: true, Mensagem: "Preparando…"}
	a.emitirProgressoPreCacheTts(prog)

	// Guard clause: dicionários vazios (falha de carga) — nada a sintetizar.
	if total == 0 {
		prog.EmAndamento = false
		prog.Mensagem = "Nenhuma palavra encontrada nos dicionários."
		a.emitirProgressoPreCacheTts(prog)
		return
	}

	// Sobe o motor uma vez de cara: se nem instalado está, nem começa o lote.
	a.ttsMutex.Lock()
	erroMotor := a.garantirMotorTts(motor)
	a.ttsMutex.Unlock()
	if erroMotor != nil {
		prog.EmAndamento = false
		prog.Mensagem = "⚠️ " + erroMotor.Error()
		a.emitirProgressoPreCacheTts(prog)
		a.emitirEstadoTts("")
		return
	}

	// Dedup por PRONÚNCIA: muitos hanzis diferentes têm o mesmo pinyin (homófonos) e por isso o mesmo
	// áudio. Guardar as chaves já cobertas nesta execução evita bater no banco (e re-sintetizar) para
	// cada homófono — é a economia central pedida por este cache-por-pinyin.
	pinyinsFeitos := make(map[string]struct{})
	falhasSeguidas := 0
	for i, hanzi := range palavras {
		// Cancelamento cooperativo: para no próximo item.
		select {
		case <-cancelar:
			prog.EmAndamento = false
			prog.Mensagem = fmt.Sprintf("Cancelado — %d sintetizados, %d já em cache.", prog.Sintetizados, prog.JaEmCache)
			a.emitirProgressoPreCacheTts(prog)
			a.emitirEstadoTts("")
			return
		default:
		}

		prog.Processados = i + 1

		// Traduz para a chave de pronúncia e pula homófonos já cobertos nesta run (sem tocar o banco).
		chave := a.traduzirHanziParaChaveTts(hanzi)
		if _, ja := pinyinsFeitos[chave]; ja {
			prog.JaEmCache++
			if prog.Processados%25 == 0 || prog.Processados == total {
				prog.Mensagem = fmt.Sprintf("%d de %d — %d sintetizados, %d já em cache", prog.Processados, total, prog.Sintetizados, prog.JaEmCache)
				a.emitirProgressoPreCacheTts(prog)
			}
			continue
		}
		pinyinsFeitos[chave] = struct{}{}

		sintetizou, erroItem := a.preCacheUmHanzi(chave, hanzi, motor)
		switch {
		case erroItem != nil:
			prog.Falhas++
			falhasSeguidas++
			// Circuit breaker: motor caiu de vez — não adianta varrer o resto falhando.
			if falhasSeguidas >= maxFalhasSeguidasPreCache {
				prog.EmAndamento = false
				prog.Mensagem = fmt.Sprintf("⚠️ Interrompido após %d falhas seguidas (%s). O motor de voz pode ter caído.", falhasSeguidas, erroItem.Error())
				a.emitirProgressoPreCacheTts(prog)
				a.emitirEstadoTts("")
				return
			}
		case sintetizou:
			prog.Sintetizados++
			falhasSeguidas = 0
		default:
			prog.JaEmCache++
			falhasSeguidas = 0
		}

		// Emite a cada 25 itens (e sempre no último) para não inundar o frontend de eventos.
		if prog.Processados%25 == 0 || prog.Processados == total {
			prog.Mensagem = fmt.Sprintf("%d de %d — %d sintetizados, %d já em cache", prog.Processados, total, prog.Sintetizados, prog.JaEmCache)
			a.emitirProgressoPreCacheTts(prog)
		}
	}

	prog.EmAndamento = false
	prog.Mensagem = fmt.Sprintf("✅ Concluído — %d sintetizados, %d já em cache, %d falhas.", prog.Sintetizados, prog.JaEmCache, prog.Falhas)
	a.emitirProgressoPreCacheTts(prog)
	a.emitirEstadoTts("")
}

// coletarPalavrasParaTts junta as formas escritas dos dois dicionários embarcados, sem repetição de
// hanzi. A dedup por PRONÚNCIA (homófonos → um só áudio) acontece depois, no loop, via a chave de
// pinyin — aqui só evitamos visitar o mesmo hanzi duas vezes.
func (a *App) coletarPalavrasParaTts() []string {
	vistos := make(map[string]struct{})
	var palavras []string
	adicionar := func(fonte []string) {
		for _, p := range fonte {
			if p == "" {
				continue
			}
			if _, ja := vistos[p]; ja {
				continue
			}
			vistos[p] = struct{}{}
			palavras = append(palavras, p)
		}
	}
	if a.Cedict != nil {
		adicionar(a.Cedict.TodasPalavras())
	}
	if a.BancoHanzi != nil {
		adicionar(a.BancoHanzi.TodosCaracteres())
	}
	return palavras
}

// preCacheUmHanzi garante no cache o áudio de uma pronúncia. Recebe a `chavePinyin` (chave do cache)
// e o `hanzi` (o que é de fato enviado ao motor — a síntese é feita a partir do hanzi para sair
// nativa). Devolve sintetizou=false se já havia cache (nada a fazer), sintetizou=true se sintetizou
// agora, ou um erro. Adquire a.ttsMutex por item, o que serializa com a leitura interativa
// (FalarPinyin) — só uma síntese por vez — e permite que um clique do usuário intercale entre os
// itens do lote em vez de esperar tudo terminar.
func (a *App) preCacheUmHanzi(chavePinyin, hanzi, motor string) (sintetizou bool, err error) {
	a.ttsMutex.Lock()
	defer a.ttsMutex.Unlock()

	if _, achou, errBusca := progresso.BuscarAudioTts(chavePinyin, motor); errBusca == nil && achou {
		return false, nil
	}

	// Rede de segurança: se o motor caiu entre itens, garantirMotorTts o ressuscita (healthcheck
	// curto quando já está no ar, então é barato no caminho feliz).
	if err := a.garantirMotorTts(motor); err != nil {
		return false, err
	}

	audio, err := a.sintetizarPinyin(hanzi)
	if err != nil {
		return false, err
	}
	if err := progresso.SalvarAudioTts(chavePinyin, motor, audio); err != nil {
		return true, fmt.Errorf("falha ao gravar no cache: %w", err)
	}
	return true, nil
}
