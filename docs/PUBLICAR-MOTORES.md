# Como publicar os motores (sidecars) do Hanzi Tracker

Este guia responde: **onde** ficam os `.exe`/`.zip` dos sidecars, como eles são gerados e publicados,
e como o app baixa e confia neles.

## Resposta curta

**Ficam neste mesmo repositório, via GitHub *Releases*** (não commitados no Git). A publicação é
**automatizada** e dividida por tipo de motor, cada um com seu workflow, script de build e tag próprios:

| Tipo | Workflow | Script | Tag | Manifesto (dados voláteis) | Artefatos |
|------|----------|--------|-----|-----------|-----------|
| **OCR (Windows)** | [publicar-motores-ocr-windows.yml](../.github/workflows/publicar-motores-ocr-windows.yml) | [builds/build_sidecars_ocr_windows.ps1](../builds/build_sidecars_ocr_windows.ps1) | `motores-ocr-windows-v*` | [motoresocr/artefatos_ocr.json](../wails_app/motoresocr/artefatos_ocr.json) | `ocr_server.zip`, `tesseract_server.zip`, `easyocr_server.zip` |
| **OCR (Linux)** | [publicar-motores-ocr-linux.yml](../.github/workflows/publicar-motores-ocr-linux.yml) | [builds/build_sidecars_ocr_linux.sh](../builds/build_sidecars_ocr_linux.sh) | `motores-ocr-linux-v*` | [motoresocr/artefatos_ocr_linux.json](../wails_app/motoresocr/artefatos_ocr_linux.json) | `ocr_server_linux.zip`, `easyocr_server_linux.zip` |
| **Voz/TTS (Windows)** | [publicar-motores-tts-windows.yml](../.github/workflows/publicar-motores-tts-windows.yml) | [builds/build_sidecars_tts_windows.ps1](../builds/build_sidecars_tts_windows.ps1) | `motores-tts-windows-v*` | [motorestts/artefatos_tts.json](../wails_app/motorestts/artefatos_tts.json) | `kokoro_server.zip`, `chattts_server.zip` |
| **Voz/TTS (Linux)** | [publicar-motores-tts-linux.yml](../.github/workflows/publicar-motores-tts-linux.yml) | [builds/build_sidecars_tts_linux.sh](../builds/build_sidecars_tts_linux.sh) | `motores-tts-linux-v*` | [motorestts/artefatos_tts_linux.json](../wails_app/motorestts/artefatos_tts_linux.json) | `kokoro_server_linux.zip`, `chattts_server_linux.zip` |
| **Escuta/STT (Windows)** | [publicar-motores-stt-windows.yml](../.github/workflows/publicar-motores-stt-windows.yml) | [builds/build_sidecars_stt_windows.ps1](../builds/build_sidecars_stt_windows.ps1) | `motores-stt-windows-v*` | [motoresstt/artefatos_stt.json](../wails_app/motoresstt/artefatos_stt.json) | `paraformer_server.zip` |
| **Escuta/STT (Linux)** | [publicar-motores-stt-linux.yml](../.github/workflows/publicar-motores-stt-linux.yml) | [builds/build_sidecars_stt_linux.sh](../builds/build_sidecars_stt_linux.sh) | `motores-stt-linux-v*` | [motoresstt/artefatos_stt_linux.json](../wails_app/motoresstt/artefatos_stt_linux.json) | `paraformer_server_linux.zip` |

