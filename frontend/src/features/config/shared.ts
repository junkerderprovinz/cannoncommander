// Shared pieces for the settings and admin tabs: relative times, the warning shake, copying, confirmations and keys
// that a popover or a recording row must see before the app's keyboard service does.
import { DAY, now } from '@/core/format';
import { type Prefs, prefs, setPref } from '@/core/prefs';
import { toast } from '@/core/toast';
import { cancelBtn, type FootButton, win, type WinEntry } from '@/core/windows';
import { plural, t } from '@/i18n';
import ConfirmBody from '@/ui/ConfirmBody.vue';

export const c = (key: string, params?: Record<string, string | number>) => t('config.' + key, params);
export const cn = (n: number, key: string, params?: Record<string, string | number>) => plural(n, 'config.' + key, params);

export function ago(at: number) {
  const d = now() - at, min = 60000, h = 60 * min;
  // The server's clock may run a little ahead of this one, which still means just now.
  if (d < -2 * min) {
    const days = Math.ceil(-d / DAY);
    return days <= 1 ? c('ago.tomorrow') : cn(days, 'ago.inDays');
  }
  if (d < 2 * min) return c('ago.now');
  if (d < h) return cn(Math.round(d / min), 'ago.minutes');
  if (d < DAY) return cn(Math.round(d / h), 'ago.hours');
  if (d < 2 * DAY) return c('ago.yesterday');
  return cn(Math.round(d / DAY), 'ago.days');
}

// How long something has been going, for the service that runs since its last start.
export function since(at: number) {
  const d = Math.max(0, now() - at), min = 60000, h = 60 * min;
  if (d < h) return cn(Math.max(1, Math.round(d / min)), 'since.minutes');
  if (d < DAY) return cn(Math.round(d / h), 'since.hours');
  return cn(Math.round(d / DAY), 'since.days');
}

export function shake(el: Element | null | undefined) {
  if (!el) return;
  el.classList.remove('glim-shake');
  void (el as HTMLElement).offsetWidth;
  el.classList.add('glim-shake');
}

// The warning for a value that cannot be saved: a note, and the field it is about shakes.
export function fail(text: string, el?: Element | null): void {
  toast(text, 'warn');
  shake(el);
}

// A save the server may refuse. On a refusal the note shows and the field shakes, and the window stays for another
// try, so the caller only goes on with a result.
export const attempt = <T>(p: Promise<T>, el?: Element | null) => p.then((value) => ({ value }), () => { fail(c('notSaved'), el); return null; });

// When something planned comes, as in 12 Min.
export function soon(at: number) {
  const d = at - now(), min = 60000, h = 60 * min;
  if (d < 2 * min) return c('soon.now');
  if (d < 2 * h) return cn(Math.round(d / min), 'soon.minutes');
  return cn(Math.round(d / h), 'soon.hours');
}

export function confirmWin({ title, text, verb, glyph = 'IconCheck', run }: { title: string; text: string[]; verb: string; glyph?: string; run: () => unknown }) {
  win({ title, body: ConfirmBody, props: { text }, foot: [cancelBtn(), { label: verb, g: glyph, primary: true, run: () => { run(); } }] });
}

// The foot of a window that walks through steps, set without the automatic way out, as each step names its own.
export const stepFoot = (w: WinEntry, foot: FootButton[]) => { w.foot = foot; };

// The app's keyboard service listens on the document in the capture phase. This listens on the window, so it runs first.
export function keysFirst(fn: (e: KeyboardEvent) => void) {
  addEventListener('keydown', fn, true);
  return () => removeEventListener('keydown', fn, true);
}

// The prefs as plain properties, so a control binds with v-model="P.side" and every change is saved at once.
export const P = new Proxy({} as Prefs, {
  get: (_, k: string) => prefs[k],
  set: (_, k: string, v) => { setPref(k, v); return true; },
});
