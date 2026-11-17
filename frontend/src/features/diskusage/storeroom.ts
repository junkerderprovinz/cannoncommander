// The storeroom tab's sections and the sentences that say, before anything happens, what each step will do.
import type { FindKind, StoreroomFind, StoreroomOp, StoreroomStep } from '@/api/storeroom';
import { t } from '@/i18n';

export type Section = Exclude<FindKind, 'cache'>;
export const SECTIONS: { id: Section; g: string }[] = [
  { id: 'template', g: 'IconFileDesign' },
  { id: 'appdata', g: 'IconFolder' },
  { id: 'container', g: 'IconContainers' },
  { id: 'network', g: 'IconNetwork' },
  { id: 'volume', g: 'IconDatabase' },
  { id: 'image', g: 'IconCube' },
];

export const keyOf = (f: { kind: string; id: string }) => `${f.kind}|${f.id}`;

// The finds by section, in the order of SECTIONS; the build cache sits with the images, which go for good as well.
export function sections(finds: StoreroomFind[]) {
  return SECTIONS.map((s) => ({ ...s, finds: finds.filter((f) => (f.kind === 'cache' ? 'image' : f.kind) === s.id) })).filter((s) => s.finds.length);
}

export function nameOf(f: { kind: string; id: string; name: string }) {
  if (f.kind === 'cache') return t('diskusage.storeroom.cache');
  if (f.kind === 'image' && !f.name) return `${t('diskusage.storeroom.untagged')} ${f.id.replace(/^sha256:/, '').slice(0, 12)}`;
  return f.name;
}

export function stepLine(op: StoreroomOp, s: StoreroomStep) {
  const name = nameOf({ kind: s.kind, id: s.from, name: s.kind === 'image' ? s.name : s.name || s.from }), to = s.to ?? '';
  if (s.problem === 'inUse' || s.problem === 'gone') return t('diskusage.storeroom.steps.inUse', { name });
  if (s.problem === 'taken') return t('diskusage.storeroom.steps.taken', { name, to });
  if (s.problem === 'failed') return t('diskusage.storeroom.steps.failed', { name, error: s.error ?? '' });
  if (op === 'restore') return t('diskusage.storeroom.steps.restore', { name, to });
  if (op === 'discard' || s.forGood) return t('diskusage.storeroom.steps.forGood', { name });
  return s.kind === 'template' || s.kind === 'appdata' ? t('diskusage.storeroom.steps.storeAt', { name, to }) : t('diskusage.storeroom.steps.store', { name });
}
