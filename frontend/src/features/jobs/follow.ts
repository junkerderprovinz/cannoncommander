// A copy, move or delete window stays open once its job has started and shows it to the end. Unless the person keeps
// it open, a job that went well closes it by itself; one that failed always leaves it open with the result.
import { markRaw } from 'vue';
import type { Job } from '@/api/ops';
import { prefs } from '@/core/prefs';
import { closeBtn, type WinEntry, winStack } from '@/core/windows';
import { t } from '@/i18n';
import { useJobs } from '@/stores/jobs';
import FollowBody from './FollowBody.vue';

export function followJob(w: WinEntry, job: Job) {
  const jobs = useJobs();
  w.body = markRaw(FollowBody);
  w.props = { id: job.id };
  w.foot = [
    { label: t('jobs.background'), g: 'IconWindowMinimise', wayOut: true },
    { label: t('jobs.cancel'), g: 'IconClose', run: () => { void jobs.cancel(jobs.find(job.id) ?? job); return false; } },
  ];
  jobs.whenDone(job, (done) => {
    if (!winStack.includes(w)) return;
    if (done.state === 'done' && !done.failedItems && prefs.keepJobWindow !== true) return w.close();
    w.foot = [{ ...closeBtn(), primary: true }];
  });
}
