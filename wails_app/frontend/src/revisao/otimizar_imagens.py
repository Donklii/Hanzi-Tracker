import os
import sys
from pathlib import Path
from PIL import Image

# Garantir codificação UTF-8 no terminal Windows
if hasattr(sys.stdout, "reconfigure"):
    sys.stdout.reconfigure(encoding="utf-8")

# ----- Seção: Constantes e Configurações -----

DIMENSAO_MAXIMA_PADRAO = 512  # Resolução máxima (largura ou altura em pixels)
PASTA_IMAGENS_PADRAO = Path(__file__).parent / "imagens_hanzi"
EXTENSOES_SUPORTADAS = {".png", ".jpg", ".jpeg", ".webp"}


# ----- Seção: Lógica Principal de Otimização -----

def otimizarImagensEmDiretorio(
    diretorioAlvo: Path = PASTA_IMAGENS_PADRAO,
    dimensaoMaxima: int = DIMENSAO_MAXIMA_PADRAO,
    soescrever: bool = True
) -> None:
    """
    Percorre a pasta de imagens e redimensiona mantendo a proporção de aspecto,
    pulando arquivos que já estejam dentro da resolução limite desejada.
    """
    if not diretorioAlvo.exists() or not diretorioAlvo.is_dir():
        print(f"[ERRO] Diretório não encontrado: {diretorioAlvo}")
        return

    arquivosImagens = [
        f for f in diretorioAlvo.iterdir()
        if f.is_file() and f.suffix.lower() in EXTENSOES_SUPORTADAS
    ]

    if not arquivosImagens:
        print(f"[AVISO] Nenhuma imagem suportada encontrada em: {diretorioAlvo}")
        return

    print(f"==================================================")
    print(f" Analisando {len(arquivosImagens)} imagens em: {diretorioAlvo.name}")
    print(f" Dimensão máxima definida: {dimensaoMaxima}px")
    print(f"==================================================\n")

    tamanhoTotalOriginal = 0
    tamanhoTotalFinal = 0
    imagensOtimizadas = 0
    imagensPuladas = 0

    for caminhoImagem in arquivosImagens:
        tamanhoOriginal = caminhoImagem.stat().st_size
        tamanhoTotalOriginal += tamanhoOriginal

        foiProcessada, largura, altura = redimensionarEhOtimizarImagem(caminhoImagem, dimensaoMaxima, soescrever)

        if not foiProcessada:
            tamanhoTotalFinal += tamanhoOriginal
            imagensPuladas += 1
            print(
                f"➖ {caminhoImagem.name:15s} | "
                f"Já no padrão ({largura}x{altura}px <= {dimensaoMaxima}px) - Pulada"
            )
            continue

        tamanhoFinal = caminhoImagem.stat().st_size
        tamanhoTotalFinal += tamanhoFinal
        imagensOtimizadas += 1

        economiaBytes = tamanhoOriginal - tamanhoFinal
        porcentagemEconomia = (economiaBytes / tamanhoOriginal * 100) if tamanhoOriginal > 0 else 0

        print(
            f"✔ {caminhoImagem.name:15s} | "
            f"Original: {formatarTamanho(tamanhoOriginal):>8s} -> "
            f"Novo: {formatarTamanho(tamanhoFinal):>8s} "
            f"({porcentagemEconomia:.1f}% menor)"
        )

    economiaTotal = tamanhoTotalOriginal - tamanhoTotalFinal
    porcentagemTotal = (economiaTotal / tamanhoTotalOriginal * 100) if tamanhoTotalOriginal > 0 else 0

    print(f"\n--------------------------------------------------")
    print(f" Resumo da Execução:")
    print(f" Imagens no total:      {len(arquivosImagens)}")
    print(f" Imagens otimizadas:    {imagensOtimizadas}")
    print(f" Imagens puladas:       {imagensPuladas} (já no padrão)")
    print(f" Tamanho inicial total: {formatarTamanho(tamanhoTotalOriginal)}")
    print(f" Tamanho final total:   {formatarTamanho(tamanhoTotalFinal)}")
    if imagensOtimizadas > 0:
        print(f" Espaço economizado:    {formatarTamanho(economiaTotal)} ({porcentagemTotal:.1f}% de redução)")
    print(f"--------------------------------------------------")


def redimensionarEhOtimizarImagem(
    caminhoImagem: Path,
    dimensaoMaxima: int,
    soescrever: bool
) -> tuple[bool, int, int]:
    """
    Redimensiona uma única imagem proporcionalmente e salva mantendo transparência.
    Retorna uma tupla (foiProcessada, largura, altura).
    """
    try:
        with Image.open(caminhoImagem) as img:
            largura, altura = img.size

            # Guard Clause: Se a imagem já estiver no padrão (menor ou igual ao limite), pula o processamento
            if largura <= dimensaoMaxima and altura <= dimensaoMaxima:
                return (False, largura, altura)

            img.thumbnail((dimensaoMaxima, dimensaoMaxima), Image.Resampling.LANCZOS)

            caminhoDestino = caminhoImagem if soescrever else caminhoImagem.with_stem(f"{caminhoImagem.stem}_otimizado")
            
            formato = img.format if img.format else "PNG"
            if formato.upper() == "PNG":
                img.save(caminhoDestino, format="PNG", optimize=True)
            elif formato.upper() in ("JPEG", "JPG"):
                img.convert("RGB").save(caminhoDestino, format="JPEG", quality=85, optimize=True)
            elif formato.upper() == "WEBP":
                img.save(caminhoDestino, format="WEBP", quality=85)
            else:
                img.save(caminhoDestino, optimize=True)

            return (True, largura, altura)

    except Exception as erro:
        print(f"[ERRO] Falha ao processar {caminhoImagem.name}: {erro}")
        return (False, 0, 0)


# ----- Seção: Utilitários -----

def formatarTamanho(tamanhoEmBytes: int) -> str:
    """
    Formata bytes para exibição legível em KB ou MB.
    """
    if tamanhoEmBytes >= 1024 * 1024:
        return f"{tamanhoEmBytes / (1024 * 1024):.2f} MB"
    return f"{tamanhoEmBytes / 1024:.1f} KB"


# ----- Seção: Ponto de Entrada -----

if __name__ == "__main__":
    # Permite passar dimensão máxima opcional via argumento CLI (ex: python otimizar_imagens.py 256)
    dimensao = int(sys.argv[1]) if len(sys.argv) > 1 and sys.argv[1].isdigit() else DIMENSAO_MAXIMA_PADRAO
    otimizarImagensEmDiretorio(dimensaoMaxima=dimensao)
