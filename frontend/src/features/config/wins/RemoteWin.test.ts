import { readFileSync } from 'node:fs';
import { join } from 'node:path';
import { afterEach, beforeAll, beforeEach, describe, expect, it, vi } from 'vitest';
import { createApp, h, nextTick } from 'vue';
import type { Remote, RemoteCatalog, RemoteTest } from '@/api/admin';
import { toasts } from '@/core/toast';
import { addMessages, setLanguage } from '@/i18n';
import { withoutLayers } from '@/testing/layers';
import en from '../en.json';
import { loadMarks } from '../remotes';
import RemoteWin from './RemoteWin.vue';

const option = (name: string, more: Record<string, unknown> = {}) => ({
  name, help: name, type: 'string', required: false, essential: true, secret: false, advanced: false, exclusive: false, ...more,
});
const CATALOG: RemoteCatalog = {
  types: [
    { id: 'webdav', description: 'WebDAV', featured: true, oauth: false, pathHint: 'folder', options: [option('url', { required: true }), option('vendor'), option('user')] },
    { id: 's3', description: 'S3', featured: true, oauth: false, pathHint: 'bucket', options: [option('access_key_id', { required: true })] },
    { id: 'smb', description: 'SMB', featured: true, oauth: false, pathHint: 'share', options: [option('host', { required: true })] },
    { id: 'quatrix', description: 'Quatrix', featured: false, oauth: false, pathHint: 'folder', options: [option('api_key', { required: true })] },
    { id: 'crypt', description: 'Encrypt a remote', featured: false, oauth: false, pathHint: 'folder', options: [option('remote', { required: true })] },
  ],
  providers: [
    { id: 'nextcloud', backend: 'webdav', preset: { vendor: 'nextcloud' } },
    { id: 's3', backend: 's3' },
    { id: 'minio', backend: 's3', preset: { provider: 'Minio' } },
    { id: 'smb', backend: 'smb' },
    { id: 'webdav', backend: 'webdav' },
    { id: 'crypt', backend: 'crypt' },
  ],
  unlisted: ['quatrix'],
};

type Sent = { method: string; path: string; body: Record<string, unknown> };
let sent: Sent[] = [];
let tested: RemoteTest;
let answerTest: Promise<void>;

function answer(status: number, body?: unknown) {
  return new Response(body === undefined ? null : JSON.stringify(body), { status, headers: { 'Content-Type': status >= 400 ? 'application/problem+json' : 'application/json' } });
}
async function server(url: string, init: RequestInit = {}) {
  const u = new URL(url, 'http://cc.test');
  const path = u.pathname.replace('/api/v1', ''), method = init.method ?? 'GET';
  const body = init.body ? JSON.parse(String(init.body)) : {};
  if (method !== 'GET') sent.push({ method, path, body });
  if (path === '/remotes/types') return answer(200, CATALOG);
  if (path === '/remotes/test') {
    await answerTest;
    return answer(200, tested);
  }
  if (path === '/remotes' && method === 'POST') return answer(201, { ...body, label: '', mounted: true, mountOnStart: true });
  if (path.startsWith('/remotes/') && method === 'PUT') return answer(200, { ...body, label: '', mounted: true, mountOnStart: true });
  return answer(204);
}

beforeAll(async () => {
  addMessages('config', { en });
  setLanguage('en');
  // The logos load with the first window, more slowly than a test waits for it.
  await loadMarks();
});
beforeEach(() => {
  sent = [];
  tested = { ok: true, entries: 3, free: 2 * 1024 ** 3 };
  answerTest = Promise.resolve();
  vi.stubGlobal('fetch', vi.fn(server));
});

let app: ReturnType<typeof createApp> | undefined;
afterEach(() => {
  app?.unmount();
  toasts.splice(0);
  document.body.innerHTML = '';
});

const settle = async () => {
  for (let i = 0; i < 8; i++) {
    await new Promise((r) => setTimeout(r, 0));
    await nextTick();
  }
};
async function open(remote: Remote | null = null, all: Remote[] = []) {
  const events = { saved: vi.fn(), close: vi.fn() };
  app = createApp({ render: () => h(RemoteWin, { remote, all, onSaved: events.saved, onClose: events.close }) });
  app.mount(document.body.appendChild(document.createElement('div')));
  await settle();
  return events;
}

