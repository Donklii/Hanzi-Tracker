// ----- Seção: Descobrimento — rastreamento global do mouse e popup de hover -----
// O Go publica a posição global do mouse no evento "mouse_pos". Aqui a posição vira: (1) o cartão
// em foco (o mais próximo do cursor, dentro de um raio) e (2) o popup nativo que aparece quando o
// cursor ESTACIONA sobre uma palavra.
//
// Tudo o que o handler lê são refs, nunca state: ele é registrado uma única vez, no mount, e seu
// closure é o do primeiro render — um state ficaria congelado ali dentro.
import { MutableRefObject, useEffect, useRef } from 'react';
import { config } from '../../wailsjs/go/models';
import { HideHoverPopup, ShowHoverPopup } from '../../wailsjs/go/main/App';
import { EventsOn } from '../../wailsjs/runtime/runtime';
import { useRefEspelho } from '../comum/useRefEspelho';
import { ABAS, Aba } from '../casca/abas';

const LADOS_CAIXA = 4;
// Raio de captura do hover quando a config não define um.
const DISTANCIA_MAXIMA_HOVER_PADRAO_PX = 220;
// Abaixo disso o cursor é considerado parado (o popup só é agendado quando ele se move de verdade).
const MOVIMENTO_MINIMO_PX = 5;
// Tempo parado até o popup abrir, quando a config não define um.
const TEMPO_PARADO_POPUP_PADRAO_MS = 500;

interface OpcoesUseRastreamentoMouse {
  cartoesRef: MutableRefObject<any[]>;
  cartaoEmFocoRef: MutableRefObject<any | null>;
  abaAtivaRef: MutableRefObject<Aba>;
  configuracoesAppRef: MutableRefObject<config.Config | null>;
  setCartaoEmFoco: (cartao: any | null) => void;
  TocarLeituraPinyin: (hanzi: string) => void;
}


export function useRastreamentoMouse(opcoes: OpcoesUseRastreamentoMouse) {
  const { cartoesRef, cartaoEmFocoRef, abaAtivaRef, configuracoesAppRef, setCartaoEmFoco } = opcoes;

  const timeoutPopupRef = useRef<any>(null);
  const ultimaPosicaoMouseRef = useRef<{ x: number, y: number }>({ x: 0, y: 0 });
  // Offset do monitor alvo: o Go manda coordenadas globais, as caixas do OCR são locais ao monitor.
  const offsetMonitorRef = useRef<{ x: number, y: number }>({ x: 0, y: 0 });
  // Enquanto o mouse está sobre um cartão da própria UI, o rastreador global não deve interferir.
  const mouseSobreCartaoUIRef = useRef<boolean>(false);

  const tocarLeituraPinyinRef = useRefEspelho(opcoes.TocarLeituraPinyin);

  useEffect(() => {
    EventsOn('mouse_pos', (data: any) => {
      if (mouseSobreCartaoUIRef.current) return;

      const localX = data.x - offsetMonitorRef.current.x;
      const localY = data.y - offsetMonitorRef.current.y;

      // Fora do Descobrimento não há caixas de OCR na tela para focar.
      const cartaoMaisProximo = abaAtivaRef.current === ABAS.Descobrimento
        ? encontrarCartaoMaisProximo(
            cartoesRef.current,
            localX,
            localY,
            configuracoesAppRef.current?.distanciaMaximaHoverPx || DISTANCIA_MAXIMA_HOVER_PADRAO_PX,
          )
        : null;

      if (!cartaoMaisProximo) {
        if (cartaoEmFocoRef.current == null) return;
        setCartaoEmFoco(null);
        HideHoverPopup();
        cancelarPopupAgendado();
        return;
      }

      setCartaoEmFoco(cartaoMaisProximo);
      if (!configuracoesAppRef.current?.habilitarPopupHover) return;

      // Só reagenda o popup quando o cursor se move: parado, o agendamento anterior amadurece.
      const desvioX = data.x - ultimaPosicaoMouseRef.current.x;
      const desvioY = data.y - ultimaPosicaoMouseRef.current.y;
      const deslocamento = Math.sqrt(desvioX * desvioX + desvioY * desvioY);
      if (deslocamento <= MOVIMENTO_MINIMO_PX) return;

      ultimaPosicaoMouseRef.current = { x: data.x, y: data.y };
      cancelarPopupAgendado();
      HideHoverPopup(); // esconde enquanto está em movimento

      const atraso = configuracoesAppRef.current?.tempoParadoPopupMs || TEMPO_PARADO_POPUP_PADRAO_MS;
      timeoutPopupRef.current = setTimeout(() => {
        ShowHoverPopup(
          cartaoMaisProximo.pinyin || '',
          cartaoMaisProximo.hanzi || '',
          cartaoMaisProximo.significados ? cartaoMaisProximo.significados.join(', ') : '',
          data.x,
          data.y,
        );

        const hanzi = cartaoMaisProximo.hanzi || cartaoMaisProximo.Hanzi;
        if (configuracoesAppRef.current?.lerPinyinAoAbrirPopup && hanzi) {
          tocarLeituraPinyinRef.current(hanzi);
        }
      }, atraso);
    });
  }, []);

  function cancelarPopupAgendado() {
    if (!timeoutPopupRef.current) return;
    clearTimeout(timeoutPopupRef.current);
  }

  return {
    // O offset muda quando o usuário troca o monitor alvo nas configurações.
    definirOffsetMonitor: (offset: { x: number, y: number }) => { offsetMonitorRef.current = offset; },
    // Ligado/desligado pelos handlers de hover dos cartões da UI.
    definirMouseSobreCartaoUI: (sobre: boolean) => { mouseSobreCartaoUIRef.current = sobre; },
  };
}


// ----- Utilitários -----

// Devolve o cartão cuja caixa está mais perto do cursor, dentro do raio. Usa distância ponto-retângulo
// (0 se o cursor está dentro da caixa) em vez de exigir colisão estrita — hover perdoa a mira.
function encontrarCartaoMaisProximo(cartoes: any[], x: number, y: number, distanciaMaxima: number): any | null {
  let maisProximo: any = null;
  let menorDistancia = Infinity;

  for (const cartao of cartoes) {
    if (!cartao.caixa || cartao.caixa.length !== LADOS_CAIXA) continue;

    const distancia = distanciaAteCaixa(x, y, cartao.caixa);
    if (distancia >= menorDistancia || distancia > distanciaMaxima) continue;

    menorDistancia = distancia;
    maisProximo = cartao;
  }

  return maisProximo;
}


// Math.sqrt e não Math.hypot: roda por cartão a cada evento de mouse (caminho quente).
function distanciaAteCaixa(x: number, y: number, caixa: number[]): number {
  const [x0, y0, x1, y1] = caixa;

  let dx = 0;
  if (x < x0) dx = x0 - x;
  else if (x > x1) dx = x - x1;

  let dy = 0;
  if (y < y0) dy = y0 - y;
  else if (y > y1) dy = y - y1;

  return Math.sqrt(dx * dx + dy * dy);
}
