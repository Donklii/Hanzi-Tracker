package revisao

import (
	"fmt"
	"math/rand/v2"
	"sort"
	"strings"
	"unicode"

	"wails_app/config"
	"wails_app/dicionario"
	"wails_app/progresso"
)

// ----- Seção: Foco de revisão (grupo global de caracteres priorizados) -----
// Expande a priorização de "em estudo" para um grupo GLOBAL e persistente, compartilhado entre
// sessões: em vez de diluir a atenção por todo o vocabulário em estudo, as revisões concentram
// os alvos num grupo pequeno até que cada caractere conclua o aprendizado (as 5 áreas com streak
// >= MetaAcertosConsecutivos) — aí ele sai e outro em estudo entra no lugar.
//
// Elegibilidade do grupo: caractere EM ESTUDO cujo aprendizado ainda NÃO está concluído. Quem
// conclui (ou deixa de estar em estudo) sai; a vaga é preenchida por SORTEIO PONDERADO — o peso
// combina TEMPO EM ESTUDO (palavras adicionadas ao estudo há mais tempo têm mais chance) com as
// VISUALIZAÇÕES DE OCR (cada vez que a palavra apareceu num scan aumenta a chance), e o sorteio é
// restrito às 10 palavras de maior peso (ver sorteioPonderadoFoco).
//
// Tamanho do grupo: manual (TamanhoFocoRevisao) ou automático (TamanhoFocoAutomatico = cabe todos
// os elegíveis até o teto). Os membros atuais são preservados entre sincronizações (sem churn):
// só saem quando perdem a elegibilidade ou quando o tamanho encolhe.
//
// A rotação fecha o ciclo com o card de sugestão de aprendida do placar (PlacarRevisao.tsx):
// o foco concentra a prática → os streaks das 5 categorias sobem → a sugestão dispara →
// o usuário marca como aprendida → a vaga abre para a próxima palavra.
//
// A sincronização é preguiçosa e idempotente: roda ao montar uma sessão e ao consultar o grupo
// (ObterFocoRevisao) — não há hooks espalhados em AddVocab/RemoveVocab.

// Limites do tamanho do grupo (o valor manual é saneado por tamanhoFocoConfigurado).
const (
	TamanhoFocoPadrao = 5
	TamanhoFocoMaximo = 20

	// ProporcaoFocoRevisao é a fatia da sessão dedicada ao grupo de foco quando ele existe: 9 de
	// cada 10 questões por padrão. Consumida em selecionarAlvos (wails_app/revisao.go).
	ProporcaoFocoRevisao = 0.9
)

// ItemFocoRevisao é um caractere do grupo enriquecido para exibição na UI da revisão.
type ItemFocoRevisao struct {
	Hanzi        string   `json:"hanzi"`
	Pinyin       string   `json:"pinyin"`
	Significados []string `json:"significados"`

	// AreasConcluidas é o número de categorias de aprendizado (entre as 5) com streak >= 3.
	// A UI mostra "n/5".
	AreasConcluidas int `json:"areasConcluidas"`
}

// tamanhoFocoConfigurado devolve o tamanho MANUAL saneado (usado quando o modo automático está off).
func (r *GerenciadorRevisao) tamanhoFocoConfigurado() int {
	tamanho := r.Config().TamanhoFocoRevisao
	if tamanho <= 0 {
		return TamanhoFocoPadrao
	}
	if tamanho > TamanhoFocoMaximo {
		return TamanhoFocoMaximo
	}
	return tamanho
}

// tamanhoFocoEfetivo resolve o tamanho do grupo nesta sincronização. No modo automático o grupo
// cabe todos os elegíveis (até o teto); no manual usa o valor configurado.
func (r *GerenciadorRevisao) tamanhoFocoEfetivo(qtdElegiveis int) int {
	if r.Config().TamanhoFocoAutomatico {
		if qtdElegiveis > TamanhoFocoMaximo {
			return TamanhoFocoMaximo
		}
		return qtdElegiveis
	}
	return r.tamanhoFocoConfigurado()
}

