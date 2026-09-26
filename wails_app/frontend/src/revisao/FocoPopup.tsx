import { useState, useEffect } from 'react';
import { config, progresso, revisao } from '../../wailsjs/go/models';
import { Interruptor } from './Interruptor';
import { AdicionarHanziFoco, RemoverHanziFoco, GetVocab } from '../../wailsjs/go/main/App';
import { useBuscaDicionario } from '../busca/useBuscaDicionario';
import { t } from '../i18n/i18n';
import './revisao.css';

// ----- Seção: Revisão — Pop-up de gerenciamento do grupo de foco -----
// Aberto pelo botão de lista do cabeçalho (GrupoFocoCabecalho). Reúne o que não cabe na linha
// única do cabeçalho: a configuração de tamanho (automático ou manual), a lista completa dos
// caracteres em foco e a adição — cuja caixa é também uma BARRA DE PESQUISA (mesma lógica da busca
// dos cards, via useBuscaDicionario): com o campo vazio a lista mostra as palavras em estudo; ao
// digitar, ela vira o resultado da pesquisa no dicionário. Clicar num item (ou "Adicionar"/Enter)
// chama AdicionarHanziFoco — que coloca em estudo o que estiver fora, se preciso.

interface FocoPopupProps {
  configuracoesApp: config.Config | null;
  AtualizarConfiguracao: (key: keyof config.Config, value: any) => void;
  foco: revisao.ItemFocoRevisao[];
  aoClicarNoFoco?: (item: revisao.ItemFocoRevisao) => void;
  recarregarFoco?: () => void;
  aoFechar: () => void;
}

const REGEX_HAN = /[㐀-鿿]/;

// Opção clicável da lista abaixo do campo (palavra em estudo ou resultado da pesquisa).
interface OpcaoAdicao {
  chave: string;
  texto: string;
  pinyin: string;
  titulo: string;
  jaNoFoco: boolean;
}

