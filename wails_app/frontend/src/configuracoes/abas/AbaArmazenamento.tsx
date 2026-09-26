// ----- Seção: Configurações — aba Armazenamento (nuvem, uso de disco e limpeza por categoria) -----
import { main, nuvem } from '../../../wailsjs/go/models';
import { AbrirPastaDados } from '../../../wailsjs/go/main/App';
import { FormatarTamanho } from '../../comum/formatacao';
import { t } from '../../i18n/i18n';

// Cores da barra de uso de armazenamento (uma por categoria; cicla se houver mais categorias).
const CORES_CATEGORIA_ARMAZENAMENTO = ['#64b5f6', '#81c784', '#ffb74d', '#ba68c8', '#f06292', '#4db6ac', '#a1887f'];

interface AbaArmazenamentoProps {
  termoBusca: string;
  infoArmazenamento: main.StorageInfo | null;
  armazenamentoOcupado: boolean;
  setConfirmacao: (c: any) => void;
  LimparCategoriaArmazenamento: (chave: string) => void;
  ExcluirTodoArmazenamento: () => void;
  infoNuvem: nuvem.Info | null;
  nuvemOcupada: boolean;
  ConectarNuvemDrive: () => void;
  SincronizarNuvemDrive: () => void;
  DesconectarNuvemDrive: () => void;
  abrirConflitoNuvem: () => void;
}

// SecaoNuvem é o cartão da sincronização com o Google Drive: conectar/sincronizar/desconectar e o
// estado da conexão. As credenciais OAuth ficam no servidor do Hanzi Tracker — o usuário só
// autoriza a conta dele no navegador (ver wails_app/nuvem/ponte.go).
function SecaoNuvem({ infoNuvem, nuvemOcupada, ConectarNuvemDrive, SincronizarNuvemDrive, DesconectarNuvemDrive, abrirConflitoNuvem }:
  Pick<AbaArmazenamentoProps, 'infoNuvem' | 'nuvemOcupada' | 'ConectarNuvemDrive' | 'SincronizarNuvemDrive' | 'DesconectarNuvemDrive' | 'abrirConflitoNuvem'>) {
  const desconectado = !infoNuvem || infoNuvem.estado === 'desconectado';

  return (
    <div className="form-group">
      <label style={{ margin: 0 }}>{t('Sincronização na Nuvem (Google Drive)')}</label>
      <small style={{ color: 'var(--cor-texto-suave)', display: 'block', margin: '4px 0 8px' }}>
        {t('Guarda o seu vocabulário e as suas configurações numa pasta "Hanzi Tracker" do seu Google Drive, e a atualiza sozinho enquanto você usa o app.')}
      </small>

      {desconectado && (
        <>
          <small style={{ color: 'var(--cor-texto-suave)', display: 'block', margin: '0 0 8px' }}>
            {t('Você autoriza a sua conta Google no navegador e pronto — o backup fica no seu próprio Drive, e só o Hanzi Tracker enxerga esse arquivo.')}
          </small>
          <button
            className="scan-btn"
            disabled={nuvemOcupada}
            style={{ opacity: nuvemOcupada ? 0.5 : 1 }}
            onClick={ConectarNuvemDrive}
          >
            {nuvemOcupada ? t('⏳ Aguardando autorização no navegador…') : t('🔗 Conectar Google Drive')}
          </button>
        </>
      )}

      {infoNuvem?.estado === 'conflito' && (
        <div style={{ border: '1px solid #ffb74d', borderRadius: '8px', padding: '12px', backgroundColor: 'var(--cor-fundo-cartao)' }}>
          <div style={{ fontSize: '13px', fontWeight: 'bold', color: '#ffb74d' }}>{t('⚠️ Já existe um backup na nuvem')}</div>
          <div style={{ fontSize: '12px', color: 'var(--cor-texto-suave)', margin: '4px 0 8px' }}>
            {t('Conectado como')}{' '}
            <strong>{infoNuvem.email}</strong>{t('. Nada será sincronizado até você escolher entre os dados deste computador e os da nuvem.')}
          </div>
          <button className="scan-btn" style={{ padding: '4px 10px', fontSize: '11px' }} disabled={nuvemOcupada} onClick={abrirConflitoNuvem}>
            {t('Resolver conflito')}
          </button>
        </div>
      )}

      {infoNuvem?.estado === 'conectado' && (
        <div style={{ border: '1px solid var(--cor-borda)', borderRadius: '8px', padding: '12px', backgroundColor: 'var(--cor-fundo-cartao)' }}>
          <div style={{ fontSize: '13px' }}>
            {t('☁️ Conectado como')} <strong>{infoNuvem.email || t('conta Google')}</strong>
          </div>
          <div style={{ fontSize: '12px', color: 'var(--cor-texto-suave)', marginTop: '2px' }}>
            {infoNuvem.ultimaSincronizacao
              ? <>{t('Última sincronização:')} {new Date(infoNuvem.ultimaSincronizacao).toLocaleString()} · {FormatarTamanho(infoNuvem.remotoBytes) || '0 MB'} {t('na nuvem')}</>
              : t('Ainda não sincronizado nesta sessão.')}
          </div>
          {infoNuvem.erro && (
            <div style={{ fontSize: '12px', color: '#f44336', marginTop: '4px' }}>⚠️ {infoNuvem.erro}</div>
          )}
          <div style={{ display: 'flex', gap: '8px', marginTop: '10px' }}>
            <button className="scan-btn" style={{ padding: '4px 10px', fontSize: '11px', opacity: nuvemOcupada ? 0.5 : 1 }} disabled={nuvemOcupada} onClick={SincronizarNuvemDrive}>
              {nuvemOcupada ? t('⏳ Sincronizando…') : t('🔄 Sincronizar agora')}
            </button>
            <button className="scan-btn" style={{ padding: '4px 10px', fontSize: '11px', backgroundColor: 'var(--cor-fundo-secundario)' }} disabled={nuvemOcupada} onClick={DesconectarNuvemDrive}>
              {t('Desconectar')}
            </button>
          </div>
        </div>
      )}
    </div>
  );
}

