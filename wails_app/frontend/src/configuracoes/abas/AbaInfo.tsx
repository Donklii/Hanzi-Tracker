// ----- Seção: Configurações — aba Info (sobre o app, atualizações e créditos) -----
import { useEffect, useState } from 'react';
import { config, main, notasversao } from '../../../wailsjs/go/models';
import { ObterEstadoAtualizacao, VerificarAtualizacao, IniciarAtualizacao, ObterNotasVersao } from '../../../wailsjs/go/main/App';
import { BrowserOpenURL } from '../../../wailsjs/runtime/runtime';
import iconeGithub from '../../assets/images/GithubIcon.png';
import { t } from '../../i18n/i18n';
import { ModalLicencas } from '../licencas/ModalLicencas';
import { ModalNotasVersao } from '../../notasVersao/ModalNotasVersao';

interface AbaInfoProps {
  termoBusca: string;
  configuracoesApp: config.Config;
  AtualizarConfiguracao: (key: keyof config.Config, value: any) => void;
}

export function AbaInfo({ termoBusca, configuracoesApp, AtualizarConfiguracao }: AbaInfoProps) {
  const [licencasAbertas, setLicencasAbertas] = useState(false);
  const [notasVersaoAbertas, setNotasVersaoAbertas] = useState(false);
  const [historicoNotas, setHistoricoNotas] = useState<notasversao.Nota[]>([]);
  const [carregandoNotas, setCarregandoNotas] = useState(false);
  const [estadoApp, setEstadoApp] = useState<main.EstadoAtualizacao | null>(null);
  const [verificando, setVerificando] = useState(false);
  const [resultadoVerificacao, setResultadoVerificacao] = useState<main.ResultadoVerificacaoAtualizacao | null>(null);
  const [erroVerificacao, setErroVerificacao] = useState<string | null>(null);

  useEffect(() => {
    ObterEstadoAtualizacao()
      .then(estado => setEstadoApp(estado))
      .catch(() => { /* falha silenciosa em dev */ });
  }, []);

  const canalConfigurado = configuracoesApp.canalAtualizacao || (estadoApp?.canalBuild || 'estavel');

  const lidarAbrirNovidades = async () => {
    setCarregandoNotas(true);
    try {
      const notas = await ObterNotasVersao();
      setHistoricoNotas(notas || []);
      setNotasVersaoAbertas(true);
    } catch {
      // falha silenciosa
    } finally {
      setCarregandoNotas(false);
    }
  };

  const lidarVerificar = async (canalParaVerificar: string) => {
    setVerificando(true);
    setErroVerificacao(null);
    setResultadoVerificacao(null);
    try {
      const res = await VerificarAtualizacao(canalParaVerificar);
      setResultadoVerificacao(res);
    } catch (err: any) {
      setErroVerificacao(typeof err === 'string' ? err : (err?.message || String(err)));
    } finally {
      setVerificando(false);
    }
  };

  const lidarTrocarCanal = (novoCanal: string) => {
    AtualizarConfiguracao('canalAtualizacao', novoCanal);
    lidarVerificar(novoCanal);
  };

  const lidarAtualizar = async () => {
    try {
      await IniciarAtualizacao(canalConfigurado);
    } catch (err: any) {
      setErroVerificacao(typeof err === 'string' ? err : (err?.message || String(err)));
    }
  };

  const correspondeAtualizacao = !termoBusca || "atualização atualizacao versão versao canal dev estável estavel update".includes(termoBusca.toLowerCase());
  const correspondeSobre = !termoBusca || "sobre créditos creditos desenvolvedor github repositório licencas licenças novidades notas versão versao changelog".includes(termoBusca.toLowerCase());

  return (
    <>
      {termoBusca && (correspondeAtualizacao || correspondeSobre) && (
        <h3 className="settings-section-title" style={{ marginTop: '32px' }}>{t('Info')}</h3>
      )}

      {correspondeAtualizacao && (
        <div className="form-group" style={{ marginBottom: '24px' }}>
          <h3 style={{ marginBottom: '16px' }}>{t('Atualizações')}</h3>

          {estadoApp?.buildLocal ? (
            <div style={{
              background: 'rgba(234, 179, 8, 0.1)',
              border: '1px solid rgba(234, 179, 8, 0.3)',
              padding: '10px 14px',
              borderRadius: '8px',
              color: '#eab308',
              marginBottom: '16px',
              fontSize: '13px'
            }}>
              {t('Build local — atualização automática desativada')}
            </div>
          ) : (
            <div style={{ color: 'var(--cor-texto-suave)', fontSize: '14px', lineHeight: '1.6', marginBottom: '16px' }}>
              <div>
                <strong style={{ color: 'var(--cor-texto)' }}>{t('Versão atual:')}</strong> {estadoApp?.versaoAtual || '—'}
              </div>
              <div style={{ fontSize: '13px' }}>
                {t('Canal do executável:')} {estadoApp?.canalBuild === 'dev' ? t('Dev (instável)') : t('Estável')}
                {estadoApp?.commit && <span> ({t('commit')} {estadoApp.commit})</span>}
              </div>
            </div>
          )}

          <div style={{ marginBottom: '16px' }}>
            <label style={{ display: 'block', marginBottom: '6px', fontWeight: 500 }}>
              {t('Canal de atualização')}
            </label>
            <select
              className="form-input"
              value={canalConfigurado}
              disabled={estadoApp?.buildLocal || verificando}
              onChange={e => lidarTrocarCanal(e.target.value)}
            >
              <option value="estavel">{t('Estável')}</option>
              <option value="dev">{t('Dev (instável)')}</option>
            </select>

            {estadoApp?.canalBuild === 'dev' && canalConfigurado === 'estavel' && (
              <small style={{ color: '#eab308', display: 'block', marginTop: '6px', lineHeight: '1.4' }}>
                {t('Você está em um build do canal Dev. Ao selecionar o canal Estável, o aplicativo voltará para a versão estável mais recente, mesmo que ela tenha uma numeração anterior.')}
              </small>
            )}

            <small style={{ color: 'var(--cor-texto-suave)', display: 'block', marginTop: '6px', lineHeight: '1.5' }}>
              {t('A verificação de atualização roda automaticamente a cada abertura do aplicativo. O canal Dev recebe cada alteração publicada na branch principal e pode conter instabilidades.')}
            </small>
          </div>

          <div style={{ display: 'flex', alignItems: 'center', gap: '12px', flexWrap: 'wrap' }}>
            <button
              className="scan-btn"
              disabled={estadoApp?.buildLocal || verificando}
              onClick={() => lidarVerificar(canalConfigurado)}
              style={{ padding: '8px 16px', fontSize: '13px' }}
            >
              {verificando ? t('Verificando…') : t('Verificar agora')}
            </button>
          </div>

          {resultadoVerificacao && (
            resultadoVerificacao.disponivel ? (
              <div style={{
                marginTop: '16px',
                padding: '14px',
                borderRadius: '8px',
                background: 'rgba(34, 197, 94, 0.12)',
                border: '1px solid rgba(34, 197, 94, 0.35)',
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'space-between',
                gap: '12px',
                flexWrap: 'wrap'
              }}>
                <span style={{ fontSize: '14px', color: '#86efac' }}>
                  {t('Versão')} <strong>{resultadoVerificacao.versaoAlvo}</strong> {t('disponível')}
                </span>
                <button
                  className="scan-btn"
                  onClick={lidarAtualizar}
                  style={{ padding: '8px 16px', fontSize: '13px', background: '#16a34a' }}
                >
                  {t('Atualizar e reiniciar')}
                </button>
              </div>
            ) : (
              <div style={{ marginTop: '12px', color: 'var(--cor-texto-suave)', fontSize: '13px' }}>
                {resultadoVerificacao.motivo ? t(resultadoVerificacao.motivo) : t('Você já está na versão mais recente.')}
              </div>
            )
          )}

          {erroVerificacao && (
            <div style={{ marginTop: '12px', color: '#ef4444', fontSize: '13px' }}>
              {erroVerificacao}
            </div>
          )}
        </div>
      )}

      {correspondeSobre && (
        <div className="form-group">
          <h3 style={{ marginBottom: '16px' }}>{t('Sobre o Hanzi Tracker')}</h3>
          <p style={{ color: 'var(--cor-texto-suave)', lineHeight: '1.5', marginBottom: '12px' }}>
            {t('O Hanzi Tracker é uma ferramenta voltada a auxiliar e otimizar o estudo do idioma chinês de forma dinâmica e interativa, fornecendo leitura contextual, revisão estruturada e reconhecimento óptico de caracteres em tempo real.')}
          </p>

          <h4 style={{ marginTop: '24px', marginBottom: '8px' }}>{t('Créditos')}</h4>
          <ul style={{ color: 'var(--cor-texto-suave)', lineHeight: '1.5', paddingLeft: '20px', marginBottom: '12px' }}>
            <li><strong>Donklii:</strong> {t('Desenvolvedor principal e criador do projeto.')}</li>
            <li><strong>Make Me a Hanzi:</strong> {t('Definições, pinyin, decomposição e etimologia dos caracteres (LGPL-3.0, com dados do Unihan — Copyright © 1991–2009 Unicode, Inc. — e do CJKlib).')}</li>
            <li><strong>Arphic Technology Co., Ltd.:</strong> {t('Traços e animações de escrita, extraídos de suas fontes pelo Make Me a Hanzi (Copyright © 1999, Arphic Public License).')}</li>
            <li><strong>CC-CEDICT:</strong> {t('Grafias, pinyin e significados das palavras (MDBG, CC BY-SA 4.0).')}</li>
            <li><strong>FrequencyWords:</strong> {t('Frequência de uso das palavras, por Hermit Dave, a partir do corpus OpenSubtitles (CC BY-SA 4.0).')}</li>
          </ul>
          <p style={{ color: 'var(--cor-texto-suave)', fontSize: '13px', lineHeight: '1.5', marginBottom: '20px' }}>
            {t('Os dados de terceiros são fornecidos "como estão", sem garantia. As atribuições exigidas, as modificações que fizemos e o texto completo de cada licença estão em Licenças de terceiros.')}
          </p>

          <div style={{ display: 'flex', flexWrap: 'wrap', gap: '8px' }}>
            <button
              className="scan-btn"
              onClick={() => BrowserOpenURL('https://github.com/Donklii/Hanzi-Tracker')}
              style={{ display: 'inline-flex', alignItems: 'center', gap: '8px', padding: '10px 16px', fontSize: '14px' }}
            >
              <img src={iconeGithub} width="20" height="20" alt="GitHub" style={{ filter: 'invert(1)' }} />
              {t('Acessar Repositório no GitHub')}
            </button>
            <button
              className="scan-btn"
              onClick={() => setLicencasAbertas(true)}
              style={{ padding: '10px 16px', fontSize: '14px' }}
            >
              {t('Licenças de terceiros')}
            </button>
            <button
              className="scan-btn"
              disabled={carregandoNotas}
              onClick={lidarAbrirNovidades}
              style={{ padding: '10px 16px', fontSize: '14px' }}
            >
              {t('Novidades')}
            </button>
          </div>
        </div>
      )}

      <ModalLicencas aberto={licencasAbertas} aoFechar={() => setLicencasAbertas(false)} />
      <ModalNotasVersao
        aberto={notasVersaoAbertas}
        titulo={t('Histórico de novidades')}
        notas={historicoNotas}
        aoFechar={() => setNotasVersaoAbertas(false)}
      />
    </>
  );
}
