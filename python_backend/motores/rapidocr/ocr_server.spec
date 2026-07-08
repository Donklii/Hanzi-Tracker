# -*- mode: python ; coding: utf-8 -*-
# Congela o microserviço de OCR (server.py) como sidecar autônomo "ocr_server" (motor padrão RapidOCR).
# Ver BUILD.md e docs/CONTRATO-OCR.md. Rode via builds/build_sidecars_ocr_{windows.ps1,linux.sh}
# (ambiente com onnxruntime-webgpu).
# Saída (onedir): dist/ocr_server/ocr_server.exe — casa com resolverMotorOcrPadrao() do app. O dist/
# sai relativo ao diretório de INVOCAÇÃO do PyInstaller (python_backend), não de onde este spec mora.

import os
from PyInstaller.utils.hooks import collect_all

# Este spec mora em motores/rapidocr/ (junto do entry server.py); python_backend é dois níveis acima
# — raiz que precisa entrar em pathex para os imports absolutos (ocr.*, principal.*, motores.*)
# resolverem, e onde os módulos locais hiddenimports abaixo são procurados.
_backend = os.path.abspath(os.path.join(SPECPATH, "..", ".."))

# onnxruntime e rapidocr são importados DINAMICAMENTE em runtime (OcrService._inicializarOcr), então a
# análise estática do PyInstaller não os enxerga. collect_all arrasta os binários (a lib do onnxruntime
# com o WebGPU/Dawn embutido e os onnxruntime_providers_*), os dados e os submódulos ocultos desses pacotes.
datas, binaries, hiddenimports = [], [], []
for _pacote in ("onnxruntime", "rapidocr_onnxruntime"):
    _d, _b, _h = collect_all(_pacote)
    datas += _d
    binaries += _b
    hiddenimports += _h

# Módulos locais (alguns importados dentro de funções, ex.: `from principal import ConstantesModule`).
hiddenimports += [
    "cv2",
    "motores.rapidocr.OcrService",
    "motores.rapidocr.ModelosManifesto",
    "ocr.GerenciadorModelosModule",
    "ocr.ServicoOcrBase",
    "ocr.ModelosOcrModule",
    "principal.ConstantesModule",
    "principal.ServidorOcrModule",
]

a = Analysis(
    [os.path.join(SPECPATH, "server.py")],
    pathex=[_backend],
    binaries=binaries,
    datas=datas,
    hiddenimports=hiddenimports,
    hookspath=[],
    runtime_hooks=[],
    # Motores dos OUTROS sidecars: ficam de fora mesmo se estiverem instalados no ambiente, mantendo
    # o pacote enxuto e sem o conflito de GPU (onnxruntime-webgpu x torch no mesmo processo).
    excludes=["easyocr", "torch", "torchvision", "paddle", "paddleocr"],
    noarchive=False,
)
pyz = PYZ(a.pure)

exe = EXE(
    pyz,
    a.scripts,
    [],
    exclude_binaries=True,
    name="ocr_server",
    console=True,  # microserviço HTTP: mantém stdout/stderr (o Go os captura); o Go oculta a janela.
    disable_windowed_traceback=False,
)
coll = COLLECT(
    exe,
    a.binaries,
    a.datas,
    name="ocr_server",
)
