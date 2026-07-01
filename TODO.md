# TODO — Pendências do Projeto Hanzi Tracker

## Documentação
- [ ] `docs/interface.png` desatualizado — regerar a captura de tela do frontend React.

## Fase 5: Motores de OCR como Sidecars — pendências restantes

Base já pronta: catálogo de motores no Go ([motores_manifesto.go](wails_app/motores_manifesto.go)),
download/extração/troca/bootstrap ([motores.go](wails_app/motores.go)), UI "Gerenciar Motores" e a release
`motores-v1` publicada, mais o CI de publicação ([.github/workflows/publicar-motores.yml](.github/workflows/publicar-motores.yml)).

Decisão de aceleração: o modelo `.onnx` é o mesmo em CPU/DirectML/CUDA — muda só o *Execution Provider* do
onnxruntime, e os três pacotes (`onnxruntime`, `onnxruntime-directml`, `onnxruntime-gpu`) são mutuamente
exclusivos (um por binário). O motor padrão `ocr_server.zip` foi congelado com `onnxruntime-directml`,
então **já é CPU + DirectML num só binário** (DirectML cobre Nvidia/AMD/Intel no Windows, com fallback
automático p/ CPU e `DirectML.dll` embutida). Por isso **não há "sidecar DirectML" separado a fazer** —
já é o padrão. **CUDA foi descartado** por custo/benefício: exigiria `onnxruntime-gpu` (binário à parte) +
~2–3 GB de DLLs do CUDA/cuDNN embutidas, só p/ Nvidia e com ganho marginal em OCR. Reabrir só se houver
demanda medida.

Sobre "modelo embutido": o `MODELOS_EMBUTIDOS` (Python) **permanece** — "embutido" aqui é no nível de
PESO, não de motor. Os pesos mobile do RapidOCR vêm dentro do pacote `rapidocr_onnxruntime`, ou seja,
dentro do próprio motor (que já é sidecar baixável). Removê-lo esvaziaria o select "Modelo de OCR" de quem
só tem o motor sem baixar pesos extras. A preocupação original ("motor não embutido") já foi resolvida
pelo sistema de motores.

- [ ] **⚠ Republicar os motores** (NECESSÁRIO — bloqueia o OCR). O `ocr_server.zip` do `motores-v1` foi
      congelado com código PRÉ-rename: lê a env var antiga `CHINESESTUDY_OCR_PORT`, então sobe na porta
      fallback 8080 e ignora a `HANZITRACKER_OCR_PORT` que o app reserva → o healthcheck falha e o motor
      não engata. Solução: republicar via CI (nova tag), que congela com o código atual (env correta +
      separador "/" do `build_sidecars.ps1` corrigido) e atualizar `motores_manifesto.go` para a nova tag +
      hashes. Como não há mais fallback de código-fonte, o app depende 100% desse sidecar.
- [ ] **Motores adicionais como sidecars:** EasyOCR (CPU e CUDA), Tesseract e PaddleOCR. Cada um empacota o
      seu runtime próprio isolado no `.exe` (substitui de vez o pip em runtime, que não funciona no
      congelado). O aviso de tamanho grande / requisito de GPU já está pronto na UI.
- [ ] **Assinatura de código** dos binários dos sidecars (exige certificado; reduz falso positivo de
      antivírus). Hoje a integridade é garantida só por sha256.
- [ ] **Recarregar o catálogo de PESOS por motor** ao trocar de motor (`TrocarMotor`) — só faz sentido com
      mais de um motor publicado.
- [ ] **Subpasta de pesos por motor futuro** (ex.: `modelos\EasyOCR\`, `modelos\Tesseract\`), isolada das
      demais (o RapidOCR já usa `modelos\RapidOCR\`).
- [ ] **Manifesto de motores/modelos remoto (futuro):** buscar o catálogo de uma URL (com cache) para
      adicionar motores/modelos sem recompilar o app.
- [ ] **Instalador do app (futuro):** deixar o usuário escolher no install quais OCRs (motores) já vêm
      embutidos — exigindo **pelo menos um** e permitindo **vários** (bundle opcional, evita o download no
      primeiro start). O download sob demanda continua disponível depois. Hoje, sem instalador, o first-run
      já baixa+instala+ativa o RapidOCR padrão sozinho (bootstrapMotorPadrao).

## Armazenamento (ideias futuras)
- [ ] Permitir mover a pasta de modelos para outro disco.
- [ ] Limpeza automática agendada de logs antigos.

## Features Futuras (independentes)
- [ ] Textbox p/ API do Google Tradutor + checkbox: trocar os pop-ups do atalho por um pop-up por linha
      com a tradução.
- [ ] Textbox p/ API do Gemini + checkbox: trocar os pop-ups por um único pop-up com um resumo feito pela IA.
- [ ] Censurar áreas nos prints antes de enviá-los ao OCR: a área do próprio app (se estiver na tela alvo)
      e a dos pop-ups (always on top).
- [ ] Ler o pinyin em voz alta: alternar entre Kokoro-82M e ChatTTS, com opção de desligar a feature.
