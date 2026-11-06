// Ordner vergleichen and Synchronisieren: the two panels side by side, file by file, and the plan carried out as a
// job. Recurring runs belong to ArrowLoop, which plans and logs them; the arrowLoop action hands the pair over.
import { type CompareState, type SyncStep, tools } from '@/api/tools';
import { run } from '@/core/actions';
import { prefs } from '@/core/prefs';
import { toast } from '@/core/toast';
import { closeBtn, win } from '@/core/windows';
import { inArchive } from '@/files/paths';
import { jobError, placeName } from '@/features/jobs/label';
import { t } from '@/i18n';
import { type RowMark, usePanels } from '@/stores/panels';
import { plural } from '@/ui/kit';
import { startJob } from '@/ui/tools';
import CompareBody from './CompareBody.vue';
import SyncBody from './SyncBody.vue';

const warn = (e: unknown) => toast((e as Error).message, 'warn');

const MARK: Record<CompareState, [string | null, string | null, string]> = {
  same: ['same', 'same', 'ok'], newerL: ['newer', 'older', 'warn'], newerR: ['older', 'newer', 'warn'],
  differ: ['differ', 'differ', 'warn'], onlyL: ['only', null, ''], onlyR: [null, 'only', ''],
};
const diffWord = (n: number) => plural(n, 'compare.diffs');

function pair(): [string, string] | null {
  const F = usePanels(), L = F.T(0).path, R = F.T(1).path;
  if (L === R) {
    toast(t('ui.sameFolder'), 'warn');
    return null;
  }
  return [L, R];
}

export async function compareDirs() {
  const two = pair();
  if (!two) return;
  const [L, R] = two, F = usePanels();
  let got;
  try {
    got = await tools.compare(L, R, { top: true, hidden: !!prefs.showHidden });
  } catch (e) {
    return warn(e);
  }
  const marks: Record<string, RowMark>[] = [{}, {}];
  for (const it of got.items) {
    const [l, r, tone] = MARK[it.state];
    if (l) marks[0]![it.name] = { label: t(`compare.marks.${l}`), tone };
    if (r) marks[1]![it.name] = { label: t(`compare.marks.${r}`), tone };
  }
  F.marks.splice(0, 2, ...marks);
  const diffs = got.items.filter((i) => i.state !== 'same');
  win({
    title: t('compare.title'), cls: 'bk', body: CompareBody, props: { left: placeName(L), right: placeName(R), counts: got.counts },
    foot: [{ start: diffs.length ? diffWord(diffs.length) : t('compare.equal') }, closeBtn(),
      { label: t('compare.markDiffs'), g: 'IconSelectAll', run: () => {
        F.T(0).marked = new Set(diffs.filter((d) => d.left).map((d) => d.name));
        F.T(1).marked = new Set(diffs.filter((d) => d.right).map((d) => d.name));
        toast(t('compare.marked', { n: diffWord(diffs.length) }));
      } },
      { label: t('compare.sync'), g: 'IconSync', primary: true, run: () => { setTimeout(() => run('sync'), 0); } }],
  });
}

export function syncWindow() {
  const two = pair();
  if (!two) return;
  const [L, R] = two;
  if (inArchive(L) || inArchive(R)) return toast(t('ui.archiveOnly'), 'warn');
  win({ title: t('compare.sync'), wide: true, cls: 'bk bk-sync', body: SyncBody, props: { left: L, right: R } });
}

export function startSync(L: string, R: string, todo: SyncStep[]) {
  return startJob('sync', { left: L, right: R, steps: todo.map(({ rel, act, to }) => ({ rel, act, to })) }, () => {
    usePanels().marks.splice(0, 2, {}, {});
    toast(t('compare.synced', { steps: plural(todo.length, 'compare.stepCount') }));
  }, { failed: (j) => t('compare.failed', { error: jobError(j) || j.state }) });
}
