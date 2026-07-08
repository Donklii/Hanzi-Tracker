# ----- Importações -----
from typing import Callable, Optional

import numpy as np

from motores.rapidocr import ModelosManifesto
from ocr.GerenciadorModelosModule import GerenciadorModelos
from ocr.ServicoOcrBase import ServicoOcrBase, MENSAGEM_CARREGANDO_MODELO
from principal import ConstantesModule


# ----- Serviço RapidOCR -----
class OcrService(ServicoOcrBase):
    """Motor padrão do Hanzi Tracker: RapidOCR sobre onnxruntime.

    CPU + WebGPU no mesmo binário (o sidecar é congelado com onnxruntime-webgpu; WebGPU
    cobre Nvidia/AMD/Intel nos dois SOs — D3D12 no Windows, Vulkan no Linux — com fallback
    automático para CPU). O EasyOCR e o Tesseract são MOTORES próprios (sidecars separados)
    — ver EasyOcrService/TesseractService.
    """

    def __init__(self) -> None:
        super().__init__()
        self._gerenciadorModelos = GerenciadorModelos(ModelosManifesto)

    def _inicializarOcr(self, aoProgredir: Optional[Callable[[str], None]] = None) -> None:
        """Inicializa o RapidOCR com o modelo/dispositivo selecionados."""
        modelo_solicitado = ConstantesModule.MODELO_OCR
        dispositivo = ConstantesModule.DISPOSITIVO_OCR

        self._liberarMotorAnterior()

        from rapidocr_onnxruntime import RapidOCR
        import onnxruntime as ort

        kwargs = {"intra_op_num_threads": ConstantesModule.THREADS_CPU_OCR}

        if ModelosManifesto.ehBaixavel(modelo_solicitado):
            # O download é feito pelo Go (escreve no AppData real, fora do sandbox da Store).
            # Aqui apenas LEMOS os arquivos; se não estiverem presentes, orientamos a baixar pela UI.
            if not self._gerenciadorModelos.modeloInstalado(modelo_solicitado):
                raise RuntimeError(
                    f"O modelo '{modelo_solicitado}' ainda não foi baixado. "
                    "Baixe-o em Configurações → OCR & Processamento → Gerenciar Modelos."
                )

            caminhos = self._gerenciadorModelos.caminhosModelo(modelo_solicitado)
            kwargs["det_model_path"] = caminhos["det"]
            kwargs["rec_model_path"] = caminhos["rec"]

        # Usa a aceleração de GPU somente se o provedor já estiver presente no onnxruntime instalado.
        # NÃO instalamos runtimes de GPU em tempo de execução: o onnxruntime já está carregado e sua
        # DLL fica travada (WinError 5), e no app distribuído (congelado) isso é impossível. Quando o
        # provedor não está disponível, caímos para CPU graciosamente — o OCR sempre funciona.
        # "directml"/"cuda" são valores LEGADOS de config (pré-WebGPU): valem como pedido de GPU.
        disponiveis = ort.get_available_providers()
        quer_gpu = dispositivo in ("webgpu", "directml", "cuda")
        usar_webgpu = quer_gpu and "WebGpuExecutionProvider" in disponiveis

        if quer_gpu and not usar_webgpu:
            # Aceleração solicitada mas indisponível nesta instalação → CPU (sem travar o app).
            self._emitirProgresso(
                aoProgredir,
                f"Aceleração por GPU ({dispositivo}) indisponível nesta versão; usando a CPU.",
            )

        self._emitirProgresso(aoProgredir, MENSAGEM_CARREGANDO_MODELO)

        # Monkey-patch temporário no InferenceSession: o rapidocr_onnxruntime não conhece o WebGPU
        # (não há kwarg use_webgpu, como há use_dml/use_cuda), então o provedor é antecipado aqui na
        # lista de providers de cada sessão. O WebGpuExecutionProvider não expõe device_id — ele usa o
        # adaptador de vídeo padrão do sistema, por isso não há seleção de GPU específica.
        original_init = ort.InferenceSession.__init__

        def patched_init(sess_self, *args, **patched_kwargs):
            # Contém a arena de memória do onnxruntime (reduz o pico de RAM/VRAM). O RapidOCR já passa
            # enable_cpu_mem_arena=False e arena_extend_strategy=kSameAsRequested; aqui só reforçamos
            # e desligamos o mem_pattern (planejamento que reserva memória antecipada).
            # O RapidOCR sempre cria a InferenceSession com sess_options=... (keyword), então basta lê-lo.
            sess_options = patched_kwargs.get("sess_options")
            if sess_options is not None:
                sess_options.enable_mem_pattern = False
                sess_options.enable_cpu_mem_arena = False

            providers = patched_kwargs.get("providers")
            if usar_webgpu and providers:
                novos_providers = [
                    p for p in providers
                    if (p[0] if isinstance(p, tuple) else p) != "WebGpuExecutionProvider"
                ]
                novos_providers.insert(0, "WebGpuExecutionProvider")
                patched_kwargs["providers"] = novos_providers

            original_init(sess_self, *args, **patched_kwargs)

        ort.InferenceSession.__init__ = patched_init
        try:
            self._ocr = RapidOCR(**kwargs)
        finally:
            # Restaura o construtor original do onnxruntime
            ort.InferenceSession.__init__ = original_init

    def _executarOcr(self, imagem: np.ndarray):
        resultado, _tempo = self._ocr(imagem)
        return resultado
