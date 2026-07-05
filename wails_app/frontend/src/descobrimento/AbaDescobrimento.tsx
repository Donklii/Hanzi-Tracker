// ----- Seção: Descobrimento -----
import { progresso } from '../../wailsjs/go/models';
import { ListaCartoes } from '../comum/ListaCartoes';

interface AbaDescobrimentoProps {
  abaAtiva: string;
  cartoes: any[];
  cartoesSecao: any[];
  vistas: any[];
  cartoesVocabulario: progresso.Vocab[];
  AoEntrarNoCartao: (c: any) => void;
  AoSairDoCartao: () => void;
  AoClicarNoCartao: (c: any) => void;
  SalvarPalavra: (hanzi: string, status: string, callback?: () => void) => void;
  DeduplicarCartoes: (lista: any[]) => any[];
  ocultarBadgeTipo?: boolean;
}

export function AbaDescobrimento(props: AbaDescobrimentoProps) {
  const {
    abaAtiva, cartoes, cartoesSecao, vistas, cartoesVocabulario,
    AoEntrarNoCartao, AoSairDoCartao, AoClicarNoCartao, SalvarPalavra, DeduplicarCartoes, ocultarBadgeTipo
  } = props;

  if (abaAtiva !== 'descobrimento' && abaAtiva !== 'tela_unica' && abaAtiva !== 'vistas') return null;

  return (
    <>
      {abaAtiva === 'descobrimento' && (
        <ListaCartoes 
          cartoesVocabulario={cartoesVocabulario} 
          AoEntrarNoCartao={AoEntrarNoCartao} 
          AoSairDoCartao={AoSairDoCartao} 
          AoClicarNoCartao={AoClicarNoCartao} 
          list={cartoes} 
          defaultStatus='visto' 
          actionBtns={(c: any) => (
            <button className="scan-btn" style={{padding: '4px 8px', fontSize: '11px'}} onClick={() => SalvarPalavra(c.hanzi || c.Hanzi, 'estudo')}>
              + Mover p/ Estudo
            </button>
          )} 
          ocultarBadgeTipo={ocultarBadgeTipo}
        />
      )}

      {abaAtiva === 'tela_unica' && (
        <ListaCartoes 
          cartoesVocabulario={cartoesVocabulario} 
          AoEntrarNoCartao={AoEntrarNoCartao} 
          AoSairDoCartao={AoSairDoCartao} 
          AoClicarNoCartao={AoClicarNoCartao} 
          list={DeduplicarCartoes(cartoesSecao)} 
          defaultStatus='visto' 
          actionBtns={(c: any) => (
            <button className="scan-btn" style={{padding: '4px 8px', fontSize: '11px'}} onClick={() => SalvarPalavra(c.hanzi || c.Hanzi, 'estudo')}>
              + Mover p/ Estudo
            </button>
          )} 
          ocultarBadgeTipo={ocultarBadgeTipo}
        />
      )}

      {abaAtiva === 'vistas' && (
        <ListaCartoes 
          list={vistas} 
          defaultStatus='visto' 
          actionBtns={(c: any) => (
            <button className="scan-btn" style={{padding: '4px 8px', fontSize: '11px'}} onClick={() => SalvarPalavra(c.hanzi || c.Hanzi, 'estudo')}>
              + Mover p/ Estudo
            </button>
          )} 
          cartoesVocabulario={cartoesVocabulario}
          AoEntrarNoCartao={AoEntrarNoCartao}
          AoSairDoCartao={AoSairDoCartao}
          AoClicarNoCartao={AoClicarNoCartao}
          ocultarBadgeTipo={ocultarBadgeTipo}
        />
      )}
    </>
  );
}
