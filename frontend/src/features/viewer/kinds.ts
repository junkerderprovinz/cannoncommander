// Which renderer a file gets, and the small helpers every renderer shares.
import formats from '@/core/formats.json';
import { locale, t } from '@/i18n';

export type ViewKind = 'dir' | 'img' | 'video' | 'audio' | 'pdf' | 'office' | 'md' | 'text' | 'code' | 'arch' | 'disk' | 'epub' | 'model' | 'file';

export interface FileNode {
  name: string;
  dir?: boolean;
  size: number;
  time: number;
  mode?: string;
  owner?: string;
  group?: string;
  link?: string;
  hidden?: boolean;
  at?: string;
  items?: number;
}

const FORMAT: Record<string, string> = Object.fromEntries((formats as { id: string; ext: string[] }[]).flatMap((f) => f.ext.map((e) => [e, f.id])));
const TEXTY = new Set(['text', 'config', 'log', 'subtitle', 'playlist', 'checksum', 'key', 'calendar', 'mail', 'weblink']);
// A few files without an extension are scripts or config all the same.
const CODE_NAMES = ['go', 'ident', 'Dockerfile', 'Makefile'];
const MODELS = ['stl', 'obj', '3mf', 'gltf', 'glb', 'ply', 'fbx'];

export const ext = (name: string) => (name.includes('.') ? name.split('.').pop()!.toLowerCase() : '');
export const formatOf = (name: string) => FORMAT[ext(name)] || '';
export const stem = (name: string) => name.replace(/\.[^.]+$/, '');

export function viewKind(n: { name: string; dir?: boolean }): ViewKind {
  if (n.dir) return 'dir';
  const e = ext(n.name);
  if (CODE_NAMES.includes(n.name)) return 'code';
  // Rotated logs such as syslog.1 or engine.log.2 are still logs.
  if (/\.log\.\d+$/.test(n.name) || /^\d$/.test(e)) return 'text';
  if (e === 'md' || e === 'markdown') return 'md';
  const f = FORMAT[e];
  if (f === 'image' || f === 'raw') return 'img';
  if (f === 'video' || f === 'audio' || f === 'pdf') return f;
  if (f === 'doc' || f === 'sheet' || f === 'slides') return 'office';
  if (f === 'archive') return 'arch';
  if (f === 'diskimage') return 'disk';
  // YAML reads like code more than like a settings file, as the prototype has it.
  if (f === 'code' || e === 'yml' || e === 'yaml') return 'code';
  if (f === 'ebook') return e === 'epub' ? 'epub' : 'file';
  if (f === 'model') return MODELS.includes(e) ? 'model' : 'file';
  if (f && TEXTY.has(f)) return 'text';
  return 'file';
}

export const editable = (n: { name: string; dir?: boolean }) => !n.dir && ['text', 'code', 'md'].includes(viewKind(n));

// Browsers draw these themselves; everything else in the picture group comes through the server's thumbnailer.
const DRAWABLE = ['jpg', 'jpeg', 'png', 'gif', 'webp', 'avif', 'svg', 'bmp', 'ico'];
export const browserDraws = (name: string) => DRAWABLE.includes(ext(name));

const LANG: Record<string, string> = {
  yml: 'YAML', yaml: 'YAML', go: 'Go', json: 'JSON', xml: 'XML', md: 'Markdown', sh: 'Shell', js: 'JavaScript', ts: 'TypeScript',
  py: 'Python', rs: 'Rust', html: 'HTML', css: 'CSS', vue: 'Vue', sql: 'SQL', toml: 'TOML', ini: 'INI',
};
const LANG_KEY: Record<string, string> = { conf: 'nginx', cfg: 'config', log: 'log', 1: 'log', srt: 'subtitle', sha256: 'checksum', txt: 'text' };
export function langOf(n: { name: string }): string {
  const e = ext(n.name);
  if (LANG[e]) return LANG[e]!;
  if (LANG_KEY[e]) return t(`viewer.lang.${LANG_KEY[e]}`);
  return n.name === 'go' ? 'Shell' : t('viewer.lang.text');
}

export function fmtDur(s: number): string {
  s = Math.max(0, Math.round(s));
  const h = Math.floor(s / 3600), m = Math.floor((s % 3600) / 60), x = String(s % 60).padStart(2, '0');
  return h ? `${h}:${String(m).padStart(2, '0')}:${x}` : `${m}:${x}`;
}

// "1 Zeile", "12 Zeilen": the key names a pair of texts under viewer.n.
export const plural = (n: number, key: string) => t(`viewer.n.${key}.${n === 1 ? 'one' : 'many'}`, { n: n.toLocaleString(locale()) });

// The same seed always gives the same numbers, so made-up decoration stays put between renders.
export function hash(s: string): number {
  let h = 2166136261;
  for (const c of String(s)) h = Math.imul(h ^ c.codePointAt(0)!, 16777619);
  return h >>> 0;
}
export function rng(s: string): () => number {
  let a = hash(s) || 1;
  return () => {
    a = (a + 0x6d2b79f5) | 0;
    let x = Math.imul(a ^ (a >>> 15), 1 | a);
    x = (x + Math.imul(x ^ (x >>> 7), 61 | x)) ^ x;
    return ((x ^ (x >>> 14)) >>> 0) / 4294967296;
  };
}

// A film in a folder like "Sintel (2010)" takes the folder's name, which carries the year.
export const filmOf = (name: string, folder: string) => {
  const f = folder.split('/').filter(Boolean).at(-1) || '';
  return /\(\d{4}\)/.test(f) ? f : name;
};
export function filmTitle(film: string) {
  const base = film.replace(/\.(mkv|mp4|webm|mov|jpg)$/i, '').replace(/\./g, ' ');
  const m = base.match(/^(.*?)\s*\(?((?:19|20)\d{2})\)?/);
  return m ? { name: m[1]!, year: m[2]! } : { name: base, year: '' };
}

export const joinPath = (folder: string, name: string) => (folder === '/' ? '' : folder) + '/' + name;
