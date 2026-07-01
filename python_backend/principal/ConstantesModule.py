# ----- Importações -----
import os

# ----- Configurações Gerais -----
TESTANDO = False

# ----- Intervalos e Timers -----
INTERVALO_CAPTURA_SEGUNDOS = 10
INTERVALO_MINIMO_SEGUNDOS = 3
INTERVALO_MAXIMO_SEGUNDOS = 60
INTERVALO_ATUALIZACAO_FILA_MS = 100

# ----- Configurações do OCR -----
# OCR feito com RapidOCR (onnxruntime): roda os modelos leves do PP-OCR em CPU, rápido
# (~1s a 1080p) e sem o bug de inferência do paddlepaddle 3.x que travava os modelos
# mobile e o mkldnn (ConvertPirAttribute2RuntimeAttribute).
CONFIANCA_MINIMA_OCR = 0.5
# Limita as threads do onnxruntime para não saturar a CPU (preserva o FPS de jogos em foco).
THREADS_CPU_OCR = 4
# Hardware selecionado para inferência: CPU ou GPU específica detectada
HARDWARE_SELECIONADO = "CPU"
# Dispositivo de execução do OCR: "cpu", "cuda", ou "directml" (definido de acordo com o hardware e a API)
DISPOSITIVO_OCR = "cpu"
# Modelo de OCR a utilizar: "RapidOCR", "RapidOCR Server", ou "EasyOCR (Download)"
MODELO_OCR = "RapidOCR"
# Limite do lado maior da captura antes do OCR (em px). 0 = resolução nativa do monitor (sem redução).
LIMITE_LADO_MAIOR_OCR = 0

# ----- Limitador de Uso de CPU -----
LIMITAR_POR_USO_CPU = False
USO_MAXIMO_CPU_PERCENT = 80.0

# ----- Limitador de Uso de GPU -----
LIMITAR_POR_USO_GPU = False
USO_MAXIMO_GPU_PERCENT = 80.0

# ----- Hover (palavra mais próxima do mouse) -----
# Distância máxima (px) entre o cursor e a caixa de uma palavra para ela ser exibida no
# painel de proximidade; além disso, o cursor é considerado longe de qualquer texto.
DISTANCIA_MAXIMA_HOVER_PX = 220
# Frequência de atualização do painel de proximidade conforme o mouse se move.
INTERVALO_ATUALIZACAO_HOVER_MS = 120
# Habilita o pop-up de tradução flutuante que segue o cursor
HABILITAR_POPUP_HOVER = False
# Tempo (dwell) que o cursor precisa ficar parado para abrir o pop-up (em ms)
TEMPO_PARADO_POPUP_MS = 500

# ----- Atalhos Globais (Hotkeys) -----
ATALHO_ESCANEAR = "ctrl+shift+e"
ATALHO_POPUP_TODOS = "ctrl+shift+t"
ATALHO_MARCAR_ESTUDO = "ctrl+shift+m"

# ----- Detecção de Mudança de Tela -----
# Amostra esparsa (1 a cada N pixels) usada para decidir se a tela mudou desde a última
# varredura automática; telas idênticas pulam o OCR e economizam CPU.
PASSO_AMOSTRA_FINGERPRINT = 16

# ----- Interface -----
LARGURA_JANELA_PADRAO = 1024
ALTURA_JANELA_PADRAO = 720

# ----- Exportação -----
NOME_PADRAO_EXPORTACAO = "palavras_chinesas"

# ----- Armazenamento (AppData) -----
# Pasta dedicada criada sob demanda em %APPDATA%\HanziTracker para o banco de progresso
# e o arquivo de configurações editáveis pelo usuário.
NOME_PASTA_DADOS_APPDATA = "HanziTracker"
NOME_BANCO_PROGRESSO = "progresso.db"
NOME_ARQUIVO_CONFIG = "configuracoes.json"


def obterPastaDados():
    """Pasta de dados do app (modelos, logs). Prioriza HANZITRACKER_DATA_DIR, que o Go injeta com o
    caminho REAL — necessário porque o Python da Microsoft Store virtualiza o %APPDATA% para um
    sandbox, divergindo de onde o Go (não-sandbox) grava config/banco.
    """
    base = os.environ.get("HANZITRACKER_DATA_DIR")
    if not base:
        raiz = os.environ.get("APPDATA") or os.path.expanduser("~")
        base = os.path.join(raiz, NOME_PASTA_DADOS_APPDATA)
    os.makedirs(base, exist_ok=True)
    return base

