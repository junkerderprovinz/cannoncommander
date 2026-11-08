import { createPinia, setActivePinia } from 'pinia';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { action, actions } from '@/core/actions';
import { host } from '@/core/host';
import type { MenuItem } from '@/core/menu';
import { registerNewMenu } from './newmenu';

beforeEach(() => {
  setActivePinia(createPinia());
  for (const id of ['newFolder', 'newFile']) action(id, { label: id, run: () => {} });
});
afterEach(() => {
  Object.assign(host, { desktop: false, os: '' });
  vi.unstubAllGlobals();
});

const subOf = () => ((actions.newMenu as unknown as MenuItem).sub ?? []).map((e) => (typeof e === 'string' ? e : 'label' in e ? (typeof e.label === 'function' ? e.label() : e.label) : e));

describe('Neu', () => {
  it('offers a folder, a shortcut, a text file and the templates of Windows', async () => {
    Object.assign(host, { desktop: true, os: 'windows' });
    vi.stubGlobal('fetch', vi.fn(async () => Response.json({ templates: [{ id: 'shellnew:.zip', name: 'ZIP-komprimierter Ordner', ext: '.zip', file: 'ZIP-komprimierter Ordner' }] })));
    await registerNewMenu();
    expect(subOf()).toEqual(['newFolder', 'filesystem.shortcut', 'newFile', '-', 'ZIP-komprimierter Ordner']);
  });

  it('leaves the shortcut out where Windows is not', async () => {
    Object.assign(host, { desktop: true, os: 'linux' });
    vi.stubGlobal('fetch', vi.fn(async () => Response.json({ templates: [] })));
    await registerNewMenu();
    expect(subOf()).toEqual(['newFolder', 'newFile']);
  });
});
