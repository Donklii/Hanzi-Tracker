// ----- Seção: Configurações — aba Motores (OCR, hardware, modelos, voz/TTS e escuta/STT) -----
import { config, main } from '../../../wailsjs/go/models';
import { FormatarTamanho } from '../../comum/formatacao';
import { ProgressoPreCacheTts } from '../../comum/tipos';
import { MOTOR_STT_WEB_SPEECH, TemWebSpeech } from '../../comum/useSTT';

interface AbaMotoresProps {
  termoBusca: string;
  configuracoesApp: config.Config;
  AtualizarConfiguracao: (key: keyof config.Config, value: any) => void;
  AplicarConfiguracao: (mudancas: Partial<config.Config>) => void;
  setConfirmacao: (c: any) => void;
  infoHardware: main.SystemHardware | null;
  ehCpuNome: (hw: string) => boolean;
  motores: main.MotorOcrInfo[];
  progressoMotor: Record<string, string>;
  baixandoMotor: string | null;
  trocandoMotor: string | null;
  BaixarMotorOcr: (nome: string) => void;
  RemoverMotorOcr: (nome: string) => void;
  TrocarMotorOcr: (nome: string) => void;
  modelos: main.ModeloOcrInfo[];
  progressoModelo: Record<string, string>;
  baixandoModelo: string | null;
  BaixarModeloOcr: (nome: string) => void;
  RemoverModeloOcr: (nome: string) => void;
  trocarModelo: (nome: string) => void;
  motoresTts: main.MotorTtsInfo[];
  progressoMotorTts: Record<string, string>;
  baixandoMotorTts: string | null;
  BaixarMotorVoz: (nome: string) => void;
  RemoverMotorVoz: (nome: string) => void;
  progressoPreCacheTts: ProgressoPreCacheTts | null;
  PreCarregarAudioTts: () => void;
  PararPreCarregarAudioTts: () => void;
  motoresStt: main.MotorSttInfo[];
  progressoMotorStt: Record<string, string>;
  baixandoMotorStt: string | null;
  BaixarMotorEscuta: (nome: string) => void;
  RemoverMotorEscuta: (nome: string) => void;
}

