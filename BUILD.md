# Build de Distribuição — Hanzi Tracker

Este documento descreve como gerar a versão distribuível do app, **com aceleração GPU por DirectML
embutida** (universal no Windows: Nvidia, AMD e Intel; sem CUDA Toolkit; sem download em runtime).

## Visão geral da arquitetura

- **Frontend**: React/TypeScript, compilado pelo Vite e embutido no app Wails (`//go:embed`).
- **App (UI + ponte)**: Go/Wails — captura de tela, dicionários, SQLite, hotkeys, e ponte HTTP com o OCR.
- **Backend de OCR**: microserviço Python (`python_backend/server.py`) usando RapidOCR (onnxruntime).
- **Overlay**: `python_backend/popup.py`.

No app distribuído, o backend Python é **congelado com PyInstaller** (não há Python/pip no usuário final).
Por isso, a aceleração GPU precisa estar **embutida no congelamento** — é o que esta receita garante.

## 1. Congelar os sidecars (forma recomendada: um comando)

Todo o processo abaixo está automatizado em [build_sidecars.ps1](build_sidecars.ps1). Na raiz do projeto:

```powershell
powershell -ExecutionPolicy Bypass -File build_sidecars.ps1
```

Ele cria um ambiente isolado (`build_env`), instala as dependências, **troca o onnxruntime pelo
onnxruntime-directml** (aceleração universal), roda o PyInstaller com os specs versionados
([ocr_server.spec](python_backend/ocr_server.spec) e [popup.spec](python_backend/popup.spec)) e gera
`python_backend/dist/ocr_server.zip` e `python_backend/dist/popup.zip`, imprimindo o **sha256** e o
tamanho de cada um (é o que você cola no manifesto de motores — ver
[docs/PUBLICAR-MOTORES.md](docs/PUBLICAR-MOTORES.md)).

### Por que os specs (e não flags soltas)

- `onnxruntime` (CPU) e `onnxruntime-directml` são **mutuamente exclusivos** (ambos instalam o pacote
  `onnxruntime/`); o `rapidocr-onnxruntime` puxa o CPU, então o script o **remove antes** de instalar o
  DirectML. É isso que embute o `DirectML.dll` e os `onnxruntime_providers_*.dll`.
- O `ocr_server.spec` usa `collect_all("onnxruntime")` + `collect_all("rapidocr_onnxruntime")` porque
  esses pacotes são **importados dinamicamente** em runtime (`OcrService._inicializarOcr`) — a análise
  estática do PyInstaller não os enxergaria. Também exclui `easyocr`/`torch`/`paddleocr` (viram sidecars
  próprios) para manter o pacote enxuto.
- O `popup.spec` usa **`console=True` (NÃO `--windowed`)**: o `popup.py` lê os comandos do Go por
  `sys.stdin`, que o PyInstaller **zera** no modo windowed — o overlay morreria na primeira leitura. O Go
  já sobe o processo com `HideWindow`, então a janela de console fica oculta de qualquer forma.

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

## 3. Empacotamento final

O **app Wails** é dono do backend de OCR e do overlay e **resolve automaticamente** qual executável
subir: `resolverMotorInicial`/`resolverComandoPopup` procuram, nesta ordem, o motor **baixado no AppData**
(`%APPDATA%\HanziTracker\motores\`, modelo padrão da Fase 5) e depois o sidecar congelado **ao lado do
app** (bundle). **Não há mais fallback para `python server.py`/`popup.py`** — todo motor é um executável
(baixado ou em bundle); se nenhum existe, o app baixa o padrão (bootstrap). Os `python_backend/*.py`
seguem sendo a *fonte* que o `build_sidecars.ps1` congela, mas não são executados pelo app. O orquestrador
`main.go` (raiz) não sobe mais o OCR — só reserva a porta e a pasta de dados e lança o app.

Há **duas formas** de distribuir os motores (as duas funcionam; dá para combinar):

- **Download sob demanda (padrão):** não envie motor nenhum junto do app. No primeiro start sem motor,
  o `bootstrapMotorPadrao` baixa o RapidOCR padrão + o overlay para o AppData e ativa. O instalador fica
  leve. Ver [docs/PUBLICAR-MOTORES.md](docs/PUBLICAR-MOTORES.md).
- **Bundle ao lado do app (offline):** deixe as pastas congeladas junto do executável:

```
HanziTracker/
  HanziTracker.exe            (app Wails — a janela "Hanzi Tracker"; sobe e gere os sidecars)
  ocr_server/ocr_server.exe   (saída do PyInstaller do server.py)
  popup/popup.exe             (saída do PyInstaller do popup.py)
```

> O orquestrador `main.go` (raiz) é só o *launcher de desenvolvimento* (`go run .`). No app distribuído,
> o Python é **congelado** (sem o sandbox do Python da Store), então o `HanziTracker.exe` pode ser
> aberto diretamente e sobe os sidecars por conta própria.

> Caminhos procurados: motor baixado → `%APPDATA%\HanziTracker\motores\<Motor>\ocr_server.exe`; bundle →
> `ocr_server/ocr_server.exe`, `dist/ocr_server/ocr_server.exe` ou `ocr_server.exe`; overlay baixado →
> `motores\_overlay\popup.exe`, bundle → `popup/popup.exe`, `dist/popup/popup.exe` ou `popup.exe`.
> Ausentes todos, o app faz o bootstrap (baixa o motor padrão + overlay e ativa).

## Comportamento da aceleração

- Com o DirectML embutido, **CPU e DirectML funcionam para todos**, sem download nem reinício.
- **CUDA** (Nvidia) não é embutido (pesado) nem instalável no congelado; ao selecioná-lo, o app
  **cai para CPU graciosamente** (ver `OcrService._inicializarOcr`). CUDA real só rodando do código-fonte
  com `onnxruntime-gpu` + CUDA Toolkit instalados.
