# ----- Importações -----
import gc
import logging
import threading
import time
from typing import Callable, Optional, Tuple

import numpy as np

from principal import ConstantesModule


# ----- Constantes do Módulo -----
MENSAGEM_CARREGANDO_MODELO = "Carregando modelo de reconhecimento de voz (primeira vez)…"
MENSAGEM_TRANSCREVENDO = "Transcrevendo fala…"

_registrador = logging.getLogger(__name__)


# ----- Base dos serviços de STT -----
class ServicoSttBase:
    """Fluxo comum a TODO motor de STT do Hanzi Tracker (Paraformer-ZH…).

    Centraliza o que não muda entre motores: a CAPTURA do microfone (push-to-talk via
    sounddevice — a webview do Wails no Linux não tem getUserMedia, então a gravação é
    nativa, aqui no sidecar), mock de testes, inicialização preguiçosa (o modelo só é
    carregado na primeira transcrição — e os pesos podem ser baixados do Hugging Face
    nesse momento) e o descarregamento por inatividade.

    Como no TTS, uma falha de inicialização NÃO é memoizada: a causa típica é transitória
    (queda de rede no meio do download dos pesos) e o STT só roda por ação explícita do
    usuário (segurar o botão do microfone).

    Cada motor implementa apenas:
      - `_inicializarStt(aoProgredir)`: carrega o modelo e deixa `self._stt` pronto
        (qualquer objeto não-None que represente o motor carregado).
      - `_executarStt(amostras, taxa)`: transcreve amostras float32 mono (-1..1) e
        devolve o texto reconhecido.
      - `_executarSttParcial(amostras, taxa, geracao)` (opcional): transcrição PARCIAL do áudio
        acumulado, para os parciais em tempo real. O default re-decodifica tudo com
        `_executarStt` (correto para motores offline); motores genuinamente streaming
        sobrescrevem para alimentar só o trecho novo.
    """

    def __init__(self) -> None:
        self._stt = None
        # Lock do MODELO: serializa transcrição, pré-aquecimento, parciais e o watchdog de
        # inatividade (o servidor HTTP é multi-thread — um /parcial pode chegar durante o /parar).
        self._lock = threading.Lock()
        self._ultimo_uso = 0.0
        self._watchdog_iniciado = False

        # Lock da GRAVAÇÃO: separado do lock do modelo porque o callback de áudio do PortAudio
        # também o adquire — e o stop() do fluxo ESPERA o callback terminar, então parar a captura
        # segurando o mesmo lock do callback seria deadlock. O fluxo é sempre encerrado FORA do lock.
        self._lockGravacao = threading.Lock()
        self._fluxo_entrada = None
        self._blocosGravados: list = []
        self._taxaGravacao = ConstantesModule.TAXA_AMOSTRAGEM_STT
        self._inicioGravacao = 0.0
        # Geração da gravação: incrementa a cada IniciarGravacao. Motores streaming a usam para
        # saber que uma fala NOVA começou e descartar o fluxo de decodificação da anterior.
        self._geracaoGravacao = 0

    # ----- Ciclo da gravação (push-to-talk) -----

    def IniciarGravacao(self) -> None:
        """Começa a capturar o microfone padrão. Uma gravação já em andamento é DESCARTADA e a
        captura recomeça do zero — deixa o contrato tolerante a um `parar` perdido pelo frontend."""
        self._soltarFluxoEntrada()

        # Guard clause: em testes não há dispositivo de áudio — o mock grava "nada".
        if ConstantesModule.TESTANDO:
            with self._lockGravacao:
                self._blocosGravados = []
                self._inicioGravacao = time.monotonic()
                self._geracaoGravacao += 1
            return

        fluxo = self._abrirFluxoEntrada()
        with self._lockGravacao:
            self._blocosGravados = []
            self._inicioGravacao = time.monotonic()
            self._geracaoGravacao += 1
            self._fluxo_entrada = fluxo
        fluxo.start()

    def PararGravacao(self) -> Tuple[np.ndarray, int]:
        """Para a captura e devolve (amostras float32 mono -1..1, taxa_hz) do que foi gravado."""
        self._soltarFluxoEntrada()
        with self._lockGravacao:
            blocos, self._blocosGravados = self._blocosGravados, []
            taxa = self._taxaGravacao
        if not blocos:
            return np.zeros(0, dtype=np.float32), taxa
        return np.concatenate(blocos).reshape(-1).astype(np.float32), taxa

    def CancelarGravacao(self) -> None:
        """Descarta a gravação em andamento (soltar o botão fora da área, troca de questão…)."""
        self._soltarFluxoEntrada()
        with self._lockGravacao:
            self._blocosGravados = []

    def _abrirFluxoEntrada(self):
        """Abre o InputStream do sounddevice na taxa alvo (16 kHz, a nativa do Paraformer); se o
        dispositivo não a suportar, cai na taxa padrão dele — o motor resampleia na transcrição."""
        import sounddevice

        def aoReceberAudio(bloco, quadros, tempo, status):  # callback da thread de áudio do PortAudio
            if status:
                _registrador.warning("Captura de áudio com status: %s", status)
            with self._lockGravacao:
                # Guard clause: teto de duração — protege contra um "parar" que nunca chega.
                if time.monotonic() - self._inicioGravacao > ConstantesModule.SEGUNDOS_MAXIMOS_GRAVACAO_STT:
                    return
                self._blocosGravados.append(bloco.copy())

        try:
            with self._lockGravacao:
                self._taxaGravacao = ConstantesModule.TAXA_AMOSTRAGEM_STT
            return sounddevice.InputStream(
                samplerate=ConstantesModule.TAXA_AMOSTRAGEM_STT, channels=1, dtype="float32",
                callback=aoReceberAudio,
            )
        except Exception:
            # Dispositivo sem suporte à taxa alvo: usa a taxa padrão do microfone.
            info = sounddevice.query_devices(kind="input")
            taxaPadrao = int(info["default_samplerate"])
            with self._lockGravacao:
                self._taxaGravacao = taxaPadrao
            return sounddevice.InputStream(
                samplerate=taxaPadrao, channels=1, dtype="float32", callback=aoReceberAudio
            )

    def _soltarFluxoEntrada(self) -> None:
        """Desanexa e encerra o fluxo de captura atual (se houver). O stop() acontece FORA do
        _lockGravacao — ele espera o callback de áudio terminar, e o callback adquire esse lock."""
        with self._lockGravacao:
            fluxo, self._fluxo_entrada = self._fluxo_entrada, None
        # Guard clause: nenhuma captura aberta.
        if fluxo is None:
            return
        try:
            fluxo.stop()
            fluxo.close()
        except Exception as erro:
            _registrador.warning("Falha ao encerrar a captura de áudio: %s", erro)

    # ----- Template principal (transcrição) -----

    def Transcrever(
        self,
        amostras: np.ndarray,
        taxa: int,
        aoProgredir: Optional[Callable[[str], None]] = None,
    ) -> str:
        """Transcreve amostras float32 mono (-1..1) em texto (hanzi)."""
        # Guard clause: em ambiente de testes, devolve texto fixo (sem carregar modelo).
        if ConstantesModule.TESTANDO:
            return ""

        # Guard clause: nada gravado (botão solto cedo demais, microfone mudo).
        if amostras.size == 0:
            raise ValueError("nenhum áudio capturado — segure o botão enquanto fala")

        # Sobe (uma vez) o watchdog que descarrega o modelo após inatividade.
        self._garantirWatchdogOcioso()

        # Todo o acesso a self._stt (init, transcrição, descarte) fica sob o lock: impede que o
        # watchdog descarregue o modelo no meio de uma transcrição.
        with self._lock:
            self._ultimo_uso = time.monotonic()

            if self._stt is None:
                self._emitirProgresso(aoProgredir, MENSAGEM_CARREGANDO_MODELO)
                try:
                    self._inicializarStt(aoProgredir)
                except Exception:
                    # Estado pode ter ficado inconsistente (download/carga parcial): descarta e
                    # libera memória para o próximo retry.
                    self._stt = None
                    gc.collect()
                    raise

            self._emitirProgresso(aoProgredir, MENSAGEM_TRANSCREVENDO)
            texto = self._executarStt(amostras, taxa)

            # Marca o fim do uso: a ociosidade é contada a partir daqui.
            self._ultimo_uso = time.monotonic()

        return (texto or "").strip()

    def TranscreverParcial(self) -> str:
        """Transcreve o áudio acumulado até agora SEM parar a captura (parciais em tempo real,
        endpoint /api/stt/parcial). Best-effort de propósito: devolve "" sempre que um parcial não
        puder sair AGORA (nada gravado, modelo ocupado ou ainda não carregado) — o tick seguinte
        do polling tenta de novo, e o /parar continua sendo a transcrição de verdade."""
        # Guard clause: em ambiente de testes não há captura nem modelo.
        if ConstantesModule.TESTANDO:
            return ""

        with self._lockGravacao:
            gravando = self._fluxo_entrada is not None
            blocos = list(self._blocosGravados)
            taxa = self._taxaGravacao
            geracao = self._geracaoGravacao
        # Guard clause: sem gravação em andamento (ou ainda sem áudio) não há o que transcrever.
        if not gravando or not blocos:
            return ""

        amostras = np.concatenate(blocos).reshape(-1).astype(np.float32)

        # Não-bloqueante: se o modelo está ocupado (carga, /parar, outro parcial), pula o tick em
        # vez de enfileirar — parcial atrasado não tem valor.
        if not self._lock.acquire(blocking=False):
            return ""
        try:
            # Guard clause: parcial NUNCA dispara a carga do modelo (pode ser um download de
            # minutos) — isso é papel do /preparar e do /parar.
            if self._stt is None:
                return ""
            self._ultimo_uso = time.monotonic()
            texto = self._executarSttParcial(amostras, taxa, geracao)
            self._ultimo_uso = time.monotonic()
            return (texto or "").strip()
        finally:
            self._lock.release()

    def PrepararModelo(self, aoProgredir: Optional[Callable[[str], None]] = None) -> None:
        """Carrega o modelo sem transcrever nada (pré-aquecimento ao entrar na revisão de
        pronúncia): a primeira transcrição real sai sem a espera do download/carga dos pesos."""
        # Guard clause: em testes não há modelo a carregar.
        if ConstantesModule.TESTANDO:
            return

        self._garantirWatchdogOcioso()
        with self._lock:
            self._ultimo_uso = time.monotonic()
            if self._stt is not None:
                return
            self._emitirProgresso(aoProgredir, MENSAGEM_CARREGANDO_MODELO)
            try:
                self._inicializarStt(aoProgredir)
            except Exception:
                self._stt = None
                gc.collect()
                raise

    # ----- Pontos de extensão (cada motor implementa) -----

    def _inicializarStt(self, aoProgredir: Optional[Callable[[str], None]] = None) -> None:
        raise NotImplementedError

    def _executarStt(self, amostras: np.ndarray, taxa: int) -> str:
        raise NotImplementedError

    def _executarSttParcial(self, amostras: np.ndarray, taxa: int, geracao: int) -> str:
        """Transcrição parcial do áudio acumulado. O default re-decodifica TUDO do zero com
        `_executarStt` — correto (e barato: falas push-to-talk têm teto de 30s) para motores
        offline como o Paraformer. Motores streaming sobrescrevem para alimentar só o trecho novo
        ao fluxo persistente, usando `geracao` para detectar o começo de uma fala nova."""
        return self._executarStt(amostras, taxa)

    # ----- Auxiliares compartilhados -----

    def _emitirProgresso(
        self, aoProgredir: Optional[Callable[[str], None]], mensagem: str
    ) -> None:
        # Guard clause: sem callback de progresso registrado
        if aoProgredir is None:
            return
        aoProgredir(mensagem)

    # ----- Descarregamento por inatividade (libera RAM quando o app fica ocioso) -----

    def _garantirWatchdogOcioso(self) -> None:
        """Sobe (uma única vez) a thread que descarrega o modelo ocioso. Idempotente; no-op em
        testes ou com a feature desligada (SEGUNDOS_OCIOSO_DESCARREGAR_STT <= 0)."""
        # Guard clauses: em testes não subimos threads; timeout 0 desliga a feature.
        if ConstantesModule.TESTANDO:
            return
        if ConstantesModule.SEGUNDOS_OCIOSO_DESCARREGAR_STT <= 0:
            return

        with self._lock:
            if self._watchdog_iniciado:
                return
            self._watchdog_iniciado = True

        thread = threading.Thread(target=self._lacoWatchdogOcioso, name="stt-watchdog-ocioso", daemon=True)
        thread.start()

    def _lacoWatchdogOcioso(self) -> None:
        """Laço da thread daemon: acorda periodicamente e descarrega o modelo se ficou ocioso além
        do limite. O daemon morre junto com o processo — não precisa de sinal de parada."""
        timeout = ConstantesModule.SEGUNDOS_OCIOSO_DESCARREGAR_STT
        intervalo = min(timeout, 15)  # granularidade da checagem (no máx. 15s)

        while True:
            time.sleep(intervalo)
            with self._lock:
                # Guard clause: nada carregado (ou já descarregado) — nada a fazer.
                if self._stt is None:
                    continue
                if time.monotonic() - self._ultimo_uso >= timeout:
                    self._descarregarPorInatividade()

    def _descarregarPorInatividade(self) -> None:
        """Solta o modelo após inatividade, liberando RAM. Recarrega sob demanda na próxima
        transcrição. DEVE ser chamado já com self._lock adquirido (feito no _lacoWatchdogOcioso)."""
        self._stt = None
        gc.collect()
        _registrador.info(
            "Motor de STT descarregado após %ds ocioso; recarrega na próxima transcrição.",
            ConstantesModule.SEGUNDOS_OCIOSO_DESCARREGAR_STT,
        )
