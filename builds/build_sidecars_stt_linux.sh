#!/usr/bin/env bash
# Congela o sidecar de ESCUTA/STT para LINUX (paraformer_server) com PyInstaller — o irmão de
# build_sidecars_tts_linux.sh. Usa um venv PRÓPRIO, como os outros motores. NÃO publica nada — só
# gera o artefato e imprime o sha256 do zip.
#
# Bem mais leve que os builds de TTS: o Paraformer roda em onnxruntime puro (embutido na wheel do
# sherpa-onnx), sem torch — nada de índice de CPU do PyTorch. O sounddevice embute o PortAudio
# (wheel manylinux), nada a instalar via apt.
#
# Zip SEM -y (symlinks viram cópias), como no TTS: o extrator do app (baixador.ExtrairZip) grava
# toda entrada como arquivo comum 0755 — um symlink zipado como symlink viraria um arquivo de
# texto quebrado.
#
# Os PESOS não são embutidos: o próprio sidecar os baixa do Hugging Face na primeira transcrição
# (cache HF redirecionado para motores_stt/<Motor>/modelos/hf) — ver docs/CONTRATO-STT.md.
#
# Uso (de qualquer pasta): bash builds/build_sidecars_stt_linux.sh
# Saída: python_backend/dist/paraformer_server_linux.zip (+ sha256 no fim).
set -euo pipefail

raiz="$(cd "$(dirname "$0")/.." && pwd)"
backend="$raiz/python_backend"

echo "== 1/3 Venv + congelamento do paraformer_server (motor de escuta) =="
venv="$raiz/build_env_paraformer"
if [[ ! -d "$venv" ]]; then
    python3 -m venv "$venv"
fi
"$venv/bin/python" -m pip install --upgrade pip
"$venv/bin/python" -m pip install -r "$raiz/python_backend/motores/paraformer/requirements.txt"
"$venv/bin/python" -m pip install pyinstaller
# PyInstaller grava dist/build relativos ao diretório de INVOCAÇÃO (cd $backend), não à pasta do .spec.
(cd "$backend" && "$venv/bin/python" -m PyInstaller --noconfirm motores/paraformer/paraformer_server.spec)

echo "== 2/3 Empacotando (zip) a saída onedir =="
# O artefato é o zip do CONTEÚDO da pasta onedir (executável na raiz do zip, como no Windows).
pasta="$backend/dist/paraformer_server"
zip_saida="$backend/dist/paraformer_server_linux.zip"
if [[ ! -d "$pasta" ]]; then
    echo "NÃO gerado: $pasta" >&2
    exit 1
fi
rm -f "$zip_saida"
(cd "$pasta" && zip -qr "$zip_saida" .)

echo "== 3/3 Artefato + sha256 (o CI usa no manifesto artefatos_stt_linux.json) =="
echo "  $zip_saida"
echo "    tamanho_bytes: $(stat -c%s "$zip_saida")"
echo "    sha256:        $(sha256sum "$zip_saida" | cut -d' ' -f1)"
echo ""
echo "Concluído. Via CI (tag motores-stt-linux-vN) o manifesto é atualizado sozinho; manualmente, cole tag/sha256/tamanho em wails_app/motoresstt/artefatos_stt_linux.json."
