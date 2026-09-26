// ----- Seção: Revisão — Placar Final -----
// Tela de celebração da sessão: anel de progresso com contagem animada, chuva de confetes quando
// o desempenho é bom (>= 60%), estatísticas de gamificação, o PROGRESSO POR PALAVRA nas 5 áreas
// de aprendizado (comparando o antes/depois da sessão, com o que avançou em destaque) e o card
// de palavras que atingiram a meta de estudo — cujo botão de marcar como aprendida conduz o
// feedback completo (salvando → sucesso/falha) sem sair da tela.
import { useEffect, useMemo, useState } from 'react';
import { revisao } from '../../wailsjs/go/models';
import { t } from '../i18n/i18n';
import { AREAS_APRENDIZADO, IconeAcertos, IconeCheck, IconeFoco, IconePontos, IconeReset, IconeSequencia, IconeTrofeu } from './IconesPlacar';
import { obterTiposBolinhas } from './progressoBolinhas';

// --- Modelos de exibição (montados pela AbaRevisao ao concluir a sessão) ---

// Uma palavra sugerida para virar APRENDIDA (meta de estudo atingida nas 5 áreas).
export interface SugestaoAprendido {
  hanzi: string;
  pinyin: string;
  significado: string;
}

// O estado de UMA área de aprendizado de uma palavra, já comparado com o retrato pré-sessão.
export interface AreaProgressoPlacar {
  area: string;
  streak: number; // acertos consecutivos após a sessão
  delta: number; // variação líquida na sessão (negativo = a sequência zerou)
  concluida: boolean; // streak >= meta
  concluidaAgora: boolean; // cruzou a meta NESTA sessão
}

// Uma palavra praticada na sessão, com as 5 áreas e o ganho total (ordena a lista do placar).
export interface ProgressoPalavraPlacar {
  hanzi: string;
  pinyin: string;
  significados: string[];
  status: string;
  areas: AreaProgressoPlacar[];
  ganhoTotal: number;
}

// Consolida o retrato pós-sessão com o retrato inicial: para cada palavra e área, o streak
// atual, o delta líquido da sessão e se a meta foi cruzada agora. Sem retrato inicial (a busca
// pré-sessão falhou), os deltas ficam em 0 e o placar mostra só o estado atual. As palavras que
// mais avançaram vêm primeiro.
export function montarProgressoPlacar(
  retrato: revisao.ProgressoRevisaoPalavras,
  antesPorHanzi: Map<string, revisao.ProgressoPalavraRevisao> | null,
): ProgressoPalavraPlacar[] {
  const meta = retrato.meta || 3;
  const itens = (retrato.palavras || []).map((p: revisao.ProgressoPalavraRevisao) => {
    const antes = antesPorHanzi ? antesPorHanzi.get(p.hanzi) : undefined;
    const areas = AREAS_APRENDIZADO.map(({ area }) => {
      const streak = (p.streaks && p.streaks[area]) || 0;
      const streakAntes = antes && antes.streaks ? (antes.streaks[area] ?? streak) : streak;
      return {
        area,
        streak,
        delta: streak - streakAntes,
        concluida: streak >= meta,
        concluidaAgora: streak >= meta && streakAntes < meta,
      };
    });
    const ganhoTotal = areas.reduce((soma, a) => soma + Math.max(a.delta, 0), 0);
    return {
      hanzi: p.hanzi,
      pinyin: p.pinyin,
      significados: p.significados || [],
      status: p.status,
      areas,
      ganhoTotal,
    };
  });
  return itens.sort((a: ProgressoPalavraPlacar, b: ProgressoPalavraPlacar) => b.ganhoTotal - a.ganhoTotal);
}

// O binding ObterSugestoesAprendidoLote devolve progresso.Vocab com campos Maiúsculos (a struct
// Go não tem tag json em Hanzi/Pinyin/Significado). Sem esta normalização o placar comparava
// item.hanzi (undefined) e o clique em "Aprendida" não surtia efeito visível na lista.
export function normalizarSugestoes(sugs: any[]): SugestaoAprendido[] {
  return (sugs || [])
    .map(s => ({
      hanzi: s.hanzi ?? s.Hanzi ?? '',
      pinyin: s.pinyin ?? s.Pinyin ?? '',
      significado: s.significado ?? s.Significado ?? '',
    }))
    .filter(s => s.hanzi !== '');
}

interface PlacarRevisaoProps {
  acertos: number;
  total: number;
  modo: string;
  pontos: number;
  melhorSequencia: number;
  sugestoes?: SugestaoAprendido[];
  progresso?: ProgressoPalavraPlacar[];
  meta?: number;
  focoNovas?: revisao.ItemFocoRevisao[]; // palavras que ENTRARAM no foco ao marcar sugestões como aprendidas
  aoRepetir: () => void;
  aoTrocarModo: () => void;
  aoAdicionarComoAprendida?: (hanzi: string, pinyin: string, significado: string) => Promise<void>;
}

