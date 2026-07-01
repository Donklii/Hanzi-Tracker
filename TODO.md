# TODO — Pendências do Projeto Hanzi Tracker

## Fase 2: Migração Completa para Go (Wails) e React

A infraestrutura básica foi migrada e o OCR já está conversando com o Frontend. Os passos a seguir devem ser executados de ponta a ponta:

- [x] **Integração do Dicionário (CEDICT) e Segmentação (Jieba) no Go:**
  - [x] Carregar o dicionário CC-CEDICT na memória usando o pacote `dicionario/cedict.go`.
  - [x] Usar `segmentacao/jieba.go` para segmentar a string bruta do OCR em palavras coerentes.
  - [x] O método `CaptureAndOCR` do Go deve retornar objetos complexos (Palavra, Pinyin, Significado, Coordenadas) em vez de apenas a string bruta.
- [x] **Configurações e Persistência (App Data):**
  - [x] O Go deve ler/escrever o `configuracoes.json` no AppData.
  - [x] Implementar o painel modal completo de configurações em React contendo os mesmos seletores da interface antiga:
    - [x] Intervalo de Varredura (Slider)
    - [x] Confiança Mínima do OCR (Slider)
    - [x] Limite de Threads do OCR (Slider)
    - [x] Hardware de Inferência (CPU/GPU) e API (DirectML/CUDA)
    - [x] Resolução de Captura e Limitadores de CPU/GPU
    - [x] Opções do Hover/Pop-up (Distância, Atraso, etc)
    - [x] Configurações de Atalhos Globais
- [x] **Loop de Varredura Automática:**
  - [x] Implementar uma Goroutine em Go para rodar a varredura a cada X segundos.
  - [x] Implementar a lógica de comparação de imagem (Fingerprint de tela) para economizar recursos caso a tela não tenha mudado.
  - [x] Respeitar os bloqueios de processamento (Limites de CPU/GPU).
- [x] **Tradução em Hover (Pop-up sob o Mouse):**
  - [x] Rastrear a posição do mouse a partir do Go.
  - [x] Exibir um Pop-up flutuante (Janela Wails transparente sem borda) quando o mouse parar sobre as coordenadas de uma caixa detectada.
- [x] **Repositório de Vocabulário (SQLite):**
  - [x] Finalizar o `progresso/sqlite.go` para suportar inserção, atualização e listagem de palavras.
  - [x] Aba de "Meu Vocabulário" em React deve puxar os dados do SQLite para exibir a tabela de palavras em estudo.
- [x] **Atalhos Globais (Hotkeys):**
  - [x] Implementar atalhos de teclado de escopo global no Go para invocar escaneamento forçado, ou marcação rápida de estudo, com base nas configurações ativas.

## Pendências de documentação
- [ ] `docs/interface.png` desatualizado — regerar a captura de tela após finalizar o frontend em React.

## Orquestração e Experiência de Desenvolvimento (main.go raiz)

- [x] **Build unificado:** `go run .` na raiz compila o frontend (`npm install` se faltar + `npm run build`)
      antes de subir o app no modo código-fonte (necessário pelo `//go:embed all:frontend/dist`).
      O `go run` do app usa `-tags "desktop,production"` (exigidas pelo Wails; sem elas o build é recusado
      com "Wails applications will not build without the correct build tags"; `production` usa os assets
      embutidos). O ramo do `.exe` compilado não recompila o frontend (apague o exe ou use `wails build`).
- [x] **Modo dev:** `go run . dev` sobe o app via `wails dev` (hot reload do frontend + recompila o Go),
      pulando o build embutido. Requer o Wails CLI instalado.
- [x] **Encerramento limpo (taskkill /T):** em Ctrl+C ou ao fechar a janela, o orquestrador derruba a
      árvore inteira (Python de OCR, app Wails e o `popup.py` neto) via `taskkill /F /T /PID`.
      (Job Object foi descartado: ao rodar via `go run`, o próprio Go já coloca os processos em jobs
      próprios e a reatribuição falhava com "Access is denied". `taskkill /T` não tem esse problema.)
      Limitação conhecida: não cobre kill abrupto do próprio orquestrador (o handler de sinal não roda).
