// Stand-in for GlimStone's Window until @glimstone/vue ships one; WindowHost.vue draws the stack with the prototype's DOM.
// One look for every dialog, the answer buttons at the end of the foot row.
import { type Component, markRaw, shallowReactive } from 'vue';
import { t } from '@/i18n';
import { closeMenu } from './menu';

export interface FootButton { label?: string; g?: string; primary?: boolean; disabled?: boolean; start?: string; run?: (el: HTMLElement) => unknown }
export interface WinOptions {
  title: string;
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
export interface WinEntry extends WinOptions { id: number; foot: FootButton[]; el: HTMLElement | null; close: () => void }

export const winStack = shallowReactive<WinEntry[]>([]);
let next = 1;

export const cancelBtn = (): FootButton => ({ label: t('core.cancel'), g: 'IconClose' });
export const closeBtn = (): FootButton => ({ label: t('core.close'), g: 'IconClose' });
const isWayOut = (label?: string) => !!label && [t('core.cancel'), t('core.close'), t('core.done')].includes(label);

// Every window can be left from its foot row: one without Abbrechen or Schließen gets one before its other buttons.
export function withWayOut(foot: FootButton[]): FootButton[] {
  if (!foot.length || foot.some((f) => isWayOut(f.label))) return foot;
  const at = foot.findIndex((f) => !f.start);
  const out = [...foot];
  out.splice(at < 0 ? out.length : at, 0, foot.some((f) => f.primary) ? cancelBtn() : closeBtn());
  return out;
}

export function win(opts: WinOptions): WinEntry {
  closeMenu();
  const entry: WinEntry = shallowReactive({
    ...opts,
    body: typeof opts.body === 'string' ? opts.body : markRaw(opts.body),
    id: next++,
    foot: withWayOut(opts.foot || []),
    el: null,
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
