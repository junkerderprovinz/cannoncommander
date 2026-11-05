// The stack of open windows, opened from anywhere with win(); WinFrame.vue draws each with GlimStone's Window.
import { type Component, markRaw, shallowReactive } from 'vue';
import { t } from '@/i18n';
import { closeMenu } from './menu';

// wayOut marks a button that leaves the window by itself, so Window adds no Abbrechen or Schließen beside it.
export interface FootButton { label?: string; g?: string; primary?: boolean; disabled?: boolean; wayOut?: boolean; start?: string; run?: (el: HTMLElement) => unknown }
export interface WinOptions {
  title: string;
  // The name of a dot after the title, for a file with unsaved changes.
  dot?: string;
  // A string is markup; a component gets `props` and a `close` prop.
  body: string | Component;
  props?: Record<string, unknown>;
  foot?: FootButton[];
  wide?: boolean;
  full?: boolean;
  cls?: string;
  onOpen?: (el: HTMLElement, close: () => void) => void;
  onClose?: () => void;
}
// A window that walks through steps changes its body props and foot while open; a locked one stays until its own
// button closes it, so a code shown only once cannot be lost to a stray Esc.
export interface WinEntry extends WinOptions { id: number; foot: FootButton[]; el: HTMLElement | null; locked: boolean; close: () => void }

export const winStack = shallowReactive<WinEntry[]>([]);
let next = 1;

export const cancelBtn = (): FootButton => ({ label: t('core.cancel'), g: 'IconClose' });
export const closeBtn = (): FootButton => ({ label: t('core.close'), g: 'IconClose' });
export const isWayOut = (f: FootButton) => !!f.wayOut || (!!f.label && [t('core.cancel'), t('core.close'), t('core.done')].includes(f.label));

export function win(opts: WinOptions): WinEntry {
  closeMenu();
  const entry: WinEntry = shallowReactive({
    ...opts,
    body: typeof opts.body === 'string' ? opts.body : markRaw(opts.body),
    id: next++,
    foot: opts.foot || [],
    el: null,
    locked: false,
    close: () => {
      const i = winStack.indexOf(entry);
      if (i < 0) return;
      winStack.splice(i, 1);
      opts.onClose?.();
      document.querySelector<HTMLElement>('.panel.active .list')?.focus();
    },
  });
  winStack.push(entry);
  return entry;
}

export const closeWin = () => winStack.at(-1)?.close();
