// ----- Seção: Configurações — aba Info (sobre o app e créditos) -----
import { BrowserOpenURL } from '../../../wailsjs/runtime/runtime';
import iconeGithub from '../../assets/images/GithubIcon.png';

interface AbaInfoProps {
  termoBusca: string;
}

export function AbaInfo({ termoBusca }: AbaInfoProps) {
  return (
    <>
      {termoBusca && <h3 className="settings-section-title" style={{ marginTop: '32px' }}>Info</h3>}
      <div className="form-group">
        <h3 style={{ marginBottom: '16px' }}>Sobre o Hanzi Tracker</h3>
        <p style={{ color: 'var(--cor-texto-suave)', lineHeight: '1.5', marginBottom: '12px' }}>
          O Hanzi Tracker é uma ferramenta voltada a auxiliar e otimizar o estudo do idioma chinês de forma dinâmica e interativa, fornecendo leitura contextual, revisão estruturada e reconhecimento óptico de caracteres em tempo real.
        </p>

        <h4 style={{ marginTop: '24px', marginBottom: '8px' }}>Créditos</h4>
        <ul style={{ color: 'var(--cor-texto-suave)', lineHeight: '1.5', paddingLeft: '20px', marginBottom: '24px' }}>
          <li><strong>Donklii:</strong> Desenvolvedor principal e criador do projeto.</li>
          <li><strong>makemeahanzi:</strong> Fornecimento de dados de traços e animações gráficas dos caracteres chineses.</li>
        </ul>

        <button
          className="scan-btn"
          onClick={() => BrowserOpenURL('https://github.com/Donklii/Hanzi-Tracker')}
          style={{ display: 'inline-flex', alignItems: 'center', gap: '8px', padding: '10px 16px', fontSize: '14px' }}
        >
          <img src={iconeGithub} width="20" height="20" alt="GitHub" style={{ filter: 'invert(1)' }} />
          Acessar Repositório no GitHub
        </button>
      </div>
    </>
  );
}
