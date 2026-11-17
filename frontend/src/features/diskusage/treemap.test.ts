import { readFileSync } from 'node:fs';
import { join } from 'node:path';
import vm from 'node:vm';
import { describe, expect, it } from 'vitest';
import { fit, nestedMap, outermost, pct, squarify } from './treemap';

// The prototype's own squarify and fit, cut out of b-50-diskusage.js, so the treemap lays out the same blocks.
const src = readFileSync(join(__dirname, '../../../../prototype/src/js/features/b-50-diskusage.js'), 'utf8');
const proto = vm.runInNewContext(`${src.slice(src.indexOf('  function squarify'), src.indexOf('  const shown ='))}\n;({ squarify, fit })`) as {
  squarify: (items: { v: number }[], x: number, y: number, w: number, h: number) => { it: { v: number }; x: number; y: number; w: number; h: number }[];
  fit: (text: string, px: number) => string;
};

describe('treemap', () => {
  it('lays out the blocks exactly as the prototype does', () => {
    const items = [500, 300, 120, 80, 60, 30, 9, 1].map((v) => ({ v }));
    for (const [w, h] of [[1200, 640], [300, 900], [1, 1]] as const) {
      expect(squarify(items, 0, 0, w, h)).toEqual(proto.squarify(items, 0, 0, w, h));
    }
  });

  it('fills the whole area', () => {
    const boxes = squarify([5, 3, 2].map((v) => ({ v })), 0, 0, 100, 50);
    expect(boxes.reduce((a, b) => a + b.w * b.h, 0)).toBeCloseTo(5000);
  });

  it('draws nothing for nothing', () => {
    expect(squarify([{ v: 0 }], 0, 0, 100, 100)).toEqual([]);
    expect(squarify([{ v: 3 }], 0, 0, 0, 100)).toEqual([]);
  });

  it('cuts labels the way the prototype does', () => {
    for (const [text, px] of [['backups', 200], ['appdata-2026-10-24.tar.zst', 80], ['abc', 20], ['dokumente', 30]] as const) {
      expect(fit(text, px)).toBe(proto.fit(text, px));
    }
  });

  it('takes a share of nothing as none', () => {
    expect(pct(5, 0)).toBe(0);
    expect(pct(1, 4)).toBe(25);
  });

  it('deletes a folder once even when something inside it is picked too', () => {
    const picked = [{ path: '/a/b' }, { path: '/a/b/c' }, { path: '/a/bc' }];
    expect(outermost(picked).map((p) => p.path)).toEqual(['/a/b', '/a/bc']);
  });
});

describe('nested treemap', () => {
  const folder = (name: string, kids: number[], more = 0) => ({ name, dir: true, size: kids.reduce((a, v) => a + v, 0) + more, files: 0, dirs: 0, mtime: '', more: more ? 3 : 0, items: kids.map((v, i) => ({ name: `${name}${i}`, dir: false, size: v, files: 1, dirs: 0, mtime: '' })) });

  it('draws every folder as a group that holds its entries', () => {
    const { groups, tiles } = nestedMap([folder('filme', [500, 300]), folder('fotos', [100, 50])], 1000, 600);
    expect(groups.map((g) => g.e.name)).toEqual(['filme', 'fotos']);
    for (const t of tiles) {
      const g = groups[t.group]!;
      expect(t.x).toBeGreaterThanOrEqual(g.x);
      expect(t.y).toBeGreaterThanOrEqual(g.y);
      expect(t.x + t.w).toBeLessThanOrEqual(g.x + g.w + 1e-9);
      expect(t.y + t.h).toBeLessThanOrEqual(g.y + g.h + 1e-9);
    }
    expect(tiles.filter((t) => t.group === 0).map((t) => t.e.name)).toEqual(['filme0', 'filme1']);
  });

  it('keeps room for what the server left out, as one plain tile', () => {
    const { tiles } = nestedMap([folder('viele', [400, 300], 300)], 1000, 600);
    const rest = tiles.find((t) => t.rest);
    expect(rest?.rest).toBe(3);
    expect(rest!.e.size).toBe(300);
  });

  it('draws a file and an empty folder as tiles of their own', () => {
    const { groups, tiles } = nestedMap([{ name: 'a.iso', dir: false, size: 50, files: 1, dirs: 0, mtime: '' }, { ...folder('leer', []), size: 20 }], 1000, 600);
    expect(groups).toEqual([]);
    expect(tiles.map((t) => [t.e.name, t.group, t.i])).toEqual([['a.iso', -1, 0], ['leer', -1, 1]]);
  });
});
