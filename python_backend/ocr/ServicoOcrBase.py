# ----- Importações -----
from typing import Any, Callable, List, Optional, Tuple

import numpy as np

from ocr.ModelosOcrModule import DeteccaoOcr
from principal import ConstantesModule


# ----- Constantes do Módulo -----
MENSAGEM_CARREGANDO_MODELO = "Carregando modelo de OCR (primeira vez)…"
MENSAGEM_RECONHECENDO = "Reconhecendo texto (OCR)…"


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

        config_atual = self._configAtual()

        # Guard clause: se a última inicialização com ESTA mesma config falhou, não tenta de novo
        # (evita repetir operações pesadas e 500s a cada varredura). Mude a config para reativar.
        if self._falha_erro is not None and self._falha_config == config_atual:
            raise self._falha_erro

        # Inicializa ou reinicializa o motor caso a configuração mude
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
            import gc
            gc.collect()
            raise RuntimeError(self._mensagemErroInferencia(erro)) from erro

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
        import gc
        self._ocr = None
        gc.collect()

    def _mensagemErroInferencia(self, erro: Exception) -> str:
        """Traduz erros de baixo nível da inferência em mensagens acionáveis para o usuário."""
        # UnicodeDecodeError aqui é quase sempre uma mensagem nativa do DirectML (em codepage do
        # Windows) que o onnxruntime tenta ler como UTF-8 — tipicamente "memória de vídeo insuficiente".
        if isinstance(erro, UnicodeDecodeError):
            return (
                "Falha na aceleração por GPU (DirectML), normalmente por falta de memória de vídeo. "
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
