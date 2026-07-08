# ----- Importações -----
import gc
import logging
import threading
import time
from typing import Any, Callable, List, Optional, Tuple

import numpy as np

from ocr.ModelosOcrModule import DeteccaoOcr
from principal import ConstantesModule


# ----- Constantes do Módulo -----
MENSAGEM_CARREGANDO_MODELO = "Carregando modelo de OCR (primeira vez)…"
MENSAGEM_RECONHECENDO = "Reconhecendo texto (OCR)…"

_registrador = logging.getLogger(__name__)


# ----- Base dos serviços de OCR -----
class ServicoOcrBase:
    """Fluxo comum a TODO motor de OCR do Hanzi Tracker (RapidOCR, EasyOCR, Tesseract…).

    Centraliza o que não muda entre motores: mock de testes, memoização de falha de
    inicialização, reinicialização quando a configuração muda, downscale da captura,
    filtro por confiança e o mapeamento das caixas de volta à escala original.

    Cada motor implementa apenas:
      - `_inicializarOcr(aoProgredir)`: carrega/configura o motor conforme ConstantesModule
        e deixa `self._ocr` pronto (qualquer objeto não-None que represente o motor carregado).
      - `_executarOcr(imagem)`: roda a inferência e devolve a lista bruta
        `[(caixa de 4 cantos, texto, confianca 0..1)]` — o formato nativo do RapidOCR/EasyOCR;
        o Tesseract converte o TSV dele para este mesmo formato.
    """

    def __init__(self) -> None:
        self._ocr = None
        self._config_carregada = None
        # Memoiza a última configuração cuja inicialização falhou, para não repetir
        # operações pesadas (e 500s) a cada varredura automática enquanto a config não muda.
        self._falha_config = None
        self._falha_erro = None
        # Serializa a inferência e o watchdog de inatividade: o servidor HTTP é single-thread, então
        # o único concorrente é a thread do watchdog. O lock garante que ela nunca descarregue o motor
        # (self._ocr = None) no meio de uma inferência. _ultimo_uso é o monotonic() do fim do último
        # scan — base da contagem de ociosidade.
        self._lock = threading.Lock()
        self._ultimo_uso = 0.0
        self._watchdog_iniciado = False

    # ----- Template principal -----

    def ReconhecerCaracteresChineses(
        self, imagem: np.ndarray, aoProgredir: Optional[Callable[[str], None]] = None
    ) -> List[DeteccaoOcr]:
        """Executa OCR na imagem e retorna as detecções (texto, confiança e caixa) acima do limiar."""
        # Guard clause: em ambiente de testes, retorna dados mockados
        if ConstantesModule.TESTANDO:
            return [
                DeteccaoOcr("中国", 0.95, (0.0, 0.0, 40.0, 20.0)),
                DeteccaoOcr("你好", 0.98, (40.0, 0.0, 80.0, 20.0)),
            ]

        # Sobe (uma vez) o watchdog que descarrega o motor após inatividade.
        self._garantirWatchdogOcioso()

        # Todo o acesso a self._ocr (init, inferência, descarte) fica sob o lock: impede que o watchdog
        # descarregue o motor no meio de uma inferência (self._ocr viraria None e quebraria _executarOcr).
        with self._lock:
            self._ultimo_uso = time.monotonic()
            config_atual = self._configAtual()

            # Guard clause: se a última inicialização com ESTA mesma config falhou, não tenta de novo
            # (evita repetir operações pesadas e 500s a cada varredura). Mude a config para reativar.
            if self._falha_erro is not None and self._falha_config == config_atual:
                raise self._falha_erro

            # Inicializa ou reinicializa o motor caso a configuração mude (ou tenha sido descarregado).
            if self._ocr is None or self._config_carregada != config_atual:
                try:
                    self._inicializarOcr(aoProgredir)
                except Exception as erro:
                    self._falha_config = config_atual
                    self._falha_erro = erro
                    raise

                # Sucesso: registra a config carregada e limpa qualquer falha memorizada
                self._config_carregada = config_atual
                self._falha_config = None
                self._falha_erro = None

            # Trata o downscale da imagem para processamento mais rápido
            altura, largura = imagem.shape[:2]
            lado_maior = max(altura, largura)
            limite = ConstantesModule.LIMITE_LADO_MAIOR_OCR

            escala = 1.0
            imagem_processar = imagem
            if limite > 0 and lado_maior > limite:
                escala = limite / lado_maior
                nova_largura = int(largura * escala)
                nova_altura = int(altura * escala)
                import cv2
                imagem_processar = cv2.resize(imagem, (nova_largura, nova_altura), interpolation=cv2.INTER_AREA)

            self._emitirProgresso(aoProgredir, MENSAGEM_RECONHECENDO)

            try:
                resultado = self._executarOcr(imagem_processar)
            except (MemoryError, UnicodeDecodeError) as erro:
                # Estado pode ter ficado inconsistente: descarta o motor e libera memória para o próximo retry.
                self._ocr = None
                gc.collect()
                raise RuntimeError(self._mensagemErroInferencia(erro)) from erro

            # Marca o fim do uso: a ociosidade é contada a partir daqui.
            self._ultimo_uso = time.monotonic()

        # Guard clause: nada detectado na imagem
        if not resultado:
            return []

        return self._extrairTextosConfiaveis(resultado, escala)

    # ----- Pontos de extensão (cada motor implementa) -----

    def _inicializarOcr(self, aoProgredir: Optional[Callable[[str], None]] = None) -> None:
        raise NotImplementedError

    def _executarOcr(self, imagem: np.ndarray) -> Any:
        raise NotImplementedError

    # ----- Auxiliares compartilhados -----

    def _configAtual(self) -> Tuple:
        """Configuração que exige recarregar o motor quando muda (vinda dos headers do Go)."""
        return (
            ConstantesModule.MODELO_OCR,
            ConstantesModule.DISPOSITIVO_OCR,
            ConstantesModule.HARDWARE_SELECIONADO,
            ConstantesModule.THREADS_CPU_OCR,
        )

    def _liberarMotorAnterior(self) -> None:
        """Solta o motor carregado antes de trocar de configuração.

        Cada motor segura centenas de MB (sessões onnxruntime/torch); sem soltar, trocar de
        config acumula memória e leva a MemoryError. O gc.collect força a coleta da instância
        antiga antes de alocar a nova.
        """
        self._ocr = None
        gc.collect()

    # ----- Descarregamento por inatividade (libera RAM/VRAM quando o app fica ocioso) -----

    def _garantirWatchdogOcioso(self) -> None:
        """Sobe (uma única vez) a thread que descarrega o motor ocioso. Idempotente; no-op em testes
        ou com a feature desligada (SEGUNDOS_OCIOSO_DESCARREGAR_OCR <= 0)."""
        # Guard clauses: em testes não subimos threads; timeout 0 desliga a feature.
        if ConstantesModule.TESTANDO:
            return
        if ConstantesModule.SEGUNDOS_OCIOSO_DESCARREGAR_OCR <= 0:
            return

        with self._lock:
            if self._watchdog_iniciado:
                return
            self._watchdog_iniciado = True

        thread = threading.Thread(target=self._lacoWatchdogOcioso, name="ocr-watchdog-ocioso", daemon=True)
        thread.start()

    def _lacoWatchdogOcioso(self) -> None:
        """Laço da thread daemon: acorda periodicamente e descarrega o motor se ficou ocioso além do
        limite. O daemon morre junto com o processo — não precisa de sinal de parada."""
        timeout = ConstantesModule.SEGUNDOS_OCIOSO_DESCARREGAR_OCR
        intervalo = min(timeout, 15)  # granularidade da checagem (no máx. 15s)

        while True:
            time.sleep(intervalo)
            with self._lock:
                # Guard clause: nada carregado (ou já descarregado) — nada a fazer.
                if self._ocr is None:
                    continue
                if time.monotonic() - self._ultimo_uso >= timeout:
                    self._descarregarPorInatividade()

    def _descarregarPorInatividade(self) -> None:
        """Solta o motor após inatividade, liberando RAM (e VRAM na aceleração por GPU). Recarrega sob demanda no
        próximo scan. DEVE ser chamado já com self._lock adquirido (feito no _lacoWatchdogOcioso)."""
        self._ocr = None
        self._config_carregada = None
        gc.collect()
        _registrador.info(
            "Motor de OCR descarregado após %ds ocioso; recarrega no próximo scan.",
            ConstantesModule.SEGUNDOS_OCIOSO_DESCARREGAR_OCR,
        )

    def _mensagemErroInferencia(self, erro: Exception) -> str:
        """Traduz erros de baixo nível da inferência em mensagens acionáveis para o usuário."""
        # UnicodeDecodeError aqui é quase sempre uma mensagem nativa do runtime de GPU (em codepage do
        # Windows) que o onnxruntime tenta ler como UTF-8 — tipicamente "memória de vídeo insuficiente".
        if isinstance(erro, UnicodeDecodeError):
            return (
                "Falha na aceleração por GPU, normalmente por falta de memória de vídeo. "
                "Troque o dispositivo para CPU ou reduza a 'Resolução de Captura' nas configurações."
            )

        return (
            "Memória insuficiente para rodar o OCR nesta resolução. "
            "Reduza a 'Resolução de Captura' nas configurações, use a CPU em vez da GPU, "
            "ou feche outros programas para liberar memória."
        )

    def _emitirProgresso(
        self, aoProgredir: Optional[Callable[[str], None]], mensagem: str
    ) -> None:
        # Guard clause: sem callback de progresso registrado
        if aoProgredir is None:
            return
        aoProgredir(mensagem)

    def _extrairTextosConfiaveis(self, resultado: Any, escala: float = 1.0) -> List[DeteccaoOcr]:
        """Filtra as detecções pelo limiar de confiança, mapeando de volta na escala original."""
        deteccoes: List[DeteccaoOcr] = []

        for box, texto, score in resultado:
            confianca = float(score)

            # Guard clause: descarta textos vazios
            if not texto:
                continue

            # Guard clause: descarta detecções com baixa confiança
            if confianca < ConstantesModule.CONFIANCA_MINIMA_OCR:
                continue

            caixa = self._caixaDoPoligono(box)
            if escala != 1.0:
                caixa = (caixa[0] / escala, caixa[1] / escala, caixa[2] / escala, caixa[3] / escala)

            deteccoes.append(DeteccaoOcr(texto, confianca, caixa))

        return deteccoes

    def _caixaDoPoligono(self, box: Any) -> Tuple[float, float, float, float]:
        """Converte o polígono de 4 cantos na caixa (x0, y0, x1, y1)."""
        xs = [float(ponto[0]) for ponto in box]
        ys = [float(ponto[1]) for ponto in box]
        return (min(xs), min(ys), max(xs), max(ys))
