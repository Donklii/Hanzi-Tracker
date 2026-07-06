#!/usr/bin/env bash
# Monta o pacote .deb do Hanzi Tracker a partir do binário Linux gerado pelo `wails build`.
#
# Uso: bash montar_deb.sh <versao> <caminho-do-binario> <caminho-do-deb-de-saida>
# Ex.: bash montar_deb.sh 1.2.0 build/bin/HanziTracker build/bin/hanzitracker_1.2.0_amd64.deb
#
# Só precisa de dpkg-deb (presente em qualquer Debian/Ubuntu, inclusive nos runners do GitHub).
# O nome do arquivo de saída fica a cargo do chamador: o workflow usa nome versionado nas releases
# estáveis e nome fixo na prerelease rolante app-dev (para o --clobber substituir o anterior).
set -euo pipefail

versao="${1:?informe a versão (ex.: 1.2.0)}"
binario="${2:?informe o caminho do binário HanziTracker}"
deb_saida="${3:?informe o caminho do .deb de saída}"
raiz="$(cd "$(dirname "$0")" && pwd)"

pacote="$(mktemp -d)"
trap 'rm -rf "$pacote"' EXIT

# Estrutura instalada: binário no PATH (minúsculo, padrão Debian), atalho de menu e ícone
# (o mesmo appicon.png do Windows; /usr/share/pixmaps aceita qualquer tamanho).
install -Dm755 "$binario"                   "$pacote/usr/bin/hanzitracker"
install -Dm644 "$raiz/hanzitracker.desktop" "$pacote/usr/share/applications/hanzitracker.desktop"
install -Dm644 "$raiz/../build/appicon.png" "$pacote/usr/share/pixmaps/hanzitracker.png"

# libgtk-3-0t64 é o nome pós-transição time_t do Ubuntu 24.04+; a alternativa cobre distros que
# mantiveram o nome antigo. O WebKitGTK 4.1 casa com a tag de build webkit2_41 do workflow.
mkdir -p "$pacote/DEBIAN"
tamanho_kb="$(du -sk "$pacote" | cut -f1)"
cat > "$pacote/DEBIAN/control" <<CONTROL
Package: hanzitracker
Version: $versao
Section: education
Priority: optional
Architecture: amd64
Installed-Size: $tamanho_kb
Maintainer: Donklii <donklii13@gmail.com>
Homepage: https://github.com/Donklii/Hanzi-Tracker
Depends: libgtk-3-0t64 | libgtk-3-0, libwebkit2gtk-4.1-0, xdg-utils
Description: Estudo de chinês com OCR de tela em tempo real
 O Hanzi Tracker lê a tela (jogos, vídeos, sites), reconhece os caracteres
 chineses e monta seu vocabulário com pinyin, tradução e revisão espaçada.
 .
 Os motores de OCR (RapidOCR/EasyOCR) e de voz (Kokoro/ChatTTS) são baixados
 sob demanda dentro do app. Sessão X11 recomendada para a captura de tela.
CONTROL

dpkg-deb --build --root-owner-group "$pacote" "$deb_saida"
echo "Pacote gerado: $deb_saida"
