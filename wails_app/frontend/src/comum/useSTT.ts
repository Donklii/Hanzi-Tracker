// ----- Reconhecimento de fala (STT) da revisão de pronúncia -----
// Dois caminhos, escolhidos em Configurações → Motores → Motor de Escuta (Config.MotorSttAtivo):
//   1. Web Speech API (SpeechRecognition), quando a webview a oferece — grátis e com resultados
//      parciais em tempo real (valor "WebSpeech" na config).
//   2. MOTOR de escuta (sidecar Paraformer-ZH/Zipformer-ZH-Streaming, ver stt.go/motoresstt): a
//      webview do Wails no Linux (WebKitGTK) não tem SpeechRecognition NEM getUserMedia, então
//      tanto a gravação do microfone quanto a transcrição acontecem no sidecar — o frontend só
//      comanda iniciar/parar/cancelar (push-to-talk). Os parciais chegam pelo evento "stt_parcial"
//      (o Go faz polling do sidecar durante a escuta); a transcrição de verdade chega no parar.
// Fallbacks: motor selecionado mas não instalado cai para a Web Speech quando ela existe;
// sem nenhum caminho, `suportado` fica false e `motivoIndisponivel` orienta a correção em
// Configurações → Motores.
import { useState, useEffect, useCallback, useRef } from 'react';
import {
  CancelarEscutaStt, DespertarMotorStt, GetConfig, IniciarEscutaStt, ListarMotoresStt, PararEscutaStt,
} from '../../wailsjs/go/main/App';
import { EventsOn } from '../../wailsjs/runtime/runtime';

// Valor de Config.MotorSttAtivo que seleciona o reconhecimento da própria webview (Web Speech API)
// em vez de um sidecar do catálogo.
export const MOTOR_STT_WEB_SPEECH = 'WebSpeech';

export function TemWebSpeech(): boolean {
  return !!((window as any).SpeechRecognition || (window as any).webkitSpeechRecognition);
}

interface OpcoesSTT {
  idioma?: string;
  continuo?: boolean;
}

type ModoSTT = 'indefinido' | 'web' | 'motor' | 'nenhum';

