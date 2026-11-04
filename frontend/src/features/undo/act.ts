// The toast button for a step the server keeps. Strg+Z and the button take the same step, so only once.
import { type ToastAct, toast } from '@/core/toast';
import { t } from '@/i18n';
import { useUndo } from '@/stores/undo';

const fromServer = new WeakSet<ToastAct>();
export const isServerAct = (a: ToastAct) => fromServer.has(a);

export async function takeStep(id: string) {
  const undo = useUndo();
  if (!undo.byId(id)) await undo.load().catch(() => {});
  const step = undo.byId(id);
  if (!step) return toast(t('undo.nothing'), 'warn');
  await undo.take(step).catch((e) => toast(e.message, 'warn'));
}

export function undoAct(id: string): ToastAct {
  const act = { label: t('core.undo'), run: () => { takeStep(id); } };
  fromServer.add(act);
  return act;
}
