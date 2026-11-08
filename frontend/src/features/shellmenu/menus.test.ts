import type { MenuEntry as GlimEntry, MenuItem as GlimItem } from '@glimstone/vue';
import { createPinia, setActivePinia } from 'pinia';
import { beforeAll, beforeEach, describe, expect, it, vi } from 'vitest';
import type { Item } from '@/api/client';
import { host } from '@/core/host';
import { closeMenu, menuState, toGlim } from '@/core/menu';
import { prefs } from '@/core/prefs';
import { loadFeatures } from '@/features';
import { registerFileActions } from '@/files/actions';
import { listMenu, rowMenu } from '@/files/menus';
import { setLanguage } from '@/i18n';
import { useListing } from '@/stores/listing';
import { usePanels } from '@/stores/panels';
import { shell } from './shell';

const HERE = '/C/Users/jdp/Downloads';
const item = (name: string, dir = false): Item => ({ name, dir, size: 1, time: 0, mode: '0664', owner: 'jdp', group: 'jdp' });
const isRow = (e: GlimEntry): e is GlimItem => e !== '-' && !('head' in e);
let posted: { url: string; body: Record<string, unknown> }[] = [];

function rows(...names: string[]) {
  usePanels().T().marked = new Set(names);
  rowMenu(30, 40);
  return toGlim(menuState.list ?? []).filter(isRow).map((r) => r.label);
}

beforeAll(() => {
  setLanguage('de');
  setActivePinia(createPinia());
  usePanels();
  registerFileActions();
  loadFeatures();
});

beforeEach(() => {
  const F = usePanels();
  F.T().path = HERE;
  useListing().lists.set(HERE, { path: HERE, items: [item('setup.exe'), item('Brief.txt'), item('Fotos', true)] });
  Object.assign(host, { desktop: true, os: 'windows' });
  Object.assign(shell, { menu: 'native', properties: true, runAs: ['.exe', '.cmd'] });
  delete prefs.menuHidden;
  delete prefs.menuOrder;
  posted = [];
  closeMenu();
  vi.stubGlobal('fetch', vi.fn(async (url: string, init?: RequestInit) => {
    if (init?.method === 'POST') posted.push({ url, body: JSON.parse(String(init.body)) });
    return new Response(null, { status: url.includes('/openwith') ? 404 : 204 });
  }));
});

describe('the row menu in the desktop app', () => {
  it('ends with the system menu, as in Windows 11', () => {
    expect(rows('Brief.txt').at(-1)).toBe('Weitere Optionen anzeigen');
  });

  it('offers to run a program as administrator below opening it', () => {
    const labels = rows('setup.exe');
    expect(labels).toContain('Als Administrator ausführen');
    expect(rows('Brief.txt')).not.toContain('Als Administrator ausführen');
  });

  it('leaves out what the person hid', () => {
    prefs.menuHidden = ['row:copy', 'row:moreOptions'];
    const labels = rows('Brief.txt');
    expect(labels).not.toContain('Kopieren');
    expect(labels.at(-1)).not.toBe('Weitere Optionen anzeigen');
  });

  it('opens the system menu at once on Shift, with the extended rows', () => {
    usePanels().T().marked = new Set(['Brief.txt']);
    rowMenu(30, 40, true);
    expect(menuState.list).toBeNull();
    expect(posted).toEqual([{ url: '/api/v1/shell/menu', body: { paths: [HERE + '/Brief.txt'], x: 30, y: 40, extended: true, background: false } }]);
  });

  it('opens the menu of the folder itself on Shift in empty space', () => {
    listMenu(5, 6, true);
    expect(posted[0]!.body).toMatchObject({ paths: [HERE], background: true });
  });

  it('stays the app\'s own in the container', () => {
    host.desktop = false;
    expect(rows('Brief.txt')).not.toContain('Weitere Optionen anzeigen');
    rowMenu(30, 40, true);
    expect(menuState.list).not.toBeNull();
    expect(posted).toEqual([]);
  });
});
