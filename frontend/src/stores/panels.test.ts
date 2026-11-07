import { createPinia, setActivePinia } from 'pinia';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { ApiError, api, toItem } from '@/api/client';
import { prefs } from '@/core/prefs';
import { toasts } from '@/core/toast';
import { emit } from '@/api/sse';
import { createFixture } from '../../scripts/fixture.js';
import { newTab, usePanels } from './panels';
import { useUser } from './user';

const fx = createFixture();
const closed = new Set<string>();
const settle = () => new Promise((r) => setTimeout(r, 0));

beforeEach(() => {
  setActivePinia(createPinia());
  vi.restoreAllMocks();
  history.replaceState(null, '', '/');
  Object.assign(prefs, { dual: true, syncBrowse: false, showHidden: false, foldersFirst: true, folderViews: {}, folderGroups: {}, folderTiles: {}, startFolder: '/mnt/user/downloads' });
  toasts.splice(0);
  closed.clear();
  vi.spyOn(api, 'fsList').mockImplementation(async (path) => {
    if (closed.has(path)) throw new ApiError(403);
    const got = fx.list(path);
    if (!got) throw new ApiError(404);
    return { path, items: got.items.map(toItem), etag: path };
  });
  vi.spyOn(api, 'fsTree').mockImplementation(async (path) => fx.tree(path) ?? { path, items: [] });
  vi.spyOn(api, 'fsMeta').mockImplementation(async (paths, columns) => fx.meta({ paths, columns }).items);
  vi.spyOn(api, 'search').mockImplementation(async (path, q, hidden) => {
    const got = fx.search(path, { q, hidden })!;
    return { items: got.items.map((h) => ({ ...toItem(h), at: h.at })), more: !!got.more };
  });
});

async function panels() {
  const F = usePanels();
  await settle();
  return F;
}
const names = (F: ReturnType<typeof usePanels>, i = 0) => F.entries(i).map((k) => k.name);

describe('panels', () => {
  it('opens the folder named in the address first', async () => {
    history.replaceState(null, '', '/#/mnt/user/isos');
    const F = await panels();
    expect(F.T(0).path).toBe('/mnt/user/isos');
    expect(names(F)).toContain('Win11_24H2_German_x64.iso');
  });

  it('keeps the way back and forth, and stays put when a folder is not there', async () => {
    const F = await panels();
    await F.go('/mnt/user/filme');
    await F.go('/mnt/user/filme/Sintel (2010)');
    expect(F.T().hist).toEqual(['/mnt/user/downloads', '/mnt/user/filme']);
    expect(await F.go('/mnt/user/gibtsnicht')).toBe(false);
    expect(F.T().path).toBe('/mnt/user/filme/Sintel (2010)');
    expect(toasts.at(-1)).toMatchObject({ text: 'Den Ordner /mnt/user/gibtsnicht gibt es nicht', kind: 'warn' });
  });

  it('opens a new tab beside a locked one', async () => {
    const F = await panels();
    F.T().locked = true;
    await F.go('/mnt/user/isos');
    expect(F.P().tabs.map((t) => t.path)).toEqual(['/mnt/user/downloads', '/mnt/user/isos']);
    expect(F.P().cur).toBe(1);
  });

  it('puts the cursor on the folder it came out of', async () => {
    const F = await panels();
    await F.go('/mnt/user/fotos/2026');
    await F.open(0, { up: true });
    expect(F.T().path).toBe('/mnt/user/fotos');
    expect(F.entries()[F.T().cursor]!.name).toBe('2026');
  });

  it('searches the folders below by default and only here when asked', async () => {
    const F = await panels();
    await F.go('/mnt/user');
    F.T().filter = '*.mkv';
    F.entries();
    await settle();
    expect(F.entries().map((k) => k.at)).toContain('/mnt/user/filme/Sintel (2010)');
    F.T().deep = false;
    expect(names(F)).toEqual([]);
    F.T().filter = 'film';
    expect(names(F)).toEqual(['filme']);
  });

  it('shows the files of every subfolder in the flat view', async () => {
    const F = await panels();
    await F.go('/mnt/user/filme');
    F.T().flat = true;
    F.entries();
    await settle();
    expect(F.entries().every((k) => !k.dir && k.at)).toBe(true);
    expect(names(F)).toContain('Tears of Steel (2012).mkv');
  });

  it('takes the other panel one step down along, where it can follow', async () => {
    const F = await panels();
    prefs.syncBrowse = true;
    await F.go('/mnt/user', 0);
    await F.go('/mnt/user', 1);
    await F.go('/mnt/user/filme', 0);
    expect(F.T(1).path).toBe('/mnt/user/filme');
    await F.go('/mnt/user', 0);
    expect(F.T(1).path).toBe('/mnt/user');
  });

  it('shows the folder of the volumes as drives unless it has a view of its own', async () => {
    const F = await panels();
    await F.go('/mnt');
    await settle();
    expect(F.viewOf(F.T())).toBe('drives');
    F.setView(F.T(), 'compact');
    expect(F.viewOf(F.T())).toBe('compact');
  });

  it('keeps the zoomed view and card size with the folder', async () => {
    const F = await panels();
    F.setZoom(F.T(), { view: 'large', tile: 200 });
    expect([F.viewOf(F.T()), F.tileOf(F.T())]).toEqual(['large', 200]);
    expect(F.tileOf(F.T(1))).toBeUndefined();
    F.setZoom(F.T(), { view: 'list' });
    expect([F.viewOf(F.T()), F.tileOf(F.T())]).toEqual(['list', undefined]);
  });

  it('shows a view picked from the menu at its own size again', async () => {
    const F = await panels();
    F.setZoom(F.T(), { view: 'gallery', tile: 320 });
    F.setView(F.T(), 'gallery');
    expect(F.tileOf(F.T())).toBeUndefined();
  });

  it('acts on the marked rows, or on the one under the cursor', async () => {
    const F = await panels();
    F.T().cursor = 1;
    expect(F.sel().map((k) => k.name)).toEqual([names(F)[1]]);
    F.T().marked = new Set(['album.flac', 'setup-notes.md']);
    expect(F.sel().map((k) => k.name).sort()).toEqual(['album.flac', 'setup-notes.md']);
  });

  it('groups a folder that keeps its grouping', async () => {
    const F = await panels();
    F.setGroup(F.T(), 'kind');
    expect(names(F).slice(0, 3)).toEqual(['unfertig', 'Big.Buck.Bunny.2008.1080p.mkv', 'album.flac']);
  });
});

