package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"image"
	"image/png"
	"io"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unicode/utf8"

	"wails_app/overlay"
	"wails_app/revisao"
	"wails_app/traducao"
	"wails_app/atualizacao"

	"github.com/wailsapp/wails/v2/pkg/runtime"
	"wails_app/config"
	"wails_app/tela"
	"wails_app/dicionario"
	"wails_app/ocr"
	"wails_app/stt"
	"wails_app/tts"
	"wails_app/nuvem"
	"wails_app/progresso"
	"wails_app/segmentacao"
	"wails_app/util"
)

// LinhaTraduzida armazena a tradução de uma linha OCR inteira (antes da segmentação em palavras).
type LinhaTraduzida struct {
	Texto    string    `json:"texto"`
	Traducao string    `json:"traducao"` // "" se não traduzida (feature off, cota estourada ou erro de API)
	Caixa    []float64 `json:"caixa"`    // caixa da LINHA inteira (res.Caixa), NÃO repartida por palavra
}

// InfoCotaTraducao é o DTO exposto ao frontend com o estado de cota de tradução.
type InfoCotaTraducao struct {
	CaracteresUsados int     `json:"caracteresUsados"`
	CotaTotal        int     `json:"cotaTotal"`
	Percentual       float64 `json:"percentual"`
	AnoMes           string  `json:"anoMes"`
}

// OcrResult represents the JSON structure from Python API
type OcrResult struct {
	Texto     string    `json:"texto"`
	Confianca float64   `json:"confianca"`
	Caixa     []float64 `json:"caixa"`
}

// FlashcardCard representa um cartão processado para o frontend
type FlashcardCard struct {
	Hanzi          string    `json:"hanzi"`
	Pinyin         string    `json:"pinyin"`
	Significados   []string  `json:"significados"`
	Confianca      float64   `json:"confianca"`
	Caixa          []float64 `json:"caixa"`
	ImageId        int       `json:"imageId,omitempty"`
	TipoHanzi      string    `json:"tipoHanzi"`
	PosicaoRanking int       `json:"posicaoRanking"`
	NivelHSK       int       `json:"nivelHSK,omitempty"`
	// Fantasma marca card cujo texto sumiu da posição original entre um scan e outro (detectado
	// pelo vigia — ver vigia_cards.go): segue listado no Descobrimento, mas fica fora do highlight,
	// da detecção de palavra próxima ao mouse e dos pop-ups.
	Fantasma bool `json:"fantasma"`
}

// App struct
type App struct {
	ctx         context.Context
	Config      config.Config
	Dicionario  *dicionario.GerenciadorDicionario // fonte única de consulta: significado, leitura, decomposição, compostos (CC-CEDICT + MakeMeAHanzi + Traçados)
	Frases      *dicionario.GerenciadorFrases      // acervo de frases p/ revisão por contexto (carga preguiçosa, escalável por arquivos em dicionario/idiomas/<idioma>/frases/)
	Compreensao *dicionario.GerenciadorCompreensao // acervo de perguntas de compreensão de leitura com contexto
	revisao     *revisao.GerenciadorRevisao
	atalhos    *util.GerenciadorAtalhos

	// mu protege o estado do último scan (lastCards/lastLinhas/lastImagemPng/lastImageHash) e
	// popupsTodosVisivel — os bindings do Wails rodam em goroutines próprias e o atalho de "pop-up de
	// tudo" chega por outra goroutine ainda (loop.go).
	mu                 sync.RWMutex
	lastImageHash      string
	lastCards          []FlashcardCard
	lastLinhas         []LinhaTraduzida
	lastImagemPng      []byte // última captura JÁ CENSURADA, guardada para o modo resumo do Gemini poder enviar a imagem
	popupsTodosVisivel bool

	// fingerprintsCropsUltimoScan guarda, por palavra, as fingerprints dos crops do último scan:
	// palavra cujo recorte não mudou de um scan para o outro está parada na tela e não ganha
	// visualização de OCR (ver visualizacoes_ocr.go). Substituído por inteiro a cada scan.
	fingerprintsCropsUltimoScan map[string]map[uint64]bool

	// vigia confere, entre um scan e outro, se os cards do último OCR continuam na tela (highlights
	// fantasmas — ver vigia_cards.go). Reconstruído por inteiro a cada scan; o ponteiro é protegido
	// por mu. vigiaMutex serializa os ticks (TryLock: tick atrasado é pulado, nunca enfileirado) e
	// ocrEmAndamento faz o vigia ceder a vez enquanto um scan completo está no meio do caminho.
	vigia          *estadoVigia
	vigiaMutex     sync.Mutex
	ocrEmAndamento atomic.Bool

	// motorOcr é dono do ciclo de vida do processo de OCR (subir/derrubar/trocar). A posse migrou do
	// orquestrador (main.go) para o app para permitir trocar de motor em runtime (Fase 5, Passo 1).
	motorOcr *ocr.GerenciadorMotorOcr

	// motorTts é dono do ciclo de vida do processo de TTS (Kokoro-82M/ChatTTS). Criado
	// PREGUIÇOSAMENTE na primeira leitura em voz alta (garantirMotorTts) — nil até lá. ttsMutex
	// serializa as leituras e protege essa criação (ver tts.go).
	motorTts *tts.GerenciadorMotorTts
	ttsMutex sync.Mutex

	// geracaoTts é o contador de "contexto" das sínteses superáveis da revisão (FalarPinyinRevisao):
	// InvalidarSintesesTts o incrementa quando a revisão muda de questão ou sai, e cada síntese
	// enfileirada que, ao chegar na vez, vê uma geração mais nova é descartada sem sintetizar — assim
	// o motor não mói um backlog de questões que o usuário já passou (ver bindings_audio.go).
	geracaoTts atomic.Int64

	// motorStt é dono do ciclo de vida do processo de STT (Paraformer-ZH…). Criado PREGUIÇOSAMENTE
	// quando a revisão de pronúncia precisa escutar o microfone (garantirMotorStt) — nil até lá.
	// sttMutex serializa as escutas e protege essa criação (ver stt.go). sttParcialParar encerra o
	// laço de polling dos parciais da escuta em andamento (nil quando não há laço; sob sttMutex).
	motorStt        *stt.GerenciadorMotorStt
	sttMutex        sync.Mutex
	sttParcialParar chan struct{}

	// Pré-carregamento do cache de TTS (ver tts_precache.go): sintetiza EM LOTE a fala de todas as
	// palavras dos dicionários. Um lote longo por vez — preCacheTtsAtivo barra reentrância e
	// preCacheTtsCancelar sinaliza o cancelamento cooperativo. Protegidos por preCacheTtsMutex.
	preCacheTtsMutex    sync.Mutex
	preCacheTtsAtivo    bool
	preCacheTtsCancelar chan struct{}

	// nuvem sincroniza o banco de progresso com o Google Drive do usuário (ver nuvem_bindings.go).
	nuvem *nuvem.Gerenciador
	tela  *tela.GerenciadorTela

	// historicoRecomendacoes guarda o histórico recente de hanzis recomendados para evitar repetições imediatas
	historicoRecomendacoes      []string
	historicoRecomendacoesMutex sync.Mutex

	// Controle de ciclo de vida e estado da atualização automática
	onceIniciarServicos       sync.Once
	sinalVerificacaoInicial   chan struct{}
	fecharSinalVerificacaoOnce sync.Once
	atualizacaoEmAndamento    atomic.Bool
	estadoAtualizacaoMutex    sync.RWMutex
	faseAtualizacao           string // "normal" | "atualizando" | "falhou"
	versaoAlvoAtualizacao     string
	erroAtualizacao           string
}

