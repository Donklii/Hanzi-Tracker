// ----- Seção: Leitura em voz alta do pinyin (compartilhado) -----
// Pede a síntese ao Go (que resolve sidecar + cache) e toca o WAV devolvido em base64. A reprodução
// acontece AQUI (webview) porque o popup nativo Win32 não tem áudio.
//
// Recebe o REF de config, não o state: também é chamada de dentro do handler de mouse_pos, cujo
// closure é o do primeiro render.
import { MutableRefObject, useRef } from 'react';
import { config } from '../../wailsjs/go/models';
import { FalarPinyin } from '../../wailsjs/go/main/App';

// Áudio que demora mais que isso já perdeu a janela de utilidade: o usuário seguiu o mouse adiante.
const VALIDADE_AUDIO_MS = 5000;

interface OpcoesUseLeituraPinyin {
  configuracoesAppRef: MutableRefObject<config.Config | null>;
  setStatus: (mensagem: string) => void;
}


export function useLeituraPinyin({ configuracoesAppRef, setStatus }: OpcoesUseLeituraPinyin) {
  // Sequencial das leituras: só a última pedida pode tocar (as anteriores viram lixo em voo).
  const idLeituraRef = useRef(0);

  const TocarLeituraPinyin = (hanzi: string) => {
    const cfg = configuracoesAppRef.current;
    if (!cfg?.habilitarLeituraPinyin || !hanzi) return;

    const idLocal = ++idLeituraRef.current;
    const tsInicio = Date.now();

    FalarPinyin(hanzi, cfg.motorTtsAtivo)
      .then(b64 => {
        if (idLeituraRef.current !== idLocal) return;          // outra palavra entrou na fila
        if (Date.now() - tsInicio > VALIDADE_AUDIO_MS) return; // síntese demorou demais
        if (!b64) return;

        new Audio('data:audio/wav;base64,' + b64).play().catch(() => { });
      })
      .catch((err: any) => {
        if (idLeituraRef.current === idLocal) {
          setStatus('⚠️ Leitura em voz alta: ' + String(err));
        }
      });
  };

  return TocarLeituraPinyin;
}
