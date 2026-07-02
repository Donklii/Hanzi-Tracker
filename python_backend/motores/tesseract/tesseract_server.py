# ----- Importações -----
import sys
import os

# Garante que python_backend (2 níveis acima: motores/tesseract/tesseract_server.py) está no path do
# Python, para os imports absolutos (ocr.*, principal.*, motores.tesseract.*) resolverem.
sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__)))))

from motores.tesseract.TesseractService import TesseractService
from motores.tesseract import ModelosManifesto
from ocr.GerenciadorModelosModule import GerenciadorModelos
from principal.ServidorOcrModule import iniciarServidor


# ----- Execução -----
# Entry do sidecar Tesseract: o servidor HTTP do contrato vive em ServidorOcrModule; aqui só se
# injeta o serviço e o gerenciador de modelos deste motor. Congelado por tesseract_server.spec
# (que também empacota o tesseract.exe + tessdata via build_sidecars.ps1).
if __name__ == '__main__':
    iniciarServidor(TesseractService(), GerenciadorModelos(ModelosManifesto))
