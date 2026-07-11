# ----- Importações -----
import json
import logging
import os
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer

from principal import ConstantesModule


# ----- Configurações e Logs -----
logging.basicConfig(level=logging.INFO)
registradorLogs = logging.getLogger(__name__)

# Versão do contrato da API de STT (ver docs/CONTRATO-STT.md). O app (Go) confere este número no
# healthcheck e recusa um sidecar de contrato incompatível. Incremente ao mudar o contrato de forma
# quebrável (endpoints, formato de requisição/resposta). O /api/stt/parcial é ADITIVO e opcional
# (o app trata 404 como "motor antigo, sem parciais"), então não exigiu bump.
VERSAO_CONTRATO_STT = 1


# ----- Servidor HTTP do contrato de STT -----
# Scaffolding compartilhado por TODOS os motores de reconhecimento de voz (Paraformer-ZH…): cada
# sidecar é um entry fino (paraformer_server.py) que injeta aqui o SEU serviço. Assim o contrato
# HTTP (docs/CONTRATO-STT.md) é implementado uma única vez e todo motor o fala por construção —
# espelha o ServidorTtsModule.py do TTS. A GRAVAÇÃO do microfone também mora aqui no sidecar
# (ServicoSttBase): a webview do Wails no Linux não tem SpeechRecognition nem getUserMedia.

def _criarHandler(servicoStt):
    """Fabrica a classe handler ligada ao serviço de STT do motor deste sidecar."""

    class RequisicaoSttHandler(BaseHTTPRequestHandler):

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
                # Sinaliza que o motor de STT está no ar e informa a versão do contrato que ele
                # fala. O health NÃO carrega o modelo (a carga é preguiçosa, na primeira
                # transcrição ou no /api/stt/preparar) — responde rápido mesmo quando os pesos
                # ainda nem foram baixados.
                self._responderJson({
                    "status": "ok",
                    "servico": "stt",
                    "motor": ConstantesModule.obterNomeMotor(),
                    "versaoContrato": VERSAO_CONTRATO_STT,
                })
                return

            self.send_response(404)
            self.end_headers()


        def do_POST(self):
            try:
                if self.path == '/api/stt/preparar':
                    # Pré-aquecimento: carrega o modelo (na primeiríssima vez, baixa os pesos do
                    # Hugging Face) sem transcrever nada. O app chama ao entrar na revisão de
                    # pronúncia, para a primeira transcrição real sair sem essa espera.
                    servicoStt.PrepararModelo(
                        aoProgredir=lambda msg: registradorLogs.info("[stt] %s", msg)
                    )
                    self._responderJson({"ok": True})
                    return

                if self.path == '/api/stt/iniciar':
                    # Push-to-talk: começa a capturar o microfone. Uma gravação em andamento é
                    # descartada e a captura recomeça (tolerante a um "parar" perdido).
                    servicoStt.IniciarGravacao()
                    self._responderJson({"ok": True})
                    return

                if self.path == '/api/stt/parcial':
                    # Transcrição PARCIAL do áudio acumulado, sem parar a captura (o app faz
                    # polling durante a escuta para os parciais em tempo real). Best-effort:
                    # devolve texto vazio quando um parcial não pode sair agora (nada gravado,
                    # modelo ocupado/não carregado) — o tick seguinte tenta de novo.
                    self._responderJson({"texto": servicoStt.TranscreverParcial()})
                    return

                if self.path == '/api/stt/parar':
                    # Para a captura e transcreve o que foi gravado. A primeira chamada pode
                    # demorar: carga do modelo + download dos pesos (se /preparar não rodou antes).
                    amostras, taxa = servicoStt.PararGravacao()
                    texto = servicoStt.Transcrever(
                        amostras, taxa, aoProgredir=lambda msg: registradorLogs.info("[stt] %s", msg)
                    )
                    self._responderJson({"texto": texto})
                    return

                if self.path == '/api/stt/cancelar':
                    # Descarta a gravação em andamento sem transcrever (soltar fora do botão,
                    # troca de questão, desmontagem da tela).
                    servicoStt.CancelarGravacao()
                    self._responderJson({"ok": True})
                    return

                self.send_response(404)
                self.end_headers()

            except Exception as erro:
                registradorLogs.error(f"Erro no endpoint {self.path}: {erro}")
                self._responderJson({"error": str(erro)}, status=500)

    return RequisicaoSttHandler


# ----- Execução -----
def iniciarServidorStt(servicoStt, porta=None):
    """Sobe o microserviço HTTP deste motor de STT na porta do app (HANZITRACKER_STT_PORT)."""
    # A porta é definida dinamicamente pelo app (stt.go) via HANZITRACKER_STT_PORT;
    # o 8091 é apenas um fallback para execução avulsa do servidor.
    if porta is None:
        porta = int(os.environ.get("HANZITRACKER_STT_PORT", "8091"))
    enderecoServidor = ('', porta)
    # ThreadingHTTPServer (e não HTTPServer): o polling de /api/stt/parcial acontece DURANTE um
    # /parar ou /preparar em andamento — single-thread, os parciais fariam fila atrás deles. A
    # concorrência real é limitada pelos locks do ServicoSttBase (modelo + gravação).
    servidorHttp = ThreadingHTTPServer(enderecoServidor, _criarHandler(servicoStt))
    registradorLogs.info(f"Iniciando Microserviço de STT ({ConstantesModule.obterNomeMotor()}) na porta {porta}...")
    servidorHttp.serve_forever()
