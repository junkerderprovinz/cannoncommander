import { createPinia, setActivePinia } from 'pinia';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import type { Item } from '@/api/client';
import { tools } from '@/api/tools';
import { keepOnDevice } from './index';

const item = (name: string, cloud?: Item['cloud']): Item => ({ name, dir: false, size: 0, time: 0, mode: '0644', owner: '', group: '', ...(cloud ? { cloud } : {}) });

beforeEach(() => { setActivePinia(createPinia()); });

describe('a file of the cloud', () => {
  it('stays on the device or gives its space back by the pinned attribute', async () => {
    const attrs = vi.spyOn(tools, 'attrs').mockResolvedValue({ changed: 2 });
    keepOnDevice([item('a.jpg', 'online'), { ...item('b.jpg', 'local'), at: '/C/Bilder/2025' }], '/C/Bilder', true);
    keepOnDevice([item('c.jpg', 'pinned')], '/C/Bilder', false);
    await Promise.resolve();
    expect(attrs.mock.calls).toEqual([
      [{ paths: ['/C/Bilder/a.jpg', '/C/Bilder/2025/b.jpg'], flags: { pinned: true } }],
      [{ paths: ['/C/Bilder/c.jpg'], flags: { pinned: false } }],
    ]);
  });
});
