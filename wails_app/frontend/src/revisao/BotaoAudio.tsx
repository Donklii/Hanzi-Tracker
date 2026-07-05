// ----- Seção: Revisão — Botão de Áudio -----
// Componente puramente visual: quem toca/sintetiza o áudio é o pai (AbaRevisao).

interface BotaoAudioProps {
  rotulo?: string;
  tocando: boolean;
  carregando: boolean;
  aoClicar: () => void;
}

export function BotaoAudio({ rotulo, tocando, carregando, aoClicar }: BotaoAudioProps) {
  return (
    <button
      className={`revisao-botao-audio ${tocando ? 'tocando' : ''}`}
      onClick={aoClicar}
      disabled={carregando}
    >
      {carregando ? (
        <span>…</span>
      ) : (
        <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
          <polygon points="11 5 6 9 2 9 2 15 6 15 11 19 11 5"></polygon>
          {tocando && <path d="M19.07 4.93a10 10 0 0 1 0 14.14M15.54 8.46a5 5 0 0 1 0 7.07"></path>}
        </svg>
      )}
      {rotulo === undefined ? 'Ouvir' : rotulo}
    </button>
  );
}
