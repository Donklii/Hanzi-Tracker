// ----- Seção: Árvore Genealógica de Decomposição -----
import { useState, useEffect, useRef, useMemo } from 'react';
import { CaractereCompleto, DecomposeCharacter, LookupWord } from '../../wailsjs/go/main/App';
import { t } from '../i18n/i18n';

const PROFUNDIDADE_MAXIMA_ARVORE = 6;
const LIMITE_GERACOES_INICIAL = 3;

const OPERADORES_IDC = new Set([
  '⿰', '⿱', '⿲', '⿳', '⿴', '⿵',
  '⿶', '⿷', '⿸', '⿹', '⿺', '⿻',
]);

const MARCADORES_DESCONHECIDOS = new Set(['?', '？']);

const IconChevronDown = () => (
  <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5" strokeLinecap="round" strokeLinejoin="round" style={{ width: '14px', height: '14px' }}>
    <polyline points="6 9 12 15 18 9" />
  </svg>
);

const IconChevronUp = () => (
  <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5" strokeLinecap="round" strokeLinejoin="round" style={{ width: '14px', height: '14px' }}>
    <polyline points="18 15 12 9 6 15" />
  </svg>
);

export interface NoArvoreDecomposicao {
  id: string;
  hanzi: string;
  caractereEquivalente?: string;
  pinyin: string;
  significado: string;
  quantidade: number;
  ehRaiz?: boolean;
  filhos: NoArvoreDecomposicao[];
}

interface DadosResolvidosCaractere {
  pinyin: string;
  significado: string;
  caractereEquivalente?: string;
  decomposicaoCrua: string;
}

interface ComponenteDireto {
  caractere: string;
  quantidade: number;
}

interface ArvoreGenealogicaDecomposicaoProps {
  hanziAlvo: string;
  pinyinAlvo?: string;
  significadoAlvo?: string;
  aoClicarNoCaractere: (char: string) => void;
  setPopupInfo: (info: {
    hanzi: string;
    pinyin: string;
    significados: string;
    x: number;
    y: number;
  } | null) => void;
}

const cacheDadosCaractere = new Map<string, DadosResolvidosCaractere>();


export function ArvoreGenealogicaDecomposicao({
  hanziAlvo,
  pinyinAlvo,
  significadoAlvo,
  aoClicarNoCaractere,
  setPopupInfo,
}: ArvoreGenealogicaDecomposicaoProps) {
  const [arvoreRaiz, setArvoreRaiz] = useState<NoArvoreDecomposicao | null>(null);
  const [carregando, setCarregando] = useState<boolean>(false);
  const [expandido, setExpandido] = useState<boolean>(false);
  const containerScrollRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    setExpandido(false);

    if (!hanziAlvo) {
      setArvoreRaiz(null);
      return;
    }

    let cancelado = false;
    setCarregando(true);

    construirArvoreCompleta(hanziAlvo, pinyinAlvo || '', significadoAlvo || '')
      .then((arvore) => {
        if (cancelado) return;
        setArvoreRaiz(arvore);
        setCarregando(false);
      })
      .catch((erro) => {
        console.error('Erro ao construir árvore de decomposição:', erro);
        if (cancelado) return;
        setArvoreRaiz(null);
        setCarregando(false);
      });

    return () => {
      cancelado = true;
    };
  }, [hanziAlvo, pinyinAlvo, significadoAlvo]);

  const totalGeracoes = useMemo(() => {
    if (!arvoreRaiz) return 0;
    return calcularTotalGeracoes(arvoreRaiz);
  }, [arvoreRaiz]);

  const temMaisGeracoes = totalGeracoes > LIMITE_GERACOES_INICIAL;
  const geracaoMaximaVisivel = expandido ? Infinity : LIMITE_GERACOES_INICIAL;

  useEffect(() => {
    if (!arvoreRaiz) return;

    const centralizarScroll = () => {
      const container = containerScrollRef.current;
      if (!container) return;

      const diferencaLargura = container.scrollWidth - container.clientWidth;
      if (diferencaLargura > 0) {
        container.scrollLeft = Math.round(diferencaLargura / 2);
      }
    };

    const frameId = requestAnimationFrame(centralizarScroll);
    const timerId = setTimeout(centralizarScroll, 50);

    return () => {
      cancelAnimationFrame(frameId);
      clearTimeout(timerId);
    };
  }, [arvoreRaiz, expandido]);

  if (carregando) {
    return (
      <div className="arvore-decomposicao-estado">
        {t('Carregando decomposição...')}
      </div>
    );
  }

  if (!arvoreRaiz || arvoreRaiz.filhos.length === 0) {
    return null;
  }

  return (
    <div className="arvore-decomposicao-wrapper">
      <div className="arvore-decomposicao-scroll" ref={containerScrollRef}>
        <div className="arvore-decomposicao-raiz">
          <NoVisualArvore
            no={arvoreRaiz}
            geracaoAtual={1}
            geracaoMaximaVisivel={geracaoMaximaVisivel}
            aoClicarNoCaractere={aoClicarNoCaractere}
            setPopupInfo={setPopupInfo}
            onExpandir={() => setExpandido(true)}
          />
        </div>
      </div>

      {temMaisGeracoes && (
        <div className="arvore-acoes-rodape">
          <button
            type="button"
            className="arvore-botao-expandir"
            onClick={() => setExpandido((prev) => !prev)}
          >
            {expandido ? (
              <>
                <IconChevronUp />
                <span>{t('Limitar às 3 primeiras gerações')}</span>
              </>
            ) : (
              <>
                <IconChevronDown />
                <span>
                  {t('Expandir gerações restantes')} (+{totalGeracoes - LIMITE_GERACOES_INICIAL})
                </span>
              </>
            )}
          </button>
        </div>
      )}
    </div>
  );
}


