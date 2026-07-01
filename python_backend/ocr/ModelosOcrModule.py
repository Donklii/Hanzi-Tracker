# ----- Importações -----
from dataclasses import dataclass
from typing import Tuple


# ----- Modelos de Domínio -----
@dataclass(frozen=True)
class DeteccaoOcr:
    """Uma região de texto detectada pelo OCR, com sua caixa delimitadora na imagem.

    `caixa` é (x0, y0, x1, y1) em pixels da imagem capturada (canto superior-esquerdo e
    inferior-direito), usada para localizar a palavra na tela em relação ao mouse.
    """

    texto: str
    confianca: float
    caixa: Tuple[float, float, float, float]