export function useSTT(opcoes: OpcoesSTT = {}) {
  const { idioma = 'zh-CN', continuo = false } = opcoes;

  const [modo, setModo] = useState<ModoSTT>('indefinido');
  const [motivoIndisponivel, setMotivoIndisponivel] = useState<string | null>(null);
  const [escutando, setEscutando] = useState(false);
  const [processando, setProcessando] = useState(false); // motor: entre soltar o botão e a transcrição chegar
  const [transcricaoParcial, setTranscricaoParcial] = useState('');
  const [transcricaoFinal, setTranscricaoFinal] = useState('');
  const [estadoMotor, setEstadoMotor] = useState(''); // mensagens do Go ("Transcrevendo fala…")
  const [erro, setErro] = useState<string | null>(null);

  const recognitionRef = useRef<any>(null);
  const iniciarPendenteRef = useRef<Promise<void> | null>(null);
  const escutandoRef = useRef(false);

  const timeoutInatividadeRef = useRef<number | null>(null);
  const pararRef = useRef<() => void>(() => {});

  const resetarTimeoutInatividade = useCallback(() => {
    if (timeoutInatividadeRef.current) {
      clearTimeout(timeoutInatividadeRef.current);
    }
    timeoutInatividadeRef.current = window.setTimeout(() => {
      pararRef.current();
    }, 12000);
  }, []);

  // ----- Decisão do caminho de escuta (Config.MotorSttAtivo) -----
  useEffect(() => {
    let cancelado = false;

    const decidir = (motorConfigurado: string) => {
      if (motorConfigurado === MOTOR_STT_WEB_SPEECH) {
        if (TemWebSpeech()) {
          setModo('web');
          return;
        }
        setModo('nenhum');
        setMotivoIndisponivel('A Web Speech API não existe nesta plataforma — selecione um motor baixável em Configurações → Motores → Motor de Escuta.');
        return;
      }

      ListarMotoresStt()
        .then(motores => {
          if (cancelado) return;
          const alvo = (motores || []).find(m => m.nome === motorConfigurado);
          const instalado = alvo ? alvo.instalado : (motores || []).some(m => m.instalado);
          if (instalado) {
            setModo('motor');
            return;
          }
          if (TemWebSpeech()) {
            setModo('web'); // fallback: motor não instalado, mas a webview sabe escutar
            return;
          }
          setModo('nenhum');
          setMotivoIndisponivel((motores || []).some(m => m.publicado)
            ? 'Baixe o motor de escuta em Configurações → Motores → Reconhecimento de Voz (STT).'
            : 'O motor de escuta ainda não foi publicado para este sistema — aguarde a próxima atualização.');
        })
        .catch(() => {
          if (cancelado) return;
          if (TemWebSpeech()) {
            setModo('web');
            return;
          }
          setModo('nenhum');
          setMotivoIndisponivel('Não foi possível consultar os motores de escuta.');
        });
    };

    GetConfig()
      .then(cfg => {
        if (!cancelado) decidir(cfg?.motorSttAtivo || '');
      })
      .catch(() => {
        if (!cancelado) decidir('');
      });

    return () => {
      cancelado = true;
    };
  }, []);

  // ----- Caminho 1: Web Speech API -----
  useEffect(() => {
    if (modo !== 'web') {
      return;
    }
    const SpeechRecognition = (window as any).SpeechRecognition || (window as any).webkitSpeechRecognition;
    if (!SpeechRecognition) {
      return;
    }

    const recognition = new SpeechRecognition();
    recognition.lang = idioma;
    recognition.continuous = continuo;
    recognition.interimResults = true;

    recognition.onstart = () => {
      setEscutando(true);
      setErro(null);
      setTranscricaoParcial('');
      setTranscricaoFinal('');
      resetarTimeoutInatividade();
    };

    recognition.onresult = (event: any) => {
      resetarTimeoutInatividade();
      let currentFinal = '';
      let currentInterim = '';

      for (let i = event.resultIndex; i < event.results.length; ++i) {
        if (event.results[i].isFinal) {
          currentFinal += event.results[i][0].transcript;
        } else {
          currentInterim += event.results[i][0].transcript;
        }
      }

      setTranscricaoParcial(currentInterim);
      if (currentFinal) {
        setTranscricaoFinal(prev => prev + currentFinal);
      }
    };

    recognition.onerror = (event: any) => {
      setErro(event.error);
      setEscutando(false);
      if (timeoutInatividadeRef.current) {
        clearTimeout(timeoutInatividadeRef.current);
        timeoutInatividadeRef.current = null;
      }
    };

    recognition.onend = () => {
      setEscutando(false);
      if (timeoutInatividadeRef.current) {
        clearTimeout(timeoutInatividadeRef.current);
        timeoutInatividadeRef.current = null;
      }
    };

    recognitionRef.current = recognition;

    return () => {
      if (recognitionRef.current) {
        recognitionRef.current.abort();
        recognitionRef.current = null;
      }
      if (timeoutInatividadeRef.current) {
        clearTimeout(timeoutInatividadeRef.current);
      }
    };
  }, [modo, idioma, continuo, resetarTimeoutInatividade]);

  // ----- Caminho 2: motor de escuta (sidecar) -----
  useEffect(() => {
    if (modo !== 'motor') {
      return;
    }

    // Pré-aquece o sidecar (boot + carga do modelo) para o primeiro push-to-talk sair sem espera
    DespertarMotorStt();

    const desligarEvento = EventsOn('stt_estado', (mensagem: string) => {
      setEstadoMotor(mensagem || '');
    });

    // Parciais em tempo real: o Go consulta o sidecar durante a escuta e emite o texto acumulado
    // (espelha o onresult do Web Speech). Cada parcial novo também renova o timeout de
    // inatividade — o usuário ainda está falando.
    const desligarParcial = EventsOn('stt_parcial', (texto: string) => {
      if (!escutandoRef.current) {
        return; // parcial atrasado de uma escuta já encerrada: o texto final é quem manda
      }
      setTranscricaoParcial(texto || '');
      resetarTimeoutInatividade();
    });

    return () => {
      desligarEvento();
      desligarParcial();
      if (escutandoRef.current) {
        CancelarEscutaStt().catch(() => { });
      }
      if (timeoutInatividadeRef.current) {
        clearTimeout(timeoutInatividadeRef.current);
      }
    };
  }, [modo, resetarTimeoutInatividade]);

  const iniciar = useCallback(() => {
    if (modo === 'web') {
      if (recognitionRef.current && !escutando) {
        try {
          setTranscricaoParcial('');
          setTranscricaoFinal('');
          recognitionRef.current.start();
        } catch (e) {
          console.error("Erro ao iniciar STT:", e);
        }
      }
      return;
    }

    if (modo !== 'motor' || escutando || processando) {
      return;
    }

    setErro(null);
    setTranscricaoParcial('');
    setTranscricaoFinal('');
    setEscutando(true);
    escutandoRef.current = true;

    resetarTimeoutInatividade();

    const pendente = IniciarEscutaStt()
      .catch((e: any) => {
        setErro(String(e));
        setEscutando(false);
        escutandoRef.current = false;
        if (timeoutInatividadeRef.current) {
          clearTimeout(timeoutInatividadeRef.current);
          timeoutInatividadeRef.current = null;
        }
        throw e;
      });
    iniciarPendenteRef.current = pendente.catch(() => { });
  }, [modo, escutando, processando, resetarTimeoutInatividade]);

  const parar = useCallback(() => {
    if (timeoutInatividadeRef.current) {
      clearTimeout(timeoutInatividadeRef.current);
      timeoutInatividadeRef.current = null;
    }

    if (modo === 'web') {
      if (recognitionRef.current && escutando) {
        recognitionRef.current.stop();
      }
      return;
    }

    if (modo !== 'motor' || !escutandoRef.current) {
      return;
    }

    setEscutando(false);
    escutandoRef.current = false;
    setProcessando(true);

    const aposIniciar = iniciarPendenteRef.current || Promise.resolve();
    aposIniciar
      .then(() => PararEscutaStt())
      .then(texto => {
        setTranscricaoParcial(''); // o texto final substitui o último parcial
        setTranscricaoFinal(texto || '');
        if (!texto) {
          setErro('Nada foi reconhecido — tente falar mais perto do microfone.');
        }
      })
      .catch((e: any) => setErro(String(e)))
      .finally(() => setProcessando(false));
  }, [modo, escutando]);

  pararRef.current = parar;

  const limpar = useCallback(() => {
    setTranscricaoFinal('');
    setTranscricaoParcial('');
    if (timeoutInatividadeRef.current) {
      clearTimeout(timeoutInatividadeRef.current);
      timeoutInatividadeRef.current = null;
    }
  }, []);

  const suportado = modo !== 'nenhum';

  return {
    suportado, motivoIndisponivel, escutando, processando,
    transcricaoParcial, transcricaoFinal, estadoMotor, erro,
    iniciar, parar, limpar,
  };
}
