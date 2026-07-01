# Congela os sidecars Python (ocr_server + popup) em .exe com PyInstaller, num ambiente isolado com
# aceleração DirectML embutida. NÃO publica nada — só gera os artefatos e imprime o sha256 de cada zip.
# Ver BUILD.md (arquitetura) e docs/PUBLICAR-MOTORES.md (como publicar os artefatos gerados aqui).
#
# Uso (na raiz do projeto):  powershell -ExecutionPolicy Bypass -File build_sidecars.ps1
# Saída: python_backend/dist/ocr_server.zip e python_backend/dist/popup.zip (+ sha256 no fim).

$ErrorActionPreference = "Stop"
$raiz    = $PSScriptRoot
$backend = Join-Path $raiz "python_backend"
$venv    = Join-Path $raiz "build_env"
$py      = Join-Path $venv "Scripts\python.exe"

Write-Host "== 1/6  Ambiente de build isolado ($venv) ==" -ForegroundColor Cyan
if (-not (Test-Path $venv)) {
    python -m venv $venv
}

Write-Host "== 2/6  Dependencias do backend (requirements.txt) ==" -ForegroundColor Cyan
& $py -m pip install --upgrade pip
& $py -m pip install -r (Join-Path $raiz "requirements.txt")

Write-Host "== 3/6  Trocando onnxruntime -> onnxruntime-directml ==" -ForegroundColor Cyan
# onnxruntime (CPU) e onnxruntime-directml sao MUTUAMENTE EXCLUSIVOS: remover antes de instalar.
& $py -m pip uninstall -y onnxruntime
& $py -m pip install "onnxruntime-directml>=1.17.0"
& $py -m pip install pyinstaller

Write-Host "   Provedores disponiveis no ambiente de build:" -ForegroundColor DarkGray
& $py -c "import onnxruntime as ort; print(ort.get_available_providers())"

Write-Host "== 4/6  Congelando sidecars (PyInstaller) ==" -ForegroundColor Cyan
Push-Location $backend
try {
    & $py -m PyInstaller --noconfirm ocr_server.spec
    & $py -m PyInstaller --noconfirm popup.spec
} finally {
    Pop-Location
}

Write-Host "== 5/6  Empacotando (zip) as saidas onedir ==" -ForegroundColor Cyan
# O onedir gera uma PASTA (exe + DLLs + dados). O artefato baixavel e o ZIP dessa pasta; o app o extrai
# em %APPDATA%\HanziTracker\motores\<nome>\. Zipamos o CONTEUDO (exe na raiz do zip).
$pares = @(
    @{ Nome = "ocr_server"; Pasta = (Join-Path $backend "dist\ocr_server"); Zip = (Join-Path $backend "dist\ocr_server.zip") },
    @{ Nome = "popup";      Pasta = (Join-Path $backend "dist\popup");      Zip = (Join-Path $backend "dist\popup.zip") }
)
foreach ($p in $pares) {
    if (-not (Test-Path $p.Pasta)) { Write-Warning "NAO gerado: $($p.Pasta)"; continue }
    if (Test-Path $p.Zip) { Remove-Item $p.Zip -Force }
    Compress-Archive -Path (Join-Path $p.Pasta "*") -DestinationPath $p.Zip
}

Write-Host "== 6/6  Artefatos + sha256 (use no manifesto de motores) ==" -ForegroundColor Cyan
foreach ($p in $pares) {
    if (-not (Test-Path $p.Zip)) { continue }
    $hash = (Get-FileHash $p.Zip -Algorithm SHA256).Hash.ToLower()
    $tam  = (Get-Item $p.Zip).Length
    Write-Host ("  {0}" -f $p.Zip) -ForegroundColor Green
    Write-Host ("    tamanho_bytes: {0}" -f $tam)
    Write-Host ("    sha256:        {0}" -f $hash)
}
Write-Host ""
Write-Host "Concluido. Publique os .zip conforme docs/PUBLICAR-MOTORES.md e cole o sha256 no manifesto." -ForegroundColor Cyan
