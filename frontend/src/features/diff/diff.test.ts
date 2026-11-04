import { readFileSync } from 'node:fs';
import { join } from 'node:path';
import vm from 'node:vm';
import { describe, expect, it } from 'vitest';
import { diffLines } from '../../../scripts/fixture-tools.js';
import { countOf, hunksOf, inline, type Piece } from './diff';

// The prototype's diffLines and inline, cut out of b-30-diff.js; inline writes markup there, here it hands pieces back.
const src = readFileSync(join(__dirname, '../../../../prototype/src/js/features/b-30-diff.js'), 'utf8');
const esc = (s: string) => String(s).replace(/[&<>"]/g, (c) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;' })[c]!);
const proto = vm.runInNewContext(`${src.slice(src.indexOf('  function diffLines'), src.indexOf('  function pickPair'))}\n;({ diffLines, inline })`, { esc }) as {
  diffLines: (a: string[], b: string[]) => { t: string; l: number | null; r: number | null }[];
  inline: (a: string, b: string) => [string, string];
};
const html = (pieces: Piece[]) => pieces.map((p) => (p.mark ? `<mark>${esc(p.text)}</mark>` : esc(p.text))).join('');

describe('file compare', () => {
  const a = ['services:', '  plex:', '    image: plex:latest', '    restart: always', '', 'volumes:', '  data:'];
  const b = ['services:', '  plex:', '    image: plex:1.41', '    ports:', '      - 32400:32400', '    restart: always', 'volumes:'];

  it('pairs lines as the prototype does', () => {
    expect(diffLines(a, b)).toEqual(proto.diffLines(a, b));
    expect(diffLines([], b)).toEqual(proto.diffLines([], b));
    expect(diffLines(a, a)).toEqual(proto.diffLines(a, a));
  });

  it('marks the middle part that differs', () => {
    for (const [x, y] of [['image: plex:latest', 'image: plex:1.41'], ['abc', 'abc'], ['', 'neu'], ['a <b> c', 'a <i> c']] as const) {
      const [l, r] = inline(x, y);
      expect([html(l), html(r)]).toEqual(proto.inline(x, y));
    }
  });

  it('counts a run of changed rows as one place', () => {
    const rows = diffLines(a, b);
    expect(hunksOf(rows)).toEqual([2, 6, 8]);
    expect(hunksOf(diffLines(a, a))).toEqual([]);
    expect([countOf(rows, 'chg'), countOf(rows, 'del'), countOf(rows, 'add')]).toEqual([1, 2, 2]);
  });
});
