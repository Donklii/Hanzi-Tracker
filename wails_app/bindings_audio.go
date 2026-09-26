package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/wailsapp/wails/v2/pkg/runtime"
	"wails_app/stt"
	"wails_app/tts"
	"wails_app/progresso"
)

// ----- Seção vinda de: bindings_tts.go -----

// Timeout longo de propósito: a primeira síntese de cada motor pode incluir o download dos
// pesos do Hugging Face (feito pelo sidecar) numa conexão lenta.
var clienteHttpTts = &http.Client{Timeout: 15 * time.Minute}

// ----- Leitura do pinyin em voz alta (TTS) -----
// Cliente do contrato de TTS (docs/CONTRATO-TTS.md) + o gatilho FalarPinyin exposto ao frontend.
// O motor de voz é um sidecar próprio (Kokoro-82M ou ChatTTS, ver pacote motorestts) que COEXISTE com
// o de OCR e sobe PREGUIÇOSAMENTE: só na primeira leitura em voz alta, nunca no startup — a feature
// é opcional e desligada por padrão.

// DespertarMotorTts garante que o motor de TTS esteja no ar (sobe o sidecar se não estiver).
// Útil para pré-aquecer o modelo em background antes da primeira chamada real de áudio.
func (a *App) DespertarMotorTts() {
	if a.Config.MotorTtsAtivo == "" {
		return
	}

	// Não bloqueamos a thread principal para não travar a interface do usuário
	// O TTS é thread-safe devido ao ttsMutex
	go func() {
		a.ttsMutex.Lock()
		defer a.ttsMutex.Unlock()

		_ = a.garantirMotorTts(a.Config.MotorTtsAtivo)
		a.emitirEstadoTts("") // Limpa o status
	}()
}

// emitirEstadoTts envia ao frontend o estado atual da síntese (barra de status). Mensagem vazia =
// terminou/limpou.
func (a *App) emitirEstadoTts(mensagem string) {
	if a.ctx == nil {
		return
	}
	runtime.EventsEmit(a.ctx, "tts_estado", mensagem)
}

// garantirMotorTts deixa o motor de voz `nome` no ar e saudável: cria o gerenciador na primeira
// chamada, sobe/troca o sidecar quando o motor pedido difere do ativo e ressuscita um processo que
// morreu. DEVE ser chamado já com a.ttsMutex adquirido (feito em FalarPinyin).
func (a *App) garantirMotorTts(nome string) error {
	if a.motorTts == nil {
		a.motorTts = tts.NovoGerenciadorMotorTts()
	}

	// Motor certo já no ar? Healthcheck curto pega o caso de processo morto (crash) — aí re-sobe.
	if a.motorTts.CatalogoAtivo() == nome {
		if err := tts.AguardarBackend(2 * time.Second); err == nil {
			return nil
		}
	}

	desc, ok := tts.ResolverMotorTts(nome)
	if !ok {
		return fmt.Errorf("o motor de voz '%s' não está instalado — baixe-o em Configurações → Geral → Gerenciar Motores de Voz", nome)
	}

	a.emitirEstadoTts("Iniciando o motor de voz…")
	return a.motorTts.Trocar(desc, 60*time.Second)
}

