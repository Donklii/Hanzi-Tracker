import { FalarPinyin } from '../../wailsjs/go/main/App';

/**
 * Solicita a síntese de áudio (ou recupera do cache do Go) e toca o áudio retornado.
 * Resolve a Promise quando o áudio termina de tocar, ou em caso de erro/timeout.
 */
export function TocarAudio(hanzi: string, motor: string): Promise<void> {
  if (!hanzi) return Promise.resolve();

  const tsInicio = Date.now();
  return new Promise((resolve) => {
    FalarPinyin(hanzi, motor || 'Kokoro-82M (Leve)')
      .then(b64 => {
        if (Date.now() - tsInicio > 5000) {
          resolve(); // Ignora se demorou muito
          return;
        }
        if (b64) {
          const audio = new Audio('data:audio/wav;base64,' + b64);
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
