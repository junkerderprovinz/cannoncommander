// Prüfsummen erstellen und prüfen, Datei teilen und Zusammenfügen. Hashing and checking run as jobs on the server;
// the windows follow them as they read and show what they found.
import { reactive, watch } from 'vue';
import type { Job } from '@/api/ops';
import { type ChecksumResult, type CombineResult, type SplitResult, tools, type VerifyResult } from '@/api/tools';
import { action } from '@/core/actions';
import { GB } from '@/core/format';
import { toast } from '@/core/toast';
import { view } from '@/core/views';
import { cancelBtn, closeBtn, win } from '@/core/windows';
import { inArchive, join, split } from '@/files/paths';
import { jobError, kindLabels } from '@/features/jobs/label';
import { t, tl } from '@/i18n';
import { useJobs } from '@/stores/jobs';
import { usePanels } from '@/stores/panels';
import { plural, setPrimary, setStart } from '@/ui/kit';
import { clean, doneToast, namesIn, reveal, startJob } from '@/ui/tools';
import CombineBody, { type CombineState } from './CombineBody.vue';
import SplitBody, { type SplitState } from './SplitBody.vue';
import SumsBody, { type SumRow, type SumsState } from './SumsBody.vue';
import VerifyBody, { type VerifyRow, type VerifyState } from './VerifyBody.vue';
import { lineName, partSize, SUMFILE } from './sums';
import './checksum.css';

const outside = (dir: string) => (inArchive(dir) ? '/mnt/user/downloads' : dir);

function sumsWindow() {
  const F = usePanels(), jobs = useJobs(), dir = F.T().path, sel = F.sel();
  const loose = sel.filter((k) => !k.dir && !SUMFILE.test(k.name));
  if (!loose.length && !sel.some((k) => k.dir)) return toast(t('checksum.noFiles'), 'warn');
  const base = sel.length === 1 ? sel[0]!.name : (split(dir).pop() || 'pruefsummen').replace('::', '');
  const st: SumsState = reactive({ algo: 'sha256', rows: [], dir: outside(dir), name: '', touched: false, job: null, result: null });
  const fresh = (): SumRow[] => loose.map((k) => ({ name: k.name, size: k.size || 0, state: 'wait' }));

  async function compute() {
    if (st.job && jobs.find(st.job.id)?.state !== 'done') await jobs.cancel(st.job).catch(() => {});
    Object.assign(st, { rows: fresh(), job: null, result: null });
    if (!st.touched) st.name = `${base}.${st.algo}`;
    try {
      const job = await jobs.start({ kind: 'checksum', params: { sources: sel.map((k) => join(k.at || dir, k.name)), algo: st.algo } });
      st.job = job;
      jobs.whenDone(job, (j) => {
        if (st.job?.id !== j.id) return;
        if (j.state === 'done') {
          st.result = j.result as ChecksumResult;
          st.rows = st.result.files.map((f) => ({ name: f.name, size: 0, state: 'done', hash: f.hash }));
        } else if (j.state === 'failed') toast(t('ui.failed', { label: t('checksum.make'), error: jobError(j) }), 'fault');
      });
    } catch (e) {
      toast((e as Error).message, 'warn');
    }
  }
  const w = win({
    title: t('checksum.make'), wide: true, cls: 'bk bk-sums', body: SumsBody, props: { st },
    foot: [{ start: ' ' }, closeBtn(), { label: t('checksum.saveAs', { ext: st.algo }), g: 'IconSave', primary: true, disabled: true, run: () => { void save(); return false; } }],
    onClose: () => { const j = st.job && jobs.find(st.job.id); if (j && j.state !== 'done') void jobs.cancel(j).catch(() => {}); },
  });

  async function save() {
    const target = clean(st.dir), name = st.name.trim();
    if (!st.result || !name) return;
    const path = join(target, name);
    const text = st.result.files.map((f) => lineName(f.hash, f.path, target)).join('');
    try {
      await tools.saveText(path, text);
      void reveal(path);
      doneToast(t('checksum.saved', { path }), path);
    } catch (e) {
      toast(t('checksum.saveFailed', { error: (e as Error).message }), 'warn');
    }
  }
  watch(() => st.algo, compute, { immediate: true });
  watch(() => {
    const j = st.job ? jobs.find(st.job.id) : null;
    return [j?.doneFiles ?? 0, j?.totalFiles ?? 0, !!st.result, st.dir, st.name, st.algo] as const;
  }, ([done, total, ended]) => {
    const files = total || st.rows.length;
    setStart(w, ended ? t('checksum.finished', { algo: t(`checksum.algos.${st.algo}`), files: plural(st.rows.length, 'ui.files') })
      : t('checksum.reading', { n: Math.min(done + 1, Math.max(files, 1)), total: files }));
    setPrimary(w, { label: t('checksum.saveAs', { ext: st.algo }), disabled: !ended || !st.name.trim() });
  }, { immediate: true });
  // Files found inside folders appear as the job reaches them.
  watch(() => (st.job ? jobs.find(st.job.id)?.current : undefined), (cur) => {
    if (!cur || st.result) return;
    let at = st.rows.findIndex((r) => r.name === cur);
    if (at < 0) { st.rows.push({ name: cur, size: 0, state: 'read' }); at = st.rows.length - 1; }
    st.rows.forEach((r, i) => { r.state = i < at ? 'done' : i === at ? 'read' : r.state === 'done' ? 'done' : 'wait'; });
  });
}

