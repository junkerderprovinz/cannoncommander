// The steps a job row offers beyond pause and cancel: its own speed and the answer to the question it waits on.
import type { Job } from '@/api/ops';
import { MB } from '@/core/format';
import { menu } from '@/core/menu';
import { t } from '@/i18n';
import { askRunning } from '@/features/transfer/conflicts';
import { useJobs } from '@/stores/jobs';

export const CAPS = [0, 25, 50, 100, 200] as const;

export function limitMenu(j: Job, el: HTMLElement) {
  const jobs = useJobs(), r = el.getBoundingClientRect();
  menu(r.left, r.bottom + 6, [
    { head: t('jobs.limitHead') },
    ...CAPS.map((v) => ({
      label: v ? t('jobs.mbs', { n: v }) : t('jobs.noLimit'), g: v ? 'IconSliders' : 'IconBolt', radio: true,
      checked: () => Math.round((j.cap || 0) / MB) === v, run: () => jobs.setCap(j, v * MB),
    })),
  ]);
}

export const answer = (j: Job) => { if (j.conflict) askRunning(j, j.conflict); };
