// ----- Seção: Configurações — aba Geral (captura, hover e leitura em voz alta) -----
import { config } from '../../../wailsjs/go/models';
import { SecaoDependente } from '../comum';
import { t } from '../../i18n/i18n';

interface AbaGeralProps {
  termoBusca: string;
  configuracoesApp: config.Config;
  AtualizarConfiguracao: (key: keyof config.Config, value: any) => void;
  monitores: any[];
}

export function AbaGeral({ termoBusca, configuracoesApp, AtualizarConfiguracao, monitores }: AbaGeralProps) {
  return (
    <>
      {termoBusca && <h3 className="settings-section-title">{t('Geral')}</h3>}

      {(!termoBusca || "idioma tradução traducao língua definições dicionário português inglês espanhol español language".includes(termoBusca.toLowerCase())) && (
        <div className="form-group">
          <label>{t('Idioma das traduções')}</label>
          <select
            className="form-input"
            value={configuracoesApp.idiomaTraducao || 'en'}
            onChange={e => AtualizarConfiguracao('idiomaTraducao', e.target.value)}
          >
            <option value="pt-BR">{t('Português (Brasil)')}</option>
            <option value="en">{t('English')}</option>
            <option value="es">{t('Espanhol')}</option>
          </select>
          <small style={{ color: 'var(--cor-texto-suave)', display: 'block', marginTop: '6px' }}>
            {t('Idioma das definições, dicas e frases mostradas nos pop-ups, cards e revisões. O inglês é usado como reserva quando algum conteúdo ainda não foi traduzido.')}{' '}
            <strong>{t('Reinicie o aplicativo')}</strong>{' '}
            {t('para aplicar a troca.')}
          </small>
        </div>
      )}

      {(!termoBusca || "confiança mínima ocr".includes(termoBusca.toLowerCase())) && (
        <div className="form-group">
          <label>{t('Confiança Mínima do OCR:')} {(configuracoesApp.confiancaMinimaOcr * 100).toFixed(0)}%</label>
          <input
            type="range"
            min="0.1" max="1" step="0.05"
            value={configuracoesApp.confiancaMinimaOcr}
            onChange={e => AtualizarConfiguracao('confiancaMinimaOcr', parseFloat(e.target.value))}
            style={{ width: '100%' }}
          />
        </div>
      )}

      {(!termoBusca || "monitor alvo tela captura".includes(termoBusca.toLowerCase())) && monitores.length > 0 && (
        <div className="form-group">
          <label>{t('Monitor Alvo (Captura OCR)')}</label>
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
            {t('Escolha de qual tela o aplicativo deve tirar o print na hora de traduzir.')}
          </small>
        </div>
      )}

      {(!termoBusca || "intervalo de captura segundos".includes(termoBusca.toLowerCase())) && (
        <div className="form-group" style={{ display: 'flex', alignItems: 'center', gap: '16px' }}>
          <label style={{ margin: 0, flex: 1 }}>{t('Intervalo de Captura Automática')}</label>
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

      {(!termoBusca || "vigia fantasma highlight destaque apagar palavras sumiram tela verificação".includes(termoBusca.toLowerCase())) && (
        <div className="form-group">
          <label style={{ display: 'flex', alignItems: 'center', gap: '8px', justifyContent: 'space-between' }}>
            <span>{t('Apagar destaques de palavras que saíram da tela (vigia)')}</span>
            <input
              type="checkbox"
              checked={configuracoesApp.vigiaCardsAtivo ?? true}
              onChange={e => AtualizarConfiguracao('vigiaCardsAtivo', e.target.checked)}
            />
          </label>
          <small style={{ color: 'var(--cor-texto-suave)', display: 'block', marginTop: '6px', paddingLeft: '24px' }}>
            {t('Entre um scan e outro, o app confere 1×/segundo se cada palavra detectada ainda está na mesma posição. Se o texto sumiu, o card continua listado no Descobrimento até o próximo scan, mas deixa de gerar highlight, pop-up e detecção perto do mouse (evita "highlights fantasmas").')}
          </small>
        </div>
      )}

      <SecaoDependente ativa={configuracoesApp.vigiaCardsAtivo ?? true}>
        {(!termoBusca || "rastrear palavras perdidas vigia procurar tela rolagem scroll".includes(termoBusca.toLowerCase())) && (
          <div className="form-group">
            <label style={{ display: 'flex', alignItems: 'center', gap: '8px', justifyContent: 'space-between' }}>
              <span>{t('Rastrear palavras perdidas pela tela')}</span>
              <input
                type="checkbox"
                checked={configuracoesApp.rastrearPalavrasPerdidas ?? false}
                onChange={e => AtualizarConfiguracao('rastrearPalavrasPerdidas', e.target.checked)}
              />
            </label>
            <small style={{ color: 'var(--cor-texto-suave)', display: 'block', marginTop: '6px', paddingLeft: '24px' }}>
              {t('Quando uma palavra some da posição original (ex.: o texto rolou), o vigia procura o recorte dela pela tela inteira e reativa o destaque na nova posição. Varre o print completo a cada segundo por palavra perdida — pode pesar em monitores de alta resolução, por isso vem desligado.')}
            </small>
          </div>
        )}
      </SecaoDependente>

      {(!termoBusca || "censurar janela app pop-up captura ocr privacidade".includes(termoBusca.toLowerCase())) && (
        <div className="form-group">
          <label style={{ display: 'flex', alignItems: 'center', gap: '8px', justifyContent: 'space-between' }}>
            <span>{t('Censurar a janela do app e os pop-ups na captura de tela enviada ao OCR')}</span>
            <input
              type="checkbox"
              checked={configuracoesApp.censurarJanelasDoApp}
              onChange={e => AtualizarConfiguracao('censurarJanelasDoApp', e.target.checked)}
            />
          </label>
          <small style={{ color: 'var(--cor-texto-suave)', display: 'block', marginTop: '6px', paddingLeft: '24px' }}>
            {t('Evita que o OCR leia de volta o texto da própria janela do Hanzi Tracker ou dos pop-ups (sempre visíveis por cima), caso estejam sobre a tela sendo escaneada.')}
          </small>
        </div>
      )}

      {(!termoBusca || "hover pop-up cursor tradução habilitar distância máxima pixels intervalo atualização ms tempo parado mouse".includes(termoBusca.toLowerCase())) && (
        <>
          <div className="form-group">
            <label style={{ display: 'flex', alignItems: 'center', gap: '8px', justifyContent: 'space-between' }}>
              <span>{t('Habilitar Pop-up de Tradução no Cursor (Hover)')}</span>
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
                <label style={{ margin: 0, flex: 1 }}>{t('Distância Máxima do Hover')}</label>
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
                <label style={{ margin: 0, flex: 1 }}>{t('Intervalo de Atualização do Hover')}</label>
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
                <label style={{ margin: 0, flex: 1 }}>{t('Tempo com o mouse parado para abrir Popup')}</label>
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
              <span>{t('Ler o Pinyin em Voz Alta')}</span>
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
                <span>{t('Ler ao abrir o pop-up do mouse')}</span>
                <input
                  type="checkbox"
                  checked={configuracoesApp.lerPinyinAoAbrirPopup}
                  onChange={e => AtualizarConfiguracao('lerPinyinAoAbrirPopup', e.target.checked)}
                />
              </label>
            </div>

            <div className="form-group">
              <label style={{ display: 'flex', alignItems: 'center', gap: '8px', justifyContent: 'space-between' }}>
                <span>{t('Ler ao expandir um card')}</span>
                <input
                  type="checkbox"
                  checked={configuracoesApp.lerPinyinAoExpandirCard}
                  onChange={e => AtualizarConfiguracao('lerPinyinAoExpandirCard', e.target.checked)}
                />
              </label>
            </div>

            <div className="form-group">
              <label style={{ display: 'flex', alignItems: 'center', gap: '8px', justifyContent: 'space-between' }}>
                <span>{t('Ler ao concluir desenho guiado')}</span>
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