const q = <T extends Element = HTMLElement>(sel: string) => document.querySelector<T>(sel);
const search = () => q<HTMLInputElement>('.glim-pick-search input')!;
const tiles = () => [...document.querySelectorAll('.glim-provider-pick .glim-readme-btn-name')].map((b) => b.textContent?.trim());
const eyebrows = () => [...document.querySelectorAll('.glim-pick-section .glim-eyebrow')].map((x) => x.textContent?.trim());
const tile = (name: string) => [...document.querySelectorAll<HTMLButtonElement>('.glim-provider-pick button.glim-readme-btn')].find((b) => b.querySelector('.glim-readme-btn-name')?.textContent === name)!;
const footButton = (label: string) => [...document.querySelectorAll<HTMLButtonElement>('.glim-modal-foot button')].find((b) => b.textContent?.trim() === label);
const footLabels = () => [...document.querySelectorAll('.glim-modal-foot button')].map((b) => b.textContent?.trim());
const click = async (el: Element | null | undefined) => {
  (el as HTMLElement).click();
  await settle();
};
const type = async (input: HTMLInputElement, text: string) => {
  input.value = text;
  input.dispatchEvent(new Event('input'));
  await settle();
};
const field = (name: string) => q<HTMLInputElement>(`[data-rf="${name}"]`)!;