function verifyWindow() {
  const F = usePanels(), jobs = useJobs(), files = view.id === 'files';
  const dir = files ? F.T().path : '/mnt/user/downloads';
  const cur = files ? F.sel().find((k) => !k.dir && SUMFILE.test(k.name)) : undefined;
  const st: VerifyState = reactive({ mode: cur ? 'file' : 'paste', file: cur ? join(cur.at || dir, cur.name) : '', text: '', dir, rows: null, empty: 'pick', seen: 0 });
  let asked = 0;
  async function check() {
    const n = ++asked;
    let lines;
    try {
      lines = await tools.parseSums(st.mode === 'file' ? { path: st.file.trim() } : { text: st.text });
    } catch {
      if (n === asked) Object.assign(st, { rows: null, empty: 'noFile' });
      return;
    }
    if (n !== asked) return;
    if (!lines.length) { Object.assign(st, { rows: null, empty: 'noSums' }); setStart(w, ''); return; }
    st.rows = lines.map((l): VerifyRow => ({ name: l.name || l.bad || '', state: 'wait', bad: !!l.bad }));
    st.seen = 0;
    try {
      const job = await jobs.start({ kind: 'verify', params: st.mode === 'file' ? { file: st.file.trim() } : { text: st.text, dir: clean(st.dir) } });
      const stop = watch(() => jobs.find(job.id)?.doneFiles ?? 0, (done) => { if (n === asked) st.seen = Math.min(done, st.rows!.length); });
      jobs.whenDone(job, (j: Job) => {
        stop();
        if (n !== asked) return;
        if (j.state !== 'done') { toast(t('ui.failed', { label: t('checksum.verify'), error: jobError(j) }), 'fault'); return; }
        (j.result as VerifyResult).lines.forEach((l, i) => Object.assign(st.rows![i]!, { state: l.state, algo: l.algo, found: l.state !== 'missing' && l.state !== 'bad' }));
        st.seen = st.rows!.length;
      });
    } catch (e) {
      toast((e as Error).message, 'warn');
    }
  }
  const w = win({
    title: t('checksum.verify'), wide: true, cls: 'bk bk-verify', body: VerifyBody, props: { st, check },
    foot: [{ start: ' ' }, closeBtn(), { label: t('checksum.check'), g: 'IconShieldOn', primary: true, run: () => { void check(); return false; } }],
  });
  watch(() => [st.seen, st.rows] as const, () => {
    const rows = st.rows;
    if (!rows) return setStart(w, '');
    const off = rows.filter((r) => r.state === 'mismatch').length;
    setStart(w, st.seen < rows.length ? t('checksum.checkingOf', { n: st.seen + 1, total: rows.length }) : off ? t('checksum.damagedHint') : '');
  }, { deep: true });
  if (cur) void check();
}

