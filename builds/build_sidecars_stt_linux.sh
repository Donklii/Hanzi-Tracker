#!/usr/bin/env bash
# Congela os sidecars de ESCUTA/STT para LINUX (paraformer_server e zipformer_streaming_server)
# com PyInstaller — o irmão de build_sidecars_tts_linux.sh. Cada motor usa um venv PRÓPRIO, como
# os outros motores. NÃO publica nada — só gera os artefatos e imprime o sha256 de cada zip.
#
# Bem mais leve que os builds de TTS: os dois motores rodam em onnxruntime puro (embutido na wheel
# do sherpa-onnx), sem torch — nada de índice de CPU do PyTorch. O sounddevice embute o PortAudio
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
# Saída: python_backend/dist/<motor>_linux.zip (+ sha256 no fim).
set -euo pipefail

raiz="$(cd "$(dirname "$0")/.." && pwd)"
backend="$raiz/python_backend"

# Motores de escuta a congelar: "<nome do entry/spec>:<pasta em motores/>:<venv de build>".
motores=(
    "paraformer_server:paraformer:build_env_paraformer"
    "zipformer_streaming_server:zipformer_streaming:build_env_zipformer_stt"
)

for motor in "${motores[@]}"; do
    IFS=':' read -r nome pasta_motor venv_nome <<<"$motor"

    echo "== Venv + congelamento do $nome (motor de escuta) =="
    venv="$raiz/$venv_nome"
    if [[ ! -d "$venv" ]]; then
        python3 -m venv "$venv"
    fi
    "$venv/bin/python" -m pip install --upgrade pip
    "$venv/bin/python" -m pip install -r "$backend/motores/$pasta_motor/requirements.txt"
    "$venv/bin/python" -m pip install pyinstaller
    # PyInstaller grava dist/build relativos ao diretório de INVOCAÇÃO (cd $backend), não à pasta do .spec.
    (cd "$backend" && "$venv/bin/python" -m PyInstaller --noconfirm "motores/$pasta_motor/$nome.spec")

    echo "== Empacotando (zip) a saída onedir do $nome =="
    # O artefato é o zip do CONTEÚDO da pasta onedir (executável na raiz do zip, como no Windows).
    pasta="$backend/dist/$nome"
    zip_saida="$backend/dist/${nome}_linux.zip"
    if [[ ! -d "$pasta" ]]; then
        echo "NÃO gerado: $pasta" >&2
        exit 1
    fi
    rm -f "$zip_saida"
    (cd "$pasta" && zip -qr "$zip_saida" .)
done

echo "== Artefatos + sha256 (o CI usa no manifesto artefatos_stt_linux.json) =="
for motor in "${motores[@]}"; do
    IFS=':' read -r nome _resto <<<"$motor"
    zip_saida="$backend/dist/${nome}_linux.zip"
    echo "  $zip_saida"
    echo "    tamanho_bytes: $(stat -c%s "$zip_saida")"
    echo "    sha256:        $(sha256sum "$zip_saida" | cut -d' ' -f1)"
done
echo ""
echo "Concluído. Via CI (tag motores-stt-linux-vN) o manifesto é atualizado sozinho; manualmente, cole tag/sha256/tamanho em wails_app/motoresstt/artefatos_stt_linux.json."
