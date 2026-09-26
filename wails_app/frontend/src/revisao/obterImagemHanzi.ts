// ----- Seção: Utilitário de Carregamento de Imagens para Hanzis/Palavras -----

// Carrega dinamicamente todas as imagens de imagens_hanzi via import.meta.glob do Vite
const IMAGENS_HANZI = import.meta.glob<{ default: string }>(
  './imagens_hanzi/*.{png,jpg,jpeg,webp,svg}',
  { eager: true }
);

export function obterUrlImagemHanzi(hanziOuPalavra: string): string | null {
  if (!hanziOuPalavra) return null;

  for (const caminho in IMAGENS_HANZI) {
    const nomeArquivo = caminho.split('/').pop() || '';
    const palavraSemExtensao = nomeArquivo.substring(0, nomeArquivo.lastIndexOf('.'));

    if (palavraSemExtensao === hanziOuPalavra) {
      const modulo = IMAGENS_HANZI[caminho];
      return typeof modulo === 'string' ? modulo : (modulo?.default || null);
    }
  }

  // Fallback para a pasta pública
  return `/imagens_hanzi/${hanziOuPalavra}.png`;
}