// hanzisComponentes extrai os hanzis "reais" de uma palavra (dedup, na ordem, ignorando as
// abreviações visuais de radical/componente). Para um caractere isolado devolve ele mesmo.
func hanzisComponentes(palavra string) []string {
	var comps []string
	vistos := make(map[string]bool)
	for _, r := range palavra {
		if !unicode.Is(unicode.Han, r) {
			continue
		}
		c := string(r)
		if _, ehAbrev := dicionario.MapaAbrevParaCompleto[c]; ehAbrev {
			continue
		}
		if vistos[c] {
			continue
		}
		vistos[c] = true
		comps = append(comps, c)
	}
	return comps
}

// aprendizadoConcluido diz se uma PALAVRA concluiu o aprendizado. Regra híbrida (o desenho de uma
// palavra multi-hanzi não pode ser feito de uma vez): as quatro categorias que a palavra pratica
// como unidade (significado, fonética, contexto, pronúncia) precisam da meta de acertos na PRÓPRIA
// palavra, e o desenho precisa estar concluído em CADA hanzi componente. Para um caractere isolado
// isso colapsa nas 5 áreas do próprio caractere. Sem banco, devolve false (não exclui na dúvida).
// aprendizadoConcluido diz se uma PALAVRA concluiu o aprendizado para os modos permitidos da revisão.
func (r *GerenciadorRevisao) aprendizadoConcluido(palavra string) bool {
	return r.aprendizadoConcluidoComModos(palavra, r.modosPermitidosRevisao())
}

// aprendizadoConcluidoComModos verifica se a palavra atingiu a meta nas categorias informadas (modosPermitidos).
func (r *GerenciadorRevisao) aprendizadoConcluidoComModos(palavra string, modosPermitidos []string) bool {
	if len(modosPermitidos) == 0 {
		modosPermitidos = r.modosPermitidosRevisao()
	}
	modosSet := make(map[string]bool, len(modosPermitidos))
	for _, m := range modosPermitidos {
		modosSet[m] = true
	}

	estatisticas, err := progresso.ObterEstatisticasPalavra(palavra)
	if err != nil {
		return false
	}
	for _, categoria := range []string{"significado", "fonetica", "contexto", "pronuncia"} {
		if modosSet[categoria] && estatisticas[categoria] < MetaAcertosConsecutivos {
			return false
		}
	}
	if modosSet["desenho"] {
		for _, hanzi := range hanzisComponentes(palavra) {
			st, err := progresso.ObterEstatisticasPalavra(hanzi)
			if err != nil {
				return false
			}
			if st["desenho"] < MetaAcertosConsecutivos {
				return false
			}
		}
	}
	return true
}

// palavrasFocoElegiveis lista as PALAVRAS em estudo (mantidas inteiras — palavras multi-hanzi
// podem ser focadas), na ordem da MAIS RECENTE para a mais antiga (GetAllVocab vem data_add DESC),
// descartando: entradas sem hanzi real, repetições e palavras cujo aprendizado já está concluído
// (filtro `concluido`). O índice 0 é o mais recente — base para o peso de recência do sorteio. O
// predicado `concluido` é injetado para permitir teste sem banco.
func palavrasFocoElegiveis(vocabulario []progresso.Vocab, concluido func(string) bool) []string {
	var elegiveis []string
	vistos := make(map[string]bool)

	for _, v := range vocabulario { // mais recente primeiro
		if v.Status != dicionario.StatusEstudo {
			continue
		}
		palavra := v.Hanzi
		if palavra == "" || vistos[palavra] || len(hanzisComponentes(palavra)) == 0 {
			continue
		}
		vistos[palavra] = true
		if concluido(palavra) {
			continue
		}
		elegiveis = append(elegiveis, palavra)
	}
	return elegiveis
}

// MaximoCandidatosSorteioFoco restringe o sorteio de vagas às N palavras de maior peso: as demais
// elegíveis ficam de fora até subirem no ranking (por recência ou por novas visualizações de OCR).
const MaximoCandidatosSorteioFoco = 10