export function FocoPopup({ configuracoesApp, AtualizarConfiguracao, foco, aoClicarNoFoco, recarregarFoco, aoFechar }: FocoPopupProps) {
  const auto = configuracoesApp?.tamanhoFocoAutomatico === true;
  const tamanho = configuracoesApp?.tamanhoFocoRevisao || 5;

  // Núcleo de busca compartilhado com a barra de pesquisa dos cards.
  const { termo, setTermo, resultados } = useBuscaDicionario();
  const buscando = termo.trim().length > 0;

  const [sugestoes, setSugestoes] = useState<progresso.Vocab[]>([]);
  const [adicionando, setAdicionando] = useState(false);
  const [erro, setErro] = useState('');

  // Palavras em estudo: a lista padrão (campo vazio). GetVocab já vem da mais recente para a mais antiga.
  useEffect(() => {
    GetVocab()
      .then(v => setSugestoes((v || []).filter(x => x.Status === 'estudo')))
      .catch(() => setSugestoes([]));
  }, [foco]);

  // ESC fecha o pop-up.
  useEffect(() => {
    function aoTeclar(e: KeyboardEvent) {
      if (e.key === 'Escape') aoFechar();
    }
    window.addEventListener('keydown', aoTeclar);
    return () => window.removeEventListener('keydown', aoTeclar);
  }, [aoFechar]);

  const noFoco = new Set(foco.map(f => f.hanzi));

  function adicionar(texto: string) {
    const t = texto.trim();
    if (!t || adicionando) return;
    setAdicionando(true);
    setErro('');

    const estaNoFoco = noFoco.has(t);
    if (!estaNoFoco && !auto) {
      const novoTamanho = Math.min(20, tamanho + 1);
      AtualizarConfiguracao('tamanhoFocoRevisao', novoTamanho);
    }

    AdicionarHanziFoco(t)
      .then(() => {
        setTermo('');
        if (recarregarFoco) recarregarFoco();
      })
      .catch(e => setErro(String(e)))
      .finally(() => setAdicionando(false));
  }

  function remover(texto: string) {
    const t = texto.trim();
    if (!t) return;
    setErro('');

    if (!auto) {
      const novoTamanho = Math.max(1, tamanho - 1);
      AtualizarConfiguracao('tamanhoFocoRevisao', novoTamanho);
    }

    RemoverHanziFoco(t)
      .then(() => {
        if (recarregarFoco) recarregarFoco();
      })
      .catch(e => setErro(String(e)));
  }

  // Uma opção já está "no foco" quando todos os seus caracteres Han já estão no grupo.
  function jaNoFoco(palavra: string): boolean {
    const hans = Array.from(palavra).filter(ch => REGEX_HAN.test(ch));
    return hans.length > 0 && hans.every(ch => noFoco.has(ch));
  }

  // A mesma div mostra as palavras em estudo (campo vazio) ou os resultados da pesquisa (ao digitar).
  const opcoes: OpcaoAdicao[] = buscando
    ? resultados.slice(0, 30).map(r => ({
        chave: r.hanzi,
        texto: r.hanzi,
        pinyin: r.pinyin || '',
        titulo: [r.pinyin, (r.significados || []).slice(0, 2).join(', ')].filter(Boolean).join(' — '),
        jaNoFoco: jaNoFoco(r.hanzi),
      }))
    : sugestoes.slice(0, 30).map(s => ({
        chave: s.Hanzi,
        texto: s.Hanzi,
        pinyin: s.Pinyin || '',
        titulo: s.Significado || '',
        jaNoFoco: jaNoFoco(s.Hanzi),
      }));

  return (
    <div className="revisao-foco-modal-overlay" onClick={aoFechar}>
      <div className="revisao-foco-modal" onClick={e => e.stopPropagation()}>
        <div className="revisao-foco-modal-cabecalho">
          <h3>{t('Grupo de foco')}</h3>
          <button className="revisao-foco-modal-fechar" onClick={aoFechar} title={t("Fechar (Esc)")}>✕</button>
        </div>

        {/* Configuração de tamanho */}
        <div className="revisao-foco-modal-secao">
          <div className="revisao-foco-modal-linha">
            <div className="revisao-foco-modal-rotulo">
              <span>{t('Tamanho automático')}</span>
              <small>{t('Cresce sozinho para caber todos os caracteres em estudo (até 20).')}</small>
            </div>
            <Interruptor ligado={auto} aoAlternar={() => AtualizarConfiguracao('tamanhoFocoAutomatico', !auto)} />
          </div>
          {!auto && (
            <div className="revisao-foco-modal-linha">
              <div className="revisao-foco-modal-rotulo">
                <span>{t('Tamanho do grupo')}</span>
                <small>{t('Quantos caracteres ficam em foco por vez.')}</small>
              </div>
              <div className="revisao-foco-stepper">
                <button type="button" onClick={() => AtualizarConfiguracao('tamanhoFocoRevisao', Math.max(1, tamanho - 1))}>−</button>
                <span>{tamanho}</span>
                <button type="button" onClick={() => AtualizarConfiguracao('tamanhoFocoRevisao', Math.min(20, tamanho + 1))}>+</button>
              </div>
            </div>
          )}
        </div>

        {/* Lista dos caracteres em foco */}
        <div className="revisao-foco-modal-secao">
          <div className="revisao-foco-modal-titulo">{t('No foco agora ({quantidade})', { quantidade: foco.length })}</div>
          {foco.length === 0 ? (
            <div className="revisao-foco-vazio">{t('Nenhum caractere em foco no momento.')}</div>
          ) : (
            <div className="revisao-foco-modal-chips">
              {foco.map(item => (
                <div
                  key={item.hanzi}
                  className="revisao-foco-chip"
                  title={item.significados?.join(', ')}
                  onClick={() => aoClicarNoFoco && aoClicarNoFoco(item)}
                >
                  <span className="revisao-foco-chip-hanzi">{item.hanzi}</span>
                  <span className="revisao-foco-chip-pinyin">{item.pinyin}</span>
                  <span className="revisao-foco-chip-progresso" title={t("Áreas de aprendizado concluídas (streak ≥ 3) — 5/5 libera a sugestão de aprendida")}>
                    {item.areasConcluidas}/5
                  </span>
                  <button
                    type="button"
                    className="revisao-foco-chip-remover"
                    title={t("Remover do foco")}
                    onClick={(e) => {
                      e.stopPropagation();
                      remover(item.hanzi);
                    }}
                  >
                    ✕
                  </button>
                </div>
              ))}
            </div>
          )}
        </div>

        {/* Adição — a caixa é também uma barra de pesquisa no dicionário */}
        <div className="revisao-foco-modal-secao">
          <div className="revisao-foco-modal-titulo">{t('Adicionar ao foco')}</div>
          <div className="revisao-foco-modal-add">
            <div className="revisao-foco-modal-busca">
              <svg className="revisao-foco-modal-busca-icone" width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
                <circle cx="11" cy="11" r="8"></circle>
                <line x1="21" y1="21" x2="16.65" y2="16.65"></line>
              </svg>
              <input
                className="revisao-foco-modal-input"
                value={termo}
                onChange={e => setTermo(e.target.value)}
                onKeyDown={e => { if (e.key === 'Enter') { e.preventDefault(); adicionar(termo); } }}
                placeholder={t("Pesquisar hanzi, pinyin ou significado…")}
                autoComplete="off"
                spellCheck={false}
                lang="zh"
              />
            </div>
            <button
              className="revisao-foco-modal-add-btn"
              onClick={() => adicionar(termo)}
              disabled={!termo.trim() || adicionando}
            >
              {adicionando ? '…' : t('Adicionar')}
            </button>
          </div>
          <div className="revisao-foco-modal-nota">
            {t('Fora do estudo? O caractere entra como "Em estudo" ao ser adicionado.')}
          </div>
          {erro && <div className="revisao-foco-modal-erro">{erro}</div>}

          <div className="revisao-foco-modal-sub">
            {buscando ? t('Resultados (clique para adicionar):') : t('Em estudo (clique para adicionar):')}
          </div>
          {opcoes.length > 0 ? (
            <div className="revisao-foco-modal-sugestoes">
              {opcoes.map(o => (
                <button
                  key={o.chave}
                  type="button"
                  className={`revisao-foco-sugestao${o.jaNoFoco ? ' ja-no-foco' : ''}`}
                  onClick={() => adicionar(o.texto)}
                  disabled={adicionando}
                  title={o.jaNoFoco ? t('Já está no foco') : (o.titulo || t('Adicionar {texto} ao foco', { texto: o.texto }))}
                >
                  <span className="revisao-foco-sugestao-hz">{o.texto}</span>
                  {o.pinyin && <span className="revisao-foco-sugestao-py">{o.pinyin}</span>}
                </button>
              ))}
            </div>
          ) : (
            <div className="revisao-foco-modal-vazio-busca">
              {buscando ? t('Nenhum resultado.') : t('Nenhuma palavra em estudo ainda.')}
            </div>
          )}
        </div>
      </div>
    </div>
  );
}
