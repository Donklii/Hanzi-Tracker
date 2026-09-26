package revisao

import (
	"iter"

	"wails_app/dicionario"
	"wails_app/revisao/busca"
)

// ----- Seção: Ponte para o Módulo Central de Busca -----
//
// Este arquivo NÃO contém lógica de busca própria: é a camada de compatibilidade que preserva os
// nomes usados pelos arquivos de atividade (significado.go, fonetica.go, contexto.go, ordenacao.go,
// desenho.go, pronuncia.go) e pelos testes deste pacote, delegando tudo para
// wails_app/revisao/busca — o pacote que de fato centraliza a busca por questões, distratores e
// atividades ideais (ver busca/busca.go). Nenhuma lógica de seleção deve ser reintroduzida aqui.

// ----- Tipos -----

type OpcaoRevisao = busca.OpcaoRevisao
type ElementoOrdenacao = busca.ElementoOrdenacao
type poolsRevisao = busca.Pools
type fonteBusca[C any] = busca.FonteBusca[C]
type criterioAceitacao[R any, C any] = busca.CriterioAceitacao[R, C]
type criterioDistrator = busca.CriterioDistrator

// ----- Constantes: Modos, Variantes e Tamanhos -----

const (
	ModoSignificado = busca.ModoSignificado
	ModoFonetica    = busca.ModoFonetica
	ModoDesenho     = busca.ModoDesenho
	ModoContexto    = busca.ModoContexto
	ModoPronuncia   = busca.ModoPronuncia
	ModoGeral       = busca.ModoGeral

	VarianteHanziParaSignificado          = busca.VarianteHanziParaSignificado
	VarianteSignificadoParaHanzi          = busca.VarianteSignificadoParaHanzi
	VarianteQuebraCabecaSignificado       = busca.VarianteQuebraCabecaSignificado
	VarianteImagemParaSignificado        = busca.VarianteImagemParaSignificado
	VarianteSignificadoParaImagem        = busca.VarianteSignificadoParaImagem
	VarianteHanziFraseParaSignificado     = busca.VarianteHanziFraseParaSignificado
	VarianteSignificadoParaHanziConhecido = busca.VarianteSignificadoParaHanziConhecido

	VarianteAudioParaHanzi       = busca.VarianteAudioParaHanzi
	VarianteHanziParaAudio       = busca.VarianteHanziParaAudio
	VarianteHanziParaPinyin      = busca.VarianteHanziParaPinyin
	VarianteFoneticaFrase        = busca.VarianteFoneticaFrase
	VarianteFoneticaTraducao     = busca.VarianteFoneticaTraducao
	VarianteFoneticaFilaPinyin   = busca.VarianteFoneticaFilaPinyin
	VarianteFoneticaPalavraPinyin = busca.VarianteFoneticaPalavraPinyin
	VarianteQuebraCabecaFonetica = busca.VarianteQuebraCabecaFonetica
	VarianteQuebraCabecaTrio     = busca.VarianteQuebraCabecaTrio

	VarianteDesenhoContexto   = busca.VarianteDesenhoContexto
	VarianteDesenhoMemoria    = busca.VarianteDesenhoMemoria
	VarianteDesenhoComponente = busca.VarianteDesenhoComponente
	VarianteDesenhoGuiado     = busca.VarianteDesenhoGuiado
	VarianteDesenhoMontagem   = busca.VarianteDesenhoMontagem

	VarianteContexto         = busca.VarianteContexto
	VarianteTraducaoContexto = busca.VarianteTraducaoContexto

	VarianteOrdenacao         = busca.VarianteOrdenacao
	VarianteOrdenacaoTraducao = busca.VarianteOrdenacaoTraducao

	VariantePronunciaFrase     = busca.VariantePronunciaFrase
	VariantePronunciaSequencia = busca.VariantePronunciaSequencia
	VariantePronunciaBaralho   = busca.VariantePronunciaBaralho
	VariantePronunciaTipo      = busca.VariantePronunciaTipo

	VarianteCompreensao          = busca.VarianteCompreensao
	VarianteCompreensaoTraduzida = busca.VarianteCompreensaoTraduzida
	VarianteRespostaDialogo      = busca.VarianteRespostaDialogo

	DificuldadeIntroducao    = busca.DificuldadeIntroducao
	DificuldadeIniciante     = busca.DificuldadeIniciante
	DificuldadeIntermediario = busca.DificuldadeIntermediario
	DificuldadeAvancado      = busca.DificuldadeAvancado

	TotalOpcoesMultiplaEscolha  = busca.TotalOpcoesMultiplaEscolha
	TotalOpcoesTraducaoContexto = busca.TotalOpcoesTraducaoContexto
	TotalPecasQuebraCabeca      = busca.TotalPecasQuebraCabeca
)