- [x] **Código morto removido:** o campo `App.pythonProcess` em `wails_app/app.go` (nunca atribuído, pois
      o server.py é iniciado pelo orquestrador raiz) foi removido do struct e do `shutdown`.
- [x] **Vazamento de memória ao trocar de motor:** `_inicializarOcr` agora libera a instância anterior
      (`self._ocr = None` + `gc.collect()`) antes de carregar a nova, evitando o acúmulo que levava ao
      `MemoryError`. Causa agravante eram processos `server.py`/`popup.py` órfãos do bug de encerramento
      (cada um segurando os modelos na RAM) — resolvido pelo `taskkill /T`.
- [x] **Erro de instalação pip mais claro + sem spam:** `_instalarPacotePip` agora junta stderr no stdout,
      desativa o aviso `[notice]` do pip (`PIP_DISABLE_PIP_VERSION_CHECK`) que mascarava a causa real, e
      resume a linha de erro relevante. O `OcrService` memoiza a config que falhou para não reinstalar
      pacotes pesados (nem devolver 500) a cada varredura — só re-tenta quando a config muda.

## Seção de Armazenamento (Configurações)

- [x] **Aba "Armazenamento" funcional:** mostra o uso de disco por categoria (modelos ONNX, modelos
      EasyOCR, cache do pip, logs, banco de vocabulário) + espaço livre do volume. Backend em
      [armazenamento.go](wails_app/armazenamento.go); `progresso.LimparVocabulario()` zera o banco via DELETE+VACUUM.
  - [x] Limpar cada categoria individualmente (botão por item; download/cache reconstruíveis).
  - [x] "Abrir pasta de dados" (Explorer em `%APPDATA%\HanziTracker`).
  - [x] "Excluir Tudo" (zona de perigo): apaga modelos, cache pip, logs e zera o vocabulário; mantém
        as preferências. Ações destrutivas passam por modal de confirmação.
- [ ] **Ideias futuras:** permitir mover a pasta de modelos para outro disco; mostrar barra de uso por
      categoria; agendar limpeza automática de logs antigos.

## Fase 4: Download Dinâmico de Modelos de OCR (AppData) — concluída no essencial

Status: o download dinâmico de **modelos ONNX** (pesos) está funcional. Quem baixa/remove é o Go
(`BaixarModelo`/`RemoverModelo`, escrita atômica + progresso) no AppData real; o Python só LÊ e expõe o
catálogo em `GET /api/modelos`. UI "Gerenciar Modelos" e o select de modelos disponíveis prontos.

Distinção crítica (motiva a Fase 5): **modelos ONNX** (RapidOCR/PP-OCR) são só arquivos — funcionam
mesmo no `.exe` compilado. **Engines pip** (EasyOCR, onnxruntime-directml/gpu) NÃO podem ser instalados
num executável congelado. Trocar de *peso* já é possível aqui; trocar de *motor* sem instalar nada é o
objetivo da **Fase 5** (motores como sidecars baixáveis).

