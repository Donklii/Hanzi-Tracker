# Build de Distribuição — Hanzi Tracker

Este documento descreve como gerar a versão distribuível do app, **com aceleração GPU por DirectML
embutida** (universal no Windows: Nvidia, AMD e Intel; sem CUDA Toolkit; sem download em runtime).

## Visão geral da arquitetura

- **Frontend**: React/TypeScript, compilado pelo Vite e embutido no app Wails (`//go:embed`).
- **App (UI + ponte)**: Go/Wails — captura de tela, dicionários, SQLite, hotkeys, e ponte HTTP com o OCR.
- **Backend de OCR**: um microserviço Python por motor, cada um na sua pasta em `python_backend/motores/`
  — `motores/rapidocr/server.py` (padrão, onnxruntime), `motores/tesseract/tesseract_server.py` e
  `motores/easyocr/easyocr_server.py`. Compartilham o servidor HTTP do contrato
  (`python_backend/principal/ServidorOcrModule.py`) e a base de serviço (`python_backend/ocr/`).

No app distribuído, o backend Python é **congelado com PyInstaller** (não há Python/pip no usuário final).
Por isso, a aceleração GPU precisa estar **embutida no congelamento** — é o que esta receita garante.

## 1. Congelar os sidecars (forma recomendada: um comando)

O processo está automatizado em dois scripts na pasta [builds/](builds/), separados por tipo de motor:
[builds/build_sidecars_ocr_windows.ps1](builds/build_sidecars_ocr_windows.ps1) (OCR) e
[builds/build_sidecars_tts_windows.ps1](builds/build_sidecars_tts_windows.ps1) (voz/TTS). Na raiz do projeto:

```powershell
powershell -ExecutionPolicy Bypass -File builds/build_sidecars_ocr_windows.ps1
# motores de voz (Kokoro-82M + ChatTTS):
powershell -ExecutionPolicy Bypass -File builds/build_sidecars_tts_windows.ps1
```

O script de OCR congela os artefatos, cada motor num venv **próprio** (`build_env`, `build_env_tesseract`,
`build_env_easyocr` — o onnxruntime-directml do RapidOCR e o torch do EasyOCR não podem conviver no
mesmo ambiente; o de voz usa `build_env_kokoro`/`build_env_chattts`), com os specs versionados dentro da
pasta de cada motor
([motores/rapidocr/ocr_server.spec](python_backend/motores/rapidocr/ocr_server.spec),
[motores/tesseract/tesseract_server.spec](python_backend/motores/tesseract/tesseract_server.spec) e
[motores/easyocr/easyocr_server.spec](python_backend/motores/easyocr/easyocr_server.spec)). No build do
RapidOCR ele **troca o onnxruntime pelo onnxruntime-directml** (aceleração universal); no do Tesseract,
copia a instalação do Tesseract (`choco install tesseract`, ou a pasta apontada por `TESSERACT_DIR`) para
dentro do pacote e garante o `chi_sim.traineddata` (tessdata_fast 4.1.0, hash conferido) — sem a
instalação, esse sidecar é pulado com aviso. Saída:
`python_backend/dist/{ocr_server,tesseract_server,easyocr_server}.zip`, imprimindo o **sha256** e o
tamanho de cada um (é o que você cola no manifesto de motores — ver
[docs/PUBLICAR-MOTORES.md](docs/PUBLICAR-MOTORES.md)).

### Por que os specs (e não flags soltas)

- `onnxruntime` (CPU) e `onnxruntime-directml` são **mutuamente exclusivos** (ambos instalam o pacote
  `onnxruntime/`); o `rapidocr-onnxruntime` puxa o CPU, então o script o **remove antes** de instalar o
  DirectML. É isso que embute o `DirectML.dll` e os `onnxruntime_providers_*.dll`.
- O `ocr_server.spec` usa `collect_all("onnxruntime")` + `collect_all("rapidocr_onnxruntime")` porque
  esses pacotes são **importados dinamicamente** em runtime (`OcrService._inicializarOcr`) — a análise
  estática do PyInstaller não os enxergaria. Também exclui `easyocr`/`torch`/`paddleocr` (viram sidecars
  próprios) para manter o pacote enxuto. Cada spec mora ao lado do seu entry (`motores/<motor>/`) e
  resolve a raiz `python_backend` via `SPECPATH` para os imports absolutos (`ocr.*`, `principal.*`,
  `motores.<motor>.*`) funcionarem no congelamento.

