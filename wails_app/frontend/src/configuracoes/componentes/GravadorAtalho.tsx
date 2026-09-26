// ----- Componente: Gravador de Atalho de Teclado Interativo -----
import React, { useState, useEffect, useRef } from 'react';
import { t } from '../../i18n/i18n';
import './GravadorAtalho.css';

interface GravadorAtalhoProps {
  valor: string;
  valorPadrao?: string;
  aoAlterar: (novoValor: string) => void;
  temConflito?: boolean;
  mensagemErro?: string;
}

export function GravadorAtalho({
  valor,
  valorPadrao,
  aoAlterar,
  temConflito,
  mensagemErro,
}: GravadorAtalhoProps) {
  const [gravando, setGravando] = useState(false);
  const containerRef = useRef<HTMLDivElement>(null);

  // Fecha modo de gravação ao clicar fora
  useEffect(() => {
    function aoClicarFora(e: MouseEvent) {
      if (containerRef.current && !containerRef.current.contains(e.target as Node)) {
        setGravando(false);
      }
    }
    if (gravando) {
      window.addEventListener('mousedown', aoClicarFora);
      return () => window.removeEventListener('mousedown', aoClicarFora);
    }
  }, [gravando]);

  // Captura as teclas pressionadas durante o modo de gravação
  const aoTeclar = (e: React.KeyboardEvent) => {
    if (!gravando) return;

    e.preventDefault();
    e.stopPropagation();

    // Se pressionar Esc isolado, cancela a gravação sem alterar
    if (e.key === 'Escape' && !e.ctrlKey && !e.shiftKey && !e.altKey && !e.metaKey) {
      setGravando(false);
      return;
    }

    // Se pressionar Backspace isolado, limpa o atalho
    if (e.key === 'Backspace' && !e.ctrlKey && !e.shiftKey && !e.altKey && !e.metaKey) {
      aoAlterar('');
      setGravando(false);
      return;
    }

    const partes: string[] = [];

    if (e.ctrlKey) partes.push('ctrl');
    if (e.shiftKey) partes.push('shift');
    if (e.altKey) partes.push('alt');
    if (e.metaKey) partes.push('win');

    // Ignora quando apenas os modificadores forem pressionados sozinhos
    const teclaNormal = e.key.toLowerCase();
    if (['control', 'shift', 'alt', 'meta'].includes(teclaNormal)) {
      return;
    }

    // Formatação de teclas conhecidas
    let nomeTecla = teclaNormal;
    if (nomeTecla === ' ') nomeTecla = 'space';
    else if (nomeTecla === 'arrowup') nomeTecla = 'up';
    else if (nomeTecla === 'arrowdown') nomeTecla = 'down';
    else if (nomeTecla === 'arrowleft') nomeTecla = 'left';
    else if (nomeTecla === 'arrowright') nomeTecla = 'right';

    partes.push(nomeTecla);

    const resultado = partes.join('+');
    aoAlterar(resultado);
    setGravando(false);
  };

  // Renderiza a combinação como badges formatados (pills <kbd>)
  const renderizarTeclas = (combo: string) => {
    if (!combo) {
      return <span className="gravador-vazio">{t('Nenhum atalho')}</span>;
    }
    return combo.split('+').map((tecla, idx) => {
      let label = tecla.toUpperCase();
      if (tecla === 'ctrl') label = 'Ctrl';
      else if (tecla === 'shift') label = 'Shift';
      else if (tecla === 'alt') label = 'Alt';
      else if (tecla === 'win') label = 'Win';
      else if (tecla === 'space') label = 'Espaço';
      else if (tecla === 'esc') label = 'Esc';

      return (
        <kbd key={idx} className="tecla-badge">
          {label}
        </kbd>
      );
    });
  };

  return (
    <div className={`gravador-atalho-wrapper ${temConflito ? 'com-conflito' : ''}`} ref={containerRef}>
      <div className="gravador-atalho-linha">
        <div
          className={`gravador-atalho-box ${gravando ? 'gravando' : ''}`}
          onClick={() => setGravando(true)}
          onKeyDown={aoTeclar}
          tabIndex={0}
          role="button"
          aria-label={t('Gravar atalho de teclado')}
        >
          {gravando ? (
            <span className="gravador-instrucao">{t('Pressione as teclas... (Esc para cancelar)')}</span>
          ) : (
            <div className="gravador-conteudo-teclas">{renderizarTeclas(valor)}</div>
          )}
        </div>

        <div className="gravador-acoes">
          {valor && (
            <button
              type="button"
              className="btn-limpar-atalho"
              onClick={(e) => {
                e.stopPropagation();
                aoAlterar('');
                setGravando(false);
              }}
              title={t('Remover atalho')}
            >
              ✕
            </button>
          )}
          {valorPadrao && valor !== valorPadrao && (
            <button
              type="button"
              className="btn-restaurar-atalho"
              onClick={(e) => {
                e.stopPropagation();
                aoAlterar(valorPadrao);
                setGravando(false);
              }}
              title={t('Restaurar padrão')}
            >
              ↺
            </button>
          )}
        </div>
      </div>

      {mensagemErro && <div className="gravador-mensagem-erro">{mensagemErro}</div>}
    </div>
  );
}
