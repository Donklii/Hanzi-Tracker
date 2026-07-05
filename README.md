# Hanzi Tracker

Aplicativo desktop flutuante que captura texto chinês na tela, executa OCR, segmenta as
palavras e exibe **pinyin (com tons)**, **significados** e a **dissecação dos Hanzis** em tempo real — pensado para
estudar chinês enquanto se joga ou lê algo em 中文.

O backend principal e a interface são **Go + React (via Wails)**; os motores de OCR e de voz rodam como
microserviços Python próprios, baixados sob demanda (ver [Motores](#motores-de-ocr-e-voz-baixados-sob-demanda)).

## Como obter (usuário final)

A forma mais simples é baixar o **instalador Windows** nas
[GitHub Releases](../../releases) deste repositório — não exige Go, Node nem Python instalados.
Durante a instalação você escolhe qual motor de OCR (e, opcionalmente, de voz) usar; o app baixa
esse motor sozinho na primeira abertura. Veja [docs/PUBLICAR-APP.md](docs/PUBLICAR-APP.md) para como
o instalador é gerado e publicado.

O restante deste documento cobre como rodar e compilar o projeto **a partir do código-fonte**
(para desenvolvimento).

## Funcionalidades

- **Pipeline em tempo real:** captura de tela (Go) → OCR (Python via HTTP) → segmentação (Go/Jieba) → dicionário (Go/CC-CEDICT) → UI (React).
- **Pinyin com tons:** converte o pinyin numerado do CC-CEDICT (`Zhong1 guo2`) para acentos (`Zhōngguó`).
- **Dissecação de Hanzis:** usando dados do MakeMeAHanzi, cada caractere pode ser clicado para ver sua etimologia, radicais, e animações de traços, incluindo redirecionamento de abreviações visuais para os hanzis completos originais.
- **Estudo/aprendido:** marque cada hanzi como **em estudo** (azul claro) ou **aprendido** (verde). O status é salvo em um banco SQLite e persiste entre sessões, inclusive populando o histórico.
- **Tradução sob o mouse:** um pop-up hover mostra em tempo real a palavra chinesa **mais próxima do cursor** na tela. Como o OCR devolve linhas inteiras, a posição de cada palavra é estimada repartindo a largura da linha proporcionalmente à quantidade de caracteres, de forma que o card focado corresponde à posição horizontal do mouse.
- **Mostrar pop-up de tudo:** um atalho global exibe simultaneamente os pop-ups de **todas** as palavras detectadas sobre a tela (toggle: aperte de novo para esconder). Os pop-ups se reposicionam e diminuem de tamanho automaticamente para não se sobreporem.
- **Tradução de linha (opcional):** ao invés de pinyin/significado por palavra, traduz cada linha OCR inteira via Google Cloud Translation (chave própria do usuário) ou via Gemini (que também pode gerar um **resumo da tela**, opcionalmente enviando a captura). Traduções são cacheadas em SQLite para não repetir custo de API.
- **Leitura em voz alta (TTS):** lê o hanzi de qualquer card em voz alta usando um motor de voz local (Kokoro-82M, leve, ou ChatTTS, mais natural e pesado), baixado sob demanda. Os áudios sintetizados ficam em cache (por pronúncia, não por hanzi — homófonos compartilham o mesmo áudio) para leituras repetidas saírem instantâneas.
- **Revisão de Hanzis:** modos de prática por **significado**, **fonética** (áudio via TTS), **desenho** (traçar o caractere) e **contexto** (frases reais do Tatoeba com lacuna), sorteando entre as palavras marcadas como "em estudo" e o dicionário geral.
- **Busca global:** campo de busca que consulta hanzi, pinyin ou significado em todo o CC-CEDICT + MakeMeAHanzi de uma vez.
- **Motores de OCR selecionáveis:** RapidOCR (padrão, leve, com aceleração DirectML/CUDA), Tesseract (CPU) e EasyOCR (CPU) — troque de motor a qualquer momento pela interface; cada um baixa seus próprios pesos sob demanda.
- **Compatibilidade hardware × modelo:** ao escolher um modelo, as opções de hardware/aceleração incompatíveis são desabilitadas (com tooltip explicando o motivo). Se o modelo selecionado não suportar o hardware atual, a configuração migra automaticamente para uma combinação suportada e um aviso é exibido.
- **Qualidade da Imagem (OCR):** controle para reduzir a resolução enviada ao OCR (mantendo a proporção do monitor), equilibrando precisão e uso de CPU/GPU.
- **Gerenciamento de Armazenamento:** seção nas configurações que mostra o espaço usado por cada categoria de dados (motores, modelos, banco, cache de tradução/voz), com opções para limpar itens individuais ou **excluir tudo**.
- **Configurações Avançadas:** interface rica (estilo Claude Desktop) para editar todas as constantes: hardware de IA (CPU vs DirectML vs CUDA), pausa por limite de uso, threads, atalhos globais, etc. Tudo salvo dinamicamente no `AppData`.

## Motores de OCR e voz (baixados sob demanda)

Os motores de OCR (RapidOCR, Tesseract, EasyOCR) e de voz (Kokoro-82M, ChatTTS) são microserviços
Python **congelados** (PyInstaller), publicados como `.zip` nas
[GitHub Releases](../../releases) deste repositório e baixados pelo próprio app para
`%APPDATA%\HanziTracker\motores_ocr\`/`motores_tts\` — não é necessário Python instalado para usar o
app (nem em produção, nem rodando `go run .` a partir do código-fonte). No primeiro start sem nenhum
motor de OCR instalado, o app baixa e ativa o RapidOCR sozinho (ou o motor escolhido no instalador,
se você usou um). Veja [docs/PUBLICAR-MOTORES.md](docs/PUBLICAR-MOTORES.md) para os detalhes.

## Requisitos (desenvolvimento)

- **Go** 1.25+
- **Node.js** (npm) para o Frontend Vite/React — instalado automaticamente pelo Wails a cada build/dev.
- **Wails CLI** (`go install github.com/wailsapp/wails/v2/cmd/wails@latest`).
- Arquivos de Dicionário em `wails_app/dicionario/`:
  - `cedict_ts.u8` (CC-CEDICT)
  - `makemeahanzi_dictionary.txt` (MakeMeAHanzi)

> **Python só é necessário se você for mexer no código dos motores** (`python_backend/`) ou recongelar
> os sidecars para publicar uma nova versão (ver [docs/BUILD.md](docs/BUILD.md) e
> [docs/PUBLICAR-MOTORES.md](docs/PUBLICAR-MOTORES.md)). Para simplesmente rodar/desenvolver o app,
> `go run .` já baixa e sobe o motor de OCR sozinho, exatamente como no app distribuído.

## Como Executar (Desenvolvimento / Uso Diário)

A maneira mais fácil de rodar o aplicativo de ponta a ponta é usando o script inicializador na raiz:

```bash
go run .
```

**O que este comando faz?**
1. Compila o frontend (Vite/React) automaticamente, se necessário.
2. Reserva uma porta local para o motor de OCR e a pasta de dados (`%APPDATA%\HanziTracker`), repassadas por variável de ambiente.
3. Procura pelo executável final compilado do Wails na pasta `wails_app/build/bin/HanziTracker.exe`.
4. Se encontrar, abre o executável. Se não, executa a aplicação Wails em modo fonte (`go run .` dentro de `wails_app`).
5. O **próprio app Wails** (não mais este orquestrador) sobe/derruba o motor de OCR: se nenhum motor estiver instalado, ele baixa e ativa o RapidOCR sozinho (first-run) — sem precisar de Python/pip instalados localmente.
6. Ao fechar a janela do aplicativo (ou com `Ctrl+C` no terminal), encerra automaticamente toda a árvore de processos (Wails + motor de OCR/voz) para evitar vazamento de memória.

### Modo de desenvolvimento do frontend

Para iterar no frontend com *hot reload* (sem recompilar o executável a cada alteração), use:

```bash
go run . dev
```

Isso abre a aplicação via `wails dev`, recarregando a UI automaticamente conforme você edita o React.

## Como Compilar / Construir o Wails (Build)

Sempre que você alterar o código-fonte (seja no backend em Go dentro de `wails_app/` ou no frontend em React dentro de `wails_app/frontend/src/`), o executável final não se atualizará sozinho. Para injetar e compilar as novas alterações (ou os novos designs visuais), execute:

```bash
cd wails_app
wails build
```

Isso fará o Wails orquestrar o `npm run build` do frontend em Vite, embutir os recursos (`//go:embed`) e compilar um binário estático e de alta performance (`HanziTracker.exe`). Após isso, voltar a executar o `go run main.go` da raiz abrirá a sua versão atualizada.

Para gerar o **instalador Windows** (NSIS, com a tela de escolha de motor) em vez do `.exe` solto, veja
[docs/PUBLICAR-APP.md](docs/PUBLICAR-APP.md) — normalmente isso é feito pela CI a cada push/release,
não manualmente.

## Onde os dados ficam

O status de estudo, configurações, motores e caches ficam persistidos automaticamente pelo Go no
diretório AppData do Windows do usuário logado:

```
%APPDATA%\HanziTracker\progresso.db               ← vocabulário + cache de tradução e de áudio TTS (SQLite)
%APPDATA%\HanziTracker\configuracoes.json          ← configurações alteradas via UI
%APPDATA%\HanziTracker\motores_ocr\<Motor>\        ← sidecar de OCR baixado (.exe + pesos em modelos\)
%APPDATA%\HanziTracker\motores_tts\<Motor>\        ← sidecar de voz baixado (.exe + pesos do Hugging Face)
```

> **Nota técnica:** o download/remoção dos motores é feito pelo **Go**, e não pelo Python. Em
> desenvolvimento, o interpretador Python (quando usado para mexer nos sidecars) costuma vir da
> Microsoft Store, que roda num sandbox e virtualiza as **escritas** em `AppData` para um diretório
> próprio — então o Python apenas **lê** os dados nesse caminho real, enquanto o Go (processo normal)
> escreve nele. No app distribuído (Python congelado, sem sandbox) essa distinção não existe.
