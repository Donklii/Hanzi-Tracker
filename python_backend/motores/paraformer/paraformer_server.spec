# -*- mode: python ; coding: utf-8 -*-
# Congela o microserviço de STT do Paraformer-ZH (paraformer_server.py) como sidecar autônomo. Os
# PESOS não vão no pacote: o próprio sidecar os baixa do Hugging Face na primeira transcrição (ou
# no /api/stt/preparar), para motores_stt\Paraformer-ZH\modelos\hf (cache HF redirecionado pelo
# entry). Ver docs/CONTRATO-STT.md. Rode via builds/build_sidecars_stt_*.{sh,ps1}. Saída (onedir):
# dist/paraformer_server/ — relativa ao diretório de INVOCAÇÃO do PyInstaller (python_backend), não
# de onde este spec mora.

import os
from PyInstaller.utils.hooks import collect_all

# Este spec mora em motores/paraformer/ (junto do entry paraformer_server.py); python_backend é
# dois níveis acima — raiz que precisa entrar em pathex para os imports absolutos (stt.*,
# principal.*, motores.*) resolverem.
_backend = os.path.abspath(os.path.join(SPECPATH, "..", ".."))

# sherpa_onnx carrega bibliotecas nativas (onnxruntime embutido) e sounddevice embute o PortAudio —
# collect_all arrasta os binários dos dois. Pacotes da lista que não estiverem no venv são pulados.
datas, binaries, hiddenimports = [], [], []
for _pacote in ("sherpa_onnx", "sounddevice", "huggingface_hub"):
    try:
        _d, _b, _h = collect_all(_pacote)
    except Exception:
        continue
    datas += _d
    binaries += _b
    hiddenimports += _h

# Módulos locais (alguns importados dentro de funções, ex.: `from principal import ConstantesModule`).
hiddenimports += [
    "motores.paraformer.ParaformerSttService",
    "stt.ServicoSttBase",
    "principal.ConstantesModule",
    "principal.ServidorSttModule",
]

a = Analysis(
    [os.path.join(SPECPATH, "paraformer_server.py")],
    pathex=[_backend],
    binaries=binaries,
    datas=datas,
    hiddenimports=hiddenimports,
    hookspath=[],
    runtime_hooks=[],
    # Motores dos OUTROS sidecars: ficam de fora mesmo se instalados no ambiente de build.
    excludes=["torch", "kokoro", "misaki", "rapidocr_onnxruntime", "easyocr", "paddle", "paddleocr", "cv2"],
    noarchive=False,
)
pyz = PYZ(a.pure)

exe = EXE(
    pyz,
    a.scripts,
    [],
    exclude_binaries=True,
    name="paraformer_server",
    console=True,  # microserviço HTTP: mantém stdout/stderr (o Go os captura); o Go oculta a janela.
    disable_windowed_traceback=False,
)
coll = COLLECT(
    exe,
    a.binaries,
    a.datas,
    name="paraformer_server",
)
