// ----- Seção: Nuvem — sincronização com o Google Drive -----
// Concentra o estado e as quatro operações de nuvem (conectar, resolver conflito, sincronizar,
// desconectar). Todas compartilham o mesmo esqueleto — ocupa → chama o Go → guarda o Info devolvido
// → libera —, então o esqueleto vive em executarOperacaoNuvem e cada operação só diz o que fazer
// depois que o Info chega.
import { useState } from 'react';
import { nuvem } from '../../wailsjs/go/models';
import {
  GetInfoNuvem, ConectarNuvem, ResolverConflitoNuvem, SincronizarNuvem, DesconectarNuvem,
} from '../../wailsjs/go/main/App';
import { EscolhaConflitoNuvem, ESTADO_NUVEM_CONFLITO } from './tipos';
import { t } from '../i18n/i18n';

interface OpcoesUseNuvem {
  setStatus: (mensagem: string) => void;
  // Chamado quando os dados locais são substituídos pelos da nuvem: quem consome precisa recarregar
  // tudo que veio dos arquivos antigos (vocabulário, uso de disco e as configurações).
  aoRestaurarDaNuvem: () => void;
}


export function useNuvem({ setStatus, aoRestaurarDaNuvem }: OpcoesUseNuvem) {
  const [infoNuvem, setInfoNuvem] = useState<nuvem.Info | null>(null);
  const [nuvemOcupada, setNuvemOcupada] = useState(false);
  const [conflitoNuvemAberto, setConflitoNuvemAberto] = useState(false);

  const executarOperacaoNuvem = (
    operacao: () => Promise<nuvem.Info>,
    aoConcluir: (info: nuvem.Info) => void,
  ) => {
    setNuvemOcupada(true);
    operacao()
      .then(info => {
        setInfoNuvem(info);
        aoConcluir(info);
      })
      .catch((err: any) => setStatus('⚠️ ' + String(err)))
      .finally(() => setNuvemOcupada(false));
  };

  const ConectarNuvemDrive = () => {
    setStatus(t('Autorize o Hanzi Tracker no navegador que acabou de abrir…'));
    executarOperacaoNuvem(ConectarNuvem, info => {
      if (info.estado === ESTADO_NUVEM_CONFLITO) {
        setConflitoNuvemAberto(true); // já existe backup na nuvem: o usuário escolhe um lado
        return;
      }
      setStatus(t('Google Drive conectado — progresso e configurações enviados para a nuvem.'));
    });
  };

  const ResolverConflitoNuvemDrive = (escolha: EscolhaConflitoNuvem) => {
    executarOperacaoNuvem(() => ResolverConflitoNuvem(escolha), () => {
      setConflitoNuvemAberto(false);
      if (escolha === 'usarNuvem') {
        aoRestaurarDaNuvem();
      }
      setStatus(t('Sincronização com o Google Drive concluída.'));
    });
  };

  const SincronizarNuvemDrive = () => {
    executarOperacaoNuvem(SincronizarNuvem, () => setStatus(t('Progresso e configurações sincronizados com o Google Drive.')));
  };

  const DesconectarNuvemDrive = () => {
    executarOperacaoNuvem(DesconectarNuvem, () => {
      setStatus(t('Google Drive desconectado (o backup continua no seu Drive).'));
    });
  };

  // Refresh best-effort exibido na aba Armazenamento: uma falha aqui (offline, token expirado) só
  // deixa o painel sem os dados da nuvem, e o erro real aparece quando o usuário aciona uma operação.
  const CarregarInfoNuvem = () => {
    GetInfoNuvem().then(setInfoNuvem).catch(() => { });
  };

  return {
    infoNuvem,
    nuvemOcupada,
    conflitoNuvemAberto,
    abrirConflitoNuvem: () => setConflitoNuvemAberto(true),
    fecharConflitoNuvem: () => setConflitoNuvemAberto(false),
    CarregarInfoNuvem,
    ConectarNuvemDrive,
    ResolverConflitoNuvemDrive,
    SincronizarNuvemDrive,
    DesconectarNuvemDrive,
  };
}
