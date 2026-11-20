import { afterEach, beforeAll, beforeEach, describe, expect, it } from 'vitest';
import { createApp, h, nextTick } from 'vue';
import { action, actions } from '@/core/actions';
import { host } from '@/core/host';
import { MENUS } from '@/core/menus';
import { firstVisit, mergeServerPrefs, PREFS_KEY, prefs } from '@/core/prefs';
import { winStack } from '@/core/windows';
import { addMessages, setLanguage } from '@/i18n';
import desktopDe from '@/features/desktop/de.json';
import desktopEn from '@/features/desktop/en.json';
import keysDe from '@/features/keys/de.json';
import keysEn from '@/features/keys/en.json';
import de from './de.json';
import en from './en.json';
import setup, { openWelcome } from '.';
import { welcomeDue, welcomeLogo } from './welcome';
import WelcomeBody from './WelcomeBody.vue';

let app: ReturnType<typeof createApp> | undefined;
let closed = 0;
const settle = async () => { for (let i = 0; i < 3; i++) await nextTick(); };
const card = (way: string) => document.querySelector<HTMLButtonElement>(`[data-way="${way}"]`)!;
const checked = () => [...document.querySelectorAll('[role="radio"][aria-checked="true"]')].map((c) => c.getAttribute('data-way'));
const kept = () => JSON.parse(localStorage.getItem(PREFS_KEY)!);

async function show() {
  app = createApp({ render: () => h(WelcomeBody, { close: () => closed++ }) });
  app.mount(document.body.appendChild(document.createElement('div')));
  await settle();
}

beforeAll(() => {
  addMessages('welcome', { de, en });
  addMessages('desktop', { de: desktopDe, en: desktopEn });
  addMessages('keys', { de: keysDe, en: keysEn });
  setLanguage('de');
  action('sidePane', { label: 'Vorschaubereich', run: () => {} });
});
beforeEach(() => {
  closed = 0;
  Object.assign(prefs, { desktopStyle: '', dual: true, clickSelect: false, keys: null, welcomed: undefined });
});
afterEach(() => {
  app?.unmount();
  app = undefined;
  document.body.innerHTML = '';
  while (winStack.length) winStack.at(-1)!.close();
});

describe('in the desktop app', () => {
  beforeEach(() => { host.desktop = true; });

  it('offers the explorer way and the commander, with the commander picked as it ships', async () => {
    await show();
    expect([...document.querySelectorAll('.wl-name')].map((n) => n.textContent)).toEqual(['Wie im Explorer', 'Wie ein Commander']);
    expect(checked()).toEqual(['commander']);
  });

  it('shows one panel the moment the explorer way is picked, and keeps it for the next start', async () => {
    await show();
    card('explorer').click();
    await settle();
    expect(prefs).toMatchObject({ desktopStyle: 'explorer', dual: false, clickSelect: true });
    expect((prefs.keys as { preset: string }).preset).toBe('explorer');
    expect(actions.sidePane!.key).toBe('Alt+P');
    expect(kept()).toMatchObject({ desktopStyle: 'explorer', dual: false });
    expect(checked()).toEqual(['explorer']);
  });

  it('brings both panels back with the commander', async () => {
    await show();
    card('explorer').click();
    card('commander').click();
    expect(prefs).toMatchObject({ desktopStyle: 'commander', dual: true, clickSelect: false, keys: null });
  });

  it('stays a commander when the window is closed without a pick', () => {
    openWelcome();
    winStack.at(-1)!.close();
    expect(prefs).toMatchObject({ desktopStyle: 'commander', dual: true, welcomed: true });
  });
});

describe('in the browser', () => {
  beforeEach(() => { host.desktop = false; });

  it('offers one panel or two and leaves the keys as they are', async () => {
    await show();
    expect([...document.querySelectorAll('.wl-name')].map((n) => n.textContent)).toEqual(['Ein Fenster', 'Zwei Fenster']);
    card('single').click();
    expect(prefs).toMatchObject({ dual: false, desktopStyle: '', clickSelect: false, keys: null });
    expect(kept().dual).toBe(false);
  });

  it('picks with the arrow keys and closes on Enter', async () => {
    await show();
    card('dual').focus();
    card('dual').dispatchEvent(new KeyboardEvent('keydown', { key: 'ArrowRight', bubbles: true }));
    await settle();
    expect(checked()).toEqual(['single']);
    expect(prefs.dual).toBe(false);
    expect(document.activeElement).toBe(card('single'));
    expect(card('single').tabIndex).toBe(0);
    expect(card('dual').tabIndex).toBe(-1);
    card('single').dispatchEvent(new KeyboardEvent('keydown', { key: 'ArrowLeft', bubbles: true }));
    card('dual').dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter', bubbles: true }));
    expect(prefs.dual).toBe(true);
    expect(closed).toBe(1);
  });
});

describe('the welcome window', () => {
  it('greets a first visit until it has been closed once', () => {
    expect(firstVisit()).toBe(true);
    expect(welcomeDue()).toBe(true);
    prefs.welcomed = true;
    expect(welcomeDue()).toBe(false);
  });

  it('leaves somebody alone whose settings the server already keeps', () => {
    mergeServerPrefs({ theme: 'dark' });
    expect(welcomeDue()).toBe(false);
  });

  it('opens again from the help menu, once at a time', () => {
    setup();
    expect(MENUS.find(([name]) => name === 'help')![1]).toContain('welcome');
    actions.welcome!.run();
    actions.welcome!.run();
    expect(winStack.map((w) => w.title)).toEqual(['Willkommen bei CannonCommander']);
  });

  it('draws the master logo with its two cannons and a puff of smoke at each muzzle, sharing nothing with the page', () => {
    const svg = new DOMParser().parseFromString(welcomeLogo(), 'text/html').querySelector('svg')!;
    expect(svg.querySelectorAll('.wl-cannon--high, .wl-cannon--low')).toHaveLength(2);
    expect(svg.querySelectorAll('.wl-puff')).toHaveLength(2);
    expect(svg.querySelector('style, [id]')).toBeNull();
    expect(svg.getAttribute('aria-hidden')).toBe('true');
  });
});
