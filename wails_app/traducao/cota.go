package traducao

import (
	"time"

	"wails_app/cota"
)

// ----- Cota mensal de caracteres da tradução -----
// Contador sob a chave "traducao" do cotas.json unificado (separado do Config principal para não
// disputar o configuracoes.json com o frontend), zerado a cada mês. A mecânica (load, reset por
// período, save, migração do cota_traducao.json antigo) vive no pacote cota, compartilhado com a
// cota do Gemini. A cota NUNCA é apagada por limpeza de armazenamento: ela contabiliza consumo já
// feito na API externa neste período.

// CotaGratuitaCaracteresMes é o free tier documentado pelo Google (Cloud Translation API v2);
// só a BASE do %, ajustável se o preço mudar — não precisa de migração de dados.
const CotaGratuitaCaracteresMes = 500_000

var contador = cota.NovoContador("traducao",
	func() string { return time.Now().Format("2006-01") },
	&cota.MigracaoAntiga{NomeArquivo: "cota_traducao.json", CampoPeriodo: "anoMes", CampoUsado: "caracteresUsados"})

// RegistrarUso incrementa o contador de caracteres usados e persiste. Chamar SÓ após chamada
// de API bem-sucedida (nunca em cache hit).
func RegistrarUso(caracteres int) error {
	return contador.Registrar(caracteres)
}

// CotaExcedida verifica se o uso atual já atingiu ou ultrapassou o limite percentual configurado.
func CotaExcedida(limitePct float64) bool {
	_, usados := contador.Carregar()
	return percentUsado(usados) >= limitePct
}

// InfoCotaParaUI devolve dados formatados para exibição na UI (thread-safe).
func InfoCotaParaUI() (caracteresUsados int, cotaTotal int, percentual float64, anoMes string) {
	anoMes, caracteresUsados = contador.Carregar()
	return caracteresUsados, CotaGratuitaCaracteresMes, percentUsado(caracteresUsados), anoMes
}

// percentUsado calcula a porcentagem do free tier já consumida.
func percentUsado(caracteresUsados int) float64 {
	return float64(caracteresUsados) / float64(CotaGratuitaCaracteresMes) * 100
}
