// ----- Seção: Configurações — aba Geral (captura, hover e leitura em voz alta) -----
import { config } from '../../../wailsjs/go/models';
import { SecaoDependente } from '../comum';

interface AbaGeralProps {
  termoBusca: string;
  configuracoesApp: config.Config;
  AtualizarConfiguracao: (key: keyof config.Config, value: any) => void;
  monitores: any[];
}

export function AbaGeral({ termoBusca, configuracoesApp, AtualizarConfiguracao, monitores }: AbaGeralProps) {
  return (
    <>
      {termoBusca && <h3 className="settings-section-title">Geral</h3>}

      {(!termoBusca || "monitor alvo tela captura".includes(termoBusca.toLowerCase())) && monitores.length > 0 && (
        <div className="form-group">
          <label>Monitor Alvo (Captura OCR)</label>
          <select
            className="form-input"
            value={configuracoesApp.monitorAlvo || 0}
            onChange={e => AtualizarConfiguracao('monitorAlvo', parseInt(e.target.value))}
          >
            {monitores.map(m => (
              <option key={m.id} value={m.id}>
                {m.nome} ({m.largura}x{m.altura})
              </option>
            ))}
          </select>
          <small style={{ color: 'var(--cor-texto-suave)', display: 'block', marginTop: '6px' }}>
            Escolha de qual tela o aplicativo deve tirar o print na hora de traduzir.
          </small>
        </div>
      )}

      {(!termoBusca || "intervalo de captura segundos".includes(termoBusca.toLowerCase())) && (
        <div className="form-group" style={{ display: 'flex', alignItems: 'center', gap: '16px' }}>
          <label style={{ margin: 0, flex: 1 }}>Intervalo de Captura Automática</label>
          <input
            type="range"
            min="3" max="60"
            value={configuracoesApp.intervaloCapturaSegundos}
            onChange={e => AtualizarConfiguracao('intervaloCapturaSegundos', parseInt(e.target.value))}
            style={{ width: '200px', margin: 0 }}
          />
          <span style={{ minWidth: '35px', textAlign: 'right', color: 'var(--cor-destaque)', fontWeight: 'bold' }}>{configuracoesApp.intervaloCapturaSegundos}s</span>
        </div>
      )}

      {(!termoBusca || "censurar janela app pop-up captura ocr privacidade".includes(termoBusca.toLowerCase())) && (
        <div className="form-group">
          <label style={{ display: 'flex', alignItems: 'center', gap: '8px', justifyContent: 'space-between' }}>
            <span>Censurar a janela do app e os pop-ups na captura de tela enviada ao OCR</span>
            <input
              type="checkbox"
              checked={configuracoesApp.censurarJanelasDoApp}
              onChange={e => AtualizarConfiguracao('censurarJanelasDoApp', e.target.checked)}
            />
          </label>
          <small style={{ color: 'var(--cor-texto-suave)', display: 'block', marginTop: '6px', paddingLeft: '24px' }}>
            Evita que o OCR leia de volta o texto da própria janela do Hanzi Tracker ou dos pop-ups
            (sempre visíveis por cima), caso estejam sobre a tela sendo escaneada.
          </small>
        </div>
      )}

      {(!termoBusca || "hover pop-up cursor tradução habilitar distância máxima pixels intervalo atualização ms tempo parado mouse".includes(termoBusca.toLowerCase())) && (
        <>
          <div className="form-group">
            <label style={{ display: 'flex', alignItems: 'center', gap: '8px', justifyContent: 'space-between' }}>
              <span>Habilitar Pop-up de Tradução no Cursor (Hover)</span>
              <input
                type="checkbox"
                checked={configuracoesApp.habilitarPopupHover}
                onChange={e => AtualizarConfiguracao('habilitarPopupHover', e.target.checked)}
              />
            </label>
          </div>

          <SecaoDependente ativa={configuracoesApp.habilitarPopupHover}>
            {(!termoBusca || "distância máxima hover pixels".includes(termoBusca.toLowerCase())) && (
              <div className="form-group" style={{ display: 'flex', alignItems: 'center', gap: '16px' }}>
                <label style={{ margin: 0, flex: 1 }}>Distância Máxima do Hover</label>
                <input
                  type="range"
                  min="50" max="500" step="10"
                  value={configuracoesApp.distanciaMaximaHoverPx}
                  onChange={e => AtualizarConfiguracao('distanciaMaximaHoverPx', parseInt(e.target.value))}
                  style={{ width: '200px', margin: 0 }}
                />
                <span style={{ minWidth: '45px', textAlign: 'right', color: 'var(--cor-destaque)', fontWeight: 'bold' }}>{configuracoesApp.distanciaMaximaHoverPx}px</span>
              </div>
            )}

            {(!termoBusca || "intervalo atualização hover ms".includes(termoBusca.toLowerCase())) && (
              <div className="form-group" style={{ display: 'flex', alignItems: 'center', gap: '16px' }}>
                <label style={{ margin: 0, flex: 1 }}>Intervalo de Atualização do Hover</label>
                <input
                  type="range"
                  min="16" max="500" step="10"
                  value={configuracoesApp.intervaloAtualizacaoHoverMs}
                  onChange={e => AtualizarConfiguracao('intervaloAtualizacaoHoverMs', parseInt(e.target.value))}
                  style={{ width: '200px', margin: 0 }}
                />
                <span style={{ minWidth: '45px', textAlign: 'right', color: 'var(--cor-destaque)', fontWeight: 'bold' }}>{configuracoesApp.intervaloAtualizacaoHoverMs}ms</span>
              </div>
            )}

            {(!termoBusca || "tempo parado popup ms mouse".includes(termoBusca.toLowerCase())) && (
              <div className="form-group" style={{ display: 'flex', alignItems: 'center', gap: '16px', margin: 0 }}>
                <label style={{ margin: 0, flex: 1 }}>Tempo com o mouse parado para abrir Popup</label>
                <input
                  type="range"
                  min="100" max="2000" step="100"
                  value={configuracoesApp.tempoParadoPopupMs}
                  onChange={e => AtualizarConfiguracao('tempoParadoPopupMs', parseInt(e.target.value))}
                  style={{ width: '200px', margin: 0 }}
                />
                <span style={{ minWidth: '55px', textAlign: 'right', color: 'var(--cor-destaque)', fontWeight: 'bold' }}>{configuracoesApp.tempoParadoPopupMs}ms</span>
              </div>
            )}
          </SecaoDependente>
        </>
      )}

      {(!termoBusca || "leitura pinyin voz alta tts falar áudio kokoro chattts pop-up card expandir".includes(termoBusca.toLowerCase())) && (
        <>
          <div className="form-group">
            <label style={{ display: 'flex', alignItems: 'center', gap: '8px', justifyContent: 'space-between' }}>
              <span>Ler o Pinyin em Voz Alta</span>
              <input
                type="checkbox"
                checked={configuracoesApp.habilitarLeituraPinyin}
                onChange={e => AtualizarConfiguracao('habilitarLeituraPinyin', e.target.checked)}
              />
            </label>
          </div>

          <SecaoDependente ativa={configuracoesApp.habilitarLeituraPinyin}>
            <div className="form-group">
              <label style={{ display: 'flex', alignItems: 'center', gap: '8px', justifyContent: 'space-between' }}>
                <span>Ler ao abrir o pop-up do mouse</span>
                <input
                  type="checkbox"
                  checked={configuracoesApp.lerPinyinAoAbrirPopup}
                  onChange={e => AtualizarConfiguracao('lerPinyinAoAbrirPopup', e.target.checked)}
                />
              </label>
            </div>

            <div className="form-group">
              <label style={{ display: 'flex', alignItems: 'center', gap: '8px', justifyContent: 'space-between' }}>
                <span>Ler ao expandir um card</span>
                <input
                  type="checkbox"
                  checked={configuracoesApp.lerPinyinAoExpandirCard}
                  onChange={e => AtualizarConfiguracao('lerPinyinAoExpandirCard', e.target.checked)}
                />
              </label>
            </div>

            <div className="form-group">
              <label style={{ display: 'flex', alignItems: 'center', gap: '8px', justifyContent: 'space-between' }}>
                <span>Ler ao concluir desenho guiado</span>
                <input
                  type="checkbox"
                  checked={configuracoesApp.lerPinyinAoCompletarDesenho}
                  onChange={e => AtualizarConfiguracao('lerPinyinAoCompletarDesenho', e.target.checked)}
                />
              </label>
            </div>

          </SecaoDependente>
        </>
      )}
    </>
  );
}
