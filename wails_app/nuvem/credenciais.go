package nuvem

// ----- Credenciais OAuth -----
// O usuário cria as próprias credenciais no Google Cloud Console (APIs e serviços → Credenciais →
// ID do cliente OAuth, tipo "App para computador", com a API do Google Drive ativada) e as cola
// na aba Armazenamento — ficam em configuracoes.json e chegam aqui via Dependencias.Credenciais.
// Em apps desktop o client secret NÃO é confidencial (documentado pelo Google): ele só identifica
// o app, não o autentica — a segurança vem do consentimento do usuário + PKCE + loopback.

// credenciais devolve o par client id/secret fornecido pela UI ("" = ainda não configurado).
func (g *Gerenciador) credenciais() (id, secret string) {
	// Guard clause: gerenciador montado sem a dependência (só acontece em teste incompleto).
	if g.dep.Credenciais == nil {
		return "", ""
	}
	return g.dep.Credenciais()
}

// Configurado informa se o usuário já preencheu as credenciais que habilitam a conexão.
func (g *Gerenciador) Configurado() bool {
	id, _ := g.credenciais()
	return id != ""
}
