# ----- Importações -----
import os
import shutil
import subprocess
import sys
import tempfile
from typing import Callable, Dict, List, Optional, Tuple

import numpy as np

from motores.tesseract import ModelosManifesto
from ocr.GerenciadorModelosModule import GerenciadorModelos
from ocr.ServicoOcrBase import ServicoOcrBase, MENSAGEM_CARREGANDO_MODELO
from principal import ConstantesModule


# ----- Constantes do Módulo -----
# PSM 11 = "sparse text": acha o máximo de texto espalhado, sem supor um layout de página — o caso
# de telas de jogo, onde falas/menus aparecem em regiões soltas (o padrão, PSM 3, supõe documento).
_PSM_TEXTO_ESPARSO = "11"
_IDIOMAS = "chi_sim+eng"


# ----- Funções puras (parsing do TSV) -----

def _ehCaractereCjk(caractere: str) -> bool:
    """Indica se o caractere é CJK (ideogramas, extensão A, pontuação CJK ou formas de largura plena)."""
    return (
        "一" <= caractere <= "鿿"   # ideogramas unificados
        or "㐀" <= caractere <= "䶿"  # extensão A
        or "　" <= caractere <= "〿"  # pontuação CJK (。、「」…)
        or "＀" <= caractere <= "￯"  # formas de largura plena (！？：…)
    )


def juntarPalavras(palavras: List[str]) -> str:
    """Junta as 'palavras' do Tesseract numa linha: sem espaço entre caracteres chineses
    (o Tesseract fragmenta o chinês em blocos), com espaço entre palavras latinas."""
    texto = ""
    for palavra in palavras:
        if not texto:
            texto = palavra
        elif _ehCaractereCjk(texto[-1]) or _ehCaractereCjk(palavra[0]):
            texto += palavra
        else:
            texto += " " + palavra
    return texto


def agruparLinhasTsv(tsv: str) -> List[Tuple[List[Tuple[float, float]], str, float]]:
    """Converte o TSV do Tesseract (uma linha por PALAVRA) no formato bruto dos serviços de OCR:
    [(caixa de 4 cantos, texto, confiança 0..1)], com uma entrada por LINHA de texto.

    Colunas do TSV: level page block par line word left top width height conf text.
    Só as linhas de nível 5 (palavra) trazem texto e confiança (conf >= 0)."""
    palavras_por_linha: Dict[Tuple[str, str, str, str], List[dict]] = {}

    for indice, linha in enumerate(tsv.splitlines()):
        # Guard clause: cabeçalho do TSV
        if indice == 0:
            continue

        campos = linha.split("\t")
        if len(campos) < 12:
            continue
        if campos[0] != "5":  # nível 5 = palavra
            continue

        confianca = float(campos[10])
        if confianca < 0:  # -1 = entrada estrutural, sem texto reconhecido
            continue

        texto = campos[11].strip()
        if not texto:
            continue

        chave = (campos[1], campos[2], campos[3], campos[4])  # (page, block, par, line)
        palavras_por_linha.setdefault(chave, []).append({
            "texto": texto,
            "confianca": confianca,
            "esquerda": int(campos[6]),
            "topo": int(campos[7]),
            "largura": int(campos[8]),
            "altura": int(campos[9]),
        })

    resultado = []
    for palavras in palavras_por_linha.values():
        texto = juntarPalavras([p["texto"] for p in palavras])
        confianca = sum(p["confianca"] for p in palavras) / len(palavras) / 100.0
        x0 = min(p["esquerda"] for p in palavras)
        y0 = min(p["topo"] for p in palavras)
        x1 = max(p["esquerda"] + p["largura"] for p in palavras)
        y1 = max(p["topo"] + p["altura"] for p in palavras)
        caixa = [(float(x0), float(y0)), (float(x1), float(y0)), (float(x1), float(y1)), (float(x0), float(y1))]
        resultado.append((caixa, texto, confianca))

    return resultado