// NewApp creates a new App application struct
func NewApp() *App {
	cfg, err := config.LoadConfig()
	if err != nil {
		fmt.Printf("Erro ao carregar config, usando default: %v\n", err)
		cfg = config.DefaultConfig()
	}

	// Regra C5: se o canal de atualização estiver vazio, assume o canal do próprio build.
	if cfg.CanalAtualizacao == "" {
		if atualizacao.CanalDoBuild() == atualizacao.CANAL_DEV {
			cfg.CanalAtualizacao = atualizacao.CANAL_DEV
		} else {
			cfg.CanalAtualizacao = atualizacao.CANAL_ESTAVEL
		}
	}

	gerenciadorDicionario, err := dicionario.NovoGerenciadorDicionario(cfg.IdiomaTraducao)
	if err != nil {
		fmt.Printf("Aviso: %v\n", err)
	}

	err = segmentacao.InitJieba()
	if err != nil {
		fmt.Printf("Aviso: Falha ao carregar dicionário Jieba: %v\n", err)
	}

	err = progresso.InitDB()
	if err != nil {
		fmt.Printf("Aviso: Falha ao inicializar banco de dados SQLite: %v\n", err)
	}

	// Conserta grafias que o antigo bug de conversão 么→幺 possa ter gravado no vocabulário (ver
	// migracao_vocabulario.go). Roda uma vez, com o dicionário e o banco já prontos.
	if corrigidas, invalidas := corrigirVocabularioCorrompido(gerenciadorDicionario); corrigidas > 0 || len(invalidas) > 0 {
		fmt.Printf("Migração de vocabulário (么/幺): %d corrigida(s); %d inválida(s) preservada(s): %v\n",
			corrigidas, len(invalidas), invalidas)
	}

	frases := dicionario.NovoGerenciadorFrases(cfg.IdiomaTraducao)
	// Frases geradas com IA pelo usuário entram no acervo junto das embarcadas, virando candidatas
	// da revisão geral. Registrada como fonte extra ANTES da primeira consulta (carga preguiçosa).
	frases.RegistrarFonteExtra(carregarFrasesUsuario)

	compreensao := dicionario.NovoGerenciadorCompreensao()
	compreensao.RegistrarFonteExtra(carregarPerguntasCompreensaoUsuario)

	app := &App{
		Config:                  cfg,
		Dicionario:              gerenciadorDicionario,
		Frases:                  frases,
		Compreensao:             compreensao,
		sinalVerificacaoInicial: make(chan struct{}),
		faseAtualizacao:         "normal",
	}
	app.revisao = revisao.NovoGerenciadorRevisao(app.Dicionario, app.Frases, func() config.Config { return app.Config })
	app.revisao.DefinirGerenciadorCompreensao(app.Compreensao)
	return app
}

