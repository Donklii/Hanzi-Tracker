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
# Dispositivo de execução do OCR: "cpu", "cuda", ou "directml" (definido de acordo com o hardware e a API)
DISPOSITIVO_OCR = "cpu"
# Modelo (PESO) de OCR a utilizar, do catálogo do motor ativo (ex.: "RapidOCR", "RapidOCR Server",
# "Tesseract Rápido", "EasyOCR Chinês"). Chega pelo header X-Ocr-Model injetado pelo Go.
MODELO_OCR = "RapidOCR"
# Limite do lado maior da captura antes do OCR (em px). 0 = resolução nativa do monitor (sem redução).
LIMITE_LADO_MAIOR_OCR = 0

# ----- Armazenamento (AppData) -----
# Pasta dedicada criada sob demanda em %APPDATA%\HanziTracker para o banco de progresso
# e o arquivo de configurações editáveis pelo usuário.
NOME_PASTA_DADOS_APPDATA = "HanziTracker"


def obterNomeMotor():
    """Nome de catálogo DESTE motor (define a subpasta de pesos modelos/<Motor>). O Go injeta
    HANZITRACKER_MOTOR ao subir o sidecar com o mesmo nome que usa em pastaModelosMotor(), garantindo
    que o Python lê os pesos exatamente onde o Go os baixa. Fallback "RapidOCR" para execução avulsa
    (dev) e para valores inválidos como segmento de pasta.
    """
    nome = os.environ.get("HANZITRACKER_MOTOR", "").strip()
    # Guard clause: ausente ou com separador de caminho (não pode escapar de modelos/)
    if not nome or os.path.basename(nome) != nome:
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
