import { createPinia, setActivePinia } from 'pinia';
import { afterEach, beforeAll, describe, expect, it, vi } from 'vitest';
import { createApp, nextTick } from 'vue';
import { settingsTabs } from '@/core/views';
import { addMessages } from '@/i18n';
import de from './de.json';
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
  addMessages('config', { de });
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
  it('open on the account and its security', () => {
    expect(settingsTabs[0]?.id).toBe('security');
    expect(settingsTabs.filter((t) => t.id === 'security')).toHaveLength(1);
  });

  it('end on integrations, system and info, as in BombVault, with no tile of their own for the app', () => {
    expect(settingsTabs.map((t) => t.id)).toEqual(['security', 'general', 'look', 'files', 'sources', 'remotes', 'users', 'rules', 'shares', 'integrations', 'system', 'info']);
  });

  it('name the tiles the other apps share as they do', () => {
    expect(label('system')).toBe('System');
    expect(tile('system')!.g).toBe('IconTabSystem');
    expect(label('integrations')).toBe('Anbindungen');
    expect(label('info')).toBe('Info');
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
