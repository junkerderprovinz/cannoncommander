// The upload queue. Files go up in chunks, two at a time; a broken connection picks up at the last chunk the server
// kept, and so does the same file chosen again after a reload, because starting it again finds the running upload.
import { reactive } from 'vue';
import { ApiError } from '@/api/client';
import { type Choice, type Job, type JobConflict, ops, type Rule, type Upload } from '@/api/ops';
import { now } from '@/core/format';
import { toast } from '@/core/toast';
import { inArchive, join } from '@/files/paths';
import { t } from '@/i18n';
import { resolveConflicts } from '@/features/transfer/conflicts';
import { useJobs } from '@/stores/jobs';
import { useListing } from '@/stores/listing';
import { usePanels } from '@/stores/panels';

export type UpState = 'waiting' | 'running' | 'paused' | 'done' | 'cancelled' | 'failed';
export interface UpItem { id: number; batch: string; file: File; name: string; size: number; path: string; loaded: number; state: UpState; rule: Rule; upload?: Upload; stop?: AbortController }

const PARALLEL = 2;
const TRIES = 3;
const OPEN: UpState[] = ['waiting', 'running', 'paused'];

export const Up = reactive({ items: [] as UpItem[], job: null as Job | null, shown: false });
let seq = 0, batches = 0;

const iso = (ms: number) => new Date(ms || now()).toISOString();
const nameOf = (f: File) => (f as File & { webkitRelativePath?: string }).webkitRelativePath || f.name;

export async function addFiles(files: Iterable<File>, path: string) {
  const listing = useListing();
  const dir = !inArchive(path) && (await listing.load(path).catch(() => null));
  if (!dir) return toast(t('upload.notHere'), 'warn');
  const list = [...files].filter((f) => f.size || f.name);
  if (!list.length) return;
  const there = new Map(dir.items.map((k) => [k.name, k]));
  const clashes: JobConflict[] = list.filter((f) => there.has(nameOf(f))).map((f) => {
    const old = there.get(nameOf(f))!;
    return { key: nameOf(f), source: { path: nameOf(f), dir: false, size: f.size, mtime: iso(f.lastModified) }, target: { path: join(path, old.name), dir: old.dir, size: old.size, mtime: iso(old.time) },
      ...(f.lastModified > old.time + 1000 ? { newer: 'source' as const } : old.time > f.lastModified + 1000 ? { newer: 'target' as const } : {}) };
  });
  const decide: Record<string, Choice> | null = clashes.length ? await resolveConflicts(clashes, path) : {};
  if (!decide) return;
  const fresh = list.filter((f) => decide[nameOf(f)] !== 'skip');
  if (!fresh.length) return;
  const job = ensureJob(path);
  for (const f of fresh) {
    Up.items.push({ id: ++seq, batch: job.id, file: f, name: nameOf(f), size: f.size, path, loaded: 0, state: 'waiting', rule: decide[nameOf(f)] ?? 'ask' });
  }
  tick();
}

function ensureJob(path: string) {
  const jobs = useJobs();
  if (Up.job && ['running', 'paused'].includes(Up.job.state)) return Up.job;
  Up.job = jobs.addLocal({
    id: `upload-${++batches}`, kind: 'upload', owner: '', state: 'running', position: Number.MAX_SAFE_INTEGER, name: '', count: 0, dest: path,
    totalBytes: 0, doneBytes: 0, totalFiles: 0, doneFiles: 0, rate: 0, left: null, cap: 0, waitForParity: false, failedItems: 0, createdAt: iso(now()),
  }, { pause: toggleAll, resume: toggleAll, cancel: cancelAll });
  return Up.job;
}

const ofJob = () => Up.items.filter((i) => i.batch === Up.job?.id);

// Rate over the last few seconds, so a slow chunk does not make the time left jump about.
const samples: [number, number][] = [];
function measure(done: number) {
  const at = now();
  samples.push([at, done]);
  while (samples.length > 2 && at - samples[0]![0] > 5000) samples.shift();
  const [t0, d0] = samples[0]!;
  return at > t0 ? ((done - d0) / (at - t0)) * 1000 : 0;
}

function tick() {
  const job = Up.job;
  if (!job) return;
  const items = ofJob(), alive = items.filter((i) => i.state !== 'cancelled');
  const paused = job.state === 'paused';
  if (!paused) {
    const running = items.filter((i) => i.state === 'running');
    for (const i of items) if (running.length < PARALLEL && i.state === 'waiting') { running.push(i); void send(i); }
  }
  const total = alive.reduce((a, i) => a + i.size, 0), done = alive.reduce((a, i) => a + i.loaded, 0);
  const rate = measure(done);
  Object.assign(job, {
    totalBytes: total, doneBytes: done, totalFiles: alive.length, doneFiles: alive.filter((i) => i.state === 'done').length,
    count: items.length, name: items[0]?.name ?? '', dest: items[0]?.path ?? job.dest, rate, left: rate > 0 ? (total - done) / rate : null,
    current: items.find((i) => i.state === 'running')?.name,
  });
  if (!items.some((i) => OPEN.includes(i.state)) && job.state !== 'cancelled' && job.state !== 'done') finish(job, items);
}

