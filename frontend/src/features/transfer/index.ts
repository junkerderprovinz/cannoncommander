// Kopieren and Verschieben: the transfer window (F5, F6), the clipboard, and the path every transfer takes, also one
// dropped onto a folder: the check with ops/preflight, Unraid's warnings, the parity question, the conflicts, the job.
import { reactive, watch } from 'vue';
import { ApiError } from '@/api/client';
import { type Job, ops, type Preflight, type TransferParams } from '@/api/ops';
import { action, run } from '@/core/actions';
import { prefs } from '@/core/prefs';
import { toast } from '@/core/toast';
import { win, type WinEntry } from '@/core/windows';
import { type Entry, toEntry } from '@/files/entries';
import { parent, split } from '@/files/paths';
import { plural, t, tl } from '@/i18n';
import { diskName, jobError, jobLabel, objects, placeName } from '@/features/jobs/label';
import { undoAct } from '@/features/undo/act';
import { useUnraid } from '@/features/unraid/state';
import { useClipboard } from '@/stores/clipboard';
import { useJobs } from '@/stores/jobs';
import { useListing } from '@/stores/listing';
import { type Transfer, usePanels } from '@/stores/panels';
import { ask } from './ask';
import { entriesOf, keep } from './clip';
import { askRunning, resolveConflicts } from './conflicts';
import LinksBody from './LinksBody.vue';
import ParityBody from './ParityBody.vue';
import { ensureDir, pathOf, shareRel } from './paths';
import SameFileBody, { type SafeWay } from './SameFileBody.vue';
import TransferBody, { type TransferState } from './TransferBody.vue';
import './transfer.css';

const verbOf = (kind: 'copy' | 'move') => t(`transfer.${kind}`);

// The computer's clipboard, which desktop.ts hands in, since only the desktop app reaches it.
let outside: typeof import('./outside') | undefined;
export const reachOutside = (m: typeof import('./outside')) => { outside = m; };
const share = (cut: boolean, from: string, rows: Entry[]) => (outside ? outside.share(cut, from, rows) : keep(cut, from, rows));
const warn = (e: unknown) => toast(e instanceof ApiError ? e.message : String(e), 'warn');

function paramsOf(job: Transfer): TransferParams {
  return {
    sources: job.items.map((k) => pathOf(k, job.from)), dest: job.dest,
    rule: job.rule ?? 'ask', verify: !!job.verify, keepAttrs: job.keepAttrs ?? true,
  };
}

// Hard links stand in for a copy where source and target share a disk; each item gets a second name in the target.
async function makeLinks(job: Transfer, files: number) {
  try {
    for (const k of job.items) await ops.link(pathOf(k, job.from), job.dest + '/' + k.name);
  } catch (e) {
    return toast(e instanceof ApiError && e.status === 409 ? t('transfer.noLink') : String((e as Error).message), 'warn');
  }
  const n = Math.max(files, job.items.length);
  toast(plural(n, 'transfer.linked'));
}

function askParity(size: number) {
  return ask<'now' | 'later'>({ title: t('transfer.parityTitle'), cls: 'xwin', body: ParityBody, props: { size } }, [
    { label: t('transfer.parityLater'), g: 'IconQueued', answer: 'later' },
    { label: t('transfer.parityNow'), g: 'IconPlay', primary: true, answer: 'now' },
  ], false);
}

function linkWindow(job: Transfer, pf: Preflight) {
  const u = pf.unraid!, kind = job.kind as 'copy' | 'move';
  return ask<'link' | 'go'>({ title: t('transfer.linksTitle'), wide: true, cls: 'bk u-win', body: LinksBody, props: { kind, dest: job.dest, links: u.links, linkable: u.linkable } }, [
    ...(u.linkable ? [{ label: t('transfer.linkWay'), g: 'IconLink', answer: 'link' as const }] : []),
    { label: t(`transfer.anyway.${kind}`), g: kind === 'copy' ? 'IconCopy' : 'IconMove', primary: true, answer: 'go' as const },
  ]);
}