var ObterDificuldadeVariante = busca.ObterDificuldadeVariante
var VariantesQuebraCabeca = busca.VariantesQuebraCabeca
var VariantesDoModo = busca.VariantesDoModo
var removerDesativadas = busca.RemoverDesativadas

type varianteGraduada = busca.VarianteGraduada

func ehQuebraCabeca(variante string) bool { return busca.EhQuebraCabeca(variante) }

func (r *GerenciadorRevisao) variantesExtras(modo string) []string {
	return r.buscador.VariantesExtras(modo)
}
func (r *GerenciadorRevisao) dificuldadeDaVariante(variante string) string {
	return r.buscador.DificuldadeDaVariante(variante)
}

// ----- Motor genérico -----

func buscarDeFontes[C any, R any](meta int, iniciais []R, fontes []fonteBusca[C], aceitar criterioAceitacao[R, C], converter func(C) R) []R {
	return busca.BuscarDeFontes(meta, iniciais, fontes, aceitar, converter)
}

func buscarDeFontesSimples[T any](meta int, iniciais []T, fontes []fonteBusca[T], aceitar criterioAceitacao[T, T]) []T {
	return busca.BuscarDeFontesSimples(meta, iniciais, fontes, aceitar)
}

func sequenciaPreguicosa[T any](produzir func() []T) iter.Seq[T] {
	return busca.SequenciaPreguicosa(produzir)
}

// ----- Critérios de distinção -----

func distintosPorHanzi(escolhidas []OpcaoRevisao, candidata dicionario.DecomposicaoHanzi) bool {
	return busca.DistintosPorHanzi(escolhidas, candidata)
}
func distintosPorSignificado(escolhidas []OpcaoRevisao, candidata dicionario.DecomposicaoHanzi) bool {
	return busca.DistintosPorSignificado(escolhidas, candidata)
}
func distintosPorSignificadoEComImagem(escolhidas []OpcaoRevisao, candidata dicionario.DecomposicaoHanzi) bool {
	return busca.DistintosPorSignificadoEComImagem(escolhidas, candidata)
}
func (r *GerenciadorRevisao) distintosPorSignificadoConhecidos() criterioDistrator {
	return r.buscador.DistintosPorSignificadoConhecidos()
}
func (r *GerenciadorRevisao) distintosPorHanziConhecidos() criterioDistrator {
	return r.buscador.DistintosPorHanziConhecidos()
}
func distintosPorPinyin(escolhidas []OpcaoRevisao, candidata dicionario.DecomposicaoHanzi) bool {
	return busca.DistintosPorPinyin(escolhidas, candidata)
}
func normalizarPinyinParaComparar(p string) string { return busca.NormalizarPinyinParaComparar(p) }

// ----- Peças de ordenação -----

func hanzisDaFrase(frase string) map[string]bool   { return busca.HanzisDaFrase(frase) }
func quebrarFraseEmPalavras(frase string) []string { return busca.QuebrarFraseEmPalavras(frase) }

func aceitarPecaDistratora(hanzisNaFrase, pinyinsCorretos map[string]bool) criterioAceitacao[ElementoOrdenacao, ElementoOrdenacao] {
	return busca.AceitarPecaDistratora(hanzisNaFrase, pinyinsCorretos)
}
func aceitarPalavraInglesaDistratora(palavrasOriginais map[string]bool) criterioAceitacao[ElementoOrdenacao, string] {
	return busca.AceitarPalavraInglesaDistratora(palavrasOriginais)
}
func sequenciaPecasDeEntradas(entradas []dicionario.DecomposicaoHanzi) iter.Seq[ElementoOrdenacao] {
	return busca.SequenciaPecasDeEntradas(entradas)
}

