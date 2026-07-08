# ----- Manifesto de Modelos de OCR Baixáveis -----
# Catálogo dos modelos ONNX (RapidOCR / PP-OCR) que podem ser baixados sob demanda
# para %APPDATA%\HanziTracker\motores_ocr\RapidOCR\modelos. Apenas modelos ONNX entram aqui: são arquivos
# puros e funcionam mesmo no executável compilado (.exe), ao contrário de engines pip
# (EasyOCR, onnxruntime-webgpu/gpu), que não podem ser instalados num exe congelado.
#
# Cada modelo lista os arquivos que o compõem (detector "det", reconhecedor "rec" e,
# opcionalmente, classificador "cls"). O campo `sha256`, quando preenchido, é conferido
# após o download para garantir integridade; vazio = verificação pulada.

# ----- Catálogo -----
# Todos os modelos abaixo usam o dicionário chinês padrão (ppocr_keys_v1) e o reconhecedor de
# altura 48 px (PP-OCRv3/v4) — compatíveis com o pipeline padrão do RapidOCR, sem configuração
# extra. Evitamos PP-OCRv2/v2.0 (altura 32) para não quebrar a inferência.
_BASE_URL = "https://huggingface.co/SWHL/RapidOCR/resolve/main"

MODELOS_BAIXAVEIS = {
    "RapidOCR Server": {
        "rotulo": "RapidOCR Server (Preciso)",
        "descricao": "Detector e reconhecedor 'server' do PP-OCRv4: máxima precisão, mais pesado.",
        "idiomas": ["zh", "en"],
        "arquivos": {
            "det": {
                "nome": "ch_PP-OCRv4_det_server_infer.onnx",
                "url": f"{_BASE_URL}/PP-OCRv4/ch_PP-OCRv4_det_server_infer.onnx",
                "desc": "Detector RapidOCR Server",
                "sha256": "",
                "tamanho_bytes": 0,
            },
            "rec": {
                "nome": "ch_PP-OCRv4_rec_server_infer.onnx",
                "url": f"{_BASE_URL}/PP-OCRv4/ch_PP-OCRv4_rec_server_infer.onnx",
                "desc": "Reconhecedor RapidOCR Server",
                "sha256": "",
                "tamanho_bytes": 0,
            },
        },
    },
    "RapidOCR Detecção Forte": {
        "rotulo": "RapidOCR Detecção Forte (Precisa + Rápida)",
        "descricao": "Detector 'server' (localiza melhor o texto) com reconhecedor leve — boa precisão sem o custo total do Server.",
        "idiomas": ["zh", "en"],
        "arquivos": {
            "det": {
                "nome": "ch_PP-OCRv4_det_server_infer.onnx",
                "url": f"{_BASE_URL}/PP-OCRv4/ch_PP-OCRv4_det_server_infer.onnx",
                "desc": "Detector RapidOCR Server",
                "sha256": "",
                "tamanho_bytes": 0,
            },
            "rec": {
                "nome": "ch_PP-OCRv4_rec_infer.onnx",
                "url": f"{_BASE_URL}/PP-OCRv4/ch_PP-OCRv4_rec_infer.onnx",
                "desc": "Reconhecedor RapidOCR (leve)",
                "sha256": "",
                "tamanho_bytes": 0,
            },
        },
    },
    "RapidOCR v3": {
        "rotulo": "RapidOCR v3 (Compatível)",
        "descricao": "Geração PP-OCRv3 — alternativa útil em fontes/telas onde o v4 reconhece pior.",
        "idiomas": ["zh", "en"],
        "arquivos": {
            "det": {
                "nome": "ch_PP-OCRv3_det_infer.onnx",
                "url": f"{_BASE_URL}/PP-OCRv3/ch_PP-OCRv3_det_infer.onnx",
                "desc": "Detector RapidOCR v3",
                "sha256": "",
                "tamanho_bytes": 0,
            },
            "rec": {
                "nome": "ch_PP-OCRv3_rec_infer.onnx",
                "url": f"{_BASE_URL}/PP-OCRv3/ch_PP-OCRv3_rec_infer.onnx",
                "desc": "Reconhecedor RapidOCR v3",
                "sha256": "",
                "tamanho_bytes": 0,
            },
        },
    },
}


# ----- Pesos embutidos no motor (não baixáveis) -----
# "Embutido" aqui é no nível de PESO, não de motor: os pesos mobile do PP-OCR já acompanham o pacote
# rapidocr_onnxruntime, ou seja, vêm DENTRO do próprio motor RapidOCR (que na Fase 5 é um sidecar
# baixável). Enquanto o motor RapidOCR estiver ativo, esses pesos estão disponíveis sem download extra.
MODELOS_EMBUTIDOS = {
    "RapidOCR": {
        "rotulo": "RapidOCR (CPU Leve)",
        "descricao": "Modelos mobile do PP-OCR que já vêm com o motor RapidOCR (sem download extra).",
        "idiomas": ["zh", "en"],
    },
}


# ----- Consultas -----
def obterModelo(nome):
    """Retorna a configuração do modelo baixável pelo nome, ou None se não existir."""
    return MODELOS_BAIXAVEIS.get(nome)


def ehBaixavel(nome):
    """Indica se o modelo é do tipo baixável (arquivos ONNX sob demanda)."""
    return nome in MODELOS_BAIXAVEIS
