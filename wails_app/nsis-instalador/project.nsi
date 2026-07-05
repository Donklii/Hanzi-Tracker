Unicode true

####
## Baseado no template padrão do Wails (v2.12.0, pkg/buildassets/build/windows/installer/project.nsi),
## com UMA adição: uma tela de escolha de motores de OCR/TTS antes da instalação (ver a seção
## "Tela de escolha de motores" abaixo). Nenhum motor é embutido no instalador — a escolha só decide
## QUAL motor o app baixa sozinho no primeiro start (o download-sob-demanda já existente em
## motores.go/bootstrapMotorPadrao continua fazendo o trabalho pesado). Ver docs/PUBLICAR-APP.md.
##
## Este arquivo é copiado para build/windows/installer/project.nsi pela CI (e pelo BUILD.md, para quem
## builda local) ANTES de "wails build -nsis" — wails_app/build/ inteiro é gerado/gitignored, então o
## fonte deste template vive aqui, fora do caminho que o Wails regenera.
####

!include "wails_tools.nsh"

# The version information for this two must consist of 4 parts
VIProductVersion "${INFO_PRODUCTVERSION}.0"
VIFileVersion    "${INFO_PRODUCTVERSION}.0"

VIAddVersionKey "CompanyName"     "${INFO_COMPANYNAME}"
VIAddVersionKey "FileDescription" "${INFO_PRODUCTNAME} Installer"
VIAddVersionKey "ProductVersion"  "${INFO_PRODUCTVERSION}"
VIAddVersionKey "FileVersion"     "${INFO_PRODUCTVERSION}"
VIAddVersionKey "LegalCopyright"  "${INFO_COPYRIGHT}"
VIAddVersionKey "ProductName"     "${INFO_PRODUCTNAME}"

# Enable HiDPI support. https://nsis.sourceforge.io/Reference/ManifestDPIAware
ManifestDPIAware true

!include "MUI.nsh"
!include "nsDialogs.nsh"
!include "LogicLib.nsh"

!define MUI_ICON "..\icon.ico"
!define MUI_UNICON "..\icon.ico"
!define MUI_FINISHPAGE_NOAUTOCLOSE # Wait on the INSTFILES page so the user can take a look into the details of the installation steps
!define MUI_ABORTWARNING # This will warn the user if they exit from the installer.

# ----- Tela de escolha de motores (OCR obrigatório + Voz opcional) -----
# Variáveis dos controles (Pop dos NSD_Create*) e do resultado escolhido ao sair da página.
Var DialogMotores
Var RadioOcrRapid
Var RadioOcrTesseract
Var RadioOcrEasyOcr
Var RadioTtsNenhum
Var RadioTtsKokoro
Var RadioTtsChatTts
Var SelMotorOcr
Var SelMotorTts

Function PaginaEscolhaMotores
    nsDialogs::Create 1018
    Pop $DialogMotores
    ${If} $DialogMotores == error
        Abort
    ${EndIf}

    ${NSD_CreateLabel} 0 0 100% 24u "Escolha o motor de reconhecimento de texto (OCR). Ele será baixado automaticamente na primeira abertura do programa — nada é instalado agora."
    Pop $0

    ${NSD_CreateRadioButton} 10 26u 100% 12u "RapidOCR — recomendado (leve, com aceleração de GPU quando disponível)"
    Pop $RadioOcrRapid
    ${NSD_SetState} $RadioOcrRapid ${BST_CHECKED}

    ${NSD_CreateRadioButton} 10 40u 100% 12u "Tesseract (apenas CPU)"
    Pop $RadioOcrTesseract

    ${NSD_CreateRadioButton} 10 54u 100% 12u "EasyOCR (apenas CPU; exige baixar um modelo extra depois)"
    Pop $RadioOcrEasyOcr

    ${NSD_CreateLabel} 0 78u 100% 24u "Escolha o motor de leitura em voz alta (opcional — pode ativar depois em Configurações):"
    Pop $0

    ${NSD_CreateRadioButton} 10 104u 100% 12u "Nenhum agora (ativar depois, se quiser)"
    Pop $RadioTtsNenhum
    ${NSD_SetState} $RadioTtsNenhum ${BST_CHECKED}

    ${NSD_CreateRadioButton} 10 118u 100% 12u "Kokoro-82M — leve e rápido"
    Pop $RadioTtsKokoro

    ${NSD_CreateRadioButton} 10 132u 100% 12u "ChatTTS — voz mais natural, porém mais pesado"
    Pop $RadioTtsChatTts

    nsDialogs::Show
