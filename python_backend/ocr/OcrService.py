# ----- Importações -----
import os
import sys
import subprocess
from typing import Any, Callable, List, Optional, Tuple
import numpy as np

from ocr.ModelosOcrModule import DeteccaoOcr
from ocr import ModelosManifesto
from ocr.GerenciadorModelosModule import GerenciadorModelos
from principal import ConstantesModule


# ----- Constantes do Módulo -----
MENSAGEM_CARREGANDO_MODELO = "Carregando modelo de OCR (primeira vez)…"
MENSAGEM_RECONHECENDO = "Reconhecendo texto (OCR)…"


# ----- Classe de Serviço -----
class OcrService:
    """Reconhecimento de texto chinês.
    
    Suporta RapidOCR (padrão, rápido) e EasyOCR (instalado sob demanda).
    """

    def __init__(self) -> None:
        self._ocr = None
        self._modelo_atual = None
        self._dispositivo_atual = None
        self._threads_atual = None
        self._gerenciadorModelos = GerenciadorModelos()
        # Memoiza a última configuração cuja inicialização falhou, para não repetir
        # instalações pesadas (e 500s) a cada varredura automática enquanto a config não muda.
        self._falha_config = None
        self._falha_erro = None

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

        config_atual = (
            ConstantesModule.MODELO_OCR,
            ConstantesModule.DISPOSITIVO_OCR,
            ConstantesModule.HARDWARE_SELECIONADO,
            ConstantesModule.THREADS_CPU_OCR,
        )

        # Guard clause: se a última inicialização com ESTA mesma config falhou, não tenta de novo
        # (evita reinstalar pacotes pesados e devolver 500 a cada varredura). Mude a config para reativar.
        if self._falha_erro is not None and self._falha_config == config_atual:
            raise self._falha_erro

        # Inicializa ou reinicializa o OCR caso a configuração mude
        if (self._ocr is None or
            self._modelo_atual != ConstantesModule.MODELO_OCR or
            self._dispositivo_atual != ConstantesModule.DISPOSITIVO_OCR or
            self._threads_atual != ConstantesModule.THREADS_CPU_OCR or
            getattr(self, "_hardware_atual", None) != ConstantesModule.HARDWARE_SELECIONADO):

            try:
                self._inicializarOcr(aoProgredir)
            except Exception as erro:
                self._falha_config = config_atual
                self._falha_erro = erro
                raise

            # Sucesso: limpa qualquer falha memorizada
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
            if self._modelo_atual == "EasyOCR (Download)":
                resultado = self._ocr.readtext(imagem_processar)
            else:
                resultado, _tempo = self._ocr(imagem_processar)
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

    def _espacoLivreMb(self, caminho: str = ".") -> float:
        """Espaço livre em disco (MB) no volume do caminho. Retorna infinito se não medir."""
        import shutil
        try:
            return shutil.disk_usage(caminho).free / (1024 * 1024)
        except Exception:
            return float("inf")

    def _instalarPacotePip(self, argumentos: List[str], nome_visual: str, aoProgredir: Optional[Callable[[str], None]], espaco_minimo_mb: int = 1500) -> None:
        import subprocess
        executavel_python = sys.executable
        if executavel_python.lower().endswith("pythonw.exe"):
            executavel_console = executavel_python.lower().replace("pythonw.exe", "python.exe")
            if os.path.exists(executavel_console):
                executavel_python = executavel_console

        # Pré-checagem de disco: evita iniciar um download pesado que falharia no meio (deixando lixo
        # parcial) por falta de espaço — caso clássico do PyTorch CUDA (~2.5 GB).
        livre_mb = self._espacoLivreMb(os.path.dirname(executavel_python) or ".")
        if livre_mb < espaco_minimo_mb:
            raise RuntimeError(
                f"Espaço em disco insuficiente para instalar o {nome_visual}: "
                f"{livre_mb / 1024:.1f} GB livres, mas são necessários ~{espaco_minimo_mb / 1024:.1f} GB.\n"
                f"Libere espaço em disco e tente novamente, ou use o RapidOCR na CPU, que não exige download."
            )

        env = os.environ.copy()
        env["PYTHONIOENCODING"] = "utf-8"
        env["PIP_NO_COLOR"] = "1"
        env["PIP_PROGRESS_BAR"] = "off"
        # Silencia o ruído "[notice] To update, run: pip install --upgrade pip", que antes
        # mascarava a causa real do erro por aparecer como última linha do stderr.
        env["PIP_DISABLE_PIP_VERSION_CHECK"] = "1"

        comando = [executavel_python, "-m", "pip"] + argumentos

        # Junta o stderr no stdout: captura toda a saída num único fluxo (a causa real do erro
        # costuma sair no stderr) e evita deadlock quando o buffer de um dos canais enche.
        processo = subprocess.Popen(
            comando,
            stdout=subprocess.PIPE,
            stderr=subprocess.STDOUT,
            env=env,
            text=True,
            encoding="utf-8",
            errors="ignore",
            creationflags=0x08000000 if sys.platform == "win32" else 0
        )

        saida_linhas: List[str] = []
        while True:
            linha = (processo.stdout.readline() if processo.stdout else "")
            if not linha and processo.poll() is not None:
                break
            if not linha:
                continue

            saida_linhas.append(linha.rstrip("\n"))
            linha_limpa = linha.strip()
            if not linha_limpa:
                continue

            if linha_limpa.startswith("Collecting"):
                pkg = linha_limpa.split()[-1]
                self._emitirProgresso(aoProgredir, f"Instalando {nome_visual}: Coletando {pkg}…")
            elif "Downloading" in linha_limpa:
                partes = linha_limpa.replace("Downloading", "").strip().split()
                if partes:
                    pkg_info = partes[0]
                    tamanho_info = ""
                    if len(partes) > 1 and partes[-1].startswith("(") and partes[-1].endswith(")"):
                        tamanho_info = f" {partes[-1]}"
                    self._emitirProgresso(aoProgredir, f"Instalando {nome_visual}: Baixando {pkg_info}{tamanho_info}…")
            elif "Installing collected packages" in linha_limpa:
                self._emitirProgresso(aoProgredir, f"Instalando {nome_visual}: Finalizando instalação…")

        processo.wait()

        # Guard clause: instalação bem-sucedida
        if processo.returncode == 0:
            return

        pasta_dados = ConstantesModule.obterPastaDados()
        log_path = os.path.join(pasta_dados, f"erro_install_{nome_visual}.log")
        with open(log_path, "w", encoding="utf-8") as f_log:
            f_log.write(f"Comando: {comando}\n")
            f_log.write(f"Código de saída: {processo.returncode}\n")
            f_log.write("--- SAÍDA (stdout+stderr) ---\n")
            f_log.write("\n".join(saida_linhas))
            f_log.write("\n")

        resumo = self._resumirErroPip(saida_linhas)

        dica = ""
        if any(t in nome_visual.lower() for t in ("cuda", "directml", "onnx")):
            dica = ("\nDica: a aceleração por GPU depende de pacotes/drivers compatíveis. "
                    "Se o erro persistir, selecione 'DirectML' (universal no Windows) ou volte para a CPU nas configurações.")

        raise RuntimeError(
            f"Falha ao instalar o {nome_visual} via pip: {resumo}\n"
            f"Consulte o log completo em: {log_path}{dica}"
        )

    def _resumirErroPip(self, linhas: List[str]) -> str:
        """Extrai a linha de erro mais relevante da saída do pip, ignorando ruído de avisos."""
        relevantes = [
            linha.strip() for linha in linhas
            if linha.strip() and not linha.strip().lower().startswith("[notice]")
        ]

        # Guard clause: nenhuma linha útil
        if not relevantes:
            return "Erro desconhecido (consulte o log)."

        marcadores = ("error", "could not", "no matching distribution", "fatal", "failed")
        for linha in reversed(relevantes):
            if any(m in linha.lower() for m in marcadores):
                return linha

        return relevantes[-1]

    def _inicializarOcr(self, aoProgredir: Optional[Callable[[str], None]] = None) -> None:
        """Inicializa o motor de OCR de acordo com a configuração selecionada."""
        modelo_solicitado = ConstantesModule.MODELO_OCR
        dispositivo = ConstantesModule.DISPOSITIVO_OCR
        hardware = ConstantesModule.HARDWARE_SELECIONADO

        # Libera o motor anterior antes de carregar o novo: cada RapidOCR/EasyOCR segura centenas
        # de MB (sessões onnxruntime/torch); sem soltar, trocar de config acumula memória e leva a
        # MemoryError. O gc.collect força a coleta da instância antiga antes de alocar a nova.
        import gc
        self._ocr = None
        gc.collect()

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
        
        if modelo_solicitado == "EasyOCR (Download)":
            # Guard clause: se o executável estiver congelado (PyInstaller)
            if getattr(sys, 'frozen', False):
                raise RuntimeError(
                    "O EasyOCR não pode ser instalado dinamicamente no executável compilado (.exe).\n"
                    "Por favor, execute o Hanzi Tracker a partir do código-fonte ou "
                    "instale o EasyOCR no seu ambiente Python antes de compilar."
                )

            try:
                import easyocr
            except ImportError:
                self._emitirProgresso(aoProgredir, "Instalando EasyOCR via pip (pode demorar alguns minutos)…")
                self._instalarPacotePip(["install", "easyocr"], "EasyOCR", aoProgredir)

                import easyocr
            
            # Verifica se os modelos do EasyOCR já estão baixados
            diretorio_modelos = os.path.expanduser(os.path.join("~", ".EasyOCR", "model"))
            detector_ok = os.path.exists(os.path.join(diretorio_modelos, "craft_mlt_25k.pth"))
            reconhecedor_ok = os.path.exists(os.path.join(diretorio_modelos, "zh_sim_g2.pth"))

            if not (detector_ok and reconhecedor_ok):
                self._emitirProgresso(aoProgredir, "Baixando modelos do EasyOCR (~45MB) (pode demorar alguns minutos)…")
            else:
                self._emitirProgresso(aoProgredir, MENSAGEM_CARREGANDO_MODELO)

            if dispositivo == "directml":
                raise RuntimeError(
                    "O modelo EasyOCR suporta apenas aceleração GPU via CUDA (Nvidia).\n"
                    "Por favor, mude o dispositivo para 'CUDA' ou 'CPU' nas configurações, ou utilize o modelo RapidOCR para DirectML."
                )

            usar_gpu = (dispositivo == "cuda")
            if usar_gpu:
                import torch
                if not torch.cuda.is_available():
                    self._emitirProgresso(aoProgredir, "Instalando PyTorch com CUDA (isso é um download bem pesado, ~2.5GB)...")
                    args = ["install", "torch", "torchvision", "--index-url", "https://download.pytorch.org/whl/cu121", "--force-reinstall"]
                    # ~2.5 GB baixados + extração exigem folga; pré-checa ~6 GB para não falhar no meio.
                    self._instalarPacotePip(args, "PyTorch CUDA", aoProgredir, espaco_minimo_mb=6000)
                    
                    # Como o pytorch requer reload após instalação de CUDA se já estava importado, alertamos o usuário
                    raise RuntimeError(
                        "O PyTorch com suporte a CUDA foi instalado com sucesso!\n\n"
                        "Por favor, REINICIE o aplicativo Hanzi Tracker para ativar a GPU no EasyOCR."
                    )

            # Passa True ao invés de string para evitar problemas de compatibilidade com versões antigas do EasyOCR
            self._ocr = easyocr.Reader(['ch_sim', 'en'], gpu=usar_gpu, verbose=False)
            
        else:
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
        
        # Atualiza o estado
        self._modelo_atual = modelo_solicitado
        self._dispositivo_atual = dispositivo
        self._threads_atual = ConstantesModule.THREADS_CPU_OCR
        self._hardware_atual = hardware
