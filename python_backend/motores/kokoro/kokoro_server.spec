# -*- mode: python ; coding: utf-8 -*-
# Congela o microserviço de TTS do Kokoro-82M (kokoro_server.py) como sidecar autônomo, com o
# PyTorch de CPU embutido. Os PESOS não vão no pacote: o próprio sidecar os baixa do Hugging Face
# na primeira síntese, para modelos\Kokoro-82M\hf (cache HF redirecionado pelo entry).
# Ver docs/CONTRATO-TTS.md. Rode via build_sidecars.ps1. Saída (onedir): dist/kokoro_server/
# kokoro_server.exe — relativa ao diretório de INVOCAÇÃO do PyInstaller (python_backend), não de
# onde este spec mora.

import os
from PyInstaller.utils.hooks import collect_all

# Este spec mora em motores/kokoro/ (junto do entry kokoro_server.py); python_backend é dois
# níveis acima — raiz que precisa entrar em pathex para os imports absolutos (tts.*, principal.*,
# motores.*) resolverem.
_backend = os.path.abspath(os.path.join(SPECPATH, "..", ".."))

# O kokoro e o misaki carregam submódulos e dados dinamicamente (vocabulários do G2P, dicionários
# do jieba/pypinyin, binário do espeak-ng usado como fallback de G2P inglês); collect_all arrasta
# tudo. torch tem hook oficial do PyInstaller — basta aparecer como hiddenimport. Pacotes da lista
# que não estiverem no venv são simplesmente pulados.
datas, binaries, hiddenimports = [], [], []
for _pacote in ("kokoro", "misaki", "jieba", "pypinyin", "pypinyin_dict", "cn2an",
                "espeakng_loader", "language_tags"):
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
    "motores.kokoro.KokoroTtsService",
    "tts.ServicoTtsBase",
    "principal.ConstantesModule",
    "principal.ServidorTtsModule",
]

a = Analysis(
    [os.path.join(SPECPATH, "kokoro_server.py")],
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
    name="kokoro_server",
    console=True,  # microserviço HTTP: mantém stdout/stderr (o Go os captura); o Go oculta a janela.
    disable_windowed_traceback=False,
)
coll = COLLECT(
    exe,
    a.binaries,
    a.datas,
    name="kokoro_server",
)
