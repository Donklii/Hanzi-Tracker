// Comando fundir_dicionarios funde o makemeahanzi e o CC-CEDICT de um idioma em uma única
// fonte da verdade (dicionario_<idioma>.jsonl), com UMA linha JSON por palavra SIMPLIFICADA.
//
// Regras da fusão:
//   - A chave de toda linha é o chinês simplificado. As formas tradicionais viram o campo
//     "tradicional" (lista; omitido quando nenhuma difere do simplificado).
//   - Cada linha do CEDICT vira uma "leitura" aninhada {pinyin, tradicional?, significados},
//     preservando a associação tradicional↔significado (ex.: 下面 xiàmiàn "embaixo" vs
//     下麵 xiàmiàn "pôr o macarrão na água").
//   - O pinyin entre colchetes colado a um hanzi citado na glosa (ex.: "variant of 臺[tai2]",
//     "CL:個|个[ge4]") é REMOVIDO das leituras — é ruído de referência, não conteúdo. Aplica-se só ao
//     CEDICT (única fonte com essa notação). O fontes/en/cedict.u8 original fica intacto; a remoção é
//     desta derivação. Ver removerPinyinAposHanzi.
//   - Entradas do makemeahanzi contribuem os campos curados (definicao, pinyin, decomposicao,
//     etimologia, radical, correspondencias) — a definição do makemeahanzi tem prioridade de
//     exibição sobre os significados do CEDICT.
//   - O campo "frequencia" traz o uso da palavra no corpus do OpenSubtitles: {percentual, posicao}
//     (ver aplicarFrequencias). Só entra em quem a lista conhece; ausente = palavra fora do corpus,
//     não frequência zero.
//   - Caracteres do makemeahanzi que são EXCLUSIVAMENTE tradicionais (nunca aparecem na coluna
//     simplificada do CEDICT) não geram linha própria: viram variante na lista "tradicional" do
//     seu simplificado. Caracteres válidos nas duas grafias (乾 de 乾隆, 后 de 皇后) mantêm linha
//     própria E aparecem como variante onde couber.
//   - Pinyin gravado acentuado (hǎo), convertido com o MESMO ConverterPinyin do app.
//
// Compactação (evita repetir nas leituras o que o topo já diz). Regras de reconstrução:
//  1. Entrada com definicao/pinyin no topo e SEM leituras: leitura única — o tradicional do
//     topo (se houver) é a grafia dela.
//  2. Leitura sem "pinyin": lê-se topo.pinyin[0].
//  3. Topo com exatamente 1 tradicional e NENHUMA leitura com "tradicional": todas as leituras
//     (inclusive a promovida ao topo) escrevem essa forma.
//  4. Nos demais casos, leitura sem "tradicional" = grafia igual ao simplificado.
//
// Entradas sem dados do makemeahanzi (palavras e caracteres só do CEDICT) têm a primeira leitura de
// CONTEÚDO promovida ao topo (significados viram "definicao", unidos por "; "). Uma leitura SECUNDÁRIA
// — que só diz "variant of ..." ou que só registra um sobrenome ("surname Wang") — tem prioridade
// NEGATIVA e cede a vez a uma leitura de conteúdo, se houver: assim "aqui" vence "variante de 這裡" e o
// sentido comum vence o sobrenome na definição principal. As marcas são sempre detectadas no INGLÊS
// original e casadas por POSIÇÃO da leitura (as traduções não têm "variant of"/"surname" para casar; o
// pt-BR preserva a ordem por ser derivado linha a linha). Esse mesmo rótulo é GRAVADO em cada leitura
// no campo "tipo" (dicionario.TIPO_LEITURA_*, ausente = conteúdo), para o consumidor distinguir
// variante/sobrenome sem reprocessar a glosa e independentemente do idioma. A promoção é evitada, quando
// embaralharia a associação tradicional↔leitura (leitura promovida com tradicional próprio em entrada
// de grafias divergentes, ex. 台风 ← 颱風/臺風) — essas ficam integralmente aninhadas.
//
// Uso (de dentro de wails_app/, por causa do import do pacote dicionario):
//
//	go run ./dicionario/fusao [idioma]
//
// O idioma padrão é "en"; para outros idiomas, cada arquivo-fonte ausente cai para o inglês
// (mesma regra de fallback do app).
//
// Entra de dicionario/fontes/ (insumos de build, FORA do //go:embed idiomas) e sai em
// dicionario/idiomas/<idioma>/dicionario.jsonl.gz — a fonte única que o app embarca e lê em runtime
// (ver banco.go). O .gz é VERSIONADO, então a gravação tem que ser byte a byte determinística: o
// cabeçalho gzip vai sem nome e sem timestamp de propósito (senão cada geração viraria um blob novo
// no git mesmo com as fontes intactas).
//
// Junto sai uma cópia PLANA (sem gzip) em dicionario/fusao/saida/<idioma>/dicionario.jsonl, só para
// visualização e apuração humana (grep, diff, abrir no editor) — não é lida pelo app nem pelo build,
// fica fora do //go:embed idiomas de propósito e não é versionada (ver .gitignore): é 100% derivável
// do .gz, que já é a fonte de verdade.
package main

