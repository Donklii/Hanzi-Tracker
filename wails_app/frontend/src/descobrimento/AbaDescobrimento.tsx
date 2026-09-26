// ----- Seção: Descobrimento -----
// Renderiza as três abas que exibem cartões crus (último OCR, acumulado da seção e histórico de
// vistas). As três usam a MESMA ListaCartoes: o que muda entre elas é só a lista de origem, então
// a aba escolhe a lista num registro em vez de repetir três blocos de JSX idênticos.
import { progresso } from '../../wailsjs/go/models';
import { ListaCartoes } from '../comum/ListaCartoes';
import { DeduplicarCartoes } from '../comum/cartoes';
import { STATUS_VOCABULARIO } from '../comum/status';
import { ABAS, Aba } from '../casca/abas';
import { t } from '../i18n/i18n';

interface AbaDescobrimentoProps {
  abaAtiva: Aba;
  cartoes: any[];
  cartoesSecao: any[];
  vistas: any[];
  cartoesVocabulario: progresso.Vocab[];
  AoEntrarNoCartao: (c: any) => void;
  AoSairDoCartao: () => void;
  AoClicarNoCartao: (c: any) => void;
  SalvarPalavra: (cartao: any, status: string) => void;
  ocultarBadgeTipo?: boolean;
  ordenarPorRanking: boolean;
}


export function AbaDescobrimento(props: AbaDescobrimentoProps) {
  const {
    abaAtiva, cartoes, cartoesSecao, vistas, cartoesVocabulario,
    AoEntrarNoCartao, AoSairDoCartao, AoClicarNoCartao, SalvarPalavra, ocultarBadgeTipo,
    ordenarPorRanking
  } = props;

  // Thunks (e não valores): só a lista da aba ativa é calculada — DeduplicarCartoes varre o
  // acumulado da seção e não deve rodar quando a aba nem está na tela.
  const listaPorAba: Partial<Record<Aba, () => any[]>> = {
    [ABAS.Descobrimento]: () => cartoes,
    [ABAS.TelaUnica]: () => DeduplicarCartoes(cartoesSecao),
    [ABAS.Vistas]: () => vistas,
  };

  const obterLista = listaPorAba[abaAtiva];
  if (!obterLista) {
    return null;
  }

  let finalRawList = obterLista();
  if (ordenarPorRanking) {
    finalRawList = [...finalRawList].sort((a, b) => {
      const posA = a.posicaoRanking || (a as any).PosicaoRanking || 0;
      const posB = b.posicaoRanking || (b as any).PosicaoRanking || 0;
      
      if (posA > 0 && posB > 0) return posA - posB;
      if (posA > 0) return -1;
      if (posB > 0) return 1;
      
      const aHanzi = a.hanzi || (a as any).Hanzi || a.Hanzi || '';
      const bHanzi = b.hanzi || (b as any).Hanzi || b.Hanzi || '';
      return aHanzi.length - bHanzi.length;
    });
  }

  return (
    <ListaCartoes
      key={abaAtiva + (ordenarPorRanking ? '-sorted' : '')} // remonta ao trocar de aba ou ordenação
      cartoesVocabulario={cartoesVocabulario}
      AoEntrarNoCartao={AoEntrarNoCartao}
      AoSairDoCartao={AoSairDoCartao}
      AoClicarNoCartao={AoClicarNoCartao}
      list={finalRawList}
      defaultStatus={STATUS_VOCABULARIO.Visto}
      SalvarPalavra={SalvarPalavra}
      ocultarBadgeTipo={ocultarBadgeTipo}
    />
  );
}