// carregarFrasesUsuario converte as frases salvas no SQLite (frases geradas com IA pelo usuário)
// para o formato do acervo. É a fonte extra registrada no GerenciadorFrases; roda uma vez, na carga preguiçosa.
func carregarFrasesUsuario() ([]dicionario.Frase, error) {
	salvas, err := progresso.GetFrasesUsuario()
	if err != nil {
		return nil, err
	}
	frases := make([]dicionario.Frase, 0, len(salvas))
	for _, f := range salvas {
		frases = append(frases, dicionario.Frase{
			Chines:      f.Chines,
			Ingles:      f.Ingles,
			Atribuicao:  f.Atribuicao,
			Tema:        f.Tema,
			Dificuldade: f.Dificuldade,
		})
	}
	return frases, nil
}

// carregarPerguntasCompreensaoUsuario converte as perguntas salvas no SQLite para o formato em memória.
func carregarPerguntasCompreensaoUsuario() ([]dicionario.PerguntaCompreensao, error) {
	salvas, err := progresso.GetPerguntasCompreensao()
	if err != nil {
		return nil, err
	}
	perguntas := make([]dicionario.PerguntaCompreensao, 0, len(salvas))
	for _, p := range salvas {
		perguntas = append(perguntas, dicionario.PerguntaCompreensao{
			Contexto:              p.Contexto,
			Pergunta:              p.Pergunta,
			Opcoes:                p.Opcoes,
			IndiceRespostaCorreta: p.IndiceRespostaCorreta,
			PerguntaTraduzida:     p.PerguntaTraduzida,
			ContextoTraduzido:     p.ContextoTraduzido,
			Tema:                  p.Tema,
			Dificuldade:           p.Dificuldade,
			Atribuicao:            p.Atribuicao,
			Script:                p.Script,
		})
	}
	return perguntas, nil
}

// startup is called when the app starts. The context is saved
// so we can call the runtime methods
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx

	// Verificação inicial síncrona sob orçamento total de 8 segundos.
	ctxVerificacao, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()

	precisaAtualizar, alvo, motivo := a.verificarAtualizacaoInicial(ctxVerificacao)
	if !precisaAtualizar {
		if motivo != "" {
			fmt.Printf("Atualização automática: %s\n", motivo)
		}
		a.definirFaseAtualizacao("normal", "", "")
		a.liberarSinalVerificacaoInicial()
		a.iniciarServicos()
		return
	}

	fmt.Printf("Atualização automática: nova versão %s encontrada (%s). Baixando atualização…\n", alvo.Manifesto.Versao, alvo.Tag)
	a.definirFaseAtualizacao("atualizando", alvo.Manifesto.Versao, "")
	a.liberarSinalVerificacaoInicial()
	go a.executarAtualizacao(alvo)
}


// iniciarServicos inicializa a tela, os loops de fundo, nuvem, OCR e overlays.
// Protegida por sync.Once para garantir execução única (seja no boot normal ou após falha de atualização).
func (a *App) iniciarServicos() {
	a.onceIniciarServicos.Do(func() {
		a.tela = tela.NovoGerenciadorTela(a.ctx, func() config.Config { return a.Config })
		a.StartBackgroundLoop()
		fmt.Println("Backend Go Inicializado.")

		// Sincronização do banco com o Google Drive (se o usuário conectou; ver nuvem_bindings.go).
		a.iniciarNuvem(a.ctx)

		// Aplica a escolha de motores feita na tela custom do instalador (ver instalador.go), ANTES de
		// resolver/bootstrapar o motor — é o que faz bootstrapMotorPadrao baixar o motor ESCOLHIDO em vez
		// do padrão do catálogo. No-op silencioso em builds de dev (sem instalador, sem marcador).
		a.aplicarEscolhaDoInstalador()

		// O app é dono do processo de OCR (subir/derrubar/trocar). Todo motor é um EXECUTÁVEL — baixado no
		// AppData ou empacotado num bundle ao lado do app; NÃO há mais fallback para `python server.py`
		// (tudo é modular/baixável). motoresocr.ResolverMotorInicial escolhe o motor preferido/padrão
		// instalado ou o bundle; se NADA existe (first-run), o bootstrap baixa+instala+ativa o RapidOCR
		// padrão sozinho.
		a.motorOcr = ocr.NovoGerenciadorMotorOcr()
		if desc, ok := ocr.ResolverMotorInicial(a.Config.MotorOcrAtivo); ok {
			if err := a.motorOcr.Iniciar(desc); err != nil {
				fmt.Printf("Aviso: falha ao subir o backend de OCR: %v\n", err)
			}

			// Espera o motor responder o healthcheck antes de anunciá-lo pronto. Roda em segundo plano
			// para não travar a UI; o frontend ouve "ocr_pronto"/"ocr_indisponivel".
			go func() {
				if err := ocr.AguardarBackend(30 * time.Second); err != nil {
					fmt.Printf("Aviso: motor de OCR indisponível: %v\n", err)
					runtime.EventsEmit(a.ctx, "ocr_indisponivel", err.Error())
					return
				}
				fmt.Println("Motor de OCR pronto (healthcheck ok).")
				runtime.EventsEmit(a.ctx, "ocr_pronto")
			}()

			// Inicializa o overlay embutido.
			overlay.Iniciar()
		} else {
			// First-run: nenhum motor instalado nem em bundle. Baixa o motor padrão (+ overlay) e ativa — tudo
			// em segundo plano; a UI acompanha por "motor_bootstrap_inicio"/"motor_download_progresso"/"ocr_pronto".
			fmt.Println("Nenhum motor de OCR encontrado — iniciando o bootstrap do motor padrão…")
			go a.bootstrapMotorPadrao()
		}
	})
}

