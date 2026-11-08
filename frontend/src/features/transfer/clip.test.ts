import { createPinia, setActivePinia } from 'pinia';
import { beforeAll, beforeEach, describe, expect, it, vi } from 'vitest';
import type { Item } from '@/api/client';
import { actions, run } from '@/core/actions';
import { host } from '@/core/host';
import { loadFeatures } from '@/features';
import { registerFileActions } from '@/files/actions';
import { setLanguage } from '@/i18n';
import { useClipboard } from '@/stores/clipboard';
import { useListing } from '@/stores/listing';
import { type Transfer, usePanels } from '@/stores/panels';

const HERE = '/mnt/user/Dokumente';
const TEMP = '/mnt/user/Temp';
const item = (name: string, dir = false): Item => ({ name, dir, size: 1, time: 0, mode: '0664', owner: 'jdp', group: 'jdp' });

let sent: string[] = [];
let jobs: Transfer[] = [];
const settle = () => new Promise((r) => setTimeout(r, 0));

beforeAll(() => {
  setLanguage('en');
  setActivePinia(createPinia());
  Object.assign(host, { desktop: false, os: 'linux' });
  usePanels();
  registerFileActions();
  loadFeatures();
  actions.transfer = { ...actions.transfer!, run: ((job: Transfer) => { jobs.push(job); return { id: 'j1' }; }) as never };
});

beforeEach(() => {
  const F = usePanels(), listing = useListing();
  F.T().path = HERE;
  F.T().marked = new Set();
  listing.lists.set(HERE, { path: HERE, items: [item('Brief.txt'), item('Fotos', true)] });
  listing.lists.set(TEMP, { path: TEMP, items: [item('a.txt')] });
  useClipboard().clear();
  sent = [];
  jobs = [];
  vi.stubGlobal('fetch', vi.fn(async (url: string) => {
    sent.push(url);
    if (url.startsWith('/api/v1/fs/list')) return new Response(null, { status: useListing().lists.has(new URL(url, 'http://x').searchParams.get('path')!) ? 304 : 404 });
    return new Response(null, { status: 204 });
  }));
});

describe('the clipboard in the container', () => {
  it('stays with the app', async () => {
    usePanels().T().marked = new Set(['Brief.txt']);
    run('clipCopy');
    await settle();
    expect(sent.filter((u) => !u.startsWith('/api/v1/fs/'))).toEqual([]);
    expect(useClipboard().clip).toEqual({ cut: false, from: HERE, items: ['Brief.txt'] });
  });

  it('takes rows from several folders, as a search shows them', async () => {
    const F = usePanels();
    useListing().lists.set(HERE, { path: HERE, items: [item('Brief.txt'), { ...item('a.txt'), at: TEMP }] });
    F.T().marked = new Set(['Brief.txt', 'a.txt']);
    run('clipCut');
    F.T().path = '/mnt/user/Ziel';
    useListing().lists.set('/mnt/user/Ziel', { path: '/mnt/user/Ziel', items: [] });
    await run('clipPaste');
    expect(jobs.map((j) => [j.kind, j.dest, j.items.map((k) => k.at + '/' + k.name)])).toEqual([['move', '/mnt/user/Ziel', [TEMP + '/a.txt', HERE + '/Brief.txt']]]);
  });

  it('has no way to other programs on the same computer', async () => {
    expect(actions.dropIn).toBeUndefined();
    expect(actions.dragOut).toBeUndefined();
    expect(await run('pasteFiles')).toBe(false);
    expect(sent.filter((u) => !u.startsWith('/api/v1/fs/'))).toEqual([]);
  });
});
