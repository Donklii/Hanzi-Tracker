# TODO — Otimizações de CPU/RAM/IPC (rodada 1)

**Status geral:** EM EXECUÇÃO <!-- executor: troque para EM EXECUÇÃO ao começar, CONCLUÍDO ao terminar tudo, BLOQUEADO se parar por bloqueio -->

> **Instruções para o executor — leia antes de começar:**
>
> 1. Execute as etapas na ordem, uma de cada vez.
> 2. Ao concluir uma etapa, rode a **Verificação** dela; só então marque `[x]` em "Concluída" e passe à próxima.
> 3. Quando a etapa indicar um **Modelo**, abra o arquivo/função indicado e siga o mesmo padrão (estrutura, nomenclatura, estilo).
> 4. Siga as instruções exatamente. Não adicione nada além do pedido, não refatore código fora do escopo, não "melhore" nada por conta própria.
> 5. Se algo impedir a etapa (arquivo não existe, erro persistente, instrução ambígua), **não improvise**: marque a etapa com `[!]`, registre o problema em "Dúvidas e bloqueios" e pare.
> 6. Se precisar desviar minimamente do plano (ex.: um nome já estava em uso), anote o desvio em "Dúvidas e bloqueios" e continue.
> 7. Não apague nem reescreva este arquivo: apenas marque checkboxes, atualize o status geral e escreva na seção "Dúvidas e bloqueios".

## Objetivo

Cortar consumo desnecessário de CPU, RAM e I/O sem mudar nenhum comportamento visível: parar de re-renderizar o frontend a 60fps à toa, parar de emitir eventos de mouse redundantes, eliminar buscas O(n²) nas listas, reutilizar conexões HTTP e parar de repetir INSERTs no SQLite a cada scan. Pronto quando: todas as etapas concluídas, `go test ./...` passa em `wails_app` e `npm run build` passa em `wails_app/frontend`.

## Contexto do projeto

- App desktop Wails v2: código Go em `wails_app/` (módulo Go próprio — rode os comandos Go de dentro de `wails_app/`), frontend React+TypeScript+Vite em `wails_app/frontend/`.
- Convenções: nomes em PT-BR, guard clauses no início das funções, comentários curtos explicando o *porquê* (não o quê).
- Comandos de verificação:
  - Go: `cd wails_app` e rodar `go build -tags desktop,production ./...` e depois `go test ./...` — ambos devem passar sem erro novo.
  - Frontend: `cd wails_app/frontend` e rodar `npm run build` — deve terminar sem erro.

## Etapas

### Etapa 1 — Remover o state morto `posicaoMouse` (re-render a 60fps)

- [x] Concluída
- **Arquivo:** `wails_app/frontend/src/App.tsx` — modificar

**O que fazer:** O state `posicaoMouse` é atualizado a cada evento de mouse mas nunca é lido no render — cada atualização re-renderiza o componente `App` inteiro dezenas de vezes por segundo. Remover duas linhas:
1. A declaração `const [posicaoMouse, setPosicaoMouse] = useState({ x: 0, y: 0 });`
2. A chamada `setPosicaoMouse({ x: data.x, y: data.y });` dentro do handler `EventsOn("mouse_pos", ...)`.

Não mexer no resto do handler (o cálculo de `localX`/`localY` e o foco de card continuam iguais).

**Verificação:** `npm run build` em `wails_app/frontend` passa; buscar `posicaoMouse` no arquivo não retorna nenhuma ocorrência.

### Etapa 2 — Remover a função morta `RenderizarListaCartoes`

- [x] Concluída
- **Arquivo:** `wails_app/frontend/src/App.tsx` — modificar

**O que fazer:** A função `const RenderizarListaCartoes = (list, defaultStatus, actionBtns) => { ... }` (declarada logo antes de `const ObterTituloJanela`) não é usada em lugar nenhum — a versão real vive em `comum/ListaCartoes.tsx`. Apagar a função inteira, da linha `const RenderizarListaCartoes = ...` até o `};` que a fecha.

**Verificação:** buscar `RenderizarListaCartoes` no projeto não retorna nenhuma ocorrência; `npm run build` passa.

### Etapa 3 — Não reenviar destaques idênticos ao overlay nativo

- [x] Concluída
- **Arquivo:** `wails_app/frontend/src/App.tsx` — modificar (o `useEffect` cujo array de dependências é `[configuracoesApp?.destacarEstudoTela, configuracoesApp?.destacarEstudoParcialTela, abaAtiva, cartoes, cartoesSecao, cartoesVocabulario, cartaoEmFoco]`)

