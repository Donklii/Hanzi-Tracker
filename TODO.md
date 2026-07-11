# TODO

## Revisão de pronúncia — STT (implementado nesta sessão; falta publicar)

- [x] **Publicar o motor de escuta Linux**: feito — release `motores-stt-linux-v1` publicada
      (sha256 preenchido em `artefatos_stt_linux.json`). Superada pela pendência de republicação
      abaixo (parciais + Zipformer).
- [x] **Publicar o motor de escuta Windows**: feito — release `motores-stt-windows-v1` publicada
      (sha256 preenchido em `artefatos_stt.json`). Superada pela pendência de republicação abaixo.
- [ ] Testar a revisão de pronúncia ponta a ponta com microfone real (gravação via sounddevice no
      sidecar + transcrição Paraformer): `bash builds/build_sidecars_stt_linux.sh` gera o bundle
      local em `python_backend/dist/paraformer_server/`, que o app já reconhece como instalado.

## Pendências

- [ ] **Publicar os motores de STT com os parciais + o novo Zipformer**: os artefatos atuais das
      releases `motores-stt-*-v1` foram congelados ANTES do endpoint `/api/stt/parcial` e não
      incluem o `zipformer_streaming_server`. Empurrar novas tags (`motores-stt-windows-v2` e
      `motores-stt-linux-v2`) para o Paraformer instalado ganhar parciais em tempo real e o
      Zipformer-ZH-Streaming sair de "Indisponível" (sha256 vazio em
      `wails_app/motoresstt/artefatos_stt*.json`). Até lá o app degrada em silêncio (404 no
      /parcial interrompe o polling).
- [ ] Nomes dos arquivos de pesos do Zipformer streaming em
      `python_backend/motores/zipformer_streaming/ZipformerStreamingSttService.py:15-18` assumidos
      do padrão do repositório HF `csukuangfj/sherpa-onnx-streaming-zipformer-zh-14M-2023-02-23`
      (`encoder-epoch-99-avg-1.int8.onnx`, `decoder-epoch-99-avg-1.onnx`,
      `joiner-epoch-99-avg-1.int8.onnx`, `tokens.txt`) sem conferir o repositório online — validar
      na primeira execução real (um nome errado falha o download no primeiro uso).
- [ ] Testes Python novos (`python_backend/tests/test_stt_base.py` — parciais — e
      `tests/test_stt_streaming.py`) escritos mas NÃO executados localmente: a máquina não tem
      Python instalado (só os stubs da Microsoft Store). Rodam no CI (`testes.yml`) no próximo
      push — conferir o resultado.
- [ ] Teste `TestObterQuestoesRevisaoTodosOsModos` em `wails_app/revisao_test.go:58` falha de forma intermitente ("modo geral: hanzi X repetido na sessão", com hanzi diferente a cada execução) — indica bug real de repetição na seleção aleatória de questões em `wails_app/revisao.go` (arquivo tem mudanças extensas ainda não commitadas). Não investigado nesta sessão (fora do escopo da renomeação da API do `overlay`); precisa de correção na lógica de sorteio/exclusão de repetidos.

## Pendências anteriores

- [ ] **Assinatura de código** dos binários dos sidecars (exige certificado; reduz falso positivo de
      antivírus). Hoje a integridade é garantida só por sha256.

### Armazenamento (ideias futuras)
- [ ] Permitir mover a pasta de modelos para outro disco.
- [ ] Limpeza automática agendada de logs antigos.
