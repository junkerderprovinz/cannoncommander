import { afterEach, beforeAll, beforeEach, describe, expect, it, vi } from 'vitest';
import { installTips } from '@/core/tips';
import { gates, inside, keyframes, rules, sheets } from '@/testing/motionCss';
import { HOLD } from './cues';
import { armLogo, fire, stopShot, type Times } from './shot';

const TIMES: Times = { tremble: 900, shot: 900, wait: 700, rebuild: 400, step: 30 };
const END = TIMES.tremble + TIMES.shot + TIMES.wait + TIMES.rebuild + 10 * TIMES.step + 100;

// jsdom lays nothing out, so every box is placed by hand along one row.
const place = (el: Element, left: number, width: number) => {
  el.getBoundingClientRect = () => ({ left, right: left + width, top: 10, bottom: 50, width, height: 40, x: left, y: 10, toJSON() {} }) as DOMRect;
  el.getClientRects = () => [el.getBoundingClientRect()] as unknown as DOMRectList;
};

let head: HTMLElement, navigated: number;
function build() {
  document.body.innerHTML = `<div class="app"><header class="head">
    <button class="brand"><span class="brand-logo"><svg class="brand-cannon"><g></g></svg><svg class="brand-ball"><g><circle r="5"/></g></svg></span></button>
    <span class="hsep"></span>
    <nav class="menubar"><button class="glim-menubar-item" data-menu="0">Datei</button><button class="glim-menubar-item" data-menu="1">Ansicht</button></nav>
    <span class="spacer"></span>
    <button data-run="palette" id="search">Suche</button>
    <button class="avatar" data-account><i>AD</i></button>
  </header><main class="main"></main></div>`;
  head = document.querySelector('.head')!;
  const at = (sel: string, left: number, width: number) => place(head.querySelector(sel)!, left, width);
  place(head, 0, 1000);
  at('.brand', 8, 120); at('.brand-logo', 12, 112); at('.brand-cannon', 12, 112); at('.brand-cannon g', 12, 84); at('.brand-ball g', 100, 24);
  at('.hsep', 140, 1); at('.menubar', 150, 200); at('[data-menu="0"]', 150, 90); at('[data-menu="1"]', 250, 100); at('.spacer', 360, 400);
  at('#search', 800, 40); at('.avatar', 900, 40);
  navigated = 0;
  head.querySelector('.brand')!.addEventListener('click', () => { navigated++; });
}

const broken = () => [...head.querySelectorAll('.fx-shattered')].map((el) => el.textContent?.trim() || el.classList[0]);
const leftovers = () => document.querySelectorAll('.fx-shards, .fx-shot, .fx-flash, .fx-gunsmoke, .fx-shard, .fx-tremble, .fx-loading, .fx-fired, .fx-shattered, .fx-rebuild, .fx-quake').length;

beforeAll(() => {
  (SVGElement.prototype as unknown as { getBBox: () => DOMRect }).getBBox = () => ({ x: 830, y: 20, width: 170, height: 180 }) as DOMRect;
});
beforeEach(() => { vi.useFakeTimers(); build(); delete document.documentElement.dataset.motion; });
afterEach(() => { stopShot(); vi.useRealTimers(); });