// shutdown is called at termination
func (a *App) shutdown(ctx context.Context) {
	// Derruba o motor de OCR (e sua árvore) — o app é dono desse processo desde a migração da posse.
	if a.motorOcr != nil {
		a.motorOcr.Encerrar()
	}
	// Derruba o motor de TTS, se alguma leitura em voz alta o subiu nesta sessão (é preguiçoso).
	if a.motorTts != nil {
		a.motorTts.Encerrar()
	}
	// Derruba o motor de STT, se a revisão de pronúncia o subiu nesta sessão (também preguiçoso).
	if a.motorStt != nil {
		a.motorStt.Encerrar()
	}
	overlay.Encerrar()
	if a.atalhos != nil {
		a.atalhos.DesativarTodos()
	}
	progresso.LimparImagensSessao()
	// Depois da limpeza das imagens de sessão, para o snapshot final subir já enxuto.
	a.encerrarNuvem()
}

// GetConfig returns the current configuration
func (a *App) GetConfig() (config.Config, error) {
	return a.Config, nil
}

// SaveConfig saves the configuration and updates the App state and global hotkeys dynamically
func (a *App) SaveConfig(newConfig config.Config) error {
	a.Config = newConfig
	a.AtualizarAtalhosGlobais()
	return config.SaveConfig(newConfig)
}

// AtualizarAtalhosGlobais ajusta reativamente os atalhos globais do SO.
func (a *App) AtualizarAtalhosGlobais() map[string]string {
	if a.atalhos == nil {
		a.atalhos = util.NovoGerenciadorAtalhos()
	}

	atalhos := map[string]string{
		"atalhoEscanear":           a.Config.AtalhoEscanear,
		"atalhoPopupTodos":         a.Config.AtalhoPopupTodos,
		"atalhoMarcarEstudo":       a.Config.AtalhoMarcarEstudo,
		"atalhoAlternarPopupHover": a.Config.AtalhoAlternarPopupHover,
	}

	handlers := map[string]func(){
		"atalhoEscanear":           func() { runtime.EventsEmit(a.ctx, "trigger_scan") },
		"atalhoPopupTodos":         a.alternarTodosPopups,
		"atalhoMarcarEstudo":       func() { runtime.EventsEmit(a.ctx, "trigger_save") },
		"atalhoAlternarPopupHover": func() { runtime.EventsEmit(a.ctx, "toggle_popup_hover") },
	}

	return a.atalhos.AtualizarAtalhos(atalhos, handlers)
}

// GetLastScreenshot retorna a última imagem escaneada codificada em base64 com prefixo data URI.
// É chamada pelo frontend para exibir o print na seção Descobrimento.
func (a *App) GetLastScreenshot() string {
	a.mu.RLock()
	defer a.mu.RUnlock()
	if len(a.lastImagemPng) == 0 {
		return ""
	}
	encoded := base64.StdEncoding.EncodeToString(a.lastImagemPng)
	return "data:image/png;base64," + encoded
}

// GetCotaTraducao retorna o estado atual da cota de tradução para exibição no frontend.
func (a *App) GetCotaTraducao() InfoCotaTraducao {
	usados, total, pct, anoMes := traducao.InfoCotaParaUI()
	return InfoCotaTraducao{
		CaracteresUsados: usados,
		CotaTotal:        total,
		Percentual:       pct,
		AnoMes:           anoMes,
	}
}

// ----- Pop-ups e destaques sobre a tela (overlay nativo) -----

// ShowHoverPopup exibe o card único perto do mouse.
func (a *App) ShowHoverPopup(pinyin, hanzi, sig string, x, y int) {
	overlay.MostrarHover(pinyin, hanzi, sig, x, y)
}

// HideHoverPopup oculta o card único.
func (a *App) HideHoverPopup() {
	overlay.OcultarHover()
}

// alternarTodosPopups liga/desliga a exibição simultânea dos pop-ups de todos os cards atuais.
// Acionado pelo atalho "mostrar pop-up de tudo" (AtalhoPopupTodos).
func (a *App) alternarTodosPopups() {
	a.mu.Lock()
	visivel := a.popupsTodosVisivel
	a.popupsTodosVisivel = !visivel
	a.mu.Unlock()

	if visivel {
		a.ocultarTodosPopups()
		return
	}
	a.mostrarTodosPopups()
}

