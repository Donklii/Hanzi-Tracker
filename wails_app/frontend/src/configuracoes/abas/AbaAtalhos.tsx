// ----- Seção: Configurações — aba Atalhos Globais -----
import { config } from '../../../wailsjs/go/models';
import { t } from '../../i18n/i18n';
import { GravadorAtalho } from '../componentes/GravadorAtalho';
import { useAtalhos } from '../../atalhos/AtalhosContext';
import './AbaAtalhos.css';

interface AbaAtalhosProps {
  termoBusca: string;
  configuracoesApp: config.Config;
  AtualizarConfiguracao: (key: keyof config.Config, value: any) => void;
}

const PADROES = {
  atalhoEscanear: 'ctrl+shift+e',
  atalhoPopupTodos: 'ctrl+shift+t',
  atalhoMarcarEstudo: 'ctrl+shift+m',
  atalhoAlternarPopupHover: 'ctrl+shift+h',
};

export function AbaAtalhos({ termoBusca, configuracoesApp, AtualizarConfiguracao }: AbaAtalhosProps) {
  const { abrirGuia } = useAtalhos();

  // Lista dos atalhos configuráveis do sistema
  const itensAtalhos = [
    {
      key: 'atalhoEscanear' as const,
      titulo: t('Atalho: Escanear Tela'),
      desc: t('Aciona o OCR instantâneo da área visível na tela principal'),
      buscaTag: 'atalho escanear',
    },
    {
      key: 'atalhoPopupTodos' as const,
      titulo: t('Atalho: Mostrar Pop-up de Tudo'),
      desc: t('Exibe ou esconde pop-ups de tradução para todas as linhas/cards'),
      buscaTag: 'atalho popup todos',
    },
    {
      key: 'atalhoMarcarEstudo' as const,
      titulo: t('Atalho: Marcar Card em Estudo'),
      desc: t('Move instantaneamente o card sob o foco para o status "Em Estudo"'),
      buscaTag: 'atalho marcar estudo vocabulario',
    },
    {
      key: 'atalhoAlternarPopupHover' as const,
      titulo: t('Atalho: Pop-up no Cursor'),
      desc: t('Ativa ou desativa a lupa de dicionário ao passar o cursor do mouse'),
      buscaTag: 'atalho hover pop-up popup cursor',
    },
  ];

  // Verifica duplicações de atalhos globais
  const obterConflito = (chave: keyof typeof PADROES, valor: string): string | undefined => {
    if (!valor) return undefined;

    const duplicado = itensAtalhos.find((item) => item.key !== chave && configuracoesApp[item.key] === valor);
    if (duplicado) {
      return t(`Atalho em uso por "${duplicado.titulo}"`);
    }
    return undefined;
  };

  const restaurarTodosPadroes = () => {
    AtualizarConfiguracao('atalhoEscanear', PADROES.atalhoEscanear);
    AtualizarConfiguracao('atalhoPopupTodos', PADROES.atalhoPopupTodos);
    AtualizarConfiguracao('atalhoMarcarEstudo', PADROES.atalhoMarcarEstudo);
    AtualizarConfiguracao('atalhoAlternarPopupHover', PADROES.atalhoAlternarPopupHover);
  };

  return (
    <div className="atalhos-container">
      {/* Cabeçalho centralizado */}
      <div className="atalhos-header-card">
        <div className="atalhos-header-info">
          <h3>🌐 {t('Atalhos Globais do Sistema')}</h3>
          <p>{t('Clique na caixa do atalho e pressione as teclas desejadas no teclado para gravar.')}</p>
        </div>
        <button type="button" className="btn-secondary btn-guia-atalhos-header" onClick={abrirGuia}>
          <span>⌨️</span> {t('Guia de Atalhos (?)')}
        </button>
      </div>

      {/* Grid de Cards dos Atalhos */}
      <div className="atalhos-cards-grid">
        {itensAtalhos.map((item) => {
          if (termoBusca && !item.buscaTag.includes(termoBusca.toLowerCase())) {
            return null;
          }

          const valorAtual = configuracoesApp[item.key];
          const conflito = obterConflito(item.key, valorAtual);

          return (
            <div key={item.key} className="atalho-row-card">
              <div className="atalho-row-info">
                <span className="atalho-row-title">{item.titulo}</span>
                <span className="atalho-row-desc">{item.desc}</span>
              </div>
              <div className="atalho-row-input">
                <GravadorAtalho
                  valor={valorAtual}
                  valorPadrao={PADROES[item.key]}
                  aoAlterar={(val) => AtualizarConfiguracao(item.key, val)}
                  temConflito={!!conflito}
                  mensagemErro={conflito}
                />
              </div>
            </div>
          );
        })}
      </div>

      {/* Rodapé com botão de restauração */}
      <div className="atalhos-footer">
        <button type="button" className="btn-secondary btn-restaurar-todos" onClick={restaurarTodosPadroes}>
          ↺ {t('Restaurar Atalhos Globais Padrão')}
        </button>
      </div>
    </div>
  );
}
