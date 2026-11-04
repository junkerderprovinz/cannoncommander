// Ordner vergleichen and Synchronisieren: the two panels side by side, file by file, and the plan carried out as a
// job. Recurring runs belong to ArrowLoop, which plans and logs them; this only hands the pair of folders over.
import { reactive } from 'vue';
import { type CompareState, type SyncDirection, type SyncStep, tools } from '@/api/tools';
import { run } from '@/core/actions';
import { prefs } from '@/core/prefs';
import { toast } from '@/core/toast';
import { cancelBtn, closeBtn, win } from '@/core/windows';
import { plural } from '@/bk/kit';
import { inArchive, split } from '@/files/paths';
import { t } from '@/i18n';
import { useJobs } from '@/stores/jobs';
import { useListing } from '@/stores/listing';
import { type RowMark, usePanels } from '@/stores/panels';
import ArrowLoopBody from './ArrowLoopBody.vue';
import CompareBody from './CompareBody.vue';
import SyncBody from './SyncBody.vue';

const label = (p: string) => useListing().placeLabel(p) || split(p).pop() || t('bk.server');
const warn = (e: unknown) => toast((e as Error).message, 'warn');

const MARK: Record<CompareState, [string | null, string | null, string]> = {
  same: ['same', 'same', 'ok'], newerL: ['newer', 'older', 'warn'], newerR: ['older', 'newer', 'warn'],
  differ: ['differ', 'differ', 'warn'], onlyL: ['only', null, ''], onlyR: [null, 'only', ''],
};
const diffWord = (n: number) => plural(n, t('compare.diff'), t('compare.diffs'));

function pair(): [string, string] | null {
  const F = usePanels(), L = F.T(0).path, R = F.T(1).path;
  if (L === R) {
    toast(t('bk.sameFolder'), 'warn');
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
    title: t('compare.title'), cls: 'bk', body: CompareBody, props: { left: label(L), right: label(R), counts: got.counts },
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
  if (inArchive(L) || inArchive(R)) return toast(t('bk.archiveOnly'), 'warn');
  win({ title: t('compare.sync'), wide: true, cls: 'bk bk-sync', body: SyncBody, props: { left: L, right: R } });
}

export async function startSync(L: string, R: string, todo: SyncStep[]) {
  const what = t('compare.label', { left: label(L), right: label(R) });
  try {
    const job = await tools.sync(L, R, todo.map(({ rel, act, to }) => ({ rel, act, to })));
    toast(t('compare.running', { label: what }), 'ok', { label: t('compare.jobs'), run: () => run('jobs') });
    useJobs().whenDone(job, (j) => {
      if (j.state === 'failed') return toast(t('compare.failed', { error: j.error || j.state }), 'fault');
      if (j.state !== 'done') return;
      usePanels().marks.splice(0, 2, {}, {});
      toast(t('compare.synced', { steps: plural(todo.length, t('compare.step'), t('compare.stepsWord')) }));
    });
  } catch (e) {
    warn(e);
  }
}

export function schedule(L: string, R: string, o: { dir: SyncDirection; mirror: boolean }) {
  const st = reactive({ name: t('compare.alNameValue', { left: split(L).pop() || '', right: split(R).pop() || '' }), every: 'daily' as const, at: '03:00' });
  win({
    title: t('compare.arrowLoop'), body: ArrowLoopBody, props: { st },
    foot: [cancelBtn(), { label: t('compare.alHand'), g: 'IconSchedules', primary: true, run: () => {
      const name = st.name.trim() || t('compare.alFallback');
      tools.arrowLoop({ name, left: L, right: R, direction: o.dir, mirror: o.mirror, every: st.every, at: st.at }).then((got) => {
        toast(t('compare.alDone', { name }), 'ok', { label: t('compare.alOpen'), run: () => { window.open(got.url, '_blank', 'noopener'); } });
      }, warn);
    } }],
  });
}
