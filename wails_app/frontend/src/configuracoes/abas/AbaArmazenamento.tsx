// ----- Seção: Configurações — aba Armazenamento (nuvem, uso de disco e limpeza por categoria) -----
import { config, main, nuvem } from '../../../wailsjs/go/models';
import { AbrirPastaDados } from '../../../wailsjs/go/main/App';
import { BrowserOpenURL } from '../../../wailsjs/runtime/runtime';
import { FormatarTamanho } from '../../comum/formatacao';

// Cores da barra de uso de armazenamento (uma por categoria; cicla se houver mais categorias).
const CORES_CATEGORIA_ARMAZENAMENTO = ['#64b5f6', '#81c784', '#ffb74d', '#ba68c8', '#f06292', '#4db6ac', '#a1887f'];

interface AbaArmazenamentoProps {
  termoBusca: string;
  infoArmazenamento: main.StorageInfo | null;
  armazenamentoOcupado: boolean;
  setConfirmacao: (c: any) => void;
  LimparCategoriaArmazenamento: (chave: string) => void;
  ExcluirTodoArmazenamento: () => void;
  configuracoesApp: config.Config;
  AtualizarConfiguracao: (key: keyof config.Config, value: any) => void;
  infoNuvem: nuvem.Info | null;
  nuvemOcupada: boolean;
  ConectarNuvemDrive: () => void;
  SincronizarNuvemDrive: () => void;
  DesconectarNuvemDrive: () => void;
  abrirConflitoNuvem: () => void;
}

