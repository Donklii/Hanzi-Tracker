package main

// ----- Seção: Rastreio de visualizações de OCR por scan -----
//
// Uma palavra só ganha +1 visualização quando ao menos uma das suas ocorrências no scan tem um
// crop (recorte da captura no local da palavra) com fingerprint diferente de TODOS os crops da
// mesma palavra no scan anterior. Palavra que persiste parada na tela produz o mesmo recorte a
// cada scan e por isso não infla o contador, mesmo que o resto da tela mude (o atalho de hash do
// frame inteiro em CaptureAndOCR não a protege nesse caso).

import "hash/fnv"

// rastreadorVisualizacoesOcr acumula as ocorrências de um scan e decide quais palavras ganham
// visualização. Cada palavra conta no máximo 1× por scan.
type rastreadorVisualizacoesOcr struct {
	fingerprintsAnteriores map[string]map[uint64]bool // palavra → fingerprints dos crops do scan anterior
	fingerprintsAtuais     map[string]map[uint64]bool // palavra → fingerprints dos crops deste scan
	palavrasContadas       map[string]bool
	palavras               []string
}


func novoRastreadorVisualizacoesOcr(fingerprintsScanAnterior map[string]map[uint64]bool) *rastreadorVisualizacoesOcr {
	return &rastreadorVisualizacoesOcr{
		fingerprintsAnteriores: fingerprintsScanAnterior,
		fingerprintsAtuais:     make(map[string]map[uint64]bool),
		palavrasContadas:       make(map[string]bool),
	}
}


// registrarOcorrencia acumula a fingerprint do crop desta ocorrência e conta a palavra caso o
// recorte seja inédito frente ao scan anterior. Crop vazio (caixa inválida ou codificação
// falhou) conta como visualização: sem fingerprint não dá para provar que é a mesma imagem, e
// subcontar esconderia a palavra da sugestão de estudo.
func (r *rastreadorVisualizacoesOcr) registrarOcorrencia(palavra string, cropBase64 string) {
	temCrop := cropBase64 != ""

	var fingerprint uint64
	if temCrop {
		fingerprint = fingerprintDeCrop(cropBase64)
		if r.fingerprintsAtuais[palavra] == nil {
			r.fingerprintsAtuais[palavra] = make(map[uint64]bool)
		}
		r.fingerprintsAtuais[palavra][fingerprint] = true
	}

	if r.palavrasContadas[palavra] {
		return
	}
	if temCrop && r.fingerprintsAnteriores[palavra][fingerprint] {
		return // recorte idêntico ao do scan anterior: palavra parada na tela, não é visualização nova
	}

	r.palavrasContadas[palavra] = true
	r.palavras = append(r.palavras, palavra)
}


// palavrasComVisualizacaoNova devolve as palavras deste scan que devem ganhar +1 visualização.
func (r *rastreadorVisualizacoesOcr) palavrasComVisualizacaoNova() []string {
	return r.palavras
}


// fingerprintsDoScan devolve o mapa palavra → fingerprints deste scan, para virar o "scan
// anterior" do próximo. Inclui também os crops de palavras NÃO contadas: uma palavra parada
// precisa seguir reconhecível como parada nos scans seguintes.
func (r *rastreadorVisualizacoesOcr) fingerprintsDoScan() map[string]map[uint64]bool {
	return r.fingerprintsAtuais
}


// fingerprintDeCrop resume o PNG-base64 do recorte num hash FNV-1a: recortes de mesmo conteúdo
// geram o mesmo PNG (encoder determinístico) e portanto a mesma fingerprint.
func fingerprintDeCrop(cropBase64 string) uint64 {
	hasher := fnv.New64a()
	_, _ = hasher.Write([]byte(cropBase64))
	return hasher.Sum64()
}
