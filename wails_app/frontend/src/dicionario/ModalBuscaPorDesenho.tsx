import React, { useState } from "react";
import { CanvasHanziLookup } from "./CanvasHanziLookup";

interface ModalBuscaPorDesenhoProps {
  isOpen: boolean;
  onClose: () => void;
  onSelect: (hanzi: string) => void;
  configuracoesApp: any;
}

export function ModalBuscaPorDesenho({ isOpen, onClose, onSelect, configuracoesApp }: ModalBuscaPorDesenhoProps) {
  const [sugestoes, setSugestoes] = useState<string[]>([]);

  if (!isOpen) return null;

  return (
    <div className="modal-overlay" onClick={onClose} style={{ zIndex: 1002 }}>
      <div
        className="modal-content"
        style={{ maxWidth: '400px', padding: '24px', flexDirection: 'column', height: 'auto' }}
        onClick={e => e.stopPropagation()}
      >
        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '16px' }}>
          <h2 style={{ fontSize: '18px', margin: 0, fontWeight: 'bold' }}>Pesquisar por Desenho</h2>
          <button
            onClick={onClose}
            style={{
              background: 'none', border: 'none', fontSize: '24px', cursor: 'pointer',
              color: 'var(--cor-texto-suave)', lineHeight: 1
            }}
          >
            ×
          </button>
        </div>

        <div style={{ display: 'flex', flexDirection: 'column', alignItems: 'center' }}>
          <label style={{ fontSize: '12px', color: 'var(--cor-texto-suave)', marginBottom: '8px', alignSelf: 'flex-start' }}>
            Desenhe o Hanzi abaixo:
            <br />
            <span style={{ opacity: 0.8 }}>⚠️ A ordem e a direção dos traços importa para o reconhecimento correto.</span>
          </label>
          <CanvasHanziLookup 
            onRecognize={(sug) => setSugestoes(sug)} 
            configuracoesApp={configuracoesApp}
          />
          
          <div style={{ display: 'flex', flexWrap: 'wrap', gap: '8px', marginTop: '12px', justifyContent: 'center', minHeight: '40px' }}>
            {sugestoes.length === 0 ? (
               <span style={{ fontSize: '12px', color: 'var(--cor-texto-suave)', marginTop: '10px' }}>
                 Nenhuma correspondência ainda.
               </span>
            ) : (
              sugestoes.map((hz, idx) => (
                <div
                  key={idx}
                  className="scan-btn"
                  style={{ fontSize: '20px', padding: '4px 12px', fontFamily: 'var(--fonte-hanzi)', cursor: 'pointer' }}
                  onClick={() => {
                    onSelect(hz);
                    onClose();
                    setSugestoes([]);
                  }}
                >{hz}</div>
              ))
            )}
          </div>
          
          {sugestoes.length > 0 && (
            <button
              className="scan-btn"
              style={{ marginTop: '16px', backgroundColor: 'var(--cor-destaque)', color: 'white' }}
              onClick={() => {
                onSelect('[DESENHO]' + sugestoes.join(''));
                onClose();
                setSugestoes([]);
              }}
            >
              Pesquisar todas as sugestões
            </button>
          )}
        </div>
      </div>
    </div>
  );
}
