import { createPinia, setActivePinia } from 'pinia';
import { afterEach, beforeAll, beforeEach, describe, expect, it, vi } from 'vitest';
import { createApp, nextTick, type Component } from 'vue';
import { addMessages } from '@/i18n';
import de from './de.json';
import setup from '.';
import Info from './tabs/Info.vue';
import System from './tabs/System.vue';

const { api, settings } = vi.hoisted(() => ({
  api: { app: vi.fn(), system: vi.fn(), checkUpdate: vi.fn(), saveApp: vi.fn(), restart: vi.fn() },
  settings: (autoCheck: boolean) => ({
    office: { kind: 'none', url: '' }, arrowloop: { url: '' }, previews: { thumbs: true, video: false }, webdav: false, terminal: false,
    remotes: { automount: true, cache: 'off', cacheSize: 0 }, updates: { autoCheck, autoUpdate: true }, logLevel: 'info',
  }),
}));
vi.mock('@/api/admin', async (original) => {
  const real = await original<typeof import('@/api/admin')>();
  return { ...real, admin: { ...real.admin, ...api } };
});

const RELEASE = 'https://github.com/junkerderprovinz/cannoncommander/releases/tag/v1.1.0';

beforeAll(() => {
  setActivePinia(createPinia());
  addMessages('config', { de });
  setup();
});

beforeEach(() => {
  vi.clearAllMocks();
  api.system.mockResolvedValue({ version: '1.0.0', release: true, deployment: 'container', go: '', os: '', arch: '', started: 0, uptime: 0, tools: {}, unraid: { available: false, socket: false }, features: {} });
  api.checkUpdate.mockResolvedValue({ version: '1.0.0', latest: '1.1.0', updateAvailable: true, releaseUrl: RELEASE });
  api.saveApp.mockImplementation(async (change: { updates?: { autoCheck: boolean } }) => settings(change.updates?.autoCheck ?? true));
});

let app: ReturnType<typeof createApp> | undefined;
afterEach(() => {
  app?.unmount();
  document.body.innerHTML = '';
});

async function mount(tab: Component) {
  app = createApp(tab);
  app.mount(document.body.appendChild(document.createElement('div')));
  for (let i = 0; i < 6; i++) await nextTick();
}
const row = (label: string) => [...document.querySelectorAll('.glim-field-row')].find((r) => r.querySelector('.glim-field-row-label')?.textContent?.trim() === label);
const toggle = (label: string) => row(label)?.querySelector<HTMLButtonElement>('[role="switch"]');

describe('the updates card', () => {
  it('asks GitHub as the page opens and shows the newer release beside the version', async () => {
    api.app.mockResolvedValue(settings(true));
    await mount(Info);
    expect(api.checkUpdate).toHaveBeenCalledTimes(1);
    const offer = document.querySelector('#a-update');
    expect(offer?.textContent).toContain('1.1.0 ist da');
    expect(offer?.getAttribute('href')).toBe(RELEASE);
  });

  it('leaves GitHub alone while checking automatically is off', async () => {
    api.app.mockResolvedValue(settings(false));
    await mount(Info);
    expect(api.checkUpdate).not.toHaveBeenCalled();
    expect(toggle(de.app.autoCheck)?.getAttribute('aria-checked')).toBe('false');
  });

  it('asks at once when checking automatically is turned on', async () => {
    api.app.mockResolvedValue(settings(false));
    await mount(Info);
    toggle(de.app.autoCheck)!.click();
    for (let i = 0; i < 4; i++) await nextTick();
    expect(api.saveApp).toHaveBeenCalledWith({ updates: { autoCheck: true } });
    expect(api.checkUpdate).toHaveBeenCalledTimes(1);
  });

  it('greys out updating automatically and says how a container is updated', async () => {
    api.app.mockResolvedValue(settings(true));
    await mount(Info);
    const labels = [...document.querySelectorAll('.glim-field-row-label')].map((l) => l.textContent?.trim());
    expect(labels.indexOf(de.app.autoUpdate)).toBe(labels.indexOf(de.app.autoCheck) + 1);
    const updating = toggle(de.app.autoUpdate)!;
    expect(updating.disabled).toBe(true);
    expect(updating.getAttribute('aria-checked')).toBe('false');
    expect(row(de.app.autoUpdate)?.querySelector('.glim-info-icon')?.getAttribute('aria-label')).toBe(de.app.autoUpdateTip);
  });

  it('lets the desktop app update itself', async () => {
    api.system.mockResolvedValue({ version: '1.0.0', release: true, deployment: 'desktop', go: '', os: '', arch: '', started: 0, uptime: 0, tools: {}, unraid: { available: false, socket: false }, features: {} });
    api.app.mockResolvedValue(settings(true));
    await mount(Info);
    const updating = toggle(de.app.autoUpdate)!;
    expect(updating.disabled).toBe(false);
    expect(updating.getAttribute('aria-checked')).toBe('true');
    expect(row(de.app.autoUpdate)?.querySelector('.glim-info-icon')?.getAttribute('aria-label')).toBe(de.app.autoUpdateDesktopTip);
    updating.click();
    for (let i = 0; i < 4; i++) await nextTick();
    expect(api.saveApp).toHaveBeenCalledWith({ updates: { autoUpdate: false } });
  });

  it('offers a restart once the desktop app has the new version in place', async () => {
    api.checkUpdate.mockResolvedValue({ version: '1.0.0', latest: '1.1.0', updateAvailable: true, releaseUrl: RELEASE, ready: '1.1.0' });
    api.app.mockResolvedValue(settings(true));
    await mount(Info);
    const offer = document.querySelector<HTMLButtonElement>('#a-update')!;
    expect(offer.textContent).toContain('Neu starten für 1.1.0');
    offer.click();
    expect(api.restart).toHaveBeenCalledTimes(1);
  });

  it('holds only the two switches, the version stays on the card above', async () => {
    api.app.mockResolvedValue(settings(true));
    await mount(Info);
    const card = [...document.querySelectorAll('.glim-notch-card')].find((c) => c.querySelector('h2')?.textContent?.trim().startsWith('Updates'))!;
    expect([...card.querySelectorAll('.glim-field-row-label')].map((l) => l.textContent?.trim())).toEqual([de.app.autoCheck, de.app.autoUpdate]);
  });

  it('leaves the service line without a call to restart', async () => {
    api.app.mockResolvedValue({ ...settings(false), restartNeeded: true });
    await mount(System);
    expect(document.body.textContent).not.toContain('Neustart nötig');
  });
});
