import React, { useState, useEffect, useMemo } from 'react';
import { progresso, main } from '../../wailsjs/go/models';
import { ListaCartoes } from '../comum/ListaCartoes';
import { STATUS_VOCABULARIO } from '../comum/status';
import { t } from '../i18n/i18n';
import { ABAS } from '../casca/abas';

interface AbaBuscaGlobalProps {
  termoBuscaGlobal: string;
  resultadosBuscaGlobal: main.FlashcardCard[];
  cartoes: any[]; // Descobrimento
  cartoesSecao: any[]; // Palavras dessa seção
  vistas: any[]; // Já vistas
  estudando: any[]; // Estudando
  aprendidas: any[]; // Vocabulário
  cartoesVocabulario: progresso.Vocab[];
  AoEntrarNoCartao: (c: any) => void;
  AoSairDoCartao: () => void;
  AoClicarNoCartao: (c: any) => void;
  ordenarPorRanking: boolean;
  scrollTargetHanzi: string | null;
  setScrollTargetHanzi: (val: string | null) => void;
  SalvarPalavra: (cartao: any, status: string) => void;
  buscaGlobalAtiva?: boolean;
  abaAtiva?: string;
}

const TITULOS_SECAO_CURTA: Record<string, string> = {
  [ABAS.Descobrimento]: 'Descobrimento (Em Tela)',
  [ABAS.TelaUnica]: 'Palavras dessa Seção',
  [ABAS.Vistas]: 'Já Vistas (Histórico)',
  [ABAS.Estudando]: 'Estudando',
  [ABAS.Aprendidas]: 'Vocabulário (Aprendidas)',
};

