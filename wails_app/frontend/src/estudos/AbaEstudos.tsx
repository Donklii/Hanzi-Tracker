import { useState } from 'react';
import { progresso } from '../../wailsjs/go/models';
import { ListaCartoes } from '../comum/ListaCartoes';
import { STATUS_VOCABULARIO } from '../comum/status';
import { t } from '../i18n/i18n';
import { PopUpBaralhoRecomendacoes } from './PopUpBaralhoRecomendacoes';

interface AbaEstudosProps {
  abaAtiva: string;
  estudando: any[];
  aprendidas: any[];
  cartoesVocabulario: progresso.Vocab[];
  AoEntrarNoCartao: (c: any) => void;
  AoSairDoCartao: () => void;
  AoClicarNoCartao: (c: any) => void;
  SalvarPalavra: (cartao: any, status: string) => void;
  ocultarBadgeTipo?: boolean;
  ordenarPorRanking: boolean;
}

export function AbaEstudos(props: AbaEstudosProps) {
  const {
    abaAtiva, estudando, aprendidas, cartoesVocabulario,
    AoEntrarNoCartao, AoSairDoCartao, AoClicarNoCartao, SalvarPalavra, ocultarBadgeTipo,
    ordenarPorRanking
  } = props;

  const [popupBaralhoAberto, setPopupBaralhoAberto] = useState(false);

  if (abaAtiva !== 'estudando' && abaAtiva !== 'aprendidas') return null;

  const sortList = (list: any[]) => {
    if (!ordenarPorRanking) return list;
    return [...list].sort((a, b) => {
      const posA = a.posicaoRanking || (a as any).PosicaoRanking || 0;
      const posB = b.posicaoRanking || (b as any).PosicaoRanking || 0;
      if (posA > 0 && posB > 0) return posA - posB;
      if (posA > 0) return -1;
      if (posB > 0) return 1;
      
      const aHanzi = a.hanzi || (a as any).Hanzi || a.Hanzi || '';
      const bHanzi = b.hanzi || (b as any).Hanzi || b.Hanzi || '';
      return aHanzi.length - bHanzi.length;
    });
  };

  const pseudoCartaoBaralho = {
    isDeckCard: true,
    hanzi: "🃏",
    pinyin: t("Recomendações"),
    significados: [t("Sorteie 3 cartas com palavras recomendadas para estudo")],
    tipoHanzi: "SISTEMA",
  };

  const listaEstudandoComBaralho = [pseudoCartaoBaralho, ...sortList(estudando)];

  const aoClicarInterno = (c: any) => {
    if (c.isDeckCard) {
      setPopupBaralhoAberto(true);
      return;
    }
    AoClicarNoCartao(c);
  };

  return (
    <>
      {abaAtiva === 'estudando' && (
        <ListaCartoes 
          cartoesVocabulario={cartoesVocabulario} 
          AoEntrarNoCartao={AoEntrarNoCartao} 
          AoSairDoCartao={AoSairDoCartao} 
          AoClicarNoCartao={aoClicarInterno} 
          list={listaEstudandoComBaralho} 
          defaultStatus={STATUS_VOCABULARIO.Estudo} 
          ocultarBadgeTipo={ocultarBadgeTipo}
          SalvarPalavra={SalvarPalavra}
        />
      )}

      {abaAtiva === 'aprendidas' && (
        <ListaCartoes 
          cartoesVocabulario={cartoesVocabulario} 
          AoEntrarNoCartao={AoEntrarNoCartao} 
          AoSairDoCartao={AoSairDoCartao} 
          AoClicarNoCartao={AoClicarNoCartao} 
          list={sortList(aprendidas)} 
          defaultStatus={STATUS_VOCABULARIO.Aprendido} 
          ocultarBadgeTipo={ocultarBadgeTipo}
          SalvarPalavra={SalvarPalavra}
        />
      )}

      <PopUpBaralhoRecomendacoes
        aberto={popupBaralhoAberto}
        aoFechar={() => setPopupBaralhoAberto(false)}
        SalvarPalavra={SalvarPalavra}
        AoClicarNoCartao={AoClicarNoCartao}
        cartoesVocabulario={cartoesVocabulario}
      />
    </>
  );
}
