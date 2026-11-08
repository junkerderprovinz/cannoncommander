import { createPinia, setActivePinia } from 'pinia';
import { beforeAll, beforeEach, describe, expect, it, vi } from 'vitest';
import type { Item } from '@/api/client';
import { actions, run } from '@/core/actions';
import { host } from '@/core/host';
import { toasts } from '@/core/toast';
import { loadFeatures } from '@/features';
import { registerFileActions } from '@/files/actions';
import { setLanguage } from '@/i18n';
import { useClipboard } from '@/stores/clipboard';
import { useListing } from '@/stores/listing';
import { type Transfer, usePanels } from '@/stores/panels';
import { share } from './outside';

const HERE = '/C/Users/jdp/Documents';
const TEMP = '/C/Temp';
const item = (name: string, dir = false): Item => ({ name, dir, size: 1, time: 0, mode: '0664', owner: 'jdp', group: 'jdp' });

interface Outside { files: string[]; cut: boolean; skipped: number }
let onClipboard: Outside;
let putFails: boolean;
let sameVolume: boolean;
let sent: { method: string; url: string; body: unknown }[] = [];
let jobs: Transfer[] = [];

function server(url: string, init?: RequestInit) {
  const method = init?.method ?? 'GET';
  if (method !== 'GET') sent.push({ method, url, body: JSON.parse(String(init!.body)) });
  if (url.startsWith('/api/v1/fs/list')) {
    const path = new URL(url, 'http://x').searchParams.get('path')!;
    return new Response(null, { status: useListing().lists.has(path) ? 304 : 404 });
  }
  if (url === '/api/v1/clipboard' && method === 'GET') return Response.json(onClipboard);
  if (url === '/api/v1/clipboard' && putFails) return Response.json({ code: 'notOnDisk', detail: 'only files on disk' }, { status: 400 });
  if (url === '/api/v1/dragdrop/in') {
    const { paths } = JSON.parse(String(init!.body)) as { paths: string[] };
    return Response.json({ files: paths.map((p) => '/' + p.replace(':', '').replaceAll('\\', '/')), cut: false, skipped: 0, sameVolume });
  }
  return new Response(null, { status: 204 });
}

const settle = () => new Promise((r) => setTimeout(r, 0));
const calls = (url: string) => sent.filter((s) => s.url === '/api/v1' + url);
const names = (job: Transfer) => job.items.map((k) => k.at + '/' + k.name);

beforeAll(() => {
  setLanguage('en');
  setActivePinia(createPinia());
  usePanels();
  registerFileActions();
  loadFeatures();
  actions.transfer = { ...actions.transfer!, run: ((job: Transfer) => { jobs.push(job); return { id: 'j1' }; }) as never };
});

beforeEach(() => {
  const F = usePanels(), listing = useListing(), clip = useClipboard();
  F.T().path = HERE;
  F.T().marked = new Set();
  listing.lists.set(HERE, { path: HERE, items: [item('Brief.txt'), item('Fotos', true)] });
  listing.lists.set(TEMP, { path: TEMP, items: [item('a.txt'), item('b.txt'), item('c.txt')] });
  Object.assign(host, { desktop: true, os: 'windows' });
  clip.clear();
  Object.assign(clip, { system: null, under: '' });
  onClipboard = { files: [], cut: false, skipped: 0 };
  putFails = false;
  sameVolume = false;
  sent = [];
  jobs = [];
  toasts.splice(0);
  vi.stubGlobal('fetch', vi.fn(async (url: string, init?: RequestInit) => server(url, init)));
});

describe('the clipboard shared with other programs', () => {
  it('takes files copied in the panels, so Explorer pastes them', async () => {
    usePanels().T().marked = new Set(['Brief.txt', 'Fotos']);
    run('clipCut');
    await settle();
    expect(calls('/clipboard')).toEqual([{ method: 'PUT', url: '/api/v1/clipboard', body: { paths: [HERE + '/Fotos', HERE + '/Brief.txt'], cut: true } }]);
    expect(useClipboard().clip).toBeNull();
    expect(useClipboard().ready()).toBe(true);
  });

  it('stays with the app in the container', async () => {
    host.desktop = false;
    usePanels().T().marked = new Set(['Brief.txt']);
    run('clipCopy');
    await settle();
    expect(sent).toEqual([]);
    expect(useClipboard().clip).toEqual({ cut: false, from: HERE, items: ['Brief.txt'] });
  });

  it('pastes files cut in Explorer as a move and then empties the clipboard', async () => {
    onClipboard = { files: [TEMP + '/a.txt', TEMP + '/b.txt'], cut: true, skipped: 1 };
    await run('clipPaste');
    expect(jobs.map((j) => [j.kind, j.dest, j.from, names(j)])).toEqual([['move', HERE, TEMP, [TEMP + '/a.txt', TEMP + '/b.txt']]]);
    expect(calls('/clipboard/moved')).toEqual([{ method: 'POST', url: '/api/v1/clipboard/moved', body: { files: [TEMP + '/a.txt', TEMP + '/b.txt'] } }]);
    expect(toasts.map((t) => t.text)).toContain('1 item stayed behind because it lies outside the folders here');
  });

  it('pastes files copied in Explorer as a copy and leaves the clipboard as it is', async () => {
    onClipboard = { files: [TEMP + '/c.txt'], cut: false, skipped: 0 };
    await run('clipPaste');
    expect(jobs.map((j) => [j.kind, names(j)])).toEqual([['copy', [TEMP + '/c.txt']]]);
    expect(calls('/clipboard/moved')).toEqual([]);
  });

  it('keeps files from an archive until another program copies something newer', async () => {
    putFails = true;
    onClipboard = { files: [TEMP + '/c.txt'], cut: false, skipped: 0 };
    const zip = HERE + '/Akten.zip::/';
    useListing().lists.set(zip, { path: zip, items: [item('Liste.txt')] });
    await share(false, zip, ['Liste.txt']);
    await run('clipPaste');
    expect(jobs.map((j) => [j.from, j.items.map((k) => k.name)])).toEqual([[zip, ['Liste.txt']]]);
    onClipboard = { files: [TEMP + '/a.txt'], cut: false, skipped: 0 };
    await run('clipPaste');
    expect(jobs.map((j) => j.from)).toEqual([zip, TEMP]);
  });

  it('turns text into a file when the clipboard holds no files', async () => {
    onClipboard = { files: [], cut: false, skipped: 0 };
    expect(await run('pasteFiles')).toBe(false);
  });
});

describe('files dropped from another program', () => {
  it('move on the same drive and are copied from another, as in Explorer', async () => {
    sameVolume = true;
    await run('dropIn', ['C:\\Temp\\a.txt'], HERE, 'auto');
    sameVolume = false;
    await run('dropIn', ['C:\\Temp\\b.txt'], HERE, 'auto');
    expect(jobs.map((j) => [j.kind, names(j)])).toEqual([['move', [TEMP + '/a.txt']], ['copy', [TEMP + '/b.txt']]]);
    expect(calls('/dragdrop/in')[0]!.body).toEqual({ paths: ['C:\\Temp\\a.txt'], to: HERE });
  });

  it('are copied with Ctrl and moved with Shift wherever they come from', async () => {
    sameVolume = true;
    await run('dropIn', ['C:\\Temp\\a.txt'], HERE, 'copy');
    sameVolume = false;
    await run('dropIn', ['C:\\Temp\\b.txt'], HERE, 'move');
    expect(jobs.map((j) => j.kind)).toEqual(['copy', 'move']);
  });
});
