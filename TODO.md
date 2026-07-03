# TODO — Pendências do Projeto Hanzi Tracker

## Documentação
- [ ] `docs/interface.png` desatualizado — regerar a captura de tela do frontend React.


## Leitura do pinyin em voz alta (TTS) — pendências restantes

Base já pronta (feature completa em código): UI (toggle mestre + 2 sub-toggles + select + "Gerenciar
Motores de Voz" na aba Geral), campos em [config.go](wails_app/config/config.go), sidecars Python
**Kokoro-82M** ([motores/kokoro/](python_backend/motores/kokoro/)) e **ChatTTS**
([motores/chattts/](python_backend/motores/chattts/)) sobre a base compartilhada
[ServicoTtsBase.py](python_backend/tts/ServicoTtsBase.py) + servidor do contrato
[ServidorTtsModule.py](python_backend/principal/ServidorTtsModule.py) (ver
[docs/CONTRATO-TTS.md](docs/CONTRATO-TTS.md)), gerenciador de processo próprio
([motor_tts.go](wails_app/motor_tts.go), porta `HANZITRACKER_TTS_PORT`, coexiste com o OCR, subida
preguiçosa na primeira leitura), catálogo/download ([motores_tts.go](wails_app/motores_tts.go) +
[motores_tts_manifesto.go](wails_app/motores_tts_manifesto.go), pasta `motores_tts\`), `FalarPinyin`
real devolvendo WAV base64 ([tts.go](wails_app/tts.go)), cache de áudio em SQLite
([progresso/tts_cache.go](wails_app/progresso/tts_cache.go), tabela `tts_audio_cache`), categorias
na aba Armazenamento, specs PyInstaller, `builds/build_sidecars_tts.ps1` e CI
([publicar-motores-tts.yml](.github/workflows/publicar-motores-tts.yml)) atualizados. Os PESOS são
baixados pelo próprio sidecar do Hugging Face na primeira síntese (cache HF redirecionado para
`modelos\<Motor>\hf`) — não há manifesto de pesos para TTS. O texto sintetizado é o **HANZI** do
card (pronúncia nativa); o pinyin é só a leitura visual.

- [ ] **Publicar os motores de voz — falta só a release.** Empurrar a tag `motores-tts-v1` (o CI
      congela `kokoro_server.zip` e `chattts_server.zip` no workflow dedicado
      [publicar-motores-tts.yml](.github/workflows/publicar-motores-tts.yml), e imprime os sha256
      no resumo do job). Depois, colar em
      [motores_tts_manifesto.go](wails_app/motores_tts_manifesto.go) o `Sha256` e o `TamanhoBytes`
      dos dois artefatos (hoje estão vazios — estado "pendente de publicação": a UI mostra o botão
      como "Indisponível" e o download é recusado; `TestCatalogoDeMotoresTtsIntegro` passa a exigir
      os campos quando o sha256 é preenchido).
- [ ] **Smoke test manual com modelo real** (nunca rodou — só o contrato foi testado de ponta a
      ponta com serviço falso): baixar o Kokoro-82M pela UI (ou build local via
      `builds/build_sidecars_tts.ps1`, que o app acha como bundle), ligar o toggle mestre + "ao abrir o pop-up"
      e conferir: primeira leitura baixa os pesos (~330 MB, status na barra), áudio toca, repetição
      sai instantânea (cache), categorias "Motores de Voz"/"Cache de Áudio (Voz)" aparecem na aba
      Armazenamento. Repetir com o ChatTTS (~1 GB). Atenção às APIs dos pacotes `kokoro`/`chattts`
      no primeiro build congelado — as versões dos requirements são `>=`; se o upstream quebrar a
      API, pinar a versão que funcionou.
- [ ] **(Futuro) Escolha de voz** do Kokoro (hoje fixa em `zf_xiaobei`; ver `VOZ_PADRAO` em
      [KokoroTtsService.py](python_backend/motores/kokoro/KokoroTtsService.py)) e velocidade de fala.


## Fase 5: Motores de OCR como Sidecars — pendências restantes

Base já pronta: catálogo de motores no Go ([motores_manifesto.go](wails_app/motores_manifesto.go)),
download/extração/troca/bootstrap ([motores.go](wails_app/motores.go)), UI "Gerenciar Motores" e a release
`motores-v4` publicada (RapidOCR + Tesseract + EasyOCR com hashes reais), mais o CI de publicação
([.github/workflows/publicar-motores-ocr.yml](.github/workflows/publicar-motores-ocr.yml)). Também prontos:
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

- [ ] **Limpar o caso legado `'EasyOCR (Download)'` no frontend** (matriz de compatibilidade
      modelo×hardware×API em `App.tsx` e `PainelConfiguracoes.tsx`): esse nome não existe em nenhum
      catálogo desde que o EasyOCR virou motor próprio (CPU-only, sem CUDA) — os ramos são código
      morto. Revisar a matriz à luz dos motores reais.
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