func (r *GerenciadorRevisao) sequenciaPecasPalavrasDoTamanho(tamanho int) iter.Seq[ElementoOrdenacao] {
	return r.buscador.SequenciaPecasPalavrasDoTamanho(tamanho)
}
func (r *GerenciadorRevisao) pecasAparentadasDaFrase(hanzisNaFrase map[string]bool) []ElementoOrdenacao {
	return r.buscador.PecasAparentadasDaFrase(hanzisNaFrase)
}
func (r *GerenciadorRevisao) sementesDistratorasDaTraducao(hanzisNaFrase map[string]bool, candidatos []dicionario.DecomposicaoHanzi, desejadas int) []string {
	return r.buscador.SementesDistratorasDaTraducao(hanzisNaFrase, candidatos, desejadas)
}
func (r *GerenciadorRevisao) sequenciaPalavrasInglesasDeSementes(sementes []string) iter.Seq[string] {
	return r.buscador.SequenciaPalavrasInglesasDeSementes(sementes)
}
func (r *GerenciadorRevisao) sequenciaPalavrasInglesasDeFrases() iter.Seq[string] {
	return r.buscador.SequenciaPalavrasInglesasDeFrases()
}
func (r *GerenciadorRevisao) frasesDistratorasPorSimilaridade(fraseChines, fraseTraducao string) []dicionario.Frase {
	return r.buscador.FrasesDistratorasPorSimilaridade(fraseChines, fraseTraducao)
}

// ----- Opções e cartas -----

// poolsDoModo separa os candidatos do modo por status e injeta o pool de já vistas frequentes
// (Vistas), que depende do ranking de frequência do dicionário e do estado da sessão no Buscador.
func (r *GerenciadorRevisao) poolsDoModo(candidatos []dicionario.DecomposicaoHanzi, mapaStatus map[string]string) poolsRevisao {
	pools := busca.PoolsDoModo(candidatos, mapaStatus)
	pools.Vistas = r.buscador.PalavrasVistasFrequentes(candidatos)
	return pools
}
func novaOpcao(entrada dicionario.DecomposicaoHanzi, correta bool) OpcaoRevisao {
	return busca.NovaOpcao(entrada, correta)
}
func (r *GerenciadorRevisao) buscarPecasQuebraCabeca(alvo dicionario.DecomposicaoHanzi, pools poolsRevisao) []OpcaoRevisao {
	return r.buscador.BuscarPecasQuebraCabeca(alvo, pools)
}
func (r *GerenciadorRevisao) buscarPecasQuebraCabecaFonetica(alvo dicionario.DecomposicaoHanzi, pools poolsRevisao) []OpcaoRevisao {
	return r.buscador.BuscarPecasQuebraCabecaFonetica(alvo, pools)
}
func (r *GerenciadorRevisao) buscarPecasQuebraCabecaTrio(alvo dicionario.DecomposicaoHanzi, pools poolsRevisao) []OpcaoRevisao {
	return r.buscador.BuscarPecasQuebraCabecaTrio(alvo, pools)
}
func buscarBaralhoPronuncia(alvo dicionario.DecomposicaoHanzi, pools poolsRevisao) []dicionario.DecomposicaoHanzi {
	return busca.BuscarBaralhoPronuncia(alvo, pools)
}
func buscarBaralhoPronunciaTipo(alvo dicionario.DecomposicaoHanzi, pools poolsRevisao) []dicionario.DecomposicaoHanzi {
	return busca.BuscarBaralhoPronunciaTipo(alvo, pools)
}
func (r *GerenciadorRevisao) buscarOpcoes(alvo dicionario.DecomposicaoHanzi, emFoco bool, pools poolsRevisao, aceitar criterioDistrator) []OpcaoRevisao {
	return r.buscador.BuscarOpcoes(alvo, emFoco, pools, aceitar)
}

// ----- Atividades ideais -----

func (r *GerenciadorRevisao) sortearVariante(opcoes ...string) string {
	return r.buscador.SortearVarianteSessao(opcoes...)
}
func varianteParaModoBase(variante string) string { return busca.VarianteParaModoBase(variante) }
func prioridadeModo(modo string, estatisticas map[string]int) int {
	return busca.PrioridadeModo(modo, estatisticas, MetaAcertosConsecutivos)
}

func (r *GerenciadorRevisao) modosPermitidosRevisao() []string { return r.buscador.ModosPermitidos() }

func (r *GerenciadorRevisao) candidatosParaModo(modo string) []dicionario.DecomposicaoHanzi {
	return r.buscador.CandidatosParaModo(modo)
}

func (r *GerenciadorRevisao) alvoElegivelNoModo(alvo alvoRevisao, modo string, candQuestao []dicionario.DecomposicaoHanzi) bool {
	return r.buscador.AlvoElegivelNoModo(alvo.entrada.Caractere, modo, candQuestao)
}

func (r *GerenciadorRevisao) ordenarModosPorPendencia(alvo alvoRevisao, modosPermitidos []string, estatisticas map[string]int) []int {
	return busca.OrdenarModosPorPendencia(alvo.emFoco, modosPermitidos, estatisticas, MetaAcertosConsecutivos)
}

func temImagemParaHanzi(palavra string) bool {
	return busca.TemImagemParaHanzi(palavra)
}
