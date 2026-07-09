// ----- Seção: Configurações — uso de disco e limpeza -----
// Estado e operações da aba Armazenamento: consulta o espaço ocupado por categoria, limpa uma
// categoria isolada ou apaga tudo. As escritas destrutivas ficam com `ocupado` ligado para a aba
// desabilitar os botões enquanto o Go trabalha.
import { useState } from 'react';
import { main } from '../../wailsjs/go/models';
import { GetStorageInfo, LimparArmazenamento, ExcluirTudo } from '../../wailsjs/go/main/App';

interface OpcoesUseArmazenamento {
  setStatus: (mensagem: string) => void;
  // Chamado após ExcluirTudo: o que foi apagado do disco (modelos, vocabulário) precisa ser relido.
  aoExcluirTudo: () => void;
}


export function useArmazenamento({ setStatus, aoExcluirTudo }: OpcoesUseArmazenamento) {
  const [infoArmazenamento, setInfoArmazenamento] = useState<main.StorageInfo | null>(null);
  const [armazenamentoOcupado, setArmazenamentoOcupado] = useState(false);

  const LimparCategoriaArmazenamento = (chave: string) => {
    setArmazenamentoOcupado(true);
    LimparArmazenamento(chave)
      .then(() => CarregarArmazenamento())
      .catch((err: any) => setStatus('⚠️ ' + String(err)))
      .finally(() => setArmazenamentoOcupado(false));
  };

  const ExcluirTodoArmazenamento = () => {
    setArmazenamentoOcupado(true);
    ExcluirTudo()
      .then(() => {
        CarregarArmazenamento();
        aoExcluirTudo();
        setStatus('Armazenamento limpo.');
      })
      .catch((err: any) => setStatus('⚠️ ' + String(err)))
      .finally(() => setArmazenamentoOcupado(false));
  };

  // Refresh best-effort do painel: se a leitura do disco falhar, a aba só fica sem os números —
  // não há ação do usuário para desfazer nem estado corrompido a reportar.
  const CarregarArmazenamento = () => {
    GetStorageInfo().then(info => setInfoArmazenamento(info)).catch(() => { });
  };

  return {
    infoArmazenamento,
    armazenamentoOcupado,
    CarregarArmazenamento,
    LimparCategoriaArmazenamento,
    ExcluirTodoArmazenamento,
  };
}
