// ----- Seção: Revisão — Grupo de foco no cabeçalho -----
// Painel compacto renderizado na barra de título do app (App.tsx) quando a aba Revisão está
// ativa, ocupando o espaço que as outras abas usam para a busca e o "Adicionar Hanzi".
// Linha única: interruptor Priorizar → UM caractere do foco por vez (rotaciona no tempo) →
// botão de lista que abre o pop-up de gerenciamento (FocoPopup). O tamanho do grupo e a adição
// manual moraram na linha antes; agora vivem no pop-up para deixar o cabeçalho enxuto.
import { useState, useEffect } from 'react';
import { config, revisao } from '../../wailsjs/go/models';
import { Interruptor } from './Interruptor';
import { FocoPopup } from './FocoPopup';
import { t } from '../i18n/i18n';
import './revisao.css';

interface GrupoFocoCabecalhoProps {
  configuracoesApp: config.Config | null;
  AtualizarConfiguracao?: (key: keyof config.Config, value: any) => void;
  foco: revisao.ItemFocoRevisao[];
  aoClicarNoFoco?: (item: revisao.ItemFocoRevisao) => void;
  recarregarFoco?: () => void;
}

const MS_ROTACAO_FOCO = 4000;

export function GrupoFocoCabecalho({ configuracoesApp, AtualizarConfiguracao, foco, aoClicarNoFoco, recarregarFoco }: GrupoFocoCabecalhoProps) {
  const priorizarFoco = configuracoesApp?.priorizarEstudoRevisao !== false;
  const [indiceRotacao, setIndiceRotacao] = useState(0);
  const [popupAberto, setPopupAberto] = useState(false);

  // Rotação: mostra um caractere de cada vez, alternando a cada MS_ROTACAO_FOCO. Com 0 ou 1 no
  // grupo não há o que rotacionar.
  useEffect(() => {
    if (!priorizarFoco || foco.length <= 1) {
      setIndiceRotacao(0);
      return;
    }
    const timer = window.setInterval(() => {
      setIndiceRotacao(i => (i + 1) % foco.length);
    }, MS_ROTACAO_FOCO);
    return () => window.clearInterval(timer);
  }, [priorizarFoco, foco.length]);

  const itemAtual = foco.length > 0 ? foco[indiceRotacao % foco.length] : null;

  // Sem AtualizarConfiguracao (prop opcional), degrada para o modo só-status: some quando
  // não há nada a mostrar.
  if (!AtualizarConfiguracao && (!priorizarFoco || foco.length === 0)) return null;

  return (
    <div
      className={`revisao-foco-painel${priorizarFoco ? '' : ' inativo'}`}
      title={priorizarFoco
        ? t('As sessões concentram 9 de cada 10 questões neste grupo até cada caractere concluir o aprendizado. Abra a lista para gerenciar.')
        : t('Priorização desligada: as sessões sorteiam entre todo o vocabulário, sem insistir num grupo pequeno.')}
    >
      {AtualizarConfiguracao && (
        <div className="revisao-foco-controles">
          <div className="revisao-foco-controle" title={t("As sessões concentram 9 de cada 10 questões nos caracteres do grupo de foco.")}>
            <span>{t('Priorizar')}</span>
            <Interruptor
              ligado={priorizarFoco}
              aoAlternar={() => AtualizarConfiguracao('priorizarEstudoRevisao', !priorizarFoco)}
            />
          </div>
        </div>
      )}

      {priorizarFoco && itemAtual && (
        <div className="revisao-foco-rotacao">
          <div
            key={itemAtual.hanzi}
            className="revisao-foco-chip"
            title={itemAtual.significados?.join(', ')}
            onClick={() => aoClicarNoFoco && aoClicarNoFoco(itemAtual)}
          >
            <span className="revisao-foco-chip-hanzi">{itemAtual.hanzi}</span>
            <span className="revisao-foco-chip-pinyin">{itemAtual.pinyin}</span>
            <span className="revisao-foco-chip-progresso" title={t("Áreas de aprendizado concluídas (streak ≥ 3) — 5/5 libera a sugestão de aprendida")}>
              {itemAtual.areasConcluidas}/5
            </span>
          </div>
        </div>
      )}

      {priorizarFoco && foco.length === 0 && (
        <div className="revisao-foco-vazio">
          {t('Nenhuma palavra em foco — marque palavras como "Em estudo" ou adicione pela lista.')}
        </div>
      )}

      {AtualizarConfiguracao && priorizarFoco && (
        <button
          type="button"
          className="revisao-foco-lista-btn"
          title={t("Ver e gerenciar o grupo de foco")}
          onClick={() => setPopupAberto(true)}
        >
          <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
            <line x1="8" y1="6" x2="21" y2="6" />
            <line x1="8" y1="12" x2="21" y2="12" />
            <line x1="8" y1="18" x2="21" y2="18" />
            <line x1="3" y1="6" x2="3.01" y2="6" />
            <line x1="3" y1="12" x2="3.01" y2="12" />
            <line x1="3" y1="18" x2="3.01" y2="18" />
          </svg>
          {foco.length > 0 && <span className="revisao-foco-lista-contador">{foco.length}</span>}
        </button>
      )}

      {popupAberto && AtualizarConfiguracao && (
        <FocoPopup
          configuracoesApp={configuracoesApp}
          AtualizarConfiguracao={AtualizarConfiguracao}
          foco={foco}
          aoClicarNoFoco={aoClicarNoFoco}
          recarregarFoco={recarregarFoco}
          aoFechar={() => setPopupAberto(false)}
        />
      )}
    </div>
  );
}
