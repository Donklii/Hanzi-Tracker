# ----- Importações -----
from typing import Callable, Optional

import numpy as np

from motores.easyocr import ModelosManifesto
from ocr.GerenciadorModelosModule import GerenciadorModelos
from ocr.ServicoOcrBase import ServicoOcrBase, MENSAGEM_CARREGANDO_MODELO
from principal import ConstantesModule


# ----- Constantes do Módulo -----
# Único modelo deste motor (detector + reconhecedor). O header X-Ocr-Model pode chegar com o nome de
# um modelo de OUTRO motor (o usuário acabou de trocar de motor e a config ainda não acompanhou);
# como só existe um catálogo aqui, ele é sempre o usado.
NOME_MODELO = "EasyOCR Chinês"


# ----- Serviço EasyOCR -----
class EasyOcrService(ServicoOcrBase):
    """Motor EasyOCR (PyTorch, CPU): detector CRAFT + reconhecedor zh_sim_g2.

    O sidecar é congelado com o torch de CPU — sem variante CUDA (descartada por custo/benefício,
    como no RapidOCR; ver Fase 5 no TODO.md). Os pesos NÃO vêm embutidos: o Go os baixa para
    motores_ocr\\EasyOCR\\modelos\\ e aqui o Reader só os lê (download_enabled=False).
    """

    def __init__(self) -> None:
        super().__init__()
        self._gerenciadorModelos = GerenciadorModelos(ModelosManifesto)

    def _inicializarOcr(self, aoProgredir: Optional[Callable[[str], None]] = None) -> None:
        """Carrega o Reader do EasyOCR com os pesos baixados pelo Go."""
        self._liberarMotorAnterior()

        # O download é feito pelo Go (escreve no AppData real). Aqui apenas LEMOS os arquivos.
        if not self._gerenciadorModelos.modeloInstalado(NOME_MODELO):
            raise RuntimeError(
                f"O modelo '{NOME_MODELO}' ainda não foi baixado (o motor EasyOCR não embute pesos). "
                "Baixe-o em Configurações → OCR & Processamento → Gerenciar Modelos."
            )

        import torch
        import easyocr

        # Limita as threads do torch para preservar o FPS do jogo em foco, como o
        # intra_op_num_threads faz no onnxruntime.
        torch.set_num_threads(ConstantesModule.THREADS_CPU_OCR)

        self._emitirProgresso(aoProgredir, MENSAGEM_CARREGANDO_MODELO)

        pasta = self._gerenciadorModelos.obterPastaModelos()
        self._ocr = easyocr.Reader(
            ["ch_sim", "en"],
            gpu=False,
            model_storage_directory=pasta,
            user_network_directory=pasta,
            download_enabled=False,
            verbose=False,
        )

    def _executarOcr(self, imagem: np.ndarray):
        # readtext já devolve [(caixa de 4 cantos, texto, confiança 0..1)] — o formato bruto da base.
        return self._ocr.readtext(imagem)
