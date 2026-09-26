package main

// ----- Seção: Vigia de highlights fantasmas -----
// A cada segundo (nunca no mesmo segundo do scan completo — ver loop.go), o vigia recaptura a tela
// e confere, card a card do último OCR, se a palavra ainda está no recorte original, comparando a
// assinatura perceptual (luma + crominância, ver assinaturaDeRegiao) do recorte atual com a última
// confirmada:
//   • assinatura dentro da margem  → a palavra segue na tela, nada a fazer (variação de fundo é
//     absorvida pela tolerância — HUD/legenda sobre cena que muda não aciona o OCR toda iteração);
//   • assinatura fora da margem    → caso ambíguo: SÓ o recorte (ampliado) vai ao OCR — se o texto
//     lido ainda contém o hanzi do card, apenas atualizamos a referência; senão o card vira
//     FANTASMA: continua listado no Descobrimento até o próximo scan, mas sai do highlight, da
//     detecção de palavra próxima ao mouse e dos pop-ups (campo Fantasma).
// Card fantasma é desconsiderado nos ticks seguintes. Com o rastreio de perdidos ligado
// (RastrearPalavrasPerdidas), o vigia ainda procura o recorte original pela tela inteira e, ao
// reencontrá-lo (ex.: texto que rolou), reativa o card na nova posição.

import (
	"bytes"
	"fmt"
	"image"
	"strings"

	"github.com/wailsapp/wails/v2/pkg/runtime"
	"wails_app/dicionario"
)

// Recortes ambíguos enviados ao OCR por tick; o excedente segue ativo e é reavaliado no próximo.
const MAXIMO_VERIFICACOES_OCR_POR_TICK = 8

// Cards perdidos procurados pela tela por tick (rotativo entre os ticks via cursorPerdidos).
const MAXIMO_BUSCAS_PERDIDOS_POR_TICK = 8

// Evento que avisa o frontend que fantasmas/posições mudaram (ele re-busca GetLastCards).
const EVENTO_VIGIA_CARDS_ATUALIZADOS = "vigia_cards_atualizados"

// Lado da grade quadrada da assinatura perceptual do recorte (16×16 células, 3 canais por célula:
// luma, Cb e Cr).
const LADO_ASSINATURA_VIGIA = 16

// Margem de tolerância na comparação das assinaturas (0–255, distância média por amostra depois de
// normalizar a média de cada canal — luma, Cb e Cr): abaixo dela a mudança é tratada como ruído de
// fundo (HUD/legenda sobre cena que muda, fade de brilho ou de cor) e NÃO aciona o OCR. A distância
// final é o MAIOR entre os três canais (ver distanciaAssinaturas) — um só canal estourando já basta.
// Considerável de propósito — calibrável na prática.
const TOLERANCIA_ASSINATURA_VIGIA = 20

// Lado mínimo do recorte enviado ao OCR: recortes menores são ampliados por vizinho-mais-próximo,
// porque o detector do sidecar erra/rejeita imagens minúsculas (uma palavra isolada é pequena).
const MINIMO_LADO_OCR_VIGIA = 48

// cardVigiado é a visão do vigia sobre um card do último scan.
type cardVigiado struct {
	indice        int             // posição do card em lastCards
	formasHanzi   []string        // grafias aceitas na verificação por OCR (forma do card + simplificado/tradicional)
	rect          image.Rectangle // recorte vigiado (caixa + respiro) em coordenadas da captura
	caixaOriginal []float64       // caixa do card no scan de origem (base para deslocar ao reencontrar)
	deslocamento  image.Point     // quanto o card andou desde o scan (rastreio de perdidos)
	assinatura    []byte          // assinatura perceptual (luma + crominância reduzidas) do recorte na última presença confirmada
	ausente       bool            // true = fantasma (não está mais na posição vigiada)
	template      []byte          // cópia dos pixels do recorte (RGBA contíguo) p/ busca de perdidos; vazio com o rastreio desligado
}

// estadoVigia é reconstruído por inteiro a cada scan (ver CaptureAndOCR) e mutado apenas pelos
// ticks do vigia (serializados por vigiaMutex). A publicação de mudanças confere, sob a.mu, se o
// estado ainda é o vigente antes de tocar em lastCards.
type estadoVigia struct {
	limites        image.Rectangle // monitor da captura de origem; mudou = estado inválido até o próximo scan
	cards          []cardVigiado
	cursorPerdidos int // rotaciona a busca de perdidos entre ticks (teto por tick)
}


