// Colours and tags on files and folders. They live on the server, keyed by path, so every user sees them. This store
// keeps the lists and the labels of each folder the interface has shown.
import { defineStore } from 'pinia';
import { reactive, ref } from 'vue';
import { call } from '@/api/client';
import { on } from '@/api/sse';
import { t } from '@/i18n';

export interface Label { colour?: string; tags?: string[] }
// An empty name stands for one of the colours the app comes with, which the interface names in its own language.
export interface Colour { id: string; name: string; hex: string; count: number }
export interface Tag { name: string; count: number }
export interface Labelled { name: string; at?: string }

const q = (params: Record<string, string>) => '?' + new URLSearchParams(params).toString();
const json = (method: string, body: unknown): RequestInit => ({ method, body: JSON.stringify(body), headers: { 'Content-Type': 'application/json' } });
const joinPath = (dir: string, name: string) => (dir === '/' ? '' : dir) + '/' + name;
const splitPath = (p: string): [string, string] => { const at = p.lastIndexOf('/'); return [p.slice(0, at) || '/', p.slice(at + 1)]; };

export const BUILT_IN = ['red', 'orange', 'yellow', 'green', 'blue', 'purple'];
export const byName = (a: string, b: string) => a.localeCompare(b, 'de');

export const useLabels = defineStore('labels', () => {
  const colours = ref<Colour[]>([]);
  const tags = ref<Tag[]>([]);
  // A folder's labels by entry name, null while they load.
  const folders = reactive(new Map<string, Record<string, Label> | null>());

  async function load() {
    const got = (await call<{ colours: Colour[]; tags: Tag[] }>('/labels')).body;
    if (!got) return;
    colours.value = got.colours;
    tags.value = [...got.tags].sort((a, b) => byName(a.name, b.name));
  }

  function inFolder(path: string) {
    if (!folders.has(path)) {
      folders.set(path, null);
      call<{ items: Record<string, Label> }>('/labels/items' + q({ path }))
        .then((r) => folders.set(path, r.body?.items ?? {}), () => folders.set(path, {}));
    }
    return folders.get(path) ?? null;
  }

  const pathOf = (k: Labelled, folder: string) => joinPath(k.at || folder, k.name);
  const of = (k: Labelled, folder: string): Label => inFolder(k.at || folder)?.[k.name] ?? {};
  const colour = (id?: string) => colours.value.find((c) => c.id === id);
  const colourName = (c: Pick<Colour, 'id' | 'name'>) => c.name || (BUILT_IN.includes(c.id) ? t(`labels.colours.${c.id}`) : c.id);
  const count = (kind: 'colour' | 'tag', name: string) => (kind === 'colour' ? colour(name)?.count : tags.value.find((x) => x.name === name)?.count) ?? 0;

  // A search word starting with # finds a tag or a colour by its name, so #rot and #prüfen both work.
  function matches(k: Labelled, word: string, folder: string) {
    const l = of(k, folder), w = word.replace(/^#/, '').toLowerCase();
    if ((l.tags || []).some((x) => x.toLowerCase() === w)) return true;
    const c = colour(l.colour);
    return !!c && (colourName(c).toLowerCase() === w || c.id === w);
  }

  // colour '' clears the colour, tags replaces the tags. The answer carries the labels after the change.
  async function set(items: Labelled[], folder: string, change: { colour?: string; tags?: string[] }) {
    const paths = items.map((k) => pathOf(k, folder));
    const got = (await call<{ items: Record<string, Label> }>('/labels/items', json('PUT', { paths, ...change }))).body;
    for (const [p, l] of Object.entries(got?.items ?? {})) {
      const [dir, name] = splitPath(p), known = folders.get(dir);
      if (!known) continue;
      if (l.colour || l.tags?.length) known[name] = l;
      else delete known[name];
    }
    await load();
  }

  async function saveColours(list: (Pick<Colour, 'name' | 'hex'> & { id?: string })[]) {
    const got = (await call<{ colours: Colour[]; tags: Tag[] }>('/labels/colours', json('PUT', { colours: list }))).body;
    if (got) colours.value = got.colours;
    forget();
  }
  async function saveTags(list: { from?: string; name: string }[]) {
    const got = (await call<{ colours: Colour[]; tags: Tag[] }>('/labels/tags', json('PUT', { tags: list }))).body;
    if (got) tags.value = [...got.tags].sort((a, b) => byName(a.name, b.name));
    forget();
  }

  // Shown folders read their labels again, so a renamed tag or a deleted colour disappears from the list at once.
  function forget(path?: string) {
    for (const p of [...folders.keys()]) {
      if (path && p !== path) continue;
      folders.delete(p);
    }
  }

  on('labels.changed', (data) => {
    const d = data as { kind?: string; path?: string } | null;
    if (d?.kind === 'items' && d.path) forget(d.path);
    else { forget(); load().catch(() => {}); }
  });

  return { colours, tags, folders, load, inFolder, of, colour, colourName, count, matches, set, saveColours, saveTags, forget };
});
