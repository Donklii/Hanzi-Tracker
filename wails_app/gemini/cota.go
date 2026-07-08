package gemini

import (
	"time"

	"wails_app/cota"
)

// ----- Cota diária de requisições do Gemini -----
// Contador sob a chave "gemini" do cotas.json unificado, zerado a cada dia. A mecânica (load,
// reset por período, save, migração do cota_gemini.json antigo) vive no pacote cota, compartilhado
// com a cota de tradução. A cota NUNCA é apagada por limpeza de armazenamento: ela contabiliza
// consumo já feito na API externa neste período.

var contador = cota.NovoContador("gemini",
	func() string { return time.Now().Format("2006-01-02") },
	&cota.MigracaoAntiga{NomeArquivo: "cota_gemini.json", CampoPeriodo: "data", CampoUsado: "requisicoesUsadas"})

// RegistrarRequisicao incrementa o contador de requisições usadas e persiste. Chamar SÓ após
// chamada de API bem-sucedida.
func RegistrarRequisicao() error {
	return contador.Registrar(1)
}

// CotaExcedida verifica se o uso atual já atingiu ou ultrapassou o limite configurado.
func CotaExcedida(limiteRequisicoes int) bool {
	_, usadas := contador.Carregar()
	return usadas >= limiteRequisicoes
}

// InfoCotaParaUI devolve dados formatados para exibição na UI (thread-safe).
func InfoCotaParaUI() (requisicoesUsadas int, data string) {
	data, requisicoesUsadas = contador.Carregar()
	return requisicoesUsadas, data
}
