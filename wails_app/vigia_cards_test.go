package main

import (
	"image"
	"image/color"
	"testing"

	"wails_app/dicionario"
)

// ----- Vigia de highlights fantasmas -----

func TestProcurarTemplateAchaRecorteDeslocado(t *testing.T) {
	// Caso de sucesso: o recorte de uma palavra que "rolou" para outra posição é reencontrado.
	origem := image.NewRGBA(image.Rect(0, 0, 60, 40))
	rectOriginal := image.Rect(5, 3, 13, 9)
	pintarPadrao(origem, rectOriginal, 100)
	template := copiarPixelsDaRegiao(origem, rectOriginal)

	destino := image.NewRGBA(image.Rect(0, 0, 60, 40))
	pintarPadrao(destino, image.Rect(30, 20, 38, 26), 100)

	ponto, achou := procurarTemplateNaTela(destino, template, rectOriginal.Dx(), rectOriginal.Dy())
	if !achou || ponto != image.Pt(30, 20) {
		t.Fatalf("esperava achar o recorte em (30,20), veio achou=%v ponto=%v", achou, ponto)
	}
}


func TestProcurarTemplateNaoAchaRecorteAusente(t *testing.T) {
	origem := image.NewRGBA(image.Rect(0, 0, 60, 40))
	rectOriginal := image.Rect(5, 3, 13, 9)
	pintarPadrao(origem, rectOriginal, 100)
	template := copiarPixelsDaRegiao(origem, rectOriginal)

	// A tela nova tem OUTRO conteúdo na mesma região (semente diferente) e nada igual ao template.
	destino := image.NewRGBA(image.Rect(0, 0, 60, 40))
	pintarPadrao(destino, rectOriginal, 200)

	if _, achou := procurarTemplateNaTela(destino, template, rectOriginal.Dx(), rectOriginal.Dy()); achou {
		t.Fatalf("não podia achar um recorte que não está mais na tela")
	}
}


func TestPublicarCardsVigiadosMarcaFantasmaEhDeslocaCaixa(t *testing.T) {
	estado := &estadoVigia{
		cards: []cardVigiado{
			{indice: 0, ausente: true, caixaOriginal: []float64{10, 10, 30, 20}},
			{indice: 1, ausente: false, caixaOriginal: []float64{50, 10, 80, 20}, deslocamento: image.Pt(0, 15)},
		},
	}
	a := &App{}
	a.vigia = estado
	a.lastCards = []FlashcardCard{
		{Hanzi: "猫", Caixa: []float64{10, 10, 30, 20}},
		{Hanzi: "狗", Caixa: []float64{50, 10, 80, 20}},
	}

	a.publicarCardsVigiados(estado)

	cards := a.GetLastCards()
	if !cards[0].Fantasma {
		t.Fatalf("esperava o card 0 (texto sumiu) marcado como fantasma")
	}
	if cards[1].Fantasma {
		t.Fatalf("o card 1 continua na tela, não podia virar fantasma")
	}
	esperada := []float64{50, 25, 80, 35}
	for i := range esperada {
		if cards[1].Caixa[i] != esperada[i] {
			t.Fatalf("esperava caixa deslocada %v, veio %v", esperada, cards[1].Caixa)
		}
	}
}


func TestPublicarCardsVigiadosDescartaEstadoSubstituido(t *testing.T) {
	// Caso de borda: um scan completo trocou o estado no meio do tick — as mudanças do tick antigo
	// valem para cards que já não existem e são descartadas.
	estadoAntigo := &estadoVigia{cards: []cardVigiado{{indice: 0, ausente: true, caixaOriginal: []float64{0, 0, 1, 1}}}}
	a := &App{}
	a.vigia = &estadoVigia{}
	a.lastCards = []FlashcardCard{{Hanzi: "猫"}}

	a.publicarCardsVigiados(estadoAntigo)

	if a.GetLastCards()[0].Fantasma {
		t.Fatalf("tick de um estado substituído não podia marcar fantasma nos cards novos")
	}
}


