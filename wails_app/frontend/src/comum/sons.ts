// ----- Seção: Revisão — Sons sintetizados (Web Audio) -----
// Jingles curtos de feedback (acerto/erro/conclusão/traço) gerados por osciladores em tempo real:
// zero assets de áudio, zero dependências, funciona offline e não engorda o binário. O timbre
// imita a linguagem sonora do Duolingo: terça ascendente no acerto (subindo o tom conforme o
// combo), queda grave no erro e arpejo de fanfarra ao concluir a sessão.
//
// O AudioContext é criado preguiçosamente na primeira nota — sempre após um clique do usuário,
// o que satisfaz a política de autoplay do WebView.

let contextoAudio: AudioContext | null = null;
let sonsHabilitados = true;

// Liga/desliga globalmente (controlado pela config sonsRevisao — ver AbaRevisao).
export function definirSonsRevisaoHabilitados(valor: boolean) {
  sonsHabilitados = valor;
}

function obterContexto(): AudioContext | null {
  if (!sonsHabilitados) return null;
  try {
    if (!contextoAudio) contextoAudio = new AudioContext();
    if (contextoAudio.state === 'suspended') contextoAudio.resume();
    return contextoAudio;
  } catch {
    return null; // sem suporte a áudio: a revisão segue muda, sem quebrar
  }
}

// tocarNota agenda uma nota com envelope (ataque de 12ms + decaimento exponencial) — o envelope
// evita os "cliques" de corte seco de osciladores.
function tocarNota(frequencia: number, aposMs: number, duracaoMs: number, tipo: OscillatorType, volume: number) {
  const ctx = obterContexto();
  if (!ctx) return;

  const inicio = ctx.currentTime + aposMs / 1000;
  const fim = inicio + duracaoMs / 1000;

  const oscilador = ctx.createOscillator();
  const ganho = ctx.createGain();
  oscilador.type = tipo;
  oscilador.frequency.setValueAtTime(frequencia, inicio);

  ganho.gain.setValueAtTime(0, inicio);
  ganho.gain.linearRampToValueAtTime(volume, inicio + 0.012);
  ganho.gain.exponentialRampToValueAtTime(0.0001, fim);

  oscilador.connect(ganho);
  ganho.connect(ctx.destination);
  oscilador.start(inicio);
  oscilador.stop(fim + 0.05);
}

// Acerto: E5→B5 ("ding" ascendente). O combo sobe o tom em 1 semitom por acerto seguido (máx. 6),
// como o som de combo do Duolingo — recompensa audível por manter a sequência.
export function tocarSomAcerto(combo = 0) {
  const fator = Math.pow(2, Math.min(combo, 6) / 12);
  tocarNota(659.26 * fator, 0, 140, 'sine', 0.22);
  tocarNota(987.77 * fator, 90, 240, 'sine', 0.2);
}

// Erro: Bb3→F3, queda grave e curta — inconfundível sem ser punitivo.
export function tocarSomErro() {
  tocarNota(233.08, 0, 160, 'triangle', 0.25);
  tocarNota(174.61, 140, 320, 'triangle', 0.25);
}

// Conclusão da sessão: arpejo de fanfarra C5–E5–G5–C6.
export function tocarSomConclusao() {
  tocarNota(523.25, 0, 130, 'triangle', 0.2);
  tocarNota(659.26, 110, 130, 'triangle', 0.2);
  tocarNota(783.99, 220, 130, 'triangle', 0.2);
  tocarNota(1046.5, 330, 420, 'triangle', 0.22);
}

// Feedback por traço no canvas de desenho: tick sutil no acerto, tum grave no erro.
export function tocarSomTracoOk() {
  tocarNota(1318.51, 0, 45, 'sine', 0.05);
}

export function tocarSomTracoErro() {
  tocarNota(196, 0, 70, 'square', 0.05);
}
