# Congela os sidecars de OCR (ocr_server + tesseract_server + easyocr_server) em .exe com PyInstaller.
# Cada motor usa um venv PROPRIO: o build do RapidOCR carrega onnxruntime-webgpu e o do EasyOCR
# carrega torch — pacotes que nao podem conviver no mesmo ambiente sem inchar/conflitar os pacotes
# finais. NAO publica nada — so gera os artefatos e imprime o sha256 de cada zip.
# Os motores de VOZ (TTS) tem script proprio: builds/build_sidecars_tts_windows.ps1.
# Ver BUILD.md (arquitetura) e docs/PUBLICAR-MOTORES.md (como publicar os artefatos gerados aqui).
#
# Uso (de qualquer pasta):  powershell -ExecutionPolicy Bypass -File builds/build_sidecars_ocr_windows.ps1
# Requisito p/ o sidecar Tesseract: instalacao do Tesseract (choco install tesseract) em
# "C:\Program Files\Tesseract-OCR" — ou aponte outra pasta via a env TESSERACT_DIR. Sem ela, esse
# sidecar e PULADO (com aviso); os demais saem normalmente.
# Saida: python_backend/dist/{ocr_server,tesseract_server,easyocr_server}.zip (+ sha256 no fim).

$ErrorActionPreference = "Stop"
# O script mora em builds/; a raiz do projeto e a pasta pai.
$raiz    = Split-Path $PSScriptRoot -Parent
$backend = Join-Path $raiz "python_backend"

# Pesos tessdata_fast embutidos no sidecar Tesseract: tag 4.1.0 (imutavel) + sha256 conferido.
$urlChiSimFast  = "https://github.com/tesseract-ocr/tessdata_fast/raw/4.1.0/chi_sim.traineddata"
$hashChiSimFast = "a5fcb6f0db1e1d6d8522f39db4e848f05984669172e584e8d76b6b3141e1f730"

# Cria (se preciso) um venv e instala os pacotes; devolve o caminho do python.exe dele.
# O "| Out-Host" e obrigatorio: sem ele, o stdout do pip entraria no VALOR DE RETORNO da funcao
# (fluxo de saida do PowerShell) e $py viraria um array de strings em vez do caminho.
function Preparar-Venv([string]$pasta, [string]$requirements) {
    if (-not (Test-Path $pasta)) {
        python -m venv $pasta | Out-Host
    }
    $py = Join-Path $pasta "Scripts\python.exe"
    & $py -m pip install --upgrade pip | Out-Host
    & $py -m pip install -r (Join-Path $raiz $requirements) | Out-Host
    & $py -m pip install pyinstaller | Out-Host
    return $py
}

Write-Host "== 1/7 Venv do RapidOCR (build_env, motores/rapidocr/requirements.txt) ==" -ForegroundColor Cyan
$pyRapid = Preparar-Venv (Join-Path $raiz "build_env") ([IO.Path]::Combine("python_backend", "motores", "rapidocr", "requirements.txt"))

Write-Host "== 2/7 Trocando onnxruntime -> onnxruntime-webgpu ==" -ForegroundColor Cyan
# onnxruntime (CPU) e onnxruntime-webgpu sao MUTUAMENTE EXCLUSIVOS (instalam o MESMO modulo
# `onnxruntime`): remover antes de instalar. O uninstall do onnxruntime-directml cobre venvs de
# build antigos (a aceleracao era DirectML antes do WebGPU; ver docs/BUILD.md).
# --force-reinstall: em venv REUTILIZADO o passo 1 reinstala o onnxruntime CPU por cima dos arquivos
# do webgpu e o uninstall acima os remove — sem o force, o pip diria "already satisfied" (o dist-info
# do webgpu sobrevive) e deixaria o venv sem o modulo. --no-deps: as dependencias (numpy, flatbuffers,
# protobuf, packaging) sao as mesmas do onnxruntime CPU, ja instaladas pelos requirements.
& $pyRapid -m pip uninstall -y onnxruntime onnxruntime-directml
& $pyRapid -m pip install --force-reinstall --no-deps "onnxruntime-webgpu>=1.27.0"

Write-Host "   Provedores disponiveis no ambiente de build:" -ForegroundColor DarkGray
& $pyRapid -c "import onnxruntime as ort; print(ort.get_available_providers())"

Write-Host "== 3/7 Congelando ocr_server (PyInstaller) ==" -ForegroundColor Cyan
# Specs por motor moram em motores/<motor>/ (ver python_backend/motores/). PyInstaller grava dist/build
# relativos ao diretorio de INVOCACAO (Push-Location $backend), nao a pasta do .spec.
Push-Location $backend
try {
    & $pyRapid -m PyInstaller --noconfirm ([IO.Path]::Combine("motores", "rapidocr", "ocr_server.spec"))
} finally {
    Pop-Location
}

Write-Host "== 4/7 Venv + congelamento do tesseract_server ==" -ForegroundColor Cyan
$pyTess = Preparar-Venv (Join-Path $raiz "build_env_tesseract") ([IO.Path]::Combine("python_backend", "motores", "tesseract", "requirements.txt"))
Push-Location $backend
try {
    & $pyTess -m PyInstaller --noconfirm ([IO.Path]::Combine("motores", "tesseract", "tesseract_server.spec"))
} finally {
    Pop-Location
}

