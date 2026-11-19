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
  const mount = (name: string, dir = false) => {
    startInline(0, '/home/jdp', name);
    app = createApp(RenameField, { name, dir });
    app.mount(document.body.appendChild(document.createElement('div')));
    return document.querySelector<HTMLInputElement>('.rn-field')!;
  };
  const press = (el: HTMLElement, key: string) => el.dispatchEvent(new KeyboardEvent('keydown', { key, bubbles: true, cancelable: true }));
  afterEach(() => {
    app?.unmount();
    document.body.innerHTML = '';
  });

  const selected = async (name: string, dir = false) => {
    const el = mount(name, dir);
    await nextTick();
    const got = el.value.slice(el.selectionStart!, el.selectionEnd!);
    app?.unmount();
    document.body.innerHTML = '';
    return got;
  };

  it("starts with a file's name selected up to its extension", async () => {
    expect(await selected('Citizen Vigilante (2026).mkv')).toBe('Citizen Vigilante (2026)');
    expect(await selected('Citizen.Vigilante.2026.German.DTSHD.DL.2160p.UHD.BluRay.DV.HDR.x265-VECTOR.mkv')).toBe('Citizen.Vigilante.2026.German.DTSHD.DL.2160p.UHD.BluRay.DV.HDR.x265-VECTOR');
    expect(await selected('album.flac')).toBe('album');
  });

  it('starts with all of a folder or an extensionless file selected', async () => {
    expect(await selected('Citizen.Vigilante.2026.German.DTSHD.DL.2160p.UHD.BluRay.DV.HDR.x265-VECTOR', true)).toBe('Citizen.Vigilante.2026.German.DTSHD.DL.2160p.UHD.BluRay.DV.HDR.x265-VECTOR');
    expect(await selected('nfo')).toBe('nfo');
    expect(await selected('.bashrc')).toBe('.bashrc');
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
