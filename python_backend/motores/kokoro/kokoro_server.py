# ----- Importações -----
import sys
import os

# Garante que python_backend (2 níveis acima: motores/kokoro/kokoro_server.py) está no path do
# Python, para os imports absolutos (tts.*, principal.*, motores.kokoro.*) resolverem.
sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__)))))

# Em execução avulsa (dev, sem o Go), assume o próprio nome de catálogo — o Go sempre injeta
# HANZITRACKER_MOTOR ao subir o sidecar (motor_tts.go), então o setdefault não o sobrescreve.
os.environ.setdefault("HANZITRACKER_MOTOR", "Kokoro-82M")

from principal import ConstantesModule

# Pesos: o Kokoro os baixa do Hugging Face na primeira síntese. Redireciona o cache do HF para a
# subpasta de modelos DESTE motor (motores_tts/<Motor>/modelos/hf) ANTES de qualquer import do
# huggingface_hub, para os pesos morarem DENTRO da pasta do próprio motor no AppData — mensuráveis/
# limpáveis pela aba Armazenamento junto com o executável (mesma estrutura do OCR).
os.environ.setdefault(
    "HF_HOME",
    os.path.join(
        ConstantesModule.obterPastaDados(), "motores_tts", ConstantesModule.obterNomeMotor(), "modelos", "hf"
    ),
)

from motores.kokoro.KokoroTtsService import KokoroTtsService
from principal.ServidorTtsModule import iniciarServidorTts


# ----- Execução -----
# Entry do sidecar Kokoro-82M (motor de voz): o servidor HTTP do contrato vive em ServidorTtsModule;
# aqui só se injeta o serviço deste motor. Congelado por kokoro_server.spec.
if __name__ == '__main__':
    iniciarServidorTts(KokoroTtsService())
