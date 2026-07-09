// ----- Seção: Descobrimento -----
// Renderiza as três abas que exibem cartões crus (último OCR, acumulado da seção e histórico de
// vistas). As três usam a MESMA ListaCartoes: o que muda entre elas é só a lista de origem, então
// a aba escolhe a lista num registro em vez de repetir três blocos de JSX idênticos.
import { progresso } from '../../wailsjs/go/models';
import { ListaCartoes } from '../comum/ListaCartoes';
import { DeduplicarCartoes } from '../comum/cartoes';
import { STATUS_VOCABULARIO } from '../comum/status';
import { ABAS, Aba } from '../casca/abas';

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
}


export function AbaDescobrimento(props: AbaDescobrimentoProps) {
  const {
    abaAtiva, cartoes, cartoesSecao, vistas, cartoesVocabulario,
    AoEntrarNoCartao, AoSairDoCartao, AoClicarNoCartao, SalvarPalavra, ocultarBadgeTipo
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

  return (
    <ListaCartoes
      key={abaAtiva} // remonta ao trocar de aba, como faziam os três blocos separados de antes
      cartoesVocabulario={cartoesVocabulario}
      AoEntrarNoCartao={AoEntrarNoCartao}
      AoSairDoCartao={AoSairDoCartao}
      AoClicarNoCartao={AoClicarNoCartao}
      list={obterLista()}
      defaultStatus={STATUS_VOCABULARIO.Visto}
      actionBtns={(cartao: any) => <BotaoMoverParaEstudo cartao={cartao} SalvarPalavra={SalvarPalavra} />}
      ocultarBadgeTipo={ocultarBadgeTipo}
    />
  );
}


interface BotaoMoverParaEstudoProps {
  cartao: any;
  SalvarPalavra: (cartao: any, status: string) => void;
}

function BotaoMoverParaEstudo({ cartao, SalvarPalavra }: BotaoMoverParaEstudoProps) {
  return (
    <button
      className="scan-btn"
      style={{ padding: '4px 8px', fontSize: '11px' }}
      onClick={() => SalvarPalavra(cartao, STATUS_VOCABULARIO.Estudo)}
    >
      + Mover p/ Estudo
    </button>
  );
}
