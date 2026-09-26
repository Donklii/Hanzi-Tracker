package dicionario

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"unicode/utf8"
)

// ----- Banco: fonte única fundida (idiomas/<idioma>/dicionario.jsonl.gz) -----
//
// Uma linha JSON por palavra SIMPLIFICADA, gerada por dicionario/fusao/fundir_dicionarios.go a partir
// de dicionario/fontes/<idioma>/. Substitui os dois loaders antigos (cedict.u8 + makemeahanzi.txt):
// o app não lê mais fonte crua nenhuma em runtime.
//
// PRIORIDADE: quem manda é o campo "definicao" do TOPO. A fusão já resolveu de onde ele veio (curado
// do makemeahanzi quando o caractere tem, senão a 1ª leitura do CEDICT promovida) — o banco não
// reabre essa decisão. As "leituras" são o detalhe por pronúncia/grafia que sobrou depois da promoção.
//
// RECONSTRUÇÃO (o inverso da compactação da fusão — ver o cabeçalho de fundir_dicionarios.go). A
// compactação apaga da leitura tudo que o topo já responde, então ler o arquivo exige repor:
//  1. topo com definicao/pinyin e SEM leituras: leitura única — o tradicional do topo é a grafia dela;
//  2. leitura sem "pinyin": vale topo.pinyin[0];
//  3. topo com exatamente 1 tradicional e NENHUMA leitura com "tradicional": todas usam essa forma;
//  4. nos demais casos, leitura sem "tradicional" = grafia igual ao simplificado.
//
// A leitura promovida ao topo é REMOVIDA da lista pela compactação, então "topo + leituras restantes"
// nunca duplica uma acepção.

// SEPARADOR_ACEPCOES é como a fusão une várias acepções dentro de "definicao" (tanto a definição
// curada do makemeahanzi quanto os significados do CEDICT promovidos ao topo). Dividir por ele é o
// que devolve a lista de significados de uma entrada achatada.
const SEPARADOR_ACEPCOES = "; "

// TIPO_LEITURA_* rotula uma leitura pela natureza da glosa, de forma INDEPENDENTE DE IDIOMA: a fusão
// detecta o rótulo no inglês original (onde as marcas "variant of"/"surname" existem) e o grava no
// arquivo, para o consumidor distinguir leitura-ponteiro de variante e registro de sobrenome sem
// depender do texto traduzido ("variante de"/"sobrenome"). Leitura de CONTEÚDO não recebe rótulo ("").
const (
	TIPO_LEITURA_VARIANTE  = "variante"
	TIPO_LEITURA_SOBRENOME = "sobrenome"
)

const (
	MAXIMO_RESULTADOS_BUSCA = 3000 // teto da busca geral (a varredura é linear sobre o dicionário todo)
	MAXIMO_COMPOSTOS        = 100  // teto de caracteres devolvidos por BuscarCompostosPor
)

// ----- DTOs públicos -----
//
// Tags em inglês: DecomposicaoHanzi e Etimologia são serializadas para o frontend, e mudar as tags
// quebraria o contrato. Não confundir com as estruturas do arquivo fundido (tags em PT), logo abaixo.

// EntradaDicionario é uma acepção de uma palavra: uma grafia + uma leitura + seus significados.
// Tipo é o rótulo language-neutral da leitura (TIPO_LEITURA_*, "" quando é conteúdo).
type EntradaDicionario struct {
	Tradicional  string
	Simplificado string
	Pinyin       string
	Significados []string
	Tipo         string
}

type Etimologia struct {
	Tipo      string `json:"type,omitempty"`
	Fonetica  string `json:"phonetic,omitempty"`
	Semantica string `json:"semantic,omitempty"`
	Dica      string `json:"hint,omitempty"`
}

