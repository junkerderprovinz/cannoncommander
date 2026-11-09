import { beforeAll, describe, expect, it } from 'vitest';
import { action, actions } from '@/core/actions';
import { bindKey, keymap } from '@/core/keymap';
import { prefs } from '@/core/prefs';
import { settingsTabs } from '@/core/views';
import { addMessages, setLanguage } from '@/i18n';
import de from './de.json';
import en from './en.json';
import setup, { commit, layout, layoutKeys, useLayout } from '.';

const SHIPPED = { F5: 'copy', F6: 'move', 'Ctrl+r': 'refresh', 'Alt+p': 'pack', 'Ctrl+k': 'palette', F1: 'shortcuts', 'Ctrl+l': 'gotoPath' };

beforeAll(async () => {
  addMessages('keys', { de, en });
  setLanguage('de');
  for (const id of ['copy', 'move', 'refresh', 'pack', 'palette', 'shortcuts', 'gotoPath', 'sidePane', 'back']) action(id, { label: id, run: () => {} });
  actions.copy!.key = 'F5';
  for (const [combo, id] of Object.entries(SHIPPED)) bindKey(combo, id);
  // Stored as a whole map, as older settings have it.
  prefs.keys = { map: { ...SHIPPED, F5: 'refresh', 'Ctrl+r': 'copy' }, preset: 'cc' };
  setup();
  await Promise.resolve();
});

describe('own keys', () => {
  it('still come from a whole stored map', () => {
    expect(keymap.F5).toBe('refresh');
    expect(keymap['Ctrl+r']).toBe('copy');
    expect(actions.copy!.key).toBe('Strg+R');
    expect(layout()).toBe('cc');
  });

  it('are kept as what they change in the layout', () => {
    commit({ ...SHIPPED, 'Ctrl+F5': 'copy', F5: undefined } as unknown as Record<string, string>, 'cc');
    expect(prefs.keys).toEqual({ preset: 'cc', own: { 'Ctrl+F5': 'copy', F5: '' } });
    expect(keymap['Ctrl+F5']).toBe('copy');
  });

  it('leave nothing stored once they are as shipped', () => {
    commit({ ...SHIPPED }, 'cc');
    expect(prefs.keys).toBeNull();
  });
});

describe('choosing how the app handles', () => {
  it('brings the layout and keeps own keys on top', () => {
    commit({ ...SHIPPED, 'Ctrl+m': 'move' }, 'cc');
    useLayout('explorer');
    expect(layout()).toBe('explorer');
    expect(keymap.F5).toBe('refresh');
    expect(keymap['Alt+p']).toBe('sidePane');
    expect(keymap['Ctrl+m']).toBe('move');
    expect(layoutKeys('explorer')['Ctrl+m']).toBeUndefined();
  });

  it('selects as in Explorer only with the Explorer layout', () => {
    useLayout('explorer');
    expect(prefs.clickSelect).toBe(true);
    useLayout('cc');
    expect(prefs.clickSelect).toBe(false);
  });

  it('turns a stored Total Commander layout into the Commander one and keeps the own keys', () => {
    prefs.keys = { preset: 'tc', own: { 'Ctrl+m': 'move' } };
    expect(layout()).toBe('cc');
    prefs.keys = { preset: 'tc', map: { ...SHIPPED, 'Ctrl+m': 'move' } };
    expect(layout()).toBe('cc');
    commit({ ...SHIPPED, 'Ctrl+m': 'move' }, layout());
    expect(prefs.keys).toEqual({ preset: 'cc', own: { 'Ctrl+m': 'move' } });
  });

  it('keeps a separate choice of selection when only keys change', () => {
    useLayout('cc');
    prefs.clickSelect = true;
    commit({ ...SHIPPED, 'Ctrl+m': 'move' }, 'cc');
    expect(prefs.clickSelect).toBe(true);
  });
});

describe('the settings tile', () => {
  it('is there under the name of the shortcuts', () => {
    const tile = settingsTabs.find((t) => t.id === 'keys')!;
    expect(typeof tile.label === 'function' ? tile.label() : tile.label).toBe('Tastenkürzel');
    expect(tile.g).toBe('IconKeyboard');
  });
});
