import React from 'react';
import { main } from '../../wailsjs/go/models';
import { t } from '../i18n/i18n';
import { rotuloTema, rotuloDificuldade } from './taxonomiaFrases';

// ----- Seção: Revisão — Metadados da Frase (Tema e Dificuldade) -----
// Exibe os chips com o Tema e Nível de Dificuldade para questões de frase na revisão.
// Quando a atividade censura a frase (ex.: fonética onde a frase fica oculta), o tema é exibido como censurado.

// ----- Seção: Dicionários de Tradução e Normalização -----

const MAPA_DIFICULDADES: Record<string, string> = {
  // Termos em Chinês enviados pela API/LLM (chaves legadas/alternativas; as da taxonomia vivem em taxonomiaFrases.ts)
  '基础': 'Iniciante',
  '中等': 'Intermediário',
  '简单': 'Fácil',
  '容易': 'Fácil',
  '困难': 'Difícil',
  '较难': 'Difícil',

  // Termos em Português / variações sem acento
  'facil': 'Fácil',
  'fácil': 'Fácil',
  'intermediario': 'Intermediário',
  'intermediário': 'Intermediário',
  'dificil': 'Difícil',
  'difícil': 'Difícil',
  'iniciante': 'Iniciante',
  'avancado': 'Avançado',
  'avançado': 'Avançado',
};

const MAPA_TEMAS: Record<string, string> = {
  // Chaves legadas/alternativas (as 18 oficiais da taxonomia vivem em taxonomiaFrases.ts)
  '日常': 'Cotidiano',
  '旅游': 'Viagem',
  '旅行': 'Viagem',
  '美食': 'Gastronomia',
  '工作': 'Trabalho',
  '职场': 'Trabalho',
  '商务': 'Negócios',
  '商业': 'Negócios',
  '学习': 'Estudos',
  '教育': 'Estudos',
  '健康': 'Saúde',
  '医疗': 'Saúde',
  '体育': 'Esportes',
  '运动': 'Esportes',
  '自然': 'Natureza',
  '环境': 'Natureza',
  '历史': 'História',
  '传统': 'Tradição',
  '艺术': 'Arte',
  '文化': 'Cultura',
  '科学': 'Ciência',
  '音乐': 'Música',
  '娱乐': 'Entretenimento',
  '休闲': 'Lazer',
  '亲情': 'Família',
  '交通': 'Transporte',
  '出行': 'Transporte',
  '社会': 'Sociedade',
  '哲学': 'Filosofia',
  '文学': 'Literatura',
  '地理': 'Geografia',
  '经济': 'Economia',
  '政治': 'Política',
  '法律': 'Direito',
};

interface MetaFraseProps {
  questao: main.QuestaoRevisao;
  respondida?: boolean;
  revelarFraseManual?: boolean;
}

export function MetaFrase({ questao, respondida, revelarFraseManual }: MetaFraseProps) {
  if (!verificarEhQuestaoComFrase(questao)) return null;

  const ehCensurada = verificarEhFraseCensurada(questao, respondida, revelarFraseManual);
  const textoTema = obterTextoTema(questao.fraseTema, ehCensurada);
  const textoDificuldade = formatarDificuldade(questao.fraseDificuldade);

  return (
    <>
      <span
        className={`revisao-chip revisao-chip-tema ${ehCensurada ? 'revisao-chip-censurado' : ''}`}
        title={ehCensurada ? t('Tema oculto porque a frase está censurada') : t('Tema da frase')}
      >
        {textoTema}
      </span>
      <span
        className="revisao-chip revisao-chip-dificuldade"
        title={t('Nível de dificuldade da frase')}
      >
        📊 {textoDificuldade}
      </span>
    </>
  );
}


// ----- Seção: Funções Auxiliares e Formatação -----

export function verificarEhQuestaoComFrase(questao?: main.QuestaoRevisao): boolean {
  if (!questao) return false;
  if (questao.fraseTema || questao.fraseDificuldade || questao.fraseOriginal) return true;

  const variantesFrase = [
    'fonetica_frase',
    'fonetica_traducao',
    'contexto',
    'traducao_contexto',
    'desenho_contexto',
    'ordenacao',
    'ordenacao_traducao',
    'pronuncia_frase',
    'pronuncia_sequencia'
  ];

  return variantesFrase.includes(questao.variante);
}


export function verificarEhFraseCensurada(
  questao: main.QuestaoRevisao,
  respondida?: boolean,
  revelarFraseManual?: boolean
): boolean {
  if (respondida) return false;
  if (revelarFraseManual) return false;

  if (questao.variante === 'fonetica_traducao') return true;
  if (questao.variante === 'fonetica_frase') return true;
  if (questao.variante === 'ordenacao') return true;

  return false;
}


function obterTextoTema(tema: string | undefined, ehCensurada: boolean): string {
  if (ehCensurada) {
    return `🏷️ 🔒 ${t('Tema censurado')}`;
  }

  if (!tema || !tema.trim()) {
    return `🏷️ ${t('Geral')}`;
  }

  const valorLimpo = tema.trim();
  const temaOficial = rotuloTema(valorLimpo);
  if (temaOficial) {
    return `🏷️ ${t(temaOficial)}`;
  }

  const temaMapeado = MAPA_TEMAS[valorLimpo];
  if (temaMapeado) {
    return `🏷️ ${t(temaMapeado)}`;
  }

  const temaFormatado = valorLimpo.charAt(0).toUpperCase() + valorLimpo.slice(1);
  return `🏷️ ${t(temaFormatado)}`;
}


export function formatarDificuldade(dificuldade?: string): string {
  if (!dificuldade || !dificuldade.trim()) return t('Normal');

  const valorLimpo = dificuldade.trim();
  const dificuldadeOficial = rotuloDificuldade(valorLimpo);
  if (dificuldadeOficial) {
    return t(dificuldadeOficial);
  }

  const valorMinusculo = valorLimpo.toLowerCase();
  const chaveMapeada = MAPA_DIFICULDADES[valorLimpo] || MAPA_DIFICULDADES[valorMinusculo];
  if (chaveMapeada) {
    return t(chaveMapeada);
  }

  const chaveFormatada = valorLimpo.charAt(0).toUpperCase() + valorLimpo.slice(1);
  return t(chaveFormatada);
}
