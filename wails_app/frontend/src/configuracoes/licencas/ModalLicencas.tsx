// ----- Seção: Configurações — licenças de terceiros (modal) -----
//
// Duas telas no mesmo modal: a LISTA das origens (com a atribuição que cada licença exige) e o
// LEITOR do texto integral de uma licença. O leitor existe porque não basta citar a licença: o
// Make Me a Hanzi manda entregar o arquivo "LGPL" junto dos dados, e a Arphic PL §1 exige reter o
// ARPHICPL.TXT inalterado em toda cópia — o texto tem que viajar dentro do app, sem depender de
// internet. Ver atribuicoes.ts.
import { useState } from 'react';
import { BrowserOpenURL } from '../../../wailsjs/runtime/runtime';
import { t } from '../../i18n/i18n';
import { ATRIBUICOES, AVISO_COMPARTILHA_IGUAL, AVISO_SEM_GARANTIA, TextoLicenca } from './atribuicoes';

interface ModalLicencasProps {
  aberto: boolean;
  aoFechar: () => void;
}

export function ModalLicencas({ aberto, aoFechar }: ModalLicencasProps) {
  const [licencaAberta, setLicencaAberta] = useState<TextoLicenca | null>(null);

  if (!aberto) return null;

  const fechar = () => {
    setLicencaAberta(null);
    aoFechar();
  };

  return (
    <div className="modal-overlay" onClick={fechar} style={{ zIndex: 3000 }}>
      <div
        className="modal-content"
        style={{ maxWidth: '760px', width: '90vw', height: '80vh', padding: '24px', flexDirection: 'column' }}
        onClick={e => e.stopPropagation()}
      >
        <div className="modal-header">
          <h2 style={{ fontSize: '18px' }}>
            {licencaAberta ? licencaAberta.rotulo : t('Licenças de terceiros')}
          </h2>
          <button className="modal-close" onClick={fechar}>×</button>
        </div>

        {licencaAberta
          ? <LeitorLicenca licenca={licencaAberta} aoVoltar={() => setLicencaAberta(null)} />
          : <ListaAtribuicoes aoAbrirLicenca={setLicencaAberta} />}
      </div>
    </div>
  );
}

// ListaAtribuicoes mostra, por origem, tudo que a licença dela manda exibir.
function ListaAtribuicoes({ aoAbrirLicenca }: { aoAbrirLicenca: (licenca: TextoLicenca) => void }) {
  return (
    <div style={{ overflowY: 'auto', flex: 1, marginTop: '8px', paddingRight: '8px' }}>
      <p style={{ color: 'var(--cor-texto-suave)', fontSize: '13px', lineHeight: 1.5, marginBottom: '8px' }}>
        {t('O Hanzi Tracker usa dados de terceiros. Abaixo estão as atribuições e os avisos que os autores exigem, e o texto completo de cada licença.')}
      </p>
      <p style={{ color: 'var(--cor-texto-suave)', fontSize: '13px', lineHeight: 1.5, marginBottom: '8px' }}>
        {t(AVISO_COMPARTILHA_IGUAL)}
      </p>
      <p style={{ color: 'var(--cor-texto-suave)', fontSize: '13px', lineHeight: 1.5, marginBottom: '20px' }}>
        {t(AVISO_SEM_GARANTIA)}
      </p>

      {ATRIBUICOES.map(atribuicao => (
        <div
          key={atribuicao.nome}
          style={{
            borderTop: '1px solid var(--cor-borda, rgba(128,128,128,0.3))',
            paddingTop: '16px',
            marginBottom: '16px',
          }}
        >
          <h4 style={{ marginBottom: '6px' }}>{atribuicao.nome}</h4>
          <p style={{ color: 'var(--cor-texto-suave)', fontSize: '13px', lineHeight: 1.5, marginBottom: '6px' }}>
            {t(atribuicao.papel)}
          </p>

          <CampoAtribuicao rotulo={t('Direitos autorais')} valor={atribuicao.copyright} />
          <CampoAtribuicao rotulo={t('Derivado de')} valor={t(atribuicao.derivacao)} />
          <CampoAtribuicao rotulo={t('Licença')} valor={atribuicao.licenca} />
          <CampoAtribuicao rotulo={t('Modificações')} valor={t(atribuicao.modificacoes)} />

          <div style={{ display: 'flex', flexWrap: 'wrap', gap: '8px', marginTop: '10px' }}>
            <button
              className="scan-btn"
              onClick={() => BrowserOpenURL(atribuicao.origem)}
              style={{ padding: '5px 12px', fontSize: '12px' }}
            >
              {t('Ver origem')}
            </button>
            {atribuicao.textos.map(licenca => (
              <button
                key={licenca.rotulo}
                className="scan-btn"
                onClick={() => aoAbrirLicenca(licenca)}
                style={{ padding: '5px 12px', fontSize: '12px' }}
              >
                {licenca.rotulo}
              </button>
            ))}
          </div>
        </div>
      ))}
    </div>
  );
}

// LeitorLicenca mostra o texto legal integral, sem tradução e sem reflow — é o documento, não a
// nossa descrição dele.
function LeitorLicenca({ licenca, aoVoltar }: { licenca: TextoLicenca; aoVoltar: () => void }) {
  return (
    <>
      <button
        className="scan-btn"
        onClick={aoVoltar}
        style={{ alignSelf: 'flex-start', padding: '5px 12px', fontSize: '12px', marginTop: '8px' }}
      >
        {t('← Voltar')}
      </button>
      <pre
        style={{
          overflow: 'auto',
          flex: 1,
          marginTop: '12px',
          padding: '12px',
          fontSize: '11px',
          lineHeight: 1.45,
          whiteSpace: 'pre-wrap',
          wordBreak: 'break-word',
          color: 'var(--cor-texto-suave)',
          background: 'rgba(128,128,128,0.08)',
          borderRadius: '6px',
        }}
      >
        {licenca.texto}
      </pre>
    </>
  );
}

// ----- Apoio -----

function CampoAtribuicao({ rotulo, valor }: { rotulo: string; valor: string }) {
  return (
    <p style={{ color: 'var(--cor-texto-suave)', fontSize: '12px', lineHeight: 1.5, margin: '3px 0' }}>
      <strong>{rotulo}:</strong> {valor}
    </p>
  );
}
