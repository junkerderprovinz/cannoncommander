import { readFileSync } from 'node:fs';
import { join } from 'node:path';
import vm from 'node:vm';
import { beforeEach, describe, expect, it } from 'vitest';
import { action, actions } from './actions';
import { isHead, type MenuEntry, type Resolved, resolveItems } from './menu';
import { allMenuItems, MENUS, menuItems, menuName } from './menus';
import { closeBtn, withWayOut } from './windows';

const register = (...ids: string[]) => ids.forEach((id) => action(id, { label: id, run: () => {} }));
const text = (v: unknown) => (typeof v === 'function' ? v() : v);
const names = (list: Resolved[]) => list.map((it) => (it === '-' ? '-' : isHead(it) ? `#${text(it.head)}` : it.id ?? text(it.label)));

beforeEach(() => {
  for (const id of Object.keys(actions)) delete actions[id];
});

describe('menu resolution', () => {
  it('turns ids into their actions and drops ids nobody has built', () => {
    register('copy', 'move');
    expect(names(resolveItems(['copy', 'rename', 'move']))).toEqual(['copy', 'move']);
  });

  it('drops separators at the ends, twice in a row and right after a heading', () => {
    register('a', 'b', 'c');
    const items: MenuEntry[] = ['-', 'a', '-', '-', 'b', { head: 'Kopf' }, '-', 'c', '-'];
    expect(names(resolveItems(items))).toEqual(['a', '-', 'b', '#Kopf', 'c']);
  });

  it('closes up a separator whose neighbours were not built', () => {
    register('a', 'c');
    expect(names(resolveItems(['a', '-', 'b', '-', 'c']))).toEqual(['a', '-', 'c']);
  });

  it('keeps plain items and side menus as they are', () => {
    const sub = { label: 'Darstellung', sub: ['view-details'] };
    expect(resolveItems([sub])).toEqual([sub]);
  });

  // The prototype's resolveItems, read from its source, decides which separators survive.
  it('keeps the same items as the prototype for every menu', () => {
    const src = readFileSync(join(__dirname, '..', '..', '..', 'prototype', 'src', 'js', '00-core.js'), 'utf8');
    const fn = src.match(/const resolveItems = [\s\S]*?;\r?\n/)![0];
    const CC = { actions: {} as Record<string, unknown> };
    const proto = vm.runInNewContext(`${fn}; resolveItems`, { CC }) as (items: unknown[]) => unknown[];
    register('newFolder', 'copy', 'delete', 'logout', 'single', 'refresh', 'back');
    Object.assign(CC.actions, actions);
    for (const [, items] of MENUS) {
      const flat = items.filter((x) => x !== 'BOOKMARKS');
      expect(resolveItems(flat)).toEqual(proto(flat));
    }
  });
});

describe('menu bar', () => {
  it('names the six menus in German', () => {
    expect(MENUS.map((_, i) => menuName(i))).toEqual(['Datei', 'Bearbeiten', 'Ansicht', 'Gehe zu', 'Werkzeuge', 'Hilfe']);
  });

  it('puts the shortcuts under a heading in Gehe zu', () => {
    const items = menuItems(3, [{ label: 'Downloads', g: 'IconDownload' }]);
    const at = items.findIndex((x) => typeof x === 'object' && 'head' in x);
    expect(text((items[at] as { head: unknown }).head)).toBe('Verknüpfungen');
    expect(items[at + 1]).toMatchObject({ label: 'Downloads' });
    expect(items).not.toContain('BOOKMARKS');
  });

  it('stacks every menu under its own heading when the bar has no room', () => {
    const heads = allMenuItems([]).filter((x): x is { head: string } => typeof x === 'object' && 'head' in x);
    expect(heads.map((h) => text(h.head))).toEqual(['Datei', 'Bearbeiten', 'Ansicht', 'Gehe zu', 'Werkzeuge', 'Hilfe']);
  });

  it('lists the Ansicht menu with its side menus and the sort heading', () => {
    register('single', 'dual', 'sortName', 'refresh');
    expect(names(resolveItems(menuItems(2)))).toEqual(['single', 'dual', '-', 'Darstellung', 'Gruppieren', '#Sortieren', 'sortName', '-', 'refresh']);
  });
});

describe('window foot row', () => {
  it('adds Schließen to a foot without a way out', () => {
    expect(withWayOut([{ label: 'Laden' }]).map((f) => f.label)).toEqual(['Schließen', 'Laden']);
  });

  it('adds Abbrechen before the primary button and after the start text', () => {
    expect(withWayOut([{ start: 'Hinweis' }, { label: 'Speichern', primary: true }]).map((f) => f.label ?? f.start)).toEqual(['Hinweis', 'Abbrechen', 'Speichern']);
  });

  it('leaves a foot that already has a way out alone', () => {
    const foot = [closeBtn()];
    expect(withWayOut(foot)).toBe(foot);
  });
});