// The trap names both safe ways: disk to disk, or share to share, which is the same folder seen from above.
function sameFileWindow(job: Transfer, pf: Preflight) {
  const trap = pf.unraid!.trap!, kind = job.kind as 'copy' | 'move', verb = verbOf(kind);
  const fromUser = job.from.startsWith('/mnt/user');
  const src = fromUser ? trap.device : split(job.from)[1]!;
  const relFrom = shareRel(job.from), relDest = shareRel(job.dest);
  const sub = (rel: string) => (rel ? '/' + rel : '');
  const shareFrom = '/mnt/user' + sub(relFrom), shareTo = '/mnt/user' + sub(relDest), diskFrom = `/mnt/${src}` + sub(relFrom);
  const st = reactive<SafeWay>({ way: 'disk', disk: trap.targets[0]?.disk ?? '' });
  const props = reactive({ kind, from: job.from, dest: job.dest, trap, st, plan: '' });
  const primary = { label: t(`transfer.safely.${kind}`), g: 'IconShieldOn', primary: true, disabled: false, run: () => { go(); } };
  const w: WinEntry = win({ title: t('transfer.sameTitle'), wide: true, cls: 'bk u-win', body: SameFileBody, props, foot: [{ start: t('transfer.background') }, primary] });
  watch(st, () => {
    const nothing = st.way === 'share' && shareFrom === shareTo;
    props.plan = st.way === 'disk' ? t('transfer.planWay', { verb, from: diskFrom, to: `/mnt/${st.disk}` + sub(relDest) })
      : nothing ? t('transfer.planNothing') : t('transfer.planWay', { verb, from: shareFrom, to: shareTo });
    w.foot = w.foot.map((f) => (f.primary ? { ...f, disabled: nothing } : f));
  }, { immediate: true });
  async function go() {
    const F = usePanels(), options = { rule: job.rule, verify: job.verify, keepAttrs: job.keepAttrs };
    if (st.way === 'share') return F.transfer(kind, job.items, shareTo, shareFrom, options);
    const to = `/mnt/${st.disk}` + sub(relDest), names = new Set(job.items.map((k) => k.name));
    const found = (await useListing().load(diskFrom).catch(() => null))?.items.filter((k) => names.has(k.name)).map(toEntry) ?? [];
    if (!found.length) return toast(t('transfer.nothingOn', { disk: diskName(src) }), 'warn');
    await ensureDir(to);
    return F.transfer(kind, found, to, diskFrom, options);
  }
}

// A file whose target is the same file under another path is left alone, so the toast counts those and says what
// went over in their place.
export function doneText(done: Job, kind: 'copy' | 'move', what: string, to: string) {
  const same = (done.result as { same?: number } | undefined)?.same ?? 0;
  if (!same) return t(kind === 'copy' ? 'transfer.copied' : 'transfer.moved', { what, to });
  if (!done.doneFiles) return plural(same, kind === 'copy' ? 'transfer.nothingCopied' : 'transfer.nothingMoved');
  return t(kind === 'copy' ? 'transfer.copiedSame' : 'transfer.movedSame', { files: plural(done.doneFiles, 'jobs.files'), to, same: plural(same, 'transfer.sameSkipped') });
}

function announce(job: Job, kind: 'copy' | 'move') {
  const what = job.count === 1 ? job.name : objects(job.count), to = placeName(job.dest);
  useJobs().whenDone(job, (done) => {
    if (done.state === 'done') {
      usePanels().T().marked = new Set();
      toast(doneText(done, kind, what, to), 'ok', done.undo ? undoAct(done.undo) : undefined);
    } else if (done.state === 'failed') toast(done.error ? t('transfer.failed', { label: jobLabel(done), error: jobError(done) }) : t('transfer.failedPlain', { label: jobLabel(done) }), 'fault');
  });
}

export async function transfer(job: Transfer) {
  if (job.kind === 'link') return makeLinks(job, 0);
  const kind = job.kind, params = paramsOf(job);
  let pf: Preflight;
  try { pf = await ops.preflight(kind, params); } catch (e) { return warn(e); }
  const u = pf.unraid;
  if (u?.trap) return sameFileWindow(job, pf);
  if (u?.links?.count) {
    const answer = await linkWindow(job, pf);
    if (answer === 'link') return makeLinks(job, u.links.count);
    if (answer !== 'go') return;
  }
  let waitForParity = false;
  if (pf.parityBusy && prefs.parityWait !== false) {
    if (!pf.big) {
      const answer = await askParity(pf.bytes);
      if (!answer) return;
      waitForParity = answer === 'later';
    } else toast(t('transfer.parityWaits'), 'warn');
  }
  if (pf.conflicts.length && params.rule === 'ask') {
    const decisions = await resolveConflicts(pf.conflicts, job.dest);
    if (!decisions) return;
    params.decisions = decisions;
  }
  if (u?.wakes.length) useUnraid().wake(u.wakes);
  try {
    const started = await useJobs().start({ kind, params, waitForParity });
    toast(t('transfer.running', { label: jobLabel(started) }), 'ok', { label: t('transfer.toJobs'), run: () => run('jobs') });
    announce(started, kind);
    return started;
  } catch (e) { return warn(e); }
}

