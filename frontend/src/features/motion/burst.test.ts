import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { fakeCanvas } from '@/testing/fakeCanvas';
import { type Blast, burst, paint, splinters } from './burst';

const area = (poly: [number, number][]) => Math.abs(poly.reduce((sum, [x, y], i) => {
  const [nx, ny] = poly[(i + 1) % poly.length]!;
  return sum + x * ny - nx * y;
}, 0)) / 2;

// The same numbers on every run, so a failure can be replayed.
const seeded = (seed: number) => () => ((seed = (seed * 16807) % 2147483647) - 1) / 2147483646;

// A menu title as wide as a word and the ball striking it at its left edge, a little above the middle.
const W = 90, H = 36, AT = { x: 0, y: 15 };
const WILD: Blast = { life: 1.6, spread: 380, carry: 300, fall: 420, spin: 4 * Math.PI, count: 90, lag: 0.09 };

describe('the splinters of a struck element', () => {
  it('are about as many as the level asks for, and more at a livelier level', () => {
    const few = splinters(W, H, AT, 1, { ...WILD, count: 28 }, seeded(7)).length;
    const many = splinters(W, H, AT, 1, WILD, seeded(7)).length;
    const most = splinters(W, H, AT, 1, { ...WILD, count: 180 }, seeded(7)).length;
    expect(many).toBeGreaterThanOrEqual(WILD.count * 0.75);
    expect(many).toBeLessThanOrEqual(WILD.count * 1.25);
    expect(few < many && many < most, `${few} ${many} ${most}`).toBe(true);
  });

  it('are small', () => {
    for (const s of splinters(W, H, AT, 1, WILD, seeded(2))) expect(area(s.poly)).toBeLessThan((W * H) / 30);
  });

  it('cover the whole element without a gap or an overlap', () => {
    for (const seed of [1, 2, 99]) {
      const list = splinters(W, H, AT, 1, WILD, seeded(seed));
      expect(list.flatMap((s) => s.poly).every(([x, y]) => x >= 0 && x <= W && y >= 0 && y <= H)).toBe(true);
      expect(list.reduce((sum, s) => sum + area(s.poly), 0)).toBeCloseTo(W * H, 6);
    }
  });

  it('break unevenly, so they read as splinters and not as tiles', () => {
    const sizes = new Set(splinters(W, H, AT, 1, WILD, seeded(5)).map((s) => Math.round(area(s.poly))));
    expect(sizes.size).toBeGreaterThan(10);
  });

  for (const [way, dir] of [['right', 1], ['left', -1]] as const) {
    it(`fly on with the ball flying ${way}`, () => {
      const at = { x: dir > 0 ? 0 : W, y: 15 }, list = splinters(W, H, at, dir, WILD, seeded(9));
      const along = list.filter((s) => Math.sign(s.vx) === dir);
      expect(along.length / list.length).toBeGreaterThan(0.85);
    });
  }

  it('fly far, most of them several times the height of the head', () => {
    const reach = splinters(W, H, AT, 1, WILD, seeded(3)).map((s) => Math.hypot(s.vx, s.vy) * WILD.life).sort((a, b) => a - b);
    expect(reach[Math.floor(reach.length / 2)]).toBeGreaterThan(WILD.spread * 0.8);
  });

  it('turn both ways, and leave the strike first where they are nearest to it', () => {
    const list = splinters(W, H, AT, 1, WILD, seeded(11));
    expect(list.some((s) => s.spin > 0) && list.some((s) => s.spin < 0)).toBe(true);
    const near = list.reduce((a, b) => (Math.hypot(b.x - AT.x, b.y - AT.y) < Math.hypot(a.x - AT.x, a.y - AT.y) ? b : a));
    expect(near.delay).toBe(Math.min(...list.map((s) => s.delay)));
  });
});

describe('the picture of an element', () => {
  let canvas: ReturnType<typeof fakeCanvas>;
  beforeEach(() => { canvas = fakeCanvas(); });
  afterEach(() => canvas.restore());

  const shown = (el: Element, left: number, width: number) => {
    el.getBoundingClientRect = () => ({ left, right: left + width, top: 10, bottom: 46, width, height: 36, x: left, y: 10, toJSON() {} }) as DOMRect;
    el.getClientRects = () => [el.getBoundingClientRect()] as unknown as DOMRectList;
  };

  it('holds the words and the ground of the element, on the ground of the bar it sits on', () => {
    document.body.innerHTML = '<header style="background-color: rgb(40, 40, 40)"><button><span>Datei</span><span class="gone">Weg</span></button></header>';
    const button = document.querySelector('button')!;
    shown(button, 200, 90);
    shown(button.firstElementChild!, 210, 40);
    const pic = paint(button)!;
    expect(canvas.drawn.texts).toEqual(['Datei']);
    expect(canvas.drawn.grounds).toBe(3);
    expect(pic.canvas.width).toBe(90 * pic.scale);
  });
});

describe('the canvas the splinters fly on', () => {
  let canvas: ReturnType<typeof fakeCanvas>;
  beforeEach(() => { vi.useFakeTimers(); canvas = fakeCanvas(); document.body.innerHTML = '<div class="layer"></div>'; });
  afterEach(() => { canvas.restore(); vi.useRealTimers(); });

  const thrown = () => {
    const flight = burst(document.querySelector('.layer')!);
    const pic = { canvas: document.createElement('canvas'), scale: 1 }, pieces = splinters(W, H, AT, 1, WILD, seeded(4));
    flight.hit(pic, 200, 10, { x: 200, y: 25 }, pieces, 1, WILD, seeded(4));
    return { flight, pieces };
  };

  it('draws every splinter and a spray of sparks while they fly', () => {
    const { pieces } = thrown();
    vi.advanceTimersByTime(20);
    expect(canvas.drawn.splinters).toBe(pieces.length);
    expect(canvas.drawn.strokes).toBeGreaterThanOrEqual(10);
  });

  it('stops drawing once the last splinter has faded', () => {
    thrown();
    vi.advanceTimersByTime((WILD.life + WILD.lag) * 1000 + 50);
    canvas.reset();
    vi.advanceTimersByTime(500);
    expect(canvas.drawn.splinters).toBe(0);
  });

  it('takes itself off the page when stopped', () => {
    const { flight } = thrown();
    flight.stop();
    expect(document.querySelector('.fx-burst')).toBeNull();
    canvas.reset();
    vi.advanceTimersByTime(100);
    expect(canvas.drawn.splinters).toBe(0);
  });
});
