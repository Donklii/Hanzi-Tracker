// ----- Seção: Comum -----
import React, { useState, useRef } from 'react';
import { progresso } from '../../wailsjs/go/models';
import { TocarAudio } from './TocarAudio';

function BotaoAudioPinyin({ hanzi, motorTtsAtivo }: { hanzi: string, motorTtsAtivo: string }) {
  const [tocando, setTocando] = useState(false);

  const tocar = async (e: React.MouseEvent) => {
    e.stopPropagation();
    if (tocando) return;
    
    setTocando(true);
    await TocarAudio(hanzi, motorTtsAtivo);
    setTocando(false);
  };

  return (
    <button
      onClick={tocar}
      style={{
        background: 'none', border: 'none', cursor: 'pointer', fontSize: '14px', 
        opacity: tocando ? 0.6 : 0.8, padding: '0 4px', marginRight: '4px',
        display: 'inline-flex', alignItems: 'center', justifyContent: 'center'
      }}
      title="Ouvir Pronúncia"
      onMouseOver={e => e.currentTarget.style.opacity = '1'}
      onMouseOut={e => e.currentTarget.style.opacity = '0.8'}
    >
      🔊
    </button>
  );
}

interface ListaCartoesProps {
  list: any[];
  defaultStatus: string;
  actionBtns: (c: any) => JSX.Element;
  cartoesVocabulario: progresso.Vocab[];
  AoEntrarNoCartao: (c: any) => void;
  AoSairDoCartao: () => void;
  AoClicarNoCartao: (c: any) => void;
  ocultarBadgeTipo?: boolean;
  motorTtsAtivo?: string;
}

export function ListaCartoes(props: ListaCartoesProps) {
  const {
    list, defaultStatus, actionBtns,
    cartoesVocabulario, AoEntrarNoCartao, AoSairDoCartao, AoClicarNoCartao, ocultarBadgeTipo, motorTtsAtivo
  } = props;

  if (list.length === 0) {
    return <div style={{ color: 'var(--cor-texto-suave)', textAlign: 'center', marginTop: '20px' }}>Nenhuma palavra encontrada.</div>;
  }
  
  const statusPorHanzi = new Map(cartoesVocabulario.map(v => [v.Hanzi, v.Status]));

  // As tags não interferem no sort, mantém a ordem original da lista
  const sortedList = list;

  return (
    <div className="cards-container">
      {sortedList.map((c, i) => {
        const hz = c.hanzi || c.Hanzi;
        const py = c.pinyin || c.Pinyin || '---';
        const sigs = c.significados ? c.significados.join(', ') : c.Significado || 'Sem tradução';
        
        const statusDB = statusPorHanzi.get(hz) || defaultStatus;
        
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

        // Badge do tipo de Hanzi
        let badgeTipoHanzi = null;
        if (!ocultarBadgeTipo) {
          const tipoHanzi = c.tipoHanzi || c.TipoHanzi;
          if (tipoHanzi === "Simplificado" || tipoHanzi === "Ambos") {
            badgeTipoHanzi = <div style={{position: 'absolute', top: '4px', left: '4px', fontSize: '9px', color: '#ffb74d', fontWeight: 'bold'}}>汉字</div>;
          } else if (tipoHanzi === "Tradicional") {
            badgeTipoHanzi = <div style={{position: 'absolute', top: '4px', left: '4px', fontSize: '9px', color: '#f44336', fontWeight: 'bold'}}>漢字</div>;
          }
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
            {badgeTipoHanzi}
            <div className="card-pinyin" style={{color: statusDB === 'estudo' ? '#64b5f6' : statusDB === 'aprendido' ? '#81c784' : 'var(--cor-destaque)', display: 'flex', alignItems: 'center', justifyContent: 'center'}}>
              <BotaoAudioPinyin hanzi={hz} motorTtsAtivo={motorTtsAtivo || ''} />
              {py}
            </div>
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
