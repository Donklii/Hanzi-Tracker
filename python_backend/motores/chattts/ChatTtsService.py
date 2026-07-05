# ----- Importações -----
from typing import Callable, Optional, Tuple

import numpy as np

from tts.ServicoTtsBase import ServicoTtsBase


# ----- Constantes do Módulo -----
# O ChatTTS sorteia um locutor a cada carga; a semente fixa estabiliza o timbre entre execuções
# (sem ela, a voz do app mudaria a cada reinício do sidecar).
SEMENTE_VOZ = 42
# Temperatura baixa = prosódia mais estável (cards são palavras/frases curtas; não queremos as
# hesitações/risadas espontâneas que o ChatTTS insere em temperaturas altas).
TEMPERATURA = 0.3
# O ChatTTS sintetiza sempre a 24 kHz.
TAXA_AMOSTRAGEM = 24000


# ----- Serviço de TTS (ChatTTS) -----
class ChatTtsService(ServicoTtsBase):
    """Motor de voz ChatTTS: prosódia conversacional natural em chinês, mais pesado que o Kokoro
    (~1 GB de pesos, síntese mais lenta em CPU).

    O texto enviado é o HANZI do card (pronúncia nativa); o pinyin exibido na UI é só leitura
    visual.
    """

    def _inicializarTts(self, aoProgredir: Optional[Callable[[str], None]] = None) -> None:
        # Import preguiçoso: o ChatTTS puxa o torch (pesado) — só carrega na primeira síntese.
        import torch
        import ChatTTS

        chat = ChatTTS.Chat()
        # source="huggingface": baixa os pesos do repositório 2Noise/ChatTTS para o cache HF (que o
        # entry redireciona a motores_tts/<Motor>/modelos/hf) na primeira carga; nas seguintes, lê do cache.
        if not chat.load(source="huggingface", compile=False):
            raise RuntimeError(
                "Falha ao carregar os modelos do ChatTTS — download do Hugging Face incompleto? "
                "Verifique a conexão e tente novamente."
            )

        torch.manual_seed(SEMENTE_VOZ)
        self._voz = chat.sample_random_speaker()
        self._tts = chat

    def _executarTts(self, texto: str) -> Tuple[np.ndarray, int]:
        import ChatTTS

        parametros = ChatTTS.Chat.InferCodeParams(spk_emb=self._voz, temperature=TEMPERATURA)
        ondas = self._tts.infer([texto], params_infer_code=parametros)

        # Guard clause: nada sintetizado
        if ondas is None or len(ondas) == 0 or ondas[0] is None:
            raise RuntimeError(f"ChatTTS não gerou áudio para o texto: {texto!r}")

        return np.asarray(ondas[0], dtype=np.float32).reshape(-1), TAXA_AMOSTRAGEM
