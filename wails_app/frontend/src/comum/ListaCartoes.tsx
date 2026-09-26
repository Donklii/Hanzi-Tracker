// ----- Seção: Comum -----
import React, { useState } from 'react';
import { progresso } from '../../wailsjs/go/models';
import { STATUS_VOCABULARIO } from './status';
import { t } from '../i18n/i18n';
import { CampoSignificadoCard } from './CampoSignificadoCard';
import './ListaCartoes.css';

interface ListaCartoesProps {
  list: any[];
  defaultStatus: string;
  SalvarPalavra: (cartao: any, status: string) => void;
  cartoesVocabulario: progresso.Vocab[];
  AoEntrarNoCartao: (c: any) => void;
  AoSairDoCartao: () => void;
  AoClicarNoCartao: (c: any) => void;
  ocultarBadgeTipo?: boolean;
}

export function ListaCartoes(props: ListaCartoesProps) {
  const {
    list, defaultStatus, SalvarPalavra,
    cartoesVocabulario, AoEntrarNoCartao, AoSairDoCartao, AoClicarNoCartao, ocultarBadgeTipo
  } = props;

  if (list.length === 0) {
    return <div style={{ color: 'var(--cor-texto-suave)', textAlign: 'center', marginTop: '20px' }}>{t('Nenhuma palavra encontrada.')}</div>;
  }

  const statusPorHanzi = new Map(cartoesVocabulario.map(v => [v.Hanzi, v.Status]));

  return (
    <div className="cards-container">
      {list.map((c, i) => {
        const hz = c.hanzi || c.Hanzi;
        const statusDB = statusPorHanzi.get(hz) || defaultStatus;
        return (
          <ItemCartao
            key={i}
            c={c}
            defaultStatus={defaultStatus}
            statusDB={statusDB}
            SalvarPalavra={SalvarPalavra}
            AoEntrarNoCartao={AoEntrarNoCartao}
            AoSairDoCartao={AoSairDoCartao}
            AoClicarNoCartao={AoClicarNoCartao}
            ocultarBadgeTipo={ocultarBadgeTipo}
          />
        );
      })}
    </div>
  );
}

interface ItemCartaoProps {
  c: any;
  defaultStatus: string;
  statusDB: string;
  SalvarPalavra: (cartao: any, status: string) => void;
  AoEntrarNoCartao: (c: any) => void;
  AoSairDoCartao: () => void;
  AoClicarNoCartao: (c: any) => void;
  ocultarBadgeTipo?: boolean;
}

