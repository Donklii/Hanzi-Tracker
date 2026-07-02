# ----- Importações -----
from typing import Callable, List, Optional, Tuple

import numpy as np

from motores.rapidocr import ModelosManifesto
from ocr.GerenciadorModelosModule import GerenciadorModelos
from ocr.ServicoOcrBase import ServicoOcrBase, MENSAGEM_CARREGANDO_MODELO
from principal import ConstantesModule


# ----- Serviço RapidOCR -----
class OcrService(ServicoOcrBase):
    """Motor padrão do Hanzi Tracker: RapidOCR sobre onnxruntime.

    CPU + DirectML no mesmo binário (o sidecar é congelado com onnxruntime-directml; DirectML
    cobre Nvidia/AMD/Intel no Windows, com fallback automático para CPU). O EasyOCR e o
    Tesseract são MOTORES próprios (sidecars separados) — ver EasyOcrService/TesseractService.
    """

    def __init__(self) -> None:
        super().__init__()
        self._gerenciadorModelos = GerenciadorModelos(ModelosManifesto)

    def _detectarHardware(self) -> Tuple[str, List[str]]:
        """Detecta o processador e as placas de vídeo (GPUs) instaladas no sistema."""
        import platform
        import subprocess

        cpu = "CPU"
        gpus: List[str] = []

        if platform.system() == "Windows":
            try:
                import winreg
                chave = winreg.OpenKey(winreg.HKEY_LOCAL_MACHINE, r"HARDWARE\DESCRIPTION\System\CentralProcessor\0")
                cpu, _ = winreg.QueryValueEx(chave, "ProcessorNameString")
                winreg.CloseKey(chave)
            except Exception:
                cpu = platform.processor() or "CPU"
        else:
            cpu = platform.processor() or "CPU"

        cpu = cpu.strip()

        if platform.system() == "Windows":
            try:
                # Consulta Win32_VideoController via PowerShell de forma silenciosa
                resultado = subprocess.run(
                    ["powershell", "-NoProfile", "-Command", "Get-CimInstance Win32_VideoController | Select-Object -ExpandProperty Name"],
                    stdout=subprocess.PIPE,
                    stderr=subprocess.PIPE,
                    text=True,
                    creationflags=0x08000000 # CREATE_NO_WINDOW
                )
                if resultado.returncode == 0:
                    linhas = [l.strip() for l in resultado.stdout.splitlines() if l.strip()]
                    gpus_filtradas = [g for g in linhas if not any(x in g.lower() for x in ["virtual", "parsec", "mirror", "remote"])]
                    gpus_usar = gpus_filtradas if gpus_filtradas else linhas
                    for g in gpus_usar:
                        if g not in gpus:
                            gpus.append(g)
            except Exception:
                pass

        return cpu, gpus

    def _inicializarOcr(self, aoProgredir: Optional[Callable[[str], None]] = None) -> None:
        """Inicializa o RapidOCR com o modelo/dispositivo/hardware selecionados."""
        modelo_solicitado = ConstantesModule.MODELO_OCR
        dispositivo = ConstantesModule.DISPOSITIVO_OCR
        hardware = ConstantesModule.HARDWARE_SELECIONADO

        self._liberarMotorAnterior()

        # Determina os índices de GPU com base na detecção
        cpu_nome, gpus = self._detectarHardware()

        try:
            dml_index = gpus.index(hardware)
        except ValueError:
            dml_index = 0

        nvidia_gpus = [g for g in gpus if "nvidia" in g.lower()]
        try:
            cuda_index = nvidia_gpus.index(hardware)
        except ValueError:
            cuda_index = 0

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
        disponiveis = ort.get_available_providers()

        usar_cuda = dispositivo == "cuda" and "CUDAExecutionProvider" in disponiveis
        usar_dml = dispositivo == "directml" and "DmlExecutionProvider" in disponiveis

        if usar_cuda:
            kwargs["use_cuda"] = True
            kwargs["det_use_cuda"] = True
            kwargs["rec_use_cuda"] = True
            kwargs["cls_use_cuda"] = True
        elif usar_dml:
            kwargs["use_dml"] = True
            kwargs["det_use_dml"] = True
            kwargs["rec_use_dml"] = True
            kwargs["cls_use_dml"] = True
        elif dispositivo in ("cuda", "directml"):
            # Aceleração solicitada mas indisponível nesta instalação → CPU (sem travar o app).
            self._emitirProgresso(
                aoProgredir,
                f"Aceleração por GPU ({dispositivo}) indisponível nesta versão; usando a CPU.",
            )

        self._emitirProgresso(aoProgredir, MENSAGEM_CARREGANDO_MODELO)

        # Monkey-patch temporário no InferenceSession para injetar o device_id correto (do hardware selecionado)
        original_init = ort.InferenceSession.__init__

        def patched_init(sess_self, *args, **patched_kwargs):
            providers = patched_kwargs.get("providers")
            if providers:
                novos_providers = []
                for p in providers:
                    if isinstance(p, tuple):
                        name, opts = p
                        opts = dict(opts) if opts else {}
                        if name == "DmlExecutionProvider":
                            opts["device_id"] = dml_index
                        elif name == "CUDAExecutionProvider":
                            opts["device_id"] = cuda_index
                        novos_providers.append((name, opts))
                    elif isinstance(p, str):
                        if p == "DmlExecutionProvider":
                            novos_providers.append((p, {"device_id": dml_index}))
                        elif p == "CUDAExecutionProvider":
                            novos_providers.append((p, {"device_id": cuda_index}))
                        else:
                            novos_providers.append(p)
                    else:
                        novos_providers.append(p)
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
