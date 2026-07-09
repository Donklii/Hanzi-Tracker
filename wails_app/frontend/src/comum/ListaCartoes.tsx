// ----- Seção: Comum -----
import React from 'react';
import { progresso } from '../../wailsjs/go/models';
import { STATUS_VOCABULARIO } from './status';
import './ListaCartoes.css';

interface ListaCartoesProps {
  list: any[];
  defaultStatus: string;
  actionBtns: (c: any) => JSX.Element;
  cartoesVocabulario: progresso.Vocab[];
  AoEntrarNoCartao: (c: any) => void;
  AoSairDoCartao: () => void;
  AoClicarNoCartao: (c: any) => void;
  ocultarBadgeTipo?: boolean;
}

export function ListaCartoes(props: ListaCartoesProps) {
  const {
    list, defaultStatus, actionBtns,
    cartoesVocabulario, AoEntrarNoCartao, AoSairDoCartao, AoClicarNoCartao, ocultarBadgeTipo
  } = props;

  if (list.length === 0) {
    return <div style={{ color: 'var(--cor-texto-suave)', textAlign: 'center', marginTop: '20px' }}>Nenhuma palavra encontrada.</div>;
  }

  const statusPorHanzi = new Map(cartoesVocabulario.map(v => [v.Hanzi, v.Status]));

  return (
    <div className="cards-container">
      {list.map((c, i) => {
        const hz = c.hanzi || c.Hanzi;
        const py = c.pinyin || c.Pinyin || '---';
        const sigs = c.significados ? c.significados.join(', ') : c.Significado || 'Sem tradução';
        
        const statusDB = statusPorHanzi.get(hz) || defaultStatus;
        
        // Destaque para palavras estudadas/aprendidas
        const cardStyle: React.CSSProperties = {};
        let badge = null;
        
        if (statusDB === STATUS_VOCABULARIO.Estudo) {
          cardStyle.borderColor = '#2196f3';
          cardStyle.backgroundColor = '#1a2733';
          badge = <div style={{position: 'absolute', top: '4px', right: '4px', fontSize: '9px', color: '#2196f3', fontWeight: 'bold'}}>ESTUDO</div>;
        } else if (statusDB === STATUS_VOCABULARIO.Aprendido) {
          cardStyle.borderColor = '#4caf50';
          cardStyle.backgroundColor = '#1e2e1e';
          badge = <div style={{position: 'absolute', top: '4px', right: '4px', fontSize: '9px', color: '#4caf50', fontWeight: 'bold'}}>APRENDIDA</div>;
        }

        // Badge do tipo de Hanzi
        let badgeTipoHanzi = null;
        if (!ocultarBadgeTipo) {
          const tipoHanzi = c.tipoHanzi || c.TipoHanzi;
          if (tipoHanzi === "SISTEMA") {
            badgeTipoHanzi = <div style={{position: 'absolute', top: '4px', right: '4px', fontSize: '9px', color: '#ff9800', fontWeight: 'bold'}}>SISTEMA</div>;
          } else if (tipoHanzi === "Simplificado" || tipoHanzi === "Ambos") {
            badgeTipoHanzi = <div style={{position: 'absolute', top: '4px', left: '4px', fontSize: '9px', color: '#ffb74d', fontWeight: 'bold'}}>汉字</div>;
          } else if (tipoHanzi === "Tradicional") {
            badgeTipoHanzi = <div style={{position: 'absolute', top: '4px', left: '4px', fontSize: '9px', color: '#f44336', fontWeight: 'bold'}}>漢字</div>;
          }
        }

        if (c.isScreenshotCard) {
          return (
            <div
              className="card"
              key={i}
              style={{...cardStyle, position: 'relative'}}
              onMouseEnter={() => AoEntrarNoCartao(c)}
              onMouseLeave={AoSairDoCartao}
              onClick={() => AoClicarNoCartao(c)}
            >
              {badgeTipoHanzi}
              <div className="card-pinyin" style={{ color: 'var(--cor-destaque)', display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
                {py}
              </div>
              <div className="card-hanzi" style={{ flexGrow: 1, display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
                <svg width="56" height="56" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round" style={{ color: 'var(--cor-texto-primario)' }}>
                  <path d="M23 19a2 2 0 0 1-2 2H3a2 2 0 0 1-2-2V8a2 2 0 0 1 2-2h4l2-3h6l2 3h4a2 2 0 0 1 2 2z"></path>
                  <circle cx="12" cy="13" r="4"></circle>
                </svg>
              </div>
              <div className="card-sigs">
                {sigs}
              </div>
            </div>
          );
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
            <div className="card-pinyin" style={{color: statusDB === STATUS_VOCABULARIO.Estudo ? '#64b5f6' : statusDB === STATUS_VOCABULARIO.Aprendido ? '#81c784' : 'var(--cor-destaque)', display: 'flex', alignItems: 'center', justifyContent: 'center'}}>
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