// executarTickVigia roda uma passada completa do vigia: captura a tela, confere a presença dos
// cards ativos, procura os perdidos (se habilitado) e publica as mudanças para o frontend.
func (a *App) executarTickVigia() {
	// Um scan completo está no meio do caminho: o vigia cede a vez (o scan reconstruirá o estado).
	if a.ocrEmAndamento.Load() {
		return
	}
	// Tick anterior ainda rodando (verificações por OCR podem passar de 1s): pula, nunca enfileira.
	if !a.vigiaMutex.TryLock() {
		return
	}
	defer a.vigiaMutex.Unlock()

	a.mu.RLock()
	estado := a.vigia
	a.mu.RUnlock()

	rastrearPerdidos := a.Config.RastrearPalavrasPerdidas
	if estado == nil || !estado.temCardsParaVigiar(rastrearPerdidos) {
		return
	}

	img, limites, regioesCensuradas, err := a.tela.CapturarMonitorCensurado()
	if err != nil {
		fmt.Printf("Aviso: vigia não conseguiu capturar a tela: %v\n", err)
		return
	}
	// Monitor/resolução mudou desde o scan: as posições vigiadas não valem mais; o próximo scan refaz.
	if limites != estado.limites {
		return
	}

	mudou := a.verificarPresencaDosCards(estado, img, regioesCensuradas, rastrearPerdidos)
	if rastrearPerdidos {
		mudou = a.procurarCardsPerdidos(estado, img) || mudou
	}
	if !mudou {
		return
	}
	a.publicarCardsVigiados(estado)
}


// verificarPresencaDosCards compara a assinatura perceptual do recorte de cada card ativo com a da
// captura nova. Dentro da margem de tolerância = presente (ignora ruído de fundo); fora dela é
// AMBÍGUO e só o OCR do recorte decide: texto ainda contém o hanzi → atualiza a referência; não
// contém (outro texto ou nada) → fantasma. Devolve se algum card mudou de estado.
// regioesCensuradas são as áreas pintadas de preto NESTE print (vindas da própria captura): card
// embaixo delas fica com o tick congelado — o preto da censura não prova que a palavra saiu da tela.
func (a *App) verificarPresencaDosCards(estado *estadoVigia, img *image.RGBA, regioesCensuradas []image.Rectangle, guardarTemplates bool) bool {
	verificacoesRestantes := MAXIMO_VERIFICACOES_OCR_POR_TICK
	mudou := false

	for i := range estado.cards {
		c := &estado.cards[i]
		if c.ausente {
			continue
		}
		if intersectaAlguma(c.rect, regioesCensuradas) {
			continue
		}

		assinaturaAtual := assinaturaDeRegiao(img, c.rect)
		if distanciaAssinaturas(assinaturaAtual, c.assinatura) <= TOLERANCIA_ASSINATURA_VIGIA {
			continue // dentro da margem: variação de fundo/anti-aliasing, não é a palavra saindo
		}
		if verificacoesRestantes == 0 {
			continue // teto do tick atingido: o card segue ativo e será reavaliado no próximo
		}
		verificacoesRestantes--

		textoLido, err := a.reconhecerTextoDoRecorte(img, c.rect)
		if err != nil {
			// Sem veredito não se derruba card — e o erro típico (sidecar fora do ar) valeria para
			// todos os recortes, então um aviso único e o tick para por aqui.
			fmt.Printf("Aviso: vigia sem veredito de OCR neste tick: %v\n", err)
			return mudou
		}

		if contemAlgumaForma(textoLido, c.formasHanzi) {
			// O texto segue na tela (fundo/anti-aliasing/tema mudou): só atualiza a referência.
			c.assinatura = assinaturaAtual
			if guardarTemplates {
				c.template = copiarPixelsDaRegiao(img, c.rect)
			}
			continue
		}

		// Outro texto (ou nada) no lugar: nada é feito sobre o texto novo — o card original vira
		// fantasma e fica fora das próximas iterações.
		c.ausente = true
		mudou = true
		fmt.Printf("Vigia: '%s' saiu da posição original (virou fantasma)\n", c.formasHanzi[0])
	}
	return mudou
}


