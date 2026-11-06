import { createPinia, setActivePinia } from 'pinia';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import type { Job } from '@/api/ops';
import { emit } from '@/api/sse';
import { type SavedSearch, tools } from '@/api/tools';
import { setLanguage } from '@/i18n';
import { BLANK, useSearch } from './search';

const hit = (name: string, at = '/mnt/user/filme') => ({ name, dir: false, size: 1, mtime: '2026-11-01T10:00:00Z', mode: '0644', at });
const saved = (id: number, over: Partial<SavedSearch['query']> = {}): SavedSearch => ({ id, name: `s${id}`, query: { ...BLANK, name: '*.mkv', ...over } });
const job = (id: string, state: Job['state'], result?: unknown): Job => ({
  id, kind: 'contentSearch', owner: 'jdp', state, position: 0, name: 'plex', count: 1, totalBytes: 0, doneBytes: 0, totalFiles: 1, doneFiles: 0,
  rate: 0, left: null, cap: 0, waitForParity: false, failedItems: 0, createdAt: '2026-11-02T09:30:00Z', ...(result ? { result } : {}),
});
const tick = () => new Promise((r) => setTimeout(r, 0));

beforeEach(() => {
  setActivePinia(createPinia());
  vi.restoreAllMocks();
});

describe('saved searches', () => {
  it('loads, saves, renames and removes', async () => {
    vi.spyOn(tools, 'saved').mockResolvedValue([saved(1)]);
    vi.spyOn(tools, 'saveSearch').mockImplementation(async (name, query) => ({ id: 2, name, query }));
    vi.spyOn(tools, 'changeSearch').mockImplementation(async (s) => s);
    vi.spyOn(tools, 'dropSearch').mockResolvedValue({ body: null });
    const S = useSearch();
    await S.load();
    await S.save('Filme', { ...BLANK, name: '*.mkv' });
    expect(S.saved.map((s) => s.name)).toEqual(['s1', 'Filme']);
    await S.rename(S.byId(2)!, 'Alle Filme');
    expect(S.byId(2)!.name).toBe('Alle Filme');
    await S.remove(S.byId(1)!);
    expect(S.saved.map((s) => s.id)).toEqual([2]);
  });

  it('keeps a search in the words the server reads, so it finds the same in every language', async () => {
    await setLanguage('de');
    vi.spyOn(tools, 'saved').mockResolvedValue([saved(1, { name: 'geändert:>01.05.2026 typ:Bild' })]);
    const save = vi.spyOn(tools, 'saveSearch').mockImplementation(async (name, query) => ({ id: 2, name, query }));
    const change = vi.spyOn(tools, 'changeSearch').mockImplementation(async (s) => s);
    const S = useSearch();
    await S.load();
    await S.save('4K', { ...BLANK, name: 'breite:>=3840 verschlüsselt:nein', min: '2,5 GB', max: '1.500,5' });
    expect(save.mock.calls[0]![1]).toMatchObject({ name: 'width:>=3840 encrypted:no', min: '2.5GB', max: '1500.5' });
    await S.rename(S.byId(1)!, 'Bilder');
    expect(change.mock.calls[0]![0].query.name).toBe('modified:>2026-05-01 type:img');
  });

  it('lists the hits of a search and keeps the old ones while it asks again', async () => {
    const find = vi.spyOn(tools, 'find').mockResolvedValue({ path: '/mnt/user', items: [{ ...hit('a.mkv'), time: 0, owner: '', group: '' }], total: 1, more: false, scanned: 9, pending: 0, unknown: [], indexed: true });
    const S = useSearch(), s = saved(1);
    expect(S.hitsOf(s).done).toBe(false);
    await tick();
    expect(S.hitsOf(s)).toMatchObject({ total: 1, done: true });
    const again = S.refresh(s);
    expect(again.items.map((h) => h.name)).toEqual(['a.mkv']);
    expect(find).toHaveBeenCalledTimes(2);
  });

  it('finds nothing for a line with unknown fields', async () => {
    vi.spyOn(tools, 'find').mockResolvedValue({ path: '/mnt/user', items: [], total: 0, more: false, scanned: 0, pending: 0, unknown: ['foo'], indexed: true });
    const S = useSearch(), s = saved(1, { name: 'foo:1' });
    S.hitsOf(s);
    await tick();
    expect(S.hitsOf(s)).toMatchObject({ items: [], total: 0, done: true });
  });

  it('runs a search for text as a job and takes its hits as they come', async () => {
    vi.spyOn(tools, 'findText').mockResolvedValue(job('c', 'queued'));
    const S = useSearch(), s = saved(1, { text: 'plex' });
    const got = S.hitsOf(s);
    await tick();
    emit({ type: 'job.created', data: job('c', 'running') });
    emit({ type: 'search.hits', data: { job: 'c', items: [hit('compose.yml', '/mnt/user/appdata')], total: 1, scanned: 40 } });
    expect(got.items.map((h) => h.name)).toEqual(['compose.yml']);
    emit({ type: 'job.done', data: job('c', 'done', { items: [hit('compose.yml', '/mnt/user/appdata'), hit('notes.md')], total: 2, scanned: 80 }) });
    await tick();
    expect(got).toMatchObject({ total: 2, done: true });
    expect(got.items.map((h) => h.name)).toEqual(['compose.yml', 'notes.md']);
  });
});
