// ----- Seção: Busca Global — campo do cabeçalho -----
// Entrada de texto para buscar no dicionário geral, com atalho para a busca por desenho do hanzi.
import { CSSProperties } from 'react';
import { t } from '../i18n/i18n';

const ESTILO_ENTRADA: CSSProperties = {
  padding: '8px 106px 8px 32px', // Space for up to 4 icons
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

const ESTILO_BOTAO_RANKING: CSSProperties = {
  position: 'absolute',
  right: '34px',
  top: '50%',
  transform: 'translateY(-50%)',
  cursor: 'pointer',
  display: 'flex',
  alignItems: 'center',
  justifyContent: 'center',
};

const ESTILO_BOTAO_HSK: CSSProperties = {
  position: 'absolute',
  right: '58px',
  top: '50%',
  transform: 'translateY(-50%)',
  cursor: 'pointer',
  display: 'flex',
  alignItems: 'center',
  justifyContent: 'center',
  fontWeight: 'bold',
  fontSize: '11px',
  userSelect: 'none',
};

const ESTILO_BOTAO_GLOBAL: CSSProperties = {
  position: 'absolute',
  right: '82px',
  top: '50%',
  transform: 'translateY(-50%)',
  cursor: 'pointer',
  display: 'flex',
  alignItems: 'center',
  justifyContent: 'center',
};

interface CampoBuscaGlobalProps {
  termoBuscaGlobal: string;
  aoMudarTermo: (termo: string) => void;
  aoAbrirBuscaPorDesenho: () => void;
  ordenarPorRanking: boolean;
  aoAlternarRanking: () => void;
  buscaGlobalAtiva: boolean;
  aoAlternarGlobal: () => void;
}

export function CampoBuscaGlobal({
  termoBuscaGlobal,
  aoMudarTermo,
  aoAbrirBuscaPorDesenho,
  ordenarPorRanking,
  aoAlternarRanking,
  buscaGlobalAtiva,
  aoAlternarGlobal
}: CampoBuscaGlobalProps) {
  const isHSKActive = termoBuscaGlobal.startsWith("[HSK]");
  const mostrarBotaoGlobal = isHSKActive || ordenarPorRanking;

  return (
    <div style={{ position: 'relative', display: 'flex', alignItems: 'center' }}>
      <div style={{ position: 'relative' }}>
        <svg style={ESTILO_ICONE_LUPA} width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
          <circle cx="11" cy="11" r="8"></circle>
          <line x1="21" y1="21" x2="16.65" y2="16.65"></line>
        </svg>

        <input
          type="text"
          placeholder={t("Pesquisar hanzi, pinyin...")}
          value={termoBuscaGlobal}
          onChange={(e) => aoMudarTermo(e.target.value)}
          style={{
            ...ESTILO_ENTRADA,
            paddingRight: mostrarBotaoGlobal ? '106px' : '82px',
          }}
        />

        {/* Botão Global (Globo) - visível apenas se HSK ou Ranking estiver ativo */}
        {mostrarBotaoGlobal && (
          <div
            onClick={aoAlternarGlobal}
            style={{
              ...ESTILO_BOTAO_GLOBAL,
              color: buscaGlobalAtiva ? 'var(--cor-destaque)' : 'var(--cor-texto-suave)',
            }}
            title={buscaGlobalAtiva ? t("Desativar busca no banco de dados global") : t("Ativar busca no banco de dados global")}
            onMouseEnter={(e) => {
              if (!buscaGlobalAtiva) {
                e.currentTarget.style.color = 'var(--cor-texto-primario)';
              }
            }}
            onMouseLeave={(e) => {
              if (!buscaGlobalAtiva) {
                e.currentTarget.style.color = 'var(--cor-texto-suave)';
              }
            }}
          >
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" style={{ width: '15px', height: '15px' }}>
              <circle cx="12" cy="12" r="10" />
              <line x1="2" y1="12" x2="22" y2="12" />
              <path d="M12 2a15.3 15.3 0 0 1 4 10 15.3 15.3 0 0 1-4 10 15.3 15.3 0 0 1-4-10 15.3 15.3 0 0 1 4-10z" />
            </svg>
          </div>
        )}

        {/* Botão de Pesquisa HSK */}
        <div
          onClick={() => {
            if (isHSKActive) {
              aoMudarTermo("");
            } else {
              if (ordenarPorRanking) {
                aoAlternarRanking();
              }
              aoMudarTermo("[HSK]");
            }
          }}
          style={{
            ...ESTILO_BOTAO_HSK,
            color: isHSKActive ? 'var(--cor-destaque)' : 'var(--cor-texto-suave)',
          }}
          title={isHSKActive ? t("Limpar pesquisa HSK") : t("Mostrar vocabulário HSK")}
          onMouseEnter={(e) => {
            if (!isHSKActive) {
              e.currentTarget.style.color = 'var(--cor-texto-primario)';
            }
          }}
          onMouseLeave={(e) => {
            if (!isHSKActive) {
              e.currentTarget.style.color = 'var(--cor-texto-suave)';
            }
          }}
        >
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" style={{ width: '16px', height: '16px' }}>
            <path d="M4 19.5A2.5 2.5 0 0 1 6.5 17H20" />
            <path d="M6.5 2H20v20H6.5A2.5 2.5 0 0 1 4 19.5v-15A2.5 2.5 0 0 1 6.5 2z" />
            <text x="7.5" y="13" fill="currentColor" stroke="none" fontSize="6.5" fontWeight="bold" fontFamily="sans-serif">HSK</text>
          </svg>
        </div>

        {/* Botão de Ordenar por Ranking */}
        <div
          onClick={() => {
            if (ordenarPorRanking) {
              aoAlternarRanking();
            } else {
              if (isHSKActive) {
                aoMudarTermo("");
              }
              aoAlternarRanking();
            }
          }}
          style={{
            ...ESTILO_BOTAO_RANKING,
            color: ordenarPorRanking ? 'var(--cor-destaque)' : 'var(--cor-texto-suave)',
          }}
          title={ordenarPorRanking ? t("Desativar ordenação por ranking") : t("Ordenar resultados por ranking")}
          onMouseEnter={(e) => {
            if (!ordenarPorRanking) {
              e.currentTarget.style.color = 'var(--cor-texto-primario)';
            }
          }}
          onMouseLeave={(e) => {
            if (!ordenarPorRanking) {
              e.currentTarget.style.color = 'var(--cor-texto-suave)';
            }
          }}
        >
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5" strokeLinecap="round" strokeLinejoin="round" style={{ width: '15px', height: '15px' }}>
            <line x1="18" y1="20" x2="18" y2="10" />
            <line x1="12" y1="20" x2="12" y2="4" />
            <line x1="6" y1="20" x2="6" y2="14" />
          </svg>
        </div>

        {/* Botão de Desenho */}
        <div
          onClick={aoAbrirBuscaPorDesenho}
          style={ESTILO_BOTAO_DESENHO}
          title={t("Pesquisar por desenho")}
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
