package main

import "testing"

// ----- Rastreio de visualizações de OCR por scan -----

// simularScan cria o rastreador do scan seguinte a partir das fingerprints do anterior e registra
// as ocorrências (palavra → crops em base64) na ordem dada.
func simularScan(fingerprintsScanAnterior map[string]map[uint64]bool, ocorrencias [][2]string) *rastreadorVisualizacoesOcr {
	rastreador := novoRastreadorVisualizacoesOcr(fingerprintsScanAnterior)
	for _, ocorrencia := range ocorrencias {
		rastreador.registrarOcorrencia(ocorrencia[0], ocorrencia[1])
	}
	return rastreador
}


func TestVisualizacaoContaSoUmaVezPorScan(t *testing.T) {
	// Caso de sucesso: primeira aparição conta; duas ocorrências novas da mesma palavra = +1 só.
	rastreador := simularScan(nil, [][2]string{
		{"猫", "cropCatA"},
		{"猫", "cropCatB"},
		{"狗", "cropDog"},
	})

	palavras := rastreador.palavrasComVisualizacaoNova()
	if len(palavras) != 2 || palavras[0] != "猫" || palavras[1] != "狗" {
		t.Fatalf("esperava [猫 狗] (cada palavra no máximo 1× por scan), veio %v", palavras)
	}
}


func TestPalavraParadaNaTelaNaoConta(t *testing.T) {
	scanUm := simularScan(nil, [][2]string{{"猫", "cropParado"}, {"狗", "cropDog"}})

	// 猫 segue com o MESMO recorte (parada na tela); 狗 sumiu; 鸟 é inédita.
	scanDois := simularScan(scanUm.fingerprintsDoScan(), [][2]string{
		{"猫", "cropParado"},
		{"鸟", "cropBird"},
	})

	palavras := scanDois.palavrasComVisualizacaoNova()
	if len(palavras) != 1 || palavras[0] != "鸟" {
		t.Fatalf("esperava só [鸟] (recorte de 猫 não mudou), veio %v", palavras)
	}

	// A palavra parada continua não contando em cadeias longas: o scan 2 preserva a fingerprint
	// dela mesmo sem tê-la contado, então o scan 3 ainda a reconhece como parada.
	scanTres := simularScan(scanDois.fingerprintsDoScan(), [][2]string{{"猫", "cropParado"}})
	if palavras := scanTres.palavrasComVisualizacaoNova(); len(palavras) != 0 {
		t.Fatalf("esperava nenhuma visualização no scan 3 (猫 parada há dois scans), veio %v", palavras)
	}
}


func TestRecorteNovoDaMesmaPalavraConta(t *testing.T) {
	scanUm := simularScan(nil, [][2]string{{"猫", "cropParado"}})

	// Uma ocorrência parada + uma inédita da mesma palavra: a inédita garante o +1.
	scanDois := simularScan(scanUm.fingerprintsDoScan(), [][2]string{
		{"猫", "cropParado"},
		{"猫", "cropNovoLugar"},
	})

	palavras := scanDois.palavrasComVisualizacaoNova()
	if len(palavras) != 1 || palavras[0] != "猫" {
		t.Fatalf("esperava [猫] (ganhou ocorrência com recorte inédito), veio %v", palavras)
	}
}


func TestPalavraQueReapareceConta(t *testing.T) {
	scanUm := simularScan(nil, [][2]string{{"猫", "cropCat"}})
	scanDois := simularScan(scanUm.fingerprintsDoScan(), nil) // 猫 saiu da tela

	// 猫 volta com o mesmo visual: só o scan ANTERIOR importa, então conta de novo.
	scanTres := simularScan(scanDois.fingerprintsDoScan(), [][2]string{{"猫", "cropCat"}})
	if palavras := scanTres.palavrasComVisualizacaoNova(); len(palavras) != 1 || palavras[0] != "猫" {
		t.Fatalf("esperava [猫] (sumiu e reapareceu), veio %v", palavras)
	}
}


func TestCropVazioSempreConta(t *testing.T) {
	// Caso de borda: sem crop não há fingerprint para comparar — contar preserva a sugestão de estudo.
	scanUm := simularScan(nil, [][2]string{{"猫", ""}})
	scanDois := simularScan(scanUm.fingerprintsDoScan(), [][2]string{{"猫", ""}})

	if palavras := scanDois.palavrasComVisualizacaoNova(); len(palavras) != 1 || palavras[0] != "猫" {
		t.Fatalf("esperava [猫] (crop vazio conta sempre), veio %v", palavras)
	}
}
