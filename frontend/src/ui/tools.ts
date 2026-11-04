// What the tool windows do with the panels and the job queue: check a typed folder, start a job and say how it went,
// and put the cursor on what a tool made.
import { type Ref, ref, watch } from 'vue';
import type { Job } from '@/api/ops';
import { run } from '@/core/actions';
import { toast } from '@/core/toast';
import { inArchive, parent, split } from '@/files/paths';
import { jobLabel } from '@/features/jobs/label';
import { t } from '@/i18n';
import { useJobs } from '@/stores/jobs';
import { useListing } from '@/stores/listing';
import { usePanels } from '@/stores/panels';
import { copyText } from './kit';

export const clean = (p: string) => p.trim().replace(/(.)\/$/, '$1');

// Whether a typed path is a folder a tool can write to, asked again as the path changes. null while it is asked.
export function folderState(path: () => string): Ref<boolean | null> {
  const state = ref<boolean | null>(null);
  const listing = useListing();
  let asked = 0;
  watch(path, (raw) => {
    const p = clean(raw), n = ++asked;
    if (!p.startsWith('/') || inArchive(p)) { state.value = false; return; }
    if (listing.lists.has(p)) { state.value = true; return; }
    state.value = null;
    listing.load(p).then(() => { if (n === asked) state.value = true; }, () => { if (n === asked) state.value = false; });
  }, { immediate: true });
  return state;
}

// Names in a folder the panels have read, to tell beforehand which one a tool would take.
export const namesIn = (dir: string) => new Set(useListing().lists.get(dir)?.items.map((k) => k.name) ?? []);
// The name the server picks when one is taken: a number before the ending.
export function freeName(names: Set<string>, name: string) {
  if (!names.has(name)) return name;
  const dot = name.lastIndexOf('.'), stem = dot > 0 ? name.slice(0, dot) : name, end = dot > 0 ? name.slice(dot) : '';
  for (let n = 1; ; n++) if (!names.has(`${stem} (${n})${end}`)) return `${stem} (${n})${end}`;
}

// Puts the cursor on a name in every panel that shows its folder, once the folder is read again.
export async function reveal(path: string) {
  const dir = parent(path), name = split(path).pop()!, F = usePanels(), listing = useListing();
  listing.changed(dir);
  await listing.load(dir).catch(() => null);
  F.panels.forEach((_, i) => {
    if (F.T(i).path !== dir) return;
    const idx = F.entries(i).findIndex((e) => e.name === name);
    if (idx >= 0) F.T(i).cursor = idx;
  });
}

export const doneToast = (text: string, path: string) => toast(text, 'ok', { label: t('ui.copyPath'), run: () => { void copyText(path); } });

// Starts a job, says that it runs and hands the finished job to done. A failure says why, in the words of how.failed
// where it has some.
export async function startJob(kind: string, params: unknown, done: (j: Job) => void, how: { running?: string; failed?: (j: Job) => string | void } = {}) {
  const jobs = useJobs();
  try {
    const job = await jobs.start({ kind, params });
    toast(how.running || t('ui.running', { label: jobLabel(job) }), 'ok', { label: t('ui.toJobs'), run: () => run('jobs') });
    jobs.whenDone(job, (j) => {
      if (j.state === 'done') done(j);
      else if (j.state === 'failed') toast(how.failed?.(j) || t('ui.failed', { label: jobLabel(j), error: j.error || '' }), 'fault');
    });
    return job;
  } catch (e) {
    toast((e as Error).message, 'warn');
    return null;
  }
}
