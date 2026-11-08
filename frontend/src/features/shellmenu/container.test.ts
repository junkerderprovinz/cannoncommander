import type { MenuEntry as GlimEntry, MenuItem as GlimItem } from '@glimstone/vue';
import { createPinia, setActivePinia } from 'pinia';
import { beforeAll, describe, expect, it, vi } from 'vitest';
import type { Item } from '@/api/client';
import { actions } from '@/core/actions';
import { host } from '@/core/host';
import { menuState, toGlim } from '@/core/menu';
import { prefs } from '@/core/prefs';
import { settingsTabs } from '@/core/views';
import { loadFeatures } from '@/features';
import { registerFileActions } from '@/files/actions';
import { rowMenu } from '@/files/menus';
import { setLanguage } from '@/i18n';
import { useListing } from '@/stores/listing';
import { usePanels } from '@/stores/panels';

const HERE = '/mnt/user/Dokumente';
const item = (name: string): Item => ({ name, dir: false, size: 1, time: 0, mode: '0664', owner: 'jdp', group: 'jdp' });
const isRow = (e: GlimEntry): e is GlimItem => e !== '-' && !('head' in e);
let asked: string[] = [];

beforeAll(() => {
  setLanguage('de');
  setActivePinia(createPinia());
  Object.assign(host, { desktop: false, os: 'linux' });
  vi.stubGlobal('fetch', vi.fn(async (url: string) => {
    asked.push(url);
    return new Response(null, { status: 404 });
  }));
  usePanels();
  registerFileActions();
  loadFeatures();
  usePanels().T().path = HERE;
  useListing().lists.set(HERE, { path: HERE, items: [item('setup.exe'), item('Brief.txt')] });
});

describe('the menus in the container', () => {
  it('stay the app\'s own, also on Shift', () => {
    asked = [];
    usePanels().T().marked = new Set(['setup.exe']);
    rowMenu(30, 40, true);
    const labels = toGlim(menuState.list ?? []).filter(isRow).map((r) => r.label);
    expect(labels).toContain('Kopieren');
    expect(labels).not.toContain('Weitere Optionen anzeigen');
    expect(labels).not.toContain('Als Administrator ausführen');
    expect(actions.systemMenu).toBeUndefined();
    expect(asked.filter((u) => u.includes('/shell'))).toEqual([]);
  });

  it('still take the person\'s shape', () => {
    prefs.menuHidden = ['row:copy'];
    usePanels().T().marked = new Set(['Brief.txt']);
    rowMenu(30, 40);
    expect(toGlim(menuState.list ?? []).filter(isRow).map((r) => r.label)).not.toContain('Kopieren');
    delete prefs.menuHidden;
  });

  it('keep the tile Kontextmenü', () => {
    expect(settingsTabs.some((tab) => tab.id === 'ctxmenu')).toBe(true);
  });
});
