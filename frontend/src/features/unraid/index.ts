// Unraid underneath /mnt/user: the share's storage in the panel foot, files still on the cache, sleeping disks in the
// tree and before a read, the parity run in the sidebar, and the rights repair. Nothing shows off Unraid.
import { h, reactive, watchEffect } from 'vue';
import { api } from '@/api/client';
import { on } from '@/api/sse';
import { type PermsResult, type PermsScan, unraid } from '@/api/unraid';
import { action, run } from '@/core/actions';
import { addTo } from '@/core/extend';
import { MENUS } from '@/core/menus';
import { toast } from '@/core/toast';
import { go as goView, view } from '@/core/views';
import { closeBtn, win } from '@/core/windows';
import { COL } from '@/files/columns';
import type { Entry } from '@/files/entries';
import { join, split } from '@/files/paths';
import { t, tl } from '@/i18n';
import { diskName } from '@/features/jobs/label';
import { useJobs } from '@/stores/jobs';
import { useListing } from '@/stores/listing';
import { usePanels } from '@/stores/panels';
import { useUser } from '@/stores/user';
import CacheBadge from './CacheBadge.vue';
import Moon from './Moon.vue';
import ParityRow from './ParityRow.vue';
import PermsBody, { type PermsState } from './PermsBody.vue';
import { names, useUnraid } from './state';
import StoreBadge from './StoreBadge.vue';
import WhereWell from './WhereWell.vue';
import './unraid.css';

// Opening, editing or checking a file reads it from its disk.
const READS = ['view', 'edit', 'quickLook', 'download', 'checksum'];

// The disk column for paths that have none yet, read in one go and kept in the listing like any other column.
async function diskMeta(paths: string[]) {
  const listing = useListing();
  const missing = paths.filter((p) => listing.meta.get(p)?.disk === undefined);
  if (!missing.length) return;
  const got = await api.fsMeta(missing, ['disk']).catch(() => ({} as Record<string, Record<string, never>>));
  for (const [p, values] of Object.entries(got)) listing.meta.set(p, { ...listing.meta.get(p), ...values });
}

async function wakeFor(items: Entry[], here: string) {
  const U = useUnraid();
  if (!U.disks.some((d) => d.spundown) || !U.place(here)) return;
  await diskMeta(items.map((k) => join(k.at || here, k.name)));
  U.wake(items.flatMap((k) => U.devicesOf(k.at || here, k.name) ?? []));
}

const onPool = (k: Entry, folder: string) => {
  const U = useUnraid();
  return !!U.devicesOf(k.at || folder, k.name)?.some((d) => U.pools.includes(d));
};

async function cacheOnly() {
  const F = usePanels(), tab = F.T();
  const turnOn = !tab.cacheOnly;
  if (turnOn) await diskMeta(F.entries().filter((k) => !k.up).map((k) => join(k.at || tab.path, k.name)));
  Object.assign(tab, { cacheOnly: turnOn, cursor: 0 });
  if (turnOn) toast(t('unraid.onCache', { n: F.entries().filter((k) => !k.up).length }));
}

async function spinDown() {
  const U = useUnraid();
  const awake = U.disks.filter((d) => !d.spundown).map((d) => d.name);
  try {
    const done = await unraid.spindown(awake);
    U.setAsleep(done, true);
    toast(t(done.length === 1 ? 'unraid.sleepsOne' : 'unraid.sleepsMany', { disks: names(done) }));
  } catch (e) {
    toast((e as Error).message, 'warn');
  }
}

// The window waits a moment for the count, so a folder that is left alone does not first show the form.
const SCAN_WAIT = 400;

