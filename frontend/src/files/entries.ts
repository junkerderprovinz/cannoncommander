// The pure steps from a folder's items to the rows a panel shows: hide, filter, sort.
import { ext, type Kind, kindOf } from '@/core/glyphs';
import type { Item } from '@/api/client';

export type Entry = Item & { kind: Kind; up?: boolean };

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
const byName: Comparer = (a, b) => a.name.localeCompare(b.name, 'de', { numeric: true });
export const COMPARERS: Record<string, Comparer> = {
  name: byName,
  size: (a, b) => a.size - b.size,
  time: (a, b) => a.time - b.time,
  type: (a, b) => ext(a.name).localeCompare(ext(b.name)) || a.name.localeCompare(b.name, 'de'),
};

export interface SortOptions { sort: string; dir: number; foldersFirst: boolean; comparer?: Comparer | null }
export function sortEntries(list: Entry[], { sort, dir, foldersFirst, comparer }: SortOptions) {
  const by = COMPARERS[sort] || comparer || byName;
  return list.sort((a, b) => (foldersFirst ? (b.dir ? 1 : 0) - (a.dir ? 1 : 0) : 0) || by(a, b) * dir);
}

export const toEntry = (k: Item): Entry => ({ ...k, kind: kindOf(k) });
