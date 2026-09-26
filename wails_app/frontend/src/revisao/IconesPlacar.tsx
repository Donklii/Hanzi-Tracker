// ----- Seção: Revisão — Ícones do placar final -----
// Ícones em traço (estilo Feather, o mesmo da barra lateral) usados na tela de finalização:
// um por área de aprendizado (as 5 colunas de streak do vocabulário) e os das estatísticas
// da sessão. Todos herdam a cor do texto via currentColor.
import { ReactNode } from 'react';

interface IconeProps {
  tamanho?: number;
}

function Icone({ tamanho = 16, children }: IconeProps & { children: ReactNode }) {
  return (
    <svg
      width={tamanho}
      height={tamanho}
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth={2}
      strokeLinecap="round"
      strokeLinejoin="round"
      aria-hidden="true"
    >
      {children}
    </svg>
  );
}

// --- Áreas de aprendizado ---

export function IconeSignificado(props: IconeProps) {
  return (
    <Icone {...props}>
      <path d="M2 3h6a4 4 0 0 1 4 4v14a3 3 0 0 0-3-3H2z" />
      <path d="M22 3h-6a4 4 0 0 0-4 4v14a3 3 0 0 1 3-3h7z" />
    </Icone>
  );
}

export function IconeFonetica(props: IconeProps) {
  return (
    <Icone {...props}>
      <polygon points="11 5 6 9 2 9 2 15 6 15 11 19 11 5" />
      <path d="M19.07 4.93a10 10 0 0 1 0 14.14M15.54 8.46a5 5 0 0 1 0 7.07" />
    </Icone>
  );
}

export function IconeDesenho(props: IconeProps) {
  return (
    <Icone {...props}>
      <path d="M17 3a2.828 2.828 0 1 1 4 4L7.5 20.5 2 22l1.5-5.5L17 3z" />
    </Icone>
  );
}

export function IconeContexto(props: IconeProps) {
  return (
    <Icone {...props}>
      <path d="M21 11.5a8.38 8.38 0 0 1-.9 3.8 8.5 8.5 0 0 1-7.6 4.7 8.38 8.38 0 0 1-3.8-.9L3 21l1.9-5.7a8.38 8.38 0 0 1-.9-3.8 8.5 8.5 0 0 1 4.7-7.6 8.38 8.38 0 0 1 3.8-.9h.5a8.48 8.48 0 0 1 8 8v.5z" />
    </Icone>
  );
}

export function IconePronuncia(props: IconeProps) {
  return (
    <Icone {...props}>
      <path d="M12 1a3 3 0 0 0-3 3v8a3 3 0 0 0 6 0V4a3 3 0 0 0-3-3z" />
      <path d="M19 10v2a7 7 0 0 1-14 0v-2" />
      <line x1="12" y1="19" x2="12" y2="23" />
      <line x1="8" y1="23" x2="16" y2="23" />
    </Icone>
  );
}

// --- Estatísticas e adornos da sessão ---

export function IconePontos(props: IconeProps) {
  return (
    <Icone {...props}>
      <polygon points="13 2 3 14 12 14 11 22 21 10 12 10 13 2" />
    </Icone>
  );
}

export function IconeSequencia(props: IconeProps) {
  return (
    <Icone {...props}>
      <path d="M8.5 14.5A2.5 2.5 0 0 0 11 12c0-1.38-.5-2-1-3-1.072-2.143-.224-4.054 2-6 .5 2.5 2 4.9 4 6.5 2 1.6 3 3.5 3 5.5a7 7 0 1 1-14 0c0-1.153.433-2.294 1-3a2.5 2.5 0 0 0 2.5 2.5z" />
    </Icone>
  );
}

export function IconeAcertos(props: IconeProps) {
  return (
    <Icone {...props}>
      <path d="M22 11.08V12a10 10 0 1 1-5.93-9.14" />
      <polyline points="22 4 12 14.01 9 11.01" />
    </Icone>
  );
}

export function IconeCheck(props: IconeProps) {
  return (
    <Icone {...props}>
      <polyline points="20 6 9 17 4 12" />
    </Icone>
  );
}

export function IconeTrofeu(props: IconeProps) {
  return (
    <Icone {...props}>
      <circle cx="12" cy="8" r="7" />
      <polyline points="8.21 13.89 7 23 12 20 17 23 15.79 13.88" />
    </Icone>
  );
}

export function IconeFoco(props: IconeProps) {
  return (
    <Icone {...props}>
      <circle cx="12" cy="12" r="10" />
      <circle cx="12" cy="12" r="6" />
      <circle cx="12" cy="12" r="2" />
    </Icone>
  );
}

export function IconeReset(props: IconeProps) {
  return (
    <Icone {...props}>
      <polyline points="1 4 1 10 7 10" />
      <path d="M3.51 15a9 9 0 1 0 2.13-9.36L1 10" />
    </Icone>
  );
}

// --- Vidas do quebra-cabeça ---

export function IconeCoracao(props: IconeProps) {
  return (
    <Icone {...props}>
      <path d="M20.84 4.61a5.5 5.5 0 0 0-7.78 0L12 5.67l-1.06-1.06a5.5 5.5 0 0 0-7.78 7.78l1.06 1.06L12 21.23l7.78-7.78 1.06-1.06a5.5 5.5 0 0 0 0-7.78z" />
    </Icone>
  );
}

export function IconeCoracaoPartido(props: IconeProps) {
  return (
    <Icone {...props}>
      <path d="M20.84 4.61a5.5 5.5 0 0 0-7.78 0L12 5.67l-1.06-1.06a5.5 5.5 0 0 0-7.78 7.78l1.06 1.06L12 21.23l7.78-7.78 1.06-1.06a5.5 5.5 0 0 0 0-7.78z" />
      <polyline points="12 5.67 10.2 9.3 13.6 12.3 11.2 16.2" />
    </Icone>
  );
}

// --- Registro por área ---
// A ordem é a mesma das colunas de streak do banco; o placar itera por aqui para desenhar as
// pílulas de área de cada palavra.

export interface AreaAprendizado {
  area: string;
  rotulo: string;
  Icone: (props: IconeProps) => JSX.Element;
}

export const AREAS_APRENDIZADO: AreaAprendizado[] = [
  { area: 'significado', rotulo: 'Significado', Icone: IconeSignificado },
  { area: 'fonetica', rotulo: 'Fonética', Icone: IconeFonetica },
  { area: 'desenho', rotulo: 'Desenho', Icone: IconeDesenho },
  { area: 'contexto', rotulo: 'Contexto', Icone: IconeContexto },
  { area: 'pronuncia', rotulo: 'Pronúncia', Icone: IconePronuncia },
];
