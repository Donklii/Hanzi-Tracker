// Interruptor deslizante (switch) compartilhado pela grade de atividades (SelecaoModoRevisao)
// e pelo painel do grupo de foco no cabeçalho (GrupoFocoCabecalho).
export function Interruptor({ ligado, aoAlternar, desabilitado, titulo }: { ligado: boolean; aoAlternar: () => void; desabilitado?: boolean; titulo?: string }) {
  return (
    <button
      type="button"
      role="switch"
      aria-checked={ligado}
      className={`revisao-interruptor${ligado ? ' ligado' : ''}`}
      disabled={desabilitado}
      title={titulo}
      onClick={e => { e.stopPropagation(); aoAlternar(); }}
    />
  );
}
