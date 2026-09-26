package main

// ----- Seção: Revisão das frases complementares traduzidas pelo Google (veredicto + reparo DeepSeek) -----
//
// Passo de QUALIDADE do complementar. O Google Tradutor não traduz chinês→português direto: ele PIVOTA
// pelo inglês (os metadados da resposta entregam os modelos `zh_en` seguido de `en_pt`). O pivô estraga
// sistematicamente as frases elípticas, campeãs no Tatoeba:
//
//	"是的, 他喜歡。"  →(en)  "Yes, he does."  →(pt)  "Sim, ele quer."      ← errado (喜歡 = gostar)
//	是的，他喜歡音樂。 →(pt)  "Sim, ele gosta de música."                    ← certo, sem elipse
//
// Aqui o DeepSeek dá um veredicto BINÁRIO por par (对/错, só isso, para poupar token de saída — mesmo
// esqueleto do frases/verificar) e REFAZ apenas as reprovadas, traduzindo DIRETO do chinês, sem inglês
// no meio. É o mesmo padrão verificar→montar que o pipeline já usa, com os papéis dos motores trocados.
//
//	Entrada:  frases/<idioma>/traducoes_google_complementares.tsv    (chinês <TAB> pt do Google)
//	Saídas:   frases/<idioma>/veredictos_complementares.tsv          (chinês <TAB> pt <TAB> ok|errado)
//	          frases/<idioma>/traducoes_deepseek_complementares.tsv  (chinês <TAB> pt do DeepSeek)
//
// Quem monta o arquivo embarcado continua sendo o `complementar` (escritor único): ele prefere a tradução
// do DeepSeek quando existe e cai na do Google no resto. Fluxo completo:
//
//	complementar  (Google traduz tudo)  →  revisar  (julga e refaz as erradas)  →  complementar  (remonta)
//
// A 2ª rodada do complementar não gasta rede: o cache do Google já cobre tudo.
//
// Retomável e crash-safe: cada lote é anexado ao arquivo assim que fecha, e uma rodada nova pula o que já
// tem veredicto (chave = chinês+tradução julgada). No fim os dois arquivos são reescritos ordenados, para
// ficarem estáveis entre rodadas. Passe "-refazer" para julgar tudo de novo do zero.
//
// Rodar da RAIZ do módulo wails_app:
//
//	go run ./dicionario/frases/revisar [idioma] [-refazer]

import (
	"errors"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"

	"wails_app/dicionario/frases/deepseek"
	"wails_app/dicionario/frases/idiomasalvo"
)

const (
	idiomaPadrao = "pt-BR"

	moldeCacheGoogle   = "dicionario/frases/%s/traducoes_google_complementares.tsv"
	moldeVeredictos    = "dicionario/frases/%s/veredictos_complementares.tsv"
	moldeCacheDeepSeek = "dicionario/frases/%s/traducoes_deepseek_complementares.tsv"

	VEREDICTO_OK     = "ok"
	VEREDICTO_ERRADO = "errado"

	PARES_POR_REQUISICAO   = 100 // veredicto: saída minúscula (só 对/错), cabe lote grande
	FRASES_POR_REQUISICAO  = 40  // tradução: saída longa, lote menor para caber no teto de tokens
	REQUISICOES_SIMULTANEAS = 6
	MAXIMO_PASSADAS        = 3

	MAXIMO_TOKENS_VEREDICTO = 4096
	MAXIMO_TOKENS_TRADUCAO  = 8192
)

// Prompts enviados em CHINÊS simplificado (não no idioma-alvo) para poupar tokens de ENTRADA — o
// tokenizador do DeepSeek representa chinês de forma bem mais compacta que línguas latinas, e é o motor
// pagando por entrada+saída em cada um dos milhares de lotes deste pipeline. Cada const abaixo vem
// comentada com o texto original em português, pra quem for ler/auditar o prompt não precisar saber
// chinês. O idioma-alvo entra por %s (o nomeChines do idiomasalvo: 巴西葡萄牙语, 西班牙语…), então o mesmo
// prompt serve qualquer idioma. Qualquer ajuste de conteúdo deve mudar os DOIS: o comentário (fonte da
// verdade humana) e a string chinesa (o que realmente vai pra API).

