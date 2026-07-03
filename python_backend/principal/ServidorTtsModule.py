# ----- Importações -----
import json
import logging
import os
from http.server import BaseHTTPRequestHandler, HTTPServer

from principal import ConstantesModule


# ----- Configurações e Logs -----
logging.basicConfig(level=logging.INFO)
registradorLogs = logging.getLogger(__name__)

# Versão do contrato da API de TTS (ver docs/CONTRATO-TTS.md). O app (Go) confere este número no
# healthcheck e recusa um sidecar de contrato incompatível. Incremente ao mudar o contrato de forma
# quebrável (endpoints, formato de requisição/resposta).
VERSAO_CONTRATO_TTS = 1


# ----- Servidor HTTP do contrato de TTS -----
# Scaffolding compartilhado por TODOS os motores de voz (Kokoro-82M, ChatTTS…): cada sidecar é um
# entry fino (kokoro_server.py, chattts_server.py) que injeta aqui o SEU serviço de TTS. Assim o
# contrato HTTP (docs/CONTRATO-TTS.md) é implementado uma única vez e todo motor o fala por
# construção — espelha o ServidorOcrModule.py do OCR.

def _criarHandler(servicoTts):
    """Fabrica a classe handler ligada ao serviço de TTS do motor deste sidecar."""

    class RequisicaoTtsHandler(BaseHTTPRequestHandler):

        def _enviarCabecalhosCors(self):
            self.send_header('Access-Control-Allow-Origin', '*')
            self.send_header('Access-Control-Allow-Methods', 'GET, POST, OPTIONS')
            self.send_header('Access-Control-Allow-Headers', 'Content-Type')


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
                # Sinaliza que o motor de TTS está no ar e informa a versão do contrato que ele fala.
                # O Go usa isto para saber que o sidecar subiu antes de mandar sintetizar. O health
                # NÃO carrega o modelo (a carga é preguiçosa, na primeira síntese) — assim ele
                # responde rápido mesmo quando os pesos ainda nem foram baixados.
                self._responderJson({
                    "status": "ok",
                    "servico": "tts",
                    "motor": ConstantesModule.obterNomeMotor(),
                    "versaoContrato": VERSAO_CONTRATO_TTS,
                })
                return

            self.send_response(404)
            self.end_headers()


        def do_POST(self):
            if self.path != '/api/tts':
                self.send_response(404)
                self.end_headers()
                return

            try:
                tamanhoConteudo = int(self.headers['Content-Length'])
                corpo = json.loads(self.rfile.read(tamanhoConteudo).decode('utf-8'))
                texto = (corpo.get("texto") or "").strip()
                if not texto:
                    raise ValueError("campo 'texto' vazio na requisição de TTS")

                # A primeira síntese pode demorar: carga do modelo + download dos pesos do
                # Hugging Face (o cache HF é redirecionado para modelos/<Motor>/hf pelo entry).
                audioWav = servicoTts.SintetizarFala(
                    texto, aoProgredir=lambda msg: registradorLogs.info("[tts] %s", msg)
                )

                self.send_response(200)
                self.send_header('Content-Type', 'audio/wav')
                self._enviarCabecalhosCors()
                self.end_headers()
                self.wfile.write(audioWav)

            except Exception as erro:
                registradorLogs.error(f"Erro ao sintetizar fala: {erro}")
                self._responderJson({"error": str(erro)}, status=500)

    return RequisicaoTtsHandler


# ----- Execução -----
def iniciarServidorTts(servicoTts, porta=None):
    """Sobe o microserviço HTTP deste motor de voz na porta do app (HANZITRACKER_TTS_PORT)."""
    # A porta é definida dinamicamente pelo app (motor_tts.go) via HANZITRACKER_TTS_PORT;
    # o 8090 é apenas um fallback para execução avulsa do servidor.
    if porta is None:
        porta = int(os.environ.get("HANZITRACKER_TTS_PORT", "8090"))
    enderecoServidor = ('', porta)
    servidorHttp = HTTPServer(enderecoServidor, _criarHandler(servicoTts))
    registradorLogs.info(f"Iniciando Microserviço de TTS ({ConstantesModule.obterNomeMotor()}) na porta {porta}...")
    servidorHttp.serve_forever()