// FalarPinyin é o gatilho de "ler o pinyin em voz alta": recebe o HANZI do card (não a string de
// pinyin romanizada — é o hanzi que garante pronúncia nativa correta) e o nome do motor de TTS
// selecionado, e devolve os bytes do WAV sintetizado em base64 (o frontend toca via <audio> — o
// popup nativo Win32 não tem áudio). Consulta primeiro o cache em disco, indexado pela PRONÚNCIA
// (pinyin do hanzi): hanzis homófonos compartilham um único áudio, então a leitura sai instantânea e
// sem custo de CPU mesmo na primeira vez que ESTE hanzi aparece, se um homófono já foi lido antes.
//
// A primeira chamada de cada motor pode demorar: sobe o sidecar, carrega o modelo e — só na
// primeiríssima vez — baixa os pesos do Hugging Face (~330 MB no Kokoro, ~1 GB no ChatTTS). O
// progresso é anunciado ao frontend via o evento "tts_estado".
func (a *App) FalarPinyin(hanzi string, motor string) (string, error) {
	if hanzi == "" {
		return "", fmt.Errorf("hanzi vazio")
	}
	if _, ok := tts.ObterMotorTtsBaixavel(motor); !ok {
		return "", fmt.Errorf("motor de TTS desconhecido: %s", motor)
	}

	// Serializa as leituras: duas sínteses simultâneas duplicariam o trabalho pesado de CPU e
	// tocariam áudios sobrepostos. O lock também protege a criação preguiçosa de a.motorTts.
	a.ttsMutex.Lock()
	defer a.ttsMutex.Unlock()
	return a.sintetizarComCache(hanzi, motor)
}

// FalarPinyinRevisao é a variante SUPERÁVEL do FalarPinyin, usada pela revisão (leitura interativa e
// pré-carregamento). Anota a geração de TTS no momento da requisição e, ao FINALMENTE ganhar a vez no
// ttsMutex, desiste sem sintetizar se a revisão já invalidou aquela geração (InvalidarSintesesTts —
// mudança de questão ou saída da aba). Assim, uma navegação rápida não faz o motor moer um backlog de
// frases de questões que o usuário já passou: só a síntese em curso (que já pegou o lock) termina; as
// que ainda esperavam na fila saem na hora. Sínteses da MESMA questão têm a mesma geração e não se
// atropelam — quem decide qual áudio de fato toca é o token de reprodução do frontend.
func (a *App) FalarPinyinRevisao(hanzi string, motor string) (string, error) {
	if hanzi == "" {
		return "", fmt.Errorf("hanzi vazio")
	}
	if _, ok := tts.ObterMotorTtsBaixavel(motor); !ok {
		return "", fmt.Errorf("motor de TTS desconhecido: %s", motor)
	}

	// Geração no momento do PEDIDO (antes de entrar na fila do lock).
	geracaoPedido := a.geracaoTts.Load()

	a.ttsMutex.Lock()
	defer a.ttsMutex.Unlock()

	// Superada? Enquanto esperávamos a vez, a revisão avançou/saiu e invalidou esta geração. Não
	// sintetiza (devolve vazio): o frontend ignora o resultado e o motor pula direto para a próxima.
	if a.geracaoTts.Load() != geracaoPedido {
		return "", nil
	}
	return a.sintetizarComCache(hanzi, motor)
}

// InvalidarSintesesTts descarta as sínteses superáveis (FalarPinyinRevisao) que ainda estão na fila:
// incrementa a geração de TTS, então toda requisição já enfileirada com a geração anterior desiste ao
// ganhar a vez. A revisão chama isto ao trocar de questão e ao sair da aba, para o motor não moer o
// backlog de contextos que o usuário já deixou. Idempotente e barato (um incremento atômico).
func (a *App) InvalidarSintesesTts() {
	a.geracaoTts.Add(1)
}

