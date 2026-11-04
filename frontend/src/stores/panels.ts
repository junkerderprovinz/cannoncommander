// The file manager's state: one or two panels, each with its own tabs, history, selection and filter.
import { defineStore } from 'pinia';
import { computed, reactive, ref, toRef, watch, watchEffect } from 'vue';
import { ApiError } from '@/api/client';
import { actions, run, setActionContext } from '@/core/actions';
import { extend } from '@/core/extend';
import { now } from '@/core/format';
import { setBookmarkEntries } from '@/core/menus';
import { hashPath, pushNav, setNavSource, syncHash } from '@/core/nav';
import { prefs, setPref } from '@/core/prefs';
import { toast } from '@/core/toast';
import { go as goView, view } from '@/core/views';
import { Columns } from '@/files/columns';
import { type Entry, groupEntries, nameMatcher, shown, sortEntries, toEntry } from '@/files/entries';
import { recordVisit } from '@/files/frequent';
import { inArchive, join, parent, split } from '@/files/paths';
import { viewOf } from '@/files/views';
import { t as tr } from '@/i18n';
import { useClipboard } from './clipboard';
import { useListing } from './listing';
import { useUser } from './user';

export interface Tab {
  path: string;
  marked: Set<string>;
  cursor: number;
  sort: string;
  dir: number;
  filter: string;
  qf: boolean;
  hist: string[];
  fwd: string[];
  editPath: boolean;
  locked?: boolean;
  pathOpen?: boolean;
  deep?: boolean;
  flat?: boolean;
  smart?: boolean;
  cacheOnly?: boolean;
}
export interface Panel { tabs: Tab[]; cur: number }
// A short word on a row, such as "kopiert", that a feature puts there and the next folder change takes away.
export interface RowMark { label: string; tone?: string }
// The kind link puts hard links instead of copies; the options are what the transfer window asked.
export interface TransferOptions { rule?: 'ask' | 'skip' | 'replace' | 'both' | 'newer'; verify?: boolean; keepAttrs?: boolean }
export interface Transfer extends TransferOptions { kind: 'copy' | 'move' | 'link'; items: Entry[]; dest: string; from: string }

const ancestors = (path: string) => split(path).map((_, k, a) => '/' + a.slice(0, k + 1).join('/')).filter((p) => !p.includes('::'));

export const newTab = (path: string): Tab => ({ path, marked: new Set(), cursor: 0, sort: 'name', dir: 1, filter: '', qf: false, hist: [], fwd: [], editPath: false });

