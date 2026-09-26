import React, { useState, useEffect } from 'react';
import { LookupWord } from '../../wailsjs/go/main/App';
import { dicionario } from '../../wailsjs/go/models';
import { t } from '../i18n/i18n';

interface CampoSignificadoCardProps {
  hanzi: string;
  significadosPadrao: string;
  aoCarregarPinyins?: (pinyinsVariantes: string) => void;
}

function obterPrioridadeTipo(item: dicionario.EntradaDicionario): number {
  const tipo = ((item as any).Tipo || (item as any).tipo || '').toLowerCase();
  if (tipo === 'sobrenome' || tipo.includes('surname')) return 1;
  if (tipo === 'variante' || tipo.includes('variant')) return 2;

  const significadosStr = item.Significados ? item.Significados.join(' ').toLowerCase() : '';
  if (significadosStr.includes('sobrenome') || significadosStr.includes('surname')) return 1;
  if (significadosStr.startsWith('variante de') || significadosStr.startsWith('variant of')) return 2;

  return 0;
}

export function CampoSignificadoCard({ hanzi, significadosPadrao, aoCarregarPinyins }: CampoSignificadoCardProps) {
  const [leituras, setLeituras] = useState<dicionario.EntradaDicionario[] | null>(null);
  const [expandido, setExpandido] = useState(false);

  useEffect(() => {
    let cancelado = false;
    setExpandido(false);
    setLeituras(null);

    if (!hanzi) return;

    LookupWord(hanzi)
      .then(entradas => {
        if (cancelado) return;
        if (entradas && entradas.length > 0) {
          if (entradas.length > 1) {
            setLeituras(entradas);
          }

          if (aoCarregarPinyins) {
            const pinyinsUnicos: string[] = [];
            for (const ent of entradas) {
              if (ent.Pinyin && !pinyinsUnicos.includes(ent.Pinyin)) {
                pinyinsUnicos.push(ent.Pinyin);
              }
            }
            if (pinyinsUnicos.length > 0) {
              aoCarregarPinyins(pinyinsUnicos.join(' / '));
            }
          }
        } else {
          setLeituras(null);
        }
      })
      .catch(erro => {
        console.error("Erro ao obter leituras do dicionário:", erro);
        if (!cancelado) setLeituras(null);
      });

    return () => {
      cancelado = true;
    };
  }, [hanzi]);

  const alternarExpandido = (evento: React.MouseEvent) => {
    evento.stopPropagation();
    setExpandido(anterior => !anterior);
  };

  const temMaisDeUmaLeitura = leituras !== null && leituras.length > 1;

  if (!expandido || !leituras) {
    return (
      <div className="card-sigs">
        <span>{significadosPadrao}</span>
        {temMaisDeUmaLeitura && (
          <div>
            <button className="botao-expandir-leituras" onClick={alternarExpandido}>
              {t('Mostrar mais...')}
            </button>
          </div>
        )}
      </div>
    );
  }

  const leiturasOrdenadas = [...leituras].sort(
    (a, b) => obterPrioridadeTipo(a) - obterPrioridadeTipo(b)
  );

  const pinyinsDiferentes = leiturasOrdenadas.some(
    item => item.Pinyin && item.Pinyin !== leiturasOrdenadas[0].Pinyin
  );

  return (
    <div className="card-sigs expandido" onClick={(e) => e.stopPropagation()}>
      <div className="lista-leituras">
        {leiturasOrdenadas.map((item, indice) => (
          <div key={indice} className="item-leitura">
            {pinyinsDiferentes && <span className="pinyin-leitura">[{item.Pinyin}] </span>}
            <span className="texto-leitura">{item.Significados ? item.Significados.join(', ') : ''}</span>
          </div>
        ))}
      </div>
      <button className="botao-expandir-leituras" onClick={alternarExpandido}>
        {t('Mostrar menos...')}
      </button>
    </div>
  );
}
