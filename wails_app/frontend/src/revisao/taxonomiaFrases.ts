// Fonte única da verdade dos rótulos de tema e dificuldade no frontend.
// Espelha wails_app/dicionario/frases/taxonomia/taxonomia.go (fonte primária da verdade no Go).
// As duas taxonomias precisam andar juntas.

export interface ItemTaxonomia {
  valor: string;
  rotulo: string;
}

export const TEMAS_FRASE: ItemTaxonomia[] = [
  { valor: '日常生活', rotulo: 'Cotidiano' },
  { valor: '家庭', rotulo: 'Família' },
  { valor: '工作职业', rotulo: 'Trabalho / Profissão' },
  { valor: '学习教育', rotulo: 'Estudo / Educação' },
  { valor: '饮食', rotulo: 'Comida / Bebida' },
  { valor: '购物', rotulo: 'Compras' },
  { valor: '旅行交通', rotulo: 'Viagem / Transporte' },
  { valor: '健康身体', rotulo: 'Saúde / Corpo' },
  { valor: '情感', rotulo: 'Emoções / Sentimentos' },
  { valor: '时间日期', rotulo: 'Tempo / Data' },
  { valor: '天气自然', rotulo: 'Clima / Natureza' },
  { valor: '社交问候', rotulo: 'Social / Saudações' },
  { valor: '兴趣娱乐', rotulo: 'Interesses / Lazer' },
  { valor: '科技', rotulo: 'Tecnologia' },
  { valor: '数字量词', rotulo: 'Números / Quantidades' },
  { valor: '地点方位', rotulo: 'Lugares / Direções' },
  { valor: '语言文化', rotulo: 'Língua / Cultura' },
  { valor: '其他', rotulo: 'Outros' },
];

export const DIFICULDADES_FRASE: ItemTaxonomia[] = [
  { valor: '入门', rotulo: 'Iniciante' },
  { valor: '初级', rotulo: 'Fácil' },
  { valor: '中级', rotulo: 'Médio' },
  { valor: '高级', rotulo: 'Avançado' },
];

export function rotuloTema(valor: string): string {
  const item = TEMAS_FRASE.find((t) => t.valor === valor);
  return item ? item.rotulo : '';
}

export function rotuloDificuldade(valor: string): string {
  const item = DIFICULDADES_FRASE.find((d) => d.valor === valor);
  return item ? item.rotulo : '';
}
