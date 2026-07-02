# ----- Testes do motor EasyOCR -----
# Integridade do manifesto de pesos (motores/easyocr/ModelosManifesto.py) — smoke tests, nada aqui
# sobe motor nem toca disco/rede.

from motores.easyocr import ModelosManifesto


# ----- Integridade do manifesto -----

def testarManifestoIntegro():
    assert ModelosManifesto.MODELOS_BAIXAVEIS, "catálogo de pesos do EasyOCR vazio"
    for nome, info in ModelosManifesto.MODELOS_BAIXAVEIS.items():
        for papel, arq in info["arquivos"].items():
            assert arq["url"].startswith("https://"), f"{nome}/{papel} com url não-https"
            assert arq["nome"].endswith(".pth"), f"{nome}/{papel} não é .pth"
            # .pth é pickle (desserializar executa código): sha256 é OBRIGATÓRIO, como em executáveis.
            assert arq["sha256"], f"{nome}/{papel} sem sha256"
            assert arq["url"].endswith(".zip"), f"{nome}/{papel}: o JaidedAI publica os pesos zipados"


def testarManifestoNaoEmbuteNada():
    # O pacote easyocr não traz pesos; o catálogo precisa refletir isso (UI exige o download).
    assert ModelosManifesto.MODELOS_EMBUTIDOS == {}
