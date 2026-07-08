// ----- Seção: Configurações — casca do painel (modal, sidebar de abas e busca) -----
// O conteúdo de cada aba vive em abas/Aba*.tsx; helpers compartilhados em comum.tsx.
import { CSSProperties } from 'react';
import { config, main } from '../../wailsjs/go/models';
import { ProgressoPreCacheTts } from './comum';
import { AbaGeral } from './abas/AbaGeral';
import { AbaEstudo } from './abas/AbaEstudo';
import { AbaMotores } from './abas/AbaMotores';
import { AbaDesempenho } from './abas/AbaDesempenho';
import { AbaAtalhos } from './abas/AbaAtalhos';
import { AbaTraducao } from './abas/AbaTraducao';
import { AbaArmazenamento } from './abas/AbaArmazenamento';
import { AbaInfo } from './abas/AbaInfo';

// Abas comuns da sidebar, na ordem de exibição (a aba Info tem botão próprio, com estilo especial).
const ABAS_SIDEBAR = [
  { chave: 'Geral', rotulo: 'Geral' },
  { chave: 'Motores', rotulo: 'Motores' },
  { chave: 'Desempenho', rotulo: 'Desempenho (Hardware)' },
  { chave: 'Atalhos', rotulo: 'Atalhos Globais' },
  { chave: 'Tradução', rotulo: 'Tradução (IA)' },
  { chave: 'Estudo', rotulo: 'Estudo' },
  { chave: 'Armazenamento', rotulo: 'Armazenamento' },
];

interface PainelConfiguracoesProps {
    painelConfigAberto: boolean;
    setPainelConfigAberto: (val: boolean) => void;
    configuracoesApp: config.Config;
    AtualizarConfiguracao: (key: keyof config.Config, value: any) => void;
    AplicarConfiguracao: (mudancas: Partial<config.Config>) => void;
    setConfirmacao: (c: any) => void;
    abaConfiguracao: string;
    setAbaConfiguracao: (val: string) => void;
    termoBusca: string;
    setTermoBusca: (val: string) => void;
    infoHardware: main.SystemHardware | null;
    resCaptura: main.Resolucao | null;
    monitores: any[];
    modelos: main.ModeloOcrInfo[];
    progressoModelo: Record<string, string>;
    baixandoModelo: string | null;
    infoArmazenamento: main.StorageInfo | null;
    infoCotaTraducao: main.InfoCotaTraducao | null;
    infoCotaGemini: main.InfoCotaGemini | null;
    armazenamentoOcupado: boolean;
    BaixarModeloOcr: (nome: string) => void;
    RemoverModeloOcr: (nome: string) => void;
    trocarModelo: (nome: string) => void;
    LimparCategoriaArmazenamento: (chave: string) => void;
    ExcluirTodoArmazenamento: () => void;
    ehCpuNome: (hw: string) => boolean;
    motores: main.MotorOcrInfo[];
    progressoMotor: Record<string, string>;
    baixandoMotor: string | null;
    trocandoMotor: string | null;
    BaixarMotorOcr: (nome: string) => void;
    RemoverMotorOcr: (nome: string) => void;
    TrocarMotorOcr: (nome: string) => void;
    motoresTts: main.MotorTtsInfo[];
    progressoMotorTts: Record<string, string>;
    baixandoMotorTts: string | null;
    BaixarMotorVoz: (nome: string) => void;
    RemoverMotorVoz: (nome: string) => void;
    progressoPreCacheTts: ProgressoPreCacheTts | null;
    PreCarregarAudioTts: () => void;
    PararPreCarregarAudioTts: () => void;
}

