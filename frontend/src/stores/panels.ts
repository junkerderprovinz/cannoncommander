// The file manager's state: one or two panels, each with its own tabs, history, selection and filter.
import { defineStore } from 'pinia';
import { computed, reactive, ref, watch } from 'vue';
import { ApiError } from '@/api/client';
import { actions, run, setActionContext } from '@/core/actions';
import { extend } from '@/core/extend';
import { setBookmarkEntries } from '@/core/menus';
import { hashPath, pushNav, setNavSource, syncHash } from '@/core/nav';
import { prefs } from '@/core/prefs';
import { toast } from '@/core/toast';
import { go as goView, view } from '@/core/views';
import { Columns } from '@/files/columns';
import { type Entry, nameMatcher, shown, sortEntries, toEntry } from '@/files/entries';
import { join, parent, split } from '@/files/paths';
import { t as tr } from '@/i18n';
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
}
export interface Panel { tabs: Tab[]; cur: number }

export const newTab = (path: string): Tab => ({ path, marked: new Set(), cursor: 0, sort: 'name', dir: 1, filter: '', qf: false, hist: [], fwd: [], editPath: false });

export const usePanels = defineStore('panels', () => {
  const listing = useListing();
  const user = useUser();
  const active = ref(0);
  const panels = reactive<Panel[]>([
    { tabs: [newTab(hashPath() || prefs.startFolder)], cur: 0 },
    { tabs: [newTab('/mnt/user')], cur: 0 },
  ]);
  const openTree = reactive(new Set(['/mnt', '/mnt/user']));
  const visited = ref<string[]>([]);
  let following = false;

  const P = (i = active.value) => panels[i]!;
  const T = (i = active.value) => P(i).tabs[P(i).cur]!;
  const other = () => (prefs.dual ? 1 - active.value : active.value);

  function compute(i: number): Entry[] {
    const t = T(i);
    const got = listing.lists.get(t.path);
    if (!got) return [];
    let list = got.items.filter(shown(prefs.showHidden)).map(toEntry);
    if (t.filter) list = list.filter(nameMatcher(t.filter));
    for (const f of extend.entryFilters) list = f(list, t.path, i);
    return sortEntries(list, { sort: t.sort, dir: t.dir, foldersFirst: prefs.foldersFirst, comparer: Columns.comparer(t.sort) });
  }
  const entriesOf = computed(() => panels.map((_, i) => compute(i)));
  const entries = (i = active.value) => entriesOf.value[i]!;
  const volume = (i = active.value) => listing.lists.get(T(i).path)?.volume;
  const loaded = (i = active.value) => listing.lists.has(T(i).path);

  function sel(i = active.value) {
    const t = T(i), list = entries(i);
    const marked = list.filter((k) => t.marked.has(k.name));
    if (marked.length) return marked;
    const c = list[t.cursor];
    return c && !c.up ? [c] : [];
  }
  const cursorEntry = (i = active.value) => entries(i)[T(i).cursor];

  // The folder is read before the tab moves, so a folder that is not there leaves the panel where it was.
  async function go(path: string, i = active.value, record = true): Promise<void> {
    for (const hook of extend.beforeGo) if ((await hook(path, i)) === false) return;
    try {
      await listing.load(path);
    } catch (e) {
      toast(e instanceof ApiError && e.status !== 404 ? e.message : tr('files.notThere'), 'warn');
      return;
    }
    const t = T(i), from = t.path;
    // A locked tab keeps its folder; going elsewhere from it opens the new folder in a tab beside it, as in Krusader.
    if (record && t.locked && t.path !== path && !following) {
      const p = P(i);
      p.tabs.splice(p.cur + 1, 0, newTab(path));
      p.cur += 1;
      return go(path, i, false);
    }
    if (record && t.path !== path) {
      t.hist.push(t.path);
      t.fwd = [];
      if (!following) pushNav(i);
    }
    Object.assign(t, { path, marked: new Set(), cursor: 0, filter: '', qf: false, editPath: false, flat: false });
    visited.value = [path, ...visited.value.filter((p) => p !== path)].slice(0, 20);
    // Synchronised browsing: one step down or up in this panel takes the other panel the same step, where it can follow.
    if (prefs.syncBrowse && prefs.dual && record && !following) {
      const o = T(1 - i);
      const target = parent(path) === from ? join(o.path, split(path).pop()!) : parent(from) === path ? parent(o.path) : null;
      if (target) {
        following = true;
        await go(target, 1 - i).finally(() => { following = false; });
      }
    }
    split(path).forEach((_, k, a) => {
      const p = '/' + a.slice(0, k + 1).join('/');
      if (!p.includes('::')) openTree.add(p);
    });
    if (view.id !== 'files') goView('files');
    syncHash();
    extend.afterGo.forEach((fn) => fn(path, i));
  }

  async function open(i: number, k: Partial<Entry> & { up?: boolean }): Promise<unknown> {
    const t = T(i);
    // A hit from deeper down opens where it lives, so everything after works on real paths again.
    if (k.at && k.at !== t.path) {
      if (k.dir) return go(join(k.at, k.name!), i);
      await go(k.at, i);
      const idx = entries(i).findIndex((e) => e.name === k.name);
      T(i).cursor = Math.max(0, idx);
      return idx >= 0 ? open(i, entries(i)[idx]!) : undefined;
    }
    if (k.up) {
      const from = split(t.path).pop()!.replace('::', '');
      await go(parent(t.path), i);
      const idx = entries(i).findIndex((e) => e.name === from);
      if (idx >= 0) T(i).cursor = idx;
      return;
    }
    if (k.link) {
      const target = await listing.load(k.link).catch(() => null);
      if (target) return go(k.link, i);
    }
    if (k.dir) return go(join(t.path, k.name!), i);
    const how = prefs.dblclick;
    if (how === 'download') return run('download');
    if (how === 'open' && ['text', 'code'].includes(k.kind!) && actions.edit) return run('edit');
    return run('view');
  }

  // Every tab shown gets its folder read, also one switched to without going anywhere.
  watch(() => panels.map((p) => p.tabs[p.cur]!.path), (paths) => {
    paths.forEach((p) => { if (!listing.lists.has(p)) listing.load(p).catch(() => toast(tr('files.notThere'), 'warn')); });
  }, { immediate: true });

  setActionContext({ sel: () => sel(), here: () => T().path });
  setNavSource({ hash: () => T().path, setActive: (p) => { active.value = p; } });
  setBookmarkEntries(() => user.bookmarks.map((b) => ({ label: b.name, g: b.g, run: () => { goView('files'); go(b.path); } })));

  return { active, panels, openTree, visited, P, T, other, entries, volume, loaded, sel, cursorEntry, go, open };
});

