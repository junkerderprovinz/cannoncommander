// The prototype's Jobs, history, transfer and delete under the names the step files use, read from the stores. A job
// shows its state in German words and its progress in percent, as the prototype's queue did.
import type { Job } from '@/api/ops';
import type { Entry } from '@/files/entries';
import { jobLabel, pctOf } from '@/features/jobs/label';
import { resolveConflicts } from '@/features/transfer/conflicts';
import { RULES } from '@/features/transfer/rules';
import { deleteItems } from '@/features/trash/trash';
import { stepLabel } from '@/features/undo/label';
import { run } from '@/core/actions';
import { GB, KB, MB } from '@/core/format';
import { useJobs } from '@/stores/jobs';
import { type TransferOptions, usePanels } from '@/stores/panels';
import { useUndo } from '@/stores/undo';

const STATE: Record<Job['state'], string> = { queued: 'wartet', running: 'läuft', paused: 'pausiert', asking: 'wartet', done: 'fertig', failed: 'fehlgeschlagen', cancelled: 'abgebrochen' };
const asPrototype = (j: Job) => ({ id: j.id, kind: j.kind, label: jobLabel(j), state: STATE[j.state], pct: pctOf(j), rate: j.rate, total: j.totalBytes, cap: j.cap, job: j });
type Shown = ReturnType<typeof asPrototype>;

export function provideOps(provide: (name: string, value: unknown) => void) {
  const jobs = useJobs(), undo = useUndo(), F = usePanels();
  provide('Jobs', {
    get list() { return jobs.list.map(asPrototype); },
    set list(_: unknown) { jobs.clear().catch(() => {}); },
    running: () => jobs.running.map(asPrototype),
    pause: (j: Shown) => jobs.pause(j.job),
    cancel: (j: Shown) => jobs.cancel(j.job),
    move: (j: Shown, to: number) => jobs.move(j.job, to),
    held: (j: Shown) => j.job.reason === 'parity',
    changed: () => {},
  });
  provide('CC.history', { get list() { return undo.list.map((s) => ({ label: stepLabel(s), at: s.at })); } });
  provide('CC.deleteItems', (sel: Entry[], path: string, after?: () => void) => deleteItems(sel, path, { after }));
  provide('CC.resolveConflicts', resolveConflicts);
  provide('CC.uploadFiles', (files: File[], path: string) => run('uploadFiles', files, path));
  provide('Files.transfer', (kind: 'copy' | 'move', items: Entry[], dest: string, from?: string, rule?: TransferOptions['rule']) => F.transfer(kind, items, dest, from, { rule }));
  provide('CLASH_RULES', RULES());
  for (const [name, value] of Object.entries({ KB, MB, GB })) provide(name, value);
}
