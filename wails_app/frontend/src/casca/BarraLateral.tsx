// ----- Seção: Casca — barra lateral de navegação -----
// Navegação principal + cartão de foco (o hanzi sob o mouse) + acesso às configurações.
// Os botões de aba são gerados a partir de GRUPOS_NAVEGACAO: adicionar uma aba é adicionar uma
// entrada no registro, não copiar mais um bloco de <button>.
import { CSSProperties, Fragment, ReactNode } from 'react';
import './BarraLateral.css';
import { ABAS, Aba } from './abas';

const ESTILO_ROTULO_GRUPO: CSSProperties = {
  fontSize: '11px',
  color: 'var(--cor-texto-suave)',
  textTransform: 'uppercase',
  fontWeight: 'bold',
};

const ESTILO_CARTAO_FOCO: CSSProperties = {
  backgroundColor: 'var(--cor-fundo-secundario)',
  padding: '12px',
  borderRadius: '8px',
  marginBottom: '16px',
  border: '1px solid var(--cor-borda)',
  minHeight: '120px',
  display: 'flex',
  flexDirection: 'column',
  alignItems: 'center',
  justifyContent: 'center',
  textAlign: 'center',
};

const AtributosIcone = {
  width: 18,
  height: 18,
  viewBox: '0 0 24 24',
  fill: 'none',
  stroke: 'currentColor',
  strokeWidth: 2,
} as const;

interface ItemNavegacao {
  aba: Aba;
  rotulo: string;
  icone: ReactNode;
}

interface GrupoNavegacao {
  titulo: string;
  margem: string;
  itens: ItemNavegacao[];
}

const GRUPOS_NAVEGACAO: GrupoNavegacao[] = [
  {
    titulo: 'Sessão Atual',
    margem: '10px 0 5px 10px',
    itens: [
      {
        aba: ABAS.Descobrimento,
        rotulo: 'Descobrimento',
        icone: (
          <svg {...AtributosIcone}>
            <path d="M2 3h6a4 4 0 0 1 4 4v14a3 3 0 0 0-3-3H2z"></path>
            <path d="M22 3h-6a4 4 0 0 0-4 4v14a3 3 0 0 1 3-3h7z"></path>
          </svg>
        ),
      },
      {
        aba: ABAS.TelaUnica,
        rotulo: 'Palavras Dessa Seção',
        icone: (
          <svg {...AtributosIcone}>
            <polygon points="12 2 2 7 12 12 22 7 12 2"></polygon>
            <polyline points="2 17 12 22 22 17"></polyline>
            <polyline points="2 12 12 17 22 12"></polyline>
          </svg>
        ),
      },
    ],
  },
  {
    titulo: 'Banco de Dados',
    margem: '20px 0 5px 10px',
    itens: [
      {
        aba: ABAS.Vistas,
        rotulo: 'Já Vistas',
        icone: (
          <svg {...AtributosIcone}>
            <path d="M1 12s4-8 11-8 11 8 11 8-4 8-11 8-11-8-11-8z"></path>
            <circle cx="12" cy="12" r="3"></circle>
          </svg>
        ),
      },
      {
        aba: ABAS.Estudando,
        rotulo: 'Estudando',
        icone: (
          <svg {...AtributosIcone}>
            <path d="M4 19.5A2.5 2.5 0 0 1 6.5 17H20"></path>
            <path d="M6.5 2H20v20H6.5A2.5 2.5 0 0 1 4 19.5v-15A2.5 2.5 0 0 1 6.5 2z"></path>
          </svg>
        ),
      },
      {
        aba: ABAS.Aprendidas,
        rotulo: 'Vocabulário',
        icone: (
          <svg {...AtributosIcone}>
            <path d="M22 11.08V12a10 10 0 1 1-5.93-9.14"></path>
            <polyline points="22 4 12 14.01 9 11.01"></polyline>
          </svg>
        ),
      },
    ],
  },
  {
    titulo: 'Prática',
    margem: '20px 0 5px 10px',
    itens: [
      {
        aba: ABAS.Revisao,
        rotulo: 'Revisão',
        icone: (
          <svg {...AtributosIcone}>
            <path d="M12 20h9"></path>
            <path d="M16.5 3.5a2.121 2.121 0 0 1 3 3L7 19l-4 1 1-4L16.5 3.5z"></path>
          </svg>
        ),
      },
    ],
  },
];

