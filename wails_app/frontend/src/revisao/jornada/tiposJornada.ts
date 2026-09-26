// ----- Seção: Jornada de Revisão — Tipos e Vocabulário -----
// Espelha `revisao/jornada/arvore.json` (carregado e validado pelo pacote Go `revisao/jornada`, e
// entregue por ObterArvoreJornada). As revisões de cada nível chegam JÁ substituídas pela
// disponibilidade de motor de voz — o frontend não repete essa regra.

export interface MapaJornadaDimensoes {
  largura: number;
  altura: number;
}

export interface NivelJornada {
  id: string; // "<ramoId>_<indice>", derivado no backend
  titulo: string;
  pos: number[]; // [x, y] do nó no mapa
  palavras: string[];
  revisoes: string[]; // chaves de REVISOES_JORNADA, em ordem; a última é sempre 'mista'
}

export interface RamoJornada {
  id: string;
  nome: string;
  icone: string; // chave de ICONES_JORNADA
  dificuldade: string; // iniciante | intermediario | avancado
  pai: string; // vazio = raiz da árvore
  tema: string; // rótulo chinês da taxonomia de frases; vazio = sem tema
  rotuloMapa: number[]; // [x, y] do rótulo do ramo
  niveis: NivelJornada[];
}

export interface ArvoreJornada {
  mapa: MapaJornadaDimensoes;
  ramos: RamoJornada[];
}

export interface DefinicaoRevisaoJornada {
  titulo: string;
  descricao: string;
  chip: string;
  icone: string; // chave de ICONES_JORNADA
}

// REVISOES_JORNADA descreve cada tipo de revisão para o usuário (modal do nível e chip da sessão).
export const REVISOES_JORNADA: Record<string, DefinicaoRevisaoJornada> = {
  palavras: {
    titulo: 'Palavras novas',
    descricao: 'Reconheça o significado das palavras do nível',
    chip: 'Significado',
    icone: 'significado',
  },
  fonetica: {
    titulo: 'Fonética',
    descricao: 'Ligue o som e a pronúncia aos caracteres',
    chip: 'Fonética',
    icone: 'fonetica',
  },
  desenho: {
    titulo: 'Desenho',
    descricao: 'Trace os caracteres de memória',
    chip: 'Desenho',
    icone: 'desenho',
  },
  frases: {
    titulo: 'Frases do tema',
    descricao: 'Complete e ordene frases reais do tema',
    chip: 'Contexto',
    icone: 'contexto',
  },
  pronuncia: {
    titulo: 'Pronúncia',
    descricao: 'Fale as frases em voz alta',
    chip: 'Pronúncia',
    icone: 'pronuncia',
  },
  mista: {
    titulo: 'Revisão final',
    descricao: 'Mistura de todas as atividades do nível',
    chip: 'Geral',
    icone: 'geral',
  },
};

// DIFICULDADES_JORNADA traduz o rótulo de dificuldade do ramo para exibição.
export const DIFICULDADES_JORNADA: Record<string, string> = {
  iniciante: 'Iniciante',
  intermediario: 'Intermediário',
  avancado: 'Avançado',
};