export function PainelConfiguracoes(props: PainelConfiguracoesProps) {
    const {
        painelConfigAberto, setPainelConfigAberto, configuracoesApp, AtualizarConfiguracao, AplicarConfiguracao, setConfirmacao,
        abaConfiguracao, setAbaConfiguracao, termoBusca, setTermoBusca, infoHardware,
        resCaptura, monitores, modelos, progressoModelo, baixandoModelo,
        infoArmazenamento, infoCotaTraducao, infoCotaGemini, armazenamentoOcupado, BaixarModeloOcr, RemoverModeloOcr, trocarModelo,
        LimparCategoriaArmazenamento, ExcluirTodoArmazenamento,
        ehCpuNome,
        motores, progressoMotor, baixandoMotor, trocandoMotor, BaixarMotorOcr, RemoverMotorOcr, TrocarMotorOcr,
        motoresTts, progressoMotorTts, baixandoMotorTts, BaixarMotorVoz, RemoverMotorVoz,
        progressoPreCacheTts, PreCarregarAudioTts, PararPreCarregarAudioTts
    } = props;

    if (!painelConfigAberto || !configuracoesApp) return null;

    // Com busca ativa, TODAS as abas ficam visíveis (cada controle se filtra pelo termo).
    const estiloAba = (chave: string): CSSProperties => ({
        display: (abaConfiguracao === chave || termoBusca) ? 'block' : 'none',
    });

    return (
        <div className="modal-overlay" onClick={() => setPainelConfigAberto(false)}>
          <div className="modal-content" onClick={e => e.stopPropagation()}>

            {/* Sidebar */}
            <div className="settings-sidebar">
              <div className="search-bar-container">
                <span className="search-icon">🔍</span>
                <input
                  type="text"
                  className="search-bar"
                  placeholder="Procurar..."
                  value={termoBusca}
                  onChange={(e) => setTermoBusca(e.target.value)}
                />
              </div>
              <h3>Configurações</h3>

              {ABAS_SIDEBAR.map(aba => (
                <button
                  key={aba.chave}
                  className={`settings-tab ${abaConfiguracao === aba.chave ? 'active' : ''}`}
                  onClick={() => setAbaConfiguracao(aba.chave)}
                >
                  {aba.rotulo}
                </button>
              ))}

              <button
                className={`settings-tab ${abaConfiguracao === 'Info' ? 'active' : ''}`}
                onClick={() => setAbaConfiguracao('Info')}
                style={{ display: 'flex', alignItems: 'center', gap: '6px', marginTop: 'auto' }}
              >
                <div style={{ display: 'flex', alignItems: 'center', gap: '6px', width: '100%' }}>
                  <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><circle cx="12" cy="12" r="10"></circle><line x1="12" y1="16" x2="12" y2="12"></line><line x1="12" y1="8" x2="12.01" y2="8"></line></svg>
                  <span>Info</span>
                  <span style={{ fontWeight: 300, fontSize: '11px', color: 'var(--cor-texto-suave)', fontStyle: 'italic', marginLeft: 'auto' }}>Beta</span>
                </div>
              </button>

            </div>

            {/* Main Area */}
            <div className="settings-main">
              <div className="settings-header">
                <div className="settings-header-top">
                  <h2>{abaConfiguracao}</h2>
                  <button className="modal-close" onClick={() => setPainelConfigAberto(false)}>×</button>
                </div>
              </div>

              <div className="settings-body">

                <div style={estiloAba('Geral')}>
                  <AbaGeral
                    termoBusca={termoBusca}
                    configuracoesApp={configuracoesApp}
                    AtualizarConfiguracao={AtualizarConfiguracao}
                    monitores={monitores}
                  />
                </div>

                <div style={estiloAba('Estudo')}>
                  <AbaEstudo
                    termoBusca={termoBusca}
                    configuracoesApp={configuracoesApp}
                    AtualizarConfiguracao={AtualizarConfiguracao}
                  />
                </div>

                <div style={estiloAba('Motores')}>
                  <AbaMotores
                    termoBusca={termoBusca}
                    configuracoesApp={configuracoesApp}
                    AtualizarConfiguracao={AtualizarConfiguracao}
                    AplicarConfiguracao={AplicarConfiguracao}
                    setConfirmacao={setConfirmacao}
                    infoHardware={infoHardware}
                    ehCpuNome={ehCpuNome}
                    motores={motores}
                    progressoMotor={progressoMotor}
                    baixandoMotor={baixandoMotor}
                    trocandoMotor={trocandoMotor}
                    BaixarMotorOcr={BaixarMotorOcr}
                    RemoverMotorOcr={RemoverMotorOcr}
                    TrocarMotorOcr={TrocarMotorOcr}
                    modelos={modelos}
                    progressoModelo={progressoModelo}
                    baixandoModelo={baixandoModelo}
                    BaixarModeloOcr={BaixarModeloOcr}
                    RemoverModeloOcr={RemoverModeloOcr}
                    trocarModelo={trocarModelo}
                    motoresTts={motoresTts}
                    progressoMotorTts={progressoMotorTts}
                    baixandoMotorTts={baixandoMotorTts}
                    BaixarMotorVoz={BaixarMotorVoz}
                    RemoverMotorVoz={RemoverMotorVoz}
                    progressoPreCacheTts={progressoPreCacheTts}
                    PreCarregarAudioTts={PreCarregarAudioTts}
                    PararPreCarregarAudioTts={PararPreCarregarAudioTts}
                  />
                </div>

                <div style={estiloAba('Desempenho')}>
                  <AbaDesempenho
                    termoBusca={termoBusca}
                    configuracoesApp={configuracoesApp}
                    AtualizarConfiguracao={AtualizarConfiguracao}
                    resCaptura={resCaptura}
                  />
                </div>

                <div style={estiloAba('Atalhos')}>
                  <AbaAtalhos
                    termoBusca={termoBusca}
                    configuracoesApp={configuracoesApp}
                    AtualizarConfiguracao={AtualizarConfiguracao}
                  />
                </div>

                <div style={estiloAba('Tradução')}>
                  <AbaTraducao
                    termoBusca={termoBusca}
                    configuracoesApp={configuracoesApp}
                    AtualizarConfiguracao={AtualizarConfiguracao}
                    AplicarConfiguracao={AplicarConfiguracao}
                    infoCotaTraducao={infoCotaTraducao}
                    infoCotaGemini={infoCotaGemini}
                  />
                </div>

                <div style={estiloAba('Armazenamento')}>
                  <AbaArmazenamento
                    termoBusca={termoBusca}
                    infoArmazenamento={infoArmazenamento}
                    armazenamentoOcupado={armazenamentoOcupado}
                    setConfirmacao={setConfirmacao}
                    LimparCategoriaArmazenamento={LimparCategoriaArmazenamento}
                    ExcluirTodoArmazenamento={ExcluirTodoArmazenamento}
                  />
                </div>

                <div style={estiloAba('Info')}>
                  <AbaInfo termoBusca={termoBusca} />
                </div>

                {termoBusca && (
                  <div style={{ textAlign: 'center', color: 'var(--cor-texto-suave)', marginTop: '32px' }}>
                    <small>Fim dos resultados da pesquisa.</small>
                  </div>
                )}

              </div>
            </div>
          </div>
        </div>
    );
}