// DecomposicaoHanzi são os dados curados de um caractere (só existem para quem veio do makemeahanzi).
type DecomposicaoHanzi struct {
	Caractere    string     `json:"character"`
	Definicao    string     `json:"definition,omitempty"`
	Pinyin       []string   `json:"pinyin,omitempty"`
	Decomposicao string     `json:"decomposition"`
	Etimologia   Etimologia `json:"etymology,omitempty"`
	Radical      string     `json:"radical"`
	Abreviacoes  []string   `json:"abreviacoes,omitempty"`
	// Correspondencias liga cada TRAÇO do caractere (mesma ordem do banco de traçados) ao componente
	// da Decomposicao que ele forma — ver componentes.go, o único consumidor.
	Correspondencias [][]int `json:"matches,omitempty"`
}

// ----- Estruturas do arquivo fundido -----
//
// Tags em PT porque é assim que a fusão grava. NÃO confundir com DecomposicaoHanzi/Etimologia, que
// têm tags em inglês por serem o contrato serializado com o frontend.

type etimologiaFundida struct {
	Tipo      string `json:"tipo,omitempty"`
	Fonetica  string `json:"fonetica,omitempty"`
	Semantica string `json:"semantica,omitempty"`
	Dica      string `json:"dica,omitempty"`
}

type leituraFundida struct {
	Pinyin       string   `json:"pinyin,omitempty"`
	Tradicional  string   `json:"tradicional,omitempty"`
	Significados []string `json:"significados,omitempty"`
	Tipo         string   `json:"tipo,omitempty"` // TIPO_LEITURA_* (variante/sobrenome); ausente = conteúdo
}

type Frequencia struct {
	Percentual float64 `json:"percentual"`
	Posicao    int     `json:"posicao"`
}

// entradaFundida é uma linha do .jsonl.gz. Campos do arquivo que ninguém consome ainda ficam de
// FORA (json.Unmarshal ignora o que não está aqui).
type entradaFundida struct {
	Simplificado     string             `json:"simplificado"`
	Tradicional      []string           `json:"tradicional,omitempty"`
	Definicao        string             `json:"definicao,omitempty"`
	Pinyin           []string           `json:"pinyin,omitempty"`
	Leituras         []leituraFundida   `json:"leituras,omitempty"`
	Decomposicao     string             `json:"decomposicao,omitempty"`
	Etimologia       *etimologiaFundida `json:"etimologia,omitempty"`
	Radical          string             `json:"radical,omitempty"`
	Correspondencias [][]int            `json:"correspondencias,omitempty"`
	Frequencia       *Frequencia        `json:"frequencia,omitempty"`
}

// Banco é a fonte única de consulta do dicionário. Os índices são montados UMA vez no Carregar
// (reconstruir a cada busca custaria alocação em caminho quente, ex.: SegmentarPorDicionario).
type Banco struct {
	entradas   map[string][]EntradaDicionario // chave: simplificado E tradicional
	caracteres map[string]DecomposicaoHanzi   // só entradas com dados curados (decomposição/radical)

	pinyinIndex  map[string][]string
	simpParaTrad map[rune]rune          // simplificado → tradicional (caracteres que diferem)
	tradParaSimp map[rune]rune          // tradicional → simplificado (caracteres que diferem)
	frequencias  map[string]*Frequencia // chave: simplificado

	// candidatosRevisao: caracteres curados com definição E pinyin, sem as abreviações visuais.
	// Universo de sorteio da revisão de hanzis — cacheado para o sorteio não varrer o mapa inteiro.
	candidatosRevisao []DecomposicaoHanzi
}

// ----- Inicialização -----

func NovoBanco() *Banco {
	return &Banco{
		entradas:     make(map[string][]EntradaDicionario),
		caracteres:   make(map[string]DecomposicaoHanzi),
		pinyinIndex:  make(map[string][]string),
		simpParaTrad: make(map[rune]rune),
		tradParaSimp: make(map[rune]rune),
		frequencias:  make(map[string]*Frequencia),
	}
}