export const usePanels = defineStore('panels', () => {
  const listing = useListing();
  const user = useUser();
  const active = ref(0);
  const panels = reactive<Panel[]>([
    { tabs: [newTab(hashPath() || prefs.startFolder)], cur: 0 },
    { tabs: [newTab('/mnt/user')], cur: 0 },
  ]);
  // The tree opens down to every folder a panel starts in.
  const openTree = reactive(new Set(['/mnt', '/mnt/user', ...panels.flatMap((p) => ancestors(p.tabs[0]!.path))]));
  const visited = ref<string[]>([]);
  const marks = reactive<Record<string, RowMark>[]>([{}, {}]);
  const clip = toRef(useClipboard(), 'clip');
  let following = false;

  const P = (i = active.value) => panels[i]!;
  const T = (i = active.value) => P(i).tabs[P(i).cur]!;
  const other = () => (prefs.dual ? 1 - active.value : active.value);

  const holdsVolumes = (path: string) => !!listing.trees.get(path)?.some((k) => k.total);
  const viewOfTab = (tab: Pick<Tab, 'path'>) => viewOf(tab.path, prefs.folderViews, holdsVolumes(tab.path));
  const setView = (tab: Pick<Tab, 'path'>, id: string) => setPref('folderViews', { ...prefs.folderViews, [tab.path]: id });
  const groupOf = (tab: Pick<Tab, 'path'>) => prefs.folderGroups?.[tab.path] || '';
  const setGroup = (tab: Pick<Tab, 'path'>, id: string) => setPref('folderGroups', { ...prefs.folderGroups, [tab.path]: id });

  // A filter searches the folders below by default; "only here" filters the rows already read.
  function source(tab: Tab): Entry[] | null {
    const keep = shown(prefs.showHidden);
    if (tab.filter && tab.deep !== false) return listing.search(tab.path, tab.filter, prefs.showHidden).items.filter(keep).map(toEntry);
    if (tab.flat) return listing.search(tab.path, '*', prefs.showHidden).items.filter((k) => keep(k) && !k.dir).map(toEntry);
    const got = listing.lists.get(tab.path);
    return got ? got.items.filter(keep).map(toEntry) : null;
  }

  const tagMatch = (path: string) => (k: Entry, word: string) => extend.tagMatchers.some((fn) => fn(k, word, path));

  function compute(i: number): Entry[] {
    const tab = T(i);
    let list = source(tab);
    if (!list) return [];
    if (tab.filter && tab.deep === false) list = list.filter((k) => nameMatcher(tab.filter, tagMatch(tab.path) as never)({ ...k, at: tab.path }));
    for (const f of extend.entryFilters) list = f(list, tab.path, i);
    for (const k of list) {
      const m = listing.metaOf(k.at || tab.path, k.name);
      if (m) k.meta = m;
    }
    sortEntries(list, { sort: tab.sort, dir: tab.dir, foldersFirst: prefs.foldersFirst, comparer: Columns.comparer(tab.sort) });
    return groupEntries(list, groupOf(tab), now());
  }
  const entriesOf = computed(() => panels.map((_, i) => compute(i)));
  const entries = (i = active.value) => entriesOf.value[i]!;
  const volume = (i = active.value) => listing.lists.get(T(i).path)?.volume;
  const loaded = (i = active.value) => {
    const tab = T(i);
    if (tab.filter && tab.deep !== false) return listing.search(tab.path, tab.filter, prefs.showHidden).done;
    if (tab.flat) return listing.search(tab.path, '*', prefs.showHidden).done;
    return listing.lists.has(tab.path);
  };

  function sel(i = active.value) {
    const tab = T(i), list = entries(i);
    const marked = list.filter((k) => tab.marked.has(k.name));
    if (marked.length) return marked;
    const c = list[tab.cursor];
    return c && !c.up ? [c] : [];
  }
  const cursorEntry = (i = active.value) => entries(i)[T(i).cursor];

  // Puts the tab into a folder at once and says whether it moved. go() calls it once the folder is read; on its own it
  // suits a folder already in the cache, such as one the test shim walks into and sets the cursor in right after.
  function enter(path: string, i = active.value, record = true): boolean {
    const tab = T(i);
    // A locked tab keeps its folder; going elsewhere from it opens the new folder in a tab beside it, as in Krusader.
    if (record && tab.locked && tab.path !== path && !following) {
      const p = P(i);
      p.tabs.splice(p.cur + 1, 0, newTab(path));
      p.cur += 1;
      enter(path, i, false);
      return false;
    }
    if (record && tab.path !== path) {
      tab.hist.push(tab.path);
      tab.fwd = [];
      if (!following) pushNav(i);
    }
    Object.assign(tab, { path, marked: new Set(), cursor: 0, filter: '', qf: false, editPath: false, flat: false });
    visited.value = [path, ...visited.value.filter((p) => p !== path)].slice(0, 20);
    if (!inArchive(path)) recordVisit(path);
    ancestors(path).forEach((p) => openTree.add(p));
    marks.splice(0, 2, {}, {});
    if (view.id !== 'files') goView('files');
    syncHash();
    extend.afterGo.forEach((fn) => fn(path, i));
    return true;
  }

  // A folder not read yet is read before the tab moves, so one that is not there leaves the panel where it was. A
  // folder in the cache opens at once, as in the prototype, and is asked for again with its ETag behind it.
  async function go(path: string, i = active.value, record = true): Promise<void> {
    for (const hook of extend.beforeGo) if ((await hook(path, i)) === false) return;
    if (listing.lists.has(path)) listing.load(path).catch(() => {});
    else {
      try {
        await listing.load(path);
      } catch (e) {
        toast(e instanceof ApiError && e.status !== 404 ? e.message : tr('files.notThere'), 'warn');
        return;
      }
    }
    const from = T(i).path;
    if (!enter(path, i, record)) return;
    // Synchronised browsing: one step down or up in this panel takes the other panel the same step, where it can follow.
    if (prefs.syncBrowse && prefs.dual && record && !following) {
      const o = T(1 - i);
      const target = parent(path) === from ? join(o.path, split(path).pop()!) : parent(from) === path ? parent(o.path) : null;
      if (target) {
        following = true;
        await go(target, 1 - i).finally(() => { following = false; });
      }
    }
  }

  async function open(i: number, k: Partial<Entry> & { up?: boolean }): Promise<unknown> {
    const tab = T(i);
    // A hit from deeper down opens where it lives, so everything after works on real paths again.
    if (k.at && k.at !== tab.path) {
      if (k.dir) return go(join(k.at, k.name!), i);
      await go(k.at, i);
      const idx = entries(i).findIndex((e) => e.name === k.name);
      T(i).cursor = Math.max(0, idx);
      return idx >= 0 ? open(i, entries(i)[idx]!) : undefined;
    }
    if (k.up) {
      const from = split(tab.path).pop()!.replace('::', '');
      await go(parent(tab.path), i);
      const idx = entries(i).findIndex((e) => e.name === from);
      if (idx >= 0) T(i).cursor = idx;
      return;
    }
    if (k.link) {
      const target = await listing.load(k.link).catch(() => null);
      if (target) return go(k.link, i);
    }
    if (k.dir) return go(join(tab.path, k.name!), i);
    if (k.archive) return go(join(tab.path, k.name + '::'), i);
    const how = prefs.dblclick;
    if (how === 'download') return run('download');
    if (how === 'open' && ['text', 'code'].includes(k.kind!) && actions.edit) return run('edit');
    return run('view');
  }

  // Copy or move by dragging; the transfer window decides what happens to names that are already there.
  async function transfer(kind: Transfer['kind'], items: Entry[], dest: string, from = T().path, options: TransferOptions = {}) {
    if (inArchive(dest) || !(await listing.load(dest).catch(() => null))) return toast(tr('files.noTarget'), 'warn');
    if (dest === from) return toast(tr('files.sameFolder'), 'warn');
    let job: Transfer | null = { kind, items, dest, from, ...options };
    for (const step of extend.transferPipeline) {
      job = step(job as never) as Transfer | null;
      if (!job) return;
    }
    return run('transfer', job);
  }

  // Every tab shown gets its folder read, also one switched to without going anywhere, and its tree for the view.
  watch(() => panels.map((p) => p.tabs[p.cur]!.path), (paths) => {
    paths.forEach((p) => {
      if (!listing.lists.has(p)) listing.load(p).catch(() => toast(tr('files.notThere'), 'warn'));
      if (!inArchive(p)) listing.tree(p).catch(() => {});
    });
  }, { immediate: true });
  watchEffect(() => listing.setShown(panels.slice(0, prefs.dual ? 2 : 1).map((p) => p.tabs[p.cur]!.path)));

  // Stepping through our own entries also changes the hash; only a hash typed or opened from outside moves the panel.
  let fromHistory = false;
  addEventListener('popstate', (e) => {
    if ((e.state as { ccNav?: number } | null)?.ccNav === undefined) return;
    fromHistory = true;
    setTimeout(() => { fromHistory = false; }, 0);
  });
  addEventListener('hashchange', () => {
    const p = hashPath();
    if (fromHistory || !p || p === T().path) return;
    go(p);
  });

  setActionContext({ sel: () => sel(), here: () => T().path });
  setNavSource({ hash: () => T().path, setActive: (p) => { active.value = p; } });
  setBookmarkEntries(() => user.bookmarks.map((b) => ({ label: b.name, g: b.g, run: () => { goView('files'); go(b.path); } })));

  return {
    active, panels, openTree, visited, marks, clip, P, T, other, entries, volume, loaded, sel, cursorEntry, enter, go, open, transfer,
    viewOf: viewOfTab, setView, groupOf, setGroup,
  };
});
