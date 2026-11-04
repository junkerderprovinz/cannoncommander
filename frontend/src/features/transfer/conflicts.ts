// The conflict window, before a transfer for every name the check found, and during one for the name a job asks about.
import { reactive } from 'vue';
import type { Choice, Job, JobConflict } from '@/api/ops';
import { toast } from '@/core/toast';
import { cancelBtn, win } from '@/core/windows';
import { t } from '@/i18n';
import { useJobs } from '@/stores/jobs';
import ConflictsBody from './ConflictsBody.vue';

function conflictWindow(conflicts: JobConflict[], dest: string, onAnswer: (choice: Record<string, Choice>, all: Choice | null) => void, onLeave: () => void) {
  const st = reactive({ choice: Object.fromEntries(conflicts.map((c) => [c.key, 'both' as Choice])), all: null as Choice | null });
  let answered = false;
  return win({
    title: t('transfer.conflicts'), wide: true, cls: 'cf-win', body: ConflictsBody, props: { conflicts, dest, st },
    foot: [{ start: t('transfer.target', { dest }) }, cancelBtn(), { label: t('transfer.next'), g: 'IconForward', primary: true, run: () => { answered = true; onAnswer({ ...st.choice }, st.all); } }],
    onClose: () => { if (!answered) onLeave(); },
  });
}

// Resolves to an answer per conflict key, or null when the window was left without one.
export const resolveConflicts = (conflicts: JobConflict[], dest: string) => new Promise<Record<string, Choice> | null>((resolve) => {
  conflictWindow(conflicts, dest, (choice) => resolve(choice), () => resolve(null));
});

// A job that meets a name on its way waits; leaving the window keeps it waiting, the jobs window can answer later.
const asking = new Set<string>();
export function askRunning(job: Job, conflict: JobConflict) {
  if (asking.has(job.id + conflict.key)) return;
  asking.add(job.id + conflict.key);
  const done = () => asking.delete(job.id + conflict.key);
  conflictWindow([conflict], job.dest || '', (choice, all) => {
    done();
    useJobs().decide(job, conflict.key, choice[conflict.key]!, !!all).catch((e) => toast(e.message, 'warn'));
  }, done);
}