// sintetizarComCache é o núcleo compartilhado por FalarPinyin e FalarPinyinRevisao: consulta o cache,
// sobe o motor se preciso, sintetiza e grava. DEVE ser chamado com a.ttsMutex já adquirido.
//
// Chave de cache = pinyin do hanzi (ver chaveCacheTts): hanzis homófonos caem na mesma chave e
// reaproveitam o mesmo áudio. `ehPalavra` separa as palavras/caracteres do dicionário (que o cache
// guarda) das FRASES (que nunca entram no cache — o WAV é grande e quase nunca se repete).
func (a *App) sintetizarComCache(hanzi string, motor string) (string, error) {
	chave, ehPalavra := a.chaveCacheTts(hanzi)

	// Cache primeiro — só para palavras: se já sintetizamos ESTA PRONÚNCIA com este motor, nem
	// precisa de sidecar. Frases pulam direto para a síntese (não são lidas nem gravadas no banco).
	if ehPalavra {
		if audio, achou, err := progresso.BuscarAudioTts(chave, motor); err == nil && achou {
			return base64.StdEncoding.EncodeToString(audio), nil
		}
	}

	if err := a.garantirMotorTts(motor); err != nil {
		a.emitirEstadoTts("")
		return "", err
	}

	a.emitirEstadoTts("Sintetizando fala… (a primeira vez pode baixar o modelo de voz)")
	defer a.emitirEstadoTts("")

	audio, err := a.sintetizarPinyin(hanzi)
	if err != nil {
		return "", err
	}

	// Só palavras do dicionário são gravadas no cache: frases ficam de fora de propósito.
	if ehPalavra {
		if err := progresso.SalvarAudioTts(chave, motor, audio); err != nil {
			fmt.Printf("Aviso: falha ao salvar o áudio no cache de TTS: %v\n", err)
		}
	}

	return base64.StdEncoding.EncodeToString(audio), nil
}

// chaveCacheTts devolve a CHAVE de cache de áudio de um texto e se ele é uma PALAVRA/caractere do
// dicionário — o que decide se pode ser cacheado. A chave de uma palavra é o seu pinyin canônico
// (com tom, minúsculo, sílabas separadas por um espaço): é a "interface" entre a SÍNTESE (feita a
// partir do hanzi, que garante a pronúncia nativa) e o CACHE (indexado por pinyin), então hanzis
// homófonos caem na MESMA chave e compartilham um único WAV — 马/码/吗 (todos "mǎ"/"ma") guardam um
// só áudio. A leitura canônica é a do TOPO da entrada do dicionário fundido (a fusão já escolheu
// entre o pinyin curado do caractere e o do CC-CEDICT, ver dicionario/banco.go).
//
// ehPalavra distingue o que O CACHE ACEITA das FRASES, que nunca entram nele:
//   - palavra do dicionário (inclui compostas e chengyu): é entrada de Buscar → chave = pinyin;
//   - caractere isolado fora do dicionário de palavras: uma única runa ainda é unidade cacheável,
//     mas sem pinyin conhecido cai na própria grafia como chave (sem dedup por pinyin);
//   - qualquer texto com mais de uma runa e sem entrada no dicionário é uma FRASE → não cacheável.
func (a *App) chaveCacheTts(hanzi string) (chave string, ehPalavra bool) {
	if a.Dicionario != nil && a.Dicionario.Banco != nil {
		if entradas := a.Dicionario.Banco.Buscar(hanzi); len(entradas) > 0 {
			if chave := normalizarChavePinyin(entradas[0].Pinyin); chave != "" {
				return chave, true
			}
		}
	}
	// Fora do dicionário de palavras: só um caractere isolado é cacheável (na própria grafia, sem
	// colisão possível — hanzi é CJK, pinyin é latino); texto maior é frase e não se cacheia.
	return hanzi, utf8.RuneCountInString(hanzi) == 1
}

// traduzirHanziParaChaveTts devolve só a chave de cache. Usado pelo pré-carregamento em lote, que
// só lida com palavras e caracteres do dicionário. Ver chaveCacheTts para a regra completa.
func (a *App) traduzirHanziParaChaveTts(hanzi string) string {
	chave, _ := a.chaveCacheTts(hanzi)
	return chave
}

// normalizarChavePinyin põe o pinyin numa forma canônica estável para servir de chave de cache:
// minúsculo, sem espaços nas pontas e com espaços internos colapsados a um só. PRESERVA os tons
// (diacríticos) — sem eles, sons diferentes (mā/mǎ) colidiriam numa chave só.
func normalizarChavePinyin(p string) string {
	return strings.Join(strings.Fields(strings.ToLower(p)), " ")
}