Write-Host "== 5/7 Empacotando o tesseract.exe + tessdata no sidecar ==" -ForegroundColor Cyan
# O tesseract.exe nao vem do pip: copiamos a instalacao inteira (exe + DLLs + tessdata, que ja traz o
# eng) para dentro do pacote congelado, onde TesseractService._resolverExecutavel a procura. Depois
# garantimos o chi_sim (tessdata_fast 4.1.0, hash conferido) — o peso chines embutido do motor.
$tesseractOrigem = if ($env:TESSERACT_DIR) { $env:TESSERACT_DIR } else { "C:\Program Files\Tesseract-OCR" }
$tesseractDist   = Join-Path $backend "dist\tesseract_server"
if (-not (Test-Path (Join-Path $tesseractOrigem "tesseract.exe"))) {
    Write-Warning "Tesseract nao encontrado em '$tesseractOrigem' (instale com 'choco install tesseract' ou aponte TESSERACT_DIR)."
    Write-Warning "O sidecar tesseract_server NAO sera empacotado."
    if (Test-Path $tesseractDist) { Remove-Item $tesseractDist -Recurse -Force }
} else {
    Copy-Item $tesseractOrigem (Join-Path $tesseractDist "tesseract") -Recurse -Force
    $tessdata = Join-Path $tesseractDist "tesseract\tessdata"
    $chiSim   = Join-Path $tessdata "chi_sim.traineddata"
    if (-not (Test-Path $chiSim)) {
        Write-Host "   Baixando chi_sim.traineddata (tessdata_fast 4.1.0)..." -ForegroundColor DarkGray
        Invoke-WebRequest -Uri $urlChiSimFast -OutFile $chiSim -UseBasicParsing
    }
    $hashLocal = (Get-FileHash $chiSim -Algorithm SHA256).Hash.ToLower()
    if ($hashLocal -ne $hashChiSimFast) {
        throw "Integridade do chi_sim.traineddata falhou: sha256 esperado $hashChiSimFast, obtido $hashLocal"
    }
}

Write-Host "== 6/7 Venv + congelamento do easyocr_server ==" -ForegroundColor Cyan
$pyEasy = Preparar-Venv (Join-Path $raiz "build_env_easyocr") ([IO.Path]::Combine("python_backend", "motores", "easyocr", "requirements.txt"))
Push-Location $backend
try {
    & $pyEasy -m PyInstaller --noconfirm ([IO.Path]::Combine("motores", "easyocr", "easyocr_server.spec"))
} finally {
    Pop-Location
}

Write-Host "== 7/7  Empacotando (zip) as saidas onedir + sha256 ==" -ForegroundColor Cyan
# O onedir gera uma PASTA (exe + DLLs + dados). O artefato baixavel e o ZIP dessa pasta; o app o extrai
# em %APPDATA%\HanziTracker\motores\<nome>\, com o exe na raiz do zip.
#
# IMPORTANTE: NAO usar Compress-Archive. O Compress-Archive do PowerShell grava os nomes das entradas com
# separador "\" em vez de "/", violando a spec ZIP; extratores que a seguem (ex.: archive/zip do Go, no
# app) tratam "\" como caractere literal, entao as entradas de diretorio nao sao reconhecidas e a extracao
# quebra. [ZipFile]::CreateFromDirectory grava "/" corretamente; includeBaseDirectory=$false poe o
# conteudo na raiz do zip (sem a pasta base).
Add-Type -AssemblyName System.IO.Compression.FileSystem
$pares = @(
    @{ Nome = "ocr_server";       Pasta = (Join-Path $backend "dist\ocr_server");       Zip = (Join-Path $backend "dist\ocr_server.zip") },
    @{ Nome = "tesseract_server"; Pasta = (Join-Path $backend "dist\tesseract_server"); Zip = (Join-Path $backend "dist\tesseract_server.zip") },
    @{ Nome = "easyocr_server";   Pasta = (Join-Path $backend "dist\easyocr_server");   Zip = (Join-Path $backend "dist\easyocr_server.zip") }
)
foreach ($p in $pares) {
    if (-not (Test-Path $p.Pasta)) { Write-Warning "NAO gerado: $($p.Pasta)"; continue }
    if (Test-Path $p.Zip) { Remove-Item $p.Zip -Force }
    [System.IO.Compression.ZipFile]::CreateFromDirectory($p.Pasta, $p.Zip, [System.IO.Compression.CompressionLevel]::Optimal, $false)
}

foreach ($p in $pares) {
    if (-not (Test-Path $p.Zip)) { continue }
    $hash = (Get-FileHash $p.Zip -Algorithm SHA256).Hash.ToLower()
    $tam  = (Get-Item $p.Zip).Length
    Write-Host ("  {0}" -f $p.Zip) -ForegroundColor Green
    Write-Host ("    tamanho_bytes: {0}" -f $tam)
    Write-Host ("    sha256:        {0}" -f $hash)
}
Write-Host ""
Write-Host "Concluido. Via CI (tag motores-ocr-windows-vN) o manifesto e atualizado sozinho. Manualmente: cole tag/sha256/tamanho em wails_app/motoresocr/artefatos_ocr.json (ver docs/PUBLICAR-MOTORES.md)." -ForegroundColor Cyan
