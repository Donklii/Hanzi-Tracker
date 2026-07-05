// ----- Seção: Estudos -----
import { progresso } from '../../wailsjs/go/models';
import { ListaCartoes } from '../comum/ListaCartoes';

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
}

export function AbaEstudos(props: AbaEstudosProps) {
  const {
    abaAtiva, estudando, aprendidas, cartoesVocabulario,
    AoEntrarNoCartao, AoSairDoCartao, AoClicarNoCartao, SalvarPalavra, ocultarBadgeTipo
  } = props;

  if (abaAtiva !== 'estudando' && abaAtiva !== 'aprendidas') return null;

  return (
    <>
      {abaAtiva === 'estudando' && (
        <ListaCartoes 
          cartoesVocabulario={cartoesVocabulario} 
          AoEntrarNoCartao={AoEntrarNoCartao} 
          AoSairDoCartao={AoSairDoCartao} 
          AoClicarNoCartao={AoClicarNoCartao} 
          list={estudando} 
          defaultStatus='estudo' 
          ocultarBadgeTipo={ocultarBadgeTipo}
          actionBtns={(c) => (
            <>
              <button className="scan-btn" style={{padding: '4px 8px', fontSize: '11px', backgroundColor: '#4caf50', flex: 1}} onClick={() => SalvarPalavra(c, 'aprendido')}>
                Aprendi
              </button>
              <button className="scan-btn" style={{padding: '4px 8px', fontSize: '11px', backgroundColor: '#f44336', flex: 1}} onClick={() => SalvarPalavra(c, 'visto')}>
                Remover
              </button>
            </>
          )} 
        />
      )}

      {abaAtiva === 'aprendidas' && (
        <ListaCartoes 
          cartoesVocabulario={cartoesVocabulario} 
          AoEntrarNoCartao={AoEntrarNoCartao} 
          AoSairDoCartao={AoSairDoCartao} 
          AoClicarNoCartao={AoClicarNoCartao} 
          list={aprendidas} 
          defaultStatus='aprendido' 
          ocultarBadgeTipo={ocultarBadgeTipo}
          actionBtns={(c) => (
            <button className="scan-btn" style={{padding: '4px 8px', fontSize: '11px', backgroundColor: '#f44336'}} onClick={() => SalvarPalavra(c, 'estudo')}>
              Reestudar
            </button>
          )} 
        />
      )}
    </>
  );
}
