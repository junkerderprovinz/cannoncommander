// Rückgängig: Strg+Z takes back the newest step, the list in the Bearbeiten menu any of the last ten. A toast that
// offers to undo something lands in the same list, so its button and Strg+Z take it back only once.
import { action } from '@/core/actions';
import { extend } from '@/core/extend';
import { bindKey } from '@/core/keymap';
import { menu } from '@/core/menu';
import { MENUS } from '@/core/menus';
import { dropToast, toast, toasts } from '@/core/toast';
import { t, tl } from '@/i18n';
import { type Step, useUndo } from '@/stores/undo';
import { isServerAct } from './act';
import { stepAgo, stepGlyph, stepLabel } from './label';

async function takeBack(step: Step) {
  const label = stepLabel(step);
  try {
    if (!(await useUndo().take(step))) return;
  } catch (e) {
    return toast((e as Error).message, 'warn');
  }
  toasts.filter((x) => x.text === label).forEach((x) => dropToast(x.id));
  toast(t('undo.taken', { label }));
}

export default function setup() {
  const undo = useUndo();
  undo.load().catch(() => {});

  extend.toastHistory.push((note) => {
    const act = note.act;
    if (!act || act.label !== t('core.undo') || isServerAct(act)) return;
    const step = undo.push(note.text, act.run);
    note.act = { label: act.label, run: () => { void takeBack({ at: step.at, local: step }); } };
  });

  action('undo', { label: tl('undo.undo'), g: 'IconUndo', key: 'Ctrl+z', run: () => {
    const step = undo.list[0];
    if (!step) return toast(t('undo.nothing'), 'warn');
    return takeBack(step);
  } });
  action('undoList', { label: tl('undo.list'), g: 'IconLog', run: () => {
    const r = document.querySelector('[data-menu="1"]')?.getBoundingClientRect() || { left: 200, bottom: 80 };
    const steps = undo.list.slice(0, 10);
    menu(r.left, r.bottom + 6, steps.length
      ? [{ head: t('undo.head') }, ...steps.map((s) => ({ label: stepLabel(s), g: stepGlyph(s), key: stepAgo(s), run: () => { void takeBack(s); } }))]
      : [{ label: t('undo.empty'), g: 'IconInfo', run: () => {} }]);
  } });
  bindKey('Ctrl+z', 'undo');

  const edit = MENUS.find(([id]) => id === 'edit')![1];
  if (!edit.includes('undo')) edit.unshift('undo', 'undoList', '-');
}
