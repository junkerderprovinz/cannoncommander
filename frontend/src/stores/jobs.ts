// The server's jobs as the job.* events report them, plus the uploads this tab drives itself, which show up in the
// same list so one window and one count cover everything that runs.
import { defineStore } from 'pinia';
import { computed, reactive, ref, watch } from 'vue';
import { type Job, type JobConflict, type JobCreate, type JobProgress, type JobSettings, ops } from '@/api/ops';
import { on } from '@/api/sse';
import { shell } from '@/shell/state';

export const OPEN: Job['state'][] = ['queued', 'running', 'paused', 'asking'];
export const isOpen = (j: Pick<Job, 'state'>) => OPEN.includes(j.state);

// A job this tab runs on its own, such as an upload: the same shape, with the steps it answers to.
export interface LocalControl { pause(): void; resume(): void; cancel(): void; retry?(): void }

type Ended = (job: Job) => void;
type Asked = (job: Job, conflict: JobConflict) => void;
type Started = (job: Job, req: JobCreate) => void;

export const useJobs = defineStore('jobs', () => {
  const server = ref<Job[]>([]);
  const locals = reactive(new Map<string, { job: Job; control: LocalControl }>());
  const settings = ref<JobSettings | null>(null);
  const waiting = new Map<string, Ended[]>();
  const endedHooks: Ended[] = [];
  const askedHooks: Asked[] = [];
  const startedHooks: Started[] = [];

  const at = (j: Job) => Date.parse(j.finishedAt || j.createdAt) || 0;
  // Open jobs in queue order, then the ended ones, the newest first, as GET /jobs lists them.
  const list = computed(() => {
    const all = [...server.value, ...[...locals.values()].map((l) => l.job)];
    const open = all.filter(isOpen).sort((a, b) => a.position - b.position);
    return [...open, ...all.filter((j) => !isOpen(j)).sort((a, b) => at(b) - at(a))];
  });
  const running = computed(() => list.value.filter(isOpen));
  watch(() => running.value.length, (n) => { shell.jobCount = n; }, { immediate: true });

  function ended(job: Job) {
    const fns = waiting.get(job.id) ?? [];
    waiting.delete(job.id);
    fns.forEach((fn) => fn(job));
    endedHooks.forEach((fn) => fn(job));
  }

  function upsert(job: Job) {
    const i = server.value.findIndex((j) => j.id === job.id);
    const was = i >= 0 ? server.value[i]! : null;
    if (i >= 0) server.value.splice(i, 1, { ...was, ...job });
    else server.value.push(job);
    if (!isOpen(job) && (!was || isOpen(was))) ended(job);
    return job;
  }
  const find = (id: string) => list.value.find((j) => j.id === id);

  async function load() {
    const got = await ops.jobs();
    const known = new Map(server.value.map((j) => [j.id, j]));
    server.value = got;
    for (const j of got) if (!isOpen(j) && known.get(j.id) && isOpen(known.get(j.id)!)) ended(j);
  }

  // A job can end, and say so over the event stream, before the answer to the call that started or changed it
  // arrives. That answer must not put it back into the queue.
  function adopt(got: Job) {
    const known = find(got.id);
    return known && !isOpen(known) ? known : upsert(got);
  }

  // Jobs asked for that the server has not answered yet, so a queue read right after a click is not empty.
  const starting = ref(0);
  async function start(req: JobCreate) {
    starting.value++;
    try {
      const job = adopt(await ops.startJob(req));
      startedHooks.forEach((fn) => fn(job, req));
      return job;
    } finally {
      starting.value--;
    }
  }

  // Runs fn once the job has ended, whether it ended before the call or later.
  function whenDone(job: Job, fn: Ended) {
    const now = find(job.id) ?? job;
    if (!isOpen(now)) return fn(now);
    waiting.set(job.id, [...(waiting.get(job.id) ?? []), fn]);
  }
  const onEnded = (fn: Ended) => endedHooks.push(fn);
  const onAsked = (fn: Asked) => askedHooks.push(fn);
  // Only the jobs this tab asks for, with what it asked, which the job itself no longer says.
  const onStarted = (fn: Started) => startedHooks.push(fn);

  const local = (j: Job) => locals.get(j.id)?.control;
  async function step(j: Job, what: 'pause' | 'resume' | 'cancel' | 'retry') {
    const own = local(j);
    if (own) return own[what]?.();
    const got = await ops.jobStep(j.id, what);
    // Only a retry opens an ended job again.
    if (what === 'retry') upsert(got);
    else adopt(got);
  }
  const pause = (j: Job) => step(j, j.state === 'paused' ? 'resume' : 'pause');
  const cancel = (j: Job) => step(j, 'cancel');
  const retry = (j: Job) => step(j, 'retry');

  // Moves a job to another place among the open ones; the server renumbers the rest and says so in job.state.
  async function move(j: Job, to: number) {
    const open = running.value, from = open.indexOf(j);
    if (from < 0 || to < 0 || to >= open.length || to === from || local(j)) return;
    const order = [...open];
    order.splice(to, 0, order.splice(from, 1)[0]!);
    order.forEach((x, k) => { if (!local(x)) x.position = k; });
    adopt(await ops.changeJob(j.id, { position: to }));
  }
  async function setCap(j: Job, cap: number) {
    adopt(await ops.changeJob(j.id, { cap }));
  }
  async function decide(j: Job, key: string, choice: Parameters<typeof ops.decide>[1]['choice'], all = false) {
    adopt(await ops.decide(j.id, { key, choice, all }));
  }

  // Ended jobs leave the list; the uploads this tab ran go with them.
  async function clear() {
    server.value = server.value.filter(isOpen);
    for (const [id, l] of locals) if (!isOpen(l.job)) locals.delete(id);
    await ops.clearJobs();
  }

  function addLocal(job: Job, control: LocalControl) {
    locals.set(job.id, { job, control });
    return locals.get(job.id)!.job;
  }
  function localChanged(job: Job, was: Job['state']) {
    if (OPEN.includes(was) && !isOpen(job)) ended(job);
  }

  async function loadSettings() {
    settings.value = await ops.jobSettings();
    return settings.value;
  }
  async function saveSettings(change: Partial<JobSettings>) {
    const next = { ...(settings.value ?? (await loadSettings())), ...change };
    settings.value = await ops.saveJobSettings(next);
    return settings.value;
  }

  on('job.created', (e) => upsert(e.data as Job));
  on('job.state', (e) => upsert(e.data as Job));
  on('job.done', (e) => upsert(e.data as Job));
  on('job.progress', (e) => {
    const p = e.data as JobProgress, j = server.value.find((x) => x.id === p.id);
    if (j) Object.assign(j, p);
  });
  on('job.conflict', (e) => {
    const { id, conflict } = e.data as { id: string; conflict: JobConflict };
    const j = server.value.find((x) => x.id === id);
    if (!j) return;
    Object.assign(j, { state: 'asking', conflict });
    askedHooks.forEach((fn) => fn(j, conflict));
  });
  on('resync', () => { load().catch(() => {}); });

  return {
    list, running, starting, settings, load, start, whenDone, onEnded, onAsked, onStarted, find, pause, cancel, retry, move, setCap, decide, clear,
    addLocal, localChanged, loadSettings, saveSettings, upsert, adopt,
  };
});
