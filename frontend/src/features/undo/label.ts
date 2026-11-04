// What a step of the undo list says it takes back.
import { now } from '@/core/format';
import { split } from '@/files/paths';
import { t } from '@/i18n';
import type { Step } from '@/stores/undo';

const GLYPH: Record<string, string> = { rename: 'IconPencil', mkdir: 'IconFolderAdd', touch: 'IconFileText', copy: 'IconCopy', move: 'IconMove', trash: 'IconTrash' };

export function stepLabel(step: Step) {
  if ('local' in step) return step.local.label;
  const s = step.server, what = s.count === 1 ? s.name : t('undo.items', { n: s.count });
  return t(`undo.kind.${s.kind}`, { what, name: s.name, to: s.kind === 'rename' ? s.to ?? '' : split(s.to || '').pop() || '' });
}
export const stepGlyph = (step: Step) => ('local' in step ? step.local.g : GLYPH[step.server.kind] || 'IconUndo');
export function stepAgo(step: Step) {
  const m = Math.round((now() - step.at) / 60000);
  return m < 1 ? t('undo.justNow') : m < 60 ? t('undo.minutes', { n: m }) : t('undo.hours', { n: Math.round(m / 60) });
}
