// ----- Seção: Modal de Configuração de Sub-Atividades de Revisão -----
// Exibe todas as variantes/sub-atividades associadas a um tipo de revisão, com badges de dificuldade,
// requisitos de motor e interruptores individuais para ativar/desativar cada tipo no sorteio.

import React from 'react';
import { config } from '../../wailsjs/go/models';
import { Interruptor } from './Interruptor';
import { t } from '../i18n/i18n';

export interface SubAtividadeInfo {
  chave: string;
  titulo: string;
  descricao: string;
  dificuldade: 'introdução' | 'iniciante' | 'intermediário' | 'avançado';
  requisitoMotor?: 'tts' | 'stt';
}

export const SUB_ATIVIDADES_POR_MODO: Record<string, SubAtividadeInfo[]> = {
  significado: [
    { chave: 'hanzi_para_significado', titulo: 'Hanzi → Significado', descricao: 'Escolha o significado correto em português/inglês a partir do Hanzi exibido.', dificuldade: 'iniciante' },
    { chave: 'significado_para_hanzi', titulo: 'Significado → Hanzi', descricao: 'Escolha o Hanzi correto para a definição exibida.', dificuldade: 'iniciante' },
    { chave: 'hanzi_frase_para_significado', titulo: 'Frase Hanzi → Significado', descricao: 'Identifique o significado com o auxílio do contexto da frase.', dificuldade: 'introdução' },
    { chave: 'significado_para_hanzi_conhecido', titulo: 'Significado → Hanzi Conhecido', descricao: 'Associe a definição ao Hanzi entre opções de caracteres praticados.', dificuldade: 'introdução' },
    { chave: 'imagem_para_significado', titulo: 'Imagem → Significado', descricao: 'Identifique o significado correspondente à imagem exibida (quando disponível).', dificuldade: 'introdução' },
    { chave: 'significado_para_imagem', titulo: 'Significado → Imagem', descricao: 'Escolha a imagem correspondente à definição exibida (quando disponível).', dificuldade: 'introdução' },
    { chave: 'quebracabeca_significado', titulo: 'Quebra-Cabeça de Significado', descricao: 'Relacione pares de Hanzis e significados no tabuleiro interativo.', dificuldade: 'intermediário' },
    { chave: 'quebracabeca_trio', titulo: 'Quebra-Cabeça Trio', descricao: 'Combine trios de Hanzi, Pinyin e Significado na mesma rodada.', dificuldade: 'avançado' },
  ],
  fonetica: [
    { chave: 'audio_para_hanzi', titulo: 'Áudio → Hanzi', descricao: 'Escute a pronúncia sintetizada e selecione o Hanzi correspondente.', dificuldade: 'introdução', requisitoMotor: 'tts' },
    { chave: 'hanzi_para_audio', titulo: 'Hanzi → Áudio', descricao: 'Veja o Hanzi e selecione a opção com o som correto.', dificuldade: 'iniciante', requisitoMotor: 'tts' },
    { chave: 'hanzi_para_pinyin', titulo: 'Hanzi → Pinyin', descricao: 'Selecione o Pinyin com as marcações de tom apropriadas.', dificuldade: 'introdução' },
    { chave: 'fonetica_palavra_pinyin', titulo: 'Pinyin da Palavra', descricao: 'Escolha o Pinyin correto para a palavra ou composto em estudo.', dificuldade: 'iniciante' },
    { chave: 'fonetica_frase', titulo: 'Fila Fonética com Frase', descricao: 'Escute a frase completa e identifique os caracteres pronunciados.', dificuldade: 'avançado', requisitoMotor: 'tts' },
    { chave: 'fonetica_traducao', titulo: 'Fonética com Tradução', descricao: 'Relacione a pronúncia do caractere à tradução contextual.', dificuldade: 'intermediário', requisitoMotor: 'tts' },
    { chave: 'fonetica_fila_pinyin', titulo: 'Fila de Pinyin', descricao: 'Monte a sequência de sílabas em Pinyin da frase.', dificuldade: 'intermediário' },
    { chave: 'quebracabeca_fonetica', titulo: 'Quebra-Cabeça Fonético', descricao: 'Ligue sons e tons aos caracteres no tabuleiro.', dificuldade: 'avançado', requisitoMotor: 'tts' },
    { chave: 'quebracabeca_trio', titulo: 'Quebra-Cabeça Trio', descricao: 'Combine trios de Hanzi, Pinyin e Significado no tabuleiro.', dificuldade: 'avançado' },
  ],
  desenho: [
    { chave: 'desenho_guiado', titulo: 'Desenho Guiado', descricao: 'Pratique a escrita seguindo o contorno visível dos traços.', dificuldade: 'introdução' },
    { chave: 'desenho_memoria', titulo: 'Desenho de Memória', descricao: 'Desenhe o Hanzi no canvas sem o contorno de fundo.', dificuldade: 'intermediário' },
    { chave: 'desenho_montagem', titulo: 'Montagem de Traços', descricao: 'Arraste e posicione as peças e componentes do Hanzi.', dificuldade: 'iniciante' },
    { chave: 'desenho_componente', titulo: 'Redesenho de Radical', descricao: 'Desenhe componentes e radicais da decomposição do Hanzi.', dificuldade: 'iniciante' },
    { chave: 'desenho_contexto', titulo: 'Desenho em Contexto', descricao: 'Escreva de memória o Hanzi que completa a lacuna da frase.', dificuldade: 'avançado' },
  ],
  contexto: [
    { chave: 'contexto', titulo: 'Completar Lacuna', descricao: 'Escolha o Hanzi adequado para preencher o espaço na frase.', dificuldade: 'iniciante' },
    { chave: 'traducao_contexto', titulo: 'Tradução de Contexto', descricao: 'Identifique a tradução correta da frase no contexto da lacuna.', dificuldade: 'intermediário' },
    { chave: 'ordenacao', titulo: 'Ordenação de Hanzi', descricao: 'Organize os blocos de caracteres chineses na sequência da frase.', dificuldade: 'avançado' },
    { chave: 'ordenacao_traducao', titulo: 'Ordenação da Tradução', descricao: 'Organize os blocos da tradução em ordem coerente.', dificuldade: 'intermediário' },
    { chave: 'compreensao', titulo: 'Compreensão de Leitura', descricao: 'Responda a perguntas de múltipla escolha baseadas em um texto/contexto em chinês.', dificuldade: 'avançado' },
    { chave: 'compreensao_traduzida', titulo: 'Compreensão Guiada', descricao: 'Responda a perguntas com o auxílio da tradução da pergunta.', dificuldade: 'iniciante' },
    { chave: 'resposta_dialogo', titulo: 'Resposta de Diálogo', descricao: 'Escolha a fala de resposta mais adequada para completar o diálogo.', dificuldade: 'avançado' },
  ],
  pronuncia: [
    { chave: 'pronuncia_tipo', titulo: 'Pronúncia por Tipo', descricao: 'Fale palavras e caracteres isolados no microfone.', dificuldade: 'iniciante', requisitoMotor: 'stt' },
    { chave: 'pronuncia_sequencia', titulo: 'Sequência Verbal', descricao: 'Pronuncie sequências de caracteres em voz alta.', dificuldade: 'intermediário', requisitoMotor: 'stt' },
    { chave: 'pronuncia_frase', titulo: 'Leitura de Frase Inteira', descricao: 'Leia a frase inteira em chinês no microfone.', dificuldade: 'intermediário', requisitoMotor: 'stt' },
    { chave: 'pronuncia_baralho', titulo: 'Baralho de Dicção', descricao: 'Pratique a dicção e tons com cartas de fala.', dificuldade: 'avançado', requisitoMotor: 'stt' },
  ],
};

