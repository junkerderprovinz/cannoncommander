import { createPinia, setActivePinia } from 'pinia';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { api, ApiError, type Listing } from '@/api/client';
import { emit, watch } from '@/api/sse';
import { useListing } from './listing';

vi.mock('@/api/sse', async (original) => ({ ...(await original<typeof import('@/api/sse')>()), watch: vi.fn() }));

const listing = (path: string, names: string[], etag = 'v1'): Listing => ({
  path, etag, items: names.map((name) => ({ name, dir: false, size: 1, time: 0, mode: '0664', owner: 'nobody', group: 'users' })),
});
const names = (l?: Listing) => l?.items.map((k) => k.name);
const settle = () => new Promise((r) => setTimeout(r, 0));

beforeEach(() => {
  setActivePinia(createPinia());
  vi.restoreAllMocks();
});

describe('listing cache', () => {
  it('asks again with the ETag and keeps its copy while the folder is unchanged', async () => {
    const fsList = vi.spyOn(api, 'fsList').mockResolvedValueOnce(listing('/a', ['x'])).mockResolvedValueOnce(null);
    const L = useListing();
    await L.load('/a');
    await L.load('/a');
    expect(fsList.mock.calls.map((c) => c[1])).toEqual([undefined, 'v1']);
    expect(names(L.lists.get('/a'))).toEqual(['x']);
  });

  it('reads a folder only once while the first answer is on its way', async () => {
    const fsList = vi.spyOn(api, 'fsList').mockResolvedValue(listing('/a', ['x']));
    const L = useListing();
    await Promise.all([L.load('/a'), L.load('/a')]);
    expect(fsList).toHaveBeenCalledOnce();
  });
});

describe('folder tree', () => {
  const gone = () => Promise.reject(new ApiError(404));

  it('asks for the folders below a path only once while the first answer is on its way', async () => {
    const fsTree = vi.spyOn(api, 'fsTree').mockResolvedValue({ path: '/a', items: [] });
    const L = useListing();
    await Promise.all([L.tree('/a'), L.tree('/a')]);
    expect(fsTree).toHaveBeenCalledOnce();
  });

  it('asks for a folder that is not there only once, however often the tree looks again', async () => {
    const fsTree = vi.spyOn(api, 'fsTree').mockImplementation(gone);
    const L = useListing();
    for (let n = 0; n < 9; n++) await expect(L.tree('/mnt/user/downloads')).rejects.toThrow();
    expect(fsTree).toHaveBeenCalledOnce();
  });

  it('asks for a missing folder again once the folder above it changed', async () => {
    const fsTree = vi.spyOn(api, 'fsTree').mockImplementationOnce(gone).mockResolvedValue({ path: '/a/b', items: [] });
    const L = useListing();
    await expect(L.tree('/a/b')).rejects.toThrow();
    emit({ type: 'fs.changed', topic: 'fs:/a' });
    await settle();
    await L.tree('/a/b');
    expect(fsTree.mock.calls.filter(([p]) => p === '/a/b')).toHaveLength(2);
  });
});

describe('live changes', () => {
  it('reads a folder on screen again in place when the server reports a change', async () => {
    vi.spyOn(api, 'fsTree').mockResolvedValue({ path: '/', items: [] });
    const fsList = vi.spyOn(api, 'fsList').mockResolvedValueOnce(listing('/a', ['x'])).mockResolvedValueOnce(listing('/a', ['x', 'y'], 'v2'));
    const L = useListing();
    await L.load('/a');
    L.setShown(['/a']);
    emit({ type: 'fs.changed', topic: 'fs:/a' });
    expect(names(L.lists.get('/a'))).toEqual(['x']);
    await settle();
    expect(fsList).toHaveBeenCalledTimes(2);
    expect(names(L.lists.get('/a'))).toEqual(['x', 'y']);
  });

  it('only forgets a folder out of sight', async () => {
    const fsList = vi.spyOn(api, 'fsList').mockResolvedValue(listing('/b', ['x']));
    const L = useListing();
    await L.load('/b');
    L.setShown(['/a']);
    emit({ type: 'fs.changed', topic: 'fs:/b' });
    await settle();
    expect(L.lists.has('/b')).toBe(false);
    expect(fsList).toHaveBeenCalledOnce();
  });

  it('reads every folder on screen again after the stream lost events', async () => {
    const fsList = vi.spyOn(api, 'fsList').mockImplementation(async (p) => listing(p, ['x']));
    const L = useListing();
    await Promise.all([L.load('/a'), L.load('/b')]);
    L.setShown(['/a', '/b']);
    const before = fsList.mock.calls.length;
    emit({ type: 'resync' });
    await settle();
    expect(new Set(fsList.mock.calls.slice(before).map((c) => c[0]))).toEqual(new Set(['/a', '/b']));
  });

  it('tells the server which folders are on screen', () => {
    useListing().setShown(['/mnt/user', '/mnt/user', '/boot']);
    expect(watch).toHaveBeenLastCalledWith(['/mnt/user', '/boot']);
  });
});

