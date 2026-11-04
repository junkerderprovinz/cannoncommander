// The toast button for a step the server keeps. Strg+Z and the button take the same step, so only once.
import { type ToastAct, toast } from '@/core/toast';
import { t } from '@/i18n';
import { useUndo } from '@/stores/undo';

// The step id rides on the button itself; the toast list keeps a reactive copy, so a lookup by object would miss it.
export interface StepAct extends ToastAct { step: string }
export const isServerAct = (a: ToastAct) => typeof (a as Partial<StepAct>).step === 'string';

export async function takeStep(id: string) {
  const undo = useUndo();
  if (!undo.byId(id)) await undo.load().catch(() => {});
  const step = undo.byId(id);
  if (!step) return toast(t('undo.nothing'), 'warn');
  await undo.take(step).catch((e) => toast(e.message, 'warn'));
}

export const undoAct = (id: string): StepAct => ({ label: t('core.undo'), step: id, run: () => { void takeStep(id); } });
