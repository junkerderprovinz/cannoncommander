// Glyphs by name, and which glyph a file gets: the format list decides, the kind is the fallback.
import coreGlyphs from '@glimstone/core/glyphs.json';
import { setGlyphResolver } from '@glimstone/vue';
import { type FunctionalComponent, h } from 'vue';
import formats from './formats.json';

type Glyph = { viewBox: string; svg: string };
const GLYPHS: Record<string, Glyph> = Object.fromEntries(
  coreGlyphs.map((g) => [g.name, { viewBox: g.viewBox, svg: g.svg.replace(/\s+/g, ' ') }]),
);

export const hasGlyph = (name: string) => name in GLYPHS;
export const glyph = (name: string): Glyph => GLYPHS[name] ?? GLYPHS.IconFile!;

// Some glyphs leave their paths unfilled and rely on the svg's own fill, as GlimStone's components set it.
export const G: FunctionalComponent<{ n: string }> = ({ n }) => {
  const d = glyph(n);
  return h('svg', { viewBox: d.viewBox, fill: 'currentColor', 'aria-hidden': 'true', innerHTML: d.svg });
};
G.props = ['n'];

export interface Format { id: string; name: string; glyph: string; ext: string[]; drawn?: boolean }
export const FORMATS = formats as Format[];
const FORMAT_GLYPH: Record<string, string> = Object.fromEntries(FORMATS.flatMap((f) => f.ext.map((e) => [e, f.glyph])));
export const FORMAT_ID: Record<string, string> = Object.fromEntries(FORMATS.flatMap((f) => f.ext.map((e) => [e, f.id])));

export type Kind = 'dir' | 'img' | 'video' | 'audio' | 'arch' | 'disk' | 'pdf' | 'office' | 'text' | 'code' | 'file';

export const ext = (name: string) => (name.includes('.') ? name.split('.').pop()!.toLowerCase() : '');

export function kindOf(n: { name: string; dir?: boolean }): Kind {
  if (n.dir) return 'dir';
  const e = ext(n.name);
  if (['jpg', 'jpeg', 'png', 'webp', 'heic', 'gif'].includes(e)) return 'img';
  if (['mkv', 'mp4', 'webm', 'mov'].includes(e)) return 'video';
  if (['mp3', 'flac', 'ogg', 'wav'].includes(e)) return 'audio';
  if (['zip', 'zst', '7z', 'rar', 'tar', 'gz'].includes(e)) return 'arch';
  if (['iso', 'img'].includes(e)) return 'disk';
  if (e === 'pdf') return 'pdf';
  if (['docx', 'xlsx', 'pptx', 'odt'].includes(e)) return 'office';
  if (['md', 'txt', 'srt', 'xml', 'json', 'cfg', 'log', 'sha256', 'conf', '1'].includes(e)) return 'text';
  if (['yml', 'yaml', 'go', 'sh', 'js', 'ts'].includes(e)) return 'code';
  return 'file';
}

const KIND_GLYPH: Record<Kind, string> = {
  dir: 'IconFolder', img: 'IconFileImage', video: 'IconFileVideo', audio: 'IconFileAudio', arch: 'IconArchive', disk: 'IconDatabase',
  pdf: 'IconFileText', office: 'IconFileText', text: 'IconFileText', code: 'IconCode', file: 'IconFile',
};

export function glyphOf(n: { name: string; dir?: boolean }): string {
  if (n.dir) return 'IconFolder';
  const e = /\.log\.\d+$/.test(n.name) ? 'log' : n.name === 'go' ? 'cfg' : ext(n.name);
  return FORMAT_GLYPH[e] || KIND_GLYPH[kindOf(n)];
}

// GlimStone's components take a glyph as a component; one per name is enough for every menu row that shows it.
const components = new Map<string, FunctionalComponent>();
export function glyphComponent(name: string): FunctionalComponent {
  if (!components.has(name)) components.set(name, () => h(G, { n: name }));
  return components.get(name)!;
}
// Button and Window pick a glyph by a key, which in this app is the glyph's own name.
setGlyphResolver((key) => (hasGlyph(key) ? glyphComponent(key) : undefined));
