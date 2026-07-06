#!/usr/bin/env bash
# Congela os sidecars de OCR para LINUX (ocr_server + easyocr_server) com PyInstaller — o equivalente
# Linux do build_sidecars_ocr_windows.ps1. Cada motor usa um venv PRÓPRIO, como no Windows. NÃO publica nada —
# só gera os artefatos e imprime o sha256 de cada zip.
#
# Diferenças em relação ao build Windows:
#   - RapidOCR: fica no onnxruntime de CPU dos requirements (o DirectML é Windows-only; não há troca).
#   - EasyOCR: o torch/torchvision vêm do índice de CPU do PyTorch — no Linux o PyPI padrão puxa as
#     wheels CUDA (+ ~5 GB de pacotes nvidia-*), inúteis aqui e grandes demais para runner e usuário.
#   - Tesseract: SEM build Linux por enquanto — o sidecar Windows empacota a instalação do choco
#     (tesseract.exe + DLLs); empacotar um tesseract relocável no Linux é outro projeto. O catálogo do
#     app já omite o Tesseract fora do Windows (motoresocr/manifesto.go).
#   - Zip SEM -y (symlinks viram cópias): o extrator do app (baixador.ExtrairZip) grava toda entrada
#     como arquivo comum 0755 — um symlink zipado como symlink viraria um arquivo de texto quebrado.
#
# Uso (de qualquer pasta): bash builds/build_sidecars_ocr_linux.sh
# Saída: python_backend/dist/{ocr_server,easyocr_server}_linux.zip (+ sha256 no fim).
set -euo pipefail

raiz="$(cd "$(dirname "$0")/.." && pwd)"
backend="$raiz/python_backend"

# Cria (se preciso) um venv e instala os pacotes. Diferente da versão PowerShell, não devolve nada:
# o chamador monta o caminho do python ("$pasta/bin/python") sozinho — evita a armadilha do stdout do
# pip contaminar o valor de retorno.
preparar_venv() {
    local pasta="$1" requirements="$2"
    if [[ ! -d "$pasta" ]]; then
        python3 -m venv "$pasta"
    fi
    "$pasta/bin/python" -m pip install --upgrade pip
    "$pasta/bin/python" -m pip install -r "$raiz/$requirements"
    "$pasta/bin/python" -m pip install pyinstaller
}

echo "== 1/4 Venv + congelamento do ocr_server (RapidOCR, onnxruntime CPU) =="
preparar_venv "$raiz/build_env" "python_backend/motores/rapidocr/requirements.txt"
# PyInstaller grava dist/build relativos ao diretório de INVOCAÇÃO (cd $backend), não à pasta do .spec.
(cd "$backend" && "$raiz/build_env/bin/python" -m PyInstaller --noconfirm motores/rapidocr/ocr_server.spec)

echo "== 2/4 Venv + congelamento do easyocr_server (torch CPU) =="
# torch/torchvision ANTES dos requirements, do índice de CPU: o easyocr declara "torch" sem pin, então
# o pip aceita os já instalados e não os troca pelas wheels CUDA do PyPI.
if [[ ! -d "$raiz/build_env_easyocr" ]]; then
    python3 -m venv "$raiz/build_env_easyocr"
fi
"$raiz/build_env_easyocr/bin/python" -m pip install --upgrade pip
"$raiz/build_env_easyocr/bin/python" -m pip install torch torchvision --index-url https://download.pytorch.org/whl/cpu
"$raiz/build_env_easyocr/bin/python" -m pip install -r "$raiz/python_backend/motores/easyocr/requirements.txt"
"$raiz/build_env_easyocr/bin/python" -m pip install pyinstaller
(cd "$backend" && "$raiz/build_env_easyocr/bin/python" -m PyInstaller --noconfirm motores/easyocr/easyocr_server.spec)

echo "== 3/4 Empacotando (zip) as saídas onedir =="
# O artefato é o zip do CONTEÚDO da pasta onedir (executável na raiz do zip, como no Windows). O cd
# para dentro da pasta reproduz o includeBaseDirectory=false do build Windows.
for nome in ocr_server easyocr_server; do
    pasta="$backend/dist/$nome"
    zip_saida="$backend/dist/${nome}_linux.zip"
    if [[ ! -d "$pasta" ]]; then
        echo "NÃO gerado: $pasta" >&2
        exit 1
    fi
    rm -f "$zip_saida"
    (cd "$pasta" && zip -qr "$zip_saida" .)
done

echo "== 4/4 Artefatos + sha256 (o CI usa no manifesto artefatos_ocr_linux.json) =="
for nome in ocr_server easyocr_server; do
    zip_saida="$backend/dist/${nome}_linux.zip"
    echo "  $zip_saida"
    echo "    tamanho_bytes: $(stat -c%s "$zip_saida")"
    echo "    sha256:        $(sha256sum "$zip_saida" | cut -d' ' -f1)"
done
echo ""
echo "Concluído. Via CI (tag motores-ocr-linux-vN) o manifesto é atualizado sozinho; manualmente, cole tag/sha256/tamanho em wails_app/motoresocr/artefatos_ocr_linux.json."
