# ----- Importações -----
import sys
import os
import json
import logging
from http.server import BaseHTTPRequestHandler, HTTPServer
import numpy as np
import cv2

# Garante que a pasta atual (python_backend) está no path do Python
sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))

from ocr.OcrService import OcrService
from ocr.GerenciadorModelosModule import GerenciadorModelos


# ----- Configurações e Logs -----
logging.basicConfig(level=logging.INFO)
registradorLogs = logging.getLogger(__name__)

# Versão do contrato da API de OCR (ver docs/CONTRATO-OCR.md). O app (Go) confere este número no
# healthcheck e recusa um sidecar de contrato incompatível. Incremente ao mudar o contrato de forma
# quebrável (endpoints, headers, formato de resposta).
VERSAO_CONTRATO_OCR = 1

servicoOcr = OcrService()
gerenciadorModelos = GerenciadorModelos()


# ----- Handlers -----
class RequisicaoOcrHandler(BaseHTTPRequestHandler):

    def _enviarCabecalhosCors(self):
        self.send_header('Access-Control-Allow-Origin', '*')
        self.send_header('Access-Control-Allow-Methods', 'GET, POST, OPTIONS')
        self.send_header('Access-Control-Allow-Headers', 'Content-Type, X-Ocr-Model, X-Ocr-Device, X-Ocr-Hardware, X-Ocr-Threads, X-Ocr-Max-Side')


    def do_OPTIONS(self):
        self.send_response(200)
        self._enviarCabecalhosCors()
        self.end_headers()


    def _responderJson(self, dados, status=200):
        self.send_response(status)
        self.send_header('Content-Type', 'application/json')
        self._enviarCabecalhosCors()
        self.end_headers()
        self.wfile.write(json.dumps(dados).encode('utf-8'))


    def do_GET(self):
        if self.path == '/api/health':
            # Sinaliza que o motor de OCR está no ar e informa a versão do contrato que ele fala.
            # O Go usa isto para saber que o sidecar subiu antes de marcá-lo pronto.
            self._responderJson({
                "status": "ok",
                "servico": "ocr",
                "motor": "RapidOCR",
                "versaoContrato": VERSAO_CONTRATO_OCR,
            })
            return

        if self.path == '/api/hardware':
            nomeCpu, listaGpus = servicoOcr._detectarHardware()
            self._responderJson({"cpu": nomeCpu, "gpus": listaGpus})
            return

        if self.path == '/api/modelos':
            self._responderJson(gerenciadorModelos.listar())
            return

        self.send_response(404)
        self.end_headers()


    def do_POST(self):
        # Download e remoção de modelos são feitos pelo Go (escreve no AppData real, fora do sandbox
        # do Python da Store). Aqui o Python só lê os modelos e expõe o catálogo em GET /api/modelos.
        if self.path != '/api/ocr':
            self.send_response(404)
            self.end_headers()
            return

        # Pegando configurações dos headers injetados pelo Go
        from principal import ConstantesModule
        
        if "X-Ocr-Model" in self.headers:
            ConstantesModule.MODELO_OCR = self.headers.get("X-Ocr-Model")
            
        if "X-Ocr-Device" in self.headers:
            ConstantesModule.DISPOSITIVO_OCR = self.headers.get("X-Ocr-Device")

        if "X-Ocr-Hardware" in self.headers:
            ConstantesModule.HARDWARE_SELECIONADO = self.headers.get("X-Ocr-Hardware")

        if "X-Ocr-Threads" in self.headers:
            ConstantesModule.THREADS_CPU_OCR = int(self.headers.get("X-Ocr-Threads"))
            
        if "X-Ocr-Max-Side" in self.headers:
            ConstantesModule.LIMITE_LADO_MAIOR_OCR = int(self.headers.get("X-Ocr-Max-Side"))
        
        tamanhoConteudo = int(self.headers['Content-Length'])
        dadosPost = self.rfile.read(tamanhoConteudo)
        
        try:
            # Espera bytes brutos do PNG vindo do frontend em Go
            vetorNp = np.frombuffer(dadosPost, np.uint8)
            imagem = cv2.imdecode(vetorNp, cv2.IMREAD_COLOR)
            
            if imagem is None:
                raise ValueError("Falha ao decodificar a imagem")
            
            # Converte de BGR para RGB (esperado pelo OcrService)
            imagemRgb = cv2.cvtColor(imagem, cv2.COLOR_BGR2RGB)
            
            resultados = servicoOcr.ReconhecerCaracteresChineses(imagemRgb)
            
            respostaJson = [
                {
                    "texto": d.texto,
                    "confianca": d.confianca,
                    "caixa": d.caixa
                } for d in resultados
            ]
            
            self.send_response(200)
            self.send_header('Content-Type', 'application/json')
            self._enviarCabecalhosCors()
            self.end_headers()
            self.wfile.write(json.dumps(respostaJson).encode('utf-8'))
            
        except Exception as erro:
            registradorLogs.error(f"Erro ao processar OCR: {erro}")
            self.send_response(500)
            self.end_headers()
            self.wfile.write(json.dumps({"error": str(erro)}).encode('utf-8'))


# ----- Execução -----
def iniciarServidor(porta=None):
    # A porta é definida dinamicamente pelo orquestrador (main.go) via HANZITRACKER_OCR_PORT;
    # o 8080 é apenas um fallback para execução avulsa do servidor.
    if porta is None:
        porta = int(os.environ.get("HANZITRACKER_OCR_PORT", "8080"))
    enderecoServidor = ('', porta)
    servidorHttp = HTTPServer(enderecoServidor, RequisicaoOcrHandler)
    registradorLogs.info(f"Iniciando Microserviço Python de OCR na porta {porta}...")
    servidorHttp.serve_forever()


if __name__ == '__main__':
    iniciarServidor()

