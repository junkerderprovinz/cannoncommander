import { createPinia, setActivePinia } from 'pinia';
import { afterEach, beforeAll, beforeEach, describe, expect, it, vi } from 'vitest';
import { createApp, nextTick } from 'vue';
import { action, labelOf } from '@/core/actions';
import { closeMenu, isHead, menuState, type MenuItem } from '@/core/menu';
import { prefs } from '@/core/prefs';
import { go } from '@/core/views';
import Head from './Head.vue';
import { shell } from './state';

// jsdom lays nothing out, so a test hands the head its size through the observer it watches itself with.
let resized: () => void = () => {};
beforeAll(() => {
  vi.stubGlobal('ResizeObserver', class { constructor(fn: () => void) { resized = fn; } observe() {} disconnect() {} });
  setActivePinia(createPinia());
  const none = () => {};
  action('jobs', { label: 'Aufgaben', g: 'IconQueued', key: 'Ctrl+j', run: none });
  action('settings', { label: 'Einstellungen', g: 'IconGear', run: none });
  action('palette', { label: 'Befehle suchen', g: 'IconSearch', key: 'Ctrl+k', run: none });
  action('logout', { label: 'Abmelden', g: 'IconSignOut', run: none });
});

let app: ReturnType<typeof createApp>;
beforeEach(async () => {
  go('files');
  shell.jobCount = 0;
  prefs.dual = true;
  const el = document.createElement('div');
  document.body.append(el);
  app = createApp(Head);
  app.mount(el);
  await nextTick();
});
afterEach(() => {
  closeMenu();
  app.unmount();
  document.body.innerHTML = '';
});

const avatarMenu = () => {
  document.querySelector<HTMLElement>('.head [data-account]')!.click();
  return (menuState.list ?? []).filter((it): it is MenuItem => it !== '-' && !isHead(it));
};

describe('the head', () => {
  it('has a button for the settings, with its words, and none for the jobs', () => {
    const settings = document.querySelector('.head [data-run="settings"]');
    expect(settings?.textContent).toContain('Einstellungen');
    expect(document.querySelector('.head [data-run="jobs"]')).toBeNull();
  });

  it('keeps the jobs in the avatar menu and no second way into the settings', () => {
    const labels = avatarMenu().map((it) => labelOf(it));
    expect(labels).toContain('Aufgaben');
    expect(labels).toContain('Befehle suchen');
    expect(labels).not.toContain('Einstellungen');
    expect(labels).not.toContain('Konto & Sicherheit');
  });

  it('counts the running jobs on the avatar and on its menu entry', async () => {
    const badge = () => document.querySelector('.head [data-account] .count')!;
    expect(badge().classList).toContain('idle');
    shell.jobCount = 3;
    await nextTick();
    expect(badge().classList).not.toContain('idle');
    expect(badge().textContent).toBe('3');
    expect(avatarMenu().find((it) => labelOf(it) === 'Aufgaben')?.count).toBe(3);
  });

  it('leaves the count off the menu entry while nothing runs', () => {
    expect(avatarMenu().find((it) => labelOf(it) === 'Aufgaben')?.count).toBeUndefined();
  });

  it('names the layout that shows and switches to the other one', async () => {
    const layout = () => document.querySelector<HTMLElement>('.head .head-layout')!;
    expect(layout().textContent).toContain('Zwei Fenster');
    expect(layout().dataset.run).toBe('single');
    prefs.dual = false;
    await nextTick();
    expect(layout().textContent).toContain('Ein Fenster');
    expect(layout().dataset.run).toBe('dual');
  });

  it('wears the same look in both layouts, since its words already say which one shows', async () => {
    // The width stage follows the length of the words, which is not what the look is about.
    const look = (sel: string) => [...document.querySelector(sel)!.classList].filter((c) => !/^(head-layout|glim-btn-(xs|sm|md|lg))$/.test(c)).sort().join(' ');
    for (const dual of [true, false]) {
      prefs.dual = dual;
      await nextTick();
      expect(document.querySelector('.head .head-layout')!.hasAttribute('aria-pressed')).toBe(false);
      expect(look('.head .head-layout')).toBe(look('.head [data-run="settings"]'));
    }
  });
});

describe('a head short of room', () => {
  // What the head's content needs: with words on its buttons, with glyphs only, and with the menus folded as well.
  const room = (width: number) => {
    const head = document.querySelector<HTMLElement>('.head')!;
    const need = () => (head.classList.contains('fold-menus') ? 900 : head.querySelector('[data-run="settings"].glim-btn-icon') ? 1020 : 1100);
    Object.defineProperty(head, 'clientWidth', { configurable: true, get: () => width });
    Object.defineProperty(head, 'scrollWidth', { configurable: true, get: () => Math.max(width, need()) });
    return head;
  };
  const settle = async () => { resized(); for (let i = 0; i < 6; i++) await nextTick(); };
  const compact = () => document.querySelector('.head [data-run="settings"]')!.classList.contains('glim-btn-icon');

  it('shows its buttons with words where they fit', async () => {
    const head = room(1200);
    await settle();
    expect(compact()).toBe(false);
    expect(head.classList.contains('fold-menus')).toBe(false);
  });

  it('first drops the words from its buttons', async () => {
    const head = room(1050);
    await settle();
    expect(compact()).toBe(true);
    expect(head.classList.contains('fold-menus')).toBe(false);
  });

  it('then folds the menus into one button, and takes both back once there is room', async () => {
    let head = room(1000);
    await settle();
    expect(compact()).toBe(true);
    expect(head.classList.contains('fold-menus')).toBe(true);
    head = room(1200);
    await settle();
    expect(head.classList.contains('fold-menus')).toBe(false);
    expect(compact()).toBe(false);
  });
});
