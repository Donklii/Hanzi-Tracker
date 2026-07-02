# ----- Importações -----
import sys
import os

# Garante que python_backend (2 níveis acima: motores/easyocr/easyocr_server.py) está no path do
# Python, para os imports absolutos (ocr.*, principal.*, motores.easyocr.*) resolverem.
sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__)))))

from motores.easyocr.EasyOcrService import EasyOcrService
from motores.easyocr import ModelosManifesto
from ocr.GerenciadorModelosModule import GerenciadorModelos
from principal.ServidorOcrModule import iniciarServidor


# ----- Execução -----
# Entry do sidecar EasyOCR: o servidor HTTP do contrato vive em ServidorOcrModule; aqui só se
# injeta o serviço e o gerenciador de modelos deste motor. Congelado por easyocr_server.spec.
if __name__ == '__main__':
    iniciarServidor(EasyOcrService(), GerenciadorModelos(ModelosManifesto))
