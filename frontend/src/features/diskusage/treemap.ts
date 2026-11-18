// The treemap of the disk usage window and the small sums its rows show.
import type { UsageEntry, UsageInner } from '@/api/tools';

export interface Box<T> { it: T; x: number; y: number; w: number; h: number }

// Bruls, Huizing and van Wijk: fill rows along the short side while the worst aspect ratio keeps improving. The items
// come largest first.
export function squarify<T extends { v: number }>(items: T[], x: number, y: number, w: number, h: number): Box<T>[] {
  const out: Box<T>[] = [];
  const total = items.reduce((a, it) => a + it.v, 0);
  if (!total || w <= 0 || h <= 0) return out;
  const scale = (w * h) / total;
  const rest = items.map((it) => ({ it, a: it.v * scale }));
  const worst = (row: { a: number }[], side: number) => {
    const s = row.reduce((a, r) => a + r.a, 0), mx = Math.max(...row.map((r) => r.a)), mn = Math.min(...row.map((r) => r.a));
    return Math.max((side * side * mx) / (s * s), (s * s) / (side * side * mn));
  };
  while (rest.length) {
    const side = Math.min(w, h);
    const row = [rest.shift()!];
    while (rest.length && worst([...row, rest[0]!], side) <= worst(row, side)) row.push(rest.shift()!);
    const s = row.reduce((a, r) => a + r.a, 0);
    if (w >= h) {
      const rw = s / h;
      let yy = y;
      for (const r of row) { const rh = r.a / rw; out.push({ it: r.it, x, y: yy, w: rw, h: rh }); yy += rh; }
      x += rw; w -= rw;
    } else {
      const rh = s / w;
      let xx = x;
      for (const r of row) { const rw = r.a / rh; out.push({ it: r.it, x: xx, y, w: rw, h: rh }); xx += rw; }
      y += rh; h -= rh;
    }
  }
  return out;
}

// A label cut to the width of its block, about 7.2 pixels a letter, or nothing where not even a few letters fit.
export const fit = (text: string, px: number) => {
  const max = Math.floor(px / 7.2);
  return text.length <= max ? text : max > 3 ? text.slice(0, max - 1) + '…' : '';
};

export const pct = (part: number, whole: number) => (whole ? (part / whole) * 100 : 0);

// A folder and something inside it may both be picked; deleting the folder already takes the inner one.
export const outermost = <T extends { path: string }>(picked: T[]) => picked.filter((r) => !picked.some((o) => o !== r && r.path.startsWith(o.path + '/')));

// A tile is an entry inside a group, or an entry of the folder that holds nothing to group, which keeps its index i
// among the items. rest counts the entries the server left out, drawn as one plain tile with their room.
export interface MapTile { e: UsageInner; group: number; i?: number; x: number; y: number; w: number; h: number; rest?: number }
export interface MapGroup { i: number; e: UsageEntry; x: number; y: number; w: number; h: number; head: boolean }

// The folder one level deep: each folder in it a group with its own largest entries, as the colour modes draw it.
export function nestedMap(items: UsageEntry[], w: number, h: number) {
  const groups: MapGroup[] = [], tiles: MapTile[] = [];
  const list = items.map((e, i) => ({ e, i, v: e.size })).filter((x) => x.v > 0);
  for (const b of squarify(list, 0, 0, w, h)) {
    const { e, i } = b.it, pad = 2, x = b.x + pad, y = b.y + pad, bw = Math.max(0, b.w - pad * 2), bh = Math.max(0, b.h - pad * 2);
    const inner = (e.items ?? []).filter((k) => k.size > 0);
    if (!e.dir || !inner.length) {
      tiles.push({ e, group: -1, i, x, y, w: bw, h: bh });
      continue;
    }
    const head = bh > 60 && bw > 70;
    const g = groups.push({ i, e, x, y, w: bw, h: bh, head }) - 1;
    const left = e.size - inner.reduce((a, k) => a + k.size, 0);
    const kids = inner.map((k) => ({ k, v: k.size, rest: undefined as number | undefined }));
    if (left > 0) kids.push({ k: { name: '', dir: false, size: left, files: 0, dirs: 0, mtime: '' }, v: left, rest: e.more || 1 });
    kids.sort((a, b) => b.v - a.v);
    const top = head ? 22 : 0;
    for (const t of squarify(kids, x + 3, y + 3 + top, bw - 6, bh - 6 - top)) {
      const p = 1.5;
      tiles.push({ e: t.it.k, group: g, x: t.x + p, y: t.y + p, w: Math.max(0, t.w - p * 2), h: Math.max(0, t.h - p * 2), ...(t.it.rest ? { rest: t.it.rest } : {}) });
    }
  }
  return { groups, tiles };
}