// Carregar lê idiomas/<idioma>/dicionario.jsonl.gz e monta todos os índices, caindo para o inglês
// quando o idioma não tiver o arquivo próprio.
func (b *Banco) Carregar(idioma string) error {
	dados, _, err := lerRecursoIdioma(idioma, recursoDicionario)
	if err != nil {
		return err
	}
	if len(dados) == 0 {
		return fmt.Errorf("dicionário não foi embarcado corretamente")
	}

	leitorGz, err := gzip.NewReader(bytes.NewReader(dados))
	if err != nil {
		return fmt.Errorf("dicionário fundido ilegível: %w", err)
	}
	defer leitorGz.Close()

	varredor := bufio.NewScanner(leitorGz)
	varredor.Buffer(make([]byte, 1<<20), 1<<20)

	// Runas que aparecem como forma SIMPLIFICADA (coluna simplificada de alguma entrada). Coletadas
	// no scan para, ao final, expurgar mapeamentos tradicional→simplificado espúrios (ver
	// removerConversoesSimplificadoEspurias): quem já é simplificado não pode ser reescrito ao
	// converter para simplificado.
	simplificadosConhecidos := make(map[rune]bool)

	for varredor.Scan() {
		linha := varredor.Bytes()
		if len(bytes.TrimSpace(linha)) == 0 {
			continue
		}

		var e entradaFundida
		if err := json.Unmarshal(linha, &e); err != nil || e.Simplificado == "" {
			continue
		}
		for _, r := range e.Simplificado {
			simplificadosConhecidos[r] = true
		}
		b.indexarEntrada(&e)
	}
	if err := varredor.Err(); err != nil {
		return err
	}

	b.removerConversoesSimplificadoEspurias(simplificadosConhecidos)
	b.vincularAbreviacoes()
	b.cachearCandidatosRevisao()
	return nil
}

// indexarEntrada explode uma linha do fundido nos índices de consulta.
func (b *Banco) indexarEntrada(e *entradaFundida) {
	for _, tradicional := range e.Tradicional {
		b.mapearConversao(tradicional, e.Simplificado)
	}

	reconstruidas := b.reconstruirEntradas(e)
	b.entradas[e.Simplificado] = append(b.entradas[e.Simplificado], reconstruidas...)
	for _, tradicional := range e.Tradicional {
		if tradicional == e.Simplificado {
			continue
		}
		b.entradas[tradicional] = append(b.entradas[tradicional], reconstruidas...)
	}

	for _, entrada := range reconstruidas {
		b.indexarPinyin(entrada.Pinyin, e.Simplificado)
	}

	if e.Frequencia != nil {
		b.frequencias[e.Simplificado] = e.Frequencia
	}

	// Sem decomposição = entrada que veio só do CEDICT; não há dado curado de caractere para guardar.
	if e.Decomposicao == "" {
		return
	}
	caractere := DecomposicaoHanzi{
		Caractere:        e.Simplificado,
		Definicao:        e.Definicao,
		Pinyin:           e.Pinyin,
		Decomposicao:     e.Decomposicao,
		Radical:          e.Radical,
		Correspondencias: e.Correspondencias,
	}
	if e.Etimologia != nil {
		caractere.Etimologia = Etimologia{
			Tipo:      e.Etimologia.Tipo,
			Fonetica:  e.Etimologia.Fonetica,
			Semantica: e.Etimologia.Semantica,
			Dica:      e.Etimologia.Dica,
		}
	}
	b.caracteres[e.Simplificado] = caractere
}

