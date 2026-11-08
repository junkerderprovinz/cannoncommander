// Symbolische Verknüpfung and Eigenschaften und Rechte: what an entry is, who owns it and who may do what with it.
import { reactive, watch } from 'vue';
import { api, type Item } from '@/api/client';
import type { Job } from '@/api/ops';
import { tools, type WinFlag } from '@/api/tools';
import { action, run } from '@/core/actions';
import { host } from '@/core/host';
import { prefs } from '@/core/prefs';
import { toast } from '@/core/toast';
import { go, view } from '@/core/views';
import { cancelBtn, win } from '@/core/windows';
import { inArchive, join, parent, split } from '@/files/paths';
import { jobError, jobLabel, kindLabels } from '@/features/jobs/label';
import { t, tl } from '@/i18n';
import { useJobs } from '@/stores/jobs';
import { useListing } from '@/stores/listing';
import { usePanels } from '@/stores/panels';
import { plural, setPrimary, setStart } from '@/ui/kit';
import { clean, doneToast, reveal } from '@/ui/tools';
import LinkBody, { type LinkState } from './LinkBody.vue';
import PropsBody, { type PropsState } from './PropsBody.vue';
import './props.css';

function symlinkWindow() {
  const F = usePanels(), dir = F.T().path, k = F.sel()[0]!;
  const other = F.T(F.other()).path;
  const st: LinkState = reactive({
    target: inArchive(dir) ? '' : join(k.at || dir, k.name), where: prefs.dual && !inArchive(other) ? other : inArchive(dir) ? '/mnt/user' : dir,
    name: k.name, ok: false, missing: false,
  });
  const w = win({
    title: t('props.symlink'), cls: 'bk', body: LinkBody, props: { st },
    foot: [{ start: ' ' }, cancelBtn(), { label: t('props.create'), g: 'IconLink', primary: true, disabled: true, run: () => {
      if (!st.ok) return false;
      const path = join(clean(st.where), st.name.trim());
      tools.symlink(st.target.trim(), path).then(() => {
        void reveal(path);
        doneToast(t('props.linked', { path }), path);
      }, (e) => toast(e.message, 'warn'));
      return true;
    } }],
  });
  watch(() => [st.ok, st.missing], () => {
    setPrimary(w, { disabled: !st.ok });
    setStart(w, st.missing ? t('props.noTarget') : '');
  }, { immediate: true });
}

// Windows keeps attributes instead of the nine rights, and the properties show those Explorer does.
const FLAGS: [Exclude<WinFlag, 'pinned'>, string][] = [['readOnly', 'R'], ['hidden', 'H'], ['archive', 'A']];

// Names the panels cannot look up anywhere: the ones Unraid uses for its shares.
const UIDS: Record<string, number> = { root: 0, nobody: 99 };
const GIDS: Record<string, number> = { root: 0, users: 100 };

async function propsWindow() {
  if (view.id !== 'files') go('files');
  const F = usePanels(), listing = useListing(), dir = F.T().path, sel = F.sel();
  const self = !sel.length;
  let nodes: Item[] = sel;
  if (self) {
    const up = dir === '/' ? null : await listing.load(parent(dir)).catch(() => null);
    const name = split(dir).pop() || '';
    nodes = [up?.items.find((k) => k.name === name) ?? { name, dir: true, size: 0, time: Date.now(), mode: '0755', owner: 'root', group: 'root' }];
  }
  const paths = self ? [dir] : sel.map((k) => join(k.at || dir, k.name));
  const ro = inArchive(dir) || (dir === '/' && self);
  const uids = { ...UIDS }, gids = { ...GIDS };
  for (const k of nodes) {
    if (k.owner && k.uid != null) uids[k.owner] = k.uid;
    if (k.group && k.gid != null) gids[k.group] = k.gid;
  }
  const modes = [...new Set(nodes.map((k) => (k.mode || (k.dir ? '0775' : '0664')).padStart(4, '0')))];
  const owners = [...new Set(nodes.map((k) => k.owner || 'nobody'))], groups = [...new Set(nodes.map((k) => k.group || 'users'))];
  const st: PropsState = reactive({
    nodes, paths, dir, self, ro, uids, gids, modes, owners, groups, mode: modes[0]!, owner: owners.length > 1 ? '' : owners[0]!, group: groups.length > 1 ? '' : groups[0]!,
    rec: false, size: nodes.reduce((a, k) => a + (k.size || 0), 0), files: nodes.filter((k) => !k.dir).length, dirs: 0, counted: !nodes.some((k) => k.dir),
    label: self ? (listing.placeLabel(dir) || split(dir).pop() || t('props.server')).replace('::', '') : '',
    win: host.os === 'windows', flags: {}, flagsAt: {},
  });
  if (st.win) {
    for (const [f, letter] of FLAGS) st.flags[f] = st.flagsAt[f] = nodes.every((k) => (k.attrs ?? '').includes(letter));
    // A listing on Windows leaves the owner out, since it costs a call of its own for every file.
    st.owner = '';
    Promise.all(paths.slice(0, 50).map((p) => api.fsStat(p).catch(() => null))).then((got) => {
      st.owner = [...new Set(got.map((k) => k?.owner).filter(Boolean))].join(', ');
    });
  }
  // Folders count what lies below them from the index, so the size and the numbers are there without a walk.
  const folders = nodes.map((k, i) => [k, paths[i]!] as const).filter(([k]) => k.dir && !k.link);
  if (folders.length && !inArchive(dir)) {
    Promise.all(folders.map(([, p]) => tools.diskUsage(p, { largest: 0 }).catch(() => null))).then((got) => {
      if (got.some((u) => !u)) return;
      const one = nodes.length === 1;
      st.size = nodes.filter((k) => !k.dir).reduce((a, k) => a + (k.size || 0), 0) + got.reduce((a, u) => a + u!.size, 0);
      st.files = nodes.filter((k) => !k.dir).length + got.reduce((a, u) => a + u!.files, 0);
      st.dirs = got.reduce((a, u) => a + u!.dirs, 0) + (one ? 0 : folders.length);
      st.counted = true;
    });
  }
  win({
    title: t('props.props'), wide: true, cls: 'bk bk-props', body: PropsBody, props: { st },
    foot: [{ start: ro ? t('props.readOnly') : ' ' }, cancelBtn(), { label: t('props.save'), g: 'IconSave', primary: true, disabled: ro, run: () => save(st) }],
  });
}