# ----- Serviço Tesseract -----
class TesseractService(ServicoOcrBase):
    """Motor Tesseract: dirige o tesseract.exe empacotado junto ao sidecar via subprocess.

    Sem bindings nativos: a imagem vai num PNG temporário e o resultado volta em TSV pelo stdout,
    agrupado por linha em agruparLinhasTsv. Os pesos embutidos (tessdata_fast) moram em
    tesseract\\tessdata ao lado do exe; os baixáveis (tessdata_best), em modelos\\Tesseract\\.
    """

    def __init__(self) -> None:
        super().__init__()
        self._gerenciadorModelos = GerenciadorModelos(ModelosManifesto)

    def _resolverExecutavel(self) -> str:
        """Localiza o tesseract.exe: empacotado ao lado do sidecar (distribuído) ou no PATH (dev)."""
        if getattr(sys, "frozen", False):
            candidato = os.path.join(os.path.dirname(sys.executable), "tesseract", "tesseract.exe")
            if os.path.isfile(candidato):
                return candidato

        achado = shutil.which("tesseract")
        if achado:
            return achado

        raise RuntimeError(
            "O executável do Tesseract não foi encontrado neste motor. "
            "Remova e baixe novamente o motor Tesseract em Configurações → Gerenciar Motores."
        )

    def _inicializarOcr(self, aoProgredir: Optional[Callable[[str], None]] = None) -> None:
        """Resolve o executável e a pasta de pesos (tessdata) do modelo selecionado."""
        self._liberarMotorAnterior()

        executavel = self._resolverExecutavel()
        modelo_solicitado = ConstantesModule.MODELO_OCR

        if ModelosManifesto.ehBaixavel(modelo_solicitado):
            # O download é feito pelo Go (escreve no AppData real). Aqui apenas LEMOS os arquivos.
            if not self._gerenciadorModelos.modeloInstalado(modelo_solicitado):
                raise RuntimeError(
                    f"O modelo '{modelo_solicitado}' ainda não foi baixado. "
                    "Baixe-o em Configurações → OCR & Processamento → Gerenciar Modelos."
                )
            tessdata = self._gerenciadorModelos.obterPastaModelos()
        else:
            # Qualquer outro nome (inclusive o modelo de outro motor, se o usuário acabou de trocar)
            # cai nos pesos embutidos — o motor sempre funciona.
            tessdata = os.path.join(os.path.dirname(executavel), "tessdata")

        self._emitirProgresso(aoProgredir, MENSAGEM_CARREGANDO_MODELO)
        self._ocr = {"executavel": executavel, "tessdata": tessdata}

    def _executarOcr(self, imagem: np.ndarray):
        import cv2

        env = os.environ.copy()
        # Tesseract paraleliza com OpenMP; limita as threads para preservar o FPS do jogo em foco,
        # como o intra_op_num_threads faz no onnxruntime.
        env["OMP_THREAD_LIMIT"] = str(ConstantesModule.THREADS_CPU_OCR)

        # PNG temporário: o tesseract.exe lê de arquivo; NamedTemporaryFile fechado antes para o
        # subprocess conseguir abrir no Windows (compartilhamento de arquivo).
        arquivo = tempfile.NamedTemporaryFile(suffix=".png", delete=False)
        try:
            arquivo.close()
            cv2.imwrite(arquivo.name, cv2.cvtColor(imagem, cv2.COLOR_RGB2BGR))

            processo = subprocess.run(
                [
                    self._ocr["executavel"], arquivo.name, "stdout",
                    "--tessdata-dir", self._ocr["tessdata"],
                    "-l", _IDIOMAS,
                    "--psm", _PSM_TEXTO_ESPARSO,
                    "tsv",
                ],
                stdout=subprocess.PIPE,
                stderr=subprocess.PIPE,
                env=env,
                text=True,
                encoding="utf-8",
                errors="ignore",
                creationflags=0x08000000 if sys.platform == "win32" else 0,  # CREATE_NO_WINDOW
            )
        finally:
            os.unlink(arquivo.name)

        if processo.returncode != 0:
            resumo = (processo.stderr or "").strip().splitlines()
            raise RuntimeError(
                "O Tesseract falhou ao reconhecer a imagem: " + (resumo[-1] if resumo else "erro desconhecido")
            )

        return agruparLinhasTsv(processo.stdout)
