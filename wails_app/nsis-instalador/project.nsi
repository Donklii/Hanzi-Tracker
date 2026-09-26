Unicode true

####
## Baseado no template padrão do Wails (v2.12.0, pkg/buildassets/build/windows/installer/project.nsi),
## com UMA adição: TRÊS telas de escolha de motores — OCR, depois voz/TTS, depois escuta/STT — antes
## da instalação (ver a seção "Telas de escolha de motores" abaixo). Nenhum motor é embutido no
## instalador — a escolha só decide QUAL motor o app baixa sozinho no primeiro start (o
## download-sob-demanda já existente em motores.go/bootstrapMotorPadrao continua fazendo o trabalho
## pesado). Ver docs/PUBLICAR-APP.md.
##
## Este arquivo é copiado para build/windows/installer/project.nsi pela CI (e pelo BUILD.md, para quem
## builda local) ANTES de "wails build -nsis" — wails_app/build/ inteiro é gerado/gitignored, então o
## fonte deste template vive aqui, fora do caminho que o Wails regenera.
##
## IMPORTANTE: salve este arquivo em UTF-8 COM BOM. Sem o BOM, o makensis lê o fonte como ANSI
## (mesmo com "Unicode true") e todos os acentos/travessões quebram na UI do instalador.
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

# Estilo Win32 que INICIA UM NOVO GRUPO de radio buttons (aplicado ao primeiro radio de cada página).
# Cada página tem um único grupo, mas marcá-lo explicitamente garante o agrupamento correto.
!ifndef WS_GROUP
    !define WS_GROUP 0x00020000
!endif

!define MUI_ICON "..\icon.ico"
!define MUI_UNICON "..\icon.ico"
!define MUI_FINISHPAGE_NOAUTOCLOSE # Wait on the INSTFILES page so the user can take a look into the details of the installation steps
!define MUI_ABORTWARNING # This will warn the user if they exit from the installer.

# ----- Telas de escolha de motores (OCR obrigatório + Voz/TTS e Escuta/STT opcionais) -----
# Uma página custom por família de motor (OCR, TTS, STT). $DialogMotores é reaproveitado entre elas
# (as páginas são sequenciais). As Sel* guardam a escolha entre idas e voltas (Voltar/Avançar) e são
# gravadas no marcador ao final. Os Radio* são os handles dos controles (Pop dos NSD_Create*).
Var DialogMotores
Var RadioIdiomaPt
Var RadioIdiomaEn
Var SelIdioma
Var RadioOcrRapid
Var RadioOcrTesseract
Var RadioOcrEasyOcr
Var RadioTtsNenhum
Var RadioTtsKokoro
Var RadioTtsChatTts
Var RadioSttNenhum
Var RadioSttParaformer
Var RadioSttZipformer
Var SelMotorOcr
Var SelMotorTts
Var SelMotorStt

# ----- Página 0: idioma das traduções (Português pré-selecionado; inglês é o fallback) -----
# Grava idiomaTraducao no mesmo marcador dos motores; o app aplica no primeiro start (ver instalador.go).
Function PaginaEscolhaIdioma
    !insertmacro MUI_HEADER_TEXT "Idioma das traduções" "Idioma das definições, dicas e frases do dicionário. Dá para trocar depois em Configurações."

    nsDialogs::Create 1018
    Pop $DialogMotores
    ${If} $DialogMotores == error
        Abort
    ${EndIf}

    ${NSD_CreateLabel} 0 0 100% 24u "Em qual idioma você quer ver as definições, dicas e frases dos caracteres? O inglês é usado como reserva quando algo ainda não foi traduzido."
    Pop $0

    ${NSD_CreateRadioButton} 10 30u 100% 12u "Português (Brasil) — recomendado"
    Pop $RadioIdiomaPt
    ${NSD_AddStyle} $RadioIdiomaPt ${WS_GROUP}

    ${NSD_CreateRadioButton} 10 43u 100% 12u "English"
    Pop $RadioIdiomaEn

    # Reflete a escolha anterior ao reentrar na página; vazio = primeiro acesso = Português.
    ${If} $SelIdioma == "en"
        ${NSD_SetState} $RadioIdiomaEn ${BST_CHECKED}
    ${Else}
        ${NSD_SetState} $RadioIdiomaPt ${BST_CHECKED}
    ${EndIf}

    nsDialogs::Show
FunctionEnd

