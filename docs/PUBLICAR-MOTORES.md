# Como publicar os motores (sidecars) do Hanzi Tracker

Este guia responde: **onde** ficam os `.exe`/`.zip` dos sidecars, como eles são gerados e publicados,
e como o app baixa e confia neles.

## Resposta curta

**Ficam neste mesmo repositório, via GitHub *Releases*** (não commitados no Git). A publicação é
**automatizada**: o workflow [publicar-motores.yml](../.github/workflows/publicar-motores.yml) congela
os 4 sidecars num runner Windows e já cria a Release com os assets anexados — basta empurrar uma tag.

> Por isso o `.gitignore` já ignora `*.exe`, `dist/` e `build_env/`: os binários **não** entram no
> histórico do Git. O que entra no Git é o **código** (specs, script, workflow) e o **manifesto** (que
> só aponta URLs + sha256, em [motores_manifesto.go](../wails_app/motores_manifesto.go)).

## Por que Releases, e não commitar o `.exe`

- O Git guarda **todo** o histórico: commitar um `.exe` de dezenas/centenas de MB incha o repositório
  para sempre (mesmo depois de apagado). Releases guardam o binário **fora** da árvore versionada.
- O GitHub **recusa** arquivos > 100 MB num push normal; *assets* de Release aceitam até **2 GB** cada.
- Releases são versionadas por *tag* (ex.: `motores-v4`), então cada versão do sidecar tem URL própria e
  imutável — exatamente o que um manifesto de download precisa.

## Passo a passo (via CI — caminho normal)

1. **Empurre uma tag `motores-vN`** (ex.: `git tag motores-v4 && git push origin motores-v4`) — ou, sem
   criar tag localmente, abra **Actions → Publicar Motores → Run workflow** e informe a tag manualmente
   (`workflow_dispatch`).
2. O workflow [publicar-motores.yml](../.github/workflows/publicar-motores.yml) roda num runner
   `windows-latest`: instala o Tesseract via choco, executa
   [build_sidecars.ps1](../build_sidecars.ps1), calcula o sha256/tamanho de cada zip e **já publica a
   Release** com os 4 artefatos anexados (`ocr_server.zip`, `popup.zip`, `tesseract_server.zip`,
   `easyocr_server.zip`) via `softprops/action-gh-release`.
3. **Pegue os hashes**: aparecem no *Step Summary* do job ("## sha256 dos motores (`<tag>`)") — não
   precisa recalcular nada manualmente.
4. **Pegue a URL de download** de cada asset (padrão estável):
   ```
   https://github.com/<usuario>/<repo>/releases/download/motores-v4/ocr_server.zip
   https://github.com/<usuario>/<repo>/releases/download/motores-v4/popup.zip
   https://github.com/<usuario>/<repo>/releases/download/motores-v4/tesseract_server.zip
   https://github.com/<usuario>/<repo>/releases/download/motores-v4/easyocr_server.zip
   ```
5. **Preencha o manifesto de motores** ([motores_manifesto.go](../wails_app/motores_manifesto.go)) com
   `url`, `sha256` e `tamanhoBytes` de cada artefato — os mesmos valores do *Step Summary*. O sha256 é
   **obrigatório** para binários: o Go confere após baixar (mecânica em `baixarArquivo`) e recusa se não
   bater. Ver o passo a passo detalhado em [TODO.md](../TODO.md) (seção "Fase 5").

## Alternativa: publicar manualmente (sem empurrar tag)

Útil para testar um build localmente antes de publicar de verdade, ou se o runner do GitHub Actions
estiver indisponível.

1. **Gere os artefatos** (na raiz do projeto, requer Windows):
   ```powershell
   powershell -ExecutionPolicy Bypass -File build_sidecars.ps1
   ```
   No fim ele imprime, para cada zip, o `tamanho_bytes` e o `sha256`. **Guarde esses valores.**
   Saída: `python_backend/dist/{ocr_server,popup,tesseract_server,easyocr_server}.zip` (o
   `tesseract_server` exige a instalação do Tesseract — `choco install tesseract` — ou `TESSERACT_DIR`).

2. **Crie a Release** apontando para uma tag (pela UI do GitHub ou pelo `gh`):
   ```powershell
   gh release create motores-v4 `
     python_backend/dist/ocr_server.zip `
     python_backend/dist/popup.zip `
     python_backend/dist/tesseract_server.zip `
     python_backend/dist/easyocr_server.zip `
     --title "Motores v4 (RapidOCR + Tesseract + EasyOCR + overlay)" `
     --notes "Sidecars congelados (PyInstaller; RapidOCR com DirectML embutido)."
   ```
   **Sem o `gh` (direto pela web):** em *Releases → Draft a new release*, os campos do formulário
   equivalem aos parâmetros do comando acima:

   | Campo na página do GitHub          | Valor a preencher                                                 |
   |-------------------------------------|---------------------------------------------------------------------|
   | **Choose a tag**                    | `motores-v4` → clique em "Create new tag: motores-v4 on publish"    |
   | **Target**                          | branch padrão (`main`) — não precisa mexer                          |
   | **Release title**                   | `Motores v4 (RapidOCR + Tesseract + EasyOCR + overlay)`              |
   | **Describe this release**           | `Sidecars congelados (PyInstaller; RapidOCR com DirectML embutido).` |
   | **Attach binaries by dropping...**  | arraste os quatro `.zip` de `python_backend/dist/`                   |

   Deixe *"Set as the latest release"* marcado e *"Set as a pre-release"* desmarcado — é o
   comportamento padrão do `gh release create` (sem `--prerelease`).

3. Siga a partir do passo 4 do fluxo via CI acima (pegar URLs + preencher o manifesto).

> Se publicar manualmente numa tag que o CI **também** dispararia (`motores-v*`), o push da tag ainda vai
> acionar o workflow — ele recongela tudo do zero e tenta publicar na mesma Release. Prefira sempre o
> fluxo via CI; use o manual só sem empurrar a tag correspondente (ou apague o workflow run depois).

## Como o app consome (Passo 5 — implementado)

- O manifesto de motores vive no Go ([wails_app/motores_manifesto.go](../wails_app/motores_manifesto.go)):
  cada motor aponta `url` + `sha256` + `tamanhoBytes` do `.zip`, mais o `executavel` (o `.exe` na raiz do
  zip). O overlay compartilhado (`popup.zip`) fica em `PopupOverlayBaixavel`.
- `BaixarMotor(nome)` ([wails_app/motores.go](../wails_app/motores.go)) baixa o `.zip` para
  `%APPDATA%\HanziTracker\motores\<Motor>\`, **verifica o sha256** (obrigatório — reusa `baixarArquivo`),
  faz **pré-checagem de espaço em disco** e extrai (com proteção contra *Zip Slip*); o overlay vai para
  `motores\_overlay\`. `RemoverMotor` apaga a pasta (recusa se o motor estiver ativo) e `TrocarMotor` faz
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
