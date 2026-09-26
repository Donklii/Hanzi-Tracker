package gemini

import (
	"encoding/json"
	"fmt"
	"strings"
)

// ----- Revisão com IA: geração de um banco de frases -----
//
// A IA gera apenas FRASES naturais (chinês + tradução) para cada caractere-alvo. O consumidor
// (wails_app/frases_ia.go) carimba o tema e a dificuldade escolhidos no backend, salva as frases
// no SQLite e as injeta no acervo de revisão geral.

// AlvoRevisaoIa é um caractere-alvo enviado à IA (não serializado; montado pelo backend).
type AlvoRevisaoIa struct {
	Hanzi     string
	Pinyin    string
	Definicao string
}

// FraseRevisaoIa é uma frase gerada pela IA, amarrada ao caractere-alvo que ela pratica.
type FraseRevisaoIa struct {
	Alvo        string `json:"alvo"`
	Frase       string `json:"frase"`
	Traducao    string `json:"traducao"`
	Tema        string `json:"tema,omitempty"`
	Dificuldade string `json:"dificuldade,omitempty"`
}

const promptFrasesRevisaoIa = `Você é um gerador e classificador de frases para um app de estudo de chinês mandarim. Gere exatamente %d frases em JSON para praticar os caracteres-alvo listados abaixo.

Caracteres-alvo (hanzi | pinyin | significado):
%s

Parâmetros da sessão:
- Tema dos assuntos: %s
- Nível de dificuldade: %s
- Clima/vibe das frases: %s
- Tipo de escrita: %s (use APENAS este tipo em todos os campos com hanzi)

Taxonomia de Temas permitidos para classificação: [日常生活, 家庭, 工作职业, 学习教育, 饮食, 购物, 旅行交通, 健康身体, 情感, 时间日期, 天气自然, 社交问候, 兴趣娱乐, 科技, 数字量词, 地点方位, 语言文化, 其他]
Escala de Dificuldades permitidas: [入门, 初级, 中级, 高级] (入门: 4-8 caracteres, HSK1; 初级: 5-10 caracteres, HSK2; 中级: 8-14 caracteres, HSK3-4; 高级: 12-20 caracteres)

Regras:
- Cada frase pratica UM dos caracteres-alvo e deve conter esse caractere EXATAMENTE UMA VEZ.
- Distribua as frases entre os alvos (repita alvos se houver mais frases que alvos), variando as frases.
- Frases inéditas, naturais e adequadas ao tema, à vibe e à dificuldade pedidos. Não repita a mesma frase.
- "traducao" é a tradução fiel da frase, em português do Brasil.
- PREENCHA OBRIGATORIAMENTE os campos "tema" e "dificuldade" em cada objeto JSON escolhendo um valor exato das listas permitidas acima.

Responda APENAS com um array JSON válido, sem markdown e sem comentários, no formato:
[
  {"alvo": "好", "frase": "今天天气很好。", "traducao": "O tempo está muito bom hoje.", "tema": "天气自然", "dificuldade": "入门"},
  {"alvo": "朋", "frase": "他是我最好的朋友。", "traducao": "Ele é meu melhor amigo.", "tema": "社交问候", "dificuldade": "初级"}
]`

// extrairJSON descasca cercas de markdown e devolve apenas o array JSON.
func extrairJSON(resposta string) string {
	idxInicio := strings.Index(resposta, "[")
	idxFim := strings.LastIndex(resposta, "]")
	if idxInicio != -1 && idxFim != -1 && idxInicio < idxFim {
		return resposta[idxInicio : idxFim+1]
	}
	return strings.TrimSpace(resposta)
}

// GerarFrasesRevisao monta o prompt e chama o Gemini para criar um banco de frases de revisão.
func GerarFrasesRevisao(apiKey, modelo string, alvos []AlvoRevisaoIa, quantidade int, tema, dificuldade, vibe, tipoHanzi string) ([]FraseRevisaoIa, error) {
	if len(alvos) == 0 {
		return nil, fmt.Errorf("lista de alvos não pode ser vazia")
	}

	instrucaoTema := tema
	if instrucaoTema == "" {
		instrucaoTema = "Livre (classifique cada frase individualmente de acordo com a taxonomia de temas permitidos)"
	} else {
		instrucaoTema = fmt.Sprintf("%s (exija que TODA frase pertença a este tema)", tema)
	}

	instrucaoDificuldade := dificuldade
	if instrucaoDificuldade == "" {
		instrucaoDificuldade = "Variado (classifique cada frase individualmente na escala 入门/初级/中级/高级 de acordo com o tamanho e complexidade)"
	} else {
		instrucaoDificuldade = fmt.Sprintf("%s (exija que TODA frase pertença a este nível de dificuldade)", dificuldade)
	}

	if vibe == "" {
		vibe = "neutro"
	}

	var sb strings.Builder
	for _, a := range alvos {
		sb.WriteString(fmt.Sprintf("%s | %s | %s\n", a.Hanzi, a.Pinyin, a.Definicao))
	}

	prompt := fmt.Sprintf(promptFrasesRevisaoIa, quantidade, sb.String(), instrucaoTema, instrucaoDificuldade, vibe, tipoHanzi)

	resposta, err := chamarGemini(apiKey, modelo, prompt, nil)
	if err != nil {
		return nil, fmt.Errorf("falha ao chamar o Gemini: %w", err)
	}

	var frases []FraseRevisaoIa
	if err := json.Unmarshal([]byte(extrairJSON(resposta)), &frases); err != nil {
		return nil, fmt.Errorf("falha ao decodificar JSON das frases geradas pela IA: %w (resposta original: %q)", err, resposta)
	}

	if len(frases) == 0 {
		return nil, fmt.Errorf("a IA retornou uma lista vazia de frases")
	}

	return frases, nil
}
