# Contrato da API de OCR (v1)

Este documento define o **contrato HTTP** que qualquer motor de OCR do Hanzi Tracker deve cumprir.
A implementação é ÚNICA e compartilhada: `python_backend/principal/ServidorOcrModule.py` sobe o
servidor do contrato, e cada motor é um *entry* fino que injeta nele o seu serviço e o seu catálogo de
pesos — `server.py` (**RapidOCR**, padrão), `tesseract_server.py` (**Tesseract**) e
`easyocr_server.py` (**EasyOCR**). Cada um vira um **sidecar** (executável autônomo) que fala
exatamente este mesmo contrato, de forma que o app (Go) conversa com qualquer motor sem saber o que
há por trás.

> **Versão do contrato:** `1`. O número está em `VERSAO_CONTRATO_OCR` (Python, `ServidorOcrModule.py`)
> e em `VersaoContratoOcr` (Go, `app.go`) — os dois devem casar. Incremente-o ao fazer qualquer mudança
> **quebrável** (novo/removido endpoint, header, ou formato de resposta). O app recusa um sidecar cujo
> `versaoContrato` seja **maior** do que o que ele entende (motor mais novo que o app).

## Transporte

- Protocolo: **HTTP/1.1** em `localhost`.
- Porta: **dinâmica**, reservada pelo orquestrador (`main.go`) e publicada em `HANZITRACKER_OCR_PORT`.
  O **app** é dono do processo do motor e o sobe nessa porta (ou pede uma livre, se rodar avulso sem o
  orquestrador). Fallback `8080` só quando a variável não está definida.
- Diretório de dados (leitura de pesos): repassado por `HANZITRACKER_DATA_DIR`; os pesos do motor
  ficam em `<dados>/modelos/<Motor>/` (ex.: `modelos/RapidOCR/`). `<Motor>` é o **nome de catálogo**
  do motor, injetado pelo orquestrador em `HANZITRACKER_MOTOR` ao subir o sidecar — o mesmo nome que o
  Go usa para **baixar** os pesos, então os dois lados concordam por construção. Fallback `RapidOCR`
  quando a variável está ausente (execução avulsa em dev).
- CORS: todo endpoint responde `Access-Control-Allow-Origin: *` e trata `OPTIONS` (preflight).

## Endpoints

### `GET /api/health`

Sinaliza que o motor subiu e informa a versão do contrato que ele fala. O app faz *polling* deste
endpoint na inicialização (e ao trocar de motor) antes de marcar o motor como **pronto**.

Resposta `200 OK`:

```json
{
  "status": "ok",
  "servico": "ocr",
  "motor": "RapidOCR",
  "versaoContrato": 1
}
```

- `status`: `"ok"` quando pronto para receber OCR. Qualquer outro valor = ainda não pronto.
- `motor`: o nome de catálogo do motor (ecoa `HANZITRACKER_MOTOR`; fallback `RapidOCR`).
- `versaoContrato`: inteiro; o app recusa se for **maior** que `VersaoContratoOcr`.

### `GET /api/hardware`

Devolve os nomes reais do hardware para a UI de compatibilidade (CPU × modelo × API).

Resposta `200 OK`:

```json
{
  "cpu": "12th Gen Intel(R) Core(TM) i5-12400F",
  "gpus": ["NVIDIA GeForce RTX 3060", "..."]
}
```

### `GET /api/modelos`

Catálogo de **pesos** (não confundir com o catálogo de *motores* da Fase 5) e o estado de cada um.
Quem **baixa/remove** os pesos é o **Go** (escreve no AppData real, fora do sandbox do Python da
Store); o motor apenas **expõe o catálogo** e **lê** os pesos. Ver `appdata-python-store-virtualizado`.

Resposta `200 OK` — lista de:

```json
{
  "nome": "RapidOCR Server",
  "rotulo": "RapidOCR Server (Preciso)",
  "descricao": "Detector e reconhecedor 'server' do PP-OCRv4...",
  "idiomas": ["zh", "en"],
  "baixavel": true,
  "embutido": false,
  "instalado": false,
  "tamanhoBytes": 0,
  "arquivos": [
    { "nome": "ch_PP-OCRv4_det_server_infer.onnx", "url": "https://.../...onnx", "sha256": "" }
  ]
}
```

- `arquivos[].sha256`: hash esperado do arquivo. Quando **preenchido**, o Go confere após o download e
  aborta em caso de divergência; **vazio** = verificação pulada (torna-se **obrigatório** para os
  binários de motor na Fase 5 e para pesos `.pth` — pickle executa código ao desserializar).
- **Peso publicado zipado:** se `arquivos[].url` termina em `.zip` mas `arquivos[].nome` não (caso do
  EasyOCR, cujo upstream publica cada `.pth` zipado), o Go baixa o zip, confere o `sha256` **do zip** e
  extrai o `nome` na pasta de modelos, descartando o zip.

### `POST /api/ocr`

Executa o OCR sobre uma imagem.

- **Corpo:** bytes brutos de um **PNG** (não multipart, não base64).
- **Headers** (injetados pelo Go a partir das configurações do usuário; todos opcionais):

  | Header           | Tipo   | Efeito                                                        |
  |------------------|--------|--------------------------------------------------------------|
  | `X-Ocr-Model`    | string | Nome do modelo/peso a usar.                                  |
  | `X-Ocr-Device`   | string | Dispositivo lógico de inferência.                           |
  | `X-Ocr-Hardware` | string | Hardware selecionado (CPU / DirectML / CUDA).               |
  | `X-Ocr-Threads`  | int    | Limite de threads de CPU.                                    |
  | `X-Ocr-Max-Side` | int    | Maior lado (px) da imagem enviada; `0`/ausente = sem limite.|

Resposta `200 OK` — lista de detecções (**uma por linha** reconhecida):

```json
[
  { "texto": "你好世界", "confianca": 0.98, "caixa": [x0, y0, x1, y1] }
]
```

- `caixa`: 4 números `[x0, y0, x1, y1]` (cantos superior-esquerdo e inferior-direito, em px da imagem
  enviada). O app estima a posição de cada palavra repartindo a largura da linha proporcionalmente à
  contagem de caracteres.
- Erro: `500` com `{ "error": "mensagem" }`.

## Encerramento

O ciclo de vida do processo do motor é gerido pelo orquestrador (`main.go`), que derruba a árvore
inteira (`taskkill /F /T /PID`) ao fechar o app. O motor não precisa de endpoint de shutdown.
