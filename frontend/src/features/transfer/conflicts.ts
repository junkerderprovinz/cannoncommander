// The conflict window, before a transfer for every name the check found, and during one for the name a job asks about.
import { reactive, watch } from 'vue';
import type { Choice, Job, JobConflict } from '@/api/ops';
import { toast } from '@/core/toast';
import { cancelBtn, type FootButton, win } from '@/core/windows';
import { t } from '@/i18n';
import { useJobs } from '@/stores/jobs';
import ConflictsBody from './ConflictsBody.vue';
import { CHOICES, pick } from './rules';

// One name at a time: a button in the foot answers it and goes on to the next. With Für alle übernehmen ticked, the
// button answers every name left, and Nur Neuere ersetzen joins the buttons. A running job may still meet names
// later, so it offers the box for its only one too.
function conflictWindow(conflicts: JobConflict[], dest: string, onAnswer: (choice: Record<string, Choice>, all: Choice | null) => void, onLeave: () => void, later = false) {
  const st = reactive({ at: 0, all: false });
  const choice: Record<string, Choice> = {};
  let answered = false;
  const forAll = () => st.all && (later || conflicts.length - st.at > 1);
  function answer(v: Choice) {
    const all = forAll(), rest = all ? conflicts.slice(st.at) : [conflicts[st.at]!];
    for (const c of rest) choice[c.key] = pick(v, c);
    st.at += rest.length;
    if (st.at < conflicts.length) return false;
    answered = true;
    onAnswer(choice, all ? v : null);
    return true;
  }
  const button = ([v, label, g]: readonly [Choice, string, string]): FootButton => ({ label, g, primary: v === 'both', run: () => answer(v) });
  const foot = (): FootButton[] => [
    { start: t('transfer.target', { dest }) }, cancelBtn(),
    ...(forAll() ? [button(['newer', t('transfer.choice.newer'), 'IconUpdate'])] : []),
    ...CHOICES().map(button),
  ];
  const w = win({
    title: t('transfer.conflicts'), wide: true, cls: 'cf-win', body: ConflictsBody, props: { conflicts, dest, st, later }, foot: foot(),
    onClose: () => { if (!answered) onLeave(); },
  });
  watch(() => [st.at, st.all], () => { w.foot = foot(); }, { flush: 'sync' });
  return w;
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
    useJobs().decide(job, conflict.key, all ?? choice[conflict.key]!, !!all).catch((e) => toast(e.message, 'warn'));
  }, done, true);
}
