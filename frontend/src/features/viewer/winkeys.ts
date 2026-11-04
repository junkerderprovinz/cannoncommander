// The core hands only Escape to an open window, so a window that wants more keys listens for them itself while it
// is on top. It listens on window in the capture phase, ahead of the core, so it can also keep Escape for itself.
import { comboOf } from '@/core/keymap';
import { winStack } from '@/core/windows';

export type WinKeyHandler = (e: KeyboardEvent, combo: string) => boolean | void;

export function winKeys(scrim: () => HTMLElement | null, handler: WinKeyHandler) {
  const fn = (e: KeyboardEvent) => {
    const el = scrim();
    if (!el || winStack.at(-1)?.el !== el) return;
    if (handler(e, comboOf(e)) === true) {
      e.preventDefault();
      e.stopPropagation();
    }
  };
  addEventListener('keydown', fn, true);
  return () => removeEventListener('keydown', fn, true);
}

// The window's title and its foot line belong to the frame; a window whose file changes rewrites them in place.
export const notchOf = (el: Element | null) => el?.closest('.scrim')?.querySelector<HTMLElement>('.notch') ?? null;
export const footStartOf = (el: Element | null) => el?.closest('.scrim')?.querySelector<HTMLElement>('.win-foot .start') ?? null;
