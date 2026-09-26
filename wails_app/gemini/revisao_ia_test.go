package gemini

import (
	"encoding/json"
	"testing"
)

func TestExtrairJSON(t *testing.T) {
	casos := []struct {
		nome     string
		entrada  string
		esperado string
	}{
		{
			nome:     "cerca markdown json",
			entrada:  "```json\n[{\"tipo\":\"lacuna\"}]\n```",
			esperado: "[{\"tipo\":\"lacuna\"}]",
		},
		{
			nome:     "resposta ja limpa",
			entrada:  "[{\"tipo\":\"lacuna\"}]",
			esperado: "[{\"tipo\":\"lacuna\"}]",
		},
		{
			nome:     "prosa antes e depois do array",
			entrada:  "Aqui está o seu JSON:\n[{\"tipo\":\"lacuna\"}]\nEspero que goste!",
			esperado: "[{\"tipo\":\"lacuna\"}]",
		},
	}

	for _, tc := range casos {
		t.Run(tc.nome, func(t *testing.T) {
			resultado := extrairJSON(tc.entrada)
			if resultado != tc.esperado {
				t.Errorf("extrairJSON(%q) = %q; esperava %q", tc.entrada, resultado, tc.esperado)
			}
		})
	}
}

func TestDecodificarFrasesRevisaoComClassificacao(t *testing.T) {
	jsonResposta := `[
		{"alvo": "好", "frase": "今天天气很好。", "traducao": "O tempo está muito bom hoje.", "tema": "天气自然", "dificuldade": "入门"},
		{"alvo": "朋", "frase": "他是我最好的朋友。", "traducao": "Ele é meu melhor amigo.", "tema": "社交问候", "dificuldade": "初级"}
	]`

	limpo := extrairJSON(jsonResposta)
	var frases []FraseRevisaoIa
	if err := json.Unmarshal([]byte(limpo), &frases); err != nil {
		t.Fatalf("erro ao decodificar JSON: %v", err)
	}

	if len(frases) != 2 {
		t.Fatalf("esperava 2 frases, vieram %d", len(frases))
	}

	if frases[0].Tema != "天气自然" || frases[0].Dificuldade != "入门" {
		t.Errorf("esperava tema 天气自然 e dificuldade 入门 na primeira frase, veio %q e %q", frases[0].Tema, frases[0].Dificuldade)
	}
}
