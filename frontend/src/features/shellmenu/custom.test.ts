import { beforeEach, describe, expect, it } from 'vitest';
import { host } from '@/core/host';
import { prefs } from '@/core/prefs';
import { customised, groups, isHidden, move, reorder, resetMenu, rowOf, setHidden, shape } from './custom';

beforeEach(() => {
  delete prefs.menuHidden;
  delete prefs.menuOrder;
  host.desktop = false;
});

describe('shape', () => {
  it('leaves a menu alone that nobody changed', () => {
    const items = ['view', 'edit', '-', 'copy', 'move', '-', 'delete'];
    expect(shape('row', items)).toEqual(items);
  });

  it('drops hidden rows and the separators they leave behind', () => {
    const hidden = (id: string) => ['copy', 'move', 'delete'].includes(id);
    expect(shape('row', ['-', 'view', '-', 'copy', 'move', '-', '-', 'props', '-', 'delete'], hidden, [])).toEqual(['view', '-', 'props']);
  });

  it('moves rows within their group and keeps rows without an id where they are', () => {
    const plain = { label: 'Colour' };
    expect(shape('row', ['copy', plain, 'move', 'rename', '-', 'delete'], () => false, ['rename', 'copy'])).toEqual(['rename', plain, 'move', 'copy', '-', 'delete']);
  });

  it('hides a feature row by its id but never a row without one', () => {
    const openWith = { id: 'openWith', label: 'Open with' }, other = { label: 'Other' };
    expect(shape('row', ['view', openWith, other], (id) => id === 'openWith' || id === '', [])).toEqual(['view', other]);
  });
});

describe('the person\'s choices', () => {
  it('keeps hidden rows apart for each menu', () => {
    setHidden('row', 'copy', true);
    expect(isHidden('row', 'copy')).toBe(true);
    expect(isHidden('list', 'copy')).toBe(false);
    expect(shape('row', ['copy', 'move'])).toEqual(['move']);
    setHidden('row', 'copy', false);
    expect(customised('row')).toBe(false);
  });

  it('moves a row only within its group', () => {
    move('row', 'move', -1);
    expect(groups('row')[1]!.slice(0, 2)).toEqual(['move', 'copy']);
    expect(shape('row', ['copy', 'move', '-', 'share'])).toEqual(['move', 'copy', '-', 'share']);
    move('row', 'copy', 1);
    move('row', 'clipCopy', 1);
    expect(groups('row')[1]).toEqual(['move', 'rename', 'copy', 'clipCut', 'clipCopy']);
  });

  it('resets one menu and leaves the other', () => {
    setHidden('row', 'delete', true);
    setHidden('list', 'refresh', true);
    move('row', 'move', -1);
    resetMenu('row');
    expect(customised('row')).toBe(false);
    expect(isHidden('list', 'refresh')).toBe(true);
  });
});

it('fills slots in the order given', () => {
  expect(reorder(['a', 'b', 'c', 'd'], ['d', 'b'], (x) => x)).toEqual(['a', 'd', 'c', 'b']);
});

it('offers the desktop rows only in the desktop app', () => {
  expect(rowOf('runAs')).toBeNull();
  expect(rowOf('folderOpen')?.g).toBe('IconFolderOpen');
  host.desktop = true;
  expect(rowOf('runAs')?.g).toBe('IconShieldOn');
});
