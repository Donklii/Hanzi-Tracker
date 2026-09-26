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
//
// Desligamento automático do microfone: com `continuo: true` a Web Speech NÃO encerra sozinha na
// primeira pausa de fala (comportamento padrão dela) — quem decide fechar é o timeout adaptativo
// daqui: uma janela longa enquanto nada foi falado (o usuário ainda está se preparando, e motores
// sem parciais dependem só dela) e uma janela curta de silêncio contada a partir do último
// resultado (pausas para respirar/pensar não fecham o microfone).
//
// Anti-escuta-infinita: como cada resultado renova a janela de silêncio, falar sem parar manteria
// o microfone aberto para sempre (tentativas ilimitadas numa escuta só). Quando `caracteresAlvo`
// é informado, dois limites dimensionados pelo tamanho do alvo encerram a escuta: a duração
// máxima total e a quantidade máxima de hanzis falados. Motivo exposto em `motivoParadaForcada`
// para o consumidor tratar como tentativa falha (perder vida) em vez de reavaliar normalmente.
import { useState, useEffect, useCallback, useRef } from 'react';
import {
  CancelarEscutaStt, DespertarMotorStt, GetConfig, IniciarEscutaStt, ListarMotoresStt, PararEscutaStt,
} from '../../wailsjs/go/main/App';
import { EventsOn } from '../../wailsjs/runtime/runtime';
import { t } from '../i18n/i18n';

// Valor de Config.MotorSttAtivo que seleciona o reconhecimento da própria webview (Web Speech API)
// em vez de um sidecar do catálogo.
export const MOTOR_STT_WEB_SPEECH = 'WebSpeech';

// Janelas do desligamento automático do microfone (timeout adaptativo):
// - MS_ESPERA_SEM_FALA: nada foi reconhecido ainda — janela longa (o usuário está se preparando;
//   é também o único limite quando o motor não publica parciais, ex.: sidecar antigo sem /parcial).
// - MS_ESPERA_SILENCIO_APOS_FALA: já houve fala — conta a partir do ÚLTIMO resultado, tolerando
//   pausas curtas para respirar/pensar sem fechar o microfone no meio da frase.
const MS_ESPERA_SEM_FALA = 12000;
const MS_ESPERA_SILENCIO_APOS_FALA = 5000;

// Limites anti-escuta-infinita (só valem quando `caracteresAlvo` > 0), dimensionados pelo tamanho
// do alvo em hanzis:
// - duração máxima total da escuta = base + tempo por caractere (limite duro, nunca renovado);
// - falar mais de FATOR_MAXIMO_HANZIS_FALADOS × os hanzis do alvo encerra a escuta na hora (mínimo de MINIMO_HANZIS_FALADOS).
const MS_ESCUTA_BASE = 10000;
const MS_ESCUTA_POR_CARACTERE_ALVO = 3000;
const FATOR_MAXIMO_HANZIS_FALADOS = 3;
const MINIMO_HANZIS_FALADOS = 5;

export function TemWebSpeech(): boolean {
  return !!((window as any).SpeechRecognition || (window as any).webkitSpeechRecognition);
}

interface OpcoesSTT {
  idioma?: string;
  continuo?: boolean;
  caracteresAlvo?: number; // hanzis do card/frase em avaliação; ativa os limites anti-escuta-infinita
}

type ModoSTT = 'indefinido' | 'web' | 'motor' | 'nenhum';

// Motivo do fechamento forçado do microfone pelos limites anti-escuta-infinita — null quando a
// escuta foi encerrada normalmente (clique do usuário, silêncio ou acerto detectado em tempo real).
export type MotivoParadaForcada = 'tempo_esgotado' | 'excesso_fala' | null;

