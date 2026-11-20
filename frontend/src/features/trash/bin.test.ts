import { createPinia, setActivePinia } from 'pinia';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { api, toItem } from '@/api/client';
import { extend } from '@/core/extend';
import { view } from '@/core/views';
import { usePanels } from '@/stores/panels';
import { createFixture } from '../../../scripts/fixture.js';
import { intoBin } from './bin';

const fx = createFixture();

beforeEach(() => {
  setActivePinia(createPinia());
  vi.restoreAllMocks();
  view.id = 'files';
  extend.beforeGo.splice(0, extend.beforeGo.length, intoBin);
  vi.spyOn(api, 'fsList').mockImplementation(async (path) => ({ path, items: (fx.list(path)?.items ?? []).map(toItem), etag: path }));
  vi.spyOn(api, 'fsTree').mockImplementation(async (path) => fx.tree(path) ?? { path, items: [] });
});

describe('a typed path into a trash folder', () => {
  it('opens the Papierkorb page and leaves the panel where it was', async () => {
    const F = usePanels();
    await F.go('/mnt/user/filme');
    const list = vi.mocked(api.fsList);
    list.mockClear();
    for (const p of ['/mnt/user/fotos/.cc-trash', '/mnt/user/fotos/.cc-trash/1f3a/urlaub.jpg']) {
      view.id = 'files';
      expect(await F.go(p)).toBe(false);
      expect(view.id).toBe('trash');
    }
    expect(list).not.toHaveBeenCalled();
    expect(F.T().path).toBe('/mnt/user/filme');
  });

  it('lets a folder that only looks like one through', async () => {
    const F = usePanels();
    expect(await F.go('/mnt/user/filme')).toBe(true);
    expect(intoBin('/mnt/user/notes.cc-trash')).toBeUndefined();
    expect(intoBin('/mnt/user/.cc-trashcan')).toBeUndefined();
    expect(view.id).toBe('files');
  });
});