const TITULOS_MODO: Record<string, string> = {
  significado: 'Atividades de Significado',
  fonetica: 'Atividades de Fonética',
  desenho: 'Atividades de Desenho',
  contexto: 'Atividades de Contexto',
  pronuncia: 'Atividades de Pronúncia',
};

interface ModalConfigAtividadesProps {
  modo: string | null;
  aberto: boolean;
  aoFechar: () => void;
  configuracoesApp: config.Config | null;
  AtualizarConfiguracao?: (key: keyof config.Config, value: any) => void;
  ttsAtivo: boolean;
  sttAtivo: boolean;
  aoEscolherExemplo?: (chave: string) => void;
}

const PESO_DIFICULDADE: Record<string, number> = {
  'introdução': 0,
  'iniciante': 1,
  'intermediário': 2,
  'avançado': 3,
};

export function ModalConfigAtividades({
  modo,
  aberto,
  aoFechar,
  configuracoesApp,
  AtualizarConfiguracao,
  ttsAtivo,
  sttAtivo,
  aoEscolherExemplo,
}: ModalConfigAtividadesProps) {
  if (!aberto || !modo) return null;

  const subAtividadesBase = SUB_ATIVIDADES_POR_MODO[modo] || [];
  const subAtividades = [...subAtividadesBase].sort((a, b) => {
    const pesoA = PESO_DIFICULDADE[a.dificuldade] ?? 0;
    const pesoB = PESO_DIFICULDADE[b.dificuldade] ?? 0;
    return pesoA - pesoB;
  });
  const desativadas = configuracoesApp?.atividadesDesativadas || [];

  const bloqueadaPorMotor = (sub: SubAtividadeInfo) => {
    if (sub.requisitoMotor === 'tts' && !ttsAtivo) return true;
    if (sub.requisitoMotor === 'stt' && !sttAtivo) return true;
    return false;
  };

  const motivoBloqueio = (sub: SubAtividadeInfo) => {
    if (sub.requisitoMotor === 'tts') return t('Requer um motor TTS ativo nas configurações.');
    if (sub.requisitoMotor === 'stt') return t('Requer um motor de reconhecimento de fala ativo nas configurações.');
    return '';
  };

  const estaAtiva = (chave: string) => desativadas.indexOf(chave) === -1;
  const qtdAtivasNoModo = subAtividades.filter(s => !bloqueadaPorMotor(s) && estaAtiva(s.chave)).length;

  function alternarSubAtividade(chave: string) {
    if (!AtualizarConfiguracao) return;
    const desligada = desativadas.indexOf(chave) !== -1;

    // Impede desativar a última sub-atividade ativa do modo
    if (!desligada && qtdAtivasNoModo <= 1) return;

    const novas = desligada ? desativadas.filter(a => a !== chave) : [...desativadas, chave];
    AtualizarConfiguracao('atividadesDesativadas', novas);
  }

  return (
    <div className="modal-config-atividades-overlay" onClick={aoFechar}>
      <div className="modal-config-atividades-conteudo" onClick={e => e.stopPropagation()}>
        <div className="modal-config-atividades-cabecalho">
          <div>
            <h3>{t(TITULOS_MODO[modo] || 'Configuração de Atividades')}</h3>
            <p className="modal-config-atividades-subtitulo">
              {t('Clique em qualquer cartão para experimentar um exemplo da atividade, ou use os interruptores para ativá-la ou desativá-la no sorteio.')}
            </p>
          </div>
          <button type="button" className="modal-config-atividades-fechar" onClick={aoFechar} title={t('Fechar')}>
            ✕
          </button>
        </div>

        <div className="modal-config-atividades-lista">
          {subAtividades.map(sub => {
            const bloqueada = bloqueadaPorMotor(sub);
            const ativa = estaAtiva(sub.chave);

            return (
              <div
                key={sub.chave}
                role="button"
                tabIndex={bloqueada ? -1 : 0}
                className={`modal-config-sub-item${ativa && !bloqueada ? '' : ' inativa'}${bloqueada ? ' bloqueada' : ''}`}
                title={
                  bloqueada
                    ? motivoBloqueio(sub)
                    : t('Clique para realizar um exemplo de {atividade}', { atividade: t(sub.titulo) })
                }
                onClick={() => {
                  if (!bloqueada && aoEscolherExemplo) {
                    aoFechar();
                    aoEscolherExemplo(sub.chave);
                  }
                }}
                onKeyDown={e => {
                  if (!bloqueada && aoEscolherExemplo && (e.key === 'Enter' || e.key === ' ')) {
                    e.preventDefault();
                    aoFechar();
                    aoEscolherExemplo(sub.chave);
                  }
                }}
              >
                <div className="modal-config-sub-info">
                  <div className="modal-config-sub-linha-titulo">
                    <span className="modal-config-sub-titulo">{t(sub.titulo)}</span>
                    <span className={`badge-dificuldade badge-${sub.dificuldade}`}>
                      {t(sub.dificuldade)}
                    </span>
                    {bloqueada ? (
                      <span className="badge-requisito">{motivoBloqueio(sub)}</span>
                    ) : (
                      <span className="badge-exemplo-acao">▶ {t('Exemplo')}</span>
                    )}
                  </div>
                  <div className="modal-config-sub-descricao">{t(sub.descricao)}</div>
                </div>

                <div className="modal-config-sub-acao" onClick={e => e.stopPropagation()}>
                  <Interruptor
                    ligado={ativa && !bloqueada}
                    desabilitado={bloqueada}
                    titulo={
                      bloqueada
                        ? motivoBloqueio(sub)
                        : ativa && qtdAtivasNoModo <= 1
                        ? t('Mantenha ao menos uma atividade ativa para este tipo de revisão.')
                        : ativa
                        ? t('Desativar atividade no sorteio')
                        : t('Ativar atividade no sorteio')
                    }
                    aoAlternar={() => alternarSubAtividade(sub.chave)}
                  />
                </div>
              </div>
            );
          })}
        </div>

        <div className="modal-config-atividades-rodape">
          <button type="button" className="revisao-config-btn-fechar" onClick={aoFechar}>
            {t('Concluído')}
          </button>
        </div>
      </div>
    </div>
  );
}
