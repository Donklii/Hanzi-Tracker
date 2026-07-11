# Contrato da API de STT (v1)

Este documento define o **contrato HTTP** que qualquer motor de escuta (reconhecimento de fala da
revisão de pronúncia) do Hanzi Tracker deve cumprir. A implementação é ÚNICA e compartilhada:
`python_backend/principal/ServidorSttModule.py` sobe o servidor do contrato, e cada motor é um
*entry* fino que injeta nele o seu serviço — `paraformer_server.py` (**Paraformer-ZH**, offline) e
`zipformer_streaming_server.py` (**Zipformer-ZH-Streaming**, streaming de verdade). Cada um vira um
**sidecar** (executável autônomo) que fala exatamente este mesmo contrato, de forma que o app (Go)
conversa com qualquer motor sem saber o que há por trás. Espelha os contratos de OCR e de TTS (ver
`CONTRATO-OCR.md` / `CONTRATO-TTS.md`).

> **Versão do contrato:** `1`. O número está em `VERSAO_CONTRATO_STT` (Python,
> `ServidorSttModule.py`) e em `VersaoContrato` (Go, `motoresstt/motoresstt.go`) — os dois devem
> casar. Incremente-o ao fazer qualquer mudança **quebrável** (novo/removido endpoint ou formato de
> requisição/resposta). O app recusa um sidecar cujo `versaoContrato` seja **maior** do que o que
> ele entende. O `/api/stt/parcial` foi ADITIVO e não exigiu bump: o app trata `404` como "motor
> antigo, sem parciais" e degrada em silêncio.

## Por que a gravação acontece NO SIDECAR

A webview do Wails no Linux (WebKitGTK) não implementa a Web Speech API (`SpeechRecognition`) e o
Wails não habilita `enable-media-stream` — nem `getUserMedia` existe. Então o sidecar é dono das
DUAS pontas: **captura o microfone** (sounddevice/PortAudio) e **transcreve** (sherpa-onnx). O
frontend só comanda o push-to-talk: `iniciar` ao pressionar o botão, `parar` ao soltar (devolve o
texto), `cancelar` para descartar. Quando o usuário seleciona a Web Speech em Configurações →
Motores → Motor de Escuta (e a webview a oferece), o frontend a usa direto e este sidecar nem sobe
(ver `useSTT.ts`).

## Transporte

- Protocolo: **HTTP/1.1** em `localhost`.
- Porta: **dinâmica**, própria do STT (`HANZITRACKER_STT_PORT`) — o motor de escuta **coexiste**
  com os de OCR e TTS, então não compartilha as portas deles. O app (Go, `stt.go`) resolve uma
  porta livre e a publica no ambiente antes de subir o sidecar. Fallback `8091` só quando a
  variável não está definida (execução avulsa em dev).
- Ciclo de vida: **preguiçoso**. O sidecar só sobe quando a revisão de pronúncia precisa dele
  (`garantirMotorStt`, em `stt.go`) — nunca no startup. Ao entrar numa questão de pronúncia, o
  frontend chama `DespertarMotorStt` (pré-aquecimento em segundo plano).
- Pesos: **o próprio sidecar os baixa** do Hugging Face na primeira transcrição ou no
  `/api/stt/preparar` (não há `ModelosManifesto.py` para STT). O cache do HF é redirecionado pelo
  *entry* para `<dados>/motores_stt/<Motor>/modelos/hf` (envs `HANZITRACKER_DATA_DIR` +
  `HANZITRACKER_MOTOR`), então os pesos moram DENTRO da pasta do próprio motor no AppData (mesma
  estrutura do TTS) — mensuráveis/limpáveis pela aba Armazenamento junto com o executável.
- CORS: todo endpoint responde `Access-Control-Allow-Origin: *` e trata `OPTIONS` (preflight).

## Endpoints

### `GET /api/health`

Sinaliza que o motor subiu e informa a versão do contrato que ele fala. O app faz *polling* deste
endpoint ao subir/trocar o motor. **Não** carrega o modelo (a carga é preguiçosa) — responde rápido
mesmo quando os pesos ainda nem foram baixados.

Resposta `200 OK`:

```json
{
  "status": "ok",
  "servico": "stt",
  "motor": "Paraformer-ZH",
  "versaoContrato": 1
}
```

- `status`: `"ok"` quando pronto para receber comandos. Qualquer outro valor = ainda não pronto.
- `motor`: o nome de catálogo do motor (ecoa `HANZITRACKER_MOTOR`).
- `versaoContrato`: inteiro; o app recusa se for **maior** que `motoresstt.VersaoContrato`.