function ItemCartao(props: ItemCartaoProps) {
  const {
    c, defaultStatus, statusDB, SalvarPalavra,
    AoEntrarNoCartao, AoSairDoCartao, AoClicarNoCartao, ocultarBadgeTipo
  } = props;

  const hz = c.hanzi || c.Hanzi;
  const py = c.pinyin || c.Pinyin || '---';
  const sigs = c.significados ? c.significados.join(', ') : c.Significado || t('Sem tradução');

  const [pinyinExibicao, setPinyinExibicao] = useState(py);

  // Destaque para palavras estudadas/aprendidas
  const cardStyle: React.CSSProperties = {};
  let badge = null;

  if (statusDB === STATUS_VOCABULARIO.Estudo) {
    cardStyle.borderColor = '#2196f3';
    cardStyle.backgroundColor = '#1a2733';
    badge = <div style={{position: 'absolute', top: '4px', right: '4px', fontSize: '9px', color: '#2196f3', fontWeight: 'bold'}}>{t('ESTUDO')}</div>;
  } else if (statusDB === STATUS_VOCABULARIO.Aprendido) {
    cardStyle.borderColor = '#4caf50';
    cardStyle.backgroundColor = '#1e2e1e';
    badge = <div style={{position: 'absolute', top: '4px', right: '4px', fontSize: '9px', color: '#4caf50', fontWeight: 'bold'}}>{t('APRENDIDA')}</div>;
  }

  // Estilização para card fantasma (sumiu da tela física)
  let badgeFantasma = null;
  if (c.fantasma) {
    cardStyle.opacity = 0.55;
    cardStyle.borderStyle = 'dashed';
    badgeFantasma = <div style={{position: 'absolute', bottom: '4px', right: '4px', fontSize: '9px', color: '#ff5252', fontWeight: 'bold'}}>{t('FANTASMA')}</div>;
  }

  // Badge do tipo de Hanzi
  let badgeTipoHanzi = null;
  if (!ocultarBadgeTipo) {
    const tipoHanzi = c.tipoHanzi || c.TipoHanzi;
    if (tipoHanzi === "SISTEMA") {
      badgeTipoHanzi = <div style={{position: 'absolute', top: '4px', right: '4px', fontSize: '9px', color: '#ff9800', fontWeight: 'bold'}}>{t('SISTEMA')}</div>;
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

const IconeBaralho = ({ size = 44 }: { size?: number }) => (
  <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round" style={{ width: `${size}px`, height: `${size}px`, color: '#64b5f6' }}>
    <rect x="2" y="6" width="14" height="16" rx="2" ry="2" />
    <path d="M6 2h14a2 2 0 0 1 2 2v14" />
    <path d="M9 12h.01" />
    <path d="M11 16h.01" />
  </svg>
);

  if (c.isDeckCard) {
    return (
      <div
        className="card"
        style={{...cardStyle, position: 'relative'}}
        onMouseEnter={() => AoEntrarNoCartao(c)}
        onMouseLeave={AoSairDoCartao}
        onClick={() => AoClicarNoCartao(c)}
      >
        {badgeTipoHanzi}
        <div className="card-pinyin" style={{ color: '#64b5f6', display: 'flex', alignItems: 'center', justifyContent: 'center', textAlign: 'center' }}>
          {py}
        </div>
        <div className="card-hanzi" style={{ flexGrow: 1, display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
          <IconeBaralho size={44} />
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
      id={`card-${hz}`}
      style={{...cardStyle, position: 'relative'}}
      onMouseEnter={() => AoEntrarNoCartao(c)}
      onMouseLeave={AoSairDoCartao}
      onClick={() => AoClicarNoCartao(c)}
    >
      {badge}
      {badgeTipoHanzi}
      {badgeFantasma}
      <div className="card-pinyin" style={{color: statusDB === STATUS_VOCABULARIO.Estudo ? '#64b5f6' : statusDB === STATUS_VOCABULARIO.Aprendido ? '#81c784' : 'var(--cor-destaque)', display: 'flex', alignItems: 'center', justifyContent: 'center', textAlign: 'center'}}>
        {pinyinExibicao}
      </div>
      <div className="card-hanzi">{hz}</div>
      <CampoSignificadoCard
        hanzi={hz}
        significadosPadrao={sigs}
        aoCarregarPinyins={(novosPinyins) => {
          if (novosPinyins) setPinyinExibicao(novosPinyins);
        }}
      />
      <div className="card-actions" onClick={(e) => e.stopPropagation()}>
        {statusDB === STATUS_VOCABULARIO.Estudo ? (
          <>
            <button className="scan-btn" style={{padding: '4px 8px', fontSize: '11px', backgroundColor: '#4caf50', flex: 1}} onClick={() => SalvarPalavra(c, STATUS_VOCABULARIO.Aprendido)}>
              {t('Aprendi')}
            </button>
            <button className="scan-btn" style={{padding: '4px 8px', fontSize: '11px', backgroundColor: '#f44336', flex: 1}} onClick={() => SalvarPalavra(c, STATUS_VOCABULARIO.Visto)}>
              {t('Remover')}
            </button>
          </>
        ) : statusDB === STATUS_VOCABULARIO.Aprendido ? (
          <button className="scan-btn" style={{padding: '4px 8px', fontSize: '11px', backgroundColor: '#f44336'}} onClick={() => SalvarPalavra(c, STATUS_VOCABULARIO.Estudo)}>
            {t('Reestudar')}
          </button>
        ) : (
          <button className="scan-btn" style={{padding: '4px 8px', fontSize: '11px'}} onClick={() => SalvarPalavra(c, STATUS_VOCABULARIO.Estudo)}>
            {t('+ Mover p/ Estudo')}
          </button>
        )}
      </div>
    </div>
  );
}
