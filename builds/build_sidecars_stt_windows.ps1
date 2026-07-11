# Congela os sidecars de ESCUTA/STT (paraformer_server e zipformer_streaming_server) em .exe com
# PyInstaller, cada um num venv PROPRIO — o irmao de build_sidecars_tts_windows.ps1. NAO publica
# nada — so gera os artefatos e imprime o sha256 de cada zip. Bem mais leve que os builds de TTS:
# os dois motores rodam em onnxruntime puro (embutido na wheel do sherpa-onnx), sem torch; o
# sounddevice embute o PortAudio.
# Ver BUILD.md (arquitetura) e docs/PUBLICAR-MOTORES.md (como publicar os artefatos gerados aqui).
#
# Uso (de qualquer pasta):  powershell -ExecutionPolicy Bypass -File builds/build_sidecars_stt_windows.ps1
# Os PESOS dos modelos NAO sao embutidos: o proprio sidecar os baixa do Hugging Face na primeira
# transcricao (cache HF redirecionado para motores_stt\<Motor>\modelos\hf) — ver docs/CONTRATO-STT.md.
# Saida: python_backend/dist/<motor>.zip (+ sha256 no fim).

$ErrorActionPreference = "Stop"
# O script mora em builds/; a raiz do projeto e a pasta pai.
$raiz    = Split-Path $PSScriptRoot -Parent
$backend = Join-Path $raiz "python_backend"

# Motores de escuta a congelar: nome do entry/spec (tambem e o nome da pasta onedir e do zip),
# pasta do motor em python_backend/motores/ e venv proprio de build.
$motores = @(
    @{ Nome = "paraformer_server";          Pasta = "paraformer";          Venv = "build_env_paraformer" },
    @{ Nome = "zipformer_streaming_server"; Pasta = "zipformer_streaming"; Venv = "build_env_zipformer_stt" }
)

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

Add-Type -AssemblyName System.IO.Compression.FileSystem

foreach ($motor in $motores) {
    Write-Host ("== Venv + congelamento do {0} (motor de escuta) ==" -f $motor.Nome) -ForegroundColor Cyan
    $py = Preparar-Venv (Join-Path $raiz $motor.Venv) ([IO.Path]::Combine("python_backend", "motores", $motor.Pasta, "requirements.txt"))
    # O spec mora em motores/<pasta>/. PyInstaller grava dist/build relativos ao diretorio de
    # INVOCACAO (Push-Location $backend), nao a pasta do .spec.
    Push-Location $backend
    try {
        & $py -m PyInstaller --noconfirm ([IO.Path]::Combine("motores", $motor.Pasta, ($motor.Nome + ".spec")))
    } finally {
        Pop-Location
    }

    Write-Host ("== Empacotando (zip) a saida onedir do {0} ==" -f $motor.Nome) -ForegroundColor Cyan
    # IMPORTANTE: NAO usar Compress-Archive (grava "\" nos nomes das entradas, violando a spec ZIP; o
    # archive/zip do Go nao extrai). [ZipFile]::CreateFromDirectory grava "/" corretamente;
    # includeBaseDirectory=$false poe o conteudo na raiz do zip (sem a pasta base).
    $pasta = Join-Path $backend ("dist\" + $motor.Nome)
    $zip   = Join-Path $backend ("dist\" + $motor.Nome + ".zip")
    if (-not (Test-Path $pasta)) { throw "NAO gerado: $pasta" }
    if (Test-Path $zip) { Remove-Item $zip -Force }
    [System.IO.Compression.ZipFile]::CreateFromDirectory($pasta, $zip, [System.IO.Compression.CompressionLevel]::Optimal, $false)
}

Write-Host "== Artefatos + sha256 (use no manifesto de motores de escuta) ==" -ForegroundColor Cyan
foreach ($motor in $motores) {
    $zip  = Join-Path $backend ("dist\" + $motor.Nome + ".zip")
    $hash = (Get-FileHash $zip -Algorithm SHA256).Hash.ToLower()
    $tam  = (Get-Item $zip).Length
    Write-Host ("  {0}" -f $zip) -ForegroundColor Green
    Write-Host ("    tamanho_bytes: {0}" -f $tam)
    Write-Host ("    sha256:        {0}" -f $hash)
}
Write-Host ""
Write-Host "Concluido. Via CI (tag motores-stt-windows-vN) o manifesto e atualizado sozinho. Manualmente: cole tag/sha256/tamanho em wails_app/motoresstt/artefatos_stt.json (ver docs/PUBLICAR-MOTORES.md)." -ForegroundColor Cyan
