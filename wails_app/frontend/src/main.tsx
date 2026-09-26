import React, { useEffect, useState } from 'react';
import { createRoot } from 'react-dom/client';
import './style.css';
import App from './App';
import { LimiteDeErro } from './comum/LimiteDeErro';
import { GetConfig, ObterEstadoAtualizacao } from '../wailsjs/go/main/App';
import { main } from '../wailsjs/go/models';
import { EventsOn } from '../wailsjs/runtime/runtime';
import { definirIdioma } from './i18n/i18n';
import { TelaAtualizacao } from './atualizacao/TelaAtualizacao';

const container = document.getElementById('root');
const root = createRoot(container!);

interface RaizAplicacaoProps {
  estadoInicial: main.EstadoAtualizacao | null;
}

function RaizAplicacao({ estadoInicial }: RaizAplicacaoProps) {
  const faseInicial = estadoInicial?.fase || 'normal';
  const noBootAtualizandoOuFalhou = faseInicial === 'atualizando' || faseInicial === 'falhou';

  const [montarApp, setMontarApp] = useState<boolean>(!noBootAtualizandoOuFalhou);
  const [mostrarTelaAtualizacao, setMostrarTelaAtualizacao] = useState<boolean>(noBootAtualizandoOuFalhou);
  const [versaoAlvo, setVersaoAlvo] = useState<string>(estadoInicial?.versaoAlvo || '');
  const [erro, setErro] = useState<string>(estadoInicial?.erro || '');

  useEffect(() => {
    const cancelarIniciada = EventsOn('atualizacao_iniciada', () => {
      setErro('');
      setMostrarTelaAtualizacao(true);
      // O evento não traz payload: a versão-alvo do fluxo manual vem do estado do backend.
      ObterEstadoAtualizacao()
        .then(estado => setVersaoAlvo(estado.versaoAlvo || ''))
        .catch(() => { /* a tela segue sem a versão */ });
    });

    const cancelarFalhou = EventsOn('atualizacao_falhou', (msgErro: any) => {
      const texto = typeof msgErro === 'string' ? msgErro : (msgErro?.toString?.() || 'Falha na atualização');
      setErro(texto);
      setMostrarTelaAtualizacao(true);
    });

    // Uma falha rápida no boot (ex.: 404 do pacote) pode ter sido emitida antes de estes listeners
    // existirem: com eles registrados, confere o estado de novo para a tela não ficar presa.
    if (faseInicial === 'atualizando') {
      ObterEstadoAtualizacao()
        .then(estado => {
          if (estado.fase === 'falhou') {
            setErro(estado.erro || 'Falha na atualização');
          }
        })
        .catch(() => { /* segue aguardando os eventos */ });
    }

    return () => {
      cancelarIniciada();
      cancelarFalhou();
    };
  }, []);

  const aoContinuar = () => {
    setMostrarTelaAtualizacao(false);
    setMontarApp(true);
  };

  return (
    <>
      {montarApp && <App />}
      {mostrarTelaAtualizacao && (
        <TelaAtualizacao
          versaoAlvo={versaoAlvo}
          erroInicial={erro}
          aoContinuar={aoContinuar}
        />
      )}
    </>
  );
}

function renderizar(estadoInicial: main.EstadoAtualizacao | null) {
  root.render(
    <React.StrictMode>
      <LimiteDeErro>
        <RaizAplicacao estadoInicial={estadoInicial} />
      </LimiteDeErro>
    </React.StrictMode>
  );
}

Promise.all([
  GetConfig()
    .then(cfg => definirIdioma(cfg.idiomaTraducao))
    .catch(() => { /* mantém pt-BR padrão */ }),
  ObterEstadoAtualizacao()
    .catch(() => null)
])
  .then(([_, estadoInicial]) => {
    renderizar(estadoInicial);
  })
  .catch(() => {
    renderizar(null);
  });