// reconstruirEntradas devolve as acepções da palavra: a do TOPO primeiro (a prioridade — ver o
// cabeçalho) e depois uma por leitura restante, repondo o que a compactação apagou (regras 1 a 4).
func (b *Banco) reconstruirEntradas(e *entradaFundida) []EntradaDicionario {
	grafia := grafiaPadrao(e)

	var entradas []EntradaDicionario
	if e.Definicao != "" {
		entradas = append(entradas, EntradaDicionario{
			Tradicional:  grafia,
			Simplificado: e.Simplificado,
			Pinyin:       primeiroPinyin(e),
			Significados: strings.Split(e.Definicao, SEPARADOR_ACEPCOES),
		})
	}

	for _, l := range e.Leituras {
		entrada := EntradaDicionario{
			Tradicional:  l.Tradicional,
			Simplificado: e.Simplificado,
			Pinyin:       l.Pinyin,
			Significados: l.Significados,
			Tipo:         l.Tipo,
		}
		if entrada.Tradicional == "" {
			entrada.Tradicional = grafia // regras 3 e 4
		}
		if entrada.Pinyin == "" {
			entrada.Pinyin = primeiroPinyin(e) // regra 2
		}
		if len(entrada.Significados) == 0 {
			entrada.Significados = strings.Split(e.Definicao, SEPARADOR_ACEPCOES)
		}
		entradas = append(entradas, entrada)
	}

	return entradas
}

// vincularAbreviacoes anexa a cada caractere completo as abreviações visuais que o representam.
func (b *Banco) vincularAbreviacoes() {
	reverso := make(map[string][]string)
	for abrev, completo := range MapaAbrevParaCompleto {
		reverso[completo] = append(reverso[completo], abrev)
	}

	for completo, abreviacoes := range reverso {
		entrada, existe := b.caracteres[completo]
		if !existe {
			continue
		}
		entrada.Abreviacoes = abreviacoes
		b.caracteres[completo] = entrada
	}
}

// cachearCandidatosRevisao separa os caracteres sorteáveis pela revisão (com definição e pinyin,
// sem as abreviações visuais, que não são palavras).
func (b *Banco) cachearCandidatosRevisao() {
	for caractere, entrada := range b.caracteres {
		if _, ehAbrev := MapaAbrevParaCompleto[caractere]; ehAbrev {
			continue
		}
		if entrada.Definicao == "" || len(entrada.Pinyin) == 0 || entrada.Pinyin[0] == "" {
			continue
		}
		b.candidatosRevisao = append(b.candidatosRevisao, entrada)
	}
}

// ----- Consultas -----

// Buscar devolve as acepções de uma palavra (indexada pelo simplificado e pelo tradicional).
func (b *Banco) Buscar(palavra string) []EntradaDicionario {
	return b.entradas[palavra]
}

// BuscarCaractere devolve os dados curados de um caractere — decomposição, etimologia, radical
// (nil quando o caractere não tem entrada curada). Não confundir com Buscar: aqui não há acepção
// de palavra composta.
func (b *Banco) BuscarCaractere(caractere string) *DecomposicaoHanzi {
	entrada, existe := b.caracteres[caractere]
	if !existe {
		// Se não existir, tenta encontrar a versão simplificada correspondente (para caracteres tradicionais)
		runas := []rune(caractere)
		if len(runas) == 1 {
			if simpRune, ok := b.tradParaSimp[runas[0]]; ok {
				if entSimp, existeSimp := b.caracteres[string(simpRune)]; existeSimp {
					copia := entSimp
					copia.Caractere = caractere
					return &copia
				}
			}
		}
		return nil
	}
	return &entrada
}

// BuscarPorPinyin devolve os hanzis simplificados que casam com o pinyin dado (sem tom/espaço).
func (b *Banco) BuscarPorPinyin(pinyin string) []string {
	return b.pinyinIndex[limparPinyinParaBusca(pinyin)]
}

// CandidatosRevisao devolve os caracteres elegíveis para a revisão (com definição e pinyin).
// O slice retornado é compartilhado — o chamador não deve modificá-lo.
func (b *Banco) CandidatosRevisao() []DecomposicaoHanzi {
	return b.candidatosRevisao
}

