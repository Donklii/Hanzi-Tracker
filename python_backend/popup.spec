# -*- mode: python ; coding: utf-8 -*-
# Congela o overlay (popup.py, tkinter) como sidecar "popup". Ver BUILD.md.
# Saída (onedir): dist/popup/popup.exe — casa com resolverComandoPopup() do app.
#
# IMPORTANTE: console=True (NÃO --windowed). O popup.py lê os comandos do Go via `sys.stdin`; no modo
# windowed o PyInstaller zera sys.stdin/stdout/stderr e o overlay morreria na primeira leitura. Com
# console, o stdin funciona normalmente e o Go já sobe o processo com HideWindow (janela oculta).

a = Analysis(
    ["popup.py"],
    pathex=[SPECPATH],
    binaries=[],
    datas=[],
    hiddenimports=[],
    hookspath=[],
    runtime_hooks=[],
    # O overlay só usa tkinter (stdlib) — mantém fora as dependências pesadas do motor de OCR.
    excludes=["onnxruntime", "rapidocr_onnxruntime", "cv2", "numpy", "easyocr", "torch", "paddleocr"],
    noarchive=False,
)
pyz = PYZ(a.pure)

exe = EXE(
    pyz,
    a.scripts,
    [],
    exclude_binaries=True,
    name="popup",
    console=True,  # necessário para o sys.stdin (comandos do Go); o Go oculta a janela via HideWindow.
    disable_windowed_traceback=False,
)
coll = COLLECT(
    exe,
    a.binaries,
    a.datas,
    name="popup",
)