Function PaginaEscolhaIdiomaSair
    StrCpy $SelIdioma "pt-BR"
    ${NSD_GetState} $RadioIdiomaEn $0
    ${If} $0 == ${BST_CHECKED}
        StrCpy $SelIdioma "en"
    ${EndIf}
FunctionEnd

# ----- Página 1: motor de OCR (obrigatório, RapidOCR pré-selecionado) -----
Function PaginaEscolhaOcr
    # Sem isto a página custom herda o cabeçalho da página anterior ("Escolha o Local da Instalação").
    !insertmacro MUI_HEADER_TEXT "Motor de OCR" "Reconhecimento de texto — o app baixa o motor escolhido sozinho na primeira abertura. Nada é instalado agora."

    nsDialogs::Create 1018
    Pop $DialogMotores
    ${If} $DialogMotores == error
        Abort
    ${EndIf}

    ${NSD_CreateLabel} 0 0 100% 24u "Escolha o motor de reconhecimento de texto (OCR). Ele é obrigatório e é baixado automaticamente na primeira vez que você abrir o app."
    Pop $0

    ${NSD_CreateRadioButton} 10 30u 100% 12u "RapidOCR — recomendado (leve, com aceleração de GPU quando disponível)"
    Pop $RadioOcrRapid
    ${NSD_AddStyle} $RadioOcrRapid ${WS_GROUP}

    ${NSD_CreateRadioButton} 10 43u 100% 12u "Tesseract (apenas CPU)"
    Pop $RadioOcrTesseract

    ${NSD_CreateRadioButton} 10 56u 100% 12u "EasyOCR (apenas CPU; exige baixar um modelo extra depois)"
    Pop $RadioOcrEasyOcr

    # Reflete a escolha anterior ao reentrar na página (Voltar/Avançar); vazio = primeiro acesso = RapidOCR.
    ${If} $SelMotorOcr == "Tesseract"
        ${NSD_SetState} $RadioOcrTesseract ${BST_CHECKED}
    ${ElseIf} $SelMotorOcr == "EasyOCR"
        ${NSD_SetState} $RadioOcrEasyOcr ${BST_CHECKED}
    ${Else}
        ${NSD_SetState} $RadioOcrRapid ${BST_CHECKED}
    ${EndIf}

    nsDialogs::Show
FunctionEnd

Function PaginaEscolhaOcrSair
    StrCpy $SelMotorOcr "RapidOCR"
    ${NSD_GetState} $RadioOcrTesseract $0
    ${If} $0 == ${BST_CHECKED}
        StrCpy $SelMotorOcr "Tesseract"
    ${EndIf}
    ${NSD_GetState} $RadioOcrEasyOcr $0
    ${If} $0 == ${BST_CHECKED}
        StrCpy $SelMotorOcr "EasyOCR"
    ${EndIf}
FunctionEnd

# ----- Página 2: motor de voz/TTS (opcional, "Nenhum agora" pré-selecionado) -----
Function PaginaEscolhaTts
    !insertmacro MUI_HEADER_TEXT "Motor de voz (leitura em voz alta)" "Opcional — dá para ativar ou trocar depois em Configurações. Nada é instalado agora."

    nsDialogs::Create 1018
    Pop $DialogMotores
    ${If} $DialogMotores == error
        Abort
    ${EndIf}

    ${NSD_CreateLabel} 0 0 100% 24u "Motor de leitura em voz alta (TTS). É opcional; o motor escolhido é baixado sozinho na primeira vez que você usar a leitura em voz alta."
    Pop $0

    ${NSD_CreateRadioButton} 10 30u 100% 12u "Nenhum agora"
    Pop $RadioTtsNenhum
    ${NSD_AddStyle} $RadioTtsNenhum ${WS_GROUP}

    ${NSD_CreateRadioButton} 10 43u 100% 12u "Kokoro-82M — leve e rápido"
    Pop $RadioTtsKokoro

    ${NSD_CreateRadioButton} 10 56u 100% 12u "ChatTTS — voz mais natural, porém mais pesado"
    Pop $RadioTtsChatTts

    # Reflete a escolha anterior ao reentrar na página; vazio = primeiro acesso = "Nenhum agora".
    ${If} $SelMotorTts == "Kokoro-82M"
        ${NSD_SetState} $RadioTtsKokoro ${BST_CHECKED}
    ${ElseIf} $SelMotorTts == "ChatTTS"
        ${NSD_SetState} $RadioTtsChatTts ${BST_CHECKED}
    ${Else}
        ${NSD_SetState} $RadioTtsNenhum ${BST_CHECKED}
    ${EndIf}

    nsDialogs::Show