interface NoVisualArvoreProps {
  no: NoArvoreDecomposicao;
  geracaoAtual: number;
  geracaoMaximaVisivel: number;
  aoClicarNoCaractere: (char: string) => void;
  setPopupInfo: ArvoreGenealogicaDecomposicaoProps['setPopupInfo'];
  onExpandir: () => void;
}


function NoVisualArvore({
  no,
  geracaoAtual,
  geracaoMaximaVisivel,
  aoClicarNoCaractere,
  setPopupInfo,
  onExpandir,
}: NoVisualArvoreProps) {
  const temFilhos = no.filhos && no.filhos.length > 0;
  const deveExibirFilhos = temFilhos && geracaoAtual < geracaoMaximaVisivel;
  const temFilhosOcultos = temFilhos && geracaoAtual >= geracaoMaximaVisivel;

  const rotuloExibicao = no.caractereEquivalente
    ? `${no.hanzi} (${no.caractereEquivalente})`
    : no.hanzi;

  const aoEntrarMouseHandler = (evento: React.MouseEvent<HTMLDivElement>) => {
    if (no.ehRaiz) return;

    const retangulo = evento.currentTarget.getBoundingClientRect();
    setPopupInfo({
      hanzi: rotuloExibicao,
      pinyin: no.pinyin || '',
      significados: no.significado || '',
      x: retangulo.left + retangulo.width / 2,
      y: retangulo.top,
    });
  };

  const aoSairMouseHandler = () => {
    if (no.ehRaiz) return;
    setPopupInfo(null);
  };

  const aoClicarHandler = () => {
    if (no.ehRaiz) return;
    setPopupInfo(null);
    aoClicarNoCaractere(no.hanzi);
  };

  return (
    <div className="arvore-no-grupo">
      <div
        className={`arvore-no-card ${no.ehRaiz ? 'arvore-no-card-raiz' : 'arvore-no-card-clicavel'}`}
        onMouseEnter={aoEntrarMouseHandler}
        onMouseLeave={aoSairMouseHandler}
        onClick={aoClicarHandler}
      >
        {no.quantidade > 1 && (
          <span className="arvore-no-quantidade" title={`${no.quantidade}x`}>
            ×{no.quantidade}
          </span>
        )}

        <div className="arvore-no-hanzi">
          {no.hanzi}
          {no.caractereEquivalente && (
            <span className="arvore-no-equivalente">({no.caractereEquivalente})</span>
          )}
        </div>

        {no.pinyin && (
          <div className="arvore-no-pinyin">{no.pinyin}</div>
        )}

        {no.significado && (
          <div className="arvore-no-significado">{no.significado}</div>
        )}

        {temFilhosOcultos && (
          <button
            type="button"
            className="arvore-no-mais-filhos"
            title={t('Expandir gerações')}
            onClick={(evento) => {
              evento.stopPropagation();
              onExpandir();
            }}
          >
            +{no.filhos.length}
          </button>
        )}
      </div>

      {deveExibirFilhos && (
        <div className="arvore-filhos">
          {no.filhos.map((filho) => (
            <div key={filho.id} className="arvore-ramo">
              <NoVisualArvore
                no={filho}
                geracaoAtual={geracaoAtual + 1}
                geracaoMaximaVisivel={geracaoMaximaVisivel}
                aoClicarNoCaractere={aoClicarNoCaractere}
                setPopupInfo={setPopupInfo}
                onExpandir={onExpandir}
              />
            </div>
          ))}
        </div>
      )}
    </div>
  );
}


function calcularTotalGeracoes(no: NoArvoreDecomposicao): number {
  if (!no.filhos || no.filhos.length === 0) {
    return 1;
  }

  let maxProfundidadeFilhos = 0;
  for (const filho of no.filhos) {
    const prof = calcularTotalGeracoes(filho);
    if (prof > maxProfundidadeFilhos) {
      maxProfundidadeFilhos = prof;
    }
  }

  return 1 + maxProfundidadeFilhos;
}


