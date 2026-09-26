import { FalarPinyin } from '../../wailsjs/go/main/App';

// "Só a requisição mais nova toca": token monotônico + referência ao áudio em curso, compartilhados
// por todas as chamadas de TocarAudio. Cada nova chamada supera as anteriores — uma síntese que só
// fica pronta depois (o usuário já clicou em outro card/hanzi) não toca, e o áudio anterior é parado
// para nunca se sobreporem.
let idUltimaReproducao = 0;
let audioEmCurso: HTMLAudioElement | null = null;

// Áudio que demora mais que isso já perdeu a janela de utilidade: descarta em vez de tocar tarde.
const VALIDADE_AUDIO_MS = 5000;

/**
 * Solicita a síntese de áudio (ou recupera do cache do Go) e toca o áudio retornado.
 * Resolve a Promise quando o áudio termina de tocar, ou em caso de erro/timeout. Apenas a chamada
 * MAIS RECENTE de fato toca (as anteriores são superadas), evitando áudios sobrepostos.
 */
export function TocarAudio(hanzi: string, motor: string): Promise<void> {
  if (!hanzi) return Promise.resolve();

  const idLocal = ++idUltimaReproducao;
  // Para o áudio anterior na hora: só a reprodução mais nova soa.
  if (audioEmCurso) {
    audioEmCurso.pause();
    audioEmCurso = null;
  }

  const tsInicio = Date.now();
  return new Promise((resolve) => {
    // Fallback pelo NOME de catálogo ("Kokoro-82M"), não pelo rótulo de UI ("Kokoro-82M (Leve)") —
    // FalarPinyin valida o nome contra o catálogo e recusaria o rótulo como motor desconhecido.
    FalarPinyin(hanzi, motor || 'Kokoro-82M')
      .then(b64 => {
        if (idLocal !== idUltimaReproducao) {
          resolve(); // outra reprodução foi pedida enquanto sintetizávamos
          return;
        }
        if (Date.now() - tsInicio > VALIDADE_AUDIO_MS) {
          resolve(); // Ignora se demorou muito
          return;
        }
        if (b64) {
          const audio = new Audio('data:audio/wav;base64,' + b64);
          audioEmCurso = audio;
          audio.onended = () => resolve();
          audio.onerror = () => resolve();
          audio.play().catch(e => {
            console.error("Erro autoplay: ", e);
            resolve();
          });
        } else {
          resolve();
        }
      })
      .catch((err) => {
        console.error("FalarPinyin erro:", err);
        resolve();
      });
  });
}
