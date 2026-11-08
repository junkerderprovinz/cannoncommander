// Folder contents, the folder tree, metadata columns and search hits, cached by path. A copy is asked for again with
// its ETag, and an fs.changed event about a folder on screen reads it again at once.
import { defineStore } from 'pinia';
import { reactive, shallowReactive } from 'vue';
import { api, ApiError, type Item, type Listing } from '@/api/client';
import { on, type ServerEvent, watch } from '@/api/sse';
import type { TreeItem } from '@/api/types';
import { windowsPaths } from '@/core/host';
import { driveName } from '@/files/drives';
import type { MetaValue } from '@/files/entries';
import { join, parent, split } from '@/files/paths';
import { t } from '@/i18n';

// unknown names the field tests of the query the server does not know, such as farbe in farbe:rot.
export interface Hits { items: Item[]; more: boolean; done: boolean; unknown?: string[] }

const SIZE_POLL = 30_000;

export const useListing = defineStore('listing', () => {
  const lists = reactive(new Map<string, Listing>());
  const trees = reactive(new Map<string, TreeItem[]>());
  const meta = shallowReactive(new Map<string, Record<string, MetaValue>>());
  const hits = reactive(new Map<string, Hits>());
  const texts = reactive(new Map<string, string>());
  // Folders the server said are not there or not for this user, with its answer, until one reads fine again.
  const missing = reactive(new Map<string, ApiError>());
  const pending = new Map<string, Promise<Listing>>();
  const pendingTrees = new Map<string, Promise<TreeItem[]>>();
  // The tree looks again whenever another folder arrives, so a folder that failed keeps its error until it or the
  // folder above it changes, and the server hears about it once.
  const failedTrees = new Map<string, unknown>();
  const asked = new Map<string, Set<string>>();
  let shown: string[] = [];
  let measuring: ReturnType<typeof setTimeout> | undefined;

  async function load(path: string): Promise<Listing> {
    if (pending.has(path)) return pending.get(path)!;
    const have = lists.get(path);
    const job = api.fsList(path, have?.etag)
      .then((fresh) => {
        const next = fresh ?? have!;
        lists.set(path, next);
        failedTrees.delete(path);
        missing.delete(path);
        if (shown.includes(path)) awaitSizes();
        return next;
      }, (e: unknown) => {
        if (e instanceof ApiError && (e.status === 404 || e.status === 403)) missing.set(path, e);
        throw e;
      })
      .finally(() => pending.delete(path));
    pending.set(path, job);
    return job;
  }

  async function tree(path: string): Promise<TreeItem[]> {
    if (trees.has(path)) return trees.get(path)!;
    if (failedTrees.has(path)) throw failedTrees.get(path);
    if (pendingTrees.has(path)) return pendingTrees.get(path)!;
    const job = api.fsTree(path)
      .then((got) => {
        const items = got.items.map((k) => (k.kind ? { ...k, label: driveName(k) } : k.path === '/net' && windowsPaths() ? { ...k, label: t('files.network') } : k));
        trees.set(path, items);
        return items;
      }, (e: unknown) => {
        failedTrees.set(path, e);
        throw e;
      })
      .finally(() => pendingTrees.delete(path));
    pendingTrees.set(path, job);
    return job;
  }

  function invalidate(path: string) {
    lists.delete(path);
    trees.delete(path);
    for (const p of [...failedTrees.keys()]) if (p === path || parent(p) === path) failedTrees.delete(p);
    for (const key of [...hits.keys()]) if (key.startsWith(path + '\n') || path.startsWith(key.split('\n')[0]! + '/')) hits.delete(key);
    for (const p of [...meta.keys()]) if (parent(p) === path) { meta.delete(p); asked.delete(p); }
  }

  // A folder on screen is read again in place, so the rows change without a blank moment; one out of sight just
  // drops its copy.
  function changed(path: string) {
    const visible = shown.includes(path);
    const keep = lists.get(path);
    invalidate(path);
    if (visible) {
      if (keep) lists.set(path, keep);
      load(path).catch(() => lists.delete(path));
    }
    if (trees.has(parent(path)) || shown.includes(parent(path))) {
      trees.delete(parent(path));
      tree(parent(path)).catch(() => {});
    }
  }

  // A feature may hold a change on screen back, such as while a window is open, and hand it to changed() later.
  let gate = (_path: string) => true;
  const setGate = (fn: (path: string) => boolean) => { gate = fn; };
  const pathOf = (e: ServerEvent) => (e.topic?.startsWith('fs:') ? e.topic.slice(3) : '');
  on('fs.changed', (e) => {
    const paths = (e.data as { paths?: string[] } | undefined)?.paths ?? [pathOf(e)];
    paths.filter((p) => p && (!shown.includes(p) || gate(p))).forEach(changed);
  });
  on('resync', () => [...lists.keys()].forEach(changed));

  // The panels say which folders they show; those are the ones the server reports changes for.
  function setShown(paths: string[]) {
    shown = [...new Set(paths)];
    watch(shown);
    awaitSizes();
  }

  // The index measures a folder long after it is first listed and says nothing when it is done, so a folder on screen
  // with sizes still to come is asked for again now and then until they are in. The ETag keeps an unchanged answer
  // small.
  function awaitSizes() {
    clearTimeout(measuring);
    const open = () => shown.filter((p) => lists.get(p)?.items.some((k) => k.sizeUnknown));
    if (!open().length) return;
    measuring = setTimeout(() => { Promise.allSettled(open().map(load)).then(awaitSizes); }, SIZE_POLL);
  }

  // Asks fs/meta only for the pairs of path and column it has not asked for yet; the rows on screen come in one go.
  const batch = { paths: new Set<string>(), cols: new Set<string>(), queued: false };
  function wantMeta(paths: string[], cols: string[]) {
    if (!cols.length) return;
    for (const p of paths) {
      const had = asked.get(p) ?? new Set<string>();
      const missing = cols.filter((c) => !had.has(c));
      if (!missing.length) continue;
      missing.forEach((c) => had.add(c));
      asked.set(p, had);
      batch.paths.add(p);
      missing.forEach((c) => batch.cols.add(c));
    }
    if (!batch.paths.size || batch.queued) return;
    batch.queued = true;
    queueMicrotask(() => {
      const ps = [...batch.paths], cs = [...batch.cols];
      Object.assign(batch, { paths: new Set(), cols: new Set(), queued: false });
      api.fsMeta(ps, cs).then((got) => {
        for (const [p, values] of Object.entries(got)) meta.set(p, { ...meta.get(p), ...values });
      }, () => {
        // Asked again on the next look, so a server that just came back fills the columns.
        for (const p of ps) asked.delete(p);
      });
    });
  }
  const metaOf = (folder: string, name: string) => meta.get(join(folder, name));

  // Search hits by folder and query; a newer query for the same folder stops the one still running.
  const running = new Map<string, AbortController>();
  function search(path: string, query: string, hidden: boolean): Hits {
    const key = `${path}\n${query}\n${hidden}`;
    const have = hits.get(key);
    if (have) return have;
    running.get(path)?.abort();
    const stop = new AbortController();
    running.set(path, stop);
    const state: Hits = { items: [], more: false, done: false };
    hits.set(key, state);
    api.search(path, query, hidden, stop.signal).then((got) => {
      hits.set(key, { ...got, done: true });
    }, () => {
      if (!stop.signal.aborted) hits.set(key, { items: [], more: false, done: true });
      else hits.delete(key);
    });
    return hits.get(key)!;
  }

  // The first lines of a text file, for its card.
  function text(path: string): string | undefined {
    if (!texts.has(path)) {
      texts.set(path, '');
      api.fsText(path, 16).then((got) => texts.set(path, got), () => {});
    }
    return texts.get(path);
  }

  // The name a place goes by, such as Shares for /mnt/user; plain folders have none.
  function placeLabel(path: string): string | undefined {
    for (const items of trees.values()) {
      const hit = items.find((x) => x.path === path);
      if (hit?.label) return hit.label;
    }
    for (const l of lists.values()) if (l.volume?.path === path) return l.volume.label;
    // A folder Explorer names in the language of the system, such as Dokumente for Documents.
    return lists.get(parent(path))?.items.find((k) => k.name === split(path).pop())?.label;
  }
  const placeOf = (path: string) => trees.get(parent(path))?.find((x) => x.path === path);

  return { lists, trees, meta, hits, missing, load, tree, invalidate, changed, setGate, setShown, wantMeta, metaOf, search, text, placeLabel, placeOf };
});