describe('the shot across the head', () => {
  it('trembles first and breaks nothing before it fires', () => {
    expect(fire(head, TIMES)).toBe(true);
    expect(head.querySelector('.brand-cannon')!.classList).toContain('fx-tremble');
    vi.advanceTimersByTime(TIMES.tremble - 1);
    expect(broken()).toEqual([]);
  });

  it('loads the ball into the barrel while the cannon trembles', () => {
    fire(head, TIMES);
    const ball = head.querySelector<HTMLElement>('.brand-ball')!;
    expect(ball.classList).toContain('fx-loading');
    expect(ball.classList).toContain('fx-tremble');
    expect(parseFloat(ball.style.getPropertyValue('--fx-reach'))).toBeGreaterThanOrEqual(100);
    vi.advanceTimersByTime(TIMES.tremble);
    expect(ball.classList).not.toContain('fx-loading');
  });

  it('fires with a flash at the muzzle and a cloud of smoke after it', () => {
    fire(head, TIMES);
    vi.advanceTimersByTime(TIMES.tremble);
    const flash = document.querySelector<HTMLElement>('.fx-flash')!;
    expect(flash).not.toBeNull();
    expect(flash.style.getPropertyValue('left')).toBe('96px');
    expect(document.querySelectorAll('.fx-gunsmoke').length).toBeGreaterThanOrEqual(4);
    expect(document.querySelector('.fx-puff')).toBeNull();
  });

  it('trails fire behind the flying ball', () => {
    fire(head, TIMES);
    vi.advanceTimersByTime(TIMES.tremble);
    const shot = document.querySelector('.fx-shot')!;
    expect(shot.firstElementChild!.classList).toContain('fx-trail');
    expect(shot.querySelector('svg')).not.toBeNull();
  });

  it('sends the ball out of the muzzle rather than from where it lay', () => {
    fire(head, TIMES);
    vi.advanceTimersByTime(TIMES.tremble);
    expect(document.querySelector<HTMLElement>('.fx-shot')!.style.getPropertyValue('left')).toBe('84px');
  });

  it('bursts what it strikes from the point of impact, with a flash and a puff of dust there', () => {
    fire(head, TIMES);
    vi.advanceTimersByTime(TIMES.tremble + TIMES.shot * 0.1);
    const layer = document.querySelector('.fx-shards')!;
    expect(layer.querySelectorAll('.fx-impact')).toHaveLength(2);
    expect(layer.querySelectorAll('.fx-dust').length).toBeGreaterThanOrEqual(2 * 3);
    expect(layer.querySelectorAll('.fx-shard').length).toBeGreaterThanOrEqual(2 * 24);
    const impact = layer.querySelector<HTMLElement>('.fx-impact')!;
    expect([impact.style.getPropertyValue('left'), impact.style.getPropertyValue('top')]).toEqual(['140px', '30px']);
  });

  it('breaks what the ball passes in the order it passes it, and nothing it never reaches', () => {
    fire(head, TIMES);
    vi.advanceTimersByTime(TIMES.tremble);
    expect(head.querySelector('.brand-logo')!.classList).toContain('fx-fired');
    expect(document.querySelector('.fx-shot')).not.toBeNull();
    vi.advanceTimersByTime(TIMES.shot * 0.1);
    expect(broken()).toEqual(['hsep', 'Datei']);
    vi.advanceTimersByTime(TIMES.shot * 0.9);
    expect(broken()).toEqual(['hsep', 'Datei', 'Ansicht', 'Suche', 'AD']);
    expect(head.querySelector('.spacer')!.classList).not.toContain('fx-shattered');
    expect(head.querySelector('.brand')!.classList).not.toContain('fx-shattered');
  });

  it('draws the shards from copies that carry none of the hooks of the real controls', () => {
    fire(head, TIMES);
    vi.advanceTimersByTime(TIMES.tremble + TIMES.shot);
    const layer = document.querySelector('.fx-shards')!;
    expect(layer.getAttribute('aria-hidden')).toBe('true');
    expect(layer.querySelectorAll('.fx-shard').length).toBeGreaterThanOrEqual(5 * 24);
    expect(layer.querySelectorAll('[data-run], [data-menu], [data-account], [id]')).toHaveLength(0);
    expect(document.querySelectorAll('[data-run="palette"]')).toHaveLength(1);
  });

  it('puts the head back together and leaves nothing behind', () => {
    fire(head, TIMES);
    vi.advanceTimersByTime(END);
    expect(leftovers()).toBe(0);
    expect(head.children).toHaveLength(6);
  });

  it('puts everything back at once when stopped mid-flight', () => {
    fire(head, TIMES);
    vi.advanceTimersByTime(TIMES.tremble + TIMES.shot / 2);
    expect(broken().length).toBeGreaterThan(0);
    stopShot();
    expect(leftovers()).toBe(0);
    vi.advanceTimersByTime(END);
    expect(leftovers()).toBe(0);
  });

  it('plays once at a time, however often it is asked', () => {
    fire(head, TIMES);
    vi.advanceTimersByTime(TIMES.tremble + 10);
    expect(fire(head, TIMES)).toBe(true);
    expect(document.querySelectorAll('.fx-shot')).toHaveLength(1);
    vi.advanceTimersByTime(END);
    expect(leftovers()).toBe(0);
  });

  it('plays nothing with motion off', () => {
    document.documentElement.dataset.motion = 'off';
    expect(fire(head, TIMES)).toBe(false);
    vi.advanceTimersByTime(END);
    expect(leftovers()).toBe(0);
  });

  it('plays nothing when the system asks for less motion', () => {
    const was = window.matchMedia;
    window.matchMedia = ((q: string) => ({ matches: q.includes('reduce') })) as unknown as typeof window.matchMedia;
    try {
      expect(fire(head, TIMES)).toBe(false);
      vi.advanceTimersByTime(END);
      expect(leftovers()).toBe(0);
    } finally {
      window.matchMedia = was;
    }
  });

  it('flings the shards in a layer over the whole page rather than inside the head', () => {
    fire(head, TIMES);
    vi.advanceTimersByTime(TIMES.tremble + TIMES.shot * 0.1);
    const layer = document.querySelector('.fx-shards')!;
    expect(layer.parentElement).toBe(document.body);
    expect(head.querySelector('.fx-shards, .fx-shard')).toBeNull();
    expect(layer.querySelectorAll('.fx-shard').length).toBeGreaterThanOrEqual(2 * 24);
  });

  it('keeps the head around the copies, so they still look like the pieces of the head', () => {
    fire(head, TIMES);
    vi.advanceTimersByTime(TIMES.tremble + TIMES.shot);
    const frame = document.querySelector('.fx-shards .fx-shard')!.closest('.head')!;
    expect(frame).not.toBe(head);
    expect(frame.closest('.fx-shards')).not.toBeNull();
    expect(document.querySelectorAll('.fx-shards .head [id], .fx-shards .head[id]')).toHaveLength(0);
  });

  it('takes the layer off the page once the head is back', () => {
    fire(head, TIMES);
    vi.advanceTimersByTime(TIMES.tremble + TIMES.shot);
    expect(document.querySelector('.fx-shards')).not.toBeNull();
    vi.advanceTimersByTime(END);
    expect(document.querySelector('.fx-shards')).toBeNull();
  });

  it('sends every shard on the way the ball flies', () => {
    fire(head, TIMES);
    vi.advanceTimersByTime(TIMES.tremble + TIMES.shot);
    const carry = [...document.querySelectorAll<HTMLElement>('.fx-shard')].map((el) => parseFloat(el.style.getPropertyValue('--fx-carry')));
    expect(carry.length).toBeGreaterThan(0);
    expect(carry.every((c) => c > 0)).toBe(true);
  });

  it('shakes the whole window the moment it fires', () => {
    fire(head, TIMES);
    const app = document.querySelector('.app')!;
    vi.advanceTimersByTime(TIMES.tremble - 1);
    expect(app.classList).not.toContain('fx-quake');
    vi.advanceTimersByTime(1);
    expect(app.classList).toContain('fx-quake');
  });

  it('stops the shake of the window with the shot', () => {
    fire(head, TIMES);
    vi.advanceTimersByTime(TIMES.tremble);
    const app = document.querySelector('.app')!;
    expect(app.classList).toContain('fx-quake');
    stopShot();
    expect(app.classList).not.toContain('fx-quake');
  });
});

