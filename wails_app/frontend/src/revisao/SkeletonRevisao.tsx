// ----- Seção: Componente Skeleton Loading da Revisão -----
import { t } from '../i18n/i18n';
import { revisao } from '../../wailsjs/go/models';

export type TipoSkeleton = 'opcoes' | 'desenho' | 'montagem' | 'pronuncia' | 'quebracabeca' | 'quebracabeca_trio' | 'fila_pinyin';

interface SkeletonRevisaoProps {
  modo?: string | null;
  questao?: revisao.QuestaoRevisao | null;
  modosDesativados?: string[];
  mensagem?: string;
}

export function determinarTipoSkeleton(
  modo?: string | null,
  questao?: revisao.QuestaoRevisao | null,
  modosDesativados?: string[]
): TipoSkeleton {
  // 1. Se já temos a primeira questão (qs[0]), a variante é 100% exata!
  if (questao?.variante) {
    const v = questao.variante;
    if (v.startsWith('desenho')) return 'desenho';
    if (v === 'quebracabeca_trio') return 'quebracabeca_trio';
    if (v.startsWith('quebracabeca')) return 'quebracabeca';
    if (v.startsWith('pronuncia')) return 'pronuncia';
    if (v === 'ordenacao' || v === 'ordenacao_traducao' || v === 'fonetica_frase' || v === 'contexto' || v === 'traducao_contexto') return 'montagem';
    if (v === 'fonetica_fila_pinyin' || v === 'fonetica_palavra_pinyin') return 'fila_pinyin';
    return 'opcoes';
  }

  // 2. Se o modo escolhido for específico
  if (modo) {
    if (modo === 'desenho') return 'desenho';
    if (modo === 'pronuncia') return 'pronuncia';
    if (modo === 'contexto' || modo === 'ordenacao') return 'montagem';
    if (modo === 'significado' || modo === 'fonetica') return 'opcoes';
  }

  // 3. Se for modo geral, verifica quais modos estão ativos nas configurações do usuário
  if (modo === 'geral' && modosDesativados) {
    const des = new Set(modosDesativados);
    const ordemModos = ['significado', 'desenho', 'contexto', 'fonetica', 'pronuncia'];
    for (const m of ordemModos) {
      if (!des.has(m)) {
        if (m === 'desenho') return 'desenho';
        if (m === 'pronuncia') return 'pronuncia';
        if (m === 'contexto') return 'montagem';
        if (m === 'significado' || m === 'fonetica') return 'opcoes';
      }
    }
  }

  return 'opcoes';
}