function transferWindow(kind: 'copy' | 'move') {
  const F = usePanels();
  const items = F.sel(), from = F.T().path;
  const st: TransferState = reactive({
    kind, way: kind, items, from, dest: prefs.dual ? F.T(F.other()).path : parent(from),
    rule: 'ask', keepAttrs: true, verify: false, pf: null,
  });
  const verb = verbOf(kind);
  const primary = () => ({
    label: st.pf?.unraid?.space ? t(`transfer.anyway.${kind}`) : verb, g: kind === 'copy' ? 'IconCopy' : 'IconMove', primary: true,
    run: () => { F.transfer(st.way, items, st.dest.trim().replace(/(.)\/$/, '$1'), from, { rule: st.rule, keepAttrs: st.keepAttrs, verify: st.verify }); },
  });
  const w = win({ title: verb, cls: 'xwin', body: TransferBody, props: { st }, foot: [{ start: t('transfer.background') }, primary()] });
  // The check runs again as the target changes, so the space warning and the hard link choice follow it.
  let asked = 0, timer: ReturnType<typeof setTimeout> | undefined;
  watch(() => st.dest, (raw) => {
    clearTimeout(timer);
    timer = setTimeout(() => {
      const dest = raw.trim().replace(/(.)\/$/, '$1'), n = ++asked;
      if (!dest || dest === from) { st.pf = null; return; }
      ops.preflight(kind, { sources: items.map((k) => pathOf(k, from)), dest, rule: 'ask' })
        .then((pf) => { if (n === asked) st.pf = pf; }, () => { if (n === asked) st.pf = null; });
    }, 150);
  }, { immediate: true });
  watch(() => st.pf, () => { w.foot = w.foot.map((f) => (f.primary ? primary() : f)); });
}

// Ctrl+V and the menu: in the desktop app the files on the computer's clipboard, else the names in the app's own. It
// says whether there was anything, so the paste event can turn a picture or a text into a file instead.
async function paste() {
  const F = usePanels(), clip = useClipboard(), dest = F.T().path;
  const got = await outside?.clipboardWins();
  if (got && (got.files.length || got.skipped)) {
    await outside!.pasteOutside(got, dest);
    return true;
  }
  const c = clip.clip;
  if (!c) return false;
  try {
    const items = c.paths ? await entriesOf(c.paths) : (await useListing().load(c.from)).items.filter((k) => c.items.includes(k.name)).map(toEntry);
    await F.transfer(c.cut ? 'move' : 'copy', items, dest, c.paths ? parent(c.paths[0]!) : c.from);
    if (c.cut) clip.clear();
  } catch (e) {
    warn(e);
  }
  return true;
}

export default function setup() {
  const F = usePanels(), clip = useClipboard(), jobs = useJobs();
  action('copy', { label: tl('transfer.copy'), g: 'IconCopy', key: 'F5', needs: 'sel', run: () => transferWindow('copy') });
  action('move', { label: tl('transfer.move'), g: 'IconMove', key: 'F6', needs: ['sel', 'writable'], run: () => transferWindow('move') });
  action('transfer', { label: tl('transfer.copy'), g: 'IconCopy', hidden: true, run: (job: Transfer) => transfer(job) });

  action('clipCut', { label: tl('transfer.clipCut'), g: 'IconMove', key: 'Ctrl+x', needs: ['sel', 'writable'], run: () => {
    const rows = F.sel();
    void share(true, F.T().path, rows);
    toast(plural(rows.length, 'transfer.cut'));
  } });
  action('clipCopy', { label: tl('transfer.clipCopy'), g: 'IconCopy', key: 'Ctrl+c', needs: 'sel', run: () => {
    const rows = F.sel();
    void share(false, F.T().path, rows);
    toast(plural(rows.length, 'transfer.inClip'));
  } });
  action('clipPaste', { label: tl('transfer.clipPaste'), g: 'IconPaste', key: 'Ctrl+v', needs: 'writable', when: () => clip.ready(), run: async () => {
    if (!(await paste())) toast(t('transfer.clipEmpty'), 'warn');
  } });
  action('pasteFiles', { label: tl('transfer.clipPaste'), g: 'IconPaste', hidden: true, needs: 'writable', run: paste });
  // A job that meets a name on its way asks here, unless someone answers it in another tab first.
  jobs.onAsked((job, conflict) => askRunning(job, conflict));
}
