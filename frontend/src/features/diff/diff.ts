// The parts of a file comparison the window works out itself: where each changed place starts, and which middle part
// of a changed line differs.
import type { DiffRow } from '@/api/tools';

export interface Piece { text: string; mark?: boolean }

// A place is a run of rows that are not equal; its first row is where a jump lands.
export const hunksOf = (rows: DiffRow[]) => rows.flatMap((r, i) => (r.t !== 'eq' && (i === 0 || rows[i - 1]!.t === 'eq') ? [i] : []));

export const countOf = (rows: DiffRow[], t: DiffRow['t']) => rows.filter((r) => r.t === t).length;

// The common start and end of two lines stay plain; what lies between them is marked on both sides.
export function inline(a: string, b: string): [Piece[], Piece[]] {
  let s = 0;
  while (s < a.length && s < b.length && a[s] === b[s]) s++;
  let e = 0;
  while (e < a.length - s && e < b.length - s && a[a.length - 1 - e] === b[b.length - 1 - e]) e++;
  const cut = (x: string): Piece[] => [
    { text: x.slice(0, s) },
    ...(x.length - e > s ? [{ text: x.slice(s, x.length - e), mark: true }] : []),
    { text: x.slice(x.length - e) },
  ].filter((p) => p.text);
  return [cut(a), cut(b)];
}
