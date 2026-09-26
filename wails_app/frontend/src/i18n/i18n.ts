import { en } from './en';
import { es } from './es';

// Registro de mapas de tradução por idioma. O idioma-fonte (pt-BR) NÃO tem mapa: t() devolve a própria
// chave (a string em português). Novos idiomas entram aqui com seu próprio mapa PT→idioma.
const mapas: Record<string, Record<string, string>> = { en, es };

let idiomaAtivo = 'pt-BR';

// definirIdioma fixa o idioma da UI. Chamada UMA vez no arranque (main.tsx), antes de renderizar.
export function definirIdioma(idioma: string): void {
  idiomaAtivo = idioma || 'pt-BR';
}

export function idiomaAtual(): string {
  return idiomaAtivo;
}

// t traduz a string-fonte (em português) para o idioma ativo. Sem tradução (ou em pt-BR), devolve a
// própria string. Suporta interpolação de {nome} via params.
export function t(chave: string, params?: Record<string, string | number>): string {
  let texto = mapas[idiomaAtivo]?.[chave] ?? chave;
  if (params) {
    for (const k in params) {
      texto = texto.replace(new RegExp('\\{' + k + '\\}', 'g'), String(params[k]));
    }
  }
  return texto;
}
