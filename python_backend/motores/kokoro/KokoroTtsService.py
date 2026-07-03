# ----- Importações -----
from typing import Callable, Optional, Tuple

import numpy as np

from tts.ServicoTtsBase import ServicoTtsBase


# ----- Constantes do Módulo -----
# Repositório dos pesos no Hugging Face (baixados na primeira síntese, para o cache HF que o entry
# redireciona a modelos/<Motor>/hf). Modelo de ~82M parâmetros, ~330 MB de download.
REPO_KOKORO = "hexgrad/Kokoro-82M"
# Voz padrão de mandarim (feminina). Outras do repositório: zf_xiaoni, zf_xiaoxiao, zf_xiaoyi,
# zm_yunjian, zm_yunxi, zm_yunxia, zm_yunyang.
VOZ_PADRAO = "zf_xiaobei"
# O Kokoro sintetiza sempre a 24 kHz.
TAXA_AMOSTRAGEM = 24000


# ----- Serviço de TTS (Kokoro-82M) -----
class KokoroTtsService(ServicoTtsBase):
    """Motor de voz Kokoro-82M: leve (roda bem em CPU) e com vozes de mandarim de boa qualidade.

    O G2P de chinês vem do misaki[zh] (jieba + pypinyin) — o texto enviado é o HANZI do card,
    garantindo pronúncia nativa (o pinyin exibido na UI é só leitura visual).
    """

    def _inicializarTts(self, aoProgredir: Optional[Callable[[str], None]] = None) -> None:
        # Import preguiçoso: o kokoro puxa o torch (pesado) — só carrega na primeira síntese.
        from kokoro import KPipeline
        self._tts = KPipeline(lang_code="z", repo_id=REPO_KOKORO)

    def _executarTts(self, texto: str) -> Tuple[np.ndarray, int]:
        # O pipeline devolve um gerador de trechos (graphemes, phonemes, audio); textos curtos de
        # card normalmente rendem um único trecho, mas concatenamos todos por robustez.
        pedacos = []
        for _, _, audio in self._tts(texto, voice=VOZ_PADRAO):
            # Guard clause: trecho sem áudio (ex.: pontuação isolada)
            if audio is None:
                continue
            if hasattr(audio, "detach"):  # tensor do torch → numpy
                pedacos.append(audio.detach().cpu().numpy())
            else:
                pedacos.append(np.asarray(audio, dtype=np.float32))

        # Guard clause: nenhum trecho sintetizado
        if not pedacos:
            raise RuntimeError(f"Kokoro não gerou áudio para o texto: {texto!r}")

        return np.concatenate(pedacos), TAXA_AMOSTRAGEM
