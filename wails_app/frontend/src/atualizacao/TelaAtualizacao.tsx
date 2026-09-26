// ----- Seção: Tela de Atualização Automática -----
import React, { useEffect, useState } from 'react';
import { EventsOn } from '../../wailsjs/runtime/runtime';
import { t } from '../i18n/i18n';
import './telaAtualizacao.css';

interface TelaAtualizacaoProps {
  versaoAlvo?: string;
  erroInicial?: string;
  aoContinuar: () => void;
}

export function TelaAtualizacao({ versaoAlvo, erroInicial, aoContinuar }: TelaAtualizacaoProps) {
  const [progresso, setProgresso] = useState<string>(t('Preparando…'));
  const [erro, setErro] = useState<string | null>(erroInicial || null);

  useEffect(() => {
    if (erroInicial) {
      setErro(erroInicial);
    }
  }, [erroInicial]);

  useEffect(() => {
    const cancelarProgresso = EventsOn('atualizacao_progresso', (msg: any) => {
      if (typeof msg === 'string') {
        setProgresso(msg);
      }
    });

    const cancelarFalhou = EventsOn('atualizacao_falhou', (msgErro: any) => {
      if (typeof msgErro === 'string') {
        setErro(msgErro);
      } else if (msgErro && typeof msgErro.toString === 'function') {
        setErro(msgErro.toString());
      } else {
        setErro(t('Erro desconhecido durante a atualização.'));
      }
    });

    return () => {
      cancelarProgresso();
      cancelarFalhou();
    };
  }, []);

  return (
    <div className="tela-atualizacao-overlay">
      <div className="tela-atualizacao-card">
        {erro ? (
          <>
            <h2 className="tela-atualizacao-titulo" style={{ color: '#ef4444' }}>
              {t('Falha na atualização')}
            </h2>
            {versaoAlvo && (
              <p className="tela-atualizacao-versao">
                {t('Versão')} {versaoAlvo}
              </p>
            )}
            <div className="tela-atualizacao-erro-container">
              {erro}
            </div>
            <p className="tela-atualizacao-aviso">
              {t('Não foi possível concluir a atualização automática. Você pode continuar usando a versão instalada.')}
            </p>
            <button
              className="scan-btn"
              onClick={aoContinuar}
              style={{ marginTop: '8px', padding: '10px 24px' }}
            >
              {t('Continuar')}
            </button>
          </>
        ) : (
          <>
            <h2 className="tela-atualizacao-titulo">
              {t('Atualizando o Hanzi Tracker')}
            </h2>
            {versaoAlvo && (
              <p className="tela-atualizacao-versao">
                {t('Versão')} {versaoAlvo}
              </p>
            )}
            <p className="tela-atualizacao-progresso-texto">
              {progresso}
            </p>
            <div className="tela-atualizacao-barra-container">
              <div className="tela-atualizacao-barra-indeterminada" />
            </div>
            <p className="tela-atualizacao-aviso">
              {t('Se o sistema pedir permissão, confirme — o app reabre sozinho.')}
            </p>
          </>
        )}
      </div>
    </div>
  );
}