FunctionEnd

Function PaginaEscolhaTtsSair
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

# ----- Página 3: motor de escuta/STT (opcional, "Nenhum agora" pré-selecionado) -----
Function PaginaEscolhaStt
    !insertmacro MUI_HEADER_TEXT "Motor de reconhecimento de fala" "Opcional — usado na revisão de pronúncia. Dá para ativar ou trocar depois em Configurações."

    nsDialogs::Create 1018
    Pop $DialogMotores
    ${If} $DialogMotores == error
        Abort
    ${EndIf}

    ${NSD_CreateLabel} 0 0 100% 24u "Motor de reconhecimento de fala (STT), usado na revisão de pronúncia. É opcional; o motor escolhido é baixado sozinho na primeira escuta."
    Pop $0

    ${NSD_CreateRadioButton} 10 30u 100% 12u "Nenhum agora"
    Pop $RadioSttNenhum
    ${NSD_AddStyle} $RadioSttNenhum ${WS_GROUP}

    ${NSD_CreateRadioButton} 10 43u 100% 12u "Paraformer-ZH — mais preciso (mandarim)"
    Pop $RadioSttParaformer

    ${NSD_CreateRadioButton} 10 56u 100% 12u "Zipformer-ZH Streaming — transcrição em tempo real (mandarim)"
    Pop $RadioSttZipformer

    # Reflete a escolha anterior ao reentrar na página; vazio = primeiro acesso = "Nenhum agora".
    ${If} $SelMotorStt == "Paraformer-ZH"
        ${NSD_SetState} $RadioSttParaformer ${BST_CHECKED}
    ${ElseIf} $SelMotorStt == "Zipformer-ZH-Streaming"
        ${NSD_SetState} $RadioSttZipformer ${BST_CHECKED}
    ${Else}
        ${NSD_SetState} $RadioSttNenhum ${BST_CHECKED}
    ${EndIf}

    nsDialogs::Show
FunctionEnd

Function PaginaEscolhaSttSair
    StrCpy $SelMotorStt ""
    ${NSD_GetState} $RadioSttParaformer $0
    ${If} $0 == ${BST_CHECKED}
        StrCpy $SelMotorStt "Paraformer-ZH"
    ${EndIf}
    ${NSD_GetState} $RadioSttZipformer $0
    ${If} $0 == ${BST_CHECKED}
        StrCpy $SelMotorStt "Zipformer-ZH-Streaming"
    ${EndIf}
FunctionEnd

!insertmacro MUI_PAGE_WELCOME # Welcome to the installer page.
# !insertmacro MUI_PAGE_LICENSE "resources\eula.txt" # Adds a EULA page to the installer
!insertmacro MUI_PAGE_DIRECTORY # In which folder install page.
Page custom PaginaEscolhaIdioma PaginaEscolhaIdiomaSair # Escolha do idioma das traduções.
Page custom PaginaEscolhaOcr PaginaEscolhaOcrSair # Escolha do motor de OCR.
Page custom PaginaEscolhaTts PaginaEscolhaTtsSair # Escolha do motor de voz/TTS.
Page custom PaginaEscolhaStt PaginaEscolhaSttSair # Escolha do motor de escuta/STT.
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
    ; embutido neste instalador. O wails.setShellContext lá em cima pôs o contexto em "all"
    ; (instalação por máquina), e com ele $APPDATA seria C:\ProgramData — pasta que o app não lê e
    ; onde, rodando sem admin, nem conseguiria apagar o marcador. Por isso o contexto vira "current"
    ; (o AppData do usuário que instalou, o mesmo das configurações) só durante a gravação.
    ${IfNot} ${Silent}
        SetShellVarContext current
        CreateDirectory "$APPDATA\HanziTracker"
        FileOpen $4 "$APPDATA\HanziTracker\instalador_escolha.json" w
        FileWrite $4 '{"motorOcr":"$SelMotorOcr","motorTts":"$SelMotorTts","motorStt":"$SelMotorStt","idiomaTraducao":"$SelIdioma"}'
        FileClose $4
        !insertmacro wails.setShellContext
    ${EndIf}

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
