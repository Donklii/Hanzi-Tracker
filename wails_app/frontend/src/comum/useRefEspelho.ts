// ----- Seção: Ref espelho (compartilhado) -----
// Mantém um ref sempre igual ao último valor de um state.
//
// Serve para handlers registrados UMA vez (EventsOn do Wails, listeners de window): o closure deles
// é o do primeiro render e enxergaria o state congelado — mas um ref é sempre o mesmo objeto, então
// ler `.current` de dentro do handler devolve o valor atual.
import { MutableRefObject, useEffect, useRef } from 'react';


export function useRefEspelho<T>(valor: T): MutableRefObject<T> {
  const ref = useRef<T>(valor);

  useEffect(() => {
    ref.current = valor;
  }, [valor]);

  return ref;
}