// procurarCardsPerdidos varre a captura atrás do recorte exato de cada card fantasma (até o teto
// por tick, rotacionando). Ao reencontrar, o card reativa na nova posição. Devolve se achou algum.
func (a *App) procurarCardsPerdidos(estado *estadoVigia, img *image.RGBA) bool {
	indicesPerdidos := estado.indicesDosPerdidosComTemplate()
	if len(indicesPerdidos) == 0 {
		return false
	}

	buscas := len(indicesPerdidos)
	if buscas > MAXIMO_BUSCAS_PERDIDOS_POR_TICK {
		buscas = MAXIMO_BUSCAS_PERDIDOS_POR_TICK
	}

	mudou := false
	for n := 0; n < buscas; n++ {
		c := &estado.cards[indicesPerdidos[(estado.cursorPerdidos+n)%len(indicesPerdidos)]]

		ponto, achou := procurarTemplateNaTela(img, c.template, c.rect.Dx(), c.rect.Dy())
		if !achou {
			continue
		}

		delta := ponto.Sub(c.rect.Min)
		c.rect = c.rect.Add(delta)
		c.deslocamento = c.deslocamento.Add(delta)
		c.assinatura = assinaturaDeRegiao(img, c.rect)
		c.ausente = false
		mudou = true
		fmt.Printf("Vigia: '%s' reencontrada na tela (card reativado)\n", c.formasHanzi[0])
	}
	estado.cursorPerdidos += buscas
	return mudou
}


// publicarCardsVigiados aplica o estado do vigia em lastCards (cópia nova, nunca mutação da
// publicada — outros bindings podem estar serializando a antiga) e avisa o frontend.
func (a *App) publicarCardsVigiados(estado *estadoVigia) {
	a.mu.Lock()
	// Um scan completo substituiu o estado durante este tick: as mudanças valem para cards que já
	// não existem, então são descartadas em silêncio.
	if a.vigia != estado {
		a.mu.Unlock()
		return
	}

	cards := make([]FlashcardCard, len(a.lastCards))
	copy(cards, a.lastCards)
	for i := range estado.cards {
		c := &estado.cards[i]
		if c.indice >= len(cards) {
			continue
		}
		cards[c.indice].Fantasma = c.ausente
		if c.deslocamento != (image.Point{}) {
			cards[c.indice].Caixa = deslocarCaixa(c.caixaOriginal, c.deslocamento)
		}
	}
	a.lastCards = cards
	popupsVisiveis := a.popupsTodosVisivel
	a.mu.Unlock()

	if popupsVisiveis {
		a.mostrarTodosPopups()
	}

	if a.ctx != nil {
		runtime.EventsEmit(a.ctx, EVENTO_VIGIA_CARDS_ATUALIZADOS, cards)
	}
}


// registrarCard adiciona um card recém-processado pelo scan ao estado em construção do vigia.
// guardarTemplate copia os pixels do recorte — necessário apenas para o rastreio de perdidos.
func (e *estadoVigia) registrarCard(img *image.RGBA, indice int, formasHanzi []string, caixaCard []float64, guardarTemplate bool) {
	rect, ok := retanguloDoCrop(img.Bounds(), caixaCard)
	if !ok {
		return // sem recorte válido não há o que vigiar (o card em si continua existindo)
	}

	vigiado := cardVigiado{
		indice:        indice,
		formasHanzi:   formasHanzi,
		rect:          rect,
		caixaOriginal: caixaCard,
		assinatura:    assinaturaDeRegiao(img, rect),
	}
	if guardarTemplate {
		vigiado.template = copiarPixelsDaRegiao(img, rect)
	}
	e.cards = append(e.cards, vigiado)
}


// reconhecerTextoDoRecorte envia SÓ o recorte (minúsculo) ao sidecar de OCR e devolve o texto
// concatenado das detecções. Sem downscale: a escala configurada vale para o frame inteiro, e o
// recorte já é pequeno demais para valer a pena reduzir.
func (a *App) reconhecerTextoDoRecorte(img *image.RGBA, rect image.Rectangle) (string, error) {
	recorte, ok := img.SubImage(rect).(*image.RGBA)
	if !ok {
		return "", fmt.Errorf("recorte de tipo inesperado")
	}

	var buf bytes.Buffer
	if err := codificadorPng.Encode(&buf, ampliarParaOcr(recorte)); err != nil {
		return "", err
	}

	resultados, err := a.enviarParaOcr(&buf, 0)
	if err != nil {
		return "", err
	}

	var texto strings.Builder
	for _, res := range resultados {
		texto.WriteString(res.Texto)
	}
	return texto.String(), nil
}


