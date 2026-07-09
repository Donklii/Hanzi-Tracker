// ----- Seção: Descobrimento — destaques na tela -----
// Desenha, por cima da tela real (janelas overlay do Go), retângulos sobre as palavras que estão
// "em estudo". Dois modos independentes:
//   • destacarEstudoTela        → destaca a palavra inteira quando ela está em estudo;
//   • destacarEstudoParcialTela → destaca só os CARACTERES em estudo dentro de uma palavra maior,
//                                 fatiando a caixa da palavra proporcionalmente.
// As caixas só vão para o Go quando mudam de verdade (comparação por JSON), senão o overlay
// piscaria a cada render.
import { useEffect, useRef } from 'react';
import { progresso } from '../../wailsjs/go/models';
import { STATUS_VOCABULARIO } from '../comum/status';
import { ABAS, Aba } from '../casca/abas';

const LADOS_CAIXA = 4;

interface OpcoesUseDestaquesTela {
  abaAtiva: Aba;
  cartoes: any[];
  cartoesSecao: any[];
  cartoesVocabulario: progresso.Vocab[];
  cartaoEmFoco: any | null;
  destacarEstudoTela?: boolean;
  destacarEstudoParcialTela?: boolean;
}


export function useDestaquesTela(opcoes: OpcoesUseDestaquesTela) {
  const {
    abaAtiva, cartoes, cartoesSecao, cartoesVocabulario, cartaoEmFoco,
    destacarEstudoTela, destacarEstudoParcialTela,
  } = opcoes;

  const ultimoDestaquesEnviadosRef = useRef<string>('');
  const ultimoDestaquesParciaisEnviadosRef = useRef<string>('');

  useEffect(() => {
    if (!bindingsDeDestaqueProntos()) return;

    if (!destacarEstudoTela && !destacarEstudoParcialTela) {
      enviarDestaquesSeMudou([], []);
      return;
    }

    const cartoesDaTela = abaAtiva === ABAS.Descobrimento
      ? cartoes
      : (abaAtiva === ABAS.TelaUnica ? cartoesSecao : []);

    if (cartoesDaTela.length === 0) {
      enviarDestaquesSeMudou([], []);
      return;
    }

    const hanzisEmEstudo = new Set(
      cartoesVocabulario.filter(v => v.Status === STATUS_VOCABULARIO.Estudo).map(v => v.Hanzi)
    );

    const caixas: number[][] = [];
    const caixasParciais: number[][] = [];

    for (const cartao of cartoesDaTela) {
      const hanzi = cartao.hanzi || cartao.Hanzi;
      if (!hanzi || !cartao.caixa || cartao.caixa.length !== LADOS_CAIXA) continue;

      if (destacarEstudoTela && hanzisEmEstudo.has(hanzi)) {
        caixas.push(cartao.caixa);
        continue;
      }

      if (!destacarEstudoParcialTela) continue;
      // O cartão sob o mouse já ganha o destaque de hover — não sobrepor o parcial nele.
      if (ehMesmaCaixa(cartao.caixa, cartaoEmFoco?.caixa)) continue;

      caixasParciais.push(...calcularCaixasParciais(hanzi, cartao.caixa, hanzisEmEstudo));
    }

    enviarDestaquesSeMudou(caixas, caixasParciais);
  }, [destacarEstudoTela, destacarEstudoParcialTela, abaAtiva, cartoes, cartoesSecao, cartoesVocabulario, cartaoEmFoco]);


  // Envia ao Go só o que mudou desde a última vez (o overlay redesenha a cada chamada).
  function enviarDestaquesSeMudou(caixas: number[][], caixasParciais: number[][]) {
    if (!bindingsDeDestaqueProntos()) return;

    const caixasJson = JSON.stringify(caixas);
    if (caixasJson !== ultimoDestaquesEnviadosRef.current) {
      // @ts-ignore — binding injetado pelo Wails em tempo de execução
      window.go.main.App.ShowEstudoHighlights(caixas);
      ultimoDestaquesEnviadosRef.current = caixasJson;
    }

    const parciaisJson = JSON.stringify(caixasParciais);
    if (parciaisJson !== ultimoDestaquesParciaisEnviadosRef.current) {
      // @ts-ignore — binding opcional: versões antigas do backend não expõem o destaque parcial
      if (window.go.main.App.ShowEstudoParcialHighlights) {
        // @ts-ignore
        window.go.main.App.ShowEstudoParcialHighlights(caixasParciais);
      }
      ultimoDestaquesParciaisEnviadosRef.current = parciaisJson;
    }
  }
}


// ----- Utilitários -----

// Fatia a caixa de uma palavra em uma caixa por caractere e devolve só as dos que estão em estudo.
// A palavra pode estar escrita na vertical (altura > largura) ou na horizontal.
function calcularCaixasParciais(hanzi: string, caixa: number[], hanzisEmEstudo: Set<string>): number[][] {
  const [x0, y0, x1, y1] = caixa;
  const largura = x1 - x0;
  const altura = y1 - y0;

  const caracteres = Array.from(hanzi); // Array.from respeita os pares substitutos do unicode
  const parciais: number[][] = [];

  for (let i = 0; i < caracteres.length; i++) {
    if (!hanzisEmEstudo.has(caracteres[i])) continue;

    const fracInicio = i / caracteres.length;
    const fracFim = (i + 1) / caracteres.length;

    parciais.push(altura > largura
      ? [x0, y0 + altura * fracInicio, x1, y0 + altura * fracFim]
      : [x0 + largura * fracInicio, y0, x0 + largura * fracFim, y1]);
  }

  return parciais;
}


function ehMesmaCaixa(caixa: number[], outra?: number[]): boolean {
  if (!outra) return false;
  return caixa.every((valor, i) => valor === outra[i]);
}


// Os bindings do Wails só existem depois que a webview conecta ao Go.
function bindingsDeDestaqueProntos(): boolean {
  // @ts-ignore
  return Boolean(window.go?.main?.App?.ShowEstudoHighlights);
}