import (
	"bufio"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"wails_app/dicionario"
)

const (
	// Insumos de build: ficam em fontes/ e NÃO entram no binário (o //go:embed idiomas é recursivo,
	// e embarcar os ~25 MB crus além do fundido seria puro desperdício).
	dirFontes = "dicionario/fontes"
	// A lista de frequência é neutra de idioma (mede o mandarim, não a língua da tradução), então
	// mora na raiz de fontes/ — sem pasta de idioma e sem fallback.
	arquivoFrequencia = "dicionario/fontes/frequencia_opensubtitles.txt.gz"
	// Produto: a fonte única que o app embarca e lê (ver dicionario/banco.go).
	moldeSaida = "dicionario/idiomas/%s/dicionario.jsonl.gz"
	// Cópia plana para inspeção humana — fora do embed, fora do git (ver comentário do pacote).
	moldeSaidaTexto = "dicionario/fusao/saida/%s/dicionario.jsonl"
)

// DIGITOS_FREQUENCIA é a precisão, em dígitos significativos, do percentual de uso gravado. Quatro
// descrevem tanto o topo (4,629%) quanto a cauda (1,17e-06%) sem arrastar para o JSONL os ~17
// dígitos que o float64 imprimiria por padrão.
const DIGITOS_FREQUENCIA = 4

// MARCA_VARIANTE é o texto que o CC-CEDICT usa numa glosa que só aponta para outra grafia ("variant
// of X", "old variant of X", "archaic variant of X", ...). Todas as variações contêm esta subcadeia,
// então casar por ela identifica a leitura-ponteiro que deve ceder a ascensão ao topo (ver
// leituraEhVariante). Detecta-se sempre no inglês original — as traduções não têm essa frase.
const MARCA_VARIANTE = "variant of"

// MARCA_SOBRENOME é a palavra com que o CC-CEDICT abre uma glosa que só registra um sobrenome
// ("surname Wang", "surname Ouyang"). Uma leitura cujos significados são apenas isso não carrega
// sentido de estudo próprio e cede a ascensão ao topo como a variante (ver leituraEhSobrenome).
// Detecta-se sempre no inglês original — a tradução troca "surname" por "sobrenome".
const MARCA_SOBRENOME = "surname"

// MAX_PALAVRAS_ALEM_DO_SOBRENOME limita quantas palavras podem seguir "surname" para a glosa ainda
// contar como só-sobrenome: o bastante para acomodar o nome (ex.: "surname Ouyang Xiu"), não uma
// acepção de conteúdo que só por acaso comece com a palavra.
const MAX_PALAVRAS_ALEM_DO_SOBRENOME = 2

// ----- Estruturas de saída -----

type Etimologia struct {
	Tipo      string `json:"tipo,omitempty"`
	Fonetica  string `json:"fonetica,omitempty"`
	Semantica string `json:"semantica,omitempty"`
	Dica      string `json:"dica,omitempty"`
}

// Frequencia é o uso da palavra no corpus do OpenSubtitles. Percentual é a fatia que ela ocupa no
// corpus INTEIRO; Posicao é o lugar dela no ranking disputado só entre as palavras que o dicionário
// conhece (1 = a mais comum). Ver aplicarFrequencias para o porquê de cada universo.
type Frequencia struct {
	Percentual float64 `json:"percentual"`
	Posicao    int     `json:"posicao"`
}

// Leitura corresponde a uma linha original do CEDICT para a palavra. Todos os campos são
// omitidos quando redundantes com o topo — ver regras de reconstrução no cabeçalho. Tipo é o rótulo
// language-neutral da leitura (dicionario.TIPO_LEITURA_*, ausente = conteúdo), detectado no inglês
// original para o consumidor distinguir variante/sobrenome sem depender do idioma da glosa.
type Leitura struct {
	Pinyin       string   `json:"pinyin,omitempty"`
	Tradicional  string   `json:"tradicional,omitempty"`
	Significados []string `json:"significados,omitempty"`
	Tipo         string   `json:"tipo,omitempty"`
}

type EntradaFundida struct {
	Simplificado     string          `json:"simplificado"`
	Tradicional      []string        `json:"tradicional,omitempty"`
	Definicao        string          `json:"definicao,omitempty"` // definição curada do makemeahanzi
	Pinyin           []string        `json:"pinyin,omitempty"`    // leituras curadas do makemeahanzi
	Frequencia       *Frequencia     `json:"frequencia,omitempty"`
	Leituras         []Leitura       `json:"leituras,omitempty"`
	Decomposicao     string          `json:"decomposicao,omitempty"`
	Etimologia       *Etimologia     `json:"etimologia,omitempty"`
	Radical          string          `json:"radical,omitempty"`
	Correspondencias json.RawMessage `json:"correspondencias,omitempty"` // campo "matches" original
}

// ----- Estruturas de entrada -----