**O que fazer:** Esse efeito roda a cada mudança de `cartaoEmFoco` (ou seja, a cada movimento do mouse entre cards) e sempre chama as funções de bridge `ShowEstudoHighlights`/`ShowEstudoParcialHighlights`, mesmo quando os boxes calculados são idênticos aos já exibidos — cada chamada cruza a bridge Go↔WebView e redesenha o overlay nativo.

1. Criar dois refs junto dos outros refs do componente: `const ultimoDestaquesEnviadosRef = useRef<string>('');` e `const ultimoDestaquesParciaisEnviadosRef = useRef<string>('');`
2. Declarar, logo acima do `useEffect`, a função `const enviarDestaquesSeMudou = (boxes: number[][], boxesParciais: number[][]) => { ... }` que:
   - serializa `boxes` com `JSON.stringify`; se a string difere de `ultimoDestaquesEnviadosRef.current`, chama `window.go.main.App.ShowEstudoHighlights(boxes)` (com o mesmo `// @ts-ignore` usado hoje) e atualiza o ref;
   - faz o mesmo para `boxesParciais` com `ultimoDestaquesParciaisEnviadosRef` e `window.go.main.App.ShowEstudoParcialHighlights`, mantendo o check de existência `if (window.go.main.App.ShowEstudoParcialHighlights)` que o código atual já faz.
3. Dentro do `useEffect`, substituir TODAS as chamadas diretas às duas funções de bridge por `enviarDestaquesSeMudou(...)`: os dois blocos de retorno antecipado passam a chamar `enviarDestaquesSeMudou([], [])`, e o envio final vira `enviarDestaquesSeMudou(boxes, boxesParciais)`.

**Verificação:** `npm run build` passa; no `useEffect` não resta nenhuma chamada direta a `ShowEstudoHighlights`/`ShowEstudoParcialHighlights` (todas passam por `enviarDestaquesSeMudou`).

### Etapa 4 — Trocar `.find` por `Map` no status dos cards (O(n²) → O(n))

- [x] Concluída
- **Arquivo:** `wails_app/frontend/src/comum/ListaCartoes.tsx` — modificar

**O que fazer:** Hoje cada card renderizado faz `cartoesVocabulario.find(v => v.Hanzi === hz)` — com a lista "Já Vistas" (que é o próprio vocabulário), isso é quadrático. Logo após o guard de lista vazia (antes do `return`), criar:

```ts
const statusPorHanzi = new Map(cartoesVocabulario.map(v => [v.Hanzi, v.Status]));
```

E trocar a linha `const statusDB = cartoesVocabulario.find(v => v.Hanzi === hz)?.Status || defaultStatus;` por `const statusDB = statusPorHanzi.get(hz) || defaultStatus;`. (`Hanzi` é chave única no banco, então o `Map` dá o mesmo resultado do `find`.)

**Verificação:** `npm run build` passa; buscar `.find(` em `ListaCartoes.tsx` não retorna nenhuma ocorrência.

### Etapa 5 — Emitir `mouse_pos` só quando a posição mudou

- [x] Concluída
- **Arquivo:** `wails_app/loop.go` — modificar (a goroutine comentada como "Goroutine separada para rastrear o mouse velozmente", dentro de `StartBackgroundLoop`)

**O que fazer:** Hoje a goroutine emite o evento `mouse_pos` a cada ~16ms mesmo com o mouse parado — cada emissão é serialização JSON + travessia da bridge. Declarar `ultimoX, ultimoY := -1, -1` antes do `for` da goroutine e só chamar `runtime.EventsEmit(...)` quando `err == nil` **e** `(x != ultimoX || y != ultimoY)`, atualizando `ultimoX, ultimoY = x, y` junto da emissão. O `time.Sleep` do fim da iteração continua rodando sempre, como hoje. Adicionar um comentário de uma linha explicando o porquê (mouse parado = evento redundante).

**Verificação:** `go build -tags desktop,production ./...` em `wails_app` passa.

### Etapa 6 — Reutilizar clientes HTTP (keep-alive com os sidecars)

- [x] Concluída
- **Arquivo:** `wails_app/app.go` — modificar (função `CaptureAndOCR`); `wails_app/tts.go` — modificar (função `sintetizarPinyin`)

**O que fazer:** `CaptureAndOCR` cria `client := &http.Client{}` a cada scan e `sintetizarPinyin` cria um cliente a cada síntese — isso impede o reuso de conexão com os sidecars.
1. Em `app.go`, adicionar a var de pacote `var clienteHttpOcr = &http.Client{}` logo abaixo da var `codificadorPng`, e em `CaptureAndOCR` remover a linha `client := &http.Client{}`, trocando `client.Do(req)` por `clienteHttpOcr.Do(req)`.
2. Em `tts.go`, adicionar a var de pacote `var clienteHttpTts = &http.Client{Timeout: 15 * time.Minute}` logo após o bloco de imports, movendo para junto dela o comentário existente sobre o timeout longo ("Timeout longo de propósito: ..."). Em `sintetizarPinyin`, remover a criação local do cliente e usar `clienteHttpTts.Post(...)`.