func TestVerificarPresencaDentroDaMargemNaoMudaNada(t *testing.T) {
	// Caminho comum do tick: a tela não mudou (distância zero, dentro da margem) — nenhum card muda
	// de estado e o OCR nem é consultado.
	rect := image.Rect(4, 3, 36, 27)
	img := image.NewRGBA(image.Rect(0, 0, 40, 30))
	pintarMetades(img, rect, 20, 220)

	estado := &estadoVigia{cards: []cardVigiado{{
		indice:      0,
		formasHanzi: []string{"猫"},
		rect:        rect,
		assinatura:  assinaturaDeRegiao(img, rect),
	}}}

	a := &App{}
	mudou := a.verificarPresencaDosCards(estado, img, nil, false)
	if mudou || estado.cards[0].ausente {
		t.Fatalf("tela inalterada não podia mudar o card (mudou=%v ausente=%v)", mudou, estado.cards[0].ausente)
	}
}


func TestVerificarPresencaCongelaCardSobRegiaoCensurada(t *testing.T) {
	// O conteúdo do recorte mudou RADICALMENTE (texto → preto), mas a região foi censurada NESTE
	// print (janela do app por cima): o tick do card congela — o preto da censura não prova que a
	// palavra saiu da tela, então nada de OCR e nada de fantasma.
	rect := image.Rect(4, 3, 36, 27)
	original := image.NewRGBA(image.Rect(0, 0, 40, 30))
	pintarMetades(original, rect, 20, 220)

	estado := &estadoVigia{cards: []cardVigiado{{
		indice:      0,
		formasHanzi: []string{"猫"},
		rect:        rect,
		assinatura:  assinaturaDeRegiao(original, rect),
	}}}

	capturaCensurada := image.NewRGBA(image.Rect(0, 0, 40, 30))
	pintarCinza(capturaCensurada, capturaCensurada.Bounds(), 0)

	a := &App{}
	mudou := a.verificarPresencaDosCards(estado, capturaCensurada, []image.Rectangle{capturaCensurada.Bounds()}, false)
	if mudou || estado.cards[0].ausente {
		t.Fatalf("card sob região censurada não podia mudar de estado (mudou=%v ausente=%v)", mudou, estado.cards[0].ausente)
	}
}


func TestAssinaturaMesmaRegiaoDistanciaZero(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 40, 30))
	pintarMetades(img, image.Rect(5, 4, 35, 26), 40, 200)
	rect := image.Rect(5, 4, 35, 26)

	if d := distanciaAssinaturas(assinaturaDeRegiao(img, rect), assinaturaDeRegiao(img, rect)); d != 0 {
		t.Fatalf("a mesma região tinha que dar distância 0, veio %d", d)
	}
}


func TestAssinaturaToleraMudancaDeBrilhoDeFundo(t *testing.T) {
	// Caso central da margem: o fundo escurece/clareia por inteiro (fade, dia/noite) mas a estrutura
	// é a mesma — a distância tem que ficar DENTRO da tolerância e NÃO acionar o OCR.
	rect := image.Rect(4, 3, 36, 27)
	base := image.NewRGBA(image.Rect(0, 0, 40, 30))
	pintarMetades(base, rect, 40, 200)

	claro := image.NewRGBA(image.Rect(0, 0, 40, 30))
	copy(claro.Pix, base.Pix)
	somarBrilho(claro, rect, 15)

	if d := distanciaAssinaturas(assinaturaDeRegiao(base, rect), assinaturaDeRegiao(claro, rect)); d > TOLERANCIA_ASSINATURA_VIGIA {
		t.Fatalf("+15 de brilho uniforme devia caber na margem (≤%d), veio %d", TOLERANCIA_ASSINATURA_VIGIA, d)
	}
}


func TestAssinaturaDetectaEstruturaDeCorComLumaIdentica(t *testing.T) {
	// vermelho e verde têm A MESMA luma (45) — só o vermelho vira verde e vice-versa entre as
	// metades. Se a distância estourar mesmo com o canal de luma perfeitamente igual célula a
	// célula, é a CROMINÂNCIA sozinha pegando a mudança — o caso relatado: um texto que rolou e
	// "sumiu" atrás de um fundo com brilho parecido, mas cor diferente, escapava da margem antiga.
	rect := image.Rect(4, 3, 36, 27)
	vermelho := color.RGBA{R: 150, G: 0, B: 0, A: 255}
	verde := color.RGBA{R: 0, G: 77, B: 0, A: 255}

	referencia := image.NewRGBA(image.Rect(0, 0, 40, 30))
	pintarMetadesColoridas(referencia, rect, vermelho, verde)

	atual := image.NewRGBA(image.Rect(0, 0, 40, 30))
	pintarMetadesColoridas(atual, rect, verde, vermelho) // metades trocadas: luma de cada célula não muda

	if d := distanciaAssinaturas(assinaturaDeRegiao(referencia, rect), assinaturaDeRegiao(atual, rect)); d <= TOLERANCIA_ASSINATURA_VIGIA {
		t.Fatalf("cores trocadas (luma idêntica) tinham que estourar a margem pela crominância (>%d), veio %d", TOLERANCIA_ASSINATURA_VIGIA, d)
	}
}


