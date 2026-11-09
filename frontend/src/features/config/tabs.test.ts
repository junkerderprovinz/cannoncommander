import { createPinia, setActivePinia } from 'pinia';
import { afterEach, beforeAll, describe, expect, it, vi } from 'vitest';
import { createApp, nextTick } from 'vue';
import { host } from '@/core/host';
import { actions } from '@/core/actions';
import { go, settingsTabs, view } from '@/core/views';
import { addMessages } from '@/i18n';
import de from './de.json';
import setupKeys from '@/features/keys';
import setupShellMenu from '@/features/shellmenu';
import { NEWS } from './help';
import setup from '.';

const { APP, SYS } = vi.hoisted(() => ({
  APP: {
    office: { kind: 'none', url: '' }, arrowloop: { url: '' }, previews: { thumbs: true, video: false }, webdav: false, terminal: false,
    remotes: { automount: true, cache: 'off', cacheSize: 0 }, updates: { autoCheck: true }, logLevel: 'info',
  },
  SYS: { version: '1.0.0', release: false, deployment: 'container', go: '', os: '', arch: '', started: 0, uptime: 0, tools: {}, unraid: { available: false, socket: false }, features: {} },
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
  // The context menu's tile comes first here, so the order cannot rest on which feature loads first.
  setupShellMenu();
  setupKeys();
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
    expect(settingsTabs.map((t) => t.id)).toEqual(['general', 'look', 'security', 'files', 'keys', 'ctxmenu', 'sources', 'remotes', 'users', 'rules', 'shares', 'integrations', 'app', 'system', 'info']);
  });

  it('leave the remotes out of the desktop app, which has nothing to mount them with', () => {
    expect(tile('remotes')!.hidden?.()).toBe(false);
    expect(actions.mounts!.when?.()).toBe(true);
    Object.assign(host, { desktop: true });
    try {
      expect(tile('remotes')!.hidden?.()).toBe(true);
      expect(actions.mounts!.when?.()).toBe(false);
    } finally {
      Object.assign(host, { desktop: false });
    }
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
    expect(units.map((u) => u.querySelector('.glim-readme-btn-name')?.textContent)).toEqual(['Windows', 'macOS', 'Linux', 'Flatpak', 'Google Play', 'F-Droid', 'Android']);
    for (const u of units) {
      expect(u.classList).toContain('glim-readme-btn-soon');
      expect(u.querySelector('.glim-readme-btn-sub')?.textContent).toBe('Bald');
    }
    expect(document.querySelectorAll('.glim-readme-btn-unit a, .glim-readme-btn-unit button')).toHaveLength(0);
  });

  it('offer a release build the desktop app of its own release', async () => {
    Object.assign(SYS, { release: true, version: '1.2.0' });
    try {
      await cardsOf('app');
      const links = [...document.querySelectorAll<HTMLAnchorElement>('.glim-readme-btn-unit a')].map((a) => a.getAttribute('href'));
      const at = 'https://github.com/junkerderprovinz/cannoncommander/releases/download/v1.2.0/cannoncommander-';
      expect(links).toEqual(['windows-amd64-installer.exe', 'windows-amd64.zip', 'macos-universal.zip', 'linux-amd64-installer', 'linux-arm64-installer',
        'linux-amd64.flatpak', 'linux-arm64.flatpak'].map((f) => at + f));
      const flatpak = [...document.querySelectorAll('.glim-readme-btn-unit')].find((u) => u.querySelector('.glim-readme-btn-name')?.textContent === 'Flatpak')!;
      expect(flatpak.className).toBe('glim-readme-btn-unit group glim-readme-btn-group glim-tile-flatpak');
      expect([...flatpak.querySelectorAll('a')].map((a) => a.getAttribute('aria-label'))).toEqual(['Flatpak x64', 'Flatpak ARM64']);
      expect(document.querySelector('.glim-notch-card')?.textContent).not.toContain('Sobald es die Builds gibt');
    } finally {
      Object.assign(SYS, { release: false, version: '1.0.0' });
    }
  });

  it('offer the server inside the desktop app instead of the desktop app', async () => {
    SYS.deployment = 'desktop';
    try {
      const cards = await cardsOf('app');
      expect(cards.map((t) => t.split(/\s/)[0])).toEqual(['Server', 'Handy-App']);
      const names = [...document.querySelectorAll('.glim-readme-btn-unit')].slice(0, 3).map((u) => u.querySelector('.glim-readme-btn-name')?.textContent);
      expect(names).toEqual(['Unraid', 'Docker', 'Quellcode']);
    } finally {
      SYS.deployment = 'container';
    }
  });

  it('show how the list looks under Aussehen, as Explorer does under its view, and keep the bars on the file list tile', async () => {
    expect(await cardsOf('look')).toEqual(['Darstellung', 'Liste', 'Beschriftungen', 'Farben']);
    app!.unmount();
    document.body.innerHTML = '';
    expect(await cardsOf('files')).toEqual(['Leisten', 'Aktualisieren']);
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
