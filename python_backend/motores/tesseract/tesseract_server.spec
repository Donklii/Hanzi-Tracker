# -*- mode: python ; coding: utf-8 -*-
# Congela o microserviço de OCR do Tesseract (tesseract_server.py) como sidecar autônomo.
# Ver docs/CONTRATO-OCR.md. Rode via build_sidecars.ps1, que DEPOIS do freeze copia o tesseract.exe
# + tessdata (chi_sim fast + eng) para dist/tesseract_server/tesseract/ — o serviço o chama por
# subprocess (TesseractService._resolverExecutavel), então ele não entra na análise do PyInstaller.
# Saída (onedir): dist/tesseract_server/tesseract_server.exe — relativa ao diretório de INVOCAÇÃO
# do PyInstaller (python_backend), não de onde este spec mora.

import os

# Este spec mora em motores/tesseract/ (junto do entry tesseract_server.py); python_backend é dois
# níveis acima — raiz que precisa entrar em pathex para os imports absolutos (ocr.*, principal.*,
# motores.*) resolverem.
_backend = os.path.abspath(os.path.join(SPECPATH, "..", ".."))

# Módulos locais (alguns importados dentro de funções) + cv2/numpy do servidor HTTP compartilhado.
hiddenimports = [
    "cv2",
    "motores.tesseract.TesseractService",
    "motores.tesseract.ModelosManifesto",
    "ocr.GerenciadorModelosModule",
    "ocr.ServicoOcrBase",
    "ocr.ModelosOcrModule",
    "principal.ConstantesModule",
    "principal.ServidorOcrModule",
]

a = Analysis(
    [os.path.join(SPECPATH, "tesseract_server.py")],
    pathex=[_backend],
    binaries=[],
    datas=[],
    hiddenimports=hiddenimports,
    hookspath=[],
    runtime_hooks=[],
    # Motores dos OUTROS sidecars: ficam de fora mesmo se instalados no ambiente de build,
    # mantendo este pacote enxuto (o Tesseract não usa Python para inferência).
    excludes=["onnxruntime", "rapidocr_onnxruntime", "easyocr", "torch", "torchvision", "paddle", "paddleocr"],
    noarchive=False,
)
pyz = PYZ(a.pure)

exe = EXE(
    pyz,
    a.scripts,
    [],
    exclude_binaries=True,
    name="tesseract_server",
    console=True,  # microserviço HTTP: mantém stdout/stderr (o Go os captura); o Go oculta a janela.
    disable_windowed_traceback=False,
)
coll = COLLECT(
    exe,
    a.binaries,
    a.datas,
    name="tesseract_server",
)
