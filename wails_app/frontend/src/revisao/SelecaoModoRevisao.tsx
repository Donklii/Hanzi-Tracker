// ----- Seção: Revisão — Tela de Seleção de Modo -----
// Quatro cards, um por modo de revisão. O contrato (props e chaves de modo) é usado pelo
// orquestrador AbaRevisao.tsx e não pode mudar.

interface SelecaoModoRevisaoProps {
  aoEscolherModo: (modo: string) => void;
  ttsAtivo: boolean;
  sttAtivo: boolean;
}

export const MODOS_REVISAO = [
  { chave: 'geral', titulo: 'Geral', descricao: 'Mistura todas as modalidades em uma única sessão.' },
  { chave: 'significado', titulo: 'Significado', descricao: 'Ligue o Hanzi ao seu significado (e vice-versa).' },
  { chave: 'fonetica', titulo: 'Fonética', descricao: 'Ligue o som do Hanzi ao caractere (e vice-versa).' },
  { chave: 'desenho', titulo: 'Desenho', descricao: 'Desenhe o Hanzi no canvas, guiado por contexto ou de memória.' },
  { chave: 'contexto', titulo: 'Contexto', descricao: 'Complete a lacuna da frase com o Hanzi correto.' },
  { chave: 'ordenacao', titulo: 'Ordenação', descricao: 'Coloque os caracteres na ordem correta para formar a frase.' },
  { chave: 'pronuncia', titulo: 'Pronúncia', descricao: 'Pratique a pronúncia falando frases ou sequências de caracteres.' },
];

const ICONES_MODO: Record<string, JSX.Element> = {
  geral: (
    <svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2"><polyline points="16 3 21 3 21 8"></polyline><line x1="4" y1="20" x2="21" y2="3"></line><polyline points="21 16 21 21 16 21"></polyline><line x1="15" y1="15" x2="21" y2="21"></line><line x1="4" y1="4" x2="9" y2="9"></line></svg>
  ),
  significado: (
    <svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2"><path d="M2 3h6a4 4 0 0 1 4 4v14a3 3 0 0 0-3-3H2z"></path><path d="M22 3h-6a4 4 0 0 0-4 4v14a3 3 0 0 1 3-3h7z"></path></svg>
  ),
  fonetica: (
    <svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2"><polygon points="11 5 6 9 2 9 2 15 6 15 11 19 11 5"></polygon><path d="M19.07 4.93a10 10 0 0 1 0 14.14M15.54 8.46a5 5 0 0 1 0 7.07"></path></svg>
  ),
  desenho: (
    <svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2"><path d="M12 19l7-7 3 3-7 7-3-3z"></path><path d="M18 13l-1.5-7.5L2 2l3.5 14.5L13 18l5-5z"></path><path d="M2 2l7.586 7.586"></path><circle cx="11" cy="11" r="2"></circle></svg>
  ),
  contexto: (
    <svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2"><line x1="17" y1="10" x2="3" y2="10"></line><line x1="21" y1="6" x2="3" y2="6"></line><line x1="21" y1="14" x2="3" y2="14"></line><line x1="17" y1="18" x2="3" y2="18"></line></svg>
  ),
  ordenacao: (
    <svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><path d="M4 9h16"/><path d="M4 15h16"/><path d="M10 3L6 7l4 4"/><path d="M14 21l4-4-4-4"/></svg>
  ),
  pronuncia: (
    <svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><path d="M12 2a3 3 0 0 0-3 3v7a3 3 0 0 0 6 0V5a3 3 0 0 0-3-3Z"></path><path d="M19 10v2a7 7 0 0 1-14 0v-2"></path><line x1="12" y1="19" x2="12" y2="22"></line></svg>
  ),
};

export function SelecaoModoRevisao({ aoEscolherModo, ttsAtivo, sttAtivo }: SelecaoModoRevisaoProps) {
  return (
    <div className="revisao-selecao-modos">
      {MODOS_REVISAO.map(m => {
        const desativado = (m.chave === 'fonetica' && !ttsAtivo) || (m.chave === 'pronuncia' && !sttAtivo);
        const titleAttr = desativado ? (m.chave === 'fonetica' ? 'Requer um motor TTS ativo nas configurações.' : 'Requer um motor de reconhecimento de fala ativo nas configurações.') : undefined;

        return (
          <button 
            key={m.chave} 
            className={`revisao-card-modo${m.chave === 'geral' ? ' revisao-card-modo-destaque' : ''}${desativado ? ' desativado' : ''}`} 
            onClick={() => { if (!desativado) aoEscolherModo(m.chave); }}
            disabled={desativado}
            title={titleAttr}
          >
            <div className="revisao-card-modo-icone">{ICONES_MODO[m.chave]}</div>
            <div className="revisao-card-modo-titulo">{m.titulo}</div>
            <div className="revisao-card-modo-descricao">{m.descricao}</div>
          </button>
        );
      })}
    </div>
  );
}