function finish(job: Job, items: UpItem[]) {
  const was = job.state;
  const n = items.filter((i) => i.state === 'done').length, failed = items.filter((i) => i.state === 'failed').length;
  Object.assign(job, { state: failed && !n ? 'failed' : 'done', rate: 0, left: null, failedItems: failed, finishedAt: iso(now()) });
  useJobs().localChanged(job, was);
  const path = items[0]?.path || '/';
  if (n) toast(t('upload.done', { what: n === 1 ? t('upload.one') : t('upload.many', { n }), path }), 'ok', Up.shown ? undefined : { label: t('upload.go'), run: () => { usePanels().go(path); } });
  if (failed) toast(t('upload.failed', { n: failed }), 'fault');
}

async function begin(i: UpItem): Promise<Upload | null> {
  const req = { path: join(i.path, i.name), size: i.size, mtime: iso(i.file.lastModified), rule: i.rule };
  try {
    return await ops.uploadStart(req);
  } catch (e) {
    const conflict = e instanceof ApiError && e.status === 409 ? (e.problem as { conflict?: JobConflict } | undefined)?.conflict : undefined;
    if (!conflict) throw e;
    // The name turned up after the check, from somewhere else; it is asked like the others.
    const answer = await resolveConflicts([{ ...conflict, key: i.name }], i.path);
    const choice = answer?.[i.name];
    if (!choice || choice === 'skip') return null;
    i.rule = choice;
    return ops.uploadStart({ ...req, rule: choice });
  }
}

async function send(i: UpItem) {
  i.state = 'running';
  i.stop = new AbortController();
  let tries = 0;
  try {
    if (!i.upload) {
      const got = await begin(i);
      if (!got) { i.state = 'cancelled'; return; }
      i.upload = got;
    }
    while (i.state === 'running' && i.upload.state === 'open' && i.upload.offset < i.size) {
      const from = i.upload.offset, chunk = i.file.slice(from, from + (i.upload.chunk || 10 * 1024 * 1024));
      try {
        i.upload = await ops.uploadChunk(i.upload.id, from, chunk, i.stop.signal);
        tries = 0;
      } catch (e) {
        if (i.stop.signal.aborted) return;
        if (++tries > TRIES) throw e;
        // What arrived is kept; the server says where to go on.
        i.upload = await ops.uploadState(i.upload.id);
      }
      i.loaded = i.upload.offset;
      tick();
    }
    if (i.state !== 'running') return;
    if (i.upload.state === 'open' && i.upload.offset >= i.size) i.upload = await ops.uploadState(i.upload.id);
    i.loaded = i.size;
    i.state = i.upload.state === 'skipped' ? 'cancelled' : 'done';
    useListing().changed(i.path);
  } catch (e) {
    if (i.state === 'running') {
      i.state = 'failed';
      toast(t('upload.oneFailed', { name: i.name, error: (e as Error).message }), 'warn');
    }
  } finally {
    tick();
  }
}

export function pauseItem(i: UpItem) {
  if (i.state === 'paused') i.state = 'waiting';
  else {
    i.stop?.abort();
    i.state = 'paused';
  }
  tick();
}

export function cancelItem(i: UpItem) {
  i.stop?.abort();
  i.state = 'cancelled';
  if (i.upload?.state === 'open') ops.uploadCancel(i.upload.id).catch(() => {});
  tick();
}

export function toggleAll() {
  const job = Up.job;
  if (!job) return;
  job.state = job.state === 'paused' ? 'running' : 'paused';
  for (const i of ofJob()) if (job.state === 'paused' && i.state === 'running') { i.stop?.abort(); i.state = 'waiting'; }
  tick();
}

function cancelAll() {
  const job = Up.job;
  if (!job) return;
  ofJob().filter((i) => OPEN.includes(i.state)).forEach(cancelItem);
  const was = job.state;
  Object.assign(job, { state: 'cancelled', rate: 0, left: null, finishedAt: iso(now()) });
  useJobs().localChanged(job, was);
}

export const pausedAll = () => Up.job?.state === 'paused';
export const clearDone = () => { Up.items = Up.items.filter((i) => OPEN.includes(i.state)); };
