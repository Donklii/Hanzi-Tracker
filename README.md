# Hanzi Tracker

Aplicativo desktop flutuante que captura texto chinês na tela, executa OCR, segmenta as
palavras e exibe **pinyin (com tons)**, **significados** e a **dissecação dos Hanzis** em tempo real — pensado para
estudar chinês enquanto se joga ou lê algo em 中文.

Nesta versão mais recente, o projeto foi **migrado para Go + React (via Wails)** para o backend principal e a interface, mantendo o Python como um eficiente microserviço apenas para o OCR.

## Funcionalidades

- **Pipeline em tempo real:** captura de tela (Go) → OCR (Python/onnxruntime via HTTP) → segmentação (Go/Jieba) → dicionário (Go/CC-CEDICT) → UI (React).
- **Pinyin com tons:** converte o pinyin numerado do CC-CEDICT (`Zhong1 guo2`) para acentos (`Zhōngguó`).
- **Dissecação de Hanzis:** usando dados do MakeMeAHanzi, cada caractere pode ser clicado para ver sua etimologia, radicais, e animações de traços, incluindo redirecionamento de abreviações visuais para os hanzis completos originais.
- **Estudo/aprendido:** marque cada hanzi como **em estudo** (azul claro) ou **aprendido** (verde). O status é salvo em um banco SQLite e persiste entre sessões, inclusive populando o histórico.
- **Tradução sob o mouse:** um pop-up hover mostra em tempo real a palavra chinesa **mais próxima do cursor** na tela. Como o OCR devolve linhas inteiras, a posição de cada palavra é estimada repartindo a largura da linha proporcionalmente à quantidade de caracteres, de forma que o card focado corresponde à posição horizontal do mouse.
- **Mostrar pop-up de tudo:** um atalho global exibe simultaneamente os pop-ups de **todas** as palavras detectadas sobre a tela (toggle: aperte de novo para esconder). Os pop-ups se reposicionam e diminuem de tamanho automaticamente para não se sobreporem.
- **Modelos de OCR baixáveis:** além do modelo embutido, é possível baixar outros modelos RapidOCR sob demanda pela própria interface (aba *OCR & Processamento → Gerenciar Modelos*). O download é feito pelo Go direto no `AppData`, com barra de progresso, e o select de modelo mostra apenas os modelos disponíveis (embutidos ou já baixados).
- **Compatibilidade hardware × modelo:** ao escolher um modelo, as opções de hardware/aceleração incompatíveis são desabilitadas (com tooltip explicando o motivo). Se o modelo selecionado não suportar o hardware atual, a configuração migra automaticamente para uma combinação suportada e um aviso é exibido.
- **Qualidade da Imagem (OCR):** controle para reduzir a resolução enviada ao OCR (mantendo a proporção do monitor), equilibrando precisão e uso de CPU/GPU.
- **Gerenciamento de Armazenamento:** seção nas configurações que mostra o espaço usado por cada categoria de dados (modelos, banco, imagens de sessão), com opções para limpar itens individuais ou **excluir tudo**.
- **Configurações Avançadas:** interface rica (estilo Claude Desktop) para editar todas as constantes: hardware de IA (CPU vs DirectML vs CUDA), pausa por limite de uso, threads, atalhos globais, etc. Tudo salvo dinamicamente no `AppData`.

## Requisitos

- **Go** 1.20+
- **Node.js** (npm) para o Frontend Vite/React.
- **Python** 3.11+ para o microserviço de OCR.
- **Wails CLI** (`go install github.com/wailsapp/wails/v2/cmd/wails@latest`).
- Arquivos de Dicionário em `wails_app/dicionario/`: 
  - `cedict_ts.u8` (CC-CEDICT)
  - `makemeahanzi_dictionary.txt` (MakeMeAHanzi)

## Como Instalar

1. Instale as dependências do motor RapidOCR (padrão) — Tesseract/EasyOCR têm os seus próprios
   `requirements.txt`, usados só pelos scripts em `builds/` ao congelar cada sidecar:
   ```bash
   pip install -r python_backend/motores/rapidocr/requirements.txt
   ```
2. Instale as dependências do frontend:
   ```bash
   cd wails_app/frontend
   npm install
   ```

## Como Executar (Desenvolvimento / Uso Diário)

A maneira mais fácil de rodar o aplicativo de ponta a ponta é usando o script inicializador na raiz:

```bash
go run .
```

**O que este comando faz?**
1. Compila o frontend (Vite/React) automaticamente, se necessário.
2. Inicializa o microserviço Python `python_backend/motores/rapidocr/server.py` (motor padrão) em uma porta local de forma invisível, injetando o diretório de dados via variável de ambiente.
3. Procura pelo executável final compilado do Wails na pasta `wails_app/build/bin/HanziTracker.exe`.
4. Se encontrar, abre o executável. Se não, executa a aplicação Wails em modo fonte (`go run .` dentro de `wails_app`).
5. Ao fechar a janela do aplicativo (ou com `Ctrl+C` no terminal), encerra automaticamente toda a árvore de processos (Wails + Python) para evitar vazamento de memória.

### Modo de desenvolvimento do frontend

Para iterar no frontend com *hot reload* (sem recompilar o executável a cada alteração), use:

```bash
go run . dev
```

Isso sobe o Python e abre a aplicação via `wails dev`, recarregando a UI automaticamente conforme você edita o React.

## Como Compilar / Construir o Wails (Build)

Sempre que você alterar o código-fonte (seja no backend em Go dentro de `wails_app/` ou no frontend em React dentro de `wails_app/frontend/src/`), o executável final não se atualizará sozinho. Para injetar e compilar as novas alterações (ou os novos designs visuais), execute:

```bash
cd wails_app
wails build
```

Isso fará o Wails orquestrar o `npm run build` do frontend em Vite, embutir os recursos (`//go:embed`) e compilar um binário estático e de alta performance (`HanziTracker.exe`). Após isso, voltar a executar o `go run main.go` da raiz abrirá a sua versão atualizada com o servidor Python engatado.

## Onde os dados ficam

O status de estudo, configurações e modelos baixados ficam persistidos automaticamente pelo Go no diretório AppData do Windows do usuário logado:

```
%APPDATA%\HanziTracker\progresso.db        ← status de vocabulário (SQLite)
%APPDATA%\HanziTracker\configuracoes.json  ← configurações alteradas via UI
%APPDATA%\HanziTracker\modelos\RapidOCR\   ← pesos de OCR baixados (.onnx); 1 subpasta por motor
```

> **Nota técnica (ambiente de dev):** o download/remoção dos modelos é feito pelo **Go**, e não pelo Python. Em desenvolvimento, o interpretador Python costuma vir da Microsoft Store, que roda num sandbox e virtualiza as **escritas** em `AppData` para um diretório próprio — então o Python apenas **lê** os modelos, enquanto o Go (processo normal) escreve no caminho real acima. No app distribuído (Python congelado, sem sandbox) essa distinção não existe.
