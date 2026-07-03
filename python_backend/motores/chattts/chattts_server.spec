# -*- mode: python ; coding: utf-8 -*-
# Congela o microserviço de TTS do ChatTTS (chattts_server.py) como sidecar autônomo, com o PyTorch
# de CPU embutido. Os PESOS (~1 GB) não vão no pacote: o próprio sidecar os baixa do Hugging Face
# na primeira síntese, para modelos\ChatTTS\hf (cache HF redirecionado pelo entry).
# Ver docs/CONTRATO-TTS.md. Rode via build_sidecars.ps1. Saída (onedir): dist/chattts_server/
# chattts_server.exe — relativa ao diretório de INVOCAÇÃO do PyInstaller (python_backend), não de
# onde este spec mora.

import os
from PyInstaller.utils.hooks import collect_all

# Este spec mora em motores/chattts/ (junto do entry chattts_server.py); python_backend é dois
# níveis acima — raiz que precisa entrar em pathex para os imports absolutos (tts.*, principal.*,
# motores.*) resolverem.
_backend = os.path.abspath(os.path.join(SPECPATH, "..", ".."))

# O ChatTTS carrega submódulos e configs dinamicamente (yamls do modelo, tokenizer, vocoder vocos);
# collect_all arrasta tudo. torch/torchaudio têm hooks oficiais do PyInstaller — basta que apareçam
# como hiddenimports. Pacotes da lista que não estiverem no venv são simplesmente pulados.
datas, binaries, hiddenimports = [], [], []
for _pacote in ("ChatTTS", "vocos", "vector_quantize_pytorch", "pybase16384", "transformers"):
    try:
        _d, _b, _h = collect_all(_pacote)
    except Exception:
        continue
    datas += _d
    binaries += _b
    hiddenimports += _h

# Módulos locais (alguns importados dentro de funções, ex.: `from principal import ConstantesModule`).
hiddenimports += [
    "torch",
    "torchaudio",
    "requests",
    "motores.chattts.ChatTtsService",
    "tts.ServicoTtsBase",
    "principal.ConstantesModule",
    "principal.ServidorTtsModule",
]

a = Analysis(
    [os.path.join(SPECPATH, "chattts_server.py")],
    pathex=[_backend],
    binaries=binaries,
    datas=datas,
    hiddenimports=hiddenimports,
    hookspath=[],
    runtime_hooks=[],
    # Motores dos OUTROS sidecars: ficam de fora mesmo se instalados no ambiente de build.
    excludes=["onnxruntime", "rapidocr_onnxruntime", "easyocr", "paddle", "paddleocr", "cv2"],
    noarchive=False,
)
pyz = PYZ(a.pure)

exe = EXE(
    pyz,
    a.scripts,
    [],
    exclude_binaries=True,
    name="chattts_server",
    console=True,  # microserviço HTTP: mantém stdout/stderr (o Go os captura); o Go oculta a janela.
    disable_windowed_traceback=False,
)
coll = COLLECT(
    exe,
    a.binaries,
    a.datas,
    name="chattts_server",
)
