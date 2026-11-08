import { describe, expect, it } from 'vitest';
import { setLanguage } from '@/i18n';
import { addMessages } from '@/i18n';
import de from './de.json';
import en from './en.json';
import { assign, changed, check, combosOf, keyLabel, mainCombo, preset, reset } from './keys';

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

  it('asks before taking a key another action has, and takes it on the second press', () => {
    expect(check(DEFAULTS, 'F2', 'copy', false)).toEqual({ ok: false, reason: 'taken', other: 'rename' });
    expect(check(DEFAULTS, 'F2', 'copy', true)).toEqual({ ok: true });
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

  it('goes back to its own keys on reset without taking one that is used elsewhere', () => {
    const moved = assign(assign(DEFAULTS, 'Ctrl+r', 'copy'), 'F5', 'refresh');
    const back = reset(moved, DEFAULTS, 'copy');
    expect(combosOf(back, 'copy')).toEqual([]);
    expect(back['Ctrl+r']).toBeUndefined();
    expect(back.F5).toBe('refresh');
  });
});

describe('the Total Commander layout', () => {
  it('moves the shared actions to Total Commander keys', () => {
    const map = preset('tc', DEFAULTS, () => true);
    expect(map['Alt+F5']).toBe('pack');
    expect(map['Alt+F9']).toBe('unpack');
    expect(map['Shift+F6']).toBe('rename');
    expect(map['Alt+p']).toBeUndefined();
    expect(map.F2).toBe('refresh');
  });

  it('leaves out actions the app does not have', () => {
    const map = preset('tc', DEFAULTS, (id) => id !== 'addBookmark');
    expect(map['Ctrl+d']).toBeUndefined();
  });

  it('is the shipped map for CannonCommander', () => {
    expect(preset('cc', DEFAULTS, () => true)).toEqual(DEFAULTS);
  });
});

describe('the Explorer layout', () => {
  it('puts Explorer keys on top and keeps the viewer on F3', () => {
    const map = preset('explorer', { ...DEFAULTS, F3: 'view', Backspace: 'up' }, () => true);
    expect(map.F5).toBe('refresh');
    expect(map['Ctrl+r']).toBe('refresh');
    expect(map['Alt+p']).toBe('quickview');
    expect(map.Backspace).toBe('back');
    expect(map.F3).toBe('view');
    expect(map['Ctrl+a']).toBe('selectAll');
  });

  it('leaves out actions the app does not have', () => {
    expect(preset('explorer', DEFAULTS, (id) => id !== 'quickview')['Alt+p']).toBe('pack');
  });
});

describe('the key a menu shows', () => {
  it('stays the shipped one while the action still has it', () => {
    setLanguage('de');
    const map = { ...DEFAULTS, 'Ctrl+F5': 'copy' };
    expect(mainCombo(map, 'copy', 'F5')).toBe('F5');
  });
});
