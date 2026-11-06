// The list's columns. Which ones show and in what order is a pref. The general ones come from fs/list; the others
// need the server to read inside the files and arrive through fs/meta for the rows on screen.
import { fmtCount, fmtListDate, fmtSize } from '@/core/format';
import { ext } from '@/core/glyphs';
import { prefs, setPref } from '@/core/prefs';
import { byName, t } from '@/i18n';
import { shell } from '@/shell/state';
import type { Comparer, Entry, MetaValue } from './entries';
import { decimals, metaWord } from './metaText';
import { split } from './paths';

// How a value from fs/meta is written: a byte count, a time in milliseconds, seconds of play, a figure with or
// without thousands separators, a yes or no, a code for a word, text with a decimal point, or text as it comes.
export type Fmt = 'size' | 'date' | 'duration' | 'int' | 'num' | 'kbps' | 'bool' | 'word' | 'decimal' | 'text';

export interface Column {
  id: string;
  group: string;
  width: string;
  min?: number;
  r?: boolean;
  mono?: boolean;
  fixed?: boolean;
  sort?: string;
  // Read through fs/meta rather than from the listing.
  meta?: Fmt;
  num?: (k: Entry) => number;
  value: (k: Entry, path: string) => string;
}

export const COLUMN_GROUPS: [id: string, glyph: string][] = [
  ['general', 'IconFile'], ['storage', 'IconDatabase'], ['rights', 'IconKey'], ['media', 'IconFileImage'], ['video', 'IconFileVideo'],
  ['music', 'IconFileAudio'], ['photos', 'IconFileRaw'], ['documents', 'IconFileDoc'], ['archives', 'IconArchive'], ['images', 'IconDisc'],
  ['programs', 'IconFileProgram'], ['checksum', 'IconHash'],
];

const RIGHTS = (mode = '0664') => [...mode.slice(-3)].map((d) => ['---', '--x', '-w-', '-wx', 'r--', 'r-x', 'rw-', 'rwx'][+d]).join('');
const MIME: Record<string, string> = {
  jpg: 'image/jpeg', jpeg: 'image/jpeg', png: 'image/png', webp: 'image/webp', heic: 'image/heic', gif: 'image/gif',
  mkv: 'video/x-matroska', mp4: 'video/mp4', webm: 'video/webm', mov: 'video/quicktime',
  mp3: 'audio/mpeg', flac: 'audio/flac', ogg: 'audio/ogg', wav: 'audio/wav',
  zip: 'application/zip', '7z': 'application/x-7z-compressed', rar: 'application/vnd.rar', tar: 'application/x-tar', gz: 'application/gzip', zst: 'application/zstd',
  iso: 'application/x-iso9660-image', pdf: 'application/pdf', odt: 'application/vnd.oasis.opendocument.text',
  docx: 'application/vnd.openxmlformats-officedocument.wordprocessingml.document', xlsx: 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet',
  md: 'text/markdown', txt: 'text/plain', log: 'text/plain', json: 'application/json', xml: 'application/xml', yml: 'application/yaml', yaml: 'application/yaml',
  sh: 'application/x-sh', js: 'text/javascript', ts: 'text/typescript', go: 'text/x-go',
};
const KIND_KEY: Record<string, string> = { img: 'image', video: 'video', audio: 'music', arch: 'archive', disk: 'disk', pdf: 'document', office: 'document', text: 'text', code: 'code' };

export const mmss = (s: number) => `${Math.floor(s / 3600) ? Math.floor(s / 3600) + ':' : ''}${String(Math.floor(s / 60) % 60).padStart(Math.floor(s / 3600) ? 2 : 1, '0')}:${String(s % 60).padStart(2, '0')}`;

export function fmtMeta(v: MetaValue | undefined, fmt: Fmt, id = ''): string {
  if (v == null || v === '') return '';
  if (typeof v === 'boolean') return t(v ? 'files.yes' : 'files.no');
  if (typeof v === 'string') return fmt === 'word' ? metaWord(id, v) : fmt === 'decimal' ? decimals(v) : v;
  switch (fmt) {
    case 'size': return fmtSize(v);
    case 'date': return fmtListDate(v);
    case 'duration': return mmss(v);
    case 'int': return fmtCount(v);
    case 'kbps': return `${fmtCount(v)} kBit/s`;
    default: return String(v);
  }
}

