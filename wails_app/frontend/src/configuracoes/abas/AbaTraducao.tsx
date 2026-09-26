// ----- Seção: Configurações — aba Tradução (Google Cloud Translation + Gemini) -----
import { config, main } from '../../../wailsjs/go/models';
import { SecaoDependente } from '../comum';
import { t } from '../../i18n/i18n';

interface AbaTraducaoProps {
  termoBusca: string;
  configuracoesApp: config.Config;
  AtualizarConfiguracao: (key: keyof config.Config, value: any) => void;
  AplicarConfiguracao: (mudancas: Partial<config.Config>) => void;
  infoCotaTraducao: main.InfoCotaTraducao | null;
  infoCotaGemini: main.InfoCotaGemini | null;
}

export function AbaTraducao({ termoBusca, configuracoesApp, AtualizarConfiguracao, AplicarConfiguracao, infoCotaTraducao, infoCotaGemini }: AbaTraducaoProps) {
  return (
    <>
      {termoBusca && <h3 className="settings-section-title" style={{ marginTop: '32px' }}>{t('Tradução (IA)')}</h3>}

      {(!termoBusca || "tradução api key google cloud".includes(termoBusca.toLowerCase())) && (
        <div className="form-group">
          <label>{t('Google Cloud Translation API Key')}</label>
          <input
            type="password"
            className="form-input"
            value={configuracoesApp.traducaoApiKey}
            onChange={e => AtualizarConfiguracao('traducaoApiKey', e.target.value)}
            placeholder={t('Cole sua API Key aqui...')}
          />
          <small style={{ color: 'var(--cor-texto-suave)', display: 'block', marginTop: '6px' }}>
            {t('Requer uma API key própria do Google Cloud Platform (GCP). Cota gratuita: 500.000 caracteres/mês.')}{' '}
            <strong>{t('Aviso:')}</strong> {t('o Google exige cartão cadastrado no GCP mesmo para usar apenas a cota gratuita.')}
          </small>
        </div>
      )}

      {(!termoBusca || "habilitar tradução de linha cota mensal uso limite guardar cache pausar".includes(termoBusca.toLowerCase())) && (
        <>
          <div className="form-group" style={{ marginTop: '16px' }}>
            <label style={{ display: 'flex', alignItems: 'center', gap: '8px', justifyContent: 'space-between' }}>
              <span>{t('Habilitar tradução por linha (Atalho de Pop-up de Tudo)')}</span>
              <input
                type="checkbox"
                checked={configuracoesApp.traducaoAtiva}
                onChange={e => e.target.checked
                  ? AplicarConfiguracao({ traducaoAtiva: true, geminiAtivo: false })
                  : AplicarConfiguracao({ traducaoAtiva: false })
                }
              />
            </label>
          </div>

          <SecaoDependente ativa={configuracoesApp.traducaoAtiva}>
            {(!termoBusca || "pausar traduções limite cota mensal".includes(termoBusca.toLowerCase())) && (
              <div className="form-group">
                <label style={{ display: 'flex', alignItems: 'center', gap: '8px', justifyContent: 'space-between' }}>
                  <span>{t('Pausar traduções ao atingir limite da cota gratuita mensal')}</span>
                  <input
                    type="checkbox"
                    checked={configuracoesApp.traducaoPausarPorCota}
                    onChange={e => AtualizarConfiguracao('traducaoPausarPorCota', e.target.checked)}
                  />
                </label>
              </div>
            )}

            <SecaoDependente ativa={configuracoesApp.traducaoPausarPorCota}>
              {(!termoBusca || "limite cota mensal percentual".includes(termoBusca.toLowerCase())) && (
                <div className="form-group" style={{ display: 'flex', alignItems: 'center', gap: '16px', margin: 0 }}>
                  <label style={{ margin: 0, flex: 1 }}>{t('Limite de Cota Mensal')}</label>
                  <input
                    type="range"
                    min="10" max="100" step="5"
                    value={configuracoesApp.traducaoLimiteCotaPercent}
                    onChange={e => AtualizarConfiguracao('traducaoLimiteCotaPercent', parseFloat(e.target.value))}
                    style={{ width: '200px', margin: 0 }}
                  />
                  <span style={{ minWidth: '40px', textAlign: 'right', color: 'var(--cor-destaque)', fontWeight: 'bold' }}>{configuracoesApp.traducaoLimiteCotaPercent}%</span>
                </div>
              )}
            </SecaoDependente>

            {(!termoBusca || "guardar cache traduções feitas".includes(termoBusca.toLowerCase())) && (
              <div className="form-group" style={{ marginTop: '16px' }}>
                <label style={{ display: 'flex', alignItems: 'center', gap: '8px', justifyContent: 'space-between' }}>
                  <span>{t('Guardar traduções já feitas para não gastar cota na mesma linha')}</span>
                  <input
                    type="checkbox"
                    checked={configuracoesApp.traducaoUsarCache}
                    onChange={e => AtualizarConfiguracao('traducaoUsarCache', e.target.checked)}
                  />
                </label>
              </div>
            )}

            {infoCotaTraducao && (!termoBusca || "uso da cota".includes(termoBusca.toLowerCase())) && (
              <div className="form-group" style={{ marginTop: '16px', padding: '12px', backgroundColor: 'var(--cor-fundo-cartao)', borderRadius: '8px', border: '1px solid var(--cor-borda)', marginBottom: 0 }}>
                <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: '8px' }}>
                  <span>{t('Uso da Cota (Mês {mes})', { mes: infoCotaTraducao.anoMes })}</span>
                  <strong>{infoCotaTraducao.percentual.toFixed(1)}%</strong>
                </div>
                <div style={{ width: '100%', height: '8px', backgroundColor: 'var(--cor-borda)', borderRadius: '4px', overflow: 'hidden' }}>
                  <div style={{
                    height: '100%',
                    width: `${Math.min(100, infoCotaTraducao.percentual)}%`,
                    backgroundColor: infoCotaTraducao.percentual >= 90 ? '#f44336' : (infoCotaTraducao.percentual >= 75 ? '#ffb74d' : 'var(--cor-destaque)')
                  }} />
                </div>
                <div style={{ fontSize: '11px', color: 'var(--cor-texto-suave)', marginTop: '8px', textAlign: 'right' }}>
                  {t('{usados} / {total} caracteres', { usados: infoCotaTraducao.caracteresUsados.toLocaleString(), total: infoCotaTraducao.cotaTotal.toLocaleString() })}
                </div>
              </div>
            )}
          </SecaoDependente>
        </>
      )}

      <h3 className="settings-section-title" style={{ marginTop: '32px' }}>{t('Google Gemini (IA)')}</h3>

      {(!termoBusca || "gemini api key google".includes(termoBusca.toLowerCase())) && (
        <div className="form-group">
          <label>{t('Gemini API Key')}</label>
          <input
            type="password"
            className="form-input"
            value={configuracoesApp.geminiApiKey || ''}
            onChange={e => AtualizarConfiguracao('geminiApiKey', e.target.value)}
            placeholder={t('Cole sua API Key do Gemini aqui...')}
          />
          <small style={{ color: 'var(--cor-texto-suave)', display: 'block', marginTop: '6px' }}>
            {t('Requer uma API key própria do Google AI Studio (gratuita).')}
          </small>
        </div>
      )}

      {(!termoBusca || "ativar modo gemini habilitar cota limite pausar resumo tradução linha".includes(termoBusca.toLowerCase())) && (
        <>
          <div className="form-group" style={{ marginTop: '16px' }}>
            <label style={{ display: 'flex', alignItems: 'center', gap: '8px', justifyContent: 'space-between' }}>
              <span>{t('Habilitar Gemini (resumo ou tradução da tela)')}</span>
              <input
                type="checkbox"
                checked={configuracoesApp.geminiAtivo || false}
                onChange={e => {
                  if (e.target.checked) {
                    const updates: any = { geminiAtivo: true, traducaoAtiva: false };
                    if (!configuracoesApp.geminiPopupResumo && !configuracoesApp.geminiPopupLinha) {
                      updates.geminiPopupResumo = true;
                    }
                    AplicarConfiguracao(updates);
                  } else {
                    AplicarConfiguracao({ geminiAtivo: false });
                  }
                }}
              />
            </label>
          </div>

          <SecaoDependente ativa={configuracoesApp.geminiAtivo || false}>
            {(!termoBusca || "modelo gemini flash pro".includes(termoBusca.toLowerCase())) && (
              <div className="form-group">
                <label>{t('Modelo do Gemini')}</label>
                <select
                  className="form-input"
                  value={configuracoesApp.geminiModelo || 'gemini-1.5-flash'}
                  onChange={e => {
                    const novoModelo = e.target.value;
                    const novoLimite = novoModelo.includes('pro') ? 50 : 1500;
                    AplicarConfiguracao({ geminiModelo: novoModelo, geminiLimiteRequisicoesDia: novoLimite });
                  }}
                >
                  <option value="gemini-1.5-flash">{t('Gemini 1.5 Flash (Rápido, Cota Alta)')}</option>
                  <option value="gemini-1.5-pro">{t('Gemini 1.5 Pro (Avançado, Cota Baixa)')}</option>
                  <option value="gemini-2.0-flash">{t('Gemini 2.0 Flash (Mais atual)')}</option>
                  <option value="gemini-2.0-pro">Gemini 2.0 Pro</option>
                </select>
                <small style={{ color: 'var(--cor-texto-suave)', display: 'block', marginTop: '6px' }}>
                  {t('Modelos Flash possuem cota gratuita muito maior (1500 req/dia) em relação aos Pro (50 req/dia).')}
                </small>
              </div>
            )}
            <div className="form-group">
              <label style={{ display: 'flex', alignItems: 'center', gap: '8px', justifyContent: 'space-between' }}>
                <span>{t('Pop-up com resumo da tela (Atalho de Pop-up de Tudo)')}</span>
                <input
                  type="checkbox"
                  checked={configuracoesApp.geminiPopupResumo || false}
                  onChange={e => {
                    if (e.target.checked) {
                      AplicarConfiguracao({ geminiPopupResumo: true, geminiPopupLinha: false });
                    } else {
                      if (!configuracoesApp.geminiPopupLinha) {
                        AplicarConfiguracao({ geminiPopupResumo: false, geminiAtivo: false });
                      } else {
                        AplicarConfiguracao({ geminiPopupResumo: false });
                      }
                    }
                  }}
                />
              </label>
            </div>

            <SecaoDependente ativa={configuracoesApp.geminiPopupResumo || false}>
              <div className="form-group">
                <label>{t('Canto do pop-up de resumo')}</label>
                <select
                  className="form-input"
                  value={configuracoesApp.geminiCantoResumo || 'superior-direito'}
                  onChange={e => AtualizarConfiguracao('geminiCantoResumo', e.target.value)}
                >
                  <option value="superior-esquerdo">{t('Superior esquerdo')}</option>
                  <option value="superior-direito">{t('Superior direito')}</option>
                  <option value="inferior-esquerdo">{t('Inferior esquerdo')}</option>
                  <option value="inferior-direito">{t('Inferior direito')}</option>
                </select>
              </div>

              <div className="form-group">
                <label style={{ display: 'flex', alignItems: 'center', gap: '8px', justifyContent: 'space-between' }}>
                  <span>{t('Enviar a imagem da tela junto (melhora o resumo)')}</span>
                  <input
                    type="checkbox"
                    checked={configuracoesApp.geminiEnviarImagem || false}
                    onChange={e => AtualizarConfiguracao('geminiEnviarImagem', e.target.checked)}
                  />
                </label>
                <small style={{ color: 'var(--cor-texto-suave)', display: 'block', marginTop: '6px' }}>
                  {t('Mais custoso: a captura inteira é enviada ao Gemini a cada resumo, consumindo muito mais tokens da sua cota — e tudo que estiver visível na tela é enviado ao Google.')}
                </small>
              </div>
            </SecaoDependente>

            <div className="form-group">
              <label style={{ display: 'flex', alignItems: 'center', gap: '8px', justifyContent: 'space-between' }}>
                <span>{t('Pop-ups de tradução em cada linha detectada (Atalho de Pop-up de Tudo)')}</span>
                <input
                  type="checkbox"
                  checked={configuracoesApp.geminiPopupLinha || false}
                  onChange={e => {
                    if (e.target.checked) {
                      AplicarConfiguracao({ geminiPopupLinha: true, geminiPopupResumo: false });
                    } else {
                      if (!configuracoesApp.geminiPopupResumo) {
                        AplicarConfiguracao({ geminiPopupLinha: false, geminiAtivo: false });
                      } else {
                        AplicarConfiguracao({ geminiPopupLinha: false });
                      }
                    }
                  }}
                />
              </label>
            </div>

            <div className="form-group">
              <label style={{ display: 'flex', alignItems: 'center', gap: '8px', justifyContent: 'space-between' }}>
                <span>{t('Pausar ao atingir o limite diário de requisições')}</span>
                <input
                  type="checkbox"
                  checked={configuracoesApp.geminiPausarPorCota || false}
                  onChange={e => AtualizarConfiguracao('geminiPausarPorCota', e.target.checked)}
                />
              </label>
            </div>

            <SecaoDependente ativa={configuracoesApp.geminiPausarPorCota || false}>
              <div className="form-group" style={{ display: 'flex', alignItems: 'center', gap: '16px', margin: 0 }}>
                <label style={{ margin: 0, flex: 1 }}>{t('Limite de requisições por dia')}</label>
                <input
                  type="number"
                  className="form-input"
                  min={1}
                  style={{ width: '100px' }}
                  value={configuracoesApp.geminiLimiteRequisicoesDia || 1500}
                  onChange={e => AtualizarConfiguracao('geminiLimiteRequisicoesDia', parseInt(e.target.value) || 1500)}
                />
              </div>
            </SecaoDependente>

            {infoCotaGemini && (() => {
              const limiteDia = configuracoesApp.geminiLimiteRequisicoesDia || 1500;
              const fracaoUsada = infoCotaGemini.requisicoesUsadas / limiteDia;
              return (
                <div className="form-group" style={{ marginTop: '16px', padding: '12px', backgroundColor: 'var(--cor-fundo-cartao)', borderRadius: '8px', border: '1px solid var(--cor-borda)', marginBottom: 0 }}>
                  <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: '8px' }}>
                    <span>{t('Uso da Cota Gemini (Dia {dia})', { dia: infoCotaGemini.data })}</span>
                    <strong>{(fracaoUsada * 100).toFixed(1)}%</strong>
                  </div>
                  <div style={{ width: '100%', height: '8px', backgroundColor: 'var(--cor-borda)', borderRadius: '4px', overflow: 'hidden' }}>
                    <div style={{
                      height: '100%',
                      width: `${Math.min(100, fracaoUsada * 100)}%`,
                      backgroundColor: fracaoUsada >= 0.9 ? '#f44336' : (fracaoUsada >= 0.75 ? '#ffb74d' : 'var(--cor-destaque)')
                    }} />
                  </div>
                  <div style={{ fontSize: '11px', color: 'var(--cor-texto-suave)', marginTop: '8px', textAlign: 'right' }}>
                    {t('{usadas} / {total} requisições hoje', { usadas: infoCotaGemini.requisicoesUsadas.toLocaleString(), total: limiteDia.toLocaleString() })}
                  </div>
                </div>
              );
            })()}

          </SecaoDependente>
        </>
      )}
    </>
  );
}