type entradaMakemeahanzi struct {
	Caractere    string          `json:"character"`
	Definicao    string          `json:"definition"`
	Pinyin       []string        `json:"pinyin"`
	Decomposicao string          `json:"decomposition"`
	Radical      string          `json:"radical"`
	Matches      json.RawMessage `json:"matches"`

	EtimologiaBruta *struct {
		Tipo      string `json:"type"`
		Fonetica  string `json:"phonetic"`
		Semantica string `json:"semantic"`
		Dica      string `json:"hint"`
	} `json:"etymology"`
}

type linhaCedict struct {
	trad, simp, pinyin string
	significados       []string
}

func main() {
	idioma := "en"
	if len(os.Args) > 1 {
		idioma = os.Args[1]
	}

	caminhoMakemeahanzi := resolverComFallback(idioma, "makemeahanzi.txt")
	caminhoCedict := resolverComFallback(idioma, "cedict.u8")
	caminhoCedictOriginal := fmt.Sprintf("%s/en/cedict.u8", dirFontes)
	caminhoSaida := fmt.Sprintf(moldeSaida, idioma)
	caminhoSaidaTexto := fmt.Sprintf(moldeSaidaTexto, idioma)
	fmt.Printf("Fontes: %s + %s\n", caminhoMakemeahanzi, caminhoCedict)

	entradasMmah := carregarMakemeahanzi(caminhoMakemeahanzi)
	linhasCedict := carregarCedict(caminhoCedict)
	fmt.Printf("Carregado: %d caracteres do makemeahanzi, %d linhas do CEDICT\n",
		len(entradasMmah), len(linhasCedict))

	// As marcas "variant of"/"surname" só existem no INGLÊS (a tradução as troca por "variante de",
	// "sobrenome" etc.), então o rótulo de cada leitura é sempre calculado sobre o original e casado por
	// posição da leitura — o pt-BR preserva a ordem por ser derivado linha a linha do en. É esse rótulo
	// que decide a prioridade negativa E vira o flag "tipo" gravado em cada leitura.
	linhasCedictOriginal := linhasCedict
	if caminhoCedict != caminhoCedictOriginal {
		linhasCedictOriginal = carregarCedict(caminhoCedictOriginal)
	}
	tiposPorSimp := detectarTiposDeLeituraPorSimplificado(linhasCedictOriginal)

	// ----- Classificação simplificado/tradicional a partir das colunas do CEDICT -----

	runasColunaSimp := map[rune]bool{}
	runasColunaTrad := map[rune]bool{}
	tradParaSimp := map[rune]map[rune]bool{} // alinhamento caractere-a-caractere
	for _, l := range linhasCedict {
		for _, r := range l.simp {
			runasColunaSimp[r] = true
		}
		for _, r := range l.trad {
			runasColunaTrad[r] = true
		}
		runasT, runasS := []rune(l.trad), []rune(l.simp)
		if l.trad == l.simp || len(runasT) != len(runasS) {
			continue
		}
		for i := range runasT {
			if runasT[i] == runasS[i] {
				continue
			}
			if tradParaSimp[runasT[i]] == nil {
				tradParaSimp[runasT[i]] = map[rune]bool{}
			}
			tradParaSimp[runasT[i]][runasS[i]] = true
		}
	}

	// ----- Passo 1: linhas do CEDICT viram entradas por simplificado, com leituras aninhadas -----

	entradas := map[string]*EntradaFundida{}
	ordemChegada := []string{} // para variantes na ordem em que o CEDICT as apresenta
	obter := func(simplificado string) *EntradaFundida {
		if e, existe := entradas[simplificado]; existe {
			return e
		}
		e := &EntradaFundida{Simplificado: simplificado}
		entradas[simplificado] = e
		ordemChegada = append(ordemChegada, simplificado)
		return e
	}

	for _, l := range linhasCedict {
		e := obter(l.simp)
		leitura := Leitura{
			Pinyin:       dicionario.ConverterPinyin(l.pinyin),
			Significados: removerPinyinDosSignificados(l.significados),
		}
		if l.trad != l.simp {
			leitura.Tradicional = l.trad
			adicionarVariante(e, l.trad)
		}
		e.Leituras = append(e.Leituras, leitura)
	}

	// ----- Passo 2: sobrepor os campos curados do makemeahanzi (caracteres que ficam) -----

	var deletados []entradaMakemeahanzi
	mantidosSemCedict := 0
	for _, m := range entradasMmah {
		runas := []rune(m.Caractere)
		ehTradicionalPuro := len(runas) == 1 &&
			runasColunaTrad[runas[0]] && !runasColunaSimp[runas[0]] &&
			len(tradParaSimp[runas[0]]) > 0
		if ehTradicionalPuro {
			deletados = append(deletados, m)
			continue
		}

		e := obter(m.Caractere)
		if len(e.Leituras) == 0 {
			mantidosSemCedict++
		}
		e.Definicao = m.Definicao
		e.Pinyin = m.Pinyin
		e.Decomposicao = m.Decomposicao
		e.Radical = m.Radical
		e.Correspondencias = m.Matches
		if m.EtimologiaBruta != nil {
			e.Etimologia = &Etimologia{
				Tipo:      m.EtimologiaBruta.Tipo,
				Fonetica:  m.EtimologiaBruta.Fonetica,
				Semantica: m.EtimologiaBruta.Semantica,
				Dica:      m.EtimologiaBruta.Dica,
			}
		}
	}

	// ----- Passo 3: caracteres tradicionais deletados viram variantes do simplificado -----

	variantesAcrescentadas, definicoesHerdadas := 0, 0
	var relatorioHerdadas []string
	for _, m := range deletados {
		runaTrad := []rune(m.Caractere)[0]
		alvo := escolherAlvo(runaTrad, tradParaSimp[runaTrad], entradas)
		e := obter(alvo)
		if adicionarVariante(e, m.Caractere) {
			variantesAcrescentadas++
		}
		// A entrada simplificada herda a definição curada quando ela própria não tem uma
		// (os ~36 casos cuja contraparte não existe no makemeahanzi). Decomposição/etimologia
		// NÃO são herdadas: descrevem o glifo tradicional, não o simplificado.
		if e.Definicao == "" && m.Definicao != "" {
			e.Definicao = m.Definicao
			if len(e.Pinyin) == 0 {
				e.Pinyin = m.Pinyin
			}
			definicoesHerdadas++
			relatorioHerdadas = append(relatorioHerdadas, fmt.Sprintf("%s→%s", m.Caractere, alvo))
		}
	}

	// ----- Passo 4: percentual de uso e posição no ranking, da lista de frequência -----

	contagens, totalTokens := carregarFrequencias(arquivoFrequencia)
	entradasComFrequencia, coberturaCorpus := aplicarFrequencias(entradas, contagens, totalTokens)

	// ----- Passo 5: compactação — promove a 1ª leitura de conteúdo onde falta topo e remove redundâncias -----

	var comp contadoresCompactacao
	for _, e := range entradas {
		c := compactarEntrada(e, tiposPorSimp[e.Simplificado])
		comp.promovidas += c.promovidas
		comp.promocoesEvitadas += c.promocoesEvitadas
		comp.promocoesDesviadas += c.promocoesDesviadas
		comp.pinyinsLimpos += c.pinyinsLimpos
		comp.tradsLimpos += c.tradsLimpos
	}

	// ----- Escrita ordenada -----

	chaves := make([]string, 0, len(entradas))
	for chave := range entradas {
		chaves = append(chaves, chave)
	}
	sort.Strings(chaves)

	abortarSe(os.MkdirAll(filepath.Dir(caminhoSaida), 0755))
	arquivoSaida, err := os.Create(caminhoSaida)
	abortarSe(err)
	defer arquivoSaida.Close()

	// Cabeçalho gzip sem nome e sem ModTime: o .gz é versionado, e o timestamp default faria cada
	// geração render um blob diferente no git mesmo sem nenhuma fonte ter mudado.
	escritorGz, err := gzip.NewWriterLevel(arquivoSaida, gzip.BestCompression)
	abortarSe(err)
	escritorGz.Name = ""
	escritorGz.ModTime = time.Time{}
	abortarSe(escreverEntradas(escritorGz, chaves, entradas))
	abortarSe(escritorGz.Close())

	abortarSe(os.MkdirAll(filepath.Dir(caminhoSaidaTexto), 0755))
	arquivoSaidaTexto, err := os.Create(caminhoSaidaTexto)
	abortarSe(err)
	defer arquivoSaidaTexto.Close()
	abortarSe(escreverEntradas(arquivoSaidaTexto, chaves, entradas))

	// ----- Relatório -----

	fmt.Printf("\nSaída: %s (%d linhas)\n", caminhoSaida, len(chaves))
	fmt.Printf("Saída plana (visualização/apuração, não versionada): %s\n", caminhoSaidaTexto)
	fmt.Printf("Caracteres do makemeahanzi deletados (tradicionais puros): %d\n", len(deletados))
	fmt.Printf("Mantidos sem nenhuma linha no CEDICT (radicais/componentes): %d\n", mantidosSemCedict)
	fmt.Printf("Variantes acrescentadas além das já vistas nas leituras: %d\n", variantesAcrescentadas)
	fmt.Printf("Definições herdadas de tradicional deletado: %d (%s)\n",
		definicoesHerdadas, strings.Join(relatorioHerdadas, " "))
	fmt.Printf("Compactação: %d promovidas ao topo (%d desviaram de uma leitura secundária: variante/sobrenome), "+
		"%d promoções evitadas (grafias divergentes), %d leituras sem pinyin repetido, "+
		"%d entradas sem tradicional repetido\n",
		comp.promovidas, comp.promocoesDesviadas, comp.promocoesEvitadas, comp.pinyinsLimpos, comp.tradsLimpos)
	fmt.Printf("Frequência: %d entradas com percentual de uso (%.1f%% do dicionário), cobrindo "+
		"%.2f%% dos %d tokens do corpus\n", entradasComFrequencia,
		float64(entradasComFrequencia)/float64(len(entradas))*100, coberturaCorpus, totalTokens)

	fmt.Println("\n--- Casos de verificação ---")
	for _, caso := range []string{"的", "发", "後", "后", "干", "好", "下面", "台风", "⺈"} {
		imprimirResumo(caso, entradas[caso])
	}
}

