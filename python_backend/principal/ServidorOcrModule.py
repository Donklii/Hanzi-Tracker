# ----- Importações -----
import json
import logging
import os
from http.server import BaseHTTPRequestHandler, HTTPServer

import numpy as np
import cv2

from principal import ConstantesModule


# ----- Configurações e Logs -----
logging.basicConfig(level=logging.INFO)
registradorLogs = logging.getLogger(__name__)

# Versão do contrato da API de OCR (ver docs/CONTRATO-OCR.md). O app (Go) confere este número no
# healthcheck e recusa um sidecar de contrato incompatível. Incremente ao mudar o contrato de forma
# quebrável (endpoints, headers, formato de resposta).
VERSAO_CONTRATO_OCR = 1


# ----- Servidor HTTP do contrato de OCR -----
# Scaffolding compartilhado por TODOS os motores (RapidOCR, EasyOCR, Tesseract…): cada sidecar é um
# entry fino (server.py, easyocr_server.py, tesseract_server.py) que injeta aqui o SEU serviço de OCR
# e o SEU gerenciador de modelos. Assim o contrato HTTP (docs/CONTRATO-OCR.md) é implementado uma
# única vez e todo motor o fala por construção.

def _criarHandler(servicoOcr, gerenciadorModelos):
    """Fabrica a classe handler ligada ao serviço/gerenciador do motor deste sidecar."""

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
                    "motor": ConstantesModule.obterNomeMotor(),
                    "versaoContrato": VERSAO_CONTRATO_OCR,
                })
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

                # Converte de BGR para RGB (esperado pelos serviços de OCR)
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

    return RequisicaoOcrHandler


# ----- Execução -----
def iniciarServidor(servicoOcr, gerenciadorModelos, porta=None):
    """Sobe o microserviço HTTP deste motor na porta do orquestrador (HANZITRACKER_OCR_PORT)."""
    # A porta é definida dinamicamente pelo orquestrador (main.go) via HANZITRACKER_OCR_PORT;
    # o 8080 é apenas um fallback para execução avulsa do servidor.
    if porta is None:
        porta = int(os.environ.get("HANZITRACKER_OCR_PORT", "8080"))
    enderecoServidor = ('', porta)
    servidorHttp = HTTPServer(enderecoServidor, _criarHandler(servicoOcr, gerenciadorModelos))
    registradorLogs.info(f"Iniciando Microserviço de OCR ({ConstantesModule.obterNomeMotor()}) na porta {porta}...")
    servidorHttp.serve_forever()