// BuscarCompostosPor devolve até MAXIMO_COMPOSTOS caracteres que usam o dado como componente.
func (b *Banco) BuscarCompostosPor(componente string) []string {
	var resultados []string
	if componente == "" {
		return resultados
	}

	for caractere, entrada := range b.caracteres {
		if caractere == componente || !strings.Contains(entrada.Decomposicao, componente) {
			continue
		}
		resultados = append(resultados, caractere)
		if len(resultados) >= MAXIMO_COMPOSTOS {
			break
		}
	}
	return resultados
}

// BuscarGeral pesquisa por hanzi, pinyin ou significado. Casamento de hanzi/pinyin tem prioridade
// sobre casamento de significado (o teto de MAXIMO_RESULTADOS_BUSCA vale para o total).
func (b *Banco) BuscarGeral(termo string) []EntradaDicionario {
	if termo == "" {
		return nil
	}
	termoMinusculo := strings.ToLower(termo)
	termoLimpo := limparPinyinParaBusca(termo)

	var prioritarios, secundarios []EntradaDicionario
	vistos := make(map[string]bool)

varredura:
	for _, entradas := range b.entradas {
		for _, e := range entradas {
			if vistos[e.Simplificado] {
				continue
			}

			prioritario := casaHanzi(e, termo) || casaPinyin(e, termoLimpo)
			if !prioritario && !casaSignificado(e, termoMinusculo) {
				continue
			}

			vistos[e.Simplificado] = true
			if !prioritario {
				secundarios = append(secundarios, e)
				continue
			}
			prioritarios = append(prioritarios, e)
			if len(prioritarios) >= MAXIMO_RESULTADOS_BUSCA {
				break varredura
			}
		}
	}

	resultados := prioritarios
	for _, s := range secundarios {
		if len(resultados) >= MAXIMO_RESULTADOS_BUSCA {
			break
		}
		resultados = append(resultados, s)
	}
	return resultados
}

// AvaliarTipoHanzi classifica o caractere como "Tradicional", "Simplificado", "Ambos" ou ""
// (desconhecido).
func (b *Banco) AvaliarTipoHanzi(hanzi string) string {
	if b == nil {
		return ""
	}

	entradas := b.entradas[hanzi]
	if len(entradas) == 0 {
		return ""
	}

	e := entradas[0]
	switch {
	case e.Simplificado == hanzi && e.Tradicional == hanzi:
		return "Ambos"
	case e.Simplificado == hanzi:
		return "Simplificado"
	case e.Tradicional == hanzi:
		return "Tradicional"
	}
	return ""
}

// ConverterTexto converte um texto entre "simplificado" e "tradicional", caractere a caractere.
// Caracteres sem mapeamento (pontuação, grafias comuns às duas formas) passam inalterados.
func (b *Banco) ConverterTexto(texto string, alvo string) string {
	if b == nil {
		return texto
	}

	var mapa map[rune]rune
	switch alvo {
	case "simplificado":
		mapa = b.tradParaSimp
	case "tradicional":
		mapa = b.simpParaTrad
	default:
		return texto
	}
	if len(mapa) == 0 {
		return texto
	}

	var construtor strings.Builder
	construtor.Grow(len(texto))
	for _, r := range texto {
		if convertido, ok := mapa[r]; ok {
			construtor.WriteRune(convertido)
			continue
		}
		construtor.WriteRune(r)
	}
	return construtor.String()
}

// FraseCompativelComTipo diz se a frase não contém caracteres exclusivos do tipo oposto ao desejado.
func (b *Banco) FraseCompativelComTipo(frase string, tipoDesejado string) bool {
	if b == nil || tipoDesejado == "ambos" || tipoDesejado == "" {
		return true
	}

	for _, r := range frase {
		tipo := b.AvaliarTipoHanzi(string(r))
		if tipoDesejado == "simplificado" && tipo == "Tradicional" {
			return false
		}
		if tipoDesejado == "tradicional" && tipo == "Simplificado" {
			return false
		}
	}
	return true
}