// sorteioPonderadoFoco embaralha os candidatos por peso combinado e devolve uma nova ordem (não
// muta a entrada). O peso favorece as palavras adicionadas HÁ MAIS TEMPO ao estudo (índice maior
// na lista data_add DESC = mais antiga = maior base, linear: i + 1) e é multiplicado pelas
// visualizações de OCR (1 + vezes vista — cada aparição num scan aumenta a chance). Só as
// MaximoCandidatosSorteioFoco palavras de maior peso participam; entre elas o sorteio é
// ponderado e sem reposição, então as favoritas entram com mais frequência sem nunca ficarem
// impossíveis para as demais do corte. É essa ordem que reconciliarFoco consome para preencher vagas.
func sorteioPonderadoFoco(candidatos []string, vezesVista map[string]int) []string {
	n := len(candidatos)
	restantes := make([]string, n)
	copy(restantes, candidatos)
	if n <= 1 {
		return restantes
	}

	pesos := make([]float64, n)
	for i, palavra := range restantes {
		pesos[i] = float64(i+1) * float64(1+vezesVista[palavra])
	}

	// Corte do ranking: ordena por peso decrescente (estável — empate mantém a ordem de recência)
	// e descarta quem ficou abaixo das MaximoCandidatosSorteioFoco primeiras posições.
	indices := make([]int, n)
	for i := range indices {
		indices[i] = i
	}
	sort.SliceStable(indices, func(a, b int) bool { return pesos[indices[a]] > pesos[indices[b]] })
	if len(indices) > MaximoCandidatosSorteioFoco {
		indices = indices[:MaximoCandidatosSorteioFoco]
	}

	sorteaveis := make([]string, 0, len(indices))
	pesosSorteaveis := make([]float64, 0, len(indices))
	for _, i := range indices {
		sorteaveis = append(sorteaveis, restantes[i])
		pesosSorteaveis = append(pesosSorteaveis, pesos[i])
	}
	restantes, pesos = sorteaveis, pesosSorteaveis

	ordem := make([]string, 0, len(restantes))
	for len(restantes) > 0 {
		total := 0.0
		for _, p := range pesos {
			total += p
		}
		alvo := rand.Float64() * total
		escolhido := len(restantes) - 1 // fallback numérico (arredondamento)
		acumulado := 0.0
		for i, p := range pesos {
			acumulado += p
			if alvo < acumulado {
				escolhido = i
				break
			}
		}
		ordem = append(ordem, restantes[escolhido])
		restantes = append(restantes[:escolhido], restantes[escolhido+1:]...)
		pesos = append(pesos[:escolhido], pesos[escolhido+1:]...)
	}
	return ordem
}

// reconciliarFoco decide a rotação do grupo preservando os membros atuais estáveis: um membro FICA
// se ainda é elegível (elegiveisSet) e cabe no tamanho; sai quem perdeu a elegibilidade (aprendizado
// concluído / saiu do estudo) ou excede o tamanho. As vagas são preenchidas na ordemPreenchimento
// recebida (já ponderada por recência), pulando quem já está no grupo.
func reconciliarFoco(atuais []string, elegiveisSet map[string]bool, ordemPreenchimento []string, tamanho int) (mantidos, removidos, adicionados []string) {
	mantidosSet := make(map[string]bool, len(atuais))
	for _, c := range atuais {
		if elegiveisSet[c] && len(mantidos) < tamanho {
			mantidos = append(mantidos, c)
			mantidosSet[c] = true
		} else {
			removidos = append(removidos, c)
		}
	}

	for _, c := range ordemPreenchimento {
		if len(mantidos)+len(adicionados) >= tamanho {
			break
		}
		if !mantidosSet[c] {
			adicionados = append(adicionados, c)
		}
	}

	return mantidos, removidos, adicionados
}

