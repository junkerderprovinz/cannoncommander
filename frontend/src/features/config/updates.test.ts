import { componentText } from '@glimstone/core';
import { createPinia, setActivePinia } from 'pinia';
import { afterEach, beforeAll, beforeEach, describe, expect, it, vi } from 'vitest';
import { createApp, nextTick, type Component } from 'vue';
import { addMessages, language } from '@/i18n';
import de from './de.json';
import setup from '.';
import Info from './tabs/Info.vue';
import System from './tabs/System.vue';

const { api, settings, system } = vi.hoisted(() => ({
  api: { app: vi.fn(), system: vi.fn(), checkUpdate: vi.fn(), saveApp: vi.fn(), restart: vi.fn(), selfUpdate: vi.fn(), updateNow: vi.fn(), modules: vi.fn() },
  settings: (autoCheck: boolean, autoUpdate = true) => ({
    office: { kind: 'none', url: '' }, arrowloop: { url: '' }, previews: { thumbs: true, video: false }, webdav: false, terminal: false,
    remotes: { automount: true, cache: 'off', cacheSize: 0 }, updates: { autoCheck, autoUpdate }, logLevel: 'info',
  }),
  system: (deployment: 'container' | 'desktop', more: object = {}) => ({
    version: '1.0.0', release: true, deployment, go: '', os: '', arch: '', started: 0, uptime: 0, tools: {}, unraid: { available: false, socket: false }, features: {}, ...more,
  }),
}));
vi.mock('@/api/admin', async (original) => {
  const real = await original<typeof import('@/api/admin')>();
  return { ...real, admin: { ...real.admin, ...api } };
});

beforeAll(() => {
  setActivePinia(createPinia());
  addMessages('config', { de });
  setup();
});

beforeEach(() => {
  vi.clearAllMocks();
  api.system.mockResolvedValue(system('container'));
  api.checkUpdate.mockResolvedValue({ version: '1.0.0', latest: '1.1.0', updateAvailable: true });
  api.saveApp.mockImplementation(async (change: { updates?: { autoCheck: boolean } }) => settings(change.updates?.autoCheck ?? true));
  api.selfUpdate.mockResolvedValue({ busy: false, percent: -1 });
  api.modules.mockRejectedValue(new Error('desktopOnly'));
});

let app: ReturnType<typeof createApp> | undefined;
afterEach(() => {
  app?.unmount();
  document.body.innerHTML = '';
  vi.useRealTimers();
});

async function mount(tab: Component) {
  app = createApp(tab);
  app.mount(document.body.appendChild(document.createElement('div')));
  for (let i = 0; i < 8; i++) await nextTick();
}
const words = () => componentText(language()).versionCard;
const card = () => document.querySelector('[data-glim="InfoVersionCard"]')!;
const button = (label: string) => [...card().querySelectorAll('button')].find((b) => (b.getAttribute('aria-label') ?? b.textContent)?.includes(label));
const toggle = () => card().querySelector<HTMLButtonElement>('[role="switch"]');

describe('the Version card', () => {
  it('asks GitHub as the page opens and leads a container to the guide for the newer release', async () => {
    api.app.mockResolvedValue(settings(true));
    await mount(Info);
    expect(api.checkUpdate).toHaveBeenCalledTimes(1);
    expect(card().textContent).toContain(words().available('1.1.0'));
    expect(button(words().guide)).toBeTruthy();
    expect(button(words().check)).toBeUndefined();
  });

  it('leaves GitHub alone while checking automatically is off', async () => {
    api.app.mockResolvedValue(settings(false));
    await mount(Info);
    expect(api.checkUpdate).not.toHaveBeenCalled();
    expect(toggle()?.getAttribute('aria-checked')).toBe('false');
    expect(card().textContent).toContain(words().autoCheck);
  });

  it('asks at once when checking automatically is turned on', async () => {
    api.app.mockResolvedValue(settings(false));
    await mount(Info);
    toggle()!.click();
    for (let i = 0; i < 4; i++) await nextTick();
    expect(api.saveApp).toHaveBeenCalledWith({ updates: { autoCheck: true } });
    expect(api.checkUpdate).toHaveBeenCalledTimes(1);
  });

  it('says why a check by hand failed', async () => {
    api.app.mockResolvedValue(settings(false));
    api.checkUpdate.mockResolvedValue(null);
    await mount(Info);
    button(words().check)!.click();
    for (let i = 0; i < 4; i++) await nextTick();
    expect(card().textContent).toContain(words().failed);
    expect(card().textContent).toContain(de.app.noAnswer);
  });

  it('lets the desktop app update itself with its one switch', async () => {
    api.system.mockResolvedValue(system('desktop'));
    api.app.mockResolvedValue(settings(true));
    await mount(Info);
    expect(card().textContent).toContain(words().auto);
    expect(toggle()?.getAttribute('aria-checked')).toBe('true');
    toggle()!.click();
    for (let i = 0; i < 4; i++) await nextTick();
    expect(api.saveApp).toHaveBeenCalledWith({ updates: { autoUpdate: false } });
  });

  it('updates the desktop app now, follows the download and restarts into it', async () => {
    vi.useFakeTimers();
    api.system.mockResolvedValue(system('desktop'));
    api.app.mockResolvedValue(settings(false, false));
    api.updateNow.mockResolvedValue({ busy: true, percent: 0 });
    await mount(Info);
    button(words().check)!.click();
    for (let i = 0; i < 4; i++) await nextTick();
    button(words().update)!.click();
    for (let i = 0; i < 4; i++) await nextTick();
    expect(api.updateNow).toHaveBeenCalledTimes(1);

    api.selfUpdate.mockResolvedValue({ busy: true, percent: 42 });
    await vi.advanceTimersByTimeAsync(1000);
    expect(card().querySelector('[role="progressbar"]')?.getAttribute('aria-valuenow')).toBe('42');

    api.selfUpdate.mockResolvedValue({ busy: false, percent: 100, ready: '1.1.0' });
    await vi.advanceTimersByTimeAsync(1000);
    for (let i = 0; i < 4; i++) await nextTick();
    button(words().restart('1.1.0'))!.click();
    expect(api.restart).toHaveBeenCalledTimes(1);
  });

  it('leaves the updates of the Flatpak to the software centre and offers no switch', async () => {
    api.system.mockResolvedValue(system('desktop', { flatpak: true }));
    api.app.mockResolvedValue(settings(true));
    await mount(Info);
    expect(card().textContent).toContain(words().store('Flathub'));
    expect(toggle()).toBeNull();
    expect(button(words().check)).toBeUndefined();
  });

  it('leaves the service line without a call to restart', async () => {
    api.app.mockResolvedValue({ ...settings(false), restartNeeded: true });
    await mount(System);
    expect(document.body.textContent).not.toContain('Neustart nötig');
  });
});