export function SkeletonRevisao({ modo, questao, modosDesativados, mensagem }: SkeletonRevisaoProps) {
  // Se ainda não temos a primeira questão para determinar a variante exata, exibe apenas a animação de carregando
  if (!questao || !questao.variante) {
    return (
      <div className="revisao-container" style={{ display: 'flex', flexDirection: 'column', alignItems: 'center', justifyContent: 'center', minHeight: '360px', gap: '16px' }}>
        <div className="revisao-spinner" style={{ width: '38px', height: '38px', borderWidth: '3px', color: 'var(--cor-destaque)' }} />
        <div style={{ color: 'var(--cor-texto-suave)', fontSize: '15px', fontWeight: 500 }}>
          {mensagem || t('Preparando questões...')}
        </div>
      </div>
    );
  }

  const tipo = determinarTipoSkeleton(modo, questao, modosDesativados);

  return (
    <div className="revisao-container revisao-skeleton-container" aria-busy="true" aria-live="polite">
      {/* Topo: Barra de Progresso + Contadores Skeleton */}
      <div className="revisao-topo">
        <div className="revisao-barra-progresso revisao-skeleton-base">
          <div className="revisao-skeleton-shimmer" />
        </div>
        <div className="revisao-topo-info">
          <div className="revisao-skeleton-chip revisao-skeleton-base">
            <div className="revisao-skeleton-shimmer" />
          </div>
          <div className="revisao-skeleton-chip revisao-skeleton-base" style={{ width: '80px' }}>
            <div className="revisao-skeleton-shimmer" />
          </div>
        </div>
      </div>

      {/* Renderização 100% fiel ao tipo de atividade */}
      <div className="revisao-questao revisao-skeleton-questao">
        {tipo === 'desenho' && (
          <>
            <div className="revisao-skeleton-rotulo revisao-skeleton-base" style={{ width: '180px' }}>
              <div className="revisao-skeleton-shimmer" />
            </div>
            <div className="revisao-skeleton-canvas-box revisao-skeleton-base">
              <div className="revisao-skeleton-shimmer" />
              <div className="revisao-skeleton-canvas-grid" />
              <div className="revisao-skeleton-canvas-diagonais" />
            </div>
            <div className="revisao-skeleton-botoes-acao">
              <div className="revisao-skeleton-botao-acao revisao-skeleton-base" style={{ width: '85px' }}><div className="revisao-skeleton-shimmer" /></div>
              <div className="revisao-skeleton-botao-acao revisao-skeleton-base" style={{ width: '85px' }}><div className="revisao-skeleton-shimmer" /></div>
              <div className="revisao-skeleton-botao-acao revisao-skeleton-base" style={{ width: '110px' }}><div className="revisao-skeleton-shimmer" /></div>
            </div>
          </>
        )}

        {tipo === 'montagem' && (
          <>
            <div className="revisao-skeleton-cartao-principal revisao-skeleton-base" style={{ height: '90px', maxWidth: '460px' }}>
              <div className="revisao-skeleton-shimmer" />
            </div>
            <div className="revisao-skeleton-slots-frase" style={{ width: '100%', maxWidth: '520px' }}>
              <div className="revisao-skeleton-slot-chip revisao-skeleton-base"><div className="revisao-skeleton-shimmer" /></div>
              <div className="revisao-skeleton-slot-chip revisao-skeleton-base" style={{ width: '90px' }}><div className="revisao-skeleton-shimmer" /></div>
              <div className="revisao-skeleton-slot-chip revisao-skeleton-base"><div className="revisao-skeleton-shimmer" /></div>
            </div>
            <div className="revisao-skeleton-banco-chips" style={{ width: '100%', maxWidth: '520px' }}>
              <div className="revisao-skeleton-chip-palavra revisao-skeleton-base" style={{ width: '85px' }}><div className="revisao-skeleton-shimmer" /></div>
              <div className="revisao-skeleton-chip-palavra revisao-skeleton-base" style={{ width: '65px' }}><div className="revisao-skeleton-shimmer" /></div>
              <div className="revisao-skeleton-chip-palavra revisao-skeleton-base" style={{ width: '95px' }}><div className="revisao-skeleton-shimmer" /></div>
              <div className="revisao-skeleton-chip-palavra revisao-skeleton-base" style={{ width: '75px' }}><div className="revisao-skeleton-shimmer" /></div>
              <div className="revisao-skeleton-chip-palavra revisao-skeleton-base" style={{ width: '90px' }}><div className="revisao-skeleton-shimmer" /></div>
            </div>
          </>
        )}

        {tipo === 'pronuncia' && (
          <>
            <div className="revisao-skeleton-cartao-principal revisao-skeleton-base" style={{ height: '110px', maxWidth: '440px' }}>
              <div className="revisao-skeleton-shimmer" />
            </div>
            <div className="revisao-skeleton-mic-container">
              <div className="revisao-skeleton-mic-wrapper">
                <div className="revisao-skeleton-mic-ring" />
                <div className="revisao-skeleton-mic-botao revisao-skeleton-base">
                  <div className="revisao-skeleton-shimmer" />
                </div>
              </div>
              <div className="revisao-skeleton-wave-bars">
                <div className="revisao-skeleton-wave-bar revisao-skeleton-base" style={{ height: '12px' }}><div className="revisao-skeleton-shimmer" /></div>
                <div className="revisao-skeleton-wave-bar revisao-skeleton-base" style={{ height: '22px' }}><div className="revisao-skeleton-shimmer" /></div>
                <div className="revisao-skeleton-wave-bar revisao-skeleton-base" style={{ height: '16px' }}><div className="revisao-skeleton-shimmer" /></div>
                <div className="revisao-skeleton-wave-bar revisao-skeleton-base" style={{ height: '24px' }}><div className="revisao-skeleton-shimmer" /></div>
                <div className="revisao-skeleton-wave-bar revisao-skeleton-base" style={{ height: '10px' }}><div className="revisao-skeleton-shimmer" /></div>
              </div>
              <div className="revisao-skeleton-tentativas-dots">
                <div className="revisao-skeleton-dot revisao-skeleton-base"><div className="revisao-skeleton-shimmer" /></div>
                <div className="revisao-skeleton-dot revisao-skeleton-base"><div className="revisao-skeleton-shimmer" /></div>
                <div className="revisao-skeleton-dot revisao-skeleton-base"><div className="revisao-skeleton-shimmer" /></div>
              </div>
            </div>
          </>
        )}

        {(tipo === 'quebracabeca' || tipo === 'quebracabeca_trio') && (
          <>
            <div className="revisao-skeleton-rotulo revisao-skeleton-base" style={{ width: '220px' }}>
              <div className="revisao-skeleton-shimmer" />
            </div>
            <div className="revisao-skeleton-quebracabeca-arena" style={{ maxWidth: tipo === 'quebracabeca_trio' ? '680px' : '520px' }}>
              <div className="revisao-skeleton-quebracabeca-coluna">
                <div className="revisao-skeleton-quebracabeca-card revisao-skeleton-base"><div className="revisao-skeleton-shimmer" /></div>
                <div className="revisao-skeleton-quebracabeca-card revisao-skeleton-base"><div className="revisao-skeleton-shimmer" /></div>
                <div className="revisao-skeleton-quebracabeca-card revisao-skeleton-base"><div className="revisao-skeleton-shimmer" /></div>
              </div>
              <div className="revisao-skeleton-quebracabeca-coluna">
                <div className="revisao-skeleton-quebracabeca-card revisao-skeleton-base"><div className="revisao-skeleton-shimmer" /></div>
                <div className="revisao-skeleton-quebracabeca-card revisao-skeleton-base"><div className="revisao-skeleton-shimmer" /></div>
                <div className="revisao-skeleton-quebracabeca-card revisao-skeleton-base"><div className="revisao-skeleton-shimmer" /></div>
              </div>
              {tipo === 'quebracabeca_trio' && (
                <div className="revisao-skeleton-quebracabeca-coluna">
                  <div className="revisao-skeleton-quebracabeca-card revisao-skeleton-base"><div className="revisao-skeleton-shimmer" /></div>
                  <div className="revisao-skeleton-quebracabeca-card revisao-skeleton-base"><div className="revisao-skeleton-shimmer" /></div>
                  <div className="revisao-skeleton-quebracabeca-card revisao-skeleton-base"><div className="revisao-skeleton-shimmer" /></div>
                </div>
              )}
            </div>
          </>
        )}

        {tipo === 'fila_pinyin' && (
          <>
            <div className="revisao-skeleton-rotulo revisao-skeleton-base" style={{ width: '200px' }}>
              <div className="revisao-skeleton-shimmer" />
            </div>
            <div className="revisao-skeleton-cartao-principal revisao-skeleton-base" style={{ height: '100px', maxWidth: '440px' }}>
              <div className="revisao-skeleton-shimmer" />
            </div>
            <div className="revisao-skeleton-pinyin-grid">
              <div className="revisao-skeleton-pinyin-slot revisao-skeleton-base"><div className="revisao-skeleton-shimmer" /></div>
              <div className="revisao-skeleton-pinyin-slot revisao-skeleton-base"><div className="revisao-skeleton-shimmer" /></div>
              <div className="revisao-skeleton-pinyin-slot revisao-skeleton-base"><div className="revisao-skeleton-shimmer" /></div>
            </div>
            <div className="revisao-skeleton-teclado-container">
              <div className="revisao-skeleton-teclado-tecla revisao-skeleton-base"><div className="revisao-skeleton-shimmer" /></div>
              <div className="revisao-skeleton-teclado-tecla revisao-skeleton-base"><div className="revisao-skeleton-shimmer" /></div>
              <div className="revisao-skeleton-teclado-tecla revisao-skeleton-base"><div className="revisao-skeleton-shimmer" /></div>
              <div className="revisao-skeleton-teclado-tecla revisao-skeleton-base"><div className="revisao-skeleton-shimmer" /></div>
              <div className="revisao-skeleton-teclado-tecla revisao-skeleton-base"><div className="revisao-skeleton-shimmer" /></div>
            </div>
          </>
        )}

        {tipo === 'opcoes' && (
          <>
            <div className="revisao-skeleton-rotulo revisao-skeleton-base">
              <div className="revisao-skeleton-shimmer" />
            </div>
            <div className="revisao-skeleton-cartao-principal revisao-skeleton-base">
              <div className="revisao-skeleton-shimmer" />
              <div className="revisao-skeleton-bloco-hanzi" />
            </div>
            <div className="revisao-skeleton-linha revisao-skeleton-base" style={{ width: '50%', margin: '16px auto 0' }}>
              <div className="revisao-skeleton-shimmer" />
            </div>
            <div className="revisao-skeleton-opcoes">
              <div className="revisao-skeleton-opcao revisao-skeleton-base">
                <div className="revisao-skeleton-opcao-badge revisao-skeleton-base"><div className="revisao-skeleton-shimmer" /></div>
                <div className="revisao-skeleton-opcao-texto revisao-skeleton-base"><div className="revisao-skeleton-shimmer" /></div>
              </div>
              <div className="revisao-skeleton-opcao revisao-skeleton-base">
                <div className="revisao-skeleton-opcao-badge revisao-skeleton-base"><div className="revisao-skeleton-shimmer" /></div>
                <div className="revisao-skeleton-opcao-texto revisao-skeleton-base"><div className="revisao-skeleton-shimmer" /></div>
              </div>
              <div className="revisao-skeleton-opcao revisao-skeleton-base">
                <div className="revisao-skeleton-opcao-badge revisao-skeleton-base"><div className="revisao-skeleton-shimmer" /></div>
                <div className="revisao-skeleton-opcao-texto revisao-skeleton-base"><div className="revisao-skeleton-shimmer" /></div>
              </div>
              <div className="revisao-skeleton-opcao revisao-skeleton-base">
                <div className="revisao-skeleton-opcao-badge revisao-skeleton-base"><div className="revisao-skeleton-shimmer" /></div>
                <div className="revisao-skeleton-opcao-texto revisao-skeleton-base"><div className="revisao-skeleton-shimmer" /></div>
              </div>
            </div>
          </>
        )}
      </div>

      {/* Mensagem Suave de Carregamento */}
      <div className="revisao-skeleton-mensagem">
        <span className="revisao-spinner" /> {mensagem || t('Preparando questões…')}
      </div>
    </div>
  );
}