// sincronizarFocoRevisao aplica a reconciliação no banco e devolve o grupo vigente, na ordem de
// entrada. Recebe o vocabulário já carregado para não repetir a consulta em quem já o tem.
func (r *GerenciadorRevisao) sincronizarFocoRevisao(vocabulario []progresso.Vocab) ([]string, error) {
	atuais, err := progresso.ObterFoco()
	if err != nil {
		return nil, err
	}

	elegiveis := palavrasFocoElegiveis(vocabulario, r.aprendizadoConcluido)
	elegiveisSet := make(map[string]bool, len(elegiveis))
	for _, c := range elegiveis {
		elegiveisSet[c] = true
	}

	// Visualizações de OCR por palavra: o multiplicador do peso do sorteio de vagas.
	vezesVista := make(map[string]int, len(vocabulario))
	for _, v := range vocabulario {
		vezesVista[v.Hanzi] = v.VezesVistaOcr
	}

	tamanho := r.tamanhoFocoEfetivo(len(elegiveis))
	ordemPreenchimento := sorteioPonderadoFoco(elegiveis, vezesVista)
	mantidos, removidos, adicionados := reconciliarFoco(atuais, elegiveisSet, ordemPreenchimento, tamanho)

	if len(removidos) > 0 || len(adicionados) > 0 {
		if err := progresso.AtualizarFoco(removidos, adicionados); err != nil {
			return nil, err
		}
	}
	return append(mantidos, adicionados...), nil
}

// SugestoesAprendido devolve, dentre as palavras informadas, as que já concluíram o aprendizado pela
// regra híbrida (aprendizadoConcluido) e ainda não estão marcadas como aprendidas — as candidatas ao
// card de sugestão do placar.
func (r *GerenciadorRevisao) SugestoesAprendido(palavras []string) ([]progresso.Vocab, error) {
	return r.SugestoesAprendidoComModos(palavras, nil)
}

// SugestoesAprendidoComModos devolve as sugestões de aprendida filtrando pelos modos informados/habilitados para a revisão.
func (r *GerenciadorRevisao) SugestoesAprendidoComModos(palavras []string, modos []string) ([]progresso.Vocab, error) {
	if len(modos) == 0 {
		modos = r.modosPermitidosRevisao()
	}
	vocabulario, err := progresso.GetAllVocab()
	if err != nil {
		return nil, err
	}
	porHanzi := make(map[string]progresso.Vocab, len(vocabulario))
	for _, v := range vocabulario {
		porHanzi[v.Hanzi] = v
	}

	vistos := make(map[string]bool)
	var sugestoes []progresso.Vocab
	for _, palavra := range palavras {
		if palavra == "" || vistos[palavra] {
			continue
		}
		vistos[palavra] = true
		v, ok := porHanzi[palavra]
		if !ok || v.Status == dicionario.StatusAprendido {
			continue
		}
		if r.aprendizadoConcluidoComModos(palavra, modos) {
			// Grafia salva = identidade: exibida como está (ver bindings_estudo.GetVocab).
			v.TipoHanzi = r.Dicionario.TipoHanzi(v.Hanzi)
			sugestoes = append(sugestoes, v)
		}
	}
	return sugestoes, nil
}

// ----- Binding: consulta do grupo de foco -----

// ObterFocoRevisao sincroniza o grupo (a marcação de aprendida no placar rotaciona na hora) e
// devolve os caracteres enriquecidos com leitura e progresso rumo à sugestão de aprendida.
func (r *GerenciadorRevisao) ObterFocoRevisao() ([]ItemFocoRevisao, error) {
	vocabulario, err := progresso.GetAllVocab()
	if err != nil {
		return nil, err
	}

	foco, err := r.sincronizarFocoRevisao(vocabulario)
	if err != nil {
		return nil, err
	}

	itens := make([]ItemFocoRevisao, 0, len(foco))
	for _, palavra := range foco {
		pinyin, significados, _ := r.Dicionario.Leitura(palavra)

		itens = append(itens, ItemFocoRevisao{
			Hanzi:           palavra,
			Pinyin:          pinyin,
			Significados:    significados,
			AreasConcluidas: r.areasConcluidas(palavra),
		})
	}
	return itens, nil
}

