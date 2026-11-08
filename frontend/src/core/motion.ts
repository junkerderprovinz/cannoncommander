// Which motion the page may play. The levels are GlimStone's: off, subtle, wild and the hidden storm. A level above
// subtle is lively by default, so one added later or a page before the attribute is set plays like wild.
const QUIET = ['off', 'subtle'];

export const levelOf = (root: HTMLElement = document.documentElement) => root.dataset.motion ?? '';
const reduced = () => !!globalThis.matchMedia?.('(prefers-reduced-motion: reduce)').matches;

export const moving = (level = levelOf()) => level !== 'off' && !reduced();
export const lively = (level = levelOf()) => !QUIET.includes(level) && !reduced();
export const stormy = (level = levelOf()) => level === 'storm' && !reduced();

export function toMs(value: string) {
  const m = value.trim().match(/^(\d*\.?\d+)(ms|s)$/);
  return m ? Math.round(parseFloat(m[1]!) * (m[2] === 's' ? 1000 : 1)) : 0;
}

const read = (name: string) => getComputedStyle(document.documentElement).getPropertyValue(name);
export const dialMs = (name: string) => toMs(read(name));
// A dial that holds a plain number, a length in pixels or an angle in degrees.
export const dial = (name: string) => parseFloat(read(name)) || 0;
