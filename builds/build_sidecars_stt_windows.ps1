# Congela o sidecar de ESCUTA/STT (paraformer_server) em .exe com PyInstaller, num venv PROPRIO —
# o irmao de build_sidecars_tts_windows.ps1. NAO publica nada — so gera o artefato e imprime o
# sha256 do zip. Bem mais leve que os builds de TTS: o Paraformer roda em onnxruntime puro
# (embutido na wheel do sherpa-onnx), sem torch; o sounddevice embute o PortAudio.
# Ver BUILD.md (arquitetura) e docs/PUBLICAR-MOTORES.md (como publicar o artefato gerado aqui).
#
# Uso (de qualquer pasta):  powershell -ExecutionPolicy Bypass -File builds/build_sidecars_stt_windows.ps1
# Os PESOS do modelo NAO sao embutidos: o proprio sidecar os baixa do Hugging Face na primeira
# transcricao (cache HF redirecionado para motores_stt\<Motor>\modelos\hf) — ver docs/CONTRATO-STT.md.
# Saida: python_backend/dist/paraformer_server.zip (+ sha256 no fim).

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

Write-Host "== 1/3  Venv + congelamento do paraformer_server (motor de escuta) ==" -ForegroundColor Cyan
$pyParaformer = Preparar-Venv (Join-Path $raiz "build_env_paraformer") ([IO.Path]::Combine("python_backend", "motores", "paraformer", "requirements.txt"))
# O spec mora em motores/paraformer/. PyInstaller grava dist/build relativos ao diretorio de
# INVOCACAO (Push-Location $backend), nao a pasta do .spec.
Push-Location $backend
try {
    & $pyParaformer -m PyInstaller --noconfirm ([IO.Path]::Combine("motores", "paraformer", "paraformer_server.spec"))
} finally {
    Pop-Location
}

Write-Host "== 2/3  Empacotando (zip) a saida onedir ==" -ForegroundColor Cyan
# IMPORTANTE: NAO usar Compress-Archive (grava "\" nos nomes das entradas, violando a spec ZIP; o
# archive/zip do Go nao extrai). [ZipFile]::CreateFromDirectory grava "/" corretamente;
# includeBaseDirectory=$false poe o conteudo na raiz do zip (sem a pasta base).
Add-Type -AssemblyName System.IO.Compression.FileSystem
$pasta = Join-Path $backend "dist\paraformer_server"
$zip   = Join-Path $backend "dist\paraformer_server.zip"
if (-not (Test-Path $pasta)) { throw "NAO gerado: $pasta" }
if (Test-Path $zip) { Remove-Item $zip -Force }
[System.IO.Compression.ZipFile]::CreateFromDirectory($pasta, $zip, [System.IO.Compression.CompressionLevel]::Optimal, $false)

Write-Host "== 3/3  Artefato + sha256 (use no manifesto de motores de escuta) ==" -ForegroundColor Cyan
$hash = (Get-FileHash $zip -Algorithm SHA256).Hash.ToLower()
$tam  = (Get-Item $zip).Length
Write-Host ("  {0}" -f $zip) -ForegroundColor Green
Write-Host ("    tamanho_bytes: {0}" -f $tam)
Write-Host ("    sha256:        {0}" -f $hash)
Write-Host ""
Write-Host "Concluido. Via CI (tag motores-stt-windows-vN) o manifesto e atualizado sozinho. Manualmente: cole tag/sha256/tamanho em wails_app/motoresstt/artefatos_stt.json (ver docs/PUBLICAR-MOTORES.md)." -ForegroundColor Cyan