Para conferir manualmente que o DirectML entrou no ambiente de build (o script já imprime isto):

```powershell
build_env\Scripts\python.exe -c "import onnxruntime as ort; print(ort.get_available_providers())"
# Esperado conter: 'DmlExecutionProvider'
```

## 2. Frontend + App Wails

```powershell
cd wails_app
wails build   # instala/compila o frontend e gera HanziTracker.exe com os assets embutidos
```

(Sem o Wails CLI: `cd wails_app/frontend && npm install && npm run build`, depois
`go build -tags "desktop,production" -o build/bin/HanziTracker.exe .`)

> O nome do executável (`HanziTracker.exe`) vem de `outputfilename` em `wails_app/wails.json`; a janela
> tem o título "Hanzi Tracker" (definido em `wails_app/main.go`).

> **Instalador Windows (NSIS):** gerado automaticamente por CI a cada push na `main` (build de teste) ou
> tag `app-vN.N.N` (release estável), com uma tela de escolha de motor de OCR/voz — ver
> [docs/PUBLICAR-APP.md](docs/PUBLICAR-APP.md).

## 3. Empacotamento final

O **app Wails** é dono do backend de OCR e **resolve automaticamente** qual executável
subir: `resolverMotorInicial` procura, nesta ordem, o motor **baixado no AppData**
(`%APPDATA%\HanziTracker\motores_ocr\`, modelo padrão da Fase 5) e depois o sidecar congelado **ao lado do
app** (bundle). **Não há mais fallback para `python server.py`** — todo motor é um executável
(baixado ou em bundle); se nenhum existe, o app baixa o padrão (bootstrap). Os `python_backend/*.py`
seguem sendo a *fonte* que os scripts em `builds/` congelam, mas não são executados pelo app. O orquestrador
`main.go` (raiz) não sobe mais o OCR — só reserva a porta e a pasta de dados e lança o app.

Há **duas formas** de distribuir os motores (as duas funcionam; dá para combinar):

- **Download sob demanda (padrão):** não envie motor nenhum junto do app. No primeiro start sem motor,
  o `bootstrapMotorPadrao` baixa o RapidOCR padrão para o AppData e ativa. O instalador fica
  leve. Ver [docs/PUBLICAR-MOTORES.md](docs/PUBLICAR-MOTORES.md).
- **Bundle ao lado do app (offline):** deixe as pastas congeladas junto do executável:

```
HanziTracker/
  HanziTracker.exe            (app Wails — a janela "Hanzi Tracker"; sobe e gere os sidecars)
  ocr_server/ocr_server.exe   (saída do PyInstaller do server.py)
```

> O orquestrador `main.go` (raiz) é só o *launcher de desenvolvimento* (`go run .`). No app distribuído,
> o Python é **congelado** (sem o sandbox do Python da Store), então o `HanziTracker.exe` pode ser
> aberto diretamente e sobe os sidecars por conta própria.

> Caminhos procurados: motor baixado → `%APPDATA%\HanziTracker\motores_ocr\<Motor>\ocr_server.exe`; bundle →
> `ocr_server/ocr_server.exe`, `dist/ocr_server/ocr_server.exe` ou `ocr_server.exe`; overlay baixado →
> `motores_ocr\_overlay\popup.exe`, bundle → `popup/popup.exe`, `dist/popup/popup.exe` ou `popup.exe`.
> Ausentes todos, o app faz o bootstrap (baixa o motor padrão + overlay e ativa).

## Comportamento da aceleração

- Com o DirectML embutido, **CPU e DirectML funcionam para todos**, sem download nem reinício.
- **CUDA** (Nvidia) não é embutido (pesado) nem instalável no congelado; ao selecioná-lo, o app
  **cai para CPU graciosamente** (ver `OcrService._inicializarOcr`). CUDA real só rodando do código-fonte
  com `onnxruntime-gpu` + CUDA Toolkit instalados.