- [x] **Correções de comunicação (pré-requisito):**
  - [x] Go injeta `X-Ocr-Hardware`; `server.py` lê e popula `ConstantesModule.HARDWARE_SELECIONADO`
        (antes o OCR sempre caía na GPU índice 0, ignorando a escolha do usuário).
  - [x] Frontend: `aplicarConfig()` aplica múltiplas chaves num único snapshot (antes a troca de
        hardware era descartada por sobrescrita de estado assíncrono do React).
  - [x] UI de compatibilidade OCR×Hardware×API: tooltips + opções desabilitadas (EasyOCR sem DirectML,
        CUDA só Nvidia, EasyOCR indisponível em GPU não-Nvidia).
  - [x] **Lógica de bloqueio invertida:** o MODELO nunca é bloqueado; o HARDWARE/API é que fica
        desabilitado quando o modelo atual não o suporta. Ao trocar para um modelo incompatível com o
        hardware atual, a config migra automaticamente para uma suportada (CPU/CUDA) e um pop-up avisa.
  - [x] **Erros de inferência tratados:** `MemoryError` e `UnicodeDecodeError` (mensagem nativa do
        DirectML mascarada — tipicamente falta de memória de vídeo) viram mensagens acionáveis
        sugerindo CPU/reduzir resolução, e o motor é descartado para reinício limpo.
  - [x] **Pré-checagem de espaço em disco:** `_instalarPacotePip` aborta com mensagem clara se faltar
        espaço (parâmetro `espaco_minimo_mb`; PyTorch CUDA exige ~6 GB) — evita download pesado falhar
        no meio com `OSError(28, 'No space left on device')` e deixar lixo parcial.
  - [x] **Sem auto-instalação de runtime de GPU em runtime:** removida a instalação de
        `onnxruntime-gpu`/`onnxruntime-directml` via pip durante a inferência. Não funcionava (DLL do
        onnxruntime já carregada → `WinError 5 Acesso negado`; impossível no `.exe` congelado). Agora usa
        a aceleração só se o provedor já estiver presente, senão **cai para CPU graciosamente** (o OCR
        nunca trava).
  - [x] **Estratégia de GPU para distribuição decidida:** embutir `onnxruntime-directml` no build
        (DirectML universal: Nvidia/AMD/Intel, sem CUDA Toolkit, sem download/reinício). Documentado em
        [BUILD.md](BUILD.md); `requirements.txt` ajustado com a nota do conflito onnxruntime × directml.
  - **Empacotamento congelado:** rastreado na **Fase 5, Passo 2** (congelar `server.py`/`popup.py` em
        `.exe` PyInstaller no modo distribuído — vira o sidecar embutido padrão). Instalação de runtime
        de GPU via pip **não** será usada (só funciona no código-fonte; impossível no congelado).
- [x] **Manifesto versionado de modelos** (`ocr/ModelosManifesto.py`) com `nome`, `url`, `sha256`,
      `tamanho_bytes`, `idiomas` por arquivo. Catálogo atual (todos chineses, dict padrão, rec altura 48px
      — compatíveis com o pipeline sem config extra; URLs verificadas com HTTP 206):
  - [x] RapidOCR (embutido) · RapidOCR Server (v4 server) · RapidOCR Detecção Forte (det server + rec leve)
        · RapidOCR v3 (PP-OCRv3). Evitado PP-OCRv2/v2.0 (altura 32 quebraria a inferência).
  - [x] Remoção segura: preserva arquivos compartilhados entre modelos (ex.: det server usado por dois).
  - [x] Select "Modelo de OCR" mostra apenas os modelos disponíveis (embutidos + já baixados); os demais
        ficam na seção "Gerenciar Modelos". EasyOCR saiu do select (não funciona no `.exe` distribuído).
  - [x] **"Qualidade da Imagem (OCR)":** o antigo "Limite Lado Maior" virou um slider que detecta a
        resolução nativa do monitor (`GetCaptureResolution` no Go) como teto/padrão, desce mantendo a
        proporção da tela (exibe `L × A`), com piso (lado menor ≥ 360px) para não chegar a 0×0. Salva 0
        quando no máximo (= sempre nativo, à prova de troca de monitor). Backend já redimensiona proporcional.
  - [x] **Download/remoção feitos pelo Go** (não pelo Python): o Python da Microsoft Store virtualiza
        ESCRITAS em %APPDATA% para um sandbox (mesmo com caminho absoluto), então os modelos baixados
        pelo Python "sumiam". O Go (não-sandbox) baixa/remove no AppData real (`BaixarModelo`/`RemoverModelo`),
        e o Python só LÊ (leitura do real funciona via overlay) e expõe `arquivos:[{nome,url}]` em
        `/api/modelos`. Endpoints `/api/modelos/baixar` e `/remover` removidos. Ver [[appdata-python-store-virtualizado]].
- [x] **Bridge Go (download/remoção):** `ListarModelos()`, `BaixarModelo(nome)` (progresso via
      `runtime.EventsEmit`, evento `modelo_download_progresso`) e `RemoverModelo(nome)` em
      [app.go](wails_app/app.go). `GET /api/modelos` (Python) expõe o catálogo + `arquivos:[{nome,url}]`.
      Os endpoints `POST /api/modelos/baixar` e `/remover` foram **descartados** por design (download
      migrou para o Go por causa do sandbox da Store).
- [x] **UI "Gerenciar Modelos" (React):** seção nas Configurações lista cada modelo com tamanho,
      ✅ instalado / ⬇️ baixar (barra de progresso via evento) / 🗑️ remover.
