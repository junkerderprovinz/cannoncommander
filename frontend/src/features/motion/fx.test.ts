import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { fakeCanvas } from '@/testing/fakeCanvas';
import type { Blast } from './burst';
import { drop, salute, shardLayer, shatter, sparks, trace } from './fx';

// The same numbers on every run, so a failure can be replayed.
const seeded = (seed: number) => () => ((seed = (seed * 16807) % 2147483647) - 1) / 2147483646;

const WILD: Blast = { life: 1.6, spread: 380, carry: 300, fall: 420, spin: 4 * Math.PI, count: 90, lag: 0.09 };

describe('the burst of a struck element', () => {
  let canvas: ReturnType<typeof fakeCanvas>;
  beforeEach(() => { vi.useFakeTimers(); canvas = fakeCanvas(); });
  afterEach(() => { canvas.restore(); vi.useRealTimers(); });

  const struck = () => {
    document.body.innerHTML = '<header class="head"><nav><button>Datei</button></nav></header>';
    const el = document.querySelector('button')!;
    el.getBoundingClientRect = () => ({ left: 200, right: 290, top: 10, bottom: 46, width: 90, height: 36, x: 200, y: 10, toJSON() {} }) as DOMRect;
    return { el, ...shardLayer() };
  };

  it('flashes and raises dust where the ball strikes, on the side it comes from', () => {
    for (const dir of [1, -1]) {
      const { el, layer, flight } = struck();
      shatter(el, { canvas: document.createElement('canvas'), scale: 1 }, flight, dir, 25, WILD, seeded(9));
      const impact = layer.querySelector<HTMLElement>('.fx-impact')!;
      expect([impact.style.getPropertyValue('left'), impact.style.getPropertyValue('top')]).toEqual([dir > 0 ? '200px' : '290px', '25px']);
      expect(layer.querySelectorAll('.fx-dust').length).toBeGreaterThanOrEqual(3);
    }
  });

  it('throws the picture of the element onto the canvas over the page, in splinters', () => {
    const { el, layer, flight } = struck();
    shatter(el, { canvas: document.createElement('canvas'), scale: 1 }, flight, 1, 25, WILD, seeded(9));
    vi.advanceTimersByTime(20);
    expect(layer.parentElement).toBe(document.body);
    expect(layer.querySelector('canvas.fx-burst')).not.toBeNull();
    expect(canvas.drawn.splinters).toBeGreaterThanOrEqual(WILD.count * 0.75);
    expect(layer.querySelector('button')).toBeNull();
  });
});

describe('the pieces a level fires', () => {
  const box = (left: number, top: number, w = 40, h = 20) => {
    const el = document.createElement('div');
    el.getBoundingClientRect = () => ({ left, top, right: left + w, bottom: top + h, width: w, height: h, x: left, y: top, toJSON() {} }) as DOMRect;
    document.body.append(el);
    return el;
  };
  const pieces = (cls: string) => [...document.querySelectorAll<HTMLElement>(`.fx-layer .${cls}`)];
  const level = (dials: Record<string, string>) => {
    for (const [k, v] of Object.entries(dials)) document.documentElement.style.setProperty(k, v);
  };

  beforeEach(() => { document.body.innerHTML = ''; document.documentElement.removeAttribute('style'); });

  it('sends as many lights across as the level asks for, side by side and one after another', () => {
    level({ '--motion-trace-count': '5', '--motion-trace-spread': '40px' });
    trace(box(0, 0), box(400, 0));
    const lights = pieces('fx-trace');
    expect(lights.length).toBe(5);
    expect(new Set(lights.map((l) => l.style.getPropertyValue('top'))).size).toBe(5);
    expect(lights.map((l) => l.style.getPropertyValue('--fx-i'))).toEqual(['0', '1', '2', '3', '4']);
  });

  it('still sends one light where the level names none', () => {
    trace(box(0, 0), box(400, 0));
    expect(pieces('fx-trace').length).toBe(1);
  });

  it('drops no more entries than the level allows', () => {
    const rows = Array.from({ length: 30 }, (_, i) => ({ el: box(0, i * 20), glyph: 'IconFile', name: `f${i}` }));
    for (const [count, want] of [['3', 3], ['16', 16]] as const) {
      document.querySelector('.fx-layer')?.remove();
      level({ '--motion-drop-count': count });
      expect(drop(rows, box(500, 500)).length).toBe(want);
      expect(pieces('fx-drop').length).toBe(want);
    }
  });

  it('fires as many stars in the salute as the level asks for', () => {
    level({ '--motion-salute-count': '21' });
    salute(box(0, 0, 1000, 50));
    expect(pieces('fx-salute').length).toBe(21);
  });

  it('throws sparks out from the middle of a finished job, each its own way, and none where the level has none', () => {
    level({ '--motion-spark-count': '10' });
    sparks(box(100, 100, 40, 40));
    const thrown = pieces('fx-spark');
    expect(thrown.length).toBe(10);
    expect(new Set(thrown.map((s) => `${s.style.getPropertyValue('left')} ${s.style.getPropertyValue('top')}`))).toEqual(new Set(['120px 120px']));
    expect(new Set(thrown.map((s) => s.style.getPropertyValue('--fx-sx'))).size).toBe(10);
    document.querySelector('.fx-layer')?.remove();
    level({ '--motion-spark-count': '0' });
    sparks(box(100, 100, 40, 40));
    expect(pieces('fx-spark').length).toBe(0);
  });
});
