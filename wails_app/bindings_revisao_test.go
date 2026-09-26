package main

import (
	"testing"

	"wails_app/config"
	"wails_app/revisao"
	"wails_app/revisao/jornada"
)

// appDeTesteJornada monta um App só com o necessário para os bindings da Jornada: RevisoesEfetivas
// lê apenas a configuração, então dicionário e frases podem ficar nulos.
func appDeTesteJornada(t *testing.T) *App {
	t.Helper()

	app := &App{Config: config.DefaultConfig()}
	app.revisao = revisao.NovoGerenciadorRevisao(nil, nil, func() config.Config { return app.Config })
	return app
}

func contaRevisoes(arvore jornada.Arvore, tipo string) int {
	total := 0
	for _, ramo := range arvore.Ramos {
		for _, nivel := range ramo.Niveis {
			for _, revisao := range nivel.Revisoes {
				if revisao == tipo {
					total++
				}
			}
		}
	}
	return total
}

func TestObterArvoreJornadaSubstituiPorMotorSemMutarOhCache(t *testing.T) {
	app := appDeTesteJornada(t)
	app.Config.MotorTtsAtivo = "nenhum"
	app.Config.MotorSttAtivo = "nenhum"

	semMotores, err := app.ObterArvoreJornada()
	if err != nil {
		t.Fatalf("ObterArvoreJornada: %v", err)
	}
	if n := contaRevisoes(semMotores, jornada.RevisaoFonetica); n != 0 {
		t.Errorf("sem TTS, a árvore não deveria ter revisão de fonética; veio %d", n)
	}
	if n := contaRevisoes(semMotores, jornada.RevisaoPronuncia); n != 0 {
		t.Errorf("sem STT, a árvore não deveria ter revisão de pronúncia; veio %d", n)
	}

	// Com os motores ligados os tipos precisam VOLTAR: se a chamada anterior tivesse escrito na
	// árvore em cache do pacote jornada, a substituição seria permanente.
	app.Config.MotorTtsAtivo = "Kokoro-82M"
	app.Config.MotorSttAtivo = "Paraformer-ZH"

	comMotores, err := app.ObterArvoreJornada()
	if err != nil {
		t.Fatalf("ObterArvoreJornada com motores: %v", err)
	}
	if contaRevisoes(comMotores, jornada.RevisaoFonetica) == 0 {
		t.Error("com TTS ativo, a fonética deveria reaparecer na árvore (o cache foi mutado?)")
	}
	if contaRevisoes(comMotores, jornada.RevisaoPronuncia) == 0 {
		t.Error("com STT ativo, a pronúncia deveria reaparecer na árvore (o cache foi mutado?)")
	}

	// O comprimento da lista de revisões nunca muda: os índices salvos em jornada_progresso
	// continuam apontando para a mesma posição.
	for iRamo, ramo := range semMotores.Ramos {
		for iNivel, nivel := range ramo.Niveis {
			outro := comMotores.Ramos[iRamo].Niveis[iNivel]
			if len(nivel.Revisoes) != len(outro.Revisoes) {
				t.Errorf("nível %q mudou de tamanho entre as configurações: %d vs %d",
					nivel.Id, len(nivel.Revisoes), len(outro.Revisoes))
			}
			if nivel.Id == "" {
				t.Errorf("nível sem id no ramo %q — o frontend não teria como registrar o progresso", ramo.Id)
			}
		}
	}
}
