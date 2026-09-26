// ----- Seção: Revisão — Botão de Áudio -----
// Componente puramente visual: quem toca/sintetiza o áudio é o pai (AbaRevisao).
import { t } from '../i18n/i18n';

interface BotaoAudioProps {
  rotulo?: string;
  tocando: boolean;
  carregando: boolean;
  aoClicar: () => void;
  // Variante "tartaruga" (leitura lenta): ícone de tartaruga e botão mais compacto.
  lento?: boolean;
  // Variante grande e quadrada (botões de áudio da montagem fonética).
  grande?: boolean;
  titulo?: string;
}

export function BotaoAudio({ rotulo, tocando, carregando, aoClicar, lento, grande, titulo }: BotaoAudioProps) {
  const tamIcone = grande ? (lento ? 18 : 34) : 20;
  return (
    <button
      className={`revisao-botao-audio ${tocando ? 'tocando' : ''} ${lento ? 'lento' : ''} ${grande ? 'grande' : ''}`}
      onClick={aoClicar}
      disabled={carregando}
      title={titulo}
    >
      {carregando ? (
        <span>…</span>
      ) : lento ? (
        <svg width={tamIcone} height={tamIcone} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
          <path d="M4 13.5a5 4.5 0 0 1 10 0"></path>
          <path d="M2.5 13.5h13"></path>
          <circle cx="16.8" cy="12.8" r="1.7"></circle>
          <path d="M6.5 13.5v2.4"></path>
          <path d="M11.5 13.5v2.4"></path>
          {tocando && <path d="M20.8 9.8a5 5 0 0 1 0 6.5"></path>}
        </svg>
      ) : (
        <svg width={tamIcone} height={tamIcone} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
          <polygon points="11 5 6 9 2 9 2 15 6 15 11 19 11 5"></polygon>
          {tocando && <path d="M19.07 4.93a10 10 0 0 1 0 14.14M15.54 8.46a5 5 0 0 1 0 7.07"></path>}
        </svg>
      )}
      {rotulo === undefined ? t('Ouvir') : rotulo}
    </button>
  );
}