// ----- Regras da fusão -----

// aplicarFrequencias grava em cada entrada conhecida pela lista o percentual de uso e a posição no
// ranking. Devolve quantas entradas receberam frequência e que fatia do corpus elas cobrem.
//
// Os dois números têm universos DIFERENTES, de propósito:
//   - Percentual divide pelo corpus inteiro (ver carregarFrequencias) — é a fatia do mandarim
//     falado que a palavra ocupa, e essa fatia não muda porque o dicionário ignora um token.
//   - Posicao é disputada só entre palavras que o dicionário conhece: "#12" quer dizer "a 12ª mais
//     comum ENTRE AS QUE DÁ PARA ESTUDAR AQUI". Deixar "gucci" e nomes próprios empurrarem palavra
//     real para baixo não diria nada a quem estuda.
//
// A ordem sai da CONTAGEM crua, não do percentual gravado: o arredondamento a DIGITOS_FREQUENCIA
// inventaria empates entre contagens vizinhas. Empate real recebe a MESMA posição e o seguinte pula
// (ranking de competição) — palavras igualmente comuns não podem ter posições diferentes decididas
// por desempate arbitrário, e isso mantém a saída idêntica a cada geração.
//
// Consequência de exibição: as posições PULAM, e a última não é o total de entradas. A cauda vista
// 1× no corpus é um empate enorme, então em en a maior posição é 67.216 para 73.580 palavras — o
// app não pode escrever "#67216 de 73580".
func aplicarFrequencias(entradas map[string]*EntradaFundida, contagens map[string]int64, totalTokens int64) (int, float64) {
	type palavraContada struct {
		palavra  string
		contagem int64
	}

	ranqueadas := make([]palavraContada, 0, len(contagens))
	for palavra, contagem := range contagens {
		if _, existe := entradas[palavra]; !existe {
			continue // token que o dicionário não conhece: nome próprio, latim, ruído de segmentação
		}
		ranqueadas = append(ranqueadas, palavraContada{palavra: palavra, contagem: contagem})
	}
	sort.Slice(ranqueadas, func(i, j int) bool {
		if ranqueadas[i].contagem != ranqueadas[j].contagem {
			return ranqueadas[i].contagem > ranqueadas[j].contagem
		}
		return ranqueadas[i].palavra < ranqueadas[j].palavra
	})

	coberturaCorpus := 0.0
	posicao, contagemDaPosicao := 0, int64(-1)
	for i, r := range ranqueadas {
		if r.contagem != contagemDaPosicao {
			posicao = i + 1
			contagemDaPosicao = r.contagem
		}
		percentual := float64(r.contagem) / float64(totalTokens) * 100
		entradas[r.palavra].Frequencia = &Frequencia{
			Percentual: arredondarSignificativos(percentual, DIGITOS_FREQUENCIA),
			Posicao:    posicao,
		}
		coberturaCorpus += percentual
	}
	return len(ranqueadas), coberturaCorpus
}