const ROTULOS_MODO: Record<string, string> = {
  geral: 'Geral',
  ia: 'Revisão com IA',
  significado: 'Significado',
  fonetica: 'Fonética',
  desenho: 'Desenho',
  contexto: 'Contexto',
  ordenacao: 'Ordenação',
  pronuncia: 'Pronúncia',
};

const CORES_CONFETE = ['#6366f1', '#10b981', '#f59e0b', '#ef4444', '#a5b4fc', '#34d399'];
const TOTAL_CONFETES = 40;

// Geometria do anel de progresso (o arco é desenhado por stroke-dashoffset).
const RAIO_ANEL = 54;
const CIRCUNFERENCIA_ANEL = 2 * Math.PI * RAIO_ANEL;

function mensagemPorFaixa(percentual: number): string {
  if (percentual === 100) return t('Perfeito! 完美!');
  if (percentual >= 80) return t('Excelente!');
  if (percentual >= 60) return t('Bom progresso!');
  if (percentual >= 40) return t('Continue praticando!');
  return t('Não desista — a repetição é o caminho.');
}

export function PlacarRevisao({
  acertos,
  total,
  modo,
  pontos,
  melhorSequencia,
  sugestoes = [],
  progresso = [],
  meta = 3,
  focoNovas = [],
  aoRepetir,
  aoTrocarModo,
  aoAdicionarComoAprendida
}: PlacarRevisaoProps) {
  const percentual = total > 0 ? Math.min(100, Math.max(0, Math.round((acertos / total) * 100))) : 0;

  // Feedback do card de sugestões: qual palavra está sendo gravada, quais já viraram
  // aprendidas nesta tela e a última cuja gravação falhou (para oferecer nova tentativa).
  const [salvandoAprendida, setSalvandoAprendida] = useState<string | null>(null);
  const [aprendidas, setAprendidas] = useState<Set<string>>(new Set());
  const [falhaAprendida, setFalhaAprendida] = useState<string | null>(null);

  // Contagem animada: o número sobe de 0 até o percentual em ~900ms (ease-out); o arco do anel
  // acompanha o mesmo valor.
  const [percentualExibido, setPercentualExibido] = useState(0);
  useEffect(() => {
    let quadro = 0;
    const inicio = performance.now();
    const duracaoMs = 900;

    function animar(agora: number) {
      const progressoAnimacao = Math.min((agora - inicio) / duracaoMs, 1);
      const suavizado = 1 - Math.pow(1 - progressoAnimacao, 3); // ease-out cúbico
      setPercentualExibido(Math.round(percentual * suavizado));
      if (progressoAnimacao < 1) quadro = requestAnimationFrame(animar);
    }

    quadro = requestAnimationFrame(animar);
    return () => cancelAnimationFrame(quadro);
  }, [percentual]);

  // Confetes: posições/cores/tempos sorteados uma única vez por montagem do placar.
  const confetes = useMemo(() => {
    if (percentual < 60) return [];
    return Array.from({ length: TOTAL_CONFETES }, (_, i) => ({
      chave: i,
      esquerda: Math.random() * 100,
      cor: CORES_CONFETE[i % CORES_CONFETE.length],
      atrasoS: Math.random() * 0.8,
      duracaoS: 2 + Math.random() * 1.5,
      tamanhoPx: 6 + Math.random() * 6,
    }));
  }, [percentual]);

  function marcarAprendida(sug: SugestaoAprendido) {
    if (!aoAdicionarComoAprendida || salvandoAprendida) return;
    setFalhaAprendida(atual => (atual === sug.hanzi ? null : atual));
    setSalvandoAprendida(sug.hanzi);
    aoAdicionarComoAprendida(sug.hanzi, sug.pinyin, sug.significado)
      .then(() => setAprendidas(prev => new Set(prev).add(sug.hanzi)))
      .catch((err: any) => {
        console.error('Erro ao marcar palavra como aprendida:', err);
        setFalhaAprendida(sug.hanzi);
      })
      .finally(() => setSalvandoAprendida(null));
  }

  return (
    <div className="revisao-placar">
      {confetes.length > 0 && (
        <div className="revisao-confetes" aria-hidden="true">
          {confetes.map(c => (
            <span
              key={c.chave}
              className="revisao-confete"
              style={{
                left: `${c.esquerda}%`,
                backgroundColor: c.cor,
                width: `${c.tamanhoPx}px`,
                height: `${c.tamanhoPx * 0.45}px`,
                animationDelay: `${c.atrasoS}s`,
                animationDuration: `${c.duracaoS}s`,
              }}
            />
          ))}
        </div>
      )}

      {/* Anel de progresso com o percentual no centro */}
      <div className={`revisao-placar-anel ${percentual === 100 ? 'perfeito' : ''}`}>
        <svg width="150" height="150" viewBox="0 0 150 150">
          <circle className="revisao-placar-anel-trilho" cx="75" cy="75" r={RAIO_ANEL} />
          <circle
            className="revisao-placar-anel-arco"
            cx="75"
            cy="75"
            r={RAIO_ANEL}
            strokeDasharray={CIRCUNFERENCIA_ANEL}
            strokeDashoffset={CIRCUNFERENCIA_ANEL * (1 - percentualExibido / 100)}
          />
        </svg>
        <div className="revisao-placar-anel-conteudo">
          <div className="revisao-placar-percentual">{percentualExibido}%</div>
          <div className="revisao-placar-percentual-sub">{acertos}/{total}</div>
        </div>
      </div>

      <div className="revisao-placar-mensagem">{mensagemPorFaixa(percentual)}</div>
      <div className="revisao-placar-detalhe">
        {t('{acertos} de {total} questões corretas — modo {modo}', { acertos, total, modo: t(ROTULOS_MODO[modo]) || modo })}
      </div>

      {/* Estatísticas de gamificação da sessão */}
      <div className="revisao-placar-estatisticas">
        <div className="revisao-placar-estatistica">
          <div className="revisao-placar-estatistica-valor pontos"><IconePontos tamanho={17} /> {pontos}</div>
          <div className="revisao-placar-estatistica-rotulo">{t('pontos')}</div>
        </div>
        <div className="revisao-placar-estatistica">
          <div className="revisao-placar-estatistica-valor sequencia"><IconeSequencia tamanho={17} /> {melhorSequencia}</div>
          <div className="revisao-placar-estatistica-rotulo">{t('melhor sequência')}</div>
        </div>
        <div className="revisao-placar-estatistica">
          <div className="revisao-placar-estatistica-valor acertos"><IconeAcertos tamanho={17} /> {acertos}</div>
          <div className="revisao-placar-estatistica-rotulo">{t('acertos')}</div>
        </div>
      </div>

      {sugestoes.length > 0 && (
        <div className="revisao-sugestoes-aprendido">
          <div className="revisao-sugestao-titulo">
            <IconeTrofeu tamanho={17} /> {t('Meta de estudo atingida!')}
          </div>
          <div className="revisao-sugestao-descricao">
            {t('Você dominou as 5 áreas destas palavras ({meta} acertos seguidos em cada). Marque como aprendidas para abrir vaga no grupo de foco.', { meta })}
          </div>
          <div className="revisao-sugestao-lista">
            {sugestoes.map(sug => {
              const marcada = aprendidas.has(sug.hanzi);
              const salvando = salvandoAprendida === sug.hanzi;
              return (
                <div key={sug.hanzi} className={`revisao-sugestao-item ${marcada ? 'marcada' : ''}`}>
                  <div className="revisao-sugestao-item-linha">
                    <div className="revisao-sugestao-item-info">
                      <span className="revisao-sugestao-item-hanzi">{sug.hanzi}</span>
                      <div className="revisao-sugestao-item-detalhe">
                        <span className="revisao-sugestao-item-pinyin">{sug.pinyin}</span>
                        <span className="revisao-sugestao-item-significado" title={sug.significado}>{sug.significado}</span>
                      </div>
                    </div>
                    {marcada ? (
                      <span className="revisao-sugestao-item-feita">
                        <IconeCheck tamanho={14} /> {t('Aprendida!')}
                      </span>
                    ) : (
                      <button
                        className="revisao-sugestao-item-btn"
                        disabled={salvandoAprendida !== null}
                        onClick={() => marcarAprendida(sug)}
                      >
                        {salvando ? <span className="revisao-spinner" /> : <IconeCheck tamanho={13} />}
                        {salvando ? t('Salvando…') : t('Aprendi!')}
                      </button>
                    )}
                  </div>
                  {falhaAprendida === sug.hanzi && (
                    <div className="revisao-sugestao-item-erro">
                      {t('Não foi possível salvar — tente de novo.')}
                    </div>
                  )}
                </div>
              );
            })}
          </div>
        </div>
      )}

      {focoNovas.length > 0 && (
        <div className="revisao-placar-foco-novas">
          <div className="revisao-placar-foco-novas-titulo"><IconeFoco tamanho={15} /> {t('Entraram no seu foco')}</div>
          <div className="revisao-placar-foco-novas-descricao">
            {t('Vaga aberta! Estas palavras da sua fila de estudo passam a ser priorizadas nas próximas revisões.')}
          </div>
          <div className="revisao-placar-foco-novas-lista">
            {focoNovas.map(item => (
              <span key={item.hanzi} className="revisao-foco-chip revisao-foco-chip-nova" title={item.significados?.join(', ')}>
                <span className="revisao-foco-chip-hanzi">{item.hanzi}</span>
                <span className="revisao-foco-chip-pinyin">{item.pinyin}</span>
              </span>
            ))}
          </div>
        </div>
      )}

      {/* Progresso por palavra: as 5 áreas de aprendizado, com o avanço da sessão em destaque */}
      {progresso.length > 0 && (
        <div className="revisao-placar-progresso">
          <div className="revisao-placar-secao-titulo">{t('Progresso da sessão')}</div>
          <div className="revisao-placar-secao-descricao">
            {t('Acertos seguidos de cada palavra por área — ao completar {meta} bolinhas nas 5 áreas, ela fica pronta para virar aprendida.', { meta })}
          </div>
          <div className="revisao-placar-progresso-lista">
            {progresso.map((item, indice) => (
              <div
                key={item.hanzi}
                className={`revisao-progresso-palavra ${aprendidas.has(item.hanzi) ? 'aprendida' : ''}`}
                style={{ animationDelay: `${Math.min(indice, 8) * 70}ms` }}
              >
                <div className="revisao-progresso-palavra-cabecalho">
                  <span className="revisao-progresso-palavra-hanzi">{item.hanzi}</span>
                  <div className="revisao-progresso-palavra-leitura">
                    <span className="revisao-progresso-palavra-pinyin">{item.pinyin}</span>
                    <span className="revisao-progresso-palavra-significado" title={item.significados.join(', ')}>
                      {item.significados.join(', ')}
                    </span>
                  </div>
                  {aprendidas.has(item.hanzi) ? (
                    <span className="revisao-progresso-palavra-chip aprendida"><IconeCheck tamanho={11} /> {t('Aprendida')}</span>
                  ) : item.ganhoTotal > 0 && (
                    <span className="revisao-progresso-palavra-chip ganho">{t('+{ganho} nesta sessão', { ganho: item.ganhoTotal })}</span>
                  )}
                </div>
                <div className="revisao-progresso-areas">
                  {item.areas.map(area => (
                    <PilulaArea key={area.area} area={area} meta={meta} />
                  ))}
                </div>
              </div>
            ))}
          </div>
        </div>
      )}

      <div className="revisao-placar-acoes">
        <button className="scan-btn" onClick={aoRepetir}>{t('Revisar novamente')}</button>
        <button
          className="scan-btn"
          style={{ backgroundColor: 'var(--cor-fundo-cartao)', border: '1px solid var(--cor-borda)' }}
          onClick={aoTrocarModo}
        >
          {modo === 'jornada' ? t('Continuar') : t('Trocar de modo')}
        </button>
      </div>
    </div>
  );
}

