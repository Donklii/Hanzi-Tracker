// ----- Seção: Configurações -----
import { useState } from 'react';
import { config, main } from '../../wailsjs/go/models';
import { AbrirPastaDados } from '../../wailsjs/go/main/App';

// ----- Cores da barra de uso de armazenamento (uma por categoria; cicla se houver mais categorias) -----
const CORES_CATEGORIA_ARMAZENAMENTO = ['#64b5f6', '#81c784', '#ffb74d', '#ba68c8', '#f06292', '#4db6ac', '#a1887f'];

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
    infoCotaTraducao: main.InfoCotaTraducao | null;
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
}

export function PainelConfiguracoes(props: PainelConfiguracoesProps) {
    // Desestruturação para manter a compatibilidade interna do código antigo
    const {
        painelConfigAberto, setPainelConfigAberto, configuracoesApp, AtualizarConfiguracao, AplicarConfiguracao, setConfirmacao,
        abaConfiguracao, setAbaConfiguracao, termoBusca, setTermoBusca, infoHardware,
        resCaptura, monitores, modelos, progressoModelo, baixandoModelo, avisoCompatibilidade,
        infoArmazenamento, infoCotaTraducao, armazenamentoOcupado, BaixarModeloOcr, RemoverModeloOcr, trocarModelo,
        CarregarArmazenamento, LimparCategoriaArmazenamento, ExcluirTodoArmazenamento,
        hardwareEhCpu, ehCpuNome, ehNvidia, apiCompativelComModelo, hardwareCompativelComModelo, rotuloModelo,
        motores, progressoMotor, baixandoMotor, trocandoMotor, BaixarMotorOcr, RemoverMotorOcr, TrocarMotorOcr,
        motoresTts, progressoMotorTts, baixandoMotorTts, BaixarMotorVoz, RemoverMotorVoz
    } = props;


    const FormatarTamanho = (bytes: number): string => {
      if (bytes === 0) return '0 B';
      const k = 1024;
      const tamanhos = ['B', 'KB', 'MB', 'GB', 'TB'];
      const i = Math.floor(Math.log(bytes) / Math.log(k));
      return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + tamanhos[i];
    };

    // Motor de OCR ativo: os modelos vêm do /api/modelos do PROCESSO desse motor, então o catálogo
    // exibido abaixo é sempre o do motor em execução (troca de motor recarrega `modelos`, ver App.tsx).
    const motorAtivo = motores.find(m => m.ativo);

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
                className={`settings-tab ${abaConfiguracao === 'Tradução' ? 'active' : ''}`}
                onClick={() => setAbaConfiguracao('Tradução')}
              >
                Tradução (IA)
              </button>
              <button
                className={`settings-tab ${abaConfiguracao === 'Armazenamento' ? 'active' : ''}`}
                onClick={() => setAbaConfiguracao('Armazenamento')}
              >
                Armazenamento
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
                          <label>Motor de TTS</label>
                          <select
                            className="form-input"
                            value={configuracoesApp.motorTtsAtivo}
                            onChange={e => AtualizarConfiguracao('motorTtsAtivo', e.target.value)}
                          >
                            <option value="Kokoro-82M">Kokoro-82M</option>
                            <option value="ChatTTS">ChatTTS</option>
                          </select>
                          <small style={{ color: 'var(--cor-texto-suave)', display: 'block', marginTop: '6px' }}>
                            O motor selecionado precisa estar instalado (abaixo). A troca vale a partir da próxima leitura;
                            o modelo de voz é baixado automaticamente na primeira vez.
                          </small>
                        </div>

                        <div className="form-group">
                          <label>Gerenciar Motores de Voz</label>
                          <small style={{ color: 'var(--cor-texto-suave)', display: 'block', marginBottom: '8px' }}>
                            O motor é o programa que sintetiza a fala. Baixe o que quiser usar ou remova para liberar espaço.
                            Os pesos do modelo são baixados pelo próprio motor na primeira leitura em voz alta.
                          </small>

                          {motoresTts.length === 0 && (
                            <div style={{ color: 'var(--cor-texto-suave)', fontSize: '12px' }}>Carregando motores…</div>
                          )}

                          {motoresTts.map(m => {
                            const emDownload = baixandoMotorTts === m.nome;
                            const msg = progressoMotorTts[m.nome];
                            return (
                              <div key={m.nome} style={{
                                border: m.ativo ? '1px solid #64b5f6' : '1px solid var(--cor-borda)',
                                borderRadius: '8px',
                                padding: '12px',
                                marginBottom: '8px',
                                backgroundColor: 'var(--cor-fundo-cartao)'
                              }}>
                                <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', gap: '12px' }}>
                                  <div style={{ flex: 1 }}>
                                    <div style={{ fontWeight: 'bold', fontSize: '13px' }}>
                                      {m.rotulo}
                                      {m.ativo && <span style={{ marginLeft: '8px', fontSize: '10px', color: '#81c784' }}>● ATIVO</span>}
                                      {m.instalado && !m.ativo && <span style={{ marginLeft: '8px', fontSize: '10px', color: '#64b5f6' }}>INSTALADO{m.tamanhoBytes ? ` · ${FormatarTamanho(m.tamanhoBytes)}` : ''}</span>}
                                      {!m.instalado && m.tamanhoBytes ? <span style={{ marginLeft: '8px', fontSize: '10px', color: 'var(--cor-texto-suave)' }}>{FormatarTamanho(m.tamanhoBytes)}</span> : null}
                                    </div>
                                    <div style={{ fontSize: '11px', color: 'var(--cor-texto-suave)', marginTop: '2px' }}>{m.descricao}</div>
                                    {m.requisitos && (
                                      <div style={{ fontSize: '10px', color: 'var(--cor-texto-suave)', marginTop: '2px' }}>Requer: {m.requisitos}</div>
                                    )}
                                  </div>
                                  <div style={{ display: 'flex', gap: '6px' }}>
                                    {!m.instalado && (
                                      <button
                                        className="scan-btn"
                                        style={{ padding: '4px 10px', fontSize: '11px', opacity: (emDownload || !m.publicado) ? 0.6 : 1 }}
                                        disabled={emDownload || baixandoMotorTts !== null || !m.publicado}
                                        title={!m.publicado ? 'Este motor ainda não foi publicado — aguarde a próxima atualização.' : undefined}
                                        onClick={() => BaixarMotorVoz(m.nome)}
                                      >
                                        {emDownload ? 'Baixando…' : (m.publicado ? '⬇️ Baixar' : 'Indisponível')}
                                      </button>
                                    )}
                                    {m.instalado && (
                                      <button
                                        className="scan-btn"
                                        style={{ padding: '4px 10px', fontSize: '11px', backgroundColor: '#f44336' }}
                                        onClick={() => RemoverMotorVoz(m.nome)}
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
                      </SecaoDependente>
                    </>
                  )}

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

                  {(!termoBusca || "estudando highlight amarelo destacar tela parcial".includes(termoBusca.toLowerCase())) && (
                    <div className="form-group">
                      <label style={{ display: 'flex', alignItems: 'center', gap: '8px', justifyContent: 'space-between' }}>
                        <span>Destacar com um quadrado amarelo Hanzis que estão "Em Estudo" quando aparecerem dentro de outras palavras</span>
                        <input 
                          type="checkbox" 
                          checked={configuracoesApp.destacarEstudoParcialTela}
                          onChange={e => AtualizarConfiguracao('destacarEstudoParcialTela', e.target.checked)}
                        />
                      </label>
                      <small style={{ color: 'var(--cor-texto-suave)', display: 'block', marginTop: '6px', paddingLeft: '24px' }}>
                        Ex: se você está estudando o caractere "好", ele receberá um highlight amarelo dentro do card "你好".
                      </small>
                    </div>
                  )}
                </div>

                {/* ---------- ABA: OCR & PROCESSAMENTO ---------- */}
                <div style={{ display: (abaConfiguracao === 'OCR' || termoBusca) ? 'block' : 'none' }}>
                  {termoBusca && <h3 className="settings-section-title" style={{marginTop: '32px'}}>OCR & Processamento</h3>}

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
                          {motorAtivo
                            ? `Modelos compatíveis com o motor ativo: ${motorAtivo.rotulo}. Outros modelos podem ser baixados em "Gerenciar Modelos", logo abaixo.`
                            : 'Outros modelos podem ser baixados em "Gerenciar Modelos", logo abaixo.'}
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

                  {(!termoBusca || "gerenciar motores engine sidecar baixar download ativar".includes(termoBusca.toLowerCase())) && (
                    <div className="form-group">
                      <label>Gerenciar Motores de OCR</label>
                      <small style={{ color: 'var(--cor-texto-suave)', display: 'block', marginBottom: '8px' }}>
                        O motor é o programa que faz o reconhecimento. Baixe motores adicionais, ative qual usar ou remova para liberar espaço. Apenas um motor fica ativo por vez.
                      </small>
                      <small style={{ color: 'var(--cor-texto-suave)', display: 'block', marginBottom: '8px', fontStyle: 'italic' }}>
                        ℹ️ Os motores são baixados como executáveis e verificados por sha256 (o download só é aceito se o hash conferir). Alguns antivírus podem sinalizá-los por heurística — é um falso positivo comum de programas empacotados.
                      </small>

                      {motores.length === 0 && (
                        <div style={{ color: 'var(--cor-texto-suave)', fontSize: '12px' }}>Carregando motores…</div>
                      )}

                      {motores.map(m => {
                        const emDownload = baixandoMotor === m.nome;
                        const emTroca = trocandoMotor === m.nome;
                        const msg = progressoMotor[m.nome];
                        return (
                          <div key={m.nome} style={{
                            border: m.ativo ? '1px solid #64b5f6' : '1px solid var(--cor-borda)',
                            borderRadius: '8px',
                            padding: '12px',
                            marginBottom: '8px',
                            backgroundColor: 'var(--cor-fundo-cartao)'
                          }}>
                            <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', gap: '12px' }}>
                              <div style={{ flex: 1 }}>
                                <div style={{ fontWeight: 'bold', fontSize: '13px' }}>
                                  {m.rotulo}
                                  {m.ativo && <span style={{ marginLeft: '8px', fontSize: '10px', color: '#81c784' }}>● ATIVO</span>}
                                  {m.instalado && !m.ativo && <span style={{ marginLeft: '8px', fontSize: '10px', color: '#64b5f6' }}>INSTALADO{m.tamanhoBytes ? ` · ${FormatarTamanho(m.tamanhoBytes)}` : ''}</span>}
                                  {!m.instalado && m.tamanhoBytes ? <span style={{ marginLeft: '8px', fontSize: '10px', color: 'var(--cor-texto-suave)' }}>{FormatarTamanho(m.tamanhoBytes)}</span> : null}
                                </div>
                                <div style={{ fontSize: '11px', color: 'var(--cor-texto-suave)', marginTop: '2px' }}>{m.descricao}</div>
                                <div style={{ fontSize: '10px', color: 'var(--cor-texto-suave)', marginTop: '2px' }}>
                                  Aceleração: {m.variante}{m.requisitos ? ` · Requer: ${m.requisitos}` : ''}
                                </div>
                              </div>
                              <div style={{ display: 'flex', gap: '6px' }}>
                                {!m.instalado && (
                                  <button
                                    className="scan-btn"
                                    style={{ padding: '4px 10px', fontSize: '11px', opacity: emDownload ? 0.6 : 1 }}
                                    disabled={emDownload || baixandoMotor !== null}
                                    onClick={() => {
                                      // Motores grandes (ou com requisito de hardware) pedem confirmação
                                      // antes do download pesado; os leves baixam direto.
                                      const pesado = m.tamanhoBytes >= 400 * 1024 * 1024 || !!m.requisitos;
                                      if (pesado) {
                                        setConfirmacao({
                                          titulo: `Baixar ${m.rotulo}?`,
                                          mensagem: `Este motor ocupa ${FormatarTamanho(m.tamanhoBytes)}${m.requisitos ? ` e requer: ${m.requisitos}` : ''}. O download pode demorar bastante.`,
                                          rotuloAcao: 'Baixar motor',
                                          acao: () => BaixarMotorOcr(m.nome),
                                        });
                                      } else {
                                        BaixarMotorOcr(m.nome);
                                      }
                                    }}
                                  >
                                    {emDownload ? 'Baixando…' : '⬇️ Baixar'}
                                  </button>
                                )}
                                {m.instalado && !m.ativo && (
                                  <button
                                    className="scan-btn"
                                    style={{ padding: '4px 10px', fontSize: '11px', opacity: emTroca ? 0.6 : 1 }}
                                    disabled={emTroca || trocandoMotor !== null}
                                    onClick={() => TrocarMotorOcr(m.nome)}
                                  >
                                    {emTroca ? 'Ativando…' : '✓ Ativar'}
                                  </button>
                                )}
                                {m.instalado && !m.ativo && (
                                  <button
                                    className="scan-btn"
                                    style={{ padding: '4px 10px', fontSize: '11px', backgroundColor: '#f44336' }}
                                    onClick={() => RemoverMotorOcr(m.nome)}
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

                  {(!termoBusca || "gerenciar modelos baixar download onnx".includes(termoBusca.toLowerCase())) && (
                    <div className="form-group">
                      <label>
                        Gerenciar Modelos{motorAtivo ? ` — motor ativo: ${motorAtivo.rotulo}` : ''}
                      </label>
                      <small style={{ color: 'var(--cor-texto-suave)', display: 'block', marginBottom: '8px' }}>
                        Modelos compatíveis com o motor de OCR ativo. Baixe-os antecipadamente ou remova-os para liberar espaço; ao trocar de motor em "Gerenciar Motores", esta lista é atualizada automaticamente.
                      </small>

                      {!motorAtivo && (
                        <div style={{ color: 'var(--cor-texto-suave)', fontSize: '12px' }}>
                          Nenhum motor de OCR ativo no momento — ative um em "Gerenciar Motores", acima, para ver os modelos compatíveis.
                        </div>
                      )}

                      {motorAtivo && modelos.length === 0 && (
                        <div style={{ color: 'var(--cor-texto-suave)', fontSize: '12px' }}>Carregando modelos…</div>
                      )}

                      {motorAtivo && modelos.map(m => {
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

                  {(!termoBusca || "dispositivo hardware ocr cpu gpu nvidia amd intel api processamento cuda directml".includes(termoBusca.toLowerCase())) && (
                    <>
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

                      <SecaoDependente ativa={configuracoesApp.hardwareSelecionado !== 'CPU' && configuracoesApp.hardwareSelecionado !== infoHardware?.cpu}>
                        {(!termoBusca || "api processamento ocr cuda directml".includes(termoBusca.toLowerCase())) && (
                          <div className="form-group" style={{ margin: 0 }}>
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
                      </SecaoDependente>
                    </>
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

                {/* ---------- ABA: TRADUÇÃO ---------- */}
                <div style={{ display: (abaConfiguracao === 'Tradução' || termoBusca) ? 'block' : 'none' }}>
                  {termoBusca && <h3 className="settings-section-title" style={{marginTop: '32px'}}>Tradução (IA)</h3>}

                  {(!termoBusca || "tradução api key google cloud".includes(termoBusca.toLowerCase())) && (
                    <div className="form-group">
                      <label>Google Cloud Translation API Key</label>
                      <input 
                        type="password"
                        className="form-input" 
                        value={configuracoesApp.traducaoApiKey}
                        onChange={e => AtualizarConfiguracao('traducaoApiKey', e.target.value)}
                        placeholder="Cole sua API Key aqui..."
                      />
                      <small style={{ color: 'var(--cor-texto-suave)', display: 'block', marginTop: '6px' }}>
                        Requer uma API key própria do Google Cloud Platform (GCP). Cota gratuita: 500.000 caracteres/mês. 
                        <strong>Aviso:</strong> o Google exige cartão cadastrado no GCP mesmo para usar apenas a cota gratuita.
                      </small>
                    </div>
                  )}

                  {(!termoBusca || "habilitar tradução de linha cota mensal uso limite guardar cache pausar".includes(termoBusca.toLowerCase())) && (
                    <>
                      <div className="form-group" style={{ marginTop: '16px' }}>
                        <label style={{ display: 'flex', alignItems: 'center', gap: '8px', justifyContent: 'space-between' }}>
                          <span>Habilitar tradução por linha (Atalho de Pop-up de Tudo)</span>
                          <input 
                            type="checkbox" 
                            checked={configuracoesApp.traducaoAtiva}
                            onChange={e => AtualizarConfiguracao('traducaoAtiva', e.target.checked)}
                          />
                        </label>
                      </div>

                      <SecaoDependente ativa={configuracoesApp.traducaoAtiva}>
                        {(!termoBusca || "pausar traduções limite cota mensal".includes(termoBusca.toLowerCase())) && (
                          <div className="form-group">
                            <label style={{ display: 'flex', alignItems: 'center', gap: '8px', justifyContent: 'space-between' }}>
                              <span>Pausar traduções ao atingir limite da cota gratuita mensal</span>
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
                              <label style={{ margin: 0, flex: 1 }}>Limite de Cota Mensal</label>
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
                              <span>Guardar traduções já feitas para não gastar cota na mesma linha</span>
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
                              <span>Uso da Cota (Mês {infoCotaTraducao.anoMes})</span>
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
                              {infoCotaTraducao.caracteresUsados.toLocaleString('pt-BR')} / {infoCotaTraducao.cotaTotal.toLocaleString('pt-BR')} caracteres
                            </div>
                          </div>
                        )}
                      </SecaoDependente>
                    </>
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

                    {infoArmazenamento && infoArmazenamento.totalBytes > 0 && (() => {
                      // Barra empilhada: cada categoria com uso ocupa sua fração do total do app.
                      const categorias = infoArmazenamento.itens.filter(it => it.bytes > 0);
                      return (
                        <div style={{ marginTop: '10px' }}>
                          <div style={{ display: 'flex', height: '10px', borderRadius: '5px', overflow: 'hidden', backgroundColor: 'var(--cor-borda)' }}>
                            {categorias.map((it, idx) => (
                              <div
                                key={it.chave}
                                title={`${it.rotulo}: ${FormatarTamanho(it.bytes)}`}
                                style={{
                                  width: `${(it.bytes / infoArmazenamento.totalBytes) * 100}%`,
                                  backgroundColor: CORES_CATEGORIA_ARMAZENAMENTO[idx % CORES_CATEGORIA_ARMAZENAMENTO.length],
                                }}
                              />
                            ))}
                          </div>
                          <div style={{ display: 'flex', flexWrap: 'wrap', gap: '6px 14px', marginTop: '8px' }}>
                            {categorias.map((it, idx) => (
                              <div key={it.chave} style={{ display: 'flex', alignItems: 'center', gap: '5px', fontSize: '11px', color: 'var(--cor-texto-suave)' }}>
                                <span style={{ width: '10px', height: '10px', borderRadius: '2px', display: 'inline-block', backgroundColor: CORES_CATEGORIA_ARMAZENAMENTO[idx % CORES_CATEGORIA_ARMAZENAMENTO.length] }} />
                                {it.rotulo} · {FormatarTamanho(it.bytes)}
                              </div>
                            ))}
                          </div>
                        </div>
                      );
                    })()}
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

// ----- Helpers -----

interface SecaoDependenteProps {
    ativa: boolean;
    children: React.ReactNode;
}

function SecaoDependente({ ativa, children }: SecaoDependenteProps) {
    if (!ativa) {
        return null;
    }

    return (
        <div style={{ paddingLeft: '24px', marginTop: '12px', marginBottom: '24px', borderLeft: '2px solid var(--cor-borda)' }}>
            {children}
        </div>
    );
}