// ObterFrequencia retorna a frequência de uso de uma palavra (simplificada ou tradicional).
func (b *Banco) ObterFrequencia(palavra string) *Frequencia {
	if b == nil {
		return nil
	}
	simplificada := palavra
	if entries, ok := b.entradas[palavra]; ok && len(entries) > 0 {
		simplificada = entries[0].Simplificado
	}
	if f, ok := b.frequencias[simplificada]; ok {
		return f
	}
	return nil
}

// ObterTopPalavrasFrequentes retorna as palavras com menor posição no ranking de frequência.
func (b *Banco) ObterTopPalavrasFrequentes(limite int) []string {
	if b == nil || len(b.frequencias) == 0 {
		return nil
	}
	type itemFreq struct {
		palavra string
		pos     int
	}
	lista := make([]itemFreq, 0, len(b.frequencias))
	for word, freq := range b.frequencias {
		if freq != nil && freq.Posicao > 0 {
			lista = append(lista, itemFreq{palavra: word, pos: freq.Posicao})
		}
	}
	sort.Slice(lista, func(i, j int) bool {
		return lista[i].pos < lista[j].pos
	})
	if len(lista) > limite {
		lista = lista[:limite]
	}
	res := make([]string, len(lista))
	for i, f := range lista {
		res[i] = f.palavra
	}
	return res
}

// ObterAlternativa retorna a contraparte alternativa (simplificada/tradicional) se houver.
func (b *Banco) ObterAlternativa(palavra string) string {
	if b == nil {
		return ""
	}
	// Tenta converter para tradicional. Se mudar, essa é a alternativa (tradicional).
	trad := b.ConverterTexto(palavra, "tradicional")
	if trad != palavra {
		return trad
	}
	// Tenta converter para simplificado. Se mudar, essa é a alternativa (simplificada).
	simp := b.ConverterTexto(palavra, "simplificado")
	if simp != palavra {
		return simp
	}
	return ""
}

// ----- Utilitários -----

// TodasPalavras devolve todas as formas escritas indexadas (simplificado e tradicional), sem
// repetição. Alimenta o pré-carregamento do cache de TTS (ver tts_precache.go).
func (b *Banco) TodasPalavras() []string {
	palavras := make([]string, 0, len(b.entradas))
	for palavra := range b.entradas {
		palavras = append(palavras, palavra)
	}
	return palavras
}

// TodosCaracteres devolve os caracteres curados, EXCETO as abreviações visuais de radical, que não
// são palavras faláveis. Alimenta o pré-carregamento do cache de TTS junto com TodasPalavras.
func (b *Banco) TodosCaracteres() []string {
	caracteres := make([]string, 0, len(b.caracteres))
	for caractere := range b.caracteres {
		if _, ehAbrev := MapaAbrevParaCompleto[caractere]; ehAbrev {
			continue
		}
		caracteres = append(caracteres, caractere)
	}
	return caracteres
}

// TotalHanzis devolve a quantidade de caracteres com dados curados (decomposição/radical).
func (b *Banco) TotalHanzis() int {
	return len(b.caracteres)
}

// CaractereCompleto resolve uma abreviação visual/radical para o caractere CJK completo equivalente.
func (b *Banco) CaractereCompleto(abrev string) string {
	if completo, existe := MapaAbrevParaCompleto[abrev]; existe {
		return completo
	}
	return abrev
}

// mapearConversao registra o par tradicional↔simplificado caractere a caractere, para ConverterTexto.
// Só grafias de mesmo comprimento são alinháveis posicionalmente.
func (b *Banco) mapearConversao(tradicional, simplificado string) {
	if tradicional == simplificado {
		return
	}
	runasT, runasS := []rune(tradicional), []rune(simplificado)
	if len(runasT) != len(runasS) {
		return
	}

	for i := range runasT {
		if runasT[i] == runasS[i] {
			continue
		}
		// Preserva o primeiro mapeamento (mais frequente) para evitar que
		// variantes raras sobrescrevam mapeamentos de caracteres comuns.
		if _, existe := b.simpParaTrad[runasS[i]]; !existe {
			b.simpParaTrad[runasS[i]] = runasT[i]
		}
		if _, existe := b.tradParaSimp[runasT[i]]; !existe {
			b.tradParaSimp[runasT[i]] = runasS[i]
		}
	}
}

