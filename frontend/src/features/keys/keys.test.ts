import { describe, expect, it } from 'vitest';
import { setLanguage } from '@/i18n';
import { addMessages } from '@/i18n';
import de from './de.json';
import en from './en.json';
import { assign, changed, check, combosOf, heldBack, keyLabel, mainCombo, ownKeys, preset, reservedBy, reset, restore, swap, withOwn } from './keys';

addMessages('keys', { de, en });
const DEFAULTS = { 'Ctrl+a': 'selectAll', F2: 'rename', F5: 'copy', 'Alt+p': 'pack', 'Alt+u': 'unpack', 'Ctrl+r': 'refresh', 'Alt+ArrowDown': 'history' };

describe('how a combo reads', () => {
  it('names the modifiers and keys in German', () => {
    setLanguage('de');
    expect(keyLabel('Ctrl+Shift+c')).toBe('Strg+Umschalt+C');
    expect(keyLabel('Alt+ArrowDown')).toBe('Alt+Runter');
    expect(keyLabel(' ')).toBe('Leertaste');
  });

  it('keeps a plus that is the key itself', () => {
    setLanguage('de');
    expect(keyLabel('Ctrl++')).toBe('Strg++');
  });

  it('follows the language', () => {
    setLanguage('en');
    expect(keyLabel('Ctrl+Delete')).toBe('Ctrl+Del');
    setLanguage('de');
  });
});

describe('a new key', () => {
  it('may not take a fixed key', () => {
    expect(check(DEFAULTS, 'Tab', 'copy', false)).toEqual({ ok: false, reason: 'fixed' });
    expect(check(DEFAULTS, 'F10', 'copy', false)).toEqual({ ok: false, reason: 'fixed' });
  });

  it('may not be a single letter or digit, which typing to jump needs', () => {
    expect(check(DEFAULTS, 'x', 'copy', false)).toEqual({ ok: false, reason: 'letter' });
    expect(check(DEFAULTS, '7', 'copy', false)).toEqual({ ok: false, reason: 'letter' });
    expect(check(DEFAULTS, 'ä', 'copy', false)).toEqual({ ok: false, reason: 'letter' });
  });

  it('may not be a key the browser keeps for its tabs and windows', () => {
    for (const combo of ['Ctrl+w', 'Ctrl+t', 'Ctrl+n', 'Ctrl+Shift+n', 'Ctrl+Tab', 'Ctrl+Shift+Tab', 'Ctrl+PageDown']) {
      expect(check(DEFAULTS, combo, 'copy', false)).toEqual({ ok: false, reason: 'browser' });
    }
  });

  it('may take those keys in the desktop app, where nothing else wants them', () => {
    expect(check(DEFAULTS, 'Ctrl+w', 'copy', true)).toEqual({ ok: true });
    expect(check(DEFAULTS, 'Ctrl+Shift+Tab', 'copy', true)).toEqual({ ok: true });
  });

  it('may not be a key the system takes before any program sees it', () => {
    expect(check(DEFAULTS, 'Alt+F4', 'copy', true)).toEqual({ ok: false, reason: 'system' });
    expect(check(DEFAULTS, 'Alt+Tab', 'copy', false)).toEqual({ ok: false, reason: 'system' });
  });

  it('names a clash with another action', () => {
    expect(check(DEFAULTS, 'F2', 'copy', false)).toEqual({ ok: false, reason: 'taken', other: 'rename' });
  });

  it('takes a key the action already has without asking', () => {
    expect(check(DEFAULTS, 'F5', 'copy', false)).toEqual({ ok: true });
  });

  it('replaces the keys the action had and takes the key from the other action', () => {
    const map = assign(DEFAULTS, 'F2', 'copy');
    expect(combosOf(map, 'copy')).toEqual(['F2']);
    expect(combosOf(map, 'rename')).toEqual([]);
    expect(changed(map, DEFAULTS, 'copy')).toBe(true);
  });

  it('hands the other action the keys this one had on a swap', () => {
    const map = swap(DEFAULTS, 'F2', 'copy');
    expect(combosOf(map, 'copy')).toEqual(['F2']);
    expect(combosOf(map, 'rename')).toEqual(['F5']);
  });

  it('goes back to its own keys on reset without taking one that is used elsewhere', () => {
    const moved = assign(assign(DEFAULTS, 'Ctrl+r', 'copy'), 'F5', 'refresh');
    const back = reset(moved, DEFAULTS, 'copy');
    expect(combosOf(back, 'copy')).toEqual([]);
    expect(back['Ctrl+r']).toBeUndefined();
    expect(back.F5).toBe('refresh');
  });
});