describe('a folder that is not there', () => {
  const lastToast = () => toasts.at(-1)?.text;

  it('starts in the home folder the server names when no start folder was picked', async () => {
    prefs.startFolder = '';
    useUser().me = { name: 'jdp', full: 'jdp', role: 'admin', home: '/mnt/user/isos' };
    const F = await panels();
    expect([F.T(0).path, F.T(1).path]).toEqual(['/mnt/user/isos', '/mnt/user/isos']);
  });

  it('opens the nearest folder above a start folder that is gone and says so', async () => {
    prefs.startFolder = '/mnt/user/downloads/weg/tiefer';
    const F = await panels();
    await settle();
    expect(F.T(0).path).toBe('/mnt/user/downloads');
    expect(names(F)).toContain('setup-notes.md');
    expect(lastToast()).toBe('Den Ordner /mnt/user/downloads/weg/tiefer gibt es nicht, darum bist du jetzt in /mnt/user/downloads');
    expect(F.T(0).hist).toEqual([]);
  });

  it('goes up from a folder the user may not open', async () => {
    closed.add('/mnt/user/filme');
    prefs.startFolder = '/mnt/user/filme';
    const F = await panels();
    await settle();
    expect(F.T(0).path).toBe('/mnt/user');
    expect(lastToast()).toBe('Den Ordner /mnt/user/filme darfst du nicht öffnen, darum bist du jetzt in /mnt/user');
  });

  it('goes up from a restored tab whose folder is gone once it is shown', async () => {
    const F = await panels();
    F.P(0).tabs.push(newTab('/mnt/user/alt'));
    F.P(0).cur = 1;
    await settle();
    await settle();
    expect(F.T(0).path).toBe('/mnt/user');
    expect(F.P(0).tabs[0]!.path).toBe('/mnt/user/downloads');
  });

  it('leaves a folder that was deleted elsewhere while it was open', async () => {
    const F = await panels();
    await F.go('/mnt/user/fotos/2026');
    await settle();
    closed.add('/mnt/user/fotos/2026');
    emit({ type: 'fs.changed', topic: 'fs:/mnt/user/fotos/2026' });
    await settle();
    await settle();
    expect(F.T(0).path).toBe('/mnt/user/fotos');
    expect(lastToast()).toMatch(/^Den Ordner \/mnt\/user\/fotos\/2026 /);
  });

  it('opens a bookmark whose folder is gone at the nearest folder above it', async () => {
    const F = await panels();
    await F.reach('/mnt/user/filme/alt/teil');
    expect(F.T(0).path).toBe('/mnt/user/filme');
    expect(F.T(0).hist).toEqual(['/mnt/user/downloads']);
    expect(lastToast()).toBe('Den Ordner /mnt/user/filme/alt/teil gibt es nicht, darum bist du jetzt in /mnt/user/filme');
  });

  it('asks for the folders below a missing start folder only once', async () => {
    prefs.startFolder = '/mnt/user/weg';
    const F = await panels();
    await settle();
    expect(F.T(0).path).toBe('/mnt/user');
    expect(vi.mocked(api.fsTree).mock.calls.filter(([p]) => p === '/mnt/user/weg').length).toBeLessThanOrEqual(1);
  });
});
