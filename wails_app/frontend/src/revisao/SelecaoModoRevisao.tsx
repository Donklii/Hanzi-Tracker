// ----- Seção: Revisão — Tela de Seleção de Modo -----
// Ordem da página: dois banners principais (Jornada e Revisão Geral, este último com painel
// expansível de configuração que inclui filtros de tema/dificuldade e subseção de geração de
// frases novas com IA) → grade de atividades com interruptores que definem o que entra na sessão
// geral (config modosRevisaoGeralDesativados). Clicar no corpo de uma atividade pratica só ela.
import React, { useEffect, useState } from 'react';
import { config } from '../../wailsjs/go/models';
import { GerarFrasesComIA } from '../../wailsjs/go/main/App';
import { Interruptor } from './Interruptor';
import { ModalFrasesIA } from './ModalFrasesIA';
import { ModalConfigAtividades } from './ModalConfigAtividades';
import { ICONES_JORNADA } from './jornada/iconesJornada';
import { TEMAS_FRASE, DIFICULDADES_FRASE } from './taxonomiaFrases';
import { t } from '../i18n/i18n';

interface SelecaoModoRevisaoProps {
  aoEscolherModo: (modo: string) => void; // chave de atividade, 'geral' ou 'jornada'
  aoEscolherExemplo?: (chave: string) => void;
  ttsAtivo: boolean;
  sttAtivo: boolean;
  configuracoesApp: config.Config | null;
  AtualizarConfiguracao?: (key: keyof config.Config, value: any) => void;
  foco: any[];
  aoClicarNoFoco?: (item: any) => void;
}

export const ATIVIDADES_REVISAO = [
  { chave: 'significado', titulo: 'Significado', descricao: 'Ligue o Hanzi ao seu significado (e vice-versa).' },
  { chave: 'fonetica', titulo: 'Fonética', descricao: 'Ligue o som do Hanzi ao caractere (e vice-versa).' },
  { chave: 'desenho', titulo: 'Desenho', descricao: 'Desenhe o Hanzi no canvas, por contexto ou de memória.' },
  { chave: 'contexto', titulo: 'Contexto', descricao: 'Compreenda, ordene e complete sentenças ou diálogos em contexto.' },
  { chave: 'pronuncia', titulo: 'Pronúncia', descricao: 'Pratique falando frases e sequências no microfone.' },
];

const ICONES_MODO: Record<string, JSX.Element> = {
  geral: (
    <svg width="28" height="28" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2"><polyline points="16 3 21 3 21 8"></polyline><line x1="4" y1="20" x2="21" y2="3"></line><polyline points="21 16 21 21 16 21"></polyline><line x1="15" y1="15" x2="21" y2="21"></line><line x1="4" y1="4" x2="9" y2="9"></line></svg>
  ),
  significado: (
    <svg width="22" height="22" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2"><path d="M2 3h6a4 4 0 0 1 4 4v14a3 3 0 0 0-3-3H2z"></path><path d="M22 3h-6a4 4 0 0 0-4 4v14a3 3 0 0 1 3-3h7z"></path></svg>
  ),
  fonetica: (
    <svg width="22" height="22" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2"><polygon points="11 5 6 9 2 9 2 15 6 15 11 19 11 5"></polygon><path d="M19.07 4.93a10 10 0 0 1 0 14.14M15.54 8.46a5 5 0 0 1 0 7.07"></path></svg>
  ),
  desenho: (
    <svg width="22" height="22" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2"><path d="M12 19l7-7 3 3-7 7-3-3z"></path><path d="M18 13l-1.5-7.5L2 2l3.5 14.5L13 18l5-5z"></path><path d="M2 2l7.586 7.586"></path><circle cx="11" cy="11" r="2"></circle></svg>
  ),
  contexto: (
    <svg width="22" height="22" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2"><line x1="17" y1="10" x2="3" y2="10"></line><line x1="21" y1="6" x2="3" y2="6"></line><line x1="21" y1="14" x2="3" y2="14"></line><line x1="17" y1="18" x2="3" y2="18"></line></svg>
  ),
  pronuncia: (
    <svg width="22" height="22" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><path d="M12 2a3 3 0 0 0-3 3v7a3 3 0 0 0 6 0V5a3 3 0 0 0-3-3Z"></path><path d="M19 10v2a7 7 0 0 1-14 0v-2"></path><line x1="12" y1="19" x2="12" y2="22"></line></svg>
  ),
};

