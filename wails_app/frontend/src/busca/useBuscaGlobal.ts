// ----- Seção: Busca Global — adaptador da busca do cabeçalho -----
// Fino wrapper sobre useBuscaDicionario (o núcleo compartilhado) preservando a API nomeada que o
// App e a AbaBuscaGlobal já consomem. A lógica de debounce/consulta vive no núcleo, reaproveitada
// também pela adição ao grupo de foco.
import { useBuscaDicionario } from './useBuscaDicionario';

export function useBuscaGlobal() {
  const { termo, setTermo, resultados } = useBuscaDicionario();
  return {
    termoBuscaGlobal: termo,
    setTermoBuscaGlobal: setTermo,
    resultadosBuscaGlobal: resultados,
  };
}