**Modelo:** `wails_app/traducao/cliente.go` → var `clienteHTTP` (mesmo padrão de cliente compartilhado de pacote).

**Verificação:** `go build -tags desktop,production ./...` e `go test ./...` em `wails_app` passam.

### Etapa 7 — Não repetir `RegistrarVisto` da mesma palavra a cada scan

- [x] Concluída
- **Arquivo:** `wails_app/progresso/sqlite.go` — modificar (funções `RegistrarVisto`, `RemoveVocab` e `LimparVocabulario`)

**O que fazer:** A cada scan o app chama `RegistrarVisto` para toda palavra e todo caractere detectado — para palavras já registradas, o `INSERT OR IGNORE` não muda nada mas ainda custa um acesso a disco. Adicionar um set de sessão em memória:

1. Declarar as vars de pacote (junto das vars de imagens de sessão já existentes no arquivo):

```go
// vistosSessao evita repetir o INSERT OR IGNORE das mesmas palavras a cada scan — para
// palavras já registradas o comando não muda nada, mas ainda custa um acesso a disco.
var (
	vistosSessaoMu sync.Mutex
	vistosSessao   = map[string]bool{}
)
```

2. Em `RegistrarVisto`, após o guard de `db == nil`: adquirir `vistosSessaoMu`, e se `vistosSessao[hanzi]` for true, liberar o lock e retornar `nil`; senão liberar o lock e seguir. Após o `db.Exec` **sem erro**, adquirir o lock de novo e marcar `vistosSessao[hanzi] = true`. NÃO segurar o lock durante o `db.Exec`.
3. Em `RemoveVocab`, após o `db.Exec` sem erro, remover a palavra do set (`delete(vistosSessao, hanzi)` sob o lock) — assim, se o usuário remover a palavra e ela reaparecer num scan, ela volta a ser registrada como hoje.
4. Em `LimparVocabulario`, após o `DELETE` bem-sucedido, zerar o set (`vistosSessao = map[string]bool{}` sob o lock), pelo mesmo motivo.

**Verificação:** `go build -tags desktop,production ./...` e `go test ./...` em `wails_app` passam.

### Etapa 8 — Parar a busca geral do CEDICT ao atingir o limite

- [x] Concluída
- **Arquivo:** `wails_app/dicionario/cedict.go` — modificar (função `BuscarGeral`)

**O que fazer:** `BuscarGeral` varre o dicionário inteiro mesmo depois de já ter 3000 resultados prioritários (o corte só acontece no fim). Rotular o `for` externo como `varredura:` e, imediatamente após a linha `priority = append(priority, e)`, adicionar:

```go
if len(priority) >= 3000 {
	break varredura
}
```

Não mexer no bloco de montagem final dos resultados (ele continua igual).

**Verificação:** `go build -tags desktop,production ./...` e `go test ./...` em `wails_app` passam.

## Fora do escopo — NÃO fazer

- Não mexer em `python_backend/` (nenhum arquivo), `wails_app/overlay/`, `wails_app/motoresocr/`, `wails_app/motorestts/`, `wails_app/gemini/` nem `wails_app/traducao/`.
- Em `wails_app/app.go`, tocar SOMENTE no que a Etapa 6 pede — as funções `CaptureAndOCR`, `mostrarTodosPopups` e `traduzirLinhasPendentes` acabaram de ser otimizadas pelo arquiteto; não "melhorá-las".
- Não renomear, refatorar ou reformatar código que as etapas não mencionam.
- Não adicionar dependências (npm ou Go), comentários extras ou tratamento de erro além do especificado.
- Não commitar nem mexer em git.

## Dúvidas e bloqueios

*(Executor: registre aqui no formato `Etapa N — descrição do problema`. O arquiteto responderá abaixo de cada item.)*

## Pendências anteriores

- [ ] **Assinatura de código** dos binários dos sidecars (exige certificado; reduz falso positivo de
      antivírus). Hoje a integridade é garantida só por sha256.
- [ ] **Instalador do app (futuro):** deixar o usuário escolher no install quais OCRs e TSS (motores) já vêm
      embutidos — exigindo **pelo menos um** e permitindo **vários** (bundle opcional, evita o download no
      primeiro start). O download sob demanda continua disponível depois. Hoje, sem instalador, o first-run
      já baixa+instala+ativa o RapidOCR padrão sozinho (bootstrapMotorPadrao).

### Armazenamento (ideias futuras)
- [ ] Permitir mover a pasta de modelos para outro disco.
- [ ] Limpeza automática agendada de logs antigos.
