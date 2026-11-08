import { afterEach, describe, expect, it } from 'vitest';
import type { Item } from '@/api/client';
import { host } from '@/core/host';
import { isHidden, shown } from './entries';

const item = (name: string, more: Partial<Item> = {}): Item => ({ name, dir: false, size: 0, time: 0, mode: '0644', owner: '', group: '', ...more });
const names = (list: Item[], showHidden: boolean, showSystem?: boolean) => list.filter(shown(showHidden, showSystem)).map((k) => k.name);

afterEach(() => { Object.assign(host, { desktop: false, os: '' }); });

describe('what a folder hides', () => {
  const list = [item('notes.txt'), item('.bashrc'), item('Thumbs.db', { hidden: true }), item('desktop.ini', { hidden: true, system: true })];

  it('hides dot files in the container, where the server says nothing else', () => {
    Object.assign(host, { desktop: false, os: 'linux' });
    expect(names(list, false)).toEqual(['notes.txt']);
    expect(isHidden(item('.bashrc'))).toBe(true);
  });

  it('shows dot files on Windows and hides by the attribute alone, as Explorer does', () => {
    Object.assign(host, { desktop: true, os: 'windows' });
    expect(names(list, false)).toEqual(['notes.txt', '.bashrc']);
    expect(isHidden(item('.bashrc'))).toBe(false);
  });

  it('keeps protected system files back until they are asked for as well', () => {
    Object.assign(host, { desktop: true, os: 'windows' });
    expect(names(list, true)).toEqual(['notes.txt', '.bashrc', 'Thumbs.db']);
    expect(names(list, true, true)).toEqual(['notes.txt', '.bashrc', 'Thumbs.db', 'desktop.ini']);
    expect(names(list, false, true)).toEqual(['notes.txt', '.bashrc']);
  });
});
