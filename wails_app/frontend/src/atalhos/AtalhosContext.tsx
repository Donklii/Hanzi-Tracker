// ----- Contexto Central de Atalhos de Teclado Reativos -----
import React, { createContext, useContext, useState, useEffect, useCallback, ReactNode } from 'react';

export type CategoriaAtalho = 'global' | 'revisao' | 'dicionario' | 'geral';

export interface AtalhoItem {
  id: string;
  combo: string;
  descricao: string;
  categoria: CategoriaAtalho;
  acao: (e: KeyboardEvent) => void;
  ignorarCamposTexto?: boolean; // Padrão: true (não dispara enquanto digita em input/textarea)
}

interface AtalhosContextType {
  atalhosLocais: AtalhoItem[];
  registrarAtalho: (item: AtalhoItem) => () => void;
  guiaVisivel: boolean;
  setGuiaVisivel: (visivel: boolean) => void;
  abrirGuia: () => void;
  fecharGuia: () => void;
}

const AtalhosContext = createContext<AtalhosContextType | undefined>(undefined);

export function AtalhosProvider({ children }: { children: ReactNode }) {
  const [atalhosMap, setAtalhosMap] = useState<Map<string, AtalhoItem>>(new Map());
  const [guiaVisivel, setGuiaVisivel] = useState(false);

  const abrirGuia = useCallback(() => setGuiaVisivel(true), []);
  const fecharGuia = useCallback(() => setGuiaVisivel(false), []);

  const registrarAtalho = useCallback((item: AtalhoItem) => {
    setAtalhosMap((prev) => {
      const proximo = new Map(prev);
      proximo.set(item.id, item);
      return proximo;
    });

    return () => {
      setAtalhosMap((prev) => {
        const proximo = new Map(prev);
        proximo.delete(item.id);
        return proximo;
      });
    };
  }, []);

  // Listener global da janela React para despachar atalhos locais registrados
  useEffect(() => {
    function aoTeclarGlobal(e: KeyboardEvent) {
      // Ignora se estiver digitando em campo de texto (salvo se ignorarCamposTexto for false)
      const elementoAtivo = document.activeElement;
      const ehCampoTexto =
        elementoAtivo &&
        (elementoAtivo.tagName === 'INPUT' ||
          elementoAtivo.tagName === 'TEXTAREA' ||
          (elementoAtivo as HTMLElement).isContentEditable);

      // Atalho nativo para abrir a Guia de Atalhos: Shift+? ou '?' (quando fora de campo de texto)
      if (!ehCampoTexto && (e.key === '?' || (e.shiftKey && e.key === '/'))) {
        e.preventDefault();
        setGuiaVisivel((v) => !v);
        return;
      }

      // Converte o evento do teclado numa string normalizada de combinação
      const partes: string[] = [];
      if (e.ctrlKey) partes.push('ctrl');
      if (e.shiftKey) partes.push('shift');
      if (e.altKey) partes.push('alt');
      if (e.metaKey) partes.push('win');

      const teclaLower = e.key.toLowerCase();
      if (!['control', 'shift', 'alt', 'meta'].includes(teclaLower)) {
        let nomeTecla = teclaLower;
        if (nomeTecla === ' ') nomeTecla = 'space';
        else if (nomeTecla === 'escape') nomeTecla = 'esc';
        partes.push(nomeTecla);
      }

      const comboNormalizado = partes.join('+');

      for (const item of atalhosMap.values()) {
        const comboItem = item.combo.toLowerCase().trim();
        const ignorarEmTexto = item.ignorarCamposTexto !== false;

        if (ehCampoTexto && ignorarEmTexto) {
          continue;
        }

        if (comboNormalizado === comboItem || e.key.toLowerCase() === comboItem) {
          item.acao(e);
          break;
        }
      }
    }

    window.addEventListener('keydown', aoTeclarGlobal);
    return () => window.removeEventListener('keydown', aoTeclarGlobal);
  }, [atalhosMap]);

  return (
    <AtalhosContext.Provider
      value={{
        atalhosLocais: Array.from(atalhosMap.values()),
        registrarAtalho,
        guiaVisivel,
        setGuiaVisivel,
        abrirGuia,
        fecharGuia,
      }}
    >
      {children}
    </AtalhosContext.Provider>
  );
}

export function useAtalhos() {
  const context = useContext(AtalhosContext);
  if (!context) {
    throw new Error('useAtalhos deve ser utilizado dentro de um <AtalhosProvider>');
  }
  return context;
}

// Hook de conveniência para registrar um atalho durante o ciclo de vida de um componente
export function useRegistrarAtalho(item: AtalhoItem) {
  const { registrarAtalho } = useAtalhos();

  useEffect(() => {
    const desregistrar = registrarAtalho(item);
    return desregistrar;
  }, [item.id, item.combo, item.descricao, item.categoria, item.ignorarCamposTexto, registrarAtalho]);
}