// INSTRUCOES_VEREDICTO recebe o nomeChines do idioma-alvo (2× %s). Em português:
//
//	Você é um revisor bilíngue de chinês mandarim e <idioma>. Sua tarefa é julgar se a tradução em
//	<idioma> transmite fielmente o sentido da frase em chinês.
const INSTRUCOES_VEREDICTO = `你是一位精通中文（普通话）和%s的双语审校员。你的任务是判断%s译文是否忠实传达了中文原句的意思。`

// MODELO_PROMPT_VEREDICTO recebe o nomeChines (2×) e as linhas numeradas "N. <chinês> ⇒ <tradução>"
// (%s final). Em português:
//
//	Cada linha abaixo tem o formato "N. FRASE_EM_CHINÊS ⇒ tradução em <idioma>".
//
//	Julgue, entrada por entrada, se a tradução transmite FIELMENTE o sentido da frase em chinês.
//
//	Responda UMA linha por entrada, no formato "N对" quando a tradução estiver correta ou "N错" quando
//	estiver errada — repita o número N e use APENAS o caractere chinês 对 (correta) ou 错 (errada). Não
//	escreva mais nada: nem espaços extras, nem pontuação, nem explicação, nem a tradução.
//
//	Estas traduções foram feitas por máquina passando pelo INGLÊS, então preste atenção especial ao
//	erro típico desse caminho: frase elíptica em que o verbo real se perde (o 喜欢 de "他喜欢" vira um
//	verbo genérico do tipo "querer/fazer" em vez do verbo certo), pronome trocado, e verbo genérico
//	(fazer/querer/ter) no lugar do verbo real da frase chinesa.
//
//	Considere ERRADA (错) quando houver erro de SENTIDO: pessoa/sujeito trocado (eu/você/ele/nós),
//	negação invertida, número ou tempo verbal errado, verbo trocado, omissão ou acréscimo que mude o
//	sentido, ou tradução que corresponde a outra frase. Considere CORRETA (对) quando só houver
//	diferença de estilo, sinônimos, pontuação ou ordem natural do idioma — tradução fiel não precisa
//	ser literal.
//
//	ENTRADAS:
//	%s
const MODELO_PROMPT_VEREDICTO = `下面每一行的格式是"N. 中文句子 ⇒ %s译文"。

请逐条判断%s译文是否忠实传达了中文原句的意思。

每条回答一行，格式为"N对"（译文正确）或"N错"（译文错误）——重复编号N，并且只使用汉字"对"或"错"。不要写任何其他内容：不要多余的空格、标点、解释，也不要写出译文本身。

这些译文是机器通过英语中转翻译的，所以要特别注意这种中转翻译常见的错误：省略句丢失了真正的动词（例如"他喜欢"里的"喜欢"被中转成泛泛的"要/做"之类的动词，而不是真正的动词）、代词搞错、用泛泛的动词（做/要/有）代替中文原句真正的动词。

出现语义错误时判为错误（错）：主语/人称搞错（我/你/他/我们）、否定颠倒、数量或时态错误、动词搞错、遗漏或增添内容改变了意思，或译文对应的是另一个句子。只有文体差异、同义词、标点或自然语序不同时判为正确（对）——忠实的翻译不需要逐字直译。

输入：
%s`

// INSTRUCOES_TRADUCAO recebe o nomeChines do idioma-alvo (2× %s). Em português:
//
//	Você é um tradutor de chinês mandarim para <idioma>. Traduz sempre DIRETAMENTE do chinês, nunca
//	passando pelo inglês. A saída é SEMPRE em <idioma> — nunca em chinês, seja simplificado ou
//	tradicional.
const INSTRUCOES_TRADUCAO = `你是一位把中文（普通话）翻译成%s的翻译员。翻译时始终直接从中文翻译，绝不经过英语中转。输出必须始终是%s——绝不能是未翻译的中文，无论简体还是繁体。`

