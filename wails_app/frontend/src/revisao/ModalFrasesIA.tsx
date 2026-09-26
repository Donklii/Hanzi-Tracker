// ----- Seção: Modal de Frases Geradas por IA -----
import { useState, useEffect } from 'react';
import { t } from '../i18n/i18n';
import { ObterFrasesIA, GerarFrasesComIA, DescartarFraseIA } from '../../wailsjs/go/main/App';
import { progresso } from '../../wailsjs/go/models';
import { rotuloTema, rotuloDificuldade } from './taxonomiaFrases';

interface ModalFrasesIAProps {
  aberto: boolean;
  aoFechar: () => void;
  configuracoesApp?: any;
  AtualizarConfiguracao?: (chave: any, valor: any) => void;
}

export function ModalFrasesIA({ aberto, aoFechar, configuracoesApp, AtualizarConfiguracao }: ModalFrasesIAProps) {
  const [listaFrases, setListaFrases] = useState<progresso.FraseUsuario[]>([]);
  const [novasFrases, setNovasFrases] = useState<Set<string>>(new Set());
  const [carregando, setCarregando] = useState(false);
  const [gerando, setGerando] = useState(false);
  const [statusMensagem, setStatusMensagem] = useState('');
  const [vibeIa, setVibeIa] = useState(configuracoesApp?.revisaoIaVibe || '');

  useEffect(() => {
    setVibeIa(configuracoesApp?.revisaoIaVibe || '');
  }, [configuracoesApp?.revisaoIaVibe]);

  useEffect(() => {
    if (!aberto) return;
    carregarFrasesBanco();
  }, [aberto]);

  function carregarFrasesBanco() {
    setCarregando(true);
    ObterFrasesIA()
      .then((itens) => {
        setListaFrases(itens || []);
      })
      .catch((err) => {
        setStatusMensagem(t('⚠️ Erro ao carregar frases: {erro}', { erro: String(err) }));
      })
      .finally(() => {
        setCarregando(false);
      });
  }

  function persistirVibe() {
    if (AtualizarConfiguracao && vibeIa !== (configuracoesApp?.revisaoIaVibe || '')) {
      AtualizarConfiguracao('revisaoIaVibe', vibeIa);
    }
  }

  function aoGerarFrases() {
    if (gerando) return;
    setGerando(true);
    setStatusMensagem('');

    const chinesesAntigos = new Set(listaFrases.map(f => f.Chines));

    GerarFrasesComIA(10)
      .then((qtdIneditas) => {
        return ObterFrasesIA().then((novasDaBase) => {
          const listaAtualizada = novasDaBase || [];
          setListaFrases(listaAtualizada);

          const conjNovas = new Set<string>();
          for (const f of listaAtualizada) {
            if (!chinesesAntigos.has(f.Chines)) {
              conjNovas.add(f.Chines);
            }
          }
          setNovasFrases(conjNovas);
          setStatusMensagem(t('✨ {qtd} nova(s) frase(s) gerada(s) com sucesso!', { qtd: qtdIneditas }));
        });
      })
      .catch((err) => {
        setStatusMensagem(t('⚠️ Erro ao gerar frases: {erro}', { erro: String(err) }));
      })
      .finally(() => {
        setGerando(false);
      });
  }

  function aoDescartarFrase(chines: string) {
    if (!chines) return;
    DescartarFraseIA(chines)
      .then(() => {
        setListaFrases(prev => prev.filter(f => f.Chines !== chines));
        setNovasFrases(prev => {
          const copia = new Set(prev);
          copia.delete(chines);
          return copia;
        });
      })
      .catch((err) => {
        setStatusMensagem(t('⚠️ Erro ao descartar frase: {erro}', { erro: String(err) }));
      });
  }

  if (!aberto) return null;

  const geminiConfigurado = !!configuracoesApp?.geminiApiKey;

  const frasesOrdenadas = [...listaFrases].sort((a, b) => {
    const ehNovaA = novasFrases.has(a.Chines) ? 1 : 0;
    const ehNovaB = novasFrases.has(b.Chines) ? 1 : 0;
    return ehNovaB - ehNovaA;
  });

  return (
    <div
      className="modal-overlay"
      onClick={aoFechar}
      style={{
        position: 'fixed',
        top: 0,
        left: 0,
        right: 0,
        bottom: 0,
        width: '100vw',
        height: '100vh',
        backgroundColor: 'rgba(0, 0, 0, 0.65)',
        backdropFilter: 'blur(4px)',
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        zIndex: 2000,
      }}
    >
      <div
        className="modal-content revisao-modal-frases-ia"
        onClick={e => e.stopPropagation()}
      >
        <div className="modal-header">
          <h2>✨ {t('Frases Geradas por IA')}</h2>
          <button className="modal-close" onClick={aoFechar}>×</button>
        </div>

        <div className="revisao-modal-controles">
          <div className="revisao-config-campo" style={{ flex: 1 }}>
            <span>{t('Clima / Vibe das Frases')}</span>
            <input
              type="text"
              className="revisao-config-input"
              placeholder={t('Ex.: poesia, cotidiano, mistério, formal...')}
              value={vibeIa}
              onChange={e => setVibeIa(e.target.value)}
              onBlur={persistirVibe}
            />
          </div>

          <button
            type="button"
            className="revisao-ia-gerar"
            style={{ alignSelf: 'flex-end', height: '38px', padding: '0 16px' }}
            disabled={!geminiConfigurado || gerando}
            onClick={aoGerarFrases}
          >
            {gerando ? (
              <>
                <span className="revisao-spinner" />
                {t('Gerando...')}
              </>
            ) : (
              t('✨ Gerar Novas Frases')
            )}
          </button>
        </div>

        {!geminiConfigurado && (
          <div className="revisao-config-aviso" style={{ marginTop: '8px' }}>
            {t('⚠️ Configure a chave de API do Gemini na aba Motores das configurações para gerar frases.')}
          </div>
        )}

        {statusMensagem && (
          <div className="revisao-config-status" style={{ marginTop: '8px' }}>
            {statusMensagem}
          </div>
        )}

        <div className="revisao-modal-tabela-container">
          {carregando ? (
            <div style={{ textAlign: 'center', padding: '32px', color: 'var(--cor-texto-suave)' }}>
              <span className="revisao-spinner" /> {t('Carregando frases...')}
            </div>
          ) : frasesOrdenadas.length === 0 ? (
            <div style={{ textAlign: 'center', padding: '32px', color: 'var(--cor-texto-suave)' }}>
              {t('Nenhuma frase gerada por IA encontrada no banco de dados.')}
            </div>
          ) : (
            <table className="revisao-modal-tabela">
              <thead>
                <tr>
                  <th>{t('Chinês')}</th>
                  <th>{t('Tradução')}</th>
                  <th>{t('Tema')}</th>
                  <th>{t('Dificuldade')}</th>
                  <th>{t('Atribuição')}</th>
                  <th style={{ textAlign: 'center' }}>{t('Ações')}</th>
                </tr>
              </thead>
              <tbody>
                {frasesOrdenadas.map((f, i) => {
                  const ehNova = novasFrases.has(f.Chines);
                  return (
                    <tr key={f.Chines + i} className={ehNova ? 'linha-nova' : ''}>
                      <td className="coluna-chines">
                        <span className="texto-chines">{f.Chines}</span>
                        {ehNova && <span className="revisao-badge-nova">{t('Nova')}</span>}
                      </td>
                      <td className="coluna-traducao">{f.Ingles}</td>
                      <td>{rotuloTema(f.Tema) || f.Tema || '-'}</td>
                      <td>{rotuloDificuldade(f.Dificuldade) || f.Dificuldade || '-'}</td>
                      <td className="coluna-atribuicao">{f.Atribuicao || '-'}</td>
                      <td style={{ textAlign: 'center' }}>
                        <button
                          type="button"
                          className="revisao-btn-descartar"
                          title={t('Descartar esta frase')}
                          onClick={() => aoDescartarFrase(f.Chines)}
                        >
                          🗑️ {t('Descartar')}
                        </button>
                      </td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          )}
        </div>
      </div>
    </div>
  );
}
