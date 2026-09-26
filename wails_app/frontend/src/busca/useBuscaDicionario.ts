// ----- Seção: Busca no dicionário geral — núcleo reutilizável (estado + debounce) -----
// Coração compartilhado da barra de pesquisa: guarda o termo digitado e os resultados vindos do
// dicionário geral, disparando a consulta só depois que o usuário para de digitar (debounce),
// para não ir ao Go a cada tecla. Usado pela busca global do cabeçalho (useBuscaGlobal) e pela
// adição ao grupo de foco (FocoPopup) — a mesma lógica, um só lugar.
import { useEffect, useState } from 'react';
import { main } from '../../wailsjs/go/models';
import { BuscarNoDicionarioGeral } from '../../wailsjs/go/main/App';

const DEBOUNCE_BUSCA_MS = 400;

export function useBuscaDicionario() {
  const [termo, setTermo] = useState('');
  const [resultados, setResultados] = useState<main.FlashcardCard[]>([]);

  useEffect(() => {
    if (!termo.trim()) {
      setResultados([]);
      return;
    }

    const temporizador = setTimeout(() => {
      BuscarNoDicionarioGeral(termo.trim())
        .then(resultados => setResultados(resultados || []))
        .catch(() => setResultados([]));
    }, DEBOUNCE_BUSCA_MS);

    return () => clearTimeout(temporizador);
  }, [termo]);

  return { termo, setTermo, resultados };
}
