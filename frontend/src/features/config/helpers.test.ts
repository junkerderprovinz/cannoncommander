import { createPinia, setActivePinia } from 'pinia';
import { afterEach, beforeAll, beforeEach, describe, expect, it, vi } from 'vitest';
import { createApp, nextTick } from 'vue';
import { addMessages } from '@/i18n';
import de from './de.json';
import setup from '.';
import Info from './tabs/Info.vue';

const { api } = vi.hoisted(() => ({
  api: { app: vi.fn(), system: vi.fn(), checkUpdate: vi.fn(), restart: vi.fn(), modules: vi.fn(), fetchModules: vi.fn(), selfUpdate: vi.fn() },
}));
vi.mock('@/api/admin', async (original) => {
  const real = await original<typeof import('@/api/admin')>();
  return { ...real, admin: { ...real.admin, ...api } };
});

const system = (deployment: 'container' | 'desktop', more: object = {}) => ({
  version: '1.4.0', release: true, deployment, go: '', os: 'windows', arch: 'amd64', started: 0, uptime: 0,
  tools: { rclone: { available: true, version: 'v1.75.2' }, ffmpeg: { available: true, version: '9.0.2', path: 'ffmpeg' }, '7zz': { available: false }, mutool: { available: false } },
  unraid: { available: false, socket: false }, features: {}, ...more,
});
const idle = { offered: true, running: false, fraction: 0, ready: false, failed: false };

beforeAll(() => {
  setActivePinia(createPinia());
  addMessages('config', { de });
  setup();
});

beforeEach(() => {
  vi.clearAllMocks();
  api.app.mockRejectedValue(new Error('not needed'));
  api.checkUpdate.mockResolvedValue(null);
  api.selfUpdate.mockResolvedValue({ busy: false, percent: -1 });
});

let app: ReturnType<typeof createApp> | undefined;
afterEach(() => {
  app?.unmount();
  document.body.innerHTML = '';
  vi.useRealTimers();
});

async function mount() {
  app = createApp(Info);
  app.mount(document.body.appendChild(document.createElement('div')));
  for (let i = 0; i < 6; i++) await nextTick();
}
const card = () => document.querySelector('[data-glim="InfoComponentsCard"]')!;
const row = (name: string) => [...card().querySelectorAll('.glim-version-row')].find((r) => r.querySelector('.glim-version-part')?.textContent?.trim() === name);

describe('the Components card', () => {
  it('lists the bundled programs by name, then GlimStone, then the installer, what each is behind its (i)', async () => {
    api.system.mockResolvedValue(system('desktop', { installer: 'v1.2.5' }));
    api.modules.mockResolvedValue(idle);
    await mount();
    expect([...card().querySelectorAll('.glim-version-part')].map((p) => p.textContent)).toEqual(['7-Zip', 'ffmpeg', 'MuPDF', 'rclone', 'GlimStone', 'PommelPorter']);
    expect(row('ffmpeg')?.querySelector('.glim-info-icon')?.getAttribute('aria-label')).toBe(de.info.ffmpegTip);
    expect(row('rclone')?.querySelector('a')?.getAttribute('href')).toBe('https://github.com/rclone/rclone/releases/latest');
    expect(row('ffmpeg')?.querySelector('a')?.getAttribute('href')).toBe('https://ffmpeg.org/download.html#releases');
    expect(card().querySelector('.glim-version-sub')).toBeNull();
    expect(card().querySelector('[role="switch"]')).toBeNull();
  });

  it('offers a download for the missing ones in the desktop app', async () => {
    api.system.mockResolvedValue(system('desktop'));
    api.modules.mockResolvedValue(idle);
    await mount();
    expect(row('ffmpeg')?.textContent).toContain('9.0.2');
    expect(row('ffmpeg')?.querySelector('button')).toBeNull();
    expect(row('7-Zip')?.querySelector('button')?.textContent).toContain('Herunterladen');
    expect(row('MuPDF')?.querySelector('button')?.textContent).toContain('Herunterladen');
  });

  it('offers no download in the container, whose image brings the programs, and names no installer', async () => {
    api.system.mockResolvedValue(system('container'));
    api.modules.mockRejectedValue(new Error('desktopOnly'));
    await mount();
    expect(row('7-Zip')?.textContent).toContain('Nicht gefunden');
    expect(row('7-Zip')?.querySelector('button')).toBeNull();
    expect(row('PommelPorter')).toBeUndefined();
  });

  it('follows the download and then offers the restart that puts the programs to work', async () => {
    vi.useFakeTimers();
    api.system.mockResolvedValue(system('desktop'));
    api.modules.mockResolvedValue(idle);
    await mount();
    api.fetchModules.mockResolvedValue({ ...idle, running: true, fraction: 0.42 });
    row('7-Zip')!.querySelector('button')!.click();
    for (let i = 0; i < 4; i++) await nextTick();
    expect(api.fetchModules).toHaveBeenCalledTimes(1);
    expect(row('7-Zip')?.textContent).toContain('42 % geladen');

    api.modules.mockResolvedValue({ ...idle, fraction: 1, ready: true });
    await vi.advanceTimersByTimeAsync(1000);
    for (let i = 0; i < 4; i++) await nextTick();
    const restart = row('MuPDF')!.querySelector('button')!;
    expect(restart.textContent).toContain('Zum Nutzen neu starten');
    restart.click();
    expect(api.restart).toHaveBeenCalledTimes(1);
  });
});
