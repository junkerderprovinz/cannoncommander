import { createPinia, setActivePinia } from 'pinia';
import { afterEach, beforeAll, describe, expect, it, vi } from 'vitest';
import { createApp, nextTick } from 'vue';
import { go, settingsTabs, view } from '@/core/views';
import { addMessages } from '@/i18n';
import de from './de.json';
import { NEWS } from './help';
import setup from '.';

const { APP, SYS } = vi.hoisted(() => ({
  APP: {
    office: { kind: 'none', url: '' }, arrowloop: { url: '' }, previews: { thumbs: true, video: false }, webdav: false, terminal: false,
    remotes: { automount: true, cache: 'off', cacheSize: 0 }, updates: { autoCheck: true }, logLevel: 'info',
  },
  SYS: { version: '1.0.0', go: '', os: '', arch: '', started: 0, uptime: 0, tools: {}, unraid: { available: false, socket: false }, features: {} },
}));
vi.mock('@/api/admin', async (original) => {
  const real = await original<typeof import('@/api/admin')>();
  return { ...real, admin: { ...real.admin, app: async () => structuredClone(APP), system: async () => structuredClone(SYS) } };
});

beforeAll(() => {
  setActivePinia(createPinia());
  // The README buttons fit their words to their width, which jsdom does not measure.
  vi.stubGlobal('ResizeObserver', class { observe() {} disconnect() {} });
  addMessages('config', { de });
  // The address the sign in provider sends the person back to, for the token window.
  history.replaceState(null, '', '/?again=token');
  setup();
});

let app: ReturnType<typeof createApp> | undefined;
afterEach(() => {
  app?.unmount();
  document.body.innerHTML = '';
});

const tile = (id: string) => settingsTabs.find((t) => t.id === id);
const label = (id: string) => {
  const l = tile(id)!.label;
  return typeof l === 'function' ? l() : l;
};
async function cardsOf(id: string) {
  app = createApp(tile(id)!.component);
  app.mount(document.body.appendChild(document.createElement('div')));
  for (let i = 0; i < 5; i++) await nextTick();
  return [...document.querySelectorAll('.glim-notch-card > h2')].map((h) => h.textContent?.trim() ?? '');
}

describe('the settings', () => {
  it('start with Allgemein and Aussehen', () => {
    expect(settingsTabs.slice(0, 2).map((t) => t.id)).toEqual(['general', 'look']);
    expect(['general', 'look'].map(label)).toEqual(['Allgemein', 'Aussehen']);
  });

  it('put the account third and end on integrations, apps, system and info, as in BombVault', () => {
    expect(settingsTabs.map((t) => t.id)).toEqual(['general', 'look', 'security', 'files', 'sources', 'remotes', 'users', 'rules', 'shares', 'integrations', 'app', 'system', 'info']);
  });

  it('lead to the account after signing in again and from the news', () => {
    expect(view).toMatchObject({ id: 'settings', arg: 'security' });
    go('files');
    NEWS.find((n) => n.id === 'security')!.go();
    expect(view).toMatchObject({ id: 'settings', arg: 'security' });
  });

  it('name the tiles the other apps share as they do', () => {
    expect(label('app')).toBe('Apps');
    expect(tile('app')!.g).toBe('IconTabApp');
    expect(label('system')).toBe('System');
    expect(tile('system')!.g).toBe('IconTabSystem');
    expect(label('integrations')).toBe('Anbindungen');
    expect(label('info')).toBe('Info');
  });

  it('offer the desktop and phone builds on the apps tile, each still to come', async () => {
    const cards = await cardsOf('app');
    expect(cards.map((t) => t.split(/\s/)[0])).toEqual(['Desktop-App', 'Handy-App']);
    const units = [...document.querySelectorAll('.glim-readme-btn-unit')];
    expect(units.map((u) => u.querySelector('.glim-readme-btn-name')?.textContent)).toEqual(['Windows', 'macOS', 'Linux', 'Google Play', 'F-Droid', 'Android']);
    for (const u of units) {
      expect(u.classList).toContain('glim-readme-btn-soon');
      expect(u.querySelector('.glim-readme-btn-sub')?.textContent).toBe('Bald');
    }
    expect(document.querySelectorAll('.glim-readme-btn-unit a, .glim-readme-btn-unit button')).toHaveLength(0);
  });

  it('keep the settings file and the service on the system tile', async () => {
    expect(await cardsOf('system')).toEqual(['Einstellungen sichern', 'Dienst']);
  });

  it('put the updates on the info tile, after the version', async () => {
    const cards = await cardsOf('info');
    expect(cards.map((t) => t.split(/\s/)[0])).toEqual(['Version', 'Updates', 'Hilfe', 'Über']);
    // One button looks for a newer version, the one beside the version numbers.
    expect(document.querySelectorAll('#a-update, #a-upd')).toHaveLength(1);
  });
});