describe('a reset whose key another action has taken', () => {
  const swapped = swap(DEFAULTS, 'F2', 'copy');

  it('knows which of its keys are held elsewhere', () => {
    expect(heldBack(swapped, DEFAULTS, 'copy')).toEqual(['F5']);
    expect(heldBack(DEFAULTS, DEFAULTS, 'copy')).toEqual([]);
  });

  it('can swap them back', () => {
    const map = restore(swapped, DEFAULTS, 'copy', true);
    expect(combosOf(map, 'copy')).toEqual(['F5']);
    expect(combosOf(map, 'rename')).toEqual(['F2']);
  });

  it('can take the key and leave the other action without it', () => {
    const map = restore(swapped, DEFAULTS, 'copy', false);
    expect(combosOf(map, 'copy')).toEqual(['F5']);
    expect(combosOf(map, 'rename')).toEqual([]);
    expect(map.F2).toBeUndefined();
  });
});

describe('which keys a binding can use', () => {
  it("leaves the browser's keys to the browser and gives them to the desktop app", () => {
    expect(reservedBy('Ctrl+t', false)).toBe('browser');
    expect(reservedBy('Ctrl+t', true)).toBeUndefined();
    expect(reservedBy('Ctrl+r', false)).toBeUndefined();
  });

  it("leaves the system's keys to the system everywhere", () => {
    expect(reservedBy('Alt+F4', false)).toBe('system');
    expect(reservedBy('Alt+F4', true)).toBe('system');
  });
});

describe('own keys', () => {
  const EXPLORER = preset('explorer', DEFAULTS, () => true);

  it('are only what differs from the layout', () => {
    const map = assign(DEFAULTS, 'Ctrl+F5', 'copy');
    expect(ownKeys(DEFAULTS, map)).toEqual({ 'Ctrl+F5': 'copy', F5: '' });
    expect(ownKeys(DEFAULTS, { ...DEFAULTS })).toEqual({});
  });

  it('come back on top of the layout they were made on', () => {
    const map = swap(DEFAULTS, 'F2', 'copy');
    expect(withOwn(DEFAULTS, ownKeys(DEFAULTS, map))).toEqual(map);
  });

  it('stay when another layout is chosen', () => {
    const own = ownKeys(DEFAULTS, assign(DEFAULTS, 'Ctrl+k', 'copy'));
    const map = withOwn(EXPLORER, own);
    expect(map.Backspace).toBe('back');
    expect(combosOf(map, 'copy')).toEqual(['Ctrl+k']);
  });

  it('let a key shipped later reach someone with keys of their own', () => {
    const own = ownKeys(DEFAULTS, assign(DEFAULTS, 'Ctrl+k', 'copy'));
    expect(withOwn({ ...DEFAULTS, 'Alt+F1': 'help' }, own)['Alt+F1']).toBe('help');
  });

  it("go back to the layout's key on reset, not the shipped one", () => {
    const map = assign(EXPLORER, 'Ctrl+F2', 'refresh');
    expect(combosOf(reset(map, EXPLORER, 'refresh'), 'refresh').sort()).toEqual(['Ctrl+r', 'F5']);
    expect(changed(EXPLORER, EXPLORER, 'copy')).toBe(false);
    expect(changed(EXPLORER, DEFAULTS, 'copy')).toBe(true);
  });
});

describe('the Commander layout', () => {
  it('is the shipped map', () => {
    expect(preset('cc', DEFAULTS, () => true)).toEqual(DEFAULTS);
  });
});

describe('the Explorer layout', () => {
  it('puts Explorer keys on top and keeps the viewer on F3', () => {
    const map = preset('explorer', { ...DEFAULTS, F3: 'view', Backspace: 'up' }, () => true);
    expect(map.F5).toBe('refresh');
    expect(map['Ctrl+r']).toBe('refresh');
    expect(map['Alt+p']).toBe('sidePane');
    expect(map.Backspace).toBe('back');
    expect(map.F3).toBe('view');
    expect(map['Ctrl+a']).toBe('selectAll');
  });

  it('leaves out actions the app does not have', () => {
    expect(preset('explorer', DEFAULTS, (id) => id !== 'sidePane')['Alt+p']).toBe('pack');
  });
});

describe('the key a menu shows', () => {
  it('stays the shipped one while the action still has it', () => {
    setLanguage('de');
    const map = { ...DEFAULTS, 'Ctrl+F5': 'copy' };
    expect(mainCombo(map, 'copy', 'F5')).toBe('F5');
  });
});