- [x] **Download atômico:** `baixarArquivo` (Go) grava em `.tmp` e renomeia no fim (evita `.onnx` corrompido).
- [x] **Verificação de integridade (`sha256`):** `baixarArquivo` (Go) calcula o sha256 durante o
      streaming e confere contra o `sha256` do manifesto antes do rename; divergência apaga o `.tmp` e
      aborta. O Python expõe o `sha256` de cada arquivo em `/api/modelos`. Campo vazio no manifesto =
      verificação pulada (hashes dos pesos atuais ainda por preencher); vira **obrigatório** ao baixar
      executáveis (ver Fase 5).
- [ ] **Manifesto remoto (futuro):** buscar o catálogo de uma URL (com cache) para adicionar modelos sem
      recompilar.

## Fase 5: Motores de OCR como Sidecars (executáveis baixáveis sob demanda)

Decisão arquitetural: **nenhum motor vem embutido no app**. Todo motor — inclusive o RapidOCR padrão — é
um **executável autônomo deste mesmo projeto** (congelado via PyInstaller, com o runtime embutido **em
build-time**) que expõe a MESMA API HTTP do `server.py` atual. No primeiro start, o app baixa o RapidOCR
(motor padrão); os demais (EasyOCR, Tesseract, PaddleOCR, variantes de GPU) são baixados sob demanda. O
app conversa por HTTP com `localhost:<porta dinâmica>`, agnóstico de qual motor está atrás.

Por que resolve o que travou antes: deixa de existir "instalar runtime no congelado" (impossível: sem
pip, DLL travada, sandbox, WinError 5) — vira "baixar binário pronto", como já fazemos com os `.onnx`.
Bônus: **isola o conflito `onnxruntime` × `onnxruntime-directml`** (cada motor no seu processo/exe) e
isola falhas (um motor que trava não derruba o app). Absorve as antigas pendências soltas de
"EasyOCR/PaddleOCR dinâmicos" e "EasyOCR + CUDA".

- **Decisão: nenhum motor embutido + bootstrap no primeiro start.** O instalador/binário do app fica
  leve (sem motor nem pesos dentro). Ao abrir pela primeira vez sem motor instalado, o app baixa o
  RapidOCR (motor padrão) automaticamente.
  - [ ] Fluxo de first-run/bootstrap no Go: detectar ausência de motor → baixar o RapidOCR padrão →
        ativar. UI de progresso; tratar offline/falha com re-tentativa; bloquear o OCR até concluir.
  - [ ] RapidOCR deixa de ser "embutido" no manifesto/UI: vira mais um motor baixável — remover o conceito
        `MODELOS_EMBUTIDOS` (Python) e o estado/rótulo "embutido" e "sempre disponível" da UI.
- [ ] **Instalador do aplicativo (futuro):** criar um instalador que deixe o usuário **escolher quais
      motores/pesos já vêm na primeira instalação** (bundle opcional), evitando o download no primeiro
      start para quem optar por incluí-los. O download sob demanda continua disponível depois.
