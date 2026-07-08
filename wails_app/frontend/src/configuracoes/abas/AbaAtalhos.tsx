// ----- Seção: Configurações — aba Atalhos Globais -----
import { config } from '../../../wailsjs/go/models';

interface AbaAtalhosProps {
  termoBusca: string;
  configuracoesApp: config.Config;
  AtualizarConfiguracao: (key: keyof config.Config, value: any) => void;
}

export function AbaAtalhos({ termoBusca, configuracoesApp, AtualizarConfiguracao }: AbaAtalhosProps) {
  return (
    <>
      {termoBusca && <h3 className="settings-section-title" style={{ marginTop: '32px' }}>Atalhos Globais</h3>}

      {(!termoBusca || "atalho escanear".includes(termoBusca.toLowerCase())) && (
        <div className="form-group">
          <label>Atalho: Escanear Tela</label>
          <input
            className="form-input"
            value={configuracoesApp.atalhoEscanear}
            onChange={e => AtualizarConfiguracao('atalhoEscanear', e.target.value)}
            placeholder="Ex: ctrl+shift+e"
          />
        </div>
      )}

      {(!termoBusca || "atalho popup todos".includes(termoBusca.toLowerCase())) && (
        <div className="form-group">
          <label>Atalho: Mostrar Pop-up de Tudo</label>
          <input
            className="form-input"
            value={configuracoesApp.atalhoPopupTodos}
            onChange={e => AtualizarConfiguracao('atalhoPopupTodos', e.target.value)}
            placeholder="Ex: ctrl+shift+t"
          />
        </div>
      )}

      {(!termoBusca || "atalho marcar estudo vocabulário".includes(termoBusca.toLowerCase())) && (
        <div className="form-group">
          <label>Atalho: Marcar Card atual em Estudo</label>
          <input
            className="form-input"
            value={configuracoesApp.atalhoMarcarEstudo}
            onChange={e => AtualizarConfiguracao('atalhoMarcarEstudo', e.target.value)}
            placeholder="Ex: ctrl+shift+m"
          />
        </div>
      )}

      {(!termoBusca || "atalho hover pop-up popup cursor".includes(termoBusca.toLowerCase())) && (
        <div className="form-group">
          <label>Atalho: Ligar/Desligar Pop-up no Cursor</label>
          <input
            className="form-input"
            value={configuracoesApp.atalhoAlternarPopupHover}
            onChange={e => AtualizarConfiguracao('atalhoAlternarPopupHover', e.target.value)}
            placeholder="Ex: ctrl+shift+h"
          />
        </div>
      )}
    </>
  );
}
