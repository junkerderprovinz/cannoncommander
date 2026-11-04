// The treemap of the disk usage window and the small sums its rows show.

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
