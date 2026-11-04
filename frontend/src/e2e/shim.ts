// The prototype's globals in the test build. The step files in prototype/tools reach into the page through CC,
// Files, FS, Jobs, DATA and a few helpers; this file puts the same names on window and answers them from the
// core services and the stores, so one step file drives the prototype and the app alike.
// main.ts loads it only when VITE_E2E is 1. Parts that later packages build announce themselves with provide().
import {
  actions, applyPrefs, blocked, closeMenu, closeWin, fmtSize, go as goView, keymap, MENUS, prefs, run, setPref, view,
  winStack,
} from '@/core';
import { DAY } from '@/core/format';
import { provideOps } from './ops';
import { devOf, provideUnraid } from './unraid';
import { provideTools } from './tools';
import { menuName } from '@/core/menus';
import { orderedTabs, settingsTabs } from '@/core/views';
import { type Action, labelOf } from '@/core/actions';
import { glyph, kindOf } from '@/core/glyphs';
import { BASE } from '@/api/client';
import type { Activity, Session, Source } from '@/api/admin';
import type { Share } from '@/api/tools';
import { device } from '@/features/config/sessions';
import { topFolders } from '@/files/frequent';
import { type Clip, useClipboard } from '@/stores/clipboard';
import { byName, type Colour, type Label, type Tag, useLabels } from '@/stores/labels';
import type { RowMark } from '@/stores/panels';
import { COLUMNS, Columns, DEFAULT_COLUMNS } from '@/files/columns';
import { join, parent, split } from '@/files/paths';
import type { Item } from '@/api/client';
import { useListing } from '@/stores/listing';
import { newTab, usePanels } from '@/stores/panels';
import { useTrash } from '@/stores/trash';
import { useUser } from '@/stores/user';

// The moment the prototype's made-up data and the fixture on disk were made; the parity runner sets the page clock to it.
const NOW = Date.UTC(2026, 10, 2, 9, 30);

interface FsNode extends Item {
  children?: FsNode[];
}

type Bag = Record<string, unknown>;
// Members handed in before the shim runs wait here; afterwards they go straight onto the live objects.
const waiting: Record<string, Bag> = {};
const live: Record<string, Bag> = {};

/**
 * Hands the shim a global or a member of one, for the parts the prototype has and the app builds later:
 * provide('Jobs', jobs), provide('CC.unraid', unraid), provide('DATA.shares', shares).
 */
export function provide(name: string, value: unknown) {
  const [owner, member] = name.split('.') as [string, string?];
  if (member) (live[owner] ?? (waiting[owner] ??= {}))[member] = value;
  else Object.defineProperty(window, owner, { value, configurable: true, writable: true });
}

// The prototype's FS over the listing cache, built on demand, so a folder the app has read is a folder FS knows.
// A node also carries the disk it lies on, as the prototype's do, read only when a step asks for it.
const pathOf = new WeakMap<object, string>();
function fsOver(listing: ReturnType<typeof useListing>) {
  const nodeOf = (item: Item, path: string): FsNode => {
    const node: FsNode = item.dir
      ? { ...item, get children() { return (listing.lists.get(path)?.items || []).map((k) => nodeOf(k, join(path, k.name))); } }
      : { ...item, get text() { return getNow<{ text: string }>(`/fs/text?path=${encodeURIComponent(path)}&max=1048576`)?.text; } };
    pathOf.set(node, path);
    return Object.defineProperty(node, 'dev', { get: () => devOf(path) });
  };
  const root = nodeOf({ name: '', dir: true, size: 0, time: 0, mode: '0755', owner: 'root', group: 'root' }, '/');
  const FS = {
    root,
    split,
    join,
    parent,
    nodeAt(path: string): FsNode | null {
      let node: FsNode | undefined = root;
      for (const part of split(path)) {
        node = node.children?.find((k) => k.name === part);
        if (!node) return null;
      }
      return node;
    },
    kids: (n: FsNode | null) => n?.children || [],
    totalSize: (n: FsNode): number => (n.dir ? FS.kids(n).reduce((a, k) => a + FS.totalSize(k), 0) : n.size || 0),
    inArchive: (path: string) => path.includes('::'),
  };
  return FS;
}

// The step files read folders synchronously, as the prototype keeps its whole tree in memory, so every folder below
// the root is read before the first step runs.
async function readAll(listing: ReturnType<typeof useListing>, path = '/') {
  const got = await listing.load(path).catch(() => null);
  for (const k of got?.items || []) if (k.dir && !k.link) await readAll(listing, join(path, k.name));
}

// The step files read some lists at once from the prototype's memory; here they come from the server the same way.
function getNow<T>(path: string): T | undefined {
  const req = new XMLHttpRequest();
  req.open('GET', BASE + path, false);
  req.send();
  return req.status === 200 ? (JSON.parse(req.responseText) as T) : undefined;
}

// GlimStone names some parts of its menus otherwise than the prototype did, and the step files still look for the old
// names: the pencil in a heading and a row that opens a submenu.
function aliasMenuParts() {
  const alias = () => {
    document.querySelectorAll<HTMLElement>('.glim-menu-head-edit').forEach((el) => {
      el.classList.add('hedit');
      el.dataset.tip ||= el.getAttribute('aria-label') || el.getAttribute('title') || '';
    });
    document.querySelectorAll<HTMLElement>('.glim-menu [aria-haspopup="menu"]').forEach((el) => { el.dataset.sub ??= ''; });
  };
  new MutationObserver(alias).observe(document.body, { childList: true, subtree: true });
}

