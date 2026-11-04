// Short notes in the corner. Pointing at one holds it, so a step it offers can still be taken.
import { reactive } from 'vue';
import { extend } from './extend';
import { prefs } from './prefs';

export type ToastKind = 'ok' | 'warn' | 'fault';
export interface ToastAct { label: string; run: () => void }
export interface Toast { id: number; text: string; kind: ToastKind; act?: ToastAct; left: number; started: number; timer?: ReturnType<typeof setTimeout> }

export const toasts = reactive<Toast[]>([]);
let next = 1;

export function dropToast(id: number) {
  const i = toasts.findIndex((t) => t.id === id);
  if (i >= 0) { clearTimeout(toasts[i]!.timer); toasts.splice(i, 1); }
}
export function holdToast(t: Toast) {
  clearTimeout(t.timer);
  t.left -= Date.now() - t.started;
}
export function armToast(t: Toast) {
  t.started = Date.now();
  t.timer = setTimeout(() => dropToast(t.id), t.left);
}

export function toast(text: string, kind: ToastKind = 'ok', act?: ToastAct) {
  // Quiet notifications drop routine confirmations; failures and anything that offers a step still show.
  if (prefs.quietToasts && kind === 'ok' && !act) return;
  toasts.push({ id: next++, text, kind, act, left: 4000, started: 0 });
  const t = toasts.at(-1)!;
  armToast(t);
  extend.toastHistory.forEach((fn) => fn(t));
}
