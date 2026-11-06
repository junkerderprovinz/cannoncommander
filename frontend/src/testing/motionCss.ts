// The app's stylesheets as the motion guards read them: every .css under src, with comments blanked so the notes
// beside a rule never count as code, and offsets kept so a finding can name its line.
import { readdirSync, readFileSync } from 'node:fs';
import { dirname, join, relative } from 'node:path';
import { fileURLToPath } from 'node:url';

export const SRC = join(dirname(fileURLToPath(import.meta.url)), '..');

export interface Sheet { file: string; css: string }

const blank = (text: string) => text.replace(/\/\*[\s\S]*?\*\//g, (m) => m.replace(/[^\n]/g, ' '));

export function sheets(): Sheet[] {
  return readdirSync(SRC, { recursive: true, encoding: 'utf8' })
    .filter((f) => f.endsWith('.css'))
    .map((f) => ({ file: relative(SRC, join(SRC, f)).replaceAll('\\', '/'), css: blank(readFileSync(join(SRC, f), 'utf8')) }));
}

export const glimstoneTokens = () => blank(readFileSync(join(SRC, '..', 'node_modules', '@glimstone', 'core', 'tokens.css'), 'utf8'));

export const lineOf = (css: string, at: number) => css.slice(0, at).split('\n').length;

// The index of the brace that closes the block opened at or after `from`.
export function closing(css: string, from: number) {
  let depth = 0;
  for (let i = css.indexOf('{', from); i < css.length; i++) {
    if (css[i] === '{') depth++;
    else if (css[i] === '}' && --depth === 0) return i;
  }
  return css.length;
}

/** The ranges of every `@media` block that plays motion only for a system that has not asked for less. */
export function gates(css: string): [number, number][] {
  return [...css.matchAll(/@media[^{]*prefers-reduced-motion:\s*no-preference[^{]*\{/g)].map((m) => [m.index, closing(css, m.index)]);
}

export const inside = (ranges: [number, number][], at: number) => ranges.some(([a, b]) => at > a && at < b);

/** Each rule's selector and body at the innermost level, with where its body starts. */
export function rules(css: string): { selector: string; body: string; at: number }[] {
  return [...css.matchAll(/([^{};]+)\{([^{}]*)\}/g)].map((m) => ({ selector: m[1]!.trim(), body: m[2]!, at: m.index + m[1]!.length }));
}

/** The keyframes blocks by name, with their frames. */
export function keyframes(css: string): { name: string; body: string; at: number }[] {
  return [...css.matchAll(/@keyframes\s+([\w-]+)\s*\{/g)].map((m) => ({ name: m[1]!, body: css.slice(m.index + m[0].length, closing(css, m.index)), at: m.index }));
}
