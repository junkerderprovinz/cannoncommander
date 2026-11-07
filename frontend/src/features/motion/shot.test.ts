import { afterEach, beforeAll, beforeEach, describe, expect, it, vi } from 'vitest';
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
  document.body.innerHTML = `<header class="head">
    <button class="brand"><span class="brand-logo"><svg class="brand-cannon"><g></g></svg><svg class="brand-ball"><g><circle r="5"/></g></svg></span></button>
    <span class="hsep"></span>
    <nav class="menubar"><button class="glim-menubar-item" data-menu="0">Datei</button><button class="glim-menubar-item" data-menu="1">Ansicht</button></nav>
    <span class="spacer"></span>
    <button data-run="palette" id="search">Suche</button>
    <button class="avatar" data-account><i>AD</i></button>
  </header>`;
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
const leftovers = () => document.querySelectorAll('.fx-shards, .fx-shot, .fx-puff, .fx-shard, .fx-tremble, .fx-fired, .fx-shattered, .fx-rebuild').length;

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
    const layer = head.querySelector('.fx-shards')!;
    expect(layer.getAttribute('aria-hidden')).toBe('true');
    expect(layer.querySelectorAll('.fx-shard').length).toBeGreaterThanOrEqual(5 * 4);
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
    expect(leftovers()).toBe(0);
  });
});

describe('the press on the logo', () => {
  let disarm: () => void;
  const brand = () => head.querySelector('.brand')!;
  const send = (el: Element, type: string, init: PointerEventInit = {}) => el.dispatchEvent(new PointerEvent(type, { bubbles: !type.endsWith('leave'), button: 0, isPrimary: true, pointerId: 1, ...init }));
  const click = () => brand().dispatchEvent(new MouseEvent('click', { bubbles: true }));

  beforeEach(() => { disarm = armLogo(window, TIMES); });
  afterEach(() => disarm());

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