- **Layout de pastas dos pesos (organização por motor):** cada motor guarda seus pesos numa subpasta
  dedicada sob `%APPDATA%\HanziTracker\modelos\<Motor>`, para não colidirem; a raiz `modelos\` é o que a
  aba de Armazenamento mede/limpa (engloba todos os motores). Os `.exe` dos sidecars ficam à parte em
  `%APPDATA%\HanziTracker\motores\`.
  - [x] RapidOCR usa `modelos\RapidOCR\` — Go baixa/remove em `pastaModelosRapidOcr()`; Python lê em
        `obterPastaModelos()`; a aba de Armazenamento aponta para a raiz `modelos\` (`pastaModelos()`).
  - [ ] Cada sidecar futuro declara sua própria subpasta de pesos (ex.: `modelos\EasyOCR\`,
        `modelos\Tesseract\`), isolada das demais.

- [x] **Passo 1 — Fundamento: contrato + backend OCR trocável (local, sem download):** — concluído; o
      gatilho pela UI e um motor concreto para trocar dependem dos Passos 3/5/6.
  - [x] Formalizar o **contrato da API de OCR** (endpoints `/api/health`, `/api/ocr`, `/api/hardware`,
        `/api/modelos`; headers `X-Ocr-*`; resposta `texto/confianca/caixa`; **versão do contrato**) em
        [docs/CONTRATO-OCR.md](docs/CONTRATO-OCR.md). O `server.py` é a implementação de referência
        (= motor padrão RapidOCR); a versão vive em `VERSAO_CONTRATO_OCR` (Python) e `VersaoContratoOcr` (Go).
  - [x] Abstrair "qual backend OCR está ativo" (caminho do executável) no `app.go`, em vez do
        `python server.py` fixo: `resolverMotorOcrPadrao` ([motor_ocr.go](wails_app/motor_ocr.go)) e
        `resolverComandoPopup` ([app.go](wails_app/app.go), overlay) escolhem o sidecar congelado quando
        presente (modo distribuído, sem Python no usuário final) e caem para o Python no código-fonte —
        fecha a pendência de [BUILD.md](BUILD.md) §3.
  - [x] **Posse do processo de OCR migrada do orquestrador para o app** (`GerenciadorMotorOcr` em
        [motor_ocr.go](wails_app/motor_ocr.go)): o `app.go` sobe o motor no `startup` e o derruba no
        `shutdown` (taskkill /T); o `main.go` deixou de iniciar o OCR (só reserva porta + pasta de dados
        via env). Habilita a troca em runtime — a posse antes ficava no orquestrador, longe da UI.
  - [x] Só um motor ativo por vez (RAM): `GerenciadorMotorOcr.Trocar` derruba o atual e sobe o novo na
        MESMA porta dinâmica (`HANZITRACKER_OCR_PORT`), aguardando o healthcheck, com **fallback ao motor
        anterior** se o novo não responder — tudo atômico sob mutex. Primitiva pronta; falta só o gatilho
        pela UI + um motor concreto para trocar (dependem do catálogo/download de sidecar, Passos 3/5/6).
  - [x] Healthcheck: `GET /api/health` (Python) devolve status + versão do contrato; o Go
        (`aguardarBackendOcr`) faz *polling* na inicialização, com timeout, e recusa contrato incompatível,
        emitindo `ocr_pronto`/`ocr_indisponivel` ao frontend. O **fallback ao motor anterior** na troca já
        está implementado em `Trocar` (acima).
- [ ] **Passo 2 — Empacotar o RapidOCR como sidecar baixável (motor padrão, NÃO embutido):** congelar
      `server.py` (+ `popup.py`) em `.exe` PyInstaller, publicado e baixado como os demais motores (no
      código-fonte ainda roda via `python ...`). É a base do bootstrap de first-run e a mesma pendência
      "Empacotamento congelado" da Fase 4.
  - [x] **Ferramenta de congelamento pronta:** specs versionados
        [ocr_server.spec](python_backend/ocr_server.spec) e [popup.spec](python_backend/popup.spec) +
        script [build_sidecars.ps1](build_sidecars.ps1) (venv isolado → troca p/ onnxruntime-directml →
        PyInstaller → zips `dist/*.zip` + sha256/tamanho impressos). Corrige o overlay para `console=True`
        (o `--windowed` zeraria o `sys.stdin` que o popup usa). Sintaxe validada; falta **rodar** o build
        (pesado, local — depende de você) e **publicar** os artefatos (ver Passo 3 e
        [docs/PUBLICAR-MOTORES.md](docs/PUBLICAR-MOTORES.md)).
- [ ] **Passo 3 — Manifesto de motores + distribuição:**
  - [ ] Catálogo de **motores** (separado dos *pesos* ONNX): nome, descrição, idiomas, versão, variante
        (CPU/CUDA/DirectML), requisitos (ex.: GPU Nvidia), URL do `.exe`, `sha256` (OBRIGATÓRIO), tamanho.
  - [ ] Publicar os `.zip` em GitHub Releases (deste mesmo repo) e referenciá-los no manifesto (URL +
        `sha256` + tamanho) — passo a passo em [docs/PUBLICAR-MOTORES.md](docs/PUBLICAR-MOTORES.md).
  - [ ] Pipeline de CI (GitHub Actions, runner `windows-latest`) que roda `build_sidecars.ps1` e anexa os
        zips à Release da tag já com o `sha256` (ver guia acima).
- [ ] **Passo 4 — Sidecar piloto: RapidOCR-DirectML:** pequeno, valida o fluxo ponta a ponta e já resolve
      a dor de GPU (DirectML isolado no seu próprio `.exe`, sem o conflito com o `onnxruntime` CPU do
      sidecar RapidOCR padrão).
- [ ] **Passo 5 — Download + ciclo de vida do sidecar (Go):**
  - [ ] Baixar o `.exe` pro AppData (`%APPDATA%\HanziTracker\motores\`) reusando `baixarArquivo`
        (escrita atômica `.tmp`+rename + progresso).
  - [ ] **Verificar `sha256` antes de habilitar/executar** o motor (não opcional — é binário).
  - [ ] Pré-checagem de espaço em disco antes de baixar sidecars grandes.
  - [ ] `BaixarMotor(nome)`, `RemoverMotor(nome)`, `TrocarMotor(nome)` (derruba ativo → sobe novo →
        healthcheck → recarrega o catálogo de pesos compatível com o motor).
- [ ] **Passo 6 — UI "Gerenciar Motores" (React):**
  - [ ] Lista de motores: embutido / baixável / baixado / **ativo**; tamanho; barra de progresso;
        selecionar ativo; remover. Avisar o tamanho grande (ex.: EasyOCR+PyTorch) e o requisito de GPU.
  - [ ] Atualizar as categorias da aba "Armazenamento" para refletir `motores\` (no lugar do cache pip).
- [ ] **Passo 7 — Motores adicionais:** EasyOCR (CPU e CUDA), Tesseract e PaddleOCR como sidecars
      (substitui de vez a ideia de instalar EasyOCR/PyTorch via pip em runtime).
- [ ] **Cuidados/segurança:**
  - [ ] `sha256` obrigatório + HTTPS; idealmente assinatura de código dos binários.
  - [ ] Aviso de **antivírus**: `.exe` PyInstaller baixado costuma ser marcado por heurística; orientar.
  - [ ] **Versão do contrato:** o app recusa sidecar de contrato incompatível (evita motor desatualizado).

## Fase 3: Refatoração da UI de Configurações (Estilo Claude/Antigravity)
- [X] **Estrutura Visual:**
  - Transformar o modal de configurações de uma lista vertical simples para um painel complexo, com uma sidebar à esquerda para navegação por categorias (ex: "Geral", "OCR", "Performance", "Atalhos").
  - Adicionar uma barra de pesquisa global dentro do modal para filtrar opções.
- [X] **Integração das Opções Faltantes no Frontend:**
  - Embora o backend Go já possua as propriedades em sua estrutura JSON (`config.go`), o frontend ainda precisa de elementos visuais para:
    - `limiteLadoMaiorOcr`
    - `limitarPorUsoCpu` e `usoMaximoCpuPercent`
    - `limitarPorUsoGpu` e `usoMaximoGpuPercent`
    - `distanciaMaximaHoverPx`, `intervaloAtualizacaoHoverMs`, `tempoParadoPopupMs`
    - Input de captura de teclado para os atalhos (`atalhoEscanear`, `atalhoPopupTodos`, `atalhoMarcarEstudo`).
- [X] **Ajustes de Valor Padrão:**
  - Ajustar o fallback no código Go/React para que `habilitarPopupHover` (e as demais flags enviadas) reflitam exatamente a nova lista exigida.

## Features Futuras (independentes)
- [ ] Adicionar textbox para conectar com API do google tradutor + checkbox para substituir os pop-up de tradução do atalho por um unico pop-up por linha com a tradução.
- [ ] EasyOCR (incl. CUDA), Tesseract e PaddleOCR como motores dinâmicos → ver **Fase 5** (sidecars
      baixáveis; não via pip em runtime, que não funciona no `.exe` congelado).
- [ ] Adicionar textbox para conectar com API do gemini + checkbox para substituir os pop-up de tradução do atalho por um unico pop-up de um resumo feito pela IA.
- [ ] Adicionar censura em espaços nos prints antes de envia-los para o OCR. Como censurar a area do aplicativo visivel, caso ele esteja na tela alvo. Fazer o mesmo para a area dos pop-ups, que já são always on top.
- [ ] Adicionar modelos para ler em voz alta o pinyin, poder deixar o usuario alternar entre Kokoro-82M e ChatTTS e desligar a feature