// contadoresCompactacao acumula o que a compactação fez em cada entrada, só para o relatório final.
type contadoresCompactacao struct {
	promovidas         int
	promocoesEvitadas  int
	promocoesDesviadas int // promoções que pularam uma leitura secundária (variante/sobrenome) para uma de conteúdo
	pinyinsLimpos      int
	tradsLimpos        int
}

// compactarEntrada aplica as regras de compactação do cabeçalho: completa o topo com a primeira
// leitura de CONTEÚDO quando o makemeahanzi não forneceu definicao/pinyin, e apaga das leituras os
// campos que o topo passa a responder (reconstrução determinística — regras 1 a 4 do cabeçalho).
// `tipos` traz, na ordem das leituras, o rótulo de cada uma (variante/sobrenome/"", calculado no inglês
// original — ver detectarTiposDeLeituraPorSimplificado): as leituras rotuladas cedem a ascensão ao topo.
func compactarEntrada(e *EntradaFundida, tipos []string) contadoresCompactacao {
	if len(e.Leituras) == 0 {
		return contadoresCompactacao{}
	}

	// Grava o rótulo (variante/sobrenome/"") em cada leitura ANTES de qualquer promoção/filtragem, para
	// ele sobreviver inclusive ao early return da promoção evitada. Depois disso, todo o resto lê o tipo
	// direto da leitura.
	for i := range e.Leituras {
		e.Leituras[i].Tipo = tipoDaLeitura(tipos, i)
	}

	// Uniformidade da grafia tradicional entre TODAS as leituras, calculada ANTES de qualquer
	// remoção: só é seguro apagar o tradicional das leituras quando todas dizem a mesma coisa
	// (senão casos como 后 — "rainha" nunca vira 後 — perderiam a exceção).
	uniforme := true
	tradUnico := e.Leituras[0].Tradicional
	for _, l := range e.Leituras[1:] {
		if l.Tradicional != tradUnico {
			uniforme = false
			break
		}
	}

	// A leitura que ascende é a primeira de CONTEÚDO (tipo ""): uma leitura rotulada (só "variant of ..."
	// ou só "surname ...") cede a vez, para "aqui" vencer "variante de 這裡" e o sentido comum vencer o
	// sobrenome. Se TODAS forem rotuladas, promove a primeira mesmo — é tudo que a palavra tem.
	escolhida := 0
	for i := range e.Leituras {
		if e.Leituras[i].Tipo == "" {
			escolhida = i
			break
		}
	}

	var contadores contadoresCompactacao
	semTopo := e.Definicao == "" && len(e.Pinyin) == 0
	if semTopo {
		// Promover a leitura escolhida embaralharia a associação tradicional↔leitura quando ela tem
		// grafia própria e as demais divergem (ex. 台风 ← 颱風/臺風): fica tudo aninhado.
		if e.Leituras[escolhida].Tradicional != "" && !uniforme {
			contadores.promocoesEvitadas = 1
			return contadores
		}
		e.Definicao = strings.Join(e.Leituras[escolhida].Significados, dicionario.SEPARADOR_ACEPCOES)
		e.Pinyin = []string{e.Leituras[escolhida].Pinyin}
		contadores.promovidas = 1
		if escolhida > 0 {
			contadores.promocoesDesviadas = 1
		}
	}

	// Redundâncias: tradicional coberto pela regra 3, pinyin pela regra 2, significados que
	// são exatamente a definição do topo (surge na leitura promovida e em duplicatas reais).
	if uniforme && tradUnico != "" {
		for i := range e.Leituras {
			e.Leituras[i].Tradicional = ""
		}
		contadores.tradsLimpos = 1
	}
	for i := range e.Leituras {
		// Comparação SEM caixa: o makemeahanzi capitaliza o pinyin de nome próprio/lugar (topo "Wǎ" de
		// 邷 "old place name") enquanto o CEDICT o traz minúsculo ("wǎ") — é a mesma sílaba, e mantê-la
		// na leitura só repetiria o que o topo já responde na reconstrução (regra 2). Tons diferem no
		// diacrítico (ǎ≠à), então EqualFold nunca funde leituras de tom distinto.
		if len(e.Pinyin) > 0 && strings.EqualFold(e.Leituras[i].Pinyin, e.Pinyin[0]) {
			e.Leituras[i].Pinyin = ""
			contadores.pinyinsLimpos++
		}
		if e.Definicao != "" && strings.Join(e.Leituras[i].Significados, dicionario.SEPARADOR_ACEPCOES) == e.Definicao {
			e.Leituras[i].Significados = nil
		}
	}

	// Leituras que ficaram sem nenhum campo já estão integralmente representadas no topo.
	restantes := e.Leituras[:0]
	for _, l := range e.Leituras {
		if l.Pinyin != "" || l.Tradicional != "" || len(l.Significados) > 0 {
			restantes = append(restantes, l)
		}
	}
	e.Leituras = restantes

	return contadores
}

