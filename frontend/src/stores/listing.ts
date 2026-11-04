// Folder contents and the folder tree, cached by path. A copy is asked for again with its ETag, and an fs.changed
// event drops it, so the next look fetches fresh.
import { defineStore } from 'pinia';
import { reactive } from 'vue';
import { api, type Listing } from '@/api/client';
import { on } from '@/api/sse';
import type { TreeItem } from '@/api/types';

export const useListing = defineStore('listing', () => {
  const lists = reactive(new Map<string, Listing>());
  const trees = reactive(new Map<string, TreeItem[]>());
  const pending = new Map<string, Promise<Listing>>();

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
  }

  on('fs.changed', (data) => {
    const paths = (data as { paths?: string[] } | null)?.paths ?? [];
    paths.forEach(invalidate);
  });

  // The name a place goes by, such as Shares for /mnt/user; plain folders have none.
  function placeLabel(path: string): string | undefined {
    for (const items of trees.values()) {
      const hit = items.find((x) => x.path === path);
      if (hit) return hit.label;
    }
    for (const l of lists.values()) if (l.volume?.path === path) return l.volume.label;
    return undefined;
  }

  return { lists, trees, load, tree, invalidate, placeLabel };
});
