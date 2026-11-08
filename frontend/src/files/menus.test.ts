import { createPinia, setActivePinia } from 'pinia';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { api } from '@/api/client';
import { menu } from '@/core/menu';
import { addListMenuItems, listMenu } from './menus';

vi.mock('@/core/menu', () => ({ menu: vi.fn() }));

beforeEach(() => {
  setActivePinia(createPinia());
  vi.spyOn(api, 'fsList').mockImplementation(async (path) => ({ path, items: [], etag: path }));
  vi.spyOn(api, 'fsTree').mockResolvedValue({ path: '/', items: [] });
});

describe('list menu', () => {
  it('puts the rows a feature adds after the ones that make something new', () => {
    const asked: string[] = [];
    addListMenuItems((path) => {
      asked.push(path);
      return ['newShortcut'];
    });
    listMenu(10, 20);
    const items = vi.mocked(menu).mock.calls[0]![2] as string[];
    expect(items.slice(0, 6)).toEqual(['newFolder', 'newFile', 'upload', 'clipPaste', 'newShortcut', '-']);
    expect(asked).toHaveLength(1);
  });
});
