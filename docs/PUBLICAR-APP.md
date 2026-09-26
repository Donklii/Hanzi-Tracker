# Como publicar o instalador do Hanzi Tracker

Este guia responde: **quando** o instalador Windows é gerado, **onde** ele fica disponível, como a
versão é decidida e como funcionam as telas de escolha de motor (OCR, voz/TTS e escuta/STT) dentro dele.

## Resposta curta

A publicação é impulsionada pelo workflow [publicar-app-windows.yml](../.github/workflows/publicar-app-windows.yml), que builda o app Wails (Go + React) e gera
o instalador via NSIS, publicando em **GitHub Releases** (mesmo lugar dos motores — ver
[PUBLICAR-MOTORES.md](PUBLICAR-MOTORES.md)):

| Gatilho | Versão do instalador | Release |
|---------|----------------------|---------|
| **Push na `main`** | `0.0.<número sequencial do workflow>` | `app-dev` — **prerelease rolante**: a mesma tag é atualizada a cada push, sempre com o instalador mais recente. O título traz o hash do commit para rastreio. |
| **Tag `app-vX.Y.Z`** (ex.: `app-v1.2.0`) | `X.Y.Z` (vem da própria tag) | Release **estável**, versionada, permanente. |
| **`workflow_dispatch`** manual | Segue a mesma regra acima, conforme a branch/tag escolhida ao disparar | — |

O instalador **não embute nenhum motor** — ele mostra três telas de escolha (RapidOCR / Tesseract /
EasyOCR para OCR; Nenhum / Kokoro-82M / ChatTTS para voz; Nenhum / Paraformer-ZH / Zipformer-ZH
Streaming para escuta) e só grava essa escolha para o app baixar sozinho no primeiro start,
reaproveitando o download-sob-demanda que já existe.