// sintetizarPinyin faz a chamada HTTP crua ao sidecar de TTS JÁ NO AR e devolve os bytes do WAV.
// NÃO consulta nem grava cache, NÃO sobe o motor e NÃO emite estado — é o núcleo compartilhado entre
// a leitura interativa (FalarPinyin) e o pré-carregamento em lote do cache (PreCarregarCacheTts). O
// chamador DEVE ter garantido o motor no ar (garantirMotorTts) sob a.ttsMutex.
func (a *App) sintetizarPinyin(hanzi string) ([]byte, error) {
	corpo, err := json.Marshal(map[string]string{"texto": hanzi})
	if err != nil {
		return nil, err
	}

	resp, err := clienteHttpTts.Post(tts.EnderecoBase()+"/api/tts", "application/json", bytes.NewReader(corpo))
	if err != nil {
		return nil, fmt.Errorf("falha ao sintetizar fala: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var respostaErro struct {
			Error string `json:"error"`
		}
		if errDecode := json.NewDecoder(resp.Body).Decode(&respostaErro); errDecode == nil && respostaErro.Error != "" {
			return nil, fmt.Errorf("motor de TTS: %s", respostaErro.Error)
		}
		return nil, fmt.Errorf("motor de TTS respondeu HTTP %d", resp.StatusCode)
	}

	audio, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("falha ao ler o áudio sintetizado: %w", err)
	}
	if len(audio) == 0 {
		return nil, fmt.Errorf("motor de TTS devolveu áudio vazio")
	}
	return audio, nil
}

// ObterClipesCacheTts faz uma verificação RÁPIDA (só leitura do banco: não sintetiza, não sobe o
// motor e não toca o ttsMutex) do cache de áudio das `palavras` — as palavras chinesas segmentadas
// de uma frase — no `motor`. Devolve os WAV em base64 na ORDEM recebida SE E SOMENTE SE todas já
// estiverem em cache; faltando qualquer uma, devolve lista vazia. É o suporte do fallback do
// frontend: com a frase inteira ainda fora do buffer, tocar as palavras já cacheadas em sequência
// aproxima a frase de imediato; faltando alguma, o frontend prefere aguardar a síntese da frase
// inteira a tocar uma sequência com lacunas. Fica FORA do ttsMutex de propósito: assim não bloqueia
// atrás da síntese da frase ainda em voo — que é justamente o caso em que este atalho é chamado.
func (a *App) ObterClipesCacheTts(palavras []string, motor string) ([]string, error) {
	if _, ok := tts.ObterMotorTtsBaixavel(motor); !ok {
		return nil, fmt.Errorf("motor de TTS desconhecido: %s", motor)
	}

	clipes := make([]string, 0, len(palavras))
	for _, palavra := range palavras {
		if palavra == "" {
			continue
		}
		chave, _ := a.chaveCacheTts(palavra)
		audio, achou, err := progresso.BuscarAudioTts(chave, motor)
		if err != nil {
			return nil, err
		}
		// Basta uma palavra faltar para a sequência sair com lacuna: desiste (o frontend aguarda).
		if !achou {
			return nil, nil
		}
		clipes = append(clipes, base64.StdEncoding.EncodeToString(audio))
	}
	return clipes, nil
}

// ----- Seção vinda de: bindings_tts_precache.go -----

