# ----- Manifesto de Modelos do Motor Tesseract -----
# Catálogo dos pesos (.traineddata) do sidecar Tesseract. Mesmo formato do
# motores/rapidocr/ModelosManifesto.py: quem BAIXA/REMOVE os arquivos é o Go, em
# %APPDATA%\HanziTracker\motores_ocr\Tesseract\modelos\; o Python só informa nome/url/sha256 e LÊ os pesos
# (via --tessdata-dir).
#
# O `sha256` é preenchido e conferido pelo Go após o download. As URLs apontam para a TAG 4.1.0 dos
# repositórios oficiais tessdata_* (imutável), não para `main` — um push no upstream não muda o
# arquivo baixado nem quebra a verificação de integridade.

# ----- Catálogo -----
_BASE_BEST = "https://github.com/tesseract-ocr/tessdata_best/raw/4.1.0"

MODELOS_BAIXAVEIS = {
    "Tesseract Preciso": {
        "rotulo": "Tesseract Preciso (tessdata_best)",
        "descricao": "Modelos 'best' oficiais do Tesseract (chinês simplificado + inglês): máxima precisão, mais lentos.",
        "idiomas": ["zh", "en"],
        "arquivos": {
            "chi": {
                "nome": "chi_sim.traineddata",
                "url": f"{_BASE_BEST}/chi_sim.traineddata",
                "desc": "Chinês simplificado (best)",
                "sha256": "4fef2d1306c8e87616d4d3e4c6c67faf5d44be3342290cf8f2f0f6e3aa7e735b",
                "tamanho_bytes": 13077423,
            },
            "eng": {
                "nome": "eng.traineddata",
                "url": f"{_BASE_BEST}/eng.traineddata",
                "desc": "Inglês (best)",
                "sha256": "8280aed0782fe27257a68ea10fe7ef324ca0f8d85bd2fd145d1c2b560bcb66ba",
                "tamanho_bytes": 15400601,
            },
        },
    },
}


# ----- Pesos embutidos no motor (não baixáveis) -----
# O sidecar Tesseract já vem com chi_sim (tessdata_fast 4.1.0) + eng na pasta tesseract\tessdata,
# empacotados pelo build_sidecars.ps1 — OCR funcional sem download extra, como o RapidOCR mobile.
MODELOS_EMBUTIDOS = {
    "Tesseract Rápido": {
        "rotulo": "Tesseract Rápido (Embutido)",
        "descricao": "Modelos rápidos (tessdata_fast) de chinês e inglês que já vêm com o motor Tesseract.",
        "idiomas": ["zh", "en"],
    },
}


# ----- Consultas -----
def obterModelo(nome):
    """Retorna a configuração do modelo baixável pelo nome, ou None se não existir."""
    return MODELOS_BAIXAVEIS.get(nome)


def ehBaixavel(nome):
    """Indica se o modelo é do tipo baixável (arquivos .traineddata sob demanda)."""
    return nome in MODELOS_BAIXAVEIS
