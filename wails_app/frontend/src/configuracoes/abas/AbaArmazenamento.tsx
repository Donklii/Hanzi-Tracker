// ----- Seção: Configurações — aba Armazenamento (uso de disco e limpeza por categoria) -----
import { main } from '../../../wailsjs/go/models';
import { AbrirPastaDados } from '../../../wailsjs/go/main/App';
import { FormatarTamanho } from '../comum';

// Cores da barra de uso de armazenamento (uma por categoria; cicla se houver mais categorias).
const CORES_CATEGORIA_ARMAZENAMENTO = ['#64b5f6', '#81c784', '#ffb74d', '#ba68c8', '#f06292', '#4db6ac', '#a1887f'];

interface AbaArmazenamentoProps {
  termoBusca: string;
  infoArmazenamento: main.StorageInfo | null;
  armazenamentoOcupado: boolean;
  setConfirmacao: (c: any) => void;
  LimparCategoriaArmazenamento: (chave: string) => void;
  ExcluirTodoArmazenamento: () => void;
}

export function AbaArmazenamento({ termoBusca, infoArmazenamento, armazenamentoOcupado, setConfirmacao, LimparCategoriaArmazenamento, ExcluirTodoArmazenamento }: AbaArmazenamentoProps) {
  return (
    <>
      {termoBusca && <h3 className="settings-section-title" style={{ marginTop: '32px' }}>Armazenamento</h3>}

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
