# Licenças dos dados

Ao distribuir o app, as atribuições abaixo devem ser preservadas.

## Onde as atribuições aparecem para o usuário

Não basta este arquivo: as licenças exigem que a atribuição viaje **dentro do app**. Ela vive em
`frontend/src/configuracoes/licencas/` (aba Info → "Licenças de terceiros"), que mostra autor,
copyright, licença, modificações e o **texto integral** de cada licença, embarcado — o Make Me a
Hanzi manda entregar o arquivo `LGPL` junto dos dados e a Arphic PL §1 exige reter o `ARPHICPL.TXT`
inalterado em toda cópia, então nenhum dos dois pode virar link para a internet.

Mexer nas fontes de dados obriga a atualizar `licencas/atribuicoes.ts` junto: a Arphic PL §2(a) e a
cláusula (c) do aviso do Unicode exigem declarar **como e quando** cada dado foi modificado, e a
CC BY-SA 4.0 §3(a)(1)(B) exige indicar as modificações.

## Insumo (`fontes/`) x produto (`idiomas/`)

- **`fontes/`** guarda os dados CRUS, como vieram da origem. Só alimentam a fusão em tempo de build e
  **não** entram no binário.
- **`idiomas/`** guarda o que o app realmente lê, já pronto, e é o que o `go:embed` embarca (o embed é
  recursivo: qualquer arquivo solto sob `idiomas/` vai parar no executável).
- `fontes/hanzi_tracados.tsv.gz` é a ÚNICA exceção à regra "fontes/ não embarca": chega da origem já no
  formato exato que o app consome (TSV gzipado, sem fusão nenhuma), então é embarcado direto de lá
  (`tracados.go`) — e é neutro de idioma, sem equivalente por idioma em `idiomas/`.

O inglês (`idiomas/en/`) é o **fallback** em runtime: quando o idioma escolhido não tem um recurso
próprio, o app usa o do inglês (ver `idiomas.go`).

## Fontes (não embarcadas)

