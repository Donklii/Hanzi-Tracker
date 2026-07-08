# ----- Importações -----
import os

# ----- Configurações Gerais -----
TESTANDO = False

# ----- Configurações do OCR -----
# OCR feito com RapidOCR (onnxruntime): roda os modelos leves do PP-OCR em CPU, rápido
# (~1s a 1080p) e sem o bug de inferência do paddlepaddle 3.x que travava os modelos
# mobile e o mkldnn (ConvertPirAttribute2RuntimeAttribute).
CONFIANCA_MINIMA_OCR = 0.5
# Limita as threads do onnxruntime para não saturar a CPU (preserva o FPS de jogos em foco).
THREADS_CPU_OCR = 4
# Hardware selecionado para inferência: CPU ou GPU específica detectada
HARDWARE_SELECIONADO = "CPU"
# Dispositivo de execução do OCR: "cpu" ou "webgpu" ("directml"/"cuda" são valores legados de
# configs antigas, tratados como pedido de GPU pelo OcrService)
DISPOSITIVO_OCR = "cpu"
# Modelo (PESO) de OCR a utilizar, do catálogo do motor ativo (ex.: "RapidOCR", "RapidOCR Server",
# "Tesseract Rápido", "EasyOCR Chinês"). Chega pelo header X-Ocr-Model injetado pelo Go.
MODELO_OCR = "RapidOCR"
# Limite do lado maior da captura antes do OCR (em px). 0 = resolução nativa do monitor (sem redução).
LIMITE_LADO_MAIOR_OCR = 0
# Descarrega o motor de OCR da memória após este tempo (em segundos) sem uso, liberando RAM (e VRAM,
# no WebGPU). É recarregado sob demanda no próximo scan (custo único de ~1-2s). Como o auto-scan
# padrão ocorre a cada 10s (IntervaloCapturaSegundos), o motor fica "quente" durante o uso ativo e só
# é descarregado quando o app fica ocioso (sem scans) por este tempo. 0 = nunca descarregar.
SEGUNDOS_OCIOSO_DESCARREGAR_OCR = 120

# ----- Configurações do TTS -----
# Limita as threads do torch na síntese de fala, pelo mesmo motivo do OCR: não saturar a CPU
# (preserva o FPS de jogos em foco).
THREADS_CPU_TTS = 4
# Descarrega o modelo de TTS da memória após este tempo (em segundos) sem uso. Maior que o do OCR
# porque a recarga do torch é bem mais cara (~10-30s) e o TTS só roda por ação explícita do usuário.
# 0 = nunca descarregar.
SEGUNDOS_OCIOSO_DESCARREGAR_TTS = 300

# ----- Armazenamento (AppData) -----
# Pasta dedicada criada sob demanda em %APPDATA%\HanziTracker para o banco de progresso
# e o arquivo de configurações editáveis pelo usuário.
NOME_PASTA_DADOS_APPDATA = "HanziTracker"


def obterNomeMotor():
    """Nome de catálogo DESTE motor: define a subpasta de pesos dele (motores_ocr/<Motor>/modelos no
    OCR, motores_tts/<Motor>/modelos/hf no TTS). O Go injeta HANZITRACKER_MOTOR ao subir o sidecar com o mesmo
    nome que usa em PastaModelosMotor(), garantindo que o Python lê os pesos exatamente onde o Go os
    baixa. Fallback "RapidOCR" para execução avulsa (dev) e para valores inválidos como segmento de pasta.
    """
    nome = os.environ.get("HANZITRACKER_MOTOR", "").strip()
    # Guard clause: ausente, com separador de caminho (/ ou \, checados os DOIS explicitamente porque
    # os.path.basename só reconhece o do SO atual — no Linux do CI, "\" não seria separador e um valor
    # como "..\fora" passaria batido) ou referência relativa (. / ..). Não pode escapar da subpasta de pesos.
    if not nome or "/" in nome or "\\" in nome or nome in (".", ".."):
        return "RapidOCR"
    return nome


def obterPastaDados():
    """Pasta de dados do app (modelos, logs). Prioriza HANZITRACKER_DATA_DIR, que o Go injeta com o
    caminho REAL — necessário porque o Python da Microsoft Store virtualiza o %APPDATA% para um
    sandbox, divergindo de onde o Go (não-sandbox) grava config/banco.
    """
    base = os.environ.get("HANZITRACKER_DATA_DIR")
    if not base:
        raiz = os.environ.get("APPDATA") or os.path.expanduser("~")
        base = os.path.join(raiz, NOME_PASTA_DADOS_APPDATA)
    os.makedirs(base, exist_ok=True)
    return base
