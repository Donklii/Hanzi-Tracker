# TODO — Pendências do Projeto Hanzi Tracker

## Documentação
- [ ] `docs/interface.png` desatualizado — regerar a captura de tela do frontend React.



## Fase 5: Motores de OCR como Sidecars — pendências restantes

Base já pronta: catálogo de motores no Go ([motores_manifesto.go](wails_app/motores_manifesto.go)),
download/extração/troca/bootstrap ([motores.go](wails_app/motores.go)), UI "Gerenciar Motores" e a release
`motores-v3` publicada (sidecar com a env `HANZITRACKER_OCR_PORT` correta), mais o CI de publicação
([.github/workflows/publicar-motores.yml](.github/workflows/publicar-motores.yml)). Também prontos:
o recarregamento do catálogo de PESOS ao trocar/ativar motor (frontend recarrega `modelos` junto com
`motores`) e a **subpasta de pesos por motor** (`modelos\<Motor>`): o Go injeta `HANZITRACKER_MOTOR`
no sidecar e baixa em `pastaModelosMotor()`; o Python lê a mesma env (`obterNomeMotor`, fallback
`RapidOCR`) — motores futuros ganham a pasta isolada automaticamente, sem código novo de caminho.

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

- [ ] **Publicar os motores adicionais (Tesseract + EasyOCR) — falta só a release.** O código está
      pronto: sidecars `tesseract_server` (tesseract.exe empacotado + tessdata_fast embutido; pesos
      `tessdata_best` baixáveis) e `easyocr_server` (PyTorch CPU; pesos CRAFT + zh_sim_g2 baixáveis,
      zipados no upstream — o Go extrai), servidor HTTP do contrato compartilhado
      (`ServidorOcrModule.py`), manifestos de pesos com sha256 pinados, specs, `build_sidecars.ps1` e o
      CI de publicação já congelam/anexam os 4 zips. PaddleOCR ficou de fora (preterido por desempenho
      de CPU); EasyOCR CUDA descartado como no RapidOCR (custo/benefício). Passos restantes:
      1. Empurrar a tag `motores-v4` (CI publica os 4 zips e imprime os sha256 no resumo do job).
      2. Atualizar em [motores_manifesto.go](wails_app/motores_manifesto.go) as URLs de `RapidOCR` e
         `PopupOverlayBaixavel` para `motores-v4` + novos hashes/tamanhos (o CI recongela tudo), e
         ADICIONAR ao `MotoresBaixaveis` (os `Nome` DEVEM casar com a subpasta de pesos/env
         `HANZITRACKER_MOTOR`):
         ```go
         "Tesseract": {
             Nome:   "Tesseract",
             Rotulo: "Tesseract (Leve)",
             Descricao: "Motor clássico do Google: leve e rápido na CPU, precisão menor em telas " +
                 "de jogo. Já vem com modelos rápidos de chinês e inglês (tessdata_fast).",
             Idiomas: []string{"zh", "en"}, Versao: "1.0.0", Variante: "CPU", Requisitos: "",
             Padrao: false, Executavel: "tesseract_server.exe",
             Artefato: ArtefatoBaixavel{Nome: "tesseract_server.zip",
                 Url: _baseReleaseMotores + "/motores-v4/tesseract_server.zip",
                 Sha256: "<sha256 do resumo do CI>", TamanhoBytes: 0 /* idem */},
         },
         "EasyOCR": {
             Nome:   "EasyOCR",
             Rotulo: "EasyOCR (Preciso, Pesado)",
             Descricao: "Motor de deep learning (PyTorch, CPU): boa precisão, pacote grande. Não " +
                 "embute pesos — baixe o modelo em Gerenciar Modelos antes do primeiro OCR.",
             Idiomas: []string{"zh", "en"}, Versao: "1.0.0", Variante: "CPU", Requisitos: "",
             Padrao: false, Executavel: "easyocr_server.exe",
             Artefato: ArtefatoBaixavel{Nome: "easyocr_server.zip",
                 Url: _baseReleaseMotores + "/motores-v4/easyocr_server.zip",
                 Sha256: "<sha256 do resumo do CI>", TamanhoBytes: 0 /* idem */},
         },
         ```
      3. Smoke test manual: baixar cada motor pela UI, trocar, OCR com peso embutido (Tesseract) e
         baixável (EasyOCR), e conferir as subpastas `modelos\Tesseract\` / `modelos\EasyOCR\`.
- [ ] **Limpar o caso legado `'EasyOCR (Download)'` no frontend** (matriz de compatibilidade
      modelo×hardware×API em `App.tsx` e `PainelConfiguracoes.tsx`): esse nome não existe em nenhum
      catálogo desde que o EasyOCR virou motor próprio (CPU-only, sem CUDA) — os ramos são código
      morto. Revisar a matriz à luz dos motores reais ao publicar o `motores-v4`.
- [ ] **Assinatura de código** dos binários dos sidecars (exige certificado; reduz falso positivo de
      antivírus). Hoje a integridade é garantida só por sha256.
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
- [ ] Ler o pinyin em voz alta: alternar entre Kokoro-82M e ChatTTS, com opção de desligar a feature.

