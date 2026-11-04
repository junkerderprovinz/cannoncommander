import { readFileSync } from 'node:fs';
import { join } from 'node:path';
import vm from 'node:vm';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { action, actions } from './actions';
import { handleKey } from './keyboard';
import { bindDefaults, bindKey, comboOf, DEFAULT_KEYS, keymap } from './keymap';
import { isMenuOpen, menu } from './menu';
import { registerView, view } from './views';
import { closeWin, win, winStack } from './windows';

const key = (k: string, mods: Partial<KeyboardEvent> = {}) => ({ key: k, ctrlKey: false, metaKey: false, altKey: false, shiftKey: false, ...mods });

// The prototype's own comboOf, read from its source, is the reference this one has to match key for key.
const prototypeComboOf = (() => {
  const src = readFileSync(join(__dirname, '..', '..', '..', 'prototype', 'src', 'js', '20-shell.js'), 'utf8');
  const fn = src.match(/const comboOf = \(e\) => \{[\s\S]*?\n\};/)![0];
  return vm.runInNewContext(`${fn}; comboOf`) as typeof comboOf;
})();

describe('comboOf', () => {
  const cases: [string, Partial<KeyboardEvent>, string][] = [
    ['a', {}, 'a'],
    ['A', { shiftKey: true }, 'a'],
    ['*', { shiftKey: true }, '*'],
    ['+', { shiftKey: true }, '+'],
    ['F4', { shiftKey: true }, 'Shift+F4'],
    ['V', { ctrlKey: true, shiftKey: true }, 'Ctrl+Shift+v'],
    ['k', { metaKey: true }, 'Ctrl+k'],
    ['ArrowLeft', { altKey: true }, 'Alt+ArrowLeft'],
    ['Enter', { altKey: true }, 'Alt+Enter'],
    ['F7', { altKey: true }, 'Alt+F7'],
    [' ', {}, ' '],
    ['Escape', {}, 'Escape'],
  ];
  it.each(cases)('turns %s into the combo the key map uses', (k, mods, want) => {
    expect(comboOf(key(k, mods))).toBe(want);
  });
  it.each(cases)('matches the prototype for %s', (k, mods) => {
    expect(comboOf(key(k, mods))).toBe(prototypeComboOf(key(k, mods)));
  });
});

describe('key map', () => {
  beforeEach(() => {
    for (const k of Object.keys(keymap)) delete keymap[k];
  });

  it('binds the prototype keys', () => {
    bindDefaults();
    expect(keymap.F5).toBe('copy');
    expect(keymap['Ctrl+Shift+v']).toBe('nextView');
    expect(Object.keys(keymap)).toHaveLength(Object.keys(DEFAULT_KEYS).length);
  });

  it('lets a key of your own replace a default', () => {
    bindDefaults();
    bindKey('F5', 'move');
    expect(keymap.F5).toBe('move');
  });
});

describe('keyboard layers', () => {
  const press = (k: string, mods: Partial<KeyboardEvent> = {}, target: Element = document.body) => {
    const e = new KeyboardEvent('keydown', { key: k, bubbles: true, cancelable: true, ...mods });
    Object.defineProperty(e, 'target', { value: target });
    handleKey(e);
    return e;
  };
  const viewKey = vi.fn();

  beforeEach(() => {
    for (const k of Object.keys(keymap)) delete keymap[k];
    for (const id of Object.keys(actions)) delete actions[id];
    while (winStack.length) closeWin();
    viewKey.mockReset();
    registerView('files', { component: {}, key: viewKey });
    registerView('settings', { page: true, component: {} });
    view.id = 'files';
  });

  it('runs the action bound to a key and keeps the key from the browser', () => {
    const fn = vi.fn();
    action('copy', { label: 'Kopieren', run: fn });
    bindKey('F5', 'copy');
    expect(press('F5').defaultPrevented).toBe(true);
    expect(fn).toHaveBeenCalledOnce();
    expect(viewKey).not.toHaveBeenCalled();
  });

  it('hands unbound keys to the active view', () => {
    press('ArrowDown');
    expect(viewKey).toHaveBeenCalledWith(expect.any(KeyboardEvent), 'ArrowDown');
  });

  it('leaves typing in a field alone', () => {
    const fn = vi.fn();
    action('selectAll', { label: 'Alles markieren', run: fn });
    bindKey('Ctrl+a', 'selectAll');
    const input = document.createElement('input');
    document.body.appendChild(input);
    press('a', { ctrlKey: true }, input);
    expect(fn).not.toHaveBeenCalled();
    expect(viewKey).not.toHaveBeenCalled();
  });

  it('lets an open menu take every key, and Escape closes it', () => {
    const fn = vi.fn();
    action('copy', { label: 'Kopieren', run: fn });
    bindKey('F5', 'copy');
    menu(10, 10, ['copy']);
    press('F5');
    expect(fn).not.toHaveBeenCalled();
    press('Escape');
    expect(isMenuOpen()).toBe(false);
  });

  it('closes the top window with Escape before anything else', () => {
    win({ title: 'Eins', body: '' });
    win({ title: 'Zwei', body: '' });
    press('Escape');
    expect(winStack.map((w) => w.title)).toEqual(['Eins']);
    expect(viewKey).not.toHaveBeenCalled();
  });

  it('leaves a full page with Escape', () => {
    view.id = 'settings';
    press('Escape');
    expect(view.id).toBe('files');
  });
});