describe('the tip of the logo', () => {
  const over = () => head.querySelector('.brand svg')!.dispatchEvent(new MouseEvent('mouseover', { bubbles: true }));
  const tip = () => document.querySelector('.bubble');
  beforeAll(() => {
    window.matchMedia ??= (() => ({ matches: false })) as unknown as typeof window.matchMedia;
    installTips();
  });
  beforeEach(() => { head.querySelector('.brand')!.setAttribute('data-tip', 'CannonCommander'); });

  it('goes away as the cannon starts to tremble and stays away until the head is back', () => {
    over();
    expect(tip()).not.toBeNull();
    fire(head, TIMES);
    expect(tip()).toBeNull();
    vi.advanceTimersByTime(TIMES.tremble + TIMES.shot / 2);
    over();
    expect(tip()).toBeNull();
    vi.advanceTimersByTime(END);
    over();
    expect(tip()).not.toBeNull();
  });

  it('comes back at once when the shot is stopped', () => {
    fire(head, TIMES);
    stopShot();
    over();
    expect(tip()).not.toBeNull();
  });
});

describe('the press on the logo', () => {
  let disarm: () => void;
  const brand = () => head.querySelector('.brand')!;
  const send = (el: Element, type: string, init: PointerEventInit = {}) => el.dispatchEvent(new PointerEvent(type, { bubbles: !type.endsWith('leave'), button: 0, isPrimary: true, pointerId: 1, ...init }));
  const click = () => brand().dispatchEvent(new MouseEvent('click', { bubbles: true }));

  beforeEach(() => { disarm = armLogo(window, TIMES); });
  afterEach(() => disarm());

  it('starts to tremble within a third of a second of the press', () => {
    send(brand(), 'pointerdown');
    vi.advanceTimersByTime(300);
    expect(head.querySelector('.brand-cannon.fx-tremble')).not.toBeNull();
  });

  it('lets a short click go to the files', () => {
    send(brand(), 'pointerdown');
    vi.advanceTimersByTime(HOLD / 2);
    send(brand(), 'pointerup');
    click();
    expect(navigated).toBe(1);
    expect(head.querySelector('.fx-tremble')).toBeNull();
  });

  it('fires on a long press and takes its click', () => {
    send(brand(), 'pointerdown');
    vi.advanceTimersByTime(HOLD);
    expect(head.querySelector('.fx-tremble')).not.toBeNull();
    send(brand(), 'pointerup');
    click();
    expect(navigated).toBe(0);
    click();
    expect(navigated).toBe(1);
  });

  it('is cancelled by a press that leaves the logo', () => {
    send(brand(), 'pointerdown');
    vi.advanceTimersByTime(HOLD / 2);
    send(brand(), 'pointerleave');
    vi.advanceTimersByTime(HOLD);
    expect(head.querySelector('.fx-tremble')).toBeNull();
  });

  it('ignores the second button', () => {
    send(brand(), 'pointerdown', { button: 2 });
    vi.advanceTimersByTime(HOLD);
    expect(head.querySelector('.fx-tremble')).toBeNull();
  });

  it('stays a plain button with motion off', () => {
    document.documentElement.dataset.motion = 'off';
    send(brand(), 'pointerdown');
    vi.advanceTimersByTime(HOLD);
    send(brand(), 'pointerup');
    click();
    expect(navigated).toBe(1);
  });

  it('puts the head back when the window changes size mid-flight', () => {
    send(brand(), 'pointerdown');
    vi.advanceTimersByTime(HOLD + TIMES.tremble + TIMES.shot / 2);
    expect(broken().length).toBeGreaterThan(0);
    window.dispatchEvent(new Event('resize'));
    expect(leftovers()).toBe(0);
  });
});

