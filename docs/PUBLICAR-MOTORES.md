# Como publicar os motores (sidecars) do Hanzi Tracker

Este guia responde: **onde** ficam os `.exe`/`.zip` gerados por [build_sidecars.ps1](../build_sidecars.ps1),
se dá para hospedar **neste mesmo projeto**, e como o app baixa e confia neles.

## Resposta curta

**Sim, pode ser neste mesmo repositório — via GitHub *Releases*, não commitando o binário no Git.**
Os artefatos são grandes e mudam a cada build; o lugar certo para eles são os *assets* de uma Release
(o `.git` fica leve, e cada asset ganha uma URL pública e estável que o app baixa por HTTPS).

> Por isso o `.gitignore` já ignora `*.exe`, `dist/` e `build_env/`: os binários **não** entram no
> histórico do Git. O que entra no Git é o **código** (specs, script) e o **manifesto** (que só aponta
> URLs + sha256).

## Por que Releases, e não commitar o `.exe`

- O Git guarda **todo** o histórico: commitar um `.exe` de dezenas/centenas de MB incha o repositório
  para sempre (mesmo depois de apagado). Releases guardam o binário **fora** da árvore versionada.
- O GitHub **recusa** arquivos > 100 MB num push normal; *assets* de Release aceitam até **2 GB** cada.
- Releases são versionadas por *tag* (ex.: `motores-v1`), então cada versão do sidecar tem URL própria e
  imutável — exatamente o que um manifesto de download precisa.

## Passo a passo (GitHub Releases, mesmo repo)

1. **Gere os artefatos** (na raiz do projeto):
   ```powershell
   powershell -ExecutionPolicy Bypass -File build_sidecars.ps1
   ```
   No fim ele imprime, para cada zip, o `tamanho_bytes` e o `sha256`. **Guarde esses valores.**
   Saída: `python_backend/dist/ocr_server.zip` e `python_backend/dist/popup.zip`.

2. **Crie uma Release** apontando para uma tag (pela UI do GitHub ou pelo `gh`):
   ```powershell
   gh release create motores-v1 `
     python_backend/dist/ocr_server.zip `
     python_backend/dist/popup.zip `
     --title "Motores v1 (RapidOCR + overlay)" `
     --notes "Sidecars congelados (PyInstaller, DirectML embutido)."
   ```
   **Sem o `gh` (direto pela web):** em *Releases → Draft a new release*, os campos do formulário
   equivalem aos parâmetros do comando acima:

   | Campo na página do GitHub          | Valor a preencher                                                 |
   |-------------------------------------|---------------------------------------------------------------------|
   | **Choose a tag**                    | `motores-v1` → clique em "Create new tag: motores-v1 on publish"    |
   | **Target**                          | branch padrão (`main`) — não precisa mexer                          |
   | **Release title**                   | `Motores v1 (RapidOCR + overlay)`                                    |
   | **Describe this release**           | `Sidecars congelados (PyInstaller, DirectML embutido).`              |
   | **Attach binaries by dropping...**  | arraste `ocr_server.zip` e `popup.zip` de `python_backend/dist/`     |

   Deixe *"Set as the latest release"* marcado e *"Set as a pre-release"* desmarcado — é o
   comportamento padrão do `gh release create` (sem `--prerelease`).

3. **Pegue a URL de download** de cada asset. O padrão é estável:
   ```
   https://github.com/<usuario>/<repo>/releases/download/motores-v1/ocr_server.zip
   https://github.com/<usuario>/<repo>/releases/download/motores-v1/popup.zip
   ```

4. **Preencha o manifesto de motores** (Passo 3 do TODO) com `url`, `sha256` e `tamanho_bytes` de cada
   artefato — os mesmos valores que o script imprimiu. O sha256 é **obrigatório** para binários: o Go
   confere após baixar (a mecânica já existe em `baixarArquivo`) e recusa se não bater.

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
- Falta só a **UI "Gerenciar Motores"** (Passo 6) que chama `ListarMotores`/`BaixarMotor`/`TrocarMotor`/
  `RemoverMotor` e escuta esses eventos.

## Alternativas de hospedagem (também por HTTPS)

- **Hugging Face** — já usamos o `SWHL/RapidOCR` para os pesos ONNX; dá para criar um repositório de
  *models* seu e subir os `.zip` (bom para arquivos grandes, CDN incluso). URL: `.../resolve/main/...`.
- **Cloudflare R2 / S3 / qualquer storage** — funciona igual: o manifesto só precisa da URL pública e do
  sha256. Prefira sempre **HTTPS**.

A escolha não muda o código do app: ele só lê `url` + `sha256` do manifesto.

## Automação opcional (CI — Passo 3)

Como o congelamento é Windows-only (PyInstaller + DirectML), uma automação usaria um *runner*
`windows-latest` no GitHub Actions para: rodar `build_sidecars.ps1`, calcular o sha256 e anexar os zips
à Release da tag. Assim cada tag `motores-v*` publica os binários já com o hash — sem build manual.

> **Aviso de antivírus:** `.exe` de PyInstaller é frequentemente marcado por heurística. Vale orientar o
> usuário na UI e, no futuro, considerar **assinatura de código** dos binários.