interface BarraLateralProps {
  abaAtiva: Aba;
  aoTrocarAba: (aba: Aba) => void;
  cartaoEmFoco: any | null;
  aoAbrirConfiguracoes: () => void;
}


export function BarraLateral({ abaAtiva, aoTrocarAba, cartaoEmFoco, aoAbrirConfiguracoes }: BarraLateralProps) {
  return (
    <div className="sidebar">
      <h1>Hanzi Tracker</h1>

      {/* Fragment (não <div>): os rótulos e botões precisam continuar sendo filhos diretos do
          flex container .sidebar, senão o .sidebar-spacer perde o empurrão para o rodapé. */}
      {GRUPOS_NAVEGACAO.map(grupo => (
        <Fragment key={grupo.titulo}>
          <div style={{ ...ESTILO_ROTULO_GRUPO, margin: grupo.margem }}>{grupo.titulo}</div>
          {grupo.itens.map(item => (
            <button
              key={item.aba}
              className={`sidebar-btn ${abaAtiva === item.aba ? 'active' : ''}`}
              onClick={() => aoTrocarAba(item.aba)}
            >
              {item.icone}
              {item.rotulo}
            </button>
          ))}
        </Fragment>
      ))}

      <div className="sidebar-spacer"></div>

      <CartaoEmFoco cartaoEmFoco={cartaoEmFoco} />

      <button className="sidebar-btn" onClick={aoAbrirConfiguracoes}>
        <svg {...AtributosIcone}>
          <circle cx="12" cy="12" r="3"></circle>
          <path d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 0 1 0 2.83 2 2 0 0 1-2.83 0l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 0 1-2 2 2 2 0 0 1-2-2v-.09A1.65 1.65 0 0 0 9 19.4a1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 0 1-2.83 0 2 2 0 0 1 0-2.83l.06-.06a1.65 1.65 0 0 0 .33-1.82 1.65 1.65 0 0 0-1.51-1H3a2 2 0 0 1-2-2 2 2 0 0 1 2-2h.09A1.65 1.65 0 0 0 4.6 9a1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 0 1 0-2.83 2 2 0 0 1 2.83 0l.06.06a1.65 1.65 0 0 0 1.82.33H9a1.65 1.65 0 0 0 1-1.51V3a2 2 0 0 1 2-2 2 2 0 0 1 2 2v.09a1.65 1.65 0 0 0 1 1.51 1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 0 1 2.83 0 2 2 0 0 1 0 2.83l-.06.06a1.65 1.65 0 0 0-.33 1.82V9a1.65 1.65 0 0 0 1.51 1H21a2 2 0 0 1 2 2 2 2 0 0 1-2 2h-.09a1.65 1.65 0 0 0-1.51 1z"></path>
        </svg>
        Configurações
      </button>
    </div>
  );
}


// Painel inferior da barra: mostra o cartão sob o mouse (rastreador global de hover).
function CartaoEmFoco({ cartaoEmFoco }: { cartaoEmFoco: any | null }) {
  if (!cartaoEmFoco) {
    return (
      <div style={ESTILO_CARTAO_FOCO}>
        <div style={{ color: 'var(--cor-texto-suave)', fontSize: '12px' }}>
          Passe o mouse sobre um texto chinês para focar
        </div>
      </div>
    );
  }

  return (
    <div style={ESTILO_CARTAO_FOCO}>
      <div style={{ color: 'var(--cor-destaque)', fontSize: '12px' }}>{cartaoEmFoco.pinyin}</div>
      <div style={{ fontSize: '28px', fontWeight: 'bold', margin: '4px 0' }}>{cartaoEmFoco.hanzi}</div>
      <div style={{ fontSize: '11px', color: 'var(--cor-texto-suave)' }}>
        {cartaoEmFoco.significados ? cartaoEmFoco.significados.join(', ') : 'Sem tradução'}
      </div>
    </div>
  );
}