async function fixPerms() {
  if (view.id !== 'files') goView('files');
  const F = usePanels(), listing = useListing();
  const sel = F.sel(), here = F.T().path;
  const path = sel.length === 1 && sel[0]!.dir && !sel[0]!.link ? join(sel[0]!.at || here, sel[0]!.name) : here;
  const name = listing.placeLabel(path) || split(path).pop() || t('unraid.perms.server');
  const st: PermsState = reactive({ path, name, scan: null });
  const scan = unraid.permsScan(path);
  const early = await Promise.race([scan, new Promise<null>((r) => setTimeout(() => r(null), SCAN_WAIT))]).catch((e: Error) => e);
  if (early instanceof Error) return toast(early.message, 'warn');
  st.scan = early;

  const foot = () => (st.scan?.blocked ? [closeBtn()] : [
    { start: t('unraid.perms.background') },
    { label: t('unraid.perms.start'), g: 'IconShieldOn', primary: true, disabled: !st.scan, run: () => { void start(); } },
  ]);
  const w = win({ title: t('unraid.perms.title'), wide: true, cls: 'bk u-win', body: PermsBody, props: { st }, foot: foot() });
  if (!st.scan) {
    scan.then((got: PermsScan) => {
      st.scan = got;
      w.foot = foot();
    }, (e: Error) => {
      w.close();
      toast(e.message, 'warn');
    });
  }

  async function start() {
    const jobs = useJobs();
    try {
      const job = await jobs.start({ kind: 'fixPerms', params: { path } });
      toast(t('unraid.perms.running', { name }), 'ok', { label: t('unraid.perms.toJobs'), run: () => run('jobs') });
      jobs.whenDone(job, (j) => {
        if (j.state === 'failed') return toast(t('unraid.perms.failed', { name, error: j.error || '' }), 'fault');
        if (j.state !== 'done') return;
        // Owners and modes changed in every folder below, not only in the one the server reports.
        for (const p of [...listing.lists.keys()]) if (p === path || p.startsWith(path + '/')) listing.load(p).catch(() => {});
        const r = j.result as PermsResult | undefined, n = r?.entries ?? 0, kept = r?.kept.length ?? 0;
        const what = t(n === 1 ? 'unraid.perms.objectOne' : 'unraid.perms.objectMany', { n });
        toast(kept ? t('unraid.perms.doneKept', { what, kept: t(kept === 1 ? 'unraid.perms.keptOne' : 'unraid.perms.keptMany', { n: kept }) }) : t('unraid.perms.done', { what }));
      });
    } catch (e) {
      toast((e as Error).message, 'warn');
    }
  }
}

export default function setup() {
  const U = useUnraid(), F = usePanels(), listing = useListing(), user = useUser();
  void U.load();
  on('resync', () => { void U.load(); });

  addTo('footerSlots', (i) => h(StoreBadge, { i }));
  addTo('nameBadges', (k, path) => h(CacheBadge, { k, path }));
  addTo('sideRows', () => h(ParityRow));
  addTo('treeDecorators', (path) => {
    const dev = path.match(/^\/mnt\/(disk\d+)$/)?.[1];
    return dev ? h(Moon, { dev }) : null;
  });
  addTo('propsWells', (path) => (U.place(path) ? h(WhereWell, { path }) : null));
  addTo('entryFilters', (items, path, i) => (F.T(i).cacheOnly ? items.filter((k) => k.up || onPool(k, path)) : items));
  addTo('actionWrappers', (a, inner) => (READS.includes(a.id) ? (...args: never[]) => {
    void wakeFor(F.sel(), F.T().path);
    return inner(...args);
  } : inner));
  // A tab that shows only the cache keeps doing so in the next folder, which needs that folder's disk column.
  watchEffect(() => {
    for (const p of F.panels) {
      const tab = p.tabs[p.cur]!, got = tab.cacheOnly ? listing.lists.get(tab.path) : undefined;
      if (got) listing.wantMeta(got.items.map((k) => join(tab.path, k.name)), ['disk']);
    }
  });

  // The server names the disks as Unraid does; the column calls them what the tree calls them.
  COL.disk!.value = (k) => (typeof k.meta?.disk === 'string' ? k.meta.disk.split(', ').filter(Boolean).map(diskName).join(', ') : '');

  action('cacheOnly', { label: tl('unraid.cacheOnly'), g: 'IconFilter', when: () => U.available, checked: () => !!F.T().cacheOnly, run: cacheOnly });
  action('fixPerms', { label: tl('unraid.perms.title'), g: 'IconShieldOn', when: () => user.admin(), run: fixPerms });
  action('spinDown', { label: tl('unraid.spinDown'), g: 'IconSleep', when: () => user.admin() && !!U.st?.write && U.disks.some((d) => !d.spundown), run: spinDown });

  const menu = (id: string) => MENUS.find(([m]) => m === id)![1];
  const place = (list: unknown[], id: string, after: string[]) => {
    if (list.includes(id)) return;
    const at = after.map((a) => list.indexOf(a)).find((k) => k >= 0) ?? list.length - 1;
    list.splice(at + 1, 0, id);
  };
  place(menu('view'), 'cacheOnly', ['syncBrowse']);
  place(menu('tools'), 'fixPerms', ['gather', 'diskUsage']);
  place(menu('tools'), 'spinDown', ['mounts']);
}