### `POST /api/stt/preparar`

Pré-aquecimento: carrega o modelo — na primeiríssima vez, **baixa os pesos** do Hugging Face
(~240 MB no Paraformer-ZH) — sem transcrever nada. O app chama em segundo plano ao entrar na
revisão de pronúncia (`DespertarMotorStt`), para a primeira transcrição real sair sem essa espera.
Idempotente: com o modelo já carregado, devolve na hora.

Resposta `200 OK`: `{ "ok": true }` · Falha: `500` com `{ "error": "<mensagem>" }`.

### `POST /api/stt/iniciar`

Push-to-talk: começa a capturar o **microfone padrão** do sistema (o usuário pressionou o botão).
Uma gravação já em andamento é **descartada** e a captura recomeça do zero — o contrato é tolerante
a um `parar` que o frontend perdeu. A captura tem teto de duração
(`SEGUNDOS_MAXIMOS_GRAVACAO_STT`, 30s): blocos além dele são ignorados.

Resposta `200 OK`: `{ "ok": true }` · Falha (ex.: sem microfone): `500` com `{ "error": "<mensagem>" }`.

### `POST /api/stt/parcial`

Transcrição **parcial** do áudio acumulado até agora, **sem parar a captura** — é o que dá os
parciais em tempo real ao caminho por motor (o app faz *polling* deste endpoint a cada ~900 ms
durante a escuta, `lacoParcialStt` em `stt.go`, e emite o texto ao frontend via o evento
`stt_parcial`). *Best-effort* de propósito: devolve `texto` **vazio** sempre que um parcial não
puder sair agora (nada gravado, modelo ocupado com outro decode ou ainda não carregado — o parcial
NUNCA dispara a carga/download do modelo) e o tick seguinte tenta de novo. O `/parar` continua
sendo a transcrição de verdade.

Como o parcial é implementado fica a cargo do serviço de cada motor: o Paraformer-ZH (offline)
re-decodifica o áudio acumulado inteiro a cada consulta; o Zipformer-ZH-Streaming mantém um fluxo
de decodificação com estado e alimenta só o trecho novo (custo constante).

Resposta `200 OK`:

```json
{ "texto": "你好" }
```

Falha: `500` com `{ "error": "<mensagem>" }`. Num sidecar **antigo** (publicado antes deste
endpoint) a resposta é `404` — o app interrompe o polling e segue sem parciais.

### `POST /api/stt/parar`

Para a captura e **transcreve** o que foi gravado (o usuário soltou o botão). A primeira chamada
pode demorar: carga do modelo e, se `/preparar` não rodou antes, o download dos pesos — o cliente
Go usa timeout longo (15 min) por causa disso e anuncia o estado ao frontend via o evento
`stt_estado`.

Resposta `200 OK`:

```json
{ "texto": "你好" }
```

- `texto`: o texto reconhecido (hanzi), já com espaços das pontas removidos. Pode ser `""` quando
  nada foi reconhecido.

Falha (nada gravado, modelo indisponível…): `500` com `{ "error": "<mensagem>" }`.

### `POST /api/stt/cancelar`

Descarta a gravação em andamento **sem transcrever** (soltar o botão fora da área, troca de
questão, desmontagem da tela). Idempotente: sem gravação em andamento, é um no-op.

Resposta `200 OK`: `{ "ok": true }`.

## Notas

- O modelo é **descarregado da RAM** após `SEGUNDOS_OCIOSO_DESCARREGAR_STT` (300s) sem uso
  (`ServicoSttBase`), recarregando sob demanda na próxima transcrição.
- A captura tenta 16 kHz mono (taxa nativa dos dois modelos); se o dispositivo não suportar, cai
  na taxa padrão dele e o sherpa-onnx resampleia na transcrição.
- O servidor é **multi-thread** (`ThreadingHTTPServer`): o polling de `/parcial` chega DURANTE um
  `/parar` ou `/preparar` em andamento. A concorrência real é limitada pelos locks do
  `ServicoSttBase` (modelo + gravação); o parcial adquire o lock do modelo em modo não-bloqueante
  e devolve vazio quando ele está ocupado.
- Os parciais chegam ao frontend pelo evento `stt_parcial`; o `stt_estado` segue preenchendo os
  vãos sem transcrição ("Transcrevendo fala…", carga do modelo etc.).