METADADOS_CONFIG = {
    "INTERVALO_CAPTURA_SEGUNDOS": {
        "rotulo": "Intervalo de Varredura (s)",
        "desc": "Frequência de atualização automática da captura de tela.",
        "categoria": "Geral",
        "tipo": "slider",
        "min": 3,
        "max": 60,
        "passo": 1,
        "sufixo": " s"
    },
    "CONFIANCA_MINIMA_OCR": {
        "rotulo": "Confiança Mínima do OCR",
        "desc": "Ignora palavras detectadas com confiança menor que esta.",
        "categoria": "OCR",
        "tipo": "slider",
        "min": 0.1,
        "max": 1.0,
        "passo": 0.05,
        "sufixo": " %"
    },
    "THREADS_CPU_OCR": {
        "rotulo": "Threads do OCR",
        "desc": "Limita o número de threads do processador usadas pelo OCR.",
        "categoria": "OCR",
        "tipo": "slider",
        "min": 1,
        "max": 8,
        "passo": 1,
        "sufixo": " threads"
    },
    "HARDWARE_SELECIONADO": {
        "rotulo": "Hardware de Inferência",
        "desc": "Selecione o processador ou placa de vídeo para rodar o OCR.",
        "categoria": "OCR",
        "tipo": "dropdown",
        "opcoes": []  # Populado dinamicamente no carregamento do diálogo
    },
    "DISPOSITIVO_OCR": {
        "rotulo": "API do Dispositivo",
        "desc": "Selecione a API de aceleração para placas Nvidia (CUDA ou DirectML).",
        "categoria": "OCR",
        "tipo": "dropdown",
        "opcoes": ["cuda", "directml"]
    },
    "MODELO_OCR": {
        "rotulo": "Modelo do OCR",
        "desc": "O motor selecionado será baixado automaticamente sob demanda.",
        "categoria": "OCR",
        "tipo": "dropdown",
        "opcoes": ["RapidOCR", "RapidOCR Server", "EasyOCR (Download)"]
    },
    "LIMITE_LADO_MAIOR_OCR": {
        "rotulo": "Qualidade da Imagem (OCR)",
        "desc": "Resolução da captura enviada ao OCR, mantendo a proporção da tela. Máximo = resolução nativa do monitor.",
        "categoria": "OCR",
        "tipo": "slider",
        "min": 480,
        "max": 1920,
        "passo": 10,
        "sufixo": " px",
        "dinamico_resolucao": True
    },
    "LIMITAR_POR_USO_CPU": {
        "rotulo": "Pular se CPU Ocupada",
        "desc": "Pula varreduras automáticas se o uso de CPU passar do limite.",
        "categoria": "Desempenho",
        "tipo": "bool"
    },
    "USO_MAXIMO_CPU_PERCENT": {
        "rotulo": "Limite de Uso de CPU (%)",
        "desc": "Percentual máximo de CPU livre exigido para rodar o OCR automático.",
        "categoria": "Desempenho",
        "tipo": "slider",
        "min": 10,
        "max": 100,
        "passo": 5,
        "sufixo": " %",
        "depende_de": "LIMITAR_POR_USO_CPU"
    },
    "LIMITAR_POR_USO_GPU": {
        "rotulo": "Pular se GPU Ocupada",
        "desc": "Pula varreduras automáticas se o uso de GPU passar do limite.",
        "categoria": "Desempenho",
        "tipo": "bool"
    },
    "USO_MAXIMO_GPU_PERCENT": {
        "rotulo": "Limite de Uso de GPU (%)",
        "desc": "Percentual máximo de uso de GPU permitido para rodar o OCR automático.",
        "categoria": "Desempenho",
        "tipo": "slider",
        "min": 10,
        "max": 100,
        "passo": 5,
        "sufixo": " %",
        "depende_de": "LIMITAR_POR_USO_GPU"
    },
    "DISTANCIA_MAXIMA_HOVER_PX": {
        "rotulo": "Distância do Mouse (px)",
        "desc": "Distância máxima entre cursor e texto para tradução rápida.",
        "categoria": "Hover/Pop-up",
        "tipo": "slider",
        "min": 50,
        "max": 500,
        "passo": 10,
        "sufixo": " px"
    },
    "INTERVALO_ATUALIZACAO_HOVER_MS": {
        "rotulo": "Freq. Rastreamento (ms)",
        "desc": "Intervalo entre checagens da posição do mouse.",
        "categoria": "Hover/Pop-up",
        "tipo": "slider",
        "min": 50,
        "max": 500,
        "passo": 10,
        "sufixo": " ms"
    },
    "HABILITAR_POPUP_HOVER": {
        "rotulo": "Habilitar Pop-up de Hover",
        "desc": "Exibe pop-up flutuante quando o mouse está parado sobre texto.",
        "categoria": "Hover/Pop-up",
        "tipo": "bool"
    },
    "TEMPO_PARADO_POPUP_MS": {
        "rotulo": "Atraso do Pop-up (ms)",
        "desc": "Tempo que o mouse deve ficar parado para abrir o pop-up.",
        "categoria": "Hover/Pop-up",
        "tipo": "slider",
        "min": 100,
        "max": 2000,
        "passo": 50,
        "sufixo": " ms",
        "depende_de": "HABILITAR_POPUP_HOVER"
    },
    "ATALHO_ESCANEAR": {
        "rotulo": "Atalho: Escanear",
        "desc": "Atalho de teclado global para escanear a tela agora.",
        "categoria": "Atalhos Globais",
        "tipo": "str"
    },
    "ATALHO_POPUP_TODOS": {
        "rotulo": "Atalho: Traduzir Tudo",
        "desc": "Atalho global para traduzir na tela todos os textos localizados.",
        "categoria": "Atalhos Globais",
        "tipo": "str"
    },
    "ATALHO_MARCAR_ESTUDO": {
        "rotulo": "Atalho: Estudo Rápido",
        "desc": "Atalho global para marcar a palavra sob o mouse como 'em estudo'.",
        "categoria": "Atalhos Globais",
        "tipo": "str"
    }
}


