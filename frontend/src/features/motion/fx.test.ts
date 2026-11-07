import { describe, expect, it } from 'vitest';
import { shards } from './fx';

const area = (poly: [number, number][]) => Math.abs(poly.reduce((sum, [x, y], i) => {
  const [nx, ny] = poly[(i + 1) % poly.length]!;
  return sum + x * ny - nx * y;
}, 0)) / 2;

// The same numbers on every run, so a failure can be replayed.
const seeded = (seed: number) => () => ((seed = (seed * 16807) % 2147483647) - 1) / 2147483646;

describe('the shards of a broken element', () => {
  it('cut every cell of the grid in two', () => {
    expect(shards(3, 2, seeded(7))).toHaveLength(12);
  });

  it('stay inside the element', () => {
    const points = shards(4, 2, seeded(3)).flat();
    expect(points.every(([x, y]) => x >= 0 && x <= 100 && y >= 0 && y <= 100)).toBe(true);
  });

  it('cover the whole element without a gap or an overlap', () => {
    for (const seed of [1, 2, 99]) {
      const total = shards(3, 2, seeded(seed)).reduce((sum, p) => sum + area(p), 0);
      expect(total).toBeCloseTo(100 * 100, 6);
    }
  });

  it('break unevenly, so they read as shards and not as a grid', () => {
    const sizes = new Set(shards(3, 2, seeded(5)).map((p) => Math.round(area(p))));
    expect(sizes.size).toBeGreaterThan(3);
  });
});