// procurarTemplateNaTela procura o recorte exato (pixels idênticos) pela captura inteira e devolve
// o canto superior esquerdo da primeira ocorrência. A âncora é a LINHA CENTRAL do template — ela
// atravessa os traços do hanzi e quase não gera falsos candidatos (a primeira linha costuma ser
// fundo liso, que casaria em milhares de posições).
func procurarTemplateNaTela(img *image.RGBA, template []byte, largura, altura int) (image.Point, bool) {
	const BYTES_POR_PIXEL = 4
	strideTemplate := largura * BYTES_POR_PIXEL
	if largura <= 0 || altura <= 0 || len(template) != altura*strideTemplate {
		return image.Point{}, false
	}
	limites := img.Bounds()
	if largura > limites.Dx() || altura > limites.Dy() {
		return image.Point{}, false
	}

	linhaAncora := altura / 2
	ancora := template[linhaAncora*strideTemplate : (linhaAncora+1)*strideTemplate]

	for y := limites.Min.Y; y <= limites.Max.Y-altura; y++ {
		linhaTela := img.Pix[img.PixOffset(limites.Min.X, y+linhaAncora):img.PixOffset(limites.Max.X, y+linhaAncora)]

		base := 0
		for base+strideTemplate <= len(linhaTela) {
			pos := bytes.Index(linhaTela[base:], ancora)
			if pos < 0 {
				break
			}
			base += pos
			// O casamento precisa estar alinhado ao início de um pixel (4 bytes); um match
			// deslocado é coincidência de canais e é descartado seguindo a busca 1 byte adiante.
			if base%BYTES_POR_PIXEL != 0 {
				base++
				continue
			}

			x := limites.Min.X + base/BYTES_POR_PIXEL
			if x+largura <= limites.Max.X && regiaoCasaComTemplate(img, template, x, y, altura, strideTemplate) {
				return image.Point{X: x, Y: y}, true
			}
			base += BYTES_POR_PIXEL
		}
	}
	return image.Point{}, false
}


// regiaoCasaComTemplate confere, linha a linha, se a região da captura com canto em (x, y) é
// byte a byte igual ao template.
func regiaoCasaComTemplate(img *image.RGBA, template []byte, x, y, altura, strideTemplate int) bool {
	for linha := 0; linha < altura; linha++ {
		inicio := img.PixOffset(x, y+linha)
		if !bytes.Equal(img.Pix[inicio:inicio+strideTemplate], template[linha*strideTemplate:(linha+1)*strideTemplate]) {
			return false
		}
	}
	return true
}


// ----- Utilitários -----

// formasDoHanzi devolve as grafias aceitas na verificação por OCR do vigia: a forma do card e as
// variantes simplificada/tradicional (o card pode ter sido convertido, mas a tela mantém a original).
func formasDoHanzi(hanzi string, entrada *dicionario.EntradaDicionario) []string {
	formas := []string{hanzi}
	if entrada == nil {
		return formas
	}
	for _, variante := range []string{entrada.Simplificado, entrada.Tradicional} {
		if variante != "" && variante != hanzi {
			formas = append(formas, variante)
		}
	}
	return formas
}


// temCardsParaVigiar diz se algum tick teria trabalho: card ativo para conferir ou, com o rastreio
// ligado, card perdido com template para procurar.
func (e *estadoVigia) temCardsParaVigiar(incluirPerdidos bool) bool {
	for i := range e.cards {
		if !e.cards[i].ausente {
			return true
		}
		if incluirPerdidos && len(e.cards[i].template) > 0 {
			return true
		}
	}
	return false
}


func (e *estadoVigia) indicesDosPerdidosComTemplate() []int {
	var indices []int
	for i := range e.cards {
		if e.cards[i].ausente && len(e.cards[i].template) > 0 {
			indices = append(indices, i)
		}
	}
	return indices
}


