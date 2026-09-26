// ----- Seção: Jornada de Revisão — Detalhe do Nível -----
// Lista as revisões do nível na ordem, com o estado derivado do contador de revisões concluídas:
// as anteriores estão feitas, a da vez tem o botão "Começar" e as seguintes ficam bloqueadas — o
// usuário só avança acertando tudo da revisão da vez.
import './jornada.css';
import { t } from '../../i18n/i18n';
import { ICONES_JORNADA } from './iconesJornada';
import { DIFICULDADES_JORNADA, NivelJornada, RamoJornada, REVISOES_JORNADA } from './tiposJornada';

interface ModalNivelJornadaProps {
  nivel: NivelJornada;
  ramo: RamoJornada;
  revisoesFeitas: number;
  aoComecar: (revIndex: number) => void;
  aoFechar: () => void;
}

export function ModalNivelJornada({ nivel, ramo, revisoesFeitas, aoComecar, aoFechar }: ModalNivelJornadaProps) {
  return (
    <div className="jornada-modal-fundo" onClick={aoFechar}>
      <div className="jornada-modal" onClick={e => e.stopPropagation()}>
        <div className="jornada-modal-cabecalho">
          <h3 className="jornada-modal-titulo">
            <span className="jornada-icone-titulo">{ICONES_JORNADA[ramo.icone] || ICONES_JORNADA.ramo}</span>
            {t(nivel.titulo)}
          </h3>
          <button className="jornada-modal-fechar" onClick={aoFechar} title={t('Fechar')}>✕</button>
        </div>

        <div className="jornada-modal-chips">
          <span className="jornada-chip-ramo">🏷️ {t(ramo.nome)}</span>
          <span className="jornada-chip-dificuldade">📊 {t(DIFICULDADES_JORNADA[ramo.dificuldade] || ramo.dificuldade)}</span>
          <span className="jornada-chip-progresso">
            {t('{feitas}/{total} revisões', { feitas: Math.min(revisoesFeitas, nivel.revisoes.length), total: nivel.revisoes.length })}
          </span>
        </div>

        <div className="jornada-modal-explicacao">
          {t('Complete as revisões em sequência. Você só avança depois de acertar todas as questões de cada revisão.')}
        </div>

        <div className="jornada-modal-revisoes">
          {nivel.revisoes.map((tipo, indice) => {
            const definicao = REVISOES_JORNADA[tipo] || REVISOES_JORNADA.palavras;
            const feita = indice < revisoesFeitas;
            const atual = indice === revisoesFeitas;
            const estado = feita ? 'feita' : atual ? 'atual' : 'bloqueada';
            // As palavras do nível aparecem na revisão que as APRESENTA (a primeira, de palavras).
            const mostrarPalavras = tipo === 'palavras';

            return (
              <div
                key={`${tipo}-${indice}`}
                className={`jornada-revisao ${estado}${(feita || atual) ? ' clicavel' : ''}`}
                onClick={(feita || atual) ? () => aoComecar(indice) : undefined}
              >
                <span className="jornada-revisao-icone">{ICONES_JORNADA[definicao.icone] || ICONES_JORNADA.significado}</span>
                <div className="jornada-revisao-textos">
                  <div className="jornada-revisao-titulo">{indice + 1}. {t(definicao.titulo)}</div>
                  <div className="jornada-revisao-descricao">{t(definicao.descricao)}</div>
                  {mostrarPalavras && <div className="jornada-revisao-palavras">{nivel.palavras.join(' · ')}</div>}
                </div>
                {feita && (
                  <button
                    className="jornada-botao-rejogar"
                    onClick={(e) => {
                      e.stopPropagation();
                      aoComecar(indice);
                    }}
                    title={t('Rejogar esta revisão')}
                  >
                    <span className="jornada-icone-rejogar">{ICONES_JORNADA.check}</span>
                    {t('Rejogar')}
                  </button>
                )}
                {atual && (
                  <button
                    className="jornada-botao-comecar"
                    onClick={(e) => {
                      e.stopPropagation();
                      aoComecar(indice);
                    }}
                  >
                    {t('Começar')}
                  </button>
                )}
                {!feita && !atual && <span className="jornada-revisao-estado bloqueada">{ICONES_JORNADA.lock}</span>}
              </div>
            );
          })}
        </div>
      </div>
    </div>
  );
}
