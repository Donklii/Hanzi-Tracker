# ----- Importações -----
import numpy as np

from principal import ConstantesModule
from stt.ServicoSttBase import ServicoSttBase


# ----- Constantes do Módulo -----
# Modelo Zipformer de mandarim GENUINAMENTE streaming (transducer do projeto sherpa-onnx): o
# decodificador consome o áudio em pedaços e mantém estado entre eles, então cada parcial custa só
# o trecho novo (custo constante) — diferente do Paraformer offline, que re-decodifica o áudio
# acumulado inteiro a cada tick. A variante 14M é pequena e rápida em CPU; em contrapartida, tende
# a ser menos precisa que o Paraformer, principalmente em caracteres isolados.
REPOSITORIO_HF = "csukuangfj/sherpa-onnx-streaming-zipformer-zh-14M-2023-02-23"
ARQUIVO_ENCODER = "encoder-epoch-99-avg-1.int8.onnx"
ARQUIVO_DECODER = "decoder-epoch-99-avg-1.onnx"  # o decoder de transducer não é quantizado (padrão do sherpa-onnx)
ARQUIVO_JOINER = "joiner-epoch-99-avg-1.int8.onnx"
ARQUIVO_TOKENS = "tokens.txt"
# Taxa nativa do modelo (o extrator de features do sherpa-onnx resampleia entradas diferentes).
TAXA_MODELO_HZ = 16000
# Cauda de silêncio anexada antes de fechar o fluxo na transcrição final: empurra as últimas
# sílabas pelo decodificador streaming (sem ela, o fim da fala pode ficar sem transcrever).
SEGUNDOS_CAUDA_SILENCIO = 0.66


# ----- Serviço de STT do Zipformer-ZH-Streaming -----
class ZipformerStreamingSttService(ServicoSttBase):
    """Reconhecimento de fala em mandarim em STREAMING de verdade (sherpa-onnx OnlineRecognizer).

    O fluxo de decodificação (OnlineStream) persiste entre os parciais de uma MESMA fala: cada
    /api/stt/parcial alimenta só o áudio novo e lê o texto acumulado. A geração de gravação
    (ServicoSttBase._geracaoGravacao) diz quando uma fala nova começou — aí o fluxo é recriado.

    Os pesos (~50 MB) são baixados do Hugging Face na primeira transcrição (ou no
    /api/stt/preparar), para o cache HF redirecionado pelo entry a
    motores_stt/<Motor>/modelos/hf — dentro da pasta do próprio motor no AppData.
    """

    def __init__(self):
        super().__init__()
        # Estado do fluxo streaming da fala ATUAL. Só é tocado sob o lock do modelo (como
        # self._stt): _executarStt/_executarSttParcial já rodam dentro dele.
        self._fluxoOnline = None      # OnlineStream persistente entre os parciais da mesma fala
        self._geracaoFluxo = -1       # geração de gravação a que o fluxo pertence
        self._quadrosAlimentados = 0  # quantas amostras da gravação já entraram no fluxo

    def _inicializarStt(self, aoProgredir=None):
        import sherpa_onnx
        from huggingface_hub import hf_hub_download

        self._emitirProgresso(aoProgredir, "Baixando os pesos do modelo (primeira vez, ~50 MB)…")
        caminhoEncoder = hf_hub_download(repo_id=REPOSITORIO_HF, filename=ARQUIVO_ENCODER)
        caminhoDecoder = hf_hub_download(repo_id=REPOSITORIO_HF, filename=ARQUIVO_DECODER)
        caminhoJoiner = hf_hub_download(repo_id=REPOSITORIO_HF, filename=ARQUIVO_JOINER)
        caminhoTokens = hf_hub_download(repo_id=REPOSITORIO_HF, filename=ARQUIVO_TOKENS)

        self._emitirProgresso(aoProgredir, "Carregando o modelo de reconhecimento…")
        self._stt = sherpa_onnx.OnlineRecognizer.from_transducer(
            tokens=caminhoTokens,
            encoder=caminhoEncoder,
            decoder=caminhoDecoder,
            joiner=caminhoJoiner,
            num_threads=max(1, ConstantesModule.THREADS_CPU_STT),
            sample_rate=TAXA_MODELO_HZ,
            feature_dim=80,
            decoding_method="greedy_search",
        )
        # Modelo recém-(re)carregado (primeira carga ou recarga pós-watchdog): um fluxo antigo
        # pertenceria ao reconhecedor descartado.
        self._fluxoOnline = None

    def _executarStt(self, amostras: np.ndarray, taxa: int) -> str:
        """Transcrição FINAL (/api/stt/parar): completa o fluxo da fala com o que faltar e o
        encerra — reaproveita tudo o que os parciais já decodificaram."""
        fluxo = self._obterFluxoDaFala(self._geracaoGravacao)
        self._alimentarFluxo(fluxo, amostras, taxa)
        fluxo.accept_waveform(taxa, np.zeros(int(taxa * SEGUNDOS_CAUDA_SILENCIO), dtype=np.float32))
        fluxo.input_finished()
        self._decodificarProntos(fluxo)
        texto = self._stt.get_result(fluxo)

        # input_finished é irreversível: a próxima fala começa um fluxo novo.
        self._fluxoOnline = None
        return texto

    def _executarSttParcial(self, amostras: np.ndarray, taxa: int, geracao: int) -> str:
        fluxo = self._obterFluxoDaFala(geracao)
        self._alimentarFluxo(fluxo, amostras, taxa)
        self._decodificarProntos(fluxo)
        return self._stt.get_result(fluxo)

    # ----- Auxiliares do fluxo streaming -----

    def _obterFluxoDaFala(self, geracao: int):
        """Devolve o fluxo da fala `geracao`, criando um novo quando a fala mudou (novo
        IniciarGravacao) ou quando ainda não há fluxo (primeiro decode após a carga do modelo)."""
        if self._fluxoOnline is None or geracao != self._geracaoFluxo:
            self._fluxoOnline = self._stt.create_stream()
            self._geracaoFluxo = geracao
            self._quadrosAlimentados = 0
        return self._fluxoOnline

    def _alimentarFluxo(self, fluxo, amostras: np.ndarray, taxa: int) -> None:
        """Entrega ao fluxo APENAS o trecho ainda não alimentado — é o que dá custo constante aos
        parciais."""
        # Guard clause: nenhum áudio novo desde o último decode.
        if amostras.size <= self._quadrosAlimentados:
            return
        fluxo.accept_waveform(taxa, amostras[self._quadrosAlimentados:])
        self._quadrosAlimentados = int(amostras.size)

    def _decodificarProntos(self, fluxo) -> None:
        while self._stt.is_ready(fluxo):
            self._stt.decode_stream(fluxo)