export function SelecaoModoRevisao({ aoEscolherModo, aoEscolherExemplo, ttsAtivo, sttAtivo, configuracoesApp, AtualizarConfiguracao, foco, aoClicarNoFoco }: SelecaoModoRevisaoProps) {
  const [painelConfigAberto, setPainelConfigAberto] = useState(false);
  const [modalIaAberto, setModalIaAberto] = useState(false);
  const [modoSubConfigAberto, setModoSubConfigAberto] = useState<string | null>(null);
  const [qtdRascunho, setQtdRascunho] = useState<string | null>(null);
  const valorQtdExibido = qtdRascunho !== null ? qtdRascunho : String(configuracoesApp?.revisaoQuantidadeQuestoes ?? 10);

  const desativados = configuracoesApp?.modosRevisaoGeralDesativados || [];
  const geminiConfigurado = !!configuracoesApp?.geminiApiKey;

  const bloqueadaPorMotor = (chave: string) =>
    (chave === 'fonetica' && !ttsAtivo) || (chave === 'pronuncia' && !sttAtivo);
  const motivoBloqueio = (chave: string) =>
    chave === 'fonetica'
      ? t('Requer um motor TTS ativo nas configurações.')
      : t('Requer um motor de reconhecimento de fala ativo nas configurações.');
  const ativaNaGeral = (chave: string) => !bloqueadaPorMotor(chave) && desativados.indexOf(chave) === -1;
  const qtdAtivas = ATIVIDADES_REVISAO.filter(a => ativaNaGeral(a.chave)).length;

  function alternarAtividade(chave: string) {
    if (!AtualizarConfiguracao) return;
    const desligada = desativados.indexOf(chave) !== -1;
    if (!desligada && qtdAtivas <= 1) return;
    const novos = desligada ? desativados.filter(m => m !== chave) : [...desativados, chave];
    AtualizarConfiguracao('modosRevisaoGeralDesativados', novos);
  }

  return (
    <div className="revisao-selecao">
      {/* --- Modos principais: banners de largura inteira --- */}
      <div className="revisao-modos-principais">
        <button className="revisao-modo-principal jornada" onClick={() => aoEscolherModo('jornada')}>
          <div className="revisao-modo-principal-icone">{ICONES_JORNADA.ramo}</div>
          <div className="revisao-modo-principal-textos">
            <div className="revisao-modo-principal-titulo">
              {t('Jornada')}
              <span className="revisao-badge-jornada">{t('Novo')}</span>
            </div>
            <div className="revisao-modo-principal-descricao">
              {t('Suba pela árvore de níveis: comece pelo essencial da língua e, a cada dificuldade concluída, escolha os caminhos e temas que quer dominar.')}
            </div>
          </div>
          <div className="revisao-modo-principal-cta">{t('Explorar →')}</div>
        </button>

        <button
          className={`revisao-modo-principal geral${painelConfigAberto ? ' aberto' : ''}`}
          onClick={() => aoEscolherModo('geral')}
        >
          <div className="revisao-modo-principal-icone">{ICONES_MODO.geral}</div>
          <div className="revisao-modo-principal-textos">
            <div className="revisao-modo-principal-titulo">{t('Revisão Geral')}</div>
            <div className="revisao-modo-principal-descricao">
              {t('Mistura as {qtdAtivas} atividades ativas abaixo em uma única sessão, priorizando o que você ainda não domina.', { qtdAtivas })}
            </div>
          </div>
          <div
            className="revisao-modo-principal-cta"
            onClick={(e) => {
              e.stopPropagation();
              setPainelConfigAberto(v => !v);
            }}
          >
            {painelConfigAberto ? t('Fechar ▴') : t('Configurar ▾')}
          </div>
        </button>
      </div>

      {/* --- Painel de configuração da Revisão Geral (expande sob os banners) --- */}
      {painelConfigAberto && (
        <div className="revisao-config-painel">
          <label className="revisao-config-campo">
            <span>{t('Tema das frases')}</span>
            <select
              className="revisao-config-input"
              value={configuracoesApp?.revisaoFiltroTema || ''}
              onChange={e => AtualizarConfiguracao && AtualizarConfiguracao('revisaoFiltroTema', e.target.value)}
            >
              <option value="">{t('Todos os temas')}</option>
              {TEMAS_FRASE.map(item => (
                <option key={item.valor} value={item.valor}>
                  {t(item.rotulo)}
                </option>
              ))}
            </select>
          </label>

          <div className="revisao-config-campo">
            <span>{t('Dificuldade')}</span>
            <div className="revisao-config-segmentado">
              <button
                type="button"
                className={!(configuracoesApp?.revisaoFiltroDificuldade) ? 'ativo' : ''}
                onClick={() => AtualizarConfiguracao && AtualizarConfiguracao('revisaoFiltroDificuldade', '')}
              >
                {t('Todas')}
              </button>
              {DIFICULDADES_FRASE.map(item => (
                <button
                  key={item.valor}
                  type="button"
                  className={configuracoesApp?.revisaoFiltroDificuldade === item.valor ? 'ativo' : ''}
                  onClick={() => AtualizarConfiguracao && AtualizarConfiguracao('revisaoFiltroDificuldade', item.valor)}
                >
                  {t(item.rotulo)}
                </button>
              ))}
            </div>
          </div>

          <div className="revisao-config-campo">
            <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
              <span>{t('Quantidade de questões')}</span>
              <input
                type="number"
                min={8}
                max={50}
                className="revisao-config-slider-valor-input"
                value={valorQtdExibido}
                onChange={e => {
                  setQtdRascunho(e.target.value);
                  const val = parseInt(e.target.value, 10);
                  if (!isNaN(val) && val >= 8 && val <= 50) {
                    AtualizarConfiguracao && AtualizarConfiguracao('revisaoQuantidadeQuestoes', val);
                  }
                }}
                onBlur={() => {
                  let val = parseInt(qtdRascunho ?? '', 10);
                  if (isNaN(val) || val < 8) val = 8;
                  if (val > 50) val = 50;
                  AtualizarConfiguracao && AtualizarConfiguracao('revisaoQuantidadeQuestoes', val);
                  setQtdRascunho(null);
                }}
              />
            </div>
            <input
              type="range"
              min={8}
              max={50}
              step={1}
              className="revisao-config-slider"
              value={configuracoesApp?.revisaoQuantidadeQuestoes ?? 10}
              onChange={e => {
                const val = parseInt(e.target.value, 10);
                if (!isNaN(val)) {
                  setQtdRascunho(null);
                  AtualizarConfiguracao && AtualizarConfiguracao('revisaoQuantidadeQuestoes', val);
                }
              }}
            />
          </div>

          <div className="revisao-config-campo">
            <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
              <span>{t('Revisar questões erradas ao final')}</span>
              <Interruptor
                ligado={configuracoesApp?.revisarErradasAoFinal ?? true}
                aoAlternar={() => {
                  AtualizarConfiguracao &&
                    AtualizarConfiguracao(
                      'revisarErradasAoFinal',
                      !(configuracoesApp?.revisarErradasAoFinal ?? true)
                    );
                }}
              />
            </div>
          </div>

          <div className="revisao-config-divisor" />

          <button
            type="button"
            className="revisao-ia-gerar"
            onClick={() => setModalIaAberto(true)}
          >
            {t('✨ Gerenciar / Gerar Frases com IA')}
          </button>
        </div>
      )}

      <ModalFrasesIA
        aberto={modalIaAberto}
        aoFechar={() => setModalIaAberto(false)}
        configuracoesApp={configuracoesApp}
        AtualizarConfiguracao={AtualizarConfiguracao}
      />

      <ModalConfigAtividades
        modo={modoSubConfigAberto}
        aberto={!!modoSubConfigAberto}
        aoFechar={() => setModoSubConfigAberto(null)}
        configuracoesApp={configuracoesApp}
        AtualizarConfiguracao={AtualizarConfiguracao}
        ttsAtivo={ttsAtivo}
        sttAtivo={sttAtivo}
        aoEscolherExemplo={aoEscolherExemplo}
      />

      {/* --- Atividades --- */}
      <div className="revisao-secao-rotulo">
        {t('Atividades')}
        <small>{t('o interruptor inclui na Revisão Geral · clique no ícone ⚙️ para configurar tipos de atividade · clique no cartão para praticar só ela')}</small>
      </div>
      <div className="revisao-atividades">
        {ATIVIDADES_REVISAO.map(a => {
          const bloqueada = bloqueadaPorMotor(a.chave);
          const ligada = ativaNaGeral(a.chave);
          return (
            <div
              key={a.chave}
              role="button"
              tabIndex={bloqueada ? -1 : 0}
              className={`revisao-atividade${ligada ? '' : ' fora'}${bloqueada ? ' bloqueada' : ''}`}
              title={bloqueada ? motivoBloqueio(a.chave) : t('Praticar só {atividade}', { atividade: t(a.titulo) })}
              onClick={() => { if (!bloqueada) aoEscolherModo(a.chave); }}
              onKeyDown={e => { if (!bloqueada && (e.key === 'Enter' || e.key === ' ')) { e.preventDefault(); aoEscolherModo(a.chave); } }}
            >
              <div className="revisao-atividade-topo">
                <button
                  type="button"
                  className="revisao-atividade-botao-config"
                  title={t('Configurar sub-atividades de {atividade}', { atividade: t(a.titulo) })}
                  onClick={(e) => {
                    e.stopPropagation();
                    setModoSubConfigAberto(a.chave);
                  }}
                >
                  <svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                    <circle cx="12" cy="12" r="3"></circle>
                    <path d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 0 1 0 2.83 2 2 0 0 1-2.83 0l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 0 1-2 2 2 2 0 0 1-2-2v-.09A1.65 1.65 0 0 0 9 19.4a1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 0 1-2.83 0 2 2 0 0 1 0-2.83l.06-.06a1.65 1.65 0 0 0 .33-1.82 1.65 1.65 0 0 0-1.51-1H3a2 2 0 0 1-2-2 2 2 0 0 1 2-2h.09A1.65 1.65 0 0 0 4.6 9a1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 0 1 0-2.83 2 2 0 0 1 2.83 0l.06.06a1.65 1.65 0 0 0 1.82.33H9a1.65 1.65 0 0 0 1-1.51V3a2 2 0 0 1 2-2 2 2 0 0 1 2 2v.09a1.65 1.65 0 0 0 1 1.51 1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 0 1 2.83 0 2 2 0 0 1 0 2.83l-.06.06a1.65 1.65 0 0 0-.33 1.82V9a1.65 1.65 0 0 0 1.51 1H21a2 2 0 0 1 2 2 2 2 0 0 1-2 2h-.09a1.65 1.65 0 0 0-1.51 1z"></path>
                  </svg>
                </button>
                <div className="revisao-atividade-icone">{ICONES_MODO[a.chave]}</div>
                {AtualizarConfiguracao && (
                  <Interruptor
                    ligado={ligada}
                    desabilitado={bloqueada}
                    titulo={
                      bloqueada ? motivoBloqueio(a.chave)
                      : ligada && qtdAtivas <= 1 ? t('A Revisão Geral precisa de ao menos uma atividade ativa.')
                      : ligada ? t('Remover da Revisão Geral') : t('Incluir na Revisão Geral')
                    }
                    aoAlternar={() => alternarAtividade(a.chave)}
                  />
                )}
              </div>
              <div className="revisao-atividade-titulo">{t(a.titulo)}</div>
              <div className="revisao-atividade-descricao">{t(a.descricao)}</div>
            </div>
          );
        })}
      </div>
    </div>
  );
}
