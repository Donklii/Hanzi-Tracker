# ----- Testes do motor de STT streaming (motores/zipformer_streaming) -----
# Exercitam a lógica INCREMENTAL do ZipformerStreamingSttService com um reconhecedor FALSO (sem
# sherpa-onnx/modelos/microfone): alimentação só do trecho novo entre parciais, recriação do fluxo
# quando uma fala nova começa (geração de gravação) e o encerramento do fluxo na transcrição final.
# O download/carga real dos pesos (sherpa_onnx/huggingface_hub) só acontece em runtime, dentro de
# _inicializarStt — fora do alcance (e da necessidade) destes testes.

import numpy as np

from motores.zipformer_streaming import ZipformerStreamingSttService as moduloZipformer
from motores.zipformer_streaming.ZipformerStreamingSttService import ZipformerStreamingSttService


# ----- Reconhecedor falso (espelha a superfície usada do sherpa_onnx.OnlineRecognizer) -----

class FluxoFalso:
    def __init__(self):
        self.quadrosRecebidos = 0
        self.decodesPendentes = 0
        self.finalizado = False

    def accept_waveform(self, taxa, amostras):
        self.quadrosRecebidos += int(amostras.size)
        self.decodesPendentes += 1

    def input_finished(self):
        self.finalizado = True
        self.decodesPendentes += 1


class ReconhecedorFalso:
    def __init__(self):
        self.fluxosCriados = []

    def create_stream(self):
        fluxo = FluxoFalso()
        self.fluxosCriados.append(fluxo)
        return fluxo

    def is_ready(self, fluxo):
        return fluxo.decodesPendentes > 0

    def decode_stream(self, fluxo):
        fluxo.decodesPendentes -= 1

    def get_result(self, fluxo):
        return f"quadros={fluxo.quadrosRecebidos}"


def _servicoComReconhecedorFalso():
    servico = ZipformerStreamingSttService()
    servico._stt = ReconhecedorFalso()
    return servico


def _amostras(tamanho: int) -> np.ndarray:
    return np.zeros(tamanho, dtype=np.float32)


# ----- Alimentação incremental (parciais) -----

def testarParcialAlimentaSoOTrechoNovo():
    servico = _servicoComReconhecedorFalso()
    servico._executarSttParcial(_amostras(100), 16000, geracao=1)
    servico._executarSttParcial(_amostras(250), 16000, geracao=1)

    fluxos = servico._stt.fluxosCriados
    assert len(fluxos) == 1  # mesma fala = mesmo fluxo persistente
    assert fluxos[0].quadrosRecebidos == 250  # 100 do primeiro tick + 150 de delta, sem repetição


def testarParcialSemAudioNovoNaoRealimenta():
    servico = _servicoComReconhecedorFalso()
    servico._executarSttParcial(_amostras(100), 16000, geracao=1)
    texto = servico._executarSttParcial(_amostras(100), 16000, geracao=1)

    assert servico._stt.fluxosCriados[0].quadrosRecebidos == 100
    assert texto == "quadros=100"  # ainda devolve o texto acumulado do fluxo


def testarFalaNovaComecaFluxoNovo():
    servico = _servicoComReconhecedorFalso()
    servico._executarSttParcial(_amostras(100), 16000, geracao=1)
    servico._executarSttParcial(_amostras(80), 16000, geracao=2)  # novo IniciarGravacao

    fluxos = servico._stt.fluxosCriados
    assert len(fluxos) == 2
    assert fluxos[1].quadrosRecebidos == 80  # a contagem de alimentação zerou com a fala nova


# ----- Transcrição final -----

def testarTranscricaoFinalCompletaEEncerraOFluxo():
    servico = _servicoComReconhecedorFalso()
    servico._geracaoGravacao = 3
    servico._executarSttParcial(_amostras(100), 16000, geracao=3)

    servico._executarStt(_amostras(160), 16000)

    fluxo = servico._stt.fluxosCriados[0]
    assert fluxo.finalizado
    cauda = int(16000 * moduloZipformer.SEGUNDOS_CAUDA_SILENCIO)
    assert fluxo.quadrosRecebidos == 160 + cauda  # delta de 60 + cauda de silêncio, sem repetição
    assert fluxo.decodesPendentes == 0  # tudo o que estava pronto foi decodificado
    assert servico._fluxoOnline is None  # input_finished é irreversível: próxima fala, fluxo novo


def testarTranscricaoFinalSemParcialAnterior():
    # Fluxo direto (nenhum parcial rodou): a final alimenta a gravação inteira de uma vez.
    servico = _servicoComReconhecedorFalso()
    servico._geracaoGravacao = 1

    texto = servico._executarStt(_amostras(200), 16000)

    cauda = int(16000 * moduloZipformer.SEGUNDOS_CAUDA_SILENCIO)
    assert texto == f"quadros={200 + cauda}"
