import { describe, expect, it } from 'vitest';
import { shards, shatter } from './fx';

const area = (poly: [number, number][]) => Math.abs(poly.reduce((sum, [x, y], i) => {
  const [nx, ny] = poly[(i + 1) % poly.length]!;
  return sum + x * ny - nx * y;
}, 0)) / 2;

// The same numbers on every run, so a failure can be replayed.
const seeded = (seed: number) => () => ((seed = (seed * 16807) % 2147483647) - 1) / 2147483646;

// A menu title as wide as a word and the ball striking it just inside its left edge, a little above the middle.
const W = 90, H = 36, AT = { x: 8, y: 15 };

describe('the shards of a broken element', () => {
  it('are many', () => {
    expect(shards(W, H, AT, seeded(7)).length).toBeGreaterThanOrEqual(24);
  });

  it('stay inside the element', () => {
    const points = shards(W, H, AT, seeded(3)).flatMap((s) => s.poly);
    expect(points.every(([x, y]) => x >= 0 && x <= 100 && y >= 0 && y <= 100)).toBe(true);
  });

  it('cover the whole element without a gap or an overlap', () => {
    for (const seed of [1, 2, 99]) {
      const total = shards(W, H, AT, seeded(seed)).reduce((sum, s) => sum + area(s.poly), 0);
      expect(total).toBeCloseTo(100 * 100, 6);
    }
  });

  it('break unevenly, so they read as shards and not as a grid', () => {
    const sizes = new Set(shards(W, H, AT, seeded(5)).map((s) => Math.round(area(s.poly))));
    expect(sizes.size).toBeGreaterThan(10);
  });

  it('crack outward from where the ball struck, the smallest pieces nearest to it', () => {
    const list = shards(W, H, AT, seeded(11));
    const near = list.filter((s) => s.dist < 0.3), far = list.filter((s) => s.dist > 0.7);
    expect(near.length).toBeGreaterThan(0);
    expect(far.length).toBeGreaterThan(0);
    const mean = (l: typeof list) => l.reduce((sum, s) => sum + area(s.poly), 0) / l.length;
    expect(mean(near)).toBeLessThan(mean(far));
  });

  it('point each piece away from the strike', () => {
    for (const s of shards(W, H, AT, seeded(4))) {
      const [cx, cy] = s.centre;
      const out = [(cx / 100) * W - AT.x, (cy / 100) * H - AT.y];
      expect(out[0]! * s.out[0] + out[1]! * s.out[1]).toBeGreaterThan(0);
    }
  });
});

describe('the burst of a struck element', () => {
  const struck = () => {
    document.body.innerHTML = '<header class="head"><nav><button>Datei</button></nav></header><div class="layer"></div>';
    const el = document.querySelector('button')!;
    el.getBoundingClientRect = () => ({ left: 200, right: 290, top: 10, bottom: 46, width: 90, height: 36, x: 200, y: 10, toJSON() {} }) as DOMRect;
    return { el, head: document.querySelector('.head')!, into: document.querySelector('.layer')! };
  };

  for (const [way, dir] of [['right', 1], ['left', -1]] as const) {
    it(`carries every shard on with the ball flying ${way}`, () => {
      const { el, head, into } = struck();
      shatter(el, head, into, dir, 25, seeded(9));
      const carry = [...into.querySelectorAll<HTMLElement>('.fx-shard')].map((s) => parseFloat(s.style.getPropertyValue('--fx-carry')));
      expect(carry.length).toBeGreaterThanOrEqual(24);
      expect(carry.every((c) => Math.sign(c) === dir)).toBe(true);
      // Some fly farther than others, so the burst spreads along the way.
      expect(Math.max(...carry.map(Math.abs)) - Math.min(...carry.map(Math.abs))).toBeGreaterThan(0.3);
    });
  }

  it('cuts the shards from copies that sit inside the head they came from', () => {
    const { el, head, into } = struck();
    shatter(el, head, into, 1, 25, seeded(9));
    const copy = into.querySelector('.fx-shard nav button');
    expect(copy?.textContent).toBe('Datei');
    expect(head.querySelectorAll('button')).toHaveLength(1);
  });
});
