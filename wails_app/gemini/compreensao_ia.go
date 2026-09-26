package gemini

import (
	"encoding/json"
	"fmt"
	"strings"
)

// ----- Seção: Estruturas de Dados -----

// PerguntaCompreensaoIa representa uma pergunta de múltipla escolha de compreensão gerada via Gemini.
type PerguntaCompreensaoIa struct {
	Alvo                  string   `json:"alvo"`
	Contexto              string   `json:"contexto"`
	Pergunta              string   `json:"pergunta"`
	Opcoes                []string `json:"opcoes"`
	IndiceRespostaCorreta int      `json:"indice_resposta_correta"`
	PerguntaTraduzida     string   `json:"pergunta_traduzida"`
	ContextoTraduzido     string   `json:"contexto_traduzido"`
	Tema                  string   `json:"tema,omitempty"`
	Dificuldade           string   `json:"dificuldade,omitempty"`
}

const promptCompreensaoIa = `Você é um elaborador de questões de compreensão de texto em chinês mandarim para exames HSK e aprendizes de nível intermediário. Gere exatamente %d exercícios de múltipla escolha em JSON para praticar os caracteres-alvo listados.

Caracteres-alvo (hanzi | pinyin | significado):
%s

Parâmetros da sessão:
- Tema dos assuntos: %s
- Nível de dificuldade: %s
- Clima/vibe dos textos: %s
- Tipo de escrita: %s (use APENAS este tipo em todos os campos com hanzi)

Taxonomia de Temas permitidos: [日常生活, 家庭, 工作职业, 学习教育, 饮食, 购物, 旅行交通, 健康身体, 情感, 时间日期, 天气自然, 社交问候, 兴趣娱乐, 科技, 数字量词, 地点方位, 语言文化, 其他]
Escala de Dificuldades permitidas: [入门, 初级, 中级, 高级]

Regras:
1. "contexto" é uma frase ou diálogo curto em chinês (6 a 18 caracteres) contendo EXATAMENTE UM dos caracteres-alvo.
2. "pergunta" é uma pergunta direta em chinês sobre a informação ou sentido do contexto.
3. "opcoes" é uma lista com EXATAMENTE 4 opções em chinês (1 correta e 3 distratores plausíveis).
4. "indice_resposta_correta" é o número inteiro (0 a 3) referente à posição da resposta correta na lista de opcoes.
5. "pergunta_traduzida" é a tradução em português do Brasil da pergunta.
6. "contexto_traduzido" é a tradução em português do Brasil do contexto.
7. Preencha "tema" e "dificuldade" obrigatoriamente.

Responda APENAS com um array JSON válido, sem markdown e sem comentários, no formato:
[
  {
    "alvo": "好",
    "contexto": "今天天气很好，我们去公园吧。",
    "pergunta": "说话人想做什么？",
    "opcoes": ["去休息", "去公园", "去工作", "去买东西"],
    "indice_resposta_correta": 1,
    "pergunta_traduzida": "O que a pessoa quer fazer?",
    "contexto_traduzido": "O tempo está muito bom hoje, vamos ao parque.",
    "tema": "天气自然",
    "dificuldade": "初级"
  }
]`

// ----- Seção: Função Principal -----

// GerarPerguntasCompreensao monta o prompt e solicita ao Gemini perguntas de múltipla escolha com contexto.
func GerarPerguntasCompreensao(apiKey, modelo string, alvos []AlvoRevisaoIa, quantidade int, tema, dificuldade, vibe, tipoHanzi string) ([]PerguntaCompreensaoIa, error) {
	if len(alvos) == 0 {
		return nil, fmt.Errorf("lista de alvos não pode ser vazia")
	}

	instrucaoTema := tema
	if instrucaoTema == "" {
		instrucaoTema = "Livre (classifique cada item individualmente conforme a taxonomia)"
	} else {
		instrucaoTema = fmt.Sprintf("%s (exija que toda pergunta pertença a este tema)", tema)
	}

	instrucaoDificuldade := dificuldade
	if instrucaoDificuldade == "" {
		instrucaoDificuldade = "Variado (escala 入门/初级/中级/高级)"
	} else {
		instrucaoDificuldade = fmt.Sprintf("%s (exija que toda pergunta pertença a este nível)", dificuldade)
	}

	if vibe == "" {
		vibe = "neutro"
	}

	var sb strings.Builder
	for _, a := range alvos {
		sb.WriteString(fmt.Sprintf("%s | %s | %s\n", a.Hanzi, a.Pinyin, a.Definicao))
	}

	prompt := fmt.Sprintf(promptCompreensaoIa, quantidade, sb.String(), instrucaoTema, instrucaoDificuldade, vibe, tipoHanzi)

	resposta, err := chamarGemini(apiKey, modelo, prompt, nil)
	if err != nil {
		return nil, fmt.Errorf("falha ao chamar o Gemini: %w", err)
	}

	var perguntas []PerguntaCompreensaoIa
	if err := json.Unmarshal([]byte(extrairJSON(resposta)), &perguntas); err != nil {
		return nil, fmt.Errorf("falha ao decodificar JSON de compreensão: %w (resposta original: %q)", err, resposta)
	}

	if len(perguntas) == 0 {
		return nil, fmt.Errorf("a IA retornou uma lista vazia de perguntas de compreensão")
	}

	return perguntas, nil
}