// detectarTiposDeLeituraPorSimplificado devolve, por simplificado, o rótulo de cada leitura (na ordem
// de aparição no CEDICT): dicionario.TIPO_LEITURA_VARIANTE, dicionario.TIPO_LEITURA_SOBRENOME ou ""
// (conteúdo). Recebe SEMPRE o inglês original: a compactação casa esses rótulos por posição da leitura,
// e a tradução não tem "variant of"/"surname" para casar. É a fonte do flag gravado em cada leitura.
func detectarTiposDeLeituraPorSimplificado(linhas []linhaCedict) map[string][]string {
	tipos := map[string][]string{}
	for _, l := range linhas {
		tipos[l.simp] = append(tipos[l.simp], classificarLeitura(l.significados))
	}
	return tipos
}

// classificarLeitura rotula uma leitura do CEDICT pela natureza da glosa. Variante e sobrenome são
// mutuamente exclusivos (exigem TODOS os significados de um tipo), então a ordem do teste não importa.
// Leitura com sentido de estudo próprio devolve "" (conteúdo) e não cede a ascensão ao topo.
func classificarLeitura(significados []string) string {
	if leituraEhVariante(significados) {
		return dicionario.TIPO_LEITURA_VARIANTE
	}
	if leituraEhSobrenome(significados) {
		return dicionario.TIPO_LEITURA_SOBRENOME
	}
	return ""
}

// leituraEhVariante diz se uma leitura do CEDICT não carrega significado próprio — TODOS os seus
// significados são ponteiros "variant of ...".
func leituraEhVariante(significados []string) bool {
	if len(significados) == 0 {
		return false
	}
	for _, s := range significados {
		if !strings.Contains(strings.ToLower(s), MARCA_VARIANTE) {
			return false
		}
	}
	return true
}

// leituraEhSobrenome diz se uma leitura do CEDICT só registra sobrenome — TODOS os seus significados
// são a palavra "surname" seguida de no máximo MAX_PALAVRAS_ALEM_DO_SOBRENOME palavras (o nome).
// "king; surname Wang" NÃO conta (a acepção "king" é conteúdo), nem uma glosa longa que só por acaso
// comece com a palavra.
func leituraEhSobrenome(significados []string) bool {
	if len(significados) == 0 {
		return false
	}
	for _, s := range significados {
		if !significadoEhSoSobrenome(s) {
			return false
		}
	}
	return true
}

