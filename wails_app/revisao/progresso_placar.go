package revisao

import (
	"wails_app/progresso"
)

// ----- Seção: Progresso por palavra para o placar da revisão -----
// O placar final compara o progresso de cada palavra praticada ANTES e DEPOIS da sessão: a
// AbaRevisao chama ProgressoPalavras ao montar a sessão (retrato inicial) e de novo ao exibir o
// placar, e destaca por área o que avançou. O streak do desenho segue a mesma regra híbrida de
// aprendizadoConcluido: numa palavra multi-hanzi ele é o MENOR streak entre os hanzis componentes.

// ProgressoPalavraRevisao é o retrato do avanço de UMA palavra nas 5 áreas de aprendizado.
type ProgressoPalavraRevisao struct {
	Hanzi        string   `json:"hanzi"`
	Pinyin       string   `json:"pinyin"`
	Significados []string `json:"significados"`

	// Status do vocabulário ("visto", "estudo", "aprendido"; vazio se a palavra não está no banco).
	Status string `json:"status"`

	// Streaks é o acerto consecutivo atual por área (significado, fonetica, desenho, contexto,
	// pronuncia). Área concluída = streak >= Meta do retrato.
	Streaks map[string]int `json:"streaks"`
}

// ProgressoRevisaoPalavras agrupa os retratos com a meta vigente, numa única travessia da ponte.
type ProgressoRevisaoPalavras struct {
	// Meta é o número de acertos consecutivos que conclui uma área (MetaAcertosConsecutivos).
	Meta     int                       `json:"meta"`
	Palavras []ProgressoPalavraRevisao `json:"palavras"`
}

// ProgressoPalavras devolve o retrato de progresso das palavras informadas (dedup, preservando a
// ordem). Palavras fora do banco entram com streaks zerados — a sessão pode tê-las apresentado
// antes de qualquer registro.
func (r *GerenciadorRevisao) ProgressoPalavras(palavras []string) (ProgressoRevisaoPalavras, error) {
	retrato := ProgressoRevisaoPalavras{Meta: MetaAcertosConsecutivos}

	vocabulario, err := progresso.GetAllVocab()
	if err != nil {
		return retrato, err
	}
	statusPorHanzi := make(map[string]string, len(vocabulario))
	for _, v := range vocabulario {
		statusPorHanzi[v.Hanzi] = v.Status
	}

	vistos := make(map[string]bool, len(palavras))
	for _, palavra := range palavras {
		if palavra == "" || vistos[palavra] {
			continue
		}
		vistos[palavra] = true

		estatisticas, err := progresso.ObterEstatisticasPalavra(palavra)
		if err != nil {
			return retrato, err
		}

		// Regra híbrida do desenho: a palavra só está tão desenhada quanto o hanzi componente
		// que menos avançou (um caractere isolado colapsa no próprio streak).
		componentes := hanzisComponentes(palavra)
		if len(componentes) > 0 {
			menor := -1
			for _, hanzi := range componentes {
				st, err := progresso.ObterEstatisticasPalavra(hanzi)
				if err != nil {
					return retrato, err
				}
				if menor == -1 || st["desenho"] < menor {
					menor = st["desenho"]
				}
			}
			estatisticas["desenho"] = menor
		}

		pinyin, significados, _ := r.Dicionario.Leitura(palavra)

		// A grafia salva é a identidade do item e vai ao placar como está (ver bindings_estudo.GetVocab).
		retrato.Palavras = append(retrato.Palavras, ProgressoPalavraRevisao{
			Hanzi:        palavra,
			Pinyin:       pinyin,
			Significados: significados,
			Status:       statusPorHanzi[palavra],
			Streaks:      estatisticas,
		})
	}
	return retrato, nil
}