// areasConcluidas conta quantas das 5 áreas a palavra já concluiu, pela mesma regra híbrida de
// aprendizadoConcluido: significado/fonética/contexto/pronúncia na própria palavra + desenho de
// todos os hanzis componentes. Alimenta o "n/5" exibido no grupo de foco.
func (r *GerenciadorRevisao) areasConcluidas(palavra string) int {
	estatisticas, err := progresso.ObterEstatisticasPalavra(palavra)
	if err != nil {
		return 0
	}
	concluidas := 0
	for _, categoria := range []string{"significado", "fonetica", "contexto", "pronuncia"} {
		if estatisticas[categoria] >= MetaAcertosConsecutivos {
			concluidas++
		}
	}
	desenhoOk := true
	for _, hanzi := range hanzisComponentes(palavra) {
		st, err := progresso.ObterEstatisticasPalavra(hanzi)
		if err != nil || st["desenho"] < MetaAcertosConsecutivos {
			desenhoOk = false
			break
		}
	}
	if desenhoOk {
		concluidas++
	}
	return concluidas
}

// ----- Binding: adição e remoção manual do grupo de foco -----

// AdicionarHanziFoco insere manualmente no grupo de foco a PALAVRA informada (um caractere ou uma
// palavra multi-hanzi, mantida inteira). Se o texto ainda não estiver "em estudo", passa a estar —
// assim fica elegível e não é expulso na próxima sincronização. Ao adicionar uma palavra nova,
// ela é inserida e o tamanho do grupo de foco aumenta em 1 (até o teto do modo manual).
func (r *GerenciadorRevisao) AdicionarHanziFoco(texto string) error {
	texto = strings.TrimSpace(texto)
	if texto == "" {
		return fmt.Errorf("informe um caractere ou palavra para adicionar ao foco")
	}
	if len(hanzisComponentes(texto)) == 0 {
		return fmt.Errorf("nenhum caractere chinês válido em %q", texto)
	}

	vocabulario, err := progresso.GetAllVocab()
	if err != nil {
		return err
	}

	// Garante a palavra "em estudo" (só mexe se ainda não estiver — evita reescrever pinyin/significado
	// ou rebaixar uma aprendida à toa quando já é estudo).
	jaEmEstudo := false
	for _, v := range vocabulario {
		if v.Hanzi == texto && v.Status == dicionario.StatusEstudo {
			jaEmEstudo = true
			break
		}
	}
	if !jaEmEstudo {
		if err := progresso.AddOuUpdateVocab(texto, dicionario.StatusEstudo); err != nil {
			return err
		}
	}

	foco, err := progresso.ObterFoco()
	if err != nil {
		return err
	}
	noFoco := make(map[string]bool, len(foco))
	for _, c := range foco {
		noFoco[c] = true
	}
	if noFoco[texto] {
		return nil // já estava no foco
	}

	// Se não estava no foco, incrementa o tamanho configurado em 1 no modo manual (até 20).
	if !r.Config().TamanhoFocoAutomatico {
		novoTamanho := r.tamanhoFocoConfigurado() + 1
		if novoTamanho > TamanhoFocoMaximo {
			novoTamanho = TamanhoFocoMaximo
		}
		cfg := r.Config()
		cfg.TamanhoFocoRevisao = novoTamanho
		_ = config.SaveConfig(cfg)
	}

	adicionar := []string{texto}

	limite := TamanhoFocoMaximo
	if !r.Config().TamanhoFocoAutomatico {
		limite = r.tamanhoFocoConfigurado()
	}
	var remover []string
	excesso := len(foco) + len(adicionar) - limite
	for i := 0; i < excesso && i < len(foco); i++ {
		remover = append(remover, foco[i])
	}

	return progresso.AtualizarFoco(remover, adicionar)
}

// RemoverHanziFoco remove manualmente a PALAVRA informada do grupo de foco e decrementa o tamanho
// configurado do grupo em 1 (mínimo 1) no modo manual.
func (r *GerenciadorRevisao) RemoverHanziFoco(texto string) error {
	texto = strings.TrimSpace(texto)
	if texto == "" {
		return fmt.Errorf("informe um caractere ou palavra para remover do foco")
	}

	if !r.Config().TamanhoFocoAutomatico {
		novoTamanho := r.tamanhoFocoConfigurado() - 1
		if novoTamanho < 1 {
			novoTamanho = 1
		}
		cfg := r.Config()
		cfg.TamanhoFocoRevisao = novoTamanho
		_ = config.SaveConfig(cfg)
	}

	return progresso.AtualizarFoco([]string{texto}, nil)
}