| Arquivo | Origem | Licença |
|---|---|---|
| `fontes/en/cedict.u8` | [CC-CEDICT](https://www.mdbg.net/chinese/dictionary?page=cedict) | CC BY-SA 4.0 |
| `fontes/en/makemeahanzi.txt` | [Make Me a Hanzi](https://github.com/skishore/makemeahanzi) — é o `dictionary.txt` dele, derivado do [Unihan](http://unicode.org/charts/unihan.html) (Copyright © 1991–2009 Unicode, Inc.) e do [CJKlib](https://github.com/cburgmer/cjklib). **Não** confundir com o `graphics.txt`, que é a origem Arphic dos traçados | LGPL-3.0 ou posterior, mais o aviso de copyright e permissão do Unicode (arquivo `LGPL` do Make Me a Hanzi) |
| `fontes/pt-BR/cedict.u8` | Tradução PT-BR dos significados do CC-CEDICT (tradicional, simplificado e pinyin inalterados; só as glosas são traduzidas) — obra derivada, mesma licença do original | CC BY-SA 4.0 |
| `fontes/pt-BR/makemeahanzi.txt` | Tradução PT-BR das definições/dicas do Make Me a Hanzi (só o texto de definição/dica é traduzido) | LGPL-3.0 |
| `fontes/frequencia_opensubtitles.txt.gz` | [FrequencyWords](https://github.com/hermitdave/FrequencyWords) de Hermit Dave (`content/2018/zh_cn/zh_cn_full.txt`), derivado do corpus [OpenSubtitles 2018](https://opus.nlpl.eu/OpenSubtitles.php) | CC BY-SA 4.0 |

## Embarcados (`go:embed`)

| Arquivo | Origem | Licença |
|---|---|---|
| `idiomas/<idioma>/dicionario.jsonl.gz` | **Obra derivada das TRÊS fontes acima somadas** (CC-CEDICT + Make Me a Hanzi + FrequencyWords), fundidas por `fusao/fundir_dicionarios.go` | CC BY-SA 4.0 **e** LGPL-3.0 — as duas ao mesmo tempo, por combinar dados das duas famílias. Distribuir o app exige atribuir as três origens |
| `fontes/hanzi_tracados.tsv.gz` (neutro de idioma) | [hanzi-writer-data](https://github.com/chanind/hanzi-writer-data) v2.0.1 (derivado do Make Me a Hanzi) | Arphic Public License |
| `idiomas/en/frases/frases_tatoeba.tsv.gz` | [Tatoeba](https://tatoeba.org) via [manythings.org/anki](https://www.manythings.org/anki/) (cmn-eng) | CC BY 2.0 FR — a atribuição por frase (coluna 3 do TSV) é exibida na interface junto de cada frase |
| `idiomas/en/frases/frases_tatoeba_extra.tsv.gz` | [Tatoeba](https://tatoeba.org) exports (cmn-eng) | CC BY 2.0 FR — a atribuição por frase (coluna 3 do TSV) é exibida na interface junto de cada frase |
| `idiomas/pt-BR/frases/frases_tatoeba.tsv.gz` | [Tatoeba](https://tatoeba.org) exports (cmn-por), casados por `frases/gerar_tatoeba`, verificados por `frases/verificar` e com as traduções incorretas refeitas pelo Google Tradutor em `frases/montar` | Frase chinesa: CC BY 2.0 FR (atribuição na coluna 3, exibida na interface). Traduções pt-BR refeitas: geradas por tradução automática do Google (o Google não reivindica direitos sobre a saída); marcadas como tais na própria atribuição |

## Formato dos arquivos gerados

- `idiomas/<idioma>/dicionario.jsonl.gz`: uma entrada JSON por linha, chaveada pelo chinês
  **simplificado**, com as leituras do CEDICT aninhadas e o campo `frequencia`
  ({`percentual`, `posicao`}). Cada leitura pode trazer um `tipo` (`variante`/`sobrenome`) que a rotula
  de forma independente do idioma (detectado no inglês original; ausente = conteúdo). 121.418 entradas.
  É a fonte ÚNICA de consulta do app — ver `banco.go`
  para as regras de reconstrução (a fusão compacta a saída apagando da leitura o que o topo já
  responde, e ler o arquivo exige repor isso). Gerado por `go run ./dicionario/fusao <idioma>` a
  partir de `fontes/<idioma>/`; a geração é **determinística byte a byte** (inclusive o cabeçalho
  gzip, sem nome nem timestamp), porque o arquivo é versionado.
  **Modificações sobre o CC-CEDICT** (a CC BY-SA 4.0 §3(a)(1)(B) exige indicá-las): além da tradução
  dos significados, a fusão (1) REMOVE das glosas o pinyin entre colchetes colado a hanzi citado
  (ex.: `CL:個|个[ge4]` → `CL:個|个`) — ruído de referência, não conteúdo; e (2) escolhe como definição
  principal a primeira leitura de conteúdo, deixando como secundária uma leitura que só aponta outra
  grafia ("variant of ...") ou que só registra um sobrenome ("surname ..."). O `fontes/en/cedict.u8`
  original permanece intacto — as modificações são só da derivação.
- `fontes/hanzi_tracados.tsv.gz`: uma linha por caractere — `caractere<TAB>json` no formato do
  [Hanzi Writer](https://hanziwriter.org) (`strokes`/`medians`). 9.574 caracteres.
- `fontes/<idioma>/makemeahanzi.txt`: uma entrada JSON por linha. A versão pt-BR é gerada a partir da
  inglesa por `traducao/makemeahanzi/mesclar/`, preservando todos os campos e trocando só
  `definition` e `etymology.hint` pela tradução. 9.574 entradas.
- `fontes/pt-BR/cedict.u8`: mesmo formato do CC-CEDICT original, gerado por
  `traducao/cedict/mesclar/`. 125.051 linhas.
- `idiomas/en/frases/frases_tatoeba.tsv.gz`: uma linha por par — `chinês<TAB>inglês<TAB>atribuição`. 32.028 pares.
- `idiomas/en/frases/frases_tatoeba_extra.tsv.gz`: uma linha por par — `chinês<TAB>inglês<TAB>atribuição`. 36.878 pares.
- `idiomas/pt-BR/frases/frases_tatoeba.tsv.gz`: uma linha por par — `chinês<TAB>português<TAB>atribuição`. 1.398 frases
  (uma por texto chinês). Produzido por um pipeline de três passos (rodar da raiz do módulo `wails_app`):
  1. Casamento cru: cada frase chinesa foi casada com suas traduções em português a partir dos despejos
     públicos do Tatoeba (`per_language/cmn` e `per_language/por` no formato `_sentences_detailed`, mais
     `exports/links.tar.bz2`) → `frases/pt-BR/frases_brutas.tsv.gz`. Esses despejos brutos NÃO são
     versionados (baixados só para gerar). O script que fazia esse casamento (`frases/gerar_tatoeba`) foi
     removido depois de rodar — os despejos do Tatoeba não serão mais atualizados, e onboarding de idiomas
     novos hoje passa por `frases/complementar` (traduz o corpus en existente) ou `frases/gerar` (cria
     conteúdo novo via DeepSeek), ambos sem depender de baixar despejos brutos por par de idioma.
  2. `go run ./dicionario/frases/verificar` colapsa a uma frase por chinês e pede ao DeepSeek um veredicto
     binário (correta/errada) de cada tradução → `frases/pt-BR/veredictos.tsv`. As frases do Tatoeba são
     colaborativas e não revisadas: ~8% traziam erro de sentido.
  3. `go run ./dicionario/frases/montar` mantém as corretas (tradução humana + atribuição do Tatoeba) e
     REFAZ as erradas com o Google Tradutor direto do chinês (cache em `frases/pt-BR/traducoes_google.tsv`),
     escrevendo o arquivo final. A frase chinesa continua sendo a do Tatoeba (CC BY 2.0 FR); a atribuição
     das refeitas mantém o crédito do chinês e marca "tradução pt-BR refeita via Google Tradutor".
  A montagem final é **determinística byte a byte** (cabeçalho gzip sem nome nem timestamp; frases
  ordenadas por id), porque o arquivo é versionado. Os intermediários (`frases_brutas.tsv.gz`,
  `veredictos.tsv`, `traducoes_google.tsv`) são versionados e ficam fora do `go:embed`.
- `fontes/frequencia_opensubtitles.txt.gz`: uma linha por palavra — `palavra<ESPAÇO>contagem`, em ordem
  decrescente. 766.612 palavras somando 85.487.865 tokens. É o `zh_cn_full.txt` do FrequencyWords
  **byte a byte** (só gzipado, para poder ser conferido contra a origem); nenhuma edição nossa.

Gerados a partir das fontes acima em 2026-07-03, 2026-07-11, 2026-07-14 (tradução pt-BR do
makemeahanzi), 2026-07-16 (tradução pt-BR do CEDICT), 2026-07-17 (frequência e fusão) e 2026-07-20
(frases pt-BR do Tatoeba, despejos de 2026-07-18; verificadas pelo DeepSeek e as incorretas refeitas
pelo Google Tradutor).
