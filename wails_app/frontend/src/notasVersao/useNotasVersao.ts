// ----- Seção: Notas de Versão -----
import { useState, useEffect, useCallback } from 'react';
import { ObterNotasVersaoNaoVistas, MarcarNotasVersaoVistas } from '../../wailsjs/go/main/App';
import { notasversao } from '../../wailsjs/go/models';
import { t } from '../i18n/i18n';

export function useNotasVersao() {
  const [aberto, setAberto] = useState(false);
  const [titulo, setTitulo] = useState('');
  const [notas, setNotas] = useState<notasversao.Nota[]>([]);

  useEffect(() => {
    let ativo = true;

    ObterNotasVersaoNaoVistas()
      .then((notasNaoVistas) => {
        if (!ativo || !notasNaoVistas || notasNaoVistas.length === 0) return;

        setNotas(notasNaoVistas);
        setTitulo(t('O Hanzi Tracker foi atualizado'));
        setAberto(true);
      })
      .catch(() => {
        // Silencioso conforme especificação
      });

    return () => {
      ativo = false;
    };
  }, []);

  const fechar = useCallback(() => {
    if (notas.length > 0) {
      const ids = notas.map((n) => n.id);
      MarcarNotasVersaoVistas(ids).catch(() => {
        // Silencioso em caso de falha de marcação
      });
    }
    setAberto(false);
  }, [notas]);

  return {
    aberto,
    titulo,
    notas,
    fechar,
  };
}
