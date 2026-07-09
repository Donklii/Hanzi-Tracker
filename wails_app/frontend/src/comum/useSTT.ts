// ----- Reconhecimento de fala (STT) da revisão de pronúncia -----
// Dois caminhos, na ordem de preferência:
//   1. Web Speech API (SpeechRecognition), quando a webview a oferece — grátis e com resultados
//      parciais em tempo real.
//   2. MOTOR de escuta (sidecar Paraformer-ZH, ver stt.go/motoresstt): a webview do Wails no Linux
//      (WebKitGTK) não tem SpeechRecognition NEM getUserMedia, então tanto a gravação do microfone
//      quanto a transcrição acontecem no sidecar — o frontend só comanda iniciar/parar/cancelar
//      (push-to-talk). Sem parciais: a transcrição chega inteira no parar.
// Sem nenhum dos dois (motor não instalado), `suportado` fica false e `motivoIndisponivel` orienta
// o download em Configurações → Motores.
import { useState, useEffect, useCallback, useRef } from 'react';
import {
  CancelarEscutaStt, DespertarMotorStt, IniciarEscutaStt, ListarMotoresStt, PararEscutaStt,
} from '../../wailsjs/go/main/App';
import { EventsOn } from '../../wailsjs/runtime/runtime';

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
  // Promessa do IniciarEscutaStt em voo: parar() espera por ela para nunca chegar ao sidecar
  // antes do iniciar (o usuário pode soltar o botão antes de o motor terminar de subir).
  const iniciarPendenteRef = useRef<Promise<void> | null>(null);
  const escutandoRef = useRef(false);

  // ----- Caminho 1: Web Speech API -----
  useEffect(() => {
    const SpeechRecognition = (window as any).SpeechRecognition || (window as any).webkitSpeechRecognition;
    if (!SpeechRecognition) {
      return; // sem Web Speech: o efeito do motor (abaixo) decide o modo
    }
    setModo('web');

    const recognition = new SpeechRecognition();
    recognition.lang = idioma;
    recognition.continuous = continuo;
    recognition.interimResults = true;

    recognition.onstart = () => {
      setEscutando(true);
      setErro(null);
      setTranscricaoParcial('');
      setTranscricaoFinal('');
    };

    recognition.onresult = (event: any) => {
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
    };

    recognition.onend = () => {
      setEscutando(false);
    };

    recognitionRef.current = recognition;

    return () => {
      if (recognitionRef.current) {
        recognitionRef.current.abort();
      }
    };
  }, [idioma, continuo]);

  // ----- Caminho 2: motor de escuta (sidecar) -----
  useEffect(() => {
    const temWebSpeech = !!((window as any).SpeechRecognition || (window as any).webkitSpeechRecognition);
    if (temWebSpeech) {
      return;
    }

    let cancelado = false;
    ListarMotoresStt()
      .then(motores => {
        if (cancelado) return;
        if ((motores || []).some(m => m.instalado)) {
          setModo('motor');
          // Pré-aquece em segundo plano: sobe o sidecar e carrega o modelo (na primeiríssima vez,
          // baixa os pesos), para o primeiro push-to-talk sair sem essa espera.
          DespertarMotorStt();
          return;
        }
        setModo('nenhum');
        setMotivoIndisponivel((motores || []).some(m => m.publicado)
          ? 'Baixe o motor de escuta em Configurações → Motores → Reconhecimento de Voz (STT).'
          : 'O motor de escuta ainda não foi publicado para este sistema — aguarde a próxima atualização.');
      })
      .catch(() => {
        if (cancelado) return;
        setModo('nenhum');
        setMotivoIndisponivel('Não foi possível consultar os motores de escuta.');
      });

    // Estado da escuta/transcrição vindo do Go ("Iniciando o motor…", "Transcrevendo fala…").
    const desligarEvento = EventsOn('stt_estado', (mensagem: string) => {
      if (!cancelado) setEstadoMotor(mensagem || '');
    });

    return () => {
      cancelado = true;
      desligarEvento();
      // Gravação órfã (desmontou com o botão pressionado): descarta no sidecar.
      if (escutandoRef.current) {
        CancelarEscutaStt().catch(() => { });
      }
    };
  }, []);

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

    // Guard clauses do modo motor: catálogo ainda carregando/indisponível, já escutando ou uma
    // transcrição anterior ainda em voo.
    if (modo !== 'motor' || escutando || processando) {
      return;
    }

    setErro(null);
    setTranscricaoParcial('');
    setTranscricaoFinal('');
    setEscutando(true);
    escutandoRef.current = true;

    const pendente = IniciarEscutaStt()
      .catch((e: any) => {
        setErro(String(e));
        setEscutando(false);
        escutandoRef.current = false;
        throw e; // o parar() em espera desiste junto
      });
    // O catch acima já tratou; engole a rejeição para não virar unhandled rejection.
    iniciarPendenteRef.current = pendente.catch(() => { });
  }, [modo, escutando, processando]);

  const parar = useCallback(() => {
    if (modo === 'web') {
      if (recognitionRef.current && escutando) {
        recognitionRef.current.stop();
      }
      return;
    }

    // Guard clause: nada sendo escutado (iniciar falhou ou nem rodou).
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
        setTranscricaoFinal(texto || '');
        if (!texto) {
          setErro('Nada foi reconhecido — tente falar mais perto do microfone.');
        }
      })
      .catch((e: any) => setErro(String(e)))
      .finally(() => setProcessando(false));
  }, [modo, escutando]);

  // limpar descarta a transcrição já CONSUMIDA pela tela. Sem isso, um efeito que reavalia quando
  // outra dependência muda (ex.: a fila de caracteres da PronunciaSequencia avança) compararia o
  // próximo alvo contra a fala ANTERIOR — a transcrição só se apagaria no próximo iniciar().
  const limpar = useCallback(() => {
    setTranscricaoFinal('');
    setTranscricaoParcial('');
  }, []);

  // `suportado` fica otimista durante a detecção ('indefinido'): evita um flash da mensagem de
  // "não suportado" enquanto ListarMotoresStt responde.
  const suportado = modo !== 'nenhum';

  return {
    suportado, motivoIndisponivel, escutando, processando,
    transcricaoParcial, transcricaoFinal, estadoMotor, erro,
    iniciar, parar, limpar,
  };
}
