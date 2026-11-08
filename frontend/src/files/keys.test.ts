import { createPinia, setActivePinia } from 'pinia';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { api, toItem } from '@/api/client';
import { prefs } from '@/core/prefs';
import { usePanels } from '@/stores/panels';
import { filesKey, jump, jumpEnd, typingClaims } from './keys';

const file = (name: string) => toItem({ name, dir: false, size: 1, mtime: '2026-11-08T10:00:00Z', mode: '644' });
const settle = () => new Promise((r) => setTimeout(r, 0));
const press = (combo: string) => filesKey(new KeyboardEvent('keydown', { key: combo.split('+').pop()! }), combo);

beforeEach(() => {
  setActivePinia(createPinia());
  history.replaceState(null, '', '/#/mnt/user/docs');
  vi.spyOn(api, 'fsList').mockImplementation(async (path) => ({ path, items: ['a.txt', 'b.txt', 'c.txt'].map(file), etag: path }));
  vi.spyOn(api, 'fsTree').mockResolvedValue({ path: '/', items: [] });
});
afterEach(() => {
  prefs.clickSelect = false;
  jumpEnd();
  vi.restoreAllMocks();
});

async function panel() {
  const F = usePanels();
  await settle();
  const names = () => [...F.T().marked].sort();
  return { F, names };
}

describe('keys in the list', () => {
  it('leave the marks alone as in a commander', async () => {
    const { F, names } = await panel();
    F.T().marked = new Set(['a.txt']);
    press('ArrowDown');
    expect(names()).toEqual(['a.txt']);
  });

  it('carry the selection along as in Explorer, and Shift takes what they pass', async () => {
    prefs.clickSelect = true;
    const { F, names } = await panel();
    const at = F.entries().findIndex((k) => k.name === 'a.txt');
    F.T().cursor = at;
    press('ArrowDown');
    expect(names()).toEqual(['b.txt']);
    press('Shift+ArrowDown');
    expect(names()).toEqual(['b.txt', 'c.txt']);
  });

  it('keep Backspace for typing while letters are typed to jump', () => {
    expect(typingClaims('Backspace')).toBe(false);
    jump.text = 'b';
    expect(typingClaims('Backspace')).toBe(true);
    expect(typingClaims('Ctrl+c')).toBe(false);
  });
});
