// ----- Seção: Comum -----
import { progresso } from '../../wailsjs/go/models';

interface ListaCartoesProps {
  list: any[];
  defaultStatus: string;
  actionBtns: (c: any) => JSX.Element;
  cartoesVocabulario: progresso.Vocab[];
  AoEntrarNoCartao: (c: any) => void;
  AoSairDoCartao: () => void;
  AoClicarNoCartao: (c: any) => void;
}

export function ListaCartoes(props: ListaCartoesProps) {
  const {
    list, defaultStatus, actionBtns,
    cartoesVocabulario, AoEntrarNoCartao, AoSairDoCartao, AoClicarNoCartao
  } = props;

  if (list.length === 0) {
    return <div style={{ color: 'var(--cor-texto-suave)', textAlign: 'center', marginTop: '20px' }}>Nenhuma palavra encontrada.</div>;
  }
  
  // As tags não interferem no sort, mantém a ordem original da lista
  const sortedList = list;

  return (
    <div className="cards-container">
      {sortedList.map((c, i) => {
        const hz = c.hanzi || c.Hanzi;
        const py = c.pinyin || c.Pinyin || '---';
        const sigs = c.significados ? c.significados.join(', ') : c.Significado || 'Sem tradução';
        
        const statusDB = cartoesVocabulario.find(v => v.Hanzi === hz)?.Status || defaultStatus;
        
        // Destaque para palavras estudadas/aprendidas
        const cardStyle: React.CSSProperties = {};
        let badge = null;
        
        if (statusDB === 'estudo') {
          cardStyle.borderColor = '#2196f3';
          cardStyle.backgroundColor = '#1a2733';
          badge = <div style={{position: 'absolute', top: '4px', right: '4px', fontSize: '9px', color: '#2196f3', fontWeight: 'bold'}}>ESTUDO</div>;
        } else if (statusDB === 'aprendido') {
          cardStyle.borderColor = '#4caf50';
          cardStyle.backgroundColor = '#1e2e1e';
          badge = <div style={{position: 'absolute', top: '4px', right: '4px', fontSize: '9px', color: '#4caf50', fontWeight: 'bold'}}>APRENDIDA</div>;
        }

        return (
          <div 
            className="card" 
            key={i}
            style={{...cardStyle, position: 'relative'}}
            onMouseEnter={() => AoEntrarNoCartao(c)}
            onMouseLeave={AoSairDoCartao}
            onClick={() => AoClicarNoCartao(c)}
          >
            {badge}
            <div className="card-pinyin" style={{color: statusDB === 'estudo' ? '#64b5f6' : statusDB === 'aprendido' ? '#81c784' : 'var(--cor-destaque)'}}>{py}</div>
            <div className="card-hanzi">{hz}</div>
            <div className="card-sigs">
              {sigs}
            </div>
            <div className="card-actions" onClick={(e) => e.stopPropagation()}>
              {actionBtns(c)}
            </div>
          </div>
        )
      })}
    </div>
  );
}
