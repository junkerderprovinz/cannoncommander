import { afterEach, beforeAll, beforeEach, describe, expect, it, vi } from 'vitest';
import { createApp, nextTick } from 'vue';
import type { Remote, RemoteCatalog, RemoteTest } from '@/api/admin';
import { addMessages, setLanguage } from '@/i18n';
import en from '../en.json';
import { loadMarks } from '../remotes';
import Remotes from './Remotes.vue';

const CATALOG: RemoteCatalog = {
  types: [{ id: 'webdav', description: 'WebDAV', featured: true, oauth: false, pathHint: 'folder', options: [] }, { id: 'alias', description: 'Alias', featured: false, oauth: false, pathHint: 'folder', options: [] }],
  providers: [{ id: 'nextcloud', backend: 'webdav', preset: { vendor: 'nextcloud' } }, { id: 'alias', backend: 'alias' }],
  unlisted: [],
};
const remote = (name: string, more: Partial<Remote>): Remote => ({
  name, type: 'webdav', path: '', mount: `/remotes/${name}`, label: '', mounted: true, mountOnStart: true, params: {}, ...more,
});

type Sent = { method: string; path: string; body: Record<string, unknown> };
let sent: Sent[] = [];
let remotes: Remote[] = [];
let tested: RemoteTest;
let answerTest: Promise<void>;

function answer(status: number, body?: unknown) {
  return new Response(body === undefined ? null : JSON.stringify(body), { status, headers: { 'Content-Type': 'application/json' } });
}
async function server(url: string, init: RequestInit = {}) {
  const path = new URL(url, 'http://cc.test').pathname.replace('/api/v1', ''), method = init.method ?? 'GET';
  const body = init.body ? JSON.parse(String(init.body)) : {};
  if (method !== 'GET') sent.push({ method, path, body });
  if (path === '/remotes' && method === 'GET') return answer(200, { remotes, rclone: { installed: true, running: true } });
  if (path === '/remotes/types') return answer(200, CATALOG);
  if (path === '/admin/app') return answer(200, { remotes: { automount: true, cache: 'writes', cacheSize: 50 } });
  if (path === '/remotes/test') {
    await answerTest;
    return answer(200, tested);
  }
  const mount = /^\/remotes\/([^/]+)\/(mount|unmount)$/.exec(path);
  if (mount) {
    remotes = remotes.map((r) => (r.name === mount[1] ? { ...r, mounted: mount[2] === 'mount' } : r));
    return answer(200, remotes.find((r) => r.name === mount[1]));
  }
  return answer(204);
}

beforeAll(async () => {
  addMessages('config', { en });
  setLanguage('en');
  await loadMarks();
});
beforeEach(() => {
  sent = [];
  remotes = [
    remote('nextcloud', { provider: 'nextcloud', params: { url: 'https://cloud.example', vendor: 'nextcloud' }, secrets: ['pass'] }),
    remote('backups', { type: 'alias', provider: 'alias', mounted: false, params: { remote: '/mnt/user/backups' } }),
  ];
  tested = { ok: true, entries: 3, free: 2 * 1024 ** 3 };
  answerTest = Promise.resolve();
  vi.stubGlobal('fetch', vi.fn(server));
  vi.stubGlobal('ResizeObserver', class { observe() {} disconnect() {} });
});

let app: ReturnType<typeof createApp> | undefined;
afterEach(() => {
  app?.unmount();
  document.body.innerHTML = '';
});

const settle = async () => {
  for (let i = 0; i < 8; i++) {
    await new Promise((r) => setTimeout(r, 0));
    await nextTick();
  }
};
async function open() {
  app = createApp(Remotes);
  app.mount(document.body.appendChild(document.createElement('div')));
  await settle();
}
const row = (name: string) => document.querySelector<HTMLElement>(`[data-glim="ConnectionRow"][aria-label="${name}"]`)!;
const button = (name: string, which: string) => row(name).querySelector<HTMLButtonElement>(`[data-connection="${which}"]`)!;
const mountButton = (name: string) => row(name).closest('.a-conn')!.querySelector<HTMLButtonElement>('[data-mount]')!;
// The button's title shows in GlimStone's bubble while it has the focus or the pointer.
async function bubble(el: HTMLElement) {
  el.focus();
  el.dispatchEvent(new Event('pointerenter'));
  el.dispatchEvent(new Event('mouseenter'));
  await settle();
  return document.body.textContent ?? '';
}
const click = async (el: Element) => {
  (el as HTMLElement).click();
  await settle();
};

describe('the list of remotes', () => {
  it('shows each remote as a connection row with its logo, what it is and where it is mounted', async () => {
    await open();
    expect([...document.querySelectorAll('[data-glim="ConnectionRow"]')].map((r) => r.getAttribute('aria-label'))).toEqual(['nextcloud', 'backups']);
    expect(row('nextcloud').querySelector('.glim-connection-status')?.textContent).toBe('Nextcloud');
    expect(row('nextcloud').querySelector('.a-conn-path')?.textContent).toBe('/remotes/nextcloud');
    expect(row('nextcloud').querySelector('.glim-connection-tile svg')).not.toBeNull();
    expect(row('backups').querySelector('.glim-connection-status')?.textContent).toBe('Alias');
  });

  it('marks a remote that is not mounted or failed to mount with a badge', async () => {
    remotes[0] = { ...remotes[0]!, error: 'connection refused' };
    await open();
    expect(row('nextcloud').querySelector('.glim-connection-badges')?.textContent).toContain('Error');
    expect(row('backups').querySelector('.glim-connection-badges')?.textContent).toContain('Unmounted');
  });

  it('tests a saved remote under its own name and says in the button how it went', async () => {
    await open();
    let go = () => {};
    answerTest = new Promise((r) => { go = r; });
    button('nextcloud', 'test').click();
    await settle();
    expect(button('nextcloud', 'test').getAttribute('aria-busy')).toBe('true');
    expect(sent.at(-1)).toMatchObject({ path: '/remotes/test', body: { name: 'nextcloud', type: 'webdav', params: { url: 'https://cloud.example', vendor: 'nextcloud' } } });
    go();
    await settle();
    expect(button('nextcloud', 'test').classList).toContain('glim-btn--ok');
    expect(await bubble(button('nextcloud', 'test'))).toContain('Connected, 2 GB free');
  });

  it('turns the test red with what went wrong', async () => {
    tested = { ok: false, entries: 0, failure: 'timeout' };
    await open();
    await click(button('nextcloud', 'test'));
    expect(button('nextcloud', 'test').classList).toContain('glim-btn--danger');
    expect(await bubble(button('nextcloud', 'test'))).toContain('The remote didn\'t answer in time');
  });

  it('mounts and unmounts beside the row', async () => {
    await open();
    expect(mountButton('nextcloud').textContent?.trim()).toBe('Unmount');
    expect(mountButton('backups').textContent?.trim()).toBe('Mount');
    await click(mountButton('nextcloud'));
    expect(sent.at(-1)).toMatchObject({ method: 'POST', path: '/remotes/nextcloud/unmount' });
    expect(mountButton('nextcloud').textContent?.trim()).toBe('Mount');
  });

  it('opens a remote to edit from its row', async () => {
    await open();
    await click(button('backups', 'edit'));
    expect([...document.querySelectorAll('[data-glim-title]')].map((w) => w.getAttribute('data-glim-title'))).toContain('Edit remote');
  });
});
