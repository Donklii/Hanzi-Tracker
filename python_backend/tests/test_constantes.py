# ----- Testes das constantes/ambiente do backend -----
# Cobre obterNomeMotor (principal/ConstantesModule.py): o nome de catálogo do motor vem de
# HANZITRACKER_MOTOR (injetada pelo Go) e define a subpasta de pesos modelos/<Motor>.

from principal import ConstantesModule


# ----- Nome de catálogo do motor (HANZITRACKER_MOTOR) -----

def testarNomeMotorVemDaEnv(monkeypatch):
    monkeypatch.setenv("HANZITRACKER_MOTOR", "EasyOCR")
    assert ConstantesModule.obterNomeMotor() == "EasyOCR"


def testarNomeMotorSemEnvCaiNoPadrao(monkeypatch):
    monkeypatch.delenv("HANZITRACKER_MOTOR", raising=False)
    assert ConstantesModule.obterNomeMotor() == "RapidOCR"


def testarNomeMotorVazioCaiNoPadrao(monkeypatch):
    monkeypatch.setenv("HANZITRACKER_MOTOR", "   ")
    assert ConstantesModule.obterNomeMotor() == "RapidOCR"


def testarNomeMotorComSeparadorEhRecusado(monkeypatch):
    # Um valor com separador escaparia de modelos/ — cai no padrão em vez de virar caminho.
    monkeypatch.setenv("HANZITRACKER_MOTOR", "..\\fora")
    assert ConstantesModule.obterNomeMotor() == "RapidOCR"
    monkeypatch.setenv("HANZITRACKER_MOTOR", "../fora")
    assert ConstantesModule.obterNomeMotor() == "RapidOCR"
