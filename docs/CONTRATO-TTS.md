# Contrato da API de TTS (v1)

Este documento define o **contrato HTTP** que qualquer motor de voz (leitura do pinyin em voz alta)
do Hanzi Tracker deve cumprir. A implementação é ÚNICA e compartilhada:
`python_backend/principal/ServidorTtsModule.py` sobe o servidor do contrato, e cada motor é um
*entry* fino que injeta nele o seu serviço — `kokoro_server.py` (**Kokoro-82M**) e
`chattts_server.py` (**ChatTTS**). Cada um vira um **sidecar** (executável autônomo) que fala
exatamente este mesmo contrato, de forma que o app (Go) conversa com qualquer motor sem saber o que
há por trás. Espelha o contrato de OCR (ver `CONTRATO-OCR.md`).

> **Versão do contrato:** `1`. O número está em `VERSAO_CONTRATO_TTS` (Python,
> `ServidorTtsModule.py`) e em `VersaoContratoTts` (Go, `tts.go`) — os dois devem casar.
> Incremente-o ao fazer qualquer mudança **quebrável** (novo/removido endpoint ou formato de
> requisição/resposta). O app recusa um sidecar cujo `versaoContrato` seja **maior** do que o que
> ele entende.

## Transporte

- Protocolo: **HTTP/1.1** em `localhost`.
- Porta: **dinâmica**, própria do TTS (`HANZITRACKER_TTS_PORT`) — o motor de voz **coexiste** com o
  de OCR, então não compartilha a porta dele. O app (Go, `motor_tts.go`) resolve uma porta livre e a
  publica no ambiente antes de subir o sidecar. Fallback `8090` só quando a variável não está
  definida (execução avulsa em dev).
- Ciclo de vida: **preguiçoso**. O sidecar só sobe na primeira leitura em voz alta da sessão
  (`garantirMotorTts`, em `tts.go`) — nunca no startup; a feature é opcional e desligada por padrão.
- Pesos: **o próprio sidecar os baixa** do Hugging Face na primeira síntese (não há
  `ModelosManifesto.py` para TTS). O cache do HF é redirecionado pelo *entry* para
  `<dados>/modelos/<Motor>/hf` (envs `HANZITRACKER_DATA_DIR` + `HANZITRACKER_MOTOR`), então os pesos
  moram no AppData do app — mensuráveis/limpáveis pela aba Armazenamento.
- CORS: todo endpoint responde `Access-Control-Allow-Origin: *` e trata `OPTIONS` (preflight).

## Endpoints

### `GET /api/health`

Sinaliza que o motor subiu e informa a versão do contrato que ele fala. O app faz *polling* deste
endpoint ao subir/trocar o motor. **Não** carrega o modelo (a carga é preguiçosa, na primeira
síntese) — responde rápido mesmo quando os pesos ainda nem foram baixados.

Resposta `200 OK`:

```json
{
  "status": "ok",
  "servico": "tts",
  "motor": "Kokoro-82M",
  "versaoContrato": 1
}
```

- `status`: `"ok"` quando pronto para receber sínteses. Qualquer outro valor = ainda não pronto.
- `motor`: o nome de catálogo do motor (ecoa `HANZITRACKER_MOTOR`).
- `versaoContrato`: inteiro; o app recusa se for **maior** que `VersaoContratoTts`.

### `POST /api/tts`

Sintetiza fala a partir de um texto. O app envia o **HANZI** do card (não a string de pinyin
romanizada — é o hanzi que garante pronúncia nativa correta; o pinyin exibido na UI é só leitura
visual).

Requisição (`Content-Type: application/json`):

```json
{ "texto": "你好" }
```

Resposta `200 OK`: os **bytes** de um arquivo **WAV (PCM 16-bit, mono, 24 kHz)** com
`Content-Type: audio/wav`. O Go os devolve ao frontend em base64 (`FalarPinyin`), que toca via
`<audio>` — a reprodução acontece na webview porque o popup nativo Win32 não tem áudio.

Resposta `500` (falha): JSON `{ "error": "<mensagem>" }`.

Notas de latência:

- A **primeira** síntese de cada motor carrega o modelo (~10-30s com torch em CPU) e, na
  primeiríssima vez, baixa os pesos do Hugging Face (~330 MB no Kokoro-82M, ~1 GB no ChatTTS). O
  cliente Go usa timeout longo (15 min) por causa disso e anuncia o estado ao frontend via o evento
  `tts_estado`.
- O app cacheia o WAV devolvido por `(hanzi, motor)` em SQLite (`tts_audio_cache`,
  `progresso/tts_cache.go`): repetições nem chegam ao sidecar.
- O modelo é **descarregado da RAM** após `SEGUNDOS_OCIOSO_DESCARREGAR_TTS` (300s) sem uso
  (`ServicoTtsBase`), recarregando sob demanda na próxima síntese.