const meta = (k: Entry, id: string) => k.meta?.[id];
const metaNum = (id: string) => (k: Entry) => { const v = meta(k, id); return typeof v === 'number' ? v : -1; };
const blocks = (k: Entry) => Math.ceil((k.size || 0) / 512);
const diskSize = (k: Entry) => Math.ceil((k.size || 0) / 4096) * 4096;

type Def = Omit<Column, 'group' | 'value'> & { value?: Column['value'] };
const general: Def[] = [
  { id: 'name', width: 'minmax(0, 1fr)', min: 14, sort: 'name', fixed: true, value: (k) => k.name },
  { id: 'size', width: '6.5rem', r: true, sort: 'size', value: (k) => (k.dir ? (prefs.folderSizes ? (k.sizeUnknown ? '' : fmtSize(k.size) || '0 B') : t('files.objShort', { n: k.items ?? 0 })) : fmtSize(k.size)) },
  { id: 'modified', width: '7.5rem', r: true, sort: 'time', value: (k) => fmtListDate(k.time) },
  { id: 'created', width: '7.5rem', r: true, num: (k) => k.btime ?? -1, value: (k) => (k.btime == null ? '' : fmtListDate(k.btime)) },
  { id: 'accessed', width: '7.5rem', r: true, meta: 'date', num: metaNum('accessed') },
  { id: 'type', width: '8.5rem', sort: 'type', value: (k) => (k.dir ? t('kinds.folder') : k.link ? t('kinds.link') : ext(k.name) ? t(`types.${k.kind}`) : t('types.file')) },
  { id: 'ext', width: '5rem', value: (k) => (k.dir ? '' : ext(k.name)) },
  { id: 'items', width: '5.5rem', r: true, num: (k) => (k.dir ? k.items ?? 0 : -1), value: (k) => (k.dir ? String(k.items ?? '') : '') },
  { id: 'path', width: '11rem', value: (_k, path) => path },
  { id: 'link', width: '11rem', value: (k) => k.link || '' },
  { id: 'disksize', width: '7rem', r: true, num: (k) => (k.dir ? -1 : diskSize(k)), value: (k) => (k.dir ? '' : fmtSize(diskSize(k)) || '0 B') },
  { id: 'kind', width: '6.5rem', value: (k) => t(`groups.${k.dir ? 'folder' : KIND_KEY[k.kind] || 'other'}`) },
  { id: 'mime', width: '10rem', mono: true, value: (k) => (k.dir ? 'inode/directory' : MIME[ext(k.name)] || 'application/octet-stream') },
  { id: 'attributes', width: '5rem', mono: true, value: (k) => [k.hidden || k.name.startsWith('.') ? 'H' : '', k.link ? 'L' : '', +(k.mode || '0664').slice(-3)[0]! < 6 ? 'R' : ''].join('') },
  { id: 'statchange', width: '7.5rem', r: true, meta: 'date', num: metaNum('statchange') },
];
const storage: Def[] = [
  { id: 'disk', width: '6rem', meta: 'text' },
  { id: 'share', width: '7rem', value: (k, path) => (path.startsWith('/mnt/user/') ? split(path)[2]! : path === '/mnt/user' && k.dir ? k.name : '') },
  { id: 'hardlinks', width: '5.5rem', r: true, num: (k) => k.nlink ?? -1, value: (k) => (k.nlink == null ? '' : String(k.nlink)) },
  { id: 'inode', width: '6.5rem', r: true, mono: true, num: (k) => k.ino ?? -1, value: (k) => (k.ino == null ? '' : String(k.ino)) },
  { id: 'blocks', width: '5.5rem', r: true, num: (k) => (k.dir ? -1 : blocks(k)), value: (k) => (k.dir ? '' : String(blocks(k))) },
];
const rights: Def[] = [
  { id: 'owner', width: '6rem', value: (k) => k.owner || 'nobody' },
  { id: 'group', width: '6rem', value: (k) => k.group || 'users' },
  { id: 'rights', width: '7rem', mono: true, value: (k) => (k.dir ? 'd' : '-') + RIGHTS(k.mode) },
  { id: 'octal', width: '4.5rem', mono: true, value: (k) => (k.mode || '0664').slice(-3) },
  { id: 'uid', width: '4rem', r: true, mono: true, value: (k) => (k.uid == null ? '' : String(k.uid)) },
  { id: 'gid', width: '4rem', r: true, mono: true, value: (k) => (k.gid == null ? '' : String(k.gid)) },
  { id: 'special', width: '6.5rem', value: (k) => specialOf(k.mode) },
];
const media: Def[] = [
  { id: 'dimensions', width: '7.5rem', r: true, meta: 'text' },
  { id: 'duration', width: '5.5rem', r: true, meta: 'duration', num: metaNum('duration') },
];
const video: Def[] = [
  { id: 'vcodec', width: '6rem', meta: 'text' }, { id: 'acodec', width: '6rem', meta: 'text' },
  { id: 'framerate', width: '5.5rem', r: true, meta: 'decimal' }, { id: 'vbitrate', width: '6.5rem', r: true, meta: 'kbps', num: metaNum('vbitrate') },
  { id: 'aspect', width: '6rem', r: true, meta: 'decimal' }, { id: 'hdr', width: '5.5rem', meta: 'text' },
  { id: 'audiotracks', width: '8rem', meta: 'word' }, { id: 'subtitles', width: '7rem', meta: 'word' },
  { id: 'season', width: '4.5rem', r: true, meta: 'num', num: metaNum('season') }, { id: 'episode', width: '4.5rem', r: true, meta: 'num', num: metaNum('episode') },
];
const music: Def[] = [
  { id: 'artist', width: '8rem', meta: 'text' }, { id: 'album', width: '8rem', meta: 'text' }, { id: 'title', width: '9rem', meta: 'text' },
  { id: 'albumartist', width: '8rem', meta: 'text' }, { id: 'track', width: '3.5rem', r: true, meta: 'num', num: metaNum('track') },
  { id: 'year', width: '4rem', r: true, meta: 'num' }, { id: 'genre', width: '6.5rem', meta: 'text' }, { id: 'composer', width: '8rem', meta: 'text' },
  { id: 'abitrate', width: '6rem', r: true, meta: 'text' }, { id: 'samplerate', width: '6rem', r: true, meta: 'decimal' }, { id: 'channels', width: '5rem', meta: 'word' },
];
const photos: Def[] = [
  { id: 'camera', width: '8rem', meta: 'text' }, { id: 'taken', width: '7.5rem', r: true, meta: 'date', num: metaNum('taken') },
  { id: 'make', width: '7rem', meta: 'text' }, { id: 'lens', width: '9rem', meta: 'text' }, { id: 'focal', width: '5.5rem', r: true, meta: 'decimal' },
  { id: 'aperture', width: '4.5rem', r: true, meta: 'decimal' }, { id: 'exposure', width: '6rem', r: true, meta: 'decimal' },
  { id: 'iso', width: '4.5rem', r: true, meta: 'num', num: metaNum('iso') }, { id: 'flash', width: '6rem', meta: 'word' }, { id: 'place', width: '8rem', meta: 'text' },
  { id: 'orientation', width: '6rem', meta: 'word' }, { id: 'depth', width: '5rem', r: true, meta: 'text' }, { id: 'dpi', width: '5.5rem', r: true, meta: 'text' },
];
const documents: Def[] = [
  { id: 'pages', width: '4.5rem', r: true, meta: 'num', num: metaNum('pages') },
  { id: 'author', width: '7rem', meta: 'text' }, { id: 'lastauthor', width: '8rem', meta: 'text' }, { id: 'doctitle', width: '9rem', meta: 'text' },
  { id: 'subject', width: '8rem', meta: 'text' }, { id: 'words', width: '5rem', r: true, meta: 'int', num: metaNum('words') },
  { id: 'lines', width: '5rem', r: true, meta: 'int', num: metaNum('lines') }, { id: 'encoding', width: '6.5rem', meta: 'text' },
  { id: 'app', width: '8rem', meta: 'text' }, { id: 'encrypted', width: '5.5rem', meta: 'bool' },
];
const archives: Def[] = [
  { id: 'unpacked', width: '7rem', r: true, meta: 'size', num: metaNum('unpacked') }, { id: 'ratio', width: '6rem', r: true, meta: 'text' },
  { id: 'entries', width: '6rem', r: true, meta: 'num', num: metaNum('entries') }, { id: 'method', width: '6.5rem', meta: 'word' },
  { id: 'archcrypt', width: '6rem', meta: 'bool' },
];
const images: Def[] = [{ id: 'fs', width: '6rem', meta: 'text' }, { id: 'volume', width: '9rem', mono: true, meta: 'text' }, { id: 'bootable', width: '5.5rem', meta: 'bool' }];
const programs: Def[] = [
  { id: 'version', width: '5.5rem', r: true, meta: 'text' }, { id: 'vendor', width: '8rem', meta: 'text' },
  { id: 'arch', width: '6rem', meta: 'word' }, { id: 'signed', width: '5rem', meta: 'bool' },
];
const checksum: Def[] = [
  { id: 'sha256', width: '9rem', mono: true, meta: 'text' }, { id: 'crc32', width: '6rem', mono: true, meta: 'text' },
  { id: 'md5', width: '9rem', mono: true, meta: 'text' }, { id: 'sha1', width: '9rem', mono: true, meta: 'text' },
];