FunctionEnd

Function PaginaEscolhaMotoresSair
    StrCpy $SelMotorOcr "RapidOCR"
    ${NSD_GetState} $RadioOcrTesseract $0
    ${If} $0 == ${BST_CHECKED}
        StrCpy $SelMotorOcr "Tesseract"
    ${EndIf}
    ${NSD_GetState} $RadioOcrEasyOcr $0
    ${If} $0 == ${BST_CHECKED}
        StrCpy $SelMotorOcr "EasyOCR"
    ${EndIf}

    StrCpy $SelMotorTts ""
    ${NSD_GetState} $RadioTtsKokoro $0
    ${If} $0 == ${BST_CHECKED}
        StrCpy $SelMotorTts "Kokoro-82M"
    ${EndIf}
    ${NSD_GetState} $RadioTtsChatTts $0
    ${If} $0 == ${BST_CHECKED}
        StrCpy $SelMotorTts "ChatTTS"
    ${EndIf}
FunctionEnd

!insertmacro MUI_PAGE_WELCOME # Welcome to the installer page.
# !insertmacro MUI_PAGE_LICENSE "resources\eula.txt" # Adds a EULA page to the installer
!insertmacro MUI_PAGE_DIRECTORY # In which folder install page.
Page custom PaginaEscolhaMotores PaginaEscolhaMotoresSair # Escolha do motor de OCR/voz.
!insertmacro MUI_PAGE_INSTFILES # Installing page.
!insertmacro MUI_PAGE_FINISH # Finished installation page.

!insertmacro MUI_UNPAGE_INSTFILES # Uinstalling page

!insertmacro MUI_LANGUAGE "PortugueseBR" # Set the Language of the installer

## The following two statements can be used to sign the installer and the uninstaller. The path to the binaries are provided in %1
#!uninstfinalize 'signtool --file "%1"'
#!finalize 'signtool --file "%1"'

Name "${INFO_PRODUCTNAME}"
OutFile "..\..\bin\${INFO_PROJECTNAME}-${ARCH}-installer.exe" # Name of the installer's file.
InstallDir "$PROGRAMFILES64\${INFO_COMPANYNAME}\${INFO_PRODUCTNAME}" # Default installing folder ($PROGRAMFILES is Program Files folder).
ShowInstDetails show # This will always show the installation details.

Function .onInit
   !insertmacro wails.checkArchitecture
FunctionEnd

Section
    !insertmacro wails.setShellContext

    !insertmacro wails.webview2runtime

    SetOutPath $INSTDIR

    !insertmacro wails.files

    CreateShortcut "$SMPROGRAMS\${INFO_PRODUCTNAME}.lnk" "$INSTDIR\${PRODUCT_EXECUTABLE}"
    CreateShortCut "$DESKTOP\${INFO_PRODUCTNAME}.lnk" "$INSTDIR\${PRODUCT_EXECUTABLE}"

    !insertmacro wails.associateFiles
    !insertmacro wails.associateCustomProtocols

    ; Grava a escolha de motores em %APPDATA%\HanziTracker para o app ler UMA VEZ no primeiro start
    ; (ver wails_app/instalador.go) e então baixar sozinho só o motor escolhido — nenhum motor é
    ; embutido neste instalador.
    CreateDirectory "$APPDATA\HanziTracker"
    FileOpen $4 "$APPDATA\HanziTracker\instalador_escolha.json" w
    FileWrite $4 '{"motorOcr":"$SelMotorOcr","motorTts":"$SelMotorTts"}'
    FileClose $4

    !insertmacro wails.writeUninstaller
SectionEnd

Section "uninstall"
    !insertmacro wails.setShellContext

    RMDir /r "$AppData\${PRODUCT_EXECUTABLE}" # Remove the WebView2 DataPath

    RMDir /r $INSTDIR

    Delete "$SMPROGRAMS\${INFO_PRODUCTNAME}.lnk"
    Delete "$DESKTOP\${INFO_PRODUCTNAME}.lnk"

    !insertmacro wails.unassociateFiles
    !insertmacro wails.unassociateCustomProtocols

    !insertmacro wails.deleteUninstaller
SectionEnd
