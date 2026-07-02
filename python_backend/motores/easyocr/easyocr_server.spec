# -*- mode: python ; coding: utf-8 -*-
# Congela o microserviço de OCR do EasyOCR (easyocr_server.py) como sidecar autônomo, com o PyTorch
# de CPU embutido (sem CUDA — descartado por custo/benefício, ver Fase 5 no TODO.md). Os PESOS não
# vão no pacote: o Go os baixa para modelos\EasyOCR\ (ver ModelosManifesto.py, neste mesmo diretório).
# Ver docs/CONTRATO-OCR.md. Rode via build_sidecars.ps1. Saída (onedir): dist/easyocr_server/
# easyocr_server.exe — relativa ao diretório de INVOCAÇÃO do PyInstaller (python_backend), não de
# onde este spec mora.

import os
from PyInstaller.utils.hooks import collect_all

# Este spec mora em motores/easyocr/ (junto do entry easyocr_server.py); python_backend é dois
# níveis acima — raiz que precisa entrar em pathex para os imports absolutos (ocr.*, principal.*,
# motores.*) resolverem.
_backend = os.path.abspath(os.path.join(SPECPATH, "..", ".."))

# O easyocr carrega submódulos e dados (characters/*, config yaml) dinamicamente; collect_all arrasta
# tudo. torch/torchvision têm hooks oficiais do PyInstaller (pyinstaller-hooks-contrib) — basta que
# apareçam como hiddenimports para os hooks entrarem em ação.
datas, binaries, hiddenimports = collect_all("easyocr")
hiddenimports += [
    "cv2",
    "torchvision",
    "motores.easyocr.EasyOcrService",
    "motores.easyocr.ModelosManifesto",
    "ocr.GerenciadorModelosModule",
    "ocr.ServicoOcrBase",
    "ocr.ModelosOcrModule",
    "principal.ConstantesModule",
    "principal.ServidorOcrModule",
]

a = Analysis(
    [os.path.join(SPECPATH, "easyocr_server.py")],
    pathex=[_backend],
    binaries=binaries,
    datas=datas,
    hiddenimports=hiddenimports,
    hookspath=[],
    runtime_hooks=[],
    # Motores dos OUTROS sidecars: ficam de fora mesmo se instalados no ambiente de build.
    excludes=["onnxruntime", "rapidocr_onnxruntime", "paddle", "paddleocr"],
    noarchive=False,
)
pyz = PYZ(a.pure)

exe = EXE(
    pyz,
    a.scripts,
    [],
    exclude_binaries=True,
    name="easyocr_server",
    console=True,  # microserviço HTTP: mantém stdout/stderr (o Go os captura); o Go oculta a janela.
    disable_windowed_traceback=False,
)
coll = COLLECT(
    exe,
    a.binaries,
    a.datas,
    name="easyocr_server",
)