func TestAssinaturaToleraMudancaUniformeDeCorDeFundo(t *testing.T) {
	// Mesmo caso de TestAssinaturaToleraMudancaDeBrilhoDeFundo, mas com um recorte colorido: um
	// deslocamento uniforme de R, G e B (fade de luz, filtro de tela) não pode acionar o OCR — só
	// mudança de ESTRUTURA de cor deve (a crominância não pode ficar mais sensível que a luma já era).
	rect := image.Rect(4, 3, 36, 27)
	vermelho := color.RGBA{R: 150, G: 0, B: 0, A: 255}
	verde := color.RGBA{R: 0, G: 77, B: 0, A: 255}

	base := image.NewRGBA(image.Rect(0, 0, 40, 30))
	pintarMetadesColoridas(base, rect, vermelho, verde)

	claro := image.NewRGBA(image.Rect(0, 0, 40, 30))
	copy(claro.Pix, base.Pix)
	somarBrilho(claro, rect, 15)

	if d := distanciaAssinaturas(assinaturaDeRegiao(base, rect), assinaturaDeRegiao(claro, rect)); d > TOLERANCIA_ASSINATURA_VIGIA {
		t.Fatalf("deslocamento uniforme de cor devia caber na margem (≤%d), veio %d", TOLERANCIA_ASSINATURA_VIGIA, d)
	}
}


func TestAssinaturaDetectaTextoQueSai(t *testing.T) {
	// A palavra (recorte estruturado, alto contraste) sai e o lugar vira fundo liso: a distância
	// tem que ESTOURAR a margem para o vigia ir ao OCR confirmar o fantasma.
	rect := image.Rect(4, 3, 36, 27)
	comTexto := image.NewRGBA(image.Rect(0, 0, 40, 30))
	pintarMetades(comTexto, rect, 20, 220)

	semTexto := image.NewRGBA(image.Rect(0, 0, 40, 30))
	pintarCinza(semTexto, rect, 120)

	if d := distanciaAssinaturas(assinaturaDeRegiao(comTexto, rect), assinaturaDeRegiao(semTexto, rect)); d <= TOLERANCIA_ASSINATURA_VIGIA {
		t.Fatalf("texto sumindo (estrutura → fundo liso) tinha que passar da margem (>%d), veio %d", TOLERANCIA_ASSINATURA_VIGIA, d)
	}
}


func TestRetanguloDoCropAplicaRespiroEhValida(t *testing.T) {
	limites := image.Rect(0, 0, 100, 100)

	rect, ok := retanguloDoCrop(limites, []float64{20, 20, 40, 30})
	if !ok || rect != image.Rect(10, 10, 50, 40) {
		t.Fatalf("esperava recorte com respiro (10,10)-(50,40), veio ok=%v rect=%v", ok, rect)
	}

	// Casos de borda: caixa malformada e caixa totalmente fora da imagem.
	if _, ok := retanguloDoCrop(limites, []float64{20, 20}); ok {
		t.Fatalf("caixa malformada não podia gerar recorte")
	}
	if _, ok := retanguloDoCrop(limites, []float64{-500, -500, -400, -450}); ok {
		t.Fatalf("caixa fora da imagem não podia gerar recorte")
	}
}


func TestRegistrarCardGuardaTemplateSoComRastreioLigado(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 50, 30))
	pintarPadrao(img, img.Bounds(), 3)
	caixa := []float64{15, 12, 25, 18}

	semRastreio := &estadoVigia{}
	semRastreio.registrarCard(img, 0, []string{"猫"}, caixa, false)
	comRastreio := &estadoVigia{}
	comRastreio.registrarCard(img, 0, []string{"猫"}, caixa, true)

	if len(semRastreio.cards) != 1 || len(semRastreio.cards[0].template) != 0 {
		t.Fatalf("sem o rastreio de perdidos o template não devia ser copiado")
	}
	rect := comRastreio.cards[0].rect
	if esperado := rect.Dx() * rect.Dy() * 4; len(comRastreio.cards[0].template) != esperado {
		t.Fatalf("template devia ter %d bytes (recorte inteiro), veio %d", esperado, len(comRastreio.cards[0].template))
	}
}