// MODELO_PROMPT_TRADUCAO recebe o nomeChines (2×) e as linhas numeradas "N. <chinês>" (%s final). Em
// português:
//
//	Traduza para <idioma> cada frase em chinês abaixo.
//
//	Traduza DIRETAMENTE do chinês. Não passe pelo inglês: em frases elípticas ("他喜欢"), repita o verbo
//	real da frase chinesa, nunca um verbo genérico como "quer"/"faz".
//
//	A tradução DEVE estar em <idioma>. NUNCA devolva a frase em chinês sem traduzir — nem idêntica, nem
//	apenas convertida entre tradicional e simplificado. Isso não é uma tradução e a entrada será
//	descartada.
//
//	Responda UMA linha por entrada, no formato "N. tradução" — repita o número N seguido de ponto e da
//	tradução no idioma-alvo. Não escreva mais nada: nem o chinês, nem explicação, nem aspas extras além
//	das que a própria frase pedir. Preserve a pontuação de diálogo da frase original.
//
//	ENTRADAS:
//	%s
const MODELO_PROMPT_TRADUCAO = `请将下面每个中文句子翻译成%s。

请直接从中文翻译，不要经过英语中转：省略句（如"他喜欢"）必须还原中文原句真正的动词，绝不能用泛泛的动词（要/做/有之类）代替。

译文必须是%s。绝不能原样返回未翻译的中文——无论是完全相同，还是仅仅转换了简繁体。这不算翻译，该条会被丢弃。

每条回答一行，格式为"N. 译文"——重复编号N，加句点，然后是译文。不要写任何其他内容：不要写中文，不要解释，也不要加句子本身没有要求的多余引号。保留原句对话的标点。

输入：
%s`

// par é uma frase chinesa com a tradução do Google que será julgada.
type par struct {
	chines   string
	traducao string
}

// registrador anexa linhas a um arquivo de forma segura entre goroutines (crash-safe por lote).
type registrador struct {
	mu      sync.Mutex
	caminho string
}

func main() {
	idioma, refazer := lerArgumentos(os.Args[1:])
	alvo, ok := idiomasalvo.PorDir(idioma)
	if !ok {
		abortar(fmt.Errorf("idioma %q desconhecido — registre-o em frases/idiomasalvo antes", idioma))
	}
	fmt.Printf("Idioma-alvo da revisão: %s (%s)\n", alvo.Dir, alvo.NomeChines)

	caminhoGoogle := fmt.Sprintf(moldeCacheGoogle, idioma)
	pares := lerPares(caminhoGoogle)
	if len(pares) == 0 {
		abortar(fmt.Errorf("nenhuma tradução do Google em %q — rode antes: go run ./dicionario/frases/complementar", caminhoGoogle))
	}
	fmt.Printf("Traduções do Google a revisar: %d\n", len(pares))

	caminhoVeredictos := fmt.Sprintf(moldeVeredictos, idioma)
	veredictos := map[string]string{}
	if refazer {
		if err := os.Remove(caminhoVeredictos); err != nil && !os.IsNotExist(err) {
			abortar(err)
		}
	} else {
		veredictos = lerVeredictosExistentes(caminhoVeredictos)
		fmt.Printf("Retomando: %d veredicto(s) já registrado(s)\n", len(veredictos))
	}

	cliente, err := deepseek.Novo()
	abortar(err)
	julgados := julgarPendentes(cliente, caminhoVeredictos, pares, veredictos, alvo.NomeChines)
	for chave, veredicto := range julgados {
		veredictos[chave] = veredicto
	}
	reescreverOrdenado(caminhoVeredictos, linhasDeVeredictos(pares, veredictos))

	caminhoDeepSeek := fmt.Sprintf(moldeCacheDeepSeek, idioma)
	reparos := lerCacheSimples(caminhoDeepSeek)
	errados := paresErrados(pares, veredictos)
	fmt.Printf("\nReprovadas pelo DeepSeek: %d · já refeitas: %d\n", len(errados), len(reparos))

	novosReparos, puladas := refazerErradas(cliente, caminhoDeepSeek, errados, reparos, alvo.NomeChines)
	for chines, pt := range novosReparos {
		reparos[chines] = pt
	}
	reescreverOrdenado(caminhoDeepSeek, linhasDeCache(reparos))

	imprimirResumo(cliente, pares, veredictos, reparos, puladas, caminhoVeredictos, caminhoDeepSeek)
}


