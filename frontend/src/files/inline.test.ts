import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { createApp, nextTick } from 'vue';
import { editing, endInline, inline, onInlineSave, saveInline, startInline } from './inline';
import RenameField from './RenameField.vue';

const saved = vi.fn(async () => {});
beforeEach(() => {
  saved.mockClear();
  onInlineSave(saved);
  endInline();
});

describe('renaming in the row', () => {
  it('marks one row of one panel', () => {
    startInline(1, '/C/Users/jdp', 'a.txt');
    expect(editing(1, '/C/Users/jdp', 'a.txt')).toBe(true);
    expect(editing(0, '/C/Users/jdp', 'a.txt')).toBe(false);
    expect(editing(1, '/C/Users/jdp', 'b.txt')).toBe(false);
  });

  it('saves a new name once, and nothing for an unchanged or empty one', async () => {
    startInline(0, '/home/jdp', 'a.txt');
    await saveInline('  b.txt ');
    expect(saved).toHaveBeenCalledWith('/home/jdp', 'a.txt', 'b.txt', 0);
    expect(inline.panel).toBe(-1);
    for (const typed of ['a.txt', '   ']) {
      startInline(0, '/home/jdp', 'a.txt');
      await saveInline(typed);
    }
    await saveInline('c.txt');
    expect(saved).toHaveBeenCalledTimes(1);
  });
});

describe('the field', () => {
  let app: ReturnType<typeof createApp> | undefined;
  const mount = (name: string) => {
    startInline(0, '/home/jdp', name);
    app = createApp(RenameField, { name });
    app.mount(document.body.appendChild(document.createElement('div')));
    return document.querySelector<HTMLInputElement>('.rn-field')!;
  };
  const press = (el: HTMLElement, key: string) => el.dispatchEvent(new KeyboardEvent('keydown', { key, bubbles: true, cancelable: true }));
  afterEach(() => {
    app?.unmount();
    document.body.innerHTML = '';
  });

  it('starts with the whole name selected, extension and all', async () => {
    for (const name of ['Brief an Oma.odt', 'archiv.tar.gz', 'nfo', '.bashrc']) {
      const el = mount(name);
      await nextTick();
      expect([el.selectionStart, el.selectionEnd], name).toEqual([0, name.length]);
      app?.unmount();
      document.body.innerHTML = '';
    }
  });

  it('keeps the new name on Enter', () => {
    const el = mount('a.txt');
    el.value = 'b.txt';
    press(el, 'Enter');
    expect(saved).toHaveBeenCalledWith('/home/jdp', 'a.txt', 'b.txt', 0);
  });

  it('keeps the new name when it loses the focus', () => {
    const el = mount('a.txt');
    el.value = 'b.txt';
    el.dispatchEvent(new FocusEvent('blur'));
    expect(saved).toHaveBeenCalledTimes(1);
  });

  it('leaves the name as it was on Esc', () => {
    const el = mount('a.txt');
    el.value = 'b.txt';
    press(el, 'Escape');
    el.dispatchEvent(new FocusEvent('blur'));
    expect(saved).not.toHaveBeenCalled();
    expect(inline.panel).toBe(-1);
  });
});