Cada workflow congela os seus sidecars no runner do SO correspondente, cria a Release com os assets
anexados **e atualiza o manifesto sozinho** — basta empurrar a tag correspondente. As tags são
**separadas** de propósito: publicar uma release de voz não recongela nem invalida as URLs dos motores
de OCR (e vice-versa), e o mesmo vale entre Windows e Linux — **cada SO tem seu próprio JSON de dados
voláteis**, e o app escolhe qual usar por `runtime.GOOS`. O restante deste guia usa o fluxo de OCR
Windows como exemplo; os demais são idênticos, trocando workflow/script/tag/manifesto pela linha da
tabela (particularidades do Linux na seção [Motores para Linux](#motores-para-linux)).

> **Manifesto = catálogo (Go) + dados voláteis (JSON).** Os rótulos, descrições e o nome do `.exe` de
> cada motor vivem no Go ([motoresocr/manifesto.go](../wails_app/motoresocr/manifesto.go),
> [motorestts/manifesto.go](../wails_app/motorestts/manifesto.go),
> [motoresstt/manifesto.go](../wails_app/motoresstt/manifesto.go)). O que **muda a cada release** — a
> tag embutida na URL, o `sha256` e o tamanho de cada zip — fica num JSON à parte
> (`artefatos_ocr.json` / `artefatos_tts.json` / `artefatos_stt.json`), embutido no binário via
> `go:embed` e injetado no catálogo por um `init()`. É esse JSON, e só ele, que o CI reescreve e
> commita a cada publicação.

> Por isso o `.gitignore` já ignora `*.exe`, `dist/` e `build_env/`: os binários **não** entram no
> histórico do Git. O que entra no Git é o **código** (specs, script, workflow) e o **manifesto** (que
> só aponta URLs + sha256, no catálogo Go + `artefatos_*.json`).

## Por que Releases, e não commitar o `.exe`

- O Git guarda **todo** o histórico: commitar um `.exe` de dezenas/centenas de MB incha o repositório
  para sempre (mesmo depois de apagado). Releases guardam o binário **fora** da árvore versionada.
- O GitHub **recusa** arquivos > 100 MB num push normal; *assets* de Release aceitam até **2 GB** cada.
- Releases são versionadas por *tag* (ex.: `motores-ocr-windows-v4`), então cada versão do sidecar tem URL própria e
  imutável — exatamente o que um manifesto de download precisa.

## Passo a passo (via CI — caminho normal)

1. **Empurre uma tag `motores-ocr-windows-vN`** (ex.: `git tag motores-ocr-windows-v5 && git push origin motores-ocr-windows-v5`) — ou, sem
   criar tag localmente, abra **Actions → Publicar Motores (OCR) → Run workflow** e informe a tag
   manualmente (`workflow_dispatch`). Para os motores de voz, use a tag `motores-tts-windows-vN` e o workflow
   **Publicar Motores (Voz/TTS)**.
2. O workflow [publicar-motores-ocr-windows.yml](../.github/workflows/publicar-motores-ocr-windows.yml) roda num runner
   `windows-latest`: instala o Tesseract via choco, executa
   [builds/build_sidecars_ocr_windows.ps1](../builds/build_sidecars_ocr_windows.ps1), calcula o sha256/tamanho de cada zip
   e **publica a Release** com os 3 artefatos anexados (`ocr_server.zip`, `tesseract_server.zip`,
   `easyocr_server.zip`) via `softprops/action-gh-release`. O de voz
   ([publicar-motores-tts-windows.yml](../.github/workflows/publicar-motores-tts-windows.yml)) faz o mesmo com
   `kokoro_server.zip` e `chattts_server.zip` (sem Tesseract).
3. **O manifesto se atualiza sozinho.** Ainda no mesmo job, depois de publicar a Release, o workflow volta
   para a ponta do `main`, reescreve o JSON de dados voláteis (`artefatos_ocr.json` / `artefatos_tts.json`)
   com a tag nova + o sha256/tamanho de cada zip, valida com `go test ./motoresocr/...` (ou
   `./motorestts/...`) e **commita direto no `main`** como `github-actions[bot]`. A URL de cada motor é
   derivada da tag (`.../releases/download/<tag>/<zip>`), então não há nada a colar à mão.
4. **Confira** (opcional): o *Step Summary* lista o sha256 de cada zip, e o commit
   `chore(motores): aponta manifesto ... para <tag> [skip ci]` aparece no `main`. Pronto — o próximo build
   do app já aponta para os motores recém-publicados.

> **Sem colar hash à mão, sem risco de loop.** O `init()` do manifesto injeta `url`/`sha256`/`tamanhoBytes`
> a partir do JSON embutido, e o Go confere o hash após baixar (mecânica em `baixador.BaixarArquivo`),
> recusando se não bater. O commit é feito com o `GITHUB_TOKEN`, que por design **não** dispara novos
> workflows — então atualizar o manifesto não reaciona a publicação.

## Alternativa: publicar manualmente (sem empurrar tag)

Útil para testar um build localmente antes de publicar de verdade, ou se o runner do GitHub Actions
estiver indisponível.

1. **Gere os artefatos** (na raiz do projeto, requer Windows):
   ```powershell
   powershell -ExecutionPolicy Bypass -File builds/build_sidecars_ocr_windows.ps1
   # motores de voz:
   powershell -ExecutionPolicy Bypass -File builds/build_sidecars_tts_windows.ps1
   ```
   No fim cada script imprime, para cada zip, o `tamanho_bytes` e o `sha256`. **Guarde esses valores.**
   Saída de OCR: `python_backend/dist/{ocr_server,tesseract_server,easyocr_server}.zip` (o
   `tesseract_server` exige a instalação do Tesseract — `choco install tesseract` — ou `TESSERACT_DIR`).
   Saída de voz: `python_backend/dist/{kokoro_server,chattts_server}.zip`.

2. **Crie a Release** apontando para uma tag (pela UI do GitHub ou pelo `gh`):
   ```powershell
   gh release create motores-v4 `
     python_backend/dist/ocr_server.zip `
     python_backend/dist/popup.zip `
     python_backend/dist/tesseract_server.zip `
     python_backend/dist/easyocr_server.zip `
     --title "Motores v4 (RapidOCR + Tesseract + EasyOCR + overlay)" `
     --notes "Sidecars congelados (PyInstaller; RapidOCR com WebGPU embutido)."
   ```
   **Sem o `gh` (direto pela web):** em *Releases → Draft a new release*, os campos do formulário
   equivalem aos parâmetros do comando acima:

   | Campo na página do GitHub          | Valor a preencher                                                 |
   |-------------------------------------|---------------------------------------------------------------------|
   | **Choose a tag**                    | `motores-v4` → clique em "Create new tag: motores-v4 on publish"    |
   | **Target**                          | branch padrão (`main`) — não precisa mexer                          |
   | **Release title**                   | `Motores v4 (RapidOCR + Tesseract + EasyOCR + overlay)`              |
   | **Describe this release**           | `Sidecars congelados (PyInstaller; RapidOCR com WebGPU embutido).` |
   | **Attach binaries by dropping...**  | arraste os quatro `.zip` de `python_backend/dist/`                   |

   Deixe *"Set as the latest release"* marcado e *"Set as a pre-release"* desmarcado — é o
   comportamento padrão do `gh release create` (sem `--prerelease`).

3. **Atualize o manifesto à mão** (o CI não roda neste caminho): edite o JSON de dados voláteis
   (`wails_app/motoresocr/artefatos_ocr.json` ou `wails_app/motorestts/artefatos_tts.json`) com a `tag` da
   release e, para cada zip, o `sha256` e o `tamanhoBytes` que o script imprimiu no passo 1. A URL é
   derivada da tag pelo `init()` do manifesto — não precisa escrevê-la. Rode `go test ./motoresocr/...`
   (ou `./motorestts/...`) dentro de `wails_app/` para conferir que o JSON casa com o catálogo.

> Se publicar manualmente numa tag que o CI **também** dispararia (`motores-v*`), o push da tag ainda vai
> acionar o workflow — ele recongela tudo do zero e tenta publicar na mesma Release. Prefira sempre o
> fluxo via CI; use o manual só sem empurrar a tag correspondente (ou apague o workflow run depois).

## Motores para Linux

Os workflows `*-linux.yml` espelham os de Windows com três particularidades:

- **Aceleração**: o RapidOCR usa o **onnxruntime-webgpu** nos dois SOs (WebGPU sobre Vulkan no
  Linux; sobre D3D12 no Windows); no EasyOCR, torch/torchvision vêm do **índice de CPU do PyTorch**
  — no Linux o PyPI padrão puxaria as wheels CUDA (+ ~5 GB de pacotes `nvidia-*`), inúteis e
  grandes demais.
- **Sem Tesseract**: o sidecar Windows empacota a instalação do choco (`tesseract.exe` + DLLs);
  empacotar um Tesseract relocável no Linux é outro projeto. O catálogo do app **omite o Tesseract
  fora do Windows** (`init()` em [motoresocr/manifesto.go](../wails_app/motoresocr/manifesto.go)).
- **Estado pré-publicação**: enquanto a primeira tag `motores-*-linux-v1` não é publicada, o
  `artefatos_*_linux.json` fica com `sha256` vazio — a UI mostra o motor como não publicado e o
  download é recusado em runtime (mesmo modelo que os motores de voz usaram antes da primeira release).

Os zips Linux levam o sufixo `_linux` no nome e o executável dentro deles não tem extensão (padrão do
PyInstaller fora do Windows) — os dois nomes são derivados por SO em
[baixador/plataforma.go](../wails_app/baixador/plataforma.go). A extração no app já garante o bit de
execução (todo arquivo sai 0755 de `baixador.ExtrairZip`).

## Como o app consome (Passo 5 — implementado)

- O manifesto de motores vive no Go ([motoresocr/manifesto.go](../wails_app/motoresocr/manifesto.go) e
  [motorestts/manifesto.go](../wails_app/motorestts/manifesto.go) para voz): cada motor aponta `url` +
  `sha256` + `tamanhoBytes` do `.zip` — os três injetados do `artefatos_*.json` por um `init()` — mais o
  `executavel` (o `.exe` na raiz do zip).
- `BaixarMotor(nome)` ([wails_app/motores.go](../wails_app/motores.go)) baixa o `.zip` para
  `%APPDATA%\HanziTracker\motores_ocr\<Motor>\`, **verifica o sha256** (obrigatório — reusa `baixarArquivo`),
  faz **pré-checagem de espaço em disco** e extrai (com proteção contra *Zip Slip*); o overlay vai para
  `motores_ocr\_overlay\`. Os pesos de cada motor ficam em `motores_ocr\<Motor>\modelos\`. `RemoverMotor`
  apaga a pasta (recusa se o motor estiver ativo) e `TrocarMotor` faz
  o hot-swap via `GerenciadorMotorOcr.Trocar` e **persiste** o motor ativo em `configuracoes.json`.
- Na inicialização, `resolverMotorInicial` sobe o motor preferido/instalado; se **nada** existe (first-run
  distribuído), `bootstrapMotorPadrao` baixa o motor padrão + o overlay e ativa — emitindo os eventos
  `motor_bootstrap_inicio` / `motor_download_progresso` / `ocr_pronto` para o frontend acompanhar.
- A UI "Gerenciar Motores" chama `ListarMotores`/`BaixarMotor`/`TrocarMotor`/`RemoverMotor` e escuta
  esses eventos.

## Alternativas de hospedagem (também por HTTPS)

- **Hugging Face** — já usamos o `SWHL/RapidOCR` para os pesos ONNX; dá para criar um repositório de
  *models* seu e subir os `.zip` (bom para arquivos grandes, CDN incluso). URL: `.../resolve/main/...`.
- **Cloudflare R2 / S3 / qualquer storage** — funciona igual: o manifesto só precisa da URL pública e do
  sha256. Prefira sempre **HTTPS**.

A escolha não muda o código do app: ele só lê `url` + `sha256` do manifesto. Trocar de hospedagem exigiria
adaptar o workflow de publicação (hoje específico para GitHub Releases via `action-gh-release`).

> **Aviso de antivírus:** `.exe` de PyInstaller é frequentemente marcado por heurística. Vale orientar o
> usuário na UI e, no futuro, considerar **assinatura de código** dos binários (já listado no TODO).