// ----- Fase 1: veredicto binário -----

// julgarPendentes julga em paralelo os pares ainda sem veredicto, anexando cada lote assim que fecha.
// Devolve chave→veredicto dos novos. Lote que não fecha após as passadas é PULADO (fica para a próxima
// rodada), nunca adivinhado.
func julgarPendentes(cliente *deepseek.Cliente, caminhoSaida string, pares []par, veredictos map[string]string, nomeChines string) map[string]string {
	var pendentes []par
	for _, p := range pares {
		if _, ok := veredictos[chaveDoPar(p)]; !ok {
			pendentes = append(pendentes, p)
		}
	}
	if len(pendentes) == 0 {
		fmt.Println("Nada a julgar: todos os pares já têm veredicto.")
		return map[string]string{}
	}
	fmt.Printf("Julgando %d par(es) com %s (%d por requisição, %d em paralelo)\n",
		len(pendentes), deepseek.MODELO, PARES_POR_REQUISICAO, REQUISICOES_SIMULTANEAS)

	saida := &registrador{caminho: caminhoSaida}
	resultado := map[string]string{}
	var muResultado sync.Mutex

	fatias := fatiarPares(pendentes, PARES_POR_REQUISICAO)
	emParalelo(len(fatias), func(indice int) {
		fatia := fatias[indice]
		parcial := julgarFatia(cliente, fatia, nomeChines, 1)

		var linhas []string
		for _, p := range fatia {
			veredicto, ok := parcial[chaveDoPar(p)]
			if !ok {
				continue // sem veredicto válido: pulado, entra na próxima rodada
			}
			linhas = append(linhas, fmt.Sprintf("%s\t%s\t%s", p.chines, p.traducao, veredicto))
		}
		saida.anexar(linhas)

		muResultado.Lock()
		for chave, veredicto := range parcial {
			resultado[chave] = veredicto
		}
		muResultado.Unlock()
		fmt.Printf("  lote julgado (%d/%d pares fecharam)\n", len(parcial), len(fatia))
	})
	return resultado
}


// julgarFatia envia uma fatia e devolve chave→veredicto. Reenvia os que voltaram sem veredicto válido,
// até MAXIMO_PASSADAS; o que sobrar fica de fora (não é erro fatal).
func julgarFatia(cliente *deepseek.Cliente, pares []par, nomeChines string, passada int) map[string]string {
	var prompt strings.Builder
	for i, p := range pares {
		fmt.Fprintf(&prompt, "%d. %s ⇒ %s\n", i+1, p.chines, p.traducao)
	}

	instrucoes := fmt.Sprintf(INSTRUCOES_VEREDICTO, nomeChines, nomeChines)
	corpo := fmt.Sprintf(MODELO_PROMPT_VEREDICTO, nomeChines, nomeChines, prompt.String())
	resposta, err := cliente.Conversar(instrucoes, corpo, MAXIMO_TOKENS_VEREDICTO)
	if errors.Is(err, deepseek.ErrConteudoBloqueado) {
		return isolarBloqueio(pares, func(sub []par) map[string]string { return julgarFatia(cliente, sub, nomeChines, 1) })
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "  aviso: fatia de %d par(es) falhou (%v) — fica para a próxima rodada\n", len(pares), err)
		return map[string]string{}
	}

	brutos := extrairVeredictos(resposta)
	resultado := map[string]string{}
	var faltantes []par
	for i, p := range pares {
		veredicto, ok := brutos[i+1]
		if !ok {
			faltantes = append(faltantes, p)
			continue
		}
		resultado[chaveDoPar(p)] = veredicto
	}

	if len(faltantes) == 0 || passada >= MAXIMO_PASSADAS {
		return resultado
	}
	for chave, veredicto := range julgarFatia(cliente, faltantes, nomeChines, passada+1) {
		resultado[chave] = veredicto
	}
	return resultado
}


// padraoVeredicto casa "N对"/"N. 错" etc.: o número da entrada seguido de 对/對 (correta) ou 错/錯 (errada).
var padraoVeredicto = regexp.MustCompile(`(\d{1,4})\s*[.\s:)]*\s*(对|對|错|錯)`)

