# ----- Testes do motor RapidOCR -----
# Smoke tests do catálogo de pesos ONNX (motores/rapidocr/ModelosManifesto.py): consultas e
# integridade do catálogo.

from motores.rapidocr import ModelosManifesto


# ----- Consultas de modelos baixáveis -----

def testarObterModeloBaixavelExistente():
    modelo = ModelosManifesto.obterModelo("RapidOCR Server")
    assert modelo is not None
    assert "det" in modelo["arquivos"]
    assert "rec" in modelo["arquivos"]


def testarObterModeloInexistenteRetornaNone():
    assert ModelosManifesto.obterModelo("Motor Que Nao Existe") is None


def testarEhBaixavel():
    assert ModelosManifesto.ehBaixavel("RapidOCR Server") is True
    # "RapidOCR" é embutido (pesos mobile do pacote), não baixável.
    assert ModelosManifesto.ehBaixavel("RapidOCR") is False
    assert ModelosManifesto.ehBaixavel("inexistente") is False


# ----- Integridade do catálogo -----

def testarTodoModeloBaixavelTemUrlHttpsEArquivosOnnx():
    assert ModelosManifesto.MODELOS_BAIXAVEIS, "catálogo de modelos baixáveis vazio"
    for nome, info in ModelosManifesto.MODELOS_BAIXAVEIS.items():
        assert info["arquivos"], f"{nome} sem arquivos"
        for papel, arq in info["arquivos"].items():
            assert arq["url"].startswith("https://"), f"{nome}/{papel} com url não-https"
            assert arq["nome"].endswith(".onnx"), f"{nome}/{papel} não é .onnx"


def testarModeloEmbutidoRapidOcrPresente():
    assert "RapidOCR" in ModelosManifesto.MODELOS_EMBUTIDOS
