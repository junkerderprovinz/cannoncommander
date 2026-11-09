import { afterEach, beforeAll, beforeEach, describe, expect, it } from 'vitest';
import { hideTip, installTips } from './tips';

beforeAll(() => {
  window.matchMedia ??= ((query: string) => ({ matches: false, media: query, addEventListener() {}, removeEventListener() {} })) as unknown as typeof window.matchMedia;
  installTips();
});

let tile: HTMLButtonElement, other: HTMLDivElement;
beforeEach(() => {
  tile = Object.assign(document.body.appendChild(document.createElement('button')), { textContent: 'Aussehen' });
  tile.dataset.tip = 'Aussehen';
  other = document.body.appendChild(document.createElement('div'));
});
afterEach(() => {
  hideTip();
  document.body.innerHTML = '';
});

const bubble = () => document.querySelector('.bubble')?.textContent ?? null;
const over = (el: Element, from: Element | null = null) => el.dispatchEvent(new MouseEvent('mouseover', { bubbles: true, relatedTarget: from }));

describe('the bubble beside a control', () => {
  it('shows the tip while the pointer is on the control', () => {
    over(tile);
    expect(bubble()).toBe('Aussehen');
  });

  it('closes on a click, which may change the view under a pointer that does not move', () => {
    over(tile);
    tile.dispatchEvent(new PointerEvent('pointerdown', { bubbles: true }));
    expect(bubble()).toBeNull();
  });

  it('closes when the pointer leaves the control, also out of the window', () => {
    over(tile);
    tile.dispatchEvent(new MouseEvent('mouseout', { bubbles: true, relatedTarget: null }));
    expect(bubble()).toBeNull();
  });

  it('stays while the pointer moves within the control', () => {
    const label = tile.appendChild(document.createElement('span'));
    over(tile);
    tile.dispatchEvent(new MouseEvent('mouseout', { bubbles: true, relatedTarget: label }));
    expect(bubble()).toBe('Aussehen');
  });

  it('closes when the window loses focus, on a key and on a scroll', () => {
    for (const close of [() => dispatchEvent(new Event('blur')), () => dispatchEvent(new KeyboardEvent('keydown', { key: 'F1' })), () => other.dispatchEvent(new Event('scroll'))]) {
      over(tile);
      close();
      expect(bubble()).toBeNull();
    }
  });
});