function splitWindow() {
  const F = usePanels(), dir = F.T().path, k = F.sel().find((x) => !x.dir)!;
  const st: SplitState = reactive({ k, size: k.size > 4 * GB ? '4096' : k.size > GB ? '1024' : '100', custom: '700', sum: true, dest: outside(dir), count: 0, ok: false });
  const w = win({
    title: t('checksum.split'), cls: 'bk', body: SplitBody, props: { st },
    foot: [{ start: t('ui.background') }, cancelBtn(), { label: t('checksum.split'), g: 'IconCompressMax', primary: true, disabled: true, run: () => {
      if (st.count < 2 || !st.ok) return false;
      const dest = clean(st.dest);
      void startJob('split', { path: join(k.at || dir, k.name), dest, partSize: partSize(st), sum: st.sum }, (j) => {
        const r = j.result as SplitResult;
        void reveal(r.first);
        doneToast(plural(r.parts, 'checksum.splitDone', { dest }), r.first);
      });
      return true;
    } }],
  });
  watch(() => [st.count, st.ok], () => setPrimary(w, { disabled: !st.ok || st.count < 2 }), { immediate: true });
}

function combineWindow() {
  const F = usePanels(), dir = F.T().path, pick = F.sel().find((k) => /\.\d{3,}$/.test(k.name));
  if (!pick) return toast(t('checksum.pickPart'), 'warn');
  const base = pick.name.replace(/\.\d{3,}$/, '');
  const st: CombineState = reactive({ dir, base, name: base, dest: outside(dir), check: namesIn(dir).has(base + '.sha256'), drop: false, file: '', ok: false });
  const w = win({
    title: t('checksum.combine'), cls: 'bk', body: CombineBody, props: { st },
    foot: [{ start: t('ui.background') }, cancelBtn(), { label: t('checksum.combine'), g: 'IconMerge', primary: true, disabled: true, run: () => {
      if (!st.ok) return false;
      const file = st.file, check = st.check;
      void startJob('combine', { path: join(dir, pick.name), name: st.name.trim(), dest: clean(st.dest), check, drop: st.drop }, (j) => {
        const r = j.result as CombineResult;
        void reveal(r.path);
        doneToast(t(r.checked ? 'checksum.combinedChecked' : 'checksum.combined', { path: r.path }), r.path);
      }, { failed: (j) => (check && /checksum/.test(j.error || '') ? t('checksum.sumWrong', { file }) : undefined) });
      return true;
    } }],
  });
  watch(() => st.ok, (ok) => setPrimary(w, { disabled: !ok }), { immediate: true });
}

export default function setup() {
  action('checksum', { label: tl('checksum.make'), g: 'IconShieldOn', needs: 'sel', run: sumsWindow });
  action('checksumVerify', { label: tl('checksum.verify'), g: 'IconShieldOn', run: verifyWindow });
  action('split', { label: tl('checksum.split'), g: 'IconCompressMax', needs: 'file', run: splitWindow });
  action('combine', { label: tl('checksum.combine'), g: 'IconMerge', needs: 'file', run: combineWindow });
  kindLabels.split = (_j, what) => t('checksum.jobSplit', { what });
  kindLabels.combine = (_j, what) => t('checksum.jobCombine', { what });
  kindLabels.checksum = (_j, what) => t('checksum.jobChecksum', { what });
  kindLabels.verify = (_j, what) => t('checksum.jobVerify', { what });
}

