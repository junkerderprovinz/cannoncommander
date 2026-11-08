import { beforeEach, describe, expect, it } from 'vitest';
import { cut, prototypeSource, runPrototype } from '../testing/prototype';
import { rank, recentFolders, recordVisit, topFolders } from './frequent';

const NOW = Date.UTC(2026, 10, 2, 9, 30);
const H = 3600000;
const protoRank = runPrototype<(v: { n: number; t: number }) => number>(
  `${cut(prototypeSource('features/e-10-handling.js'), 'const rank = (v) => {', '};')}; rank`, { Date: { now: () => NOW } },
);

describe('frequent folders', () => {
  beforeEach(() => localStorage.clear());

  it.each([[1, 0.5], [3, 2], [2, 30], [9, 100], [4, 400], [1, 24], [1, 168]])('ranks %i visits %f hours ago as the prototype does', (n, hours) => {
    const v = { n, t: NOW - hours * H };
    expect(rank(v, NOW)).toBe(protoRank(v));
  });

  it('puts a folder used often lately before one used more long ago', () => {
    for (let k = 0; k < 6; k++) recordVisit('/mnt/user/backups', NOW - 300 * H);
    for (let k = 0; k < 2; k++) recordVisit('/mnt/user/filme', NOW - 0.2 * H);
    recordVisit('/mnt/user/isos', NOW - 30 * H);
    expect(topFolders(2, '', NOW)).toEqual(['/mnt/user/filme', '/mnt/user/backups']);
    expect(topFolders(8, '/mnt/user/filme', NOW)).toEqual(['/mnt/user/backups', '/mnt/user/isos']);
  });

  it('lists the folders used last, however often, the latest first', () => {
    for (let k = 0; k < 6; k++) recordVisit('/C/Users/jdp/Downloads', NOW - 3 * H);
    recordVisit('/C/Users/jdp/Bilder', NOW - H);
    recordVisit('/D/Spiele', NOW - 2 * H);
    expect(recentFolders(2)).toEqual(['/C/Users/jdp/Bilder', '/D/Spiele']);
    expect(recentFolders(6, ['/D/Spiele'])).toEqual(['/C/Users/jdp/Bilder', '/C/Users/jdp/Downloads']);
  });
});
