import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { fakeCanvas } from '@/testing/fakeCanvas';
import type { Blast } from './burst';
import { shardLayer, shatter } from './fx';

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