describe('the look of the shot', () => {
  const css = sheets().find((x) => x.file === 'features/motion/motion.css')!.css;
  const levels = sheets().find((x) => x.file === 'styles/motion.css')!.css;
  // A dial's number at a level, a duration in milliseconds.
  const dial = (level: string, name: string) => {
    const at = levels.indexOf(level === 'wild' ? ':root {' : `:root[data-motion="${level}"] {`);
    const m = levels.slice(at, levels.indexOf('}', at)).match(new RegExp(`--motion-${name}:\\s*(-?[\\d.]+)(ms|s|px|deg)?;`));
    return m ? +m[1]! * (m[2] === 's' ? 1000 : 1) : NaN;
  };
  // The swing of every beat of the tremble, as a share of the dial that sets its distance.
  const beats = () => [...keyframes(css).find((k) => k.name === 'fx-tremble')!.body.matchAll(/([\d.]+)%\s*\{([^}]*)\}/g)]
    .map((m) => ({ at: +m[1]!, swing: Math.max(...[...m[2]!.matchAll(/--motion-tremble-dist[^)]*\)\s*\*\s*(-?[\d.]+)/g)].map((x) => Math.abs(+x[1]!))), tilt: /--motion-tremble-tilt/.test(m[2]!) }));

  it('trembles visibly from the first beats', () => {
    expect(beats().filter((b) => b.at <= 10).every((b) => b.swing >= 0.15)).toBe(true);
  });

  it('shakes hardest and tilts in the last quarter before the shot', () => {
    const last = beats().filter((b) => b.at >= 75);
    expect(Math.max(...last.map((b) => b.swing))).toBeGreaterThanOrEqual(1);
    expect(last.every((b) => b.tilt)).toBe(true);
  });

  it('grows every dial of the shot from off through subtle and wild to the storm', () => {
    for (const name of ['tremble-dur', 'tremble-dist', 'tremble-tilt', 'quake-dur', 'quake-dist', 'shard-carry', 'shard-fall', 'shard-spread']) {
      const [off, subtle, wild, storm] = ['off', 'subtle', 'wild', 'storm'].map((l) => dial(l, name));
      expect([name, off], name).toEqual([name, 0]);
      expect(0 < subtle! && subtle! < wild! && wild! < storm!, `${name}: ${subtle} ${wild} ${storm}`).toBe(true);
    }
  });

  it('trembles long and hard before it fires', () => {
    const peak = Math.max(...beats().map((b) => b.swing));
    expect(dial('subtle', 'tremble-dur')).toBeGreaterThanOrEqual(650);
    expect(dial('wild', 'tremble-dur')).toBeGreaterThanOrEqual(1100);
    expect(dial('subtle', 'tremble-dist') * peak).toBeGreaterThanOrEqual(3);
    expect(dial('wild', 'tremble-dist') * peak).toBeGreaterThanOrEqual(6);
  });

  it('flings the shards well past the head', () => {
    // The head is 3.4rem high, about 54 px.
    expect(dial('wild', 'shard-fall')).toBeGreaterThanOrEqual(120);
    expect(dial('wild', 'shard-carry')).toBeGreaterThanOrEqual(120);
  });

  it('shakes the window for a moment by a few pixels and lets it settle', () => {
    for (const level of ['subtle', 'wild', 'storm']) {
      expect(dial(level, 'quake-dur'), level).toBeLessThanOrEqual(600);
      expect(dial(level, 'quake-dist'), level).toBeLessThanOrEqual(10);
    }
    const quake = keyframes(css).find((k) => k.name === 'fx-quake')!;
    expect(quake.body).toMatch(/to\s*\{[^}]*transform:\s*none/);
    const rule = rules(css).find((r) => /\.fx-quake\b/.test(r.selector))!;
    expect(rule.body).toMatch(/animation:\s*fx-quake\s+var\(--motion-quake-dur/);
    expect(inside(gates(css), rule.at)).toBe(true);
  });

  it('lays the shards over everything else on the page', () => {
    const top = Math.max(...sheets().flatMap(({ css: c }) => rules(c).filter((r) => !/\.fx-shards\b/.test(r.selector))
      .flatMap((r) => [...r.body.matchAll(/z-index:\s*(\d+)/g)].map((m) => +m[1]!))));
    const layer = rules(css).find((r) => /^\.fx-shards\s*$/.test(r.selector) && /z-index/.test(r.body))!;
    expect(+layer.body.match(/z-index:\s*(\d+)/)![1]!).toBeGreaterThan(top);
  });

  it('burns the muzzle flash and the trail in fire colours rather than the accent', () => {
    const fiery = rules(css).filter((r) => /\.fx-(flash|trail)\b/.test(r.selector) && /background/.test(r.body));
    expect(fiery.length).toBeGreaterThanOrEqual(3);
    expect(fiery.filter((r) => r.body.includes('--accent')).map((r) => r.selector)).toEqual([]);
    expect(fiery.every((r) => r.body.includes('--fx-fire-'))).toBe(true);
  });
});
