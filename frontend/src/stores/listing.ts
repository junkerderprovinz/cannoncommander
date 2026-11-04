// Folder contents, the folder tree, metadata columns and search hits, cached by path. A copy is asked for again with
// its ETag, and an fs.changed event about a folder on screen reads it again at once.
import { defineStore } from 'pinia';
import { reactive, shallowReactive } from 'vue';
import { api, type Item, type Listing } from '@/api/client';
import { on, type ServerEvent, watch } from '@/api/sse';
import type { TreeItem } from '@/api/types';
import type { MetaValue } from '@/files/entries';
import { join, parent } from '@/files/paths';

export interface Hits { items: Item[]; more: boolean; done: boolean }

export const useListing = defineStore('listing', () => {
  const lists = reactive(new Map<string, Listing>());
  const trees = reactive(new Map<string, TreeItem[]>());
  const meta = shallowReactive(new Map<string, Record<string, MetaValue>>());
  const hits = reactive(new Map<string, Hits>());
  const texts = reactive(new Map<string, string>());
  const pending = new Map<string, Promise<Listing>>();
  const asked = new Map<string, Set<string>>();
  let shown: string[] = [];

  async function load(path: string): Promise<Listing> {
    if (pending.has(path)) return pending.get(path)!;
    const have = lists.get(path);
    const job = api.fsList(path, have?.etag)
      .then((fresh) => {
        const next = fresh ?? have!;
        lists.set(path, next);
        return next;
      })
      .finally(() => pending.delete(path));
    pending.set(path, job);
    return job;
  }

  async function tree(path: string): Promise<TreeItem[]> {
    if (trees.has(path)) return trees.get(path)!;
    const got = (await api.fsTree(path)).items;
    trees.set(path, got);
    return got;
  }

  function invalidate(path: string) {
    lists.delete(path);
    trees.delete(path);
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

  const pathOf = (e: ServerEvent) => (e.topic?.startsWith('fs:') ? e.topic.slice(3) : '');
  on('fs.changed', (e) => {
    const paths = (e.data as { paths?: string[] } | undefined)?.paths ?? [pathOf(e)];
    paths.filter(Boolean).forEach(changed);
  });
  on('resync', () => [...lists.keys()].forEach(changed));

  // The panels say which folders they show; those are the ones the server reports changes for.
  function setShown(paths: string[]) {
    shown = [...new Set(paths)];
    watch(shown);
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
      if (hit) return hit.label;
    }
    for (const l of lists.values()) if (l.volume?.path === path) return l.volume.label;
    return undefined;
  }
  const placeOf = (path: string) => trees.get(parent(path))?.find((x) => x.path === path);

  return { lists, trees, meta, hits, load, tree, invalidate, changed, setShown, wantMeta, metaOf, search, text, placeLabel, placeOf };
});
