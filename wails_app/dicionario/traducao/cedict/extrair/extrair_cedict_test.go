package main

import "testing"

// ----- Extração dos lotes do CEDICT (funções puras, sem arquivo) -----

func TestRemoverPinyinAposHanziSoCortaOhQueSegueHanzi(t *testing.T) {
	casos := []struct {
		texto    string
		esperado string
	}{
		{"variant of 臺[tai2]", "variant of 臺"},                     // referência clássica: hanzi+pinyin
		{"CL:個|个[ge4],位[wei4]", "CL:個|个,位"},                        // referências em sequência, tradicional|simplificado preservado
		{"Taiwan pr. [xia4]", "Taiwan pr. [xia4]"},                 // colchete sem hanzi antes é conteúdo
		{"also pr. [lei2] in Taiwan", "also pr. [lei2] in Taiwan"}, // idem no meio da glosa
		{"臺[tai2][xxx] em sequência", "臺 em sequência"},            // colchetes encadeados após o mesmo hanzi
		{"sem colchete nenhum", "sem colchete nenhum"},             // glosa comum passa intacta
	}
	for _, caso := range casos {
		if limpo := removerPinyinAposHanzi(caso.texto); limpo != caso.esperado {
			t.Fatalf("removerPinyinAposHanzi(%q) devia dar %q, veio %q", caso.texto, caso.esperado, limpo)
		}
	}
}


func TestFormatarLinhaSeparaLeiturasComBarraDupla(t *testing.T) {
	umaLeitura := grupoCedict{
		simplificado:         "世界",
		significadosPorLinha: [][]string{{"world", "CL:個|个"}},
	}
	if linha := formatarLinha(umaLeitura); linha != "世界 | TRADUÇÃO: world/CL:個|个\n" {
		t.Fatalf("linha de leitura única errada: %q", linha)
	}

	duasLeituras := grupoCedict{
		simplificado:         "好",
		significadosPorLinha: [][]string{{"good", "well"}, {"to be fond of", "to like"}},
	}
	if linha := formatarLinha(duasLeituras); linha != "好 | TRADUÇÃO: good/well ‖ to be fond of/to like\n" {
		t.Fatalf("linha multi-leitura errada: %q", linha)
	}
}
