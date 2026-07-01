# -*- mode: python ; coding: utf-8 -*-
# Congela o microserviço de OCR (server.py) como sidecar autônomo "ocr_server" (motor padrão RapidOCR).
# Ver BUILD.md e docs/CONTRATO-OCR.md. Rode via build_sidecars.ps1 (ambiente com onnxruntime-directml).
# Saída (onedir): dist/ocr_server/ocr_server.exe — casa com resolverMotorOcrPadrao() do app.

from PyInstaller.utils.hooks import collect_all

# onnxruntime e rapidocr são importados DINAMICAMENTE em runtime (OcrService._inicializarOcr), então a
# análise estática do PyInstaller não os enxerga. collect_all arrasta os binários (inclui DirectML.dll
# e os onnxruntime_providers_*.dll), os dados e os submódulos ocultos desses pacotes.
datas, binaries, hiddenimports = [], [], []
for _pacote in ("onnxruntime", "rapidocr_onnxruntime"):
    _d, _b, _h = collect_all(_pacote)
    datas += _d
    binaries += _b
    hiddenimports += _h

# Módulos locais (alguns importados dentro de funções, ex.: `from principal import ConstantesModule`).
hiddenimports += ["cv2", "ocr.OcrService", "ocr.GerenciadorModelosModule", "principal.ConstantesModule"]

a = Analysis(
    ["server.py"],
    pathex=[SPECPATH],
    binaries=binaries,
    datas=datas,
    hiddenimports=hiddenimports,
    hookspath=[],
    runtime_hooks=[],
    # Motores opcionais que NÃO fazem parte deste sidecar (viram sidecars próprios na Fase 5). Ficam de
    # fora mesmo se estiverem instalados no ambiente, mantendo o pacote enxuto e sem o conflito de GPU.
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
