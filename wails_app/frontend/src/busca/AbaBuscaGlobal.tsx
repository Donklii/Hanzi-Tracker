import React, { useState, useEffect } from 'react';
import { progresso, main } from '../../wailsjs/go/models';
import { ListaCartoes } from '../comum/ListaCartoes';

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
}

export function AbaBuscaGlobal(props: AbaBuscaGlobalProps) {
  const {
    termoBuscaGlobal, resultadosBuscaGlobal, cartoes, cartoesSecao, vistas, estudando, aprendidas,
    cartoesVocabulario, AoEntrarNoCartao, AoSairDoCartao, AoClicarNoCartao
  } = props;

  const [limiteDisplay, setLimiteDisplay] = useState(50);
  const observer = React.useRef<IntersectionObserver | null>(null);
  const bottomRef = React.useRef<HTMLDivElement>(null);

  // Infite scroll super simples usando IntersectionObserver
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
  }, [resultadosBuscaGlobal]);

  // Resetar o limite quando o termo mudar
  useEffect(() => {
    setLimiteDisplay(50);
  }, [termoBuscaGlobal]);

  // Lógica de Agrupamento
  // Prioridade: Vocabulário > Estudando > Já Vistas > Palavras da seção > Descobrimento > Ainda não visto
  
  const grupoVocabulario: main.FlashcardCard[] = [];
  const grupoEstudando: main.FlashcardCard[] = [];
  const grupoVistas: main.FlashcardCard[] = [];
  const grupoSecao: main.FlashcardCard[] = [];
  const grupoDescobrimento: main.FlashcardCard[] = [];
  const grupoNaoVisto: main.FlashcardCard[] = [];

  const isInList = (list: any[], hanzi: string) => {
    return list.some(item => (item.hanzi || item.Hanzi) === hanzi);
  };

  resultadosBuscaGlobal.forEach(res => {
    const hanzi = res.hanzi || (res as any).Hanzi;
    
    if (isInList(aprendidas, hanzi)) {
      grupoVocabulario.push(res);
    } else if (isInList(estudando, hanzi)) {
      grupoEstudando.push(res);
    } else if (isInList(vistas, hanzi)) {
      grupoVistas.push(res);
    } else if (isInList(cartoesSecao, hanzi)) {
      grupoSecao.push(res);
    } else if (isInList(cartoes, hanzi)) {
      grupoDescobrimento.push(res);
    } else {
      grupoNaoVisto.push(res);
    }
  });

  const sortResults = (list: main.FlashcardCard[]) => {
    const termClean = termoBuscaGlobal.toLowerCase().replace(/\s/g, "");
    return list.sort((a, b) => {
        const aHanzi = a.hanzi || (a as any).Hanzi || '';
        const bHanzi = b.hanzi || (b as any).Hanzi || '';
        const aPinyin = (a.pinyin || (a as any).Pinyin || '').toLowerCase();
        const bPinyin = (b.pinyin || (b as any).Pinyin || '').toLowerCase();
        
        const aPinyinClean = aPinyin.normalize('NFD').replace(/[\u0300-\u036f]/g, "").replace(/\s/g, "");
        const bPinyinClean = bPinyin.normalize('NFD').replace(/[\u0300-\u036f]/g, "").replace(/\s/g, "");

        // 1. Exato Hanzi
        if (aHanzi === termClean && bHanzi !== termClean) return -1;
        if (bHanzi === termClean && aHanzi !== termClean) return 1;

        // 2. Exato Pinyin
        if (aPinyinClean === termClean && bPinyinClean !== termClean) return -1;
        if (bPinyinClean === termClean && aPinyinClean !== termClean) return 1;

        // 3. Pinyin Starts With
        const aStarts = aPinyinClean.startsWith(termClean);
        const bStarts = bPinyinClean.startsWith(termClean);
        if (aStarts && !bStarts) return -1;
        if (bStarts && !aStarts) return 1;

        // 4. Avaliação de Significado
        const originalTerm = termoBuscaGlobal.toLowerCase().trim();
        const evalMeaning = (meanings: string[], term: string) => {
            if (!term) return { hasExactIsolated: false, hasFirstWord: false, hasAnyIsolated: false };
            let hasExactIsolated = false;
            let hasFirstWord = false;
            let hasAnyIsolated = false;
            try {
                const regex = new RegExp(`\\b${term}\\b`, 'i');
                for (const m of meanings) {
                    const lowerM = m.toLowerCase();
                    const match = lowerM.match(regex);
                    if (match) {
                        hasAnyIsolated = true;
                        if (match.index === 0) hasFirstWord = true;
                        if (lowerM === term) hasExactIsolated = true;
                    }
                }
            } catch (e) {
                // Ignore invalid regex if term has special regex chars
            }
            return { hasExactIsolated, hasFirstWord, hasAnyIsolated };
        };

        const aMeanings = a.significados || (a as any).Significados || [];
        const bMeanings = b.significados || (b as any).Significados || [];
        const aEval = evalMeaning(aMeanings, originalTerm);
        const bEval = evalMeaning(bMeanings, originalTerm);

        if (aEval.hasExactIsolated && !bEval.hasExactIsolated) return -1;
        if (bEval.hasExactIsolated && !aEval.hasExactIsolated) return 1;
        
        if (aEval.hasFirstWord && !bEval.hasFirstWord) return -1;
        if (bEval.hasFirstWord && !aEval.hasFirstWord) return 1;
        
        if (aEval.hasAnyIsolated && !bEval.hasAnyIsolated) return -1;
        if (bEval.hasAnyIsolated && !aEval.hasAnyIsolated) return 1;

        // 5. Hanzi length (shorter first)
        if (aHanzi.length !== bHanzi.length) {
             return aHanzi.length - bHanzi.length;
        }

        return 0;
    });
  };

  const renderGroup = (title: string, list: main.FlashcardCard[], statusClass: string) => {
    if (list.length === 0) return null;
    const sortedSliced = sortResults(list).slice(0, limiteDisplay);
    return (
      <div style={{ marginBottom: '24px' }}>
        <h3 className="settings-section-title" style={{ marginTop: '0' }}>{title} ({list.length})</h3>
        <ListaCartoes
          cartoesVocabulario={cartoesVocabulario}
          AoEntrarNoCartao={AoEntrarNoCartao}
          AoSairDoCartao={AoSairDoCartao}
          AoClicarNoCartao={AoClicarNoCartao}
          list={sortedSliced}
          defaultStatus={statusClass}
          actionBtns={() => <></>}
        />
        {list.length > limiteDisplay && (
          <div style={{ textAlign: 'center', marginTop: '12px', color: 'var(--cor-texto-suave)', fontSize: '12px' }}>
            Deslize para ver mais
          </div>
        )}
      </div>
    );
  };

  if (resultadosBuscaGlobal.length === 0) {
    return <div style={{ color: 'var(--cor-texto-suave)' }}>Nenhum resultado encontrado.</div>;
  }

  return (
    <div style={{ display: 'flex', flexDirection: 'column', paddingBottom: '40px' }}>
      {renderGroup('Estudando', grupoEstudando, 'estudo')}
      {renderGroup('Vocabulário (Aprendidas)', grupoVocabulario, 'aprendido')}
      {renderGroup('Descobrimento (Em Tela)', grupoDescobrimento, '')}
      {renderGroup('Palavras dessa Seção', grupoSecao, '')}
      {renderGroup('Já Vistas (Histórico)', grupoVistas, 'visto')}
      {renderGroup('Ainda Não Visto', grupoNaoVisto, '')}
      <div ref={bottomRef} style={{ height: '40px' }}></div>
    </div>
  );
}
