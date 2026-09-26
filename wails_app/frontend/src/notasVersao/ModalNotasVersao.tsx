// ----- Seção: Notas de Versão -----
import { notasversao } from '../../wailsjs/go/models';
import { t, idiomaAtual } from '../i18n/i18n';

interface ModalNotasVersaoProps {
  aberto: boolean;
  titulo: string;
  notas: notasversao.Nota[];
  aoFechar: () => void;
}

export function ModalNotasVersao({ aberto, titulo, notas, aoFechar }: ModalNotasVersaoProps) {
  if (!aberto || !notas || notas.length === 0) return null;

  return (
    <div className="modal-overlay" onClick={aoFechar} style={{ zIndex: 3000 }}>
      <div
        className="modal-content"
        style={{
          maxWidth: '560px',
          width: '90%',
          maxHeight: '85vh',
          display: 'flex',
          flexDirection: 'column',
          padding: '24px',
          height: 'auto',
        }}
        onClick={(e) => e.stopPropagation()}
      >
        <div className="modal-header">
          <h2 style={{ fontSize: '18px', margin: 0, color: 'var(--cor-texto-primario)' }}>
            {titulo}
          </h2>
          <button className="modal-close" onClick={aoFechar}>
            ×
          </button>
        </div>

        <div
          style={{
            flex: 1,
            overflowY: 'auto',
            paddingRight: '6px',
            marginTop: '16px',
            display: 'flex',
            flexDirection: 'column',
            gap: '24px',
          }}
        >
          {notas.map((nota) => (
            <div key={nota.id} style={{ display: 'flex', flexDirection: 'column', gap: '12px' }}>
              <div style={{ borderBottom: '1px solid var(--cor-borda)', paddingBottom: '8px' }}>
                <h3
                  style={{
                    fontSize: '16px',
                    fontWeight: 600,
                    margin: '0 0 4px 0',
                    color: 'var(--cor-texto-primario)',
                  }}
                >
                  {nota.titulo}
                </h3>
                <div style={{ fontSize: '12px', color: 'var(--cor-texto-secundario)' }}>
                  {formatarDataNota(nota.data)}
                </div>
              </div>

              <div style={{ display: 'flex', flexDirection: 'column', gap: '14px' }}>
                {nota.grupos?.map((grupo, indiceGrupo) => (
                  <div key={indiceGrupo} style={{ display: 'flex', flexDirection: 'column', gap: '6px' }}>
                    {grupo.titulo ? (
                      <h4
                        style={{
                          fontSize: '14px',
                          fontWeight: 600,
                          margin: 0,
                          color: 'var(--cor-texto-primario)',
                        }}
                      >
                        {grupo.titulo}
                      </h4>
                    ) : null}

                    <ul
                      style={{
                        margin: 0,
                        paddingLeft: '20px',
                        display: 'flex',
                        flexDirection: 'column',
                        gap: '4px',
                      }}
                    >
                      {grupo.itens?.map((item, indiceItem) => (
                        <li
                          key={indiceItem}
                          style={{
                            fontSize: '13px',
                            lineHeight: 1.5,
                            color: 'var(--cor-texto-primario)',
                          }}
                        >
                          {item}
                        </li>
                      ))}
                    </ul>
                  </div>
                ))}
              </div>
            </div>
          ))}
        </div>

        <button
          className="scan-btn"
          style={{
            marginTop: '20px',
            alignSelf: 'flex-end',
            padding: '8px 20px',
            cursor: 'pointer',
          }}
          onClick={aoFechar}
        >
          {t('Entendi')}
        </button>
      </div>
    </div>
  );
}


function formatarDataNota(dataIso: string): string {
  if (!dataIso) return '';

  const partes = dataIso.split('-');
  if (partes.length !== 3) return dataIso;

  const ano = parseInt(partes[0], 10);
  const mes = parseInt(partes[1], 10) - 1;
  const dia = parseInt(partes[2], 10);

  if (isNaN(ano) || isNaN(mes) || isNaN(dia)) return dataIso;

  const objetoData = new Date(ano, mes, dia);
  return objetoData.toLocaleDateString(idiomaAtual(), {
    year: 'numeric',
    month: 'long',
    day: 'numeric',
  });
}
