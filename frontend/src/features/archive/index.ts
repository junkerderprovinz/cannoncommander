// Verpacken, Entpacken and Archiv testen. All three run as jobs; a damaged archive keeps a red mark in the list until
// a test passes again.
import { h, reactive, watch } from 'vue';
import { ApiError } from '@/api/client';
import { type PackResult, type TestedArchive, tools, type UnpackResult } from '@/api/tools';
import { action } from '@/core/actions';
import { addTo } from '@/core/extend';
import { fmtSize } from '@/core/format';
import { MENUS } from '@/core/menus';
import { prefs } from '@/core/prefs';
import { toast } from '@/core/toast';
import { cancelBtn, closeBtn, win } from '@/core/windows';
import type { Entry } from '@/files/entries';
import { inArchive, join, parent, split } from '@/files/paths';
import { kindLabels } from '@/features/jobs/label';
import { t, tl } from '@/i18n';
import { useJobs } from '@/stores/jobs';
import { usePanels } from '@/stores/panels';
import { plural, setPrimary, setStart } from '@/ui/kit';
import { clean, doneToast, reveal, startJob } from '@/ui/tools';
import PackBody, { type PackState } from './PackBody.vue';
import TestBody, { type TestRow } from './TestBody.vue';
import UnpackBody, { type UnpackState } from './UnpackBody.vue';
import { stripArchive } from './names';
import './archive.css';

const isArchive = (k: Entry) => !k.dir && (!!k.archive || k.kind === 'arch' || /\.(7z|rar|zip)\.0*1$/i.test(k.name));

let formats: Promise<string[]> | null = null;
const packFormats = () => (formats ??= tools.archiveFormats().then((f) => f.pack, () => { formats = null; return []; }));

// What an archive would weigh, from how well each kind of file packs, so the window can say it before the job runs.
function estimate(items: Entry[], fmt: string, level: number) {
  const squeezed = (k: Entry) => (k.dir ? 0.6 : ['img', 'video', 'audio', 'arch', 'disk'].includes(k.kind) ? 0.97 : ['pdf', 'office'].includes(k.kind) ? 0.82 : 0.3);
  const per = (k: Entry) => (k.size || 0) * (level === 0 ? 1 : squeezed(k) * (1.06 - level * 0.012) * (fmt === '7z' ? 0.94 : fmt === 'tar.zst' ? 0.96 : 1));
  return Math.max(120, items.reduce((a, k) => a + per(k), 0));
}

function packWindow() {
  const F = usePanels(), src = F.T().path, sel = F.sel();
  const other = prefs.dual ? F.T(F.other()).path : src;
  const one = sel.length === 1 ? sel[0]! : null;
  const name = one ? (one.dir ? one.name : stripArchive(one.name.replace(/\.[^.]+$/, '')) || one.name) : (split(src).pop() || 'Archiv').replace('::', '');
  const st: PackState = reactive({
    items: sel, name, format: 'zip', level: '6', password: '', dest: inArchive(other) ? parent(src) : other, formats: ['zip', 'tar.zst', '7z', 'rar'], file: '',
  });
  packFormats().then((can) => { if (can.length) st.formats = st.formats.filter((f) => can.includes(f)); });
  const w = win({
    title: t('archive.pack'), cls: 'bk', body: PackBody, props: { st },
    foot: [{ start: ' ' }, cancelBtn(), { label: t('archive.pack'), g: 'IconArchive', primary: true, disabled: true, run: () => {
      if (!st.file) return false;
      const dest = clean(st.dest);
      void startJob('pack', {
        sources: sel.map((k) => join(k.at || src, k.name)), dest, name: st.name.trim(), format: st.format,
        level: Number(st.level), ...(st.password ? { password: st.password } : {}),
      }, (j) => {
        const path = (j.result as PackResult).path;
        void reveal(path);
        doneToast(t('archive.packed', { path }), path);
      });
      return true;
    } }],
  });
  watch(() => [st.file, st.format, st.level], () => {
    setStart(w, t('archive.about', { size: fmtSize(estimate(sel, st.format, Number(st.level))) }));
    setPrimary(w, { disabled: !st.file });
  }, { immediate: true });
}

