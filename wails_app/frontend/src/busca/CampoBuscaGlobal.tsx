// ----- Seção: Busca Global — campo do cabeçalho -----
// Entrada de texto para buscar no dicionário geral, com atalho para a busca por desenho do hanzi.
import { CSSProperties } from 'react';

const ESTILO_ENTRADA: CSSProperties = {
  padding: '8px 36px 8px 32px',
  borderRadius: '8px',
  border: '1px solid var(--cor-borda)',
  backgroundColor: 'var(--cor-fundo-secundario)',
  color: 'var(--cor-texto-primario)',
  fontSize: '13px',
  width: '240px',
};

const ESTILO_ICONE_LUPA: CSSProperties = {
  position: 'absolute',
  left: '10px',
  top: '50%',
  transform: 'translateY(-50%)',
  color: 'var(--cor-texto-suave)',
};

const ESTILO_BOTAO_DESENHO: CSSProperties = {
  position: 'absolute',
  right: '10px',
  top: '50%',
  transform: 'translateY(-50%)',
  cursor: 'pointer',
  color: 'var(--cor-texto-suave)',
  display: 'flex',
  alignItems: 'center',
  justifyContent: 'center',
};

interface CampoBuscaGlobalProps {
  termoBuscaGlobal: string;
  aoMudarTermo: (termo: string) => void;
  aoAbrirBuscaPorDesenho: () => void;
}


export function CampoBuscaGlobal({ termoBuscaGlobal, aoMudarTermo, aoAbrirBuscaPorDesenho }: CampoBuscaGlobalProps) {
  return (
    <div style={{ position: 'relative', display: 'flex', alignItems: 'center' }}>
      <div style={{ position: 'relative' }}>
        <svg style={ESTILO_ICONE_LUPA} width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
          <circle cx="11" cy="11" r="8"></circle>
          <line x1="21" y1="21" x2="16.65" y2="16.65"></line>
        </svg>

        <input
          type="text"
          placeholder="Pesquisar hanzi, pinyin..."
          value={termoBuscaGlobal}
          onChange={(e) => aoMudarTermo(e.target.value)}
          style={ESTILO_ENTRADA}
        />

        <div
          onClick={aoAbrirBuscaPorDesenho}
          style={ESTILO_BOTAO_DESENHO}
          title="Pesquisar por desenho"
          onMouseEnter={(e) => e.currentTarget.style.color = 'var(--cor-texto-primario)'}
          onMouseLeave={(e) => e.currentTarget.style.color = 'var(--cor-texto-suave)'}
        >
          <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
            <path d="M12 20h9"></path>
            <path d="M16.5 3.5a2.121 2.121 0 0 1 3 3L7 19l-4 1 1-4L16.5 3.5z"></path>
          </svg>
        </div>
      </div>
    </div>
  );
}