// removerConversoesSimplificadoEspurias tira do mapa tradicional→simplificado as runas que também
// são, elas mesmas, uma forma SIMPLIFICADA canônica (constam da coluna simplificada de alguma
// entrada). Conserta a colisão estrutural do CC-CEDICT em que um caractere simplificado é listado
// como forma tradicional de OUTRO: a entrada de 幺, por exemplo, traz 么 na lista "tradicional", o
// que gravava tradParaSimp[么]=幺 e fazia ConverterTexto(_, "simplificado") reescrever 么→幺 e
// corromper 什么→什幺, 怎么→怎幺, 那么→那幺... O 么 partícula já é simplificado; converter para
// simplificado tem de deixá-lo intacto. A direção oposta (simpParaTrad) NÃO recebe filtro simétrico
// de propósito: ali ele apagaria pares legítimos de caracteres válidos nas duas grafias (后→後).
func (b *Banco) removerConversoesSimplificadoEspurias(simplificadosConhecidos map[rune]bool) {
	for trad := range b.tradParaSimp {
		if simplificadosConhecidos[trad] {
			delete(b.tradParaSimp, trad)
		}
	}
}

// indexarPinyin acrescenta o hanzi ao índice reverso de pinyin, sem repetir.
func (b *Banco) indexarPinyin(pinyin, simplificado string) {
	chave := limparPinyinParaBusca(pinyin)
	if chave == "" {
		return
	}

	for _, h := range b.pinyinIndex[chave] {
		if h == simplificado {
			return
		}
	}
	b.pinyinIndex[chave] = append(b.pinyinIndex[chave], simplificado)
}

// grafiaPadrao devolve a forma tradicional que vale para as leituras sem campo próprio (regras 1, 3
// e 4): a única tradicional do topo quando nenhuma leitura discorda, senão o próprio simplificado.
func grafiaPadrao(e *entradaFundida) string {
	if len(e.Tradicional) != 1 {
		return e.Simplificado
	}
	for _, l := range e.Leituras {
		if l.Tradicional != "" {
			return e.Simplificado
		}
	}
	return e.Tradicional[0]
}

// primeiroPinyin devolve a leitura principal do topo (regra 2), "" quando a entrada não tem pinyin.
func primeiroPinyin(e *entradaFundida) string {
	if len(e.Pinyin) == 0 {
		return ""
	}
	return e.Pinyin[0]
}

// casaHanzi diz se o termo aparece na grafia simplificada ou tradicional da entrada.
func casaHanzi(e EntradaDicionario, termo string) bool {
	return strings.Contains(e.Simplificado, termo) || strings.Contains(e.Tradicional, termo)
}

// casaPinyin diz se o termo (já normalizado por limparPinyinParaBusca) aparece no pinyin da entrada.
func casaPinyin(e EntradaDicionario, termoLimpo string) bool {
	if termoLimpo == "" {
		return false
	}
	return strings.Contains(limparPinyinParaBusca(e.Pinyin), termoLimpo)
}

// casaSignificado diz se o termo (já minúsculo) aparece em alguma acepção da entrada.
func casaSignificado(e EntradaDicionario, termoMinusculo string) bool {
	for _, significado := range e.Significados {
		if strings.Contains(strings.ToLower(significado), termoMinusculo) {
			return true
		}
	}
	return false
}

// ehCaractereUnico diz se a palavra é um hanzi isolado. É o que separa "caractere" de "palavra
// composta" nas consultas — a fusão não marca essa diferença, ela sai da contagem de runas.
func ehCaractereUnico(palavra string) bool {
	return utf8.RuneCountInString(palavra) == 1
}
