// A copy, move or delete window stays open once its job has started and shows it to the end. Unless the person keeps
// it open, a job that went well closes it by itself; one that failed always leaves it open with the result.
import { markRaw, watch } from 'vue';
import type { Job } from '@/api/ops';
import { prefs } from '@/core/prefs';
import { closeBtn, type FootButton, type WinEntry, winStack } from '@/core/windows';
import { t } from '@/i18n';
import { useJobs } from '@/stores/jobs';
import { usePanels } from '@/stores/panels';
import DetailsToggle from './DetailsToggle.vue';
import JobBody from './JobBody.vue';

export { toggleDetails } from './details';

const opensTarget = (j: Job) => (j.kind === 'copy' || j.kind === 'move') && !!j.dest;

export function followJob(w: WinEntry, job: Job, from?: string) {
  const jobs = useJobs();
  const now = () => jobs.find(job.id) ?? job;
  const keep = (run: () => unknown) => () => { void run(); return false; };
  function foot(j: Job): FootButton[] {
    const background = { label: t('jobs.background'), g: 'IconWindowMinimise', wayOut: true };
    const cancel = { label: t('jobs.cancel'), g: 'IconClose', run: keep(() => jobs.cancel(now())) };
    switch (j.state) {
      case 'running':
        return [background, { label: t('jobs.pause'), g: 'IconPause', run: keep(() => jobs.pause(now())) }, cancel];
      case 'paused':
        return [background, cancel, { label: t('jobs.resume'), g: 'IconPlay', primary: true, run: keep(() => jobs.pause(now())) }];
      case 'done':
        return [
          ...(opensTarget(j) ? [{ label: t('jobs.openTarget'), g: 'IconFolderOpen', run: () => { void usePanels().go(j.dest!); } }] : []),
          { ...closeBtn(), primary: true },
        ];
      case 'failed':
      case 'cancelled':
        return [{ label: t('jobs.retry'), g: 'IconRefresh', run: keep(() => jobs.retry(now())) }, { ...closeBtn(), primary: true }];
      default:
        return [background, cancel];
    }
  }
  w.body = markRaw(JobBody);
  w.startBody = markRaw(DetailsToggle);
  w.props = { id: job.id, from };
  const stop = watch(() => now().state, (state, was) => {
    if (!winStack.includes(w)) return stop();
    if (state === 'done' && was && was !== 'done' && !now().failedItems && prefs.keepJobWindow !== true) {
      stop();
      return w.close();
    }
    w.foot = foot(now());
  }, { immediate: true, flush: 'sync' });
}