export function AbaBuscaGlobal(props: AbaBuscaGlobalProps) {
  const {
    termoBuscaGlobal, resultadosBuscaGlobal, cartoes, cartoesSecao, vistas, estudando, aprendidas,
    cartoesVocabulario, AoEntrarNoCartao, AoSairDoCartao, AoClicarNoCartao, ordenarPorRanking,
    scrollTargetHanzi, setScrollTargetHanzi, SalvarPalavra, buscaGlobalAtiva = false, abaAtiva
  } = props;

  const [limiteDisplay, setLimiteDisplay] = useState(50);
  const [nivelFiltroHSK, setNivelFiltroHSK] = useState<number>(0); // 0 = Todos
  const observer = React.useRef<IntersectionObserver | null>(null);
  const bottomRef = React.useRef<HTMLDivElement>(null);

  const ehRankingGeral = termoBuscaGlobal === "[RANKING]";
  const ehHSKMode = termoBuscaGlobal.startsWith("[HSK]");

  // Infinite scroll usando IntersectionObserver
  useEffect(() => {
    if (observer.current) observer.current.disconnect();
    
    observer.current = new IntersectionObserver(entries => {
      if (entries[0].isIntersecting) {
        setLimiteDisplay(prev => prev + 50);
      }
    });
    
    if (bottomRef.current) {
      observer.current.observe(bottomRef.current);
    }
    
    return () => observer.current?.disconnect();
  }, [resultadosBuscaGlobal, nivelFiltroHSK, buscaGlobalAtiva]);

  // Resetar o limite quando o termo, nível ou modo global mudar
  useEffect(() => {
    setLimiteDisplay(50);
  }, [termoBuscaGlobal, nivelFiltroHSK, buscaGlobalAtiva]);

  // Monitora scrollTargetHanzi
  useEffect(() => {
    if (!scrollTargetHanzi) return;
    const allResults = resultadosBuscaGlobal;
    const idx = allResults.findIndex(c => (c.hanzi || (c as any).Hanzi) === scrollTargetHanzi);
    if (idx !== -1 && idx >= limiteDisplay) {
      setLimiteDisplay(idx + 20);
    }
  }, [scrollTargetHanzi, resultadosBuscaGlobal]);

  // Scroll e highlight de card alvo
  useEffect(() => {
    if (scrollTargetHanzi) {
      const el = document.getElementById(`card-${scrollTargetHanzi}`);
      if (el) {
        const timer = setTimeout(() => {
          el.scrollIntoView({ behavior: 'smooth', block: 'center' });
          el.classList.add('highlight-flash');
          const cleanTimer = setTimeout(() => {
            el.classList.remove('highlight-flash');
          }, 1500);
          return () => clearTimeout(cleanTimer);
        }, 150);
        setScrollTargetHanzi(null);
        return () => clearTimeout(timer);
      }
    }
  }, [scrollTargetHanzi, resultadosBuscaGlobal, limiteDisplay]);

  // Pseudo-cartão de captura de tela
  const cartaoCaptura = cartoes.find((c: any) => c.isScreenshotCard);
  const SINONIMOS_CAPTURA = ['captura', 'print', 'screenshot', 'tela', 'imagem', 'ocr'];
  const termoBuscaLimpo = termoBuscaGlobal.toLowerCase().trim();
  const cartaoCapturaCombina = !ehHSKMode && !ehRankingGeral && !!cartaoCaptura && termoBuscaLimpo.length >= 2 &&
    SINONIMOS_CAPTURA.some(s => s.includes(termoBuscaLimpo));

  // Obtém os cartões da aba/seção ativa atual
  const cartoesSecaoAtual = useMemo(() => {
    if (!abaAtiva) return [];
    if (abaAtiva === ABAS.Estudando) return estudando;
    if (abaAtiva === ABAS.Aprendidas) return aprendidas;
    if (abaAtiva === ABAS.Descobrimento) return cartoes;
    if (abaAtiva === ABAS.TelaUnica) return cartoesSecao;
    if (abaAtiva === ABAS.Vistas) return vistas;
    return [];
  }, [abaAtiva, estudando, aprendidas, cartoes, cartoesSecao, vistas]);

  // Agrupamento de cartões divididos por Nível HSK (1 a 7)
  const gruposHSKPorNivel = useMemo(() => {
    if (!ehHSKMode) return {};
    const grupos: Record<number, main.FlashcardCard[]> = { 1: [], 2: [], 3: [], 4: [], 5: [], 6: [], 7: [] };

    if (buscaGlobalAtiva) {
      // Modo Global: usa todo o acervo do banco de dados HSK
      resultadosBuscaGlobal.forEach(res => {
        const lvl = res.nivelHSK || (res as any).NivelHSK || 1;
        if (grupos[lvl]) {
          grupos[lvl].push(res);
        } else {
          grupos[7].push(res);
        }
      });
    } else {
      // Modo Seção Atual: filtra apenas as palavras presentes na aba ativa
      const setSecaoAtual = new Set(cartoesSecaoAtual.map((item: any) => item.hanzi || item.Hanzi));
      resultadosBuscaGlobal.forEach(res => {
        const hanzi = res.hanzi || (res as any).Hanzi;
        if (!setSecaoAtual.has(hanzi)) return;

        const lvl = res.nivelHSK || (res as any).NivelHSK || 1;
        if (grupos[lvl]) {
          grupos[lvl].push(res);
        } else {
          grupos[7].push(res);
        }
      });
    }

    return grupos;
  }, [ehHSKMode, buscaGlobalAtiva, resultadosBuscaGlobal, cartoesSecaoAtual]);

  // Agrupamento padrão para busca textual normal
  const { grupoEstudando, grupoVocabulario, grupoDescobrimento, grupoSecao, grupoVistas, grupoNaoVisto } = useMemo(() => {
    if (ehRankingGeral || ehHSKMode) {
      const vazio: main.FlashcardCard[] = [];
      return { grupoEstudando: vazio, grupoVocabulario: vazio, grupoDescobrimento: vazio, grupoSecao: vazio, grupoVistas: vazio, grupoNaoVisto: vazio };
    }

    const setAprendidas = new Set(aprendidas.map((item: any) => item.hanzi || item.Hanzi));
    const setEstudando = new Set(estudando.map((item: any) => item.hanzi || item.Hanzi));
    const setVistas = new Set(vistas.map((item: any) => item.hanzi || item.Hanzi));
    const setSecao = new Set(cartoesSecao.map((item: any) => item.hanzi || item.Hanzi));
    const setCartoes = new Set(cartoes.map((item: any) => item.hanzi || item.Hanzi));

    const gVocabulario: main.FlashcardCard[] = [];
    const gEstudando: main.FlashcardCard[] = [];
    const gVistas: main.FlashcardCard[] = [];
    const gSecao: main.FlashcardCard[] = [];
    const gDescobrimento: main.FlashcardCard[] = [];
    const gNaoVisto: main.FlashcardCard[] = [];

    resultadosBuscaGlobal.forEach(res => {
      const hanzi = res.hanzi || (res as any).Hanzi;

      if (setAprendidas.has(hanzi)) {
        gVocabulario.push(res);
      } else if (setEstudando.has(hanzi)) {
        gEstudando.push(res);
      } else if (setVistas.has(hanzi)) {
        gVistas.push(res);
      } else if (setSecao.has(hanzi)) {
        gSecao.push(res);
      } else if (setCartoes.has(hanzi)) {
        gDescobrimento.push(res);
      } else {
        gNaoVisto.push(res);
      }
    });

    if (cartaoCapturaCombina && cartaoCaptura) {
      gDescobrimento.push(cartaoCaptura);
    }

    return {
      grupoEstudando: gEstudando,
      grupoVocabulario: gVocabulario,
      grupoDescobrimento: gDescobrimento,
      grupoSecao: gSecao,
      grupoVistas: gVistas,
      grupoNaoVisto: gNaoVisto,
    };
  }, [resultadosBuscaGlobal, aprendidas, estudando, vistas, cartoesSecao, cartoes, ehRankingGeral, ehHSKMode, cartaoCapturaCombina, cartaoCaptura]);

  // Lista fatiada para o Ranking Geral (antiga medalha com Global = ON)
  const listaRankingFatiada = useMemo(() => {
    if (!ehRankingGeral) return [];
    return resultadosBuscaGlobal.slice(0, limiteDisplay);
  }, [ehRankingGeral, resultadosBuscaGlobal, limiteDisplay]);

  function sortResults(list: main.FlashcardCard[]) {
    if (ordenarPorRanking) {
      return list.sort((a, b) => {
        const posA = a.posicaoRanking || (a as any).PosicaoRanking || 0;
        const posB = b.posicaoRanking || (b as any).PosicaoRanking || 0;
        if (posA > 0 && posB > 0) return posA - posB;
        if (posA > 0) return -1;
        if (posB > 0) return 1;
        const aHanzi = a.hanzi || (a as any).Hanzi || '';
        const bHanzi = b.hanzi || (b as any).Hanzi || '';
        return aHanzi.length - bHanzi.length;
      });
    }

    const termClean = termoBuscaGlobal.toLowerCase().replace(/\s/g, "");
    return list.sort((a, b) => {
        const aHanzi = a.hanzi || (a as any).Hanzi || '';
        const bHanzi = b.hanzi || (b as any).Hanzi || '';
        const aPinyin = (a.pinyin || (a as any).Pinyin || '').toLowerCase();
        const bPinyin = (b.pinyin || (b as any).Pinyin || '').toLowerCase();
        
        const aPinyinClean = aPinyin.normalize('NFD').replace(/[\u0300-\u036f]/g, "").replace(/\s/g, "");
        const bPinyinClean = bPinyin.normalize('NFD').replace(/[\u0300-\u036f]/g, "").replace(/\s/g, "");

        if (aHanzi === termClean && bHanzi !== termClean) return -1;
        if (bHanzi === termClean && aHanzi !== termClean) return 1;

        if (aPinyinClean === termClean && bPinyinClean !== termClean) return -1;
        if (bPinyinClean === termClean && aPinyinClean !== termClean) return 1;

        const aStarts = aPinyinClean.startsWith(termClean);
        const bStarts = bPinyinClean.startsWith(termClean);
        if (aStarts && !bStarts) return -1;
        if (bStarts && !aStarts) return 1;

        if (aHanzi.length !== bHanzi.length) {
             return aHanzi.length - bHanzi.length;
        }

        return 0;
    });
  }

  const renderGroup = (title: string, list: main.FlashcardCard[], statusClass: string) => {
    if (list.length === 0) return null;
    const sortedSliced = sortResults([...list]).slice(0, limiteDisplay);
    return (
      <div style={{ marginBottom: '24px' }}>
        <h3 className="settings-section-title" style={{ marginTop: '0' }}>{t(title)} ({list.length})</h3>
        <ListaCartoes
          cartoesVocabulario={cartoesVocabulario}
          AoEntrarNoCartao={AoEntrarNoCartao}
          AoSairDoCartao={AoSairDoCartao}
          AoClicarNoCartao={AoClicarNoCartao}
          list={sortedSliced}
          defaultStatus={statusClass}
          SalvarPalavra={SalvarPalavra}
        />
        {list.length > limiteDisplay && (
          <div style={{ textAlign: 'center', marginTop: '12px', color: 'var(--cor-texto-suave)', fontSize: '12px' }}>
            {t('Deslize para ver mais')}
          </div>
        )}
      </div>
    );
  };

  // ----- MODO RANKING GERAL (Efeito antigo da medalha com Global = ON) -----
  if (ehRankingGeral) {
    return (
      <div style={{ display: 'flex', flexDirection: 'column', paddingBottom: '40px' }}>
        <h3 className="settings-section-title" style={{ marginTop: '0' }}>{t('Ranking Geral (Banco de Dados)')} ({resultadosBuscaGlobal.length})</h3>
        <ListaCartoes
          cartoesVocabulario={cartoesVocabulario}
          AoEntrarNoCartao={AoEntrarNoCartao}
          AoSairDoCartao={AoSairDoCartao}
          AoClicarNoCartao={AoClicarNoCartao}
          list={listaRankingFatiada}
          defaultStatus=""
          SalvarPalavra={SalvarPalavra}
        />
        {resultadosBuscaGlobal.length > limiteDisplay && (
          <div style={{ textAlign: 'center', marginTop: '12px', color: 'var(--cor-texto-suave)', fontSize: '12px' }}>
            {t('Deslize para ver mais')}
          </div>
        )}
        <div ref={bottomRef} style={{ height: '40px' }}></div>
      </div>
    );
  }

  // Componente de Cabeçalho com Chips do HSK
  const renderCabecalhoChipsHSK = () => {
    const niveis = [1, 2, 3, 4, 5, 6, 7];
    const totalHSKEscopo = niveis.reduce((acc, lvl) => acc + (gruposHSKPorNivel[lvl]?.length || 0), 0);
    const tituloSecao = abaAtiva ? TITULOS_SECAO_CURTA[abaAtiva] || 'Seção Atual' : 'Seção Atual';

    const tituloModo = buscaGlobalAtiva 
      ? t('Vocabulário HSK Global') 
      : t(`Vocabulário HSK — ${tituloSecao}`);

    return (
      <div style={{ marginBottom: '20px' }}>
        <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: '12px' }}>
          <h2 className="settings-section-title" style={{ marginTop: 0, marginBottom: 0, fontSize: '16px' }}>
            {tituloModo} ({totalHSKEscopo})
          </h2>
        </div>

        {/* Chips de filtro por Nível HSK */}
        <div style={{ display: 'flex', gap: '8px', flexWrap: 'wrap' }}>
          <button
            onClick={() => setNivelFiltroHSK(0)}
            style={{
              padding: '4px 12px',
              borderRadius: '16px',
              border: '1px solid var(--cor-borda)',
              backgroundColor: nivelFiltroHSK === 0 ? 'var(--cor-destaque)' : 'var(--cor-fundo-secundario)',
              color: nivelFiltroHSK === 0 ? '#fff' : 'var(--cor-texto-primario)',
              cursor: 'pointer',
              fontSize: '12px',
              fontWeight: 'bold',
              transition: 'all 0.2s ease',
            }}
          >
            {t('Todos Níveis')}
          </button>

          {niveis.map(lvl => {
            const count = gruposHSKPorNivel[lvl]?.length || 0;
            const label = lvl === 7 ? 'HSK 7-9' : `HSK ${lvl}`;
            const active = nivelFiltroHSK === lvl;
            return (
              <button
                key={lvl}
                onClick={() => setNivelFiltroHSK(lvl)}
                style={{
                  padding: '4px 12px',
                  borderRadius: '16px',
                  border: '1px solid var(--cor-borda)',
                  backgroundColor: active ? 'var(--cor-destaque)' : 'var(--cor-fundo-secundario)',
                  color: active ? '#fff' : 'var(--cor-texto-primario)',
                  cursor: 'pointer',
                  fontSize: '12px',
                  fontWeight: active ? 'bold' : 'normal',
                  transition: 'all 0.2s ease',
                }}
              >
                {label} ({count})
              </button>
            );
          })}
        </div>
      </div>
    );
  };

  // ----- RENDERIZADOR PROGRESSIVO DE SEÇÕES DE NÍVEIS HSK (Compartilhado entre Global e Seção Atual) -----
  const renderProgressiveHSK = () => {
    const niveis = [1, 2, 3, 4, 5, 6, 7];
    const niveisExibicao = nivelFiltroHSK === 0 ? niveis : [nivelFiltroHSK];

    let limiteRestante = limiteDisplay;
    const elementos: React.ReactNode[] = [];

    const totalHSKEscopo = niveis.reduce((acc, lvl) => acc + (gruposHSKPorNivel[lvl]?.length || 0), 0);

    if (totalHSKEscopo === 0) {
      return (
        <div style={{ color: 'var(--cor-texto-suave)', marginTop: '16px' }}>
          {t('Nenhuma palavra HSK encontrada nesta seção para o filtro selecionado.')}
        </div>
      );
    }

    for (const lvl of niveisExibicao) {
      const list = gruposHSKPorNivel[lvl] || [];
      if (list.length === 0) continue;

      if (limiteRestante <= 0) {
        break;
      }

      const qtdExibir = Math.min(list.length, limiteRestante);
      const sortedSliced = list.slice(0, qtdExibir);
      const title = lvl === 7 ? 'HSK Nível 7-9 (Superior)' : `HSK Nível ${lvl}`;
      const concluido = qtdExibir >= list.length;

      elementos.push(
        <div key={lvl} style={{ marginBottom: '28px' }}>
          <h3 className="settings-section-title" style={{ marginTop: '0', display: 'flex', alignItems: 'center', gap: '8px' }}>
            <span>{title}</span>
            <span style={{ fontSize: '12px', fontWeight: 'normal', color: 'var(--cor-texto-suave)' }}>
              ({qtdExibir} / {list.length} palavras)
            </span>
          </h3>
          <ListaCartoes
            cartoesVocabulario={cartoesVocabulario}
            AoEntrarNoCartao={AoEntrarNoCartao}
            AoSairDoCartao={AoSairDoCartao}
            AoClicarNoCartao={AoClicarNoCartao}
            list={sortedSliced}
            defaultStatus=""
            SalvarPalavra={SalvarPalavra}
          />
          {!concluido && (
            <div style={{ textAlign: 'center', marginTop: '12px', color: 'var(--cor-texto-suave)', fontSize: '12px' }}>
              {t('Role até o final para carregar mais palavras deste nível...')}
            </div>
          )}
        </div>
      );

      limiteRestante -= qtdExibir;

      if (!concluido) {
        break;
      }
    }

    return (
      <>
        {elementos}
        <div ref={bottomRef} style={{ height: '40px' }}></div>
      </>
    );
  };

  // ----- MODO HSK (Global ou Seção Atual) -----
  if (ehHSKMode) {
    return (
      <div style={{ display: 'flex', flexDirection: 'column', paddingBottom: '40px' }}>
        {renderCabecalhoChipsHSK()}
        {renderProgressiveHSK()}
      </div>
    );
  }

  if (resultadosBuscaGlobal.length === 0 && !cartaoCapturaCombina) {
    return <div style={{ color: 'var(--cor-texto-suave)' }}>{t('Nenhum resultado encontrado.')}</div>;
  }

  // ----- MODO BUSCA PADRÃO -----
  return (
    <div style={{ display: 'flex', flexDirection: 'column', paddingBottom: '40px' }}>
      {renderGroup('Estudando', grupoEstudando, STATUS_VOCABULARIO.Estudo)}
      {renderGroup('Vocabulário (Aprendidas)', grupoVocabulario, STATUS_VOCABULARIO.Aprendido)}
      {renderGroup('Descobrimento (Em Tela)', grupoDescobrimento, '')}
      {renderGroup('Palavras dessa Seção', grupoSecao, '')}
      {renderGroup('Já Vistas (Histórico)', grupoVistas, STATUS_VOCABULARIO.Visto)}
      {renderGroup('Ainda Não Visto', grupoNaoVisto, '')}
      <div ref={bottomRef} style={{ height: '40px' }}></div>
    </div>
  );
}