// extrairVeredictos varre a resposta e devolve numero→ok|errado. Ignora qualquer prosa fora do padrão.
func extrairVeredictos(resposta string) map[int]string {
	veredictos := map[int]string{}
	for _, linha := range strings.Split(resposta, "\n") {
		m := padraoVeredicto.FindStringSubmatch(linha)
		if m == nil {
			continue
		}
		numero, err := strconv.Atoi(m[1])
		if err != nil {
			continue
		}
		if m[2] == "对" || m[2] == "對" {
			veredictos[numero] = VEREDICTO_OK
			continue
		}
		veredictos[numero] = VEREDICTO_ERRADO
	}
	return veredictos
}


// ----- Fase 2: reparo das reprovadas (tradução direta) -----

// refazerErradas traduz, direto do chinês, as reprovadas que ainda não têm reparo. Devolve os novos
// reparos e quantas ficaram sem (pulam esta rodada e seguem com a tradução do Google).
func refazerErradas(cliente *deepseek.Cliente, caminhoSaida string, errados []par, reparos map[string]string, nomeChines string) (map[string]string, int) {
	var faltantes []par
	for _, p := range errados {
		if reparos[p.chines] == "" {
			faltantes = append(faltantes, p)
		}
	}
	if len(faltantes) == 0 {
		fmt.Println("Nada a refazer: todas as reprovadas já têm tradução do DeepSeek.")
		return map[string]string{}, 0
	}
	fmt.Printf("Refazendo %d tradução(ões) direto do chinês (%d por requisição)\n", len(faltantes), FRASES_POR_REQUISICAO)

	saida := &registrador{caminho: caminhoSaida}
	resultado := map[string]string{}
	var muResultado sync.Mutex

	fatias := fatiarPares(faltantes, FRASES_POR_REQUISICAO)
	emParalelo(len(fatias), func(indice int) {
		fatia := fatias[indice]
		parcial := traduzirFatia(cliente, fatia, nomeChines, 1)

		var linhas []string
		for _, p := range fatia {
			pt, ok := parcial[p.chines]
			if !ok {
				continue
			}
			linhas = append(linhas, fmt.Sprintf("%s\t%s", p.chines, pt))
		}
		saida.anexar(linhas)

		muResultado.Lock()
		for chines, pt := range parcial {
			resultado[chines] = pt
		}
		muResultado.Unlock()
		fmt.Printf("  lote refeito (%d/%d frases fecharam)\n", len(parcial), len(fatia))
	})
	return resultado, len(faltantes) - len(resultado)
}


// traduzirFatia envia uma fatia e devolve chinês→tradução. Reenvia o que faltou, até MAXIMO_PASSADAS.
func traduzirFatia(cliente *deepseek.Cliente, pares []par, nomeChines string, passada int) map[string]string {
	var prompt strings.Builder
	for i, p := range pares {
		fmt.Fprintf(&prompt, "%d. %s\n", i+1, p.chines)
	}

	instrucoes := fmt.Sprintf(INSTRUCOES_TRADUCAO, nomeChines, nomeChines)
	corpo := fmt.Sprintf(MODELO_PROMPT_TRADUCAO, nomeChines, nomeChines, prompt.String())
	resposta, err := cliente.Conversar(instrucoes, corpo, MAXIMO_TOKENS_TRADUCAO)
	if errors.Is(err, deepseek.ErrConteudoBloqueado) {
		return isolarBloqueio(pares, func(sub []par) map[string]string { return traduzirFatia(cliente, sub, nomeChines, 1) })
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "  aviso: fatia de %d frase(s) falhou (%v) — fica para a próxima rodada\n", len(pares), err)
		return map[string]string{}
	}

	brutas := extrairTraducoes(resposta)
	resultado := map[string]string{}
	var faltantes []par
	for i, p := range pares {
		pt, ok := brutas[i+1]
		if !ok {
			faltantes = append(faltantes, p)
			continue
		}
		resultado[p.chines] = pt
	}

	if len(faltantes) == 0 || passada >= MAXIMO_PASSADAS {
		return resultado
	}
	for chines, pt := range traduzirFatia(cliente, faltantes, nomeChines, passada+1) {
		resultado[chines] = pt
	}
	return resultado
}


