// ----- Modal Guia de Atalhos (Cheat Sheet de Teclado) -----
import React from 'react';
import { useAtalhos } from './AtalhosContext';
import { config } from '../../wailsjs/go/models';
import { t } from '../i18n/i18n';
import './ModalGuiaAtalhos.css';

interface ModalGuiaAtalhosProps {
  configuracoesApp?: config.Config | null;
}

export function ModalGuiaAtalhos({ configuracoesApp }: ModalGuiaAtalhosProps) {
  const { guiaVisivel, fecharGuia, atalhosLocais } = useAtalhos();

  if (!guiaVisivel) return null;

  // Lista dos atalhos globais do SO
  const atalhosGlobais = [
    {
      nome: t('Atalho: Escanear Tela'),
      combo: configuracoesApp?.atalhoEscanear || 'ctrl+shift+e',
      desc: t('Aciona o OCR instantâneo da tela principal'),
    },
    {
      nome: t('Atalho: Mostrar Pop-up de Tudo'),
      combo: configuracoesApp?.atalhoPopupTodos || 'ctrl+shift+t',
      desc: t('Alterna a exibição de todos os pop-ups ou tradução por linha'),
    },
    {
      nome: t('Atalho: Marcar Card atual em Estudo'),
      combo: configuracoesApp?.atalhoMarcarEstudo || 'ctrl+shift+m',
      desc: t('Adiciona a palavra focada diretamente ao seu vocabulário'),
    },
    {
      nome: t('Atalho: Ligar/Desligar Pop-up no Cursor'),
      combo: configuracoesApp?.atalhoAlternarPopupHover || 'ctrl+shift+h',
      desc: t('Ativa ou desativa a lupa de vocabulário ao passar o mouse'),
    },
  ];

  // Agrupa os atalhos locais por categoria
  const atalhosGeraisFixos = [
    { combo: '?', desc: t('Abrir / fechar esta Guia de Atalhos') },
    { combo: 'Esc', desc: t('Fechar modais, pop-ups ou cancelar ações') },
    { combo: '1 — 4', desc: t('Escolher alternativas (1ª a 4ª) nas sessões de estudo') },
    { combo: 'Enter / Space', desc: t('Avançar para a próxima questão após responder') },
    { combo: 'Enter', desc: t('Confirmar texto digitado em buscas e formulários') },
  ];

  const renderizarKbd = (combo: string) => {
    if (!combo) return <span className="guia-sem-atalho">{t('Desativado')}</span>;
    return combo.split('+').map((k, i) => (
      <kbd key={i} className="guia-kbd">
        {k.trim().toUpperCase()}
      </kbd>
    ));
  };

  return (
    <div className="guia-modal-backdrop" onClick={fecharGuia}>
      <div className="guia-modal-container" onClick={(e) => e.stopPropagation()}>
        <div className="guia-modal-header">
          <h2>⌨️ {t('Guia de Atalhos de Teclado')}</h2>
          <button className="guia-modal-fechar" onClick={fecharGuia} title={t('Fechar')}>
            ✕
          </button>
        </div>

        <div className="guia-modal-corpo">
          {/* Seção Atalhos Globais (SO) */}
          <section className="guia-secao">
            <h3>🌐 {t('Atalhos Globais (Sistema Operacional)')}</h3>
            <p className="guia-subtitulo">{t('Funcionam mesmo quando o aplicativo está em segundo plano')}</p>

            <div className="guia-grid">
              {atalhosGlobais.map((item, index) => (
                <div key={index} className="guia-card">
                  <div className="guia-card-info">
                    <span className="guia-card-nome">{item.nome}</span>
                    <span className="guia-card-desc">{item.desc}</span>
                  </div>
                  <div className="guia-card-combo">{renderizarKbd(item.combo)}</div>
                </div>
              ))}
            </div>
          </section>

          {/* Seção Atalhos Internos do App */}
          <section className="guia-secao">
            <h3>⚡ {t('Atalhos de Navegação e Estudo')}</h3>

            <div className="guia-grid">
              {atalhosGeraisFixos.map((item, index) => (
                <div key={index} className="guia-card">
                  <span className="guia-card-desc">{item.desc}</span>
                  <div className="guia-card-combo">{renderizarKbd(item.combo)}</div>
                </div>
              ))}

              {atalhosLocais.map((item) => (
                <div key={item.id} className="guia-card">
                  <span className="guia-card-desc">{item.descricao}</span>
                  <div className="guia-card-combo">{renderizarKbd(item.combo)}</div>
                </div>
              ))}
            </div>
          </section>
        </div>

        <div className="guia-modal-footer">
          <span>{t('Dica: Você pode alterar os atalhos globais em Configurações → Atalhos Globais.')}</span>
          <button className="btn-guia-entendi" onClick={fecharGuia}>
            {t('Entendi')}
          </button>
        </div>
      </div>
    </div>
  );
}