// mostrarTodosPopups envia todos os cards do último scan ao overlay nativo para exibição simultânea.
// Quando a tradução está ativa e há linhas traduzidas, monta os pop-ups por LINHA (um pop-up por
// linha OCR com a tradução) em vez do modo padrão (um pop-up por palavra com pinyin/significado).
func (a *App) mostrarTodosPopups() {
	bounds := a.limitesMonitorAlvo()
	offX := bounds.Min.X
	offY := bounds.Min.Y

	a.mu.RLock()
	linhasCopia := make([]LinhaTraduzida, len(a.lastLinhas))
	copy(linhasCopia, a.lastLinhas)
	cardsCopia := make([]FlashcardCard, len(a.lastCards))
	copy(cardsCopia, a.lastCards)
	a.mu.RUnlock()

	// Desvio para Gemini: se ativo, tem chave e temos linhas, tentamos exibir pelo Gemini primeiro.
	if a.Config.GeminiAtivo && a.Config.GeminiApiKey != "" && len(linhasCopia) > 0 {
		if a.Config.GeminiPopupResumo {
			if a.mostrarResumoGemini(linhasCopia, offX, offY, bounds.Dx(), bounds.Dy()) {
				return
			}
		} else if a.Config.GeminiPopupLinha {
			if a.mostrarPopupsLinhaGemini(linhasCopia, offX, offY, bounds.Dx(), bounds.Dy()) {
				return
			}
		}
	}

	// Modo LINHA: se a tradução está ativa e há pelo menos uma linha com tradução não-vazia,
	// mostra um pop-up por LINHA (Hanzi = texto original, Sig = tradução, Pinyin = "").
	if !a.Config.GeminiAtivo && a.Config.TraducaoAtiva && a.Config.TraducaoApiKey != "" && len(linhasCopia) > 0 {
		a.traduzirLinhasPendentes(linhasCopia)

		var itens []overlay.ItemPopup
		temTraducao := false
		for i := range linhasCopia {
			l := &linhasCopia[i]
			if l.Traducao == "" {
				continue
			}
			temTraducao = true
			if len(l.Caixa) == 4 {
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

		if temTraducao {
			overlay.MostrarTodos(itens, bounds.Dx(), bounds.Dy())
			return
		}
	}

	// Modo PALAVRA (padrão): um pop-up por palavra com pinyin/significado. Card fantasma (o texto
	// já saiu da tela — ver vigia_cards.go) não gera pop-up.
	var itens []overlay.ItemPopup
	for _, c := range cardsCopia {
		if len(c.Caixa) != 4 || c.Fantasma {
			continue
		}
		itens = append(itens, overlay.ItemPopup{
			Pinyin:     "",
			Hanzi:      "",
			Sig:        strings.Join(c.Significados, ", "),
			SoTraducao: true,
			X0:         int(c.Caixa[0]) + offX,
			Y0:         int(c.Caixa[1]) + offY,
			X1:         int(c.Caixa[2]) + offX,
			Y1:         int(c.Caixa[3]) + offY,
		})
	}

	overlay.MostrarTodos(itens, bounds.Dx(), bounds.Dy())
}

// ocultarTodosPopups remove todos os pop-ups exibidos pelo "mostrar pop-up de tudo".
func (a *App) ocultarTodosPopups() {
	overlay.OcultarTodos()
	overlay.OcultarResumo()
}

// traduzirLinhasPendentes preenche in place a Traducao das linhas com hanzi que ainda não a têm:
// primeiro pelo cache (se ligado) e depois com UMA chamada em LOTE à API — a cota é cobrada por
// caractere, então o lote custa o mesmo que N chamadas sequenciais e o pop-up aparece muito mais
// rápido. Falha de API apenas deixa as traduções vazias (o chamador ignora linhas sem tradução).
func (a *App) traduzirLinhasPendentes(linhas []LinhaTraduzida) {
	var pendentes []int
	for i := range linhas {
		l := &linhas[i]
		if !contemHanzi(l.Texto) || l.Traducao != "" {
			continue
		}

		if a.ehHanziOuPalavraNoDicionario(l.Texto) {
			_, significados, _ := a.Dicionario.Leitura(strings.TrimSpace(l.Texto))
			if len(significados) > 0 {
				l.Traducao = significados[0]
				continue
			}
		}

		if a.Config.TraducaoUsarCache {
			if cached, achou, err := progresso.BuscarTraducaoCache(l.Texto); err == nil && achou {
				l.Traducao = cached
				continue
			}
		}
		pendentes = append(pendentes, i)
	}

	// Guard clauses: nada a traduzir, ou a pausa por cota barrou a chamada de API.
	if len(pendentes) == 0 {
		return
	}
	if a.Config.TraducaoPausarPorCota && traducao.CotaExcedida(a.Config.TraducaoLimiteCotaPercent) {
		return
	}

	textos := make([]string, len(pendentes))
	totalCaracteres := 0
	for j, idx := range pendentes {
		textos[j] = linhas[idx].Texto
		totalCaracteres += len([]rune(linhas[idx].Texto))
	}

	traducoes, err := traducao.TraduzirLote(a.Config.TraducaoApiKey, textos, "pt")
	if err != nil {
		fmt.Printf("Aviso: tradução em lote falhou: %v\n", err)
		return
	}
	_ = traducao.RegistrarUso(totalCaracteres)

	for j, idx := range pendentes {
		if traducoes[j] == "" {
			continue
		}
		linhas[idx].Traducao = traducoes[j]
		if a.Config.TraducaoUsarCache && !a.ehHanziOuPalavraNoDicionario(linhas[idx].Texto) {
			_ = progresso.SalvarTraducaoCache(linhas[idx].Texto, traducoes[j])
		}
	}
}

// ShowHighlight desenha uma borda ao redor de uma área específica
func (a *App) ShowHighlight(x0, y0, x1, y1 int) {
	bounds := a.limitesMonitorAlvo()
	overlay.MostrarDestaque(x0+bounds.Min.X, y0+bounds.Min.Y, x1+bounds.Min.X, y1+bounds.Min.Y)
}

// ShowEstudoHighlights envia molduras azuis para indicar palavras em estudo
func (a *App) ShowEstudoHighlights(boxes [][]float64) {
	overlay.MostrarDestaquesEstudo(a.ajustarCaixasAoMonitor(boxes))
}

// ShowEstudoParcialHighlights envia molduras amarelas para indicar caracteres individuais em estudo dentro de palavras
func (a *App) ShowEstudoParcialHighlights(boxes [][]float64) {
	overlay.MostrarDestaquesEstudoParcial(a.ajustarCaixasAoMonitor(boxes))
}

// ajustarCaixasAoMonitor desloca caixas locais do monitor de captura para coordenadas absolutas de
// tela (multi-monitor: o monitor alvo pode não começar em 0,0).
func (a *App) ajustarCaixasAoMonitor(boxes [][]float64) [][]float64 {
	bounds := a.limitesMonitorAlvo()

	ajustadas := make([][]float64, 0, len(boxes))
	for _, box := range boxes {
		if len(box) != 4 {
			continue
		}
		ajustadas = append(ajustadas, []float64{
			box[0] + float64(bounds.Min.X),
			box[1] + float64(bounds.Min.Y),
			box[2] + float64(bounds.Min.X),
			box[3] + float64(bounds.Min.Y),
		})
	}
	return ajustadas
}

// ----- Captura de tela + OCR -----

// codificadorPng é o encoder compartilhado das capturas e dos crops. BestSpeed porque os PNGs ou
// vão ao sidecar por localhost ou viram crops minúsculos de card — em ambos os casos o tamanho
// extra é irrelevante e a compressão padrão gastava CPU visível a cada scan.
var codificadorPng = png.Encoder{CompressionLevel: png.BestSpeed}
var clienteHttpOcr = &http.Client{}





func (a *App) CaptureAndOCR() ([]FlashcardCard, error) {
	// Sinaliza o scan completo para o vigia de fantasmas ceder a vez (ver vigia_cards.go).
	a.ocrEmAndamento.Store(true)
	defer a.ocrEmAndamento.Store(false)

	// As regiões censuradas são ignoradas aqui: o OCR não lê o preto da censura, e o vigia — o único
	// interessado nelas — recebe as da SUA captura a cada tick.
	img, bounds, _, err := a.tela.CapturarMonitorCensurado()
	if err != nil {
		return nil, err
	}

	// Fingerprint nos PIXELS CRUS (FNV-1a), ANTES de codificar: no auto-scan o caso comum é a tela
	// não ter mudado, e hashear os bytes do RGBA é muito mais barato que pagar a compressão PNG do
	// frame inteiro só para descobrir isso. A censura já foi aplicada acima, então o hash a reflete.
	hasher := fnv.New64a()
	_, _ = hasher.Write(img.Pix)
	hash := fmt.Sprintf("%x", hasher.Sum64())

	a.mu.Lock()
	if a.lastImageHash == hash {
		cards := a.lastCards
		a.mu.Unlock()
		return cards, nil
	}
	a.lastImageHash = hash
	fingerprintsScanAnterior := a.fingerprintsCropsUltimoScan
	// A tela mudou: o overlay de "pop-up de tudo" ficou obsoleto, então o ocultamos.
	popupsEstavamVisiveis := a.popupsTodosVisivel
	a.popupsTodosVisivel = false
	a.mu.Unlock()

	if popupsEstavamVisiveis {
		a.ocultarTodosPopups()
	}

	// Encode to PNG bytes. BestSpeed: a imagem vai ao sidecar por localhost, então o payload maior
	// não custa nada — e a compressão padrão gastava dezenas de ms de CPU por scan.
	var buf bytes.Buffer
	err = codificadorPng.Encode(&buf, img)
	if err != nil {
		return nil, fmt.Errorf("failed to encode PNG: %w", err)
	}
	imagemPng := append([]byte(nil), buf.Bytes()...)

	results, err := a.enviarParaOcr(&buf, a.ladoMaximoOcr(bounds))
	if err != nil {
		return nil, err
	}

	// Process strings using Jieba and CEDICT
	var cards []FlashcardCard
	var linhas []LinhaTraduzida

	// Crops dos cards deste scan, gravados em disco de uma vez no fim do loop (uma transação por
	// scan, não um fsync por palavra — ver progresso.SalvarImagensSessaoLote).
	var cropsPendentes []string
	var indicesCardsComCrop []int

	// Visualizações de OCR deste scan: só conta palavra cujo recorte mudou frente ao scan
	// anterior — palavra parada na tela não infla o contador (ver visualizacoes_ocr.go).
	rastreadorVisualizacoes := novoRastreadorVisualizacoesOcr(fingerprintsScanAnterior)

	// Estado do vigia de fantasmas deste scan (ver vigia_cards.go): posição, fingerprint e formas
	// de cada card, para os ticks entre scans detectarem cards que sumiram da tela. Os templates
	// (pixels do recorte) só são guardados com o rastreio de perdidos ligado — é só ele que os usa.
	vigiaEmConstrucao := &estadoVigia{limites: bounds}
	guardarTemplates := a.Config.RastrearPalavrasPerdidas

	for _, res := range results {
		// Pular palavras com confiança abaixo da configurada
		if res.Confianca < a.Config.ConfiancaMinimaOcr {
			continue
		}

		// A tradução da LINHA inteira é preenchida sob demanda em mostrarTodosPopups().
		linhas = append(linhas, LinhaTraduzida{
			Texto:    res.Texto,
			Traducao: "",
			Caixa:    res.Caixa,
		})

		palavras := segmentacao.SegmentarTextoChines(res.Texto)

		var refinedPalavras []string
		for _, p := range palavras {
			if a.Dicionario.TemEntrada(p) {
				refinedPalavras = append(refinedPalavras, p)
				continue
			}
			// FMM split para preservar compostos
			refinedPalavras = append(refinedPalavras, a.Dicionario.SegmentarPorDicionario(p)...)
		}

		// O OCR devolve a caixa da LINHA inteira; como cada palavra dela vira um card,
		// repartimos a largura da linha proporcionalmente à quantidade de caracteres de
		// cada palavra. Assim cada card recebe uma sub-caixa horizontal aproximada e o
		// hover consegue identificar qual card está realmente sob o mouse.
		totalRunes := 0
		for _, p := range refinedPalavras {
			totalRunes += utf8.RuneCountInString(p)
		}
		offsetRunes := 0

		for _, p := range refinedPalavras {
			_, significados, entradaCedict := a.Dicionario.Leitura(p)
			if entradaCedict != nil {
				p = a.converterCardParaTipoGerado(p, entradaCedict)
			}

			caixaCard := subCaixaDaPalavra(res.Caixa, p, totalRunes, offsetRunes)
			offsetRunes += utf8.RuneCountInString(p)

			// Ignorar cartões de caracteres individuais que não têm nenhum significado (como lixo de OCR)
			// ou que são componentes visuais/radicais avulsos (como 氵, 冫, 亻, etc).
			_, ehAbrev := dicionario.MapaAbrevParaCompleto[p]
			if utf8.RuneCountInString(p) == 1 && (len(significados) == 0 || ehAbrev) {
				continue
			}

			base64Img := a.croparCardEmBase64(img, caixaCard)

			card := a.montarCardDinamico(p, res.Confianca, caixaCard)
			cards = append(cards, card)

			if base64Img != "" {
				cropsPendentes = append(cropsPendentes, base64Img)
				indicesCardsComCrop = append(indicesCardsComCrop, len(cards)-1)
			}

			// Salvar no histórico de "Já Vistas"
			progresso.RegistrarVisto(p)
			a.registrarHanzisIndividuais(p)

			rastreadorVisualizacoes.registrarOcorrencia(p, base64Img)
			vigiaEmConstrucao.registrarCard(img, len(cards)-1, formasDoHanzi(p, entradaCedict), caixaCard, guardarTemplates)
		}
	}

	// Grava os crops do scan em disco de uma vez e preenche os ids nos cards correspondentes.
	if ids, err := progresso.SalvarImagensSessaoLote(cropsPendentes); err == nil {
		for i, indice := range indicesCardsComCrop {
			cards[indice].ImageId = ids[i]
		}
	}

	// Soma +1 visualização de OCR às palavras com recorte inédito (best-effort, como os demais
	// registros de progresso do scan: uma falha aqui não pode derrubar a entrega dos cards).
	_ = progresso.RegistrarVisualizacoesOcr(rastreadorVisualizacoes.palavrasComVisualizacaoNova())

	a.mu.Lock()
	a.lastCards = cards
	a.lastLinhas = linhas
	a.lastImagemPng = imagemPng
	a.fingerprintsCropsUltimoScan = rastreadorVisualizacoes.fingerprintsDoScan()
	a.vigia = vigiaEmConstrucao
	a.mu.Unlock()

	if a.ctx != nil {
		runtime.EventsEmit(a.ctx, "ocr_imagem_atualizada")
	}

	return cards, nil
}


// montarCardDinamico cria um FlashcardCard a partir de um caractere ou palavra em chinês (hanzi),
// consultando o dicionário e ranking em runtime (dinamicamente) para garantir a tradução e dados corretos.
func (a *App) montarCardDinamico(hanzi string, confianca float64, caixa []float64) FlashcardCard {
	pinyin, significados, _ := a.Dicionario.Leitura(hanzi)

	posicao := 0
	if a.Dicionario != nil && a.Dicionario.Banco != nil {
		freq := a.Dicionario.Banco.ObterFrequencia(hanzi)
		if freq != nil {
			posicao = freq.Posicao
		}
	}

	return FlashcardCard{
		Hanzi:          hanzi,
		Pinyin:         pinyin,
		Significados:   significados,
		Confianca:      confianca,
		Caixa:          caixa,
		TipoHanzi:      a.Dicionario.TipoHanzi(hanzi),
		PosicaoRanking: posicao,
	}
}



// enviarParaOcr manda um PNG ao sidecar de OCR com os headers de configuração e devolve as
// detecções decodificadas. ladoMaximo limita o lado maior da imagem no sidecar (0 = sem redução) —
// o scan completo passa a escala configurada, o vigia manda recortes minúsculos sem downscale.
func (a *App) enviarParaOcr(corpoPng *bytes.Buffer, ladoMaximo int) ([]OcrResult, error) {
	req, err := http.NewRequest("POST", ocr.EnderecoBase()+"/api/ocr", corpoPng)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/octet-stream")

	// Inject Config from App
	req.Header.Set("X-Ocr-Model", a.Config.ModeloOcr)
	req.Header.Set("X-Ocr-Device", a.Config.DispositivoOcr)
	req.Header.Set("X-Ocr-Hardware", a.Config.HardwareSelecionado)
	req.Header.Set("X-Ocr-Threads", fmt.Sprintf("%d", a.Config.ThreadsCpuOcr))
	req.Header.Set("X-Ocr-Max-Side", fmt.Sprintf("%d", ladoMaximo))

	resp, err := clienteHttpOcr.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to call Python API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error (%d): %s", resp.StatusCode, string(body))
	}

	var results []OcrResult
	if err := json.NewDecoder(resp.Body).Decode(&results); err != nil {
		return nil, fmt.Errorf("failed to decode JSON response: %w", err)
	}
	return results, nil
}

// ladoMaximoOcr converte a Escala de Resolução configurada (%) no limite do lado maior da captura
// (em px) enviado ao sidecar. 0 = resolução nativa (sem redução).
func (a *App) ladoMaximoOcr(bounds image.Rectangle) int {
	maxSidePct := a.Config.EscalaResolucaoOcr
	if maxSidePct <= 0 || maxSidePct > 100 {
		maxSidePct = 100
	}
	if maxSidePct == 100 {
		return 0
	}

	maior := bounds.Dx()
	if bounds.Dy() > maior {
		maior = bounds.Dy()
	}
	return int(float64(maior) * (float64(maxSidePct) / 100.0))
}

// converterCardParaTipoGerado troca a forma do card (simplificado↔tradicional) conforme a config
// de Tipo de Hanzi Gerado; a config de Exibição, quando restrita, tem precedência. Só se aplica a
// leituras vindas do CEDICT (a entrada casada traz as duas formas).
func (a *App) converterCardParaTipoGerado(p string, entrada *dicionario.EntradaDicionario) string {
	tipoGerado := a.Config.TipoHanziGerado
	if a.Config.TipoHanziExibicao != "" && a.Config.TipoHanziExibicao != "ambos" {
		tipoGerado = a.Config.TipoHanziExibicao
	}

	if tipoGerado == "simplificado" && entrada.Simplificado != "" {
		return entrada.Simplificado
	}
	if tipoGerado == "tradicional" && entrada.Tradicional != "" {
		return entrada.Tradicional
	}
	return p
}

// subCaixaDaPalavra reparte a caixa da LINHA inteira proporcionalmente à quantidade de caracteres
// da palavra, detectando a orientação (horizontal vs vertical) pelo aspect ratio do bounding box.
func subCaixaDaPalavra(caixaLinha []float64, palavra string, totalRunes, offsetRunes int) []float64 {
	if len(caixaLinha) != 4 || totalRunes <= 0 {
		return caixaLinha
	}

	nRunes := utf8.RuneCountInString(palavra)
	x0 := caixaLinha[0]
	y0 := caixaLinha[1]
	largura := caixaLinha[2] - caixaLinha[0]
	altura := caixaLinha[3] - caixaLinha[1]

	fracInicio := float64(offsetRunes) / float64(totalRunes)
	fracFim := float64(offsetRunes+nRunes) / float64(totalRunes)

	if altura > largura {
		// Texto vertical: distribui ao longo de Y, mantém X inteiro
		return []float64{x0, y0 + altura*fracInicio, caixaLinha[2], y0 + altura*fracFim}
	}
	// Texto horizontal (ou quadrado): distribui ao longo de X, mantém Y inteiro
	return []float64{x0 + largura*fracInicio, y0, x0 + largura*fracFim, caixaLinha[3]}
}

// croparCardEmBase64 recorta a região do card na captura (com o respiro padrão) e devolve o PNG
// em base64 — "" quando a caixa é inválida ou a codificação falha (o card só fica sem imagem).
func (a *App) croparCardEmBase64(img *image.RGBA, caixaCard []float64) string {
	rect, ok := retanguloDoCrop(img.Bounds(), caixaCard)
	if !ok {
		return ""
	}

	var bufImg bytes.Buffer
	if err := codificadorPng.Encode(&bufImg, img.SubImage(rect)); err != nil {
		return ""
	}
	return base64.StdEncoding.EncodeToString(bufImg.Bytes())
}

// Respiro ao redor da caixa do card nos recortes (crop salvo no banco e recorte vigiado).
const RESPIRO_CROP_PX = 10

// retanguloDoCrop devolve o retângulo do recorte de um card (caixa + respiro, contido na imagem).
// É a fonte única da geometria do recorte: o crop do card e o vigia de fantasmas precisam olhar
// exatamente para os mesmos pixels (ver vigia_cards.go). ok=false para caixa inválida/recorte vazio.
func retanguloDoCrop(limites image.Rectangle, caixaCard []float64) (image.Rectangle, bool) {
	if len(caixaCard) != 4 {
		return image.Rectangle{}, false
	}

	rect := image.Rect(
		int(caixaCard[0])-RESPIRO_CROP_PX,
		int(caixaCard[1])-RESPIRO_CROP_PX,
		int(caixaCard[2])+RESPIRO_CROP_PX,
		int(caixaCard[3])+RESPIRO_CROP_PX,
	)
	rect = rect.Intersect(limites) // certifica que não ultrapassa os limites da imagem
	if rect.Empty() {
		return image.Rectangle{}, false
	}
	return rect, true
}

// ----- Getters do último scan -----

func (a *App) GetLastCards() []FlashcardCard {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.lastCards
}

// GetSessionImage returns the base64 image from SQLite
func (a *App) GetSessionImage(id int) string {
	base64, err := progresso.GetImagemSessao(id)
	if err != nil {
		return ""
	}
	return base64
}

func (a *App) ehHanziOuPalavraNoDicionario(texto string) bool {
	texto = strings.TrimSpace(texto)
	if texto == "" || !contemHanzi(texto) {
		return false
	}
	if a.Dicionario == nil {
		return false
	}
	return a.Dicionario.TemEntrada(texto)
}