export function AbaArmazenamento({ termoBusca, infoArmazenamento, armazenamentoOcupado, setConfirmacao, LimparCategoriaArmazenamento, ExcluirTodoArmazenamento,
  infoNuvem, nuvemOcupada, ConectarNuvemDrive, SincronizarNuvemDrive, DesconectarNuvemDrive, abrirConflitoNuvem }: AbaArmazenamentoProps) {
  return (
    <>
      {termoBusca && <h3 className="settings-section-title" style={{ marginTop: '32px' }}>{t('Armazenamento')}</h3>}

      {(!termoBusca || "nuvem google drive sincronização backup conectar conta".includes(termoBusca.toLowerCase())) && (
        <SecaoNuvem
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
          <label style={{ margin: 0 }}>{t('Uso de Disco')}</label>
          <button
            className="scan-btn"
            style={{ padding: '4px 10px', fontSize: '11px' }}
            onClick={() => AbrirPastaDados()}
          >
            {t('📂 Abrir pasta de dados')}
          </button>
        </div>

        {infoArmazenamento && (
          <div style={{ fontSize: '12px', color: 'var(--cor-texto-suave)', marginTop: '6px' }}>
            {t('App usa')} <strong>{FormatarTamanho(infoArmazenamento.totalBytes) || '0 MB'}</strong>
            {infoArmazenamento.discoTotal > 0 && (
              <> · {t('Disco:')} <strong style={{ color: infoArmazenamento.discoLivre < 1024 * 1024 * 1024 ? '#f44336' : 'inherit' }}>
                {t('{livres} livres de {total}', { livres: FormatarTamanho(infoArmazenamento.discoLivre), total: FormatarTamanho(infoArmazenamento.discoTotal) })}
              </strong></>
            )}
          </div>
        )}
        {!infoArmazenamento && (
          <div style={{ fontSize: '12px', color: 'var(--cor-texto-suave)', marginTop: '6px' }}>{t('Calculando…')}</div>
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
                    title={`${t(it.rotulo)}: ${FormatarTamanho(it.bytes)}`}
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
                    {t(it.rotulo)} · {FormatarTamanho(it.bytes)}
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
                {t(item.rotulo)}
                <span style={{ marginLeft: '8px', fontSize: '11px', color: 'var(--cor-destaque)' }}>
                  {FormatarTamanho(item.bytes) || '0 MB'}
                </span>
                {item.perigoso && <span style={{ marginLeft: '8px', fontSize: '10px', color: '#f44336', fontWeight: 'bold' }}>{t('DADOS DO USUÁRIO')}</span>}
              </div>
              <div style={{ fontSize: '11px', color: 'var(--cor-texto-suave)', marginTop: '2px' }}>{t(item.descricao)}</div>
            </div>
            <button
              className="scan-btn"
              style={{ padding: '4px 10px', fontSize: '11px', backgroundColor: item.perigoso ? '#f44336' : undefined, opacity: (armazenamentoOcupado || item.bytes === 0) ? 0.5 : 1 }}
              disabled={armazenamentoOcupado || item.bytes === 0}
              onClick={() => {
                if (item.perigoso) {
                  setConfirmacao({
                    titulo: t('Apagar o vocabulário?'),
                    mensagem: t('Isso apaga TODAS as suas palavras (vistas, em estudo e aprendidas). Esta ação não pode ser desfeita.'),
                    rotuloAcao: t('Apagar vocabulário'),
                    acao: () => LimparCategoriaArmazenamento(item.chave),
                  });
                } else {
                  LimparCategoriaArmazenamento(item.chave);
                }
              }}
            >
              {t('🗑️ Limpar')}
            </button>
          </div>
        </div>
      ))}

      <div className="form-group" style={{ marginTop: '24px', borderTop: '1px solid var(--cor-borda)', paddingTop: '16px' }}>
        <label style={{ color: '#f44336' }}>{t('Zona de Perigo')}</label>
        <small style={{ color: 'var(--cor-texto-suave)', display: 'block', marginBottom: '8px' }}>
          {t('Apaga todos os modelos baixados, o cache de instalação, os logs e zera o vocabulário. As suas preferências (configurações) são mantidas.')}
        </small>
        <button
          className="scan-btn"
          style={{ backgroundColor: '#f44336', opacity: armazenamentoOcupado ? 0.5 : 1 }}
          disabled={armazenamentoOcupado}
          onClick={() => setConfirmacao({
            titulo: t('Excluir tudo?'),
            mensagem: t('Serão apagados: modelos de OCR baixados, modelos do EasyOCR, cache do pip, logs e TODO o vocabulário. As preferências serão mantidas. Esta ação não pode ser desfeita.'),
            rotuloAcao: t('Excluir tudo'),
            acao: () => ExcluirTodoArmazenamento(),
          })}
        >
          {t('🧹 Excluir Tudo')}
        </button>
      </div>
    </>
  );
}
