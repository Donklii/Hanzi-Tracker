#!/usr/bin/env bash
# Congela os sidecars de VOZ/TTS para LINUX (kokoro_server + chattts_server) com PyInstaller — o
# equivalente Linux do build_sidecars_tts_windows.ps1. Cada motor usa um venv PRÓPRIO, como no Windows. NÃO
# publica nada — só gera os artefatos e imprime o sha256 de cada zip.
#
# Diferenças em relação ao build Windows:
#   - torch (e torchaudio, no ChatTTS) vêm do índice de CPU do PyTorch — no Linux o PyPI padrão puxa
#     as wheels CUDA (+ ~5 GB de pacotes nvidia-*), inúteis aqui e grandes demais.
#   - O espeak-ng do fallback de G2P do Kokoro vem da wheel espeakng_loader (tem build manylinux),
#     coletada pelo kokoro_server.spec — nada a instalar via apt.
#   - Zip SEM -y (symlinks viram cópias): o extrator do app (baixador.ExtrairZip) grava toda entrada
#     como arquivo comum 0755 — um symlink zipado como symlink viraria um arquivo de texto quebrado.
#
# Os PESOS não são embutidos: o próprio sidecar os baixa do Hugging Face na primeira síntese (cache
# HF redirecionado para motores_tts/<Motor>/modelos/hf) — ver docs/CONTRATO-TTS.md.
#
# Uso (de qualquer pasta): bash builds/build_sidecars_tts_linux.sh
# Saída: python_backend/dist/{kokoro_server,chattts_server}_linux.zip (+ sha256 no fim).
set -euo pipefail

raiz="$(cd "$(dirname "$0")/.." && pwd)"
backend="$raiz/python_backend"

# Cria (se preciso) um venv com torch de CPU + requirements do motor + PyInstaller. O torch entra
# ANTES dos requirements, do índice de CPU: kokoro/chattts declaram "torch" sem pin, então o pip
# aceita o já instalado e não o troca pelas wheels CUDA do PyPI.
preparar_venv_torch_cpu() {
    local pasta="$1" requirements="$2" extras="$3"
    if [[ ! -d "$pasta" ]]; then
        python3 -m venv "$pasta"
    fi
    "$pasta/bin/python" -m pip install --upgrade pip
    # shellcheck disable=SC2086 — $extras é uma lista de pacotes separada por espaço, de propósito.
    "$pasta/bin/python" -m pip install torch $extras --index-url https://download.pytorch.org/whl/cpu
    "$pasta/bin/python" -m pip install -r "$raiz/$requirements"
    "$pasta/bin/python" -m pip install pyinstaller
}

echo "== 1/4 Venv + congelamento do kokoro_server (motor de voz) =="
preparar_venv_torch_cpu "$raiz/build_env_kokoro" "python_backend/motores/kokoro/requirements.txt" ""
# PyInstaller grava dist/build relativos ao diretório de INVOCAÇÃO (cd $backend), não à pasta do .spec.
(cd "$backend" && "$raiz/build_env_kokoro/bin/python" -m PyInstaller --noconfirm motores/kokoro/kokoro_server.spec)

echo "== 2/4 Venv + congelamento do chattts_server (motor de voz) =="
preparar_venv_torch_cpu "$raiz/build_env_chattts" "python_backend/motores/chattts/requirements.txt" "torchaudio"
(cd "$backend" && "$raiz/build_env_chattts/bin/python" -m PyInstaller --noconfirm motores/chattts/chattts_server.spec)

echo "== 3/4 Empacotando (zip) as saídas onedir =="
# O artefato é o zip do CONTEÚDO da pasta onedir (executável na raiz do zip, como no Windows). O cd
# para dentro da pasta reproduz o includeBaseDirectory=false do build Windows.
for nome in kokoro_server chattts_server; do
    pasta="$backend/dist/$nome"
    zip_saida="$backend/dist/${nome}_linux.zip"
    if [[ ! -d "$pasta" ]]; then
        echo "NÃO gerado: $pasta" >&2
        exit 1
    fi
    rm -f "$zip_saida"
    (cd "$pasta" && zip -qr "$zip_saida" .)
done

echo "== 4/4 Artefatos + sha256 (o CI usa no manifesto artefatos_tts_linux.json) =="
for nome in kokoro_server chattts_server; do
    zip_saida="$backend/dist/${nome}_linux.zip"
    echo "  $zip_saida"
    echo "    tamanho_bytes: $(stat -c%s "$zip_saida")"
    echo "    sha256:        $(sha256sum "$zip_saida" | cut -d' ' -f1)"
done
echo ""
echo "Concluído. Via CI (tag motores-tts-linux-vN) o manifesto é atualizado sozinho; manualmente, cole tag/sha256/tamanho em wails_app/motorestts/artefatos_tts_linux.json."
