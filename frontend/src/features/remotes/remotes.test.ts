import { createPinia, setActivePinia } from 'pinia';
import { afterEach, beforeAll, describe, expect, it, vi } from 'vitest';
import { createApp, nextTick } from 'vue';
import type { Remote } from '@/api/admin';
import { host } from '@/core/host';
import { addMessages } from '@/i18n';
import { usePanels } from '@/stores/panels';
import { useUser } from '@/stores/user';
import de from './de.json';
import RemotesCard from './RemotesCard.vue';
import { reloadRemotes } from './store';

const { LIST, saved } = vi.hoisted(() => ({
  LIST: [] as Remote[],
  saved: vi.fn(async () => {}),
}));
vi.mock('@/api/admin', async (original) => {
  const real = await original<typeof import('@/api/admin')>();
  return { ...real, admin: { ...real.admin, remotes: async () => ({ remotes: structuredClone(LIST), rclone: { installed: true, running: true } }) } };
});
vi.mock('@/api/client', async (original) => {
  const real = await original<typeof import('@/api/client')>();
  return { ...real, api: { ...real.api, saveBookmarks: saved } };
});

const remote = (name: string, mounted: boolean, error?: string): Remote => ({
  name, type: 'webdav', path: '', mount: '/remotes/' + name, label: 'rclone: ' + name, mounted, mountOnStart: mounted, error,
});

beforeAll(() => {
  setActivePinia(createPinia());
  addMessages('remotes', { de });
  usePanels();
});

let app: ReturnType<typeof createApp> | undefined;
afterEach(() => {
  app?.unmount();
  document.body.innerHTML = '';
  Object.assign(host, { desktop: false });
});

async function card() {
  await reloadRemotes();
  app = createApp(RemotesCard);
  app.use(createPinia());
  app.mount(document.body.appendChild(document.createElement('div')));
  await nextTick();
  return [...document.querySelectorAll<HTMLElement>('[data-remote]')];
}

describe('the remotes in the sidebar', () => {
  it('show each remote with its state', async () => {
    LIST.splice(0, LIST.length, remote('cloud', true), remote('nas', false), remote('box', false, 'no such host'));
    const rows = await card();
    expect(rows.map((r) => r.dataset.remote)).toEqual(['cloud', 'nas', 'box']);
    expect([...document.querySelectorAll('#remotes .meta')].map((m) => m.textContent)).toEqual(['Ausgehängt', 'Fehler']);
    expect(rows[1]!.closest('.unmounted')).not.toBeNull();
    expect(rows[0]!.closest('.unmounted')).toBeNull();
  });

  it('pin a remote to quick access in one click and take it out again', async () => {
    LIST.splice(0, LIST.length, remote('cloud', true));
    await card();
    const user = useUser();
    user.bookmarks = [];
    document.querySelector<HTMLButtonElement>('[data-pin="cloud"]')!.click();
    expect(user.bookmarks).toEqual([{ name: 'cloud', path: '/remotes/cloud', g: 'IconCloud' }]);
    expect(saved).toHaveBeenCalledTimes(1);
    await nextTick();
    expect(document.querySelector('[data-pin="cloud"]')!.getAttribute('aria-pressed')).toBe('true');
    document.querySelector<HTMLButtonElement>('[data-pin="cloud"]')!.click();
    expect(user.bookmarks).toEqual([]);
  });

  it('stay out of the desktop app, which mounts nothing', async () => {
    LIST.splice(0, LIST.length, remote('cloud', true));
    Object.assign(host, { desktop: true });
    expect(await card()).toEqual([]);
  });
});
