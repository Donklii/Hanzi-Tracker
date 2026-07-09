# ----- Importações -----
import numpy as np

from principal import ConstantesModule
from stt.ServicoSttBase import ServicoSttBase


# ----- Constantes do Módulo -----
# Modelo Paraformer de mandarim (FunASR) exportado para ONNX pelo projeto sherpa-onnx: preciso em
# frases E em caracteres isolados (o caso da revisão de pronúncia em sequência), rápido em CPU e
# sem torch — a inferência é onnxruntime puro. A variante int8 corta o download/RAM pela metade
# com perda de precisão irrelevante para este uso.
REPOSITORIO_HF = "csukuangfj/sherpa-onnx-paraformer-zh-2023-09-14"
ARQUIVO_MODELO = "model.int8.onnx"
ARQUIVO_TOKENS = "tokens.txt"
# Taxa nativa do modelo (o extrator de features do sherpa-onnx resampleia entradas diferentes).
TAXA_MODELO_HZ = 16000


# ----- Serviço de STT do Paraformer-ZH -----
class ParaformerSttService(ServicoSttBase):
    """Reconhecimento de fala em mandarim com o Paraformer (sherpa-onnx).

    Os pesos (~240 MB) são baixados do Hugging Face na primeira transcrição (ou no
    /api/stt/preparar), para o cache HF redirecionado pelo entry a
    motores_stt/<Motor>/modelos/hf — dentro da pasta do próprio motor no AppData.
    """

    def _inicializarStt(self, aoProgredir=None):
        import sherpa_onnx
        from huggingface_hub import hf_hub_download

        self._emitirProgresso(aoProgredir, "Baixando os pesos do modelo (primeira vez, ~240 MB)…")
        caminhoModelo = hf_hub_download(repo_id=REPOSITORIO_HF, filename=ARQUIVO_MODELO)
        caminhoTokens = hf_hub_download(repo_id=REPOSITORIO_HF, filename=ARQUIVO_TOKENS)

        self._emitirProgresso(aoProgredir, "Carregando o modelo de reconhecimento…")
        self._stt = sherpa_onnx.OfflineRecognizer.from_paraformer(
            paraformer=caminhoModelo,
            tokens=caminhoTokens,
            num_threads=max(1, ConstantesModule.THREADS_CPU_STT),
            sample_rate=TAXA_MODELO_HZ,
            feature_dim=80,
            decoding_method="greedy_search",
        )

    def _executarStt(self, amostras: np.ndarray, taxa: int) -> str:
        fluxo = self._stt.create_stream()
        fluxo.accept_waveform(taxa, amostras)
        self._stt.decode_stream(fluxo)
        return fluxo.result.text
