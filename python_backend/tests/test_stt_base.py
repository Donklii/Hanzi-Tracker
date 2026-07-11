# ----- Testes da base de STT (stt/ServicoSttBase.py) -----
# Exercitam o fluxo comum a todos os motores de reconhecimento de voz com um serviço FALSO (sem
# sherpa-onnx/modelos/microfone): template de transcrição, validação de entrada, mock de testes e o
# ciclo da gravação no modo de teste (sem dispositivo de áudio). O contrato HTTP em si
# (ServidorSttModule) é um wrapper fino sobre estes métodos — coberto pelo smoke test manual.

import numpy as np
import pytest

from principal import ConstantesModule
from stt.ServicoSttBase import ServicoSttBase


# ----- Serviço falso (sem modelo real) -----

class ServicoSttFalso(ServicoSttBase):
    """Devolve um texto fixo — determinístico e sem dependências pesadas."""

    def __init__(self):
        super().__init__()
        self.inicializacoes = 0
        self.ultimaTaxaRecebida = None

    def _inicializarStt(self, aoProgredir=None):
        self.inicializacoes += 1
        self._stt = object()  # qualquer não-None marca "modelo carregado"

    def _executarStt(self, amostras, taxa):
        self.ultimaTaxaRecebida = taxa
        return "  你好 "  # espaços de propósito: valida o strip do template


@pytest.fixture
def servicoFalso():
    # Watchdog desligado nos testes (não subir thread daemon); TESTANDO False para exercitar o
    # caminho real (init + transcrição do serviço falso), restaurando tudo ao final.
    testandoOriginal = ConstantesModule.TESTANDO
    timeoutOriginal = ConstantesModule.SEGUNDOS_OCIOSO_DESCARREGAR_STT
    ConstantesModule.TESTANDO = False
    ConstantesModule.SEGUNDOS_OCIOSO_DESCARREGAR_STT = 0
    yield ServicoSttFalso()
    ConstantesModule.TESTANDO = testandoOriginal
    ConstantesModule.SEGUNDOS_OCIOSO_DESCARREGAR_STT = timeoutOriginal


def _amostrasDeFala():
    return np.linspace(-0.5, 0.5, 1600, dtype=np.float32)


# ----- Template de transcrição -----

def testarTranscreverDevolveTextoLimpo(servicoFalso):
    texto = servicoFalso.Transcrever(_amostrasDeFala(), 16000)
    assert texto == "你好"


def testarTranscreverPropagaTaxaDeAmostragem(servicoFalso):
    servicoFalso.Transcrever(_amostrasDeFala(), 44100)
    assert servicoFalso.ultimaTaxaRecebida == 44100


def testarTranscreverRecusaAudioVazio(servicoFalso):
    with pytest.raises(ValueError):
        servicoFalso.Transcrever(np.zeros(0, dtype=np.float32), 16000)


def testarModeloCarregaUmaVezSo(servicoFalso):
    servicoFalso.Transcrever(_amostrasDeFala(), 16000)
    servicoFalso.Transcrever(_amostrasDeFala(), 16000)
    assert servicoFalso.inicializacoes == 1


def testarPrepararModeloCarregaSemTranscrever(servicoFalso):
    servicoFalso.PrepararModelo()
    assert servicoFalso.inicializacoes == 1
    # A transcrição seguinte reaproveita o modelo já carregado.
    servicoFalso.Transcrever(_amostrasDeFala(), 16000)
    assert servicoFalso.inicializacoes == 1


def testarFalhaDeInicializacaoNaoEhMemoizada(servicoFalso):
    # Primeira carga falha (ex.: rede caiu no download dos pesos)…
    def inicializarQuebrado(aoProgredir=None):
        raise RuntimeError("rede caiu")
    servicoFalso._inicializarStt = inicializarQuebrado
    with pytest.raises(RuntimeError):
        servicoFalso.Transcrever(_amostrasDeFala(), 16000)

    # …e o retry seguinte tenta de novo do zero (não fica travado no erro).
    servicoFalso._inicializarStt = lambda aoProgredir=None: setattr(servicoFalso, "_stt", object())
    assert servicoFalso.Transcrever(_amostrasDeFala(), 16000) == "你好"


def testarProgressoEhEmitidoNaCargaENaTranscricao(servicoFalso):
    mensagens = []
    servicoFalso.Transcrever(_amostrasDeFala(), 16000, aoProgredir=mensagens.append)
    assert len(mensagens) == 2  # carregando modelo + transcrevendo


# ----- Parciais em tempo real (TranscreverParcial) -----

def _simularGravacaoEmAndamento(servico, amostras):
    # TranscreverParcial só exige um fluxo de entrada não-None e blocos acumulados — em teste,
    # qualquer objeto marca "capturando" sem tocar em dispositivo de áudio nenhum.
    servico._fluxo_entrada = object()
    servico._blocosGravados = [amostras]


def testarParcialTranscreveDuranteAGravacao(servicoFalso):
    servicoFalso.PrepararModelo()
    _simularGravacaoEmAndamento(servicoFalso, _amostrasDeFala())
    assert servicoFalso.TranscreverParcial() == "你好"


def testarParcialSemGravacaoDevolveVazio(servicoFalso):
    servicoFalso.PrepararModelo()
    assert servicoFalso.TranscreverParcial() == ""


def testarParcialNaoCarregaOModelo(servicoFalso):
    # Parcial nunca paga a carga do modelo (potencial download de minutos) — isso é papel do
    # /preparar e do /parar; enquanto isso ele devolve vazio.
    _simularGravacaoEmAndamento(servicoFalso, _amostrasDeFala())
    assert servicoFalso.TranscreverParcial() == ""
    assert servicoFalso.inicializacoes == 0


def testarParcialComModeloOcupadoDevolveVazio(servicoFalso):
    servicoFalso.PrepararModelo()
    _simularGravacaoEmAndamento(servicoFalso, _amostrasDeFala())
    # Outra thread (um /parar em andamento) segura o lock do modelo: o parcial pula o tick.
    with servicoFalso._lock:
        assert servicoFalso.TranscreverParcial() == ""


def testarParcialNoModoDeTesteDevolveVazio():
    testandoOriginal = ConstantesModule.TESTANDO
    ConstantesModule.TESTANDO = True
    try:
        assert ServicoSttFalso().TranscreverParcial() == ""
    finally:
        ConstantesModule.TESTANDO = testandoOriginal


# ----- Mock de testes (TESTANDO=True): sem modelo nem microfone -----

def testarModoDeTesteNaoCarregaModelo():
    testandoOriginal = ConstantesModule.TESTANDO
    ConstantesModule.TESTANDO = True
    try:
        servico = ServicoSttFalso()
        assert servico.Transcrever(_amostrasDeFala(), 16000) == ""
        assert servico.inicializacoes == 0
    finally:
        ConstantesModule.TESTANDO = testandoOriginal


def testarCicloDeGravacaoNoModoDeTeste():
    # Em TESTANDO, iniciar/parar não tocam em dispositivo de áudio nenhum e devolvem 0 amostras.
    testandoOriginal = ConstantesModule.TESTANDO
    ConstantesModule.TESTANDO = True
    try:
        servico = ServicoSttFalso()
        servico.IniciarGravacao()
        amostras, taxa = servico.PararGravacao()
        assert amostras.size == 0
        assert taxa == ConstantesModule.TAXA_AMOSTRAGEM_STT
        servico.CancelarGravacao()  # idempotente, sem gravação em andamento
    finally:
        ConstantesModule.TESTANDO = testandoOriginal
