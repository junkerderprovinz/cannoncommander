// The list's columns. Which ones show and in what order is a pref. These are the ones plain fs/list data can fill;
// the metadata columns need the server to read the files.
import { fmtListDate, fmtSize } from '@/core/format';
import { ext } from '@/core/glyphs';
import { prefs } from '@/core/prefs';
import { t } from '@/i18n';
import { shell } from '@/shell/state';
import type { Comparer, Entry } from './entries';

export interface Column {
  id: string;
  label: () => string;
  group: string;
  width: string;
  min?: number;
  r?: boolean;
  mono?: boolean;
  fixed?: boolean;
  sort?: string;
  num?: (k: Entry) => number;
  value: (k: Entry, path: string) => string;
}

const RIGHTS = (mode = '0664') => [...mode.slice(-3)].map((d) => ['---', '--x', '-w-', '-wx', 'r--', 'r-x', 'rw-', 'rwx'][+d]).join('');
const label = (id: string) => () => t(`columns.${id}`);

export const COLUMNS: Column[] = [
  { id: 'name', label: label('name'), group: 'general', width: 'minmax(0, 1fr)', min: 14, sort: 'name', fixed: true, value: (k) => k.name },
  { id: 'size', label: label('size'), group: 'general', width: '6.5rem', r: true, sort: 'size',
    value: (k) => (k.dir ? (prefs.folderSizes ? fmtSize(k.size) || '0 B' : t('files.objShort', { n: k.items ?? 0 })) : fmtSize(k.size)) },
  { id: 'modified', label: label('modified'), group: 'general', width: '7.5rem', r: true, sort: 'time', value: (k) => fmtListDate(k.time) },
  { id: 'type', label: label('type'), group: 'general', width: '8.5rem', sort: 'type',
    value: (k) => (k.dir ? t('kinds.folder') : k.link ? t('kinds.link') : ext(k.name) ? t(`types.${k.kind}`) : t('types.file')) },
  { id: 'ext', label: label('ext'), group: 'general', width: '5rem', value: (k) => (k.dir ? '' : ext(k.name)) },
  { id: 'items', label: label('items'), group: 'general', width: '5.5rem', r: true, num: (k) => (k.dir ? k.items ?? 0 : -1), value: (k) => (k.dir ? String(k.items ?? '') : '') },
  { id: 'path', label: label('path'), group: 'general', width: '11rem', value: (_k, path) => path },
  { id: 'link', label: label('link'), group: 'general', width: '11rem', value: (k) => k.link || '' },
  { id: 'owner', label: label('owner'), group: 'rights', width: '6rem', value: (k) => k.owner || 'nobody' },
  { id: 'group', label: label('group'), group: 'rights', width: '6rem', value: (k) => k.group || 'users' },
  { id: 'rights', label: label('rights'), group: 'rights', width: '7rem', mono: true, value: (k) => (k.dir ? 'd' : '-') + RIGHTS(k.mode) },
  { id: 'octal', label: label('octal'), group: 'rights', width: '4.5rem', mono: true, value: (k) => (k.mode || '0664').slice(-3) },
];
export const COL: Record<string, Column> = Object.fromEntries(COLUMNS.map((c) => [c.id, c]));
export const DEFAULT_COLUMNS = ['name', 'size', 'modified'];

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
  // Sorting by any column: the four classic keys keep their own comparers, the rest compare numbers or text.
  comparer(sort: string): Comparer | null {
    const c = COLUMNS.find((x) => x.id === sort || x.sort === sort);
    if (!c || ['name', 'size', 'time', 'type'].includes(sort)) return null;
    if (c.num) return (a, b) => c.num!(a) - c.num!(b);
    return (a, b) => String(c.value(a, '')).localeCompare(String(c.value(b, '')), 'de', { numeric: true });
  },
};