async function construirArvoreCompleta(
  hanziAlvo: string,
  pinyinAlvo: string,
  significadoAlvo: string
): Promise<NoArvoreDecomposicao> {
  const caracteres = Array.from(hanziAlvo);

  if (caracteres.length > 1) {
    const visitadosIniciais = new Set<string>([hanziAlvo]);
    const filhos = await Promise.all(
      caracteres.map((char, idx) =>
        construirNoCaractere(char, `raiz.${idx}`, visitadosIniciais, 1, 1)
      )
    );

    return {
      id: 'raiz',
      hanzi: hanziAlvo,
      pinyin: pinyinAlvo,
      significado: significadoAlvo,
      quantidade: 1,
      ehRaiz: true,
      filhos,
    };
  }

  const noUnico = await construirNoCaractere(hanziAlvo, 'raiz', new Set<string>(), 0, 1);
  return {
    ...noUnico,
    ehRaiz: true,
    pinyin: pinyinAlvo || noUnico.pinyin,
    significado: significadoAlvo || noUnico.significado,
  };
}


async function construirNoCaractere(
  caractere: string,
  idNo: string,
  ancestraisVisitados: Set<string>,
  profundidade: number,
  quantidade: number
): Promise<NoArvoreDecomposicao> {
  const dados = await resolverDadosCaractere(caractere);

  if (profundidade >= PROFUNDIDADE_MAXIMA_ARVORE || ancestraisVisitados.has(caractere)) {
    return {
      id: idNo,
      hanzi: caractere,
      caractereEquivalente: dados.caractereEquivalente,
      pinyin: dados.pinyin,
      significado: dados.significado,
      quantidade,
      filhos: [],
    };
  }

  const proximoVisitados = new Set(ancestraisVisitados);
  proximoVisitados.add(caractere);

  const componentesDiretos = extrairComponentesDiretos(dados.decomposicaoCrua, caractere)
    .filter((comp) => !proximoVisitados.has(comp.caractere));

  const filhos = await Promise.all(
    componentesDiretos.map((comp, idx) =>
      construirNoCaractere(
        comp.caractere,
        `${idNo}.${idx}`,
        proximoVisitados,
        profundidade + 1,
        comp.quantidade
      )
    )
  );

  return {
    id: idNo,
    hanzi: caractere,
    caractereEquivalente: dados.caractereEquivalente,
    pinyin: dados.pinyin,
    significado: dados.significado,
    quantidade,
    filhos,
  };
}


async function resolverDadosCaractere(caractere: string): Promise<DadosResolvidosCaractere> {
  const emCache = cacheDadosCaractere.get(caractere);
  if (emCache) {
    return emCache;
  }

  const [decomposicao, entradas, completo] = await Promise.all([
    DecomposeCharacter(caractere).catch(() => null),
    LookupWord(caractere).catch(() => []),
    CaractereCompleto(caractere).catch(() => ''),
  ]);

  let pinyin = '';
  let significado = '';
  let caractereEquivalente: string | undefined = undefined;

  if (entradas && entradas.length > 0) {
    pinyin = entradas[0].Pinyin || '';
    significado = entradas[0].Significados ? entradas[0].Significados.join(', ') : '';
  }

  if (decomposicao) {
    if (!pinyin && decomposicao.pinyin && decomposicao.pinyin.length > 0) {
      pinyin = decomposicao.pinyin.join(', ');
    }
    if (decomposicao.definition) {
      significado = decomposicao.definition;
    }
  }

  if (completo && completo !== caractere) {
    caractereEquivalente = completo;
    if (!pinyin || !significado) {
      const [decomposicaoCompleto, entradasCompleto] = await Promise.all([
        DecomposeCharacter(completo).catch(() => null),
        LookupWord(completo).catch(() => []),
      ]);

      if (!pinyin) {
        if (entradasCompleto && entradasCompleto.length > 0 && entradasCompleto[0].Pinyin) {
          pinyin = entradasCompleto[0].Pinyin;
        } else if (decomposicaoCompleto?.pinyin && decomposicaoCompleto.pinyin.length > 0) {
          pinyin = decomposicaoCompleto.pinyin.join(', ');
        }
      }

      if (!significado) {
        if (decomposicaoCompleto?.definition) {
          significado = decomposicaoCompleto.definition;
        } else if (entradasCompleto && entradasCompleto.length > 0 && entradasCompleto[0].Significados) {
          significado = entradasCompleto[0].Significados.join(', ');
        }
      }
    }
  }

  const resultado: DadosResolvidosCaractere = {
    pinyin,
    significado,
    caractereEquivalente,
    decomposicaoCrua: decomposicao?.decomposition || '',
  };

  cacheDadosCaractere.set(caractere, resultado);
  return resultado;
}


function extrairComponentesDiretos(decomposicaoCrua: string, caracterePai: string): ComponenteDireto[] {
  if (!decomposicaoCrua) {
    return [];
  }

  const contagem = new Map<string, number>();
  const ordem: string[] = [];

  for (const char of Array.from(decomposicaoCrua)) {
    if (
      OPERADORES_IDC.has(char) ||
      MARCADORES_DESCONHECIDOS.has(char) ||
      char === caracterePai ||
      !char.trim()
    ) {
      continue;
    }

    if (!contagem.has(char)) {
      ordem.push(char);
      contagem.set(char, 1);
    } else {
      contagem.set(char, (contagem.get(char) || 1) + 1);
    }
  }

  return ordem.map((caractere) => ({
    caractere,
    quantidade: contagem.get(caractere) || 1,
  }));
}
