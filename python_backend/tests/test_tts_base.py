# ----- Testes da base de TTS (tts/ServicoTtsBase.py) -----
# Exercitam o fluxo comum a todos os motores de voz com um serviço FALSO (sem torch/modelos):
# codificação WAV, clipping, validação de entrada e o mock de testes. O contrato HTTP em si
# (ServidorTtsModule) é um wrapper fino sobre SintetizarFala — coberto pelo smoke test manual.

import io
import wave

import numpy as np
import pytest

from principal import ConstantesModule
from tts.ServicoTtsBase import ServicoTtsBase


# ----- Serviço falso (sem modelo real) -----

class ServicoTtsFalso(ServicoTtsBase):
    """Devolve uma rampa fixa de amostras — determinística e sem dependências pesadas."""

    def _inicializarTts(self, aoProgredir=None):
        self._tts = object()  # qualquer não-None marca "motor carregado"

    def _executarTts(self, texto):
        # Rampa que ESTOURA o intervalo -1..1 de propósito: valida o clipping do _codificarWav.
        return np.linspace(-1.5, 1.5, 480, dtype=np.float32), 24000


@pytest.fixture
def servicoFalso():
    # Watchdog desligado nos testes (não subir thread daemon); TESTANDO False para exercitar o
    # caminho real (init + síntese do serviço falso), restaurando tudo ao final.
    testandoOriginal = ConstantesModule.TESTANDO
    timeoutOriginal = ConstantesModule.SEGUNDOS_OCIOSO_DESCARREGAR_TTS
    ConstantesModule.TESTANDO = False
    ConstantesModule.SEGUNDOS_OCIOSO_DESCARREGAR_TTS = 0
    yield ServicoTtsFalso()
    ConstantesModule.TESTANDO = testandoOriginal
    ConstantesModule.SEGUNDOS_OCIOSO_DESCARREGAR_TTS = timeoutOriginal


def _decodificarWav(dados: bytes):
    with wave.open(io.BytesIO(dados), "rb") as arquivoWav:
        return arquivoWav.getparams(), arquivoWav.readframes(arquivoWav.getnframes())


# ----- Codificação WAV -----

def testarSintetizarFalaGeraWavPcm16Mono(servicoFalso):
    dados = servicoFalso.SintetizarFala("你好")

    parametros, quadros = _decodificarWav(dados)
    assert parametros.nchannels == 1
    assert parametros.sampwidth == 2  # PCM 16-bit
    assert parametros.framerate == 24000
    assert parametros.nframes == 480
    assert len(quadros) == 480 * 2


def testarClippingLimitaAmplitudeAoIntervaloDoInt16(servicoFalso):
    dados = servicoFalso.SintetizarFala("你好")

    _, quadros = _decodificarWav(dados)
    amostras = np.frombuffer(quadros, dtype=np.int16)
    # A rampa do serviço falso vai de -1.5 a 1.5: sem clipping, o int16 daria overflow (wrap).
    assert amostras.max() == 32767
    assert amostras.min() == -32767


# ----- Validação de entrada -----

def testarTextoVazioLevantaErro(servicoFalso):
    with pytest.raises(ValueError):
        servicoFalso.SintetizarFala("   ")


# ----- Mock de ambiente de testes -----

def testarModoTestandoDevolveWavSemCarregarModelo():
    testandoOriginal = ConstantesModule.TESTANDO
    ConstantesModule.TESTANDO = True
    try:
        # ServicoTtsBase puro (sem _inicializarTts implementado): só funciona porque o mock de
        # TESTANDO responde antes de qualquer inicialização.
        dados = ServicoTtsBase().SintetizarFala("你好")
    finally:
        ConstantesModule.TESTANDO = testandoOriginal

    parametros, _ = _decodificarWav(dados)
    assert parametros.nchannels == 1
    assert parametros.framerate == 24000