// Pílula de UMA área de uma palavra: ícone da área + bolinhas do streak (limitadas à meta) +
// o que mudou nesta sessão (badge +N, aviso de sequência zerada ou o check de área dominada).
function PilulaArea({ area, meta }: { area: AreaProgressoPlacar; meta: number }) {
  const info = AREAS_APRENDIZADO.find(a => a.area === area.area);
  if (!info) return null;

  const rotuloTraduzido = t(info.rotulo);
  const acertosSeguidos = area.streak;
  const bolinhas = obterTiposBolinhas(area.streak);
  let titulo = t('{rotulo}: {acertosSeguidos} acertos seguidos', { rotulo: rotuloTraduzido, acertosSeguidos });
  if (area.concluidaAgora) titulo += ' ' + t('— meta atingida nesta sessão! 🎉');
  else if (area.concluida) titulo += ' ' + t('— área dominada');
  else if (area.delta > 0) titulo += ' ' + t('(+{delta} nesta sessão)', { delta: area.delta });
  else if (area.delta < 0) titulo += ' ' + t('— a sequência zerou nesta sessão');

  const classe = [
    'revisao-area-pilula',
    area.concluida ? 'concluida' : '',
    area.concluidaAgora ? 'concluida-agora' : '',
    area.delta > 0 ? 'avancou' : '',
    area.delta < 0 ? 'zerou' : '',
  ].filter(Boolean).join(' ');

  return (
    <div className={classe} title={titulo}>
      <info.Icone tamanho={13} />
      <span className="revisao-area-pontinhos">
        {bolinhas.map((tipo, i) => (
          <span key={i} className={`revisao-area-pontinho ${tipo === 'vazia' ? '' : `cheio ${tipo}`}`} />
        ))}
      </span>
      {area.delta > 0 && <span className="revisao-area-delta">+{area.delta}</span>}
      {area.delta < 0 && <span className="revisao-area-reset"><IconeReset tamanho={10} /></span>}
      {area.concluida && <span className="revisao-area-check"><IconeCheck tamanho={11} /></span>}
    </div>
  );
}
