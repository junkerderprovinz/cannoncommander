// Saved searches and their hits. A saved search waits in the menu of the search field and, opened, lists its
// current hits in the panel instead of the folder. Each open asks the server again.
import { defineStore } from 'pinia';
import { reactive, ref } from 'vue';
import type { Job } from '@/api/ops';
import { on } from '@/api/sse';
import { type ContentHits, type Hit, type SavedSearch, type SearchQuery, serverSearch, toHit, tools } from '@/api/tools';
import { useJobs } from './jobs';

export const BLANK: SearchQuery = { name: '', text: '', in: '/mnt/user', hidden: false, min: '', max: '', when: 'any', type: 'all' };

export interface SavedHits { items: Hit[]; total: number; done: boolean }

export const useSearch = defineStore('search', () => {
  const saved = ref<SavedSearch[]>([]);
  const hits = reactive(new Map<number, SavedHits>());
  // The last criteria of the search window, so it opens again where it was left.
  const last = ref<SearchQuery | null>(null);
  const textJobs = new Map<string, (e: ContentHits) => void>();

  const byId = (id: number | null | undefined) => saved.value.find((s) => s.id === id);

  async function load() {
    saved.value = await tools.saved();
  }
  async function save(name: string, query: SearchQuery) {
    const s = await tools.saveSearch(name, serverSearch(query));
    saved.value = [...saved.value, s];
    return s;
  }
  async function rename(s: SavedSearch, name: string) {
    const got = await tools.changeSearch({ ...s, name, query: serverSearch(s.query) });
    saved.value = saved.value.map((x) => (x.id === s.id ? got : x));
  }
  async function remove(s: SavedSearch) {
    await tools.dropSearch(s);
    saved.value = saved.value.filter((x) => x.id !== s.id);
    hits.delete(s.id);
  }
  // Undoing a removal saves the search anew; the server hands out a fresh id for it.
  const restore = (s: SavedSearch) => save(s.name, s.query);

  // A text search reads every file, so it runs as a job and its hits trickle in.
  function followText(job: Job, into: SavedHits) {
    textJobs.set(job.id, (e) => {
      into.items.push(...e.items.map(toHit));
      into.total = e.total;
      if (e.done) into.done = true;
    });
    useJobs().whenDone(job, async (j) => {
      textJobs.delete(j.id);
      const result = (j.result ?? (await tools.job(j.id).catch(() => j)).result) as { items?: Parameters<typeof toHit>[0][]; total?: number } | undefined;
      if (result?.items) Object.assign(into, { items: result.items.map(toHit), total: result.total ?? result.items.length });
      into.done = true;
    });
  }

  function refresh(s: SavedSearch): SavedHits {
    // The hits from before stay on screen until the new ones are there, so a panel showing them does not flicker.
    const was = hits.get(s.id), text = !!s.query.text?.trim();
    const into = reactive<SavedHits>({ items: was && !text ? was.items : [], total: was && !text ? was.total : 0, done: false });
    hits.set(s.id, into);
    if (text) {
      tools.findText(s.query).then((job) => followText(job, into), () => { into.done = true; });
    } else {
      tools.find(s.query, 2000).then((got) => Object.assign(into, { items: got.unknown.length ? [] : got.items, total: got.unknown.length ? 0 : got.total, done: true }),
        () => { into.done = true; });
    }
    return into;
  }
  const hitsOf = (s: SavedSearch) => hits.get(s.id) ?? refresh(s);

  on('search.hits', (e) => textJobs.get((e.data as ContentHits).job)?.(e.data as ContentHits));

  return { saved, hits, last, byId, load, save, rename, remove, restore, refresh, hitsOf };
});