describe('the window that adds a remote', () => {
  it('opens on the products in their groups under an open search, with Close as the way out', async () => {
    await open();
    expect(q('.glim-pick-window')).not.toBeNull();
    expect(q('[data-glim-title]')?.getAttribute('data-glim-title')).toBe('Add a remote');
    expect(search().placeholder).toBe('Search providers');
    expect(eyebrows()).toEqual(['Clouds', 'Object storage', 'Servers and protocols', 'On another remote', 'Other rclone backends: 1']);
    expect(tiles()).toEqual(['Nextcloud', 'MinIO', 'Custom S3', 'SMB', 'WebDAV', 'Encrypted', 'quatrix']);
    expect(tile('SMB').querySelector('.glim-readme-btn-sub')?.textContent).toBe('Windows share');
    expect(footLabels()).toEqual(['Close']);
  });

  it('finds a product by rclone\'s name for it and says which tiles take the rest when nothing matches', async () => {
    await open();
    await type(search(), 'webdav');
    expect(tiles()).toEqual(['Nextcloud', 'WebDAV']);
    await type(search(), 'gopher');
    expect(tiles()).toEqual([]);
    expect(q('.glim-pick-none')?.textContent).toBe('Nothing matches “gopher”. Most other services work with Custom S3 or WebDAV.');
    await click(q('.glim-pick-search button'));
    expect(search().value).toBe('');
    expect(tiles()).toHaveLength(7);
  });

  it('goes from a tile to its fields in the same window and back to the tiles', async () => {
    await open();
    await click(tile('Nextcloud'));
    expect(q('.glim-pick-window--form')).not.toBeNull();
    expect(q('.glim-subcard .glim-panel-heading')?.textContent).toContain('Nextcloud');
    expect(field('url')).not.toBeNull();
    expect(q<HTMLInputElement>('#a-rname')!.value).toBe('nextcloud');
    expect(footLabels()).toEqual(['Cancel', 'Back', 'Test', 'Add']);
    await click(footButton('Back'));
    expect(q('.glim-pick-window--form')).toBeNull();
    expect(tiles()).toContain('Nextcloud');
  });

  it('holds the tiles still and lets a long form scroll in the window body', async () => {
    // Vitest leaves the stylesheets out, and which part scrolls is up to them.
    const sheet = (path: string) => document.head.appendChild(Object.assign(document.createElement('style'), { textContent: withoutLayers(readFileSync(path, 'utf8')) }));
    const sheets = [sheet(join(import.meta.dirname, '../../../../node_modules/@glimstone/core/tokens.css')), sheet(join(import.meta.dirname, '../config.css'))];
    try {
      await open();
      const body = () => getComputedStyle(q('.glim-window-body')!).overflowY;
      expect(body()).toBe('hidden');
      await click(tile('Nextcloud'));
      expect(body()).toBe('auto');
    } finally {
      sheets.forEach((s) => s.remove());
    }
  });

  it('offers a backend without a product of its own as a tile and asks for its fields', async () => {
    await open();
    await click(tile('quatrix'));
    expect(field('api_key')).not.toBeNull();
    expect(q<HTMLInputElement>('#a-rname')!.value).toBe('quatrix');
  });

  it('tests the connection in the foot, keeps the button\'s words and says the outcome beside it', async () => {
    await open();
    await click(tile('Nextcloud'));
    await type(field('url'), 'https://cloud.example');
    let go = () => {};
    answerTest = new Promise((r) => { go = r; });
    footButton('Test')!.click();
    await settle();
    expect(footButton('Test')!.getAttribute('aria-busy')).toBe('true');
    go();
    await settle();
    expect(footButton('Test')!.getAttribute('aria-busy')).toBeNull();
    expect(footButton('Test')!.classList).toContain('glim-btn--ok');
    expect(q('.glim-window-start')?.textContent).toBe('Connected, 2 GB free');
    expect(sent.at(-1)).toMatchObject({ path: '/remotes/test', body: { type: 'webdav', params: { url: 'https://cloud.example', vendor: 'nextcloud' } } });
  });

  it('marks a test that failed and keeps the window open', async () => {
    tested = { ok: false, entries: 0, failure: 'timeout' };
    await open();
    await click(tile('Nextcloud'));
    await type(field('url'), 'https://cloud.example');
    await click(footButton('Test'));
    expect(footButton('Test')!.classList).toContain('glim-btn--danger');
    expect(q('.glim-window-start')?.textContent).toBe('The remote didn\'t answer in time');
    expect(q('.glim-pick-window--form')).not.toBeNull();
  });

  it('adds the remote under /remotes and closes', async () => {
    const events = await open();
    await click(tile('Nextcloud'));
    await type(field('url'), 'https://cloud.example');
    await click(footButton('Add'));
    expect(sent.at(-1)).toEqual({
      method: 'POST', path: '/remotes',
      body: { name: 'nextcloud', type: 'webdav', path: '', mount: '/remotes/nextcloud', params: { url: 'https://cloud.example', vendor: 'nextcloud' } },
    });
    expect(events.close).toHaveBeenCalledOnce();
    expect(events.saved).toHaveBeenCalledOnce();
    expect(toasts.at(-1)?.text).toBe('nextcloud is mounted at /remotes/nextcloud');
  });

  it('sends nothing while a field rclone needs is empty', async () => {
    const events = await open();
    await click(tile('Nextcloud'));
    await click(footButton('Add'));
    expect(sent).toEqual([]);
    expect(events.close).not.toHaveBeenCalled();
    expect(toasts.at(-1)?.text).toBe('Address is still missing');
  });

  it('closes from its foot while the tiles show', async () => {
    const events = await open();
    await click(footButton('Close'));
    expect(events.close).toHaveBeenCalledOnce();
  });
});

describe('the window that edits a remote', () => {
  const saved: Remote = { name: 'cloud', type: 'webdav', provider: 'nextcloud', path: '', mount: '/remotes/cloud', label: '', mounted: true, mountOnStart: true, params: { url: 'https://cloud.example', vendor: 'nextcloud' } };

  it('opens straight at the fields, without tiles and without Back', async () => {
    await open(saved, [saved]);
    expect(q('.glim-pick-window')).toBeNull();
    expect(q('[data-glim-title]')?.getAttribute('data-glim-title')).toBe('Edit remote');
    expect(field('url').value).toBe('https://cloud.example');
    expect(footLabels()).toEqual(['Cancel', 'Test', 'Save']);
  });

  it('saves the changes under the same name', async () => {
    const events = await open(saved, [saved]);
    await type(field('url'), 'https://other.example');
    await click(footButton('Save'));
    expect(sent.at(-1)).toMatchObject({ method: 'PUT', path: '/remotes/cloud', body: { name: 'cloud', params: { url: 'https://other.example' } } });
    expect(events.close).toHaveBeenCalledOnce();
  });
});