// The sticky, setgid and setuid bits stand in the fourth digit from the end of the mode.
function specialOf(mode = '0664') {
  const bits = mode.length > 3 ? +mode.slice(-4, -3) : 0;
  return [bits & 1 ? 'Sticky' : '', bits & 2 ? 'SGID' : '', bits & 4 ? 'SUID' : ''].filter(Boolean).join(', ');
}

const byGroup: Record<string, Def[]> = { general, storage, rights, media, video, music, photos, documents, archives, images, programs, checksum };
export const COLUMNS: Column[] = Object.entries(byGroup).flatMap(([group, defs]) => defs.map((d) => ({
  ...d,
  group,
  value: d.value ?? ((k: Entry) => fmtMeta(meta(k, d.id), d.meta!, d.id)),
})));
export const COL: Record<string, Column> = Object.fromEntries(COLUMNS.map((c) => [c.id, c]));
export const DEFAULT_COLUMNS = ['name', 'size', 'modified'];
export const columnLabel = (id: string) => t(`columns.${id}`);

const narrow = () => shell.width <= 760;

export const Columns = {
  shown: () => ((prefs.columns as string[]) || DEFAULT_COLUMNS).filter((id) => COL[id]),
  // A phone has room for the name and the size; the chosen columns stay as they are for wider screens.
  visible: () => (narrow() ? Columns.shown().filter((id) => id === 'name' || id === 'size') : Columns.shown()),
  width: (id: string) => (prefs.colWidths?.[id] ? `${prefs.colWidths[id]}rem` : COL[id]!.width),
  template: () => Columns.visible().map(Columns.width).join(' '),
  // Every row is its own grid, so they line up only when all share one width; below it the list scrolls sideways.
  minWidth: () => {
    if (narrow()) return '0';
    const ids = Columns.visible();
    return `${ids.reduce((a, id) => a + (COL[id]!.min || parseFloat(Columns.width(id))), 0) + (ids.length - 1) + 1}rem`;
  },
  sortKey: (c: Column) => c.sort || c.id,
  set: (ids: string[]) => setPref('columns', ids.includes('name') ? ids : ['name', ...ids]),
  reset() {
    prefs.colWidths = {};
    Columns.set(DEFAULT_COLUMNS);
  },
  toggle(id: string) {
    const cur = Columns.shown();
    if (COL[id]!.fixed) return;
    Columns.set(cur.includes(id) ? cur.filter((x) => x !== id) : [...cur, id]);
  },
  // Sorting by any column: the four classic keys keep their own comparers, the rest compare numbers or text.
  comparer(sort: string): Comparer | null {
    const c = COLUMNS.find((x) => x.id === sort || x.sort === sort);
    if (!c || ['name', 'size', 'time', 'type'].includes(sort)) return null;
    if (c.num) return (a, b) => c.num!(a) - c.num!(b);
    return (a, b) => byName(String(c.value(a, '')), String(c.value(b, '')));
  },
  // The fs/meta columns the panel has to ask for: the visible ones and the one it sorts by.
  metaWanted: (sort: string, extra: string[] = []) => [...new Set([...Columns.visible(), sort, ...extra])].filter((id) => COL[id]?.meta),
};
