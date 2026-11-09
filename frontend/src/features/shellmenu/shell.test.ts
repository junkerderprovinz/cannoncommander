import { createPinia, setActivePinia } from 'pinia';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { api } from '@/api/client';
import { host } from '@/core/host';
import { menuState } from '@/core/menu';
import type { MenuItem } from '@/core/menu';
import type { Entry } from '@/files/entries';
import { actionRows, canAsk, iconUrl, sandboxOffered, setThumbnails, shell, systemMenu } from './shell';

const entry = (name: string, more: Partial<Entry> = {}) => ({ name, time: 7, ...more }) as Entry;

let posted: { url: string; body: unknown }[] = [];
let actions: unknown[] = [];
function server(url: string, init?: RequestInit) {
  if (init?.method === 'POST') {
    posted.push({ url, body: JSON.parse(String(init.body)) });
    if (url === '/api/v1/shell/thumbnails') return Response.json({ way: 'sandbox', restricted: true, allowed: true });
    return new Response(null, { status: 204 });
  }
  if (url.startsWith('/api/v1/shell/actions?')) return Response.json({ actions });
  return new Response(null, { status: 404 });
}

beforeEach(() => {
  setActivePinia(createPinia());
  vi.restoreAllMocks();
  posted = [];
  vi.stubGlobal('fetch', vi.fn(async (url: string, init?: RequestInit) => server(url, init)));
  vi.spyOn(api, 'fsList').mockImplementation(async (path) => ({ path, items: [], etag: path }));
  vi.spyOn(api, 'fsTree').mockResolvedValue({ path: '/', items: [] });
  host.desktop = true;
  Object.assign(shell, { menu: 'native', properties: true, runAs: ['.exe'] });
});

afterEach(() => vi.unstubAllGlobals());

describe('iconUrl', () => {
  it('gives a program its own icon by its path and keeps the glyph for the rest', () => {
    const url = iconUrl(entry('setup.EXE'), '/C/tools', 16, false)!;
    expect(url).toContain('path=%2FC%2Ftools%2Fsetup.EXE');
    expect(url).toContain('v=7');
    expect(iconUrl(entry('notes.txt'), '/C/tools', 16, false)).toBeNull();
  });

  it('asks for the icon of a type once for every file of it', () => {
    expect(iconUrl(entry('a.txt'), '/C/x', 16, true)).toBe(iconUrl(entry('b.txt'), '/D/y', 16, true));
    expect(iconUrl(entry('README'), '/C/x', 16, true)).toContain('name=file&');
  });

  it('picks the next size the system keeps', () => {
    expect(iconUrl(entry('a.exe'), '/C', 20, false)).toContain('size=32');
    expect(iconUrl(entry('a.exe'), '/C', 300, false)).toContain('size=256');
  });

  it('asks a folder by its path, since it may have a picture of its own', () => {
    expect(iconUrl(entry('Documents', { dir: true }), '/C/Users/jdp', 16, true)).toContain('path=%2FC%2FUsers%2Fjdp%2FDocuments');
  });

  it('leaves the container and the way up alone', () => {
    expect(iconUrl(entry('..', { up: true, dir: true }), '/C', 16, true)).toBeNull();
    host.desktop = false;
    expect(iconUrl(entry('a.exe'), '/C', 16, true)).toBeNull();
  });
});

describe('the system menu', () => {
  it('is not asked for archives or the top of all drives', () => {
    expect(canAsk(['/C/a.zip::/x.txt'])).toBe(false);
    expect(canAsk(['/'])).toBe(false);
    expect(canAsk([])).toBe(false);
    expect(canAsk(['/C/a.txt'])).toBe(true);
    shell.menu = '';
    expect(canAsk(['/C/a.txt'])).toBe(false);
  });

  it('sends the place in pixels of the screen', async () => {
    vi.stubGlobal('devicePixelRatio', 1.5);
    await systemMenu(['/C/a.txt'], 100, 41, { extended: true });
    expect(posted).toEqual([{ url: '/api/v1/shell/menu', body: { paths: ['/C/a.txt'], x: 150, y: 62, extended: true } }]);
  });

  it('shows the actions of other file managers as the app\'s own menu on Linux', async () => {
    shell.menu = 'actions';
    actions = [{ key: 'nemo:a', label: 'Shrink' }];
    await systemMenu(['/home/jdp/a.png'], 10, 20);
    expect(menuState.list?.map((it) => (it as MenuItem).label)).toEqual(['Shrink']);
  });
});

describe('actionRows', () => {
  it('puts the actions of a submenu under its name', () => {
    const rows = actionRows(['/a'], [{ key: 'a', label: 'One', group: 'Images' }, { key: 'b', label: 'Two' }, { key: 'c', label: 'Three', group: 'Images' }]) as MenuItem[];
    expect(rows.map((r) => r.label)).toEqual(['Images', 'Two']);
    expect((rows[0]!.sub as MenuItem[]).map((r) => r.label)).toEqual(['One', 'Three']);
  });

  it('says so when nothing fits', () => {
    expect((actionRows(['/a'], [])[0] as MenuItem).disabled).toBe(true);
  });
});

describe('thumbnails behind AppArmor', () => {
  it('offers the rule only where the kernel keeps the sandbox out, and taking it away once it is there', () => {
    shell.thumbnails = undefined;
    expect(sandboxOffered()).toBe(false);
    shell.thumbnails = { way: 'cache', restricted: false, allowed: false };
    expect(sandboxOffered()).toBe(false);
    shell.thumbnails = { way: 'cache', restricted: true, allowed: false };
    expect(sandboxOffered()).toBe(true);
    shell.thumbnails = { way: 'sandbox', restricted: true, allowed: false };
    expect(sandboxOffered()).toBe(false);
    shell.thumbnails = { way: 'sandbox', restricted: true, allowed: true };
    expect(sandboxOffered()).toBe(true);
  });

  it('takes the state the server finds after the rule changed', async () => {
    shell.thumbnails = { way: 'cache', restricted: true, allowed: false };
    await setThumbnails(true);
    expect(posted).toEqual([{ url: '/api/v1/shell/thumbnails', body: { allow: true } }]);
    expect(shell.thumbnails).toEqual({ way: 'sandbox', restricted: true, allowed: true });
  });
});