function saveFlags(st: PropsState) {
  const flags = Object.fromEntries(Object.entries(st.flags).filter(([f, on]) => on !== st.flagsAt[f as WinFlag]));
  if (!Object.keys(flags).length && !st.rec) return true;
  const listing = useListing(), done = (n: number) => { listing.changed(st.dir); toast(t('props.flagsSaved', { objects: plural(n, 'ui.objects') })); };
  tools.attrs({ paths: st.paths, flags: st.rec ? { ...st.flags } : flags, recursive: st.rec }).then((got) => {
    if ('changed' in got) return done(got.changed);
    const jobs = useJobs(), job = jobs.upsert(got as Job);
    toast(t('ui.running', { label: jobLabel(job) }), 'ok', { label: t('ui.toJobs'), run: () => run('jobs') });
    jobs.whenDone(job, (j) => {
      if (j.state === 'done') done(j.totalFiles);
      else if (j.state === 'failed') toast(t('ui.failed', { label: jobLabel(j), error: jobError(j) }), 'fault');
    });
  }, (e) => toast(e.message, 'warn'));
  return true;
}

function save(st: PropsState) {
  if (st.ro) return false;
  if (st.win) return saveFlags(st);
  if (!/^[0-7]{4}$/.test(st.mode)) { toast(t('props.badMode'), 'warn'); return false; }
  const req = { paths: st.paths, mode: st.mode, ...(st.owner ? { owner: st.owner } : {}), ...(st.group ? { group: st.group } : {}), recursive: st.rec };
  const said = { owner: st.owner || t('props.ownerStays'), group: st.group || t('props.groupStays'), mode: st.mode.slice(1) };
  const listing = useListing();
  tools.attrs(req).then((got) => {
    if ('changed' in got) {
      listing.changed(st.dir);
      return toast(t('props.saved', { objects: plural(got.changed, 'ui.objects'), ...said }));
    }
    const jobs = useJobs(), job = jobs.upsert(got as Job);
    toast(t('ui.running', { label: jobLabel(job) }), 'ok', { label: t('ui.toJobs'), run: () => run('jobs') });
    jobs.whenDone(job, (j) => {
      listing.changed(st.dir);
      if (j.state === 'done') toast(t('props.saved', { objects: plural(j.totalFiles, 'ui.objects'), ...said }));
      else if (j.state === 'failed') toast(t('ui.failed', { label: jobLabel(j), error: jobError(j) }), 'fault');
    });
  }, (e) => toast(e.message, 'warn'));
  return true;
}

export default function setup() {
  action('symlink', { label: tl('props.symlink'), g: 'IconLink', needs: 'sel', run: symlinkWindow });
  action('props', { label: tl('props.props'), g: 'IconSliders', key: 'Alt+Enter', run: () => { void propsWindow(); } });
  kindLabels.attrsRecursive = (_j, what) => t('props.jobAttrs', { what });
}