// assinaturaDeRegiao reduz a região a uma grade LADO×LADO de luma E crominância (Cb, Cr) média por
// bloco — a base perceptual barata da comparação com margem do vigia (a média do bloco dilui ruído
// fino, mas muda quando o conteúdo do recorte muda de verdade). Luma sozinha não distinguia um texto
// que saiu da tela de um fundo revelado que, por coincidência, tem brilho parecido — a crominância
// cobre esse ponto cego, já que a cor do texto quase sempre difere da cor do fundo por trás dele.
// Devolve os 3 planos concatenados: [0:celulas)=luma, [celulas:2×celulas)=Cb, [2×celulas:3×celulas)=Cr.
func assinaturaDeRegiao(img *image.RGBA, rect image.Rectangle) []byte {
	celulas := LADO_ASSINATURA_VIGIA * LADO_ASSINATURA_VIGIA
	assinatura := make([]byte, celulas*3)
	largura := rect.Dx()
	altura := rect.Dy()
	if largura <= 0 || altura <= 0 {
		return assinatura
	}

	for celY := 0; celY < LADO_ASSINATURA_VIGIA; celY++ {
		y0, y1 := faixaDaCelula(rect.Min.Y, rect.Max.Y, altura, celY)
		for celX := 0; celX < LADO_ASSINATURA_VIGIA; celX++ {
			x0, x1 := faixaDaCelula(rect.Min.X, rect.Max.X, largura, celX)
			indice := celY*LADO_ASSINATURA_VIGIA + celX
			luma, cb, cr := corMediaYCbCr(img, x0, y0, x1, y1)
			assinatura[indice] = luma
			assinatura[celulas+indice] = cb
			assinatura[2*celulas+indice] = cr
		}
	}
	return assinatura
}


// distanciaAssinaturas devolve a MAIOR distância entre os 3 canais (luma, Cb, Cr) — usar o maior em
// vez de somar/misturar preserva a sensibilidade de cada canal isolado: um fundo com o mesmo brilho
// do texto original, só que de outra cor (o caso do texto que rolou e "sumiu" atrás de um fundo
// parecido em luma), ainda estoura pela crominância mesmo com a distância de luma em zero. Tamanhos
// incompatíveis (ou que não dividem em 3 canais iguais) = incomparável = distância máxima (força a
// checagem por OCR).
func distanciaAssinaturas(atual, referencia []byte) int {
	if len(atual) == 0 || len(atual) != len(referencia) || len(atual)%3 != 0 {
		return 255
	}

	celulas := len(atual) / 3
	distanciaLuma := distanciaCanalNormalizada(atual[:celulas], referencia[:celulas])
	distanciaCb := distanciaCanalNormalizada(atual[celulas:2*celulas], referencia[celulas:2*celulas])
	distanciaCr := distanciaCanalNormalizada(atual[2*celulas:], referencia[2*celulas:])

	maior := distanciaLuma
	if distanciaCb > maior {
		maior = distanciaCb
	}
	if distanciaCr > maior {
		maior = distanciaCr
	}
	return maior
}


// distanciaCanalNormalizada devolve a distância média por amostra (0–255) entre dois planos do MESMO
// canal (dois planos de luma, ou dois de Cb, ou dois de Cr), DEPOIS de descontar a média de cada um —
// assim uma variação uniforme do canal inteiro (fade de brilho, filtro de cor) fica perto de zero, e
// só mudança de ESTRUTURA (texto que sai/troca) sobe a distância. Núcleo repetido pelos 3 canais em
// distanciaAssinaturas.
func distanciaCanalNormalizada(atual, referencia []byte) int {
	if len(atual) == 0 || len(atual) != len(referencia) {
		return 255
	}

	mediaAtual := mediaBytes(atual)
	mediaRef := mediaBytes(referencia)
	soma := 0
	for i := range atual {
		d := (int(atual[i]) - mediaAtual) - (int(referencia[i]) - mediaRef)
		if d < 0 {
			d = -d
		}
		soma += d
	}
	return soma / len(atual)
}