// significadoEhSoSobrenome diz se uma glosa é apenas "surname" mais o nome (até
// MAX_PALAVRAS_ALEM_DO_SOBRENOME palavras). A comparação é da PRIMEIRA palavra, no inglês original.
func significadoEhSoSobrenome(significado string) bool {
	palavras := strings.Fields(significado)
	if len(palavras) == 0 || len(palavras) > 1+MAX_PALAVRAS_ALEM_DO_SOBRENOME {
		return false
	}
	return strings.ToLower(palavras[0]) == MARCA_SOBRENOME
}

// escreverEntradas grava uma linha JSON por chave, na ordem dada, no escritor informado — usada tanto
// para o .jsonl.gz embarcado quanto para a cópia plana de visualização/apuração (ver main).
func escreverEntradas(escritor io.Writer, chaves []string, entradas map[string]*EntradaFundida) error {
	bufonado := bufio.NewWriterSize(escritor, 1<<20)
	codificador := json.NewEncoder(bufonado)
	codificador.SetEscapeHTML(false)
	for _, chave := range chaves {
		if err := codificador.Encode(entradas[chave]); err != nil {
			return err
		}
	}
	return bufonado.Flush()
}

// adicionarVariante acrescenta uma forma tradicional à lista da entrada, sem duplicar.
// Retorna true se a forma ainda não estava presente.
func adicionarVariante(e *EntradaFundida, tradicional string) bool {
	for _, v := range e.Tradicional {
		if v == tradicional {
			return false
		}
	}
	e.Tradicional = append(e.Tradicional, tradicional)
	return true
}

// escolherAlvo decide a qual entrada simplificada um caractere tradicional deletado se anexa.
// Preferência: alvo cuja leitura já cita esse tradicional; depois alvo com entrada existente;
// por fim o primeiro candidato em ordem de codepoint (raro: 16 tradicionais têm >1 simplificado).
func escolherAlvo(runaTrad rune, candidatos map[rune]bool, entradas map[string]*EntradaFundida) string {
	ordenados := make([]string, 0, len(candidatos))
	for r := range candidatos {
		ordenados = append(ordenados, string(r))
	}
	sort.Strings(ordenados)

	for _, c := range ordenados {
		e, existe := entradas[c]
		if !existe {
			continue
		}
		for _, l := range e.Leituras {
			if l.Tradicional == string(runaTrad) {
				return c
			}
		}
	}
	for _, c := range ordenados {
		if _, existe := entradas[c]; existe {
			return c
		}
	}
	return ordenados[0]
}

// ----- Carga dos arquivos-fonte -----

func carregarMakemeahanzi(caminho string) []entradaMakemeahanzi {
	arquivo, err := os.Open(caminho)
	abortarSe(err)
	defer arquivo.Close()

	var entradas []entradaMakemeahanzi
	varredor := bufio.NewScanner(arquivo)
	varredor.Buffer(make([]byte, 1<<20), 1<<20)
	for varredor.Scan() {
		linha := varredor.Text()
		if linha == "" {
			continue
		}
		var e entradaMakemeahanzi
		if err := json.Unmarshal([]byte(linha), &e); err != nil || e.Caractere == "" {
			fmt.Printf("Aviso: linha ignorada do makemeahanzi: %.60s\n", linha)
			continue
		}
		entradas = append(entradas, e)
	}
	abortarSe(varredor.Err())
	return entradas
}

func carregarCedict(caminho string) []linhaCedict {
	arquivo, err := os.Open(caminho)
	abortarSe(err)
	defer arquivo.Close()

	var linhas []linhaCedict
	varredor := bufio.NewScanner(arquivo)
	varredor.Buffer(make([]byte, 1<<20), 1<<20)
	for varredor.Scan() {
		linha := varredor.Text()
		if strings.HasPrefix(linha, "#") || strings.TrimSpace(linha) == "" {
			continue
		}
		// Formato: Tradicional Simplificado [pin1 yin1] /significado 1/significado 2/
		partes := strings.SplitN(linha, " [", 2)
		if len(partes) != 2 {
			continue
		}
		palavras := strings.Fields(partes[0])
		if len(palavras) != 2 {
			continue
		}
		resto := strings.SplitN(partes[1], "] /", 2)
		if len(resto) != 2 {
			continue
		}
		linhas = append(linhas, linhaCedict{
			trad:         palavras[0],
			simp:         palavras[1],
			pinyin:       resto[0],
			significados: strings.Split(strings.TrimSuffix(resto[1], "/"), "/"),
		})
	}
	abortarSe(varredor.Err())
	return linhas
}