export async function installShim() {
  aliasMenuParts();
  const listing = useListing(), panels = usePanels(), user = useUser(), clipboard = useClipboard(), labels = useLabels();
  provideOps(provide);
  provideUnraid(provide, (n) => pathOf.get(n));
  provideTools(provide);
  await readAll(listing);
  // The prototype starts with two tabs on the left and the backups on the right, and the step files count on it.
  [['/mnt/user/downloads', '/mnt/user/fotos/2026'], ['/mnt/user/backups']].forEach((paths, i) => Object.assign(panels.P(i), { tabs: paths.map(newTab), cur: 0 }));
  panels.openTree.clear();
  for (const p of ['/mnt', '/mnt/user']) panels.openTree.add(p);

  // The prototype's actions carry their label as text; the app's work it out in the language of the moment.
  // The plumbing the app runs as actions with what a caller hands it stays out of the list the step files walk.
  const plumbing = new Set(['transfer', 'uploadFiles']);
  const shownActions = new Proxy(actions, {
    get: (all, id) => { const a = Reflect.get(all, id) as Action | undefined; return a && typeof a === 'object' ? { ...a, label: labelOf(a) } : a; },
    ownKeys: (all) => Reflect.ownKeys(all).filter((id) => !plumbing.has(id as string)),
  });
  const CC = {
    get actions() { return shownActions; },
    run,
    blocked,
    keymap,
    prefs,
    setPref,
    applyPrefs,
    // Vue draws on its own after every change; the step files still ask for it after setting state by hand.
    render: () => {},
    closeWin,
    go: goView,
    get view() { return view.id; },
    orderedTabs: () => orderedTabs(settingsTabs),
    get settingsTab_() { return view.id === 'settings' ? document.querySelector<HTMLElement>('[data-stab][aria-selected="true"]')?.dataset.stab : undefined; },
    topFolders: (n?: number, skip?: string) => topFolders(n, skip),
    admin: { ddVal: (el: HTMLElement | null) => el?.dataset.val },
    get signedIn() { return !!user.me.name; },
    ...waiting.CC,
  };

  const Files = {
    get active() { return panels.active; },
    set active(i: number) { panels.active = i; },
    T: panels.T,
    P: panels.P,
    entries: panels.entries,
    sel: panels.sel,
    cursorEntry: panels.cursorEntry,
    get openTree() { return panels.openTree; },
    get clip() { return clipboard.clip; },
    set clip(c: Clip | null) { clipboard.clip = c; },
    get marks() { return panels.marks; },
    set marks(m: Record<string, RowMark>[]) { panels.marks.splice(0, panels.marks.length, ...m); },
    // The prototype changes folder at once; the store waits for the server. A folder already read changes at once
    // here too, so a step that sets the cursor right after going there sets it in the new folder.
    go(path: string, i = panels.active) {
      if (listing.lists.has(path)) panels.enter(path, i);
      else void panels.go(path, i);
    },
    ...waiting.Files,
  };

  const DATA = {
    get bookmarks() { return user.bookmarks; },
    get trash() { return useTrash().items; },
    labels: new Proxy({}, {
      get: (_, path: string) => {
        const l = getNow<{ items: Record<string, Label> }>('/labels/items?path=' + encodeURIComponent(parent(path)))?.items[split(path).pop()!];
        return l && { color: l.colour, tags: l.tags || [] };
      },
    }),
    get sources() { return getNow<{ sources: Source[] }>('/admin/sources')?.sources ?? []; },
    get sessions() { return (getNow<{ items: Session[] }>('/me/sessions')?.items ?? []).map((s) => ({ ...s, device: device(s.userAgent).name })); },
    get activity() { return getNow<{ items: Activity[] }>('/admin/activity?limit=500')?.items ?? []; },
    get shares() { return (getNow<Share[]>('/shares') ?? []).map((s) => ({ ...s, path: s.paths[0] })); },
    ...waiting.DATA,
  };

  Object.assign(live, { CC, Files, DATA });
  const globals: Record<string, unknown> = {
    CC,
    Files,
    DATA,
    FS: { ...fsOver(listing), kindOf },
    Columns,
    COLUMNS,
    viewOf: panels.viewOf,
    DEFAULT_COLUMNS,
    // The step files find a menu by its German title, as the prototype names them; the id stands third.
    MENUS: MENUS.map(([id, items], i) => [menuName(i), items, id]),
    winStack,
    closeMenu,
    $: (s: string, r: ParentNode = document) => r.querySelector(s),
    $$: (s: string, r: ParentNode = document) => [...r.querySelectorAll(s)],
    fmtSize,
    esc: (s: unknown) => String(s ?? '').replace(/[&<>"]/g, (c) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;' })[c]!),
    g: (n: string) => { const d = glyph(n); return `<svg viewBox="${d.viewBox}" fill="currentColor" aria-hidden="true">${d.svg}</svg>`; },
    NOW,
    DAY,
  };
  for (const [name, value] of Object.entries(globals)) Object.defineProperty(window, name, { value, configurable: true, writable: true });
  // The colours and tags as the server keeps them right now, in the prototype's shapes.
  const labelsNow = () => getNow<{ colours: Colour[]; tags: Tag[] }>('/labels') ?? { colours: [], tags: [] };
  Object.defineProperty(window, 'LABEL_COLORS', { configurable: true, get: () => labelsNow().colours.map((c) => [c.id, labels.colourName(c), c.hex]) });
  Object.defineProperty(window, 'Labels', { configurable: true, value: { tags: () => labelsNow().tags.map((x) => x.name).sort(byName) } });
  document.documentElement.dataset.e2e = 'ready';
}
