# Congela os sidecars de VOZ/TTS (kokoro_server + chattts_server) em .exe com PyInstaller. Cada motor
# usa um venv PROPRIO (ambos carregam torch, mas com dependencias de modelo distintas — kokoro/misaki
# vs. ChatTTS/vocos — que e mais limpo isolar). NAO publica nada — so gera os artefatos e imprime o
# sha256 de cada zip. Os motores de OCR tem script proprio: builds/build_sidecars_ocr.ps1.
# Ver BUILD.md (arquitetura) e docs/PUBLICAR-MOTORES.md (como publicar os artefatos gerados aqui).
#
# Uso (de qualquer pasta):  powershell -ExecutionPolicy Bypass -File builds/build_sidecars_tts.ps1
# Os PESOS dos modelos NAO sao embutidos: o proprio sidecar os baixa do Hugging Face na primeira
# sintese (cache HF redirecionado para modelos\<Motor>\hf) — ver docs/CONTRATO-TTS.md.
# Saida: python_backend/dist/{kokoro_server,chattts_server}.zip (+ sha256 no fim).

$ErrorActionPreference = "Stop"
# O script mora em builds/; a raiz do projeto e a pasta pai.
$raiz    = Split-Path $PSScriptRoot -Parent
$backend = Join-Path $raiz "python_backend"

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

Write-Host "== 1/4  Venv + congelamento do kokoro_server (motor de voz) ==" -ForegroundColor Cyan
$pyKokoro = Preparar-Venv (Join-Path $raiz "build_env_kokoro") ([IO.Path]::Combine("python_backend", "motores", "kokoro", "requirements.txt"))
# Specs por motor moram em motores/<motor>/. PyInstaller grava dist/build relativos ao diretorio de
# INVOCACAO (Push-Location $backend), nao a pasta do .spec.
Push-Location $backend
try {
    & $pyKokoro -m PyInstaller --noconfirm ([IO.Path]::Combine("motores", "kokoro", "kokoro_server.spec"))
} finally {
    Pop-Location
}

Write-Host "== 2/4  Venv + congelamento do chattts_server (motor de voz) ==" -ForegroundColor Cyan
$pyChat = Preparar-Venv (Join-Path $raiz "build_env_chattts") ([IO.Path]::Combine("python_backend", "motores", "chattts", "requirements.txt"))
Push-Location $backend
try {
    & $pyChat -m PyInstaller --noconfirm ([IO.Path]::Combine("motores", "chattts", "chattts_server.spec"))
} finally {
    Pop-Location
}

Write-Host "== 3/4  Empacotando (zip) as saidas onedir ==" -ForegroundColor Cyan
# O onedir gera uma PASTA (exe + DLLs + dados). O artefato baixavel e o ZIP dessa pasta; o app o extrai
# em %APPDATA%\HanziTracker\motores_tts\<nome>\, com o exe na raiz do zip.
#
# IMPORTANTE: NAO usar Compress-Archive. O Compress-Archive do PowerShell grava os nomes das entradas com
# separador "\" em vez de "/", violando a spec ZIP; extratores que a seguem (ex.: archive/zip do Go, no
# app) tratam "\" como caractere literal, entao as entradas de diretorio nao sao reconhecidas e a extracao
# quebra. [ZipFile]::CreateFromDirectory grava "/" corretamente; includeBaseDirectory=$false poe o
# conteudo na raiz do zip (sem a pasta base).
Add-Type -AssemblyName System.IO.Compression.FileSystem
$pares = @(
    @{ Nome = "kokoro_server";  Pasta = (Join-Path $backend "dist\kokoro_server");  Zip = (Join-Path $backend "dist\kokoro_server.zip") },
    @{ Nome = "chattts_server"; Pasta = (Join-Path $backend "dist\chattts_server"); Zip = (Join-Path $backend "dist\chattts_server.zip") }
)
foreach ($p in $pares) {
    if (-not (Test-Path $p.Pasta)) { Write-Warning "NAO gerado: $($p.Pasta)"; continue }
    if (Test-Path $p.Zip) { Remove-Item $p.Zip -Force }
    [System.IO.Compression.ZipFile]::CreateFromDirectory($p.Pasta, $p.Zip, [System.IO.Compression.CompressionLevel]::Optimal, $false)
}

Write-Host "== 4/4  Artefatos + sha256 (use no manifesto de motores de voz) ==" -ForegroundColor Cyan
foreach ($p in $pares) {
    if (-not (Test-Path $p.Zip)) { continue }
    $hash = (Get-FileHash $p.Zip -Algorithm SHA256).Hash.ToLower()
    $tam  = (Get-Item $p.Zip).Length
    Write-Host ("  {0}" -f $p.Zip) -ForegroundColor Green
    Write-Host ("    tamanho_bytes: {0}" -f $tam)
    Write-Host ("    sha256:        {0}" -f $hash)
}
Write-Host ""
Write-Host "Concluido. Publique os .zip conforme docs/PUBLICAR-MOTORES.md e cole o sha256 no manifesto (wails_app/motores_tts_manifesto.go)." -ForegroundColor Cyan
