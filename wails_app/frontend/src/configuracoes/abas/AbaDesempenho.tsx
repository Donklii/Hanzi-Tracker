// ----- Seção: Configurações — aba Desempenho (resolução do OCR e limites de CPU/GPU) -----
import { config, main } from '../../../wailsjs/go/models';
import { SecaoDependente } from '../comum';

interface AbaDesempenhoProps {
  termoBusca: string;
  configuracoesApp: config.Config;
  AtualizarConfiguracao: (key: keyof config.Config, value: any) => void;
  resCaptura: main.Resolucao | null;
}

export function AbaDesempenho({ termoBusca, configuracoesApp, AtualizarConfiguracao, resCaptura }: AbaDesempenhoProps) {
  return (
    <>
      {termoBusca && <h3 className="settings-section-title" style={{ marginTop: '32px' }}>Desempenho (Hardware)</h3>}

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
            <label>Qualidade da Imagem (OCR): {pct}% ({wExib} × {hExib}){ehNativo ? ' — nativo' : ''}</label>
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
              Menor resolução = mais rápido e menos memória, porém menos preciso. Resolução nativa atual: {wNat} × {hNat}.
            </small>
          </div>
        );
      })()}

      {(!termoBusca || "threads cpu ocr".includes(termoBusca.toLowerCase())) && (
        <div className="form-group" style={{ display: 'flex', alignItems: 'center', gap: '16px' }}>
          <label style={{ margin: 0, flex: 1 }}>Núcleos/Threads CPU permitidos para OCR</label>
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
              <span>Pausar escaneamentos se uso da CPU estiver muito alto</span>
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
                <label style={{ margin: 0, flex: 1 }}>Tolerância de Uso CPU</label>
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
              <span>Pausar escaneamentos se uso da GPU estiver muito alto</span>
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
                <label style={{ margin: 0, flex: 1 }}>Tolerância de Uso GPU</label>
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
