import { beforeAll, beforeEach, describe, expect, it } from 'vitest';
import { action } from './actions';
import { installNav, Nav, pushNav, setNavSource } from './nav';

let active = 0;
const stepped: string[] = [];
const panel = (i: number) => {
  const el = document.createElement('section');
  el.className = 'panel';
  el.dataset.panel = String(i);
  el.append(document.createElement('div'));
  return document.body.appendChild(el);
};
const over = (el: Element | null) => (el ?? document.body).dispatchEvent(new PointerEvent('pointerover', { bubbles: true }));
const sideButton = (back: boolean) => dispatchEvent(new PopStateEvent('popstate', { state: { ccNav: Nav.at + (back ? -1 : 1) } }));

beforeAll(() => {
  setNavSource({ hash: () => '', setActive: (p) => { active = p; } });
  action('back', { label: 'back', run: () => { stepped.push(`back ${active}`); } });
  action('forward', { label: 'forward', run: () => { stepped.push(`forward ${active}`); } });
  installNav();
});

beforeEach(() => {
  document.body.innerHTML = '';
  stepped.length = 0;
  over(null);
});

describe("the mouse's side buttons", () => {
  it('step the panel switched to, not the one that went through the folders', () => {
    active = 1;
    pushNav(1);
    pushNav(1);
    active = 0;
    sideButton(true);
    sideButton(false);
    expect(stepped).toEqual(['back 0', 'forward 0']);
  });

  it('step the panel under the pointer and make it the active one', () => {
    const left = panel(0);
    panel(1);
    active = 1;
    pushNav(1);
    over(left.firstElementChild);
    sideButton(true);
    expect(stepped).toEqual(['back 0']);
    expect(active).toBe(0);
  });

  it('step the active panel while the pointer is outside both', () => {
    panel(0);
    active = 0;
    pushNav(0);
    active = 1;
    over(document.body);
    sideButton(true);
    expect(stepped).toEqual(['back 1']);
  });
});
