// Glyphs by name, and which glyph a file gets: the format list decides, the kind is the fallback.
import coreGlyphs from '@glimstone/core/glyphs.json';
import { setGlyphResolver } from '@glimstone/vue';
import { type FunctionalComponent, h } from 'vue';
import formats from './formats.json';

type Glyph = { viewBox: string; svg: string };
// Panels and columns, which GlimStone has no glyphs for, drawn in its manner: window frames on the 14 unit grid, with
// the top edge as wide as a title bar.
const DRAWN: Record<string, Glyph> = {
  IconPaneOne: { viewBox: '0 0 14 14', svg: '<path fill-rule="evenodd" clip-rule="evenodd" d="M2 1h10a2 2 0 0 1 2 2v8a2 2 0 0 1-2 2H2a2 2 0 0 1-2-2V3a2 2 0 0 1 2-2Zm.25 3.5a.5.5 0 0 0-.5.5v5.75a.5.5 0 0 0 .5.5h9.5a.5.5 0 0 0 .5-.5V5a.5.5 0 0 0-.5-.5Z"/>' },
  IconPaneTwo: { viewBox: '0 0 14 14', svg: '<path fill-rule="evenodd" clip-rule="evenodd" d="M2 1h2.5a2 2 0 0 1 2 2v8a2 2 0 0 1-2 2H2a2 2 0 0 1-2-2V3a2 2 0 0 1 2-2Zm.25 3.5a.5.5 0 0 0-.5.5v5.75a.5.5 0 0 0 .5.5h2a.5.5 0 0 0 .5-.5V5a.5.5 0 0 0-.5-.5ZM9.5 1H12a2 2 0 0 1 2 2v8a2 2 0 0 1-2 2H9.5a2 2 0 0 1-2-2V3a2 2 0 0 1 2-2Zm.25 3.5a.5.5 0 0 0-.5.5v5.75a.5.5 0 0 0 .5.5h2a.5.5 0 0 0 .5-.5V5a.5.5 0 0 0-.5-.5Z"/>' },
  IconPaneLeft: { viewBox: '0 0 14 14', svg: '<path d="M2 1h2.5a2 2 0 0 1 2 2v8a2 2 0 0 1-2 2H2a2 2 0 0 1-2-2V3a2 2 0 0 1 2-2Z"/><path fill-rule="evenodd" clip-rule="evenodd" d="M9.5 1H12a2 2 0 0 1 2 2v8a2 2 0 0 1-2 2H9.5a2 2 0 0 1-2-2V3a2 2 0 0 1 2-2Zm.25 3.5a.5.5 0 0 0-.5.5v5.75a.5.5 0 0 0 .5.5h2a.5.5 0 0 0 .5-.5V5a.5.5 0 0 0-.5-.5Z"/>' },
  IconPaneRight: { viewBox: '0 0 14 14', svg: '<path fill-rule="evenodd" clip-rule="evenodd" d="M2 1h2.5a2 2 0 0 1 2 2v8a2 2 0 0 1-2 2H2a2 2 0 0 1-2-2V3a2 2 0 0 1 2-2Zm.25 3.5a.5.5 0 0 0-.5.5v5.75a.5.5 0 0 0 .5.5h2a.5.5 0 0 0 .5-.5V5a.5.5 0 0 0-.5-.5Z"/><path d="M9.5 1H12a2 2 0 0 1 2 2v8a2 2 0 0 1-2 2H9.5a2 2 0 0 1-2-2V3a2 2 0 0 1 2-2Z"/>' },
  IconPaneSend: { viewBox: '0 0 14 14', svg: '<path fill-rule="evenodd" clip-rule="evenodd" d="M2 1h2.5a2 2 0 0 1 2 2v8a2 2 0 0 1-2 2H2a2 2 0 0 1-2-2V3a2 2 0 0 1 2-2Zm.25 3.5a.5.5 0 0 0-.5.5v5.75a.5.5 0 0 0 .5.5h2a.5.5 0 0 0 .5-.5V5a.5.5 0 0 0-.5-.5ZM9.5 1H12a2 2 0 0 1 2 2v8a2 2 0 0 1-2 2H9.5a2 2 0 0 1-2-2V3a2 2 0 0 1 2-2Zm.25 3.5a.5.5 0 0 0-.5.5v5.75a.5.5 0 0 0 .5.5h2a.5.5 0 0 0 .5-.5V5a.5.5 0 0 0-.5-.5Z"/><path d="M10.1 6.1l2 1.65-2 1.65Z"/>' },
  IconPaneSame: { viewBox: '0 0 14 14', svg: '<path fill-rule="evenodd" clip-rule="evenodd" d="M2 1h2.5a2 2 0 0 1 2 2v8a2 2 0 0 1-2 2H2a2 2 0 0 1-2-2V3a2 2 0 0 1 2-2Zm.25 3.5a.5.5 0 0 0-.5.5v5.75a.5.5 0 0 0 .5.5h2a.5.5 0 0 0 .5-.5V5a.5.5 0 0 0-.5-.5ZM9.5 1H12a2 2 0 0 1 2 2v8a2 2 0 0 1-2 2H9.5a2 2 0 0 1-2-2V3a2 2 0 0 1 2-2Zm.25 3.5a.5.5 0 0 0-.5.5v5.75a.5.5 0 0 0 .5.5h2a.5.5 0 0 0 .5-.5V5a.5.5 0 0 0-.5-.5Z"/><path d="M2.5 6.6h1.5v1.1H2.5ZM2.5 8.4h1.5v1.1H2.5ZM10 6.6h1.5v1.1H10ZM10 8.4h1.5v1.1H10Z"/>' },
  IconColumns: { viewBox: '0 0 14 14', svg: '<path fill-rule="evenodd" clip-rule="evenodd" d="M2 1h10a2 2 0 0 1 2 2v8a2 2 0 0 1-2 2H2a2 2 0 0 1-2-2V3a2 2 0 0 1 2-2Zm-.25 2.25v7.5c0 .28.22.5.5.5h2.4V2.75h-2.4a.5.5 0 0 0-.5.5Zm4.15-.5v8.5h2.2V2.75Zm3.45 0v8.5h2.4a.5.5 0 0 0 .5-.5v-7.5a.5.5 0 0 0-.5-.5Z"/>' },
  // The places of the desktop app: the home folder, the folders used last and a drive to eject.
  IconHome: { viewBox: '0 0 14 14', svg: '<path d="M6.35.6a1 1 0 0 1 1.3 0l6 5.1a.75.75 0 0 1-.48 1.32H12.5V13a1 1 0 0 1-1 1H9V9.75a.75.75 0 0 0-.75-.75h-2.5a.75.75 0 0 0-.75.75V14H2.5a1 1 0 0 1-1-1V7.02H.83A.75.75 0 0 1 .35 5.7Z"/>' },
  IconClock: { viewBox: '0 0 14 14', svg: '<path fill-rule="evenodd" clip-rule="evenodd" d="M7 0a7 7 0 1 1 0 14A7 7 0 0 1 7 0Zm-.75 3.25v4.06l2.72 2.72 1.06-1.06-2.28-2.28V3.25Z"/>' },
  IconEject: { viewBox: '0 0 14 14', svg: '<path d="M6.2 1.9a1 1 0 0 1 1.6 0l4.7 6A1 1 0 0 1 11.7 9.5H2.3a1 1 0 0 1-.8-1.6Z"/><rect x="1.5" y="11" width="11" height="2" rx="1"/>' },
};
const GLYPHS: Record<string, Glyph> = {
  ...Object.fromEntries(coreGlyphs.map((g) => [g.name, { viewBox: g.viewBox, svg: g.svg.replace(/\s+/g, ' ') }])),
  ...DRAWN,
};

export const hasGlyph = (name: string) => name in GLYPHS;
export const glyph = (name: string): Glyph => GLYPHS[name] ?? GLYPHS.IconFile!;

// Some glyphs leave their paths unfilled and rely on the svg's own fill, as GlimStone's components set it.
// Glyphs that point along the line of text, which a right to left page mirrors.
const ALONG = new Set(['IconBack', 'IconForward', 'IconMove']);
export const G: FunctionalComponent<{ n: string }> = ({ n }) => {
  const d = glyph(n);
  return h('svg', { viewBox: d.viewBox, fill: 'currentColor', 'aria-hidden': 'true', class: ALONG.has(n) ? 'glyph-flip' : undefined, innerHTML: d.svg });
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
// A picture in a glyph's place, such as another program's icon. GlimStone's menu row takes any component there.
const pictures = new Map<string, FunctionalComponent>();
export function pictureComponent(src: string): FunctionalComponent {
  if (!pictures.has(src)) pictures.set(src, () => h('img', { src, alt: '', class: 'glyph-picture', draggable: false }));
  return pictures.get(src)!;
}
// Button and Window pick a glyph by a key, which in this app is the glyph's own name.
setGlyphResolver((key) => (hasGlyph(key) ? glyphComponent(key) : undefined));
