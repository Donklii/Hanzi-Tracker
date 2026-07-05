# Licenças dos dados embarcados

Os arquivos de dados deste diretório são embarcados no binário (`go:embed`) e têm as
seguintes origens e licenças. Ao distribuir o app, estas atribuições devem ser preservadas.

| Arquivo | Origem | Licença |
|---|---|---|
| `cedict_ts.u8` | [CC-CEDICT](https://www.mdbg.net/chinese/dictionary?page=cedict) | CC BY-SA 4.0 |
| `makemeahanzi_dictionary.txt` | [Make Me a Hanzi](https://github.com/skishore/makemeahanzi) | LGPL-3.0 (dados derivados do Arphic PL) |
| `hanzi_tracados.tsv.gz` | [hanzi-writer-data](https://github.com/chanind/hanzi-writer-data) v2.0.1 (derivado do Make Me a Hanzi) | Arphic Public License |
| `frases_tatoeba.tsv.gz` | [Tatoeba](https://tatoeba.org) via [manythings.org/anki](https://www.manythings.org/anki/) (cmn-eng) | CC BY 2.0 FR — a atribuição por frase (coluna 3 do TSV) é exibida na interface junto de cada frase |

## Formato dos arquivos gerados

- `hanzi_tracados.tsv.gz`: uma linha por caractere — `caractere<TAB>json` no formato do
  [Hanzi Writer](https://hanziwriter.org) (`strokes`/`medians`). 9.574 caracteres.
- `frases_tatoeba.tsv.gz`: uma linha por par — `chinês<TAB>inglês<TAB>atribuição`. 32.028 pares.

Ambos foram gerados a partir das fontes acima em 2026-07-03 (download em 2026-07-03).
