// The fixture is plain JavaScript for Vite's server; the tests read the prototype through it.
import type { Connect } from 'vite';

type Node = { name: string; dir?: boolean; children?: Node[]; [key: string]: unknown };
type Column = { id: string; label: string; group: string; width: string; r?: boolean; mono?: boolean; fixed?: boolean; sort?: string; num?: (k: unknown) => number; value?: (k: unknown, path: string) => string };

export function loadPrototype(): {
  FS: {
    root: Node; FREE: Record<string, number>; TOTAL: Record<string, number>; LABEL: Record<string, string>;
    nodeAt(path: string): Node | null; kids(n: Node | null): Node[]; join(path: string, name: string): string; parent(path: string): string;
    split(path: string): string[]; kindOf(n: Node): string; ext(name: string): string; totalSize(n: Node): number;
  };
  COL: Record<string, Column>;
  hashOf: (s: string) => number;
  DAY: number;
  NOW: number;
  CC: { prefs: Record<string, unknown> };
  // Runs more prototype code in the same sandbox, where the data and the columns already live.
  run<T = unknown>(code: string): T;
};

export function createFixture(): {
  list(path: string): { path: string; items: import('../src/api/types').FsItem[] } | null;
  tree(path: string): import('../src/api/types').FsTree | null;
  meta(req: { paths: string[]; columns: string[] }): import('../src/api/types').FsMeta;
  search(path: string, query: string, hidden: boolean): import('../src/api/types').SearchResult | null;
  text(path: string, lines: number): { text: string } | null;
  thumb(path: string): string | null;
};

export function fixtureMiddleware(): Connect.NextHandleFunction;