export function useSTT(opcoes: OpcoesSTT = {}) {
  const { idioma = 'zh-CN', continuo = false, caracteresAlvo = 0 } = opcoes;

  const [modo, setModo] = useState<ModoSTT>('indefinido');
  const [motivoIndisponivel, setMotivoIndisponivel] = useState<string | null>(null);
  const [escutando, setEscutando] = useState(false);
  const [processando, setProcessando] = useState(false); // motor: entre soltar o botão e a transcrição chegar
  const [transcricaoParcial, setTranscricaoParcial] = useState('');
  const [transcricaoFinal, setTranscricaoFinal] = useState('');
  const [estadoMotor, setEstadoMotor] = useState(''); // mensagens do Go ("Transcrevendo fala…")
  const [erro, setErro] = useState<string | null>(null);
  const [motivoParadaForcada, setMotivoParadaForcada] = useState<MotivoParadaForcada>(null);

  const recognitionRef = useRef<any>(null);
  const iniciarPendenteRef = useRef<Promise<void> | null>(null);
  const escutandoRef = useRef(false);

  const timeoutInatividadeRef = useRef<number | null>(null);
  const timeoutDuracaoMaximaRef = useRef<number | null>(null); // limite duro, nunca renovado
  const houveFalaRef = useRef(false); // já chegou algum resultado nesta escuta? (encurta a janela)
  const pararRef = useRef<() => void>(() => {});

  // O alvo muda entre escutas (ex.: próximo card do baralho) sem remontar o hook — os timers e as
  // checagens leem sempre o valor corrente pela ref.
  const caracteresAlvoRef = useRef(caracteresAlvo);
  caracteresAlvoRef.current = caracteresAlvo;

  const resetarTimeoutInatividade = useCallback(() => {
    if (timeoutInatividadeRef.current) {
      clearTimeout(timeoutInatividadeRef.current);
    }
    const janelaMs = houveFalaRef.current ? MS_ESPERA_SILENCIO_APOS_FALA : MS_ESPERA_SEM_FALA;
    timeoutInatividadeRef.current = window.setTimeout(() => {
      pararRef.current();
    }, janelaMs);
  }, []);

  // Arma o limite duro de duração da escuta, proporcional ao tamanho do alvo. Chamado uma vez por
  // escuta — ao contrário do timeout de inatividade, nenhum resultado o renova.
  const armarLimiteDuracaoMaxima = useCallback(() => {
    if (timeoutDuracaoMaximaRef.current) {
      clearTimeout(timeoutDuracaoMaximaRef.current);
      timeoutDuracaoMaximaRef.current = null;
    }
    if (!caracteresAlvoRef.current) {
      return; // sem alvo informado, só os timeouts de inatividade limitam a escuta
    }
    const msLimite = MS_ESCUTA_BASE + caracteresAlvoRef.current * MS_ESCUTA_POR_CARACTERE_ALVO;
    timeoutDuracaoMaximaRef.current = window.setTimeout(() => {
      setMotivoParadaForcada('tempo_esgotado');
      pararRef.current();
    }, msLimite);
  }, []);

  // Encerra a escuta na hora quando o usuário já falou mais de 3× os hanzis do alvo (mínimo de 5) — falar sem
  // parar deixa de render tentativas ilimitadas.
  const verificarLimiteDeFala = useCallback((textoFalado: string) => {
    const alvo = caracteresAlvoRef.current;
    if (!alvo) {
      return;
    }
    const limiteHanzis = Math.max(MINIMO_HANZIS_FALADOS, alvo * FATOR_MAXIMO_HANZIS_FALADOS);
    if (contarHanzis(textoFalado) <= limiteHanzis) {
      return;
    }
    setMotivoParadaForcada('excesso_fala');
    pararRef.current();
  }, []);

  const limparTimeoutsEscuta = useCallback(() => {
    if (timeoutInatividadeRef.current) {
      clearTimeout(timeoutInatividadeRef.current);
      timeoutInatividadeRef.current = null;
    }
    if (timeoutDuracaoMaximaRef.current) {
      clearTimeout(timeoutDuracaoMaximaRef.current);
      timeoutDuracaoMaximaRef.current = null;
    }
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
        setMotivoIndisponivel(t('A Web Speech API não existe nesta plataforma — selecione um motor baixável em Configurações → Motores → Motor de Escuta.'));
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
            ? t('Baixe o motor de escuta em Configurações → Motores → Reconhecimento de Voz (STT).')
            : t('O motor de escuta ainda não foi publicado para este sistema — aguarde a próxima atualização.'));
        })
        .catch(() => {
          if (cancelado) return;
          if (TemWebSpeech()) {
            setModo('web');
            return;
          }
          setModo('nenhum');
          setMotivoIndisponivel(t('Não foi possível consultar os motores de escuta.'));
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
      setMotivoParadaForcada(null);
      houveFalaRef.current = false;
      resetarTimeoutInatividade();
      armarLimiteDuracaoMaxima();
    };

    recognition.onresult = (event: any) => {
      houveFalaRef.current = true;
      resetarTimeoutInatividade();
      let currentFinal = '';
      let currentInterim = '';
      let textoDaSessao = ''; // transcrição inteira da escuta, para o limite de hanzis falados

      for (let i = 0; i < event.results.length; ++i) {
        textoDaSessao += event.results[i][0].transcript;
        if (i < event.resultIndex) {
          continue; // resultado antigo: já está acumulado em transcricaoFinal
        }
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
      verificarLimiteDeFala(textoDaSessao);
    };

    recognition.onerror = (event: any) => {
      setErro(event.error);
      setEscutando(false);
      limparTimeoutsEscuta();
    };

    recognition.onend = () => {
      setEscutando(false);
      limparTimeoutsEscuta();
    };

    recognitionRef.current = recognition;

    return () => {
      if (recognitionRef.current) {
        recognitionRef.current.abort();
        recognitionRef.current = null;
      }
      limparTimeoutsEscuta();
    };
  }, [modo, idioma, continuo, resetarTimeoutInatividade, armarLimiteDuracaoMaxima, verificarLimiteDeFala, limparTimeoutsEscuta]);

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
      if (texto) {
        houveFalaRef.current = true;
      }
      resetarTimeoutInatividade();
      verificarLimiteDeFala(texto || '');
    });

    return () => {
      desligarEvento();
      desligarParcial();
      if (escutandoRef.current) {
        CancelarEscutaStt().catch(() => { });
      }
      limparTimeoutsEscuta();
    };
  }, [modo, resetarTimeoutInatividade, verificarLimiteDeFala, limparTimeoutsEscuta]);

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
    setMotivoParadaForcada(null);
    setEscutando(true);
    escutandoRef.current = true;
    houveFalaRef.current = false;

    resetarTimeoutInatividade();
    armarLimiteDuracaoMaxima();

    const pendente = IniciarEscutaStt()
      .catch((e: any) => {
        setErro(String(e));
        setEscutando(false);
        escutandoRef.current = false;
        limparTimeoutsEscuta();
        throw e;
      });
    iniciarPendenteRef.current = pendente.catch(() => { });
  }, [modo, escutando, processando, resetarTimeoutInatividade, armarLimiteDuracaoMaxima, limparTimeoutsEscuta]);

  const parar = useCallback(() => {
    limparTimeoutsEscuta();

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
          setErro(t('Nada foi reconhecido — tente falar mais perto do microfone.'));
        }
      })
      .catch((e: any) => setErro(String(e)))
      .finally(() => setProcessando(false));
  }, [modo, escutando, limparTimeoutsEscuta]);

  pararRef.current = parar;

  const limpar = useCallback(() => {
    setTranscricaoFinal('');
    setTranscricaoParcial('');
    // Consome o motivo da parada forçada: sem isso, uma mudança de estado do consumidor (ex.: a
    // fila avançar para o próximo card) reexecutaria o efeito de avaliação e deduziria uma
    // segunda vida pelo mesmo fechamento forçado.
    setMotivoParadaForcada(null);
    limparTimeoutsEscuta();
  }, [limparTimeoutsEscuta]);

  const [nivelAudio, setNivelAudio] = useState(0); // 0.0 a 1.0 (nível de volume da voz do usuário)

  // Monitoramento de áudio em tempo real via Web Audio API (amplitude da voz do usuário)
  useEffect(() => {
    if (!escutando) {
      setNivelAudio(0);
      return;
    }

    let stream: MediaStream | null = null;
    let audioCtx: AudioContext | null = null;
    let animId: number | null = null;
    let cancelado = false;

    const iniciarMonitorAudio = async () => {
      try {
        if (!navigator.mediaDevices?.getUserMedia) return;
        stream = await navigator.mediaDevices.getUserMedia({ audio: true });
        if (cancelado) {
          stream.getTracks().forEach(t => t.stop());
          return;
        }

        const AudioContextClass = window.AudioContext || (window as any).webkitAudioContext;
        if (!AudioContextClass) return;

        audioCtx = new AudioContextClass();
        const source = audioCtx.createMediaStreamSource(stream);
        const analyser = audioCtx.createAnalyser();
        analyser.fftSize = 256;
        analyser.smoothingTimeConstant = 0.3;
        source.connect(analyser);

        const dataArray = new Uint8Array(analyser.frequencyBinCount);
        let volumeSuave = 0;

        // Parâmetros de calibração do áudio (Deadzone + Curva suave de fala)
        const LIMITE_DEADZONE = 24; // ignora ruído de fundo, respiração e ventoinha abaixo deste nível
        const ESCALA_VOLUME_FALA = 80;

        const atualizar = () => {
          if (cancelado) return;
          analyser.getByteFrequencyData(dataArray);

          // Média de amplitude na faixa principal da voz humana (bins 2 a 40)
          let soma = 0;
          const limiteBins = Math.min(dataArray.length, 40);
          for (let i = 2; i < limiteBins; i++) {
            soma += dataArray[i];
          }
          const media = soma / (limiteBins - 2);

          // Aplica deadzone: ignora qualquer som abaixo do limite de ruído ambiente
          const amplitudeUtil = Math.max(0, media - LIMITE_DEADZONE);
          const volLinear = Math.min(1, amplitudeUtil / ESCALA_VOLUME_FALA);
          
          // Curva quadrática para eliminar ruídos residuais e deixar a escala natural
          const volAlvo = Math.pow(volLinear, 1.4);

          // Suavização fluida (EMA) com transição macia
          if (volAlvo > volumeSuave) {
            volumeSuave = volumeSuave * 0.3 + volAlvo * 0.7;
          } else {
            volumeSuave = volumeSuave * 0.85;
          }

          // Arredonda valores irrisórios para 0 para estabilizar o estado de repouso
          const nivelFinal = volumeSuave < 0.02 ? 0 : volumeSuave;

          setNivelAudio(nivelFinal);
          animId = requestAnimationFrame(atualizar);
        };

        atualizar();
      } catch (e) {
        // Se getUserMedia falhar/bloquear, mantém nível zerado (usando o fallback de transcrição)
      }
    };

    iniciarMonitorAudio();

    return () => {
      cancelado = true;
      if (animId) cancelAnimationFrame(animId);
      if (stream) {
        stream.getTracks().forEach(t => t.stop());
      }
      if (audioCtx && audioCtx.state !== 'closed') {
        audioCtx.close().catch(() => {});
      }
      setNivelAudio(0);
    };
  }, [escutando]);

  // Quando há fala sendo reconhecida em tempo real (parcial), garante nível mínimo caso o WebAudio seja bloqueado
  const nivelAudioFinal = Math.max(
    nivelAudio,
    (escutando && transcricaoParcial) ? 0.6 : 0
  );

  const suportado = modo !== 'nenhum';

  return {
    suportado, motivoIndisponivel, escutando, processando,
    transcricaoParcial, transcricaoFinal, estadoMotor, erro, motivoParadaForcada,
    nivelAudio: nivelAudioFinal,
    iniciar, parar, limpar,
  };
}


// ----- Utilitários -----

// Conta só os caracteres chineses (bloco CJK unificado) — pontuação, espaços e latinos que o
// reconhecedor intercala não contam para o limite de fala.
function contarHanzis(texto: string): number {
  return (texto.match(/[一-鿿]/g) || []).length;
}