// padraoTraducao casa "N. tradução" no início da linha (o número é do prompt, não do texto traduzido).
var padraoTraducao = regexp.MustCompile(`^\s*(\d{1,4})\s*[.:)\]]\s*(.+?)\s*$`)

// padraoLetraLatina casa qualquer letra latina (a-z, incluindo acentuadas). Uma "tradução" sem nenhuma
// serve de sinal de que o modelo não traduziu de verdade — só devolveu o chinês de volta (idêntico ou
// convertido entre tradicional e simplificado), falha observada apesar da instrução explícita no prompt.
var padraoLetraLatina = regexp.MustCompile(`\p{Latin}`)

// extrairTraducoes varre a resposta e devolve numero→tradução. Linha sem o prefixo numerado, vazia, ou
// que não passa de chinês devolvido sem tradução é ignorada — cai nos "faltantes" e é reenviada.
func extrairTraducoes(resposta string) map[int]string {
	traducoes := map[int]string{}
	for _, linha := range strings.Split(resposta, "\n") {
		m := padraoTraducao.FindStringSubmatch(linha)
		if m == nil {
			continue
		}
		numero, err := strconv.Atoi(m[1])
		if err != nil {
			continue
		}
		texto := strings.TrimSpace(m[2])
		if texto == "" || !padraoLetraLatina.MatchString(texto) {
			continue
		}
		traducoes[numero] = texto
	}
	return traducoes
}


// ----- Entrada e saída dos arquivos -----

// lerPares lê o cache do Google e devolve os pares ordenados por chinês (saída estável entre rodadas).
func lerPares(caminho string) []par {
	cache := lerCacheSimples(caminho)
	pares := make([]par, 0, len(cache))
	for chines, traducao := range cache {
		pares = append(pares, par{chines: chines, traducao: traducao})
	}
	sort.Slice(pares, func(i, j int) bool { return pares[i].chines < pares[j].chines })
	return pares
}


// lerCacheSimples lê um TSV de 2 colunas (chinês <TAB> tradução). Arquivo ausente devolve mapa vazio.
func lerCacheSimples(caminho string) map[string]string {
	cache := map[string]string{}
	dados, err := os.ReadFile(caminho)
	if err != nil {
		return cache
	}
	for _, linha := range strings.Split(string(dados), "\n") {
		campos := strings.Split(linha, "\t")
		if len(campos) == 2 && campos[0] != "" && campos[1] != "" {
			cache[campos[0]] = campos[1]
		}
	}
	return cache
}


// lerVeredictosExistentes relê um veredictos anterior para retomar sem repagar (chave = chinês+tradução).
func lerVeredictosExistentes(caminho string) map[string]string {
	veredictos := map[string]string{}
	dados, err := os.ReadFile(caminho)
	if err != nil {
		return veredictos
	}
	for _, linha := range strings.Split(string(dados), "\n") {
		campos := strings.Split(linha, "\t")
		if len(campos) != 3 {
			continue
		}
		if campos[2] != VEREDICTO_OK && campos[2] != VEREDICTO_ERRADO {
			continue
		}
		veredictos[campos[0]+"\t"+campos[1]] = campos[2]
	}
	return veredictos
}


