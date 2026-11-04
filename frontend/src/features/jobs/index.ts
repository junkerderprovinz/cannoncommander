// Aufgaben: the jobs window, the job settings kept in step with the server, and a notice when a long job ends.
import { watch } from 'vue';
import type { Job, JobSettings } from '@/api/ops';
import { action } from '@/core/actions';
import { MB } from '@/core/format';
import { prefs } from '@/core/prefs';
import { toast } from '@/core/toast';
import { closeBtn, win } from '@/core/windows';
import { t, tl } from '@/i18n';
import { useJobs } from '@/stores/jobs';
import { useUser } from '@/stores/user';
import JobsBody from './JobsBody.vue';
import { jobLabel, took } from './label';
import './jobs.css';

// The queue settings live on the server; the prefs carry them for the settings page, the speed in MB/s.
function syncSettings() {
  const jobs = useJobs();
  const toPrefs = (s: JobSettings) => Object.assign(prefs, {
    jobMode: s.mode, speedCap: Math.round(s.cap / MB), parityWait: s.parityWait, notifyUnraid: s.notifyUnraid, notifyAfter: s.notifyAfter,
  });
  const fromPrefs = (): JobSettings => ({
    mode: prefs.jobMode === 'parallel' ? 'parallel' : 'serial', cap: (Number(prefs.speedCap) || 0) * MB,
    parityWait: prefs.parityWait !== false, notifyUnraid: !!prefs.notifyUnraid, notifyAfter: Number(prefs.notifyAfter ?? 60),
  });
  jobs.loadSettings().then((s) => {
    toPrefs(s);
    watch(fromPrefs, (want) => {
      const have = jobs.settings;
      if (have && JSON.stringify(have) === JSON.stringify({ ...have, ...want })) return;
      jobs.saveSettings(want).catch(() => {
        if (useUser().admin()) toast(t('jobs.settingsFailed'), 'warn');
        if (jobs.settings) toPrefs(jobs.settings);
      });
    }, { deep: true });
  }, () => {});
}

// A finished job is reported once, if it ran long enough to have been forgotten.
function notifyWhenLong(j: Job) {
  if (!prefs.notifyDone || !['done', 'failed'].includes(j.state) || typeof Notification === 'undefined' || Notification.permission !== 'granted') return;
  const secs = ((Date.parse(j.finishedAt || '') || Date.now()) - (Date.parse(j.startedAt || j.createdAt) || Date.now())) / 1000;
  if (secs < Number(prefs.notifyAfter ?? 60)) return;
  new Notification(j.state === 'done' ? t('jobs.notifyDone') : t('jobs.notifyFailed'), { body: t('jobs.notifyBody', { label: jobLabel(j), took: took(secs) }), tag: 'cannoncommander-job' });
}

export function jobsWindow() {
  const jobs = useJobs();
  jobs.load().catch(() => {});
  return win({
    title: t('jobs.title'), wide: true, body: JobsBody,
    foot: [{ start: t('jobs.start') }, { label: t('jobs.clear'), g: 'IconEmpty', run: () => { jobs.clear().catch(() => {}); return false; } }, closeBtn()],
  });
}

export default function setup() {
  const jobs = useJobs();
  action('jobs', { label: tl('jobs.title'), g: 'IconQueued', run: jobsWindow });
  jobs.load().catch(() => {});
  jobs.onEnded(notifyWhenLong);
  syncSettings();
}
