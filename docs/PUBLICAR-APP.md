# Como publicar o instalador do Hanzi Tracker

Este guia responde: **quando** o instalador Windows é gerado, **onde** ele fica disponível, como a
versão é decidida e como funciona a tela de escolha de motor de OCR/voz dentro dele.

## Resposta curta

A publicação é **automatizada** pelo workflow
[publicar-app.yml](../.github/workflows/publicar-app.yml), que builda o app Wails (Go + React) e gera
o instalador via NSIS, publicando em **GitHub Releases** (mesmo lugar dos motores — ver
[PUBLICAR-MOTORES.md](PUBLICAR-MOTORES.md)):

| Gatilho | Versão do instalador | Release |
|---------|----------------------|---------|
| **Push na `main`** | `0.0.<número sequencial do workflow>` | `app-dev` — **prerelease rolante**: a mesma tag é atualizada a cada push, sempre com o instalador mais recente. O título traz o hash do commit para rastreio. |
| **Tag `app-vX.Y.Z`** (ex.: `app-v1.2.0`) | `X.Y.Z` (vem da própria tag) | Release **estável**, versionada, permanente. |
| **`workflow_dispatch`** manual | Segue a mesma regra acima, conforme a branch/tag escolhida ao disparar | — |

O instalador **não embute nenhum motor de OCR/voz** — ele mostra uma tela de escolha (RapidOCR /
Tesseract / EasyOCR para OCR; Nenhum / Kokoro-82M / ChatTTS para voz) e só grava essa escolha para o
app baixar sozinho no primeiro start, reaproveitando o download-sob-demanda que já existe.

## Por que uma release "dev" rolante, e não uma a cada commit

Builda a cada push na `main` (não a cada commit de toda branch): o app em si é leve (Go + React, sem
congelamento PyInstaller), então isso custa só um a dois minutos de CI. Uma tag fixa (`app-dev`) é
reaproveitada a cada execução em vez de criar uma release nova por push — assim a aba de Releases não
enche de dezenas de entradas sem significado, e tags `app-vX.Y.Z` continuam reservadas para versões
"de verdade" que valem a pena anunciar ao usuário final.

## Passo a passo (via CI — caminho normal)

1. **Push normal na `main`** → já dispara sozinho e atualiza a release `app-dev` (prerelease) com o
   instalador mais recente. Bom para sempre ter algo pronto pra testar.
2. **Publicar uma versão estável**: `git tag app-v1.2.0 && git push origin app-v1.2.0` (ou, sem criar
   tag localmente, **Actions → Publicar App (Instalador) → Run workflow**, escolhendo a tag no
   dropdown "Use workflow from"). O workflow reconhece o padrão `app-v*` e publica uma release estável
   e permanente com esse número de versão.
3. O workflow [publicar-app.yml](../.github/workflows/publicar-app.yml) roda num runner
   `windows-latest`: instala o NSIS via choco, instala o Wails CLI, calcula a versão (tabela acima),
   grava-a em `wails_app/wails.json` (`info.productVersion`), copia o template NSIS customizado
   ([nsis-instalador/project.nsi](../wails_app/nsis-instalador/project.nsi)) para
   `wails_app/build/windows/installer/` (pasta gerada e gitignorada — por isso o template-fonte mora
   fora dela) e roda `wails build -nsis`. O instalador sai em
   `wails_app/build/bin/HanziTracker-amd64-installer.exe` e é anexado à release.

> **Primeira execução:** dispare manualmente pela aba Actions (`workflow_dispatch`) antes de confiar no
> gatilho automático — é a única forma de validar de ponta a ponta que o `makensis` compila o
> `project.nsi` customizado sem erro (isso não dá pra testar localmente sem instalar o NSIS).

## A tela de escolha de motores (dentro do instalador)

Definida em [nsis-instalador/project.nsi](../wails_app/nsis-instalador/project.nsi), como uma página
custom do NSIS (`nsDialogs`) inserida entre a escolha de pasta e a instalação dos arquivos:

- **OCR** (obrigatório, RapidOCR pré-selecionado): RapidOCR / Tesseract / EasyOCR.
- **Voz/TTS** (opcional, "Nenhum" pré-selecionado): Nenhum / Kokoro-82M / ChatTTS.

Ao concluir a instalação, a escolha é gravada em texto simples (não precisa de plugin de JSON no
NSIS) em `%APPDATA%\HanziTracker\instalador_escolha.json`. **Nenhum motor é baixado nem embutido pelo
instalador** — ele só grava a escolha.

No primeiro start, `aplicarEscolhaDoInstalador` ([wails_app/instalador.go](../wails_app/instalador.go))
lê esse marcador, valida os nomes contra o catálogo real (`motoresocr`/`motorestts`), grava em
`Config.MotorOcrAtivo`/`Config.MotorTtsAtivo` e **apaga o marcador** (aplica uma única vez). Isso roda
ANTES de `bootstrapMotorPadrao` ([wails_app/motores.go](../wails_app/motores.go)), que agora baixa o
motor de `Config.MotorOcrAtivo` quando ele nomeia uma entrada válida do catálogo — caindo de volta no
motor marcado `Padrao` (RapidOCR) só quando não há escolha (builds de dev, sem instalador).

> Builds de dev (`go run .` ou `wails dev`, sem passar pelo instalador) nunca encontram o marcador — o
> comportamento é exatamente o de sempre (RapidOCR baixado automaticamente no first-run).

## Alternativa: gerar o instalador manualmente (sem CI)

Útil para testar a tela de escolha de motores localmente antes de confiar no CI. Requer Windows +
[NSIS instalado](https://nsis.sourceforge.io/Download) (garanta que `makensis` esteja no PATH) + o
Wails CLI (`go install github.com/wailsapp/wails/v2/cmd/wails@v2.12.0`).

```powershell
cd wails_app
New-Item -ItemType Directory -Force -Path build/windows/installer
Copy-Item nsis-instalador/project.nsi build/windows/installer/project.nsi -Force
wails build -nsis -platform windows/amd64
```

O instalador sai em `wails_app/build/bin/HanziTracker-amd64-installer.exe`. A versão embutida é a que
estiver em `wails_app/wails.json` → `info.productVersion` no momento do build (o CI a sobrescreve
sozinho; localmente, edite-a à mão se quiser testar um número específico).

## Por que o template NSIS não fica em `build/`

`wails_app/build/` inteiro é gerado (e gitignorado) pelo Wails a cada build — inclusive
`build/windows/installer/project.nsi`, se ele não existir ainda, o Wails escreve ali o template
padrão. Por isso o template customizado (com a tela de escolha de motores) vive versionado em
[nsis-instalador/](../wails_app/nsis-instalador/), fora do caminho que o Wails regenera, e é **copiado**
para dentro de `build/windows/installer/` logo antes do `wails build -nsis` (tanto no workflow quanto
no passo manual acima) — assim o Wails encontra o arquivo já presente e usa o nosso em vez de escrever
o padrão por cima.