// ampliarParaOcr amplia por vizinho-mais-próximo (fator inteiro) recortes cujo menor lado é menor
// que MINIMO_LADO_OCR_VIGIA; recortes já grandes voltam intactos. Bordas nítidas preservadas, sem
// dependência externa — o objetivo é só dar pixels suficientes para o detector do sidecar.
func ampliarParaOcr(origem *image.RGBA) image.Image {
	limites := origem.Bounds()
	menorLado := limites.Dx()
	if limites.Dy() < menorLado {
		menorLado = limites.Dy()
	}
	if menorLado <= 0 || menorLado >= MINIMO_LADO_OCR_VIGIA {
		return origem
	}

	fator := (MINIMO_LADO_OCR_VIGIA + menorLado - 1) / menorLado
	destino := image.NewRGBA(image.Rect(0, 0, limites.Dx()*fator, limites.Dy()*fator))
	for y := 0; y < destino.Rect.Dy(); y++ {
		origemY := limites.Min.Y + y/fator
		for x := 0; x < destino.Rect.Dx(); x++ {
			destino.SetRGBA(x, y, origem.RGBAAt(limites.Min.X+x/fator, origemY))
		}
	}
	return destino
}


// faixaDaCelula devolve o intervalo [ini, fim) de pixels de uma célula da grade da assinatura,
// garantindo pelo menos 1 pixel e sem estourar o limite (recorte menor que a grade tem células de 1px).
func faixaDaCelula(minimo, maximo, tamanho, indice int) (int, int) {
	ini := minimo + indice*tamanho/LADO_ASSINATURA_VIGIA
	if ini >= maximo {
		ini = maximo - 1
	}
	fim := minimo + (indice+1)*tamanho/LADO_ASSINATURA_VIGIA
	if fim <= ini {
		fim = ini + 1
	}
	if fim > maximo {
		fim = maximo
	}
	return ini, fim
}


// corMediaYCbCr devolve a luma média (Rec. 601 aproximada) e a crominância média (Cb, Cr) dos
// pixels de um retângulo da captura, num único passe. Os pesos de Cb/Cr somam zero por canal —
// R=G=B (cinza) sempre dá Cb=Cr=128, e um deslocamento uniforme de R, G e B (fade de luz) não move
// nem Cb nem Cr, só a luma — mantendo a mesma tolerância a ruído de fundo que a luma já tinha.
func corMediaYCbCr(img *image.RGBA, x0, y0, x1, y1 int) (luma, cb, cr byte) {
	somaY, somaCb, somaCr, n := 0, 0, 0, 0
	for y := y0; y < y1; y++ {
		for x := x0; x < x1; x++ {
			i := img.PixOffset(x, y)
			r, g, b := int(img.Pix[i]), int(img.Pix[i+1]), int(img.Pix[i+2])
			somaY += (r*77 + g*150 + b*29) >> 8
			somaCb += 128 + ((-r*43 - g*85 + b*128) >> 8)
			somaCr += 128 + ((r*128 - g*107 - b*21) >> 8)
			n++
		}
	}
	if n == 0 {
		return 0, 128, 128
	}
	return byte(somaY / n), byte(somaCb / n), byte(somaCr / n)
}


func mediaBytes(valores []byte) int {
	if len(valores) == 0 {
		return 0
	}
	soma := 0
	for _, v := range valores {
		soma += int(v)
	}
	return soma / len(valores)
}


// copiarPixelsDaRegiao devolve uma cópia contígua (stride = largura×4) dos pixels da região.
func copiarPixelsDaRegiao(img *image.RGBA, rect image.Rectangle) []byte {
	strideRegiao := rect.Dx() * 4
	pixels := make([]byte, 0, rect.Dy()*strideRegiao)
	for y := rect.Min.Y; y < rect.Max.Y; y++ {
		inicio := img.PixOffset(rect.Min.X, y)
		pixels = append(pixels, img.Pix[inicio:inicio+strideRegiao]...)
	}
	return pixels
}


func contemAlgumaForma(texto string, formas []string) bool {
	for _, forma := range formas {
		if strings.Contains(texto, forma) {
			return true
		}
	}
	return false
}


func intersectaAlguma(rect image.Rectangle, regioes []image.Rectangle) bool {
	for _, regiao := range regioes {
		if rect.Overlaps(regiao) {
			return true
		}
	}
	return false
}


func deslocarCaixa(caixa []float64, deslocamento image.Point) []float64 {
	if len(caixa) != 4 {
		return caixa
	}
	return []float64{
		caixa[0] + float64(deslocamento.X),
		caixa[1] + float64(deslocamento.Y),
		caixa[2] + float64(deslocamento.X),
		caixa[3] + float64(deslocamento.Y),
	}
}
