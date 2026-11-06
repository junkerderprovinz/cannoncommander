// The pure steps from a folder's items to the rows a panel shows: hide, filter, sort and group.
import type { Item } from '@/api/client';
import { DAY } from '@/core/format';
import { ext, type Kind, kindOf } from '@/core/glyphs';
import { byName, byText } from '@/i18n';

export type MetaValue = string | number | boolean | null;
export type Entry = Item & { kind: Kind; up?: boolean; meta?: Record<string, MetaValue> };

export const shown = (showHidden: boolean) => (k: Item) => showHidden || (!k.hidden && !k.name.startsWith('.'));

// Words in a search match anywhere in the name; a word with * or ? is a pattern for the whole name, so *.mkv finds
// every film, and a word starting with # finds a tag or a colour.
export function nameMatcher(query: string, tagMatch: (k: Item, word: string) => boolean = () => false) {
  const parts = query.trim().toLowerCase().split(/\s+/).filter(Boolean).map((p) => (/[*?]/.test(p)
    ? new RegExp('^' + p.replace(/[.+^${}()|[\]\\]/g, '\\$&').replace(/\*/g, '.*').replace(/\?/g, '.') + '$', 'i')
    : p));
  return (k: Item) => parts.some((p) => (typeof p !== 'string' ? p.test(k.name) : p.startsWith('#') ? tagMatch(k, p) : k.name.toLowerCase().includes(p)));
}

export type Comparer = (a: Entry, b: Entry) => number;
const sortName: Comparer = (a, b) => byName(a.name, b.name);
export const COMPARERS: Record<string, Comparer> = {
  name: sortName,
  size: (a, b) => a.size - b.size,
  time: (a, b) => a.time - b.time,
  type: (a, b) => byText(ext(a.name), ext(b.name)) || byText(a.name, b.name),
};

export interface SortOptions { sort: string; dir: number; foldersFirst: boolean; comparer?: Comparer | null }
export function sortEntries(list: Entry[], { sort, dir, foldersFirst, comparer }: SortOptions) {
  const by = COMPARERS[sort] || comparer || byName;
  return list.sort((a, b) => (foldersFirst ? (b.dir ? 1 : 0) - (a.dir ? 1 : 0) : 0) || by(a, b) * dir);
}

export const toEntry = (k: Item): Entry => ({ ...k, kind: kindOf(k) });

// Grouping splits the list into sections with a heading. Each group sorts by its rank and keeps the order of the sort
// within a rank, so the sections follow each other and their rows stay sorted. Names are text keys, so pdf and office
// share the section "document".
const AGE: [string, number][] = [['today', 1], ['yesterday', 2], ['week', 7], ['month', 31], ['year', 365], ['older', Infinity]];
const SIZE: [string, number][] = [['huge', 1e9], ['big', 1e8], ['medium', 1e6], ['small', 1], ['empty', 0]];
const KIND_ORDER: string[] = ['dir', 'video', 'audio', 'img', 'pdf', 'office', 'text', 'code', 'arch', 'disk', 'file'];
const KIND_GROUP: Partial<Record<Kind, string>> = { img: 'image', video: 'video', audio: 'music', arch: 'archive', disk: 'disk', pdf: 'document', office: 'document', text: 'text', code: 'code' };

export interface Group { g: string; rank: (k: Entry, now: number) => number; name: (k: Entry, now: number) => string }
export const GROUPS: Record<string, Group> = {
  kind: { g: 'IconFile', rank: (k) => KIND_ORDER.indexOf(k.dir ? 'dir' : k.kind), name: (k) => (k.dir ? 'folder' : KIND_GROUP[k.kind] || 'other') },
  date: { g: 'IconCalendar', rank: (k, now) => AGE.findIndex(([, d]) => now - k.time < d * DAY), name: (k, now) => AGE.find(([, d]) => now - k.time < d * DAY)![0] },
  size: { g: 'IconDatabase', rank: (k) => (k.dir ? -1 : SIZE.findIndex(([, s]) => (k.size || 0) >= s)), name: (k) => (k.dir ? 'folder' : SIZE.find(([, s]) => (k.size || 0) >= s)![0]) },
};

export function groupEntries(list: Entry[], group: string, now: number) {
  const grp = GROUPS[group];
  if (grp) list.sort((a, b) => grp.rank(a, now) - grp.rank(b, now));
  return list;
}

export interface GroupHead { name: string; count: number }
// The heading that goes before each row that starts a section, or nothing for the rows inside one.
export function groupHeads(list: Entry[], group: string, now: number): (GroupHead | null)[] {
  const grp = GROUPS[group];
  if (!grp) return list.map(() => null);
  const names = list.map((k) => grp.name(k, now));
  const counts = new Map<string, number>();
  names.forEach((n) => counts.set(n, (counts.get(n) ?? 0) + 1));
  return names.map((n, i) => (i > 0 && names[i - 1] === n ? null : { name: n, count: counts.get(n)! }));
}
