# ----- Importações -----
import sys
import os

# Garante que python_backend (2 níveis acima: motores/zipformer_streaming/zipformer_streaming_server.py)
# está no path do Python, para os imports absolutos (stt.*, principal.*, motores.zipformer_streaming.*)
# resolverem.
sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__)))))

# Em execução avulsa (dev, sem o Go), assume o próprio nome de catálogo — o Go sempre injeta
# HANZITRACKER_MOTOR ao subir o sidecar (sidecar.go), então o setdefault não o sobrescreve.
os.environ.setdefault("HANZITRACKER_MOTOR", "Zipformer-ZH-Streaming")

from principal import ConstantesModule

# Pesos: o Zipformer os baixa do Hugging Face na primeira transcrição (ou no /api/stt/preparar).
# Redireciona o cache do HF para a subpasta de modelos DESTE motor (motores_stt/<Motor>/modelos/hf)
# ANTES de qualquer import do huggingface_hub, para os pesos morarem DENTRO da pasta do próprio
# motor no AppData — mensuráveis/limpáveis pela aba Armazenamento junto com o executável (mesma
# estrutura do TTS).
os.environ.setdefault(
    "HF_HOME",
    os.path.join(
        ConstantesModule.obterPastaDados(), "motores_stt", ConstantesModule.obterNomeMotor(), "modelos", "hf"
    ),
)

from motores.zipformer_streaming.ZipformerStreamingSttService import ZipformerStreamingSttService
from principal.ServidorSttModule import iniciarServidorStt


# ----- Execução -----
# Entry do sidecar Zipformer-ZH-Streaming (motor de reconhecimento de voz streaming): o servidor
# HTTP do contrato vive em ServidorSttModule; aqui só se injeta o serviço deste motor. Congelado
# por zipformer_streaming_server.spec.
if __name__ == '__main__':
    iniciarServidorStt(ZipformerStreamingSttService())
