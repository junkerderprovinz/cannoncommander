// What the tool windows share, as the prototype's b-00-kit.js holds it for them: counted words, copying, back and
// forward for a window that walks through folders, and the foot row that follows the window's state.
import { copyText as writeClipboard } from '@glimstone/core';
import type { SelectorItem, SelectOption } from '@glimstone/vue';
import { glyphComponent } from '@/core/glyphs';
import { winNav } from '@/core/nav';
import { toast } from '@/core/toast';
import { closeBtn, type FootButton, type WinEntry } from '@/core/windows';
import { t } from '@/i18n';
import './texts';
import './bk.css';

// Choices written as [value, label, glyph name], for a Selector or a Select.
export type Choice<V> = readonly [value: V, label: string, glyph?: string];
export const segItems = <V extends string | number>(choices: readonly Choice<V>[]): SelectorItem<V>[] => choices.map(([id, label, g]) => ({ id, label, glyph: g ? glyphComponent(g) : undefined }));
export const selectOptions = <V extends string>(choices: readonly Choice<V>[]): SelectOption<V>[] => choices.map(([value, label, g]) => ({ value, label, glyph: g ? glyphComponent(g) : undefined }));

export { num, plural } from '@/i18n';

// The warning shake of a field or button, started over when it is still running.
export function shake(el: Element | null | undefined) {
  if (!el) return;
  el.classList.remove('glim-shake');
  void (el as HTMLElement).offsetWidth;
  el.classList.add('glim-shake');
}

export async function copyText(text: string, done = t('ui.copied')) {
  if (await writeClipboard(text)) toast(done);
}
export const copyWords = () => ({ copy: t('ui.copy'), copied: t('ui.copied') });
export const eyeWords = () => ({ show: t('ui.show'), hide: t('ui.hide') });
export const pickerWords = () => ({ apply: t('ui.take'), hue: t('ui.hue'), area: t('ui.shade'), hex: t('ui.colourValue') });


// The mouse's side buttons step through the folders a window has shown, and closing it takes those steps back out.
export function folderSteps(w: unknown, get: () => string, set: (path: string) => void) {
  const past: string[] = [], ahead: string[] = [];
  const nav = winNav(w, {
    back: () => { if (!past.length) return; ahead.push(get()); set(past.pop()!); },
    forward: () => { if (!ahead.length) return; past.push(get()); set(ahead.pop()!); },
  });
  return {
    go(path: string) {
      if (path === get()) return;
      past.push(get());
      ahead.length = 0;
      set(path);
      nav.push();
    },
    close: nav.close,
  };
}

// A tool window's foot: the note at the start, Schließen, then its own buttons. It is rebuilt whenever the window's
// state changes, so a button's label and whether it can be pressed always match what the window shows.
export function setFoot(w: WinEntry, start: string, buttons: FootButton[]) {
  w.foot = [{ start: start || ' ' }, closeBtn(), ...buttons];
}
export const setPrimary = (w: WinEntry, patch: Partial<FootButton>) => { w.foot = w.foot.map((f) => (f.primary ? { ...f, ...patch } : f)); };
// An empty start text keeps its place, so the buttons do not jump while a window fills it in.
export const setStart = (w: WinEntry, text: string) => { w.foot = w.foot.map((f) => (f.start !== undefined ? { ...f, start: text || ' ' } : f)); };

// Enter in a field of a tool window does what its primary button does.
export function pressPrimary(e: KeyboardEvent) {
  if (!(e.target as HTMLElement).closest('input')) return;
  e.preventDefault();
  (e.target as HTMLElement).closest('.glim-window')?.querySelector<HTMLButtonElement>('.glim-modal-foot .glim-btn--accent:not(:disabled)')?.click();
}