export function AbaMotores(props: AbaMotoresProps) {
  const {
    termoBusca, configuracoesApp, AtualizarConfiguracao, AplicarConfiguracao, setConfirmacao,
    infoHardware, ehCpuNome,
    motores, progressoMotor, baixandoMotor, trocandoMotor, BaixarMotorOcr, RemoverMotorOcr, TrocarMotorOcr,
    modelos, progressoModelo, baixandoModelo, BaixarModeloOcr, RemoverModeloOcr, trocarModelo,
    motoresTts, progressoMotorTts, baixandoMotorTts, BaixarMotorVoz, RemoverMotorVoz,
    progressoPreCacheTts, PreCarregarAudioTts, PararPreCarregarAudioTts,
    motoresStt, progressoMotorStt, baixandoMotorStt, BaixarMotorEscuta, RemoverMotorEscuta,
  } = props;

  // Motor de OCR ativo: os modelos vêm do /api/modelos do PROCESSO desse motor, então o catálogo
  // exibido abaixo é sempre o do motor em execução (troca de motor recarrega `modelos`, ver App.tsx).
  const motorAtivo = motores.find(m => m.ativo);

  // Aceleração suportada pelo motor ativo (derivada da `variante`: "CPU/WebGPU" ou "CPU").
  // O hardware de processamento depende disto: motor só-CPU não oferece GPU; WebGPU vale em
  // qualquer GPU (Nvidia/AMD/Intel — D3D12 no Windows, Vulkan no Linux).
  const varianteMotor = (motorAtivo?.variante || 'CPU').toLowerCase();
  const motorSoCpu = !varianteMotor.includes('webgpu');
  const nomeCpu = infoHardware?.cpu || 'CPU';
  const hardwareEhCpu = ehCpuNome(configuracoesApp?.hardwareSelecionado || 'CPU');

  // Cor do feedback por status: verde (sucesso), vermelho (erro), neutro (em andamento).
  const corProgresso = (msg: string) =>
    msg.startsWith('✅') ? '#81c784' : msg.startsWith('⚠️') ? '#f44336' : 'var(--cor-texto-suave)';

  return (
    <>
      {termoBusca && <h3 className="settings-section-title" style={{ marginTop: '32px' }}>Motores (OCR, TTS & STT)</h3>}

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
      {(!termoBusca || "hardware dispositivo processamento ocr cpu gpu nvidia amd intel api webgpu vulkan aceleração".includes(termoBusca.toLowerCase())) && (
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
                  AplicarConfiguracao({
                    hardwareSelecionado: val,
                    dispositivoOcr: val === nomeCpu ? 'cpu' : 'webgpu',
                  });
                }}
              >
                <option value={nomeCpu} title="Compatível com todos os motores de OCR.">{nomeCpu} (CPU)</option>
                {infoHardware?.gpus?.map(gpu => (
                  <option key={gpu} value={gpu} title="Aceleração via WebGPU — funciona em qualquer GPU (Nvidia, AMD, Intel).">
                    {gpu}
                  </option>
                ))}
              </select>

              {!hardwareEhCpu && (
                <small style={{ color: 'var(--cor-texto-suave)', display: 'block', marginTop: '6px' }}>
                  Aceleração via WebGPU (D3D12 no Windows; Vulkan no Linux). O processamento usa o
                  adaptador de vídeo padrão do sistema.
                </small>
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

      <h4 style={{ marginTop: '32px', marginBottom: '16px', color: 'var(--cor-destaque)' }}>Reconhecimento de Voz (STT)</h4>

      {(!termoBusca || "motor de stt escuta reconhecimento de voz fala pronúncia microfone baixar download web speech navegador".includes(termoBusca.toLowerCase())) && (() => {
        const webSpeechDisponivel = TemWebSpeech();
        return (
        <>
          <div className="form-group">
            <label>Motor de Escuta</label>
            <select
              className="form-input"
              value={configuracoesApp.motorSttAtivo}
              onChange={e => AtualizarConfiguracao('motorSttAtivo', e.target.value)}
            >
              <option
                value={MOTOR_STT_WEB_SPEECH}
                disabled={!webSpeechDisponivel}
                title={webSpeechDisponivel
                  ? 'Reconhecimento de voz da própria webview — sem download, mas exige conexão e suporte da plataforma.'
                  : 'A webview desta plataforma não oferece a Web Speech API.'}
              >
                Web Speech (navegador){!webSpeechDisponivel ? ' — indisponível nesta plataforma' : ''}
              </option>
              {motoresStt.map(m => (
                <option key={m.nome} value={m.nome}>{m.nome}</option>
              ))}
            </select>
            <small style={{ color: 'var(--cor-texto-suave)', display: 'block', marginTop: '6px' }}>
              Usado na revisão de pronúncia (falar no microfone). "Web Speech" usa o reconhecimento
              embutido da webview (sem download, requer internet e suporte da plataforma); os demais
              são motores locais — o selecionado precisa estar instalado (abaixo) e o modelo de
              reconhecimento é baixado automaticamente na primeira escuta.
            </small>
          </div>

          <div className="form-group">
            <label>Gerenciar Motores de Escuta</label>
            <small style={{ color: 'var(--cor-texto-suave)', display: 'block', marginBottom: '8px' }}>
              O motor é o programa que grava o microfone e transcreve a fala. Baixe o que quiser usar ou
              remova para liberar espaço. Os pesos do modelo são baixados pelo próprio motor na primeira escuta.
            </small>

            {motoresStt.length === 0 && (
              <div style={{ color: 'var(--cor-texto-suave)', fontSize: '12px' }}>Carregando motores…</div>
            )}

            {motoresStt.map(m => {
              const emDownload = baixandoMotorStt === m.nome;
              const msg = progressoMotorStt[m.nome];
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
                          disabled={emDownload || baixandoMotorStt !== null || !m.publicado}
                          title={!m.publicado ? 'Este motor ainda não foi publicado — aguarde a próxima atualização.' : undefined}
                          onClick={() => BaixarMotorEscuta(m.nome)}
                        >
                          {emDownload ? 'Baixando…' : (m.publicado ? '⬇️ Baixar' : 'Indisponível')}
                        </button>
                      )}
                      {m.instalado && (
                        <button
                          className="scan-btn"
                          style={{ padding: '4px 10px', fontSize: '11px', backgroundColor: '#f44336' }}
                          onClick={() => RemoverMotorEscuta(m.nome)}
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
        );
      })()}
    </>
  );
}
