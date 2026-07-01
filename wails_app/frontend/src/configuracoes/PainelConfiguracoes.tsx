// ----- Seção: Configurações -----
import { useState } from 'react';
import { config, main } from '../../wailsjs/go/models';
import { AbrirPastaDados } from '../../wailsjs/go/main/App';

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
    avisoCompatibilidade: string | null;
    infoArmazenamento: main.StorageInfo | null;
    armazenamentoOcupado: boolean;
    BaixarModeloOcr: (nome: string) => void;
    RemoverModeloOcr: (nome: string) => void;
    trocarModelo: (nome: string) => void;
    CarregarArmazenamento: () => void;
    LimparCategoriaArmazenamento: (chave: string) => void;
    ExcluirTodoArmazenamento: () => void;
    hardwareEhCpu: () => boolean;
    ehCpuNome: (hw: string) => boolean;
    ehNvidia: (hw: string) => boolean;
    apiCompativelComModelo: (modelo: string, api: string) => boolean;
    hardwareCompativelComModelo: (modelo: string, hw: string) => boolean;
    rotuloModelo: (m: string) => string;
}

export function PainelConfiguracoes(props: PainelConfiguracoesProps) {
    // Desestruturação para manter a compatibilidade interna do código antigo
    const {
        painelConfigAberto, setPainelConfigAberto, configuracoesApp, AtualizarConfiguracao, AplicarConfiguracao, setConfirmacao,
        abaConfiguracao, setAbaConfiguracao, termoBusca, setTermoBusca, infoHardware,
        resCaptura, monitores, modelos, progressoModelo, baixandoModelo, avisoCompatibilidade,
        infoArmazenamento, armazenamentoOcupado, BaixarModeloOcr, RemoverModeloOcr, trocarModelo,
        CarregarArmazenamento, LimparCategoriaArmazenamento, ExcluirTodoArmazenamento,
        hardwareEhCpu, ehCpuNome, ehNvidia, apiCompativelComModelo, hardwareCompativelComModelo, rotuloModelo
    } = props;


    const FormatarTamanho = (bytes: number): string => {
      if (bytes === 0) return '0 B';
      const k = 1024;
      const tamanhos = ['B', 'KB', 'MB', 'GB', 'TB'];
      const i = Math.floor(Math.log(bytes) / Math.log(k));
      return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + tamanhos[i];
    };

    if (!painelConfigAberto || !configuracoesApp) return null;

    return (
        <>
            {/* Settings Modal Overlay */}
      
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
              
              <button 
                className={`settings-tab ${abaConfiguracao === 'Geral' ? 'active' : ''}`}
                onClick={() => setAbaConfiguracao('Geral')}
              >
                Geral
              </button>
              <button 
                className={`settings-tab ${abaConfiguracao === 'OCR' ? 'active' : ''}`}
                onClick={() => setAbaConfiguracao('OCR')}
              >
                OCR & Processamento
              </button>
              <button 
                className={`settings-tab ${abaConfiguracao === 'Desempenho' ? 'active' : ''}`}
                onClick={() => setAbaConfiguracao('Desempenho')}
              >
                Desempenho (Hardware)
              </button>
              <button
                className={`settings-tab ${abaConfiguracao === 'Atalhos' ? 'active' : ''}`}
                onClick={() => setAbaConfiguracao('Atalhos')}
              >
                Atalhos Globais
              </button>
              <button
                className={`settings-tab ${abaConfiguracao === 'Armazenamento' ? 'active' : ''}`}
                onClick={() => setAbaConfiguracao('Armazenamento')}
              >
                Armazenamento
              </button>
              <button
                className={`settings-tab ${abaConfiguracao === 'Estudando' ? 'active' : ''}`}
                onClick={() => setAbaConfiguracao('Estudando')}
              >
                Estudando
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
                
                {/* ---------- ABA: GERAL ---------- */}
                <div style={{ display: (abaConfiguracao === 'Geral' || termoBusca) ? 'block' : 'none' }}>
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

                  {(!termoBusca || "hover pop-up cursor tradução habilitar".includes(termoBusca.toLowerCase())) && (
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
                  )}

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
                    <div className="form-group" style={{ display: 'flex', alignItems: 'center', gap: '16px' }}>
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
                </div>

                {/* ---------- ABA: OCR & PROCESSAMENTO ---------- */}
                <div style={{ display: (abaConfiguracao === 'OCR' || termoBusca) ? 'block' : 'none' }}>
                  {termoBusca && <h3 className="settings-section-title" style={{marginTop: '32px'}}>OCR & Processamento</h3>}

                  {(!termoBusca || "modelo de ocr".includes(termoBusca.toLowerCase())) && (() => {
                    // Mostra apenas os modelos disponíveis (embutidos + já baixados). Os baixáveis
                    // ainda não instalados aparecem na seção "Gerenciar Modelos" abaixo.
                    const modelosDisponiveis = modelos.filter(m => m.embutido || m.instalado);
                    const atualDisponivel = modelosDisponiveis.some(m => m.nome === configuracoesApp.modeloOcr);
                    return (
                      <div className="form-group">
                        <label>Modelo de OCR</label>
                        <select
                          className="form-input"
                          value={configuracoesApp.modeloOcr}
                          onChange={e => trocarModelo(e.target.value)}
                        >
                          {/* Mantém o valor salvo visível mesmo que o modelo não esteja instalado
                              (sem rotular "indisponível" antes da lista carregar) */}
                          {!atualDisponivel && configuracoesApp.modeloOcr && (
                            <option value={configuracoesApp.modeloOcr}>
                              {modelos.length > 0 ? `${configuracoesApp.modeloOcr} (indisponível — baixe abaixo)` : configuracoesApp.modeloOcr}
                            </option>
                          )}
                          {modelosDisponiveis.map(m => (
                            <option key={m.nome} value={m.nome} title={m.descricao}>
                              {m.rotulo}
                            </option>
                          ))}
                        </select>
                        <small style={{ color: 'var(--cor-texto-suave)', display: 'block', marginTop: '6px' }}>
                          Outros modelos podem ser baixados em "Gerenciar Modelos", logo abaixo.
                        </small>
                      </div>
                    );
                  })()}

                  {(!termoBusca || "confiança mínima ocr".includes(termoBusca.toLowerCase())) && (
                    <div className="form-group">
                      <label>Confiança Mínima do OCR: {(configuracoesApp.confiancaMinimaOcr * 100).toFixed(0)}%</label>
                      <input 
                        type="range" 
                        min="0.1" max="1" step="0.05"
                        value={configuracoesApp.confiancaMinimaOcr}
                        onChange={e => AtualizarConfiguracao('confiancaMinimaOcr', parseFloat(e.target.value))}
                        style={{ width: '100%' }}
                      />
                    </div>
                  )}

                  {(!termoBusca || "qualidade da imagem ocr resolução captura".includes(termoBusca.toLowerCase())) && (() => {
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
                          Maior resolução = mais precisão, porém mais lento. Resolução nativa atual: {wNat} × {hNat}.
                        </small>
                      </div>
                    );
                  })()}

                  {(!termoBusca || "gerenciar modelos baixar download onnx".includes(termoBusca.toLowerCase())) && (
                    <div className="form-group">
                      <label>Gerenciar Modelos (Download Dinâmico)</label>
                      <small style={{ color: 'var(--cor-texto-suave)', display: 'block', marginBottom: '8px' }}>
                        Os modelos de OCR são baixados automaticamente quando você seleciona o motor correspondente. Aqui você pode baixá-los antecipadamente ou removê-los para liberar espaço.
                      </small>

                      {modelos.length === 0 && (
                        <div style={{ color: 'var(--cor-texto-suave)', fontSize: '12px' }}>Carregando modelos…</div>
                      )}

                      {modelos.map(m => {
                        const emDownload = baixandoModelo === m.nome;
                        const msg = progressoModelo[m.nome];
                        return (
                          <div key={m.nome} style={{
                            border: '1px solid var(--cor-borda)',
                            borderRadius: '8px',
                            padding: '12px',
                            marginBottom: '8px',
                            backgroundColor: 'var(--cor-fundo-cartao)'
                          }}>
                            <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', gap: '12px' }}>
                              <div style={{ flex: 1 }}>
                                <div style={{ fontWeight: 'bold', fontSize: '13px' }}>
                                  {m.rotulo}
                                  {m.embutido && <span style={{ marginLeft: '8px', fontSize: '10px', color: '#81c784' }}>EMBUTIDO</span>}
                                  {m.instalado && !m.embutido && <span style={{ marginLeft: '8px', fontSize: '10px', color: '#64b5f6' }}>INSTALADO{m.tamanhoBytes ? ` · ${FormatarTamanho(m.tamanhoBytes)}` : ''}</span>}
                                </div>
                                <div style={{ fontSize: '11px', color: 'var(--cor-texto-suave)', marginTop: '2px' }}>{m.descricao}</div>
                              </div>
                              <div style={{ display: 'flex', gap: '6px' }}>
                                {m.baixavel && !m.instalado && (
                                  <button
                                    className="scan-btn"
                                    style={{ padding: '4px 10px', fontSize: '11px', opacity: emDownload ? 0.6 : 1 }}
                                    disabled={emDownload || baixandoModelo !== null}
                                    onClick={() => BaixarModeloOcr(m.nome)}
                                  >
                                    {emDownload ? 'Baixando…' : '⬇️ Baixar'}
                                  </button>
                                )}
                                {m.baixavel && m.instalado && (
                                  <button
                                    className="scan-btn"
                                    style={{ padding: '4px 10px', fontSize: '11px', backgroundColor: '#f44336' }}
                                    onClick={() => RemoverModeloOcr(m.nome)}
                                  >
                                    🗑️ Remover
                                  </button>
                                )}
                              </div>
                            </div>
                            {msg && (
                              <div style={{ fontSize: '11px', color: 'var(--cor-texto-suave)', marginTop: '8px' }}>{msg}</div>
                            )}
                          </div>
                        );
                      })}
                    </div>
                  )}
                </div>

                {/* ---------- ABA: DESEMPENHO ---------- */}
                <div style={{ display: (abaConfiguracao === 'Desempenho' || termoBusca) ? 'block' : 'none' }}>
                  {termoBusca && <h3 className="settings-section-title" style={{marginTop: '32px'}}>Desempenho (Hardware)</h3>}

                  {(!termoBusca || "dispositivo hardware ocr cpu gpu nvidia amd intel".includes(termoBusca.toLowerCase())) && (
                    <div className="form-group">
                      <label>Hardware de Processamento OCR</label>
                      <select
                        className="form-input"
                        value={
                          configuracoesApp.hardwareSelecionado === 'CPU' ? (infoHardware?.cpu || 'CPU') :
                          configuracoesApp.hardwareSelecionado === 'GPU' ? (infoHardware?.gpus?.[0] || 'GPU') :
                          configuracoesApp.hardwareSelecionado
                        }
                        onChange={e => {
                          const val = e.target.value;
                          const mudancas: Partial<config.Config> = { hardwareSelecionado: val };

                          if (val === infoHardware?.cpu || val === 'CPU') {
                            mudancas.dispositivoOcr = 'cpu';
                          } else if (configuracoesApp.modeloOcr === 'EasyOCR (Download)') {
                            // EasyOCR só acelera por CUDA; opção GPU não-Nvidia fica desabilitada
                            mudancas.dispositivoOcr = 'cuda';
                          } else {
                            // RapidOCR: DirectML é universal no Windows
                            mudancas.dispositivoOcr = 'directml';
                          }

                          AplicarConfiguracao(mudancas);
                        }}
                      >
                        <option value={infoHardware?.cpu || 'CPU'} title="Compatível com todos os modelos de OCR.">{infoHardware?.cpu || 'Processador (CPU)'}</option>
                        {infoHardware?.gpus?.map(gpu => {
                          const incompat = !hardwareCompativelComModelo(configuracoesApp.modeloOcr, gpu);
                          return (
                            <option
                              key={gpu}
                              value={gpu}
                              disabled={incompat}
                              title={incompat
                                ? `Incompatível com ${rotuloModelo(configuracoesApp.modeloOcr)}, que em GPU exige uma placa Nvidia (CUDA). Troque o modelo ou use a CPU.`
                                : (ehNvidia(gpu) ? 'Suporta CUDA e DirectML.' : 'Suporta DirectML (não há CUDA fora da Nvidia).')}
                            >
                              {gpu}{incompat ? ` — incompatível com ${rotuloModelo(configuracoesApp.modeloOcr)}` : ''}
                            </option>
                          );
                        })}
                      </select>
                    </div>
                  )}

                  {(!termoBusca || "api processamento ocr cuda directml".includes(termoBusca.toLowerCase())) && 
                    configuracoesApp.hardwareSelecionado !== 'CPU' && 
                    configuracoesApp.hardwareSelecionado !== infoHardware?.cpu && (
                    <div className="form-group" style={{paddingLeft: '24px'}}>
                      <label>API de Processamento da Placa de Vídeo</label>
                      <select
                        className="form-input"
                        value={configuracoesApp.dispositivoOcr}
                        onChange={e => AtualizarConfiguracao('dispositivoOcr', e.target.value)}
                      >
                        <option
                          value="directml"
                          disabled={configuracoesApp.modeloOcr === 'EasyOCR (Download)'}
                          title={configuracoesApp.modeloOcr === 'EasyOCR (Download)'
                            ? 'O EasyOCR não suporta DirectML. Use CUDA (Nvidia) ou rode em CPU.'
                            : 'Aceleração universal no Windows — funciona em qualquer GPU (Nvidia, AMD, Intel).'}
                        >
                          DirectML (Padrão Windows / Universal)
                        </option>
                        <option
                          value="cuda"
                          disabled={!ehNvidia(configuracoesApp.hardwareSelecionado)}
                          title={ehNvidia(configuracoesApp.hardwareSelecionado)
                            ? 'Aceleração CUDA, exclusiva de placas Nvidia.'
                            : 'CUDA é exclusivo de placas Nvidia; a GPU selecionada não é Nvidia.'}
                        >
                          CUDA (Exclusivo Nvidia)
                        </option>
                      </select>
                      <small style={{ color: 'var(--cor-texto-suave)', display: 'block', marginTop: '6px' }}>
                        {configuracoesApp.modeloOcr === 'EasyOCR (Download)'
                          ? 'EasyOCR: aceleração apenas por CUDA (Nvidia).'
                          : 'RapidOCR: DirectML funciona em qualquer GPU; CUDA é exclusivo Nvidia.'}
                      </small>
                    </div>
                  )}

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

                  {(!termoBusca || "limitar uso máximo cpu".includes(termoBusca.toLowerCase())) && (
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
                      
                      {configuracoesApp.limitarPorUsoCpu && (
                        <div className="form-group" style={{ paddingLeft: '24px', display: 'flex', alignItems: 'center', gap: '16px' }}>
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
                    </>
                  )}

                  {(!termoBusca || "limitar uso máximo gpu".includes(termoBusca.toLowerCase())) && (
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
                      
                      {configuracoesApp.limitarPorUsoGpu && (
                        <div className="form-group" style={{ paddingLeft: '24px', display: 'flex', alignItems: 'center', gap: '16px' }}>
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
                    </>
                  )}
                </div>

                {/* ---------- ABA: ATALHOS ---------- */}
                <div style={{ display: (abaConfiguracao === 'Atalhos' || termoBusca) ? 'block' : 'none' }}>
                  {termoBusca && <h3 className="settings-section-title" style={{marginTop: '32px'}}>Atalhos Globais</h3>}

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
                </div>

                {/* ---------- ABA: ARMAZENAMENTO ---------- */}
                <div style={{ display: (abaConfiguracao === 'Armazenamento' || termoBusca) ? 'block' : 'none' }}>
                  {termoBusca && <h3 className="settings-section-title" style={{marginTop: '32px'}}>Armazenamento</h3>}

                  <div className="form-group">
                    <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
                      <label style={{ margin: 0 }}>Uso de Disco</label>
                      <button
                        className="scan-btn"
                        style={{ padding: '4px 10px', fontSize: '11px' }}
                        onClick={() => AbrirPastaDados()}
                      >
                        📂 Abrir pasta de dados
                      </button>
                    </div>

                    {infoArmazenamento && (
                      <div style={{ fontSize: '12px', color: 'var(--cor-texto-suave)', marginTop: '6px' }}>
                        App usa <strong>{FormatarTamanho(infoArmazenamento.totalBytes) || '0 MB'}</strong>
                        {infoArmazenamento.discoTotal > 0 && (
                          <> · Disco: <strong style={{ color: infoArmazenamento.discoLivre < 1024 * 1024 * 1024 ? '#f44336' : 'inherit' }}>
                            {FormatarTamanho(infoArmazenamento.discoLivre)} livres
                          </strong> de {FormatarTamanho(infoArmazenamento.discoTotal)}</>
                        )}
                      </div>
                    )}
                    {!infoArmazenamento && (
                      <div style={{ fontSize: '12px', color: 'var(--cor-texto-suave)', marginTop: '6px' }}>Calculando…</div>
                    )}
                  </div>

                  {infoArmazenamento?.itens.map(item => (
                    <div key={item.chave} style={{
                      border: '1px solid var(--cor-borda)',
                      borderRadius: '8px',
                      padding: '12px',
                      marginBottom: '8px',
                      backgroundColor: 'var(--cor-fundo-cartao)'
                    }}>
                      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', gap: '12px' }}>
                        <div style={{ flex: 1 }}>
                          <div style={{ fontWeight: 'bold', fontSize: '13px' }}>
                            {item.rotulo}
                            <span style={{ marginLeft: '8px', fontSize: '11px', color: 'var(--cor-destaque)' }}>
                              {FormatarTamanho(item.bytes) || '0 MB'}
                            </span>
                            {item.perigoso && <span style={{ marginLeft: '8px', fontSize: '10px', color: '#f44336', fontWeight: 'bold' }}>DADOS DO USUÁRIO</span>}
                          </div>
                          <div style={{ fontSize: '11px', color: 'var(--cor-texto-suave)', marginTop: '2px' }}>{item.descricao}</div>
                        </div>
                        <button
                          className="scan-btn"
                          style={{ padding: '4px 10px', fontSize: '11px', backgroundColor: item.perigoso ? '#f44336' : undefined, opacity: (armazenamentoOcupado || item.bytes === 0) ? 0.5 : 1 }}
                          disabled={armazenamentoOcupado || item.bytes === 0}
                          onClick={() => {
                            if (item.perigoso) {
                              setConfirmacao({
                                titulo: 'Apagar o vocabulário?',
                                mensagem: `Isso apaga TODAS as suas palavras (vistas, em estudo e aprendidas). Esta ação não pode ser desfeita.`,
                                rotuloAcao: 'Apagar vocabulário',
                                acao: () => LimparCategoriaArmazenamento(item.chave),
                              });
                            } else {
                              LimparCategoriaArmazenamento(item.chave);
                            }
                          }}
                        >
                          🗑️ Limpar
                        </button>
                      </div>
                    </div>
                  ))}

                  <div className="form-group" style={{ marginTop: '24px', borderTop: '1px solid var(--cor-borda)', paddingTop: '16px' }}>
                    <label style={{ color: '#f44336' }}>Zona de Perigo</label>
                    <small style={{ color: 'var(--cor-texto-suave)', display: 'block', marginBottom: '8px' }}>
                      Apaga todos os modelos baixados, o cache de instalação, os logs e zera o vocabulário. As suas preferências (configurações) são mantidas.
                    </small>
                    <button
                      className="scan-btn"
                      style={{ backgroundColor: '#f44336', opacity: armazenamentoOcupado ? 0.5 : 1 }}
                      disabled={armazenamentoOcupado}
                      onClick={() => setConfirmacao({
                        titulo: 'Excluir tudo?',
                        mensagem: 'Serão apagados: modelos de OCR baixados, modelos do EasyOCR, cache do pip, logs e TODO o vocabulário. As preferências serão mantidas. Esta ação não pode ser desfeita.',
                        rotuloAcao: 'Excluir tudo',
                        acao: () => ExcluirTodoArmazenamento(),
                      })}
                    >
                      🧹 Excluir Tudo
                    </button>
                  </div>
                </div>

                {/* ---------- ABA: ESTUDANDO ---------- */}
                <div style={{ display: (abaConfiguracao === 'Estudando' || termoBusca) ? 'block' : 'none' }}>
                  {termoBusca && <h3 className="settings-section-title" style={{marginTop: '32px'}}>Estudando</h3>}

                  {(!termoBusca || "estudando highlight azul destacar tela".includes(termoBusca.toLowerCase())) && (
                    <div className="form-group">
                      <label style={{ display: 'flex', alignItems: 'center', gap: '8px', justifyContent: 'space-between' }}>
                        <span>Destacar com um quadrado azul nativo os Hanzis recém-escaneados que já estão "Em Estudo"</span>
                        <input 
                          type="checkbox" 
                          checked={configuracoesApp.destacarEstudoTela}
                          onChange={e => AtualizarConfiguracao('destacarEstudoTela', e.target.checked)}
                        />
                      </label>
                      <small style={{ color: 'var(--cor-texto-suave)', display: 'block', marginTop: '6px', paddingLeft: '24px' }}>
                        Eles serão destacados na tela logo após o escaneamento caso você permaneça na aba de Descobrimento ou de Palavras dessa Seção.
                      </small>
                    </div>
                  )}
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
      </>
    );
}