O mesmo esquema de gatilhos existe para **Linux**:
[publicar-app-linux.yml](../.github/workflows/publicar-app-linux.yml) builda o app no Ubuntu e anexa
um pacote `.deb` **na mesma release** (`app-dev` rolante ou `app-vX.Y.Z` estável) — ver a seção
[O pacote Linux (.deb)](#o-pacote-linux-deb).

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
3. O workflow [publicar-app-windows.yml](../.github/workflows/publicar-app-windows.yml) roda num runner
   `windows-latest`: instala o NSIS via choco, instala o Wails CLI, calcula a versão (tabela acima),
   grava-a em `wails_app/wails.json` (`info.productVersion`), copia o template NSIS customizado
   ([nsis-instalador/project.nsi](../wails_app/nsis-instalador/project.nsi)) para
   `wails_app/build/windows/installer/` (pasta gerada e gitignorada — por isso o template-fonte mora
   fora dela) e roda `wails build -nsis`. O instalador sai em
   `wails_app/build/bin/HanziTracker-amd64-installer.exe` e é anexado à release.

> **Primeira execução:** dispare manualmente pela aba Actions (`workflow_dispatch`) antes de confiar no
> gatilho automático — é a única forma de validar de ponta a ponta que o `makensis` compila o
> `project.nsi` customizado sem erro (isso não dá pra testar localmente sem instalar o NSIS).

## O pacote Linux (.deb)

O workflow [publicar-app-linux.yml](../.github/workflows/publicar-app-linux.yml) espelha os gatilhos
do Windows (push na `main` → `.deb` dev com nome fixo `hanzitracker_dev_amd64.deb` na prerelease
rolante `app-dev`; tag `app-v*` → `.deb` versionado na release estável). Os dois workflows anexam
**na mesma release**; a criação concorrente é tratada com o `gh` CLI (se um criar primeiro, o outro
só anexa o arquivo).

O empacotamento fica em [linux-instalador/](../wails_app/linux-instalador/):
`montar_deb.sh` monta o `.deb` com `dpkg-deb` (binário em `/usr/bin/hanzitracker`, atalho de menu
`hanzitracker.desktop`, ícone reaproveitando o mesmo `build/appicon.png` do Windows). Não há tela de
escolha de motores (o NSIS é Windows-only): no Linux o app baixa o RapidOCR sozinho no first-run, como
um build de dev do Windows. Os motores Linux têm workflows próprios (tags `motores-ocr-linux-v*` /
`motores-tts-linux-v*` — ver [PUBLICAR-MOTORES.md](PUBLICAR-MOTORES.md#motores-para-linux)); enquanto a
primeira release deles não sai, o app sobe sem OCR/TTS (o manifesto Linux fica com sha256 vazio e o
download é recusado com aviso).

Limitações conhecidas da build Linux (documentar na release quando divulgar):

- **Compatibilidade**: buildado no `ubuntu-latest` (24.04) com WebKitGTK 4.1 → requer Ubuntu 24.04+,
  Debian 13+, Mint 22+ (ou equivalentes com `libwebkit2gtk-4.1` e glibc ≥ 2.39).
- **X11 recomendado**: a captura de tela (`kbinani/screenshot`) e os atalhos globais
  (`golang.design/x/hotkey`) falam X11; em sessão Wayland a captura de apps nativos pode sair
  preta/vazia, e num ambiente sem X (Wayland puro sem XWayland) a lib de hotkey aborta o app no start.
- **Sem overlay**: os pop-ups desenhados por cima do jogo são janelas Win32
  ([overlay/overlay_outros.go](../wails_app/overlay/overlay_outros.go) é no-op fora do Windows) — a
  interface principal do app funciona normalmente.

Para gerar o `.deb` manualmente num Linux (ou WSL) com as dependências
(`libgtk-3-dev libwebkit2gtk-4.1-dev libx11-dev` + Wails CLI):

```bash
cd wails_app
wails build -platform linux/amd64 -tags webkit2_41
bash linux-instalador/montar_deb.sh 1.2.0 build/bin/HanziTracker build/bin/hanzitracker_1.2.0_amd64.deb
```

## As telas de escolha de motores (dentro do instalador)

Definidas em [nsis-instalador/project.nsi](../wails_app/nsis-instalador/project.nsi), como três páginas
custom do NSIS (`nsDialogs`) — uma por família de motor — inseridas entre a escolha de pasta e a
instalação dos arquivos:

- **OCR** (obrigatório, RapidOCR pré-selecionado): RapidOCR / Tesseract / EasyOCR.
- **Voz/TTS** (opcional, "Nenhum" pré-selecionado): Nenhum / Kokoro-82M / ChatTTS.
- **Escuta/STT** (opcional, "Nenhum" pré-selecionado): Nenhum / Paraformer-ZH / Zipformer-ZH Streaming.

Ao concluir a instalação, a escolha é gravada em texto simples (não precisa de plugin de JSON no
NSIS) em `%APPDATA%\HanziTracker\instalador_escolha.json`. **Nenhum motor é baixado nem embutido pelo
instalador** — ele só grava a escolha.

No primeiro start, `aplicarEscolhaDoInstalador` ([wails_app/instalador.go](../wails_app/instalador.go))
lê esse marcador, valida os nomes contra o catálogo real (`motoresocr`/`motorestts`/`motoresstt`), grava
em `Config.MotorOcrAtivo`/`Config.MotorTtsAtivo`/`Config.MotorSttAtivo` e **apaga o marcador** (aplica
uma única vez). Isso roda
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

## Atualização automática

O Hanzi Tracker conta com um sistema integrado de verificação e aplicação de atualizações automáticas, atendendo aos canais **Estável** (releases versionadas `app-vX.Y.Z`) e **Dev** (prerelease rolante `app-dev`). Para detalhes sobre a redação e inclusão de notas de versão embutidas, consulte o [Guia de Notas de Versão](NOTAS-DE-VERSAO.md).

### Manifestos por sistema operacional

A cada release publicada pela CI (tanto no push da branch `main` quanto em tags `app-v*`), um arquivo de manifesto é gerado e anexado aos assets da release:
- Windows: `atualizacao-windows.json`
- Linux: `atualizacao-linux.json`

Os manifestos ficam disponíveis na URL pública do GitHub Releases:
`https://github.com/Donklii/Hanzi-Tracker/releases/download/<tag>/<manifesto>`

Estrutura do manifesto:
```json
{
  "versao": "1.2.0",
  "commit": "40_caracteres_do_sha_completo",
  "dataBuild": "2026-09-10T12:00:00Z",
  "arquivo": "HanziTracker-amd64-installer.exe",
  "sha256": "hash_sha256_em_hex_minusculo",
  "tamanhoBytes": 12345678
}
```

### Regras de decisão por canal (Contrato C4)

A necessidade de atualização é avaliada a cada inicialização (ou sob demanda em Configurações → Info) através de regras estritas:

| Canal configurado | Canal do build | Critério para atualizar |
|---|---|---|
| `dev` | qualquer | `alvo.commit != meu.Commit` **e** `alvo.dataBuild` posterior a `meu.DataBuild` |
| `estavel` | `estavel` | `alvo.versao > meu.Versao` (comparação semântica X.Y.Z estritamente numérica) |
| `estavel` | `dev` | `alvo.commit != meu.Commit` — retorna para a versão estável mais recente mesmo em downgrade |

Casos em que o aplicativo **nunca** atualiza:
- Build local (`go run` ou `wails dev`): a variável `Versao` não é preenchida por ldflags, desativando a verificação automática.
- Flag `--pular-atualizacao` passada nos argumentos (`os.Args`) — vale só para a verificação automática da abertura; o "Verificar agora" de Configurações → Info continua funcionando nessa sessão.
- Instalações que não atendem aos requisitos de ambiente atualizável.
- Falhas de conexão, limites de taxa da API ou erros de parse (o app loga o aviso e segue o fluxo normal).

### Fluxo de instalação por plataforma

#### Windows
- **Requisito de instalação:** o executável corrente deve conter o desinstalador `uninstall.exe` em seu diretório base (garantia de instalação prévia pelo instalador NSIS).
- **Download e verificação:** o pacote do instalador é baixado em `%TEMP%/HanziTracker-atualizacao/`, validando o hash sha256 e reservando o dobro do espaço em disco.
- **Execução desacoplada:** como o executável em uso não pode ser sobrescrito e o instalador NSIS requer elevação de privilégios de administrador, o app dispara um processo PowerShell oculto via `Start-Process -Verb RunAs -Wait` com argumentos `/S /D=<pasta_instalacao>` e encerra o aplicativo principal.
- **Tratamento de cancelamento/erro:** se o usuário recusar a permissão de administrador no UAC ou a instalação falhar, o script auxiliar reabre automaticamente a versão anterior passando o argumento `--pular-atualizacao`, evitando loops de prompt na mesma sessão. Ao concluir com sucesso, o script reabre a nova versão atualizada.

#### Linux
- **Requisito de instalação:** o binário em execução deve estar localizado em `/usr/bin/hanzitracker` (instalado via pacote `.deb`) e o utilitário `pkexec` deve estar acessível no PATH do sistema.
- **Instalação síncrona:** a instalação é executada via `pkexec dpkg -i <deb>`. Caso ocorra erro ou o usuário cancele a senha no prompt de autenticação, o aplicativo permanece aberto em sua versão corrente.
- **Relançamento:** após a instalação bem-sucedida, o `.deb` temporário é removido e um processo em segundo plano desacoplado (`Setsid: true`) aguarda o encerramento do PID anterior para reexecutar `/usr/bin/hanzitracker`.

### Observações operacionais

- **Instalações legadas:** usuários com instalações anteriores a esta funcionalidade devem efetuar uma atualização manual baixando a versão correspondente para receber os novos binários e manifestos.
- **Inicialização limpa com `--pular-atualizacao`:** para abrir o aplicativo sem efetuar checagens de rede ou disparar downloads de atualização automática, basta iniciar o executável com a flag `--pular-atualizacao`.