// ----- Pré-carregamento do cache de áudio (TTS) -----
// Sintetiza EM LOTE a fala de todas as palavras dos dicionários embarcados (CC-CEDICT +
// MakeMeAHanzi) e grava cada WAV no cache de TTS (em arquivos por pinyin+motor), para que a leitura
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
	if _, ok := tts.ObterMotorTtsBaixavel(motor); !ok {
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
	if a.Dicionario != nil && a.Dicionario.Banco != nil {
		adicionar(a.Dicionario.Banco.TodasPalavras())
		adicionar(a.Dicionario.Banco.TodosCaracteres())
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

// ----- Seção vinda de: bindings_stt.go -----

// ----- Escuta do microfone e transcrição (STT) -----
// Cliente do contrato de STT (docs/CONTRATO-STT.md) + os gatilhos push-to-talk expostos ao
// frontend (revisão de pronúncia). A GRAVAÇÃO acontece no PRÓPRIO sidecar (sounddevice): a webview
// do Wails no Linux não tem SpeechRecognition nem getUserMedia, então o frontend só comanda
// iniciar/parar/cancelar. O motor de STT é um sidecar próprio (Paraformer-ZH, ver pacote
// motoresstt) que COEXISTE com os de OCR e TTS e sobe PREGUIÇOSAMENTE: só quando a revisão de
// pronúncia precisa escutar, nunca no startup.

// Timeout longo de propósito: o primeiro "parar" (ou o "preparar" do pré-aquecimento) pode incluir
// o download dos pesos do Hugging Face (feito pelo sidecar) numa conexão lenta.
var clienteHttpStt = &http.Client{Timeout: 15 * time.Minute}

// Intervalo entre consultas de transcrição parcial ao sidecar durante uma escuta (o "tempo real"
// do caminho por motor — o Web Speech tem parciais nativos, o sidecar responde a polling).
const intervaloParcialStt = 900 * time.Millisecond

// Cliente curto próprio dos parciais: um tick que não respondeu logo é descartado (o seguinte pega
// o texto mais novo) — não pode herdar os 15 min do cliente principal.
var clienteHttpSttParcial = &http.Client{Timeout: 10 * time.Second}

// emitirEstadoStt envia ao frontend o estado atual da escuta/transcrição (mensagem na tela de
// pronúncia). Mensagem vazia = terminou/limpou.
func (a *App) emitirEstadoStt(mensagem string) {
	if a.ctx == nil {
		return
	}
	runtime.EventsEmit(a.ctx, "stt_estado", mensagem)
}

// garantirMotorStt deixa o motor de STT `nome` no ar e saudável: cria o gerenciador na primeira
// chamada, sobe/troca o sidecar quando o motor pedido difere do ativo e ressuscita um processo que
// morreu. DEVE ser chamado já com a.sttMutex adquirido.
func (a *App) garantirMotorStt(nome string) error {
	if a.motorStt == nil {
		a.motorStt = stt.NovoGerenciadorMotorStt()
	}

	// Motor certo já no ar? Healthcheck curto pega o caso de processo morto (crash) — aí re-sobe.
	if a.motorStt.CatalogoAtivo() == nome {
		if err := stt.AguardarBackend(2 * time.Second); err == nil {
			return nil
		}
	}

	desc, ok := stt.ResolverMotorStt(nome)
	if !ok {
		return fmt.Errorf("o motor de escuta '%s' não está instalado — baixe-o em Configurações → Motores → Reconhecimento de Voz", nome)
	}

	a.emitirEstadoStt("Iniciando o motor de escuta…")
	return a.motorStt.Trocar(desc, 60*time.Second)
}

// DespertarMotorStt pré-aquece o motor de STT em segundo plano: sobe o sidecar e manda carregar o
// modelo (na primeiríssima vez, baixa ~240 MB de pesos do Hugging Face). O frontend chama ao
// entrar numa questão de pronúncia, para o primeiro push-to-talk sair sem essa espera. No-op
// silencioso quando o motor não está instalado — a UI já orienta o download.
func (a *App) DespertarMotorStt() {
	nome := a.Config.MotorSttAtivo
	if nome == "" {
		return
	}
	if _, ok := stt.ResolverMotorStt(nome); !ok {
		return
	}

	// Não bloqueamos a thread principal para não travar a interface do usuário.
	go func() {
		a.sttMutex.Lock()
		defer a.sttMutex.Unlock()

		if err := a.garantirMotorStt(nome); err != nil {
			a.emitirEstadoStt("")
			return
		}
		a.emitirEstadoStt("Preparando o reconhecimento de voz… (a primeira vez baixa o modelo)")
		_, err := a.chamarSidecarStt("/api/stt/preparar")
		if err != nil {
			fmt.Printf("Aviso: falha ao preparar o motor de STT: %v\n", err)
		}
		a.emitirEstadoStt("") // limpa o status
	}()
}

// IniciarEscutaStt começa a captura do microfone no sidecar (push-to-talk: o frontend chama ao
// PRESSIONAR o botão). Sobe o motor se preciso — na primeira escuta da sessão isso inclui o boot
// do sidecar (segundos); o pré-aquecimento (DespertarMotorStt) normalmente já pagou esse custo.
func (a *App) IniciarEscutaStt() error {
	nome := a.Config.MotorSttAtivo
	if _, ok := stt.ObterMotorSttBaixavel(nome); !ok {
		return fmt.Errorf("motor de STT desconhecido: %s", nome)
	}

	a.sttMutex.Lock()
	defer a.sttMutex.Unlock()

	if err := a.garantirMotorStt(nome); err != nil {
		a.emitirEstadoStt("")
		return err
	}

	// Derruba um eventual laço de parciais órfão ANTES de recomeçar a captura, para nenhum tick da
	// escuta anterior decodificar (e emitir) em cima da gravação nova.
	a.pararPollingParcialSttSemLock()

	a.emitirEstadoStt("")
	if _, err := a.chamarSidecarStt("/api/stt/iniciar"); err != nil {
		return err
	}

	// Escuta no ar: sobe o laço que consulta a transcrição parcial e a emite ao frontend — é o
	// que dá "tempo real" ao caminho por motor (espelha os parciais nativos do Web Speech).
	a.iniciarPollingParcialStt()
	return nil
}

// PararEscutaStt para a captura e devolve o texto transcrito (o frontend chama ao SOLTAR o botão).
// A primeira transcrição de cada sessão pode demorar: carga do modelo e — só na primeiríssima
// vez — download dos pesos do Hugging Face (~240 MB). O progresso é anunciado ao frontend via o
// evento "stt_estado".
func (a *App) PararEscutaStt() (string, error) {
	a.sttMutex.Lock()
	defer a.sttMutex.Unlock()

	// A escuta terminou: derruba o laço de parciais antes de transcrever, para nenhum parcial
	// atrasado chegar DEPOIS do texto final.
	a.pararPollingParcialSttSemLock()

	// Guard clause: nenhum motor no ar = nenhuma escuta em andamento (ex.: iniciar falhou).
	if a.motorStt == nil || a.motorStt.CatalogoAtivo() == "" {
		return "", fmt.Errorf("nenhuma escuta em andamento")
	}

	a.emitirEstadoStt("Transcrevendo fala… (a primeira vez pode baixar o modelo)")
	defer a.emitirEstadoStt("")

	corpo, err := a.chamarSidecarStt("/api/stt/parar")
	if err != nil {
		return "", err
	}

	var resposta struct {
		Texto string `json:"texto"`
	}
	if err := json.Unmarshal(corpo, &resposta); err != nil {
		return "", fmt.Errorf("resposta inválida do motor de STT: %w", err)
	}
	return resposta.Texto, nil
}

// CancelarEscutaStt descarta uma gravação em andamento sem transcrever (soltar o botão fora da
// área, troca de questão, desmontagem da tela). Idempotente: sem motor no ar, é um no-op.
func (a *App) CancelarEscutaStt() error {
	a.sttMutex.Lock()
	defer a.sttMutex.Unlock()

	a.pararPollingParcialSttSemLock()

	// Guard clause: nada rodando — nada a cancelar.
	if a.motorStt == nil || a.motorStt.CatalogoAtivo() == "" {
		return nil
	}
	_, err := a.chamarSidecarStt("/api/stt/cancelar")
	return err
}

// ----- Parciais em tempo real (polling de /api/stt/parcial) -----

// iniciarPollingParcialStt sobe a goroutine que consulta a transcrição parcial da escuta em
// andamento e a emite ao frontend via o evento "stt_parcial". DEVE ser chamado com a.sttMutex
// adquirido e com a escuta já iniciada no sidecar.
func (a *App) iniciarPollingParcialStt() {
	a.pararPollingParcialSttSemLock() // defesa contra um laço órfão de uma escuta anterior
	parar := make(chan struct{})
	a.sttParcialParar = parar
	go a.lacoParcialStt(parar)
}

// pararPollingParcialSttSemLock encerra o laço de parciais da escuta atual (no-op sem laço no ar).
// DEVE ser chamado com a.sttMutex adquirido.
func (a *App) pararPollingParcialSttSemLock() {
	if a.sttParcialParar == nil {
		return
	}
	close(a.sttParcialParar)
	a.sttParcialParar = nil
}

// lacoParcialStt é o corpo da goroutine de parciais: a cada tick pede ao sidecar a transcrição do
// áudio acumulado e emite o texto quando ele muda. Roda SEM o sttMutex de propósito — segurá-lo
// bloquearia o /parar; um tick que colidir com o parar apenas falha e é descartado. O laço morre
// pelo canal `parar` (fechado no parar/cancelar/nova escuta) ou quando o sidecar responde 404
// (motor antigo, sem o endpoint — os parciais degradam em silêncio).
func (a *App) lacoParcialStt(parar chan struct{}) {
	ticker := time.NewTicker(intervaloParcialStt)
	defer ticker.Stop()

	ultimoTexto := ""
	for {
		select {
		case <-parar:
			return
		case <-ticker.C:
		}

		resp, err := clienteHttpSttParcial.Post(stt.EnderecoBase()+"/api/stt/parcial", "application/json", bytes.NewReader(nil))
		if err != nil {
			continue // sidecar ocupado ou tick perdido: o próximo tenta de novo
		}
		if resp.StatusCode == http.StatusNotFound {
			resp.Body.Close()
			return // motor sem /parcial: não há o que consultar até a próxima escuta
		}

		var resposta struct {
			Texto string `json:"texto"`
		}
		errDecode := json.NewDecoder(resp.Body).Decode(&resposta)
		resp.Body.Close()
		if errDecode != nil || resp.StatusCode != http.StatusOK {
			continue
		}
		// "" = parcial indisponível neste tick (nada gravado/modelo ocupado), não "texto apagado".
		if resposta.Texto == "" || resposta.Texto == ultimoTexto {
			continue
		}
		ultimoTexto = resposta.Texto
		a.emitirParcialStt(resposta.Texto)
	}
}

// emitirParcialStt envia ao frontend a transcrição parcial da escuta em andamento.
func (a *App) emitirParcialStt(texto string) {
	if a.ctx == nil {
		return
	}
	runtime.EventsEmit(a.ctx, "stt_parcial", texto)
}

// chamarSidecarStt faz um POST cru a um endpoint do sidecar de STT JÁ NO AR e devolve o corpo da
// resposta. O chamador DEVE ter garantido o motor no ar (garantirMotorStt) sob a.sttMutex.
func (a *App) chamarSidecarStt(endpoint string) ([]byte, error) {
	resp, err := clienteHttpStt.Post(stt.EnderecoBase()+endpoint, "application/json", bytes.NewReader(nil))
	if err != nil {
		return nil, fmt.Errorf("falha ao falar com o motor de escuta: %w", err)
	}
	defer resp.Body.Close()

	var corpo bytes.Buffer
	if _, err := corpo.ReadFrom(resp.Body); err != nil {
		return nil, fmt.Errorf("falha ao ler a resposta do motor de escuta: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		var respostaErro struct {
			Error string `json:"error"`
		}
		if errDecode := json.Unmarshal(corpo.Bytes(), &respostaErro); errDecode == nil && respostaErro.Error != "" {
			return nil, fmt.Errorf("motor de STT: %s", respostaErro.Error)
		}
		return nil, fmt.Errorf("motor de STT respondeu HTTP %d", resp.StatusCode)
	}
	return corpo.Bytes(), nil
}