func TestFormasDoHanziIncluiVariantes(t *testing.T) {
	entrada := &dicionario.EntradaDicionario{Simplificado: "马", Tradicional: "馬"}
	formas := formasDoHanzi("马", entrada)
	if len(formas) != 2 || formas[0] != "马" || formas[1] != "馬" {
		t.Fatalf("esperava [马 馬], veio %v", formas)
	}
	if !contemAlgumaForma("小馬過河", formas) {
		t.Fatalf("texto com a variante tradicional tinha que casar")
	}
	if contemAlgumaForma("小狗", formas) {
		t.Fatalf("texto sem nenhuma forma não podia casar")
	}
}


// ----- Utilitários -----

// pintarMetades pinta a metade esquerda com `escuro` e a direita com `claro` (todos os canais
// iguais): estrutura de alto contraste que sobrevive ao downscale da assinatura.
func pintarMetades(img *image.RGBA, rect image.Rectangle, escuro, claro byte) {
	meio := rect.Min.X + rect.Dx()/2
	for y := rect.Min.Y; y < rect.Max.Y; y++ {
		for x := rect.Min.X; x < rect.Max.X; x++ {
			i := img.PixOffset(x, y)
			valor := escuro
			if x >= meio {
				valor = claro
			}
			img.Pix[i], img.Pix[i+1], img.Pix[i+2], img.Pix[i+3] = valor, valor, valor, 255
		}
	}
}


// pintarMetadesColoridas pinta a metade esquerda com `corEsquerda` e a direita com `corDireita`
// (RGBA completo, não necessariamente cinza) — usada para testar a crominância da assinatura.
func pintarMetadesColoridas(img *image.RGBA, rect image.Rectangle, corEsquerda, corDireita color.RGBA) {
	meio := rect.Min.X + rect.Dx()/2
	for y := rect.Min.Y; y < rect.Max.Y; y++ {
		for x := rect.Min.X; x < rect.Max.X; x++ {
			cor := corEsquerda
			if x >= meio {
				cor = corDireita
			}
			img.SetRGBA(x, y, cor)
		}
	}
}


// pintarCinza preenche a região com um tom de cinza uniforme (fundo liso, sem estrutura).
func pintarCinza(img *image.RGBA, rect image.Rectangle, valor byte) {
	for y := rect.Min.Y; y < rect.Max.Y; y++ {
		for x := rect.Min.X; x < rect.Max.X; x++ {
			i := img.PixOffset(x, y)
			img.Pix[i], img.Pix[i+1], img.Pix[i+2], img.Pix[i+3] = valor, valor, valor, 255
		}
	}
}


// somarBrilho soma `delta` (com saturação em 255) aos canais de cor da região — simula o fundo
// clareando/escurecendo por inteiro sem mexer na estrutura.
func somarBrilho(img *image.RGBA, rect image.Rectangle, delta int) {
	for y := rect.Min.Y; y < rect.Max.Y; y++ {
		for x := rect.Min.X; x < rect.Max.X; x++ {
			i := img.PixOffset(x, y)
			for canal := 0; canal < 3; canal++ {
				valor := int(img.Pix[i+canal]) + delta
				if valor > 255 {
					valor = 255
				}
				img.Pix[i+canal] = byte(valor)
			}
		}
	}
}


// pintarPadrao pinta a região com um padrão determinístico RELATIVO ao canto dela: o mesmo padrão
// pintado em outra posição produz exatamente os mesmos bytes (essencial para simular texto que rolou).
func pintarPadrao(img *image.RGBA, rect image.Rectangle, semente byte) {
	for y := rect.Min.Y; y < rect.Max.Y; y++ {
		for x := rect.Min.X; x < rect.Max.X; x++ {
			i := img.PixOffset(x, y)
			localX := byte(x - rect.Min.X)
			localY := byte(y - rect.Min.Y)
			img.Pix[i] = semente + localX*7 + localY*13
			img.Pix[i+1] = semente ^ (localX*3 + localY*5)
			img.Pix[i+2] = semente + localX + localY
			img.Pix[i+3] = 255
		}
	}
}