// SecaoNuvem é o cartão da sincronização com o Google Drive: credenciais OAuth (coladas pelo
// usuário), conectar/sincronizar/desconectar e o estado da conexão.
function SecaoNuvem({ configuracoesApp, AtualizarConfiguracao, infoNuvem, nuvemOcupada, ConectarNuvemDrive, SincronizarNuvemDrive, DesconectarNuvemDrive, abrirConflitoNuvem }:
  Pick<AbaArmazenamentoProps, 'configuracoesApp' | 'AtualizarConfiguracao' | 'infoNuvem' | 'nuvemOcupada' | 'ConectarNuvemDrive' | 'SincronizarNuvemDrive' | 'DesconectarNuvemDrive' | 'abrirConflitoNuvem'>) {
  const desconectado = !infoNuvem || infoNuvem.estado === 'desconectado' || infoNuvem.estado === 'nao_configurado';
  const temCredenciais = !!configuracoesApp.driveClientId && !!configuracoesApp.driveClientSecret;

  return (
    <div className="form-group">
      <label style={{ margin: 0 }}>Sincronização na Nuvem (Google Drive)</label>
      <small style={{ color: 'var(--cor-texto-suave)', display: 'block', margin: '4px 0 8px' }}>
        Guarda uma cópia do seu banco de vocabulário no seu Google Drive e a atualiza sozinho enquanto você usa o app.
      </small>

      {desconectado && (
        <>
          <input
            type="text"
            className="form-input"
            value={configuracoesApp.driveClientId || ''}
            onChange={e => AtualizarConfiguracao('driveClientId', e.target.value)}
            placeholder="Client ID (….apps.googleusercontent.com)"
          />
          <input
            type="password"
            className="form-input"
            style={{ marginTop: '6px' }}
            value={configuracoesApp.driveClientSecret || ''}
            onChange={e => AtualizarConfiguracao('driveClientSecret', e.target.value)}
            placeholder="Client Secret"
          />
          <small style={{ color: 'var(--cor-texto-suave)', display: 'block', margin: '6px 0 8px' }}>
            Requer credenciais OAuth próprias:{' '}
            <a
              href="#"
              style={{ color: 'var(--cor-destaque)' }}
              onClick={e => { e.preventDefault(); BrowserOpenURL('https://console.cloud.google.com/apis/credentials'); }}
            >
              crie no Google Cloud Console
            </a>{' '}
            um "ID do cliente OAuth" do tipo <strong>App para computador</strong>, com a API do Google Drive ativada no projeto, e cole o par aqui.
          </small>
          <button
            className="scan-btn"
            disabled={nuvemOcupada || !temCredenciais}
            style={{ opacity: (nuvemOcupada || !temCredenciais) ? 0.5 : 1 }}
            title={temCredenciais ? undefined : 'Preencha o Client ID e o Client Secret para conectar.'}
            onClick={ConectarNuvemDrive}
          >
            {nuvemOcupada ? '⏳ Aguardando autorização no navegador…' : '🔗 Conectar Google Drive'}
          </button>
        </>
      )}

      {infoNuvem?.estado === 'conflito' && (
        <div style={{ border: '1px solid #ffb74d', borderRadius: '8px', padding: '12px', backgroundColor: 'var(--cor-fundo-cartao)' }}>
          <div style={{ fontSize: '13px', fontWeight: 'bold', color: '#ffb74d' }}>⚠️ Já existe um backup na nuvem</div>
          <div style={{ fontSize: '12px', color: 'var(--cor-texto-suave)', margin: '4px 0 8px' }}>
            Conectado como <strong>{infoNuvem.email}</strong>. Nada será sincronizado até você escolher entre os dados deste computador e os da nuvem.
          </div>
          <button className="scan-btn" style={{ padding: '4px 10px', fontSize: '11px' }} disabled={nuvemOcupada} onClick={abrirConflitoNuvem}>
            Resolver conflito
          </button>
        </div>
      )}

      {infoNuvem?.estado === 'conectado' && (
        <div style={{ border: '1px solid var(--cor-borda)', borderRadius: '8px', padding: '12px', backgroundColor: 'var(--cor-fundo-cartao)' }}>
          <div style={{ fontSize: '13px' }}>
            ☁️ Conectado como <strong>{infoNuvem.email || 'conta Google'}</strong>
          </div>
          <div style={{ fontSize: '12px', color: 'var(--cor-texto-suave)', marginTop: '2px' }}>
            {infoNuvem.ultimaSincronizacao
              ? <>Última sincronização: {new Date(infoNuvem.ultimaSincronizacao).toLocaleString()} · {FormatarTamanho(infoNuvem.remotoBytes) || '0 MB'} na nuvem</>
              : 'Ainda não sincronizado nesta sessão.'}
          </div>
          {infoNuvem.erro && (
            <div style={{ fontSize: '12px', color: '#f44336', marginTop: '4px' }}>⚠️ {infoNuvem.erro}</div>
          )}
          <div style={{ display: 'flex', gap: '8px', marginTop: '10px' }}>
            <button className="scan-btn" style={{ padding: '4px 10px', fontSize: '11px', opacity: nuvemOcupada ? 0.5 : 1 }} disabled={nuvemOcupada} onClick={SincronizarNuvemDrive}>
              {nuvemOcupada ? '⏳ Sincronizando…' : '🔄 Sincronizar agora'}
            </button>
            <button className="scan-btn" style={{ padding: '4px 10px', fontSize: '11px', backgroundColor: 'var(--cor-fundo-secundario)' }} disabled={nuvemOcupada} onClick={DesconectarNuvemDrive}>
              Desconectar
            </button>
          </div>
        </div>
      )}
    </div>
  );
}

export function AbaArmazenamento({ termoBusca, infoArmazenamento, armazenamentoOcupado, setConfirmacao, LimparCategoriaArmazenamento, ExcluirTodoArmazenamento,
  configuracoesApp, AtualizarConfiguracao, infoNuvem, nuvemOcupada, ConectarNuvemDrive, SincronizarNuvemDrive, DesconectarNuvemDrive, abrirConflitoNuvem }: AbaArmazenamentoProps) {
  return (
    <>
      {termoBusca && <h3 className="settings-section-title" style={{ marginTop: '32px' }}>Armazenamento</h3>}

      {(!termoBusca || "nuvem google drive sincronização backup conectar conta credenciais client id secret oauth".includes(termoBusca.toLowerCase())) && (
        <SecaoNuvem
          configuracoesApp={configuracoesApp}
          AtualizarConfiguracao={AtualizarConfiguracao}
          infoNuvem={infoNuvem}
          nuvemOcupada={nuvemOcupada}
          ConectarNuvemDrive={ConectarNuvemDrive}
          SincronizarNuvemDrive={SincronizarNuvemDrive}
          DesconectarNuvemDrive={DesconectarNuvemDrive}
          abrirConflitoNuvem={abrirConflitoNuvem}
        />
      )}

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
    </>
  );
}
