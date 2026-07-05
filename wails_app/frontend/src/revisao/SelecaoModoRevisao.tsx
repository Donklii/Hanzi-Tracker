// ----- Seção: Revisão — Tela de Seleção de Modo -----
// Quatro cards, um por modo de revisão. O contrato (props e chaves de modo) é usado pelo
// orquestrador AbaRevisao.tsx e não pode mudar.

interface SelecaoModoRevisaoProps {
  aoEscolherModo: (modo: string) => void;
}

export const MODOS_REVISAO = [
  { chave: 'significado', titulo: 'Significado', descricao: 'Ligue o Hanzi ao seu significado (e vice-versa).' },
  { chave: 'fonetica', titulo: 'Fonética', descricao: 'Ligue o som do Hanzi ao caractere (e vice-versa).' },
  { chave: 'desenho', titulo: 'Desenho', descricao: 'Desenhe o Hanzi no canvas, guiado por contexto ou de memória.' },
  { chave: 'contexto', titulo: 'Contexto', descricao: 'Complete a lacuna da frase com o Hanzi correto.' },
];

const ICONES_MODO: Record<string, JSX.Element> = {
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
};

export function SelecaoModoRevisao({ aoEscolherModo }: SelecaoModoRevisaoProps) {
  return (
    <div className="revisao-selecao-modos">
      {MODOS_REVISAO.map(m => (
        <button key={m.chave} className="revisao-card-modo" onClick={() => aoEscolherModo(m.chave)}>
          <div className="revisao-card-modo-icone">{ICONES_MODO[m.chave]}</div>
          <div className="revisao-card-modo-titulo">{m.titulo}</div>
          <div className="revisao-card-modo-descricao">{m.descricao}</div>
        </button>
      ))}
    </div>
  );
}