function unpackWindow() {
  const F = usePanels(), src = F.T().path, inside = inArchive(src), sel = F.sel();
  const archives = inside ? [] : sel.filter(isArchive);
  if (!inside && !archives.length) return toast(t('archive.pickArchive'), 'warn');
  const other = F.T(F.other()).path;
  const st: UnpackState = reactive({
    inside, items: inside ? sel : archives, archive: inside ? split(src).find((p) => p.endsWith('::'))!.replace('::', '') : '',
    dest: inside ? (inArchive(other) ? '/mnt/user/downloads' : other) : prefs.dual && !inArchive(other) ? other : src,
    own: !inside, clash: 'both', password: '', locked: false, ok: false, size: inside ? sel.reduce((a, k) => a + (k.size || 0), 0) : null,
  });
  const paths = archives.map((k) => join(k.at || src, k.name));
  // The unpacked size comes from each archive's index, which also tells whether one needs its password.
  if (!inside) {
    Promise.all(paths.map((p) => tools.archiveInfo(p).catch((e) => {
      if (e instanceof ApiError && e.status === 428) st.locked = true;
      return null;
    }))).then((infos) => {
      if (infos.some((i) => i?.encrypted)) st.locked = true;
      if (infos.every(Boolean)) st.size = infos.reduce((a, i) => a + i!.unpacked, 0);
    });
  }
  const w = win({
    title: t('archive.unpack'), cls: 'bk', body: UnpackBody, props: { st },
    foot: [{ start: t('ui.background') }, cancelBtn(), { label: t('archive.unpack'), g: 'IconArchive', primary: true, disabled: true, run: () => {
      if (!st.ok) return false;
      const dest = clean(st.dest);
      void startJob('unpack', {
        archives: inside ? [src] : paths, ...(inside ? { items: sel.map((k) => k.name) } : {}), dest, own: st.own, clash: st.clash,
        ...(st.password ? { password: st.password } : {}),
      }, (j) => {
        const r = j.result as UnpackResult, first = r.paths[0];
        const path = st.own && first ? first : dest;
        if (first) void reveal(first);
        doneToast(r.skipped ? t('archive.unpackedSkipped', { path, n: r.skipped }) : t('archive.unpacked', { path }), path);
      });
      return true;
    } }],
  });
  watch(() => [st.ok, st.size], () => {
    setPrimary(w, { disabled: !st.ok });
    setStart(w, st.size != null ? t('archive.yields', { size: fmtSize(st.size) || '0 B' }) : t('ui.background'));
  }, { immediate: true });
}

// Damage found by the last test, by path, until a test of the same archive passes.
const damaged = reactive(new Map<string, string>());
const damageOf = (a: TestedArchive) => (a.damage ? (a.damage.entry ? t('archive.damage', { reason: a.damage.reason, entry: a.damage.entry }) : a.damage.reason) : '');

function testWindow() {
  const F = usePanels(), dir = F.T().path, sel = F.sel().filter(isArchive);
  if (!sel.length) return toast(t('archive.pickArchives'), 'warn');
  const rows: TestRow[] = reactive(sel.map((k) => ({ k, path: join(k.at || dir, k.name), size: k.size || 0, members: 0, done: 0, state: 'waiting' as TestRow['state'], why: '' })));
  const st = reactive({ rows, sum: t('archive.checking'), job: '' });
  const w = win({
    title: sel.length === 1 ? t('archive.test') : t('archive.testMany', { n: sel.length }), wide: true, cls: 'bk bk-atest', body: TestBody, props: { st },
    foot: [{ start: t('archive.everyMember') }, closeBtn()],
  });
  const jobs = useJobs();
  // While the job reads, its bytes tell how far it got through the archives in their order.
  watch(() => (st.job ? jobs.find(st.job)?.doneBytes ?? 0 : 0), (done) => {
    let at = 0;
    for (const r of rows) {
      if (r.state === 'ok' || r.state === 'broken' || r.state === 'locked') continue;
      const part = r.size ? Math.min(1, Math.max(0, (done - at) / r.size)) : done > at ? 1 : 0;
      r.done = part;
      r.state = part > 0 ? 'testing' : 'waiting';
      at += r.size;
    }
  });
  startJob('testArchive', { paths: rows.map((r) => r.path) }, (j) => {
    for (const a of (j.result as { archives: TestedArchive[] }).archives) {
      const r = rows.find((x) => x.path === a.path);
      if (!r) continue;
      Object.assign(r, { members: a.members, done: 1, state: a.password ? 'locked' : a.ok ? 'ok' : 'broken', why: a.password ? t('archive.lockedTip') : damageOf(a) });
      if (a.ok) damaged.delete(a.path);
      else if (a.damage) damaged.set(a.path, damageOf(a));
    }
    const bad = rows.filter((r) => r.state === 'broken').length;
    st.sum = bad ? plural(bad, 'archive.broken') : rows.length === 1 ? t('archive.oneOk') : t('archive.allOk');
    setStart(w, bad ? t('archive.stayRed') : t('archive.fine'));
  }).then((started) => { st.job = started?.id ?? ''; });
}

export default function setup() {
  action('pack', { label: tl('archive.pack'), g: 'IconArchive', key: 'Alt+P', needs: 'sel', run: packWindow });
  action('unpack', { label: tl('archive.unpack'), g: 'IconArchive', key: 'Alt+U', needs: 'sel', run: unpackWindow });
  action('testArchive', { label: tl('archive.test'), g: 'IconCheck', needs: 'sel', run: testWindow });
  kindLabels.pack = (_j, what) => t('archive.jobPack', { what });
  kindLabels.unpack = (_j, what) => t('archive.jobUnpack', { what });
  kindLabels.testArchive = (_j, what) => t('archive.jobTest', { what });
  addTo('nameBadges', (k, path) => {
    const why = damaged.get(join(k.at || path, k.name));
    return why ? h('span', { class: 'pill fault bk-damaged', 'data-tip': why }, t('archive.damaged')) : null;
  });
  const file = MENUS.find(([id]) => id === 'file')![1];
  if (!file.includes('testArchive')) file.splice(file.indexOf('unpack') + 1, 0, 'testArchive');
}
