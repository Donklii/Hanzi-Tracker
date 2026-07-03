# ----- Importações -----
import gc
import io
import logging
import threading
import time
import wave
from typing import Callable, Optional, Tuple

import numpy as np

from principal import ConstantesModule


# ----- Constantes do Módulo -----
MENSAGEM_CARREGANDO_MODELO = "Carregando modelo de voz (primeira vez)…"
MENSAGEM_SINTETIZANDO = "Sintetizando fala…"

_registrador = logging.getLogger(__name__)


# ----- Base dos serviços de TTS -----
class ServicoTtsBase:
    """Fluxo comum a TODO motor de TTS do Hanzi Tracker (Kokoro-82M, ChatTTS…).

    Centraliza o que não muda entre motores: mock de testes, inicialização preguiçosa
    (o modelo só é carregado na primeira síntese — e os pesos podem ser baixados do
    Hugging Face nesse momento), descarregamento por inatividade e a codificação WAV.

    Diferente do OCR, uma falha de inicialização NÃO é memoizada: a causa típica é
    transitória (queda de rede no meio do download dos pesos), e memoizar exigiria
    reiniciar o sidecar para tentar de novo. O custo do retry é aceitável porque o TTS
    só roda por ação explícita do usuário (hover/clique), não em varredura automática.

    Cada motor implementa apenas:
      - `_inicializarTts(aoProgredir)`: carrega o modelo e deixa `self._tts` pronto
        (qualquer objeto não-None que represente o motor carregado).
      - `_executarTts(texto)`: sintetiza e devolve `(amostras float32 mono -1..1, taxa_hz)`.
    """

    def __init__(self) -> None:
        self._tts = None
        # Serializa a síntese e o watchdog de inatividade: o servidor HTTP é single-thread, então
        # o único concorrente é a thread do watchdog. O lock garante que ela nunca descarregue o
        # motor (self._tts = None) no meio de uma síntese. _ultimo_uso é o monotonic() do fim da
        # última síntese — base da contagem de ociosidade.
        self._lock = threading.Lock()
        self._ultimo_uso = 0.0
        self._watchdog_iniciado = False

    # ----- Template principal -----

    def SintetizarFala(
        self, texto: str, aoProgredir: Optional[Callable[[str], None]] = None
    ) -> bytes:
        """Sintetiza `texto` em fala e devolve os bytes de um arquivo WAV (PCM 16-bit mono)."""
        # Guard clause: em ambiente de testes, devolve um WAV de silêncio curto (sem carregar modelo)
        if ConstantesModule.TESTANDO:
            return self._codificarWav(np.zeros(2400, dtype=np.float32), 24000)

        # Guard clause: nada a sintetizar
        if not texto or not texto.strip():
            raise ValueError("texto vazio")

        # Sobe (uma vez) o watchdog que descarrega o motor após inatividade.
        self._garantirWatchdogOcioso()

        # Todo o acesso a self._tts (init, síntese, descarte) fica sob o lock: impede que o watchdog
        # descarregue o motor no meio de uma síntese (self._tts viraria None e quebraria _executarTts).
        with self._lock:
            self._ultimo_uso = time.monotonic()

            if self._tts is None:
                self._emitirProgresso(aoProgredir, MENSAGEM_CARREGANDO_MODELO)
                self._limitarThreadsTorch()
                try:
                    self._inicializarTts(aoProgredir)
                except Exception:
                    # Estado pode ter ficado inconsistente (download/carga parcial): descarta e
                    # libera memória para o próximo retry.
                    self._tts = None
                    gc.collect()
                    raise

            self._emitirProgresso(aoProgredir, MENSAGEM_SINTETIZANDO)
            amostras, taxa = self._executarTts(texto.strip())

            # Marca o fim do uso: a ociosidade é contada a partir daqui.
            self._ultimo_uso = time.monotonic()

        return self._codificarWav(amostras, taxa)

    # ----- Pontos de extensão (cada motor implementa) -----

    def _inicializarTts(self, aoProgredir: Optional[Callable[[str], None]] = None) -> None:
        raise NotImplementedError

    def _executarTts(self, texto: str) -> Tuple[np.ndarray, int]:
        raise NotImplementedError

    # ----- Auxiliares compartilhados -----

    def _limitarThreadsTorch(self) -> None:
        """Limita as threads do torch para não saturar a CPU (preserva o FPS de jogos em foco),
        espelhando o que o OCR faz com o onnxruntime. No-op se o motor não usa torch."""
        try:
            import torch
            torch.set_num_threads(max(1, ConstantesModule.THREADS_CPU_TTS))
        except ImportError:
            return

    def _codificarWav(self, amostras: np.ndarray, taxa: int) -> bytes:
        """Converte amostras float32 mono (-1..1) em bytes de um WAV PCM 16-bit."""
        plano = np.asarray(amostras, dtype=np.float32).reshape(-1)
        # Clipping defensivo: modelos ocasionalmente estouram levemente o intervalo -1..1, e o
        # overflow do int16 viraria um estalo alto.
        inteiros = (np.clip(plano, -1.0, 1.0) * 32767.0).astype(np.int16)

        buffer = io.BytesIO()
        with wave.open(buffer, "wb") as arquivoWav:
            arquivoWav.setnchannels(1)
            arquivoWav.setsampwidth(2)  # 16-bit
            arquivoWav.setframerate(taxa)
            arquivoWav.writeframes(inteiros.tobytes())
        return buffer.getvalue()

    def _emitirProgresso(
        self, aoProgredir: Optional[Callable[[str], None]], mensagem: str
    ) -> None:
        # Guard clause: sem callback de progresso registrado
        if aoProgredir is None:
            return
        aoProgredir(mensagem)

    # ----- Descarregamento por inatividade (libera RAM quando o app fica ocioso) -----

    def _garantirWatchdogOcioso(self) -> None:
        """Sobe (uma única vez) a thread que descarrega o motor ocioso. Idempotente; no-op em testes
        ou com a feature desligada (SEGUNDOS_OCIOSO_DESCARREGAR_TTS <= 0)."""
        # Guard clauses: em testes não subimos threads; timeout 0 desliga a feature.
        if ConstantesModule.TESTANDO:
            return
        if ConstantesModule.SEGUNDOS_OCIOSO_DESCARREGAR_TTS <= 0:
            return

        with self._lock:
            if self._watchdog_iniciado:
                return
            self._watchdog_iniciado = True

        thread = threading.Thread(target=self._lacoWatchdogOcioso, name="tts-watchdog-ocioso", daemon=True)
        thread.start()

    def _lacoWatchdogOcioso(self) -> None:
        """Laço da thread daemon: acorda periodicamente e descarrega o motor se ficou ocioso além do
        limite. O daemon morre junto com o processo — não precisa de sinal de parada."""
        timeout = ConstantesModule.SEGUNDOS_OCIOSO_DESCARREGAR_TTS
        intervalo = min(timeout, 15)  # granularidade da checagem (no máx. 15s)

        while True:
            time.sleep(intervalo)
            with self._lock:
                # Guard clause: nada carregado (ou já descarregado) — nada a fazer.
                if self._tts is None:
                    continue
                if time.monotonic() - self._ultimo_uso >= timeout:
                    self._descarregarPorInatividade()

    def _descarregarPorInatividade(self) -> None:
        """Solta o motor após inatividade, liberando RAM. Recarrega sob demanda na próxima síntese
        (custo de ~10-30s de carga do torch — por isso o timeout do TTS é maior que o do OCR).
        DEVE ser chamado já com self._lock adquirido (feito no _lacoWatchdogOcioso)."""
        self._tts = None
        gc.collect()
        _registrador.info(
            "Motor de TTS descarregado após %ds ocioso; recarrega na próxima síntese.",
            ConstantesModule.SEGUNDOS_OCIOSO_DESCARREGAR_TTS,
        )
