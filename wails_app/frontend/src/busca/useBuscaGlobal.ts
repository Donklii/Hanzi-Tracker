// ----- Seção: Busca Global — estado e debounce -----
// Isola o termo digitado no cabeçalho e os resultados vindos do dicionário geral. A consulta só
// dispara após o usuário parar de digitar (debounce), evitando uma ida ao Go por tecla.
import { useEffect, useState } from 'react';
import { main } from '../../wailsjs/go/models';
import { BuscarNoDicionarioGeral } from '../../wailsjs/go/main/App';

const DEBOUNCE_BUSCA_MS = 400;


export function useBuscaGlobal() {
  const [termoBuscaGlobal, setTermoBuscaGlobal] = useState('');
  const [resultadosBuscaGlobal, setResultadosBuscaGlobal] = useState<main.FlashcardCard[]>([]);

  useEffect(() => {
    if (!termoBuscaGlobal.trim()) {
      setResultadosBuscaGlobal([]);
      return;
    }

    const temporizador = setTimeout(() => {
      BuscarNoDicionarioGeral(termoBuscaGlobal.trim())
        .then(resultados => setResultadosBuscaGlobal(resultados || []));
    }, DEBOUNCE_BUSCA_MS);

    return () => clearTimeout(temporizador);
  }, [termoBuscaGlobal]);

  return { termoBuscaGlobal, setTermoBuscaGlobal, resultadosBuscaGlobal };
}
