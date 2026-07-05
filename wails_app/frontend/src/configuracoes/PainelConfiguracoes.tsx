// ----- Seção: Configurações -----
import { useState } from 'react';
import { config, main } from '../../wailsjs/go/models';
import { AbrirPastaDados } from '../../wailsjs/go/main/App';
import { BrowserOpenURL } from '../../wailsjs/runtime/runtime';
import iconeGithub from '../assets/images/GithubIcon.png';

// ----- Cores da barra de uso de armazenamento (uma por categoria; cicla se houver mais categorias) -----
const CORES_CATEGORIA_ARMAZENAMENTO = ['#64b5f6', '#81c784', '#ffb74d', '#ba68c8', '#f06292', '#4db6ac', '#a1887f'];

// Andamento do pré-carregamento em lote do cache de áudio (espelha main.ProgressoPreCacheTts do Go).
interface ProgressoPreCacheTts {
    total: number;
    processados: number;
    sintetizados: number;
    jaEmCache: number;
    falhas: number;
    emAndamento: boolean;
    mensagem: string;
}

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
    infoCotaGemini: main.InfoCotaGemini | null;
    armazenamentoOcupado: boolean;
    BaixarModeloOcr: (nome: string) => void;
    RemoverModeloOcr: (nome: string) => void;
    trocarModelo: (nome: string) => void;
    CarregarArmazenamento: () => void;
    LimparCategoriaArmazenamento: (chave: string) => void;
    ExcluirTodoArmazenamento: () => void;
    ehCpuNome: (hw: string) => boolean;
    ehNvidia: (hw: string) => boolean;
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
    // Desestruturação para manter a compatibilidade interna do código antigo
    const {
        painelConfigAberto, setPainelConfigAberto, configuracoesApp, AtualizarConfiguracao, AplicarConfiguracao, setConfirmacao,
        abaConfiguracao, setAbaConfiguracao, termoBusca, setTermoBusca, infoHardware,
        resCaptura, monitores, modelos, progressoModelo, baixandoModelo, avisoCompatibilidade,
        infoArmazenamento, infoCotaTraducao, infoCotaGemini, armazenamentoOcupado, BaixarModeloOcr, RemoverModeloOcr, trocarModelo,
        CarregarArmazenamento, LimparCategoriaArmazenamento, ExcluirTodoArmazenamento,
        ehCpuNome, ehNvidia,
        motores, progressoMotor, baixandoMotor, trocandoMotor, BaixarMotorOcr, RemoverMotorOcr, TrocarMotorOcr,
        motoresTts, progressoMotorTts, baixandoMotorTts, BaixarMotorVoz, RemoverMotorVoz,
        progressoPreCacheTts, PreCarregarAudioTts, PararPreCarregarAudioTts
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

    // Acelerações suportadas pelo motor ativo (derivadas da `variante`: "CPU/DirectML", "CPU", …).
    // O hardware de processamento depende disto: motor só-CPU não oferece GPU; DirectML vale em qualquer
    // GPU; CUDA só em Nvidia.
    const varianteMotor = (motorAtivo?.variante || 'CPU').toLowerCase();
    const motorSuportaDml = varianteMotor.includes('directml');
    const motorSuportaCuda = varianteMotor.includes('cuda');
    const motorSoCpu = !motorSuportaDml && !motorSuportaCuda;
    const nomeCpu = infoHardware?.cpu || 'CPU';
    const hardwareEhCpu = ehCpuNome(configuracoesApp?.hardwareSelecionado || 'CPU');
    // Uma GPU é utilizável pelo motor ativo se ele acelera por DirectML (qualquer GPU) ou por CUDA numa Nvidia.
    const gpuUtilizavel = (gpu: string) => motorSuportaDml || (motorSuportaCuda && ehNvidia(gpu));

    // Cor do feedback por status: verde (sucesso), vermelho (erro), neutro (em andamento).
    const corProgresso = (msg: string) =>
        msg.startsWith('✅') ? '#81c784' : msg.startsWith('⚠️') ? '#f44336' : 'var(--cor-texto-suave)';

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
                className={`settings-tab ${abaConfiguracao === 'Motores' ? 'active' : ''}`}
                onClick={() => setAbaConfiguracao('Motores')}
              >
                Motores
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
                className={`settings-tab ${abaConfiguracao === 'Estudo' ? 'active' : ''}`}
                onClick={() => setAbaConfiguracao('Estudo')}
              >
                Estudo
              </button>
              <button
                className={`settings-tab ${abaConfiguracao === 'Armazenamento' ? 'active' : ''}`}
                onClick={() => setAbaConfiguracao('Armazenamento')}
              >
                Armazenamento
              </button>
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

                </div>

                {/* ---------- ABA: ESTUDO ---------- */}
                <div style={{ display: (abaConfiguracao === 'Estudo' || termoBusca) ? 'block' : 'none' }}>
                  {termoBusca && <h3 className="settings-section-title" style={{marginTop: '32px'}}>Estudo</h3>}

                  {(!termoBusca || "revisão priorizar caracteres em estudo hanzi sorteio".includes(termoBusca.toLowerCase())) && (
                    <div className="form-group">
                      <label style={{ display: 'flex', alignItems: 'center', gap: '8px', justifyContent: 'space-between' }}>
                        <span>Priorizar caracteres em estudo nas revisões</span>
                        <input
                          type="checkbox"
                          checked={configuracoesApp.priorizarEstudoRevisao}
                          onChange={e => AtualizarConfiguracao('priorizarEstudoRevisao', e.target.checked)}
                        />
                      </label>
                      <small style={{ color: 'var(--cor-texto-suave)', display: 'block', marginTop: '6px', paddingLeft: '24px' }}>
                        As sessões de revisão sorteiam primeiro os hanzis marcados como "Estudando". Quando
                        houver poucos em estudo, o restante vem aleatoriamente do dicionário para evitar
                        repetições.
                      </small>
                    </div>
                  )}

                  {(!termoBusca || "revisão sons efeitos sonoros acerto erro jingle".includes(termoBusca.toLowerCase())) && (
                    <div className="form-group">
                      <label style={{ display: 'flex', alignItems: 'center', gap: '8px', justifyContent: 'space-between' }}>
                        <span>Sons de acerto e erro nas revisões</span>
                        <input
                          type="checkbox"
                          checked={configuracoesApp.sonsRevisao}
                          onChange={e => AtualizarConfiguracao('sonsRevisao', e.target.checked)}
                        />
                      </label>
                      <small style={{ color: 'var(--cor-texto-suave)', display: 'block', marginTop: '6px', paddingLeft: '24px' }}>
                        Jingles curtos de feedback ao responder (acerto, erro, sequência e fim de sessão),
                        gerados pelo próprio app — não dependem do motor de voz.
                      </small>
                    </div>
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
                  {(!termoBusca || "hanzi tradicional simplificado ambos tipo exibir cards".includes(termoBusca.toLowerCase())) && (
                    <div className="form-group">
                      <label style={{ display: 'flex', alignItems: 'center', gap: '8px', justifyContent: 'space-between' }}>
                        <span>Tipo de Hanzi exibido nas listas e revisão</span>
                        <ToggleOpcoes 
                          opcoes={[
                            { valor: 'ambos', rotulo: 'Ambos' },
                            { valor: 'tradicional', rotulo: 'Tradicional' },
                            { valor: 'simplificado', rotulo: 'Simplificado' }
                          ]}
                          valor={configuracoesApp.tipoHanziExibicao || 'simplificado'}
                          onChange={v => AtualizarConfiguracao('tipoHanziExibicao', v)}
                        />
                      </label>
                      <small style={{ color: 'var(--cor-texto-suave)', display: 'block', marginTop: '6px', paddingLeft: '24px' }}>
                        Filtra as abas de Descobrimento, Estudos e Revisão para mostrar apenas o tipo desejado (A busca global ignora este filtro).
                      </small>
                    </div>
                  )}

                  <SecaoDependente ativa={configuracoesApp.tipoHanziExibicao === 'ambos'}>
                    {(!termoBusca || "hanzi tradicional simplificado ambos tipo gerar cards".includes(termoBusca.toLowerCase())) && (
                      <div className="form-group">
                        <label style={{ display: 'flex', alignItems: 'center', gap: '8px', justifyContent: 'space-between' }}>
                          <span>Tipo de Hanzi gerado pelo OCR</span>
                          <ToggleOpcoes 
                            opcoes={[
                              { valor: 'ambos', rotulo: 'Ambos' },
                              { valor: 'tradicional', rotulo: 'Tradicional' },
                              { valor: 'simplificado', rotulo: 'Simplificado' }
                            ]}
                            valor={configuracoesApp.tipoHanziGerado || 'ambos'}
                            onChange={v => AtualizarConfiguracao('tipoHanziGerado', v)}
                          />
                        </label>
                        <small style={{ color: 'var(--cor-texto-suave)', display: 'block', marginTop: '6px', paddingLeft: '24px' }}>
                          Quando o OCR detectar texto, os cards gerados serão convertidos para o tipo escolhido caso a palavra possua a respectiva versão.
                        </small>
                      </div>
                    )}
                  </SecaoDependente>

                  <SecaoDependente ativa={configuracoesApp.tipoHanziExibicao !== 'ambos'}>
                    {(!termoBusca || "restringir busca pesquisa desenho hanzi".includes(termoBusca.toLowerCase())) && (
                      <div className="form-group">
                        <label style={{ display: 'flex', alignItems: 'center', gap: '8px', justifyContent: 'space-between' }}>
                          <span>Aplicar restrição de tipo na pesquisa por desenho</span>
                          <input
                            type="checkbox"
                            checked={configuracoesApp.restringirHanziDesenho ?? true}
                            onChange={e => AtualizarConfiguracao('restringirHanziDesenho', e.target.checked)}
                          />
                        </label>
                        <small style={{ color: 'var(--cor-texto-suave)', display: 'block', marginTop: '6px', paddingLeft: '24px' }}>
                          Ao desenhar um Hanzi para pesquisar, exibir apenas resultados do tipo selecionado acima.
                        </small>
                      </div>
                    )}
                  </SecaoDependente>
                </div>

                {/* ---------- ABA: MOTORES ---------- */}
                <div style={{ display: (abaConfiguracao === 'Motores' || termoBusca) ? 'block' : 'none' }}>
                  {termoBusca && <h3 className="settings-section-title" style={{marginTop: '32px'}}>Motores (OCR & TTS)</h3>}

                  <h4 style={{ marginBottom: '16px', color: 'var(--cor-destaque)' }}>Reconhecimento de Texto (OCR)</h4>

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

                  {/* ----- Hardware de Processamento: depende do motor de OCR ativo (variante) ----- */}
                  {(!termoBusca || "hardware dispositivo processamento ocr cpu gpu nvidia amd intel api cuda directml aceleração".includes(termoBusca.toLowerCase())) && (
                    <div className="form-group">
                      <label>Hardware de Processamento{motorAtivo ? ` — ${motorAtivo.rotulo}` : ''}</label>

                      {motorSoCpu ? (
                        <>
                          <input className="form-input" value={nomeCpu} disabled readOnly />
                          <small style={{ color: 'var(--cor-texto-suave)', display: 'block', marginTop: '6px' }}>
                            {motorAtivo ? `${motorAtivo.rotulo} roda apenas em CPU — não há opção de GPU para este motor.` : 'Este motor roda apenas em CPU.'}
                          </small>
                        </>
                      ) : (
                        <>
                          <select
                            className="form-input"
                            value={hardwareEhCpu ? nomeCpu : configuracoesApp.hardwareSelecionado}
                            onChange={e => {
                              const val = e.target.value;
                              const mudancas: Partial<config.Config> = { hardwareSelecionado: val };
                              if (val === nomeCpu) {
                                mudancas.dispositivoOcr = 'cpu';
                              } else {
                                // GPU: usa a API padrão suportada pelo motor (DirectML se houver, senão CUDA).
                                mudancas.dispositivoOcr = motorSuportaDml ? 'directml' : 'cuda';
                              }
                              AplicarConfiguracao(mudancas);
                            }}
                          >
                            <option value={nomeCpu} title="Compatível com todos os motores de OCR.">{nomeCpu} (CPU)</option>
                            {infoHardware?.gpus?.map(gpu => {
                              const usavel = gpuUtilizavel(gpu);
                              return (
                                <option
                                  key={gpu}
                                  value={gpu}
                                  disabled={!usavel}
                                  title={usavel
                                    ? (ehNvidia(gpu) ? 'Suporta DirectML e CUDA.' : 'Suporta DirectML.')
                                    : `${motorAtivo?.rotulo || 'Este motor'} não acelera nesta GPU (em GPU exige CUDA, exclusivo Nvidia). Use a CPU ou uma placa Nvidia.`}
                                >
                                  {gpu}{usavel ? '' : ' — incompatível'}
                                </option>
                              );
                            })}
                          </select>

                          {!hardwareEhCpu && (
                            <div style={{ paddingLeft: '24px', marginTop: '12px', borderLeft: '2px solid var(--cor-borda)' }}>
                              <label style={{ fontSize: '12px' }}>API de Processamento da Placa de Vídeo</label>
                              <select
                                className="form-input"
                                value={configuracoesApp.dispositivoOcr}
                                onChange={e => AtualizarConfiguracao('dispositivoOcr', e.target.value)}
                              >
                                <option
                                  value="directml"
                                  disabled={!motorSuportaDml}
                                  title={motorSuportaDml ? 'Aceleração universal no Windows — funciona em qualquer GPU (Nvidia, AMD, Intel).' : 'O motor ativo não suporta DirectML.'}
                                >
                                  DirectML (Padrão Windows / Universal)
                                </option>
                                <option
                                  value="cuda"
                                  disabled={!(motorSuportaCuda && ehNvidia(configuracoesApp.hardwareSelecionado))}
                                  title={motorSuportaCuda
                                    ? (ehNvidia(configuracoesApp.hardwareSelecionado) ? 'Aceleração CUDA, exclusiva de placas Nvidia.' : 'CUDA é exclusivo de placas Nvidia; a GPU selecionada não é Nvidia.')
                                    : 'O motor ativo não suporta CUDA.'}
                                >
                                  CUDA (Exclusivo Nvidia)
                                </option>
                              </select>
                              <small style={{ color: 'var(--cor-texto-suave)', display: 'block', marginTop: '6px' }}>
                                {motorSuportaDml && motorSuportaCuda
                                  ? 'DirectML funciona em qualquer GPU; CUDA é exclusivo Nvidia.'
                                  : motorSuportaDml
                                  ? 'Este motor acelera por DirectML (funciona em qualquer GPU).'
                                  : 'Este motor acelera apenas por CUDA (Nvidia).'}
                              </small>
                            </div>
                          )}
                        </>
                      )}
                    </div>
                  )}

                  {/* ----- Motor de OCR (engine) e Modelo de OCR agrupados em painel retrátil ----- */}
                  {(!termoBusca || "motor de ocr gerenciar motores engine sidecar baixar download ativar trocar modelo modelos onnx".includes(termoBusca.toLowerCase())) && (
                    <details open={!!termoBusca} style={{ textAlign: 'left', marginBottom: '16px', border: '1px solid var(--cor-borda)', borderRadius: '8px', padding: '12px', backgroundColor: 'var(--cor-fundo-cartao)' }}>
                      <summary style={{ cursor: 'pointer', fontWeight: 'bold' }}>
                        Motores e Modelos de OCR
                      </summary>
                      <div style={{ marginTop: '16px' }}>

                  {/* ----- Motor de OCR (engine): escolha primária. Seleção estilo rádio + feedback colorido ----- */}
                  {(!termoBusca || "motor de ocr gerenciar motores engine sidecar baixar download ativar trocar".includes(termoBusca.toLowerCase())) && (
                    <div className="form-group">
                      <div style={{ display: 'flex', alignItems: 'center', gap: '6px' }}>
                        <label style={{ margin: 0 }}>Motor de OCR</label>
                        <span
                          title="Os motores são baixados como executáveis e verificados por sha256 (o download só é aceito se o hash conferir). Alguns antivírus podem sinalizá-los por heurística — é um falso positivo comum de programas empacotados."
                          style={{ cursor: 'help', fontSize: '12px' }}
                        >
                          ℹ️
                        </span>
                      </div>
                      <small style={{ color: 'var(--cor-texto-suave)', display: 'block', margin: '4px 0 8px' }}>
                        Clique num motor instalado para ativá-lo — apenas um fica ativo por vez.
                      </small>

                      {motores.length === 0 && (
                        <div style={{ color: 'var(--cor-texto-suave)', fontSize: '12px' }}>Carregando motores…</div>
                      )}

                      {motores.map(m => {
                        const emDownload = baixandoMotor === m.nome;
                        const emTroca = trocandoMotor === m.nome;
                        const msg = progressoMotor[m.nome];
                        const ocupado = baixandoMotor !== null || trocandoMotor !== null;
                        const podeAtivar = m.instalado && !m.ativo && !ocupado;
                        return (
                          <div
                            key={m.nome}
                            onClick={() => podeAtivar && TrocarMotorOcr(m.nome)}
                            title={podeAtivar ? `Ativar ${m.rotulo}` : undefined}
                            style={{
                              border: m.ativo ? '2px solid var(--cor-destaque)' : '1px solid var(--cor-borda)',
                              borderRadius: '8px',
                              padding: '12px',
                              marginBottom: '8px',
                              backgroundColor: 'var(--cor-fundo-cartao)',
                              cursor: podeAtivar ? 'pointer' : 'default',
                              opacity: emTroca ? 0.7 : 1,
                              transition: 'border-color 0.15s ease, opacity 0.15s ease',
                            }}
                          >
                            <div style={{ display: 'flex', alignItems: 'center', gap: '12px' }}>
                              {/* Indicador de seleção (rádio): preenchido no ativo, contornado nos instalados,
                                  tracejado nos ainda não baixados. */}
                              <span style={{
                                width: '18px', height: '18px', flexShrink: 0, borderRadius: '50%', boxSizing: 'border-box',
                                border: m.ativo
                                  ? '5px solid var(--cor-destaque)'
                                  : m.instalado
                                  ? '2px solid var(--cor-texto-suave)'
                                  : '2px dashed var(--cor-borda)',
                              }} />
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
                              <div style={{ display: 'flex', gap: '6px' }} onClick={e => e.stopPropagation()}>
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
                                {podeAtivar && (
                                  <button
                                    className="scan-btn"
                                    style={{ padding: '4px 10px', fontSize: '11px' }}
                                    onClick={() => TrocarMotorOcr(m.nome)}
                                  >
                                    ✓ Ativar
                                  </button>
                                )}
                                {m.instalado && !m.ativo && (
                                  <button
                                    className="scan-btn"
                                    style={{ padding: '4px 10px', fontSize: '11px', backgroundColor: '#f44336', opacity: ocupado ? 0.6 : 1 }}
                                    disabled={ocupado}
                                    onClick={() => RemoverMotorOcr(m.nome)}
                                  >
                                    🗑️ Remover
                                  </button>
                                )}
                              </div>
                            </div>
                            {(emTroca || msg) && (
                              <div style={{ fontSize: '11px', color: emTroca ? 'var(--cor-texto-suave)' : corProgresso(msg), marginTop: '8px', paddingLeft: '30px' }}>
                                {emTroca ? 'Ativando motor…' : msg}
                              </div>
                            )}
                          </div>
                        );
                      })}
                    </div>
                  )}

                  {/* ----- Modelo de OCR: seleção + download/remoção num único bloco (sem duplicar a lista de instalados) ----- */}
                  {(!termoBusca || "modelo de ocr gerenciar modelos baixar download onnx".includes(termoBusca.toLowerCase())) && (() => {
                    // O seletor só lista o que já pode ser usado (embutidos + baixados); modelos ainda
                    // não baixados aparecem como itens compactos logo abaixo, com botão de baixar.
                    const modelosDisponiveis = modelos.filter(m => m.embutido || m.instalado);
                    const modelosParaBaixar = modelos.filter(m => m.baixavel && !m.instalado);
                    const atualDisponivel = modelosDisponiveis.some(m => m.nome === configuracoesApp.modeloOcr);
                    const modeloAtualInfo = modelos.find(m => m.nome === configuracoesApp.modeloOcr);

                    return (
                      <div className="form-group">
                        <label>Modelo de OCR{motorAtivo ? ` — ${motorAtivo.rotulo}` : ''}</label>

                        {!motorAtivo && (
                          <div style={{ color: 'var(--cor-texto-suave)', fontSize: '12px' }}>
                            Nenhum motor de OCR ativo — ative um em "Motor de OCR", acima.
                          </div>
                        )}

                        {motorAtivo && modelos.length === 0 && (
                          <div style={{ color: 'var(--cor-texto-suave)', fontSize: '12px' }}>Carregando modelos…</div>
                        )}

                        {motorAtivo && modelos.length > 0 && (
                          <>
                            <div style={{ display: 'flex', gap: '8px' }}>
                              <select
                                className="form-input"
                                style={{ flex: 1 }}
                                value={configuracoesApp.modeloOcr}
                                onChange={e => trocarModelo(e.target.value)}
                              >
                                {/* Mantém o valor salvo visível mesmo que o modelo não esteja instalado
                                    (sem rotular "indisponível" antes da lista carregar) */}
                                {!atualDisponivel && configuracoesApp.modeloOcr && (
                                  <option value={configuracoesApp.modeloOcr}>
                                    {configuracoesApp.modeloOcr} (indisponível — baixe abaixo)
                                  </option>
                                )}
                                {modelosDisponiveis.map(m => (
                                  <option key={m.nome} value={m.nome} title={m.descricao}>
                                    {m.rotulo}{m.embutido ? ' (embutido)' : ''}
                                  </option>
                                ))}
                              </select>
                              {modeloAtualInfo?.baixavel && modeloAtualInfo.instalado && (
                                <button
                                  className="scan-btn"
                                  style={{ padding: '8px 10px', fontSize: '11px', backgroundColor: '#f44336' }}
                                  title="Remover o modelo selecionado"
                                  onClick={() => RemoverModeloOcr(modeloAtualInfo.nome)}
                                >
                                  🗑️
                                </button>
                              )}
                            </div>
                            {progressoModelo[configuracoesApp.modeloOcr] && (
                              <div style={{ fontSize: '11px', color: corProgresso(progressoModelo[configuracoesApp.modeloOcr]), marginTop: '6px' }}>
                                {progressoModelo[configuracoesApp.modeloOcr]}
                              </div>
                            )}

                            {modelosParaBaixar.length > 0 && (
                              <div style={{ marginTop: '10px' }}>
                                <small style={{ color: 'var(--cor-texto-suave)', display: 'block', marginBottom: '6px' }}>
                                  Outros modelos compatíveis com {motorAtivo.rotulo}, disponíveis para baixar:
                                </small>
                                {modelosParaBaixar.map(m => {
                                  const emDownload = baixandoModelo === m.nome;
                                  const msg = progressoModelo[m.nome];
                                  return (
                                    <div key={m.nome} style={{
                                      display: 'flex', justifyContent: 'space-between', alignItems: 'center', gap: '10px',
                                      padding: '8px 10px', border: '1px solid var(--cor-borda)', borderRadius: '6px', marginTop: '6px',
                                    }}>
                                      <div style={{ flex: 1 }} title={m.descricao}>
                                        <div style={{ fontSize: '12px', fontWeight: 'bold' }}>
                                          {m.rotulo}{m.tamanhoBytes ? <span style={{ fontWeight: 'normal', color: 'var(--cor-texto-suave)' }}> · {FormatarTamanho(m.tamanhoBytes)}</span> : null}
                                        </div>
                                        {msg && <div style={{ fontSize: '10px', color: corProgresso(msg), marginTop: '2px' }}>{msg}</div>}
                                      </div>
                                      <button
                                        className="scan-btn"
                                        style={{ padding: '4px 10px', fontSize: '11px', opacity: emDownload ? 0.6 : 1 }}
                                        disabled={emDownload || baixandoModelo !== null}
                                        onClick={() => BaixarModeloOcr(m.nome)}
                                      >
                                        {emDownload ? 'Baixando…' : '⬇️ Baixar'}
                                      </button>
                                    </div>
                                  );
                                })}
                              </div>
                            )}
                          </>
                        )}
                      </div>
                    );
                  })()}

                      </div>
                    </details>
                  )}

                  <h4 style={{ marginTop: '32px', marginBottom: '16px', color: 'var(--cor-destaque)' }}>Síntese de Voz (TTS)</h4>

                  {(!termoBusca || "motor de tts gerenciar motores voz baixar download".includes(termoBusca.toLowerCase())) && (
                    <>
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
                    </>
                  )}

                  {(!termoBusca || "pré-carregar cache áudio pronúncia baixar todas palavras dicionário tts offline".includes(termoBusca.toLowerCase())) && (() => {
                    const prog = progressoPreCacheTts;
                    const emAndamento = !!prog?.emAndamento;
                    const pct = prog && prog.total > 0 ? Math.round((prog.processados / prog.total) * 100) : 0;
                    return (
                      <div className="form-group">
                        <label>Pré-carregar áudio de todas as palavras</label>
                        <small style={{ color: 'var(--cor-texto-suave)', display: 'block', marginBottom: '8px' }}>
                          Sintetiza e guarda no cache a pronúncia de todas as palavras dos dicionários (CC-CEDICT + MakeMeAHanzi)
                          usando o motor selecionado acima. Depois disso, a leitura em voz alta de qualquer card sai na hora e sem uso de CPU.
                          É uma operação longa (dezenas de milhares de sínteses); roda em segundo plano, pula o que já está em cache e pode ser parada a qualquer momento.
                        </small>

                        <div style={{ display: 'flex', gap: '8px', alignItems: 'center' }}>
                          {!emAndamento && (
                            <button className="scan-btn" style={{ padding: '6px 14px', fontSize: '12px' }} onClick={PreCarregarAudioTts}>
                              ⬇️ Baixar todas as pronúncias
                            </button>
                          )}
                          {emAndamento && (
                            <button className="scan-btn" style={{ padding: '6px 14px', fontSize: '12px', backgroundColor: '#f44336' }} onClick={PararPreCarregarAudioTts}>
                              ⏹️ Parar
                            </button>
                          )}
                        </div>

                        {prog && (
                          <div style={{ marginTop: '10px' }}>
                            {emAndamento && prog.total > 0 && (
                              <div style={{ height: '6px', borderRadius: '3px', backgroundColor: 'var(--cor-borda)', overflow: 'hidden', marginBottom: '6px' }}>
                                <div style={{ height: '100%', width: `${pct}%`, backgroundColor: '#64b5f6', transition: 'width 0.2s' }} />
                              </div>
                            )}
                            <div style={{ fontSize: '11px', color: 'var(--cor-texto-suave)' }}>
                              {emAndamento && prog.total > 0 ? `${pct}% · ` : ''}{prog.mensagem}
                            </div>
                          </div>
                        )}
                      </div>
                    );
                  })()}
                </div>

                {/* ---------- ABA: DESEMPENHO ---------- */}
                <div style={{ display: (abaConfiguracao === 'Desempenho' || termoBusca) ? 'block' : 'none' }}>
                  {termoBusca && <h3 className="settings-section-title" style={{marginTop: '32px'}}>Desempenho (Hardware)</h3>}

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

                  <h3 className="settings-section-title" style={{marginTop: '32px'}}>Google Gemini (IA)</h3>

                  {(!termoBusca || "gemini api key google".includes(termoBusca.toLowerCase())) && (
                    <div className="form-group">
                      <label>Gemini API Key</label>
                      <input 
                        type="password"
                        className="form-input" 
                        value={configuracoesApp.geminiApiKey || ''}
                        onChange={e => AtualizarConfiguracao('geminiApiKey', e.target.value)}
                        placeholder="Cole sua API Key do Gemini aqui..."
                      />
                      <small style={{ color: 'var(--cor-texto-suave)', display: 'block', marginTop: '6px' }}>
                        Requer uma API key própria do Google AI Studio (gratuita).
                      </small>
                    </div>
                  )}


                  {(!termoBusca || "ativar modo gemini habilitar cota limite pausar resumo tradução linha".includes(termoBusca.toLowerCase())) && (
                    <>
                      <div className="form-group" style={{ marginTop: '16px' }}>
                        <label style={{ display: 'flex', alignItems: 'center', gap: '8px', justifyContent: 'space-between' }}>
                          <span>Habilitar Gemini (resumo ou tradução da tela)</span>
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
                            <label>Modelo do Gemini</label>
                            <select 
                              className="form-input" 
                              value={configuracoesApp.geminiModelo || 'gemini-1.5-flash'}
                              onChange={e => {
                                const novoModelo = e.target.value;
                                const novoLimite = novoModelo.includes('pro') ? 50 : 1500;
                                AplicarConfiguracao({ geminiModelo: novoModelo, geminiLimiteRequisicoesDia: novoLimite });
                              }}
                            >
                              <option value="gemini-1.5-flash">Gemini 1.5 Flash (Rápido, Cota Alta)</option>
                              <option value="gemini-1.5-pro">Gemini 1.5 Pro (Avançado, Cota Baixa)</option>
                              <option value="gemini-2.0-flash">Gemini 2.0 Flash (Mais atual)</option>
                              <option value="gemini-2.0-pro">Gemini 2.0 Pro</option>
                            </select>
                            <small style={{ color: 'var(--cor-texto-suave)', display: 'block', marginTop: '6px' }}>
                              Modelos Flash possuem cota gratuita muito maior (1500 req/dia) em relação aos Pro (50 req/dia).
                            </small>
                          </div>
                        )}
                        <div className="form-group">
                          <label style={{ display: 'flex', alignItems: 'center', gap: '8px', justifyContent: 'space-between' }}>
                            <span>Pop-up com resumo da tela (Atalho de Pop-up de Tudo)</span>
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
                            <label>Canto do pop-up de resumo</label>
                            <select 
                              className="form-input" 
                              value={configuracoesApp.geminiCantoResumo || 'superior-direito'}
                              onChange={e => AtualizarConfiguracao('geminiCantoResumo', e.target.value)}
                            >
                              <option value="superior-esquerdo">Superior esquerdo</option>
                              <option value="superior-direito">Superior direito</option>
                              <option value="inferior-esquerdo">Inferior esquerdo</option>
                              <option value="inferior-direito">Inferior direito</option>
                            </select>
                          </div>

                          <div className="form-group">
                            <label style={{ display: 'flex', alignItems: 'center', gap: '8px', justifyContent: 'space-between' }}>
                              <span>Enviar a imagem da tela junto (melhora o resumo)</span>
                              <input 
                                type="checkbox" 
                                checked={configuracoesApp.geminiEnviarImagem || false}
                                onChange={e => AtualizarConfiguracao('geminiEnviarImagem', e.target.checked)}
                              />
                            </label>
                            <small style={{ color: 'var(--cor-texto-suave)', display: 'block', marginTop: '6px' }}>
                              Mais custoso: a captura inteira é enviada ao Gemini a cada resumo, consumindo muito mais tokens da sua cota — e tudo que estiver visível na tela é enviado ao Google.
                            </small>
                          </div>
                        </SecaoDependente>

                        <div className="form-group">
                          <label style={{ display: 'flex', alignItems: 'center', gap: '8px', justifyContent: 'space-between' }}>
                            <span>Pop-ups de tradução em cada linha detectada (Atalho de Pop-up de Tudo)</span>
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
                            <span>Pausar ao atingir o limite diário de requisições</span>
                            <input 
                              type="checkbox" 
                              checked={configuracoesApp.geminiPausarPorCota || false}
                              onChange={e => AtualizarConfiguracao('geminiPausarPorCota', e.target.checked)}
                            />
                          </label>
                        </div>

                        <SecaoDependente ativa={configuracoesApp.geminiPausarPorCota || false}>
                          <div className="form-group" style={{ display: 'flex', alignItems: 'center', gap: '16px', margin: 0 }}>
                            <label style={{ margin: 0, flex: 1 }}>Limite de requisições por dia</label>
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

                        {infoCotaGemini && (
                          <div className="form-group" style={{ marginTop: '16px', padding: '12px', backgroundColor: 'var(--cor-fundo-cartao)', borderRadius: '8px', border: '1px solid var(--cor-borda)', marginBottom: 0 }}>
                            <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: '8px' }}>
                              <span>Uso da Cota Gemini (Dia {infoCotaGemini.data})</span>
                              <strong>{((infoCotaGemini.requisicoesUsadas / (configuracoesApp.geminiLimiteRequisicoesDia || 1500)) * 100).toFixed(1)}%</strong>
                            </div>
                            <div style={{ width: '100%', height: '8px', backgroundColor: 'var(--cor-borda)', borderRadius: '4px', overflow: 'hidden' }}>
                              <div style={{ 
                                height: '100%', 
                                width: `${Math.min(100, (infoCotaGemini.requisicoesUsadas / (configuracoesApp.geminiLimiteRequisicoesDia || 1500)) * 100)}%`,
                                backgroundColor: (infoCotaGemini.requisicoesUsadas / (configuracoesApp.geminiLimiteRequisicoesDia || 1500)) >= 0.9 ? '#f44336' : ((infoCotaGemini.requisicoesUsadas / (configuracoesApp.geminiLimiteRequisicoesDia || 1500)) >= 0.75 ? '#ffb74d' : 'var(--cor-destaque)')
                              }} />
                            </div>
                            <div style={{ fontSize: '11px', color: 'var(--cor-texto-suave)', marginTop: '8px', textAlign: 'right' }}>
                              {infoCotaGemini.requisicoesUsadas.toLocaleString('pt-BR')} / {(configuracoesApp.geminiLimiteRequisicoesDia || 1500).toLocaleString('pt-BR')} requisições hoje
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



                {/* ---------- ABA: INFO ---------- */}
                <div style={{ display: (abaConfiguracao === 'Info' || termoBusca) ? 'block' : 'none' }}>
                  {termoBusca && <h3 className="settings-section-title" style={{marginTop: '32px'}}>Info</h3>}
                  <div className="form-group">
                    <h3 style={{ marginBottom: '16px' }}>Sobre o Hanzi Tracker</h3>
                    <p style={{ color: 'var(--cor-texto-suave)', lineHeight: '1.5', marginBottom: '12px' }}>
                      O Hanzi Tracker é uma ferramenta voltada a auxiliar e otimizar o estudo do idioma chinês de forma dinâmica e interativa, fornecendo leitura contextual, revisão estruturada e reconhecimento óptico de caracteres em tempo real.
                    </p>
                    
                    <h4 style={{ marginTop: '24px', marginBottom: '8px' }}>Créditos</h4>
                    <ul style={{ color: 'var(--cor-texto-suave)', lineHeight: '1.5', paddingLeft: '20px', marginBottom: '24px' }}>
                      <li><strong>Donklii:</strong> Desenvolvedor principal e criador do projeto.</li>
                      <li><strong>makemeahanzi:</strong> Fornecimento de dados de traços e animações gráficas dos caracteres chineses.</li>
                    </ul>

                    <button 
                      className="scan-btn" 
                      onClick={() => BrowserOpenURL('https://github.com/Donklii/Hanzi-Tracker')}
                      style={{ display: 'inline-flex', alignItems: 'center', gap: '8px', padding: '10px 16px', fontSize: '14px' }}
                    >
                      <img src={iconeGithub} width="20" height="20" alt="GitHub" style={{ filter: 'invert(1)' }} />
                      Acessar Repositório no GitHub
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

function ToggleOpcoes({ opcoes, valor, onChange }: { opcoes: {valor: string, rotulo: string}[], valor: string, onChange: (v: string) => void }) {
  return (
    <div style={{ display: 'flex', backgroundColor: 'var(--cor-fundo-secundario)', borderRadius: '6px', padding: '2px', border: '1px solid var(--cor-borda)' }}>
      {opcoes.map(opcao => (
        <button
          key={opcao.valor}
          onClick={() => onChange(opcao.valor)}
          style={{
            flex: 1,
            padding: '6px 16px',
            border: 'none',
            borderRadius: '4px',
            backgroundColor: valor === opcao.valor ? 'var(--cor-destaque)' : 'transparent',
            color: valor === opcao.valor ? '#fff' : 'var(--cor-texto-suave)',
            cursor: 'pointer',
            fontWeight: valor === opcao.valor ? 'bold' : 'normal',
            transition: 'all 0.2s',
            fontSize: '13px'
          }}
        >
          {opcao.rotulo}
        </button>
      ))}
    </div>
  );
}