// carregarFrequencias lê a lista de frequência (uma linha "palavra<espaço>contagem") e devolve a
// contagem de cada palavra mais o total de tokens do corpus — o denominador de todo percentual.
//
// Esse total é o corpus INTEIRO, não só os tokens que casam com o dicionário: o percentual responde
// "que fatia do mandarim falado esta palavra ocupa", então nomes próprios, latim e ruído de
// segmentação têm de continuar pesando no total — descartá-los inflaria todo mundo.
//
// O casamento (em aplicarFrequencias) é só pela grafia SIMPLIFICADA, a chave do dicionário, que é o
// que a lista zh_cn traz por construção. Formas tradicionais não são somadas à entrada simplificada
// de propósito: em caracteres válidos nas duas grafias (后/後) isso contaria o token duas vezes.
func carregarFrequencias(caminho string) (map[string]int64, int64) {
	arquivo, err := os.Open(caminho)
	abortarSe(err)
	defer arquivo.Close()

	leitorGz, err := gzip.NewReader(arquivo)
	abortarSe(err)
	defer leitorGz.Close()

	contagens := map[string]int64{}
	var totalTokens int64
	varredor := bufio.NewScanner(leitorGz)
	varredor.Buffer(make([]byte, 1<<20), 1<<20)
	for varredor.Scan() {
		linha := varredor.Text()
		if strings.TrimSpace(linha) == "" {
			continue
		}
		palavra, contagemTexto, achou := strings.Cut(linha, " ")
		contagem, err := strconv.ParseInt(contagemTexto, 10, 64)
		if !achou || palavra == "" || err != nil {
			fmt.Printf("Aviso: linha ignorada da lista de frequência: %.60s\n", linha)
			continue
		}
		contagens[palavra] += contagem
		totalTokens += contagem
	}
	abortarSe(varredor.Err())
	if totalTokens == 0 {
		abortarSe(fmt.Errorf("lista de frequência %s não rendeu nenhuma contagem", caminho))
	}
	return contagens, totalTokens
}

// resolverComFallback devolve o caminho da FONTE no idioma pedido, caindo para o inglês quando o
// arquivo não existir (mesma regra de lerRecursoIdioma no app, só que sobre fontes/ em vez do embed).
func resolverComFallback(idioma, nomeArquivo string) string {
	caminho := fmt.Sprintf("%s/%s/%s", dirFontes, idioma, nomeArquivo)
	if _, err := os.Stat(caminho); err == nil {
		return caminho
	}
	return fmt.Sprintf("%s/en/%s", dirFontes, nomeArquivo)
}

// ----- Apoio -----

// tipoDaLeitura devolve o rótulo (dicionario.TIPO_LEITURA_*, "" = conteúdo) da leitura na posição i,
// segundo os tipos calculados no inglês original. Índice fora do alcance (desalinhamento improvável
// entre idioma e original) conta como CONTEÚDO — preserva o comportamento antigo de promover a 1ª leitura.
func tipoDaLeitura(tipos []string, i int) string {
	if i < len(tipos) {
		return tipos[i]
	}
	return ""
}

var padraoPinyinAposHanzi = regexp.MustCompile(`(\p{Han})\[[^\]]*\]`)

// removerPinyinAposHanzi apaga o pinyin entre colchetes colado a um hanzi citado na glosa do CEDICT
// (ex.: "variant of 臺[tai2]" → "variant of 臺", "CL:個|个[ge4]" → "CL:個|个"). Colchetes que NÃO
// seguem hanzi (ex.: "Taiwan pr. [tai2]") são conteúdo e ficam. O laço cobre colchetes em sequência
// ("臺[tai2][xxx]"), que o ReplaceAll sozinho deixaria escapar. É a MESMA remoção do extrator, aqui
// aplicada às fontes traduzidas (a mesclagem não reinjeta mais) e ao inglês original, que fica intacto.
func removerPinyinAposHanzi(texto string) string {
	for {
		novo := padraoPinyinAposHanzi.ReplaceAllString(texto, "${1}")
		if novo == texto {
			return novo
		}
		texto = novo
	}
}

// removerPinyinDosSignificados aplica removerPinyinAposHanzi a cada significado de uma leitura.
func removerPinyinDosSignificados(significados []string) []string {
	limpos := make([]string, len(significados))
	for i, s := range significados {
		limpos[i] = removerPinyinAposHanzi(s)
	}
	return limpos
}

func imprimirResumo(chave string, e *EntradaFundida) {
	if e == nil {
		fmt.Printf("%s: SEM LINHA PRÓPRIA (esperado para tradicional deletado)\n", chave)
		return
	}
	frequencia := "ausente (fora do corpus)"
	if e.Frequencia != nil {
		frequencia = fmt.Sprintf("%g%% (#%d)", e.Frequencia.Percentual, e.Frequencia.Posicao)
	}
	fmt.Printf("%s: trad=%v leituras=%d freq=%s def=%.40q decomp=%v\n",
		chave, e.Tradicional, len(e.Leituras), frequencia, e.Definicao, e.Decomposicao != "")
}

// arredondarSignificativos corta o float para N dígitos significativos, mantendo a escala (0,004629
// vira 0,004629, não 0,0046). É o que impede o JSONL de gravar 4.628938204103...e+00 no lugar de
// 4.629 — o float64 imprime todos os dígitos que precisa para round-trip, e a fusão não quer isso.
func arredondarSignificativos(valor float64, digitos int) float64 {
	arredondado, err := strconv.ParseFloat(strconv.FormatFloat(valor, 'g', digitos, 64), 64)
	abortarSe(err)
	return arredondado
}

func abortarSe(err error) {
	if err != nil {
		fmt.Println("ERRO:", err)
		os.Exit(1)
	}
}
