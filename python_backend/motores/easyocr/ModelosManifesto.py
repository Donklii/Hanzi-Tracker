# ----- Manifesto de Modelos do Motor EasyOCR -----
# Catálogo dos pesos (.pth) do sidecar EasyOCR. Mesmo formato do motores/rapidocr/ModelosManifesto.py:
# quem BAIXA/REMOVE os arquivos é o Go, em %APPDATA%\HanziTracker\motores_ocr\EasyOCR\modelos\; o Python só
# informa nome/url/sha256 e LÊ os pesos (model_storage_directory do easyocr.Reader).
#
# Particularidades deste motor:
# - As releases oficiais do JaidedAI publicam cada peso ZIPADO (um .pth na raiz do zip). O Go detecta
#   a URL .zip, confere o sha256 DO ZIP e extrai o `nome` (.pth) na pasta de modelos.
# - O `sha256` é OBRIGATÓRIO aqui: .pth é pickle do PyTorch (desserializar executa código), então um
#   peso adulterado é tão perigoso quanto um executável.
# - NÃO há peso embutido: o pacote easyocr não traz modelos (baixa no primeiro uso, o que o app não
#   permite ao Python). O primeiro OCR exige baixar o modelo em "Gerenciar Modelos".

# ----- Catálogo -----
_BASE_JAIDED = "https://github.com/JaidedAI/EasyOCR/releases/download"

MODELOS_BAIXAVEIS = {
    "EasyOCR Chinês": {
        "rotulo": "EasyOCR Chinês (CRAFT + zh_sim_g2)",
        "descricao": "Detector CRAFT + reconhecedor oficial de chinês simplificado e inglês do EasyOCR. Obrigatório: este motor não embute pesos.",
        "idiomas": ["zh", "en"],
        "arquivos": {
            "det": {
                "nome": "craft_mlt_25k.pth",
                "url": f"{_BASE_JAIDED}/pre-v1.1.6/craft_mlt_25k.zip",
                "desc": "Detector CRAFT",
                "sha256": "8dc6a1c703a89ed56308ef742d26ebd45c656248cbbbda6e7fe60e569f873e65",
                "tamanho_bytes": 77251756,
            },
            "rec": {
                "nome": "zh_sim_g2.pth",
                "url": f"{_BASE_JAIDED}/v1.3/zh_sim_g2.zip",
                "desc": "Reconhecedor chinês simplificado + inglês",
                "sha256": "eed38bdd9e612bd150f13d2f70fd8e5b24c79c2e2fe621a02603b38d385503e9",
                "tamanho_bytes": 20288076,
            },
        },
    },
}


# ----- Pesos embutidos no motor (não baixáveis) -----
# Vazio de propósito: ver o cabeçalho — o EasyOCR não embute pesos.
MODELOS_EMBUTIDOS = {}


# ----- Consultas -----
def obterModelo(nome):
    """Retorna a configuração do modelo baixável pelo nome, ou None se não existir."""
    return MODELOS_BAIXAVEIS.get(nome)


def ehBaixavel(nome):
    """Indica se o modelo é do tipo baixável (arquivos .pth sob demanda)."""
    return nome in MODELOS_BAIXAVEIS