func (r *registrador) anexar(linhas []string) {
	if len(linhas) == 0 {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()

	arquivo, err := os.OpenFile(r.caminho, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	abortar(err)
	defer arquivo.Close()
	_, err = arquivo.WriteString(strings.Join(linhas, "\n") + "\n")
	abortar(err)
}


// reescreverOrdenado troca o arquivo (que cresceu por anexação) por uma versão ordenada e sem repetição.
func reescreverOrdenado(caminho string, linhas []string) {
	sort.Strings(linhas)
	abortar(os.WriteFile(caminho, []byte(strings.Join(linhas, "\n")+"\n"), 0o644))
}


func linhasDeVeredictos(pares []par, veredictos map[string]string) []string {
	var linhas []string
	for _, p := range pares {
		veredicto, ok := veredictos[chaveDoPar(p)]
		if !ok {
			continue
		}
		linhas = append(linhas, fmt.Sprintf("%s\t%s\t%s", p.chines, p.traducao, veredicto))
	}
	return linhas
}


func linhasDeCache(cache map[string]string) []string {
	linhas := make([]string, 0, len(cache))
	for chines, pt := range cache {
		linhas = append(linhas, chines+"\t"+pt)
	}
	return linhas
}


// ----- Utilitários -----

// isolarBloqueio divide o lote ao meio recursivamente até isolar e PULAR (nunca adivinhar) só o(s) par(es)
// que a moderação da API bloqueia (400 "Content Exists Risk") — as demais entradas do lote são
// reenviadas via `chamar` e passam normalmente. Usado tanto pelo veredicto quanto pela tradução: os dois
// operam sobre []par e devolvem map[string]string, só a chave de identificação muda dentro de `chamar`.
func isolarBloqueio(pares []par, chamar func([]par) map[string]string) map[string]string {
	if len(pares) == 1 {
		fmt.Fprintf(os.Stderr, "  aviso: bloqueado pela moderação, pulado: %q\n", pares[0].chines)
		return map[string]string{}
	}
	meio := len(pares) / 2
	resultado := chamar(pares[:meio])
	for chave, valor := range chamar(pares[meio:]) {
		resultado[chave] = valor
	}
	return resultado
}


// emParalelo roda indices 0..total-1 com no máximo REQUISICOES_SIMULTANEAS em voo.
func emParalelo(total int, tarefa func(indice int)) {
	var wg sync.WaitGroup
	vagas := make(chan struct{}, REQUISICOES_SIMULTANEAS)
	for i := 0; i < total; i++ {
		wg.Add(1)
		vagas <- struct{}{}
		go func(indice int) {
			defer wg.Done()
			defer func() { <-vagas }()
			tarefa(indice)
		}(i)
	}
	wg.Wait()
}


func fatiarPares(pares []par, tamanho int) [][]par {
	var fatias [][]par
	for inicio := 0; inicio < len(pares); inicio += tamanho {
		fim := inicio + tamanho
		if fim > len(pares) {
			fim = len(pares)
		}
		fatias = append(fatias, pares[inicio:fim])
	}
	return fatias
}


func paresErrados(pares []par, veredictos map[string]string) []par {
	var errados []par
	for _, p := range pares {
		if veredictos[chaveDoPar(p)] == VEREDICTO_ERRADO {
			errados = append(errados, p)
		}
	}
	return errados
}


func chaveDoPar(p par) string { return p.chines + "\t" + p.traducao }


func lerArgumentos(args []string) (idioma string, refazer bool) {
	idioma = idiomaPadrao
	for _, arg := range args {
		if arg == "-refazer" || arg == "--refazer" {
			refazer = true
			continue
		}
		idioma = arg
	}
	return idioma, refazer
}


func imprimirResumo(cliente *deepseek.Cliente, pares []par, veredictos map[string]string, reparos map[string]string, puladas int, caminhoVeredictos, caminhoDeepSeek string) {
	ok, errado := 0, 0
	for _, p := range pares {
		switch veredictos[chaveDoPar(p)] {
		case VEREDICTO_OK:
			ok++
		case VEREDICTO_ERRADO:
			errado++
		}
	}
	entrada, saida := cliente.Tokens()

	fmt.Printf("\n===== RESUMO =====\n")
	fmt.Printf("Veredictos: %s\n", caminhoVeredictos)
	fmt.Printf("Reparos:    %s\n", caminhoDeepSeek)
	fmt.Printf("Julgadas: %d · corretas (对): %d · erradas (错): %d · sem veredicto ainda: %d\n",
		ok+errado, ok, errado, len(pares)-ok-errado)
	fmt.Printf("Refeitas pelo DeepSeek: %d · ainda sem reparo: %d\n", len(reparos), puladas)
	fmt.Printf("Tokens: %d de entrada + %d de saída ≈ US$ %.4f\n", entrada, saida, cliente.CustoEstimadoUsd())
	fmt.Printf("\nAgora remonte o arquivo embarcado: go run ./dicionario/frases/complementar\n")
}


func abortar(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, "erro:", err)
		os.Exit(1)
	}
}
