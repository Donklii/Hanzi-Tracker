# ----- Importações -----
import os
from typing import Dict, List

from principal import ConstantesModule


# ----- Gerenciador de Modelos de OCR -----
class GerenciadorModelos:
    """Lista os pesos de OCR de UM motor em %APPDATA%\\HanziTracker\\motores_ocr\\<Motor>\\modelos.

    Genérico para qualquer motor: recebe o MANIFESTO de pesos dele (módulo com
    MODELOS_BAIXAVEIS/MODELOS_EMBUTIDOS/obterModelo — ex.: motores.rapidocr.ModelosManifesto) e
    resolve a subpasta pelo nome do motor (HANZITRACKER_MOTOR, via obterNomeMotor). Sem valor
    padrão: cada motor está numa pasta própria em motores_ocr/, então o chamador SEMPRE informa o seu.
    """

    def __init__(self, manifesto) -> None:
        self._manifesto = manifesto

    def obterPastaModelos(self) -> str:
        """Retorna (criando se preciso) a pasta de pesos DESTE motor na pasta de dados do app.

        Os pesos ficam DENTRO da pasta do próprio motor (motores_ocr/<Motor>/modelos, ex.:
        motores_ocr/RapidOCR/modelos), junto do executável dele, sem repetir o nome do motor numa
        árvore modelos/ paralela. O nome vem de HANZITRACKER_MOTOR (injetada pelo Go ao subir o
        sidecar) e deve casar com `PastaModelosMotor()` do Go, que baixa os pesos.
        """
        pasta = os.path.join(
            ConstantesModule.obterPastaDados(), "motores_ocr", ConstantesModule.obterNomeMotor(), "modelos"
        )
        os.makedirs(pasta, exist_ok=True)
        return pasta

    def caminhoArquivo(self, nomeArquivo: str) -> str:
        return os.path.join(self.obterPastaModelos(), nomeArquivo)

    def caminhosModelo(self, nomeModelo: str) -> Dict[str, str]:
        """Mapa tipo -> caminho local (det/rec/cls) dos arquivos de um modelo baixável."""
        modelo = self._manifesto.obterModelo(nomeModelo)

        # Guard clause: modelo não é baixável
        if modelo is None:
            return {}

        return {tipo: self.caminhoArquivo(arq["nome"]) for tipo, arq in modelo["arquivos"].items()}

    def modeloInstalado(self, nomeModelo: str) -> bool:
        """Indica se todos os arquivos do modelo já estão presentes no disco."""
        caminhos = self.caminhosModelo(nomeModelo)

        # Guard clause: modelo desconhecido nunca está instalado
        if not caminhos:
            return False

        return all(os.path.exists(c) for c in caminhos.values())

    def tamanhoInstalado(self, nomeModelo: str) -> int:
        """Soma o tamanho (em bytes) dos arquivos já presentes no disco."""
        total = 0
        for caminho in self.caminhosModelo(nomeModelo).values():
            if os.path.exists(caminho):
                total += os.path.getsize(caminho)
        return total

    def listar(self) -> List[dict]:
        """Lista os modelos (embutidos + baixáveis) com o estado atual para a UI."""
        lista: List[dict] = []

        for nome, info in self._manifesto.MODELOS_EMBUTIDOS.items():
            lista.append({
                "nome": nome,
                "rotulo": info["rotulo"],
                "descricao": info["descricao"],
                "idiomas": info["idiomas"],
                "baixavel": False,
                "embutido": True,
                "instalado": True,
                "tamanhoBytes": 0,
            })

        for nome, info in self._manifesto.MODELOS_BAIXAVEIS.items():
            instalado = self.modeloInstalado(nome)
            lista.append({
                "nome": nome,
                "rotulo": info["rotulo"],
                "descricao": info["descricao"],
                "idiomas": info["idiomas"],
                "baixavel": True,
                "embutido": False,
                "instalado": instalado,
                "tamanhoBytes": self.tamanhoInstalado(nome) if instalado else 0,
                # Quem baixa/remove os arquivos é o Go (processo não-sandbox, escreve no AppData real).
                # O Python só informa nome, url e sha256 (o Go confere a integridade após baixar) e LÊ
                # os modelos para carregar no RapidOCR.
                "arquivos": [
                    {"nome": arq["nome"], "url": arq["url"], "sha256": arq.get("sha256", "")}
                    for arq in info["arquivos"].values()
                ],
            })

        return lista
