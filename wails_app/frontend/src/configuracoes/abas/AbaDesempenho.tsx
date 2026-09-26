// ----- Seção: Configurações — aba Desempenho (resolução do OCR e limites de CPU/GPU) -----
import { config, main, tela } from '../../../wailsjs/go/models';
import { SecaoDependente } from '../comum';
import { t } from '../../i18n/i18n';

interface AbaDesempenhoProps {
  termoBusca: string;
  configuracoesApp: config.Config;
  AtualizarConfiguracao: (key: keyof config.Config, value: any) => void;
  resCaptura: tela.Resolucao | null;
  AplicarConfiguracao: (mudancas: Partial<config.Config>) => void;
  infoHardware: main.SystemHardware | null;
  ehCpuNome: (hw: string) => boolean;
  motores: main.MotorOcrInfo[];
}

export function AbaDesempenho({
  termoBusca, configuracoesApp, AtualizarConfiguracao, resCaptura,
  AplicarConfiguracao, infoHardware, ehCpuNome, motores
}: AbaDesempenhoProps) {
  const motorAtivo = motores.find(m => m.ativo);
  const varianteMotor = (motorAtivo?.variante || 'CPU').toLowerCase();
  const motorSoCpu = !varianteMotor.includes('webgpu');
  const nomeCpu = infoHardware?.cpu || 'CPU';
  const hardwareEhCpu = ehCpuNome(configuracoesApp?.hardwareSelecionado || 'CPU');

  return (
    <>
      {termoBusca && <h3 className="settings-section-title" style={{ marginTop: '32px' }}>{t('Desempenho (Hardware)')}</h3>}

      {(!termoBusca || "hardware dispositivo processamento ocr cpu gpu nvidia amd intel api webgpu vulkan aceleração".includes(termoBusca.toLowerCase())) && (
        <div className="form-group">
          <label>{t('Hardware de Processamento')}{motorAtivo ? ` — ${t(motorAtivo.rotulo)}` : ''}</label>

          {motorSoCpu ? (
            <>
              <input className="form-input" value={nomeCpu} disabled readOnly />
              <small style={{ color: 'var(--cor-texto-suave)', display: 'block', marginTop: '6px' }}>
                {motorAtivo ? t('{nomeMotor} roda apenas em CPU — não há opção de GPU para este motor.', { nomeMotor: t(motorAtivo.rotulo) }) : t('Este motor roda apenas em CPU.')}
              </small>
            </>
          ) : (
            <>
              <select
                className="form-input"
                value={hardwareEhCpu ? nomeCpu : configuracoesApp.hardwareSelecionado}
                onChange={e => {
                  const val = e.target.value;
                  AplicarConfiguracao({
                    hardwareSelecionado: val,
                    dispositivoOcr: val === nomeCpu ? 'cpu' : 'webgpu',
                  });
                }}
              >
                <option value={nomeCpu} title={t('Compatível com todos os motores de OCR.')}>{nomeCpu} (CPU)</option>
                {infoHardware?.gpus?.map(gpu => (
                  <option key={gpu} value={gpu} title={t('Aceleração via WebGPU — funciona em qualquer GPU (Nvidia, AMD, Intel).')}>
                    {gpu}
                  </option>
                ))}
              </select>

              {!hardwareEhCpu && (
                <small style={{ color: 'var(--cor-texto-suave)', display: 'block', marginTop: '6px' }}>
                  {t('Aceleração via WebGPU (D3D12 no Windows; Vulkan no Linux). O processamento usa o adaptador de vídeo padrão do sistema.')}
                </small>
              )}
            </>
          )}
        </div>
      )}

      {(!termoBusca || "qualidade da imagem ocr resolução captura desempenho".includes(termoBusca.toLowerCase())) && (() => {
        const pct = configuracoesApp.escalaResolucaoOcr || 100;
        const ehNativo = pct >= 100;

        // Apenas para fins de exibição visual amigável: calculamos a resolução resultante atual
        const wNat = resCaptura?.largura || 1920;
        const hNat = resCaptura?.altura || 1080;
        const ladoMaiorNat = Math.max(wNat, hNat);
        const ratio = pct / 100.0;
        const valorLadoMaior = Math.round(ratio * ladoMaiorNat);
        const ratioMenor = Math.min(wNat, hNat) / ladoMaiorNat;
        const ladoMenorCalc = Math.round(valorLadoMaior * ratioMenor);
        const wExib = wNat >= hNat ? valorLadoMaior : ladoMenorCalc;
        const hExib = wNat >= hNat ? ladoMenorCalc : valorLadoMaior;

        return (
          <div className="form-group">
            <label>{t('Qualidade da Imagem (OCR):')} {pct}% ({wExib} × {hExib}){ehNativo ? ` — ${t('nativo')}` : ''}</label>
            <input
              type="range"
              min={10}
              max={100}
              step={5}
              value={pct}
              onChange={e => {
                AtualizarConfiguracao('escalaResolucaoOcr', parseInt(e.target.value));
              }}
              style={{ width: '100%' }}
            />
            <small style={{ color: 'var(--cor-texto-suave)', display: 'block', marginTop: '6px' }}>
              {t('Menor resolução = mais rápido e menos memória, porém menos preciso. Resolução nativa atual: {resolucaoNativa}.', { resolucaoNativa: `${wNat} × ${hNat}` })}
            </small>
          </div>
        );
      })()}

      {(!termoBusca || "threads cpu ocr".includes(termoBusca.toLowerCase())) && (
        <div className="form-group" style={{ display: 'flex', alignItems: 'center', gap: '16px' }}>
          <label style={{ margin: 0, flex: 1 }}>{t('Núcleos/Threads CPU permitidos para OCR')}</label>
          <input
            type="range"
            min="1" max="16"
            value={configuracoesApp.threadsCpuOcr}
            onChange={e => AtualizarConfiguracao('threadsCpuOcr', parseInt(e.target.value))}
            style={{ width: '200px', margin: 0 }}
          />
          <span style={{ minWidth: '35px', textAlign: 'right', color: 'var(--cor-destaque)', fontWeight: 'bold' }}>{configuracoesApp.threadsCpuOcr}</span>
        </div>
      )}

      {(!termoBusca || "limitar uso máximo cpu tolerância".includes(termoBusca.toLowerCase())) && (
        <>
          <div className="form-group">
            <label style={{ display: 'flex', alignItems: 'center', gap: '8px', justifyContent: 'space-between' }}>
              <span>{t('Pausar escaneamentos se uso da CPU estiver muito alto')}</span>
              <input
                type="checkbox"
                checked={configuracoesApp.limitarPorUsoCpu}
                onChange={e => AtualizarConfiguracao('limitarPorUsoCpu', e.target.checked)}
              />
            </label>
          </div>

          <SecaoDependente ativa={configuracoesApp.limitarPorUsoCpu}>
            {(!termoBusca || "tolerância de uso cpu máximo".includes(termoBusca.toLowerCase())) && (
              <div className="form-group" style={{ display: 'flex', alignItems: 'center', gap: '16px', margin: 0 }}>
                <label style={{ margin: 0, flex: 1 }}>{t('Tolerância de Uso CPU')}</label>
                <input
                  type="range"
                  min="10" max="100" step="5"
                  value={configuracoesApp.usoMaximoCpuPercent}
                  onChange={e => AtualizarConfiguracao('usoMaximoCpuPercent', parseFloat(e.target.value))}
                  style={{ width: '200px', margin: 0 }}
                />
                <span style={{ minWidth: '40px', textAlign: 'right', color: 'var(--cor-destaque)', fontWeight: 'bold' }}>{configuracoesApp.usoMaximoCpuPercent}%</span>
              </div>
            )}
          </SecaoDependente>
        </>
      )}

      {(!termoBusca || "limitar uso máximo gpu tolerância".includes(termoBusca.toLowerCase())) && (
        <>
          <div className="form-group">
            <label style={{ display: 'flex', alignItems: 'center', gap: '8px', justifyContent: 'space-between' }}>
              <span>{t('Pausar escaneamentos se uso da GPU estiver muito alto')}</span>
              <input
                type="checkbox"
                checked={configuracoesApp.limitarPorUsoGpu}
                onChange={e => AtualizarConfiguracao('limitarPorUsoGpu', e.target.checked)}
              />
            </label>
          </div>

          <SecaoDependente ativa={configuracoesApp.limitarPorUsoGpu}>
            {(!termoBusca || "tolerância de uso gpu máximo".includes(termoBusca.toLowerCase())) && (
              <div className="form-group" style={{ display: 'flex', alignItems: 'center', gap: '16px', margin: 0 }}>
                <label style={{ margin: 0, flex: 1 }}>{t('Tolerância de Uso GPU')}</label>
                <input
                  type="range"
                  min="10" max="100" step="5"
                  value={configuracoesApp.usoMaximoGpuPercent}
                  onChange={e => AtualizarConfiguracao('usoMaximoGpuPercent', parseFloat(e.target.value))}
                  style={{ width: '200px', margin: 0 }}
                />
                <span style={{ minWidth: '40px', textAlign: 'right', color: 'var(--cor-destaque)', fontWeight: 'bold' }}>{configuracoesApp.usoMaximoGpuPercent}%</span>
              </div>
            )}
          </SecaoDependente>
        </>
      )}
    </>
  );
}
