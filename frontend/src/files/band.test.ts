import { afterEach, describe, expect, it } from 'vitest';
import { freeSpace, useBand } from './band';

// Three rows of 20 pixels under each other, 300 wide, with the name in the first 100.
function list() {
  document.body.innerHTML = '<div class="list"><div class="lhead"><button class="lh">Name</button></div>'
    + ['..', 'a.txt', 'b.txt'].map((n, i) => `<div class="row" data-idx="${i}"><span class="name"><span class="t">${n}</span></span><span class="cell">1 KB</span></div>`).join('')
    + '</div>';
  const rows = [...document.querySelectorAll<HTMLElement>('.row')];
  rows.forEach((r, i) => { r.getBoundingClientRect = () => ({ left: 0, right: 300, top: 20 + i * 20, bottom: 40 + i * 20, width: 300, height: 20, x: 0, y: 20 + i * 20, toJSON() {} }); });
  let marked = new Set<string>(['z.txt']);
  const band = useBand({ rows: () => rows, nameAt: (i) => ['', 'a.txt', 'b.txt'][i]!, marked: () => marked, mark: (m) => { marked = m; } });
  return { band, rows, marked: () => marked };
}

// jsdom has mouse events but no pointer events, so a mouse event stands in with the pointer's type added.
function press(target: Element, x: number, y: number, { pointerType = 'mouse', ctrlKey = false } = {}) {
  const e = new MouseEvent('pointerdown', { button: 0, clientX: x, clientY: y, ctrlKey });
  Object.defineProperty(e, 'target', { value: target });
  Object.defineProperty(e, 'pointerType', { value: pointerType });
  return e as PointerEvent;
}
const fire = (type: string, x: number, y: number) => dispatchEvent(new MouseEvent(type, { clientX: x, clientY: y }));

afterEach(() => { document.body.innerHTML = ''; });

describe('frame', () => {
  it('starts on free space and beside the name in a row view, never on the name or the header', () => {
    const { rows } = list();
    expect(freeSpace(document.querySelector('.list')!, true)).toBe(true);
    expect(freeSpace(rows[1]!.querySelector('.cell')!, true)).toBe(true);
    expect(freeSpace(rows[1]!.querySelector('.cell')!, false)).toBe(false);
    expect(freeSpace(rows[1]!.querySelector('.t')!, true)).toBe(false);
    expect(freeSpace(document.querySelector('.lh')!, true)).toBe(false);
  });

  it('marks every entry it touches instead of what was marked, and leaves the row that goes up', () => {
    const { band, rows, marked } = list();
    band.down(press(rows[0]!.querySelector('.cell')!, 200, 25), true);
    fire('pointermove', 210, 45);
    expect(band.frame.on).toBe(true);
    expect([...marked()]).toEqual(['a.txt']);
    fire('pointermove', 210, 65);
    expect([...marked()].sort()).toEqual(['a.txt', 'b.txt']);
    fire('pointerup', 210, 65);
    expect(band.frame.on).toBe(false);
    expect(band.drawn()).toBe(true);
    expect(band.drawn()).toBe(false);
  });

  it('adds to the marks with Ctrl', () => {
    const { band, rows, marked } = list();
    band.down(press(rows[2]!.querySelector('.cell')!, 200, 65, { ctrlKey: true }), true);
    fire('pointermove', 200, 75);
    fire('pointerup', 200, 75);
    expect([...marked()].sort()).toEqual(['b.txt', 'z.txt']);
  });

  it('is a plain click when the mouse hardly moves', () => {
    const { band, rows, marked } = list();
    band.down(press(rows[1]!.querySelector('.cell')!, 200, 30), true);
    fire('pointermove', 202, 31);
    fire('pointerup', 202, 31);
    expect(band.drawn()).toBe(false);
    expect([...marked()]).toEqual(['z.txt']);
  });

  it('leaves touch to the touch selection', () => {
    const { band } = list();
    band.down(press(document.querySelector('.list')!, 10, 10, { pointerType: 'touch' }), true);
    fire('pointermove', 200, 70);
    expect(band.frame.on).toBe(false);
  });
});