describe('metadata columns', () => {
  it('asks for the rows on screen in one request and never twice', async () => {
    const fsMeta = vi.spyOn(api, 'fsMeta').mockResolvedValue({ '/a/x.mkv': { duration: 60 }, '/a/y.mkv': { duration: 90 } });
    const L = useListing();
    L.wantMeta(['/a/x.mkv'], ['duration']);
    L.wantMeta(['/a/y.mkv', '/a/x.mkv'], ['duration']);
    await settle();
    expect(fsMeta).toHaveBeenCalledOnce();
    expect(fsMeta).toHaveBeenCalledWith(['/a/x.mkv', '/a/y.mkv'], ['duration']);
    expect(L.metaOf('/a', 'y.mkv')).toEqual({ duration: 90 });
    L.wantMeta(['/a/x.mkv'], ['duration', 'vcodec']);
    await settle();
    expect(fsMeta).toHaveBeenLastCalledWith(['/a/x.mkv'], ['vcodec']);
  });

  it('asks again after the server failed', async () => {
    const fsMeta = vi.spyOn(api, 'fsMeta').mockRejectedValueOnce(new Error('down')).mockResolvedValue({});
    const L = useListing();
    L.wantMeta(['/a/x.mkv'], ['duration']);
    await settle();
    L.wantMeta(['/a/x.mkv'], ['duration']);
    await settle();
    expect(fsMeta).toHaveBeenCalledTimes(2);
  });
});

describe('search', () => {
  it('keeps the hits of a query and stops the query before it', async () => {
    const signals: AbortSignal[] = [];
    const search = vi.spyOn(api, 'search').mockImplementation(async (_p, q, _h, signal) => {
      signals.push(signal!);
      return { items: [{ name: q, dir: false, size: 0, time: 0, mode: '0664', owner: '', group: '', at: '/a' }], more: false };
    });
    const L = useListing();
    L.search('/a', 'mk', false);
    L.search('/a', 'mkv', false);
    await settle();
    expect(signals[0]!.aborted).toBe(true);
    expect(L.search('/a', 'mkv', false)).toMatchObject({ done: true, items: [{ name: 'mkv' }] });
    expect(search).toHaveBeenCalledTimes(2);
  });
});

describe('folder sizes still to come', () => {
  const withFolder = (sizeUnknown: boolean, etag: string): Listing => ({
    path: '/a', etag, items: [{ name: 'filme', dir: true, size: sizeUnknown ? 0 : 900, time: 0, mode: '0755', owner: 'nobody', group: 'users', ...(sizeUnknown ? { sizeUnknown } : {}) }],
  });

  it('asks a folder on screen again until the index has measured its folders', async () => {
    vi.useFakeTimers();
    try {
      const fsList = vi.spyOn(api, 'fsList')
        .mockResolvedValueOnce(withFolder(true, 'v1'))
        .mockResolvedValueOnce(null)
        .mockResolvedValueOnce(withFolder(false, 'v2'));
      const L = useListing();
      L.setShown(['/a']);
      await L.load('/a');
      await vi.advanceTimersByTimeAsync(30_000);
      expect(L.lists.get('/a')!.items[0]!.sizeUnknown).toBe(true);
      await vi.advanceTimersByTimeAsync(30_000);
      expect(L.lists.get('/a')!.items[0]).toMatchObject({ size: 900 });
      expect(L.lists.get('/a')!.items[0]!.sizeUnknown).toBeUndefined();
      await vi.advanceTimersByTimeAsync(120_000);
      expect(fsList).toHaveBeenCalledTimes(3);
    } finally {
      vi.useRealTimers();
    }
  });

  it('leaves a folder out of sight alone', async () => {
    vi.useFakeTimers();
    try {
      const fsList = vi.spyOn(api, 'fsList').mockResolvedValue(withFolder(true, 'v1'));
      const L = useListing();
      L.setShown(['/b']);
      await L.load('/a');
      await vi.advanceTimersByTimeAsync(120_000);
      expect(fsList).toHaveBeenCalledOnce();
    } finally {
      vi.useRealTimers();
    }
  });
});

describe('the name of a place', () => {
  it('takes the name Explorer shows for a folder, as its listing gives it', () => {
    const L = useListing();
    const users = listing('/C/Users', ['Public']);
    users.items[0]!.label = 'Öffentlich';
    L.lists.set('/C/Users', users);
    L.trees.set('/C/Users', [{ name: 'Public', path: '/C/Users/Public', hasDirs: true }]);
    expect(L.placeLabel('/C/Users/Public')).toBe('Öffentlich');
    expect(L.placeLabel('/C/Users/Anna')).toBeUndefined();
  });
});
