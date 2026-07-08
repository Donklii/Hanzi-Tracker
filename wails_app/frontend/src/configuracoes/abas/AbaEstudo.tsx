// ----- Seção: Configurações — aba Estudo (revisão, destaques e tipo de hanzi) -----
import { config } from '../../../wailsjs/go/models';
import { SecaoDependente } from '../comum';

interface AbaEstudoProps {
  termoBusca: string;
  configuracoesApp: config.Config;
  AtualizarConfiguracao: (key: keyof config.Config, value: any) => void;
}

export function AbaEstudo({ termoBusca, configuracoesApp, AtualizarConfiguracao }: AbaEstudoProps) {
  return (
    <>
      {termoBusca && <h3 className="settings-section-title" style={{ marginTop: '32px' }}>Estudo</h3>}

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
    </>
  );
}

// ToggleOpcoes é o seletor segmentado (Ambos/Tradicional/Simplificado) usado só nesta aba.
function ToggleOpcoes({ opcoes, valor, onChange }: { opcoes: { valor: string, rotulo: string }[], valor: string, onChange: (v: string) => void }) {
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
