# ----- Testes do motor Tesseract -----
# Integridade do manifesto de pesos (motores/tesseract/ModelosManifesto.py) e o parser de TSV
# (motores/tesseract/TesseractService.py) — funções puras, nada aqui sobe motor nem toca disco/rede.

from motores.tesseract import ModelosManifesto
from motores.tesseract.TesseractService import agruparLinhasTsv, juntarPalavras


# ----- Integridade do manifesto -----

def testarManifestoIntegro():
    assert ModelosManifesto.MODELOS_BAIXAVEIS, "catálogo de pesos do Tesseract vazio"
    for nome, info in ModelosManifesto.MODELOS_BAIXAVEIS.items():
        for papel, arq in info["arquivos"].items():
            assert arq["url"].startswith("https://"), f"{nome}/{papel} com url não-https"
            assert arq["nome"].endswith(".traineddata"), f"{nome}/{papel} não é .traineddata"
            assert arq["sha256"], f"{nome}/{papel} sem sha256"
            assert arq["tamanho_bytes"] > 0, f"{nome}/{papel} sem tamanho"


def testarManifestoTemModeloEmbutido():
    # O sidecar embute tessdata_fast (chi_sim + eng): o motor funciona sem download extra.
    assert "Tesseract Rápido" in ModelosManifesto.MODELOS_EMBUTIDOS
    assert ModelosManifesto.ehBaixavel("Tesseract Preciso") is True
    assert ModelosManifesto.ehBaixavel("Tesseract Rápido") is False


# ----- Junção de palavras -----

def testarJuntarPalavrasChinesSemEspaco():
    assert juntarPalavras(["你好", "世界"]) == "你好世界"


def testarJuntarPalavrasLatinasComEspaco():
    assert juntarPalavras(["Hello", "World"]) == "Hello World"


def testarJuntarPalavrasMistas():
    # Fronteira CJK ↔ latino nunca ganha espaço; latino ↔ latino ganha.
    assert juntarPalavras(["你好", "HP", "10"]) == "你好HP 10"


# ----- Agrupamento do TSV em linhas -----

_TSV_EXEMPLO = "\n".join([
    "level\tpage_num\tblock_num\tpar_num\tline_num\tword_num\tleft\ttop\twidth\theight\tconf\ttext",
    "1\t1\t0\t0\t0\t0\t0\t0\t800\t600\t-1\t",           # página (estrutural, sem texto)
    "5\t1\t1\t1\t1\t1\t10\t20\t40\t30\t90\t你好",       # linha 1, palavra 1
    "5\t1\t1\t1\t1\t2\t50\t20\t40\t30\t80\t世界",       # linha 1, palavra 2
    "5\t1\t2\t1\t1\t1\t10\t100\t50\t20\t70\tStart",     # linha 2 (outro bloco)
    "5\t1\t2\t1\t1\t2\t70\t100\t50\t20\t60\tGame",      # linha 2, palavra 2
    "5\t1\t3\t1\t1\t1\t0\t200\t10\t10\t-1\tlixo",       # conf -1: descartada
    "5\t1\t3\t1\t1\t2\t0\t200\t10\t10\t95\t ",          # texto vazio: descartada
])


def testarAgruparLinhasTsv():
    resultado = agruparLinhasTsv(_TSV_EXEMPLO)
    assert len(resultado) == 2

    caixa1, texto1, conf1 = resultado[0]
    assert texto1 == "你好世界"
    assert conf1 == (90 + 80) / 2 / 100.0
    # União das caixas das palavras: x0=10, y0=20, x1=90, y1=50 (4 cantos).
    assert caixa1 == [(10.0, 20.0), (90.0, 20.0), (90.0, 50.0), (10.0, 50.0)]

    _, texto2, conf2 = resultado[1]
    assert texto2 == "Start Game"
    assert conf2 == (70 + 60) / 2 / 100.0


def testarAgruparLinhasTsvVazio():
    assert agruparLinhasTsv("") == []
    assert agruparLinhasTsv("level\tpage_num\tblock_num\tpar_num\tline_num\tword_num\tleft\ttop\twidth\theight\tconf\ttext") == []
