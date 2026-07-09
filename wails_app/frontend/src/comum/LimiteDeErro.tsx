// ----- Seção: Limite de Erro (compartilhado) -----
// Captura exceções de render em qualquer subárvore e mostra uma tela de falha legível em vez de
// derrubar o app inteiro (tela branca). Envolve o <App/> na raiz — ver main.tsx.
import React, { ErrorInfo, ReactNode } from 'react';

interface PropsLimiteDeErro {
  children: ReactNode;
}

interface EstadoLimiteDeErro {
  houveErro: boolean;
  erro: any;
}


export class LimiteDeErro extends React.Component<PropsLimiteDeErro, EstadoLimiteDeErro> {
  constructor(props: PropsLimiteDeErro) {
    super(props);
    this.state = { houveErro: false, erro: null };
  }

  static getDerivedStateFromError(erro: any): EstadoLimiteDeErro {
    return { houveErro: true, erro };
  }

  componentDidCatch(erro: any, infoErro: ErrorInfo) {
    console.error('LimiteDeErro capturou uma exceção de render', erro, infoErro);
  }

  render() {
    if (!this.state.houveErro) {
      return this.props.children;
    }

    return (
      <div style={{ color: 'red', padding: '20px', backgroundColor: 'white' }}>
        <h1>Algo deu errado.</h1>
        <pre>{this.state.erro?.toString()}</pre>
        <pre>{this.state.erro?.stack}</pre>
      </div>
    );
  }
}
